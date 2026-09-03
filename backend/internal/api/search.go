package api

import (
	"net/http"
	"strings"

	"github.com/leenzstra/bebradio/backend/internal/youtube"
)

const (
	defaultSearchLimit = 5
	maxSearchLimit     = 25
	maxSearchQueryLen  = 200
)

type searchRequest struct {
	Query string `json:"query"`
	Limit int    `json:"limit"`
}

// handleSearch looks a query up on YouTube.
//
// A failed lookup answers with an empty list rather than an error status: the
// search box is a live-as-you-type affordance, and YouTube being briefly
// unreachable should show "nothing found", not an error banner. The failure is
// logged, because a persistent one means yt-dlp needs attention.
func (s *Server) handleSearch(w http.ResponseWriter, r *http.Request) {
	var req searchRequest
	if err := decodeJSON(r, &req); err != nil {
		s.writeServiceError(w, r, err)
		return
	}

	query := strings.TrimSpace(req.Query)
	if query == "" {
		writeJSON(w, s.logger(r), http.StatusOK, []youtube.SearchResult{})
		return
	}
	if len(query) > maxSearchQueryLen {
		query = query[:maxSearchQueryLen]
	}

	limit := req.Limit
	if limit <= 0 {
		limit = defaultSearchLimit
	}
	if limit > maxSearchLimit {
		limit = maxSearchLimit
	}

	results, err := s.yt.Search(r.Context(), query, limit)
	if err != nil {
		s.logger(r).Warn("youtube search failed", "query", query, "error", err)
		results = nil
	}
	if results == nil {
		results = []youtube.SearchResult{}
	}
	writeJSON(w, s.logger(r), http.StatusOK, results)
}
