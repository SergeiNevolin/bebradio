package api

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/leenzstra/bebradio/backend/internal/auth"
	"github.com/leenzstra/bebradio/backend/internal/config"
	"github.com/leenzstra/bebradio/backend/internal/hub"
	"github.com/leenzstra/bebradio/backend/internal/room"
	"github.com/leenzstra/bebradio/backend/internal/store/memory"
	"github.com/leenzstra/bebradio/backend/internal/users"
	"github.com/leenzstra/bebradio/backend/internal/youtube"
	"github.com/leenzstra/bebradio/backend/internal/youtube/youtubetest"
)

// harness is a running service backed entirely by in-memory collaborators.
type harness struct {
	t      *testing.T
	server *httptest.Server
	rooms  *room.Service
	store  *memory.Store
	yt     *youtubetest.Fake
}

func newHarness(t *testing.T) *harness {
	t.Helper()

	cfg := testConfig()
	st := memory.New()
	yt := youtubetest.New()
	log := slog.New(slog.NewTextHandler(io.Discard, nil))

	tokens := auth.NewTokens(cfg.SecretKey, cfg.JWTExpiry)
	passwords := auth.NewPasswords(cfg.BcryptCost)
	connections := hub.New(log)

	rooms := room.New(room.Deps{
		Store: st, Hub: connections, YouTube: yt,
		Tokens: tokens, Passwords: passwords, Config: cfg, Logger: log,
	})
	t.Cleanup(rooms.Shutdown)

	server := httptest.NewServer(NewServer(Deps{
		Rooms: rooms, Users: users.New(st, tokens, passwords),
		Hub: connections, YouTube: yt, Store: st, Config: cfg, Logger: log,
	}).Routes())
	t.Cleanup(server.Close)

	return &harness{t: t, server: server, rooms: rooms, store: st, yt: yt}
}

func testConfig() config.Config {
	return config.Config{
		CORSOrigins:         []string{"http://localhost:3000"},
		MaxRequestBytes:     1 << 20,
		SecretKey:           "test-secret",
		JWTExpiry:           time.Hour,
		BcryptCost:          4, // the cheapest bcrypt allows; tests are not a benchmark
		MaxChatMessages:     100,
		MaxChatTextLen:      2000,
		StreamRefreshMargin: 600 * time.Second,
		AutoAdvanceInterval: 50 * time.Millisecond,
		AutoAdvanceGrace:    2500 * time.Millisecond,
		AdvanceDedupWindow:  time.Second,
		ReactionEmojis:      config.DefaultReactionEmojis,
		RadioRefillAt:       1,
		RadioBatch:          3,
	}
}

// response is a decoded HTTP reply.
type response struct {
	status int
	body   []byte
}

// json decodes the body into dst, failing the test if it will not parse.
func (r response) decode(t *testing.T, dst any) {
	t.Helper()
	if err := json.Unmarshal(r.body, dst); err != nil {
		t.Fatalf("decoding %s: %v", r.body, err)
	}
}

// object decodes the body into a generic map.
func (r response) object(t *testing.T) map[string]any {
	t.Helper()
	var out map[string]any
	r.decode(t, &out)
	return out
}

func (r response) errorMessage(t *testing.T) string {
	t.Helper()
	msg, _ := r.object(t)["error"].(string)
	return msg
}

// do sends a request. A nil body sends none; token, when set, is sent as a
// bearer credential.
func (h *harness) do(method, path string, body any, token string) response {
	h.t.Helper()

	var reader io.Reader
	if body != nil {
		encoded, err := json.Marshal(body)
		if err != nil {
			h.t.Fatalf("encoding request body: %v", err)
		}
		reader = bytes.NewReader(encoded)
	}

	req, err := http.NewRequestWithContext(h.t.Context(), method, h.server.URL+path, reader)
	if err != nil {
		h.t.Fatalf("building request: %v", err)
	}
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}

	res, err := h.server.Client().Do(req)
	if err != nil {
		h.t.Fatalf("%s %s: %v", method, path, err)
	}
	defer res.Body.Close()

	payload, err := io.ReadAll(res.Body)
	if err != nil {
		h.t.Fatalf("reading response body: %v", err)
	}
	return response{status: res.StatusCode, body: payload}
}

// register creates an account and returns its bearer token.
func (h *harness) register(username, email string) string {
	h.t.Helper()
	res := h.do(http.MethodPost, "/api/auth/register", map[string]string{
		"email": email, "username": username, "password": "pass123",
	}, "")
	if res.status != http.StatusOK {
		h.t.Fatalf("register(%s): status %d, body %s", username, res.status, res.body)
	}
	token, _ := res.object(h.t)["token"].(string)
	if token == "" {
		h.t.Fatalf("register(%s) returned no token: %s", username, res.body)
	}
	return token
}

