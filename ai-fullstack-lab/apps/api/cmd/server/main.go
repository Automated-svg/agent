package main

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"example.com/ai-fullstack-lab/api/internal/tasks"
)

func env(key, fallback string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return fallback
}
func main() {
	store, err := tasks.Open(env("DATA_FILE", ".var/tasks.json"))
	if err != nil {
		slog.Error("open store", "error", err)
		os.Exit(1)
	}
	handler, err := tasks.NewHandler(store)
	if err != nil {
		slog.Error("create handler", "error", err)
		os.Exit(1)
	}
	server := &http.Server{Addr: env("ADDR", "127.0.0.1:8080"), Handler: handler, ReadHeaderTimeout: 5 * time.Second, ReadTimeout: 10 * time.Second, WriteTimeout: 15 * time.Second, IdleTimeout: 60 * time.Second}
	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()
	go func() {
		<-ctx.Done()
		timeout, done := context.WithTimeout(context.Background(), 5*time.Second)
		defer done()
		if err := server.Shutdown(timeout); err != nil {
			slog.Error("shutdown", "error", err)
		}
	}()
	slog.Info("api listening", "addr", server.Addr)
	if err := server.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
		slog.Error("serve", "error", err)
		os.Exit(1)
	}
}
