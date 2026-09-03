package room

import (
	"context"
	"slices"
	"strings"

	"github.com/leenzstra/bebradio/backend/internal/domain"
	"github.com/leenzstra/bebradio/backend/internal/ids"
)

// Connection is one listener's live link to a room. The WebSocket handler gets
// one when it connects and hands every incoming message back through it.
//
// It exists so the transport does not have to know about rooms, locks or
// persistence: it reads frames, decodes them, and calls the matching method.
type Connection struct {
	svc *Service
	e   *entry
	// id identifies this connection within the room, for presence.
	id uint64
}

// Connect verifies a listener may enter a room and returns the live connection
// along with the state to send them first.
//
// connID must be unique for the lifetime of the process; the hub's client id is
// what the WebSocket handler passes.
func (s *Service) Connect(ctx context.Context, roomID string, connID uint64, caller Caller) (*Connection, domain.RoomDTO, error) {
	e, err := s.requireAccess(ctx, roomID, caller)
	if err != nil {
		return nil, domain.RoomDTO{}, err
	}
	return &Connection{svc: s, e: e, id: connID}, s.snapshot(e), nil
}

// Disconnect forgets the listener and tells the room who is left.
func (c *Connection) Disconnect() {
	c.e.mu.Lock()
	c.e.state.DropConnection(c.id)
	c.e.mu.Unlock()
	c.svc.broadcast(c.e)
}

// Identify records who is on the other end of the connection and announces the
// updated listener list.
//
// An anonymous listener is given an identity derived from the connection, so
// two guests in the same room are counted separately while one person's two
// tabs are counted once.
func (c *Connection) Identify(userID, username string) {
	name := sanitizeUsername(username)
	identity := userID
	if identity == "" {
		identity = anonymousID(c.id)
	}

	c.e.mu.Lock()
	if userID != "" {
		c.e.state.SetUser(c.id, userID)
	}
	c.e.state.SetPresence(c.id, domain.Listener{ID: identity, Name: name})
	c.e.mu.Unlock()

	c.svc.broadcast(c.e)
}

// NoteUser records the account a message claimed to come from, for connections
// that send actions without ever having introduced themselves.
func (c *Connection) NoteUser(userID string) {
	if userID == "" {
		return
	}
	c.e.mu.Lock()
	if _, known := c.e.state.Users[c.id]; !known {
		c.e.state.SetUser(c.id, userID)
	}
	c.e.mu.Unlock()
}

// Playback applies a playback command from a connected listener. Access was
// checked when the connection was made.
func (c *Connection) Playback(ctx context.Context, cmd PlaybackCommand) {
	c.svc.applyPlayback(ctx, c.e, cmd)
}

// Chat stores a chat line and relays it to the room.
//
// Chat is relayed on its own rather than as part of a room snapshot, so a busy
// conversation does not make every client re-render the queue and player.
func (c *Connection) Chat(ctx context.Context, userID, username, text string) {
	text = strings.TrimSpace(text)
	if text == "" {
		return
	}
	if max := c.svc.cfg.MaxChatTextLen; max > 0 && len(text) > max {
		text = text[:max]
	}

	msg := domain.NewChatMessage(userID, sanitizeUsername(username), text)

	c.e.mu.Lock()
	c.e.state.AppendMessage(msg, c.svc.cfg.MaxChatMessages)
	roomID := c.e.state.ID
	c.e.mu.Unlock()

	if err := c.svc.store.AppendMessage(ctx, roomID, msg); err != nil {
		c.svc.log.Error("persisting chat message", "room_id", roomID, "error", err)
	}
	c.svc.hub.Broadcast(roomID, map[string]any{"type": "chat", "message": msg.DTO()})
}

// Reaction relays an emoji to the room. Reactions are never stored, and only
// emoji on the configured allowlist are passed on, so a client cannot broadcast
// arbitrary text.
func (c *Connection) Reaction(emoji, username string) {
	if !slices.Contains(c.svc.cfg.ReactionEmojis, emoji) {
		return
	}
	c.e.mu.Lock()
	roomID := c.e.state.ID
	c.e.mu.Unlock()

	c.svc.hub.Broadcast(roomID, map[string]any{
		"type":     "reaction",
		"id":       ids.Short(),
		"emoji":    emoji,
		"username": sanitizeUsername(username),
	})
}

// Vote records a like or dislike.
//
// A track the room has turned against is skipped: once the current track has
// more dislikes than likes, it is dropped and playback moves on.
func (c *Connection) Vote(ctx context.Context, userID, trackID string, vote int) {
	if userID == "" || trackID == "" {
		return
	}

	c.e.mu.Lock()
	c.e.state.SetVote(userID, trackID, vote)
	current := c.e.state.CurrentTrack()
	rejected := false
	if current != nil && current.ID == trackID {
		tally := c.e.state.TrackVotes(trackID)
		rejected = tally.Dislikes > tally.Likes
	}
	advanced := false
	if rejected {
		advanced = c.e.state.GoNext(c.svc.cfg.AdvanceDedupWindow.Seconds())
		clear(c.e.state.SkipVotes)
	}
	c.e.mu.Unlock()

	c.svc.persistVotes(ctx, c.e)
	c.svc.finishTrackChange(ctx, c.e, advanced)
}

// SkipVote toggles one listener's vote to skip the current track. Once at least
// half the room wants it gone, it goes.
func (c *Connection) SkipVote(ctx context.Context, userID string) {
	if userID == "" {
		return
	}

	c.e.mu.Lock()
	roomID := c.e.state.ID
	votes := c.e.state.ToggleSkipVote(userID)
	c.e.mu.Unlock()

	// The threshold is half the room, counted against at least two listeners:
	// somebody listening alone can still skip, while a busy room needs a real
	// share of it to agree.
	listeners := max(c.svc.hub.Count(roomID), 2)

	advanced := false
	if votes >= listeners/2 {
		c.e.mu.Lock()
		advanced = c.e.state.GoNext(c.svc.cfg.AdvanceDedupWindow.Seconds())
		clear(c.e.state.SkipVotes)
		c.e.mu.Unlock()
	}

	c.svc.finishTrackChange(ctx, c.e, advanced)
}

// ClearSkipVotes discards the pending skip votes, which happens when the track
// they were cast against is no longer playing.
func (c *Connection) ClearSkipVotes(ctx context.Context) {
	c.e.mu.Lock()
	clear(c.e.state.SkipVotes)
	c.e.mu.Unlock()
	c.svc.finishTrackChange(ctx, c.e, false)
}

// Touch marks the room as recently used so it is not evicted from memory while
// somebody is connected.
func (c *Connection) Touch() {
	c.svc.mu.Lock()
	c.e.lastTouched = timeNow()
	c.svc.mu.Unlock()
}

// finishTrackChange persists, refreshes and broadcasts after an action that may
// have moved the queue on.
func (s *Service) finishTrackChange(ctx context.Context, e *entry, advanced bool) {
	if advanced {
		if _, err := s.refreshTracks(ctx, e, currentAndNext); err != nil {
			s.log.Warn("refreshing streams after track change", "error", err)
		}
		s.persistTracks(ctx, e)
	}
	s.broadcast(e)
	s.maybeRefill(e)
}

// sanitizeUsername trims a client-supplied display name to something safe to
// show to the rest of the room.
func sanitizeUsername(name string) string {
	name = strings.TrimSpace(name)
	if name == "" {
		return "Anonymous"
	}
	runes := []rune(name)
	if len(runes) > 30 {
		name = string(runes[:30])
	}
	return name
}

func anonymousID(connID uint64) string {
	return "anon:" + itoa(connID)
}
