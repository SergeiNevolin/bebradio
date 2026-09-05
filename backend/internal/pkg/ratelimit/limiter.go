package ratelimit

import (
	"net/http"
	"strings"
	"sync"
	"time"
)

type SlidingWindowLimiter struct {
	maxRequests  int
	windowSeconds int
	hits         map[string][]time.Time
	mu           sync.Mutex
}

func New(maxRequests, windowSeconds int) *SlidingWindowLimiter {
	return &SlidingWindowLimiter{
		maxRequests:  maxRequests,
		windowSeconds: windowSeconds,
		hits:         make(map[string][]time.Time),
	}
}

func (l *SlidingWindowLimiter) Allow(key string) bool {
	now := time.Now()
	l.mu.Lock()
	defer l.mu.Unlock()

	cutoff := now.Add(-time.Duration(l.windowSeconds) * time.Second)
	times := l.hits[key]
	filtered := make([]time.Time, 0)
	for _, t := range times {
		if t.After(cutoff) {
			filtered = append(filtered, t)
		}
	}
	l.hits[key] = filtered

	if len(filtered) >= l.maxRequests {
		return false
	}
	l.hits[key] = append(l.hits[key], now)
	return true
}

func (l *SlidingWindowLimiter) Remaining(key string) int {
	now := time.Now()
	l.mu.Lock()
	defer l.mu.Unlock()

	cutoff := now.Add(-time.Duration(l.windowSeconds) * time.Second)
	times := l.hits[key]
	count := 0
	for _, t := range times {
		if t.After(cutoff) {
			count++
		}
	}
	remaining := l.maxRequests - count
	if remaining < 0 {
		return 0
	}
	return remaining
}

func (l *SlidingWindowLimiter) Middleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ip := clientIP(r)
		if !l.Allow(ip) {
			w.Header().Set("Retry-After", "60")
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusTooManyRequests)
			w.Write([]byte(`{"error":"Too many requests","retry_after":60}`))
			return
		}
		next.ServeHTTP(w, r)
	})
}

func clientIP(r *http.Request) string {
	forwarded := r.Header.Get("X-Forwarded-For")
	if forwarded != "" {
		parts := strings.SplitN(forwarded, ",", 2)
		if len(parts) > 0 {
			return strings.TrimSpace(parts[0])
		}
	}
	host := r.RemoteAddr
	if idx := strings.LastIndex(host, ":"); idx != -1 {
		host = host[:idx]
	}
	return host
}
