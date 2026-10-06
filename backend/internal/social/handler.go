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

	social := &Social{
		FollowerID: FollowerID,
		VloggerID:  req.VloggerID,
	}
	if err := h.service.Follow(c.Request.Context(), social); err != nil {
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
	social := &Social{
		FollowerID: FollowerID,
		VloggerID:  req.VloggerID,
	}
	if err := h.service.Unfollow(c.Request.Context(), social); err != nil {
		response.FailByErr(c, err)
		return
	}
	response.Success(c, nil)
}

func (h *SocialHandler) GetAllFollowers(c *gin.Context) {
	var req GetAllFollowersRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.FailWithDataByErrCode(c, apierror.ErrBadRequest, gin.H{"error": err.Error()})
		return
	}

	vloggerID := req.VloggerID
	if vloggerID == 0 {
		vloggerID, _ = mw.GetAccountIDByGIN(c)
	}
	followers, err := h.service.GetAllFollowers(c.Request.Context(), vloggerID)
	if err != nil {
		response.FailByErr(c, err)
		return
	}
	if followers == nil {
		followers = []*account.Account{}
	}
	followerCount, _ := h.service.CountFollowers(c.Request.Context(), vloggerID)
	response.Success(c, GetAllFollowersResponse{Followers: followers, FollowerCount: followerCount})
}

func (h *SocialHandler) GetAllVloggers(c *gin.Context) {
	var req GetAllVloggersRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.FailWithDataByErrCode(c, apierror.ErrBadRequest, gin.H{"error": err.Error()})
		return
	}

	followerID := req.FollowerID
	if followerID == 0 {
		followerID, _ = mw.GetAccountIDByGIN(c)
	}

	vloggers, err := h.service.GetAllVloggers(c.Request.Context(), followerID)
	if err != nil {
		response.FailByErr(c, err)
		return
	}
	if vloggers == nil {
		vloggers = []*account.Account{}
	}
	vloggerCount, _ := h.service.CountVloggers(c.Request.Context(), followerID)
	response.Success(c, GetAllVloggersResponse{Vloggers: vloggers, VloggerCount: vloggerCount})
}

func (h *SocialHandler) GetCounts(c *gin.Context) {
	accountID, _ := mw.GetAccountIDByGIN(c)
	followerCount, _ := h.service.CountFollowers(c.Request.Context(), accountID)
	vloggerCount, _ := h.service.CountVloggers(c.Request.Context(), accountID)
	response.Success(c, SocialCounts{FollowerCount: followerCount, VloggerCount: vloggerCount})
}
