package http

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/bebradio/backend-go/internal/domain/entity"
	"github.com/bebradio/backend-go/internal/domain/repository"
	"github.com/bebradio/backend-go/internal/infrastructure/redisc"
	"github.com/bebradio/backend-go/internal/usecase"
	"github.com/go-chi/chi/v5"
)

var errTrackNotAvailable = errors.New("Track not available")

func (s *Server) handleCreateRoom(w http.ResponseWriter, r *http.Request) {
	userID, ok := s.getUserRequired(r)
	if !ok {
		s.writeError(w, 401, "Not authenticated")
		return
	}

	var req struct {
		Name     string `json:"name"`
		Password string `json:"password"`
		IsStream bool   `json:"is_stream"`
	}
	json.NewDecoder(r.Body).Decode(&req)
	if req.Name == "" {
		req.Name = "My Room"
	}

	// Streams are admin-run: only admins may create them.
	if req.IsStream {
		admin, _ := s.isAdmin(userID)
		if !admin {
			s.writeError(w, 403, "Only admins can create streams")
			return
		}
	}

	ctx := r.Context()
	rm, access, err := s.room.CreateRoom(ctx, req.Name, userID, req.Password, req.IsStream)
	if err != nil {
		s.log.Error("create room failed", "error", err, "user_id", userID)
		s.writeError(w, 500, "Failed to create room")
		return
	}

	// Streams must be discoverable by the autoadvance tick from birth:
	// create the playback hash now so the 24/7 mashup refill finds the room
	// even before the first track is queued. The tick fills the queue itself.
	if req.IsStream {
		redisc.SetPlayback(ctx, s.rdb, rm.ID, &redisc.PlaybackState{})
	}

	result := redisc.BuildToDict(ctx, s.rdb, rm)
	result["access"] = access
	s.writeJSON(w, 200, result)
}

func (s *Server) handleListRooms(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	limit := parseIntDefault(r.URL.Query().Get("limit"), 50)
	offset := parseIntDefault(r.URL.Query().Get("offset"), 0)
	if limit < 0 {
		limit = 0
	}
	if limit > 100 {
		limit = 100
	}
	if offset < 0 {
		offset = 0
	}
	rooms, err := s.room.ListPublicRooms(ctx, limit, offset)
	if err != nil {
		s.log.Error("list rooms failed", "error", err)
		s.writeError(w, 500, "Failed to list rooms")
		return
	}
	if rooms == nil {
		rooms = []map[string]any{}
	}
	s.writeJSON(w, 200, rooms)
}

func (s *Server) handleRecentRooms(w http.ResponseWriter, r *http.Request) {
	userID, ok := s.getUserRequired(r)
	if !ok {
		s.writeError(w, 401, "Not authenticated")
		return
	}
	ctx := r.Context()
	rooms, err := s.room.RecentRooms(ctx, userID, 6)
	if err != nil {
		s.log.Error("recent rooms failed", "error", err)
		s.writeError(w, 500, "Failed to get recent rooms")
		return
	}
	s.writeJSON(w, 200, rooms)
}

func (s *Server) handleRecordVisit(w http.ResponseWriter, r *http.Request) {
	roomID := chi.URLParam(r, "roomID")
	userID := s.getUserOptional(r)
	if userID == "" {
		s.writeJSON(w, 200, map[string]any{"ok": true})
		return
	}
	if err := s.room.RecordVisit(userID, roomID); err != nil {
		s.log.Error("record visit failed", "error", err)
	}
	s.writeJSON(w, 200, map[string]any{"ok": true})
}

func (s *Server) handleGetRoom(w http.ResponseWriter, r *http.Request) {
	roomID := chi.URLParam(r, "roomID")
	access := r.URL.Query().Get("access")
	userID := s.getUserOptional(r)
	ctx := r.Context()

	rm, err := s.room.GetOrLoadRoom(ctx, roomID)
	if err != nil {
		s.writeError(w, 404, "Room not found")
		return
	}

	if !s.room.HasRoomAccess(rm, userID, access) {
		s.writeJSON(w, 200, map[string]any{
			"id":           rm.ID,
			"name":         rm.Name,
			"has_password": true,
			"locked":       true,
		})
		return
	}

	result := redisc.BuildToDict(ctx, s.rdb, rm)
	if rm.PasswordHash != nil && userID != "" && userID == rm.OwnerID {
		t, err := s.room.CreateAccessToken(rm.ID)
		if err != nil {
			s.log.Error("failed to create access token", "room_id", rm.ID, "error", err)
		} else if t != "" {
			result["access"] = t
		}
	}
	s.writeJSON(w, 200, result)
}

