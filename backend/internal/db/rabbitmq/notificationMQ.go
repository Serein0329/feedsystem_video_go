package rabbitmq

import (
	"errors"
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
	{LikeExchange, NotificationLikeQueue, LikeLikeRK},
	{CommentExchange, NotificationCommentQueue, CommentPublishRK},
	{SocialExchange, NotificationSocialQueue, SocialFollowRK},
	{CommentExchange, NotificationMentionQueue, CommentMentionRK},
}

func NotificationQueues() []string {
	return []string{NotificationLikeQueue, NotificationCommentQueue, NotificationSocialQueue, NotificationMentionQueue}
}

func NewNotificationMQ(base *RabbitMQ) error {
	if base == nil {
		return errors.New("rabbitmq is nil")
	}
	ch, err := base.NewChannel()
	if err != nil {
		return err
	}
	defer func() { _ = ch.Close() }()

	var firstErr error
	for _, t := range notificationTopics {
		if err := DeclareTopic(ch, t.exchange, t.queue, t.bindingKey); err != nil {
			if firstErr == nil {
				firstErr = err
			}
		}
	}
	return firstErr
}
