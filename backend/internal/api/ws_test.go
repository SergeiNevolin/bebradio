package api

import (
	"encoding/json"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/gorilla/websocket"
)

// wsClient is one listener's live connection, wrapped for assertions.
type wsClient struct {
	t    *testing.T
	conn *websocket.Conn
}

// dialWS opens a WebSocket to a room. query is appended to the URL as-is.
func (h *harness) dialWS(roomID, query string) *wsClient {
	h.t.Helper()

	url := "ws" + strings.TrimPrefix(h.server.URL, "http") + "/ws/" + roomID + query
	conn, _, err := websocket.DefaultDialer.DialContext(h.t.Context(), url, nil)
	if err != nil {
		h.t.Fatalf("dialling %s: %v", url, err)
	}
	h.t.Cleanup(func() { _ = conn.Close() })
	return &wsClient{t: h.t, conn: conn}
}

func (c *wsClient) send(payload map[string]any) {
	c.t.Helper()
	if err := c.conn.WriteJSON(payload); err != nil {
		c.t.Fatalf("sending %v: %v", payload, err)
	}
}

// receive reads the next message, failing the test if none arrives.
func (c *wsClient) receive() map[string]any {
	c.t.Helper()
	if err := c.conn.SetReadDeadline(time.Now().Add(5 * time.Second)); err != nil {
		c.t.Fatalf("setting read deadline: %v", err)
	}
	_, data, err := c.conn.ReadMessage()
	if err != nil {
		c.t.Fatalf("reading message: %v", err)
	}
	var out map[string]any
	if err := json.Unmarshal(data, &out); err != nil {
		c.t.Fatalf("decoding %s: %v", data, err)
	}
	return out
}

// receiveUntil reads messages until one satisfies match, so a test is not
// tripped up by the unrelated state broadcasts a busy room produces.
func (c *wsClient) receiveUntil(what string, match func(map[string]any) bool) map[string]any {
	c.t.Helper()
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		msg := c.receive()
		if match(msg) {
			return msg
		}
	}
	c.t.Fatalf("no message matching %s arrived", what)
	return nil
}

func isType(name string) func(map[string]any) bool {
	return func(msg map[string]any) bool { return msg["type"] == name }
}

// isRoomState matches a full room snapshot, which carries no "type" field.
func isRoomState(msg map[string]any) bool {
	_, typed := msg["type"]
	_, queued := msg["queue"]
	return !typed && queued
}

func TestWebSocketSendsTheRoomStateOnConnect(t *testing.T) {
	h := newHarness(t)
	token := h.register("Alice", "a@b.com")
	roomID, _ := h.createRoom(token, "Live", nil)

	client := h.dialWS(roomID, "")

	state := client.receive()
	if state["id"] != roomID {
		t.Errorf("first message = %v, want the room state", state)
	}
}

func TestWebSocketRejectsAnUnknownRoom(t *testing.T) {
	h := newHarness(t)

	msg := h.dialWS("XXXXXX", "").receive()

	if msg["error"] != "Room not found" {
		t.Errorf("message = %v, want a not-found error", msg)
	}
}

// A locked room must tell the client to prompt for a password rather than just
// dropping the connection.
func TestWebSocketRejectsALockedRoom(t *testing.T) {
	h := newHarness(t)
	token := h.register("Alice", "a@b.com")
	password := "hunter2"
	roomID, _ := h.createRoom(token, "Locked", &password)
	h.rooms.Forget(roomID)

	msg := h.dialWS(roomID, "").receive()

	if msg["error"] != "Password required" || msg["locked"] != true {
		t.Errorf("message = %v, want a locked notice", msg)
	}
}

func TestWebSocketAcceptsAValidAccessToken(t *testing.T) {
	h := newHarness(t)
	token := h.register("Alice", "a@b.com")
	password := "hunter2"
	roomID, access := h.createRoom(token, "Locked", &password)

	state := h.dialWS(roomID, "?access="+access).receive()

	if state["id"] != roomID {
		t.Errorf("message = %v, want the room state", state)
	}
}

