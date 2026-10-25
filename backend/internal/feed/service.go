package feed

import (
	"context"
	"encoding/json"
	"errors"
	rediscache "feedsystem_video_go/internal/db/redis"
	"feedsystem_video_go/internal/pkg/logger"
	"feedsystem_video_go/internal/video"
	"fmt"
	"log/slog"
	"slices"
	"strconv"
	"time"

	"github.com/patrickmn/go-cache"
	"github.com/redis/go-redis/v9"
	"golang.org/x/sync/singleflight"
)

type FeedService struct {
	repo         *FeedRepository
	likeRepo     *video.LikeRepository
	rediscache   *rediscache.Client
	localcache   *cache.Cache
	cacheTTL     time.Duration
	requestGroup singleflight.Group
}

type CachedFeedData struct {
	PublicVideos []video.Video `json:"public_videos"`
}

func NewFeedService(repo *FeedRepository, likeRepo *video.LikeRepository, rediscache *rediscache.Client) *FeedService {
	return &FeedService{repo: repo, likeRepo: likeRepo, rediscache: rediscache, localcache: cache.New(5*time.Second, 10*time.Second), cacheTTL: 24 * time.Hour}
}

// ListLatest 查询最新视频 (冷热分离 + 游标分页)
func (f *FeedService) ListLatest(ctx context.Context, limit int, latestBefore time.Time, viewerAccountID uint) (ListLatestResponse, error) {
	listLatestFromDB := func(ctx context.Context, limit int, latestBefore time.Time, viewerAccountID uint) (ListLatestResponse, error) {
		videos, err := f.repo.ListLatest(ctx, limit, latestBefore)
		if err != nil {
			return ListLatestResponse{}, err
		}
		if len(videos) == 0 {
			return ListLatestResponse{}, nil
		}
		feedVideos, err := f.buildFeedVideos(ctx, videos, viewerAccountID)
		if err != nil {
			return ListLatestResponse{}, err
		}
		return ListLatestResponse{
			VideoList: feedVideos,
			NextTime:  videos[len(videos)-1].CreateTime.UnixMilli(),
			HasMore:   len(videos) == limit,
		}, nil
	}

	if f.rediscache == nil {
		return listLatestFromDB(ctx, limit, latestBefore, viewerAccountID)
	}

	// 获取 ZSET 中最老的一条数据
	cacheKey := f.rediscache.Key("feed:listByLatest")
	zsetTail, err := f.rediscache.ZRangeWithScores(ctx, cacheKey, 0, 0)

	if err != nil {
		return listLatestFromDB(ctx, limit, latestBefore, viewerAccountID)
	}

	if len(zsetTail) == 0 {
		v, err, _ := f.requestGroup.Do("ListLatest", func() (any, error) {
			dbVideos, err := f.repo.ListLatest(ctx, 1000, time.Time{})
			if err != nil {
				return nil, err
			}
			if len(dbVideos) == 0 {
				return "EMPTY_DB", nil // 防无限递归
			}

			// 重建 ZSET
			var zElements []redis.Z
			for _, vid := range dbVideos {
				zElements = append(zElements, redis.Z{
					Score:  float64(vid.CreateTime.UnixMilli()),
					Member: fmt.Sprintf("%d", vid.ID),
				})
			}
			bgCtx, cancel := context.WithTimeout(ctx, 2*time.Second)
			defer cancel()
			if err := f.rediscache.ZAdd(bgCtx, cacheKey, zElements...); err != nil {
				logger.Info("Feed : Zadd cache failed", slog.String("key", cacheKey), slog.Any("error", err))
			}
			return dbVideos, nil
		})

		if err != nil && !errors.Is(err, errors.New("zadd failed")) {
			return ListLatestResponse{}, err
		}
		if v == "EMPTY_DB" {
			return ListLatestResponse{}, nil
		}
		videos := v.([]*video.Video)
		feedVideos, err := f.buildFeedVideos(ctx, videos, viewerAccountID)
		if err != nil {
			return ListLatestResponse{}, err
		}
		return ListLatestResponse{
			VideoList: feedVideos,
			NextTime:  videos[len(videos)-1].CreateTime.UnixMilli(),
			HasMore:   len(videos) == limit,
		}, nil

	}

	watermark := int64(zsetTail[0].Score)
	var baseVideos []*video.Video

	if latestBefore.UnixMilli() <= watermark {
		//冷数据降级查库
		sfKey := fmt.Sprintf("cold:listLatest:%d:%d", limit, latestBefore.UnixMilli())
		v, err, _ := f.requestGroup.Do(sfKey, func() (any, error) {
			return f.repo.ListLatest(ctx, limit, latestBefore)
		})
		if err != nil {
			return ListLatestResponse{}, err
		}
		baseVideos = v.([]*video.Video) // 不回写 ZSET，防止冷数据污染热点时间线
	} else {
		// 热数据直接查redis
		maxScore := fmt.Sprintf("%d", latestBefore.UnixMilli()-1) // 防重复

		videoIDsStr, err := f.rediscache.ZRevRangeByScore(ctx, cacheKey, maxScore, "-inf", 0, int64(limit))
		if err != nil {
			return ListLatestResponse{}, err
		}

		var videoIDs []uint
		for _, idStr := range videoIDsStr {
			if id, err := strconv.ParseUint(idStr, 10, 64); err == nil {
				videoIDs = append(videoIDs, uint(id))
			}
		}

		if len(videoIDs) > 0 {
			baseVideos, err = f.getVideoByIDs(ctx, videoIDs)
			if err != nil {
				return ListLatestResponse{}, err
			}
		}

		// 刚好击穿了冷热边界
		if len(baseVideos) < limit {
			remainLimit := limit - len(baseVideos) // 计算还差几个

			var coldCursor time.Time
			if len(baseVideos) > 0 {
				coldCursor = baseVideos[len(baseVideos)-1].CreateTime
			} else {
				coldCursor = latestBefore
			}

			sfKey := fmt.Sprintf("break:listLatest:%d:%d", remainLimit, coldCursor.UnixMilli())
			v, err, _ := f.requestGroup.Do(sfKey, func() (any, error) {
				return f.repo.ListLatest(ctx, remainLimit, coldCursor)
			})

			if err == nil {
				coldVideos := v.([]*video.Video)
				baseVideos = append(baseVideos, coldVideos...)
			}
		}
	}
	if len(baseVideos) == 0 {
		return ListLatestResponse{}, nil
	}

	var nextTime int64
	nextTime = baseVideos[len(baseVideos)-1].CreateTime.UnixMilli()
	hasMore := len(baseVideos) == limit
	feedVideos, err := f.buildFeedVideos(ctx, baseVideos, viewerAccountID)
	if err != nil {
		return ListLatestResponse{}, err
	}
	return ListLatestResponse{
		VideoList: feedVideos,
		NextTime:  nextTime,
		HasMore:   hasMore,
	}, nil
}

