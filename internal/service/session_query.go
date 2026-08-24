package service

import (
	"context"

	"github.com/lsj/copylingo/internal/model"
)

type unfinishedSessionRepo interface {
	GetOldestUnfinished(ctx context.Context, userID int64) (*model.Session, error)
}

// SessionQueryService exposes session lookup operations used by scheduled jobs.
type SessionQueryService struct {
	sessionRepo unfinishedSessionRepo
}

func NewSessionQueryService(sessionRepo unfinishedSessionRepo) *SessionQueryService {
	return &SessionQueryService{sessionRepo: sessionRepo}
}

// GetOldestUnfinished returns the user's highest-priority unfinished session.
func (s *SessionQueryService) GetOldestUnfinished(ctx context.Context, userID int64) (*model.Session, error) {
	return s.sessionRepo.GetOldestUnfinished(ctx, userID)
}
