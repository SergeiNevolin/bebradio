package http

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
)

func registerAdmin(t *testing.T, d *testDeps, email, username, password string) string {
	t.Helper()
	token := registerUser(t, d, email, username, password)
	if err := d.userRepo.SetRole(userIDFromToken(token), "admin"); err != nil {
		t.Fatalf("promote to admin: %v", err)
	}
	return token
}

// seedYoutubeQueue enqueues one YouTube track and returns room + track ids.
func seedYoutubeQueue(t *testing.T, d *testDeps, token, roomName, url string) (string, string) {
	t.Helper()
	roomID := createRoomForQueue(t, d, token, roomName)
	qw := postQueue(t, d, roomID, token, map[string]string{"url": url})
	if qw.Code != 200 {
		t.Fatalf("seed queue: expected 200, got %d: %s", qw.Code, qw.Body.String())
	}
	var added map[string]any
	json.Unmarshal(qw.Body.Bytes(), &added)
	trackID, _ := added["id"].(string)
	if trackID == "" {
		t.Fatal("seed queue: no track id in response")
	}
	return roomID, trackID
}

func importQueueTrack(t *testing.T, d *testDeps, roomID, trackID, token string) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest("POST", "/api/rooms/"+roomID+"/queue/"+trackID+"/import", nil)
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	w := httptest.NewRecorder()
	d.server.Router.ServeHTTP(w, req)
	return w
}

// importMediaMocks wires the music-service doubles for a successful import:
// cached audio streams out, the re-upload mints lib ids, covers accepted.
func importMediaMocks(d *testDeps, uploaded *string, coverCalled *bool) {
	d.media.EnsureFn = func(items []map[string]any) ([]string, error) {
		var ids []string
		for _, item := range items {
			if id, ok := item["media_id"].(string); ok {
				ids = append(ids, id)
			}
		}
		return ids, nil
	}
	d.media.ContentFn = func(mediaID, rangeHeader string) (int64, string, []byte, error) {
		return 200, "audio/mp4", []byte("fake-m4a-bytes"), nil
	}
	d.media.UploadTrackFn = func(filename string, body io.Reader) (map[string]any, error) {
		data, _ := io.ReadAll(body)
		*uploaded = filename + ":" + string(data)
		return map[string]any{"id": "lib1", "media_id": "mm1", "status": "processing"}, nil
	}
	d.media.UploadTrackCoverFn = func(mediaID, filename string, body io.Reader) error {
		*coverCalled = true
		io.Copy(io.Discard, body)
		return nil
	}
}

func TestHandleImportQueueTrackSuccess(t *testing.T) {
	d := setupTestServer(t)
	admin := registerAdmin(t, d, "admin@test.com", "admin", "pass123")
	roomID, trackID := seedYoutubeQueue(t, d, admin, "Import Room", "https://youtu.be/x")

	var uploaded string
	var coverCalled bool
	importMediaMocks(d, &uploaded, &coverCalled)

	w := importQueueTrack(t, d, roomID, trackID, admin)
	if w.Code != 202 {
		t.Fatalf("expected 202, got %d: %s", w.Code, w.Body.String())
	}
	var resp map[string]any
	json.Unmarshal(w.Body.Bytes(), &resp)
	if resp["source"] != "upload" {
		t.Errorf("expected source 'upload', got %v", resp["source"])
	}
	if resp["id"] != "lib1" {
		t.Errorf("expected library id 'lib1', got %v", resp["id"])
	}
	if resp["status"] != "processing" {
		t.Errorf("expected processing, got %v", resp["status"])
	}
	if resp["owner_id"] != userIDFromToken(admin) {
		t.Errorf("expected owner to be the admin, got %v", resp["owner_id"])
	}
	if uploaded != "queue-t1.m4a:fake-m4a-bytes" {
		t.Errorf("expected cached audio re-uploaded, got %q", uploaded)
	}
	if coverCalled {
		t.Error("no thumbnail on the queued track: cover must be skipped, not fail")
	}
	if _, err := d.trackRepo.FindByID("lib1"); err != nil {
		t.Error("expected library row persisted")
	}
}

