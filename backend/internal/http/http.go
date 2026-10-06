package http

import (
	"context"
	"errors"
	"feedsystem_video_go/internal/config"
	rabbitmq2 "feedsystem_video_go/internal/db/rabbitmq"
	"feedsystem_video_go/internal/db/redis"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"gorm.io/gorm"
)

func Start(cfg *config.Config, db *gorm.DB, cache *redis.Client, rmq *rabbitmq2.RabbitMQ) {
	server := &http.Server{
		Addr:         ":" + cfg.Server.Port,
		Handler:      SetGIN(db, cache, rmq),
		ReadTimeout:  cfg.Server.ReadTimeout,
		WriteTimeout: cfg.Server.WriteTimeout,
	}
	go func() {
		log.Printf("server start success in 0.0.0.0:%s", cfg.Server.Port)
		if err := server.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			log.Fatalf("server start failed: %v", err)
		}
	}()
	stop(server)
}

func stop(srv *http.Server) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	defer func() {
	}()
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit
	if err := srv.Shutdown(ctx); err != nil {
		log.Printf("server shutdown failed: %v", err)
	}
	log.Println("server stop success")
}
