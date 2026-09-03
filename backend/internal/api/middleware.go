package api

import (
	"context"
	"log/slog"
	"net/http"
	"slices"
	"strings"
	"time"

	"github.com/go-chi/chi/v5/middleware"

	"github.com/leenzstra/bebradio/backend/internal/domain"
	"github.com/leenzstra/bebradio/backend/internal/room"
)

type contextKey int

const (
	userContextKey contextKey = iota
	loggerContextKey
)

// corsMiddleware answers cross-origin requests for the configured origins.
//
// The allowlist is echoed back one origin at a time rather than as a wildcard,
// because the browser refuses to send credentials to a wildcard origin -- and
// the client sends an Authorization header on most requests.
func corsMiddleware(origins []string) func(http.Handler) http.Handler {
	allowAll := slices.Contains(origins, "*")

	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			origin := r.Header.Get("Origin")
			allowed := origin != "" && (allowAll || slices.Contains(origins, origin))

			if allowed {
				h := w.Header()
				h.Set("Access-Control-Allow-Origin", origin)
				h.Set("Access-Control-Allow-Credentials", "true")
				// The response varies by origin, so a shared cache must not
				// serve one origin's response to another.
				h.Add("Vary", "Origin")
			}

			if r.Method == http.MethodOptions && r.Header.Get("Access-Control-Request-Method") != "" {
				if allowed {
					h := w.Header()
					h.Set("Access-Control-Allow-Methods", "GET, POST, PUT, PATCH, DELETE, OPTIONS")
					h.Set("Access-Control-Allow-Headers", "Authorization, Content-Type")
					h.Set("Access-Control-Max-Age", "600")
				}
				w.WriteHeader(http.StatusNoContent)
				return
			}

			next.ServeHTTP(w, r)
		})
	}
}

// requestLogger records one line per request, with the request id so a client
// report can be traced to a server log entry.
func requestLogger(log *slog.Logger) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			entry := log.With("request_id", middleware.GetReqID(r.Context()))
			ctx := context.WithValue(r.Context(), loggerContextKey, entry)

			ww := middleware.NewWrapResponseWriter(w, r.ProtoMajor)
			started := time.Now()
			next.ServeHTTP(ww, r.WithContext(ctx))

			level := slog.LevelInfo
			if ww.Status() >= http.StatusInternalServerError {
				level = slog.LevelError
			}
			entry.Log(r.Context(), level, "request",
				"method", r.Method,
				"path", r.URL.Path,
				"status", ww.Status(),
				"bytes", ww.BytesWritten(),
				"duration_ms", time.Since(started).Milliseconds(),
			)
		})
	}
}

// recoverer turns a panic in a handler into a 500 rather than a dropped
// connection, and logs the stack.
func recoverer(log *slog.Logger) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			defer func() {
				rec := recover()
				if rec == nil {
					return
				}
				// A client that goes away mid-response makes net/http panic
				// with this sentinel; it is not a fault worth logging.
				if rec == http.ErrAbortHandler {
					panic(rec)
				}
				log.Error("handler panicked",
					"method", r.Method, "path", r.URL.Path, "panic", rec)
				writeError(w, log, http.StatusInternalServerError, "Internal server error")
			}()
			next.ServeHTTP(w, r)
		})
	}
}

// limitBody caps how much of a request body will be read.
func limitBody(max int64) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			r.Body = http.MaxBytesReader(w, r.Body, max)
			next.ServeHTTP(w, r)
		})
	}
}

// authenticate resolves a bearer token, if one is present, and puts the account
// on the request context.
//
// It never rejects a request: many endpoints serve anonymous callers, and the
// ones that do not use requireUser. A token that is expired or unknown is
// treated exactly like no token at all.
func (s *Server) authenticate(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		token := bearerToken(r)
		if token == "" {
			next.ServeHTTP(w, r)
			return
		}
		user, err := s.users.FromToken(r.Context(), token)
		if err != nil {
			next.ServeHTTP(w, r)
			return
		}
		next.ServeHTTP(w, r.WithContext(context.WithValue(r.Context(), userContextKey, &user)))
	})
}

// requireUser rejects a request that is not signed in.
func (s *Server) requireUser(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if currentUser(r) == nil {
			writeError(w, s.logger(r), http.StatusUnauthorized, "Not authenticated")
			return
		}
		next.ServeHTTP(w, r)
	})
}

func bearerToken(r *http.Request) string {
	header := r.Header.Get("Authorization")
	if header == "" {
		return ""
	}
	scheme, token, found := strings.Cut(header, " ")
	if !found || !strings.EqualFold(scheme, "Bearer") {
		return ""
	}
	return strings.TrimSpace(token)
}

// currentUser returns the signed-in account, or nil for an anonymous request.
func currentUser(r *http.Request) *domain.User {
	user, _ := r.Context().Value(userContextKey).(*domain.User)
	return user
}

// callerFrom builds the identity the room service works with: who is signed in,
// plus any room-access token from the query string.
func callerFrom(r *http.Request) room.Caller {
	caller := room.Caller{AccessToken: r.URL.Query().Get("access")}
	if user := currentUser(r); user != nil {
		caller.UserID = user.ID
		caller.Username = user.Username
	}
	return caller
}

// logger returns the request-scoped logger, falling back to the server's.
func (s *Server) logger(r *http.Request) *slog.Logger {
	if log, ok := r.Context().Value(loggerContextKey).(*slog.Logger); ok {
		return log
	}
	return s.log
}
