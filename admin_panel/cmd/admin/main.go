package main

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"net/url"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/BAITC-Hacks/hack-1ad72463-apex-ai/admin_panel/internal/app"
	"github.com/jackc/pgx/v5/pgxpool"
)

func main() {
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
func env(k, d string) string {
	if v := os.Getenv(k); v != "" {
		return v
	}
	return d
}
func run() error {
	if len(os.Args) != 2 || (os.Args[1] != "init" && os.Args[1] != "serve") {
		return errors.New("usage: admin init|serve")
	}
	if os.Getenv("DATABASE_URL") == "" {
		return errors.New("DATABASE_URL is required")
	}
	cfg, err := pgxpool.ParseConfig(os.Getenv("DATABASE_URL"))
	if err != nil {
		return errors.New("invalid DATABASE_URL")
	}
	cfg.MaxConns = 5
	cfg.ConnConfig.ConnectTimeout = 3 * time.Second
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	pool, err := pgxpool.NewWithConfig(ctx, cfg)
	if err != nil {
		return errors.New("cannot initialize database pool")
	}
	defer pool.Close()
	if os.Args[1] == "init" {
		initCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
		defer cancel()
		password, created, err := app.Initialize(initCtx, pool, os.Getenv("ADMIN_INITIAL_PASSWORD"))
		if err != nil {
			return err
		}
		if created {
			fmt.Printf("Username: admin\nTemporary password: %s\nChange the password on first login.\n", password)
		} else {
			fmt.Println("Admin already exists; password unchanged.")
		}
		return nil
	}
	origin := env("ADMIN_ORIGIN", "https://testrr.shop")
	parsed, err := url.Parse(origin)
	if err != nil || parsed.Host == "" || parsed.Path != "" || parsed.RawQuery != "" || parsed.Fragment != "" || parsed.User != nil || (parsed.Scheme != "https" && parsed.Scheme != "http") {
		return errors.New("ADMIN_ORIGIN must be an origin without a path")
	}
	address := env("ADMIN_ADDR", "127.0.0.1:18081")
	host, _, err := net.SplitHostPort(address)
	if err != nil {
		return errors.New("invalid ADMIN_ADDR")
	}
	insecure := os.Getenv("ADMIN_INSECURE_HTTP") == "true"
	if insecure && (!net.ParseIP(host).IsLoopback() || !net.ParseIP(parsed.Hostname()).IsLoopback()) {
		return errors.New("insecure cookies are allowed only with loopback address and origin")
	}
	if !insecure && parsed.Scheme != "https" {
		return errors.New("HTTPS ADMIN_ORIGIN required unless local ADMIN_INSECURE_HTTP=true")
	}
	trusted := os.Getenv("ADMIN_TRUST_PROXY") == "true"
	if trusted && !net.ParseIP(host).IsLoopback() {
		return errors.New("trusted proxy requires loopback ADMIN_ADDR")
	}
	logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))
	handler := app.New(pool, app.Config{Origin: origin, SecureCookie: !insecure, TrustProxy: trusted}, logger)
	server := &http.Server{Addr: address, Handler: handler, ReadHeaderTimeout: 3 * time.Second, ReadTimeout: 20 * time.Second, WriteTimeout: 20 * time.Second, IdleTimeout: 60 * time.Second, MaxHeaderBytes: 16 * 1024}
	done := make(chan error, 1)
	go func() { logger.Info("admin listening", "address", address); done <- server.ListenAndServe() }()
	select {
	case err := <-done:
		if errors.Is(err, http.ErrServerClosed) {
			return nil
		}
		return err
	case <-ctx.Done():
		shutdown, cancel := context.WithTimeout(context.Background(), 15*time.Second)
		defer cancel()
		return server.Shutdown(shutdown)
	}
}
