// Package room is the heart of the service: it owns the live state of every
// listening room, decides who may do what to one, persists the results and
// pushes the new state to everyone connected.
//
// # Concurrency
//
// A room is touched by every listener's WebSocket, by HTTP requests, and by two
// background loops. Each room carries its own mutex, held for the whole of any
// read or mutation of its state; the registry has a separate mutex guarding
// only the room map. Locks are always taken registry-then-room and never the
// other way round.
//
// The one rule that matters more than the rest: no network call -- to
// PostgreSQL, to yt-dlp -- ever happens while a room lock is held. Anything
// slow is done in three steps: read what is needed under the lock, do the slow
// work with the lock released, then re-acquire and apply the result to whatever
// state is there by then. Rooms are read far more often than they are written,
// and a single yt-dlp call can take tens of seconds, so holding a lock across
// one would stall an entire room.
package room

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"sync"
	"time"

	"github.com/leenzstra/bebradio/backend/internal/auth"
	"github.com/leenzstra/bebradio/backend/internal/config"
	"github.com/leenzstra/bebradio/backend/internal/domain"
	"github.com/leenzstra/bebradio/backend/internal/hub"
	"github.com/leenzstra/bebradio/backend/internal/store"
	"github.com/leenzstra/bebradio/backend/internal/youtube"
)

// Errors returned by the service. The API layer maps each to a status code and
// a message; nothing here knows about HTTP.
var (
	// ErrNotFound means no room has that code.
	ErrNotFound = errors.New("room: not found")
	// ErrLocked means the room is password-protected and the caller has not
	// proved they may enter it.
	ErrLocked = errors.New("room: password required")
	// ErrWrongPassword means the supplied room password did not match.
	ErrWrongPassword = errors.New("room: incorrect password")
	// ErrNotOwner means the action is reserved for the room's creator.
	ErrNotOwner = errors.New("room: only the room owner may do that")
	// ErrAnonymousAddDenied means the room does not accept tracks from people
	// who are not signed in.
	ErrAnonymousAddDenied = errors.New("room: anonymous users cannot add tracks to this room")
	// ErrLookupFailed means YouTube could not be asked, or had nothing to give.
	ErrLookupFailed = errors.New("room: could not fetch video info")
)

// Caller is who is making a request, as far as the service is concerned.
type Caller struct {
	// UserID is the authenticated account, or empty for an anonymous caller.
	UserID string
	// Username is the display name to attribute actions to.
	Username string
	// AccessToken is a room-access token, proving the caller supplied the
	// room's password earlier.
	AccessToken string
}

// SettingsUpdate carries a partial change to a room's settings. A nil field
// means "leave unchanged"; for Password, a pointer to the empty string means
// "remove the password".
type SettingsUpdate struct {
	AllowAnonymousAdd *bool
	IsPrivate         *bool
	AutoRadio         *bool
	Password          *string
}

// Lyrics is the caption payload for the karaoke view.
type Lyrics struct {
	Available bool          `json:"available"`
	TrackID   *string       `json:"track_id"`
	Lang      string        `json:"lang,omitempty"`
	Auto      bool          `json:"auto,omitempty"`
	Cues      []youtube.Cue `json:"cues"`
}

// entry is one room's live state plus the lock that guards it.
type entry struct {
	mu    sync.Mutex
	state *domain.Room
	// lastTouched is when the room was last read or written, used to evict
	// rooms nobody is listening to.
	lastTouched time.Time
}

// Service owns the live rooms.
type Service struct {
	store     store.Store
	hub       *hub.Hub
	yt        youtube.Client
	tokens    *auth.Tokens
	passwords *auth.Passwords
	cfg       config.Config
	log       *slog.Logger

	mu    sync.Mutex
	rooms map[string]*entry

	// background is the context for work that outlives the request that
	// triggered it, such as an auto-radio refill. It is cancelled by Shutdown.
	background context.Context
	stop       context.CancelFunc
	wg         sync.WaitGroup
}