func (s *Server) handleUpdateRoom(w http.ResponseWriter, r *http.Request) {
	roomID := chi.URLParam(r, "roomID")
	userID, ok := s.getUserRequired(r)
	if !ok {
		s.writeError(w, 401, "Not authenticated")
		return
	}

	ctx := r.Context()
	rm, err := s.room.GetOrLoadRoom(ctx, roomID)
	if err != nil {
		s.writeError(w, 404, "Room not found")
		return
	}
	if rm.OwnerID != userID {
		s.writeError(w, 403, "Only the room owner can change settings")
		return
	}

	var req struct {
		AllowAnonymousAdd *bool   `json:"allow_anonymous_add"`
		IsPrivate         *bool   `json:"is_private"`
		AutoRadio         *bool   `json:"auto_radio"`
		MashupRadio       *bool   `json:"mashup_radio"`
		Password          *string `json:"password"`
		StreamFilter      *struct {
			Q      *string `json:"q"`
			Title  *string `json:"title"`
			Artist *string `json:"artist"`
			Owner  *string `json:"owner"`
		} `json:"stream_filter"`
	}
	json.NewDecoder(r.Body).Decode(&req)

	if err := s.room.UpdateRoomSettings(ctx, rm, req.AllowAnonymousAdd, req.IsPrivate, req.AutoRadio, req.MashupRadio, req.Password); err != nil {
		s.log.Error("update room settings failed", "error", err, "room_id", roomID)
		s.writeError(w, 500, "Failed to update room settings")
		return
	}

	// Stream filter (24/7 mashup stream): author/title narrowing for the
	// library refill. Owner or admin, streams only.
	if req.StreamFilter != nil {
		if _, code, msg := s.patchStreamFilter(ctx, rm, roomID, userID,
			req.StreamFilter.Q, req.StreamFilter.Title, req.StreamFilter.Artist, req.StreamFilter.Owner); code != 0 {
			s.writeError(w, code, msg)
			return
		}
	}

	result := redisc.BuildToDict(ctx, s.rdb, rm)
	s.manager.Broadcast(roomID, result)
	s.writeJSON(w, 200, result)
}

func (s *Server) handleDeleteRoom(w http.ResponseWriter, r *http.Request) {
	roomID := chi.URLParam(r, "roomID")
	userID, ok := s.getUserRequired(r)
	if !ok {
		s.writeError(w, 401, "Not authenticated")
		return
	}

	ctx := r.Context()
	rm, err := s.room.GetOrLoadRoom(ctx, roomID)
	if err != nil {
		s.writeError(w, 404, "Room not found")
		return
	}
	if rm.OwnerID != userID {
		admin, _ := s.isAdmin(userID)
		if !admin {
			s.writeError(w, 403, "Only the room owner can delete the room")
			return
		}
	}

	if err := s.room.DeleteRoom(ctx, rm); err != nil {
		s.log.Error("delete room failed", "error", err, "room_id", roomID)
		s.writeError(w, 500, "Failed to delete room")
		return
	}
	s.writeJSON(w, 200, map[string]any{"ok": true})
}

func (s *Server) handleJoinRoom(w http.ResponseWriter, r *http.Request) {
	roomID := chi.URLParam(r, "roomID")

	var req struct {
		Username string `json:"username"`
		Password string `json:"password"`
	}
	json.NewDecoder(r.Body).Decode(&req)
	if req.Username == "" {
		req.Username = "Anonymous"
	}

	ctx := r.Context()
	rm, err := s.room.GetOrLoadRoom(ctx, roomID)
	if err != nil {
		s.writeError(w, 404, "Room not found")
		return
	}

	access, err := s.room.JoinRoom(rm, req.Password, s.getUserOptional(r))
	if err != nil {
		if be, ok := err.(*usecase.BusinessError); ok {
			s.writeError(w, be.Code, be.Message)
			return
		}
		s.log.Error("join room failed", "error", err, "room_id", roomID)
		s.writeError(w, 500, "Internal error")
		return
	}

	result := redisc.BuildToDict(ctx, s.rdb, rm)
	result["username"] = req.Username
	result["access"] = access
	s.writeJSON(w, 200, result)
}

