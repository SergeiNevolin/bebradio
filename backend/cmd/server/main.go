package main

import (
	"context"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/bebradio/backend-go/internal/config"
	httpDeliv "github.com/bebradio/backend-go/internal/delivery/http"
	"github.com/bebradio/backend-go/internal/delivery/ws"
	"github.com/bebradio/backend-go/internal/domain/repository"
	"github.com/bebradio/backend-go/internal/infrastructure/auth"
	"github.com/bebradio/backend-go/internal/infrastructure/media"
	"github.com/bebradio/backend-go/internal/infrastructure/postgres"
	"github.com/bebradio/backend-go/internal/infrastructure/worker"
	"github.com/bebradio/backend-go/internal/usecase"
	"github.com/go-chi/chi/v5"
	"github.com/gorilla/websocket"
)

func main() {
	log := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{
		Level: slog.LevelInfo,
	}))
	slog.SetDefault(log)

	cfg := config.Load()
	log.Info("starting bebradio backend", "port", cfg.Port)

	db, err := postgres.New(cfg.DatabaseURL, log)
	if err != nil {
		log.Error("failed to connect to database", "error", err)
		os.Exit(1)
	}
	defer db.Close()

	if err := db.Migrate(); err != nil {
		log.Error("failed to run migrations", "error", err)
		os.Exit(1)
	}

	authService := auth.New(cfg.SecretKey, cfg.JWTExpireHours)
	mediaSvc := media.NewClient(cfg.MediaServiceURL)

	userRepo := postgres.NewUserRepo(db.Pool)
	roomRepo := postgres.NewRoomRepo(db.Pool)

	var mediaClient repository.MediaClient = mediaSvc

	authUC := usecase.NewAuthUsecase(userRepo, authService, log)
	roomUC := usecase.NewRoomUsecase(roomRepo, userRepo, mediaClient, authService, log)
	userUC := usecase.NewUserUsecase(userRepo, log)
	searchUC := usecase.NewSearchUsecase(mediaClient, cfg, log)
	mediaUC := usecase.NewMediaUsecase(mediaClient, cfg, log)
	playbackUC := usecase.NewPlaybackUsecase()
	chatUC := usecase.NewChatUsecase(roomRepo, log)
	radioUC := usecase.NewRadioUsecase(mediaClient, cfg, log)

	connManager := ws.NewConnectionManager(log)
	wsHandler := ws.NewHandler(connManager, roomUC, playbackUC, chatUC, radioUC, mediaUC, cfg, log)

	httpServer := httpDeliv.NewServer(cfg, log, authUC, roomUC, userUC, searchUC, mediaUC, playbackUC, connManager)

	// Add WebSocket endpoint to the HTTP server's router
	upgrader := websocket.Upgrader{
		CheckOrigin: func(r *http.Request) bool { return true },
	}
	httpServer.Router.Get("/ws/{roomID}", func(w http.ResponseWriter, r *http.Request) {
		roomID := chi.URLParam(r, "roomID")
		access := r.URL.Query().Get("access")

		conn, err := upgrader.Upgrade(w, r, nil)
		if err != nil {
			log.Error("websocket upgrade failed", "error", err)
			return
		}

		wsHandler.HandleWebSocket(conn, roomID, access)
	})

	// Start background workers
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	autoAdvance := worker.NewAutoAdvance(roomUC, playbackUC, mediaUC, radioUC, connManager, cfg, log)
	go autoAdvance.Run(ctx)

	// HTTP server
	srv := &http.Server{
		Addr:         ":" + cfg.Port,
		Handler:      httpServer.Router,
		ReadTimeout:  15 * time.Second,
		WriteTimeout: 15 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	// Graceful shutdown
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)

	go func() {
		log.Info("server listening", "port", cfg.Port)
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Error("server error", "error", err)
			os.Exit(1)
		}
	}()

	<-sigCh
	log.Info("shutting down...")

	cancel()

	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer shutdownCancel()

	if err := srv.Shutdown(shutdownCtx); err != nil {
		log.Error("shutdown error", "error", err)
	}

	log.Info("server stopped")
}
