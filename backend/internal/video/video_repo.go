package video

import (
	"context"
	"errors"
	"feedsystem_video_go/internal/pkg/apierror"
	"strings"

	"gorm.io/gorm"
)

type VideoRepository struct {
	db *gorm.DB
}

func NewVideoRepository(db *gorm.DB) *VideoRepository {
	return &VideoRepository{db: db}
}

func (vr *VideoRepository) CreateVideo(ctx context.Context, video *Video) error {
	if err := vr.db.WithContext(ctx).Create(video).Error; err != nil {
		return err
	}
	return nil
}

func (vr *VideoRepository) DeleteVideo(ctx context.Context, id uint) error {
	return vr.db.WithContext(ctx).Delete(&Video{}, id).Error
}

func (vr *VideoRepository) ListByAuthorID(ctx context.Context, authorID, lastID uint, limit int) ([]Video, error) {
	var videos []Video
	query := vr.db.WithContext(ctx).Where("author_id = ?", authorID)
	if lastID > 0 {
		query = query.Where("id < ?", lastID)
	}
	if err := query.Order("id desc").Limit(limit).Find(&videos).Error; err != nil {
		return nil, err
	}
	return videos, nil
}

func (vr *VideoRepository) SearchByKeyword(ctx context.Context, keyword string, lastID uint, limit int) ([]Video, error) {
	var videos []Video
	escaped := strings.NewReplacer(`\`, `\\`, `%`, `\%`, `_`, `\_`).Replace(keyword)
	pattern := "%" + escaped + "%"

	query := vr.db.WithContext(ctx).
		Where(`title LIKE ? ESCAPE '\\' OR description LIKE ? ESCAPE '\\'`, pattern, pattern)
	if lastID > 0 {
		query = query.Where("id < ?", lastID)
	}
	if err := query.Order("id desc").Limit(limit).Find(&videos).Error; err != nil {
		return nil, err
	}
	return videos, nil
}

func (vr *VideoRepository) GetByID(ctx context.Context, id uint) (*Video, error) {
	var video Video
	if err := vr.db.WithContext(ctx).First(&video, id).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, apierror.ErrVideoNotFound
		}
		return nil, err
	}
	return &video, nil
}

func (vr *VideoRepository) IsExist(ctx context.Context, id uint) (bool, error) {
	var count int64
	if err := vr.db.WithContext(ctx).Model(&Video{}).Where("id = ?", id).Count(&count).Error; err != nil {
		return false, err
	}
	return count > 0, nil
}

func (vr *VideoRepository) CountByAuthor(ctx context.Context, authorID uint) (int64, error) {
	var count int64
	if err := vr.db.WithContext(ctx).Model(&Video{}).Where("author_id = ?", authorID).Count(&count).Error; err != nil {
		return 0, err
	}
	return count, nil
}

func (vr *VideoRepository) TotalLikesByAuthor(ctx context.Context, authorID uint) (int64, error) {
	var total int64
	if err := vr.db.WithContext(ctx).Model(&Video{}).Select("COALESCE(SUM(likes_count), 0)").Where("author_id = ?", authorID).Scan(&total).Error; err != nil {
		return 0, err
	}
	return total, nil
}
