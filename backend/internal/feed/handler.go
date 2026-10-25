package feed

import (
	"feedsystem_video_go/internal/mw"
	"feedsystem_video_go/internal/pkg/apierror"
	"feedsystem_video_go/internal/pkg/response"
	"time"

	"github.com/gin-gonic/gin"
)

type FeedHandler struct {
	service *FeedService
}

func NewFeedHandler(service *FeedService) *FeedHandler {
	return &FeedHandler{service: service}
}

func (f *FeedHandler) ListLatest(c *gin.Context) {
	var req ListLatestRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.FailWithDataByErrCode(c, apierror.ErrBadRequest, gin.H{"error": err.Error()})
		return
	}
	if req.Limit <= 0 || req.Limit > 50 {
		req.Limit = 20
	}
	latestTime := time.Now()
	if req.LatestTime > 0 {
		latestTime = time.UnixMilli(req.LatestTime)
	}
	viewerAccountID, _ := mw.GetAccountIDByGIN(c)
	res, err := f.service.ListLatest(c.Request.Context(), req.Limit, latestTime, viewerAccountID)
	if err != nil {
		response.FailByErrCode(c, apierror.ErrInternalServer.WithInternal(err))
		return
	}
	res.VideoList = nonNilFeedVideoItems(res.VideoList)
	response.Success(c, res)
}

func (f *FeedHandler) ListLikesCount(c *gin.Context) {
	var req ListLikesCountRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.FailWithDataByErrCode(c, apierror.ErrBadRequest, gin.H{"error": err.Error()})
		return
	}
	if req.Limit <= 0 || req.Limit > 50 {
		req.Limit = 20
	}

	if req.LastLikesCount != nil || req.LastID != nil {
		if req.LastLikesCount == nil || req.LastID == nil {
			response.FailWithDataByErrCode(c, apierror.ErrBadRequest, gin.H{"error": "likes_count_before and id_before must be provided together"})
			return
		}
	}
	viewerAccountID, _ := mw.GetAccountIDByGIN(c)
	res, err := f.service.ListLikesCount(c.Request.Context(), req.Limit, req.LastID, req.LastLikesCount, viewerAccountID)
	if err != nil {
		response.FailByErrCode(c, apierror.ErrInternalServer.WithInternal(err))
		return
	}
	res.VideoList = nonNilFeedVideoItems(res.VideoList)
	response.Success(c, res)
}

func (f *FeedHandler) ListByFollowing(c *gin.Context) {
	var req ListByFollowingRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.FailWithDataByErrCode(c, apierror.ErrBadRequest, gin.H{"error": err.Error()})
		return
	}
	if req.Limit <= 0 || req.Limit > 50 {
		req.Limit = 20
	}
	viewerAccountID, _ := mw.GetAccountIDByGIN(c)
	res, err := f.service.ListByFollowing(c.Request.Context(), req.Limit, req.LatestID, viewerAccountID)
	if err != nil {
		response.FailByErrCode(c, apierror.ErrInternalServer.WithInternal(err))
		return
	}
	res.VideoList = nonNilFeedVideoItems(res.VideoList)
	response.Success(c, res)
}

func (f *FeedHandler) ListByPopularity(c *gin.Context) {
	var req ListByPopularityRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.FailWithDataByErrCode(c, apierror.ErrBadRequest, gin.H{"error": err.Error()})
		return
	}
	if req.Limit <= 0 || req.Limit > 50 {
		req.Limit = 20
	}

	viewerAccountID, _ := mw.GetAccountIDByGIN(c)
	res, err := f.service.ListByPopularity(c.Request.Context(), req.Limit, req.Asof, req.Offset, viewerAccountID)
	if err != nil {
		response.FailByErrCode(c, apierror.ErrInternalServer.WithInternal(err))
		return
	}
	res.VideoList = nonNilFeedVideoItems(res.VideoList)
	response.Success(c, res)
}

func (f *FeedHandler) ListByTag(c *gin.Context) {
	var req ListByTagRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.FailWithDataByErrCode(c, apierror.ErrBadRequest, gin.H{"error": err.Error()})
		return
	}
	if req.Limit <= 0 || req.Limit > 50 {
		req.Limit = 20
	}
	viewerAccountID, _ := mw.GetAccountIDByGIN(c)
	resp, err := f.service.ListByTag(c.Request.Context(), req.TagName, req.Limit, req.LastID, viewerAccountID)
	if err != nil {
		response.FailByErrCode(c, apierror.ErrInternalServer.WithInternal(err))
		return
	}
	resp.VideoList = nonNilFeedVideoItems(resp.VideoList)
	response.Success(c, resp)
}

func nonNilFeedVideoItems(items []FeedVideoItem) []FeedVideoItem {
	if items == nil {
		return []FeedVideoItem{}
	}
	return items
}
