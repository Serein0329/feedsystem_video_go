package message

import (
	"context"
	"feedsystem_video_go/internal/account"
	"feedsystem_video_go/internal/pkg/apierror"
)

type Service struct {
	msgRepo     *Repository
	accountRepo *account.AccountRepository
}

func NewService(repo *Repository, accRepo *account.AccountRepository) *Service {
	return &Service{msgRepo: repo, accountRepo: accRepo}
}

func (s *Service) Send(ctx context.Context, m *Message) error {
	exist, err := s.accountRepo.IsExist(ctx, m.ToID)
	if err != nil {
		return err
	}
	if !exist {
		return apierror.ErrUserNotFound
	}
	return s.msgRepo.Send(ctx, m)
}

func (s *Service) List(ctx context.Context, userID, peerID, lastID uint, limit int) (ListResponse, error) {
	exist, err := s.accountRepo.IsExist(ctx, peerID)
	if err != nil {
		return ListResponse{}, err
	}
	if !exist {
		return ListResponse{}, apierror.ErrUserNotFound
	}
	msgs, err := s.msgRepo.List(ctx, userID, peerID, lastID, limit)
	if err != nil {
		return ListResponse{}, err
	}
	if len(msgs) > 0 {
		lastID = msgs[len(msgs)-1].ID
	}
	return ListResponse{
		Messages: msgs,
		HasMore:  len(msgs) == limit,
		NextID:   lastID,
	}, nil
}
