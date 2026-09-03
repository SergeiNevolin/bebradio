// Package hub tracks the WebSocket connections open on each room and fans
// messages out to them.
//
// A gorilla/websocket connection may only be written from one goroutine, so
// every connection owns a writer goroutine and a buffered outbound queue.
// Broadcasting is therefore never blocked by a slow client: a connection whose
// queue is full is dropped instead, and its reader sees the closed connection
// and cleans up.
package hub

import (
	"encoding/json"
	"log/slog"
	"sync"
	"sync/atomic"
	"time"

	"github.com/gorilla/websocket"
)

const (
	// sendQueue is how many messages may be waiting on one connection before
	// it is considered too slow to keep.
	sendQueue = 32
	// writeWait bounds a single write to a client.
	writeWait = 10 * time.Second
	// pongWait is how long a client may go without answering a ping.
	pongWait = 60 * time.Second
	// pingPeriod must be shorter than pongWait so a ping is always outstanding
	// well before the deadline.
	pingPeriod = 25 * time.Second
	// maxMessageBytes caps an inbound frame. Client messages are small JSON
	// objects; anything larger is a bug or an attack.
	maxMessageBytes = 64 * 1024
)

// frame is one queued outbound message. A frame with a done channel is one the
// sender is waiting on, so it can know the bytes reached the wire before it
// closes the connection.
type frame struct {
	data []byte
	done chan struct{}
	// final marks the last message on a connection: a clean WebSocket close
	// follows it, so the browser can tell a deliberate rejection from a dropped
	// connection and not reconnect in a loop.
	final bool
}

// Client is one open WebSocket connection.
type Client struct {
	// ID uniquely identifies this connection for the lifetime of the process.
	// Rooms key presence by it rather than by pointer, so no room ever holds a
	// reference to a closed connection.
	ID     uint64
	RoomID string

	conn      *websocket.Conn
	send      chan frame
	closeOnce sync.Once
	closed    chan struct{}
	log       *slog.Logger
}

// Hub is the set of connections currently open, grouped by room.
type Hub struct {
	mu    sync.RWMutex
	rooms map[string]map[uint64]*Client
	log   *slog.Logger

	nextID atomic.Uint64
}

// New returns an empty hub.
func New(log *slog.Logger) *Hub {
	if log == nil {
		log = slog.Default()
	}
	return &Hub{rooms: map[string]map[uint64]*Client{}, log: log}
}

// Add registers a connection against a room and starts its writer goroutine.
// The caller must call Remove when the connection ends.
func (h *Hub) Add(roomID string, conn *websocket.Conn) *Client {
	c := &Client{
		ID:     h.nextID.Add(1),
		RoomID: roomID,
		conn:   conn,
		send:   make(chan frame, sendQueue),
		closed: make(chan struct{}),
		log:    h.log,
	}

	h.mu.Lock()
	if h.rooms[roomID] == nil {
		h.rooms[roomID] = map[uint64]*Client{}
	}
	h.rooms[roomID][c.ID] = c
	h.mu.Unlock()

	go c.writePump()
	return c
}

// Remove deregisters a connection and closes it.
func (h *Hub) Remove(c *Client) {
	if c == nil {
		return
	}
	h.mu.Lock()
	if conns, ok := h.rooms[c.RoomID]; ok {
		delete(conns, c.ID)
		if len(conns) == 0 {
			delete(h.rooms, c.RoomID)
		}
	}
	h.mu.Unlock()
	c.Close()
}

// Count returns how many connections are open on a room.
func (h *Hub) Count(roomID string) int {
	h.mu.RLock()
	defer h.mu.RUnlock()
	return len(h.rooms[roomID])
}

// Rooms returns the ids of rooms with at least one open connection.
func (h *Hub) Rooms() []string {
	h.mu.RLock()
	defer h.mu.RUnlock()
	out := make([]string, 0, len(h.rooms))
	for id := range h.rooms {
		out = append(out, id)
	}
	return out
}