// Announcing yourself puts you in the listener list, and everyone already in
// the room is told.
func TestWebSocketPresence(t *testing.T) {
	h := newHarness(t)
	token := h.register("Alice", "a@b.com")
	roomID, _ := h.createRoom(token, "Live", nil)

	alice := h.dialWS(roomID, "")
	alice.receive() // the initial state

	alice.send(map[string]any{"action": "hello", "user_id": "u1", "username": "Alice"})

	state := alice.receiveUntil("a listener list containing Alice", func(msg map[string]any) bool {
		listeners, ok := msg["listeners"].([]any)
		return ok && len(listeners) == 1
	})

	listener := state["listeners"].([]any)[0].(map[string]any)
	if listener["id"] != "u1" || listener["name"] != "Alice" {
		t.Errorf("listener = %v", listener)
	}
	if state["user_count"] != float64(1) {
		t.Errorf("user_count = %v, want 1", state["user_count"])
	}
}

// An anonymous listener gets an identity derived from their connection, so two
// guests are counted separately.
func TestWebSocketCountsAnonymousListenersSeparately(t *testing.T) {
	h := newHarness(t)
	token := h.register("Alice", "a@b.com")
	roomID, _ := h.createRoom(token, "Live", nil)

	first := h.dialWS(roomID, "")
	first.receive()
	first.send(map[string]any{"action": "hello", "username": "Guest"})
	first.receiveUntil("the first listener", func(msg map[string]any) bool {
		listeners, ok := msg["listeners"].([]any)
		return ok && len(listeners) == 1
	})

	second := h.dialWS(roomID, "")
	second.receive()
	second.send(map[string]any{"action": "hello", "username": "Guest"})

	state := second.receiveUntil("two listeners", func(msg map[string]any) bool {
		listeners, ok := msg["listeners"].([]any)
		return ok && len(listeners) == 2
	})
	if state["user_count"] != float64(2) {
		t.Errorf("user_count = %v, want 2", state["user_count"])
	}
}

// Chat is relayed on its own rather than inside a room snapshot, so a busy
// conversation does not make every client re-render the player and queue.
func TestWebSocketChatIsRelayedToEveryone(t *testing.T) {
	h := newHarness(t)
	token := h.register("Alice", "a@b.com")
	roomID, _ := h.createRoom(token, "Live", nil)

	alice := h.dialWS(roomID, "")
	alice.receive()
	bob := h.dialWS(roomID, "")
	bob.receive()

	alice.send(map[string]any{
		"action": "chat", "user_id": "u1", "username": "Alice", "text": "  hello everyone  ",
	})

	msg := bob.receiveUntil("a chat message", isType("chat"))
	chat := msg["message"].(map[string]any)
	if chat["text"] != "hello everyone" {
		t.Errorf("text = %v, want it trimmed", chat["text"])
	}
	if chat["username"] != "Alice" || chat["user_id"] != "u1" {
		t.Errorf("chat = %v", chat)
	}

	// It must also have been stored, so it is there on the next page load.
	contents, err := h.store.LoadRoom(t.Context(), roomID)
	if err != nil {
		t.Fatalf("LoadRoom() error = %v", err)
	}
	if len(contents.Messages) != 1 || contents.Messages[0].Text != "hello everyone" {
		t.Errorf("stored messages = %+v", contents.Messages)
	}
}

func TestWebSocketIgnoresEmptyChat(t *testing.T) {
	h := newHarness(t)
	token := h.register("Alice", "a@b.com")
	roomID, _ := h.createRoom(token, "Live", nil)

	client := h.dialWS(roomID, "")
	client.receive()
	client.send(map[string]any{"action": "chat", "username": "Alice", "text": "   "})
	// A message that would provoke a reply, so the test does not have to wait
	// on a timeout to know the empty one produced nothing.
	client.send(map[string]any{"action": "hello", "username": "Alice"})

	if got := client.receive(); got["type"] == "chat" {
		t.Errorf("an empty chat message was relayed: %v", got)
	}
}

// Only emoji on the allowlist are relayed, so a client cannot broadcast
// arbitrary text into everybody's player.
func TestWebSocketReactions(t *testing.T) {
	h := newHarness(t)
	token := h.register("Alice", "a@b.com")
	roomID, _ := h.createRoom(token, "Live", nil)

	alice := h.dialWS(roomID, "")
	alice.receive()
	bob := h.dialWS(roomID, "")
	bob.receive()

	alice.send(map[string]any{"action": "reaction", "emoji": "not-an-emoji", "username": "Alice"})
	alice.send(map[string]any{"action": "reaction", "emoji": "\U0001F525", "username": "Alice"})

	msg := bob.receiveUntil("a reaction", isType("reaction"))
	if msg["emoji"] != "\U0001F525" {
		t.Errorf("emoji = %v, want the allowlisted one", msg["emoji"])
	}
	if msg["username"] != "Alice" || msg["id"] == "" {
		t.Errorf("reaction = %v", msg)
	}
}

