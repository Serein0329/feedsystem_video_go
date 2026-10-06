package account

import (
	"errors"
	"feedsystem_video_go/internal/pkg/apierror"
	"feedsystem_video_go/internal/pkg/jwt"
	"feedsystem_video_go/internal/pkg/randhex"
	"feedsystem_video_go/internal/pkg/response"
	"fmt"
	"os"
	"path"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"
)

type AccountHandler struct {
	accountService *AccountService
}

func NewAccountHandler(accountService *AccountService) *AccountHandler {
	return &AccountHandler{accountService: accountService}
}
func (h *AccountHandler) CreateAccount(c *gin.Context) {
	var req CreateAccountRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.FailWithDataByErrCode(c, apierror.ErrBadRequest, gin.H{"error": err.Error()})
		return
	}
	if err := h.accountService.CreateAccount(c.Request.Context(), &Account{
		Username: req.Username,
		Password: req.Password,
	}); err != nil {
		response.FailByErr(c, err)
		return
	}
	response.Success(c, nil)
}

func (h *AccountHandler) Rename(c *gin.Context) {
	var req RenameRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.FailWithDataByErrCode(c, apierror.ErrBadRequest, gin.H{"error": err.Error()})
		return
	}
	accountID, _ := getAccountIDByGIN(c)
	token, err := h.accountService.Rename(c.Request.Context(), accountID, req.NewUsername)
	if err != nil {
		response.FailByErr(c, err)
		return
	}
	response.Success(c, gin.H{"token": token})
}

func (h *AccountHandler) ChangePassword(c *gin.Context) {
	id, _ := getAccountIDByGIN(c)
	var req ChangePasswordRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.FailWithDataByErrCode(c, apierror.ErrBadRequest, gin.H{"error": err.Error()})
		return
	}
	if err := h.accountService.ChangePassword(c.Request.Context(), id, req.OldPassword, req.NewPassword); err != nil {
		response.FailByErr(c, err)
		return
	}
	response.Success(c, nil)
}

func (h *AccountHandler) FindByID(c *gin.Context) {
	var req FindByIDRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.FailWithDataByErrCode(c, apierror.ErrBadRequest, gin.H{"error": err.Error()})
		return
	}
	if account, err := h.accountService.FindByID(c.Request.Context(), req.ID); err != nil {
		response.FailByErr(c, err)
	} else {
		response.Success(c, account)
	}
}

func (h *AccountHandler) FindByUsername(c *gin.Context) {
	var req FindByUsernameRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.FailWithDataByErrCode(c, apierror.ErrBadRequest, gin.H{"error": err.Error()})
		return
	}
	if account, err := h.accountService.FindByUsername(c.Request.Context(), req.Username); err != nil {
		response.FailByErr(c, err)
	} else {
		response.Success(c, account)
	}
}

func (h *AccountHandler) Login(c *gin.Context) {
	var req LoginRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.FailWithDataByErrCode(c, apierror.ErrBadRequest, gin.H{"error": err.Error()})
		return
	}
	accessToken, refreshToken, err := h.accountService.Login(c.Request.Context(), req.Username, req.Password)
	if err != nil {
		response.FailByErr(c, err)
		return
	}
	response.Success(c, LoginResponse{Token: accessToken, RefreshToken: refreshToken})
}

func (h *AccountHandler) Logout(c *gin.Context) {
	accountID, _ := getAccountIDByGIN(c)
	if err := h.accountService.Logout(c.Request.Context(), accountID); err != nil {
		response.FailByErr(c, err)
		return
	}
	response.Success(c, nil)
}

func (h *AccountHandler) UploadAvatar(c *gin.Context) {
	accountID, _ := getAccountIDByGIN(c)
	f, err := c.FormFile("file")
	if err != nil {
		response.FailWithDataByErrCode(c, apierror.ErrBadRequest, gin.H{"error": "missing file"})
		return
	}
	const maxSize = 10 << 20
	if f.Size <= 0 || f.Size > maxSize {
		response.FailWithDataByErrCode(c, apierror.ErrBadRequest, gin.H{"error": "file size exceeds 10MB limit"})
		return
	}
	ext := strings.ToLower(filepath.Ext(f.Filename))
	switch ext {
	case ".jpg", ".jpeg", ".png", ".webp":
	default:
		response.FailWithDataByErrCode(c, apierror.ErrBadRequest, gin.H{"error": "only .jpg/.jpeg/.png/.webp allowed"})
		return
	}
	dir := filepath.Join(".run", "uploads", "avatars", strconv.FormatUint(uint64(accountID), 10))
	if err := os.MkdirAll(dir, 0o755); err != nil {
		response.FailByErrCode(c, apierror.ErrInternalServer.WithInternal(err))
		return
	}
	filename, err := randhex.New(16)
	if err != nil {
		response.FailByErrCode(c, apierror.ErrInternalServer.WithInternal(err))
		return
	}
	filename = filename + ext
	absPath := filepath.Join(dir, filename)
	if err := c.SaveUploadedFile(f, absPath); err != nil {
		response.FailByErrCode(c, apierror.ErrInternalServer.WithInternal(err))
		return
	}
	urlPath := path.Join("/static", "avatars", strconv.FormatUint(uint64(accountID), 10), filename)
	avatarURL := buildAbsoluteURL(c, urlPath)
	if err := h.accountService.UpdateAvatar(c.Request.Context(), accountID, avatarURL); err != nil {
		response.FailByErrCode(c, apierror.ErrInternalServer.WithInternal(err))
		return
	}
	response.Success(c, gin.H{"avatar_url": avatarURL})
}

func (h *AccountHandler) UpdateProfile(c *gin.Context) {
	accountID, _ := getAccountIDByGIN(c)
	var req UpdateProfileRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.FailWithDataByErrCode(c, apierror.ErrBadRequest, gin.H{"error": err.Error()})
		return
	}

	if err := h.accountService.UpdateProfile(c.Request.Context(), accountID, &req); err != nil {
		response.FailByErr(c, err)
		return
	}
	response.Success(c, nil)
}

func (h *AccountHandler) Refresh(c *gin.Context) {
	var req RefreshRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.FailWithDataByErrCode(c, apierror.ErrBadRequest, gin.H{"error": err.Error()})
		return
	}

	claims, err := jwt.ParseToken(req.RefreshToken)
	if err != nil {
		response.FailWithDataByErrCode(c, apierror.ErrUnauthorized, gin.H{"error": "invalid or expired token"})
		return
	}

	newToken, err := h.accountService.RefreshAccessToken(c.Request.Context(), claims.AccountID, req.RefreshToken)
	if err != nil {
		response.FailByErr(c, err)
		return
	}
	if newToken == "" {
		response.FailWithDataByErrCode(c, apierror.ErrBadRequest, gin.H{"error": "invalid token"})
	} else {
		response.Success(c, gin.H{"token": newToken})
	}
}

func buildAbsoluteURL(c *gin.Context, p string) string {
	scheme := "http"
	if c.Request.TLS != nil {
		scheme = "https"
	}
	if xf := c.GetHeader("X-Forwarded-Proto"); xf != "" {
		scheme = xf
	}
	return fmt.Sprintf("%s://%s%s", scheme, c.Request.Host, p)
}

func getAccountIDByGIN(c *gin.Context) (uint, error) {
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
