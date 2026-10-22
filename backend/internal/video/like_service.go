package video

import (
	"context"
	rabbitmq2 "feedsystem_video_go/internal/db/rabbitmq"
	rediscache "feedsystem_video_go/internal/db/redis"
	"feedsystem_video_go/internal/pkg/apierror"
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

func (s *LikeService) Like(ctx context.Context, videoID, accountID uint, username string) error {
	ok, err := s.VideoRepo.IsExist(ctx, videoID)
	if err != nil {
		return err
	}
	if !ok {
		return apierror.ErrVideoNotFound
	}

	isLiked, err := s.repo.IsLiked(ctx, videoID, accountID)
	if err != nil {
		return err
	}
	if isLiked {
		return nil
	}

	mysqlEnqueued := false
	redisEnqueued := false
	if s.likeMQ != nil {
		if err := s.likeMQ.Like(ctx, videoID, accountID, username); err == nil {
			mysqlEnqueued = true
		}
	}
	if s.popularityMQ != nil {
		if err := s.popularityMQ.Update(ctx, videoID, 1); err == nil {
			redisEnqueued = true
		}
	}
	if mysqlEnqueued && redisEnqueued {
		return nil
	}

	if !mysqlEnqueued {
		if err := s.repo.Like(ctx, videoID, accountID); err != nil {
			return err
		}
	}

	if !redisEnqueued {
		UpdatePopularityCache(ctx, s.cache, videoID, 1)
	}
	return nil
}

func (s *LikeService) Unlike(ctx context.Context, videoID, accountID uint) error {
	ok, err := s.VideoRepo.IsExist(ctx, videoID)
	if err != nil {
		return err
	}
	if !ok {
		return apierror.ErrVideoNotFound
	}

	isLiked, err := s.repo.IsLiked(ctx, videoID, accountID)
	if err != nil {
		return err
	}
	if !isLiked {
		return nil
	}

	mysqlEnqueued := false
	redisEnqueued := false
	if s.likeMQ != nil {
		if err := s.likeMQ.Unlike(ctx, videoID, accountID); err == nil {
			mysqlEnqueued = true
		}
	}
	if s.popularityMQ != nil {
		if err := s.popularityMQ.Update(ctx, videoID, -1); err == nil {
			redisEnqueued = true
		}
	}
	if mysqlEnqueued && redisEnqueued {
		return nil
	}

	if !mysqlEnqueued {
		if err := s.repo.Unlike(ctx, videoID, accountID); err != nil {
			return err
		}
	}

	if !redisEnqueued {
		UpdatePopularityCache(ctx, s.cache, videoID, -1)
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

func (s *LikeService) ListLikedVideos(ctx context.Context, accountID, lastID uint, limit int) (ListMyLikedVideosResponse, error) {
	rows, err := s.repo.ListLikedVideos(ctx, accountID, lastID, limit)
	if err != nil {
		return ListMyLikedVideosResponse{}, err
	}
	videos := make([]Video, 0, len(rows))
	for i := range rows {
		videos = append(videos, rows[i].Video)
	}
	if len(rows) == 0 {
		return ListMyLikedVideosResponse{LastID: lastID}, nil
	}
	return ListMyLikedVideosResponse{
		Videos:  videos,
		LastID:  rows[len(rows)-1].LikeID,
		HasMore: len(rows) == limit,
	}, nil
}
