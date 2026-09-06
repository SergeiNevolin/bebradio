package http

import (
	"bytes"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/bebradio/backend-go/internal/domain/entity"
	"github.com/bebradio/backend-go/internal/usecase"
	"github.com/go-chi/chi/v5"
)

func (s *Server) handleListMashups(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query().Get("q")
	sort := r.URL.Query().Get("sort")
	limit := parseIntDefault(r.URL.Query().Get("limit"), 30)
	offset := parseIntDefault(r.URL.Query().Get("offset"), 0)
	if limit > 100 {
		limit = 100
	}

	items, err := s.mashup.List(q, sort, limit, offset, s.getUserOptional(r))
	if err != nil {
		s.log.Error("list mashups failed", "error", err)
		s.writeError(w, 500, "Failed to list mashups")
		return
	}
	s.writeJSON(w, 200, items)
}

func (s *Server) handleLikedMashups(w http.ResponseWriter, r *http.Request) {
	userID, ok := s.getUserRequired(r)
	if !ok {
		s.writeError(w, 401, "Not authenticated")
		return
	}
	limit := parseIntDefault(r.URL.Query().Get("limit"), 30)
	offset := parseIntDefault(r.URL.Query().Get("offset"), 0)
	if limit > 100 {
		limit = 100
	}
	items, err := s.mashup.ListLiked(userID, limit, offset)
	if err != nil {
		s.log.Error("list liked mashups failed", "error", err, "user_id", userID)
		s.writeError(w, 500, "Failed to list mashups")
		return
	}
	s.writeJSON(w, 200, items)
}

func (s *Server) handleLikeMashup(w http.ResponseWriter, r *http.Request)   { s.toggleLike(w, r, true) }
func (s *Server) handleUnlikeMashup(w http.ResponseWriter, r *http.Request) { s.toggleLike(w, r, false) }

func (s *Server) toggleLike(w http.ResponseWriter, r *http.Request, like bool) {
	userID, ok := s.getUserRequired(r)
	if !ok {
		s.writeError(w, 401, "Not authenticated")
		return
	}
	mashupID := chi.URLParam(r, "mashupID")
	var (
		res map[string]any
		err error
	)
	if like {
		res, err = s.mashup.Like(mashupID, userID)
	} else {
		res, err = s.mashup.Unlike(mashupID, userID)
	}
	if err != nil {
		if be, ok := err.(*usecase.BusinessError); ok {
			s.writeError(w, be.Code, be.Message)
			return
		}
		s.log.Error("toggle mashup like failed", "error", err, "mashup_id", mashupID, "like", like)
		s.writeError(w, 500, "Failed to update like")
		return
	}
	s.writeJSON(w, 200, res)
}

func (s *Server) handleMyMashups(w http.ResponseWriter, r *http.Request) {
	userID, ok := s.getUserRequired(r)
	if !ok {
		s.writeError(w, 401, "Not authenticated")
		return
	}
	items, err := s.mashup.ListMine(userID)
	if err != nil {
		s.log.Error("list my mashups failed", "error", err, "user_id", userID)
		s.writeError(w, 500, "Failed to list mashups")
		return
	}
	s.writeJSON(w, 200, items)
}

func (s *Server) handleGetMashup(w http.ResponseWriter, r *http.Request) {
	m, err := s.mashup.Get(chi.URLParam(r, "mashupID"), s.getUserOptional(r))
	if err != nil {
		s.writeError(w, 404, "Mashup not found")
		return
	}
	s.writeJSON(w, 200, m.ToDict())
}

// handleUploadMashupCover streams a cover image straight through to media-service.
// Owner-only; the server-wide ReadTimeout is lifted and the body is capped.
func (s *Server) handleUploadMashupCover(w http.ResponseWriter, r *http.Request) {
	userID, ok := s.getUserRequired(r)
	if !ok {
		s.writeError(w, 401, "Not authenticated")
		return
	}
	if !s.uploadLimiter.Allow(userID) {
		w.Header().Set("Retry-After", "3600")
		s.writeError(w, 429, "Upload rate limit reached, try again later")
		return
	}

	_ = http.NewResponseController(w).SetReadDeadline(time.Now().Add(2 * time.Minute))
	r.Body = http.MaxBytesReader(w, r.Body, s.config.MashupCoverMaxSize+(4<<10))
	reader, err := r.MultipartReader()
	if err != nil {
		s.writeError(w, 400, "Expected multipart/form-data")
		return
	}

	mashupID := chi.URLParam(r, "mashupID")
	for {
		part, err := reader.NextPart()
		if err == io.EOF {
			break
		}
		if err != nil {
			s.writeError(w, 400, "Malformed upload")
			return
		}
		if part.FormName() == "file" {
			if err := s.mashup.SetCover(mashupID, userID, part.FileName(), part); err != nil {
				_ = part.Close()
				if be, ok := err.(*usecase.BusinessError); ok {
					s.writeError(w, be.Code, be.Message)
					return
				}
				s.log.Error("set mashup cover failed", "error", err, "mashup_id", mashupID)
				s.writeError(w, 500, "Cover upload failed")
				return
			}
			_ = part.Close()
			s.writeJSON(w, 200, map[string]any{"ok": true})
			return
		}
		_ = part.Close()
	}
	s.writeError(w, 400, "No file provided")
}

