package http

import (
	"context"
	"encoding/json"
	"net/http"
	"strings"
	"time"

	"github.com/bebradio/backend-go/internal/domain/entity"
	"github.com/bebradio/backend-go/internal/domain/repository"
	"github.com/bebradio/backend-go/internal/infrastructure/redisc"
	"github.com/bebradio/backend-go/internal/usecase"
	"github.com/go-chi/chi/v5"
)

func (s *Server) handleCreateStream(w http.ResponseWriter, r *http.Request) {
	userID, ok := s.getUserRequired(r)
	if !ok {
		s.writeError(w, 401, "Not authenticated")
		return
	}
	user, err := s.user.GetUser(userID)
	if err != nil {
		s.writeError(w, 401, "Not authenticated")
		return
	}
	var req struct {
		Name string `json:"name"`
	}
	_ = json.NewDecoder(r.Body).Decode(&req)
	if strings.TrimSpace(req.Name) == "" {
		req.Name = "Stream"
	}
	st, err := s.stream.CreateStream(r.Context(), req.Name, user)
	if err != nil {
		if be, ok := err.(*usecase.BusinessError); ok {
			s.writeError(w, be.Code, be.Message)
			return
		}
		s.log.Error("create stream failed", "error", err, "user_id", userID)
		s.writeError(w, 500, "Failed to create stream")
		return
	}
	s.writeJSON(w, 200, s.stream.BuildToDict(r.Context(), st))
}

func (s *Server) handleListStreams(w http.ResponseWriter, r *http.Request) {
	limit := parseIntDefault(r.URL.Query().Get("limit"), 50)
	offset := parseIntDefault(r.URL.Query().Get("offset"), 0)
	if limit < 0 {
		limit = 0
	}
	if limit > 100 {
		limit = 100
	}
	if offset < 0 {
		offset = 0
	}
	streams, err := s.stream.ListPublicStreams(r.Context(), limit, offset)
	if err != nil {
		s.log.Error("list streams failed", "error", err)
		s.writeError(w, 500, "Failed to list streams")
		return
	}
	if streams == nil {
		streams = []map[string]any{}
	}
	s.writeJSON(w, 200, streams)
}

func (s *Server) handleGetStream(w http.ResponseWriter, r *http.Request) {
	streamID := chi.URLParam(r, "streamID")
	st, err := s.stream.GetOrLoadStream(r.Context(), streamID)
	if err != nil {
		s.writeError(w, 404, "Stream not found")
		return
	}
	s.writeJSON(w, 200, s.stream.BuildToDict(r.Context(), st))
}

func (s *Server) handleDeleteStream(w http.ResponseWriter, r *http.Request) {
	streamID := chi.URLParam(r, "streamID")
	userID, ok := s.getUserRequired(r)
	if !ok {
		s.writeError(w, 401, "Not authenticated")
		return
	}
	st, err := s.stream.GetOrLoadStream(r.Context(), streamID)
	if err != nil {
		s.writeError(w, 404, "Stream not found")
		return
	}
	if !s.stream.IsController(st, userID) {
		s.writeError(w, 403, "Only the stream owner can delete it")
		return
	}
	if err := s.stream.DeleteStream(r.Context(), st); err != nil {
		s.log.Error("delete stream failed", "error", err, "stream_id", streamID)
		s.writeError(w, 500, "Failed to delete stream")
		return
	}
	s.writeJSON(w, 200, map[string]any{"ok": true})
}

