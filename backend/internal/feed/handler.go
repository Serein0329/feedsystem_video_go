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
		req.Limit = 10
	}
	var latestTime time.Time
	if req.LatestTime > 0 {
		latestTime = time.UnixMilli(req.LatestTime)
	}
	viewerAccountID, _ := mw.GetAccountIDByGIN(c)
	feedItems, err := f.service.ListLatest(c.Request.Context(), req.Limit, latestTime, viewerAccountID)
	if err != nil {
		response.FailByErrCode(c, apierror.ErrInternalServer.WithInternal(err))
		return
	}
	feedItems.VideoList = nonNilFeedVideoItems(feedItems.VideoList)
	response.Success(c, feedItems)
}

func (f *FeedHandler) ListLikesCount(c *gin.Context) {
	var req ListLikesCountRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.FailWithDataByErrCode(c, apierror.ErrBadRequest, gin.H{"error": err.Error()})
		return
	}
	if req.Limit <= 0 || req.Limit > 50 {
		req.Limit = 10
	}

	var cursor *LikesCountCursor
	if req.LikesCountBefore != nil || req.IDBefore != nil {
		if req.LikesCountBefore == nil || req.IDBefore == nil {
			response.FailWithDataByErrCode(c, apierror.ErrBadRequest, gin.H{"error": "likes_count_before and id_before must be provided together"})
			return
		}
		cursor = &LikesCountCursor{
			LikesCount: *req.LikesCountBefore,
			ID:         *req.IDBefore,
		}
	}
	viewerAccountID, _ := mw.GetAccountIDByGIN(c)
	feedItems, err := f.service.ListLikesCount(c.Request.Context(), req.Limit, cursor, viewerAccountID)
	if err != nil {
		response.FailByErrCode(c, apierror.ErrInternalServer.WithInternal(err))
		return
	}
	feedItems.VideoList = nonNilFeedVideoItems(feedItems.VideoList)
	response.Success(c, feedItems)
}

func (f *FeedHandler) ListByFollowing(c *gin.Context) {
	var req ListByFollowingRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.FailWithDataByErrCode(c, apierror.ErrBadRequest, gin.H{"error": err.Error()})
		return
	}
	if req.Limit <= 0 || req.Limit > 50 {
		req.Limit = 10
	}
	viewerAccountID, _ := mw.GetAccountIDByGIN(c)
	var latestTime time.Time
	if req.LatestTime > 0 {
		latestTime = time.Unix(req.LatestTime, 0)
	}
	feedItems, err := f.service.ListByFollowing(c.Request.Context(), req.Limit, latestTime, viewerAccountID)
	if err != nil {
		response.FailByErrCode(c, apierror.ErrInternalServer.WithInternal(err))
		return
	}
	feedItems.VideoList = nonNilFeedVideoItems(feedItems.VideoList)
	response.Success(c, feedItems)
}

func (f *FeedHandler) ListByPopularity(c *gin.Context) {
	var req ListByPopularityRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.FailWithDataByErrCode(c, apierror.ErrBadRequest, gin.H{"error": err.Error()})
		return
	}
	if req.Limit <= 0 || req.Limit > 50 {
		req.Limit = 10
	}
	viewerAccountID, _ := mw.GetAccountIDByGIN(c)

	var latestPopularity int64
	var latestBefore time.Time
	var latestIDBefore uint
	anyCursor := !req.LatestBefore.IsZero() || req.LatestIDBefore != nil
	if anyCursor {
		if req.LatestBefore.IsZero() || req.LatestIDBefore == nil {
			response.FailWithDataByErrCode(c, apierror.ErrBadRequest, gin.H{"error": "latest_before and latest_id_before must be provided together"})
			return
		}
		latestPopularity = req.LatestPopularity
		latestBefore = req.LatestBefore
		latestIDBefore = *req.LatestIDBefore
	}
	resp, err := f.service.ListByPopularity(
		c.Request.Context(),
		req.Limit,
		req.AsOf,
		req.Offset,
		viewerAccountID,
		latestPopularity,
		latestBefore,
		latestIDBefore,
	)
	if err != nil {
		response.FailByErrCode(c, apierror.ErrInternalServer.WithInternal(err))
		return
	}
	resp.VideoList = nonNilFeedVideoItems(resp.VideoList)
	response.Success(c, resp)
}

func (f *FeedHandler) ListByTag(c *gin.Context) {
	var req ListByTagRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.FailWithDataByErrCode(c, apierror.ErrBadRequest, gin.H{"error": err.Error()})
		return
	}
	if req.Limit <= 0 || req.Limit > 50 {
		req.Limit = 10
	}
	var lastTime time.Time
	if req.LatestTime > 0 {
		lastTime = time.UnixMilli(req.LatestTime)
	}
	viewerAccountID, _ := mw.GetAccountIDByGIN(c)
	resp, err := f.service.ListByTag(c.Request.Context(), req.TagName, req.Limit, lastTime, viewerAccountID)
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
