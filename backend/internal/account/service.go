package account

import (
	"context"
	rediscache "feedsystem_video_go/internal/db/redis"
	"feedsystem_video_go/internal/pkg/apierror"
	"feedsystem_video_go/internal/pkg/hash"
	"feedsystem_video_go/internal/pkg/jwt"
	"log"
	"strings"
	"time"
)

type AccountService struct {
	accountRepository *AccountRepository
	cache             *rediscache.Client
}

func NewAccountService(accountRepository *AccountRepository, cache *rediscache.Client) *AccountService {
	return &AccountService{accountRepository: accountRepository, cache: cache}
}

func (as *AccountService) CreateAccount(ctx context.Context, account *Account) error {
	passwordHash, err := hash.HashPassword(account.Password)
	if err != nil {
		return err
	}
	account.Password = passwordHash
	if err := as.accountRepository.CreateAccount(ctx, account); err != nil {
		return err
	}
	return nil
}

func (as *AccountService) Rename(ctx context.Context, accountID uint, newUsername string) (string, error) {
	token, err := jwt.GenerateToken(accountID, newUsername)
	if err != nil {
		return "", err
	}

	if err := as.accountRepository.RenameWithToken(ctx, accountID, newUsername, token); err != nil {
		return "", err
	}
	if as.cache != nil {
		cacheCtx, cancel := context.WithTimeout(ctx, 50*time.Millisecond)
		defer cancel()

		key := as.cache.Key("account:%d:access", accountID)
		if err := as.cache.SetBytes(cacheCtx, key, []byte(token), 24*time.Hour); err != nil {
			log.Printf("failed to set cache key:%s, err:%v", key, err)
		}
	}
	return token, nil
}

func (as *AccountService) ChangePassword(ctx context.Context, id uint, oldPassword, newPassword string) error {
	account, err := as.FindByID(ctx, id)
	if err != nil {
		return err
	}
	if !hash.ComparePassword(account.Password, oldPassword) {
		return apierror.ErrUserPasswordWrong
	}
	passwordHash, err := hash.HashPassword(newPassword)
	if err != nil {
		return err
	}
	if err := as.accountRepository.ChangePassword(ctx, account.ID, passwordHash); err != nil {
		return err
	}
	if err := as.Logout(ctx, account.ID); err != nil {
		return err
	}
	return nil
}

func (as *AccountService) FindByID(ctx context.Context, id uint) (*Account, error) {
	if account, err := as.accountRepository.FindByID(ctx, id); err != nil {
		return nil, err
	} else {
		return account, nil
	}
}

func (as *AccountService) FindByUsername(ctx context.Context, username string) (*Account, error) {
	if account, err := as.accountRepository.FindByUsername(ctx, username); err != nil {
		return nil, err
	} else {
		return account, nil
	}
}

func (as *AccountService) Login(ctx context.Context, username, password string) (string, string, error) {
	account, err := as.FindByUsername(ctx, username)
	if err != nil {
		return "", "", err
	}
	if !hash.ComparePassword(account.Password, password) {
		return "", "", apierror.ErrUserPasswordWrong
	}
	accessToken, err := jwt.GenerateToken(account.ID, account.Username)
	if err != nil {
		return "", "", err
	}
	refreshToken, err := jwt.GenerateRefreshToken(account.ID, account.Username)
	if err != nil {
		return "", "", err
	}
	if err := as.accountRepository.Login(ctx, account.ID, accessToken, refreshToken); err != nil {
		return "", "", err
	}
	if as.cache != nil {
		cacheCtx, cancel := context.WithTimeout(ctx, 50*time.Millisecond)
		defer cancel()

		accessKey := as.cache.Key("account:%d:access", account.ID)
		refreshKey := as.cache.Key("account:%d:refresh", account.ID)
		if err := as.cache.SetBytes(cacheCtx, accessKey, []byte(accessToken), 24*time.Hour); err != nil {
			log.Printf("failed to set cache key:%s, err:%v", accessKey, err)
		}
		if err := as.cache.SetBytes(cacheCtx, refreshToken, []byte(refreshToken), 7*24*time.Hour); err != nil {
			log.Printf("failed to set cache key:%s, err:%v", refreshKey, err)
		}
	}
	return accessToken, refreshToken, nil
}

