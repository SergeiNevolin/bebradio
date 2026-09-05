package http

import (
	"encoding/json"
	"net/http"
	"strings"

	"github.com/bebradio/backend-go/internal/usecase"
)

func (s *Server) handleRegister(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Email    string `json:"email"`
		Username string `json:"username"`
		Password string `json:"password"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		s.writeError(w, 400, "Invalid request body")
		return
	}

	req.Email = strings.ToLower(strings.TrimSpace(req.Email))
	req.Username = strings.TrimSpace(req.Username)

	if !strings.Contains(req.Email, "@") || !strings.Contains(req.Email, ".") {
		s.writeError(w, 400, "Invalid email")
		return
	}
	if len(req.Username) < 2 || len(req.Username) > 30 {
		s.writeError(w, 400, "Username must be 2-30 characters")
		return
	}

	user, token, err := s.auth.Register(req.Email, req.Username, req.Password)
	if err != nil {
		if be, ok := err.(*usecase.BusinessError); ok {
			s.writeError(w, be.Code, be.Message)
			return
		}
		s.writeError(w, 500, "Internal server error")
		return
	}

	s.writeJSON(w, 200, map[string]any{
		"token": token,
		"user":  user.ProfileWithEmail(),
	})
}

func (s *Server) handleLogin(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Email    string `json:"email"`
		Password string `json:"password"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		s.writeError(w, 400, "Invalid request body")
		return
	}

	req.Email = strings.ToLower(strings.TrimSpace(req.Email))

	user, token, err := s.auth.Login(req.Email, req.Password)
	if err != nil {
		if be, ok := err.(*usecase.BusinessError); ok {
			s.writeError(w, be.Code, be.Message)
			return
		}
		s.writeError(w, 500, "Internal server error")
		return
	}

	s.writeJSON(w, 200, map[string]any{
		"token": token,
		"user":  user.ProfileWithEmail(),
	})
}

func (s *Server) handleMe(w http.ResponseWriter, r *http.Request) {
	userID, ok := s.getUserRequired(r)
	if !ok {
		s.writeError(w, 401, "Not authenticated")
		return
	}

	user, err := s.auth.GetUserByID(userID)
	if err != nil {
		s.writeError(w, 401, "User not found")
		return
	}

	s.writeJSON(w, 200, map[string]any{"user": user.PublicProfile()})
}
