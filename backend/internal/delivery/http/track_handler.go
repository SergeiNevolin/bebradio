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

func (s *Server) handleListTracks(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query().Get("q")
	sort := r.URL.Query().Get("sort")
	limit := parseIntDefault(r.URL.Query().Get("limit"), 30)
	offset := parseIntDefault(r.URL.Query().Get("offset"), 0)
	if limit > 100 {
		limit = 100
	}

	items, err := s.tracks.List(q, sort, limit, offset, s.getUserOptional(r))
	if err != nil {
		s.log.Error("list tracks failed", "error", err)
		s.writeError(w, 500, "Failed to list tracks")
		return
	}
	s.writeJSON(w, 200, items)
}

func (s *Server) handleLikedTracks(w http.ResponseWriter, r *http.Request) {
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
	items, err := s.tracks.ListLiked(userID, limit, offset)
	if err != nil {
		s.log.Error("list liked tracks failed", "error", err, "user_id", userID)
		s.writeError(w, 500, "Failed to list tracks")
		return
	}
	s.writeJSON(w, 200, items)
}

func (s *Server) handleLikeTrack(w http.ResponseWriter, r *http.Request)   { s.toggleLike(w, r, true) }
func (s *Server) handleUnlikeTrack(w http.ResponseWriter, r *http.Request) { s.toggleLike(w, r, false) }

func (s *Server) toggleLike(w http.ResponseWriter, r *http.Request, like bool) {
	userID, ok := s.getUserRequired(r)
	if !ok {
		s.writeError(w, 401, "Not authenticated")
		return
	}
	trackID := chi.URLParam(r, "trackID")
	var (
		res map[string]any
		err error
	)
	if like {
		res, err = s.tracks.Like(trackID, userID)
	} else {
		res, err = s.tracks.Unlike(trackID, userID)
	}
	if err != nil {
		if be, ok := err.(*usecase.BusinessError); ok {
			s.writeError(w, be.Code, be.Message)
			return
		}
		s.log.Error("toggle track like failed", "error", err, "track_id", trackID, "like", like)
		s.writeError(w, 500, "Failed to update like")
		return
	}
	s.writeJSON(w, 200, res)
}

func (s *Server) handleMyTracks(w http.ResponseWriter, r *http.Request) {
	userID, ok := s.getUserRequired(r)
	if !ok {
		s.writeError(w, 401, "Not authenticated")
		return
	}
	items, err := s.tracks.ListMine(userID)
	if err != nil {
		s.log.Error("list my tracks failed", "error", err, "user_id", userID)
		s.writeError(w, 500, "Failed to list tracks")
		return
	}
	s.writeJSON(w, 200, items)
}

func (s *Server) handleGetTrack(w http.ResponseWriter, r *http.Request) {
	t, err := s.tracks.Get(chi.URLParam(r, "trackID"), s.getUserOptional(r))
	if err != nil {
		s.writeError(w, 404, "Track not found")
		return
	}
	s.writeJSON(w, 200, t.ToDict())
}

// handleUploadTrackCover streams a cover image straight through to music-service.
// Owner-only; the server-wide ReadTimeout is lifted and the body is capped.
func (s *Server) handleUploadTrackCover(w http.ResponseWriter, r *http.Request) {
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

	trackID := chi.URLParam(r, "trackID")
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
			if err := s.tracks.SetCover(trackID, userID, part.FileName(), part); err != nil {
				_ = part.Close()
				if be, ok := err.(*usecase.BusinessError); ok {
					s.writeError(w, be.Code, be.Message)
					return
				}
				s.log.Error("set track cover failed", "error", err, "track_id", trackID)
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

func (s *Server) handleDeleteTrack(w http.ResponseWriter, r *http.Request) {
	userID, ok := s.getUserRequired(r)
	if !ok {
		s.writeError(w, 401, "Not authenticated")
		return
	}
	if err := s.tracks.Delete(chi.URLParam(r, "trackID"), userID); err != nil {
		if be, ok := err.(*usecase.BusinessError); ok {
			s.writeError(w, be.Code, be.Message)
			return
		}
		s.log.Error("delete track failed", "error", err)
		s.writeError(w, 500, "Failed to delete track")
		return
	}
	s.writeJSON(w, 200, map[string]any{"ok": true})
}

// handleUploadTrack streams a multipart upload straight through to music-service.
// The server-wide ReadTimeout (15s) is lifted for this one request, the body is
// capped with MaxBytesReader, and only authorized users may reach it.
func (s *Server) handleUploadTrack(w http.ResponseWriter, r *http.Request) {
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
	var created *entity.Track

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
			created, err = s.tracks.Create(userID, title, artist, part.FileName(), part)
			if err != nil {
				if be, ok := err.(*usecase.BusinessError); ok {
					s.writeError(w, be.Code, be.Message)
					return
				}
				s.log.Error("create track failed", "error", err, "user_id", userID)
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
		if err := s.tracks.SetCover(created.ID, userID, coverName, bytes.NewReader(coverBytes)); err != nil {
			// Cover is optional: log and still return the created track.
			s.log.Warn("track cover on upload failed", "error", err, "track_id", created.ID)
		} else if fresh, err := s.tracks.Get(created.ID, userID); err == nil {
			created = fresh
		}
	}
	s.writeJSON(w, 202, created.ToDict())
}

// handleStreamTrack serves the audio of any track row (library or queue copy)
// by id, streaming music-service through with io.Copy rather than buffering.
// Uploads in "processing"/"failed" state have no file yet.
func (s *Server) handleStreamTrack(w http.ResponseWriter, r *http.Request) {
	t, err := s.tracks.FindByID(chi.URLParam(r, "trackID"))
	if err != nil {
		s.writeError(w, 404, "Track not found")
		return
	}
	if t.Source == entity.TrackSourceUpload && t.Status != entity.TrackStatusReady {
		s.writeError(w, 404, "Track not ready")
		return
	}
	if t.MediaID == "" {
		s.writeError(w, 404, "Track not found")
		return
	}
	prefix := "/v1/music/"
	if t.Source == entity.TrackSourceUpload {
		prefix = "/v1/uploads/"
	}
	s.proxyTrackMedia(w, r, prefix+t.MediaID)
}

// handleStreamTrackCover serves the cover image of any track row by id.
func (s *Server) handleStreamTrackCover(w http.ResponseWriter, r *http.Request) {
	t, err := s.tracks.FindByID(chi.URLParam(r, "trackID"))
	if err != nil {
		s.writeError(w, 404, "Track not found")
		return
	}
	if !t.HasCover || t.MediaID == "" {
		s.writeError(w, 404, "Cover not found")
		return
	}
	s.proxyTrackMedia(w, r, "/v1/uploads/"+t.MediaID+"/cover")
}

func (s *Server) proxyTrackMedia(w http.ResponseWriter, r *http.Request, upstreamPath string) {
	upstream := strings.TrimRight(s.config.MusicServiceURL, "/") + upstreamPath

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
		s.log.Error("stream track failed", "error", err, "upstream", upstreamPath)
		s.writeError(w, 502, "Music service unavailable")
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
