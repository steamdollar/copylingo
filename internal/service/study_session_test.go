package service

import (
	"context"
	"errors"
	"fmt"
	"reflect"
	"strings"
	"testing"

	"github.com/jmoiron/sqlx"

	"github.com/lsj/copylingo/internal/model"
	"github.com/lsj/copylingo/internal/testutil"
)

type mockStudyMaterialStore struct {
	getMaterialsByPlanFn func(
		ctx context.Context,
		userID int64,
		language,
		level string,
		levels []string,
		plan model.StudySessionPlan,
	) ([]model.Material, error)
}

func (m *mockStudyMaterialStore) GetMaterialsByPlan(
	ctx context.Context,
	userID int64,
	language,
	level string,
	levels []string,
	plan model.StudySessionPlan,
) ([]model.Material, error) {
	return m.getMaterialsByPlanFn(
		ctx,
		userID,
		language,
		level,
		levels,
		plan,
	)
}

type mockStudySessionStore struct {
	createSessionFn   func(session *model.Session) (int, error)
	createMaterialsFn func(
		sessionID int,
		materialIDs []int,
	) error
}

func (m *mockStudySessionStore) CreateSessionInTx(
	ctx context.Context,
	tx *sqlx.Tx,
	session *model.Session,
) (int, error) {
	if m.createSessionFn != nil {
		return m.createSessionFn(session)
	}
	return 99, nil
}

func (m *mockStudySessionStore) CreateSessionMaterialsInTx(
	ctx context.Context,
	tx *sqlx.Tx,
	sessionID int,
	materialIDs []int,
) error {
	if m.createMaterialsFn != nil {
		return m.createMaterialsFn(
			sessionID,
			materialIDs,
		)
	}
	return nil
}

var studyTestDB = testutil.TransactionDB()

func TestBuildStudySessionCreatesOrderedMaterials(t *testing.T) {
	ctx := context.Background()
	userID := int64(123)

	materialStore := &mockStudyMaterialStore{
		getMaterialsByPlanFn: func(
			ctx context.Context,
			gotUserID int64,
			language,
			level string,
			levels []string,
			plan model.StudySessionPlan,
		) ([]model.Material, error) {
			want := []string{"N5", "N4"}
			if gotUserID != userID || language != "ja" || level != "N5" || !reflect.DeepEqual(
				levels,
				want,
			) {
				t.Fatalf(
					"GetMaterialsByPlan args = (%d, %s, %v), want (%d, ja, %v)",
					gotUserID,
					language,
					levels,
					userID,
					want,
				)
			}
			if !reflect.DeepEqual(
				plan,
				morningStudySessionPlan,
			) {
				t.Fatalf(
					"plan = %+v, want %+v",
					plan,
					morningStudySessionPlan,
				)
			}
			return []model.Material{{ID: 10}, {ID: 11}}, nil
		},
	}
	sessionStore := &mockStudySessionStore{
		createSessionFn: func(session *model.Session) (int, error) {
			if session.UserID != userID ||
				session.Type != model.SessionStudy ||
				session.Mode != model.SessionModeStudy ||
				session.Status != model.SessionPending ||
				session.TotalQuestions != 2 {
				t.Fatalf(
					"unexpected session: %+v",
					session,
				)
			}
			return 99, nil
		},
		createMaterialsFn: func(
			sessionID int,
			materialIDs []int,
		) error {
			if sessionID != 99 {
				t.Fatalf(
					"session ID for materials = %d, want 99",
					sessionID,
				)
			}
			if !reflect.DeepEqual(
				materialIDs,
				[]int{10, 11},
			) {
				t.Fatalf(
					"material IDs = %v, want [10 11]",
					materialIDs,
				)
			}
			return nil
		},
	}

	svc := newStudySessionService(
		materialStore,
		sessionStore,
		studyTestDB,
	)
	session, err := svc.BuildStudySession(
		ctx,
		userID,
		"ja",
		"N5",
		StudyProfileMorning,
		0,
	)
	if err != nil {
		t.Fatalf(
			"BuildStudySession failed: %v",
			err,
		)
	}
	if session == nil || session.ID != 99 {
		t.Fatalf(
			"session = %+v, want id 99",
			session,
		)
	}
}

