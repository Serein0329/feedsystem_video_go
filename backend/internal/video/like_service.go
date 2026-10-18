package video

import (
	"context"
	rabbitmq2 "feedsystem_video_go/internal/db/rabbitmq"
	rediscache "feedsystem_video_go/internal/db/redis"
	"feedsystem_video_go/internal/pkg/apierror"
	"time"

	"gorm.io/gorm"
)

type LikeService struct {
	repo         *LikeRepository
	VideoRepo    *VideoRepository
	cache        *rediscache.Client
	likeMQ       *rabbitmq2.LikeMQ
	popularityMQ *rabbitmq2.PopularityMQ
}

func NewLikeService(repo *LikeRepository, videoRepo *VideoRepository, cache *rediscache.Client, likeMQ *rabbitmq2.LikeMQ, popularityMQ *rabbitmq2.PopularityMQ) *LikeService {
	return &LikeService{repo: repo, VideoRepo: videoRepo, cache: cache, likeMQ: likeMQ, popularityMQ: popularityMQ}
}

func (s *LikeService) Like(ctx context.Context, like *Like) error {
	ok, err := s.VideoRepo.IsExist(ctx, like.VideoID)
	if err != nil {
		return err
	}
	if !ok {
		return apierror.ErrVideoNotFound
	}

	isLiked, err := s.repo.IsLiked(ctx, like.VideoID, like.AccountID)
	if err != nil {
		return err
	}
	if isLiked {
		return nil
	}

	like.CreatedAt = time.Now()
	mysqlEnqueued := false
	redisEnqueued := false
	if s.likeMQ != nil {
		if err := s.likeMQ.Like(ctx, like.AccountID, like.VideoID); err == nil {
			mysqlEnqueued = true
		}
	}
	if s.popularityMQ != nil {
		if err := s.popularityMQ.Update(ctx, like.VideoID, 1); err == nil {
			redisEnqueued = true
		}
	}
	if mysqlEnqueued && redisEnqueued {
		return nil
	}

	if !mysqlEnqueued {
		err := s.repo.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
			if err := tx.Create(&like).Error; err != nil {
				return err
			}
			if err := tx.Model(&Video{}).Where("id = ?", like.VideoID).
				UpdateColumn("likes_count", gorm.Expr("likes_count + 1")).Error; err != nil {
				return err
			}
			return tx.Model(&Video{}).Where("id = ?", like.VideoID).
				UpdateColumn("popularity", gorm.Expr("popularity + 1")).Error
		})
		if err != nil {
			return err
		}
	}

	if !redisEnqueued {
		UpdatePopularityCache(ctx, s.cache, like.VideoID, 1)
	}
	return nil
}

func (s *LikeService) Unlike(ctx context.Context, like *Like) error {
	ok, err := s.VideoRepo.IsExist(ctx, like.VideoID)
	if err != nil {
		return err
	}
	if !ok {
		return apierror.ErrVideoNotFound
	}

	isLiked, err := s.repo.IsLiked(ctx, like.VideoID, like.AccountID)
	if err != nil {
		return err
	}
	if !isLiked {
		return nil
	}

	mysqlEnqueued := false
	redisEnqueued := false
	if s.likeMQ != nil {
		if err := s.likeMQ.Unlike(ctx, like.AccountID, like.VideoID); err == nil {
			mysqlEnqueued = true
		}
	}
	if s.popularityMQ != nil {
		if err := s.popularityMQ.Update(ctx, like.VideoID, -1); err == nil {
			redisEnqueued = true
		}
	}
	if mysqlEnqueued && redisEnqueued {
		return nil
	}

	if !mysqlEnqueued {
		err := s.repo.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
			if err := tx.Where("video_id = ? AND account_id = ?", like.VideoID, like.AccountID).Delete(&Like{}).Error; err != nil {
				return err
			}
			//GREATEST 返回参数里最大的那个值
			if err := tx.Model(&Video{}).Where("id = ?", like.VideoID).
				UpdateColumn("likes_count", gorm.Expr("GREATEST(likes_count - 1, 0)")).Error; err != nil {
				return err
			}
			return tx.Model(&Video{}).Where("id = ?", like.VideoID).
				UpdateColumn("popularity", gorm.Expr("GREATEST(popularity - 1, 0)")).Error
		})
		if err != nil {
			return err
		}
	}

	if !redisEnqueued {
		UpdatePopularityCache(ctx, s.cache, like.VideoID, -1)
	}
	return nil
}

func (s *LikeService) IsLiked(ctx context.Context, videoID, accountID uint) (bool, error) {
	isExist, err := s.VideoRepo.IsExist(ctx, videoID)
	if err != nil {
		return false, err
	}
	if !isExist {
		return false, apierror.ErrVideoNotFound
	}
	return s.repo.IsLiked(ctx, videoID, accountID)
}

func (s *LikeService) ListLikedVideos(ctx context.Context, accountID uint) ([]Video, error) {
	return s.repo.ListLikedVideos(ctx, accountID)
}
