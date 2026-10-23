package main

import (
	"feedsystem_video_go/internal/config"
	"feedsystem_video_go/internal/db/mysql"
	"feedsystem_video_go/internal/db/rabbitmq"
	rediscache "feedsystem_video_go/internal/db/redis"
	apphttp "feedsystem_video_go/internal/http"
	"feedsystem_video_go/internal/pkg/logger"
	"log/slog"
	"os"

	"github.com/joho/godotenv"
)

func main() {
	//初始化日志
	logger.Init()

	//导入.env文件到环境变量
	if err := godotenv.Load(); err != nil {
		logger.Info("Load .env file failed", slog.Any("error", err))
	}

	//获取配置文件
	configPath := os.Getenv("CONFIG_PATH")
	if configPath == "" {
		configPath = "configs/config.yaml"
	}
	logger.Info("Loading config from " + configPath)
	cfg, usedDefault, err := config.LoadLocalDev(configPath)
	if err != nil {
		logger.Fatal("Load config failed", slog.Any("error", err))
	}
	if usedDefault {
		logger.Info("Config file not found, using default local config", slog.String("path", configPath))
	} else {
		logger.Info("Config loaded from file: " + configPath)
	}

	// 连接数据库
	sqlDB, err := mysql.NewDB(&cfg.Database)
	if err != nil {
		logger.Fatal("Connect database failed", slog.Any("error", err))
	}
	if err := mysql.AutoMigrate(sqlDB); err != nil {
		logger.Fatal("Auto migrate database failed", slog.Any("error", err))
	}
	defer func() { _ = mysql.CloseDB(sqlDB) }()

	// 连接 Redis (可选，用于缓存)
	cache, err := rediscache.NewCache(&cfg.Redis)
	if err != nil {
		logger.Warn("Redis connect failed", slog.Any("error", err))
		cache = nil
	} else {
		defer func() { _ = cache.Close() }()
		logger.Info("Redis connected")
	}

	// 连接 RabbitMQ (可选，用于消息队列)
	rmq, err := rabbitmq.NewRabbitMQ(&cfg.RabbitMQ)
	if err != nil {
		logger.Warn("RabbitMQ connect failed", slog.Any("error", err))
		rmq = nil
	} else {
		defer func() { _ = rmq.Close() }()
		logger.Info("RabbitMQ connected")
	}

	// Pprof
	pprofServer, err := apphttp.NewPprofServer(&cfg.Pprof)
	if err != nil {
		logger.Warn("Pprof start failed", slog.Any("error", err))
	}
	if pprofServer != nil {
		defer pprofServer.Shutdown()
	}

	// GIN
	if err := apphttp.Start(&cfg.Server, sqlDB, cache, rmq); err != nil {
		logger.Fatal("App start failed", slog.Any("error", err))
	}
}
