package service

import (
	"context"
	"fmt"

	"github.com/lsj/copylingo/internal/model"
)

const studySessionMaterialCount = 8

type studyMaterialStore interface {
	GetForStudySession(ctx context.Context, userID int64, language, level string, limit int) ([]model.Material, error)
}

type studySessionStore interface {
	CreateSession(ctx context.Context, s *model.Session) error
}

type studySessionMaterialStore interface {
	CreateSessionMaterials(ctx context.Context, sms []model.SessionMaterial) error
}

// StudySessionService creates material-based study sessions.
type StudySessionService struct {
	materialRepo        studyMaterialStore
	sessionRepo         studySessionStore
	sessionMaterialRepo studySessionMaterialStore
}

func NewStudySessionService(
	materialRepo studyMaterialStore,
	sessionRepo studySessionStore,
	sessionMaterialRepo studySessionMaterialStore,
) *StudySessionService {
	return &StudySessionService{
		materialRepo:        materialRepo,
		sessionRepo:         sessionRepo,
		sessionMaterialRepo: sessionMaterialRepo,
	}
}

func (s *StudySessionService) BuildStudySession(
	ctx context.Context,
	userID int64,
	language, level string,
) (*model.Session, error) {
	materials, err := s.materialRepo.GetForStudySession(ctx, userID, language, level, studySessionMaterialCount)
	if err != nil {
		return nil, fmt.Errorf("build study session fetch materials user_id=%d language=%s level=%s: %w",
			userID, language, level, err)
	}
	if len(materials) == 0 {
		return nil, nil
	}

	session := &model.Session{
		UserID:         userID,
		Type:           model.SessionStudy,
		Mode:           model.SessionModeStudy,
		Status:         model.SessionPending,
		TotalQuestions: len(materials),
	}
	if err := s.sessionRepo.CreateSession(ctx, session); err != nil {
		return nil, fmt.Errorf("build study session create user_id=%d: %w", userID, err)
	}

	sessionMaterials := make([]model.SessionMaterial, 0, len(materials))
	for i, material := range materials {
		sessionMaterials = append(sessionMaterials, model.SessionMaterial{
			SessionID:     session.ID,
			MaterialID:    material.ID,
			MaterialOrder: i,
		})
	}
	if err := s.sessionMaterialRepo.CreateSessionMaterials(ctx, sessionMaterials); err != nil {
		return nil, fmt.Errorf("build study session create materials session_id=%d: %w", session.ID, err)
	}

	return session, nil
}
