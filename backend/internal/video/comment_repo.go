package video

import (
	"context"
	"errors"
	"feedsystem_video_go/internal/pkg/apierror"

	"gorm.io/gorm"
)

type CommentRepository struct {
	db *gorm.DB
}

func NewCommentRepository(db *gorm.DB) *CommentRepository {
	return &CommentRepository{db: db}
}

func (r *CommentRepository) CreateComment(ctx context.Context, comment *Comment) error {
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Create(comment).Error; err != nil {
			return err
		}
		return tx.Model(&Video{}).Where("id = ?", comment.VideoID).
			UpdateColumn("popularity", gorm.Expr("popularity + 1")).Error
	})
}

func (r *CommentRepository) DeleteComment(ctx context.Context, comment *Comment) error {
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		res := tx.Delete(&Comment{}, comment.ID)
		if res.Error != nil {
			return res.Error
		}
		if res.RowsAffected == 0 {
			return nil
		}
		return tx.Model(&Video{}).Where("id = ?", comment.VideoID).
			UpdateColumn("popularity", gorm.Expr("GREATEST(popularity - 1, 0)")).Error
	})
}

func (r *CommentRepository) GetAll(ctx context.Context, videoID, lastID uint, limit int) ([]Comment, error) {
	var comments []Comment
	query := r.db.WithContext(ctx).Where("video_id = ?", videoID)
	if lastID > 0 {
		query = query.Where("id < ?", lastID)
	}
	if err := query.Order("id desc").Limit(limit).Find(&comments).Error; err != nil {
		return nil, err
	}
	return comments, nil
}

func (r *CommentRepository) IsExist(ctx context.Context, commentID uint) (bool, error) {
	var count int64
	if err := r.db.WithContext(ctx).Model(&Comment{}).Where("id = ?", commentID).Count(&count).Error; err != nil {
		return false, err
	}
	return count > 0, nil
}

func (r *CommentRepository) GetByID(ctx context.Context, id uint) (*Comment, error) {
	var comment Comment
	if err := r.db.WithContext(ctx).First(&comment, id).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, apierror.ErrCommentNotFound
		}
		return nil, err
	}
	return &comment, nil
}
