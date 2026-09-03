package api

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/gorilla/websocket"

	"github.com/leenzstra/bebradio/backend/internal/hub"
	"github.com/leenzstra/bebradio/backend/internal/room"
)

// actionTimeout bounds the work one client message may do. A message can end up
// re-resolving stream URLs through yt-dlp, so the budget is generous -- but it
// is not unbounded, or a stuck lookup would pin the connection's reader
// goroutine forever.
const actionTimeout = 90 * time.Second

// clientMessage is one instruction from a listener's browser.
//
// The client sends a single flat object per action, so every field a room
// action might need is declared here and only the relevant ones are read.
type clientMessage struct {
	Action   string   `json:"action"`
	UserID   string   `json:"user_id"`
	Username string   `json:"username"`
	Index    *int     `json:"index"`
	Position *float64 `json:"position"`
	Text     string   `json:"text"`
	TrackID  string   `json:"track_id"`
	Vote     int      `json:"vote"`
	Emoji    string   `json:"emoji"`
}

// handleWebSocket serves a listener's live connection to a room.
//
// The upgrade happens before the room is checked so that a rejection can be
// explained over the socket itself: the browser client cannot read the body of
// a failed handshake, but it can read one message and then a close.
func (s *Server) handleWebSocket(w http.ResponseWriter, r *http.Request) {
	roomID := room.NormalizeID(chi.URLParam(r, "roomID"))
	log := s.logger(r).With("room_id", roomID)

	conn, err := s.upgrader.Upgrade(w, r, nil)
	if err != nil {
		// Upgrade has already written its own error response.
		log.Debug("websocket upgrade failed", "error", err)
		return
	}

	client := s.hub.Add(roomID, conn)
	defer s.hub.Remove(client)

	caller := room.Caller{AccessToken: r.URL.Query().Get("access")}
	if user := currentUser(r); user != nil {
		caller.UserID = user.ID
		caller.Username = user.Username
	}

	// The connection is not tied to the request context: r.Context() is
	// cancelled as soon as the handler returns on some servers, and this
	// connection outlives the handshake by hours.
	connCtx := context.Background()

	live, snapshot, err := s.rooms.Connect(connCtx, roomID, client.ID, caller)
	if err != nil {
		s.rejectConnection(client, err, log)
		return
	}
	defer live.Disconnect()

	client.SendJSON(snapshot)

	if err := client.ConfigureReader(); err != nil {
		log.Debug("configuring websocket reader", "error", err)
		return
	}

	for {
		data, err := client.ReadMessage()
		if err != nil {
			if websocket.IsUnexpectedCloseError(err, websocket.CloseNormalClosure, websocket.CloseGoingAway) {
				log.Debug("websocket closed unexpectedly", "error", err)
			}
			return
		}

		var msg clientMessage
		if err := json.Unmarshal(data, &msg); err != nil {
			log.Debug("discarding malformed client message", "error", err)
			continue
		}
		s.dispatch(connCtx, live, msg)
	}
}

// rejectConnection tells a client why it may not join, then closes the socket.
func (s *Server) rejectConnection(client *hub.Client, err error, log *slog.Logger) {
	switch {
	case errors.Is(err, room.ErrNotFound):
		client.SendJSONAndClose(map[string]any{"error": "Room not found"})
	case errors.Is(err, room.ErrLocked):
		// The client watches for `locked` to decide whether to show the
		// password prompt instead of an error.
		client.SendJSONAndClose(map[string]any{"error": "Password required", "locked": true})
	default:
		log.Debug("rejecting websocket connection", "error", err)
		client.SendJSONAndClose(map[string]any{"error": "Could not join room"})
	}
}

// dispatch runs one client instruction.
func (s *Server) dispatch(parent context.Context, live *room.Connection, msg clientMessage) {
	ctx, cancel := context.WithTimeout(parent, actionTimeout)
	defer cancel()

	live.Touch()
	// Actions carry the account they come from, so a connection that starts
	// acting without having introduced itself is still attributed.
	live.NoteUser(msg.UserID)

	switch msg.Action {
	case "hello":
		live.Identify(msg.UserID, msg.Username)
	case "reaction":
		live.Reaction(msg.Emoji, msg.Username)
	case "chat":
		live.Chat(ctx, msg.UserID, msg.Username, msg.Text)
	case "vote":
		live.Vote(ctx, msg.UserID, msg.TrackID, msg.Vote)
	case "skip_vote":
		live.SkipVote(ctx, msg.UserID)
	case "clear_skip_votes":
		live.ClearSkipVotes(ctx)
	case "next", "prev", "jump", "seek", "sync":
		live.Playback(ctx, room.PlaybackCommand{
			Action:   msg.Action,
			Index:    msg.Index,
			Position: msg.Position,
		})
	default:
		// An unknown action is ignored: a newer client may send actions this
		// build does not know about, and dropping the connection over one
		// would be worse than doing nothing.
	}
}
