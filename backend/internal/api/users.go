package api

import (
	"net/http"

	"github.com/go-chi/chi/v5"
)

type updateProfileRequest struct {
	// Both fields are pointers so an omitted key means "leave unchanged"
	// rather than "set to empty".
	Bio       *string `json:"bio"`
	AvatarURL *string `json:"avatar_url"`
}

// maxProfileFieldLen caps the free-text profile fields. They are rendered in
// other people's browsers, so they are bounded here rather than left to the
// database column.
const maxProfileFieldLen = 500

func (s *Server) handleGetOwnProfile(w http.ResponseWriter, r *http.Request) {
	user := currentUser(r)
	writeJSON(w, s.logger(r), http.StatusOK, userResponse{User: user.DTO(true)})
}

func (s *Server) handleUpdateOwnProfile(w http.ResponseWriter, r *http.Request) {
	var req updateProfileRequest
	if err := decodeJSON(r, &req); err != nil {
		s.writeServiceError(w, r, err)
		return
	}
	if err := checkLen("bio", req.Bio); err != nil {
		s.writeServiceError(w, r, err)
		return
	}
	if err := checkLen("avatar_url", req.AvatarURL); err != nil {
		s.writeServiceError(w, r, err)
		return
	}

	updated, err := s.users.UpdateProfile(r.Context(), currentUser(r).ID, req.Bio, req.AvatarURL)
	if err != nil {
		s.writeServiceError(w, r, err)
		return
	}
	writeJSON(w, s.logger(r), http.StatusOK, userResponse{User: updated.DTO(true)})
}

// handleGetProfile returns somebody else's public profile. It deliberately
// omits their email address.
func (s *Server) handleGetProfile(w http.ResponseWriter, r *http.Request) {
	user, err := s.users.ByID(r.Context(), chi.URLParam(r, "userID"))
	if err != nil {
		s.writeServiceError(w, r, err)
		return
	}
	writeJSON(w, s.logger(r), http.StatusOK, userResponse{User: user.DTO(false)})
}

func checkLen(field string, value *string) error {
	if value != nil && len(*value) > maxProfileFieldLen {
		return errBadRequest(field + " is too long")
	}
	return nil
}
