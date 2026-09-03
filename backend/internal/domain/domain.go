// Package domain holds the listening-room state and the rules that move it
// forward. It is deliberately free of I/O: nothing here talks to the database,
// to YouTube, or to a WebSocket.
//
// # Synchronisation
//
// A Room is shared by every listener's connection plus the server's background
// loops, but it carries no lock of its own. Callers must hold the room's lock
// for the whole of any read or mutation; package room owns that lock and is the
// only place rooms are handed out. Keeping the mutex outside the state keeps
// these rules pure and directly testable.
package domain

import (
	"sort"
	"time"

	"github.com/leenzstra/bebradio/backend/internal/ids"
)

// Now returns the current wall-clock time as fractional epoch seconds, the
// representation used on the wire and in the database.
func Now() float64 {
	return float64(time.Now().UnixNano()) / float64(time.Second)
}

// Track is one entry in a room's queue.
type Track struct {
	ID        string
	Title     string
	Artist    string
	URL       string
	Thumbnail string
	Duration  int
	AddedBy   string
	AddedAt   float64
	// SourceURL is the original YouTube watch URL, kept so the playable URL
	// (a googlevideo link that expires after a few hours) can be re-resolved on
	// demand.
	SourceURL string
	// StreamExpiresAt is the epoch second at which URL stops working; zero
	// means "unknown".
	StreamExpiresAt float64
}

// TrackInfo is the subset of a YouTube lookup a Track is built from. It mirrors
// youtube.TrackInfo without importing it, keeping this package I/O-free.
type TrackInfo struct {
	Title     string
	Artist    string
	StreamURL string
	Thumbnail string
	Duration  int
	SourceURL string
	ExpiresAt float64
}

// NewTrackFromInfo builds a queue track from a YouTube lookup result.
func NewTrackFromInfo(info TrackInfo, addedBy string) *Track {
	title := info.Title
	if title == "" {
		title = "Unknown"
	}
	artist := info.Artist
	if artist == "" {
		artist = "Unknown"
	}
	if addedBy == "" {
		addedBy = "Anonymous"
	}
	return &Track{
		ID:              ids.Short(),
		Title:           title,
		Artist:          artist,
		URL:             info.StreamURL,
		Thumbnail:       info.Thumbnail,
		Duration:        info.Duration,
		AddedBy:         addedBy,
		AddedAt:         Now(),
		SourceURL:       info.SourceURL,
		StreamExpiresAt: info.ExpiresAt,
	}
}

// ChatMessage is one line of room chat.
type ChatMessage struct {
	ID        string
	UserID    string
	Username  string
	Text      string
	CreatedAt float64
}

// NewChatMessage builds a chat message stamped with the current time.
func NewChatMessage(userID, username, text string) ChatMessage {
	return ChatMessage{
		ID:        ids.Short(),
		UserID:    userID,
		Username:  username,
		Text:      text,
		CreatedAt: Now(),
	}
}

// TrackVote is one listener's like or dislike of one track.
type TrackVote struct {
	UserID  string
	TrackID string
	Vote    int
}

// Listener is a person currently connected to a room.
type Listener struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

// User is an account.
type User struct {
	ID           string
	Email        string
	Username     string
	PasswordHash string
	Bio          string
	AvatarURL    string
	CreatedAt    float64
}

// Room is the live state of one listening room: what is queued, where playback
// has got to, who is connected, and what has been said.
type Room struct {
	ID                string
	Name              string
	OwnerID           string
	Queue             []*Track
	CurrentIndex      int
	IsPlaying         bool
	Position          float64
	LastSyncAt        float64
	CreatedAt         float64
	AllowAnonymousAdd bool
	IsPrivate         bool
	PasswordHash      string
	AutoRadio         bool
	Messages          []ChatMessage
	Votes             []TrackVote
	SkipVotes         map[string]struct{}

	// --- runtime-only state (never persisted) ---

	// Users maps a connection to the account id it identified itself with.
	Users map[uint64]string
	// Presence maps a connection to the identity shown in the listener list.
	Presence map[uint64]Listener
	// LastAdvanceAt is the epoch of the last successful GoNext; it guards
	// against double-skips.
	LastAdvanceAt float64
	// RadioSeedURL is the source URL of the most recently played track, used to
	// seed auto-radio.
	RadioSeedURL string
	// RadioSeen holds video ids already queued by auto-radio this session, so a
	// later refill does not offer the same track again.
	RadioSeen map[string]struct{}
	// RadioFilling is true while a background auto-radio refill is in flight.
	RadioFilling bool
}

