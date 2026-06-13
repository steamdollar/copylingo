package service

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/lsj/copylingo/internal/model"
)

type mockStudyMaterialStore struct {
	getForStudySessionFn func(ctx context.Context, userID int64, language, level string, limit int) ([]model.Material, error)
}

func (m *mockStudyMaterialStore) GetForStudySession(
	ctx context.Context,
	userID int64,
	language, level string,
	limit int,
) ([]model.Material, error) {
	return m.getForStudySessionFn(ctx, userID, language, level, limit)
}

type mockStudySessionStore struct {
	createSessionFn func(ctx context.Context, s *model.Session) error
}

func (m *mockStudySessionStore) CreateSession(ctx context.Context, s *model.Session) error {
	return m.createSessionFn(ctx, s)
}

type mockStudySessionMaterialStore struct {
	createSessionMaterialsFn func(ctx context.Context, sms []model.SessionMaterial) error
}

func (m *mockStudySessionMaterialStore) CreateSessionMaterials(ctx context.Context, sms []model.SessionMaterial) error {
	return m.createSessionMaterialsFn(ctx, sms)
}

func TestBuildStudySessionCreatesOrderedMaterials(t *testing.T) {
	ctx := context.Background()
	userID := int64(123)

	materialStore := &mockStudyMaterialStore{
		getForStudySessionFn: func(ctx context.Context, gotUserID int64, language, level string, limit int) ([]model.Material, error) {
			if gotUserID != userID || language != "ja" || level != "N5" {
				t.Fatalf("GetForStudySession args = (%d, %s, %s), want (%d, ja, N5)",
					gotUserID, language, level, userID)
			}
			if limit != 8 {
				t.Fatalf("limit = %d, want 8", limit)
			}
			return []model.Material{{ID: 10}, {ID: 11}}, nil
		},
	}
	sessionStore := &mockStudySessionStore{
		createSessionFn: func(ctx context.Context, s *model.Session) error {
			if s.UserID != userID ||
				s.Type != model.SessionStudy ||
				s.Mode != model.SessionModeStudy ||
				s.Status != model.SessionPending ||
				s.TotalQuestions != 2 {
				t.Fatalf("unexpected session: %+v", s)
			}
			s.ID = 99
			return nil
		},
	}
	sessionMaterialStore := &mockStudySessionMaterialStore{
		createSessionMaterialsFn: func(ctx context.Context, sms []model.SessionMaterial) error {
			if len(sms) != 2 {
				t.Fatalf("len(sessionMaterials) = %d, want 2", len(sms))
			}
			for i, wantMaterialID := range []int{10, 11} {
				if sms[i].SessionID != 99 || sms[i].MaterialID != wantMaterialID || sms[i].MaterialOrder != i {
					t.Fatalf("sessionMaterial[%d] = %+v", i, sms[i])
				}
			}
			return nil
		},
	}

	svc := NewStudySessionService(materialStore, sessionStore, sessionMaterialStore)
	session, err := svc.BuildStudySession(ctx, userID, "ja", "N5")
	if err != nil {
		t.Fatalf("BuildStudySession failed: %v", err)
	}
	if session == nil || session.ID != 99 {
		t.Fatalf("session = %+v, want id 99", session)
	}
}

func TestBuildStudySessionNoMaterialsReturnsNil(t *testing.T) {
	ctx := context.Background()
	materialStore := &mockStudyMaterialStore{
		getForStudySessionFn: func(ctx context.Context, userID int64, language, level string, limit int) ([]model.Material, error) {
			return nil, nil
		},
	}
	sessionStore := &mockStudySessionStore{
		createSessionFn: func(ctx context.Context, s *model.Session) error {
			t.Fatal("CreateSession should not be called")
			return nil
		},
	}
	sessionMaterialStore := &mockStudySessionMaterialStore{}

	svc := NewStudySessionService(materialStore, sessionStore, sessionMaterialStore)
	session, err := svc.BuildStudySession(ctx, 123, "ja", "N5")
	if err != nil {
		t.Fatalf("BuildStudySession failed: %v", err)
	}
	if session != nil {
		t.Fatalf("session = %+v, want nil", session)
	}
}

func TestBuildStudySessionWrapsMaterialError(t *testing.T) {
	ctx := context.Background()
	materialStore := &mockStudyMaterialStore{
		getForStudySessionFn: func(ctx context.Context, userID int64, language, level string, limit int) ([]model.Material, error) {
			return nil, errors.New("db down")
		},
	}
	svc := NewStudySessionService(materialStore, nil, nil)

	_, err := svc.BuildStudySession(ctx, 123, "ja", "N5")
	if err == nil {
		t.Fatal("expected error")
	}
	if !strings.Contains(err.Error(), "build study session fetch materials") ||
		!strings.Contains(err.Error(), "db down") {
		t.Fatalf("unexpected error: %v", err)
	}
}
