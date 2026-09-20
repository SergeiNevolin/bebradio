package http

import (
	"encoding/json"
	"net/http"
)

func (s *Server) handleAdminPromote(w http.ResponseWriter, r *http.Request) {
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

	var req struct {
		UserID string `json:"user_id"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.UserID == "" {
		s.writeError(w, 400, "user_id required")
		return
	}

	if err := s.user.SetRole(req.UserID, "admin"); err != nil {
		s.log.Error("promote user failed", "error", err)
		s.writeError(w, 500, "Failed to promote user")
		return
	}
	s.writeJSON(w, 200, map[string]any{"ok": true})
}

func (s *Server) handleAdminDemote(w http.ResponseWriter, r *http.Request) {
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

	var req struct {
		UserID string `json:"user_id"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.UserID == "" {
		s.writeError(w, 400, "user_id required")
		return
	}

	if err := s.user.SetRole(req.UserID, "user"); err != nil {
		s.log.Error("demote user failed", "error", err)
		s.writeError(w, 500, "Failed to demote user")
		return
	}
	s.writeJSON(w, 200, map[string]any{"ok": true})
}

func (s *Server) handleAdminSearchUsers(w http.ResponseWriter, r *http.Request) {
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

	q := r.URL.Query().Get("q")
	if q == "" {
		s.writeJSON(w, 200, []any{})
		return
	}

	users, err := s.user.SearchByUsername(q, 20)
	if err != nil {
		s.log.Error("search users failed", "error", err)
		s.writeError(w, 500, "Failed to search users")
		return
	}

	type userResp struct {
		ID       string `json:"id"`
		Username string `json:"username"`
		Role     string `json:"role"`
	}
	result := make([]userResp, 0, len(users))
	for _, u := range users {
		result = append(result, userResp{ID: u.ID, Username: u.Username, Role: u.Role})
	}
	s.writeJSON(w, 200, result)
}
