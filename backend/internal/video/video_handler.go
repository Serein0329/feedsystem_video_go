package video

import (
	"feedsystem_video_go/internal/mw"
	"feedsystem_video_go/internal/pkg/apierror"
	"feedsystem_video_go/internal/pkg/randhex"
	"feedsystem_video_go/internal/pkg/response"
	"fmt"
	"os"
	"path"
	"path/filepath"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
)

type VideoHandler struct {
	service *VideoService
}

func NewVideoHandler(service *VideoService) *VideoHandler {
	return &VideoHandler{service: service}
}

func (vh *VideoHandler) PublishVideo(c *gin.Context) {
	var req PublishVideoRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.FailWithDataByErrCode(c, apierror.ErrBadRequest, gin.H{"error": err.Error()})
		return
	}

	req.PlayURL = strings.TrimSpace(req.PlayURL)
	req.CoverURL = strings.TrimSpace(req.CoverURL)
	req.Title = strings.TrimSpace(req.Title)
	if len(req.Title) == 0 {
		response.FailWithDataByErrCode(c, apierror.ErrBadRequest, gin.H{"error": "标题不为空"})
		return
	}

	authorId, _ := mw.GetAccountIDByGIN(c)
	username, _ := mw.GetUsernameByGIN(c)
	video := &Video{
		AuthorID:    authorId,
		Username:    username,
		Title:       req.Title,
		Description: req.Description,
		PlayURL:     req.PlayURL,
		CoverURL:    req.CoverURL,
		CreateTime:  time.Now(),
	}
	if err := vh.service.Publish(c.Request.Context(), video); err != nil {
		response.FailByErrCode(c, apierror.ErrInternalServer.WithInternal(err))
		return
	}
	response.Success(c, video)
}

func (vh *VideoHandler) UploadVideo(c *gin.Context) {
	f, err := c.FormFile("file")
	if err != nil {
		response.FailWithDataByErrCode(c, apierror.ErrBadRequest, gin.H{"error": "missing file"})
		return
	}

	const maxSize = 200 << 20
	if f.Size <= 0 || f.Size > maxSize {
		response.FailWithDataByErrCode(c, apierror.ErrBadRequest, gin.H{"error": "file size max 200MB"})
		return
	}

	ext := strings.ToLower(filepath.Ext(f.Filename))
	if ext != ".mp4" {
		response.FailWithDataByErrCode(c, apierror.ErrBadRequest, gin.H{"error": "only .mp4 is allowed"})
		return
	}
	authorId, _ := mw.GetAccountIDByGIN(c)
	date := time.Now().Format("20060102")
	relDir := filepath.Join("videos", fmt.Sprintf("%d", authorId), date)
	root := filepath.Join(".run", "uploads")
	absDir := filepath.Join(root, relDir)
	if err := os.MkdirAll(absDir, 0o755); err != nil {
		response.FailByErrCode(c, apierror.ErrInternalServer.WithInternal(err))
		return
	}

	filename, err := randhex.New(16)
	if err != nil {
		response.FailByErrCode(c, apierror.ErrInternalServer.WithInternal(err))
		return
	}
	filename = filename + ext
	absPath := filepath.Join(absDir, filename)

	if err := c.SaveUploadedFile(f, absPath); err != nil {
		response.FailByErrCode(c, apierror.ErrInternalServer.WithInternal(err))
		return
	}

	urlPath := path.Join("/static", "videos", fmt.Sprintf("%d", authorId), date, filename)

	response.Success(c, gin.H{"play_url": buildAbsoluteURL(c, urlPath)})
}