func TestBuildStudySessionDoesNotPublishIDWhenMaterialInsertFails(t *testing.T) {
	materialStore := &mockStudyMaterialStore{
		getMaterialsByPlanFn: func(
			_ context.Context,
			_ int64,
			_,
			_ string,
			_ []string,
			_ model.StudySessionPlan,
		) ([]model.Material, error) {
			return []model.Material{{ID: 10}}, nil
		},
	}
	var createdSession *model.Session
	sessionStore := &mockStudySessionStore{
		createSessionFn: func(session *model.Session) (int, error) {
			createdSession = session
			return 99, nil
		},
		createMaterialsFn: func(
			sessionID int,
			materialIDs []int,
		) error {
			return errors.New("material insert failed")
		},
	}

	session, err := newStudySessionService(
		materialStore,
		sessionStore,
		studyTestDB,
	).BuildStudySession(
		context.Background(),
		123,
		"ja",
		"N5",
		StudyProfileMorning,
		0,
	)
	if err == nil || !strings.Contains(
		err.Error(),
		"material insert failed",
	) {
		t.Fatalf(
			"error = %v, want material insert failure",
			err,
		)
	}
	if session != nil || createdSession == nil || createdSession.ID != 0 {
		t.Fatalf(
			"returned session = %+v, created session = %+v",
			session,
			createdSession,
		)
	}
}

func TestBuildStudySessionUsesAdjacentJapaneseLevelScope(t *testing.T) {
	materialStore := &mockStudyMaterialStore{
		getMaterialsByPlanFn: func(
			_ context.Context,
			_ int64,
			language,
			_ string,
			levels []string,
			_ model.StudySessionPlan,
		) ([]model.Material, error) {
			want := []string{"N5", "N4", "N3"}
			if !reflect.DeepEqual(
				levels,
				want,
			) {
				t.Fatalf(
					"session levels = %v, want %v",
					levels,
					want,
				)
			}
			return []model.Material{{ID: 10}}, nil
		},
	}
	sessionStore := &mockStudySessionStore{}

	session, err := newStudySessionService(
		materialStore,
		sessionStore,
		studyTestDB,
	).
		BuildStudySession(
			context.Background(),
			123,
			"ja",
			"N4",
			StudyProfileMorning,
			0,
		)
	if err != nil || session == nil || session.ID != 99 {
		t.Fatalf(
			"BuildStudySession() = %+v, %v",
			session,
			err,
		)
	}
}

func TestBuildStudySessionUsesRequestedLimit(t *testing.T) {
	ctx := context.Background()
	userID := int64(123)

	materialStore := &mockStudyMaterialStore{
		getMaterialsByPlanFn: func(
			ctx context.Context,
			gotUserID int64,
			language,
			level string,
			levels []string,
			plan model.StudySessionPlan,
		) ([]model.Material, error) {
			want := []string{"N5", "N4"}
			if gotUserID != userID || language != "ja" || level != "N5" || !reflect.DeepEqual(
				levels,
				want,
			) {
				t.Fatalf(
					"GetMaterialsByPlan args = (%d, %s, %v), want (%d, ja, %v)",
					gotUserID,
					language,
					levels,
					userID,
					want,
				)
			}
			if !reflect.DeepEqual(
				plan,
				morningStudySessionPlan,
			) {
				t.Fatalf(
					"plan = %+v, want %+v",
					plan,
					morningStudySessionPlan,
				)
			}
			return []model.Material{{ID: 10}}, nil
		},
	}
	sessionStore := &mockStudySessionStore{}

	svc := newStudySessionService(
		materialStore,
		sessionStore,
		studyTestDB,
	)
	session, err := svc.BuildStudySession(
		ctx,
		userID,
		"ja",
		"N5",
		StudyProfileMorning,
		20,
	)
	if err != nil {
		t.Fatalf(
			"BuildStudySession failed: %v",
			err,
		)
	}
	if session == nil || session.ID != 99 {
		t.Fatalf(
			"session = %+v, want id 99",
			session,
		)
	}
}