// Deps are the collaborators a Service needs.
type Deps struct {
	Store     store.Store
	Hub       *hub.Hub
	YouTube   youtube.Client
	Tokens    *auth.Tokens
	Passwords *auth.Passwords
	Config    config.Config
	Logger    *slog.Logger
}

// New returns a service with no rooms loaded.
func New(deps Deps) *Service {
	log := deps.Logger
	if log == nil {
		log = slog.Default()
	}
	ctx, cancel := context.WithCancel(context.Background())
	return &Service{
		store:      deps.Store,
		hub:        deps.Hub,
		yt:         deps.YouTube,
		tokens:     deps.Tokens,
		passwords:  deps.Passwords,
		cfg:        deps.Config,
		log:        log,
		rooms:      map[string]*entry{},
		background: ctx,
		stop:       cancel,
	}
}

// Shutdown cancels background work and waits for it to finish.
func (s *Service) Shutdown() {
	s.stop()
	s.wg.Wait()
}

// go runs fn in the background, tied to the service's lifetime so Shutdown
// waits for it. Background work gets its own timeout: it must not inherit a
// request's deadline, because the request is usually already answered.
func (s *Service) goBackground(name string, timeout time.Duration, fn func(context.Context)) {
	s.wg.Add(1)
	go func() {
		defer s.wg.Done()
		defer func() {
			if r := recover(); r != nil {
				s.log.Error("background task panicked", "task", name, "panic", r)
			}
		}()
		ctx, cancel := context.WithTimeout(s.background, timeout)
		defer cancel()
		fn(ctx)
	}()
}

// --- Registry ---

// getOrLoad returns the live entry for a room, loading it from the store the
// first time it is asked for.
func (s *Service) getOrLoad(ctx context.Context, roomID string) (*entry, error) {
	roomID = NormalizeID(roomID)
	if roomID == "" {
		return nil, ErrNotFound
	}

	s.mu.Lock()
	if e, ok := s.rooms[roomID]; ok {
		e.lastTouched = time.Now()
		s.mu.Unlock()
		return e, nil
	}
	s.mu.Unlock()

	contents, err := s.store.LoadRoom(ctx, roomID)
	if err != nil {
		if errors.Is(err, store.ErrNotFound) {
			return nil, ErrNotFound
		}
		return nil, fmt.Errorf("room: loading %s: %w", roomID, err)
	}

	loaded := &entry{state: fromContents(contents), lastTouched: time.Now()}

	s.mu.Lock()
	e, raced := s.rooms[roomID]
	if raced {
		// Another goroutine loaded the same room while this one was in the
		// store; theirs wins, so both callers see one shared state.
		e.lastTouched = time.Now()
	} else {
		s.rooms[roomID] = loaded
		e = loaded
	}
	s.mu.Unlock()

	if !raced {
		// A room pulled from the store may have been idle for hours, in which
		// case its current track's stream URL has expired. Refresh it before
		// anybody tries to play it.
		if changed, err := s.refreshTracks(ctx, e, currentOnly); err != nil {
			s.log.Warn("refreshing stream on load", "room_id", roomID, "error", err)
		} else if changed {
			s.persistTracks(ctx, e)
		}
	}
	return e, nil
}

// peek returns the live entry for a room only if it is already loaded.
func (s *Service) peek(roomID string) (*entry, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	e, ok := s.rooms[NormalizeID(roomID)]
	return e, ok
}

// Forget drops a room from the live registry.
//
// Its persisted state is untouched, so the next request for it loads it again
// from the store; what is lost is the transient state -- who is connected, how
// far into the current track playback has got -- exactly as a restart would
// lose it. Nothing needs to call this in normal operation, since idle rooms are
// evicted on their own.
func (s *Service) Forget(roomID string) {
	s.forget(roomID)
}

// forget drops a room from the live registry. Its persisted state is untouched;
// the next request for it loads it again.
func (s *Service) forget(roomID string) {
	s.mu.Lock()
	delete(s.rooms, NormalizeID(roomID))
	s.mu.Unlock()
}

// NormalizeID puts a room code into its canonical form. Room codes are
// uppercase, and people paste them in any case.
func NormalizeID(roomID string) string {
	return strings.ToUpper(strings.TrimSpace(roomID))
}

