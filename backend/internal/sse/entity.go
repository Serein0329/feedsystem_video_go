package sse

import "time"

type Notification struct {
	ID          uint      `gorm:"primaryKey" json:"id"`
	RecipientID uint      `gorm:"index;not null" json:"recipient_id"`
	SenderID    uint      `gorm:"not null" json:"sender_id"`
	Username    string    `gorm:"not null" json:"username"`
	Type        string    `gorm:"type:varchar(50);not null" json:"type"`
	TargetID    uint      `json:"target_id"`
	Content     string    `gorm:"type:varchar(255)" json:"content"`
	IsRead      bool      `gorm:"default:false" json:"is_read"`
	CreatedAt   time.Time `gorm:"autoCreateTime" json:"created_at"`
}

type ListRequest struct {
	Limit  int  `json:"limit"`
	LastID uint `json:"last_id"`
}

type ListResponse struct {
	Notifications []Notification `json:"notifications"`
	HasMore       bool           `json:"has_more"`
	NextID        uint           `json:"next_id"`
}
