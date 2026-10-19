package main

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"time"

	"github.com/gribeiro239/go-tasks-api/internal/httpapi"
	"github.com/gribeiro239/go-tasks-api/internal/task"
)

func main() {
	if err := run(); err != nil {
		slog.Error("servidor encerrado", "error", err)
		os.Exit(1)
	}
}

func run() error {
	store, err := task.Open(env("DATA_FILE", "data/tasks.json"))
	if err != nil {
		return err
	}
	server := &http.Server{
		Addr:              env("HTTP_ADDR", "127.0.0.1:8080"),
		Handler:           httpapi.New(store),
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       10 * time.Second,
		WriteTimeout:      10 * time.Second,
		IdleTimeout:       60 * time.Second,
	}
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	defer stop()
	serverErrors := make(chan error, 1)
	go func() { serverErrors <- server.ListenAndServe() }()
	slog.Info("API de tarefas iniciando", "address", server.Addr)
	select {
	case err := <-serverErrors:
		if errors.Is(err, http.ErrServerClosed) {
			return nil
		}
		return err
	case <-ctx.Done():
		slog.Info("encerrando API")
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		return server.Shutdown(shutdownCtx)
	}
}

func env(key, fallback string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return fallback
}
