package ws

import (
	"encoding/json"
	"log/slog"
	"sync"

	"github.com/gorilla/websocket"
)

type ConnectionManager struct {
	mu          sync.RWMutex
	connections map[string]map[*websocket.Conn]bool
	log         *slog.Logger
}

func NewConnectionManager(log *slog.Logger) *ConnectionManager {
	return &ConnectionManager{
		connections: make(map[string]map[*websocket.Conn]bool),
		log:         log,
	}
}

func (cm *ConnectionManager) Connect(roomID string, conn *websocket.Conn) {
	cm.mu.Lock()
	defer cm.mu.Unlock()
	if cm.connections[roomID] == nil {
		cm.connections[roomID] = make(map[*websocket.Conn]bool)
	}
	cm.connections[roomID][conn] = true
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
	cm.mu.Lock()
	defer cm.mu.Unlock()

	conns := cm.connections[roomID]
	if len(conns) == 0 {
		return
	}

	jsonData, err := json.Marshal(data)
	if err != nil {
		cm.log.Error("failed to marshal broadcast data", "error", err)
		return
	}

	var dead []*websocket.Conn
	for conn := range conns {
		if err := conn.WriteMessage(websocket.TextMessage, jsonData); err != nil {
			dead = append(dead, conn)
		}
	}

	for _, conn := range dead {
		delete(conns, conn)
	}
	if len(conns) == 0 {
		delete(cm.connections, roomID)
	}
}

func (cm *ConnectionManager) GetCount(roomID string) int {
	cm.mu.RLock()
	defer cm.mu.RUnlock()
	return len(cm.connections[roomID])
}

func (cm *ConnectionManager) SendJSON(roomID string, conn *websocket.Conn, data any) {
	if err := conn.WriteJSON(data); err != nil {
		cm.log.Error("failed to send json", "room_id", roomID, "error", err)
	}
}
