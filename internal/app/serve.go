package app

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"
	"time"

	"github.com/mosimosi228/kit/auth"
	kitlogger "github.com/mosimosi228/kit/logger"
	"github.com/mosimosi228/ovpn-dash/internal/geo"
	"github.com/mosimosi228/ovpn-dash/internal/httpserver"
	"github.com/mosimosi228/ovpn-dash/internal/settingsdb"
	"github.com/mosimosi228/ovpn-dash/internal/setup"
)

// ServeOptions controls the HTTP daemon.
type ServeOptions struct {
	Dir     string
	Listen  string
	BaseCtx context.Context
}

// Serve starts ovpn-dash (blocks until cancel or error).
func Serve(opts ServeOptions) error {
	if opts.Dir == "" {
		opts.Dir = os.Getenv("OVPNDASH_DIR")
	}
	if opts.Dir == "" {
		opts.Dir = "./ovpn-dash-data"
	}
	if opts.Listen == "" {
		opts.Listen = getenv("OVPNDASH_LISTEN", "127.0.0.1:7474")
	}
	if opts.BaseCtx == nil {
		ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
		defer stop()
		opts.BaseCtx = ctx
	}
	if err := os.MkdirAll(opts.Dir, 0o700); err != nil {
		return err
	}

	log := kitlogger.StartLogs(&kitlogger.Options{
		Level:          getenv("OVPNDASH_LOG_LEVEL", "info"),
		OutputDir:      filepath.Join(opts.Dir, "logs"),
		OutputFileName: "ovpn-dash.log",
	}, []kitlogger.LoggerHandler{kitlogger.ConsoleType, kitlogger.FileType})
	slog.SetDefault(log)

	tok, err := httpserver.EnsureSetupToken(opts.Dir)
	if err != nil {
		return err
	}
	hint := tok
	if len(hint) > 8 {
		hint = hint[:8] + "…"
	}
	log.Info("data dir ready", slog.String("dir", opts.Dir), slog.String("setup_token_hint", hint))

	db, err := settingsdb.Open(opts.Dir)
	if err != nil {
		return fmt.Errorf("settingsdb: %w", err)
	}
	defer db.Close()

	jwtSecret, err := loadOrCreateJWTSecret(opts.BaseCtx, db)
	if err != nil {
		return err
	}
	tokens, err := auth.New(auth.Config{Type: auth.TypeJWT, JWTSecret: jwtSecret})
	if err != nil {
		return fmt.Errorf("auth: %w", err)
	}

	h := &httpserver.Handler{
		Dir:    opts.Dir,
		DB:     db,
		Tokens: tokens,
		Log:    log,
		Geo:    geo.New(),
	}
	handler := h.Routes()

	srv := &http.Server{
		Addr:              opts.Listen,
		Handler:           handler,
		ReadHeaderTimeout: 10 * time.Second,
		IdleTimeout:       120 * time.Second,
	}

	errCh := make(chan error, 1)
	go func() {
		log.Info("ovpn-dash listening", slog.String("addr", opts.Listen))
		if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			errCh <- err
		}
		close(errCh)
	}()

	select {
	case <-opts.BaseCtx.Done():
	case err := <-errCh:
		if err != nil {
			return err
		}
	}

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	return srv.Shutdown(shutdownCtx)
}

func loadOrCreateJWTSecret(ctx context.Context, db *settingsdb.DB) (string, error) {
	if v, _ := db.GetMeta(ctx, setup.KeyJWTSecret); v != "" {
		return v, nil
	}
	raw := make([]byte, 32)
	if _, err := rand.Read(raw); err != nil {
		return "", err
	}
	sec := hex.EncodeToString(raw)
	if err := db.SetMeta(ctx, setup.KeyJWTSecret, sec); err != nil {
		return "", err
	}
	return sec, nil
}

func getenv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}
