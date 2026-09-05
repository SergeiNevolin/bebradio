package http

import (
	"encoding/json"
	"log/slog"
	"net/http"
	"strings"

	"github.com/bebradio/backend-go/internal/config"
	"github.com/bebradio/backend-go/internal/delivery/ws"
	"github.com/bebradio/backend-go/internal/usecase"
	"github.com/go-chi/chi/v5"
	chiMiddleware "github.com/go-chi/chi/v5/middleware"
)

type Server struct {
	Router   *chi.Mux
	config   *config.Config
	log      *slog.Logger
	auth     *usecase.AuthUsecase
	room     *usecase.RoomUsecase
	user     *usecase.UserUsecase
	search   *usecase.SearchUsecase
	media    *usecase.MediaUsecase
	playback *usecase.PlaybackUsecase
	manager  *ws.ConnectionManager
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
	manager *ws.ConnectionManager,
) *Server {
	s := &Server{
		Router:   chi.NewRouter(),
		config:   config,
		log:      log,
		auth:     auth,
		room:     room,
		user:     user,
		search:   search,
		media:    media,
		playback: playback,
		manager:  manager,
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
			r.Post("/{roomID}/visit", s.handleRecordVisit)
			r.Get("/{roomID}/lyrics", s.handleGetLyrics)
			r.Post("/{roomID}/playback", s.handlePlayback)
		})

		r.Post("/search", s.handleSearch)
		r.Get("/media/{trackID}", s.handleStream)

		r.Route("/users", func(r chi.Router) {
			r.Get("/me", s.handleGetMe)
			r.Put("/me", s.handleUpdateMe)
			r.Get("/{userID}", s.handleGetUser)
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
