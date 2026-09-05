package http

import (
	"bytes"
	"encoding/json"
	"log/slog"
	"net/http/httptest"
	"os"
	"strings"
	"testing"

	"github.com/bebradio/backend-go/internal/config"
	"github.com/bebradio/backend-go/internal/delivery/ws"
	"github.com/bebradio/backend-go/internal/domain/entity"
	"github.com/bebradio/backend-go/internal/domain/repository"
	"github.com/bebradio/backend-go/internal/usecase"
)

var testLog = slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError}))

type testDeps struct {
	userRepo  *repository.MockUserRepo
	roomRepo  *repository.MockRoomRepo
	media     *repository.MockMediaClient
	auth      *usecase.AuthUsecase
	authUC    *usecase.AuthUsecase
	roomUC    *usecase.RoomUsecase
	userUC    *usecase.UserUsecase
	searchUC  *usecase.SearchUsecase
	mediaUC   *usecase.MediaUsecase
	playback  *usecase.PlaybackUsecase
	server    *Server
}

func setupTestServer(t *testing.T) *testDeps {
	t.Helper()

	userRepo := repository.NewMockUserRepo()
	roomRepo := repository.NewMockRoomRepo()
	mediaClient := repository.NewMockMediaClient()
	mediaClient.SearchFn = func(q string, l int) ([]map[string]any, error) {
		return []map[string]any{{"title": "Test Video", "media_id": "m1"}}, nil
	}
	mediaClient.ResolveFn = func(url string) (map[string]any, error) {
		return map[string]any{"media_id": "m1", "title": "Test Video", "duration": float64(180)}, nil
	}
	mediaClient.EnsureFn = func(items []map[string]any) ([]string, error) {
		var ids []string
		for _, item := range items {
			if id, ok := item["media_id"].(string); ok {
				ids = append(ids, id)
			}
		}
		return ids, nil
	}
	mediaClient.CaptionsFn = func(s, l string) (map[string]any, error) {
		return map[string]any{"lang": l, "auto": false, "cues": []any{}}, nil
	}

	cfg := &config.Config{
		MaxDuration: 3600,
		CORSOrigins: []string{"http://localhost:3000"},
	}

	authBridge := repository.NewMockAuthBridge()
	authBridge.DecodeTokenFn = func(t string) (string, error) {
		return strings.TrimPrefix(t, "token_"), nil
	}
	authBridge.VerifyPasswordFn = func(p, h string) bool {
		return h == "hashed_"+p
	}
	auth := usecase.NewAuthUsecase(userRepo, authBridge, testLog)
	roomUC := usecase.NewRoomUsecase(roomRepo, userRepo, mediaClient, authBridge, testLog)
	userUC := usecase.NewUserUsecase(userRepo, testLog)
	searchUC := usecase.NewSearchUsecase(mediaClient, cfg, testLog)
	mediaUC := usecase.NewMediaUsecase(mediaClient, cfg, testLog)
	playback := usecase.NewPlaybackUsecase()

	srv := NewServer(cfg, testLog, auth, roomUC, userUC, searchUC, mediaUC, playback, ws.NewConnectionManager(testLog))

	return &testDeps{
		userRepo: userRepo,
		roomRepo: roomRepo,
		media:    mediaClient,
		auth:     auth,
		authUC:   auth,
		roomUC:   roomUC,
		userUC:   userUC,
		searchUC: searchUC,
		mediaUC:  mediaUC,
		playback: playback,
		server:   srv,
	}
}

