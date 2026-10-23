package social

import (
	"feedsystem_video_go/internal/account"
)

type Social struct {
	ID         uint `gorm:"primaryKey"`
	FollowerID uint `gorm:"not null;uniqueIndex:idx_social_follower_vlogger,priority:1"`
	VloggerID  uint `gorm:"not null;index:idx_social_vlogger;uniqueIndex:idx_social_follower_vlogger,priority:2"`
}

type FollowRequest struct {
	VloggerID uint `json:"vlogger_id" binding:"required"`
}

type UnfollowRequest struct {
	VloggerID uint `json:"vlogger_id" binding:"required"`
}

type IsfollowedRequest struct {
	VloggerID uint `json:"vlogger_id" binding:"required"`
}

type GetAllFollowersRequest struct {
	VloggerID uint `json:"vlogger_id"`
	Limit     int  `json:"limit"`
	LastID    uint `json:"last_id"`
}

type GetAllVloggersRequest struct {
	FollowerID uint `json:"follower_id"`
	Limit      int  `json:"limit"`
	LastID     uint `json:"last_id"`
}

type GetAllFollowersResponse struct {
	Followers []account.Account `json:"followers"`
	NextID    uint              `json:"next_id"`
	HasMore   bool              `json:"has_more"`
}

type GetAllVloggersResponse struct {
	Vloggers []account.Account `json:"vloggers"`
	NextID   uint              `json:"next_id"`
	HasMore  bool              `json:"has_more"`
}

type GetCountsResponse struct {
	FollowerCount int64 `json:"follower_count"`
	VloggerCount  int64 `json:"vlogger_count"`
}