// createRoom makes a room and returns its code plus any access token.
func (h *harness) createRoom(token, name string, password *string) (string, string) {
	h.t.Helper()
	body := map[string]any{"name": name}
	if password != nil {
		body["password"] = *password
	}
	res := h.do(http.MethodPost, "/api/rooms", body, token)
	if res.status != http.StatusOK {
		h.t.Fatalf("createRoom: status %d, body %s", res.status, res.body)
	}
	obj := res.object(h.t)
	id, _ := obj["id"].(string)
	access, _ := obj["access"].(string)
	return id, access
}

// --- Auth ---

func TestRegister(t *testing.T) {
	h := newHarness(t)

	res := h.do(http.MethodPost, "/api/auth/register", map[string]string{
		"email": "a@b.com", "username": "Alice", "password": "secret",
	}, "")

	if res.status != http.StatusOK {
		t.Fatalf("status = %d, body %s", res.status, res.body)
	}
	obj := res.object(t)
	if obj["token"] == "" {
		t.Error("no token returned")
	}
	user := obj["user"].(map[string]any)
	if user["email"] != "a@b.com" || user["username"] != "Alice" {
		t.Errorf("user = %v", user)
	}
}

func TestRegisterRejectsDuplicates(t *testing.T) {
	h := newHarness(t)
	h.register("Alice", "a@b.com")

	t.Run("email", func(t *testing.T) {
		res := h.do(http.MethodPost, "/api/auth/register", map[string]string{
			"email": "a@b.com", "username": "Bob", "password": "other",
		}, "")
		if res.status != http.StatusConflict {
			t.Fatalf("status = %d, want 409; body %s", res.status, res.body)
		}
		if got := res.errorMessage(t); got != "Email already registered" {
			t.Errorf("error = %q", got)
		}
	})

	t.Run("username", func(t *testing.T) {
		res := h.do(http.MethodPost, "/api/auth/register", map[string]string{
			"email": "c@d.com", "username": "Alice", "password": "other",
		}, "")
		if res.status != http.StatusConflict {
			t.Fatalf("status = %d, want 409; body %s", res.status, res.body)
		}
		if got := res.errorMessage(t); got != "Username already taken" {
			t.Errorf("error = %q", got)
		}
	})
}

func TestRegisterRejectsInvalidInput(t *testing.T) {
	h := newHarness(t)

	cases := map[string]map[string]string{
		"invalid email":  {"email": "not-an-email", "username": "Alice", "password": "secret"},
		"short username": {"email": "a@b.com", "username": "A", "password": "secret"},
	}
	for name, body := range cases {
		t.Run(name, func(t *testing.T) {
			res := h.do(http.MethodPost, "/api/auth/register", body, "")
			if res.status != http.StatusUnprocessableEntity {
				t.Errorf("status = %d, want 422; body %s", res.status, res.body)
			}
			if res.errorMessage(t) == "" {
				t.Error("no error message returned")
			}
		})
	}
}

func TestLogin(t *testing.T) {
	h := newHarness(t)
	h.register("Alice", "a@b.com")

	t.Run("correct password", func(t *testing.T) {
		res := h.do(http.MethodPost, "/api/auth/login",
			map[string]string{"email": "a@b.com", "password": "pass123"}, "")
		if res.status != http.StatusOK {
			t.Fatalf("status = %d, body %s", res.status, res.body)
		}
		obj := res.object(t)
		if obj["token"] == "" {
			t.Error("no token returned")
		}
		if obj["user"].(map[string]any)["username"] != "Alice" {
			t.Errorf("user = %v", obj["user"])
		}
	})

	for name, body := range map[string]map[string]string{
		"wrong password":  {"email": "a@b.com", "password": "wrong"},
		"unknown account": {"email": "nobody@b.com", "password": "x"},
	} {
		t.Run(name, func(t *testing.T) {
			res := h.do(http.MethodPost, "/api/auth/login", body, "")
			if res.status != http.StatusUnauthorized {
				t.Errorf("status = %d, want 401; body %s", res.status, res.body)
			}
			// Both cases must read the same, or the endpoint would reveal
			// which addresses have accounts.
			if got := res.errorMessage(t); got != "Invalid email or password" {
				t.Errorf("error = %q", got)
			}
		})
	}
}

func TestAuthMe(t *testing.T) {
	h := newHarness(t)
	token := h.register("TestUser", "test@test.com")

	t.Run("with a token", func(t *testing.T) {
		res := h.do(http.MethodGet, "/api/auth/me", nil, token)
		if res.status != http.StatusOK {
			t.Fatalf("status = %d, body %s", res.status, res.body)
		}
		if res.object(t)["user"].(map[string]any)["username"] != "TestUser" {
			t.Errorf("user = %v", res.object(t)["user"])
		}
	})

	for name, tok := range map[string]string{"no token": "", "invalid token": "invalid"} {
		t.Run(name, func(t *testing.T) {
			if res := h.do(http.MethodGet, "/api/auth/me", nil, tok); res.status != http.StatusUnauthorized {
				t.Errorf("status = %d, want 401; body %s", res.status, res.body)
			}
		})
	}
}

// --- Profiles ---