func TestWebSocketPlaybackReachesEveryone(t *testing.T) {
	h := newHarness(t)
	token := h.register("Alice", "a@b.com")
	roomID, _ := h.createRoom(token, "Live", nil)
	for i := 0; i < 2; i++ {
		h.do(http.MethodPost, "/api/rooms/"+roomID+"/queue", map[string]string{"url": trackURLs[i]}, token)
	}

	alice := h.dialWS(roomID, "")
	alice.receive()
	bob := h.dialWS(roomID, "")
	bob.receive()

	alice.send(map[string]any{"action": "seek", "position": 42.5})

	state := bob.receiveUntil("a seek", func(msg map[string]any) bool {
		if !isRoomState(msg) {
			return false
		}
		position, ok := msg["position"].(float64)
		return ok && position >= 42
	})
	if position := state["position"].(float64); position < 42 || position > 44 {
		t.Errorf("position = %v, want about 42.5", position)
	}
}

// A track the room turns against is dropped: more dislikes than likes on the
// current track skips it.
func TestWebSocketDownvotingSkipsTheCurrentTrack(t *testing.T) {
	h := newHarness(t)
	token := h.register("Alice", "a@b.com")
	roomID, _ := h.createRoom(token, "Live", nil)
	for i := 0; i < 2; i++ {
		h.do(http.MethodPost, "/api/rooms/"+roomID+"/queue", map[string]string{"url": trackURLs[i]}, token)
	}

	client := h.dialWS(roomID, "")
	state := client.receive()
	currentID := state["current_track"].(map[string]any)["id"].(string)

	client.send(map[string]any{
		"action": "vote", "user_id": "u1", "track_id": currentID, "vote": -1,
	})

	after := client.receiveUntil("the track being skipped", func(msg map[string]any) bool {
		if !isRoomState(msg) {
			return false
		}
		queue, ok := msg["queue"].([]any)
		return ok && len(queue) == 1
	})
	if track := after["current_track"].(map[string]any); track["id"] == currentID {
		t.Error("the downvoted track is still playing")
	}
}

func TestWebSocketLikeDoesNotSkip(t *testing.T) {
	h := newHarness(t)
	token := h.register("Alice", "a@b.com")
	roomID, _ := h.createRoom(token, "Live", nil)
	for i := 0; i < 2; i++ {
		h.do(http.MethodPost, "/api/rooms/"+roomID+"/queue", map[string]string{"url": trackURLs[i]}, token)
	}

	client := h.dialWS(roomID, "")
	state := client.receive()
	currentID := state["current_track"].(map[string]any)["id"].(string)

	client.send(map[string]any{"action": "vote", "user_id": "u1", "track_id": currentID, "vote": 1})

	after := client.receiveUntil("the vote tally", func(msg map[string]any) bool {
		if !isRoomState(msg) {
			return false
		}
		votes, ok := msg["track_votes"].(map[string]any)
		return ok && votes["likes"] == float64(1)
	})
	if len(after["queue"].([]any)) != 2 {
		t.Error("a like should not remove the track")
	}
}

// The skip threshold is half the room, counted against at least two listeners.
// Somebody listening on their own can therefore skip; a busier room cannot be
// skipped by one person.
func TestWebSocketSkipVoteBySoleListener(t *testing.T) {
	h := newHarness(t)
	token := h.register("Alice", "a@b.com")
	roomID, _ := h.createRoom(token, "Live", nil)
	for i := 0; i < 2; i++ {
		h.do(http.MethodPost, "/api/rooms/"+roomID+"/queue", map[string]string{"url": trackURLs[i]}, token)
	}

	client := h.dialWS(roomID, "")
	client.receive()

	client.send(map[string]any{"action": "skip_vote", "user_id": "u1"})

	state := client.receiveUntil("the track being skipped", func(msg map[string]any) bool {
		if !isRoomState(msg) {
			return false
		}
		queue, ok := msg["queue"].([]any)
		return ok && len(queue) == 1
	})
	// The votes are cleared once they have been acted on, so the next track
	// does not inherit them.
	if voters := state["skip_voters"].([]any); len(voters) != 0 {
		t.Errorf("skip_voters = %v, want them cleared after the skip", voters)
	}
}