// Queue track sources.
//
// POST /api/rooms/:id/queue accepts either form (old clients send only one):
//   - {"track_id": ...} — library-backed track (mashup/upload today). The row
//     already exists in the tracks table; any ready row is queueable
//     regardless of its source (see entity.IsQueueableLibraryTrack).
//   - {"url": ...} (+ optional {"source": "youtube"}) — external URL resolved
//     via media-service.
//
// To plug a new source in the future:
//   - library-backed (spotify import, soundcloud, ...): write ready rows into
//     the tracks table — no queue changes needed;
//   - url-resolved: add a case to resolveURLTrack below.
func (s *Server) handleAddToQueue(w http.ResponseWriter, r *http.Request) {
	roomID := chi.URLParam(r, "roomID")
	access := r.URL.Query().Get("access")
	userID := s.getUserOptional(r)
	ctx := r.Context()

	rm, err := s.room.GetOrLoadRoom(ctx, roomID)
	if err != nil {
		s.writeError(w, 404, "Room not found")
		return
	}

	if !s.room.HasRoomAccess(rm, userID, access) {
		s.writeError(w, 403, "This room is password protected")
		return
	}

	// Streams are admin-curated: only admins feed the queue.
	if rm.IsStream {
		admin, _ := s.isAdmin(userID)
		if !admin {
			s.writeError(w, 403, "Only admins can add tracks to streams")
			return
		}
	}

	var req struct {
		Source  string `json:"source"`
		URL     string `json:"url"`
		TrackID string `json:"track_id"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		s.writeError(w, 400, "Invalid request body")
		return
	}

	// added_by is server-assigned: the authenticated username, or
	// "Anonymous". A client-supplied name would allow impersonation.
	addedBy := "Anonymous"
	if userID != "" {
		if user, err := s.auth.GetUserByID(userID); err == nil {
			addedBy = user.Username
		}
	}

	var track *entity.Track
	switch {
	case req.TrackID != "":
		track, err = s.resolveLibraryTrack(req.TrackID, userID, addedBy)
		if err != nil {
			s.writeError(w, 400, err.Error())
			return
		}
	case req.URL != "":
		qtrack, qerr := s.resolveURLTrack(req.Source, req.URL, addedBy)
		if qerr != nil {
			s.writeError(w, qerr.code, qerr.msg)
			return
		}
		track = qtrack
	default:
		s.writeError(w, 400, "Provide url or track_id")
		return
	}

	s.enqueueQueueTrack(w, rm, roomID, track)
}

// resolveLibraryTrack snapshots a stored library row (mashup/upload today,
// any ready row tomorrow) into a queue entry.
func (s *Server) resolveLibraryTrack(trackID, viewerID, addedBy string) (*entity.Track, error) {
	lib, err := s.tracks.Get(trackID, viewerID)
	if err != nil || !entity.IsQueueableLibraryTrack(lib) {
		return nil, errTrackNotAvailable
	}
	return entity.QueueCopyFromLibrary(lib, addedBy), nil
}

type queueResolveError struct {
	code int
	msg  string
}

func (e *queueResolveError) Error() string { return e.msg }

// resolveURLTrack resolves an external URL into a queue entry. source is a
// hint for forward compat ("" means youtube, the only URL source today).
// A new URL provider plugs in as a new case here.
func (s *Server) resolveURLTrack(source, rawURL, addedBy string) (*entity.Track, *queueResolveError) {
	switch source {
	case "", entity.TrackSourceYouTube:
		// youtube — current and only URL-resolved source.
	default:
		return nil, &queueResolveError{code: 400, msg: "Unsupported source"}
	}

	info, err := s.media.FetchTrack(rawURL)
	if err != nil {
		s.log.Error("fetch track failed", "error", err, "url", rawURL)
		return nil, &queueResolveError{code: 400, msg: "Could not fetch video info"}
	}

	duration, _ := info["duration"].(float64)
	if int(duration) > s.config.MaxDuration {
		return nil, &queueResolveError{code: 400, msg: "Video too long"}
	}

	track := entity.TrackFromYouTube(info, addedBy)
	if track.ID == "" {
		s.log.Error("resolve returned no track id", "url", rawURL)
		return nil, &queueResolveError{code: 502, msg: "Music service unavailable, try again"}
	}
	if track.SourceURL == "" {
		track.SourceURL = rawURL
	}
	return track, nil
}

// isDuplicateQueueTrack reports whether track is already queued. Library rows
// dedupe by id; URL-resolved rows additionally by source URL (a double-click
// must not append the same video twice).
func isDuplicateQueueTrack(queued, track *entity.Track) bool {
	if queued.ID == track.ID {
		return true
	}
	if track.SourceURL != "" && queued.SourceURL == track.SourceURL {
		return true
	}
	return false
}

// enqueueQueueTrack appends track under the add lock (duplicate check + RPush
// must be atomic, otherwise a double-click adds the same track twice),
// auto-starts a lone queue, persists and broadcasts.
func (s *Server) enqueueQueueTrack(w http.ResponseWriter, rm *entity.Room, roomID string, track *entity.Track) {
	ctx := context.Background()

	// Check for duplicates in Redis queue (under lock: check+append must
	// be atomic or a double-click appends the same track twice).
	s.addMu.Lock()
	queue, _ := redisc.GetQueue(ctx, s.rdb, roomID)
	for _, t := range queue {
		if isDuplicateQueueTrack(t, track) {
			s.addMu.Unlock()
			s.writeJSON(w, 200, t.ToDict())
			return
		}
	}
	redisc.AppendTrack(ctx, s.rdb, roomID, track)
	if track.SourceURL != "" {
		redisc.SetPlaybackField(ctx, s.rdb, roomID, redisc.FieldRadioSeedURL, track.SourceURL)
	}

	queueLen, _ := redisc.GetQueueLen(ctx, s.rdb, roomID)
	if queueLen == 1 {
		ps, _ := redisc.GetPlayback(ctx, s.rdb, roomID)
		if ps == nil {
			ps = &redisc.PlaybackState{}
		}
		ps.IsPlaying = true
		ps.Position = 0
		ps.LastSyncAt = time.Now()
		redisc.SetPlayback(ctx, s.rdb, roomID, ps)
	}
	s.addMu.Unlock()

	go func() {
		if err := s.room.SaveTracks(ctx, rm); err != nil {
			s.log.Error("save tracks failed", "error", err, "room_id", roomID)
		}
	}()

	s.manager.Broadcast(roomID, redisc.BuildToDict(ctx, s.rdb, rm))
	s.writeJSON(w, 200, track.ToDict())
}

// handleImportQueueTrack saves a YouTube queue track into the bebradio
// library (admin only): the cached audio is re-uploaded through music-service
// together with the track thumbnail as cover. music-service has no
// import-from-cache endpoint, so the bytes travel through this handler
// (capped at MashupMaxSize, never buffered twice).
//
// The new row starts "processing" (202); the client polls GET /api/tracks/:id
// until it flips to ready, exactly like a regular upload.
func (s *Server) handleImportQueueTrack(w http.ResponseWriter, r *http.Request) {
	roomID := chi.URLParam(r, "roomID")
	trackID := chi.URLParam(r, "trackID")
	userID, ok := s.getUserRequired(r)
	if !ok {
		s.writeError(w, 401, "Not authenticated")
		return
	}
	admin, _ := s.isAdmin(userID)
	if !admin {
		s.writeError(w, 403, "Admin only")
		return
	}

	ctx := r.Context()
	if _, err := s.room.GetOrLoadRoom(ctx, roomID); err != nil {
		s.writeError(w, 404, "Room not found")
		return
	}

	queue, _ := redisc.GetQueue(ctx, s.rdb, roomID)
	var queued *entity.Track
	for _, t := range queue {
		if t.ID == trackID {
			queued = t
			break
		}
	}
	if queued == nil {
		s.writeError(w, 404, "Track not in queue")
		return
	}
	if entity.IsLibrarySource(queued.Source) {
		s.writeError(w, 400, "Track is already in the library")
		return
	}
	if queued.MediaID == "" || queued.SourceURL == "" {
		s.writeError(w, 400, "Track has no downloadable media")
		return
	}

	ready, err := s.media.EnsureMedia([]map[string]any{
		{"media_id": queued.MediaID, "source_url": queued.SourceURL},
	})
	if err != nil {
		s.log.Error("import ensure failed", "error", err, "track_id", trackID)
		s.writeError(w, 502, "Music service unavailable, try again")
		return
	}
	ensured := false
	for _, id := range ready {
		if id == queued.MediaID {
			ensured = true
			break
		}
	}
	if !ensured {
		s.writeError(w, 502, "Could not fetch track audio")
		return
	}

	status, _, audio, err := s.media.StreamContent(queued.MediaID, "")
	if err != nil || (status != 200 && status != 206) || len(audio) == 0 {
		s.log.Error("import stream failed", "error", err, "track_id", trackID, "status", status)
		s.writeError(w, 502, "Could not fetch track audio")
		return
	}
	if int64(len(audio)) > s.config.MashupMaxSize {
		s.writeError(w, 413, "Track too large to import")
		return
	}

	// Cover is best-effort: an unreachable thumbnail never fails the import.
	var cover io.Reader
	coverName := ""
	if data := fetchRemoteImage(queued.Thumbnail, s.config.MashupCoverMaxSize); data != nil {
		cover = bytes.NewReader(data)
		coverName = "cover.jpg"
	} else if queued.Thumbnail != "" {
		s.log.Warn("queue import cover fetch failed, continuing without cover",
			"room_id", roomID, "track_id", trackID)
	}

	imported, err := s.tracks.Import(userID, queued.Title, queued.Artist,
		fmt.Sprintf("queue-%s.m4a", queued.ID), bytes.NewReader(audio), coverName, cover)
	if err != nil {
		if be, ok := err.(*usecase.BusinessError); ok {
			s.writeError(w, be.Code, be.Message)
			return
		}
		s.log.Error("queue import failed", "error", err, "room_id", roomID, "track_id", trackID)
		s.writeError(w, 500, "Import failed")
		return
	}
	s.writeJSON(w, 202, imported.ToDict())
}

// fetchRemoteImage GETs a remote image with a timeout, capping the body at
// maxSize. Returns nil on any failure (callers treat covers as best-effort).
func fetchRemoteImage(rawURL string, maxSize int64) []byte {
	if !strings.HasPrefix(rawURL, "http://") && !strings.HasPrefix(rawURL, "https://") {
		return nil
	}
	client := &http.Client{Timeout: 30 * time.Second}
	resp, err := client.Get(rawURL)
	if err != nil {
		return nil
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil
	}
	if ct := resp.Header.Get("Content-Type"); ct != "" && !strings.HasPrefix(ct, "image/") {
		return nil
	}
	data, err := io.ReadAll(io.LimitReader(resp.Body, maxSize+1))
	if err != nil || int64(len(data)) > maxSize {
		return nil
	}
	return data
}

func (s *Server) handleGetLyrics(w http.ResponseWriter, r *http.Request) {
	roomID := chi.URLParam(r, "roomID")
	access := r.URL.Query().Get("access")
	userID := s.getUserOptional(r)
	lang := r.URL.Query().Get("lang")
	ctx := r.Context()

	rm, err := s.room.GetOrLoadRoom(ctx, roomID)
	if err != nil {
		s.writeError(w, 404, "Room not found")
		return
	}

	if !s.room.HasRoomAccess(rm, userID, access) {
		s.writeError(w, 403, "This room is password protected")
		return
	}

	track := redisc.CurrentTrack(ctx, s.rdb, roomID)
	if track == nil || track.MediaID == "" {
		s.writeJSON(w, 200, map[string]any{
			"available": false,
			"track_id":  nil,
			"cues":      []any{},
		})
		return
	}

		subs, _ := s.media.FetchSubtitles(track.MediaID, lang)
	cues, _ := subs["cues"].([]any)
	s.writeJSON(w, 200, map[string]any{
		"available": len(cues) > 0,
		"track_id":  track.ID,
		"lang":      subs["lang"],
		"auto":      subs["auto"],
		"cues":      cues,
	})
}

// Stream filter: which mashups the 24/7 stream refills from (author/title).
// GET is open to anyone with room access; PUT is owner/admin on stream rooms.

func (s *Server) handleGetStreamFilter(w http.ResponseWriter, r *http.Request) {
	roomID := chi.URLParam(r, "roomID")
	access := r.URL.Query().Get("access")
	userID := s.getUserOptional(r)
	ctx := r.Context()

	rm, err := s.room.GetOrLoadRoom(ctx, roomID)
	if err != nil {
		s.writeError(w, 404, "Room not found")
		return
	}
	if !s.room.HasRoomAccess(rm, userID, access) {
		s.writeError(w, 403, "This room is password protected")
		return
	}
	sf, _ := redisc.GetStreamFilter(ctx, s.rdb, roomID)
	if sf == nil {
		sf = &redisc.StreamFilter{}
	}
	s.writeJSON(w, 200, map[string]any{
		"q": sf.Query, "title": sf.Title, "artist": sf.Artist, "owner": sf.Owner,
	})
}

func (s *Server) handleSetStreamFilter(w http.ResponseWriter, r *http.Request) {
	roomID := chi.URLParam(r, "roomID")
	userID, ok := s.getUserRequired(r)
	if !ok {
		s.writeError(w, 401, "Not authenticated")
		return
	}
	ctx := r.Context()
	rm, err := s.room.GetOrLoadRoom(ctx, roomID)
	if err != nil {
		s.writeError(w, 404, "Room not found")
		return
	}
	var req struct {
		Q      *string `json:"q"`
		Title  *string `json:"title"`
		Artist *string `json:"artist"`
		Owner  *string `json:"owner"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		s.writeError(w, 400, "Invalid request body")
		return
	}
	cur, code, msg := s.patchStreamFilter(ctx, rm, roomID, userID, req.Q, req.Title, req.Artist, req.Owner)
	if code != 0 {
		s.writeError(w, code, msg)
		return
	}
	s.manager.Broadcast(roomID, redisc.BuildToDict(ctx, s.rdb, rm))
	s.writeJSON(w, 200, map[string]any{
		"q": cur.Query, "title": cur.Title, "artist": cur.Artist, "owner": cur.Owner,
	})
}