// fromContents turns stored contents into live room state.
func fromContents(c store.RoomContents) *domain.Room {
	r := domain.NewRoom(c.Room.ID, c.Room.Name, c.Room.OwnerID)
	r.AllowAnonymousAdd = c.Room.AllowAnonymousAdd
	r.IsPrivate = c.Room.IsPrivate
	r.PasswordHash = c.Room.PasswordHash
	r.AutoRadio = c.Room.AutoRadio
	r.CreatedAt = c.Room.CreatedAt
	r.Queue = c.Tracks
	r.Messages = c.Messages
	r.Votes = c.Votes
	return r
}

func toRecord(r *domain.Room) store.RoomRecord {
	return store.RoomRecord{
		ID:                r.ID,
		Name:              r.Name,
		OwnerID:           r.OwnerID,
		AllowAnonymousAdd: r.AllowAnonymousAdd,
		IsPrivate:         r.IsPrivate,
		PasswordHash:      r.PasswordHash,
		AutoRadio:         r.AutoRadio,
		CreatedAt:         r.CreatedAt,
	}
}

// --- Snapshots, persistence and broadcast ---

// snapshot renders a room for the wire, taking the room lock only for as long
// as the copy takes.
func (s *Service) snapshot(e *entry) domain.RoomDTO {
	e.mu.Lock()
	defer e.mu.Unlock()
	return e.state.Snapshot(s.cfg.MaxChatMessages)
}

// broadcast pushes the room's current state to everyone connected to it.
func (s *Service) broadcast(e *entry) {
	snap := s.snapshot(e)
	s.hub.Broadcast(snap.ID, snap)
}

// persistTracks writes the room's queue, logging rather than surfacing a
// failure: the queue in memory is what listeners are hearing, and losing a
// write is not a reason to fail the action that caused it.
func (s *Service) persistTracks(ctx context.Context, e *entry) {
	e.mu.Lock()
	roomID := e.state.ID
	tracks := make([]*domain.Track, len(e.state.Queue))
	for i, t := range e.state.Queue {
		cp := *t
		tracks[i] = &cp
	}
	e.mu.Unlock()

	if err := s.store.ReplaceTracks(ctx, roomID, tracks); err != nil {
		s.log.Error("persisting queue", "room_id", roomID, "error", err)
	}
}

func (s *Service) persistVotes(ctx context.Context, e *entry) {
	e.mu.Lock()
	roomID := e.state.ID
	votes := append([]domain.TrackVote(nil), e.state.Votes...)
	e.mu.Unlock()

	if err := s.store.ReplaceVotes(ctx, roomID, votes); err != nil {
		s.log.Error("persisting votes", "room_id", roomID, "error", err)
	}
}

func (s *Service) persistRoom(ctx context.Context, e *entry) error {
	e.mu.Lock()
	record := toRecord(e.state)
	e.mu.Unlock()

	if err := s.store.SaveRoom(ctx, record); err != nil {
		return fmt.Errorf("room: saving settings: %w", err)
	}
	return nil
}

// --- Access control ---

// hasAccess reports whether a caller may see or change a room's full state.
//
// Rooms without a password are open. Otherwise the owner always has access, and
// everyone else needs a room-access token, which is issued only after the
// password has been checked.
func (s *Service) hasAccess(r *domain.Room, caller Caller) bool {
	if r.PasswordHash == "" {
		return true
	}
	if caller.UserID != "" && caller.UserID == r.OwnerID {
		return true
	}
	return s.tokens.VerifyRoomAccess(caller.AccessToken, r.ID)
}

// requireAccess resolves a room and checks the caller may act on it.
func (s *Service) requireAccess(ctx context.Context, roomID string, caller Caller) (*entry, error) {
	e, err := s.getOrLoad(ctx, roomID)
	if err != nil {
		return nil, err
	}
	e.mu.Lock()
	allowed := s.hasAccess(e.state, caller)
	e.mu.Unlock()
	if !allowed {
		return nil, ErrLocked
	}
	return e, nil
}
