package message

import (
	"context"

	"gorm.io/gorm"
)

type Repository struct{ db *gorm.DB }

func NewRepository(db *gorm.DB) *Repository { return &Repository{db: db} }

func (r *Repository) Send(ctx context.Context, m *Message) error {
	return r.db.WithContext(ctx).Create(m).Error
}

func (r *Repository) List(ctx context.Context, userID, peerID, lastID uint, limit int) ([]Message, error) {
	var msgs []Message
	query := r.db.WithContext(ctx).Where("(from_id = ? AND to_id = ?) OR (from_id = ? AND to_id = ?)", userID, peerID, peerID, userID)
	if lastID > 0 {
		query = query.Where("id < ?", lastID)
	}
	if err := query.Order("id desc").Limit(limit).Find(&msgs).Error; err != nil {
		return nil, err
	}
	return msgs, nil
}