// patchStreamFilter applies a partial stream-filter update behind the shared
// stream permission checks (streams only, owner or admin). It returns the
// stored filter, or a non-zero HTTP status + message for the caller to
// render — both the PATCH-embedded and the dedicated PUT endpoint share it,
// so the checks cannot drift apart.
func (s *Server) patchStreamFilter(ctx context.Context, rm *entity.Room, roomID, userID string, q, title, artist, owner *string) (*redisc.StreamFilter, int, string) {
	if !rm.IsStream {
		return nil, 400, "Stream filter is only available for stream rooms"
	}
	admin, _ := s.isAdmin(userID)
	if rm.OwnerID != userID && !admin {
		return nil, 403, "Only the room owner can change the stream filter"
	}
	cur, _ := redisc.GetStreamFilter(ctx, s.rdb, roomID)
	if cur == nil {
		cur = &redisc.StreamFilter{}
	}
	if q != nil {
		cur.Query = strings.TrimSpace(*q)
	}
	if title != nil {
		cur.Title = strings.TrimSpace(*title)
	}
	if artist != nil {
		cur.Artist = strings.TrimSpace(*artist)
	}
	if owner != nil {
		cur.Owner = strings.TrimSpace(*owner)
	}
	if err := redisc.SetStreamFilter(ctx, s.rdb, roomID, cur); err != nil {
		s.log.Error("set stream filter failed", "error", err, "room_id", roomID)
		return nil, 500, "Failed to update stream filter"
	}
	return cur, 0, ""
}

