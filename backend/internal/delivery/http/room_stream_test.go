package http

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

// createStream creates an admin-run stream room via the API.
func createStream(t *testing.T, d *testDeps, token, name string) string {
	t.Helper()
	body, _ := json.Marshal(map[string]any{"name": name, "is_stream": true})
	req := httptest.NewRequest("POST", "/api/rooms/", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+token)
	w := httptest.NewRecorder()
	d.server.Router.ServeHTTP(w, req)
	if w.Code != 200 {
		t.Fatalf("create stream: expected 200, got %d: %s", w.Code, w.Body.String())
	}
	var resp map[string]any
	json.Unmarshal(w.Body.Bytes(), &resp)
	id, _ := resp["id"].(string)
	if id == "" {
		t.Fatal("create stream: no room id in response")
	}
	return id
}

func TestCreateStreamRequiresAdmin(t *testing.T) {
	d := setupTestServer(t)
	token := registerUser(t, d, "user@test.com", "user", "pass123")

	body, _ := json.Marshal(map[string]any{"name": "Pirate Stream", "is_stream": true})
	req := httptest.NewRequest("POST", "/api/rooms/", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+token)
	w := httptest.NewRecorder()
	d.server.Router.ServeHTTP(w, req)

	if w.Code != 403 {
		t.Errorf("expected 403, got %d: %s", w.Code, w.Body.String())
	}
}

func TestCreateStreamAsAdmin(t *testing.T) {
	d := setupTestServer(t)
	admin := registerAdmin(t, d, "admin@test.com", "admin", "pass123")

	roomID := createStream(t, d, admin, "Official Stream")

	rm, err := d.roomRepo.FindByID(roomID)
	if err != nil {
		t.Fatalf("stream room not persisted: %v", err)
	}
	if !rm.IsStream {
		t.Error("expected IsStream flag persisted")
	}

	// The flag is listed and served back to clients.
	req := httptest.NewRequest("GET", "/api/rooms/", nil)
	w := httptest.NewRecorder()
	d.server.Router.ServeHTTP(w, req)
	var rooms []map[string]any
	json.Unmarshal(w.Body.Bytes(), &rooms)
	found := false
	for _, r := range rooms {
		if r["id"] == roomID {
			found = true
			if r["is_stream"] != true {
				t.Errorf("expected is_stream in room list, got %v", r["is_stream"])
			}
		}
	}
	if !found {
		t.Error("stream room missing from public list")
	}
}

func TestStreamQueueAdminOnly(t *testing.T) {
	d := setupTestServer(t)
	admin := registerAdmin(t, d, "admin@test.com", "admin", "pass123")
	user := registerUser(t, d, "user@test.com", "user", "pass123")

	roomID := createStream(t, d, admin, "Official Stream")

	// Admin feeds the stream.
	if w := postQueue(t, d, roomID, admin, map[string]string{"url": "https://youtu.be/x"}); w.Code != 200 {
		t.Errorf("admin add: expected 200, got %d: %s", w.Code, w.Body.String())
	}

	// Regular users are refused, even authenticated ones.
	if w := postQueue(t, d, roomID, user, map[string]string{"url": "https://youtu.be/y"}); w.Code != 403 {
		t.Errorf("user add: expected 403, got %d: %s", w.Code, w.Body.String())
	}

	// Anonymous listeners are refused as well.
	if w := postQueue(t, d, roomID, "", map[string]string{"url": "https://youtu.be/z"}); w.Code != 403 {
		t.Errorf("anonymous add: expected 403, got %d: %s", w.Code, w.Body.String())
	}

	// Only the admin's track landed in the queue.
	req := httptest.NewRequest("GET", "/api/rooms/"+roomID, nil)
	req.Header.Set("Authorization", "Bearer "+admin)
	w := httptest.NewRecorder()
	d.server.Router.ServeHTTP(w, req)
	var room map[string]any
	json.Unmarshal(w.Body.Bytes(), &room)
	queue, _ := room["queue"].([]any)
	if len(queue) != 1 {
		t.Errorf("expected exactly the admin track in queue, got %d", len(queue))
	}
	if room["is_stream"] != true {
		t.Errorf("expected is_stream in room state, got %v", room["is_stream"])
	}
}

func TestRegularRoomQueueUnaffected(t *testing.T) {
	d := setupTestServer(t)
	user := registerUser(t, d, "user@test.com", "user", "pass123")

	// Plain rooms keep the old behavior: anyone with access adds tracks.
	roomID := createRoomForQueue(t, d, user, "Plain Room")
	if w := postQueue(t, d, roomID, user, map[string]string{"url": "https://youtu.be/x"}); w.Code != http.StatusOK {
		t.Errorf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
}
