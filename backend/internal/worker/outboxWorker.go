package worker

import (
	"context"
	"encoding/json"
	rabbitmq2 "feedsystem_video_go/internal/db/rabbitmq"
	rediscache "feedsystem_video_go/internal/db/redis"
	"feedsystem_video_go/internal/video"
	"fmt"
	"log"
	"time"

	oredis "github.com/redis/go-redis/v9"
	"gorm.io/gorm"
)

func StartProducer(db *gorm.DB, tmq *rabbitmq2.TimelineMQ) {
	if tmq == nil {
		log.Printf("Outbox poller disabled: timeline mq is not initialized")
		return
	}

	go func() {
		for {
			var messages []video.OutboxMsg

			err := db.Where("status = ?", "pending").Order("create_time ASC").Limit(100).Find(&messages).Error

			if err != nil || len(messages) == 0 {
				time.Sleep(1 * time.Second)
				continue
			}

			for _, msg := range messages {
				err := tmq.PublishVideo(context.Background(), msg.VideoID, msg.CreateTime)

				if err == nil {
					if err := db.Delete(&msg).Error; err != nil {
						log.Printf("DB删除 outbox 消息失败: id=%d, err=%v", msg.ID, err)
					}
				} else {
					log.Printf("投递MQ失败: VideoID: %d, err: %v", msg.VideoID, err)
				}
			}
		}
	}()
}

func StartConsumer(queueName string, redisClient *rediscache.Client, rmq *rabbitmq2.RabbitMQ) {
	if redisClient == nil {
		log.Printf("Timeline consumer disabled: redis is not initialized")
		return
	}

	go func() {
		for {
			// 每次重连创建独立的 Channel，不与发布者共用，因为一旦这个 Channel 上任何方法出错，它就进入 closed 状态，永久失效——不能恢复、不能重用。
			ch, err := rmq.NewChannel()
			if err != nil {
				log.Printf("Timeline consumer: 创建 Channel 失败: %v, 5秒后重试", err)
				time.Sleep(5 * time.Second)
				continue
			}

			if err := ch.Qos(10, 0, false); err != nil {
				log.Printf("Timeline consumer: QoS 设置失败: %v,5秒后重试", err)
				_ = ch.Close()
				time.Sleep(5 * time.Second)
				continue
			}

			msgs, err := ch.Consume(queueName, "", false, false, false, false, nil)
			if err != nil {
				log.Printf("Timeline consumer: 注册消费失败: %v, 5秒后重试", err)
				_ = ch.Close()
				time.Sleep(5 * time.Second)
				continue
			}

			log.Printf("Timeline consumer 已启动, queue=%s", queueName)

			for msg := range msgs {
				var event rabbitmq2.TimelineEvent
				if err := json.Unmarshal(msg.Body, &event); err != nil {
					log.Printf("Timeline consumer: 反序列化失败: %v", err)
					_ = msg.Ack(false)
					continue
				}

				ctx, cancel := context.WithTimeout(context.Background(), 500*time.Millisecond)
				timelineKey := redisClient.Key("feed:listByLatest")
				err = redisClient.ZAdd(ctx, timelineKey, oredis.Z{
					Score:  float64(event.CreateTime),
					Member: fmt.Sprintf("%d", event.VideoID),
				})

				if err != nil {
					log.Printf("Timeline consumer: 写入Zset失败: %v", err)
					_ = msg.Nack(false, true)
					cancel()
					continue
				}

				if err := redisClient.ZRemRangeByRank(ctx, timelineKey, 0, -1001); err != nil {
					log.Printf("Timeline consumer: ZRem失败: %v", err)
				}

				_ = msg.Ack(false)
				cancel()
			}

			// msgs channel 关闭说明 AMQP Channel 断开，关闭并重连
			_ = ch.Close()
			log.Printf("Timeline consumer: Channel 断开, 5秒后重连...")
			time.Sleep(5 * time.Second)
		}
	}()
}