func TestProfileEndpoints(t *testing.T) {
	h := newHarness(t)
	token := h.register("Alice", "a@b.com")

	own := h.do(http.MethodGet, "/api/users/me", nil, token)
	if own.status != http.StatusOK {
		t.Fatalf("GET /api/users/me: status %d, body %s", own.status, own.body)
	}
	user := own.object(t)["user"].(map[string]any)
	if user["email"] != "a@b.com" {
		t.Errorf("own profile should carry the email address: %v", user)
	}
	userID := user["id"].(string)

	updated := h.do(http.MethodPut, "/api/users/me",
		map[string]string{"bio": "I like music", "avatar_url": "https://img.example/a.png"}, token)
	if updated.status != http.StatusOK {
		t.Fatalf("PUT /api/users/me: status %d, body %s", updated.status, updated.body)
	}
	if got := updated.object(t)["user"].(map[string]any)["bio"]; got != "I like music" {
		t.Errorf("bio = %v", got)
	}

	public := h.do(http.MethodGet, "/api/users/"+userID, nil, "")
	if public.status != http.StatusOK {
		t.Fatalf("GET /api/users/{id}: status %d, body %s", public.status, public.body)
	}
	if _, leaked := public.object(t)["user"].(map[string]any)["email"]; leaked {
		t.Error("another user's profile carried their email address")
	}

	if res := h.do(http.MethodGet, "/api/users/nope", nil, ""); res.status != http.StatusNotFound {
		t.Errorf("unknown user: status %d, want 404", res.status)
	}
	if res := h.do(http.MethodGet, "/api/users/me", nil, ""); res.status != http.StatusUnauthorized {
		t.Errorf("unauthenticated own profile: status %d, want 401", res.status)
	}
}

// --- Rooms ---

func TestCreateRoom(t *testing.T) {
	h := newHarness(t)
	token := h.register("Alice", "a@b.com")

	res := h.do(http.MethodPost, "/api/rooms", map[string]string{"name": "Test Room"}, token)
	if res.status != http.StatusOK {
		t.Fatalf("status = %d, body %s", res.status, res.body)
	}

	obj := res.object(t)
	if obj["name"] != "Test Room" {
		t.Errorf("name = %v", obj["name"])
	}
	if id, _ := obj["id"].(string); len(id) != 6 {
		t.Errorf("id = %v, want 6 characters", obj["id"])
	}
	if queue := obj["queue"].([]any); len(queue) != 0 {
		t.Errorf("queue = %v, want empty", queue)
	}
	if obj["is_playing"] != false || obj["position"] != float64(0) {
		t.Errorf("a new room should be idle: %v", obj)
	}
}

func TestCreateRoomDefaultsAndAuth(t *testing.T) {
	h := newHarness(t)
	token := h.register("Alice", "a@b.com")

	res := h.do(http.MethodPost, "/api/rooms", map[string]any{}, token)
	if res.status != http.StatusOK {
		t.Fatalf("status = %d, body %s", res.status, res.body)
	}
	if res.object(t)["name"] != "My Room" {
		t.Errorf("name = %v, want the default", res.object(t)["name"])
	}

	if anon := h.do(http.MethodPost, "/api/rooms", map[string]string{"name": "X"}, ""); anon.status != http.StatusUnauthorized {
		t.Errorf("anonymous create: status %d, want 401", anon.status)
	}
}

// The browser sends `password: null` when the field was left blank; that must
// create an open room, not one locked with an empty password.
func TestCreateRoomWithNullPassword(t *testing.T) {
	h := newHarness(t)
	token := h.register("Alice", "a@b.com")

	res := h.do(http.MethodPost, "/api/rooms", map[string]any{"name": "Open", "password": nil}, token)
	if res.status != http.StatusOK {
		t.Fatalf("status = %d, body %s", res.status, res.body)
	}
	if res.object(t)["has_password"] != false {
		t.Error("a null password should leave the room open")
	}
}

func TestListRooms(t *testing.T) {
	h := newHarness(t)
	token := h.register("Alice", "a@b.com")

	empty := h.do(http.MethodGet, "/api/rooms", nil, "")
	if empty.status != http.StatusOK {
		t.Fatalf("status = %d, body %s", empty.status, empty.body)
	}
	var rooms []map[string]any
	empty.decode(t, &rooms)
	if len(rooms) != 0 {
		t.Fatalf("rooms = %v, want none", rooms)
	}

	h.createRoom(token, "Room A", nil)
	h.createRoom(token, "Room B", nil)

	listed := h.do(http.MethodGet, "/api/rooms", nil, "")
	listed.decode(t, &rooms)
	if len(rooms) != 2 {
		t.Fatalf("rooms = %v, want 2", rooms)
	}
	names := map[string]bool{}
	for _, r := range rooms {
		names[r["name"].(string)] = true
	}
	if !names["Room A"] || !names["Room B"] {
		t.Errorf("names = %v", names)
	}
}

