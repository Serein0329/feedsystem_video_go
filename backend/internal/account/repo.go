package account

import (
	"context"
	"errors"
	"feedsystem_video_go/internal/pkg/apierror"
	"strings"

	"gorm.io/gorm"
)

type AccountRepository struct {
	db *gorm.DB
}

func NewAccountRepository(db *gorm.DB) *AccountRepository {
	return &AccountRepository{db: db}
}

func (ar *AccountRepository) CreateAccount(ctx context.Context, username, password string) error {
	if err := ar.db.WithContext(ctx).Create(&Account{Username: username, Password: password}).Error; err != nil {
		if errors.Is(err, gorm.ErrDuplicatedKey) {
			return apierror.ErrUserExist
		}
		return err
	}
	return nil
}

func (ar *AccountRepository) Rename(ctx context.Context, id uint, newUsername, token, refreshToken string) error {
	return ar.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		result := tx.Model(&Account{}).Where("id = ?", id).Update("username", newUsername)
		if result.Error != nil {
			if errors.Is(result.Error, gorm.ErrDuplicatedKey) {
				return apierror.ErrUserExist
			}
			return result.Error
		}
		if result.RowsAffected == 0 {
			return apierror.New(apierror.ErrUserNoFieldsToUpdate.HttpStatus, apierror.ErrUserNoFieldsToUpdate.Code, "与原用户名一样")
		}
		if err := tx.Model(&Account{}).Where("id = ?", id).Updates(map[string]any{
			"token":         token,
			"refresh_token": refreshToken,
		}).Error; err != nil {
			return err
		}
		return nil
	})
}

func (ar *AccountRepository) ChangePassword(ctx context.Context, id uint, newPassword string) error {
	return ar.db.WithContext(ctx).Model(&Account{}).Where("id = ?", id).Update("password", newPassword).Error
}

func (ar *AccountRepository) FindByID(ctx context.Context, id uint) (*Account, error) {
	var account Account
	if err := ar.db.WithContext(ctx).First(&account, id).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, apierror.ErrUserNotFound
		}
		return nil, err
	}
	return &account, nil
}

func (ar *AccountRepository) FindByUsername(ctx context.Context, username string) (*Account, error) {
	var account Account
	if err := ar.db.WithContext(ctx).Where("username = ?", username).First(&account).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, apierror.ErrUserNotFound
		}
		return nil, err
	}
	return &account, nil
}

func (ar *AccountRepository) FindSimilarName(ctx context.Context, username string, lastID uint, limit int) ([]*Account, error) {
	var account []*Account
	escaped := strings.NewReplacer(`\`, `\\`, `%`, `\%`, `_`, `\_`).Replace(username)
	query := ar.db.WithContext(ctx).Where(`username LIKE ? ESCAPE '\\'`, "%"+escaped+"%")
	if lastID > 0 {
		query = query.Where("id > ?", lastID)
	}
	if err := query.Order("id").Limit(limit).Find(&account).Error; err != nil {
		return nil, err
	}
	return account, nil
}

func (ar *AccountRepository) IsExist(ctx context.Context, id uint) (bool, error) {
	var count int64
	if err := ar.db.WithContext(ctx).Model(&Account{}).Where("id = ?", id).Count(&count).Error; err != nil {
		return false, err
	}
	return count > 0, nil
}

func (ar *AccountRepository) Login(ctx context.Context, id uint, token, refreshToken string) error {
	return ar.db.WithContext(ctx).Model(&Account{}).Where("id = ?", id).Updates(map[string]any{"token": token, "refresh_token": refreshToken}).Error
}

func (ar *AccountRepository) Logout(ctx context.Context, id uint) error {
	return ar.db.WithContext(ctx).Model(&Account{}).Where("id = ?", id).Updates(map[string]any{"token": "", "refresh_token": ""}).Error
}

func (ar *AccountRepository) UpdateToken(ctx context.Context, id uint, token string) error {
	return ar.db.WithContext(ctx).Model(&Account{}).Where("id = ?", id).Update("token", token).Error
}

func (ar *AccountRepository) UpdateProfile(ctx context.Context, id uint, bio, avatarURL string) error {
	return ar.db.WithContext(ctx).Model(&Account{}).Where("id = ?", id).Updates(map[string]any{
		"bio":        bio,
		"avatar_url": avatarURL,
	}).Error
}

func (ar *AccountRepository) FindAll(ctx context.Context) ([]*Account, error) {
	var accounts []*Account
	if err := ar.db.WithContext(ctx).Find(&accounts).Error; err != nil {
		return nil, err
	}
	return accounts, nil
}
