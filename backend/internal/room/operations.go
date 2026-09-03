package room

import (
	"context"
	"fmt"

	"github.com/leenzstra/bebradio/backend/internal/domain"
	"github.com/leenzstra/bebradio/backend/internal/ids"
	"github.com/leenzstra/bebradio/backend/internal/youtube"
)

// Create makes a new room owned by ownerID. A non-empty password locks it: only
// the owner, and anyone who supplies the password, may enter.
//
// The returned snapshot carries an access token, so the creator does not have
// to type the password they just chose.
func (s *Service) Create(ctx context.Context, name, ownerID, password string) (domain.RoomDTO, error) {
	r := domain.NewRoom(ids.Room(), name, ownerID)
	if password != "" {
		hash, err := s.passwords.Hash(password)
		if err != nil {
			return domain.RoomDTO{}, err
		}
		r.PasswordHash = hash
	}

	if err := s.store.SaveRoom(ctx, toRecord(r)); err != nil {
		return domain.RoomDTO{}, fmt.Errorf("room: creating: %w", err)
	}

	e := &entry{state: r}
	s.mu.Lock()
	s.rooms[r.ID] = e
	s.mu.Unlock()

	snap := s.snapshot(e)
	token, err := s.tokens.CreateRoomAccess(r.ID)
	if err != nil {
		return domain.RoomDTO{}, err
	}
	snap.Access = token
	return snap, nil
}

// View returns a room as the caller is allowed to see it.
//
// A caller without access to a password-protected room gets only its name, so
// the join prompt can show what they are being asked to unlock. The owner of a
// locked room is handed an access token along the way, which is what lets them
// open their own room in a fresh browser without retyping the password.
func (s *Service) View(ctx context.Context, roomID string, caller Caller) (domain.RoomDTO, *domain.LockedRoomDTO, error) {
	e, err := s.getOrLoad(ctx, roomID)
	if err != nil {
		return domain.RoomDTO{}, nil, err
	}

	e.mu.Lock()
	allowed := s.hasAccess(e.state, caller)
	isOwner := caller.UserID != "" && caller.UserID == e.state.OwnerID
	locked := e.state.PasswordHash != ""
	e.mu.Unlock()

	if !allowed {
		e.mu.Lock()
		stripped := e.state.LockedSnapshot()
		e.mu.Unlock()
		return domain.RoomDTO{}, &stripped, nil
	}

	snap := s.snapshot(e)
	if locked && isOwner {
		token, err := s.tokens.CreateRoomAccess(snap.ID)
		if err != nil {
			return domain.RoomDTO{}, nil, err
		}
		snap.Access = token
	}
	return snap, nil, nil
}

// Join checks a room's password and, on success, issues an access token
// alongside the room's current state.
func (s *Service) Join(ctx context.Context, roomID, password string) (domain.RoomDTO, string, error) {
	e, err := s.getOrLoad(ctx, roomID)
	if err != nil {
		return domain.RoomDTO{}, "", err
	}

	e.mu.Lock()
	hash := e.state.PasswordHash
	e.mu.Unlock()

	if hash != "" && !s.passwords.Verify(password, hash) {
		return domain.RoomDTO{}, "", ErrWrongPassword
	}

	snap := s.snapshot(e)
	token, err := s.tokens.CreateRoomAccess(snap.ID)
	if err != nil {
		return domain.RoomDTO{}, "", err
	}
	return snap, token, nil
}

// UpdateSettings applies a partial settings change. Only the room's owner may
// call it.
func (s *Service) UpdateSettings(ctx context.Context, roomID string, caller Caller, update SettingsUpdate) (domain.RoomDTO, error) {
	e, err := s.getOrLoad(ctx, roomID)
	if err != nil {
		return domain.RoomDTO{}, err
	}

	var passwordHash string
	if update.Password != nil && *update.Password != "" {
		// Hashing is deliberately slow, so it happens before the lock is taken.
		if passwordHash, err = s.passwords.Hash(*update.Password); err != nil {
			return domain.RoomDTO{}, err
		}
	}

	e.mu.Lock()
	if e.state.OwnerID != caller.UserID || caller.UserID == "" {
		e.mu.Unlock()
		return domain.RoomDTO{}, ErrNotOwner
	}
	if update.AllowAnonymousAdd != nil {
		e.state.AllowAnonymousAdd = *update.AllowAnonymousAdd
	}
	if update.IsPrivate != nil {
		e.state.IsPrivate = *update.IsPrivate
	}
	if update.AutoRadio != nil {
		e.state.AutoRadio = *update.AutoRadio
	}
	if update.Password != nil {
		e.state.PasswordHash = passwordHash
	}
	e.mu.Unlock()

	if err := s.persistRoom(ctx, e); err != nil {
		return domain.RoomDTO{}, err
	}

	s.broadcast(e)
	s.maybeRefill(e)
	return s.snapshot(e), nil
}

