package api

import (
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"strings"

	"github.com/leenzstra/bebradio/backend/internal/room"
	"github.com/leenzstra/bebradio/backend/internal/users"
)

// errorBody is the single error shape the API speaks. The browser client reads
// `error` from every failed response, so it is the same for a validation
// failure, a missing room and an internal fault.
type errorBody struct {
	Error string `json:"error"`
	// NeedsPassword tells the client to show the room's password prompt rather
	// than an error.
	NeedsPassword bool `json:"needs_password,omitempty"`
}

// writeJSON sends a value as JSON. A failure part-way through the body cannot
// be reported to the client -- the status line has already gone -- so it is
// logged instead.
func writeJSON(w http.ResponseWriter, log *slog.Logger, status int, payload any) {
	body, err := json.Marshal(payload)
	if err != nil {
		log.Error("encoding response", "error", err)
		http.Error(w, `{"error":"Internal server error"}`, http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	if _, err := w.Write(body); err != nil {
		log.Debug("writing response body", "error", err)
	}
}

func writeError(w http.ResponseWriter, log *slog.Logger, status int, message string) {
	writeJSON(w, log, status, errorBody{Error: message})
}

// decodeJSON reads a JSON request body into dst, rejecting anything malformed
// or oversized with a message the client can show.
func decodeJSON(r *http.Request, dst any) error {
	dec := json.NewDecoder(r.Body)
	if err := dec.Decode(dst); err != nil {
		if errors.Is(err, io.EOF) {
			return errBadRequest("Request body is required")
		}
		var maxErr *http.MaxBytesError
		if errors.As(err, &maxErr) {
			return errBadRequest("Request body is too large")
		}
		return errBadRequest("Request body is not valid JSON")
	}
	// A second JSON value in the body means the client sent something other
	// than the single object this endpoint expects.
	if dec.More() {
		return errBadRequest("Request body must contain a single JSON object")
	}
	return nil
}

// badRequest is a client error carrying the message to show.
type badRequest struct{ msg string }

func (e badRequest) Error() string { return e.msg }

func errBadRequest(msg string) error { return badRequest{msg: msg} }

// writeServiceError maps a domain error to a status code and a message.
//
// Every error the API can return passes through here, so the mapping lives in
// one place and no handler has to remember which code goes with which failure.
// Anything unrecognised is a fault on our side: it is logged in full and
// reported to the client as a bare 500, so an internal message never leaks.
func (s *Server) writeServiceError(w http.ResponseWriter, r *http.Request, err error) {
	log := s.logger(r)

	var bad badRequest
	switch {
	case errors.As(err, &bad):
		writeError(w, log, http.StatusBadRequest, bad.msg)

	// --- Rooms ---
	case errors.Is(err, room.ErrNotFound):
		writeError(w, log, http.StatusNotFound, "Room not found")
	case errors.Is(err, room.ErrLocked):
		writeJSON(w, log, http.StatusForbidden, errorBody{
			Error:         "This room is password protected",
			NeedsPassword: true,
		})
	case errors.Is(err, room.ErrWrongPassword):
		writeJSON(w, log, http.StatusForbidden, errorBody{
			Error:         "Incorrect room password",
			NeedsPassword: true,
		})
	case errors.Is(err, room.ErrNotOwner):
		writeError(w, log, http.StatusForbidden, "Only the room owner can do that")
	case errors.Is(err, room.ErrAnonymousAddDenied):
		writeError(w, log, http.StatusForbidden, "Anonymous users cannot add tracks to this room")
	case errors.Is(err, room.ErrLookupFailed):
		writeError(w, log, http.StatusBadRequest, "Could not fetch video info")

	// --- Users ---
	case errors.Is(err, users.ErrEmailTaken):
		writeError(w, log, http.StatusConflict, "Email already registered")
	case errors.Is(err, users.ErrUsernameTaken):
		writeError(w, log, http.StatusConflict, "Username already taken")
	case errors.Is(err, users.ErrInvalidCredentials):
		writeError(w, log, http.StatusUnauthorized, "Invalid email or password")
	case errors.Is(err, users.ErrNotFound):
		writeError(w, log, http.StatusNotFound, "User not found")
	case errors.Is(err, users.ErrInvalidInput):
		// 422 matches what the previous service returned for a body that
		// parsed but did not validate.
		writeError(w, log, http.StatusUnprocessableEntity, validationMessage(err))

	default:
		log.Error("unhandled request failure",
			"method", r.Method, "path", r.URL.Path, "error", err)
		writeError(w, log, http.StatusInternalServerError, "Internal server error")
	}
}

// validationMessage strips the wrapping sentinel from a validation error, so
// the client sees "username must be 2-30 characters" rather than the package
// prefix.
func validationMessage(err error) string {
	msg := err.Error()
	if _, rest, found := strings.Cut(msg, ": "); found && rest != "" {
		return capitalize(rest)
	}
	return capitalize(msg)
}

func capitalize(s string) string {
	if s == "" {
		return s
	}
	return strings.ToUpper(s[:1]) + s[1:]
}
