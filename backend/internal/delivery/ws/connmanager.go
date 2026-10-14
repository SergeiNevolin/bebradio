package ws

import (
	"encoding/json"
	"log/slog"
	"sync"
	"time"

	"github.com/gorilla/websocket"
)

// connEntry serializes concurrent writers to one connection: gorilla allows a
// single concurrent writer, while Broadcast and SendJSON run on arbitrary
// goroutines (HTTP handlers, WS loop, autoadvance ticker).
type connEntry struct {
	conn   *websocket.Conn
	mu     sync.Mutex
	userID string
}

type ConnectionManager struct {
	mu          sync.RWMutex
	connections map[string]map[*websocket.Conn]*connEntry
	// throttle collapses full-state snapshot storms per room (chatty rooms
	// otherwise serialize and push the whole queue on every vote, message
	// and reaction). See Broadcast.
	throttle map[string]*broadcastSlot
	log      *slog.Logger
}

// broadcastMinInterval caps full-room snapshot traffic: at most one
// snapshot per room per interval goes out immediately, anything arriving
// sooner collapses into a single latest-wins flush.
const broadcastMinInterval = 250 * time.Millisecond

type broadcastSlot struct {
	lastSent   time.Time
	pending    any
	hasPending bool
	timer      *time.Timer
}

func NewConnectionManager(log *slog.Logger) *ConnectionManager {
	return &ConnectionManager{
		connections: make(map[string]map[*websocket.Conn]*connEntry),
		throttle:    make(map[string]*broadcastSlot),
		log:         log,
	}
}

func (cm *ConnectionManager) Connect(roomID string, conn *websocket.Conn) {
	cm.mu.Lock()
	defer cm.mu.Unlock()
	if cm.connections[roomID] == nil {
		cm.connections[roomID] = make(map[*websocket.Conn]*connEntry)
	}
	cm.connections[roomID][conn] = &connEntry{conn: conn}
}

// BindUser records the authenticated user_id for a connection.
func (cm *ConnectionManager) BindUser(roomID string, conn *websocket.Conn, userID string) {
	cm.mu.Lock()
	defer cm.mu.Unlock()
	if entry, ok := cm.connections[roomID][conn]; ok {
		entry.userID = userID
	}
}

// GetUserID returns the user_id bound during hello, or an empty string.
func (cm *ConnectionManager) GetUserID(roomID string, conn *websocket.Conn) string {
	cm.mu.RLock()
	defer cm.mu.RUnlock()
	if entry, ok := cm.connections[roomID][conn]; ok {
		return entry.userID
	}
	return ""
}

func (cm *ConnectionManager) Disconnect(roomID string, conn *websocket.Conn) {
	cm.mu.Lock()
	defer cm.mu.Unlock()
	if conns, ok := cm.connections[roomID]; ok {
		delete(conns, conn)
		if len(conns) == 0 {
			delete(cm.connections, roomID)
		}
	}
	// No listeners left: drop any coalesced snapshot and its timer so a
	// dead room holds no goroutines or payloads.
	if len(cm.connections[roomID]) == 0 {
		if slot, ok := cm.throttle[roomID]; ok {
			if slot.timer != nil {
				slot.timer.Stop()
			}
			delete(cm.throttle, roomID)
		}
	}
}

func (cm *ConnectionManager) Broadcast(roomID string, data any) {
	// Targeted messages (chat, reactions) carry a "type" key and are small:
	// they always go out immediately. Full room snapshots have no "type"
	// key and collapse latest-wins under the throttle.
	if !isSnapshot(data) {
		cm.send(roomID, data)
		return
	}

	now := time.Now()
	cm.mu.Lock()
	slot, ok := cm.throttle[roomID]
	if !ok {
		slot = &broadcastSlot{}
		cm.throttle[roomID] = slot
	}
	if !slot.armed() && now.Sub(slot.lastSent) >= broadcastMinInterval {
		slot.lastSent = now
		cm.mu.Unlock()
		cm.send(roomID, data)
		return
	}
	slot.pending = data
	slot.hasPending = true
	if slot.timer == nil {
		delay := broadcastMinInterval - now.Sub(slot.lastSent)
		if delay < 0 {
			delay = 0
		}
		slot.timer = time.AfterFunc(delay, func() { cm.flushRoom(roomID) })
	}
	cm.mu.Unlock()
}

// isSnapshot reports whether data is a full room-state dump (as opposed to
// a small targeted event like {"type": "chat"/"reaction", ...}).
func isSnapshot(data any) bool {
	m, ok := data.(map[string]any)
	if !ok {
		return true
	}
	_, targeted := m["type"]
	return !targeted
}

func (slot *broadcastSlot) armed() bool {
	return slot.timer != nil
}

// flushRoom delivers the latest coalesced snapshot, if any.
func (cm *ConnectionManager) flushRoom(roomID string) {
	cm.mu.Lock()
	slot, ok := cm.throttle[roomID]
	if !ok || !slot.hasPending {
		if ok {
			slot.timer = nil
		}
		cm.mu.Unlock()
		return
	}
	data := slot.pending
	slot.pending = nil
	slot.hasPending = false
	slot.timer = nil
	slot.lastSent = time.Now()
	cm.mu.Unlock()

	cm.send(roomID, data)
}

func (cm *ConnectionManager) send(roomID string, data any) {
	// Snapshot under a read lock; slow writes must not block other rooms.
	cm.mu.RLock()
	entries := make([]*connEntry, 0, len(cm.connections[roomID]))
	for _, entry := range cm.connections[roomID] {
		entries = append(entries, entry)
	}
	cm.mu.RUnlock()

	if len(entries) == 0 {
		return
	}

	jsonData, err := json.Marshal(data)
	if err != nil {
		cm.log.Error("failed to marshal broadcast data", "error", err)
		return
	}

	var dead []*websocket.Conn
	for _, entry := range entries {
		entry.mu.Lock()
		entry.conn.SetWriteDeadline(time.Now().Add(10 * time.Second))
		err := entry.conn.WriteMessage(websocket.TextMessage, jsonData)
		entry.mu.Unlock()
		if err != nil {
			dead = append(dead, entry.conn)
		}
	}

	if len(dead) == 0 {
		return
	}
	cm.mu.Lock()
	defer cm.mu.Unlock()
	if live, ok := cm.connections[roomID]; ok {
		for _, conn := range dead {
			delete(live, conn)
		}
		if len(live) == 0 {
			delete(cm.connections, roomID)
		}
	}
}

func (cm *ConnectionManager) GetCount(roomID string) int {
	cm.mu.RLock()
	defer cm.mu.RUnlock()
	return len(cm.connections[roomID])
}

func (cm *ConnectionManager) SendJSON(roomID string, conn *websocket.Conn, data any) {
	cm.mu.RLock()
	entry, ok := cm.connections[roomID][conn]
	cm.mu.RUnlock()
	if !ok {
		// Already disconnected: best-effort direct write, as before.
		if err := conn.WriteJSON(data); err != nil {
			cm.log.Error("failed to send json", "room_id", roomID, "error", err)
		}
		return
	}
	entry.mu.Lock()
	defer entry.mu.Unlock()
	entry.conn.SetWriteDeadline(time.Now().Add(10 * time.Second))
	if err := entry.conn.WriteJSON(data); err != nil {
		cm.log.Error("failed to send json", "room_id", roomID, "error", err)
	}
}
