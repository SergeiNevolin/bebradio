package http

import (
	"encoding/json"
	"net/http"
	"time"

	"github.com/bebradio/backend-go/internal/domain/entity"
	"github.com/bebradio/backend-go/internal/pkg/id"
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

	rm, access, err := s.room.CreateRoom(req.Name, userID, req.Password)
	if err != nil {
		s.log.Error("create room failed", "error", err, "user_id", userID)
		s.writeError(w, 500, "Failed to create room")
		return
	}

	result := rm.ToDict()
	result["access"] = access
	s.writeJSON(w, 200, result)
}

func (s *Server) handleListRooms(w http.ResponseWriter, r *http.Request) {
	rooms, err := s.room.ListPublicRooms()
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
	rooms, err := s.room.RecentRooms(userID, 6)
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

	rm, err := s.room.GetOrLoadRoom(roomID)
	if err != nil {
		s.writeError(w, 404, "Room not found")
		return
	}

	if !s.room.HasRoomAccess(rm, userID, access) {
		s.writeJSON(w, 200, map[string]any{
			"id":            rm.ID,
			"name":          rm.Name,
			"has_password":  true,
			"locked":        true,
		})
		return
	}

	result := rm.ToDict()
	if rm.PasswordHash != nil && userID != "" && userID == rm.OwnerID {
		// Owner gets an access token for password-protected rooms
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

	rm, err := s.room.GetOrLoadRoom(roomID)
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

	if err := s.room.UpdateRoomSettings(rm, req.AllowAnonymousAdd, req.IsPrivate, req.AutoRadio, req.Password); err != nil {
		s.log.Error("update room settings failed", "error", err, "room_id", roomID)
		s.writeError(w, 500, "Failed to update room settings")
		return
	}

	s.manager.Broadcast(roomID, rm.ToDict())
	s.writeJSON(w, 200, rm.ToDict())
}

func (s *Server) handleDeleteRoom(w http.ResponseWriter, r *http.Request) {
	roomID := chi.URLParam(r, "roomID")
	userID, ok := s.getUserRequired(r)
	if !ok {
		s.writeError(w, 401, "Not authenticated")
		return
	}

	rm, err := s.room.GetOrLoadRoom(roomID)
	if err != nil {
		s.writeError(w, 404, "Room not found")
		return
	}
	if rm.OwnerID != userID {
		s.writeError(w, 403, "Only the room owner can delete the room")
		return
	}

	if err := s.room.DeleteRoom(rm); err != nil {
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

	rm, err := s.room.GetOrLoadRoom(roomID)
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

	s.writeJSON(w, 200, map[string]any{
		"room":     rm.ToDict(),
		"username": req.Username,
		"access":   access,
	})
}

func (s *Server) handleAddToQueue(w http.ResponseWriter, r *http.Request) {
	roomID := chi.URLParam(r, "roomID")
	access := r.URL.Query().Get("access")
	userID := s.getUserOptional(r)

	rm, err := s.room.GetOrLoadRoom(roomID)
	if err != nil {
		s.writeError(w, 404, "Room not found")
		return
	}

	if !s.room.HasRoomAccess(rm, userID, access) {
		s.writeError(w, 403, "This room is password protected")
		return
	}

	var req struct {
		URL      string `json:"url"`
		AddedBy  string `json:"added_by"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		s.writeError(w, 400, "Invalid request body")
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

	// Get username for added_by
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

	track := entity.TrackFromYouTube(info, addedBy)
	track.ID = id.NewHex(8)
	if track.SourceURL == "" {
		track.SourceURL = req.URL
	}

	rm.Mu.Lock()
	rm.Queue = append(rm.Queue, track)
	if track.SourceURL != "" {
		rm.RadioSeedURL = track.SourceURL
	}
	if len(rm.Queue) == 1 {
		rm.IsPlaying = true
		rm.Position = 0
		rm.LastSyncAt = time.Now()
	}
	rm.Mu.Unlock()

	go func() {
		if err := s.room.SaveTracks(rm); err != nil {
			s.log.Error("save tracks failed", "error", err, "room_id", roomID)
		}
	}()

	s.manager.Broadcast(roomID, rm.ToDict())

	result := track.ToDict()
	s.writeJSON(w, 200, result)
}

func (s *Server) handleGetLyrics(w http.ResponseWriter, r *http.Request) {
	roomID := chi.URLParam(r, "roomID")
	access := r.URL.Query().Get("access")
	userID := s.getUserOptional(r)
	lang := r.URL.Query().Get("lang")

	rm, err := s.room.GetOrLoadRoom(roomID)
	if err != nil {
		s.writeError(w, 404, "Room not found")
		return
	}

	if !s.room.HasRoomAccess(rm, userID, access) {
		s.writeError(w, 403, "This room is password protected")
		return
	}

	track := rm.CurrentTrack()
	if track == nil || track.SourceURL == "" {
		s.writeJSON(w, 200, map[string]any{
			"available": false,
			"track_id":  nil,
			"cues":      []any{},
		})
		return
	}

	subs, _ := s.media.FetchSubtitles(track.SourceURL, lang)
	cues, _ := subs["cues"].([]any)
	s.writeJSON(w, 200, map[string]any{
		"available": len(cues) > 0,
		"track_id":  track.ID,
		"lang":      subs["lang"],
		"auto":      subs["auto"],
		"cues":      cues,
	})
}

func (s *Server) handlePlayback(w http.ResponseWriter, r *http.Request) {
	roomID := chi.URLParam(r, "roomID")
	access := r.URL.Query().Get("access")
	userID := s.getUserOptional(r)

	rm, err := s.room.GetOrLoadRoom(roomID)
	if err != nil {
		s.writeError(w, 404, "Room not found")
		return
	}

	if !s.room.HasRoomAccess(rm, userID, access) {
		s.writeError(w, 403, "This room is password protected")
		return
	}

	var req struct {
		Action   string   `json:"action"`
		Position *float64 `json:"position"`
		Index    *int     `json:"index"`
	}
	json.NewDecoder(r.Body).Decode(&req)

	switch req.Action {
	case "next":
		s.playback.GoNext(rm)
	case "prev":
		s.playback.GoPrev(rm)
	case "jump":
		if req.Index != nil {
			s.playback.JumpTo(rm, *req.Index)
		}
	case "seek":
		if req.Position != nil {
			s.playback.SeekTo(rm, *req.Position)
		}
	}

	s.writeJSON(w, 200, rm.ToDict())
}