// Delete removes a room and everything in it. Only the room's owner may call
// it. Everyone connected is told, so their clients can navigate away.
func (s *Service) Delete(ctx context.Context, roomID string, caller Caller) error {
	e, err := s.getOrLoad(ctx, roomID)
	if err != nil {
		return err
	}

	e.mu.Lock()
	id := e.state.ID
	owner := e.state.OwnerID
	e.mu.Unlock()

	if caller.UserID == "" || owner != caller.UserID {
		return ErrNotOwner
	}

	s.hub.Broadcast(id, map[string]any{"type": "room_deleted", "room_id": id})
	s.forget(id)

	if err := s.store.DeleteRoom(ctx, id); err != nil {
		return fmt.Errorf("room: deleting: %w", err)
	}
	return nil
}

// ListPublic returns every room not marked private.
//
// A room that is loaded in memory reports live figures -- who is listening,
// what is queued, whether it is playing. One that is not has nobody in it, so
// the stored track count is all there is to report.
func (s *Service) ListPublic(ctx context.Context) ([]domain.RoomSummaryDTO, error) {
	stored, err := s.store.ListPublicRooms(ctx)
	if err != nil {
		return nil, fmt.Errorf("room: listing: %w", err)
	}

	out := make([]domain.RoomSummaryDTO, 0, len(stored))
	for _, rec := range stored {
		summary := domain.RoomSummaryDTO{
			ID:          rec.ID,
			Name:        rec.Name,
			TrackCount:  rec.TrackCount,
			HasPassword: rec.HasPassword,
		}
		if e, ok := s.peek(rec.ID); ok {
			e.mu.Lock()
			summary.Name = e.state.Name
			summary.TrackCount = len(e.state.Queue)
			summary.IsPlaying = e.state.IsPlaying
			summary.UserCount = len(e.state.Listeners())
			summary.HasPassword = e.state.PasswordHash != ""
			e.mu.Unlock()
		}
		out = append(out, summary)
	}
	return out, nil
}

// AddTrack looks a YouTube URL up and appends it to a room's queue.
func (s *Service) AddTrack(ctx context.Context, roomID, url string, caller Caller, fallbackName string) (domain.TrackDTO, error) {
	e, err := s.requireAccess(ctx, roomID, caller)
	if err != nil {
		return domain.TrackDTO{}, err
	}

	e.mu.Lock()
	anonymousAllowed := e.state.AllowAnonymousAdd
	e.mu.Unlock()
	if caller.UserID == "" && !anonymousAllowed {
		return domain.TrackDTO{}, ErrAnonymousAddDenied
	}

	// The lookup shells out to yt-dlp and can take tens of seconds, so it
	// happens with no lock held.
	info, err := s.yt.FetchTrack(ctx, url)
	if err != nil {
		s.log.Warn("fetching track", "url", url, "error", err)
		return domain.TrackDTO{}, ErrLookupFailed
	}

	addedBy := caller.Username
	if caller.UserID == "" {
		addedBy = fallbackName
	}
	if addedBy == "" {
		addedBy = "Anonymous"
	}

	track := domain.NewTrackFromInfo(toDomainInfo(info), addedBy)
	if track.SourceURL == "" {
		track.SourceURL = url
	}

	e.mu.Lock()
	e.state.Enqueue(track)
	dto := track.DTO()
	e.mu.Unlock()

	s.persistTracks(ctx, e)
	s.broadcast(e)
	return dto, nil
}