func TestHandleImportQueueTrackWithCover(t *testing.T) {
	d := setupTestServer(t)
	admin := registerAdmin(t, d, "admin@test.com", "admin", "pass123")

	thumb := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "image/jpeg")
		w.Write([]byte("fake-jpeg-bytes"))
	}))
	defer thumb.Close()
	d.media.ResolveFn = func(url string) (map[string]any, error) {
		return map[string]any{
			"id": "t9", "media_id": "m9", "title": "Clip", "artist": "Channel",
			"thumbnail": thumb.URL + "/thumb.jpg", "duration": float64(180),
		}, nil
	}

	roomID, trackID := seedYoutubeQueue(t, d, admin, "Cover Room", "https://youtu.be/y")

	var uploaded string
	var coverCalled bool
	var coverMedia string
	importMediaMocks(d, &uploaded, &coverCalled)
	d.media.UploadTrackCoverFn = func(mediaID, filename string, body io.Reader) error {
		coverCalled = true
		coverMedia = mediaID
		data, _ := io.ReadAll(body)
		if string(data) != "fake-jpeg-bytes" {
			t.Errorf("expected thumbnail bytes forwarded, got %q", data)
		}
		return nil
	}

	w := importQueueTrack(t, d, roomID, trackID, admin)
	if w.Code != 202 {
		t.Fatalf("expected 202, got %d: %s", w.Code, w.Body.String())
	}
	if !coverCalled {
		t.Error("expected YouTube thumbnail uploaded as cover")
	}
	if coverMedia != "mm1" {
		t.Errorf("expected cover on the new media id 'mm1', got %q", coverMedia)
	}
	if !d.trackRepo.CoverSet["lib1"] {
		t.Error("expected cover flag persisted on the library row")
	}
}

func TestHandleImportQueueTrackForbidden(t *testing.T) {
	d := setupTestServer(t)
	admin := registerAdmin(t, d, "admin@test.com", "admin", "pass123")
	user := registerUser(t, d, "user@test.com", "user", "pass123")
	roomID, trackID := seedYoutubeQueue(t, d, admin, "Import Room", "https://youtu.be/x")

	if w := importQueueTrack(t, d, roomID, trackID, user); w.Code != 403 {
		t.Errorf("non-admin: expected 403, got %d", w.Code)
	}
	if w := importQueueTrack(t, d, roomID, trackID, ""); w.Code != 401 {
		t.Errorf("anonymous: expected 401, got %d", w.Code)
	}
}

func TestHandleImportQueueTrackNotInQueue(t *testing.T) {
	d := setupTestServer(t)
	admin := registerAdmin(t, d, "admin@test.com", "admin", "pass123")
	roomID := createRoomForQueue(t, d, admin, "Empty Room")

	if w := importQueueTrack(t, d, roomID, "ghost", admin); w.Code != 404 {
		t.Errorf("expected 404, got %d", w.Code)
	}
}

func TestHandleImportQueueTrackAlreadyLibrary(t *testing.T) {
	d := setupTestServer(t)
	admin := registerAdmin(t, d, "admin@test.com", "admin", "pass123")
	d.trackRepo.Create(readyUpload("lib1", userIDFromToken(admin), "media111"))
	roomID := createRoomForQueue(t, d, admin, "Lib Room")

	qw := postQueue(t, d, roomID, admin, map[string]string{"track_id": "lib1"})
	if qw.Code != 200 {
		t.Fatalf("seed library track: expected 200, got %d", qw.Code)
	}
	if w := importQueueTrack(t, d, roomID, "lib1", admin); w.Code != 400 {
		t.Errorf("library track: expected 400, got %d: %s", w.Code, w.Body.String())
	}
}

func TestHandleImportQueueTrackMediaDown(t *testing.T) {
	d := setupTestServer(t)
	admin := registerAdmin(t, d, "admin@test.com", "admin", "pass123")
	roomID, trackID := seedYoutubeQueue(t, d, admin, "Down Room", "https://youtu.be/x")

	// Ensure reports nothing ready: audio can't be fetched.
	d.media.EnsureFn = func(items []map[string]any) ([]string, error) {
		return nil, nil
	}
	if w := importQueueTrack(t, d, roomID, trackID, admin); w.Code != 502 {
		t.Errorf("expected 502, got %d", w.Code)
	}
	if n, _ := d.trackRepo.CountByOwner(userIDFromToken(admin)); n != 0 {
		t.Errorf("expected no library row on media failure, got %d", n)
	}
}

func TestHandleImportQueueTrackRoomNotFound(t *testing.T) {
	d := setupTestServer(t)
	admin := registerAdmin(t, d, "admin@test.com", "admin", "pass123")

	if w := importQueueTrack(t, d, "NOPE", "t1", admin); w.Code != 404 {
		t.Errorf("expected 404, got %d", w.Code)
	}
}