// With four listeners connected, one vote is short of the half needed.
func TestWebSocketSkipVoteNeedsHalfABusyRoom(t *testing.T) {
	h := newHarness(t)
	token := h.register("Alice", "a@b.com")
	roomID, _ := h.createRoom(token, "Live", nil)
	for i := 0; i < 2; i++ {
		h.do(http.MethodPost, "/api/rooms/"+roomID+"/queue", map[string]string{"url": trackURLs[i]}, token)
	}

	clients := make([]*wsClient, 4)
	for i := range clients {
		clients[i] = h.dialWS(roomID, "")
		clients[i].receive()
	}

	clients[0].send(map[string]any{"action": "skip_vote", "user_id": "u1"})

	state := clients[0].receiveUntil("the skip vote being registered", func(msg map[string]any) bool {
		if !isRoomState(msg) {
			return false
		}
		voters, ok := msg["skip_voters"].([]any)
		return ok && len(voters) == 1
	})
	if len(state["queue"].([]any)) != 2 {
		t.Error("one vote in a room of four should not move the queue on")
	}
}

func TestWebSocketDeletingARoomNotifiesListeners(t *testing.T) {
	h := newHarness(t)
	token := h.register("Alice", "a@b.com")
	roomID, _ := h.createRoom(token, "Live", nil)

	client := h.dialWS(roomID, "")
	client.receive()

	if res := h.do(http.MethodDelete, "/api/rooms/"+roomID, nil, token); res.status != http.StatusOK {
		t.Fatalf("deleting the room: %d %s", res.status, res.body)
	}

	msg := client.receiveUntil("the deletion notice", isType("room_deleted"))
	if msg["room_id"] != roomID {
		t.Errorf("notice = %v", msg)
	}
}

// An action this build does not know about must be ignored, not fatal: a newer
// client may send one, and dropping the connection would be worse.
func TestWebSocketIgnoresUnknownActions(t *testing.T) {
	h := newHarness(t)
	token := h.register("Alice", "a@b.com")
	roomID, _ := h.createRoom(token, "Live", nil)

	client := h.dialWS(roomID, "")
	client.receive()

	client.send(map[string]any{"action": "teleport"})
	client.send(map[string]any{"action": "hello", "username": "Alice"})

	state := client.receiveUntil("the room still responding", isRoomState)
	if state["id"] != roomID {
		t.Errorf("state = %v", state)
	}
}

func TestWebSocketIgnoresMalformedFrames(t *testing.T) {
	h := newHarness(t)
	token := h.register("Alice", "a@b.com")
	roomID, _ := h.createRoom(token, "Live", nil)

	client := h.dialWS(roomID, "")
	client.receive()

	if err := client.conn.WriteMessage(websocket.TextMessage, []byte("{not json")); err != nil {
		t.Fatalf("sending a malformed frame: %v", err)
	}
	client.send(map[string]any{"action": "hello", "username": "Alice"})

	if state := client.receiveUntil("the room still responding", isRoomState); state["id"] != roomID {
		t.Errorf("state = %v", state)
	}
}

// Leaving must take the listener out of the room, or a room would appear busy
// long after everyone had gone.
func TestWebSocketDisconnectClearsPresence(t *testing.T) {
	h := newHarness(t)
	token := h.register("Alice", "a@b.com")
	roomID, _ := h.createRoom(token, "Live", nil)

	watcher := h.dialWS(roomID, "")
	watcher.receive()
	watcher.send(map[string]any{"action": "hello", "user_id": "watcher", "username": "Watcher"})
	watcher.receiveUntil("the watcher joining", func(msg map[string]any) bool {
		listeners, ok := msg["listeners"].([]any)
		return ok && len(listeners) == 1
	})

	leaver := h.dialWS(roomID, "")
	leaver.receive()
	leaver.send(map[string]any{"action": "hello", "user_id": "leaver", "username": "Leaver"})
	watcher.receiveUntil("the second listener joining", func(msg map[string]any) bool {
		listeners, ok := msg["listeners"].([]any)
		return ok && len(listeners) == 2
	})

	if err := leaver.conn.Close(); err != nil {
		t.Fatalf("closing the connection: %v", err)
	}

	watcher.receiveUntil("the listener list shrinking again", func(msg map[string]any) bool {
		listeners, ok := msg["listeners"].([]any)
		return ok && len(listeners) == 1
	})
}
