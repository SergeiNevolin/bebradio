// Package api exposes the service over HTTP and WebSocket.
//
// Handlers here do three things and no more: read the request, call a service,
// and render the result. Every decision about who may do what lives in package
// room or package users, so the same rules apply whether a listener arrives
// over HTTP or over a WebSocket.
package api

import (
	"log/slog"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/gorilla/websocket"

	"github.com/leenzstra/bebradio/backend/internal/config"
	"github.com/leenzstra/bebradio/backend/internal/hub"
	"github.com/leenzstra/bebradio/backend/internal/room"
	"github.com/leenzstra/bebradio/backend/internal/store"
	"github.com/leenzstra/bebradio/backend/internal/users"
	"github.com/leenzstra/bebradio/backend/internal/youtube"
)

// Server holds everything the HTTP handlers need.
type Server struct {
	rooms    *room.Service
	users    *users.Service
	hub      *hub.Hub
	yt       youtube.Client
	store    store.Store
	cfg      config.Config
	log      *slog.Logger
	upgrader websocket.Upgrader
}

// Deps are the collaborators a Server needs.
type Deps struct {
	Rooms   *room.Service
	Users   *users.Service
	Hub     *hub.Hub
	YouTube youtube.Client
	Store   store.Store
	Config  config.Config
	Logger  *slog.Logger
}

// NewServer wires the handlers together.
func NewServer(deps Deps) *Server {
	log := deps.Logger
	if log == nil {
		log = slog.Default()
	}
	return &Server{
		rooms: deps.Rooms,
		users: deps.Users,
		hub:   deps.Hub,
		yt:    deps.YouTube,
		store: deps.Store,
		cfg:   deps.Config,
		log:   log,
		upgrader: websocket.Upgrader{
			HandshakeTimeout: 10 * 1e9,
			ReadBufferSize:   4096,
			WriteBufferSize:  4096,
			// The WebSocket carries no cookie-based authority: a listener
			// proves who they are with tokens in the URL, which a cross-site
			// page cannot obtain. Origin is therefore not a security boundary
			// here, and enforcing it would break the reverse-proxy setups the
			// service is deployed behind.
			CheckOrigin: func(*http.Request) bool { return true },
		},
	}
}

// Routes returns the fully-wired HTTP handler.
func (s *Server) Routes() http.Handler {
	r := chi.NewRouter()

	r.Use(middleware.RequestID)
	r.Use(middleware.RealIP)
	r.Use(recoverer(s.log))
	r.Use(requestLogger(s.log))
	r.Use(corsMiddleware(s.cfg.CORSOrigins))

	// Liveness: the process is up and serving.
	r.Get("/health", s.handleHealth)
	// Readiness: the process can also reach its database. A load balancer
	// should drain an instance that fails this while leaving it running.
	r.Get("/ready", s.handleReady)

	// The WebSocket route is registered outside the API group: an upgraded
	// connection lives for hours, so it must not inherit the body limit or any
	// request timeout meant for short HTTP calls.
	r.Get("/ws/{roomID}", s.handleWebSocket)

	r.Route("/api", func(r chi.Router) {
		r.Use(limitBody(s.cfg.MaxRequestBytes))
		r.Use(s.authenticate)

		r.Route("/auth", func(r chi.Router) {
			r.Post("/register", s.handleRegister)
			r.Post("/login", s.handleLogin)
			r.Get("/me", s.handleAuthMe)
		})

		r.Route("/users", func(r chi.Router) {
			r.With(s.requireUser).Get("/me", s.handleGetOwnProfile)
			r.With(s.requireUser).Put("/me", s.handleUpdateOwnProfile)
			r.Get("/{userID}", s.handleGetProfile)
		})

		r.Post("/search", s.handleSearch)

		r.Route("/rooms", func(r chi.Router) {
			r.Get("/", s.handleListRooms)
			r.With(s.requireUser).Post("/", s.handleCreateRoom)

			r.Route("/{roomID}", func(r chi.Router) {
				r.Get("/", s.handleGetRoom)
				r.With(s.requireUser).Patch("/", s.handleUpdateRoom)
				r.With(s.requireUser).Delete("/", s.handleDeleteRoom)
				r.Post("/join", s.handleJoinRoom)
				r.Post("/queue", s.handleAddToQueue)
				r.Post("/playback", s.handlePlayback)
				r.Get("/lyrics", s.handleLyrics)
			})
		})
	})

	return r
}

func (s *Server) handleHealth(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, s.logger(r), http.StatusOK, map[string]string{"status": "ok"})
}

func (s *Server) handleReady(w http.ResponseWriter, r *http.Request) {
	if err := s.store.Ping(r.Context()); err != nil {
		s.logger(r).Error("readiness check failed", "error", err)
		writeJSON(w, s.logger(r), http.StatusServiceUnavailable,
			map[string]string{"status": "unavailable", "reason": "database unreachable"})
		return
	}
	writeJSON(w, s.logger(r), http.StatusOK, map[string]string{"status": "ready"})
}
