package api

import (
	"encoding/json"
	"net/http"
	"strings"

	"github.com/go-chi/chi/v5"

	"github.com/leenzstra/bebradio/backend/internal/domain"
	"github.com/leenzstra/bebradio/backend/internal/room"
)

const (
	defaultRoomName = "My Room"
	maxRoomNameLen  = 255
)

type createRoomRequest struct {
	Name     string  `json:"name"`
	Password *string `json:"password"`
}

type joinRoomRequest struct {
	Username string  `json:"username"`
	Password *string `json:"password"`
}

type joinRoomResponse struct {
	Room     domain.RoomDTO `json:"room"`
	Username string         `json:"username"`
	Access   string         `json:"access"`
}

type addTrackRequest struct {
	URL     string `json:"url"`
	AddedBy string `json:"added_by"`
}

type playbackRequest struct {
	Action   string   `json:"action"`
	Position *float64 `json:"position"`
	Index    *int     `json:"index"`
}

// roomSettingsRequest is a partial settings change.
//
// Password needs three states, not two: absent means "leave the password
// alone", an empty string means "remove it", and a value means "set it". A
// plain *string cannot tell an absent key from an explicit null, so the raw
// JSON is kept and decoded after the fact.
type roomSettingsRequest struct {
	AllowAnonymousAdd *bool           `json:"allow_anonymous_add"`
	IsPrivate         *bool           `json:"is_private"`
	AutoRadio         *bool           `json:"auto_radio"`
	Password          json.RawMessage `json:"password"`
}

// settingsUpdate converts the request into the service's update type.
func (req roomSettingsRequest) settingsUpdate() (room.SettingsUpdate, error) {
	update := room.SettingsUpdate{
		AllowAnonymousAdd: req.AllowAnonymousAdd,
		IsPrivate:         req.IsPrivate,
		AutoRadio:         req.AutoRadio,
	}
	if len(req.Password) == 0 {
		return update, nil
	}

	// An explicit null reads the same as an empty string: remove the password.
	if string(req.Password) == "null" {
		empty := ""
		update.Password = &empty
		return update, nil
	}
	var password string
	if err := json.Unmarshal(req.Password, &password); err != nil {
		return room.SettingsUpdate{}, errBadRequest("password must be a string")
	}
	update.Password = &password
	return update, nil
}

func (s *Server) handleCreateRoom(w http.ResponseWriter, r *http.Request) {
	var req createRoomRequest
	if err := decodeJSON(r, &req); err != nil {
		s.writeServiceError(w, r, err)
		return
	}

	name := strings.TrimSpace(req.Name)
	if name == "" {
		name = defaultRoomName
	}
	if len(name) > maxRoomNameLen {
		s.writeServiceError(w, r, errBadRequest("Room name is too long"))
		return
	}

	var password string
	if req.Password != nil {
		password = *req.Password
	}

	created, err := s.rooms.Create(r.Context(), name, currentUser(r).ID, password)
	if err != nil {
		s.writeServiceError(w, r, err)
		return
	}
	writeJSON(w, s.logger(r), http.StatusOK, created)
}

func (s *Server) handleListRooms(w http.ResponseWriter, r *http.Request) {
	rooms, err := s.rooms.ListPublic(r.Context())
	if err != nil {
		s.writeServiceError(w, r, err)
		return
	}
	writeJSON(w, s.logger(r), http.StatusOK, rooms)
}

// handleGetRoom returns a room, or -- for a password-protected room the caller
// has not unlocked -- a stripped payload carrying only its name. That case is a
// 200, not a 403: the room exists and the client is meant to prompt for the
// password rather than report an error.
func (s *Server) handleGetRoom(w http.ResponseWriter, r *http.Request) {
	snapshot, locked, err := s.rooms.View(r.Context(), chi.URLParam(r, "roomID"), callerFrom(r))
	if err != nil {
		s.writeServiceError(w, r, err)
		return
	}
	if locked != nil {
		writeJSON(w, s.logger(r), http.StatusOK, locked)
		return
	}
	writeJSON(w, s.logger(r), http.StatusOK, snapshot)
}

func (s *Server) handleUpdateRoom(w http.ResponseWriter, r *http.Request) {
	var req roomSettingsRequest
	if err := decodeJSON(r, &req); err != nil {
		s.writeServiceError(w, r, err)
		return
	}
	update, err := req.settingsUpdate()
	if err != nil {
		s.writeServiceError(w, r, err)
		return
	}

	updated, err := s.rooms.UpdateSettings(r.Context(), chi.URLParam(r, "roomID"), callerFrom(r), update)
	if err != nil {
		s.writeServiceError(w, r, err)
		return
	}
	writeJSON(w, s.logger(r), http.StatusOK, updated)
}

func (s *Server) handleDeleteRoom(w http.ResponseWriter, r *http.Request) {
	if err := s.rooms.Delete(r.Context(), chi.URLParam(r, "roomID"), callerFrom(r)); err != nil {
		s.writeServiceError(w, r, err)
		return
	}
	writeJSON(w, s.logger(r), http.StatusOK, map[string]bool{"ok": true})
}

func (s *Server) handleJoinRoom(w http.ResponseWriter, r *http.Request) {
	var req joinRoomRequest
	if err := decodeJSON(r, &req); err != nil {
		s.writeServiceError(w, r, err)
		return
	}

	var password string
	if req.Password != nil {
		password = *req.Password
	}
	username := strings.TrimSpace(req.Username)
	if username == "" {
		username = "Anonymous"
	}

	snapshot, access, err := s.rooms.Join(r.Context(), chi.URLParam(r, "roomID"), password)
	if err != nil {
		s.writeServiceError(w, r, err)
		return
	}
	writeJSON(w, s.logger(r), http.StatusOK, joinRoomResponse{
		Room:     snapshot,
		Username: username,
		Access:   access,
	})
}

func (s *Server) handleAddToQueue(w http.ResponseWriter, r *http.Request) {
	var req addTrackRequest
	if err := decodeJSON(r, &req); err != nil {
		s.writeServiceError(w, r, err)
		return
	}
	url := strings.TrimSpace(req.URL)
	if url == "" {
		s.writeServiceError(w, r, errBadRequest("A track URL is required"))
		return
	}

	track, err := s.rooms.AddTrack(r.Context(), chi.URLParam(r, "roomID"), url, callerFrom(r), req.AddedBy)
	if err != nil {
		s.writeServiceError(w, r, err)
		return
	}
	writeJSON(w, s.logger(r), http.StatusOK, track)
}

func (s *Server) handlePlayback(w http.ResponseWriter, r *http.Request) {
	var req playbackRequest
	if err := decodeJSON(r, &req); err != nil {
		s.writeServiceError(w, r, err)
		return
	}

	updated, err := s.rooms.Playback(r.Context(), chi.URLParam(r, "roomID"), callerFrom(r),
		room.PlaybackCommand{Action: req.Action, Index: req.Index, Position: req.Position})
	if err != nil {
		s.writeServiceError(w, r, err)
		return
	}
	writeJSON(w, s.logger(r), http.StatusOK, updated)
}

func (s *Server) handleLyrics(w http.ResponseWriter, r *http.Request) {
	lyrics, err := s.rooms.LyricsFor(r.Context(),
		chi.URLParam(r, "roomID"), r.URL.Query().Get("lang"), callerFrom(r))
	if err != nil {
		s.writeServiceError(w, r, err)
		return
	}
	writeJSON(w, s.logger(r), http.StatusOK, lyrics)
}