// NewRoom returns an empty room owned by ownerID.
func NewRoom(id, name, ownerID string) *Room {
	if id == "" {
		id = ids.Room()
	}
	now := Now()
	return &Room{
		ID:                id,
		Name:              name,
		OwnerID:           ownerID,
		AllowAnonymousAdd: true,
		LastSyncAt:        now,
		CreatedAt:         now,
		SkipVotes:         map[string]struct{}{},
		Users:             map[uint64]string{},
		Presence:          map[uint64]Listener{},
		RadioSeen:         map[string]struct{}{},
	}
}

// CurrentTrack returns the track being played, or nil when the queue is empty
// or the index has drifted out of range.
func (r *Room) CurrentTrack() *Track {
	if len(r.Queue) > 0 && r.CurrentIndex >= 0 && r.CurrentIndex < len(r.Queue) {
		return r.Queue[r.CurrentIndex]
	}
	return nil
}

// TrackByID returns the queued track with the given id, or nil.
func (r *Room) TrackByID(id string) *Track {
	for _, t := range r.Queue {
		if t.ID == id {
			return t
		}
	}
	return nil
}

// CurrentPosition is how far into the current track playback has got, in
// seconds. While playing it extrapolates from the last sync point, so every
// client that asks gets the same answer without the server ticking a counter.
func (r *Room) CurrentPosition() float64 {
	if r.IsPlaying {
		return r.Position + (Now() - r.LastSyncAt)
	}
	return r.Position
}

// VoteCount is the like/dislike tally for one track.
type VoteCount struct {
	Likes    int `json:"likes"`
	Dislikes int `json:"dislikes"`
}

// TrackVotes tallies the votes cast on one track.
func (r *Room) TrackVotes(trackID string) VoteCount {
	var out VoteCount
	for _, v := range r.Votes {
		if v.TrackID != trackID {
			continue
		}
		switch v.Vote {
		case 1:
			out.Likes++
		case -1:
			out.Dislikes++
		}
	}
	return out
}

// SetVote records or clears one listener's vote on one track. A vote of 0 (or
// anything outside -1/1) clears it.
func (r *Room) SetVote(userID, trackID string, vote int) {
	kept := r.Votes[:0]
	for _, v := range r.Votes {
		if v.UserID == userID && v.TrackID == trackID {
			continue
		}
		kept = append(kept, v)
	}
	r.Votes = kept
	if vote == 1 || vote == -1 {
		r.Votes = append(r.Votes, TrackVote{UserID: userID, TrackID: trackID, Vote: vote})
	}
}

// ToggleSkipVote flips one listener's vote to skip the current track and
// reports how many listeners now want it skipped.
func (r *Room) ToggleSkipVote(userID string) int {
	if r.SkipVotes == nil {
		r.SkipVotes = map[string]struct{}{}
	}
	if _, ok := r.SkipVotes[userID]; ok {
		delete(r.SkipVotes, userID)
	} else {
		r.SkipVotes[userID] = struct{}{}
	}
	return len(r.SkipVotes)
}

// Listeners returns the distinct people currently connected. Logged-in users
// are de-duplicated by account id (one person with two tabs is one listener);
// each anonymous connection counts once.
func (r *Room) Listeners() []Listener {
	seen := make(map[string]string, len(r.Presence))
	for _, info := range r.Presence {
		seen[info.ID] = info.Name
	}
	out := make([]Listener, 0, len(seen))
	for id, name := range seen {
		out = append(out, Listener{ID: id, Name: name})
	}
	// Map iteration order is random; sort so repeated snapshots of unchanged
	// state are byte-identical and clients do not see phantom reorderings.
	sort.Slice(out, func(i, j int) bool { return out[i].ID < out[j].ID })
	return out
}

// AppendMessage adds a chat message, trimming the in-memory backlog to max.
func (r *Room) AppendMessage(msg ChatMessage, max int) {
	r.Messages = append(r.Messages, msg)
	if max > 0 && len(r.Messages) > max {
		r.Messages = append([]ChatMessage(nil), r.Messages[len(r.Messages)-max:]...)
	}
}

// SetPresence records the identity a connection announced.
func (r *Room) SetPresence(conn uint64, listener Listener) {
	if r.Presence == nil {
		r.Presence = map[uint64]Listener{}
	}
	r.Presence[conn] = listener
}

// SetUser records the account a connection is authenticated as.
func (r *Room) SetUser(conn uint64, userID string) {
	if r.Users == nil {
		r.Users = map[uint64]string{}
	}
	r.Users[conn] = userID
}

// DropConnection forgets everything tied to a disconnected connection.
func (r *Room) DropConnection(conn uint64) {
	delete(r.Presence, conn)
	delete(r.Users, conn)
}

// distinctUsers counts the distinct accounts announced over the room's
// connections. It backs the listener count when presence has not been reported.
func (r *Room) distinctUsers() int {
	seen := make(map[string]struct{}, len(r.Users))
	for _, id := range r.Users {
		seen[id] = struct{}{}
	}
	return len(seen)
}