func TestGetRoom(t *testing.T) {
	h := newHarness(t)
	token := h.register("Alice", "a@b.com")
	roomID, _ := h.createRoom(token, "R1", nil)

	res := h.do(http.MethodGet, "/api/rooms/"+roomID, nil, "")
	if res.status != http.StatusOK {
		t.Fatalf("status = %d, body %s", res.status, res.body)
	}
	if res.object(t)["id"] != roomID {
		t.Errorf("id = %v, want %q", res.object(t)["id"], roomID)
	}

	if missing := h.do(http.MethodGet, "/api/rooms/XXXXXX", nil, ""); missing.status != http.StatusNotFound {
		t.Errorf("unknown room: status %d, want 404", missing.status)
	}
}

func TestRoomSettingsDefaults(t *testing.T) {
	h := newHarness(t)
	token := h.register("Alice", "a@b.com")
	roomID, _ := h.createRoom(token, "S", nil)

	obj := h.do(http.MethodGet, "/api/rooms/"+roomID, nil, "").object(t)
	if obj["allow_anonymous_add"] != true {
		t.Error("allow_anonymous_add should default to true")
	}
	if obj["is_private"] != false {
		t.Error("is_private should default to false")
	}
	if obj["owner_id"] == "" {
		t.Error("owner_id should be set")
	}
}

func TestUpdateRoomSettings(t *testing.T) {
	h := newHarness(t)
	owner := h.register("Owner", "owner@test.com")
	other := h.register("Other", "other@test.com")
	roomID, _ := h.createRoom(owner, "S", nil)

	t.Run("owner", func(t *testing.T) {
		res := h.do(http.MethodPatch, "/api/rooms/"+roomID,
			map[string]any{"allow_anonymous_add": false, "is_private": true}, owner)
		if res.status != http.StatusOK {
			t.Fatalf("status = %d, body %s", res.status, res.body)
		}
		obj := res.object(t)
		if obj["allow_anonymous_add"] != false || obj["is_private"] != true {
			t.Errorf("settings = %v", obj)
		}
	})

	t.Run("another user", func(t *testing.T) {
		res := h.do(http.MethodPatch, "/api/rooms/"+roomID, map[string]any{"is_private": true}, other)
		if res.status != http.StatusForbidden {
			t.Errorf("status = %d, want 403; body %s", res.status, res.body)
		}
	})

	t.Run("anonymous", func(t *testing.T) {
		res := h.do(http.MethodPatch, "/api/rooms/"+roomID, map[string]any{"is_private": true}, "")
		if res.status != http.StatusUnauthorized {
			t.Errorf("status = %d, want 401; body %s", res.status, res.body)
		}
	})
}

func TestPrivateRoomIsHiddenButStillReachable(t *testing.T) {
	h := newHarness(t)
	token := h.register("Alice", "a@b.com")
	roomID, _ := h.createRoom(token, "Secret", nil)

	if res := h.do(http.MethodPatch, "/api/rooms/"+roomID, map[string]any{"is_private": true}, token); res.status != http.StatusOK {
		t.Fatalf("making the room private: status %d, body %s", res.status, res.body)
	}

	var rooms []map[string]any
	h.do(http.MethodGet, "/api/rooms", nil, "").decode(t, &rooms)
	if len(rooms) != 0 {
		t.Errorf("a private room appeared in the public list: %v", rooms)
	}

	direct := h.do(http.MethodGet, "/api/rooms/"+roomID, nil, "")
	if direct.status != http.StatusOK || direct.object(t)["is_private"] != true {
		t.Errorf("a private room should still be reachable by code: %d %s", direct.status, direct.body)
	}
}

func TestAutoRadioSettingSurvivesAReload(t *testing.T) {
	h := newHarness(t)
	token := h.register("Alice", "a@b.com")
	roomID, _ := h.createRoom(token, "R", nil)

	res := h.do(http.MethodPatch, "/api/rooms/"+roomID, map[string]any{"auto_radio": true}, token)
	if res.status != http.StatusOK || res.object(t)["auto_radio"] != true {
		t.Fatalf("enabling auto-radio: %d %s", res.status, res.body)
	}

	h.rooms.Forget(roomID)

	reloaded := h.do(http.MethodGet, "/api/rooms/"+roomID, nil, "")
	if reloaded.object(t)["auto_radio"] != true {
		t.Error("auto_radio did not survive the room being reloaded")
	}
}