// ListLikesCount 按照点赞数查询视频
func (f *FeedService) ListLikesCount(ctx context.Context, limit int, lastID *uint, lastLikesCount *int64, viewerAccountID uint) (ListLikesCountResponse, error) {
	key := fmt.Sprintf("ListlikesCount:limit=%d:last_id=%d:lastLikesCount=%d", limit, lastID, lastLikesCount)
	v, err, _ := f.requestGroup.Do(key, func() (any, error) {
		return f.repo.ListLikesCount(ctx, limit, lastID, lastLikesCount)
	})
	if err != nil {
		return ListLikesCountResponse{}, err
	}
	videos := v.([]*video.Video)
	if len(videos) == 0 {
		return ListLikesCountResponse{}, nil
	}
	feedVideos, err := f.buildFeedVideos(ctx, videos, viewerAccountID)
	if err != nil {
		return ListLikesCountResponse{}, err
	}
	res := ListLikesCountResponse{
		VideoList:      feedVideos,
		HasMore:        len(videos) == limit,
		NextID:         &videos[len(videos)-1].ID,
		NextLikesCount: &videos[len(videos)-1].LikesCount,
	}
	return res, nil
}

// ListByFollowing 按照关注列表查询视频
func (f *FeedService) ListByFollowing(ctx context.Context, limit int, lastID uint, viewerAccountID uint) (ListByFollowingResponse, error) {
	listByFollowingFromDB := func() (ListByFollowingResponse, error) {
		videos, err := f.repo.ListByFollowing(ctx, limit, viewerAccountID, lastID)
		if err != nil {
			return ListByFollowingResponse{}, err
		}
		if len(videos) == 0 {
			return ListByFollowingResponse{NextID: lastID}, nil
		}
		feedVideos, err := f.buildFeedVideos(ctx, videos, viewerAccountID)
		if err != nil {
			return ListByFollowingResponse{}, err
		}
		return ListByFollowingResponse{
			VideoList: feedVideos,
			NextID:    videos[len(videos)-1].ID,
			HasMore:   len(videos) == limit,
		}, nil
	}

	var cacheKey string
	if f.rediscache != nil {
		cacheKey = f.rediscache.Key("feed:listByFollowing:accountID=%d:limit=%d:lastID=%d", viewerAccountID, limit, lastID)

		b, err := f.rediscache.GetBytes(ctx, cacheKey)
		if err == nil {
			var cached ListByFollowingResponse
			if err := json.Unmarshal(b, &cached); err == nil {
				return cached, nil
			}
		} else if rediscache.IsMiss(err) { // 缓存未命中
			lockKey := f.rediscache.Key("lock:%s", cacheKey)
			// 缓存未命中，尝试加锁
			token, locked, _ := f.rediscache.Lock(ctx, lockKey, 5*time.Second)
			if locked {
				defer func() {
					ctx, cancel := context.WithTimeout(context.Background(), time.Second)
					defer cancel()
					_ = f.rediscache.Unlock(ctx, lockKey, token)
				}()
				if b, err := f.rediscache.GetBytes(ctx, cacheKey); err == nil {
					var cached ListByFollowingResponse
					if err := json.Unmarshal(b, &cached); err == nil {
						return cached, nil
					}
				} else { // 缓存未命中，从数据库中查询
					resp, err := listByFollowingFromDB()
					if err != nil {
						return ListByFollowingResponse{}, err
					}
					if b, err := json.Marshal(resp); err == nil {
						ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
						defer cancel()
						if err := f.rediscache.SetBytes(ctx, cacheKey, b, f.cacheTTL); err != nil {
							logger.Info("Feed : Set cache failed", slog.String("key", cacheKey), slog.Any("error", err))
						}
					}
					return resp, nil
				}
			} else {
				for range 5 {
					time.Sleep(20 * time.Millisecond)
					if b, err := f.rediscache.GetBytes(ctx, cacheKey); err == nil {
						var cached ListByFollowingResponse
						if err := json.Unmarshal(b, &cached); err == nil {
							return cached, nil
						}
					}
				}
			}
		}
	}

	resp, err := listByFollowingFromDB()
	if err != nil {
		return ListByFollowingResponse{}, err
	}
	if cacheKey != "" {
		if b, err := json.Marshal(resp); err == nil {
			cacheCtx, cancel := context.WithTimeout(ctx, 50*time.Millisecond)
			defer cancel()
			if err := f.rediscache.SetBytes(cacheCtx, cacheKey, b, f.cacheTTL); err != nil {
				logger.Info("Feed : Set cache failed", slog.String("key", cacheKey), slog.Any("error", err))
			}
		}
	}
	return resp, nil
}