func (vh *VideoHandler) UploadCover(c *gin.Context) {
	f, err := c.FormFile("file")
	if err != nil {
		response.FailWithDataByErrCode(c, apierror.ErrBadRequest, gin.H{"error": "missing file"})
		return
	}

	const maxSize = 10 << 20
	if f.Size <= 0 || f.Size > maxSize {
		response.FailWithDataByErrCode(c, apierror.ErrBadRequest, gin.H{"error": "file size max 10MB"})
		return
	}

	ext := strings.ToLower(filepath.Ext(f.Filename))
	switch ext {
	case ".jpg", ".jpeg", ".png", ".webp":
	default:
		response.FailWithDataByErrCode(c, apierror.ErrBadRequest, gin.H{"error": "only .jpg/.jpeg/.webp allowed"})
		return
	}

	authorId, _ := mw.GetAccountIDByGIN(c)
	date := time.Now().Format("20060102")
	relDir := filepath.Join("covers", fmt.Sprintf("%d", authorId), date)
	root := filepath.Join(".run", "uploads")
	absDir := filepath.Join(root, relDir)
	if err := os.MkdirAll(absDir, 0o755); err != nil {
		response.FailByErrCode(c, apierror.ErrInternalServer.WithInternal(err))
		return
	}

	filename, err := randhex.New(16)
	if err != nil {
		response.FailByErrCode(c, apierror.ErrInternalServer.WithInternal(err))
		return
	}
	filename = filename + ext
	absPath := filepath.Join(absDir, filename)

	if err := c.SaveUploadedFile(f, absPath); err != nil {
		response.FailByErrCode(c, apierror.ErrInternalServer.WithInternal(err))
		return
	}
	urlPath := path.Join("/static", "covers", fmt.Sprintf("%d", authorId), date, filename)

	response.Success(c, gin.H{"cover_url": buildAbsoluteURL(c, urlPath)})
}

func (vh *VideoHandler) SearchVideos(c *gin.Context) {
	var req SearchVideosRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.FailWithDataByErrCode(c, apierror.ErrBadRequest, gin.H{"error": err.Error()})
		return
	}
	req.Keyword = strings.TrimSpace(req.Keyword)
	if req.Keyword == "" {
		response.FailWithDataByErrCode(c, apierror.ErrBadRequest, gin.H{"error": "搜索关键词不能为空"})
		return
	}
	if req.Limit <= 0 || req.Limit > 50 {
		req.Limit = 20
	}
	res, err := vh.service.SearchVideos(c.Request.Context(), req.Keyword, req.LastID, req.Limit)
	if err != nil {
		response.FailByErr(c, err)
		return
	}
	if res.Videos == nil {
		res.Videos = []Video{}
	}
	response.SuccessWithPage(c, res.Videos, res.HasMore, res.NextID)
}

func (vh *VideoHandler) DeleteVideo(c *gin.Context) {
	var req DeleteVideoRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.FailWithDataByErrCode(c, apierror.ErrBadRequest, gin.H{"error": err.Error()})
		return
	}
	authorId, _ := mw.GetAccountIDByGIN(c)
	if err := vh.service.Delete(c.Request.Context(), req.ID, authorId); err != nil {
		response.FailByErr(c, err)
		return
	}
	response.Success(c, nil)
}

func (vh *VideoHandler) ListByAuthorID(c *gin.Context) {
	var req ListByAuthorIDRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.FailWithDataByErrCode(c, apierror.ErrBadRequest, gin.H{"error": err.Error()})
		return
	}

	if req.Limit <= 0 || req.Limit > 50 {
		req.Limit = 20
	}
	res, err := vh.service.ListByAuthorID(c.Request.Context(), req.AuthorID, req.LastID, req.Limit)
	if err != nil {
		response.FailByErr(c, err)
		return
	}
	if res.Videos == nil {
		res.Videos = []Video{}
	}
	response.SuccessWithPage(c, res.Videos, res.HasMore, res.NextID)
}

func (vh *VideoHandler) GetByID(c *gin.Context) {
	var req GetByIDRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.FailWithDataByErrCode(c, apierror.ErrBadRequest, gin.H{"error": err.Error()})
		return
	}
	video, err := vh.service.GetByID(c.Request.Context(), req.ID)
	if err != nil {
		response.FailByErr(c, err)
		return
	}
	response.Success(c, video)
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
