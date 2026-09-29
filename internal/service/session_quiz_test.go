package service

import (
	"context"
	"errors"
	"slices"
	"testing"

	"github.com/lsj/copylingo/internal/model"
)

// fakeQuestionRepo implements QuestionRepo for review-Quiz tests. The embedded
// nil interface makes an unexpected repository call panic.
type fakeQuestionRepo struct {
	QuestionRepo
	dueCount    int
	dueCountErr error
	dueLimit    int
}

func (r *fakeQuestionRepo) GetDueReviewCount(
	context.Context,
	int64,
	string,
	[]string,
) (int, error) {
	return r.dueCount, r.dueCountErr
}

func (r *fakeQuestionRepo) GetDueReviews(
	_ context.Context,
	_ int64,
	_,
	currentLevel string,
	_ []string,
	limit,
	_ int,
	_ ...model.QuestionCategory,
) ([]model.Question, error) {
	r.dueLimit = limit
	questions := make(
		[]model.Question,
		limit,
	)
	for i := range questions {
		questions[i] = model.Question{ID: i + 1, ProficiencyLevel: currentLevel}
	}
	return questions, nil
}

type fakeStreakRepo struct {
	userIDs []int64
	err     error
}

func (r *fakeStreakRepo) UpdateStreak(
	_ context.Context,
	userID int64,
) error {
	r.userIDs = append(
		r.userIDs,
		userID,
	)
	return r.err
}

func quizStateWithStatus(
	sessionID int,
	userID int64,
	status model.SessionStatus,
	answered bool,
) *model.QuizActiveSessionState {
	state := quizActiveSessionTestState(
		sessionID,
		userID,
		answered,
	)
	state.Session.Status = status
	return state
}

func TestSessionServiceStartQuiz(t *testing.T) {
	const sessionID = 10
	const ownerID = int64(7)
	prepareErr := errors.New("db load failed")
	startErr := errors.New("db start failed")

	tests := []struct {
		name          string
		seeded        *model.QuizActiveSessionState
		callerID      int64
		startErr      error
		reloadErr     error
		wantErr       error
		wantStarted   bool
		wantReloaded  bool
		wantStatus    model.SessionStatus
		wantAnswerKey bool
	}{
		{
			name: "pending reloads from DB after start",
			seeded: quizStateWithStatus(
				sessionID,
				ownerID,
				model.SessionPending,
				false,
			),
			callerID:     ownerID,
			wantStarted:  true,
			wantReloaded: true,
			wantStatus:   model.SessionInProgress,
		},
		{
			name: "in-progress keeps Redis answers",
			seeded: quizStateWithStatus(
				sessionID,
				ownerID,
				model.SessionInProgress,
				true,
			),
			callerID:      ownerID,
			wantStarted:   true,
			wantStatus:    model.SessionInProgress,
			wantAnswerKey: true,
		},
		{
			name: "non-owner rejected before DB start",
			seeded: quizStateWithStatus(
				sessionID,
				ownerID,
				model.SessionPending,
				false,
			),
			callerID: ownerID + 1,
			wantErr:  ErrQuizActiveSessionUserMismatch,
		},
		{
			name: "zero owner skips owner check",
			seeded: quizStateWithStatus(
				sessionID,
				0,
				model.SessionInProgress,
				true,
			),
			callerID:      ownerID,
			wantStarted:   true,
			wantStatus:    model.SessionInProgress,
			wantAnswerKey: true,
		},
		{
			name: "reload failure after start is prepare failure",
			seeded: quizStateWithStatus(
				sessionID,
				ownerID,
				model.SessionPending,
				false,
			),
			callerID:     ownerID,
			reloadErr:    prepareErr,
			wantErr:      ErrQuizStatePrepareFailed,
			wantStarted:  true,
			wantReloaded: true,
		},
		{
			name: "DB start failure is not prepare failure",
			seeded: quizStateWithStatus(
				sessionID,
				ownerID,
				model.SessionPending,
				false,
			),
			callerID:    ownerID,
			startErr:    startErr,
			wantErr:     startErr,
			wantStarted: true,
		},
	}

	for _, tt := range tests {
		t.Run(
			tt.name,
			func(t *testing.T) {
				ctx := context.Background()
				store := newFakeQuizSessionStore()
				if err := store.Save(
					ctx,
					sessionID,
					tt.seeded,
				); err != nil {
					t.Fatalf(
						"seed failed: %v",
						err,
					)
				}
				started, reloaded := false, false
				svc := NewSessionService(SessionDeps{
					SessionRepo: &fakeSessionRepo{startFn: func(id int) error {
						started = true
						return tt.startErr
					}},
					QuizActiveSessionRepo: &fakeQuizActiveSessionRepo{loadFn: func(
						context.Context,
						int,
					) (*model.QuizActiveSessionState, error) {
						reloaded = true
						if tt.reloadErr != nil {
							return nil, tt.reloadErr
						}
						return quizStateWithStatus(
							sessionID,
							ownerID,
							model.SessionInProgress,
							false,
						), nil
					}},
					Stores: SessionStores{Quiz: store},
				})

				state, err := svc.StartQuiz(
					ctx,
					sessionID,
					tt.callerID,
				)
				if tt.wantErr != nil {
					if !errors.Is(
						err,
						tt.wantErr,
					) {
						t.Fatalf(
							"error = %v, want %v",
							err,
							tt.wantErr,
						)
					}
					if tt.wantErr == startErr && errors.Is(
						err,
						ErrQuizStatePrepareFailed,
					) {
						t.Fatalf(
							"DB start error must not be reported as prepare failure: %v",
							err,
						)
					}
				} else if err != nil {
					t.Fatalf(
						"StartQuiz failed: %v",
						err,
					)
				}
				if started != tt.wantStarted || reloaded != tt.wantReloaded {
					t.Fatalf(
						"started=%v reloaded=%v, want started=%v reloaded=%v",
						started,
						reloaded,
						tt.wantStarted,
						tt.wantReloaded,
					)
				}
				if tt.wantErr != nil {
					return
				}
				if state.Session.Status != tt.wantStatus {
					t.Fatalf(
						"status = %s, want %s",
						state.Session.Status,
						tt.wantStatus,
					)
				}
				if answered := state.Items[0].SessionQuestion.IsCorrect != nil; answered != tt.wantAnswerKey {
					t.Fatalf(
						"answered = %v, want %v",
						answered,
						tt.wantAnswerKey,
					)
				}
			},
		)
	}
}

