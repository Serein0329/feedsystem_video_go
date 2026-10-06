package social

import (
	"context"
	"feedsystem_video_go/internal/account"
	"feedsystem_video_go/internal/db/rabbitmq"
	rediscache "feedsystem_video_go/internal/db/redis"
	"feedsystem_video_go/internal/pkg/apierror"
	"log"
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

func (s *SocialService) Follow(ctx context.Context, social *Social) error {
	if social.VloggerID == social.FollowerID {
		return apierror.ErrSocialNotFollowSelf
	}
	isExist, err := s.accountrepo.IsExist(ctx, social.VloggerID)
	if err != nil {
		return err
	}
	if !isExist {
		return apierror.ErrUserNotFound
	}

	if err := s.repo.Follow(ctx, social); err != nil {
		return err
	}

	s.invalidateFollowingFeedCache(ctx, social.FollowerID)

	// 最后发 MQ（用于通知），失败只记日志不影响业务
	if s.socialMQ != nil {
		if err := s.socialMQ.Follow(ctx, social.FollowerID, social.VloggerID); err != nil {
			log.Printf("social MQ Follow 发布失败: %v", err)
		}
	}
	return nil
}

func (s *SocialService) Unfollow(ctx context.Context, social *Social) error {
	isExist, err := s.accountrepo.IsExist(ctx, social.VloggerID)
	if err != nil {
		return err
	}
	if !isExist {
		return apierror.ErrUserNotFound
	}
	isFollowed, err := s.repo.IsFollowed(ctx, social)
	if err != nil {
		return err
	}
	if !isFollowed {
		return apierror.ErrSocialeNotExist
	}

	if err := s.repo.Unfollow(ctx, social); err != nil {
		return err
	}

	s.invalidateFollowingFeedCache(context.Background(), social.FollowerID)

	if s.socialMQ != nil {
		if err := s.socialMQ.UnFollow(ctx, social.FollowerID, social.VloggerID); err != nil {
			log.Printf("social MQ UnFollow 发布失败: %v", err)
		}
	}
	return nil
}

func (s *SocialService) invalidateFollowingFeedCache(ctx context.Context, accountID uint) {
	if s.cache == nil {
		return
	}
	pattern := s.cache.Key("feed:listByFollowing:*:accountID=%d:*", accountID)
	if err := s.cache.DelByPattern(ctx, pattern); err != nil {
		log.Printf("失效 Following 缓存失败: accountID=%d, err=%v", accountID, err)
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
