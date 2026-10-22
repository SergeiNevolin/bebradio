package http

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func createStream(t *testing.T, d *testDeps, token, name string) string {
	t.Helper()
	body, _ := json.Marshal(map[string]any{"name": name})
	req := httptest.NewRequest("POST", "/api/streams/", bytes.NewReader(body))
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
		t.Fatal("create stream: no stream id in response")
	}
	return id
}

func postStreamQueue(t *testing.T, d *testDeps, streamID, token string, payload map[string]string) *httptest.ResponseRecorder {
	t.Helper()
	body, _ := json.Marshal(payload)
	req := httptest.NewRequest("POST", "/api/streams/"+streamID+"/queue", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	w := httptest.NewRecorder()
	d.server.Router.ServeHTTP(w, req)
	return w
}

func TestCreateStreamRequiresAdmin(t *testing.T) {
	d := setupTestServer(t)
	token := registerUser(t, d, "user@test.com", "user", "pass123")

	body, _ := json.Marshal(map[string]any{"name": "Pirate Stream"})
	req := httptest.NewRequest("POST", "/api/streams/", bytes.NewReader(body))
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

	streamID := createStream(t, d, admin, "Official Stream")

	st, err := d.streamRepo.FindByID(streamID)
	if err != nil {
		t.Fatalf("stream not persisted: %v", err)
	}
	if st.Features.MashupRadio.Enabled {
		t.Error("expected Mashup Radio disabled by default so new streams start empty")
	}

	req := httptest.NewRequest("GET", "/api/streams/", nil)
	w := httptest.NewRecorder()
	d.server.Router.ServeHTTP(w, req)
	var streams []map[string]any
	json.Unmarshal(w.Body.Bytes(), &streams)
	found := false
	for _, s := range streams {
		if s["id"] == streamID {
			found = true
		}
	}
	if !found {
		t.Error("stream missing from public list")
	}

	// Rooms and streams are separate collections.
	req = httptest.NewRequest("GET", "/api/rooms/", nil)
	w = httptest.NewRecorder()
	d.server.Router.ServeHTTP(w, req)
	var rooms []map[string]any
	json.Unmarshal(w.Body.Bytes(), &rooms)
	for _, r := range rooms {
		if r["id"] == streamID {
			t.Error("stream leaked into room list")
		}
	}
}

func TestStreamQueueAdminOnly(t *testing.T) {
	d := setupTestServer(t)
	admin := registerAdmin(t, d, "admin@test.com", "admin", "pass123")
	user := registerUser(t, d, "user@test.com", "user", "pass123")

	streamID := createStream(t, d, admin, "Official Stream")

	if w := postStreamQueue(t, d, streamID, admin, map[string]string{"url": "https://youtu.be/x"}); w.Code != 200 {
		t.Errorf("admin add: expected 200, got %d: %s", w.Code, w.Body.String())
	}
	if w := postStreamQueue(t, d, streamID, user, map[string]string{"url": "https://youtu.be/y"}); w.Code != 403 {
		t.Errorf("user add: expected 403, got %d: %s", w.Code, w.Body.String())
	}
	if w := postStreamQueue(t, d, streamID, "", map[string]string{"url": "https://youtu.be/z"}); w.Code != 401 {
		t.Errorf("anonymous add: expected 401, got %d: %s", w.Code, w.Body.String())
	}

	req := httptest.NewRequest("GET", "/api/streams/"+streamID, nil)
	w := httptest.NewRecorder()
	d.server.Router.ServeHTTP(w, req)
	var stream map[string]any
	json.Unmarshal(w.Body.Bytes(), &stream)
	queue, _ := stream["queue"].([]any)
	if len(queue) != 1 {
		t.Errorf("expected exactly the admin track in queue, got %d", len(queue))
	}
	if stream["kind"] != "stream" {
		t.Errorf("expected kind stream, got %v", stream["kind"])
	}
}

func TestRegularRoomQueueUnaffected(t *testing.T) {
	d := setupTestServer(t)
	user := registerUser(t, d, "user@test.com", "user", "pass123")

	roomID := createRoomForQueue(t, d, user, "Plain Room")
	if w := postQueue(t, d, roomID, user, map[string]string{"url": "https://youtu.be/x"}); w.Code != http.StatusOK {
		t.Errorf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
}