func (s *Server) handleDeleteMashup(w http.ResponseWriter, r *http.Request) {
	userID, ok := s.getUserRequired(r)
	if !ok {
		s.writeError(w, 401, "Not authenticated")
		return
	}
	if err := s.mashup.Delete(chi.URLParam(r, "mashupID"), userID); err != nil {
		if be, ok := err.(*usecase.BusinessError); ok {
			s.writeError(w, be.Code, be.Message)
			return
		}
		s.log.Error("delete mashup failed", "error", err)
		s.writeError(w, 500, "Failed to delete mashup")
		return
	}
	s.writeJSON(w, 200, map[string]any{"ok": true})
}

// handleUploadMashup streams a multipart upload straight through to media-service.
// The server-wide ReadTimeout (15s) is lifted for this one request, the body is
// capped with MaxBytesReader, and only authorized users may reach it.
func (s *Server) handleUploadMashup(w http.ResponseWriter, r *http.Request) {
	userID, ok := s.getUserRequired(r)
	if !ok {
		s.writeError(w, 401, "Not authenticated")
		return
	}
	if !s.uploadLimiter.Allow(userID) {
		w.Header().Set("Retry-After", "3600")
		s.writeError(w, 429, "Upload rate limit reached, try again later")
		return
	}

	// Point fix for cmd/server/main.go's ReadTimeout: 15s, which would otherwise
	// abort any upload slower than 15 seconds.
	_ = http.NewResponseController(w).SetReadDeadline(time.Now().Add(10 * time.Minute))

	r.Body = http.MaxBytesReader(w, r.Body, s.config.MashupMaxSize+(1<<20))
	reader, err := r.MultipartReader()
	if err != nil {
		s.writeError(w, 400, "Expected multipart/form-data")
		return
	}

	var title, artist, coverName string
	var coverBytes []byte
	var created *entity.Mashup

	for {
		part, err := reader.NextPart()
		if err == io.EOF {
			break
		}
		if err != nil {
			s.writeError(w, 400, "Malformed upload")
			return
		}

		switch part.FormName() {
		case "title":
			b, _ := io.ReadAll(io.LimitReader(part, 4096))
			title = strings.TrimSpace(string(b))
		case "artist":
			b, _ := io.ReadAll(io.LimitReader(part, 4096))
			artist = strings.TrimSpace(string(b))
		case "cover":
			// Buffered (small cap) so it can be forwarded after the row exists;
			// the frontend sends this part before "file".
			coverBytes, _ = io.ReadAll(io.LimitReader(part, s.config.MashupCoverMaxSize))
			coverName = part.FileName()
		case "file":
			created, err = s.mashup.Create(userID, title, artist, part.FileName(), part)
			if err != nil {
				if be, ok := err.(*usecase.BusinessError); ok {
					s.writeError(w, be.Code, be.Message)
					return
				}
				s.log.Error("create mashup failed", "error", err, "user_id", userID)
				s.writeError(w, 500, "Upload failed")
				return
			}
		}
		_ = part.Close()
	}

	if created == nil {
		s.writeError(w, 400, "No file provided")
		return
	}

	if len(coverBytes) > 0 {
		if err := s.mashup.SetCover(created.ID, userID, coverName, bytes.NewReader(coverBytes)); err != nil {
			// Cover is optional: log and still return the created mashup.
			s.log.Warn("mashup cover on upload failed", "error", err, "mashup_id", created.ID)
		} else if fresh, err := s.mashup.Get(created.ID, userID); err == nil {
			created = fresh
		}
	}
	s.writeJSON(w, 202, created.ToDict())
}

// handleStreamMashup is a dev-only fallback (prod serves /api/mashups/media/ via
// nginx). It streams the media-service response through with io.Copy rather than
// buffering it, unlike handleStream for room tracks.
func (s *Server) handleStreamMashup(w http.ResponseWriter, r *http.Request) {
	s.proxyMashupMedia(w, r, "")
}

// handleStreamMashupCover is the dev-only fallback for cover images (prod serves
// /api/mashups/media/ via nginx).
func (s *Server) handleStreamMashupCover(w http.ResponseWriter, r *http.Request) {
	s.proxyMashupMedia(w, r, "/cover")
}

func (s *Server) proxyMashupMedia(w http.ResponseWriter, r *http.Request, suffix string) {
	mediaID := chi.URLParam(r, "mediaID")
	upstream := strings.TrimRight(s.config.MediaServiceURL, "/") + "/v1/mashups/" + mediaID + suffix

	req, err := http.NewRequestWithContext(r.Context(), "GET", upstream, nil)
	if err != nil {
		s.writeError(w, 500, "Bad upstream request")
		return
	}
	if rng := r.Header.Get("Range"); rng != "" {
		req.Header.Set("Range", rng)
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		s.log.Error("stream mashup failed", "error", err, "media_id", mediaID)
		s.writeError(w, 502, "Media service unavailable")
		return
	}
	defer resp.Body.Close()

	for _, h := range []string{"Content-Type", "Content-Length", "Content-Range", "Accept-Ranges", "Cache-Control"} {
		if v := resp.Header.Get(h); v != "" {
			w.Header().Set(h, v)
		}
	}
	w.WriteHeader(resp.StatusCode)
	io.Copy(w, resp.Body)
}

func parseIntDefault(s string, def int) int {
	if s == "" {
		return def
	}
	if n, err := strconv.Atoi(s); err == nil {
		return n
	}
	return def
}