func (f *FeedService) ListByPopularity(ctx context.Context, limit int, asof, offset int64, viewerAccountID uint) (ListByPopularityResponse, error) {
	asofTime := time.Now().Truncate(time.Minute)
	if asof > 0 {
		asofTime = time.Unix(asof, 0).Truncate(time.Minute)
	}
	// Redis 热榜（稳定分页：as_of + offset）
	if f.rediscache != nil {
		dest := f.rediscache.Key("video:hot:merge:1m:%s", asofTime.Format("200601021504")) // 快照key：同一个as_of页内复用
		exists, _ := f.rediscache.Exists(ctx, dest)
		if !exists {
			lockKey := f.rediscache.Key("lock:%s", dest)
			token, lock, err := f.rediscache.Lock(ctx, lockKey, 5*time.Second)
			if err == nil && lock {
				defer func() {
					ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
					defer cancel()
					_ = f.rediscache.Unlock(ctx, lockKey, token)
				}()
				exists, _ := f.rediscache.Exists(ctx, dest)
				if !exists {
					const win = 60
					keys := make([]string, 0, win)
					for i := range win {
						keys = append(keys, f.rediscache.Key("video:hot:1m:%s", asofTime.Add(-time.Duration(i)*time.Minute).Format("200601021504")))
					}
					if err := f.rediscache.ZUnionStore(ctx, dest, keys, "SUM"); err != nil {
						logger.Info("Feed : ZUnionStore cache failed", slog.String("key", dest), slog.Any("error", err))
					}
					cacheCtx, cancel := context.WithTimeout(ctx, 50*time.Millisecond)
					if err := f.rediscache.Expire(cacheCtx, dest, 2*time.Minute); err != nil {
						logger.Info("Feed : Set cache expire failed", slog.String("key", dest), slog.Any("error", err))
					}
					cancel()
				}
			}
		}

		start := offset
		stop := start + int64(limit) - 1
		members, err := f.rediscache.ZRevRange(ctx, dest, start, stop)
		if err == nil && len(members) > 0 {
			ids := make([]uint, 0, len(members))
			for _, m := range members {
				u, err := strconv.ParseUint(m, 10, 64)
				if err == nil && u > 0 {
					ids = append(ids, uint(u))
				}
			}

			videos, err := f.repo.GetByIDs(ctx, ids)
			if err == nil {
				byID := make(map[uint]*video.Video, len(videos))
				for _, v := range videos {
					byID[v.ID] = v
				}
				ordered := buildOrderedResult(ids, byID)
				feedVideos, err := f.buildFeedVideos(ctx, ordered, viewerAccountID)
				if err != nil {
					return ListByPopularityResponse{}, err
				}
				return ListByPopularityResponse{
					VideoList:  feedVideos,
					NextOffset: offset + int64(len(members)),
					Asof:       asofTime.Unix(),
					HasMore:    len(members) == limit,
				}, err
			}
		}
	}

	key := fmt.Sprintf("listByPopularity:limit=%d:asof=%d:offset=%d", limit, asof, offset)
	data, err, _ := f.requestGroup.Do(key, func() (any, error) {
		videos, err := f.repo.ListByPopularity(ctx, limit, asofTime, offset)
		if err != nil {
			return nil, err
		}
		return videos, nil
	})
	videos, ok := data.([]*video.Video)
	if !ok {
		return ListByPopularityResponse{}, err
	}
	if len(videos) == 0 {
		return ListByPopularityResponse{}, nil
	}
	items, err := f.buildFeedVideos(ctx, videos, viewerAccountID)
	if err != nil {
		return ListByPopularityResponse{}, err
	}
	resp := ListByPopularityResponse{
		VideoList: items,
		HasMore:   len(items) == limit,
		Asof:      asofTime.Unix(),
	}
	resp.NextOffset = offset + int64(len(videos))
	return resp, nil
}

