package service

import (
	"context"
	"errors"
	"fmt"
	"reflect"
	"strings"
	"testing"

	"github.com/lsj/copylingo/internal/model"
)

type mockStudyMaterialStore struct {
	getForStudySessionFn func(ctx context.Context, userID int64, language, level string, levels []string, plan model.StudySessionPlan) ([]model.Material, error)
}

func (m *mockStudyMaterialStore) GetForStudySession(
	ctx context.Context,
	userID int64,
	language, level string,
	levels []string,
	plan model.StudySessionPlan,
) ([]model.Material, error) {
	return m.getForStudySessionFn(ctx, userID, language, level, levels, plan)
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
		getForStudySessionFn: func(ctx context.Context, gotUserID int64, language, level string, levels []string, plan model.StudySessionPlan) ([]model.Material, error) {
			want := []string{"N5", "N4"}
			if gotUserID != userID || language != "ja" || level != "N5" || !reflect.DeepEqual(levels, want) {
				t.Fatalf("GetForStudySession args = (%d, %s, %v), want (%d, ja, %v)",
					gotUserID, language, levels, userID, want)
			}
			if !reflect.DeepEqual(plan, morningStudySessionPlan) {
				t.Fatalf("plan = %+v, want %+v", plan, morningStudySessionPlan)
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

func TestBuildStudySessionUsesAdjacentJapaneseLevelScope(t *testing.T) {
	materialStore := &mockStudyMaterialStore{
		getForStudySessionFn: func(
			_ context.Context,
			_ int64,
			language, _ string,
			levels []string,
			_ model.StudySessionPlan,
		) ([]model.Material, error) {
			want := []string{"N5", "N4", "N3"}
			if !reflect.DeepEqual(levels, want) {
				t.Fatalf("session levels = %v, want %v", levels, want)
			}
			return []model.Material{{ID: 10}}, nil
		},
	}
	sessionStore := &mockStudySessionStore{
		createSessionFn: func(_ context.Context, s *model.Session) error {
			s.ID = 99
			return nil
		},
	}
	sessionMaterialStore := &mockStudySessionMaterialStore{
		createSessionMaterialsFn: func(_ context.Context, _ []model.SessionMaterial) error {
			return nil
		},
	}

	session, err := NewStudySessionService(materialStore, sessionStore, sessionMaterialStore).
		BuildStudySession(context.Background(), 123, "ja", "N4")
	if err != nil || session == nil || session.ID != 99 {
		t.Fatalf("BuildStudySession() = %+v, %v", session, err)
	}
}

func TestBuildStudySessionWithLimitUsesRequestedLimit(t *testing.T) {
	ctx := context.Background()
	userID := int64(123)

	materialStore := &mockStudyMaterialStore{
		getForStudySessionFn: func(ctx context.Context, gotUserID int64, language, level string, levels []string, plan model.StudySessionPlan) ([]model.Material, error) {
			want := []string{"N5", "N4"}
			if gotUserID != userID || language != "ja" || level != "N5" || !reflect.DeepEqual(levels, want) {
				t.Fatalf("GetForStudySession args = (%d, %s, %v), want (%d, ja, %v)",
					gotUserID, language, levels, userID, want)
			}
			if !reflect.DeepEqual(plan, morningStudySessionPlan) {
				t.Fatalf("plan = %+v, want %+v", plan, morningStudySessionPlan)
			}
			return []model.Material{{ID: 10}}, nil
		},
	}
	sessionStore := &mockStudySessionStore{
		createSessionFn: func(ctx context.Context, s *model.Session) error {
			s.ID = 99
			return nil
		},
	}
	sessionMaterialStore := &mockStudySessionMaterialStore{
		createSessionMaterialsFn: func(ctx context.Context, sms []model.SessionMaterial) error {
			return nil
		},
	}

	svc := NewStudySessionService(materialStore, sessionStore, sessionMaterialStore)
	session, err := svc.BuildStudySessionWithLimit(ctx, userID, "ja", "N5", 20)
	if err != nil {
		t.Fatalf("BuildStudySessionWithLimit failed: %v", err)
	}
	if session == nil || session.ID != 99 {
		t.Fatalf("session = %+v, want id 99", session)
	}
}

func TestBuildStudySessionWithLimitRejectsOutOfRangeLimit(t *testing.T) {
	svc := NewStudySessionService(nil, nil, nil)

	for _, limit := range []int{0, -1, MaxStudySessionMaterialCount + 1} {
		_, err := svc.BuildStudySessionWithLimit(context.Background(), 123, "ja", "N5", limit)
		if err == nil {
			t.Fatalf("limit %d: expected error", limit)
		}
		if !strings.Contains(err.Error(), "invalid limit") {
			t.Fatalf("limit %d: unexpected error: %v", limit, err)
		}
	}
}

func TestBuildStudySessionWithProfileUsesFixedPlan(t *testing.T) {
	tests := []struct {
		profile StudySessionProfile
		want    model.StudySessionPlan
	}{
		{profile: StudyProfileMorning, want: studyPlan(8, 7, 1, 3, 1, 0)},
		{profile: StudyProfileEvening, want: studyPlan(4, 14, 1, 3, 0, 2)},
	}

	for _, tt := range tests {
		t.Run(string(tt.profile), func(t *testing.T) {
			var got model.StudySessionPlan
			svc := newPlanCaptureStudySessionService(&got)
			session, err := svc.BuildStudySessionWithProfile(
				context.Background(), 123, "ja", "N5", tt.profile,
			)
			if err != nil {
				t.Fatalf("BuildStudySessionWithProfile failed: %v", err)
			}
			if session == nil {
				t.Fatal("session = nil, want session")
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Fatalf("plan = %+v, want %+v", got, tt.want)
			}
		})
	}
}

func TestBuildStudySessionWithProfileRejectsUnknownProfile(t *testing.T) {
	svc := NewStudySessionService(nil, nil, nil)
	_, err := svc.BuildStudySessionWithProfile(
		context.Background(), 123, "ja", "N5", StudySessionProfile("night"),
	)
	if err == nil || !strings.Contains(err.Error(), "invalid profile") {
		t.Fatalf("error = %v, want invalid profile", err)
	}
}

func TestBuildStudySessionWithLimitScalesMorningPlan(t *testing.T) {
	tests := []struct {
		limit int
		want  model.StudySessionPlan
	}{
		{limit: 1, want: studyPlan(1, 0, 0, 0, 0, 0)},
		{limit: 2, want: studyPlan(1, 1, 0, 0, 0, 0)},
		{limit: 15, want: studyPlan(6, 5, 1, 2, 1, 0)},
		{limit: 20, want: studyPlan(8, 7, 1, 3, 1, 0)},
		{limit: 50, want: studyPlan(20, 18, 3, 7, 2, 0)},
	}

	for _, tt := range tests {
		t.Run(fmt.Sprintf("limit_%d", tt.limit), func(t *testing.T) {
			var got model.StudySessionPlan
			svc := newPlanCaptureStudySessionService(&got)
			if _, err := svc.BuildStudySessionWithLimit(
				context.Background(), 123, "ja", "N5", tt.limit,
			); err != nil {
				t.Fatalf("BuildStudySessionWithLimit failed: %v", err)
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Fatalf("plan = %+v, want %+v", got, tt.want)
			}
			if got.TotalMaterialCount() != tt.limit {
				t.Fatalf("plan total = %d, want %d", got.TotalMaterialCount(), tt.limit)
			}
			if got.Quotas[2].NewCount+got.Quotas[2].ReviewCount > 2 {
				t.Fatalf("reading quota = %+v, exceeds cap 2", got.Quotas[2])
			}
		})
	}
}

func studyPlan(vocabNew, vocabReview, grammarNew, grammarReview, readingNew, readingReview int) model.StudySessionPlan {
	return model.StudySessionPlan{Quotas: []model.StudyMaterialQuota{
		{Category: model.MaterialCategoryVocabulary, NewCount: vocabNew, ReviewCount: vocabReview},
		{Category: model.MaterialCategoryGrammar, NewCount: grammarNew, ReviewCount: grammarReview},
		{Category: model.MaterialCategoryReading, NewCount: readingNew, ReviewCount: readingReview},
	}}
}

func newPlanCaptureStudySessionService(capture *model.StudySessionPlan) *StudySessionService {
	return NewStudySessionService(
		&mockStudyMaterialStore{getForStudySessionFn: func(
			_ context.Context,
			_ int64,
			_, _ string,
			_ []string,
			plan model.StudySessionPlan,
		) ([]model.Material, error) {
			*capture = plan
			return []model.Material{{ID: 10}}, nil
		}},
		&mockStudySessionStore{createSessionFn: func(_ context.Context, s *model.Session) error {
			s.ID = 99
			return nil
		}},
		&mockStudySessionMaterialStore{createSessionMaterialsFn: func(
			_ context.Context, _ []model.SessionMaterial,
		) error {
			return nil
		}},
	)
}

func TestBuildStudySessionNoMaterialsReturnsNil(t *testing.T) {
	ctx := context.Background()
	materialStore := &mockStudyMaterialStore{
		getForStudySessionFn: func(ctx context.Context, userID int64, language, level string, levels []string, plan model.StudySessionPlan) ([]model.Material, error) {
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
		getForStudySessionFn: func(ctx context.Context, userID int64, language, level string, levels []string, plan model.StudySessionPlan) ([]model.Material, error) {
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
