package ws

import (
	"encoding/json"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/gorilla/websocket"
)

var testLog = slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError}))

var upgrader = websocket.Upgrader{CheckOrigin: func(r *http.Request) bool { return true }}

// testWS returns a client conn and a server conn that are connected to each other.
// The server conn is what should be used with ConnectionManager (matching production).
func testWS(t *testing.T) (client *websocket.Conn, server *websocket.Conn, cleanup func()) {
	t.Helper()

	ready := make(chan struct{})
	var sConn *websocket.Conn

	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var err error
		sConn, err = upgrader.Upgrade(w, r, nil)
		if err != nil {
			return
		}
		close(ready)
		// Keep connection alive - just drain messages
		for {
			if _, _, err := sConn.ReadMessage(); err != nil {
				return
			}
		}
	}))

	wsURL := "ws" + strings.TrimPrefix(ts.URL, "http")
	client, _, err := websocket.DefaultDialer.Dial(wsURL, nil)
	if err != nil {
		ts.Close()
		t.Fatalf("dial failed: %v", err)
	}

	<-ready

	return client, sConn, func() {
		client.Close()
		sConn.Close()
		ts.Close()
	}
}

func TestGetCountEmpty(t *testing.T) {
	cm := NewConnectionManager(testLog)
	if cm.GetCount("room1") != 0 {
		t.Error("expected 0 for empty room")
	}
}

func TestConnectAndGetCount(t *testing.T) {
	cm := NewConnectionManager(testLog)
	_, serverConn, cleanup := testWS(t)
	defer cleanup()

	cm.Connect("room1", serverConn)
	if cm.GetCount("room1") != 1 {
		t.Errorf("expected 1, got %d", cm.GetCount("room1"))
	}
}

func TestMultipleConnections(t *testing.T) {
	cm := NewConnectionManager(testLog)
	_, s1, cleanup1 := testWS(t)
	_, s2, cleanup2 := testWS(t)
	defer cleanup1()
	defer cleanup2()

	cm.Connect("room1", s1)
	cm.Connect("room1", s2)
	if cm.GetCount("room1") != 2 {
		t.Errorf("expected 2, got %d", cm.GetCount("room1"))
	}
}

func TestMultipleRoomsCount(t *testing.T) {
	cm := NewConnectionManager(testLog)
	_, s1, cleanup1 := testWS(t)
	_, s2, cleanup2 := testWS(t)
	defer cleanup1()
	defer cleanup2()

	cm.Connect("room1", s1)
	cm.Connect("room2", s2)
	if cm.GetCount("room1") != 1 {
		t.Errorf("expected room1=1, got %d", cm.GetCount("room1"))
	}
	if cm.GetCount("room2") != 1 {
		t.Errorf("expected room2=1, got %d", cm.GetCount("room2"))
	}
}

func TestDisconnect(t *testing.T) {
	cm := NewConnectionManager(testLog)
	_, serverConn, cleanup := testWS(t)
	defer cleanup()

	cm.Connect("room1", serverConn)
	if cm.GetCount("room1") != 1 {
		t.Fatal("expected 1")
	}

	cm.Disconnect("room1", serverConn)
	if cm.GetCount("room1") != 0 {
		t.Errorf("expected 0 after disconnect, got %d", cm.GetCount("room1"))
	}
}

func TestDisconnectCleansUpEmptyRoom(t *testing.T) {
	cm := NewConnectionManager(testLog)
	_, serverConn, cleanup := testWS(t)
	defer cleanup()

	cm.Connect("room1", serverConn)
	cm.Disconnect("room1", serverConn)

	if cm.GetCount("room1") != 0 {
		t.Error("room should be cleaned up after last disconnect")
	}
}

func TestDisconnectFromNonexistent(t *testing.T) {
	cm := NewConnectionManager(testLog)
	_, serverConn, cleanup := testWS(t)
	defer cleanup()

	cm.Disconnect("nonexistent", serverConn)
}

