package mw

import (
	"context"
	"errors"
	"feedsystem_video_go/internal/account"
	rediscache "feedsystem_video_go/internal/db/redis"
	"feedsystem_video_go/internal/pkg/apierror"
	"feedsystem_video_go/internal/pkg/jwt"
	"feedsystem_video_go/internal/pkg/response"
	"log"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
)

func JWTAuthHandler(repo *account.AccountRepository, cache *rediscache.Client, isHard bool) gin.HandlerFunc {
	return func(c *gin.Context) {
		authHeader := c.GetHeader("Authorization")
		if authHeader == "" {
			if isHard {
				response.FailWithDataByErrCode(c, apierror.ErrUnauthorized, gin.H{"error": "missing authorization header"})
				c.Abort()
				return
			}
			c.Next()
			return
		}

		parts := strings.SplitN(authHeader, " ", 2)
		if len(parts) != 2 || !strings.EqualFold(parts[0], "Bearer") {
			response.FailWithDataByErrCode(c, apierror.ErrUnauthorized, gin.H{"error": "invalid authorization header"})
			c.Abort()
			return
		}

		tokenString := parts[1]

		claims, err := jwt.ParseToken(tokenString)
		if err != nil {
			response.FailWithDataByErrCode(c, apierror.ErrUnauthorized, gin.H{"error": "invalid or expired token"})
			c.Abort()
			return
		}

		key := cache.Key("account:%d:access", claims.AccountID)
		// 先查 Redis
		if cache != nil {
			cacheCtx, cancel := context.WithTimeout(c.Request.Context(), 50*time.Millisecond)
			defer cancel()

			b, err := cache.GetBytes(cacheCtx, key)
			if err == nil {
				if string(b) != tokenString {
					response.FailWithDataByErrCode(c, apierror.ErrUnauthorized, gin.H{"error": "token has been revoked"})
					c.Abort()
					return
				}
				c.Set("accountID", claims.AccountID)
				c.Set("username", claims.Username)
				c.Next()
				return
			}
		}

		// Redis 故障/未启用：查 DB 兜底
		accountInfo, err := repo.FindByID(c.Request.Context(), claims.AccountID)
		if err != nil || accountInfo.Token == "" || accountInfo.Token != tokenString {
			response.FailWithDataByErrCode(c, apierror.ErrUnauthorized, gin.H{"error": "token has been revoked"})
			c.Abort()
			return
		}

		if cache != nil {
			cacheCtx, cancel := context.WithTimeout(c.Request.Context(), 50*time.Millisecond)
			defer cancel()

			if err := cache.SetBytes(cacheCtx, key, []byte(tokenString), 24*time.Hour); err != nil {
				log.Printf("failed to set cache: %v", err)
			}
		}

		c.Set("accountID", claims.AccountID)
		c.Set("username", claims.Username)
		c.Next()
	}
}

func GetAccountIDByGIN(c *gin.Context) (uint, error) {
	uidValue, exists := c.Get("accountID")
	if !exists {
		return 0, errors.New("accountID not found")
	}

	accountID, ok := uidValue.(uint)
	if !ok {
		return 0, errors.New("accountID has invalid type")
	}

	return accountID, nil
}

func GetUsernameByGIN(c *gin.Context) (string, error) {
	val, exists := c.Get("username")
	if !exists {
		return "", errors.New("username not found")
	}

	username, ok := val.(string)
	if !ok {
		return "", errors.New("username has invalid type")
	}

	return username, nil
}
