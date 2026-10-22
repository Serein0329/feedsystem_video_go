package social

import (
	"feedsystem_video_go/internal/account"
	"feedsystem_video_go/internal/mw"
	"feedsystem_video_go/internal/pkg/apierror"
	"feedsystem_video_go/internal/pkg/response"

	"github.com/gin-gonic/gin"
)

type SocialHandler struct {
	service *SocialService
}

func NewSocialHandler(service *SocialService) *SocialHandler {
	return &SocialHandler{service: service}
}

func (h *SocialHandler) Follow(c *gin.Context) {
	var req FollowRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.FailWithDataByErrCode(c, apierror.ErrBadRequest, gin.H{"error": err.Error()})
		return
	}

	FollowerID, _ := mw.GetAccountIDByGIN(c)
	username, _ := mw.GetUsernameByGIN(c)
	if err := h.service.Follow(c.Request.Context(), FollowerID, req.VloggerID, username); err != nil {
		response.FailByErr(c, err)
		return
	}
	response.Success(c, nil)
}

func (h *SocialHandler) Unfollow(c *gin.Context) {
	var req UnfollowRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.FailWithDataByErrCode(c, apierror.ErrBadRequest, gin.H{"error": err.Error()})
		return
	}
	FollowerID, _ := mw.GetAccountIDByGIN(c)
	if err := h.service.Unfollow(c.Request.Context(), FollowerID, req.VloggerID); err != nil {
		response.FailByErr(c, err)
		return
	}
	response.Success(c, nil)
}

func (h *SocialHandler) Isfollowed(c *gin.Context) {
	var req IsfollowedRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.FailWithDataByErrCode(c, apierror.ErrBadRequest, gin.H{"error": err.Error()})
		return
	}
	FollowerID, _ := mw.GetAccountIDByGIN(c)
	if isfollowed, err := h.service.Isfollowed(c.Request.Context(), FollowerID, req.VloggerID); err != nil {
		response.FailByErr(c, err)
		return
	} else {
		response.Success(c, gin.H{"isFollowed": isfollowed})
	}
}

func (h *SocialHandler) GetAllFollowers(c *gin.Context) {
	var req GetAllFollowersRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.FailWithDataByErrCode(c, apierror.ErrBadRequest, gin.H{"error": err.Error()})
		return
	}

	if req.VloggerID == 0 {
		req.VloggerID, _ = mw.GetAccountIDByGIN(c)
	}
	followers, err := h.service.GetAllFollowers(c.Request.Context(), req.VloggerID)
	if err != nil {
		response.FailByErr(c, err)
		return
	}
	if followers == nil {
		followers = []*account.Account{}
	}
	followerCount, _ := h.service.CountFollowers(c.Request.Context(), req.VloggerID)
	response.Success(c, GetAllFollowersResponse{Followers: followers, FollowerCount: followerCount})
}

func (h *SocialHandler) GetAllVloggers(c *gin.Context) {
	var req GetAllVloggersRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.FailWithDataByErrCode(c, apierror.ErrBadRequest, gin.H{"error": err.Error()})
		return
	}

	if req.FollowerID == 0 {
		req.FollowerID, _ = mw.GetAccountIDByGIN(c)
	}

	vloggers, err := h.service.GetAllVloggers(c.Request.Context(), req.FollowerID)
	if err != nil {
		response.FailByErr(c, err)
		return
	}
	if vloggers == nil {
		vloggers = []*account.Account{}
	}
	vloggerCount, _ := h.service.CountVloggers(c.Request.Context(), req.FollowerID)
	response.Success(c, GetAllVloggersResponse{Vloggers: vloggers, VloggerCount: vloggerCount})
}

func (h *SocialHandler) GetCounts(c *gin.Context) {
	accountID, _ := mw.GetAccountIDByGIN(c)
	followerCount, _ := h.service.CountFollowers(c.Request.Context(), accountID)
	vloggerCount, _ := h.service.CountVloggers(c.Request.Context(), accountID)
	response.Success(c, GetCountsResponse{FollowerCount: followerCount, VloggerCount: vloggerCount})
}
