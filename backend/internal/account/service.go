package account

import (
	"context"
	rediscache "feedsystem_video_go/internal/db/redis"
	"feedsystem_video_go/internal/pkg/apierror"
	"feedsystem_video_go/internal/pkg/hash"
	"feedsystem_video_go/internal/pkg/jwt"
	"feedsystem_video_go/internal/pkg/logger"
	"log/slog"
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

func (as *AccountService) CreateAccount(ctx context.Context, username, password string) error {
	passwordHash, err := hash.HashPassword(password)
	if err != nil {
		return err
	}
	password = passwordHash
	if err := as.accountRepository.CreateAccount(ctx, username, password); err != nil {
		return err
	}
	return nil
}

func (as *AccountService) Rename(ctx context.Context, accountID uint, newUsername string) (RenameResponse, error) {
	token, err := jwt.GenerateToken(accountID, newUsername)
	if err != nil {
		return RenameResponse{}, err
	}
	refreshToken, err := jwt.GenerateRefreshToken(accountID, newUsername)
	if err != nil {
		return RenameResponse{}, err
	}

	if err := as.accountRepository.Rename(ctx, accountID, newUsername, token, refreshToken); err != nil {
		return RenameResponse{}, err
	}
	if as.cache != nil {
		cacheCtx, cancel := context.WithTimeout(ctx, 50*time.Millisecond)
		defer cancel()

		accessKey := as.cache.Key("account:%d:access", accountID)
		refreshKey := as.cache.Key("account:%d:refresh", accountID)
		if err := as.cache.SetBytes(cacheCtx, accessKey, []byte(token), 24*time.Hour); err != nil {
			logger.Info("Account : Set cache failed", slog.String("key", accessKey), slog.Any("error", err))
		}
		if err := as.cache.SetBytes(cacheCtx, refreshKey, []byte(refreshToken), 7*24*time.Hour); err != nil {
			logger.Info("Account : Set cache failed", slog.String("key", refreshKey), slog.Any("error", err))
		}
	}
	return RenameResponse{
		Token:        token,
		RefreshToken: refreshToken,
	}, nil
}

func (as *AccountService) ChangePassword(ctx context.Context, id uint, oldPassword, newPassword string) error {
	account, err := as.accountRepository.FindByID(ctx, id)
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
	return as.accountRepository.FindByID(ctx, id)
}

func (as *AccountService) FindByUsername(ctx context.Context, username string) (*Account, error) {
	return as.accountRepository.FindByUsername(ctx, username)
}

func (as *AccountService) FindSimilarName(ctx context.Context, username string, lastID uint, limit int) (FindSimilarNameResponse, error) {
	accs, err := as.accountRepository.FindSimilarName(ctx, username, lastID, limit)
	if err != nil {
		return FindSimilarNameResponse{}, err
	}
	if len(accs) == 0 {
		return FindSimilarNameResponse{NextID: lastID}, nil
	}
	return FindSimilarNameResponse{
		Accounts: accs,
		HasMore:  len(accs) == limit,
		NextID:   accs[len(accs)-1].ID,
	}, nil
}

func (as *AccountService) Login(ctx context.Context, username, password string) (LoginResponse, error) {
	account, err := as.accountRepository.FindByUsername(ctx, username)
	if err != nil {
		return LoginResponse{}, err
	}
	if !hash.ComparePassword(account.Password, password) {
		return LoginResponse{}, apierror.ErrUserPasswordWrong
	}
	accessToken, err := jwt.GenerateToken(account.ID, account.Username)
	if err != nil {
		return LoginResponse{}, err
	}
	refreshToken, err := jwt.GenerateRefreshToken(account.ID, account.Username)
	if err != nil {
		return LoginResponse{}, err
	}
	if err := as.accountRepository.Login(ctx, account.ID, accessToken, refreshToken); err != nil {
		return LoginResponse{}, err
	}
	if as.cache != nil {
		cacheCtx, cancel := context.WithTimeout(ctx, 50*time.Millisecond)
		defer cancel()

		accessKey := as.cache.Key("account:%d:access", account.ID)
		refreshKey := as.cache.Key("account:%d:refresh", account.ID)
		if err := as.cache.SetBytes(cacheCtx, accessKey, []byte(accessToken), 24*time.Hour); err != nil {
			logger.Info("Account : Set cache failed", slog.String("key", accessKey), slog.Any("error", err))
		}
		if err := as.cache.SetBytes(cacheCtx, refreshKey, []byte(refreshToken), 7*24*time.Hour); err != nil {
			logger.Info("Account : Set cache failed", slog.String("key", refreshKey), slog.Any("error", err))
		}
	}
	return LoginResponse{
		Token:        accessToken,
		RefreshToken: refreshToken,
	}, nil
}

func (as *AccountService) Logout(ctx context.Context, accountID uint) error {
	account, err := as.FindByID(ctx, accountID)
	if err != nil {
		return err
	}
	if account.Token == "" && account.RefreshToken == "" {
		return nil
	}
	if as.cache != nil {
		cacheCtx, cancel := context.WithTimeout(ctx, 50*time.Millisecond)
		defer cancel()

		accessKey := as.cache.Key("account:%d:access", account.ID)
		refreshKey := as.cache.Key("account:%d:refresh", account.ID)
		if err := as.cache.Del(cacheCtx, accessKey); err != nil {
			logger.Info("Account : Del cache failed", slog.String("key", accessKey), slog.Any("error", err))
		}
		if err := as.cache.Del(cacheCtx, refreshKey); err != nil {
			logger.Info("Account : Del cache failed", slog.String("key", refreshKey), slog.Any("error", err))
		}
	}
	return as.accountRepository.Logout(ctx, account.ID)
}

func (as *AccountService) UpdateProfile(ctx context.Context, accountID uint, bio, avatarURL string) error {
	bio = strings.TrimSpace(bio)
	avatarURL = strings.TrimSpace(avatarURL)
	return as.accountRepository.UpdateProfile(ctx, accountID, bio, avatarURL)
}

func (as *AccountService) IsExist(ctx context.Context, accountID uint) (bool, error) {
	return as.accountRepository.IsExist(ctx, accountID)
}

func (as *AccountService) RefreshAccessToken(ctx context.Context, id uint, username, refreshToken string) (string, error) {
	if as.cache != nil {
		cacheCtx, cancel := context.WithTimeout(ctx, 50*time.Millisecond)
		defer cancel()
		cacheRefresh, err := as.cache.GetBytes(cacheCtx, as.cache.Key("account:%d:refresh", id))
		if err == nil {
			if string(cacheRefresh) == refreshToken {
				newToken, err := jwt.GenerateToken(id, username)
				if err != nil {
					return "", err
				}
				if err := as.accountRepository.UpdateToken(ctx, id, newToken); err != nil {
					return "", err
				}
				key := as.cache.Key("account:%d:access", id)
				if err := as.cache.SetBytes(cacheCtx, key, []byte(newToken), 24*time.Hour); err != nil {
					logger.Info("Account : Set cache failed", slog.String("key", key), slog.Any("error", err))
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
			cacheCtx, cancel := context.WithTimeout(ctx, 50*time.Millisecond)
			defer cancel()
			key := as.cache.Key("account:%d:access", id)
			if err := as.cache.SetBytes(cacheCtx, key, []byte(newToken), 24*time.Hour); err != nil {
				logger.Info("Account : Set cache failed", slog.String("key", key), slog.Any("error", err))
			}
		}
		return newToken, nil
	}
	return "", nil
}