func TestBuildStudySessionRejectsOutOfRangeLimit(t *testing.T) {
	svc := newStudySessionService(
		nil,
		nil,
		nil,
	)

	for _, limit := range []int{-1, MaxStudySessionMaterialCount + 1} {
		_, err := svc.BuildStudySession(
			context.Background(),
			123,
			"ja",
			"N5",
			StudyProfileMorning,
			limit,
		)
		if err == nil {
			t.Fatalf(
				"limit %d: expected error",
				limit,
			)
		}
		if !strings.Contains(
			err.Error(),
			"invalid limit",
		) {
			t.Fatalf(
				"limit %d: unexpected error: %v",
				limit,
				err,
			)
		}
	}
}

func TestBuildStudySessionUsesFixedPlan(t *testing.T) {
	tests := []struct {
		profile StudySessionProfile
		limit   int
		want    model.StudySessionPlan
	}{
		{profile: StudyProfileMorning, want: studyPlan(
			8,
			7,
			1,
			3,
			1,
			0,
		)},
		{profile: StudyProfileEvening, want: studyPlan(
			4,
			14,
			1,
			3,
			0,
			2,
		)},
		{profile: StudyProfileEvening, limit: 10, want: studyPlan(
			4,
			14,
			1,
			3,
			0,
			2,
		)},
	}

	for _, tt := range tests {
		t.Run(
			fmt.Sprintf(
				"%s_limit_%d",
				tt.profile,
				tt.limit,
			),
			func(t *testing.T) {
				var got model.StudySessionPlan
				svc := newPlanCaptureStudySessionService(&got)
				session, err := svc.BuildStudySession(
					context.Background(),
					123,
					"ja",
					"N5",
					tt.profile,
					tt.limit,
				)
				if err != nil {
					t.Fatalf(
						"BuildStudySession failed: %v",
						err,
					)
				}
				if session == nil {
					t.Fatal("session = nil, want session")
				}
				if !reflect.DeepEqual(
					got,
					tt.want,
				) {
					t.Fatalf(
						"plan = %+v, want %+v",
						got,
						tt.want,
					)
				}
			},
		)
	}
}

func TestBuildStudySessionRejectsUnknownProfile(t *testing.T) {
	svc := newStudySessionService(
		nil,
		nil,
		nil,
	)
	_, err := svc.BuildStudySession(
		context.Background(),
		123,
		"ja",
		"N5",
		StudySessionProfile("night"),
		0,
	)
	if err == nil || !strings.Contains(
		err.Error(),
		"invalid profile",
	) {
		t.Fatalf(
			"error = %v, want invalid profile",
			err,
		)
	}
}

func TestBuildStudySessionScalesMorningPlan(t *testing.T) {
	tests := []struct {
		limit int
		want  model.StudySessionPlan
	}{
		{limit: 1, want: studyPlan(
			1,
			0,
			0,
			0,
			0,
			0,
		)},
		{limit: 2, want: studyPlan(
			1,
			1,
			0,
			0,
			0,
			0,
		)},
		{limit: 15, want: studyPlan(
			6,
			5,
			1,
			2,
			1,
			0,
		)},
		{limit: 20, want: studyPlan(
			8,
			7,
			1,
			3,
			1,
			0,
		)},
		{limit: 50, want: studyPlan(
			20,
			18,
			3,
			7,
			2,
			0,
		)},
	}

	for _, tt := range tests {
		t.Run(
			fmt.Sprintf(
				"limit_%d",
				tt.limit,
			),
			func(t *testing.T) {
				var got model.StudySessionPlan
				svc := newPlanCaptureStudySessionService(&got)
				if _, err := svc.BuildStudySession(
					context.Background(),
					123,
					"ja",
					"N5",
					StudyProfileMorning,
					tt.limit,
				); err != nil {
					t.Fatalf(
						"BuildStudySession failed: %v",
						err,
					)
				}
				if !reflect.DeepEqual(
					got,
					tt.want,
				) {
					t.Fatalf(
						"plan = %+v, want %+v",
						got,
						tt.want,
					)
				}
				if got.TotalMaterialCount() != tt.limit {
					t.Fatalf(
						"plan total = %d, want %d",
						got.TotalMaterialCount(),
						tt.limit,
					)
				}
				if got.Quotas[2].NewCount+got.Quotas[2].ReviewCount > 2 {
					t.Fatalf(
						"reading quota = %+v, exceeds cap 2",
						got.Quotas[2],
					)
				}
			},
		)
	}
}

