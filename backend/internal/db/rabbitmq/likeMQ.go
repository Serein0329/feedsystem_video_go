package rabbitmq

import (
	"context"
	"errors"
	"feedsystem_video_go/internal/pkg/randhex"
	"time"

	amqp "github.com/rabbitmq/amqp091-go"
)

type LikeMQ struct {
	ch *amqp.Channel
}

const (
	LikeExchange   = "like.events"
	LikeQueue      = "like.events"
	LikeBindingKey = "like.*"

	LikeLikeRK   = "like.like"
	LikeUnlikeRK = "like.unlike"
)

type LikeEvent struct {
	EventID    string    `json:"event_id"`
	Action     string    `json:"action"`
	UserID     uint      `json:"user_id"`
	Username   string    `json:"username"`
	VideoID    uint      `json:"video_id"`
	OccurredAt time.Time `json:"occurred_at"`
}

func NewLikeMQ(base *RabbitMQ) (*LikeMQ, error) {
	if base == nil {
		return nil, errors.New("rabbitmq is nil")
	}
	ch, err := base.NewChannel()
	if err != nil {
		return nil, err
	}
	if err := DeclareTopic(ch, LikeExchange, LikeQueue, LikeBindingKey); err != nil {
		_ = ch.Close()
		return nil, err
	}
	return &LikeMQ{ch: ch}, nil
}

func (l *LikeMQ) Like(ctx context.Context, videoID, userID uint, username string) error {
	return l.publish(ctx, "like", LikeLikeRK, LikeEvent{
		UserID:   userID,
		VideoID:  videoID,
		Username: username,
	})
}

func (l *LikeMQ) Unlike(ctx context.Context, videoID, userID uint) error {
	return l.publish(ctx, "unlike", LikeUnlikeRK, LikeEvent{
		UserID:  userID,
		VideoID: videoID,
	})
}

func (l *LikeMQ) publish(ctx context.Context, action, routingKey string, evt LikeEvent) error {
	if l == nil || l.ch == nil {
		return errors.New("like mq is not initialized")
	}
	id, err := randhex.New(16)
	if err != nil {
		return err
	}
	evt.EventID = id
	evt.Action = action
	evt.OccurredAt = time.Now()
	return PublishJSON(ctx, l.ch, LikeExchange, routingKey, evt)
}