func (f *FeedService) ListByTag(ctx context.Context, tagName string, limit int, lastID uint, viewerAccountID uint) (ListByTagResponse, error) {
	videos, err := f.repo.ListByTag(ctx, tagName, limit, lastID)
	if err != nil {
		return ListByTagResponse{}, err
	}
	if len(videos) == 0 {
		return ListByTagResponse{}, nil
	}
	var res ListByTagResponse
	res.VideoList, err = f.buildFeedVideos(ctx, videos, viewerAccountID)
	if err != nil {
		return ListByTagResponse{}, err
	}
	res.HasMore = len(videos) == limit
	res.NextID = videos[len(videos)-1].ID
	return res, nil
}

func (f *FeedService) getVideoByIDs(ctx context.Context, videoIDs []uint) ([]*video.Video, error) {
	// GetVideoByIDs 批量获取视频信息
	// 采用 L1(本地缓存) -> L2(Redis) -> L3(MySQL) 三级架构
	if len(videoIDs) == 0 {
		return []*video.Video{}, nil
	}

	videoMap := make(map[uint]*video.Video)
	//L1:本地缓存
	var missedL1 []uint
	for _, id := range videoIDs {
		cacheKey := f.rediscache.Key("video:entity:%d", id)
		if f.localcache != nil {
			if v, found := f.localcache.Get(cacheKey); found {
				if data, ok := v.(video.Video); ok {
					videoMap[id] = &data
					continue
				}
			}
		}
		// 记录未命中的 ID，准备进入下一级缓存
		missedL1 = append(missedL1, id)
	}

	if len(missedL1) == 0 {
		return buildOrderedResult(videoIDs, videoMap), nil
	}

	//L2:redis
	var missedL2 []uint
	cacheKeys := make([]string, len(missedL1))
	for i, id := range missedL1 {
		cacheKeys[i] = f.rediscache.Key("video:entity:%d", id)
	}

	cacheCtx, cancel := context.WithTimeout(ctx, 50*time.Millisecond)
	results, err := f.rediscache.MGet(cacheCtx, cacheKeys...)
	cancel()

	if err == nil {
		for i, res := range results {
			id := missedL1[i]
			if res != nil {
				if str, ok := res.(string); ok {
					var v video.Video
					if err := json.Unmarshal([]byte(str), &v); err == nil {
						videoMap[id] = &v
						// 回写更新 L1 本地缓存
						if f.localcache != nil {
							f.localcache.Set(cacheKeys[i], v, 5*time.Second)
						}
						continue
					}
				}
			}
			missedL2 = append(missedL2, id)
		}
	} else {
		// 如果 Redis 挂了或者超时了，全部降级到 L3
		missedL2 = missedL1
		logger.Info("Feed : L2 Redis MGet 失败，全部降级到 MySQL", slog.Any("error", err))
	}

	if len(missedL2) == 0 {
		return buildOrderedResult(videoIDs, videoMap), nil
	}

	// L3: MySQL —— 一次 IN 查询批量取回所有未命中 ID
	sfKey := fmt.Sprintf("getVideoByIDs:%v", batchIDsKey(missedL2))
	v, err, _ := f.requestGroup.Do(sfKey, func() (any, error) {
		videos, err := f.repo.GetByIDs(ctx, missedL2)
		if err != nil {
			return nil, err
		}

		for _, vv := range videos {
			safeCopy := *vv
			cacheKey := f.rediscache.Key("video:entity:%d", safeCopy.ID)
			// 回写 L1 本地缓存
			if f.localcache != nil {
				f.localcache.Set(cacheKey, safeCopy, 5*time.Second)
			}
			// 异步回写 L2 Redis
			if b, err := json.Marshal(safeCopy); err == nil {
				go func(k string, b []byte) {
					setCtx, setCancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
					defer setCancel()

					if err := f.rediscache.SetBytes(setCtx, k, b, video.EntityCacheTTL); err != nil {
						logger.Info("Feed : Set cache failed", slog.String("key", k), slog.Any("error", err))
					}
				}(cacheKey, b)
			}
		}
		return videos, nil
	})

	if err != nil {
		logger.Info("Feed : L3 MySQL 批量查询失败", slog.Any("error", err))
		return buildOrderedResult(videoIDs, videoMap), nil
	}

	for _, vv := range v.([]*video.Video) {
		safeCopy := *vv
		videoMap[safeCopy.ID] = &safeCopy
	}

	return buildOrderedResult(videoIDs, videoMap), nil
}