func studyPlan(
	vocabNew,
	vocabReview,
	grammarNew,
	grammarReview,
	readingNew,
	readingReview int,
) model.StudySessionPlan {
	return model.StudySessionPlan{Quotas: []model.StudyMaterialQuota{
		{Category: model.MaterialCategoryVocabulary, NewCount: vocabNew, ReviewCount: vocabReview},
		{Category: model.MaterialCategoryGrammar, NewCount: grammarNew, ReviewCount: grammarReview},
		{Category: model.MaterialCategoryReading, NewCount: readingNew, ReviewCount: readingReview},
	}}
}

func newPlanCaptureStudySessionService(capture *model.StudySessionPlan) *studySessionService {
	return newStudySessionService(
		&mockStudyMaterialStore{getMaterialsByPlanFn: func(
			_ context.Context,
			_ int64,
			_,
			_ string,
			_ []string,
			plan model.StudySessionPlan,
		) ([]model.Material, error) {
			*capture = plan
			return []model.Material{{ID: 10}}, nil
		}},
		&mockStudySessionStore{},
		studyTestDB,
	)
}

func TestBuildStudySessionNoMaterialsReturnsNil(t *testing.T) {
	ctx := context.Background()
	materialStore := &mockStudyMaterialStore{
		getMaterialsByPlanFn: func(
			ctx context.Context,
			userID int64,
			language,
			level string,
			levels []string,
			plan model.StudySessionPlan,
		) ([]model.Material, error) {
			return nil, nil
		},
	}
	sessionStore := &mockStudySessionStore{
		createSessionFn: func(session *model.Session) (int, error) {
			t.Fatal("CreateSessionInTx should not be called")
			return 0, nil
		},
	}
	svc := newStudySessionService(
		materialStore,
		sessionStore,
		studyTestDB,
	)
	session, err := svc.BuildStudySession(
		ctx,
		123,
		"ja",
		"N5",
		StudyProfileMorning,
		0,
	)
	if err != nil {
		t.Fatalf(
			"BuildStudySession failed: %v",
			err,
		)
	}
	if session != nil {
		t.Fatalf(
			"session = %+v, want nil",
			session,
		)
	}
}

func TestBuildStudySessionWrapsMaterialError(t *testing.T) {
	ctx := context.Background()
	materialStore := &mockStudyMaterialStore{
		getMaterialsByPlanFn: func(
			ctx context.Context,
			userID int64,
			language,
			level string,
			levels []string,
			plan model.StudySessionPlan,
		) ([]model.Material, error) {
			return nil, errors.New("db down")
		},
	}
	svc := newStudySessionService(
		materialStore,
		nil,
		nil,
	)

	_, err := svc.BuildStudySession(
		ctx,
		123,
		"ja",
		"N5",
		StudyProfileMorning,
		0,
	)
	if err == nil {
		t.Fatal("expected error")
	}
	if !strings.Contains(
		err.Error(),
		"build study session fetch materials",
	) ||
		!strings.Contains(
			err.Error(),
			"db down",
		) {
		t.Fatalf(
			"unexpected error: %v",
			err,
		)
	}
}
