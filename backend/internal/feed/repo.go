package feed

import (
	"context"
	"feedsystem_video_go/internal/social"
	"feedsystem_video_go/internal/video"
	"time"

	"gorm.io/gorm"
)

type FeedRepository struct {
	db *gorm.DB
}

func NewFeedRepository(db *gorm.DB) *FeedRepository {
	return &FeedRepository{db: db}
}

func (repo *FeedRepository) ListLatest(ctx context.Context, limit int, latestBefore time.Time) ([]*video.Video, error) {
	var videos []*video.Video
	query := repo.db.WithContext(ctx).Model(&video.Video{}).Order("create_time DESC")
	if !latestBefore.IsZero() {
		query = query.Where("create_time < ?", latestBefore)
	}
	if err := query.Limit(limit).Find(&videos).Error; err != nil {
		return nil, err
	}
	return videos, nil
}

func (repo *FeedRepository) ListLikesCount(ctx context.Context, limit int, lastID *uint, lastLikesCount *int64) ([]*video.Video, error) {
	var videos []*video.Video
	query := repo.db.WithContext(ctx).Model(&video.Video{}).
		Order("likes_count DESC, id DESC")

	if lastID != nil && lastLikesCount != nil {
		query = query.Where(
			"(likes_count < ?) OR (likes_count = ? AND id < ?)",
			lastLikesCount,
			lastLikesCount, lastID,
		)
	}

	if err := query.Limit(limit).Find(&videos).Error; err != nil {
		return nil, err
	}
	return videos, nil
}

func (repo *FeedRepository) ListByFollowing(ctx context.Context, limit int, viewerAccountID, lastID uint) ([]*video.Video, error) {
	var videos []*video.Video
	followingSubQuery := repo.db.WithContext(ctx).
		Model(&social.Social{}).
		Select("vlogger_id").
		Where("follower_id = ?", viewerAccountID)

	query := repo.db.WithContext(ctx).Model(&video.Video{}).
		Where("author_id IN (?)", followingSubQuery)
	if lastID > 0 {
		query = query.Where("id < ?", lastID)
	}
	if err := query.Order("id DESC").Limit(limit).Find(&videos).Error; err != nil {
		return nil, err
	}
	return videos, nil
}

func (repo *FeedRepository) ListByPopularity(ctx context.Context, limit int, asofTime time.Time, offset int64) ([]*video.Video, error) {
	var videos []*video.Video
	if err := repo.db.WithContext(ctx).Model(&video.Video{}).
		Where("create_time < ?", asofTime).
		Order("popularity DESC,create_time DESC").Offset(int(offset)).Limit(limit).Find(&videos).Error; err != nil {
		return nil, err
	}
	return videos, nil
}

func (repo *FeedRepository) GetByIDs(ctx context.Context, ids []uint) ([]*video.Video, error) {
	var videos []*video.Video
	if len(ids) == 0 {
		return videos, nil
	}
	if err := repo.db.WithContext(ctx).Model(&video.Video{}).
		Where("id IN ?", ids).Find(&videos).Error; err != nil {
		return nil, err
	}
	return videos, nil
}

func (repo *FeedRepository) ListByTag(ctx context.Context, tagName string, limit int, lastID uint) ([]*video.Video, error) {
	var videos []*video.Video
	query := repo.db.WithContext(ctx).Model(&video.Video{}).
		Joins("JOIN video_tags ON video_tags.video_id = videos.id").
		Joins("JOIN tags ON tags.id = video_tags.tag_id").
		Where("tags.name = ?", tagName)
	if lastID > 0 {
		query = query.Where("videos.id < ?", lastID)
	}
	if err := query.Order("videos.id desc").Limit(limit).Find(&videos).Error; err != nil {
		return nil, err
	}
	return videos, nil
}
