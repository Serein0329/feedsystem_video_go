package video

import "time"

type Video struct {
	ID          uint      `gorm:"primaryKey" json:"id"`
	AuthorID    uint      `gorm:"index;not null" json:"author_id"`
	Username    string    `gorm:"type:varchar(255);not null" json:"username"`
	Title       string    `gorm:"type:varchar(255);not null" json:"title"`
	Description string    `gorm:"type:varchar(255);" json:"description,omitempty"`
	PlayURL     string    `gorm:"type:varchar(255);not null" json:"play_url"`
	CoverURL    string    `gorm:"type:varchar(255);not null" json:"cover_url"`
	CreateTime  time.Time `gorm:"autoCreateTime;index:idx_videos_create_time,sort:desc;index:idx_videos_popularity_create_time,priority:2,sort:desc" json:"create_time"`
	LikesCount  int64     `gorm:"not null;default:0;index:idx_videos_likes_count,sort:desc" json:"likes_count"`
	Popularity  int64     `gorm:"not null;default:0;index:idx_videos_popularity_create_time,priority:1,sort:desc" json:"popularity"`
}

type PublishVideoRequest struct {
	Title       string `json:"title" binding:"required,max=255"`
	Description string `json:"description" binding:"max=255"`
	PlayURL     string `json:"play_url" binding:"required,url"`
	CoverURL    string `json:"cover_url" binding:"required,url"`
}

type DeleteVideoRequest struct {
	ID uint `json:"id" binding:"required"`
}

type ListByAuthorIDRequest struct {
	AuthorID uint `json:"author_id" binding:"required"`
}

type GetDetailRequest struct {
	ID uint `json:"id" binding:"required"`
}

type SearchVideosRequest struct {
	Keyword string `json:"keyword" binding:"required,max=50"`
	Limit   int    `json:"limit"`
	LastID  uint   `json:"last_id"`
}

type UpdateLikesCountRequest struct {
	ID         uint  `json:"id"`
	LikesCount int64 `json:"likes_count"`
}

type SearchVideosResponse struct {
	Videos  []*Video `json:"videos"`
	HasMore bool     `json:"has_more"`
	LastID  uint     `json:"last_id"`
}