// Broadcast sends payload to every connection on a room. Encoding happens once
// for the whole room.
func (h *Hub) Broadcast(roomID string, payload any) {
	h.mu.RLock()
	n := len(h.rooms[roomID])
	h.mu.RUnlock()
	if n == 0 {
		return
	}

	data, err := json.Marshal(payload)
	if err != nil {
		h.log.Error("encoding broadcast payload", "room_id", roomID, "error", err)
		return
	}

	h.mu.RLock()
	clients := make([]*Client, 0, len(h.rooms[roomID]))
	for _, c := range h.rooms[roomID] {
		clients = append(clients, c)
	}
	h.mu.RUnlock()

	for _, c := range clients {
		c.enqueue(data)
	}
}

// SendJSON queues one payload to a single connection.
func (c *Client) SendJSON(payload any) {
	data, err := json.Marshal(payload)
	if err != nil {
		c.log.Error("encoding message", "client_id", c.ID, "error", err)
		return
	}
	c.enqueue(data)
}

// SendJSONAndClose writes one last message and then shuts the connection down.
//
// It waits for the message to reach the wire first: a rejection is the only
// thing the client will ever hear on this connection, and closing before the
// writer had a chance to send it would leave the browser with nothing but an
// abnormal close.
func (c *Client) SendJSONAndClose(payload any) {
	defer c.Close()

	data, err := json.Marshal(payload)
	if err != nil {
		c.log.Error("encoding final message", "client_id", c.ID, "error", err)
		return
	}

	done := make(chan struct{})
	select {
	case c.send <- frame{data: data, done: done, final: true}:
	case <-c.closed:
		return
	default:
		return
	}

	select {
	case <-done:
	case <-c.closed:
	case <-time.After(writeWait):
	}
}

// enqueue hands a frame to the writer goroutine. A client whose queue is full
// has stopped reading; it is closed rather than allowed to stall the room.
func (c *Client) enqueue(data []byte) {
	select {
	case <-c.closed:
	case c.send <- frame{data: data}:
	default:
		c.log.Warn("dropping slow websocket client", "client_id", c.ID, "room_id", c.RoomID)
		c.Close()
	}
}

// Close shuts the connection down. It is safe to call more than once and from
// any goroutine.
func (c *Client) Close() {
	c.closeOnce.Do(func() {
		close(c.closed)
		_ = c.conn.Close()
	})
}

// Closed returns a channel that is closed once the connection is shut down.
func (c *Client) Closed() <-chan struct{} { return c.closed }

// ReadMessage reads the next text frame from the client. It returns an error
// once the connection ends, which is the reader's signal to stop.
func (c *Client) ReadMessage() ([]byte, error) {
	_, data, err := c.conn.ReadMessage()
	return data, err
}

// ConfigureReader applies the read limits and the pong handler that keep the
// connection's liveness deadline moving. Call it once, before reading.
func (c *Client) ConfigureReader() error {
	c.conn.SetReadLimit(maxMessageBytes)
	if err := c.conn.SetReadDeadline(time.Now().Add(pongWait)); err != nil {
		return err
	}
	c.conn.SetPongHandler(func(string) error {
		return c.conn.SetReadDeadline(time.Now().Add(pongWait))
	})
	return nil
}

// write sends one frame and, if anybody is waiting on it, releases them
// whether the write succeeded or not.
func (c *Client) write(f frame) error {
	if f.done != nil {
		defer close(f.done)
	}
	if err := c.conn.SetWriteDeadline(time.Now().Add(writeWait)); err != nil {
		return err
	}
	if err := c.conn.WriteMessage(websocket.TextMessage, f.data); err != nil {
		return err
	}
	if f.final {
		_ = c.conn.WriteMessage(websocket.CloseMessage,
			websocket.FormatCloseMessage(websocket.CloseNormalClosure, ""))
	}
	return nil
}

// writePump owns the connection's write side: it is the only goroutine that
// writes, which is what gorilla/websocket requires.
func (c *Client) writePump() {
	ticker := time.NewTicker(pingPeriod)
	defer func() {
		ticker.Stop()
		c.Close()
	}()

	for {
		select {
		case <-c.closed:
			return

		case f := <-c.send:
			if err := c.write(f); err != nil {
				return
			}

		case <-ticker.C:
			if err := c.conn.SetWriteDeadline(time.Now().Add(writeWait)); err != nil {
				return
			}
			if err := c.conn.WriteMessage(websocket.PingMessage, nil); err != nil {
				return
			}
		}
	}
}
