package http

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"time"

	"github.com/bebradio/backend-go/internal/domain/entity"
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
	}
	json.NewDecoder(r.Body).Decode(&req)
	if req.Name == "" {
		req.Name = "My Room"
	}

	ctx := r.Context()
	rm, access, err := s.room.CreateRoom(ctx, req.Name, userID, req.Password)
	if err != nil {
		s.log.Error("create room failed", "error", err, "user_id", userID)
		s.writeError(w, 500, "Failed to create room")
		return
	}

	result := redisc.BuildToDict(ctx, s.rdb, rm)
	result["access"] = access
	s.writeJSON(w, 200, result)
}

func (s *Server) handleListRooms(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	rooms, err := s.room.ListPublicRooms(ctx)
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
		Password          *string `json:"password"`
	}
	json.NewDecoder(r.Body).Decode(&req)

	if err := s.room.UpdateRoomSettings(ctx, rm, req.AllowAnonymousAdd, req.IsPrivate, req.AutoRadio, req.Password); err != nil {
		s.log.Error("update room settings failed", "error", err, "room_id", roomID)
		s.writeError(w, 500, "Failed to update room settings")
		return
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

	var req struct {
		Source  string `json:"source"`
		URL     string `json:"url"`
		TrackID string `json:"track_id"`
		AddedBy string `json:"added_by"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		s.writeError(w, 400, "Invalid request body")
		return
	}

	addedBy := req.AddedBy
	if addedBy == "" {
		addedBy = "Anonymous"
	}
	if userID != "" {
		user, err := s.auth.GetUserByID(userID)
		if err == nil {
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
		redisc.SetPlaybackField(ctx, s.rdb, roomID, "radio_seed_url", track.SourceURL)
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
