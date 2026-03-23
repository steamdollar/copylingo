package service

import (
	"context"

	"github.com/lsj/copylingo/internal/model"
)

// Additional convenience methods that bridge repositories for use in handlers.

// GetUser retrieves or creates a user (convenience wrapper on GraderService).
func (g *GraderService) GetUser(ctx context.Context, telegramID int64, username string) (*model.User, error) {
	return g.repos.User.GetOrCreate(ctx, telegramID, username)
}

// GetAllUsers returns all registered users (for scheduler).
func (g *GraderService) GetAllUsers(ctx context.Context) ([]model.User, error) {
	return g.repos.User.GetAllUsers(ctx)
}

// GetPendingSessions returns pending sessions for a user.
func (s *SessionBuilderService) GetPendingSessions(ctx context.Context, userID int64) ([]model.Session, error) {
	return s.repos.Session.GetPendingSessions(ctx, userID)
}

// GetSession returns a session by ID.
func (s *SessionBuilderService) GetSession(ctx context.Context, sessionID int) (*model.Session, error) {
	return s.repos.Session.GetByID(ctx, sessionID)
}

// StartSession marks a session as in_progress.
func (s *SessionBuilderService) StartSession(ctx context.Context, sessionID int) error {
	return s.repos.Session.Start(ctx, sessionID)
}

// GetQuestion returns a question by ID.
func (s *SessionBuilderService) GetQuestion(ctx context.Context, questionID int) (*model.Question, error) {
	return s.repos.Question.GetByID(ctx, questionID)
}

// GetSessionQuestions returns all questions for a session.
func (s *SessionBuilderService) GetSessionQuestions(ctx context.Context, sessionID int) ([]model.SessionQuestion, error) {
	return s.repos.SessionQuestion.GetBySession(ctx, sessionID)
}