func (s *Server) handleAddToStreamQueue(w http.ResponseWriter, r *http.Request) {
	streamID := chi.URLParam(r, "streamID")
	userID, ok := s.getUserRequired(r)
	if !ok {
		s.writeError(w, 401, "Not authenticated")
		return
	}
	ctx := r.Context()
	st, err := s.stream.GetOrLoadStream(ctx, streamID)
	if err != nil {
		s.writeError(w, 404, "Stream not found")
		return
	}
	if !s.stream.IsController(st, userID) {
		s.writeError(w, 403, "Only admins can add tracks to streams")
		return
	}

	var req struct {
		Source  string `json:"source"`
		URL     string `json:"url"`
		TrackID string `json:"track_id"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		s.writeError(w, 400, "Invalid request body")
		return
	}

	addedBy := "Anonymous"
	if user, err := s.auth.GetUserByID(userID); err == nil {
		addedBy = user.Username
	}

	var track *entity.Track
	switch {
	case req.TrackID != "":
		track, err = s.resolveLibraryTrack(req.TrackID, userID, addedBy)
		if err != nil {
			s.writeError(w, 400, err.Error())
			return
		}
	case req.URL != "":
		qtrack, qerr := s.resolveURLTrack(req.Source, req.URL, addedBy)
		if qerr != nil {
			s.writeError(w, qerr.code, qerr.msg)
			return
		}
		track = qtrack
	default:
		s.writeError(w, 400, "Provide url or track_id")
		return
	}

	s.addMu.Lock()
	queue, _ := redisc.GetQueue(ctx, s.rdb, st.Ref())
	for _, t := range queue {
		if isDuplicateQueueTrack(t, track) {
			s.addMu.Unlock()
			s.writeJSON(w, 200, t.ToDict())
			return
		}
	}
	redisc.AppendTrack(ctx, s.rdb, st.Ref(), track)
	queueLen, _ := redisc.GetQueueLen(ctx, s.rdb, st.Ref())
	if queueLen == 1 {
		ps, _ := redisc.GetPlayback(ctx, s.rdb, st.Ref())
		if ps == nil {
			ps = &redisc.PlaybackState{}
		}
		ps.IsPlaying = true
		ps.Position = 0
		ps.LastSyncAt = time.Now()
		ps.CurrentStartedAt = time.Now()
		redisc.SetPlayback(ctx, s.rdb, st.Ref(), ps)
	}
	s.addMu.Unlock()

	go func() {
		if err := s.stream.SaveTracks(context.Background(), st); err != nil {
			s.log.Error("save stream tracks failed", "error", err, "stream_id", streamID)
		}
	}()

	s.manager.Broadcast(st.Ref(), s.stream.BuildToDict(ctx, st))
	s.writeJSON(w, 200, track.ToDict())
}

func (s *Server) handleSetMashupRadio(w http.ResponseWriter, r *http.Request) {
	streamID := chi.URLParam(r, "streamID")
	userID, ok := s.getUserRequired(r)
	if !ok {
		s.writeError(w, 401, "Not authenticated")
		return
	}
	ctx := r.Context()
	st, err := s.stream.GetOrLoadStream(ctx, streamID)
	if err != nil {
		s.writeError(w, 404, "Stream not found")
		return
	}
	if !s.stream.IsController(st, userID) {
		s.writeError(w, 403, "Only the stream owner can change features")
		return
	}
	var req struct {
		Enabled *bool `json:"enabled"`
		Filter  *struct {
			Q      *string `json:"q"`
			Title  *string `json:"title"`
			Artist *string `json:"artist"`
			Owner  *string `json:"owner"`
		} `json:"filter"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		s.writeError(w, 400, "Invalid request body")
		return
	}
	filter := st.Features.MashupRadio.Filter
	if req.Filter != nil {
		if req.Filter.Q != nil {
			filter.Query = strings.TrimSpace(*req.Filter.Q)
		}
		if req.Filter.Title != nil {
			filter.Title = strings.TrimSpace(*req.Filter.Title)
		}
		if req.Filter.Artist != nil {
			filter.Artist = strings.TrimSpace(*req.Filter.Artist)
		}
		if req.Filter.Owner != nil {
			filter.Owner = strings.TrimSpace(*req.Filter.Owner)
		}
	}
	if err := s.stream.UpdateMashupRadio(ctx, st, req.Enabled, &filter); err != nil {
		s.log.Error("update mashup radio failed", "error", err, "stream_id", streamID)
		s.writeError(w, 500, "Failed to update stream feature")
		return
	}
	result := s.stream.BuildToDict(ctx, st)
	s.manager.Broadcast(st.Ref(), result)
	s.writeJSON(w, 200, result)
}

