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
	conn *websocket.Conn
	mu   sync.Mutex
}

type ConnectionManager struct {
	mu          sync.RWMutex
	connections map[string]map[*websocket.Conn]*connEntry
	log         *slog.Logger
}

func NewConnectionManager(log *slog.Logger) *ConnectionManager {
	return &ConnectionManager{
		connections: make(map[string]map[*websocket.Conn]*connEntry),
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

func (cm *ConnectionManager) Disconnect(roomID string, conn *websocket.Conn) {
	cm.mu.Lock()
	defer cm.mu.Unlock()
	if conns, ok := cm.connections[roomID]; ok {
		delete(conns, conn)
		if len(conns) == 0 {
			delete(cm.connections, roomID)
		}
	}
}

func (cm *ConnectionManager) Broadcast(roomID string, data any) {
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
