package video

import (
	"context"
	"encoding/json"
	"errors"
	"feedsystem_video_go/internal/db/rabbitmq"
	rediscache "feedsystem_video_go/internal/db/redis"
	"feedsystem_video_go/internal/pkg/apierror"
	"feedsystem_video_go/internal/pkg/logger"
	"log/slog"
	"strings"
	"time"

	"github.com/redis/go-redis/v9"
	"gorm.io/gorm"
)

type VideoService struct {
	repo       *VideoRepository
	cache      *rediscache.Client
	cacheTTL   time.Duration
	timelineMQ *rabbitmq.TimelineMQ
}

func NewVideoService(repo *VideoRepository, cache *rediscache.Client, popularityMQ *rabbitmq.TimelineMQ) *VideoService {
	return &VideoService{repo: repo, cache: cache, cacheTTL: 5 * time.Minute, timelineMQ: popularityMQ}
}

func (vs *VideoService) Publish(ctx context.Context, video *Video) error {
	video.Title = strings.TrimSpace(video.Title)
	video.PlayURL = strings.TrimSpace(video.PlayURL)
	video.CoverURL = strings.TrimSpace(video.CoverURL)

	tags := ExtractTags(video.Description)
	if err := vs.repo.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Create(video).Error; err != nil {
			return err
		}
		for _, tagName := range tags {
			var tag Tag
			if err := tx.Where("name = ?", tagName).FirstOrCreate(&tag, Tag{Name: tagName}).Error; err != nil {
				return err
			}
			if err := tx.Create(&VideoTag{VideoID: video.ID, TagID: tag.ID}).Error; err != nil {
				return err
			}
		}
		return nil
	}); err != nil {
		return err
	}
	if err := vs.timelineMQ.PublishVideo(ctx, video.ID, video.CreateTime); err != nil {
		logger.Info("publish msg failed",
			slog.String("excahnge", rabbitmq.TimelineExchange),
			slog.String("bindingkey", rabbitmq.TimelineBindingKey),
			slog.Any("error", err))
	}
	return nil
}

func (vs *VideoService) SearchVideos(ctx context.Context, keyword string, lastID uint, limit int) (SearchVideosResponse, error) {
	videos, err := vs.repo.SearchByKeyword(ctx, keyword, lastID, limit)
	if err != nil {
		return SearchVideosResponse{}, err
	}
	if len(videos) == 0 {
		// 保留请求传入的游标，避免客户端把 last_id=0 当成“重新从头查询”
		return SearchVideosResponse{LastID: lastID}, nil
	}
	return SearchVideosResponse{
		Videos:  videos,
		HasMore: len(videos) == limit,
		LastID:  videos[len(videos)-1].ID,
	}, nil
}

func (vs *VideoService) Delete(ctx context.Context, id uint, authorID uint) error {
	video, err := vs.repo.GetByID(ctx, id)
	if err != nil {
		return err
	}
	if video.AuthorID != authorID {
		return apierror.ErrForbidden
	}
	if err := vs.repo.DeleteVideo(ctx, id); err != nil {
		return err
	}
	if vs.cache != nil {
		cahceCtx, cancel := context.WithTimeout(ctx, 50*time.Millisecond)
		defer cancel()
		cacheKey := vs.cache.Key("video:entity:%d", id)
		if err = vs.cache.Del(cahceCtx, cacheKey); err != nil {
			logger.Info("Del cache failed", slog.String("key", cacheKey), slog.Any("error", err))
		}
	}
	return nil
}

func (vs *VideoService) ListByAuthorID(ctx context.Context, authorID uint) ([]Video, error) {
	videos, err := vs.repo.ListByAuthorID(ctx, authorID)
	if err != nil {
		return nil, err
	}
	return videos, nil
}

func (vs *VideoService) GetDetail(ctx context.Context, id uint) (*Video, error) {
	cacheKey := vs.cache.Key("video:entity:%d", id)

	getCached := func() (*Video, error) {
		opCtx, cancel := context.WithTimeout(ctx, 50*time.Millisecond)
		defer cancel()

		b, err := vs.cache.GetBytes(opCtx, cacheKey)
		if err != nil {
			return nil, err
		}
		var cached Video
		if err := json.Unmarshal(b, &cached); err != nil {
			return nil, err
		}
		return &cached, err
	}

	setCached := func(video *Video) {
		b, err := json.Marshal(video)
		if err != nil {
			return
		}
		opCtx, cancel := context.WithTimeout(ctx, 50*time.Millisecond)
		defer cancel()
		if err := vs.cache.SetBytes(opCtx, cacheKey, b, vs.cacheTTL); err != nil {
			logger.Info("Set cache failed", slog.String("key", cacheKey), slog.Any("error", err))
		}
	}

	if vs.cache != nil {
		if v, err := getCached(); err == nil {
			return v, nil
		} else if errors.Is(err, redis.Nil) {
			lockKey := vs.cache.Key("lock:%s", cacheKey)

			lockCtx, lockCancel := context.WithTimeout(ctx, 50*time.Millisecond)
			token, locked, lockErr := vs.cache.Lock(lockCtx, lockKey, 2*time.Second)
			lockCancel()

			if lockErr == nil && locked {
				defer func() { _ = vs.cache.Unlock(ctx, lockKey, token) }()
				if v, err := getCached(); err == nil {
					return v, nil
				}

				video, err := vs.repo.GetByID(ctx, id)
				if err != nil {
					return nil, err
				}
				setCached(video)
				return video, nil
			}

			// 没拿到锁：等待别人回填缓存
			for range 5 {
				select {
				case <-ctx.Done():
					return nil, ctx.Err()
				case <-time.After(20 * time.Millisecond):
				}
				if v, err := getCached(); err == nil {
					return v, nil
				}
			}
		}
	}

	video, err := vs.repo.GetByID(ctx, id)
	if err != nil {
		return nil, err
	}
	if vs.cache != nil {
		setCached(video)
	}
	return video, nil
}