// PlaybackCommand is a transport-neutral playback instruction. Index and
// Position are only read by the actions that need them.
type PlaybackCommand struct {
	Action   string
	Index    *int
	Position *float64
}

// Playback applies a playback command on behalf of an HTTP caller, checking
// room access first.
func (s *Service) Playback(ctx context.Context, roomID string, caller Caller, cmd PlaybackCommand) (domain.RoomDTO, error) {
	e, err := s.requireAccess(ctx, roomID, caller)
	if err != nil {
		return domain.RoomDTO{}, err
	}
	s.applyPlayback(ctx, e, cmd)
	return s.snapshot(e), nil
}

// applyPlayback runs a playback command against an already-authorised room,
// persisting and broadcasting whatever changed.
func (s *Service) applyPlayback(ctx context.Context, e *entry, cmd PlaybackCommand) {
	var queueChanged, trackChanged bool

	e.mu.Lock()
	switch cmd.Action {
	case "next":
		queueChanged = e.state.GoNext(s.cfg.AdvanceDedupWindow.Seconds())
		trackChanged = queueChanged
	case "prev":
		trackChanged = e.state.GoPrev()
	case "jump":
		if cmd.Index != nil {
			trackChanged = e.state.JumpTo(*cmd.Index)
		}
	case "seek", "sync":
		if cmd.Position != nil {
			e.state.SeekTo(*cmd.Position)
		}
	}
	e.mu.Unlock()

	// Moving to a different track is the moment its stream URL has to be live,
	// and the moment to get the one after it ready too.
	if trackChanged {
		if changed, err := s.refreshTracks(ctx, e, currentAndNext); err != nil {
			s.log.Warn("refreshing streams after playback change", "error", err)
		} else if changed {
			queueChanged = true
		}
	}
	if queueChanged {
		s.persistTracks(ctx, e)
	}

	s.broadcast(e)
	s.maybeRefill(e)
}

// LyricsFor returns timed captions for whatever is playing in a room.
func (s *Service) LyricsFor(ctx context.Context, roomID, lang string, caller Caller) (Lyrics, error) {
	e, err := s.requireAccess(ctx, roomID, caller)
	if err != nil {
		return Lyrics{}, err
	}

	e.mu.Lock()
	track := e.state.CurrentTrack()
	var trackID, sourceURL string
	if track != nil {
		trackID, sourceURL = track.ID, track.SourceURL
	}
	e.mu.Unlock()

	if track == nil {
		return Lyrics{Available: false, TrackID: nil, Cues: []youtube.Cue{}}, nil
	}
	if sourceURL == "" {
		id := trackID
		return Lyrics{Available: false, TrackID: &id, Cues: []youtube.Cue{}}, nil
	}

	subs, err := s.yt.FetchSubtitles(ctx, sourceURL, lang)
	if err != nil {
		// No captions is an ordinary outcome, not a failure the caller can act
		// on: the karaoke view simply says there are none.
		s.log.Debug("fetching subtitles", "source_url", sourceURL, "error", err)
		id := trackID
		return Lyrics{Available: false, TrackID: &id, Cues: []youtube.Cue{}}, nil
	}

	cues := subs.Cues
	if cues == nil {
		cues = []youtube.Cue{}
	}
	id := trackID
	return Lyrics{
		Available: len(cues) > 0,
		TrackID:   &id,
		Lang:      subs.Lang,
		Auto:      subs.Auto,
		Cues:      cues,
	}, nil
}

// IsOwner reports whether the caller created the room.
func (s *Service) IsOwner(ctx context.Context, roomID string, userID string) (bool, error) {
	e, err := s.getOrLoad(ctx, roomID)
	if err != nil {
		return false, err
	}
	e.mu.Lock()
	defer e.mu.Unlock()
	return userID != "" && e.state.OwnerID == userID, nil
}

func toDomainInfo(info youtube.TrackInfo) domain.TrackInfo {
	return domain.TrackInfo{
		Title:     info.Title,
		Artist:    info.Artist,
		StreamURL: info.StreamURL,
		Thumbnail: info.Thumbnail,
		Duration:  info.Duration,
		SourceURL: info.SourceURL,
		ExpiresAt: info.ExpiresAt,
	}
}
