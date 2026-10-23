package worker

import (
	"context"
	"encoding/json"
	"errors"
	rabbitmq2 "feedsystem_video_go/internal/db/rabbitmq"
	"feedsystem_video_go/internal/pkg/logger"
	"feedsystem_video_go/internal/sse"
	"feedsystem_video_go/internal/video"
	"fmt"
	"log/slog"
	"time"

	amqp "github.com/rabbitmq/amqp091-go"
	"gorm.io/gorm"
)

type Notification = sse.Notification

type NotificationWorker struct {
	queue string
	ch    *amqp.Channel
	db    *gorm.DB
	hub   NotificationHub
}

type NotificationHub interface {
	Push(userID uint, n *Notification)
}

func NewNotificationWorker(queue string, ch *amqp.Channel, db *gorm.DB, hub NotificationHub) *NotificationWorker {
	return &NotificationWorker{queue: queue, ch: ch, db: db, hub: hub}
}

func (w *NotificationWorker) Run(ctx context.Context) error {
	if w == nil || w.ch == nil {
		return fmt.Errorf("NotificationWorker-%s is not initialized", w.queue)
	}
	if w.queue == "" {
		return errors.New("queue is required")
	}

	deliveries, err := w.ch.Consume(w.queue, "", false, false, false, false, nil)
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

func (w *NotificationWorker) handleDelivery(ctx context.Context, d amqp.Delivery) {
	const maxRetries = 3
	for i := 0; i <= maxRetries; i++ {
		select {
		case <-ctx.Done():
			_ = d.Nack(false, true)
			return
		default:
		}
		if err := w.process(ctx, d); err != nil {
			if i >= maxRetries {
				logger.Info("NotificationWorker : 重试后仍失败, 丢弃", slog.String("queue", w.queue), slog.Int("maxRetries", maxRetries), slog.Any("error", err))
				_ = d.Nack(false, !d.Redelivered)
				return
			}
			wait := time.Duration(1<<i) * time.Second
			logger.Info(fmt.Sprintf("NotificationWorker : 处理失败, 等待一顿时间后重试(%d/%d)", i+1, maxRetries), slog.String("queue", w.queue), slog.Int("wait", int(wait)), slog.Any("error", err))
			time.Sleep(wait)
			continue
		}
		_ = d.Ack(false)
		return
	}
}

func (w *NotificationWorker) process(ctx context.Context, d amqp.Delivery) error {
	body := d.Body
	if len(body) == 0 {
		return nil
	}
	routingKey := d.RoutingKey

	var notifs []*Notification

	switch {
	case routingKey == "like.like":
		var evt rabbitmq2.LikeEvent
		//错误数据返回nil直接仍
		if err := json.Unmarshal(body, &evt); err != nil {
			return nil
		}
		if evt.UserID == 0 || evt.VideoID == 0 {
			return nil
		}
		var authorID uint
		if err := w.db.WithContext(ctx).Model(&video.Video{}).Where("id = ?", evt.VideoID).Select("author_id").Scan(&authorID).Error; err != nil {
			return err
		}
		if authorID == 0 || authorID == evt.UserID {
			return nil
		}
		notifs = append(notifs, &Notification{
			RecipientID: authorID,
			SenderID:    evt.UserID,
			Type:        "like",
			TargetID:    evt.VideoID,
			Username:    evt.Username,
			Content:     "点赞了你的视频",
		})
	case routingKey == "comment.publish":
		var evt rabbitmq2.CommentEvent
		if err := json.Unmarshal(body, &evt); err != nil {
			return nil
		}
		if evt.AuthorID == 0 || evt.VideoID == 0 {
			return nil
		}
		var authorID uint
		if err := w.db.WithContext(ctx).Model(&video.Video{}).Where("id = ?", evt.VideoID).Select("author_id").Scan(&authorID).Error; err != nil {
			return err
		}
		if authorID == 0 || authorID == evt.AuthorID {
			return nil
		}
		notifs = append(notifs, &Notification{
			RecipientID: authorID,
			SenderID:    evt.AuthorID,
			Type:        "comment",
			TargetID:    evt.VideoID,
			Username:    evt.Username,
			Content:     "评论了你的视频",
		})
	case routingKey == "comment.mention":
		var evt rabbitmq2.CommentEvent
		if err := json.Unmarshal(body, &evt); err != nil {
			return nil
		}
		if evt.AuthorID == 0 || evt.VideoID == 0 {
			return nil
		}
		for _, uid := range evt.MentionedIDs {
			if uid == 0 || uid == evt.AuthorID {
				continue
			}
			notifs = append(notifs, &Notification{
				RecipientID: uid,
				SenderID:    evt.AuthorID,
				Type:        "mention",
				TargetID:    evt.VideoID,
				Username:    evt.Username,
				Content:     "在评论中提到了你",
			})
		}
	case routingKey == "social.follow":
		var evt rabbitmq2.SocialEvent
		if err := json.Unmarshal(body, &evt); err != nil {
			return nil
		}
		if evt.FollowerID == 0 || evt.VloggerID == 0 {
			return nil
		}
		notifs = append(notifs, &Notification{
			RecipientID: evt.VloggerID,
			SenderID:    evt.FollowerID,
			Type:        "follow",
			TargetID:    evt.FollowerID,
			Username:    evt.Username,
			Content:     "关注了你",
		})
	}

	if notifs == nil || len(notifs) == 0 {
		return nil
	}
	if err := w.db.WithContext(ctx).Create(notifs).Error; err != nil {
		return err
	}
	if w.hub != nil {
		for _, notif := range notifs {
			w.hub.Push(notif.RecipientID, notif)
		}
	}
	return nil
}
