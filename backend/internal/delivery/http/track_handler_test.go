package http

import (
	"bytes"
	"encoding/json"
	"io"
	"mime/multipart"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/bebradio/backend-go/internal/domain/entity"
)

// userIDFromToken mirrors the mock auth bridge: the token is "token_<id>".
func userIDFromToken(token string) string { return strings.TrimPrefix(token, "token_") }

func multipartBody(t *testing.T, fields map[string]string, fileField, fileName, fileData string) (*bytes.Buffer, string) {
	t.Helper()
	buf := &bytes.Buffer{}
	mw := multipart.NewWriter(buf)
	for k, v := range fields {
		_ = mw.WriteField(k, v)
	}
	if fileField != "" {
		fw, _ := mw.CreateFormFile(fileField, fileName)
		io.WriteString(fw, fileData)
	}
	mw.Close()
	return buf, mw.FormDataContentType()
}

func readyUpload(id, ownerID, mediaID string) *entity.Track {
	return &entity.Track{
		ID: id, Source: entity.TrackSourceUpload, OwnerID: ownerID,
		Title: "T", Status: entity.TrackStatusReady, MediaID: mediaID,
	}
}

func TestHandleUploadTrackRequiresAuth(t *testing.T) {
	d := setupTestServer(t)
	body, ct := multipartBody(t, nil, "file", "a.mp3", "xxxx")

	req := httptest.NewRequest("POST", "/api/tracks/", body)
	req.Header.Set("Content-Type", ct)
	w := httptest.NewRecorder()
	d.server.Router.ServeHTTP(w, req)

	if w.Code != 401 {
		t.Errorf("expected 401, got %d", w.Code)
	}
}

func TestHandleUploadTrackSuccess(t *testing.T) {
	d := setupTestServer(t)
	token := registerUser(t, d, "u@test.com", "uploader", "pass123")
	d.media.UploadTrackFn = func(filename string, body io.Reader) (map[string]any, error) {
		io.Copy(io.Discard, body)
		return map[string]any{"id": "t1", "media_id": "m1", "status": "processing"}, nil
	}

	body, ct := multipartBody(t, map[string]string{"title": "Night Mix", "artist": "Me"}, "file", "mix.mp3", "audio-bytes")
	req := httptest.NewRequest("POST", "/api/tracks/", body)
	req.Header.Set("Content-Type", ct)
	req.Header.Set("Authorization", "Bearer "+token)
	w := httptest.NewRecorder()
	d.server.Router.ServeHTTP(w, req)

	if w.Code != 202 {
		t.Fatalf("expected 202, got %d: %s", w.Code, w.Body.String())
	}
	var resp map[string]any
	json.Unmarshal(w.Body.Bytes(), &resp)
	if resp["status"] != "processing" {
		t.Errorf("expected processing, got %v", resp["status"])
	}
	if resp["title"] != "Night Mix" {
		t.Errorf("expected title 'Night Mix', got %v", resp["title"])
	}
	if resp["source"] != "upload" {
		t.Errorf("expected source 'upload', got %v", resp["source"])
	}
	if _, leaked := resp["media_id"]; leaked {
		t.Error("media_id must not be exposed")
	}
}

func TestHandleListTracks(t *testing.T) {
	d := setupTestServer(t)
	d.trackRepo.Create(readyUpload("m1", "o1", "aaa"))
	d.trackRepo.Create(readyUpload("m2", "o1", "bbb"))

	req := httptest.NewRequest("GET", "/api/tracks/", nil)
	w := httptest.NewRecorder()
	d.server.Router.ServeHTTP(w, req)

	if w.Code != 200 {
		t.Fatalf("expected 200, got %d", w.Code)
	}
	var resp []map[string]any
	json.Unmarshal(w.Body.Bytes(), &resp)
	if len(resp) != 2 {
		t.Errorf("expected 2 tracks, got %d", len(resp))
	}
	if resp[0]["url"] == "" {
		t.Error("ready upload should carry url")
	}
}

