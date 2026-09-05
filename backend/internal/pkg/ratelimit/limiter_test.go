package ratelimit

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestAllowWithinLimit(t *testing.T) {
	lim := New(5, 60)

	for i := 0; i < 5; i++ {
		if !lim.Allow("key1") {
			t.Errorf("expected allow on request %d", i+1)
		}
	}
}

func TestDenyOverLimit(t *testing.T) {
	lim := New(3, 60)

	lim.Allow("key1")
	lim.Allow("key1")
	lim.Allow("key1")

	if lim.Allow("key1") {
		t.Error("expected deny after exceeding limit")
	}
}

func TestDifferentKeysIndependent(t *testing.T) {
	lim := New(2, 60)

	lim.Allow("key1")
	lim.Allow("key1")

	if !lim.Allow("key2") {
		t.Error("expected allow for different key")
	}
}

func TestRemaining(t *testing.T) {
	lim := New(5, 60)

	if lim.Remaining("key1") != 5 {
		t.Errorf("expected 5 remaining, got %d", lim.Remaining("key1"))
	}

	lim.Allow("key1")
	lim.Allow("key1")

	if lim.Remaining("key1") != 3 {
		t.Errorf("expected 3 remaining, got %d", lim.Remaining("key1"))
	}
}

func TestWindowExpiry(t *testing.T) {
	lim := New(2, 1) // 2 requests per 1 second

	lim.Allow("key1")
	lim.Allow("key1")

	if lim.Allow("key1") {
		t.Error("expected deny within window")
	}

	// Wait for window to expire
	time.Sleep(1100 * time.Millisecond)

	if !lim.Allow("key1") {
		t.Error("expected allow after window expiry")
	}
}

func TestMiddlewareAllows(t *testing.T) {
	lim := New(10, 60)
	handler := lim.Middleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(200)
	}))

	req := httptest.NewRequest("GET", "/", nil)
	req.RemoteAddr = "192.168.1.1:1234"
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	if w.Code != 200 {
		t.Errorf("expected 200, got %d", w.Code)
	}
}

func TestMiddlewareDenies(t *testing.T) {
	lim := New(1, 60)
	handler := lim.Middleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(200)
	}))

	req1 := httptest.NewRequest("GET", "/", nil)
	req1.RemoteAddr = "192.168.1.1:1234"
	w1 := httptest.NewRecorder()
	handler.ServeHTTP(w1, req1)

	req2 := httptest.NewRequest("GET", "/", nil)
	req2.RemoteAddr = "192.168.1.1:1234"
	w2 := httptest.NewRecorder()
	handler.ServeHTTP(w2, req2)

	if w2.Code != 429 {
		t.Errorf("expected 429, got %d", w2.Code)
	}
}

func TestMiddlewareDifferentIPs(t *testing.T) {
	lim := New(1, 60)
	handler := lim.Middleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(200)
	}))

	req1 := httptest.NewRequest("GET", "/", nil)
	req1.RemoteAddr = "192.168.1.1:1234"
	w1 := httptest.NewRecorder()
	handler.ServeHTTP(w1, req1)

	req2 := httptest.NewRequest("GET", "/", nil)
	req2.RemoteAddr = "192.168.1.2:1234"
	w2 := httptest.NewRecorder()
	handler.ServeHTTP(w2, req2)

	if w2.Code != 200 {
		t.Errorf("expected 200 for different IP, got %d", w2.Code)
	}
}

func TestMiddlewareXForwardedFor(t *testing.T) {
	lim := New(1, 60)
	handler := lim.Middleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(200)
	}))

	req1 := httptest.NewRequest("GET", "/", nil)
	req1.RemoteAddr = "127.0.0.1:1234"
	req1.Header.Set("X-Forwarded-For", "10.0.0.1, 10.0.0.2")
	w1 := httptest.NewRecorder()
	handler.ServeHTTP(w1, req1)

	req2 := httptest.NewRequest("GET", "/", nil)
	req2.RemoteAddr = "127.0.0.1:5678"
	req2.Header.Set("X-Forwarded-For", "10.0.0.1, 10.0.0.3")
	w2 := httptest.NewRecorder()
	handler.ServeHTTP(w2, req2)

	if w2.Code != 429 {
		t.Errorf("expected 429 for same forwarded IP, got %d", w2.Code)
	}
}

func TestClientIPDirect(t *testing.T) {
	req := httptest.NewRequest("GET", "/", nil)
	req.RemoteAddr = "192.168.1.100:8080"

	ip := clientIP(req)
	if ip != "192.168.1.100" {
		t.Errorf("expected '192.168.1.100', got '%s'", ip)
	}
}

func TestClientIPForwarded(t *testing.T) {
	req := httptest.NewRequest("GET", "/", nil)
	req.RemoteAddr = "127.0.0.1:8080"
	req.Header.Set("X-Forwarded-For", "10.0.0.1, 10.0.0.2")

	ip := clientIP(req)
	if ip != "10.0.0.1" {
		t.Errorf("expected '10.0.0.1', got '%s'", ip)
	}
}
