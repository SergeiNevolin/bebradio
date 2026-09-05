package http

import (
	"encoding/json"
	"net/http"

	"github.com/go-chi/chi/v5"
)

func (s *Server) handleSearch(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Query string `json:"query"`
		Limit int    `json:"limit"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		s.writeError(w, 400, "Invalid request body")
		return
	}
	if req.Query == "" {
		s.writeJSON(w, 200, []any{})
		return
	}
	if req.Limit <= 0 {
		req.Limit = 5
	}

	results, err := s.search.SearchYouTube(req.Query, req.Limit)
	if err != nil {
		s.writeError(w, 500, "Search failed")
		return
	}
	if results == nil {
		results = []map[string]any{}
	}
	s.writeJSON(w, 200, results)
}

func (s *Server) handleStream(w http.ResponseWriter, r *http.Request) {
	trackID := chi.URLParam(r, "trackID")
	if trackID == "" {
		s.writeError(w, 400, "Invalid track ID")
		return
	}

	rangeHeader := r.Header.Get("Range")
	statusCode, contentType, body, err := s.media.StreamContent(trackID, rangeHeader)
	if err != nil {
		s.writeError(w, 502, "Media service unavailable")
		return
	}

	w.Header().Set("Content-Type", contentType)
	w.Header().Set("Accept-Ranges", "bytes")
	w.Header().Set("Cache-Control", "public, max-age=3600")
	w.WriteHeader(int(statusCode))
	w.Write(body)
}
