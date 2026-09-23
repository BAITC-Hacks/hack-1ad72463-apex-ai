package main

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/BAITC-Hacks/hack-1ad72463-apex-ai/backend/internal/config"
	"github.com/BAITC-Hacks/hack-1ad72463-apex-ai/backend/internal/httpapi"
	"github.com/BAITC-Hacks/hack-1ad72463-apex-ai/backend/internal/postgres"
)

func main() {
	if err := run(); err != nil {
		slog.Error("server stopped", "error", err)
		os.Exit(1)
	}
}
func run() error {
	if len(os.Args) > 1 {
		if len(os.Args) != 2 || os.Args[1] != "healthcheck" {
			return fmt.Errorf("usage: server [healthcheck]")
		}
		client := &http.Client{Timeout: 3 * time.Second}
		resp, err := client.Get("http://127.0.0.1:8080/readyz")
		if err != nil {
			return err
		}
		defer resp.Body.Close()
		_, _ = io.Copy(io.Discard, resp.Body)
		if resp.StatusCode != 200 {
			return fmt.Errorf("not ready: HTTP %d", resp.StatusCode)
		}
		return nil
	}
	logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))
	cfg, err := config.Load()
	if err != nil {
		return err
	}
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	store, err := postgres.Open(ctx, cfg.DatabaseURL)
	if err != nil {
		return err
	}
	defer store.Close()
	srv := &http.Server{Addr: cfg.Address, Handler: httpapi.New(store, logger, cfg.Origins), ReadHeaderTimeout: 3 * time.Second, ReadTimeout: 10 * time.Second, WriteTimeout: 10 * time.Second, IdleTimeout: 60 * time.Second, MaxHeaderBytes: 16 * 1024}
	done := make(chan error, 1)
	go func() { logger.Info("listening", "address", cfg.Address); done <- srv.ListenAndServe() }()
	select {
	case err := <-done:
		if errors.Is(err, http.ErrServerClosed) {
			return nil
		}
		return err
	case <-ctx.Done():
		shutdown, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		return srv.Shutdown(shutdown)
	}
}
