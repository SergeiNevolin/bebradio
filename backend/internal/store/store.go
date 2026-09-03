// Package store defines how rooms, users and their contents are persisted.
//
// The interface is what the rest of the service depends on; package
// store/postgres is the production implementation and store/memory is an
// in-process one used by tests.
package store

import (
	"context"
	"errors"

	"github.com/leenzstra/bebradio/backend/internal/domain"
)

// ErrNotFound is returned when a lookup by identifier matches nothing.
var ErrNotFound = errors.New("store: not found")

// ErrConflict is returned when a write would violate a uniqueness constraint,
// such as registering an email or username that is already taken.
var ErrConflict = errors.New("store: already exists")

// RoomRecord is a room's persisted settings, without its queue, chat or votes.
type RoomRecord struct {
	ID                string
	Name              string
	OwnerID           string
	AllowAnonymousAdd bool
	IsPrivate         bool
	PasswordHash      string
	AutoRadio         bool
	CreatedAt         float64
}

// RoomContents is everything belonging to a room, loaded together.
type RoomContents struct {
	Room     RoomRecord
	Tracks   []*domain.Track
	Messages []domain.ChatMessage
	Votes    []domain.TrackVote
}

// PublicRoom is one entry in the public room listing, as stored.
type PublicRoom struct {
	ID          string
	Name        string
	TrackCount  int
	HasPassword bool
}

// Store is the service's persistence port.
//
// Implementations must be safe for concurrent use. Every method takes a context
// and must honour its cancellation.
type Store interface {
	// --- Users ---

	CreateUser(ctx context.Context, user domain.User) error
	UserByID(ctx context.Context, id string) (domain.User, error)
	UserByEmail(ctx context.Context, email string) (domain.User, error)
	UserByUsername(ctx context.Context, username string) (domain.User, error)
	// UpdateProfile writes the editable parts of a profile and returns the
	// stored result.
	UpdateProfile(ctx context.Context, id string, bio, avatarURL *string) (domain.User, error)

	// --- Rooms ---

	// SaveRoom inserts or updates a room's settings. It never touches the
	// room's queue, chat or votes.
	SaveRoom(ctx context.Context, room RoomRecord) error
	// LoadRoom returns a room and everything belonging to it, or ErrNotFound.
	LoadRoom(ctx context.Context, id string) (RoomContents, error)
	// DeleteRoom removes a room and everything belonging to it.
	DeleteRoom(ctx context.Context, id string) error
	// ListPublicRooms returns every room not marked private.
	ListPublicRooms(ctx context.Context) ([]PublicRoom, error)

	// --- Room contents ---

	// ReplaceTracks makes the stored queue match tracks exactly, preserving
	// order. The whole queue is rewritten because tracks are reordered and
	// removed as often as they are added.
	ReplaceTracks(ctx context.Context, roomID string, tracks []*domain.Track) error
	// AppendMessage stores one chat message.
	AppendMessage(ctx context.Context, roomID string, msg domain.ChatMessage) error
	// ReplaceVotes makes the stored votes for a room match votes exactly.
	ReplaceVotes(ctx context.Context, roomID string, votes []domain.TrackVote) error

	// Ping reports whether the backing store is reachable.
	Ping(ctx context.Context) error
	// Close releases the store's resources.
	Close()
}
