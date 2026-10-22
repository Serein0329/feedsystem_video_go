package social

import (
	"context"
	"feedsystem_video_go/internal/account"
	"feedsystem_video_go/internal/db/rabbitmq"
	rediscache "feedsystem_video_go/internal/db/redis"
	"feedsystem_video_go/internal/pkg/apierror"
	"feedsystem_video_go/internal/pkg/logger"
	"log/slog"
)

type SocialService struct {
	repo        *SocialRepository
	accountrepo *account.AccountRepository
	socialMQ    *rabbitmq.SocialMQ
	cache       *rediscache.Client
}

func NewSocialService(repo *SocialRepository, accountrepo *account.AccountRepository, socialMQ *rabbitmq.SocialMQ, cache *rediscache.Client) *SocialService {
	return &SocialService{repo: repo, accountrepo: accountrepo, socialMQ: socialMQ, cache: cache}
}

func (s *SocialService) Follow(ctx context.Context, followerID, vloggerID uint, username string) error {
	if followerID == vloggerID {
		return apierror.ErrSocialNotFollowSelf
	}
	isExist, err := s.accountrepo.IsExist(ctx, vloggerID)
	if err != nil {
		return err
	}
	if !isExist {
		return apierror.ErrUserNotFound
	}

	mysqlEnqueued := false
	if s.socialMQ != nil {
		if err := s.socialMQ.Follow(ctx, followerID, vloggerID, username); err == nil {
			mysqlEnqueued = true
		}
	}
	if !mysqlEnqueued {
		if err := s.repo.Follow(ctx, followerID, vloggerID); err != nil {
			return err
		}
	}
	s.invalidateFollowingFeedCache(ctx, followerID)
	return nil
}

func (s *SocialService) Unfollow(ctx context.Context, followerID, vloggerID uint) error {
	isExist, err := s.accountrepo.IsExist(ctx, vloggerID)
	if err != nil {
		return err
	}
	if !isExist {
		return apierror.ErrUserNotFound
	}

	if err := s.repo.Unfollow(ctx, followerID, vloggerID); err != nil {
		return err
	}
	s.invalidateFollowingFeedCache(ctx, followerID)
	return nil
}

func (s *SocialService) Isfollowed(ctx context.Context, followerID, vloggerID uint) (bool, error) {
	return s.repo.IsFollowed(ctx, followerID, vloggerID)
}
func (s *SocialService) invalidateFollowingFeedCache(ctx context.Context, accountID uint) {
	if s.cache == nil {
		return
	}
	pattern := s.cache.Key("feed:listByFollowing:*:accountID=%d:*", accountID)
	if err := s.cache.DelByPattern(ctx, pattern); err != nil {
		logger.Info("Del cache failed", slog.String("key", pattern), slog.Any("error", err))
	}
}

func (s *SocialService) GetAllFollowers(ctx context.Context, VloggerID uint) ([]*account.Account, error) {
	isExist, err := s.accountrepo.IsExist(ctx, VloggerID)
	if err != nil {
		return nil, err
	}
	if !isExist {
		return nil, apierror.ErrUserNotFound
	}
	return s.repo.GetAllFollowers(ctx, VloggerID)
}

func (s *SocialService) GetAllVloggers(ctx context.Context, FollowerID uint) ([]*account.Account, error) {
	isExist, err := s.accountrepo.IsExist(ctx, FollowerID)
	if err != nil {
		return nil, err
	}
	if !isExist {
		return nil, apierror.ErrUserNotFound
	}
	return s.repo.GetAllVloggers(ctx, FollowerID)
}

func (s *SocialService) CountFollowers(ctx context.Context, vloggerID uint) (int64, error) {
	return s.repo.CountFollowers(ctx, vloggerID)
}

func (s *SocialService) CountVloggers(ctx context.Context, followerID uint) (int64, error) {
	return s.repo.CountVloggers(ctx, followerID)
}
