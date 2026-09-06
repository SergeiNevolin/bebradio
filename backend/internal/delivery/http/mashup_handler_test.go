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

func TestHandleUploadMashupRequiresAuth(t *testing.T) {
	d := setupTestServer(t)
	body, ct := multipartBody(t, nil, "file", "a.mp3", "xxxx")

	req := httptest.NewRequest("POST", "/api/mashups/", body)
	req.Header.Set("Content-Type", ct)
	w := httptest.NewRecorder()
	d.server.Router.ServeHTTP(w, req)

	if w.Code != 401 {
		t.Errorf("expected 401, got %d", w.Code)
	}
}

func TestHandleUploadMashupSuccess(t *testing.T) {
	d := setupTestServer(t)
	token := registerUser(t, d, "u@test.com", "uploader", "pass123")
	d.media.UploadMashupFn = func(mediaID, filename string, body io.Reader) error {
		io.Copy(io.Discard, body)
		return nil
	}

	body, ct := multipartBody(t, map[string]string{"title": "Night Mix", "artist": "Me"}, "file", "mix.mp3", "audio-bytes")
	req := httptest.NewRequest("POST", "/api/mashups/", body)
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
	if _, leaked := resp["media_id"]; leaked {
		t.Error("media_id must not be exposed")
	}
}

func TestHandleListMashups(t *testing.T) {
	d := setupTestServer(t)
	d.mashupRepo.Create(&entity.Mashup{ID: "m1", OwnerID: "o1", Title: "Alpha", Status: "ready", MediaID: "aaa"})
	d.mashupRepo.Create(&entity.Mashup{ID: "m2", OwnerID: "o1", Title: "Beta", Status: "ready", MediaID: "bbb"})

	req := httptest.NewRequest("GET", "/api/mashups/", nil)
	w := httptest.NewRecorder()
	d.server.Router.ServeHTTP(w, req)

	if w.Code != 200 {
		t.Fatalf("expected 200, got %d", w.Code)
	}
	var resp []map[string]any
	json.Unmarshal(w.Body.Bytes(), &resp)
	if len(resp) != 2 {
		t.Errorf("expected 2 mashups, got %d", len(resp))
	}
	if resp[0]["stream_url"] == nil {
		t.Error("ready mashup should carry stream_url")
	}
}

func TestHandleDeleteMashupForbiddenForNonOwner(t *testing.T) {
	d := setupTestServer(t)
	owner := registerUser(t, d, "owner@test.com", "owner", "pass123")
	other := registerUser(t, d, "other@test.com", "other", "pass123")
	_ = owner

	// resolve owner's real id via /api/auth/me is overkill; store row with a known owner id
	d.mashupRepo.Create(&entity.Mashup{ID: "m1", OwnerID: "not-the-other-user", MediaID: "abc", Status: "ready"})

	req := httptest.NewRequest("DELETE", "/api/mashups/m1", nil)
	req.Header.Set("Authorization", "Bearer "+other)
	w := httptest.NewRecorder()
	d.server.Router.ServeHTTP(w, req)

	if w.Code != 403 {
		t.Errorf("expected 403, got %d: %s", w.Code, w.Body.String())
	}
}

