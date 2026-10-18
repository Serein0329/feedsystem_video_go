package worker

import (
	"context"
	"encoding/json"
	"errors"
	"feedsystem_video_go/internal/account"
	rabbitmq2 "feedsystem_video_go/internal/db/rabbitmq"
	"feedsystem_video_go/internal/sse"
	"feedsystem_video_go/internal/video"
	"log"
	"time"

	amqp "github.com/rabbitmq/amqp091-go"
	"gorm.io/gorm"
)

type Notification = sse.Notification

type NotificationWorker struct {
	rmq *rabbitmq2.RabbitMQ
	db  *gorm.DB
	hub NotificationHub
}

type NotificationHub interface {
	Push(userID uint, n *Notification)
}

func NewNotificationWorker(rmq *rabbitmq2.RabbitMQ, db *gorm.DB, hub NotificationHub) *NotificationWorker {
	return &NotificationWorker{rmq: rmq, db: db, hub: hub}
}

func (w *NotificationWorker) Run(ctx context.Context) error {
	if w == nil || w.rmq == nil {
		return errors.New("notification worker is not initialized")
	}
	for _, q := range rabbitmq2.NotificationQueues() {
		go func(queue string) {
			for {
				if ctx.Err() != nil {
					return
				}
				ch, err := w.rmq.NewChannel()
				if err != nil {
					log.Printf("notificationWorker-%s: 创建 Channel 失败: %v, 5秒后重试", queue, err)
					time.Sleep(5 * time.Second)
					continue
				}
				deliveries, err := ch.Consume(queue, "", false, false, false, false, nil)
				if err != nil {
					log.Printf("notificationWorker-%s: 消费消息失败: %v, 5秒后重试", queue, err)
					goto consumeLoop
				}
				for {
					select {
					case <-ctx.Done():
						return
					case d, ok := <-deliveries:
						if !ok {
							log.Printf("notificationWorker-%s: 通道关闭, 5秒后重试", queue)
							goto consumeLoop
						}
						w.handleDelivery(ctx, d)
					}
				}
			consumeLoop:
				_ = ch.Close()
				time.Sleep(5 * time.Second)
			}
		}(q)
	}
	return nil
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
				log.Printf("notification worker: 重试 %d 次后仍失败, 丢弃: %v", maxRetries, err)
				_ = d.Ack(false)
				return
			}
			wait := time.Duration(1<<i) * time.Second
			log.Printf("notification worker: 处理失败, %v 后重试 (%d/%d): %v", wait, i+1, maxRetries, err)
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
		if err := w.db.WithContext(ctx).Model(&struct {
			ID       uint
			AuthorID uint
		}{}).Table("videos").Where("id = ?", evt.VideoID).Select("author_id").Scan(&authorID).Error; err != nil {
			return err
		}
		if authorID == 0 || authorID == evt.UserID {
			return nil
		}
		var userName string
		if err := w.db.WithContext(ctx).Model(&account.Account{}).Select("username").Where("id = ?", evt.UserID).Scan(&userName).Error; err != nil {
			return err
		}
		notifs = append(notifs, &Notification{RecipientID: authorID, SenderID: evt.UserID, Type: "like", TargetID: evt.VideoID, Content: userName + "点赞了你的视频"})
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
		notifs = append(notifs, &Notification{RecipientID: authorID, SenderID: evt.AuthorID, Type: "comment", TargetID: evt.VideoID, Content: evt.Username + "评论了你的视频"})
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
				Content:     evt.Username + " 在评论中提到了你",
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
		var userName string
		if err := w.db.WithContext(ctx).Model(&account.Account{}).Select("username").Where("id = ?", evt.FollowerID).Scan(&userName).Error; err != nil {
			return err
		}
		notifs = append(notifs, &Notification{RecipientID: evt.VloggerID, SenderID: evt.FollowerID, Type: "follow", TargetID: evt.FollowerID, Content: userName + "关注了你"})
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
