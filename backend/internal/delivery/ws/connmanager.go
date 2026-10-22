package ws

import (
	"encoding/json"
	"log/slog"
	"sync"
	"time"

	"github.com/bebradio/backend-go/internal/domain/entity"
	"github.com/bebradio/backend-go/internal/infrastructure/redisc"
	"github.com/gorilla/websocket"
)

type connEntry struct {
	conn   *websocket.Conn
	mu     sync.Mutex
	userID string
}

type ConnectionManager struct {
	mu          sync.RWMutex
	connections map[entity.SessionRef]map[*websocket.Conn]*connEntry
	throttle    map[entity.SessionRef]*broadcastSlot
	log         *slog.Logger
}

const broadcastMinInterval = 250 * time.Millisecond

type broadcastSlot struct {
	lastSent   time.Time
	pending    any
	hasPending bool
	timer      *time.Timer
}

func NewConnectionManager(log *slog.Logger) *ConnectionManager {
	return &ConnectionManager{
		connections: make(map[entity.SessionRef]map[*websocket.Conn]*connEntry),
		throttle:    make(map[entity.SessionRef]*broadcastSlot),
		log:         log,
	}
}

func (cm *ConnectionManager) Connect(session any, conn *websocket.Conn) {
	ref := redisc.AsRef(session)
	cm.mu.Lock()
	defer cm.mu.Unlock()
	if cm.connections[ref] == nil {
		cm.connections[ref] = make(map[*websocket.Conn]*connEntry)
	}
	cm.connections[ref][conn] = &connEntry{conn: conn}
}

func (cm *ConnectionManager) BindUser(session any, conn *websocket.Conn, userID string) {
	ref := redisc.AsRef(session)
	cm.mu.Lock()
	defer cm.mu.Unlock()
	if entry, ok := cm.connections[ref][conn]; ok {
		entry.userID = userID
	}
}

func (cm *ConnectionManager) GetUserID(session any, conn *websocket.Conn) string {
	ref := redisc.AsRef(session)
	cm.mu.RLock()
	defer cm.mu.RUnlock()
	if entry, ok := cm.connections[ref][conn]; ok {
		return entry.userID
	}
	return ""
}

func (cm *ConnectionManager) Disconnect(session any, conn *websocket.Conn) {
	ref := redisc.AsRef(session)
	cm.mu.Lock()
	defer cm.mu.Unlock()
	if conns, ok := cm.connections[ref]; ok {
		delete(conns, conn)
		if len(conns) == 0 {
			delete(cm.connections, ref)
		}
	}
	if len(cm.connections[ref]) == 0 {
		if slot, ok := cm.throttle[ref]; ok {
			if slot.timer != nil {
				slot.timer.Stop()
			}
			delete(cm.throttle, ref)
		}
	}
}

func (cm *ConnectionManager) Broadcast(session any, data any) {
	ref := redisc.AsRef(session)
	if !isSnapshot(data) {
		cm.send(ref, data)
		return
	}

	now := time.Now()
	cm.mu.Lock()
	slot, ok := cm.throttle[ref]
	if !ok {
		slot = &broadcastSlot{}
		cm.throttle[ref] = slot
	}
	if !slot.armed() && now.Sub(slot.lastSent) >= broadcastMinInterval {
		slot.lastSent = now
		cm.mu.Unlock()
		cm.send(ref, data)
		return
	}
	slot.pending = data
	slot.hasPending = true
	if slot.timer == nil {
		delay := broadcastMinInterval - now.Sub(slot.lastSent)
		if delay < 0 {
			delay = 0
		}
		slot.timer = time.AfterFunc(delay, func() { cm.flushRoom(ref) })
	}
	cm.mu.Unlock()
}

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

func (cm *ConnectionManager) flushRoom(ref entity.SessionRef) {
	cm.mu.Lock()
	slot, ok := cm.throttle[ref]
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

	cm.send(ref, data)
}

func (cm *ConnectionManager) send(ref entity.SessionRef, data any) {
	cm.mu.RLock()
	entries := make([]*connEntry, 0, len(cm.connections[ref]))
	for _, entry := range cm.connections[ref] {
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
	if live, ok := cm.connections[ref]; ok {
		for _, conn := range dead {
			delete(live, conn)
		}
		if len(live) == 0 {
			delete(cm.connections, ref)
		}
	}
}

func (cm *ConnectionManager) GetCount(session any) int {
	ref := redisc.AsRef(session)
	cm.mu.RLock()
	defer cm.mu.RUnlock()
	return len(cm.connections[ref])
}

func (cm *ConnectionManager) SendJSON(session any, conn *websocket.Conn, data any) {
	ref := redisc.AsRef(session)
	cm.mu.RLock()
	entry, ok := cm.connections[ref][conn]
	cm.mu.RUnlock()
	if !ok {
		if err := conn.WriteJSON(data); err != nil {
			cm.log.Error("failed to send json", "session", ref.String(), "error", err)
		}
		return
	}
	entry.mu.Lock()
	defer entry.mu.Unlock()
	entry.conn.SetWriteDeadline(time.Now().Add(10 * time.Second))
	if err := entry.conn.WriteJSON(data); err != nil {
		cm.log.Error("failed to send json", "session", ref.String(), "error", err)
	}
}
