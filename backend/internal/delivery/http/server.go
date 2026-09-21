package http

import (
	"encoding/json"
	"log/slog"
	"net/http"
	"strings"
	"sync"

	"github.com/bebradio/backend-go/internal/config"
	"github.com/bebradio/backend-go/internal/delivery/ws"
	"github.com/bebradio/backend-go/internal/pkg/ratelimit"
	"github.com/bebradio/backend-go/internal/usecase"
	"github.com/go-chi/chi/v5"
	chiMiddleware "github.com/go-chi/chi/v5/middleware"
	"github.com/redis/go-redis/v9"
)

type Server struct {
	Router        *chi.Mux
	config        *config.Config
	log           *slog.Logger
	auth          *usecase.AuthUsecase
	room          *usecase.RoomUsecase
	user          *usecase.UserUsecase
	search        *usecase.SearchUsecase
	media         *usecase.MediaUsecase
	playback      *usecase.PlaybackUsecase
	tracks        *usecase.TrackUsecase
	manager       *ws.ConnectionManager
	rdb           *redis.Client
	uploadLimiter *ratelimit.SlidingWindowLimiter
	// addMu serializes the check-and-append critical section of handleAddToQueue
	// (duplicate check + RPush must be atomic, otherwise a double-click adds
	// the same track twice and the "extra" copy later plays as if resurrected).
	// Held only around fast Redis ops, never across network I/O.
	addMu sync.Mutex
}

func NewServer(
	config *config.Config,
	log *slog.Logger,
	auth *usecase.AuthUsecase,
	room *usecase.RoomUsecase,
	user *usecase.UserUsecase,
	search *usecase.SearchUsecase,
	media *usecase.MediaUsecase,
	playback *usecase.PlaybackUsecase,
	tracks *usecase.TrackUsecase,
	manager *ws.ConnectionManager,
	rdb *redis.Client,
) *Server {
	uploadLimit := config.RateLimitUpload
	if uploadLimit <= 0 {
		uploadLimit = 5
	}
	s := &Server{
		Router:        chi.NewRouter(),
		config:        config,
		log:           log,
		auth:          auth,
		room:          room,
		user:          user,
		search:        search,
		media:         media,
		playback:      playback,
		tracks:        tracks,
		manager:       manager,
		rdb:           rdb,
		uploadLimiter: ratelimit.New(uploadLimit, 3600),
	}
	s.setupRoutes()
	return s
}

func (s *Server) setupRoutes() {
	s.Router.Use(chiMiddleware.Logger)
	s.Router.Use(chiMiddleware.Recoverer)
	s.Router.Use(s.corsMiddleware)

	s.Router.Route("/api", func(r chi.Router) {
		r.Route("/auth", func(r chi.Router) {
			r.Post("/register", s.handleRegister)
			r.Post("/login", s.handleLogin)
			r.Get("/me", s.handleMe)
		})

		r.Route("/rooms", func(r chi.Router) {
			r.Post("/", s.handleCreateRoom)
			r.Get("/", s.handleListRooms)
			r.Get("/recent", s.handleRecentRooms)
			r.Get("/{roomID}", s.handleGetRoom)
			r.Patch("/{roomID}", s.handleUpdateRoom)
			r.Delete("/{roomID}", s.handleDeleteRoom)
			r.Post("/{roomID}/join", s.handleJoinRoom)
			r.Post("/{roomID}/queue", s.handleAddToQueue)
			r.Post("/{roomID}/queue/{trackID}/import", s.handleImportQueueTrack)
			r.Post("/{roomID}/visit", s.handleRecordVisit)
			r.Get("/{roomID}/lyrics", s.handleGetLyrics)
		})

		r.Post("/search", s.handleSearch)
		r.Get("/music/{trackID}", s.handleStream)

		r.Route("/tracks", func(r chi.Router) {
			r.Get("/", s.handleListTracks)                      // ?q=&sort=recent|top&limit=&offset=  (optional auth -> liked)
			r.Post("/", s.handleUploadTrack)                    // auth, multipart: file (+ optional cover)
			r.Get("/mine", s.handleMyTracks)                    // auth
			r.Get("/liked", s.handleLikedTracks)                // auth
			r.Get("/{trackID}", s.handleGetTrack)               // optional auth -> liked
			r.Get("/{trackID}/audio", s.handleStreamTrack)      // streams music-service, Range-aware
			r.Get("/{trackID}/cover", s.handleStreamTrackCover) // cover image
			r.Delete("/{trackID}", s.handleDeleteTrack)         // auth + owner
			r.Post("/{trackID}/like", s.handleLikeTrack)        // auth
			r.Delete("/{trackID}/like", s.handleUnlikeTrack)    // auth
			r.Put("/{trackID}/cover", s.handleUploadTrackCover) // auth + owner, multipart: file
			r.Patch("/{trackID}", s.handleUpdateTrack)          // auth + owner, JSON: {title, artist}
		})

		r.Route("/users", func(r chi.Router) {
			r.Get("/me", s.handleGetMe)
			r.Put("/me", s.handleUpdateMe)
			r.Get("/{userID}", s.handleGetUser)
		})

		r.Route("/admin", func(r chi.Router) {
			r.Post("/promote", s.handleAdminPromote)
			r.Post("/demote", s.handleAdminDemote)
			r.Get("/users", s.handleAdminSearchUsers)
		})
	})
}

func (s *Server) Handler() http.Handler {
	return s.Router
}

func (s *Server) writeJSON(w http.ResponseWriter, status int, data any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(data)
}

func (s *Server) writeError(w http.ResponseWriter, status int, message string) {
	s.writeJSON(w, status, map[string]string{"error": message})
}

func (s *Server) getUserID(r *http.Request) string {
	auth := r.Header.Get("Authorization")
	if auth == "" {
		return ""
	}
	token := strings.TrimPrefix(auth, "Bearer ")
	userID, err := s.auth.DecodeToken(token)
	if err != nil {
		return ""
	}
	return userID
}

func (s *Server) getUserOptional(r *http.Request) string {
	return s.getUserID(r)
}

func (s *Server) getUserRequired(r *http.Request) (string, bool) {
	id := s.getUserID(r)
	if id == "" {
		return "", false
	}
	return id, true
}

func (s *Server) isAdmin(userID string) (bool, error) {
	u, err := s.user.GetUser(userID)
	if err != nil {
		return false, err
	}
	return u.IsAdmin(), nil
}

func (s *Server) corsMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		origin := r.Header.Get("Origin")
		allowed := false
		for _, o := range s.config.CORSOrigins {
			if o == origin || o == "*" {
				allowed = true
				break
			}
		}
		if allowed {
			w.Header().Set("Access-Control-Allow-Origin", origin)
		}
		w.Header().Set("Access-Control-Allow-Credentials", "true")
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, PATCH, DELETE, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization")

		if r.Method == "OPTIONS" {
			w.WriteHeader(204)
			return
		}
		next.ServeHTTP(w, r)
	})
}
