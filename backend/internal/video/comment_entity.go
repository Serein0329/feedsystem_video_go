package video

import "time"

type Comment struct {
	ID        uint      `gorm:"primaryKey" json:"id"`
	Username  string    `gorm:"index" json:"username"`
	VideoID   uint      `gorm:"index" json:"video_id"`
	AuthorID  uint      `gorm:"index" json:"author_id"`
	Content   string    `gorm:"type:text" json:"content"`
	CreatedAt time.Time `gorm:"autoCreateTime" json:"created_at"`
}

type PublishCommentRequest struct {
	VideoID uint   `json:"video_id" binding:"required"`
	Content string `json:"content" binding:"required"`
}

type DeleteCommentRequest struct {
	CommentID uint `json:"comment_id" binding:"required"`
}

type GetAllCommentsRequest struct {
	VideoID uint `json:"video_id" binding:"required"`
	Limit   int  `json:"limit"`
	LastID  uint `json:"last_id"`
}

type GetAllCommentsResponse struct {
	Comments []Comment `json:"comments"`
	HasMore  bool      `json:"has_more"`
	NextID   uint      `json:"next_id"`
}
