package video

import "time"

type Like struct {
	ID        uint      `gorm:"primaryKey" json:"id"`
	VideoID   uint      `gorm:"uniqueIndex:idx_like_video_account;not null" json:"video_id"`
	AccountID uint      `gorm:"uniqueIndex:idx_like_video_account;not null;index:idx_like_account" json:"account_id"`
	CreatedAt time.Time `gorm:"autoCreateTime" json:"created_at"`
}

type LikedVideo struct {
	Video
	LikeID uint `gorm:"column:like_id" json:"-"`
}

type LikeRequest struct {
	VideoID uint `json:"video_id" binding:"required"`
}

type ListMyLikedVideosRequest struct {
	LastID uint `json:"last_id"`
	Limit  int  `json:"limit"`
}

type ListMyLikedVideosResponse struct {
	Videos  []Video `json:"videos"`
	NextID  uint    `json:"next_id"`
	HasMore bool    `json:"has_more"`
}
