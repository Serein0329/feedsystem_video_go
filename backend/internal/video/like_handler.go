package video

import (
	"feedsystem_video_go/internal/mw"
	"feedsystem_video_go/internal/pkg/apierror"
	"feedsystem_video_go/internal/pkg/response"

	"github.com/gin-gonic/gin"
)

type LikeHandler struct {
	service *LikeService
}

func NewLikeHandler(service *LikeService) *LikeHandler {
	return &LikeHandler{service: service}
}

func (lh *LikeHandler) Like(c *gin.Context) {
	var req LikeRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.FailWithDataByErrCode(c, apierror.ErrBadRequest, gin.H{"error": err.Error()})
		return
	}

	accountID, _ := mw.GetAccountIDByGIN(c)
	username, _ := mw.GetUsernameByGIN(c)
	if err := lh.service.Like(c.Request.Context(), req.VideoID, accountID, username); err != nil {
		response.FailByErr(c, err)
		return
	}
	response.Success(c, nil)
}

func (lh *LikeHandler) Unlike(c *gin.Context) {
	var req LikeRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.FailWithDataByErrCode(c, apierror.ErrBadRequest, gin.H{"error": err.Error()})
		return
	}
	accountID, _ := mw.GetAccountIDByGIN(c)
	if err := lh.service.Unlike(c.Request.Context(), req.VideoID, accountID); err != nil {
		response.FailByErr(c, err)
		return
	}
	response.Success(c, nil)
}

func (lh *LikeHandler) IsLiked(c *gin.Context) {
	var req LikeRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.FailWithDataByErrCode(c, apierror.ErrBadRequest, gin.H{"error": err.Error()})
		return
	}

	accountID, _ := mw.GetAccountIDByGIN(c)
	isLiked, err := lh.service.IsLiked(c.Request.Context(), req.VideoID, accountID)
	if err != nil {
		response.FailByErr(c, err)
		return
	}
	response.Success(c, gin.H{"is_liked": isLiked})
}

func (lh *LikeHandler) ListMyLikedVideos(c *gin.Context) {
	var req ListMyLikedVideosRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.FailWithDataByErrCode(c, apierror.ErrBadRequest, gin.H{"error": err.Error()})
		return
	}

	if req.Limit <= 0 || req.Limit > 50 {
		req.Limit = 20
	}
	accountID, _ := mw.GetAccountIDByGIN(c)
	res, err := lh.service.ListLikedVideos(c.Request.Context(), accountID, req.LastID, req.Limit)
	if err != nil {
		response.FailByErrCode(c, apierror.ErrInternalServer.WithInternal(err))
		return
	}
	if res.Videos == nil {
		res.Videos = []Video{}
	}
	response.SuccessWithPage(c, res.Videos, res.HasMore, res.LastID)
}