func (as *AccountService) Logout(ctx context.Context, accountID uint) error {
	account, err := as.FindByID(ctx, accountID)
	if err != nil {
		return err
	}
	if account.Token == "" {
		return nil
	}
	if as.cache != nil {
		cacheCtx, cancel := context.WithTimeout(ctx, 50*time.Millisecond)
		defer cancel()

		accessKey := as.cache.Key("account:%d:access", account.ID)
		refreshKey := as.cache.Key("account:%d:refresh", account.ID)
		if err := as.cache.Del(cacheCtx, accessKey); err != nil {
			log.Printf("failed to del cache key:%s, err:%v", accessKey, err)
		}
		if err := as.cache.Del(cacheCtx, refreshKey); err != nil {
			log.Printf("failed to del cache key:%s, err:%v", refreshKey, err)
		}
	}
	return as.accountRepository.Logout(ctx, account.ID)
}

func (as *AccountService) UpdateAvatar(ctx context.Context, accountID uint, avatarURL string) error {
	return as.accountRepository.UpdateAvatar(ctx, accountID, avatarURL)
}

func (as *AccountService) FindAll(ctx context.Context) ([]*Account, error) {
	return as.accountRepository.FindAll(ctx)
}

func (as *AccountService) UpdateProfile(ctx context.Context, accountID uint, req *UpdateProfileRequest) error {
	updates := map[string]any{}
	if req.Bio != "" {
		updates["bio"] = strings.TrimSpace(req.Bio)
	}
	if req.AvatarURL != "" {
		updates["avatar_url"] = strings.TrimSpace(req.AvatarURL)
	}
	if len(updates) == 0 {
		return apierror.ErrUserNoFieldsToUpdate
	}
	return as.accountRepository.UpdateFields(ctx, accountID, updates)
}

func (as *AccountService) RefreshAccessToken(ctx context.Context, id uint, refreshToken string) (string, error) {
	if as.cache != nil {
		cacheCtx, cancel := context.WithTimeout(ctx, 50*time.Millisecond)
		defer cancel()
		cacheRefresh, err := as.cache.GetBytes(cacheCtx, as.cache.Key("account:%d:refresh", id))
		if err == nil {
			if string(cacheRefresh) == refreshToken {
				account, err := as.FindByID(ctx, id)
				if err != nil {
					return "", err
				}
				newToken, err := jwt.GenerateToken(id, account.Username)
				if err != nil {
					return "", err
				}
				if err := as.accountRepository.UpdateToken(ctx, id, newToken); err != nil {
					return "", err
				}
				key := as.cache.Key("account:%d:access", id)
				if err := as.cache.SetBytes(cacheCtx, key, []byte(newToken), 24*time.Hour); err != nil {
					log.Printf("failed to set cache key:%s, err:%v", key, err)
				}
				return newToken, nil
			}
		}
	}

	accounts, err := as.FindByID(ctx, id)
	if err != nil {
		return "", err
	}
	if accounts.RefreshToken == refreshToken {
		newToken, err := jwt.GenerateToken(id, accounts.Username)
		if err != nil {
			return "", err
		}
		if err := as.accountRepository.UpdateToken(ctx, id, newToken); err != nil {
			return "", err
		}
		if as.cache != nil {
			key := as.cache.Key("account:%d:access", id)
			if err := as.cache.SetBytes(ctx, key, []byte(newToken), 24*time.Hour); err != nil {
				log.Printf("failed to set cache key:%s, err:%v", key, err)
			}
		}
		return newToken, nil
	}
	return "", nil
}