func TestBroadcastToEmptyRoom(t *testing.T) {
	cm := NewConnectionManager(testLog)
	cm.Broadcast("empty", map[string]string{"msg": "hello"})
}

func TestBroadcast(t *testing.T) {
	cm := NewConnectionManager(testLog)
	clientConn, serverConn, cleanup := testWS(t)
	defer cleanup()

	cm.Connect("room1", serverConn)

	data := map[string]string{"type": "chat", "message": "hello"}
	cm.Broadcast("room1", data)

	// Read the broadcast message on the client side
	clientConn.SetReadDeadline(time.Now().Add(2 * time.Second))
	_, msg, err := clientConn.ReadMessage()
	if err != nil {
		t.Fatalf("failed to read message: %v", err)
	}
	var received map[string]any
	json.Unmarshal(msg, &received)
	if received["type"] != "chat" || received["message"] != "hello" {
		t.Errorf("unexpected message: %s", string(msg))
	}
}

func TestBroadcastDeadConnection(t *testing.T) {
	cm := NewConnectionManager(testLog)
	_, serverConn, cleanup := testWS(t)
	defer cleanup()

	cm.Connect("room1", serverConn)
	serverConn.Close()

	// Broadcast should not crash
	cm.Broadcast("room1", map[string]string{"msg": "test"})

	if cm.GetCount("room1") != 0 {
		t.Errorf("expected dead connection removed, got count %d", cm.GetCount("room1"))
	}
}

func TestConcurrentBroadcasts(t *testing.T) {
	cm := NewConnectionManager(testLog)
	type connPair struct {
		client *websocket.Conn
		server *websocket.Conn
	}
	pairs := make([]connPair, 5)
	cleanups := make([]func(), 5)
	for i := 0; i < 5; i++ {
		c, s, cleanup := testWS(t)
		pairs[i] = connPair{c, s}
		cleanups[i] = cleanup
		cm.Connect("room1", s)
	}
	for _, c := range cleanups {
		defer c()
	}

	var wg sync.WaitGroup
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func(n int) {
			defer wg.Done()
			cm.Broadcast("room1", map[string]int{"n": n})
		}(i)
	}
	wg.Wait()
}

func TestConcurrentConnectDisconnect(t *testing.T) {
	cm := NewConnectionManager(testLog)

	// Pre-create connections to avoid too many servers
	type connPair struct {
		client *websocket.Conn
		server *websocket.Conn
	}
	pairs := make([]connPair, 10)
	cleanups := make([]func(), 10)
	for i := 0; i < 10; i++ {
		c, s, cleanup := testWS(t)
		pairs[i] = connPair{c, s}
		cleanups[i] = cleanup
	}
	for _, c := range cleanups {
		defer c()
	}

	var wg sync.WaitGroup
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func(n int) {
			defer wg.Done()
			cm.Connect("room1", pairs[n].server)
			cm.Disconnect("room1", pairs[n].server)
		}(i)
	}
	wg.Wait()

	if cm.GetCount("room1") != 0 {
		t.Errorf("expected 0 after all disconnects, got %d", cm.GetCount("room1"))
	}
}

func TestSendJSON(t *testing.T) {
	cm := NewConnectionManager(testLog)
	clientConn, serverConn, cleanup := testWS(t)
	defer cleanup()

	// SendJSON writes to the connection (server→client direction)
	data := map[string]string{"type": "reaction", "emoji": "thumbsup"}
	cm.SendJSON("room1", serverConn, data)

	clientConn.SetReadDeadline(time.Now().Add(2 * time.Second))
	_, msg, err := clientConn.ReadMessage()
	if err != nil {
		t.Fatalf("failed to read message: %v", err)
	}
	var received map[string]any
	json.Unmarshal(msg, &received)
	if received["emoji"] != "thumbsup" {
		t.Errorf("unexpected message: %s", string(msg))
	}
}