func TestSessionServiceBuildReviewQuiz(t *testing.T) {
	user := model.User{ID: 7, Language: "ja", ProficiencyLevel: "N5"}

	tests := []struct {
		name        string
		dueCount    int
		dueCountErr error
		wantErr     error
		wantLimit   int
	}{
		{
			name:     "no due items",
			dueCount: 0,
			wantErr:  ErrNoDueReviews,
		},
		{
			name:        "due count error is treated as no due items",
			dueCount:    5,
			dueCountErr: errors.New("count failed"),
			wantErr:     ErrNoDueReviews,
		},
		{
			name:      "backlog is capped",
			dueCount:  40,
			wantLimit: maxReviewQuizQuestions,
		},
		{
			name:      "small backlog uses its size",
			dueCount:  4,
			wantLimit: 4,
		},
	}

	for _, tt := range tests {
		t.Run(
			tt.name,
			func(t *testing.T) {
				questions := &fakeQuestionRepo{dueCount: tt.dueCount, dueCountErr: tt.dueCountErr}
				sessions := &fakeSessionRepo{}
				svc := NewSessionService(SessionDeps{
					QuestionRepo: questions,
					SessionRepo:  sessions,
					SessionQuestionRepo: &mockSessionQuestionStore{createSessionQuestionsFn: func(
						context.Context,
						[]model.SessionQuestion,
					) error {
						return nil
					}},
				})

				session, err := svc.BuildReviewQuiz(
					context.Background(),
					user,
				)
				if tt.wantErr != nil {
					if !errors.Is(
						err,
						tt.wantErr,
					) {
						t.Fatalf(
							"error = %v, want %v",
							err,
							tt.wantErr,
						)
					}
					if len(sessions.created) != 0 {
						t.Fatalf(
							"created %d sessions, want none",
							len(sessions.created),
						)
					}
					return
				}
				if err != nil {
					t.Fatalf(
						"BuildReviewQuiz failed: %v",
						err,
					)
				}
				if questions.dueLimit != tt.wantLimit || session.TotalQuestions != tt.wantLimit {
					t.Fatalf(
						"due limit=%d total=%d, want %d",
						questions.dueLimit,
						session.TotalQuestions,
						tt.wantLimit,
					)
				}
				if session.Type != model.SessionReview {
					t.Fatalf(
						"type = %s, want review",
						session.Type,
					)
				}
			},
		)
	}
}