func (f *FeedService) buildFeedVideos(ctx context.Context, videos []*video.Video, viewerAccountID uint) ([]FeedVideoItem, error) {
	feedVideos := make([]FeedVideoItem, 0, len(videos))
	videoIDs := make([]uint, len(videos))
	for i, v := range videos {
		videoIDs[i] = v.ID
	}
	likedMap, err := f.likeRepo.BatchGetLiked(ctx, videoIDs, viewerAccountID)
	if err != nil {
		return nil, err
	}
	for _, v := range videos {
		feedVideos = append(feedVideos, FeedVideoItem{
			ID:          v.ID,
			Author:      FeedAuthor{ID: v.AuthorID, Username: v.Username},
			Title:       v.Title,
			Description: v.Description,
			PlayURL:     v.PlayURL,
			CoverURL:    v.CoverURL,
			CreateTime:  v.CreateTime.UnixMilli(),
			LikesCount:  v.LikesCount,
			IsLiked:     likedMap[v.ID],
		})
	}
	return feedVideos, nil
}

func buildOrderedResult(orderedIDs []uint, dataMap map[uint]*video.Video) []*video.Video {
	res := make([]*video.Video, 0, len(orderedIDs))
	for _, id := range orderedIDs {
		if v, exits := dataMap[id]; exits && v != nil {
			res = append(res, v)
		}
	}
	return res
}

func batchIDsKey(ids []uint) string {
	sorted := make([]uint, len(ids))
	copy(sorted, ids)
	slices.Sort(sorted)
	return fmt.Sprintf("%v", sorted)
}