func TestHandleDeleteTrackForbiddenForNonOwner(t *testing.T) {
	d := setupTestServer(t)
	owner := registerUser(t, d, "owner@test.com", "owner", "pass123")
	other := registerUser(t, d, "other@test.com", "other", "pass123")
	_ = owner

	// resolve owner's real id via /api/auth/me is overkill; store row with a known owner id
	d.trackRepo.Create(readyUpload("m1", "not-the-other-user", "abc"))

	req := httptest.NewRequest("DELETE", "/api/tracks/m1", nil)
	req.Header.Set("Authorization", "Bearer "+other)
	w := httptest.NewRecorder()
	d.server.Router.ServeHTTP(w, req)

	if w.Code != 403 {
		t.Errorf("expected 403, got %d: %s", w.Code, w.Body.String())
	}
}

func TestHandleDeleteTrackNotFound(t *testing.T) {
	d := setupTestServer(t)
	token := registerUser(t, d, "u@test.com", "user", "pass123")

	req := httptest.NewRequest("DELETE", "/api/tracks/ghost", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	w := httptest.NewRecorder()
	d.server.Router.ServeHTTP(w, req)

	if w.Code != 404 {
		t.Errorf("expected 404, got %d", w.Code)
	}
}

func TestHandleMyTracksRequiresAuth(t *testing.T) {
	d := setupTestServer(t)
	req := httptest.NewRequest("GET", "/api/tracks/mine", nil)
	w := httptest.NewRecorder()
	d.server.Router.ServeHTTP(w, req)

	if w.Code != 401 {
		t.Errorf("expected 401, got %d", w.Code)
	}
}

func TestHandleLikeAndUnlikeTrack(t *testing.T) {
	d := setupTestServer(t)
	token := registerUser(t, d, "liker@test.com", "liker", "pass123")
	d.trackRepo.Create(readyUpload("m1", "someone", "abc"))

	req := httptest.NewRequest("POST", "/api/tracks/m1/like", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	w := httptest.NewRecorder()
	d.server.Router.ServeHTTP(w, req)
	if w.Code != 200 {
		t.Fatalf("like: expected 200, got %d: %s", w.Code, w.Body.String())
	}
	var res map[string]any
	json.Unmarshal(w.Body.Bytes(), &res)
	if res["likes"] != float64(1) || res["liked"] != true {
		t.Fatalf("unexpected like body: %v", res)
	}

	req = httptest.NewRequest("DELETE", "/api/tracks/m1/like", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	w = httptest.NewRecorder()
	d.server.Router.ServeHTTP(w, req)
	if w.Code != 200 {
		t.Fatalf("unlike: expected 200, got %d: %s", w.Code, w.Body.String())
	}
	json.Unmarshal(w.Body.Bytes(), &res)
	if res["likes"] != float64(0) || res["liked"] != false {
		t.Fatalf("unexpected unlike body: %v", res)
	}
}

func TestHandleLikeTrackRequiresAuth(t *testing.T) {
	d := setupTestServer(t)
	d.trackRepo.Create(readyUpload("m1", "someone", "abc"))

	req := httptest.NewRequest("POST", "/api/tracks/m1/like", nil)
	w := httptest.NewRecorder()
	d.server.Router.ServeHTTP(w, req)
	if w.Code != 401 {
		t.Errorf("expected 401, got %d", w.Code)
	}
}

func TestHandleLikedTracks(t *testing.T) {
	d := setupTestServer(t)
	token := registerUser(t, d, "liker@test.com", "liker", "pass123")
	uid := userIDFromToken(token)
	d.trackRepo.Create(readyUpload("m1", "o", "abc"))
	d.trackRepo.Create(readyUpload("m2", "o", "def"))
	if _, err := d.trackRepo.Like("m2", uid); err != nil {
		t.Fatal(err)
	}

	req := httptest.NewRequest("GET", "/api/tracks/liked", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	w := httptest.NewRecorder()
	d.server.Router.ServeHTTP(w, req)
	if w.Code != 200 {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
	var resp []map[string]any
	json.Unmarshal(w.Body.Bytes(), &resp)
	if len(resp) != 1 || resp[0]["id"] != "m2" {
		t.Errorf("expected only m2, got %v", resp)
	}
}

func TestHandleUploadTrackCoverOwnerOnly(t *testing.T) {
	d := setupTestServer(t)
	token := registerUser(t, d, "owner@test.com", "owner", "pass123")
	other := registerUser(t, d, "other@test.com", "other", "pass123")
	d.trackRepo.Create(readyUpload("m1", userIDFromToken(token), "abc"))

	var coverMedia string
	d.media.UploadTrackCoverFn = func(mediaID, filename string, body io.Reader) error {
		io.Copy(io.Discard, body)
		coverMedia = mediaID
		return nil
	}

	// Non-owner -> 403.
	body, ct := multipartBody(t, nil, "file", "cover.jpg", "img-bytes")
	req := httptest.NewRequest("PUT", "/api/tracks/m1/cover", body)
	req.Header.Set("Content-Type", ct)
	req.Header.Set("Authorization", "Bearer "+other)
	w := httptest.NewRecorder()
	d.server.Router.ServeHTTP(w, req)
	if w.Code != 403 {
		t.Fatalf("non-owner: expected 403, got %d: %s", w.Code, w.Body.String())
	}

	// Owner -> 200 and media client called.
	body, ct = multipartBody(t, nil, "file", "cover.jpg", "img-bytes")
	req = httptest.NewRequest("PUT", "/api/tracks/m1/cover", body)
	req.Header.Set("Content-Type", ct)
	req.Header.Set("Authorization", "Bearer "+token)
	w = httptest.NewRecorder()
	d.server.Router.ServeHTTP(w, req)
	if w.Code != 200 {
		t.Fatalf("owner: expected 200, got %d: %s", w.Code, w.Body.String())
	}
	if coverMedia != "abc" {
		t.Errorf("expected media client called with 'abc', got %q", coverMedia)
	}
	if !d.trackRepo.CoverSet["m1"] {
		t.Error("expected cover flag persisted")
	}
}

func TestHandleUploadTrackWithCover(t *testing.T) {
	d := setupTestServer(t)
	token := registerUser(t, d, "u@test.com", "uploader", "pass123")
	d.media.UploadTrackFn = func(filename string, body io.Reader) (map[string]any, error) {
		io.Copy(io.Discard, body)
		return map[string]any{"id": "t9", "media_id": "m9", "status": "processing"}, nil
	}
	coverCalled := false
	d.media.UploadTrackCoverFn = func(mediaID, filename string, body io.Reader) error {
		io.Copy(io.Discard, body)
		coverCalled = true
		return nil
	}

	buf := &bytes.Buffer{}
	mw := multipart.NewWriter(buf)
	_ = mw.WriteField("title", "With Art")
	cw, _ := mw.CreateFormFile("cover", "art.jpg")
	io.WriteString(cw, "image-bytes")
	fw, _ := mw.CreateFormFile("file", "mix.mp3")
	io.WriteString(fw, "audio-bytes")
	mw.Close()

	req := httptest.NewRequest("POST", "/api/tracks/", buf)
	req.Header.Set("Content-Type", mw.FormDataContentType())
	req.Header.Set("Authorization", "Bearer "+token)
	w := httptest.NewRecorder()
	d.server.Router.ServeHTTP(w, req)

	if w.Code != 202 {
		t.Fatalf("expected 202, got %d: %s", w.Code, w.Body.String())
	}
	if !coverCalled {
		t.Error("expected the cover to be forwarded to music-service")
	}
}

func TestHandleGetTrack(t *testing.T) {
	d := setupTestServer(t)
	d.trackRepo.Create(&entity.Track{
		ID: "m9", Source: entity.TrackSourceUpload, OwnerID: "o1",
		Title: "Solo", Status: "processing", MediaID: "zzz",
	})

	req := httptest.NewRequest("GET", "/api/tracks/m9", nil)
	w := httptest.NewRecorder()
	d.server.Router.ServeHTTP(w, req)

	if w.Code != 200 {
		t.Fatalf("expected 200, got %d", w.Code)
	}
	var resp map[string]any
	json.Unmarshal(w.Body.Bytes(), &resp)
	if resp["id"] != "m9" || resp["status"] != "processing" {
		t.Errorf("unexpected body: %v", resp)
	}
	if resp["url"] != "" {
		t.Error("processing track should not have url yet")
	}
}

func TestHandleStreamTrackNotReady(t *testing.T) {
	d := setupTestServer(t)
	d.trackRepo.Create(&entity.Track{
		ID: "m9", Source: entity.TrackSourceUpload, OwnerID: "o1",
		Title: "Solo", Status: "processing", MediaID: "zzz",
	})

	req := httptest.NewRequest("GET", "/api/tracks/m9/audio", nil)
	w := httptest.NewRecorder()
	d.server.Router.ServeHTTP(w, req)

	if w.Code != 404 {
		t.Errorf("expected 404 for processing track audio, got %d", w.Code)
	}
}

func TestHandleStreamTrackMissing(t *testing.T) {
	d := setupTestServer(t)

	req := httptest.NewRequest("GET", "/api/tracks/ghost/audio", nil)
	w := httptest.NewRecorder()
	d.server.Router.ServeHTTP(w, req)

	if w.Code != 404 {
		t.Errorf("expected 404, got %d", w.Code)
	}
}

func TestHandleAddToQueueWithTrackID(t *testing.T) {
	d := setupTestServer(t)
	token := registerUser(t, d, "u@test.com", "user", "pass123")
	d.trackRepo.Create(readyUpload("lib1", userIDFromToken(token), "media111"))

	// Create a room.
	body, _ := json.Marshal(map[string]string{"name": "Queue Room"})
	req := httptest.NewRequest("POST", "/api/rooms/", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+token)
	w := httptest.NewRecorder()
	d.server.Router.ServeHTTP(w, req)
	var createResp map[string]any
	json.Unmarshal(w.Body.Bytes(), &createResp)
	roomID := createResp["id"].(string)

	// Enqueue the library track by id (no YouTube URL involved).
	queueBody, _ := json.Marshal(map[string]string{"track_id": "lib1"})
	qr := httptest.NewRequest("POST", "/api/rooms/"+roomID+"/queue", bytes.NewReader(queueBody))
	qr.Header.Set("Content-Type", "application/json")
	qr.Header.Set("Authorization", "Bearer "+token)
	qw := httptest.NewRecorder()
	d.server.Router.ServeHTTP(qw, qr)
	if qw.Code != 200 {
		t.Fatalf("queue add by id: expected 200, got %d: %s", qw.Code, qw.Body.String())
	}
	var added map[string]any
	json.Unmarshal(qw.Body.Bytes(), &added)
	if added["source"] != "upload" {
		t.Errorf("expected source 'upload', got %v", added["source"])
	}
	if added["url"] != "/api/tracks/lib1/audio" {
		t.Errorf("expected library audio url, got %v", added["url"])
	}
	if added["id"] != "lib1" {
		t.Errorf("queue row reuses the library id, got %v", added["id"])
	}

	// Re-adding is idempotent: same entry, no duplicate.
	qr2 := httptest.NewRequest("POST", "/api/rooms/"+roomID+"/queue", bytes.NewReader(queueBody))
	qr2.Header.Set("Content-Type", "application/json")
	qr2.Header.Set("Authorization", "Bearer "+token)
	qw2 := httptest.NewRecorder()
	d.server.Router.ServeHTTP(qw2, qr2)
	if qw2.Code != 200 {
		t.Fatalf("re-add: expected 200, got %d: %s", qw2.Code, qw2.Body.String())
	}
	var addedAgain map[string]any
	json.Unmarshal(qw2.Body.Bytes(), &addedAgain)
	if addedAgain["id"] != "lib1" {
		t.Errorf("re-add must return the queued entry, got %v", addedAgain["id"])
	}

	// Unknown id -> 400.
	badBody, _ := json.Marshal(map[string]string{"track_id": "ghost"})
	br := httptest.NewRequest("POST", "/api/rooms/"+roomID+"/queue", bytes.NewReader(badBody))
	br.Header.Set("Content-Type", "application/json")
	br.Header.Set("Authorization", "Bearer "+token)
	bw := httptest.NewRecorder()
	d.server.Router.ServeHTTP(bw, br)
	if bw.Code != 400 {
		t.Errorf("unknown track_id: expected 400, got %d", bw.Code)
	}
}