func registerUser(t *testing.T, d *testDeps, email, username, password string) string {
	t.Helper()
	body, _ := json.Marshal(map[string]string{"email": email, "username": username, "password": password})
	req := httptest.NewRequest("POST", "/api/auth/register", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	d.server.Router.ServeHTTP(w, req)

	var resp map[string]any
	json.Unmarshal(w.Body.Bytes(), &resp)
	token, _ := resp["token"].(string)
	return token
}

// === Auth Handler Tests ===

func TestHandleRegisterSuccess(t *testing.T) {
	d := setupTestServer(t)
	body, _ := json.Marshal(map[string]string{"email": "test@test.com", "username": "testuser", "password": "pass123"})
	req := httptest.NewRequest("POST", "/api/auth/register", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	d.server.Router.ServeHTTP(w, req)

	if w.Code != 200 {
		t.Errorf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
	var resp map[string]any
	json.Unmarshal(w.Body.Bytes(), &resp)
	if resp["token"] == nil || resp["token"] == "" {
		t.Error("expected non-empty token")
	}
	user := resp["user"].(map[string]any)
	if user["username"] != "testuser" {
		t.Errorf("expected username 'testuser', got '%v'", user["username"])
	}
}

func TestHandleRegisterInvalidEmail(t *testing.T) {
	d := setupTestServer(t)
	body, _ := json.Marshal(map[string]string{"email": "not-an-email", "username": "testuser", "password": "pass123"})
	req := httptest.NewRequest("POST", "/api/auth/register", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	d.server.Router.ServeHTTP(w, req)

	if w.Code != 400 {
		t.Errorf("expected 400, got %d", w.Code)
	}
}

func TestHandleRegisterShortUsername(t *testing.T) {
	d := setupTestServer(t)
	body, _ := json.Marshal(map[string]string{"email": "test@test.com", "username": "a", "password": "pass123"})
	req := httptest.NewRequest("POST", "/api/auth/register", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	d.server.Router.ServeHTTP(w, req)

	if w.Code != 400 {
		t.Errorf("expected 400, got %d", w.Code)
	}
}

func TestHandleRegisterLongUsername(t *testing.T) {
	d := setupTestServer(t)
	body, _ := json.Marshal(map[string]string{"email": "test@test.com", "username": "thisusernameiswaytoolongfortheregex", "password": "pass123"})
	req := httptest.NewRequest("POST", "/api/auth/register", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	d.server.Router.ServeHTTP(w, req)

	if w.Code != 400 {
		t.Errorf("expected 400, got %d", w.Code)
	}
}

func TestHandleRegisterDuplicateEmail(t *testing.T) {
	d := setupTestServer(t)
	body, _ := json.Marshal(map[string]string{"email": "dup@test.com", "username": "user1", "password": "pass123"})
	req := httptest.NewRequest("POST", "/api/auth/register", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	d.server.Router.ServeHTTP(w, req)

	body2, _ := json.Marshal(map[string]string{"email": "dup@test.com", "username": "user2", "password": "pass123"})
	req2 := httptest.NewRequest("POST", "/api/auth/register", bytes.NewReader(body2))
	req2.Header.Set("Content-Type", "application/json")
	w2 := httptest.NewRecorder()
	d.server.Router.ServeHTTP(w2, req2)

	if w2.Code != 409 {
		t.Errorf("expected 409, got %d", w2.Code)
	}
}

func TestHandleLoginSuccess(t *testing.T) {
	d := setupTestServer(t)
	registerUser(t, d, "test@test.com", "testuser", "pass123")

	body, _ := json.Marshal(map[string]string{"email": "test@test.com", "password": "pass123"})
	req := httptest.NewRequest("POST", "/api/auth/login", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	d.server.Router.ServeHTTP(w, req)

	if w.Code != 200 {
		t.Errorf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
}

func TestHandleLoginWrongPassword(t *testing.T) {
	d := setupTestServer(t)
	registerUser(t, d, "test@test.com", "testuser", "pass123")

	body, _ := json.Marshal(map[string]string{"email": "test@test.com", "password": "wrong"})
	req := httptest.NewRequest("POST", "/api/auth/login", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	d.server.Router.ServeHTTP(w, req)

	if w.Code != 401 {
		t.Errorf("expected 401, got %d", w.Code)
	}
}

func TestHandleMeAuthenticated(t *testing.T) {
	d := setupTestServer(t)
	token := registerUser(t, d, "test@test.com", "testuser", "pass123")

	req := httptest.NewRequest("GET", "/api/auth/me", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	w := httptest.NewRecorder()

	d.server.Router.ServeHTTP(w, req)

	if w.Code != 200 {
		t.Errorf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
	var resp map[string]any
	json.Unmarshal(w.Body.Bytes(), &resp)
	user := resp["user"].(map[string]any)
	if user["username"] != "testuser" {
		t.Errorf("expected username 'testuser', got '%v'", user["username"])
	}
}

func TestHandleMeNotAuthenticated(t *testing.T) {
	d := setupTestServer(t)

	req := httptest.NewRequest("GET", "/api/auth/me", nil)
	w := httptest.NewRecorder()

	d.server.Router.ServeHTTP(w, req)

	if w.Code != 401 {
		t.Errorf("expected 401, got %d", w.Code)
	}
}

func TestHandleMeInvalidToken(t *testing.T) {
	d := setupTestServer(t)

	req := httptest.NewRequest("GET", "/api/auth/me", nil)
	req.Header.Set("Authorization", "Bearer invalid-token")
	w := httptest.NewRecorder()

	d.server.Router.ServeHTTP(w, req)

	if w.Code != 401 {
		t.Errorf("expected 401, got %d", w.Code)
	}
}

// === Room Handler Tests ===

func TestHandleCreateRoom(t *testing.T) {
	d := setupTestServer(t)
	token := registerUser(t, d, "test@test.com", "testuser", "pass123")

	body, _ := json.Marshal(map[string]string{"name": "Test Room"})
	req := httptest.NewRequest("POST", "/api/rooms/", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+token)
	w := httptest.NewRecorder()

	d.server.Router.ServeHTTP(w, req)

	if w.Code != 200 {
		t.Errorf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
	var resp map[string]any
	json.Unmarshal(w.Body.Bytes(), &resp)
	if resp["name"] != "Test Room" {
		t.Errorf("expected name 'Test Room', got '%v'", resp["name"])
	}
}

func TestHandleCreateRoomNoAuth(t *testing.T) {
	d := setupTestServer(t)

	body, _ := json.Marshal(map[string]string{"name": "Test Room"})
	req := httptest.NewRequest("POST", "/api/rooms/", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	d.server.Router.ServeHTTP(w, req)

	if w.Code != 401 {
		t.Errorf("expected 401, got %d", w.Code)
	}
}

func TestHandleCreateRoomDefaultName(t *testing.T) {
	d := setupTestServer(t)
	token := registerUser(t, d, "test@test.com", "testuser", "pass123")

	body, _ := json.Marshal(map[string]string{})
	req := httptest.NewRequest("POST", "/api/rooms/", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+token)
	w := httptest.NewRecorder()

	d.server.Router.ServeHTTP(w, req)

	if w.Code != 200 {
		t.Errorf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
	var resp map[string]any
	json.Unmarshal(w.Body.Bytes(), &resp)
	if resp["name"] != "My Room" {
		t.Errorf("expected default name 'My Room', got '%v'", resp["name"])
	}
}

func TestHandleListRoomsEmpty(t *testing.T) {
	d := setupTestServer(t)

	req := httptest.NewRequest("GET", "/api/rooms/", nil)
	w := httptest.NewRecorder()

	d.server.Router.ServeHTTP(w, req)

	if w.Code != 200 {
		t.Errorf("expected 200, got %d", w.Code)
	}
	var resp []any
	json.Unmarshal(w.Body.Bytes(), &resp)
	if len(resp) != 0 {
		t.Errorf("expected empty list, got %d items", len(resp))
	}
}

func TestHandleListRooms(t *testing.T) {
	d := setupTestServer(t)
	token := registerUser(t, d, "test@test.com", "testuser", "pass123")

	body, _ := json.Marshal(map[string]string{"name": "Room1"})
	req := httptest.NewRequest("POST", "/api/rooms/", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+token)
	w := httptest.NewRecorder()
	d.server.Router.ServeHTTP(w, req)

	req2 := httptest.NewRequest("GET", "/api/rooms/", nil)
	w2 := httptest.NewRecorder()
	d.server.Router.ServeHTTP(w2, req2)

	var resp []any
	json.Unmarshal(w2.Body.Bytes(), &resp)
	if len(resp) != 1 {
		t.Errorf("expected 1 room, got %d", len(resp))
	}
}

func TestHandleGetRoom(t *testing.T) {
	d := setupTestServer(t)
	token := registerUser(t, d, "test@test.com", "testuser", "pass123")

	body, _ := json.Marshal(map[string]string{"name": "Test Room"})
	req := httptest.NewRequest("POST", "/api/rooms/", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+token)
	w := httptest.NewRecorder()
	d.server.Router.ServeHTTP(w, req)

	var createResp map[string]any
	json.Unmarshal(w.Body.Bytes(), &createResp)
	roomID := createResp["id"].(string)

	req2 := httptest.NewRequest("GET", "/api/rooms/"+roomID, nil)
	w2 := httptest.NewRecorder()
	d.server.Router.ServeHTTP(w2, req2)

	if w2.Code != 200 {
		t.Errorf("expected 200, got %d", w2.Code)
	}
}

func TestHandleGetRoomNotFound(t *testing.T) {
	d := setupTestServer(t)

	req := httptest.NewRequest("GET", "/api/rooms/nonexistent", nil)
	w := httptest.NewRecorder()

	d.server.Router.ServeHTTP(w, req)

	if w.Code != 404 {
		t.Errorf("expected 404, got %d", w.Code)
	}
}

func TestHandleUpdateRoomOwner(t *testing.T) {
	d := setupTestServer(t)
	token := registerUser(t, d, "test@test.com", "testuser", "pass123")

	body, _ := json.Marshal(map[string]string{"name": "Room"})
	req := httptest.NewRequest("POST", "/api/rooms/", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+token)
	w := httptest.NewRecorder()
	d.server.Router.ServeHTTP(w, req)

	var createResp map[string]any
	json.Unmarshal(w.Body.Bytes(), &createResp)
	roomID := createResp["id"].(string)

	updateBody, _ := json.Marshal(map[string]bool{"is_private": true})
	req2 := httptest.NewRequest("PATCH", "/api/rooms/"+roomID, bytes.NewReader(updateBody))
	req2.Header.Set("Content-Type", "application/json")
	req2.Header.Set("Authorization", "Bearer "+token)
	w2 := httptest.NewRecorder()
	d.server.Router.ServeHTTP(w2, req2)

	if w2.Code != 200 {
		t.Errorf("expected 200, got %d: %s", w2.Code, w2.Body.String())
	}
}

func TestHandleUpdateRoomNotOwner(t *testing.T) {
	d := setupTestServer(t)
	token1 := registerUser(t, d, "owner@test.com", "owner", "pass123")
	token2 := registerUser(t, d, "other@test.com", "other", "pass123")

	body, _ := json.Marshal(map[string]string{"name": "Room"})
	req := httptest.NewRequest("POST", "/api/rooms/", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+token1)
	w := httptest.NewRecorder()
	d.server.Router.ServeHTTP(w, req)

	var createResp map[string]any
	json.Unmarshal(w.Body.Bytes(), &createResp)
	roomID := createResp["id"].(string)

	updateBody, _ := json.Marshal(map[string]bool{"is_private": true})
	req2 := httptest.NewRequest("PATCH", "/api/rooms/"+roomID, bytes.NewReader(updateBody))
	req2.Header.Set("Content-Type", "application/json")
	req2.Header.Set("Authorization", "Bearer "+token2)
	w2 := httptest.NewRecorder()
	d.server.Router.ServeHTTP(w2, req2)

	if w2.Code != 403 {
		t.Errorf("expected 403, got %d", w2.Code)
	}
}

func TestHandleDeleteRoom(t *testing.T) {
	d := setupTestServer(t)
	token := registerUser(t, d, "test@test.com", "testuser", "pass123")

	body, _ := json.Marshal(map[string]string{"name": "Room"})
	req := httptest.NewRequest("POST", "/api/rooms/", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+token)
	w := httptest.NewRecorder()
	d.server.Router.ServeHTTP(w, req)

	var createResp map[string]any
	json.Unmarshal(w.Body.Bytes(), &createResp)
	roomID := createResp["id"].(string)

	req2 := httptest.NewRequest("DELETE", "/api/rooms/"+roomID, nil)
	req2.Header.Set("Authorization", "Bearer "+token)
	w2 := httptest.NewRecorder()
	d.server.Router.ServeHTTP(w2, req2)

	if w2.Code != 200 {
		t.Errorf("expected 200, got %d", w2.Code)
	}

	// Verify room is gone
	req3 := httptest.NewRequest("GET", "/api/rooms/"+roomID, nil)
	w3 := httptest.NewRecorder()
	d.server.Router.ServeHTTP(w3, req3)
	if w3.Code != 404 {
		t.Errorf("expected 404 after delete, got %d", w3.Code)
	}
}

func TestHandleDeleteRoomNotOwner(t *testing.T) {
	d := setupTestServer(t)
	token1 := registerUser(t, d, "owner@test.com", "owner", "pass123")
	token2 := registerUser(t, d, "other@test.com", "other", "pass123")

	body, _ := json.Marshal(map[string]string{"name": "Room"})
	req := httptest.NewRequest("POST", "/api/rooms/", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+token1)
	w := httptest.NewRecorder()
	d.server.Router.ServeHTTP(w, req)

	var createResp map[string]any
	json.Unmarshal(w.Body.Bytes(), &createResp)
	roomID := createResp["id"].(string)

	req2 := httptest.NewRequest("DELETE", "/api/rooms/"+roomID, nil)
	req2.Header.Set("Authorization", "Bearer "+token2)
	w2 := httptest.NewRecorder()
	d.server.Router.ServeHTTP(w2, req2)

	if w2.Code != 403 {
		t.Errorf("expected 403, got %d", w2.Code)
	}
}

func TestHandleJoinRoomNoPassword(t *testing.T) {
	d := setupTestServer(t)
	token := registerUser(t, d, "test@test.com", "testuser", "pass123")

	body, _ := json.Marshal(map[string]string{"name": "Room"})
	req := httptest.NewRequest("POST", "/api/rooms/", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+token)
	w := httptest.NewRecorder()
	d.server.Router.ServeHTTP(w, req)

	var createResp map[string]any
	json.Unmarshal(w.Body.Bytes(), &createResp)
	roomID := createResp["id"].(string)

	joinBody, _ := json.Marshal(map[string]string{"username": "joiner"})
	req2 := httptest.NewRequest("POST", "/api/rooms/"+roomID+"/join", bytes.NewReader(joinBody))
	req2.Header.Set("Content-Type", "application/json")
	w2 := httptest.NewRecorder()
	d.server.Router.ServeHTTP(w2, req2)

	if w2.Code != 200 {
		t.Errorf("expected 200, got %d: %s", w2.Code, w2.Body.String())
	}
}

func TestHandleJoinRoomNotFound(t *testing.T) {
	d := setupTestServer(t)

	joinBody, _ := json.Marshal(map[string]string{"username": "joiner"})
	req := httptest.NewRequest("POST", "/api/rooms/nonexistent/join", bytes.NewReader(joinBody))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	d.server.Router.ServeHTTP(w, req)

	if w.Code != 404 {
		t.Errorf("expected 404, got %d", w.Code)
	}
}

// === Search Handler Tests ===

func TestHandleSearchSuccess(t *testing.T) {
	d := setupTestServer(t)

	body, _ := json.Marshal(map[string]any{"query": "test song", "limit": 5})
	req := httptest.NewRequest("POST", "/api/search", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	d.server.Router.ServeHTTP(w, req)

	if w.Code != 200 {
		t.Errorf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
	var resp []any
	json.Unmarshal(w.Body.Bytes(), &resp)
	if len(resp) == 0 {
		t.Error("expected results")
	}
}

func TestHandleSearchEmptyQuery(t *testing.T) {
	d := setupTestServer(t)

	body, _ := json.Marshal(map[string]any{"query": ""})
	req := httptest.NewRequest("POST", "/api/search", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	d.server.Router.ServeHTTP(w, req)

	if w.Code != 200 {
		t.Errorf("expected 200, got %d", w.Code)
	}
	var resp []any
	json.Unmarshal(w.Body.Bytes(), &resp)
	if len(resp) != 0 {
		t.Errorf("expected empty results, got %d", len(resp))
	}
}

func TestHandleSearchInvalidBody(t *testing.T) {
	d := setupTestServer(t)

	req := httptest.NewRequest("POST", "/api/search", bytes.NewReader([]byte("not json")))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	d.server.Router.ServeHTTP(w, req)

	if w.Code != 400 {
		t.Errorf("expected 400, got %d", w.Code)
	}
}

// === CORS Tests ===

func TestCORSMiddlewareAllowed(t *testing.T) {
	d := setupTestServer(t)

	req := httptest.NewRequest("OPTIONS", "/api/rooms/", nil)
	req.Header.Set("Origin", "http://localhost:3000")
	w := httptest.NewRecorder()

	d.server.Router.ServeHTTP(w, req)

	if w.Code != 204 {
		t.Errorf("expected 204 for OPTIONS, got %d", w.Code)
	}
	if w.Header().Get("Access-Control-Allow-Origin") != "http://localhost:3000" {
		t.Errorf("expected CORS origin header, got '%s'", w.Header().Get("Access-Control-Allow-Origin"))
	}
}

func TestCORSMiddlewareDisallowed(t *testing.T) {
	d := setupTestServer(t)

	req := httptest.NewRequest("OPTIONS", "/api/rooms/", nil)
	req.Header.Set("Origin", "http://evil.com")
	w := httptest.NewRecorder()

	d.server.Router.ServeHTTP(w, req)

	if w.Header().Get("Access-Control-Allow-Origin") != "" {
		t.Error("expected no CORS origin for disallowed origin")
	}
}

// === User Handler Tests ===

func TestHandleGetMe(t *testing.T) {
	d := setupTestServer(t)
	token := registerUser(t, d, "test@test.com", "testuser", "pass123")

	req := httptest.NewRequest("GET", "/api/users/me", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	w := httptest.NewRecorder()

	d.server.Router.ServeHTTP(w, req)

	if w.Code != 200 {
		t.Errorf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
}

func TestHandleGetMeNoAuth(t *testing.T) {
	d := setupTestServer(t)

	req := httptest.NewRequest("GET", "/api/users/me", nil)
	w := httptest.NewRecorder()

	d.server.Router.ServeHTTP(w, req)

	if w.Code != 401 {
		t.Errorf("expected 401, got %d", w.Code)
	}
}

func TestHandleGetUser(t *testing.T) {
	d := setupTestServer(t)
	userRepo := d.userRepo
	userRepo.Create(&entity.User{ID: "u1", Username: "publicuser", Email: "pub@test.com"})

	req := httptest.NewRequest("GET", "/api/users/u1", nil)
	w := httptest.NewRecorder()

	d.server.Router.ServeHTTP(w, req)

	if w.Code != 200 {
		t.Errorf("expected 200, got %d", w.Code)
	}
	var resp map[string]any
	json.Unmarshal(w.Body.Bytes(), &resp)
	user := resp["user"].(map[string]any)
	if user["username"] != "publicuser" {
		t.Errorf("expected username 'publicuser', got '%v'", user["username"])
	}
	// Should not expose email in public profile
	if _, exists := user["email"]; exists {
		t.Error("public profile should not expose email")
	}
}

func TestHandleGetUserNotFound(t *testing.T) {
	d := setupTestServer(t)

	req := httptest.NewRequest("GET", "/api/users/nonexistent", nil)
	w := httptest.NewRecorder()

	d.server.Router.ServeHTTP(w, req)

	if w.Code != 404 {
		t.Errorf("expected 404, got %d", w.Code)
	}
}

func TestHandlePlayback(t *testing.T) {
	d := setupTestServer(t)
	token := registerUser(t, d, "test@test.com", "testuser", "pass123")

	body, _ := json.Marshal(map[string]string{"name": "Room"})
	req := httptest.NewRequest("POST", "/api/rooms/", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+token)
	w := httptest.NewRecorder()
	d.server.Router.ServeHTTP(w, req)

	var createResp map[string]any
	json.Unmarshal(w.Body.Bytes(), &createResp)
	roomID := createResp["id"].(string)

	playBody, _ := json.Marshal(map[string]string{"action": "next"})
	req2 := httptest.NewRequest("POST", "/api/rooms/"+roomID+"/playback", bytes.NewReader(playBody))
	req2.Header.Set("Content-Type", "application/json")
	w2 := httptest.NewRecorder()
	d.server.Router.ServeHTTP(w2, req2)

	if w2.Code != 200 {
		t.Errorf("expected 200, got %d: %s", w2.Code, w2.Body.String())
	}
}
