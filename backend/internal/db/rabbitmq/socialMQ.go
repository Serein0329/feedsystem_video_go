package rabbitmq

import (
	"context"
	"errors"
	"feedsystem_video_go/internal/pkg/randhex"
	"time"

	amqp "github.com/rabbitmq/amqp091-go"
)

type SocialMQ struct {
	ch *amqp.Channel
}

const (
	SocialExchange   = "social.events"
	SocialQueue      = "social.events"
	SocialBindingKey = "social.*"

	SocialFollowRK   = "social.follow"
	SocialUnfollowRK = "social.unfollow"
)

type SocialEvent struct {
	EventID    string    `json:"event_id"`
	Action     string    `json:"action"`
	FollowerID uint      `json:"follower_id"`
	VloggerID  uint      `json:"vlogger_id"`
	Username   string    `json:"username"`
	OccurredAt time.Time `json:"occurred_at"`
}

func NewSocialMQ(base *RabbitMQ) (*SocialMQ, error) {
	if base == nil {
		return nil, errors.New("rabbitmq is nil")
	}
	ch, err := base.NewChannel()
	if err != nil {
		return nil, err
	}
	if err := DeclareTopic(ch, SocialExchange, SocialQueue, SocialBindingKey); err != nil {
		_ = ch.Close()
		return nil, err
	}
	return &SocialMQ{ch: ch}, nil
}

func (s *SocialMQ) Follow(ctx context.Context, followerID, vloggerID uint, username string) error {
	return s.publish(ctx, "follow", SocialFollowRK, SocialEvent{
		FollowerID: followerID,
		VloggerID:  vloggerID,
		Username:   username,
	})
}

func (s *SocialMQ) UnFollow(ctx context.Context, followerID, vloggerID uint) error {
	return s.publish(ctx, "unfollow", SocialUnfollowRK, SocialEvent{
		FollowerID: followerID,
		VloggerID:  vloggerID,
	})
}

func (s *SocialMQ) publish(ctx context.Context, action, routingKey string, evt SocialEvent) error {
	if s == nil || s.ch == nil {
		return errors.New("social mq is not initialized")
	}
	id, err := randhex.New(16)
	if err != nil {
		return err
	}
	evt.EventID = id
	evt.Action = action
	evt.OccurredAt = time.Now()
	return PublishJSON(ctx, s.ch, SocialExchange, routingKey, evt)
}