func (s *Server) handleStreamRefill(w http.ResponseWriter, r *http.Request) {
	streamID := chi.URLParam(r, "streamID")
	userID, ok := s.getUserRequired(r)
	if !ok {
		s.writeError(w, 401, "Not authenticated")
		return
	}
	ctx := r.Context()
	st, err := s.stream.GetOrLoadStream(ctx, streamID)
	if err != nil {
		s.writeError(w, 404, "Stream not found")
		return
	}
	if !s.stream.IsController(st, userID) {
		s.writeError(w, 403, "Only the stream owner can refill it")
		return
	}

	var req struct {
		Count int `json:"count"`
	}
	_ = json.NewDecoder(r.Body).Decode(&req)
	count := req.Count
	if count <= 0 {
		count = s.config.RadioBatch
	}
	if count <= 0 {
		count = 3
	}
	if count > 10 {
		count = 10
	}

	sf := st.Features.MashupRadio.Filter
	filter := repository.TrackFilter{Query: sf.Query, Title: sf.Title, Artist: sf.Artist, Owner: sf.Owner}
	queue, _ := redisc.GetQueue(ctx, s.rdb, st.Ref())
	exclude := make([]string, 0, len(queue))
	queued := make(map[string]bool, len(queue))
	for _, t := range queue {
		exclude = append(exclude, t.ID)
		queued[t.ID] = true
	}

	addedBy := "Radio"
	if u, err := s.auth.GetUserByID(userID); err == nil {
		addedBy = u.Username
	}

	libs, err := s.tracks.RandomForStream(filter, count, exclude)
	if err != nil {
		s.log.Error("stream refill failed", "error", err, "stream_id", streamID)
		s.writeError(w, 500, "Refill failed")
		return
	}
	if len(libs) == 0 {
		libs, err = s.tracks.RandomForStream(filter, count, nil)
		if err != nil {
			s.log.Error("stream refill retry failed", "error", err, "stream_id", streamID)
			s.writeError(w, 500, "Refill failed")
			return
		}
		kept := libs[:0]
		for _, l := range libs {
			if !queued[l.ID] {
				kept = append(kept, l)
			}
		}
		libs = kept
	}
	if len(libs) == 0 {
		s.writeError(w, 404, "No tracks match the stream filter")
		return
	}

	s.addMu.Lock()
	added := make([]map[string]any, 0, len(libs))
	for _, lib := range libs {
		if !entity.IsQueueableLibraryTrack(lib) {
			continue
		}
		track := entity.QueueCopyFromLibrary(lib, addedBy)
		dup := false
		if fresh, _ := redisc.GetQueue(ctx, s.rdb, st.Ref()); fresh != nil {
			for _, q := range fresh {
				if isDuplicateQueueTrack(q, track) {
					dup = true
					break
				}
			}
		}
		if dup {
			continue
		}
		redisc.AppendTrack(ctx, s.rdb, st.Ref(), track)
		redisc.AddRadioSeen(ctx, s.rdb, st.Ref(), track.ID)
		added = append(added, track.ToDict())
	}
	if len(added) > 0 {
		ps, _ := redisc.GetPlayback(ctx, s.rdb, st.Ref())
		if ps == nil {
			ps = &redisc.PlaybackState{}
		}
		if !ps.IsPlaying {
			ps.IsPlaying = true
			ps.Position = 0
			ps.LastSyncAt = time.Now()
			ps.CurrentStartedAt = time.Now()
			redisc.SetPlayback(ctx, s.rdb, st.Ref(), ps)
		}
	}
	s.addMu.Unlock()

	if len(added) == 0 {
		s.writeError(w, 404, "No tracks match the stream filter")
		return
	}
	go func() {
		if err := s.stream.SaveTracks(context.Background(), st); err != nil {
			s.log.Error("save stream tracks after refill failed", "error", err, "stream_id", streamID)
		}
	}()
	s.manager.Broadcast(st.Ref(), s.stream.BuildToDict(ctx, st))
	s.writeJSON(w, 200, map[string]any{"added": added})
}

func (s *Server) handleGetStreamLyrics(w http.ResponseWriter, r *http.Request) {
	streamID := chi.URLParam(r, "streamID")
	st, err := s.stream.GetOrLoadStream(r.Context(), streamID)
	if err != nil {
		s.writeError(w, 404, "Stream not found")
		return
	}
	track := redisc.CurrentTrack(r.Context(), s.rdb, st.Ref())
	if track == nil || track.MediaID == "" {
		s.writeJSON(w, 200, map[string]any{"available": false, "track_id": nil, "cues": []any{}})
		return
	}
	subs, _ := s.media.FetchSubtitles(track.MediaID, r.URL.Query().Get("lang"))
	cues, _ := subs["cues"].([]any)
	s.writeJSON(w, 200, map[string]any{
		"available": len(cues) > 0,
		"track_id":  track.ID,
		"lang":      subs["lang"],
		"auto":      subs["auto"],
		"cues":      cues,
	})
}
