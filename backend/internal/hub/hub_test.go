package hub

import (
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"

	"github.com/gorilla/websocket"
)

// testHub returns a hub and a server that registers every connection made to it.
//
// The connections are real WebSockets rather than fakes: the hub's whole job is
// getting bytes onto them safely from several goroutines at once, which a fake
// connection would not exercise.
func testHub(t *testing.T) (*Hub, *httptest.Server) {
	t.Helper()

	h := New(slog.New(slog.NewTextHandler(io.Discard, nil)))
	upgrader := websocket.Upgrader{CheckOrigin: func(*http.Request) bool { return true }}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		conn, err := upgrader.Upgrade(w, r, nil)
		if err != nil {
			return
		}
		client := h.Add(r.URL.Query().Get("room"), conn)
		defer h.Remove(client)
		// Hold the connection open until the client goes away.
		for {
			if _, _, err := conn.ReadMessage(); err != nil {
				return
			}
		}
	}))
	t.Cleanup(server.Close)

	return h, server
}

func dial(t *testing.T, server *httptest.Server, roomID string) *websocket.Conn {
	t.Helper()
	url := "ws" + server.URL[len("http"):] + "?room=" + roomID
	conn, _, err := websocket.DefaultDialer.DialContext(t.Context(), url, nil)
	if err != nil {
		t.Fatalf("dialling: %v", err)
	}
	t.Cleanup(func() { _ = conn.Close() })
	return conn
}

// waitForCount blocks until a room reports the expected number of connections,
// since registration completes on the server's own goroutine.
func waitForCount(t *testing.T, h *Hub, roomID string, want int) {
	t.Helper()
	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) {
		if h.Count(roomID) == want {
			return
		}
		time.Sleep(5 * time.Millisecond)
	}
	t.Fatalf("room %s has %d connections, want %d", roomID, h.Count(roomID), want)
}

func readJSON(t *testing.T, conn *websocket.Conn) map[string]any {
	t.Helper()
	if err := conn.SetReadDeadline(time.Now().Add(3 * time.Second)); err != nil {
		t.Fatalf("setting read deadline: %v", err)
	}
	_, data, err := conn.ReadMessage()
	if err != nil {
		t.Fatalf("reading: %v", err)
	}
	var out map[string]any
	if err := json.Unmarshal(data, &out); err != nil {
		t.Fatalf("decoding %s: %v", data, err)
	}
	return out
}

func TestBroadcastReachesEveryConnectionInTheRoom(t *testing.T) {
	h, server := testHub(t)

	first := dial(t, server, "ROOM01")
	second := dial(t, server, "ROOM01")
	elsewhere := dial(t, server, "ROOM02")
	waitForCount(t, h, "ROOM01", 2)
	waitForCount(t, h, "ROOM02", 1)

	h.Broadcast("ROOM01", map[string]any{"hello": "room one"})

	for _, conn := range []*websocket.Conn{first, second} {
		if got := readJSON(t, conn)["hello"]; got != "room one" {
			t.Errorf("received %v, want the broadcast", got)
		}
	}

	// The other room must not have heard it.
	h.Broadcast("ROOM02", map[string]any{"hello": "room two"})
	if got := readJSON(t, elsewhere)["hello"]; got != "room two" {
		t.Errorf("room two received %v, want its own broadcast", got)
	}
}

func TestBroadcastToAnEmptyRoomIsHarmless(t *testing.T) {
	h, _ := testHub(t)
	h.Broadcast("NOBODY", map[string]any{"hello": "anyone"})
}

func TestCountAndRoomsTrackConnections(t *testing.T) {
	h, server := testHub(t)

	if got := h.Count("ROOM01"); got != 0 {
		t.Errorf("Count() on an unknown room = %d, want 0", got)
	}

	conn := dial(t, server, "ROOM01")
	waitForCount(t, h, "ROOM01", 1)

	if rooms := h.Rooms(); len(rooms) != 1 || rooms[0] != "ROOM01" {
		t.Errorf("Rooms() = %v, want [ROOM01]", rooms)
	}

	if err := conn.Close(); err != nil {
		t.Fatalf("closing: %v", err)
	}
	waitForCount(t, h, "ROOM01", 0)

	// An emptied room is forgotten entirely, so the map does not grow without
	// bound as rooms come and go.
	if rooms := h.Rooms(); len(rooms) != 0 {
		t.Errorf("Rooms() = %v, want none", rooms)
	}
}