func TestHandleDeleteMashupNotFound(t *testing.T) {
	d := setupTestServer(t)
	token := registerUser(t, d, "u@test.com", "user", "pass123")

	req := httptest.NewRequest("DELETE", "/api/mashups/ghost", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	w := httptest.NewRecorder()
	d.server.Router.ServeHTTP(w, req)

	if w.Code != 404 {
		t.Errorf("expected 404, got %d", w.Code)
	}
}

func TestHandleMyMashupsRequiresAuth(t *testing.T) {
	d := setupTestServer(t)
	req := httptest.NewRequest("GET", "/api/mashups/mine", nil)
	w := httptest.NewRecorder()
	d.server.Router.ServeHTTP(w, req)

	if w.Code != 401 {
		t.Errorf("expected 401, got %d", w.Code)
	}
}

func TestHandleLikeAndUnlikeMashup(t *testing.T) {
	d := setupTestServer(t)
	token := registerUser(t, d, "liker@test.com", "liker", "pass123")
	d.mashupRepo.Create(&entity.Mashup{ID: "m1", OwnerID: "someone", MediaID: "abc", Status: "ready"})

	req := httptest.NewRequest("POST", "/api/mashups/m1/like", nil)
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

	req = httptest.NewRequest("DELETE", "/api/mashups/m1/like", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	w = httptest.NewRecorder()
	d.server.Router.ServeHTTP(w, req)
	if w.Code != 200 {
		t.Fatalf("unlike: expected 200, got %d", w.Code)
	}
	json.Unmarshal(w.Body.Bytes(), &res)
	if res["likes"] != float64(0) || res["liked"] != false {
		t.Fatalf("unexpected unlike body: %v", res)
	}
}

func TestHandleLikeMashupRequiresAuth(t *testing.T) {
	d := setupTestServer(t)
	d.mashupRepo.Create(&entity.Mashup{ID: "m1", OwnerID: "someone", MediaID: "abc", Status: "ready"})

	req := httptest.NewRequest("POST", "/api/mashups/m1/like", nil)
	w := httptest.NewRecorder()
	d.server.Router.ServeHTTP(w, req)
	if w.Code != 401 {
		t.Errorf("expected 401, got %d", w.Code)
	}
}

func TestHandleLikedMashups(t *testing.T) {
	d := setupTestServer(t)
	token := registerUser(t, d, "liker@test.com", "liker", "pass123")
	uid := userIDFromToken(token)
	d.mashupRepo.Create(&entity.Mashup{ID: "m1", OwnerID: "o", MediaID: "abc", Status: "ready"})
	d.mashupRepo.Create(&entity.Mashup{ID: "m2", OwnerID: "o", MediaID: "def", Status: "ready"})
	if _, err := d.mashupRepo.Like("m2", uid); err != nil {
		t.Fatal(err)
	}

	req := httptest.NewRequest("GET", "/api/mashups/liked", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	w := httptest.NewRecorder()
	d.server.Router.ServeHTTP(w, req)
	if w.Code != 200 {
		t.Fatalf("expected 200, got %d", w.Code)
	}
	var resp []map[string]any
	json.Unmarshal(w.Body.Bytes(), &resp)
	if len(resp) != 1 || resp[0]["id"] != "m2" {
		t.Errorf("expected only m2, got %v", resp)
	}
}

func TestHandleUploadMashupCoverOwnerOnly(t *testing.T) {
	d := setupTestServer(t)
	token := registerUser(t, d, "owner@test.com", "owner", "pass123")
	other := registerUser(t, d, "other@test.com", "other", "pass123")
	d.mashupRepo.Create(&entity.Mashup{ID: "m1", OwnerID: userIDFromToken(token), MediaID: "abc", Status: "ready"})

	var coverMedia string
	d.media.UploadMashupCoverFn = func(mediaID, filename string, body io.Reader) error {
		io.Copy(io.Discard, body)
		coverMedia = mediaID
		return nil
	}

	// Non-owner -> 403.
	body, ct := multipartBody(t, nil, "file", "cover.jpg", "img-bytes")
	req := httptest.NewRequest("PUT", "/api/mashups/m1/cover", body)
	req.Header.Set("Content-Type", ct)
	req.Header.Set("Authorization", "Bearer "+other)
	w := httptest.NewRecorder()
	d.server.Router.ServeHTTP(w, req)
	if w.Code != 403 {
		t.Fatalf("non-owner: expected 403, got %d: %s", w.Code, w.Body.String())
	}

	// Owner -> 200 and media client called.
	body, ct = multipartBody(t, nil, "file", "cover.jpg", "img-bytes")
	req = httptest.NewRequest("PUT", "/api/mashups/m1/cover", body)
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
	if !d.mashupRepo.CoverSet["m1"] {
		t.Error("expected cover flag persisted")
	}
}

func TestHandleUploadMashupWithCover(t *testing.T) {
	d := setupTestServer(t)
	token := registerUser(t, d, "u@test.com", "uploader", "pass123")
	d.media.UploadMashupFn = func(mediaID, filename string, body io.Reader) error {
		io.Copy(io.Discard, body)
		return nil
	}
	coverCalled := false
	d.media.UploadMashupCoverFn = func(mediaID, filename string, body io.Reader) error {
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

	req := httptest.NewRequest("POST", "/api/mashups/", buf)
	req.Header.Set("Content-Type", mw.FormDataContentType())
	req.Header.Set("Authorization", "Bearer "+token)
	w := httptest.NewRecorder()
	d.server.Router.ServeHTTP(w, req)

	if w.Code != 202 {
		t.Fatalf("expected 202, got %d: %s", w.Code, w.Body.String())
	}
	if !coverCalled {
		t.Error("expected the cover to be forwarded to media-service")
	}
}

func TestHandleGetMashup(t *testing.T) {
	d := setupTestServer(t)
	d.mashupRepo.Create(&entity.Mashup{ID: "m9", OwnerID: "o1", Title: "Solo", Status: "processing", MediaID: "zzz"})

	req := httptest.NewRequest("GET", "/api/mashups/m9", nil)
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
	if _, has := resp["stream_url"]; has {
		t.Error("processing mashup should not have stream_url yet")
	}
}
