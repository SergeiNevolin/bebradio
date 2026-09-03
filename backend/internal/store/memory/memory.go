// Package memory implements store.Store in process.
//
// It exists so the HTTP, WebSocket and room logic can be exercised without a
// database. It is not a production store: nothing survives a restart.
package memory

import (
	"context"
	"sort"
	"strings"
	"sync"

	"github.com/leenzstra/bebradio/backend/internal/domain"
	"github.com/leenzstra/bebradio/backend/internal/store"
)

// Store is an in-process store.Store.
type Store struct {
	mu       sync.RWMutex
	users    map[string]domain.User
	rooms    map[string]store.RoomRecord
	tracks   map[string][]*domain.Track
	messages map[string][]domain.ChatMessage
	votes    map[string][]domain.TrackVote
	// seq orders rooms in the public listing the way created_at does in
	// PostgreSQL, without depending on two rooms created in the same
	// microsecond having distinct timestamps.
	seq     map[string]int
	nextSeq int
}

// New returns an empty in-memory store.
func New() *Store {
	return &Store{
		users:    map[string]domain.User{},
		rooms:    map[string]store.RoomRecord{},
		tracks:   map[string][]*domain.Track{},
		messages: map[string][]domain.ChatMessage{},
		votes:    map[string][]domain.TrackVote{},
		seq:      map[string]int{},
	}
}

func (s *Store) Ping(context.Context) error { return nil }
func (s *Store) Close()                     {}

// --- Users ---

func (s *Store) CreateUser(_ context.Context, user domain.User) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	for _, existing := range s.users {
		if strings.EqualFold(existing.Email, user.Email) || existing.Username == user.Username {
			return store.ErrConflict
		}
	}
	s.users[user.ID] = user
	return nil
}

func (s *Store) UserByID(_ context.Context, id string) (domain.User, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	u, ok := s.users[id]
	if !ok {
		return domain.User{}, store.ErrNotFound
	}
	return u, nil
}

func (s *Store) UserByEmail(_ context.Context, email string) (domain.User, error) {
	return s.findUser(func(u domain.User) bool { return strings.EqualFold(u.Email, email) })
}

func (s *Store) UserByUsername(_ context.Context, username string) (domain.User, error) {
	return s.findUser(func(u domain.User) bool { return u.Username == username })
}

func (s *Store) findUser(match func(domain.User) bool) (domain.User, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	for _, u := range s.users {
		if match(u) {
			return u, nil
		}
	}
	return domain.User{}, store.ErrNotFound
}

func (s *Store) UpdateProfile(_ context.Context, id string, bio, avatarURL *string) (domain.User, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	u, ok := s.users[id]
	if !ok {
		return domain.User{}, store.ErrNotFound
	}
	if bio != nil {
		u.Bio = *bio
	}
	if avatarURL != nil {
		u.AvatarURL = *avatarURL
	}
	s.users[id] = u
	return u, nil
}

// --- Rooms ---

func (s *Store) SaveRoom(_ context.Context, room store.RoomRecord) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if existing, ok := s.rooms[room.ID]; ok {
		// Ownership and creation time are fixed once a room exists.
		room.OwnerID = existing.OwnerID
		room.CreatedAt = existing.CreatedAt
	} else {
		s.seq[room.ID] = s.nextSeq
		s.nextSeq++
	}
	s.rooms[room.ID] = room
	return nil
}

func (s *Store) LoadRoom(_ context.Context, id string) (store.RoomContents, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	room, ok := s.rooms[id]
	if !ok {
		return store.RoomContents{}, store.ErrNotFound
	}
	return store.RoomContents{
		Room:     room,
		Tracks:   cloneTracks(s.tracks[id]),
		Messages: append([]domain.ChatMessage(nil), s.messages[id]...),
		Votes:    append([]domain.TrackVote(nil), s.votes[id]...),
	}, nil
}

func (s *Store) DeleteRoom(_ context.Context, id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.rooms, id)
	delete(s.tracks, id)
	delete(s.messages, id)
	delete(s.votes, id)
	delete(s.seq, id)
	return nil
}

func (s *Store) ListPublicRooms(_ context.Context) ([]store.PublicRoom, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := []store.PublicRoom{}
	for id, room := range s.rooms {
		if room.IsPrivate {
			continue
		}
		out = append(out, store.PublicRoom{
			ID:          id,
			Name:        room.Name,
			TrackCount:  len(s.tracks[id]),
			HasPassword: room.PasswordHash != "",
		})
	}
	sort.Slice(out, func(i, j int) bool { return s.seq[out[i].ID] < s.seq[out[j].ID] })
	return out, nil
}

// --- Room contents ---

func (s *Store) ReplaceTracks(_ context.Context, roomID string, tracks []*domain.Track) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.tracks[roomID] = cloneTracks(tracks)
	return nil
}

func (s *Store) AppendMessage(_ context.Context, roomID string, msg domain.ChatMessage) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.messages[roomID] = append(s.messages[roomID], msg)
	return nil
}

func (s *Store) ReplaceVotes(_ context.Context, roomID string, votes []domain.TrackVote) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.votes[roomID] = append([]domain.TrackVote(nil), votes...)
	return nil
}

// cloneTracks deep-copies a queue, so a stored track can never be mutated
// through a pointer the caller still holds -- the mistake a real database
// makes impossible.
func cloneTracks(tracks []*domain.Track) []*domain.Track {
	if tracks == nil {
		return nil
	}
	out := make([]*domain.Track, len(tracks))
	for i, t := range tracks {
		cp := *t
		out[i] = &cp
	}
	return out
}
