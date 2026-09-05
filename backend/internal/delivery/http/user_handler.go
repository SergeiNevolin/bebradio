package http

import (
	"encoding/json"
	"net/http"

	"github.com/bebradio/backend-go/internal/usecase"
	"github.com/go-chi/chi/v5"
)

func (s *Server) handleGetMe(w http.ResponseWriter, r *http.Request) {
	userID, ok := s.getUserRequired(r)
	if !ok {
		s.writeError(w, 401, "Not authenticated")
		return
	}

	user, err := s.user.GetProfile(userID)
	if err != nil {
		s.writeError(w, 404, "User not found")
		return
	}

	s.writeJSON(w, 200, map[string]any{"user": user.ProfileWithEmail()})
}

func (s *Server) handleUpdateMe(w http.ResponseWriter, r *http.Request) {
	userID, ok := s.getUserRequired(r)
	if !ok {
		s.writeError(w, 401, "Not authenticated")
		return
	}

	var req struct {
		Bio       *string `json:"bio"`
		AvatarURL *string `json:"avatar_url"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		s.writeError(w, 400, "Invalid request body")
		return
	}

	user, err := s.user.UpdateProfile(userID, req.Bio, req.AvatarURL)
	if err != nil {
		s.writeError(w, 500, "Failed to update profile")
		return
	}

	s.writeJSON(w, 200, map[string]any{"user": user.ProfileWithEmail()})
}

func (s *Server) handleGetUser(w http.ResponseWriter, r *http.Request) {
	userID := chi.URLParam(r, "userID")

	user, err := s.user.GetUser(userID)
	if err != nil {
		s.writeError(w, 404, "User not found")
		return
	}

	s.writeJSON(w, 200, map[string]any{"user": user.PublicProfile()})
}

var _ = usecase.BusinessError{}
