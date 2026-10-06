package message

import (
	"context"
	"strings"
)

type Service struct{ repo *Repository }

func NewService(repo *Repository) *Service { return &Service{repo: repo} }

func (s *Service) Send(ctx context.Context, m *Message) error {
	m.Content = strings.TrimSpace(m.Content)
	return s.repo.Send(ctx, m)
}

func (s *Service) List(ctx context.Context, userID, peerID uint, limit int) ([]Message, error) {
	return s.repo.List(ctx, userID, peerID, limit)
}
