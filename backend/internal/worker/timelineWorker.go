package worker

import (
	"context"
	"encoding/json"
	"errors"
	rabbitmq2 "feedsystem_video_go/internal/db/rabbitmq"
	rediscache "feedsystem_video_go/internal/db/redis"
	"feedsystem_video_go/internal/pkg/logger"
	"fmt"
	"log/slog"
	"time"

	amqp "github.com/rabbitmq/amqp091-go"
	oredis "github.com/redis/go-redis/v9"
)

type TimelineWorker struct {
	ch    *amqp.Channel
	cache *rediscache.Client
	queue string
}

func NewTimelineWorker(ch *amqp.Channel, cache *rediscache.Client, queue string) *TimelineWorker {
	return &TimelineWorker{ch: ch, cache: cache, queue: queue}
}

func (w *TimelineWorker) Run(ctx context.Context) error {
	if w == nil || w.ch == nil {
		return errors.New("timeline worker is not initialized")
	}
	if w.queue == "" {
		return errors.New("queue is required")
	}

	deliveries, err := w.ch.Consume(
		w.queue,
		"",
		false,
		false,
		false,
		false,
		nil,
	)
	if err != nil {
		return err
	}

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case d, ok := <-deliveries:
			if !ok {
				return errors.New("deliveries channel closed")
			}
			w.handleDelivery(ctx, d)
		}
	}
}

func (w *TimelineWorker) handleDelivery(ctx context.Context, d amqp.Delivery) {
	const maxRetries = 3
	for i := 0; i <= maxRetries; i++ {
		select {
		case <-ctx.Done():
			_ = d.Nack(false, true)
			return
		default:
		}
		if err := w.process(ctx, d.Body); err != nil {
			if i >= maxRetries {
				logger.Info("TimelineWorker:重试后仍失败, 丢弃", slog.Int("maxRetries", maxRetries), slog.Any("error", err))
				_ = d.Ack(false)
				return
			}
			wait := time.Duration(1<<i) * time.Second
			logger.Info(fmt.Sprintf("TimelineWorker:处理失败, 等待一顿时间后重试(%d/%d)", i+1, maxRetries), slog.Int("wait", int(wait)), slog.Any("error", err))
			time.Sleep(wait)
			continue
		}
		_ = d.Ack(false)
		return
	}
}

func (w *TimelineWorker) process(ctx context.Context, body []byte) error {
	var evt rabbitmq2.TimelineEvent
	if err := json.Unmarshal(body, &evt); err != nil {
		// 解析事件失败，直接丢弃
		return nil
	}

	ctx, cancel := context.WithTimeout(ctx, 500*time.Millisecond)
	defer cancel()
	cacheKey := w.cache.Key("feed:listByLatest")
	if err := w.cache.ZAdd(ctx, cacheKey, oredis.Z{
		Score:  float64(evt.CreateTime),
		Member: fmt.Sprintf("%d", evt.VideoID),
	}); err != nil {
		return err
	}

	if err := w.cache.ZRemRangeByRank(ctx, cacheKey, 0, -1001); err != nil {
		logger.Info("TimelineWorker:ZRemRangeByRank失败", slog.String("key", cacheKey), slog.Any("error", err))
	}
	return nil
}
