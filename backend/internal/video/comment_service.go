package video

import (
	"context"
	"feedsystem_video_go/internal/account"
	rabbitmq2 "feedsystem_video_go/internal/db/rabbitmq"
	rediscache "feedsystem_video_go/internal/db/redis"
	"feedsystem_video_go/internal/pkg/apierror"
	"feedsystem_video_go/internal/pkg/logger"
	"feedsystem_video_go/internal/sse"
	"log/slog"
	"regexp"
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
	exists, err := s.VideoRepository.IsExist(ctx, comment.VideoID)
	if err != nil {
		return err
	}
	if !exists {
		return apierror.ErrVideoNotFound
	}

	mysqlEnqueued := false
	if s.commentMQ != nil {
		if err := s.commentMQ.Publish(ctx, comment.Username, comment.VideoID, comment.AuthorID, comment.Content); err == nil {
			mysqlEnqueued = true
		}
	}
	if !mysqlEnqueued {
		if err := s.repo.CreateComment(ctx, comment); err != nil {
			return err
		}
	}

	redisEnqueued := false
	if s.popularityMQ != nil {
		if err := s.popularityMQ.Update(ctx, comment.VideoID, 1); err == nil {
			redisEnqueued = true
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
	if s.commentMQ != nil {
		if err := s.commentMQ.Delete(ctx, commentID, comment.VideoID); err == nil {
			mysqlEnqueued = true
		}
	}
	if !mysqlEnqueued {
		if err := s.repo.DeleteComment(ctx, comment); err != nil {
			return err
		}
	}

	redisEnqueued := false
	if s.popularityMQ != nil {
		if err := s.popularityMQ.Update(ctx, comment.VideoID, -1); err == nil {
			redisEnqueued = true
		}
	}
	if !redisEnqueued {
		UpdatePopularityCache(ctx, s.cache, comment.VideoID, -1)
	}
	return nil
}

func (s *CommentService) GetAll(ctx context.Context, videoID, lastID uint, limit int) (GetAllCommentsResponse, error) {
	exists, err := s.VideoRepository.IsExist(ctx, videoID)
	if err != nil {
		return GetAllCommentsResponse{}, err
	}
	if !exists {
		return GetAllCommentsResponse{}, apierror.ErrVideoNotFound
	}
	comments, err := s.repo.GetAll(ctx, videoID, lastID, limit)
	if err != nil {
		return GetAllCommentsResponse{}, err
	}
	if len(comments) == 0 {
		return GetAllCommentsResponse{LastID: lastID}, nil
	}
	return GetAllCommentsResponse{
		Comments: comments,
		HasMore:  len(comments) == limit,
		LastID:   comments[len(comments)-1].ID,
	}, nil
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
			Content:     comment.Content,
		})
	}
	if len(notifs) == 0 {
		return
	}
	reciIDs := make([]uint, 0, len(notifs))
	for _, notif := range notifs {
		reciIDs = append(reciIDs, notif.RecipientID)
	}
	if s.commentMQ != nil {
		if err := s.commentMQ.Mention(ctx, comment.Username, comment.VideoID, comment.AuthorID, reciIDs); err == nil {
			return
		}
	}
	if err := s.repo.db.WithContext(ctx).Create(&notifs).Error; err != nil {
		logger.Info("DB create mention notifications failed", slog.Any("error", err))
	}
}