// The hub is written to from every listener's goroutine plus the background
// loops at once. Under -race, this is what proves the locking holds.
func TestConcurrentBroadcastsAndDisconnects(t *testing.T) {
	h, server := testHub(t)

	const listeners = 12
	for i := 0; i < listeners; i++ {
		conn := dial(t, server, "BUSY01")
		// Drain, so a listener never fills its queue and gets dropped for being
		// slow.
		go func() {
			for {
				if _, _, err := conn.ReadMessage(); err != nil {
					return
				}
			}
		}()
	}
	waitForCount(t, h, "BUSY01", listeners)

	var wg sync.WaitGroup
	for i := 0; i < 32; i++ {
		wg.Add(1)
		go func(n int) {
			defer wg.Done()
			h.Broadcast("BUSY01", map[string]any{"n": n})
			_ = h.Count("BUSY01")
			_ = h.Rooms()
		}(i)
	}
	wg.Wait()
}

// rawClient builds a Client with no writer goroutine behind it, so its outbound
// queue can be filled on purpose. Going through the hub would start a writer
// that drains the queue as fast as the test fills it.
func rawClient(t *testing.T) *Client {
	t.Helper()

	upgraded := make(chan *websocket.Conn, 1)
	upgrader := websocket.Upgrader{CheckOrigin: func(*http.Request) bool { return true }}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		conn, err := upgrader.Upgrade(w, r, nil)
		if err != nil {
			return
		}
		upgraded <- conn
	}))
	t.Cleanup(server.Close)

	dial(t, server, "ROOM01")

	select {
	case conn := <-upgraded:
		return &Client{
			ID:     1,
			RoomID: "ROOM01",
			conn:   conn,
			send:   make(chan frame, sendQueue),
			closed: make(chan struct{}),
			log:    slog.New(slog.NewTextHandler(io.Discard, nil)),
		}
	case <-time.After(3 * time.Second):
		t.Fatal("the connection was never upgraded")
		return nil
	}
}

// A client that has stopped reading must be dropped once its queue fills, not
// allowed to hold broadcasting up for the rest of the room.
func TestEnqueueDropsAClientWhoseQueueIsFull(t *testing.T) {
	c := rawClient(t)

	// Exactly enough to fill the queue: none of these may be dropped.
	for i := 0; i < sendQueue; i++ {
		c.enqueue([]byte(`{"n":1}`))
	}
	select {
	case <-c.Closed():
		t.Fatal("the client was dropped before its queue was full")
	default:
	}

	// One more has nowhere to go.
	c.enqueue([]byte(`{"n":1}`))

	select {
	case <-c.Closed():
	case <-time.After(time.Second):
		t.Error("a client whose queue overflowed was not dropped")
	}
}

// Broadcasting must never wait on a client, whatever state it is in.
func TestBroadcastDoesNotBlockOnAStuckClient(t *testing.T) {
	h, server := testHub(t)

	dial(t, server, "ROOM01") // dialled and then never read from
	waitForCount(t, h, "ROOM01", 1)

	started := time.Now()
	for i := 0; i < sendQueue*8; i++ {
		h.Broadcast("ROOM01", map[string]any{"n": i})
	}

	// Had a broadcast waited on the client, this would be seconds.
	if elapsed := time.Since(started); elapsed > time.Second {
		t.Errorf("broadcasting took %s, want it not to block on a client", elapsed)
	}
}

func TestSendJSONAndCloseDeliversItsMessage(t *testing.T) {
	h := New(slog.New(slog.NewTextHandler(io.Discard, nil)))
	upgrader := websocket.Upgrader{CheckOrigin: func(*http.Request) bool { return true }}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		conn, err := upgrader.Upgrade(w, r, nil)
		if err != nil {
			return
		}
		client := h.Add("ROOM01", conn)
		defer h.Remove(client)
		client.SendJSONAndClose(map[string]any{"error": "Room not found"})
	}))
	t.Cleanup(server.Close)

	conn := dial(t, server, "ROOM01")

	// The rejection must arrive before the connection goes; closing first would
	// leave the browser with nothing to explain what happened.
	if got := readJSON(t, conn)["error"]; got != "Room not found" {
		t.Errorf("received %v, want the rejection", got)
	}
	if _, _, err := conn.ReadMessage(); err == nil {
		t.Error("the connection stayed open after the final message")
	}
}
