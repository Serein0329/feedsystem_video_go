package video

import (
	"context"
	rediscache "feedsystem_video_go/internal/db/redis"
	"feedsystem_video_go/internal/pkg/logger"
	"log/slog"
	"strconv"
	"time"
)

// UpdatePopularityCache 更新视频流行度缓存
func UpdatePopularityCache(ctx context.Context, cache *rediscache.Client, id uint, change int64) {
	if cache == nil || id == 0 || change == 0 {
		return
	}

	cacheKey := cache.Key("video:entity:%d", id)
	if err := cache.Del(ctx, cacheKey); err != nil {
		logger.Info("Del cache failed", slog.String("key", cacheKey), slog.Any("error", err))
	}
	now := time.Now()
	windowKey := cache.Key("video:hot:%s", now.Format("200601021504"))
	member := strconv.FormatUint(uint64(id), 10)

	opCtx, cancel := context.WithTimeout(ctx, 50*time.Millisecond)
	defer cancel()

	_ = cache.ZincrBy(opCtx, windowKey, member, float64(change))
	_ = cache.Expire(opCtx, windowKey, 2*time.Hour)
}