func TestSessionServiceShowQuizQuestion(t *testing.T) {
	ctx := context.Background()
	const sessionID = 10
	store := newFakeQuizSessionStore()
	state := quizStateWithStatus(
		sessionID,
		7,
		model.SessionInProgress,
		false,
	)
	state.Items = append(
		state.Items,
		state.Items[0],
	)
	state.Items[1].Question.ID = 2
	state.Items[1].SessionQuestion.QuestionID = 2
	if err := store.Save(
		ctx,
		sessionID,
		state,
	); err != nil {
		t.Fatalf(
			"seed failed: %v",
			err,
		)
	}
	svc := NewSessionService(SessionDeps{Stores: SessionStores{Quiz: store}})

	got, err := svc.ShowQuizQuestion(
		ctx,
		sessionID,
		1,
	)
	if err != nil || got.CurrentIndex != 1 || store.values[sessionID].CurrentIndex != 1 {
		t.Fatalf(
			"move to 1: err=%v returned=%v stored=%d",
			err,
			got,
			store.values[sessionID].CurrentIndex,
		)
	}

	// Past-the-end renders the finish screen without moving the cursor.
	got, err = svc.ShowQuizQuestion(
		ctx,
		sessionID,
		2,
	)
	if err != nil || got.CurrentIndex != 1 || store.values[sessionID].CurrentIndex != 1 {
		t.Fatalf(
			"past end: err=%v stored=%d",
			err,
			store.values[sessionID].CurrentIndex,
		)
	}

	if _, err := svc.ShowQuizQuestion(
		ctx,
		sessionID,
		-1,
	); !errors.Is(
		err,
		ErrQuizActiveSessionQuestionNotFound,
	) {
		t.Fatalf(
			"negative index error = %v, want ErrQuizActiveSessionQuestionNotFound",
			err,
		)
	}
}

func TestSessionServiceCompleteQuiz(t *testing.T) {
	const sessionID = 10
	const ownerID = int64(7)

	newAnsweredState := func() *model.QuizActiveSessionState {
		state := quizStateWithStatus(
			sessionID,
			ownerID,
			model.SessionInProgress,
			true,
		)
		wordOrder := state.Items[0]
		wordOrder.Question.ID = 2
		wordOrder.Question.Type = model.QuestionWordOrder
		wordOrder.SessionQuestion.QuestionID = 2
		state.Items = append(
			state.Items,
			wordOrder,
		)
		return state
	}

	t.Run(
		"owner flushes, updates streak, deletes and returns word-order IDs",
		func(t *testing.T) {
			ctx := context.Background()
			store := newFakeQuizSessionStore()
			_ = store.Save(
				ctx,
				sessionID,
				newAnsweredState(),
			)
			streak := &fakeStreakRepo{}
			flushed := false
			svc := NewSessionService(SessionDeps{
				QuizActiveSessionRepo: &fakeQuizActiveSessionRepo{flushFn: func(
					context.Context,
					*model.QuizActiveSessionState,
				) error {
					flushed = true
					return nil
				}},
				UserRepo: streak,
				Stores:   SessionStores{Quiz: store},
			})

			got, err := svc.CompleteQuiz(
				ctx,
				sessionID,
				ownerID,
			)
			if err != nil {
				t.Fatalf(
					"CompleteQuiz failed: %v",
					err,
				)
			}
			if !flushed || !slices.Equal(
				streak.userIDs,
				[]int64{ownerID},
			) {
				t.Fatalf(
					"flushed=%v streak=%v",
					flushed,
					streak.userIDs,
				)
			}
			if _, ok := store.values[sessionID]; ok {
				t.Fatal("working set must be deleted after completion")
			}
			if got.TotalQuestions != 2 || !slices.Equal(
				got.WordOrderQuestionIDs,
				[]int{2},
			) {
				t.Fatalf(
					"result = %+v",
					got,
				)
			}
		},
	)

	t.Run(
		"non-owner fails without streak or delete",
		func(t *testing.T) {
			ctx := context.Background()
			store := newFakeQuizSessionStore()
			_ = store.Save(
				ctx,
				sessionID,
				newAnsweredState(),
			)
			streak := &fakeStreakRepo{}
			svc := NewSessionService(SessionDeps{
				QuizActiveSessionRepo: &fakeQuizActiveSessionRepo{},
				UserRepo:              streak,
				Stores:                SessionStores{Quiz: store},
			})

			_, err := svc.CompleteQuiz(
				ctx,
				sessionID,
				ownerID+1,
			)
			if !errors.Is(
				err,
				ErrQuizActiveSessionUserMismatch,
			) {
				t.Fatalf(
					"error = %v, want ErrQuizActiveSessionUserMismatch",
					err,
				)
			}
			if len(streak.userIDs) != 0 {
				t.Fatalf(
					"streak updated for non-owner: %v",
					streak.userIDs,
				)
			}
			if _, ok := store.values[sessionID]; !ok {
				t.Fatal("working set must survive a rejected completion")
			}
		},
	)
}