func TestDeleteRoom(t *testing.T) {
	h := newHarness(t)
	owner := h.register("Owner", "owner@test.com")
	other := h.register("Other", "other@test.com")

	t.Run("another user cannot", func(t *testing.T) {
		roomID, _ := h.createRoom(owner, "D", nil)
		if res := h.do(http.MethodDelete, "/api/rooms/"+roomID, nil, other); res.status != http.StatusForbidden {
			t.Fatalf("status = %d, want 403; body %s", res.status, res.body)
		}
		if res := h.do(http.MethodGet, "/api/rooms/"+roomID, nil, ""); res.status != http.StatusOK {
			t.Error("the room should have survived")
		}
	})

	t.Run("anonymous cannot", func(t *testing.T) {
		roomID, _ := h.createRoom(owner, "D", nil)
		if res := h.do(http.MethodDelete, "/api/rooms/"+roomID, nil, ""); res.status != http.StatusUnauthorized {
			t.Errorf("status = %d, want 401", res.status)
		}
	})

	t.Run("the owner can", func(t *testing.T) {
		roomID, _ := h.createRoom(owner, "D", nil)
		res := h.do(http.MethodDelete, "/api/rooms/"+roomID, nil, owner)
		if res.status != http.StatusOK || res.object(t)["ok"] != true {
			t.Fatalf("status = %d, body %s", res.status, res.body)
		}
		if res := h.do(http.MethodGet, "/api/rooms/"+roomID, nil, ""); res.status != http.StatusNotFound {
			t.Errorf("the deleted room is still reachable: %d", res.status)
		}

		var rooms []map[string]any
		h.do(http.MethodGet, "/api/rooms", nil, "").decode(t, &rooms)
		for _, r := range rooms {
			if r["id"] == roomID {
				t.Error("the deleted room is still listed")
			}
		}
	})

	t.Run("unknown room", func(t *testing.T) {
		if res := h.do(http.MethodDelete, "/api/rooms/XXXXXX", nil, owner); res.status != http.StatusNotFound {
			t.Errorf("status = %d, want 404", res.status)
		}
	})
}

func TestJoinRoom(t *testing.T) {
	h := newHarness(t)
	token := h.register("Alice", "a@b.com")
	roomID, _ := h.createRoom(token, "J", nil)

	res := h.do(http.MethodPost, "/api/rooms/"+roomID+"/join", map[string]string{"username": "Bob"}, "")
	if res.status != http.StatusOK {
		t.Fatalf("status = %d, body %s", res.status, res.body)
	}
	if res.object(t)["username"] != "Bob" {
		t.Errorf("username = %v", res.object(t)["username"])
	}

	if missing := h.do(http.MethodPost, "/api/rooms/XXXXXX/join", map[string]string{"username": "X"}, ""); missing.status != http.StatusNotFound {
		t.Errorf("unknown room: status %d, want 404", missing.status)
	}
}

// --- Password-protected rooms ---

func TestPasswordProtectedRoomFlow(t *testing.T) {
	h := newHarness(t)
	token := h.register("Alice", "a@b.com")

	password := "hunter2"
	roomID, access := h.createRoom(token, "Locked", &password)
	if access == "" {
		t.Fatal("creating a locked room should hand the creator an access token")
	}

	// Drop the room from memory, so every step below exercises the reload path.
	h.rooms.Forget(roomID)

	t.Run("a stranger sees only the name", func(t *testing.T) {
		res := h.do(http.MethodGet, "/api/rooms/"+roomID, nil, "")
		if res.status != http.StatusOK {
			t.Fatalf("status = %d, body %s", res.status, res.body)
		}
		obj := res.object(t)
		if obj["locked"] != true || obj["has_password"] != true {
			t.Errorf("payload = %v, want it marked locked", obj)
		}
		for _, leaked := range []string{"queue", "owner_id", "messages"} {
			if _, present := obj[leaked]; present {
				t.Errorf("the locked payload leaked %q", leaked)
			}
		}
	})

	t.Run("the owner gets in without the password", func(t *testing.T) {
		res := h.do(http.MethodGet, "/api/rooms/"+roomID, nil, token)
		obj := res.object(t)
		if _, locked := obj["locked"]; locked {
			t.Fatalf("the owner was locked out: %v", obj)
		}
		if obj["access"] == "" {
			t.Error("the owner was not issued an access token")
		}
	})

	t.Run("the wrong password is refused", func(t *testing.T) {
		res := h.do(http.MethodPost, "/api/rooms/"+roomID+"/join",
			map[string]string{"username": "Bob", "password": "nope"}, "")
		if res.status != http.StatusForbidden {
			t.Fatalf("status = %d, want 403; body %s", res.status, res.body)
		}
		if res.object(t)["needs_password"] != true {
			t.Errorf("payload = %v, want needs_password", res.object(t))
		}
	})

	t.Run("a missing password is refused", func(t *testing.T) {
		res := h.do(http.MethodPost, "/api/rooms/"+roomID+"/join", map[string]string{"username": "Bob"}, "")
		if res.status != http.StatusForbidden {
			t.Errorf("status = %d, want 403", res.status)
		}
	})

	t.Run("the right password grants access", func(t *testing.T) {
		join := h.do(http.MethodPost, "/api/rooms/"+roomID+"/join",
			map[string]string{"username": "Bob", "password": "hunter2"}, "")
		if join.status != http.StatusOK {
			t.Fatalf("status = %d, body %s", join.status, join.body)
		}
		granted, _ := join.object(t)["access"].(string)
		if granted == "" {
			t.Fatal("no access token issued")
		}

		res := h.do(http.MethodGet, "/api/rooms/"+roomID+"?access="+granted, nil, "")
		obj := res.object(t)
		if _, locked := obj["locked"]; locked {
			t.Errorf("a valid access token did not unlock the room: %v", obj)
		}
		if queue, ok := obj["queue"].([]any); !ok || len(queue) != 0 {
			t.Errorf("queue = %v", obj["queue"])
		}
	})
}

