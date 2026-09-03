// Command bebradio serves the listening-room backend: the HTTP API, the
// WebSocket that keeps every listener in a room in step, and the background
// loops that keep playback moving when the clients go quiet.
package main

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/leenzstra/bebradio/backend/internal/api"
	"github.com/leenzstra/bebradio/backend/internal/auth"
	"github.com/leenzstra/bebradio/backend/internal/config"
	"github.com/leenzstra/bebradio/backend/internal/hub"
	"github.com/leenzstra/bebradio/backend/internal/room"
	"github.com/leenzstra/bebradio/backend/internal/store/postgres"
	"github.com/leenzstra/bebradio/backend/internal/users"
	"github.com/leenzstra/bebradio/backend/internal/youtube"
)

// healthcheckFlag makes the binary probe its own health endpoint and exit with
// the result. It is what the container image's HEALTHCHECK runs, so the image
// needs no HTTP client of its own.
const healthcheckFlag = "-healthcheck"

func main() {
	if len(os.Args) > 1 && os.Args[1] == healthcheckFlag {
		if err := healthcheck(); err != nil {
			fmt.Fprintf(os.Stderr, "unhealthy: %v\n", err)
			os.Exit(1)
		}
		return
	}

	if err := run(); err != nil {
		// The logger may not exist yet when configuration fails, so this last
		// resort goes straight to stderr.
		fmt.Fprintf(os.Stderr, "fatal: %v\n", err)
		os.Exit(1)
	}
}

// healthcheck asks the running server whether it is serving.
//
// It probes liveness rather than readiness on purpose: an instance that has
// lost its database should be reported to the load balancer as not ready, but
// it should not be killed and restarted, because restarting will not bring the
// database back.
func healthcheck() error {
	addr := os.Getenv("ADDR")
	if addr == "" {
		addr = ":8000"
	}
	if strings.HasPrefix(addr, ":") {
		addr = "127.0.0.1" + addr
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, "http://"+addr+"/health", nil)
	if err != nil {
		return err
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("health endpoint returned %s", resp.Status)
	}
	return nil
}

func run() error {
	cfg, err := config.Load()
	if err != nil {
		return err
	}

	log := newLogger(cfg)
	slog.SetDefault(log)
	for _, warning := range cfg.Warnings {
		log.Warn(warning)
	}

	// Signals are trapped before anything is opened, so a Ctrl-C during a slow
	// database connect still shuts down cleanly.
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	db, err := postgres.Open(ctx, postgres.Options{
		DatabaseURL:    cfg.DatabaseURL,
		MaxConns:       cfg.DBMaxConns,
		MinConns:       cfg.DBMinConns,
		ConnectTimeout: cfg.DBConnectTimeout,
	})
	if err != nil {
		return err
	}
	defer db.Close()

	if err := db.Migrate(ctx); err != nil {
		return err
	}
	log.Info("database ready")

	tokens := auth.NewTokens(cfg.SecretKey, cfg.JWTExpiry)
	passwords := auth.NewPasswords(cfg.BcryptCost)

	connections := hub.New(log)
	yt := youtube.New(youtube.Options{
		BinaryPath:       cfg.YTDLPPath,
		JSRuntime:        cfg.YTDLPJSRuntime,
		Timeout:          cfg.YTDLPTimeout,
		Concurrency:      cfg.YTDLPConcurrency,
		SubtitleCacheMax: cfg.SubtitleCacheMax,
		Logger:           log,
	})

	rooms := room.New(room.Deps{
		Store:     db,
		Hub:       connections,
		YouTube:   yt,
		Tokens:    tokens,
		Passwords: passwords,
		Config:    cfg,
		Logger:    log,
	})
	defer rooms.Shutdown()

	server := api.NewServer(api.Deps{
		Rooms:   rooms,
		Users:   users.New(db, tokens, passwords),
		Hub:     connections,
		YouTube: yt,
		Store:   db,
		Config:  cfg,
		Logger:  log,
	})

	httpServer := &http.Server{
		Addr:              cfg.Addr,
		Handler:           server.Routes(),
		ReadHeaderTimeout: 10 * time.Second,
		ReadTimeout:       cfg.ReadTimeout,
		// WriteTimeout is left unset: it would apply to an upgraded WebSocket
		// too, cutting off every listener after one timeout's worth of
		// listening. The hub sets its own deadline on each write instead.
		IdleTimeout: cfg.IdleTimeout,
		ErrorLog:    slog.NewLogLogger(log.Handler(), slog.LevelWarn),
	}

	var wg sync.WaitGroup

	// The maintenance loop stops before the HTTP server does, so it cannot
	// touch a room while connections are being drained.
	maintenanceCtx, stopMaintenance := context.WithCancel(context.Background())
	wg.Add(1)
	go func() {
		defer wg.Done()
		rooms.Run(maintenanceCtx)
	}()

	serverErr := make(chan error, 1)
	go func() {
		log.Info("listening", "addr", cfg.Addr)
		if err := httpServer.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			serverErr <- err
		}
		close(serverErr)
	}()

	select {
	case err := <-serverErr:
		stopMaintenance()
		wg.Wait()
		if err != nil {
			return fmt.Errorf("http server: %w", err)
		}
		return nil
	case <-ctx.Done():
		log.Info("shutdown signal received", "timeout", cfg.ShutdownTimeout)
	}

	shutdownCtx, cancel := context.WithTimeout(context.Background(), cfg.ShutdownTimeout)
	defer cancel()

	stopMaintenance()
	wg.Wait()

	if err := httpServer.Shutdown(shutdownCtx); err != nil {
		// Shutdown returns the deadline error when connections are still open.
		// WebSocket connections are long-lived by design and will not close on
		// their own, so this is expected rather than a fault.
		log.Warn("closing connections timed out; forcing shutdown", "error", err)
		if err := httpServer.Close(); err != nil {
			log.Error("forcing shutdown", "error", err)
		}
	}

	log.Info("stopped")
	return nil
}

func newLogger(cfg config.Config) *slog.Logger {
	opts := &slog.HandlerOptions{Level: cfg.LogLevel}
	if cfg.LogFormat == "text" {
		return slog.New(slog.NewTextHandler(os.Stdout, opts))
	}
	return slog.New(slog.NewJSONHandler(os.Stdout, opts))
}
