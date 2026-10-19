package worker

import (
	"context"
	"encoding/json"
	rabbitmq2 "feedsystem_video_go/internal/db/rabbitmq"
	rediscache "feedsystem_video_go/internal/db/redis"
	"feedsystem_video_go/internal/pkg/logger"
	"fmt"
	"log/slog"
	"time"

	oredis "github.com/redis/go-redis/v9"
)

func StartOutboxConsumer(queueName string, redisClient *rediscache.Client, rmq *rabbitmq2.RabbitMQ) {
	if redisClient == nil || rmq == nil {
		logger.Info("OutboxWorker Consumer: disable for rabbitmq/redis is not initialized")
		return
	}

	go func() {
		for {
			// 每次重连创建独立的 Channel，不与发布者共用，因为一旦这个 Channel 上任何方法出错，它就进入 closed 状态，永久失效——不能恢复、不能重用。
			ch, err := rmq.NewChannel()
			if err != nil {
				logger.Info("OutboxWorker Consumer: 创建Channel 失败, 5秒后重试", slog.Any("error", err))
				time.Sleep(5 * time.Second)
				continue
			}

			if err := ch.Qos(10, 0, false); err != nil {
				logger.Info("OutboxWorker Consumer: QoS设置失败,5秒后重试", slog.Any("error", err))
				_ = ch.Close()
				time.Sleep(5 * time.Second)
				continue
			}

			msgs, err := ch.Consume(queueName, "", false, false, false, false, nil)
			if err != nil {
				logger.Info("OutboxWorker Consumer: 消费队列失败, 5秒后重试", slog.String("queue", queueName), slog.Any("error", err))
				_ = ch.Close()
				time.Sleep(5 * time.Second)
				continue
			}

			for msg := range msgs {
				var event rabbitmq2.TimelineEvent
				if err := json.Unmarshal(msg.Body, &event); err != nil {
					_ = msg.Ack(false)
					continue
				}

				ctx, cancel := context.WithTimeout(context.Background(), 500*time.Millisecond)
				cacheKey := redisClient.Key("feed:listByLatest")
				if err = redisClient.ZAdd(ctx, cacheKey, oredis.Z{
					Score:  float64(event.CreateTime),
					Member: fmt.Sprintf("%d", event.VideoID),
				}); err != nil {
					logger.Info("OutboxWorker Consumer: 写入Zset失败", slog.String("key", cacheKey), slog.Any("error", err))
					_ = msg.Nack(false, true)
					cancel()
					continue
				}

				if err := redisClient.ZRemRangeByRank(ctx, cacheKey, 0, -1001); err != nil {
					logger.Info("OutboxWorker Consumer: ZRemRangeByRank失败", slog.String("key", cacheKey), slog.Any("error", err))
				}

				_ = msg.Ack(false)
				cancel()
			}

			// msgs channel 关闭说明 AMQP Channel 断开，关闭并重连
			_ = ch.Close()
			logger.Info("OutboxWorker Consumer: Channel 断开, 5秒后重连...")
			time.Sleep(5 * time.Second)
		}
	}()
}
