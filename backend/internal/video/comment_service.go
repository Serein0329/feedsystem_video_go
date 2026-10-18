package video

import (
	"context"
	"feedsystem_video_go/internal/account"
	rabbitmq2 "feedsystem_video_go/internal/db/rabbitmq"
	rediscache "feedsystem_video_go/internal/db/redis"
	"feedsystem_video_go/internal/pkg/apierror"
	"feedsystem_video_go/internal/sse"
	"log"
	"regexp"
	"strings"

	"gorm.io/gorm"
)

type CommentService struct {
	repo            *CommentRepository
	VideoRepository *VideoRepository
	cache           *rediscache.Client
	commentMQ       *rabbitmq2.CommentMQ
	popularityMQ    *rabbitmq2.PopularityMQ
}

func NewCommentService(repo *CommentRepository, videoRepo *VideoRepository, cache *rediscache.Client, commentMQ *rabbitmq2.CommentMQ, popularityMQ *rabbitmq2.PopularityMQ) *CommentService {
	return &CommentService{repo: repo, VideoRepository: videoRepo, cache: cache, commentMQ: commentMQ, popularityMQ: popularityMQ}
}

func (s *CommentService) Publish(ctx context.Context, comment *Comment) error {
	comment.Content = strings.TrimSpace(comment.Content)

	exists, err := s.VideoRepository.IsExist(ctx, comment.VideoID)
	if err != nil {
		return err
	}
	if !exists {
		return apierror.ErrVideoNotFound
	}

	mysqlEnqueued := false
	redisEnqueued := false
	if s.commentMQ != nil {
		if err := s.commentMQ.Publish(ctx, comment.Username, comment.VideoID, comment.AuthorID, comment.Content); err == nil {
			mysqlEnqueued = true
		}
	}
	if s.popularityMQ != nil {
		if err := s.popularityMQ.Update(ctx, comment.VideoID, 1); err == nil {
			redisEnqueued = true
		}
	}
	if mysqlEnqueued && redisEnqueued {
		s.notifyMentions(ctx, comment)
		return nil
	}

	if !mysqlEnqueued {
		if err := s.repo.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
			if err := tx.Create(comment).Error; err != nil {
				return err
			}
			return tx.Model(&Video{}).Where("id = ?", comment.VideoID).
				UpdateColumn("popularity", gorm.Expr("popularity + 1")).Error
		}); err != nil {
			return err
		}
	}

	if !redisEnqueued {
		UpdatePopularityCache(ctx, s.cache, comment.VideoID, 1)
	}
	s.notifyMentions(ctx, comment)
	return nil
}

func (s *CommentService) Delete(ctx context.Context, commentID uint, accountID uint) error {
	comment, err := s.repo.GetByID(ctx, commentID)
	if err != nil {
		return err
	}
	if comment.AuthorID != accountID {
		return apierror.ErrForbidden
	}

	mysqlEnqueued := false
	redisEnqueued := false
	if s.commentMQ != nil {
		if err := s.commentMQ.Delete(ctx, commentID); err == nil {
			mysqlEnqueued = true
		}
	}
	if s.popularityMQ != nil {
		if err := s.popularityMQ.Update(ctx, comment.VideoID, -1); err == nil {
			redisEnqueued = true
		}
	}
	if mysqlEnqueued && redisEnqueued {
		return nil
	}

	if !mysqlEnqueued {
		if err := s.repo.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
			if err := tx.Delete(&Comment{}, commentID).Error; err != nil {
				return err
			}
			return tx.Model(&Video{}).Where("id = ?", comment.VideoID).
				UpdateColumn("popularity", gorm.Expr("GREATEST(popularity - 1, 0)")).Error
		}); err != nil {
			return err
		}
	}

	if !redisEnqueued {
		UpdatePopularityCache(ctx, s.cache, comment.VideoID, -1)
	}
	return nil
}

func (s *CommentService) GetAll(ctx context.Context, videoID uint) ([]Comment, error) {
	exists, err := s.VideoRepository.IsExist(ctx, videoID)
	if err != nil {
		return nil, err
	}
	if !exists {
		return nil, apierror.ErrVideoNotFound
	}
	return s.repo.GetAllComments(ctx, videoID)
}

func (s *CommentService) notifyMentions(ctx context.Context, comment *Comment) {
	var mentionRegex = regexp.MustCompile(`@(\w+)`)
	matches := mentionRegex.FindAllStringSubmatch(comment.Content, -1)
	if len(matches) == 0 {
		return
	}
	notifs := make([]*sse.Notification, 0, len(matches))
	seen := make(map[string]bool)
	for _, m := range matches {
		username := m[1]
		if seen[username] || username == comment.Username {
			continue
		}
		seen[username] = true
		var accID uint
		if err := s.repo.db.WithContext(ctx).Model(&account.Account{}).Select("id").Where("username = ?", username).Scan(&accID).Error; err != nil || accID == 0 {
			continue
		}
		notifs = append(notifs, &sse.Notification{
			RecipientID: accID,
			SenderID:    comment.AuthorID,
			Type:        "mention",
			TargetID:    comment.VideoID,
			Content:     comment.Username + " 在评论中提到了你",
		})
	}
	reciIDs := make([]uint, 0, len(notifs))
	for _, notif := range notifs {
		reciIDs = append(reciIDs, notif.RecipientID)
	}
	if s.commentMQ != nil {
		if err := s.commentMQ.Mention(ctx, comment.Username, comment.VideoID, comment.AuthorID, reciIDs); err != nil {
			log.Printf("comment mention failed: %v", err)
		}
	}
	if err := s.repo.db.WithContext(ctx).Create(&notifs).Error; err != nil {
		log.Printf("create mention notification failed: %v", err)
	}
}
