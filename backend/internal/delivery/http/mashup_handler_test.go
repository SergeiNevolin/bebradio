package http

import (
	"bytes"
	"encoding/json"
	"io"
	"mime/multipart"
	"net/http/httptest"
	"testing"

	"github.com/bebradio/backend-go/internal/domain/entity"
)

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
