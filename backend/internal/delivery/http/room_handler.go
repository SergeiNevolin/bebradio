package http

import (
	"encoding/json"
	"net/http"
	"time"

	"github.com/bebradio/backend-go/internal/domain/entity"
	"github.com/bebradio/backend-go/internal/infrastructure/redisc"
	"github.com/bebradio/backend-go/internal/usecase"
	"github.com/go-chi/chi/v5"
)

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
		s.writeError(w, 403, "Only the room owner can delete the room")
		return
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

	access, err := s.room.JoinRoom(rm, req.Password)
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
	if req.TrackID != "" {
		lib, err := s.tracks.Get(req.TrackID, userID)
		if err != nil || lib.Source != entity.TrackSourceUpload || lib.Status != entity.TrackStatusReady {
			s.writeError(w, 400, "Track not available")
			return
		}
		// Check for duplicates in Redis queue (under lock: check+append must
		// be atomic or a double-click appends the same track twice).
		s.addMu.Lock()
		queue, _ := redisc.GetQueue(ctx, s.rdb, roomID)
		for _, t := range queue {
			if t.ID == lib.ID {
				s.addMu.Unlock()
				s.writeJSON(w, 200, t.ToDict())
				return
			}
		}
		track = entity.QueueCopyFromUpload(lib, addedBy)
		redisc.AppendTrack(ctx, s.rdb, roomID, track)

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
		return
	}

	info, err := s.media.FetchTrack(req.URL)
	if err != nil {
		s.log.Error("fetch track failed", "error", err, "url", req.URL)
		s.writeError(w, 400, "Could not fetch video info")
		return
	}

	duration, _ := info["duration"].(float64)
	if int(duration) > s.config.MaxDuration {
		s.writeError(w, 400, "Video too long")
		return
	}

	track = entity.TrackFromYouTube(info, addedBy)
	if track.ID == "" {
		s.log.Error("resolve returned no track id", "url", req.URL)
		s.writeError(w, 502, "Music service unavailable, try again")
		return
	}
	if track.SourceURL == "" {
		track.SourceURL = req.URL
	}

	// Duplicate check (under lock, same as the upload branch): without it a
	// double-click appends the same video twice and the leftover copy plays
	// later as if the skipped track "came back".
	s.addMu.Lock()
	queue, _ := redisc.GetQueue(ctx, s.rdb, roomID)
	for _, t := range queue {
		if t.ID == track.ID || (track.SourceURL != "" && t.SourceURL == track.SourceURL) {
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