func TestAccessTokenFromAnotherRoomIsRejected(t *testing.T) {
	h := newHarness(t)
	token := h.register("Alice", "a@b.com")
	password := "p"

	_, accessA := h.createRoom(token, "A", &password)
	roomB, _ := h.createRoom(token, "B", &password)

	res := h.do(http.MethodGet, "/api/rooms/"+roomB+"?access="+accessA, nil, "")
	if res.object(t)["locked"] != true {
		t.Error("one room's access token opened another")
	}
}

func TestSetAndRemoveRoomPassword(t *testing.T) {
	h := newHarness(t)
	token := h.register("Alice", "a@b.com")
	roomID, _ := h.createRoom(token, "S", nil)

	set := h.do(http.MethodPatch, "/api/rooms/"+roomID, map[string]any{"password": "secret"}, token)
	if set.object(t)["has_password"] != true {
		t.Fatalf("setting a password: %s", set.body)
	}

	// An unrelated settings change must not drop the password.
	unrelated := h.do(http.MethodPatch, "/api/rooms/"+roomID, map[string]any{"is_private": true}, token)
	if unrelated.object(t)["has_password"] != true {
		t.Error("an unrelated change dropped the room password")
	}

	removed := h.do(http.MethodPatch, "/api/rooms/"+roomID, map[string]any{"password": ""}, token)
	if removed.object(t)["has_password"] != false {
		t.Error("an empty password should remove the room password")
	}
}

func TestLockedRoomRefusesQueueAndPlayback(t *testing.T) {
	h := newHarness(t)
	token := h.register("Alice", "a@b.com")
	password := "hunter2"
	roomID, _ := h.createRoom(token, "Locked", &password)
	h.rooms.Forget(roomID)

	queue := h.do(http.MethodPost, "/api/rooms/"+roomID+"/queue", map[string]string{"url": "https://x"}, "")
	if queue.status != http.StatusForbidden {
		t.Errorf("queue: status %d, want 403; body %s", queue.status, queue.body)
	}
	if queue.object(t)["needs_password"] != true {
		t.Errorf("queue payload = %v, want needs_password", queue.object(t))
	}

	playback := h.do(http.MethodPost, "/api/rooms/"+roomID+"/playback", map[string]string{"action": "next"}, "")
	if playback.status != http.StatusForbidden {
		t.Errorf("playback: status %d, want 403; body %s", playback.status, playback.body)
	}
}

// --- Queue and playback ---

func TestAddToQueue(t *testing.T) {
	h := newHarness(t)
	token := h.register("Alice", "a@b.com")
	roomID, _ := h.createRoom(token, "Q", nil)

	res := h.do(http.MethodPost, "/api/rooms/"+roomID+"/queue",
		map[string]string{"url": "https://www.youtube.com/watch?v=dQw4w9WgXcQ"}, token)
	if res.status != http.StatusOK {
		t.Fatalf("status = %d, body %s", res.status, res.body)
	}
	if res.object(t)["added_by"] != "Alice" {
		t.Errorf("added_by = %v, want the signed-in username", res.object(t)["added_by"])
	}

	room := h.do(http.MethodGet, "/api/rooms/"+roomID, nil, "").object(t)
	if queue := room["queue"].([]any); len(queue) != 1 {
		t.Errorf("queue = %v, want one track", queue)
	}
	if room["is_playing"] != true {
		t.Error("adding the first track should start playback")
	}
}

func TestAddToQueueRejectsAFailedLookup(t *testing.T) {
	h := newHarness(t)
	token := h.register("Alice", "a@b.com")
	roomID, _ := h.createRoom(token, "Q", nil)
	h.yt.FetchTrackFn = func(context.Context, string) (youtube.TrackInfo, error) {
		return youtube.TrackInfo{}, errors.New("yt-dlp is unavailable")
	}

	res := h.do(http.MethodPost, "/api/rooms/"+roomID+"/queue", map[string]string{"url": "https://x"}, token)
	if res.status != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400; body %s", res.status, res.body)
	}
	if got := res.errorMessage(t); got != "Could not fetch video info" {
		t.Errorf("error = %q", got)
	}
}

func TestAddToQueueRequiresAURL(t *testing.T) {
	h := newHarness(t)
	token := h.register("Alice", "a@b.com")
	roomID, _ := h.createRoom(token, "Q", nil)

	if res := h.do(http.MethodPost, "/api/rooms/"+roomID+"/queue", map[string]string{"url": "  "}, token); res.status != http.StatusBadRequest {
		t.Errorf("status = %d, want 400; body %s", res.status, res.body)
	}
}

