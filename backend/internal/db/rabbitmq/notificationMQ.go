package rabbitmq

import (
	"errors"
	"log"
)

const (
	NotificationLikeQueue    = "notification.like"
	NotificationCommentQueue = "notification.comment"
	NotificationSocialQueue  = "notification.social"
	NotificationMentionQueue = "notification.mention"
)

var notificationTopics = []struct {
	exchange   string
	queue      string
	bindingKey string
}{
	{likeExchange, NotificationLikeQueue, likeLikeRK},
	{commentExchange, NotificationCommentQueue, commentPublishRK},
	{socialExchange, NotificationSocialQueue, socialFollowRK},
	{commentExchange, NotificationMentionQueue, commentMentionRK},
}

func NotificationQueues() []string {
	return []string{NotificationLikeQueue, NotificationCommentQueue, NotificationSocialQueue, NotificationMentionQueue}
}

func DeclareNotificationTopics(base *RabbitMQ) error {
	if base == nil {
		return errors.New("rabbitmq base is nil")
	}
	ch, err := base.NewChannel()
	if err != nil {
		return err
	}
	defer func() { _ = ch.Close() }()

	var firstErr error
	for _, t := range notificationTopics {
		if err := DeclareTopic(ch, t.exchange, t.queue, t.bindingKey); err != nil {
			log.Printf("notification topic init failed: exchange=%s queue=%s: %v", t.exchange, t.queue, err)
			if firstErr == nil {
				firstErr = err
			}
		}
	}
	return firstErr
}
