package api

import (
	"net/http"

	"github.com/leenzstra/bebradio/backend/internal/domain"
)

type registerRequest struct {
	Email    string `json:"email"`
	Username string `json:"username"`
	Password string `json:"password"`
}

type loginRequest struct {
	Email    string `json:"email"`
	Password string `json:"password"`
}

type tokenResponse struct {
	Token string         `json:"token"`
	User  domain.UserDTO `json:"user"`
}

type userResponse struct {
	User domain.UserDTO `json:"user"`
}

func (s *Server) handleRegister(w http.ResponseWriter, r *http.Request) {
	var req registerRequest
	if err := decodeJSON(r, &req); err != nil {
		s.writeServiceError(w, r, err)
		return
	}

	token, user, err := s.users.Register(r.Context(), req.Email, req.Username, req.Password)
	if err != nil {
		s.writeServiceError(w, r, err)
		return
	}
	writeJSON(w, s.logger(r), http.StatusOK, tokenResponse{Token: token, User: user.DTO(true)})
}

func (s *Server) handleLogin(w http.ResponseWriter, r *http.Request) {
	var req loginRequest
	if err := decodeJSON(r, &req); err != nil {
		s.writeServiceError(w, r, err)
		return
	}

	token, user, err := s.users.Login(r.Context(), req.Email, req.Password)
	if err != nil {
		s.writeServiceError(w, r, err)
		return
	}
	writeJSON(w, s.logger(r), http.StatusOK, tokenResponse{Token: token, User: user.DTO(true)})
}

// handleAuthMe returns the signed-in account. Unlike the other authenticated
// endpoints it answers 401 itself rather than through requireUser, because the
// browser client calls it on every page load to find out whether its stored
// token is still good.
func (s *Server) handleAuthMe(w http.ResponseWriter, r *http.Request) {
	user := currentUser(r)
	if user == nil {
		writeError(w, s.logger(r), http.StatusUnauthorized, "Not authenticated")
		return
	}
	writeJSON(w, s.logger(r), http.StatusOK, userResponse{User: user.DTO(true)})
}
