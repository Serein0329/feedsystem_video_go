package social

import (
	"context"
	"errors"
	"feedsystem_video_go/internal/account"

	"gorm.io/gorm"
)

type SocialRepository struct {
	db *gorm.DB
}

func NewSocialRepository(db *gorm.DB) *SocialRepository {
	return &SocialRepository{db: db}
}

func (r *SocialRepository) Follow(ctx context.Context, followerID, vloggerID uint) error {
	if err := r.db.WithContext(ctx).Create(&Social{
		FollowerID: followerID,
		VloggerID:  vloggerID,
	}).Error; err != nil {
		if errors.Is(err, gorm.ErrDuplicatedKey) {
			return nil
		}
		return err
	}
	return nil
}

func (r *SocialRepository) Unfollow(ctx context.Context, followerID, vloggerID uint) error {
	return r.db.WithContext(ctx).
		Where("follower_id = ? AND vlogger_id = ?", followerID, vloggerID).
		Delete(&Social{}).Error
}

func (r *SocialRepository) IsFollowed(ctx context.Context, followerID, vloggerID uint) (bool, error) {
	var count int64
	if err := r.db.WithContext(ctx).
		Model(&Social{}).
		Where("follower_id = ? AND vlogger_id = ?", followerID, vloggerID).
		Count(&count).Error; err != nil {
		return false, err
	}
	return count > 0, nil
}

func (r *SocialRepository) GetAllFollowers(ctx context.Context, VloggerID uint) ([]*account.Account, error) {
	var followers []*account.Account
	if err := r.db.WithContext(ctx).
		Model(&account.Account{}).
		Select("accounts.*").
		Joins("JOIN socials s ON s.follower_id = accounts.id").
		Where("s.vlogger_id = ?", VloggerID).
		Limit(200).
		Find(&followers).Error; err != nil {
		return nil, err
	}
	return followers, nil
}

func (r *SocialRepository) GetAllVloggers(ctx context.Context, FollowerID uint) ([]*account.Account, error) {
	var vloggers []*account.Account
	if err := r.db.WithContext(ctx).
		Model(&account.Account{}).
		Select("accounts.*").
		Joins("JOIN socials s ON s.vlogger_id = accounts.id").
		Where("s.follower_id = ?", FollowerID).
		Limit(200).
		Find(&vloggers).Error; err != nil {
		return nil, err
	}
	return vloggers, nil
}

func (r *SocialRepository) CountFollowers(ctx context.Context, vloggerID uint) (int64, error) {
	var count int64
	if err := r.db.WithContext(ctx).Model(&Social{}).Where("vlogger_id = ?", vloggerID).Count(&count).Error; err != nil {
		return 0, err
	}
	return count, nil
}

func (r *SocialRepository) CountVloggers(ctx context.Context, followerID uint) (int64, error) {
	var count int64
	if err := r.db.WithContext(ctx).Model(&Social{}).Where("follower_id = ?", followerID).Count(&count).Error; err != nil {
		return 0, err
	}
	return count, nil
}