func TestPlayback(t *testing.T) {
	h := newHarness(t)
	token := h.register("Alice", "a@b.com")
	roomID, _ := h.createRoom(token, "P", nil)

	for i := 0; i < 3; i++ {
		if res := h.do(http.MethodPost, "/api/rooms/"+roomID+"/queue",
			map[string]string{"url": trackURLs[i]}, token); res.status != http.StatusOK {
			t.Fatalf("seeding the queue: %s", res.body)
		}
	}

	t.Run("jump", func(t *testing.T) {
		res := h.do(http.MethodPost, "/api/rooms/"+roomID+"/playback",
			map[string]any{"action": "jump", "index": 2}, "")
		obj := res.object(t)
		if obj["current_index"] != float64(2) {
			t.Errorf("current_index = %v, want 2", obj["current_index"])
		}
		// A playing room extrapolates its position from the last sync point, so
		// the answer is a hair past zero rather than exactly zero.
		if position := obj["position"].(float64); position < 0 || position > 1 {
			t.Errorf("position = %v, want the track restarted", position)
		}
	})

	t.Run("prev", func(t *testing.T) {
		res := h.do(http.MethodPost, "/api/rooms/"+roomID+"/playback", map[string]any{"action": "prev"}, "")
		if res.object(t)["current_index"] != float64(1) {
			t.Errorf("after prev: %v", res.object(t))
		}
	})

	t.Run("seek", func(t *testing.T) {
		res := h.do(http.MethodPost, "/api/rooms/"+roomID+"/playback",
			map[string]any{"action": "seek", "position": 42.5}, "")
		if got := res.object(t)["position"].(float64); got < 42.4 || got > 43.5 {
			t.Errorf("position = %v, want about 42.5", got)
		}
	})

	t.Run("next drops the finished track", func(t *testing.T) {
		before := len(h.do(http.MethodGet, "/api/rooms/"+roomID, nil, "").object(t)["queue"].([]any))
		res := h.do(http.MethodPost, "/api/rooms/"+roomID+"/playback", map[string]any{"action": "next"}, "")
		after := len(res.object(t)["queue"].([]any))
		if after != before-1 {
			t.Errorf("queue went from %d to %d, want one fewer", before, after)
		}
	})

	t.Run("unknown room", func(t *testing.T) {
		if res := h.do(http.MethodPost, "/api/rooms/XXXXXX/playback", map[string]any{"action": "next"}, ""); res.status != http.StatusNotFound {
			t.Errorf("status = %d, want 404", res.status)
		}
	})
}

// Advancing must reach the database, or a restart would replay a track the room
// has already finished.
func TestPlaybackPersistsAcrossAReload(t *testing.T) {
	h := newHarness(t)
	token := h.register("Alice", "a@b.com")
	roomID, _ := h.createRoom(token, "P", nil)

	for i := 0; i < 3; i++ {
		h.do(http.MethodPost, "/api/rooms/"+roomID+"/queue", map[string]string{"url": trackURLs[i]}, token)
	}
	h.do(http.MethodPost, "/api/rooms/"+roomID+"/playback", map[string]any{"action": "next"}, "")

	h.rooms.Forget(roomID)

	reloaded := h.do(http.MethodGet, "/api/rooms/"+roomID, nil, "").object(t)
	if queue := reloaded["queue"].([]any); len(queue) != 2 {
		t.Errorf("reloaded queue = %v, want 2 tracks", queue)
	}
}

// --- Lyrics ---

func TestLyricsEndpoint(t *testing.T) {
	h := newHarness(t)
	token := h.register("Alice", "a@b.com")
	roomID, _ := h.createRoom(token, "K", nil)

	t.Run("no track", func(t *testing.T) {
		res := h.do(http.MethodGet, "/api/rooms/"+roomID+"/lyrics", nil, "")
		if res.status != http.StatusOK {
			t.Fatalf("status = %d, body %s", res.status, res.body)
		}
		obj := res.object(t)
		if obj["available"] != false || obj["track_id"] != nil {
			t.Errorf("payload = %v", obj)
		}
		if cues, ok := obj["cues"].([]any); !ok || len(cues) != 0 {
			t.Errorf("cues = %v, want an empty array", obj["cues"])
		}
	})

	h.yt.SubtitlesFn = func(context.Context, string, string) (youtube.Subtitles, error) {
		return youtube.Subtitles{
			Lang: "en", Auto: true,
			Cues: []youtube.Cue{{Start: 1, Dur: 2, Text: "sing along"}},
		}, nil
	}
	h.do(http.MethodPost, "/api/rooms/"+roomID+"/queue", map[string]string{"url": trackURLs[0]}, token)

	t.Run("with captions", func(t *testing.T) {
		res := h.do(http.MethodGet, "/api/rooms/"+roomID+"/lyrics", nil, "")
		obj := res.object(t)
		if obj["available"] != true || obj["auto"] != true {
			t.Errorf("payload = %v", obj)
		}
		cues := obj["cues"].([]any)
		if len(cues) != 1 || cues[0].(map[string]any)["text"] != "sing along" {
			t.Errorf("cues = %v", cues)
		}
	})
}

// --- Search ---

