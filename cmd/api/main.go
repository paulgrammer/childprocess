package main

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"runtime"
	"syscall"
	"time"

	"github.com/paulgrammer/childprocess/internal/executor"
	"github.com/paulgrammer/childprocess/internal/httpapi"
	"github.com/paulgrammer/childprocess/internal/jobs"
	"github.com/paulgrammer/childprocess/internal/webhook"
)

func main() {
	// Logger
	level := parseLogLevel(getenv("LOG_LEVEL", "INFO"))
	slog.SetDefault(slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: level})))

	// Config via env with sensible defaults
	addr := getenv("API_ADDR", ":8080")
	poolSize := getEnvInt("POOL_SIZE", runtime.NumCPU())
	maxWebhookRetries := getEnvInt("WEBHOOK_MAX_RETRIES", 5)
	webhookTimeoutSec := getEnvInt("WEBHOOK_TIMEOUT_SEC", 10)

	// Core components
	store := jobs.NewInMemoryStore()
	sender := webhook.NewHTTPSender(time.Duration(webhookTimeoutSec)*time.Second, maxWebhookRetries)
	streamer := jobs.NewLogStreamer()
	runner := executor.NewExecRunner()
	manager, err := jobs.NewManager(poolSize, store, sender, runner, streamer)
	if err != nil {
		slog.Error("failed to initialize manager", "error", err)
		os.Exit(1)
	}
	defer manager.Stop()

	mux := httpapi.NewRouter(manager, streamer)

	srv := &http.Server{
		Addr:              addr,
		Handler:           mux,
		ReadTimeout:       15 * time.Second,
		ReadHeaderTimeout: 10 * time.Second,
		WriteTimeout:      60 * time.Second,
		IdleTimeout:       120 * time.Second,
	}

	go func() {
		slog.Info("server listening", "addr", addr)
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			slog.Error("server error", "error", err)
			os.Exit(1)
		}
	}()

	// Graceful shutdown
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit
	slog.Info("shutdown signal received")

	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	if err := srv.Shutdown(ctx); err != nil {
		slog.Error("server shutdown error", "error", err)
	}
}

func getenv(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}

func getEnvInt(key string, def int) int {
	if v := os.Getenv(key); v != "" {
		var out int
		_, err := fmt.Sscanf(v, "%d", &out)
		if err == nil {
			return out
		}
	}
	return def
}

func parseLogLevel(s string) slog.Level {
	switch s {
	case "DEBUG", "debug":
		return slog.LevelDebug
	case "INFO", "info":
		return slog.LevelInfo
	case "WARN", "warning", "warn":
		return slog.LevelWarn
	case "ERROR", "error":
		return slog.LevelError
	default:
		return slog.LevelInfo
	}
}