// Stream refill: manually enqueue N random mashups honouring the room's
// stream filter. Owner/admin on stream rooms only. The automatic 24/7 refill
// keeps running regardless; this is the "add some randomness now" button.
func (s *Server) handleStreamRefill(w http.ResponseWriter, r *http.Request) {
	roomID := chi.URLParam(r, "roomID")
	userID, ok := s.getUserRequired(r)
	if !ok {
		s.writeError(w, 401, "Not authenticated")
		return
	}
	ctx := r.Context()
	rm, err := s.room.GetOrLoadRoom(ctx, roomID)
	if err != nil {
		s.writeError(w, 404, "Room not found")
		return
	}
	if !rm.IsStream {
		s.writeError(w, 400, "Stream refill is only available for stream rooms")
		return
	}
	admin, _ := s.isAdmin(userID)
	if rm.OwnerID != userID && !admin {
		s.writeError(w, 403, "Only the room owner can refill the stream")
		return
	}

	var req struct {
		Count int `json:"count"`
	}
	_ = json.NewDecoder(r.Body).Decode(&req)
	count := req.Count
	if count <= 0 {
		count = s.config.RadioBatch
	}
	if count <= 0 {
		count = 3
	}
	if count > 10 {
		count = 10
	}

	sf, _ := redisc.GetStreamFilter(ctx, s.rdb, roomID)
	filter := repository.TrackFilter{}
	if sf != nil {
		filter = repository.TrackFilter{Query: sf.Query, Title: sf.Title, Artist: sf.Artist, Owner: sf.Owner}
	}

	queue, _ := redisc.GetQueue(ctx, s.rdb, roomID)
	exclude := make([]string, 0, len(queue))
	queued := make(map[string]bool, len(queue))
	for _, t := range queue {
		exclude = append(exclude, t.ID)
		queued[t.ID] = true
	}

	addedBy := "Radio"
	if u, err := s.auth.GetUserByID(userID); err == nil {
		addedBy = u.Username
	}

	libs, err := s.tracks.RandomForStream(filter, count, exclude)
	if err != nil {
		s.log.Error("stream refill failed", "error", err, "room_id", roomID)
		s.writeError(w, 500, "Refill failed")
		return
	}
	if len(libs) == 0 {
		// Base smaller than the queue (or filter too narrow): loop once
		// without exclusions rather than returning nothing.
		libs, err = s.tracks.RandomForStream(filter, count, nil)
		if err != nil {
			s.log.Error("stream refill retry failed", "error", err, "room_id", roomID)
			s.writeError(w, 500, "Refill failed")
			return
		}
		kept := libs[:0]
		for _, l := range libs {
			if !queued[l.ID] {
				kept = append(kept, l)
			}
		}
		libs = kept
	}
	if len(libs) == 0 {
		s.writeError(w, 404, "No mashups match the stream filter")
		return
	}

	s.addMu.Lock()
	added := make([]map[string]any, 0, len(libs))
	for _, lib := range libs {
		if !entity.IsQueueableLibraryTrack(lib) {
			continue
		}
		track := entity.QueueCopyFromLibrary(lib, addedBy)
		// Re-check under lock: a concurrent refill may have queued it.
		dup := false
		if fresh, _ := redisc.GetQueue(ctx, s.rdb, roomID); fresh != nil {
			for _, q := range fresh {
				if isDuplicateQueueTrack(q, track) {
					dup = true
					break
				}
			}
		}
		if dup {
			continue
		}
		redisc.AppendTrack(ctx, s.rdb, roomID, track)
		if track.ID != "" {
			redisc.AddRadioSeen(ctx, s.rdb, roomID, track.ID)
		}
		added = append(added, track.ToDict())
	}
	// Ensure the stream (re)starts once tracks are queued.
	if len(added) > 0 {
		ps, _ := redisc.GetPlayback(ctx, s.rdb, roomID)
		if ps == nil {
			ps = &redisc.PlaybackState{}
		}
		if !ps.IsPlaying {
			ps.IsPlaying = true
			ps.Position = 0
			ps.LastSyncAt = time.Now()
			if len(added) > 0 {
				ps.CurrentStartedAt = time.Now()
			}
			redisc.SetPlayback(ctx, s.rdb, roomID, ps)
		}
	}
	s.addMu.Unlock()

	if len(added) == 0 {
		s.writeError(w, 404, "No mashups match the stream filter")
		return
	}
	go func() {
		if err := s.room.SaveTracks(ctx, rm); err != nil {
			s.log.Error("save tracks after stream refill failed", "error", err, "room_id", roomID)
		}
	}()

	s.manager.Broadcast(roomID, redisc.BuildToDict(ctx, s.rdb, rm))
	s.writeJSON(w, 200, map[string]any{"added": added})
}
