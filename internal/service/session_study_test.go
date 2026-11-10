package service

import (
	"context"
	"errors"
	"slices"
	"testing"
	"time"

	"github.com/lsj/copylingo/internal/model"
)

func TestSessionServiceFinishStudy(t *testing.T) {
	const sessionID = 20
	const ownerID = int64(7)

	tests := []struct {
		name          string
		studiedFirst  bool
		lastOrder     int
		wantErr       error
		notWantErr    error
		wantFlushed   bool
		wantStateLeft bool
	}{
		{
			name:         "marks last material and completes",
			studiedFirst: true,
			lastOrder:    1,
			wantFlushed:  true,
		},
		{
			name:          "unknown last material fails before completion",
			studiedFirst:  true,
			lastOrder:     5,
			wantErr:       ErrStudyFinalMarkFailed,
			notWantErr:    ErrStudyCompleteFailed,
			wantStateLeft: true,
		},
		{
			name:          "incomplete session fails at completion",
			studiedFirst:  false,
			lastOrder:     1,
			wantErr:       ErrStudyCompleteFailed,
			notWantErr:    ErrStudyFinalMarkFailed,
			wantStateLeft: true,
		},
	}
	for _, tt := range tests {
		t.Run(
			tt.name,
			func(t *testing.T) {
				ctx := context.Background()
				state := studyActiveState(
					sessionID,
					ownerID,
					model.SessionInProgress,
				)
				if tt.studiedFirst {
					state.MarkStudied(
						0,
						time.Now(),
					)
				}
				store := newFakeStudySessionStore()
				_ = store.Save(
					ctx,
					sessionID,
					state,
				)
				flushed := false
				sessionService := NewSessionService(SessionDependencies{
					StudyActiveSessionRepository: &fakeStudyActiveRepository{flushFunc: func(
						context.Context,
						*model.StudyActiveSessionState,
					) error {
						flushed = true
						return nil
					}},
					Stores: SessionStores{Study: store},
				})

				err := sessionService.FinishStudy(
					ctx,
					sessionID,
					ownerID,
					tt.lastOrder,
				)
				if tt.wantErr == nil && err != nil {
					t.Fatalf(
						"FinishStudy failed: %v",
						err,
					)
				}
				if tt.wantErr != nil && (!errors.Is(
					err,
					tt.wantErr,
				) || errors.Is(
					err,
					tt.notWantErr,
				)) {
					t.Fatalf(
						"error = %v, want only %v",
						err,
						tt.wantErr,
					)
				}
				if flushed != tt.wantFlushed {
					t.Fatalf(
						"flushed = %v, want %v",
						flushed,
						tt.wantFlushed,
					)
				}
				if _, left := store.values[sessionID]; left != tt.wantStateLeft {
					t.Fatalf(
						"working set present = %v, want %v",
						left,
						tt.wantStateLeft,
					)
				}
			},
		)
	}
}

// slotQuestionRepository and slotMaterialRepository report no content while recording
// which builder a slot reached.
type slotQuestionRepository struct {
	QuestionRepository
	dueLimits []int
}

func (repository *slotQuestionRepository) GetDueReviews(
	_ context.Context,
	_ int64,
	_,
	_ string,
	_ []string,
	limit,
	_ int,
	_ ...model.QuestionCategory,
) ([]model.Question, error) {
	repository.dueLimits = append(
		repository.dueLimits,
		limit,
	)
	return nil, nil
}

func (repository *slotQuestionRepository) GetNewQuestions(
	context.Context,
	int64,
	string,
	[]string,
	string,
	[]int,
	int,
	int,
) ([]model.Question, error) {
	return nil, nil
}

type slotMaterialRepository struct {
	planTotals []int
}

func (repository *slotMaterialRepository) GetMaterialsByPlan(
	_ context.Context,
	_ int64,
	_,
	_ string,
	_ []string,
	plan model.StudySessionPlan,
) ([]model.Material, error) {
	repository.planTotals = append(
		repository.planTotals,
		plan.TotalMaterialCount(),
	)
	return nil, nil
}

func TestSessionServiceBuildForSlot(t *testing.T) {
	user := model.User{ID: 7, Language: "ja", ProficiencyLevel: "N5"}
	morningStudyTotal := morningStudySessionPlan.TotalMaterialCount()
	eveningStudyTotal := eveningStudySessionPlan.TotalMaterialCount()

	tests := []struct {
		slot           model.SessionSlot
		wantPlanTotal  int
		wantDueLimitIn int
	}{
		{slot: model.SessionSlotMorningStudy, wantPlanTotal: morningStudyTotal},
		{slot: model.SessionSlotEveningStudy, wantPlanTotal: eveningStudyTotal},
		// General due pool is fetched with total+2 rows (17 morning, 17 evening).
		{slot: model.SessionSlotMorningQuiz, wantDueLimitIn: 19},
		{slot: model.SessionSlotEveningQuiz, wantDueLimitIn: 19},
	}
	for _, tt := range tests {
		t.Run(
			string(tt.slot),
			func(t *testing.T) {
				questions := &slotQuestionRepository{}
				materials := &slotMaterialRepository{}
				sessionService := NewSessionService(SessionDependencies{
					QuestionRepository: questions,
					MaterialRepository: materials,
				})

				session, err := sessionService.BuildForSlot(
					context.Background(),
					user,
					tt.slot,
				)
				if err != nil || session != nil {
					t.Fatalf(
						"BuildForSlot = %v, %v; want nil session without error",
						session,
						err,
					)
				}
				if tt.wantPlanTotal != 0 && (!slices.Equal(
					materials.planTotals,
					[]int{tt.wantPlanTotal},
				) || len(questions.dueLimits) != 0) {
					t.Fatalf(
						"study slot: plan totals=%v due=%v, want plan %d only",
						materials.planTotals,
						questions.dueLimits,
						tt.wantPlanTotal,
					)
				}
				if tt.wantDueLimitIn != 0 && (!slices.Contains(
					questions.dueLimits,
					tt.wantDueLimitIn,
				) || len(materials.planTotals) != 0) {
					t.Fatalf(
						"quiz slot: due limits=%v plans=%v, want due limit %d",
						questions.dueLimits,
						materials.planTotals,
						tt.wantDueLimitIn,
					)
				}
			},
		)
	}

	if _, err := NewSessionService(SessionDependencies{}).BuildForSlot(
		context.Background(),
		user,
		model.SessionSlot("night"),
	); err == nil {
		t.Fatal("unsupported slot must fail")
	}
}