func TestSearch(t *testing.T) {
	h := newHarness(t)
	h.yt.SearchFn = func(_ context.Context, query string, _ int) ([]youtube.SearchResult, error) {
		return []youtube.SearchResult{{ID: "abc", Title: "Found: " + query}}, nil
	}

	res := h.do(http.MethodPost, "/api/search", map[string]any{"query": "hello", "limit": 5}, "")
	if res.status != http.StatusOK {
		t.Fatalf("status = %d, body %s", res.status, res.body)
	}
	var results []map[string]any
	res.decode(t, &results)
	if len(results) != 1 || results[0]["title"] != "Found: hello" {
		t.Errorf("results = %v", results)
	}
}

func TestSearchWithAnEmptyQuery(t *testing.T) {
	h := newHarness(t)

	res := h.do(http.MethodPost, "/api/search", map[string]any{"query": "   "}, "")
	var results []map[string]any
	res.decode(t, &results)
	if len(results) != 0 {
		t.Errorf("results = %v, want none", results)
	}
	if h.yt.CallCount("Search") != 0 {
		t.Error("an empty query should not reach YouTube")
	}
}

// A search failure answers with an empty list rather than an error, so a
// transient yt-dlp problem shows as "nothing found" instead of a broken page.
func TestSearchAbsorbsAFailure(t *testing.T) {
	h := newHarness(t)
	h.yt.SearchFn = func(context.Context, string, int) ([]youtube.SearchResult, error) {
		return nil, errors.New("yt-dlp is unavailable")
	}

	res := h.do(http.MethodPost, "/api/search", map[string]any{"query": "hello"}, "")
	if res.status != http.StatusOK {
		t.Fatalf("status = %d, want 200; body %s", res.status, res.body)
	}
	var results []map[string]any
	res.decode(t, &results)
	if results == nil || len(results) != 0 {
		t.Errorf("results = %v, want an empty array", results)
	}
}

// --- Transport-level behaviour ---

func TestMalformedJSONIsRejected(t *testing.T) {
	h := newHarness(t)

	req, err := http.NewRequestWithContext(t.Context(), http.MethodPost,
		h.server.URL+"/api/auth/login", bytes.NewReader([]byte("{not json")))
	if err != nil {
		t.Fatalf("building request: %v", err)
	}
	req.Header.Set("Content-Type", "application/json")

	res, err := h.server.Client().Do(req)
	if err != nil {
		t.Fatalf("sending request: %v", err)
	}
	defer res.Body.Close()

	if res.StatusCode != http.StatusBadRequest {
		t.Errorf("status = %d, want 400", res.StatusCode)
	}
}

func TestHealthAndReadiness(t *testing.T) {
	h := newHarness(t)

	for path, want := range map[string]string{"/health": "ok", "/ready": "ready"} {
		res := h.do(http.MethodGet, path, nil, "")
		if res.status != http.StatusOK {
			t.Errorf("GET %s: status %d, body %s", path, res.status, res.body)
		}
		if got := res.object(t)["status"]; got != want {
			t.Errorf("GET %s: status field = %v, want %q", path, got, want)
		}
	}
}

func TestCORSAllowsTheConfiguredOrigin(t *testing.T) {
	h := newHarness(t)

	req, err := http.NewRequestWithContext(t.Context(), http.MethodOptions, h.server.URL+"/api/rooms", nil)
	if err != nil {
		t.Fatalf("building request: %v", err)
	}
	req.Header.Set("Origin", "http://localhost:3000")
	req.Header.Set("Access-Control-Request-Method", "POST")

	res, err := h.server.Client().Do(req)
	if err != nil {
		t.Fatalf("sending request: %v", err)
	}
	defer res.Body.Close()

	if got := res.Header.Get("Access-Control-Allow-Origin"); got != "http://localhost:3000" {
		t.Errorf("Access-Control-Allow-Origin = %q", got)
	}
	// A wildcard would stop the browser sending the Authorization header.
	if got := res.Header.Get("Access-Control-Allow-Credentials"); got != "true" {
		t.Errorf("Access-Control-Allow-Credentials = %q", got)
	}
}

func TestCORSRefusesAnUnknownOrigin(t *testing.T) {
	h := newHarness(t)

	req, err := http.NewRequestWithContext(t.Context(), http.MethodGet, h.server.URL+"/api/rooms", nil)
	if err != nil {
		t.Fatalf("building request: %v", err)
	}
	req.Header.Set("Origin", "https://evil.example")

	res, err := h.server.Client().Do(req)
	if err != nil {
		t.Fatalf("sending request: %v", err)
	}
	defer res.Body.Close()

	if got := res.Header.Get("Access-Control-Allow-Origin"); got != "" {
		t.Errorf("Access-Control-Allow-Origin = %q, want none for an unlisted origin", got)
	}
}

var trackURLs = []string{
	"https://www.youtube.com/watch?v=aaaaaaaaaaa",
	"https://www.youtube.com/watch?v=bbbbbbbbbbb",
	"https://www.youtube.com/watch?v=ccccccccccc",
}
