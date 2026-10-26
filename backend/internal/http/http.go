package http

import (
	"context"
	"errors"
	"feedsystem_video_go/internal/config"
	rabbitmq2 "feedsystem_video_go/internal/db/rabbitmq"
	"feedsystem_video_go/internal/db/redis"
	"feedsystem_video_go/internal/pkg/logger"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"gorm.io/gorm"
)

func Start(cfg *config.ServerConfig, db *gorm.DB, cache *redis.Client, rmq *rabbitmq2.RabbitMQ) error {
	if cfg == nil {
		return errors.New("config is nil")
	}
	ln, err := net.Listen("tcp", ":"+cfg.Port)
	if err != nil {
		return fmt.Errorf("failed to start app server in port %q: %w", cfg.Port, err)
	}

	ctx, workerCancel := context.WithCancel(context.Background())
	defer workerCancel()
	server := &http.Server{
		Handler:           SetGIN(ctx, db, cache, rmq),
		ReadHeaderTimeout: 5 * time.Second,
	}

	serveErr := make(chan error, 1)
	go func() {
		logger.Info("App listening", slog.String("port", cfg.Port))
		serveErr <- server.Serve(ln)
	}()
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	defer signal.Stop(quit)

	select {
	case err := <-serveErr:
		if err != nil && !errors.Is(err, http.ErrServerClosed) {
			return fmt.Errorf("app server serve failed: %w", err)
		}
		return nil
	case <-quit:
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		if err := server.Shutdown(ctx); err != nil {
			logger.Warn("App server shutdown failed", slog.Any("error", err))
		} else {
			logger.Info("App shutdown success")
		}
		return nil
	}
}
