package service

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"testing"

	"github.com/lsj/copylingo/internal/model"
)

// newQuizTestSession seeds state into a fake Quiz store and wires a
// SessionService that grades with llm.
func newQuizTestSession(
	t *testing.T,
	state *model.QuizActiveSessionState,
	llm QuizGradingLLM,
) (*SessionService, *fakeQuizSessionStore) {
	t.Helper()
	store := newFakeQuizSessionStore()
	if err := store.Save(
		context.Background(),
		state.Session.ID,
		state,
	); err != nil {
		t.Fatalf(
			"seed failed: %v",
			err,
		)
	}
	return NewSessionService(SessionDeps{
		Stores: SessionStores{Quiz: store},
		LLM:    llm,
	}), store
}

func submitTestState(question model.Question) *model.QuizActiveSessionState {
	return activeStateForQuestion(
		10,
		question,
		false,
	)
}

func recordedAnswer(
	t *testing.T,
	store *fakeQuizSessionStore,
) (string, *bool) {
	t.Helper()
	sq := store.values[10].Items[0].SessionQuestion
	if sq.UserAnswer == nil {
		return "", sq.IsCorrect
	}
	return *sq.UserAnswer, sq.IsCorrect
}

func TestSessionServiceSubmitQuizOption(t *testing.T) {
	question := model.Question{
		ID:            1,
		Type:          model.QuestionMultipleChoice,
		CorrectAnswer: "B",
		Options:       json.RawMessage(`["A","B"]`),
	}

	t.Run(
		"records graded option",
		func(t *testing.T) {
			svc, store := newQuizTestSession(
				t,
				submitTestState(question),
				nil,
			)
			got, err := svc.SubmitQuizOption(
				context.Background(),
				1,
				10,
				1,
				1,
			)
			if err != nil {
				t.Fatalf(
					"SubmitQuizOption failed: %v",
					err,
				)
			}
			if !got.IsCorrect || got.Answer != "B" || got.QuestionIndex != 0 || got.TotalQuestions != 1 {
				t.Fatalf(
					"result = %+v",
					got,
				)
			}
			if answer, correct := recordedAnswer(
				t,
				store,
			); answer != "B" || correct == nil || !*correct {
				t.Fatalf(
					"recorded %q/%v, want B/true",
					answer,
					correct,
				)
			}
		},
	)

	tests := []struct {
		name       string
		answered   bool
		questionID int
		optionIdx  int
		wantErr    error
	}{
		{name: "option out of range", questionID: 1, optionIdx: 2, wantErr: ErrQuizInvalidOption},
		{name: "negative option", questionID: 1, optionIdx: -1, wantErr: ErrQuizInvalidOption},
		{name: "already answered is stale", answered: true, questionID: 1, optionIdx: 0, wantErr: ErrQuizAnswerStale},
		{name: "question not in session", questionID: 99, optionIdx: 0, wantErr: ErrQuizActiveSessionQuestionNotFound},
	}
	for _, tt := range tests {
		t.Run(
			tt.name,
			func(t *testing.T) {
				state := submitTestState(question)
				if tt.answered {
					correct := true
					state.Items[0].SessionQuestion.IsCorrect = &correct
				}
				svc, _ := newQuizTestSession(
					t,
					state,
					nil,
				)
				_, err := svc.SubmitQuizOption(
					context.Background(),
					1,
					10,
					tt.questionID,
					tt.optionIdx,
				)
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
			},
		)
	}

	t.Run(
		"existing but not current question is stale",
		func(t *testing.T) {
			state := submitTestState(question)
			second := state.Items[0]
			second.Question.ID = 2
			second.SessionQuestion.QuestionID = 2
			state.Items = append(
				state.Items,
				second,
			)
			svc, _ := newQuizTestSession(
				t,
				state,
				nil,
			)
			_, err := svc.SubmitQuizOption(
				context.Background(),
				1,
				10,
				2,
				0,
			)
			if !errors.Is(
				err,
				ErrQuizAnswerStale,
			) {
				t.Fatalf(
					"error = %v, want ErrQuizAnswerStale",
					err,
				)
			}
		},
	)

	t.Run(
		"missing working set is unavailable",
		func(t *testing.T) {
			// Neither Redis nor the DB has the session.
			svc := NewSessionService(SessionDeps{
				Stores: SessionStores{Quiz: newFakeQuizSessionStore()},
				QuizActiveSessionRepo: &fakeQuizActiveSessionRepo{loadFn: func(
					context.Context,
					int,
				) (*model.QuizActiveSessionState, error) {
					return nil, sql.ErrNoRows
				}},
			})
			_, err := svc.SubmitQuizOption(
				context.Background(),
				1,
				10,
				1,
				0,
			)
			if !errors.Is(
				err,
				ErrQuizStateUnavailable,
			) {
				t.Fatalf(
					"error = %v, want ErrQuizStateUnavailable",
					err,
				)
			}
		},
	)
}

func TestSessionServiceSubmitQuizText(t *testing.T) {
	t.Run(
		"fill-blank is trimmed and lowercased",
		func(t *testing.T) {
			svc, store := newQuizTestSession(
				t,
				submitTestState(model.Question{ID: 1, Type: model.QuestionFillBlank, CorrectAnswer: "ka"}),
				nil,
			)
			got, err := svc.SubmitQuizText(
				context.Background(),
				QuizTextAnswer{UserID: 1, SessionID: 10, QuestionIndex: 0, Text: "  KA "},
			)
			if err != nil || !got.IsCorrect || got.Answer != "ka" {
				t.Fatalf(
					"result = %+v err=%v",
					got,
					err,
				)
			}
			if answer, _ := recordedAnswer(
				t,
				store,
			); answer != "ka" {
				t.Fatalf(
					"recorded %q, want ka",
					answer,
				)
			}
		},
	)

	t.Run(
		"index out of range is question not found",
		func(t *testing.T) {
			svc, _ := newQuizTestSession(
				t,
				submitTestState(model.Question{ID: 1, Type: model.QuestionFillBlank, CorrectAnswer: "ka"}),
				nil,
			)
			_, err := svc.SubmitQuizText(
				context.Background(),
				QuizTextAnswer{UserID: 1, SessionID: 10, QuestionIndex: 1, Text: "x"},
			)
			if !errors.Is(
				err,
				ErrQuizActiveSessionQuestionNotFound,
			) {
				t.Fatalf(
					"error = %v, want ErrQuizActiveSessionQuestionNotFound",
					err,
				)
			}
		},
	)
}

func TestSessionServiceSubmitQuizWordOrder(t *testing.T) {
	question := model.Question{
		ID:            1,
		Type:          model.QuestionWordOrder,
		CorrectAnswer: "わたしはがくせい",
		Options:       json.RawMessage(`["がくせい","わたしは"]`),
	}

	svc, store := newQuizTestSession(
		t,
		submitTestState(question),
		nil,
	)
	for _, selection := range [][]int{{1}, {1, 1}, {1, 2}} {
		if _, err := svc.SubmitQuizWordOrder(
			context.Background(),
			1,
			10,
			1,
			selection,
		); !errors.Is(
			err,
			ErrQuizInvalidOption,
		) {
			t.Fatalf(
				"selection %v error = %v, want ErrQuizInvalidOption",
				selection,
				err,
			)
		}
	}

	got, err := svc.SubmitQuizWordOrder(
		context.Background(),
		1,
		10,
		1,
		[]int{1, 0},
	)
	if err != nil || !got.IsCorrect || got.Answer != "わたしはがくせい" {
		t.Fatalf(
			"result = %+v err=%v",
			got,
			err,
		)
	}
	if answer, _ := recordedAnswer(
		t,
		store,
	); answer != "わたしはがくせい" {
		t.Fatalf(
			"recorded %q",
			answer,
		)
	}
}

// A caller that does not own the session must be rejected before anything is recorded.
func TestSessionServiceSubmitQuizRejectsOtherUser(t *testing.T) {
	const otherUserID int64 = 2
	tests := []struct {
		name     string
		question model.Question
		submit   func(svc *SessionService) error
	}{
		{
			name: "option",
			question: model.Question{
				ID:            1,
				Type:          model.QuestionMultipleChoice,
				CorrectAnswer: "a",
				Options:       json.RawMessage(`["a","b"]`),
			},
			submit: func(svc *SessionService) error {
				_, err := svc.SubmitQuizOption(
					context.Background(),
					otherUserID,
					10,
					1,
					0,
				)
				return err
			},
		},
		{
			name: "word order",
			question: model.Question{
				ID:            1,
				Type:          model.QuestionWordOrder,
				CorrectAnswer: "わたしはがくせい",
				Options:       json.RawMessage(`["がくせい","わたしは"]`),
			},
			submit: func(svc *SessionService) error {
				_, err := svc.SubmitQuizWordOrder(
					context.Background(),
					otherUserID,
					10,
					1,
					[]int{1, 0},
				)
				return err
			},
		},
		{
			name:     "text",
			question: model.Question{ID: 1, Type: model.QuestionFillBlank, CorrectAnswer: "ka"},
			submit: func(svc *SessionService) error {
				_, err := svc.SubmitQuizText(
					context.Background(),
					QuizTextAnswer{UserID: otherUserID, SessionID: 10, QuestionIndex: 0, Text: "ka"},
				)
				return err
			},
		},
	}
	for _, tt := range tests {
		t.Run(
			tt.name,
			func(t *testing.T) {
				svc, store := newQuizTestSession(
					t,
					submitTestState(tt.question),
					nil,
				)
				err := tt.submit(svc)
				if !errors.Is(
					err,
					ErrQuizActiveSessionUserMismatch,
				) {
					t.Fatalf(
						"error = %v, want ErrQuizActiveSessionUserMismatch",
						err,
					)
				}
				if answer, isCorrect := recordedAnswer(
					t,
					store,
				); answer != "" || isCorrect != nil {
					t.Fatalf(
						"answer recorded: %q %v",
						answer,
						isCorrect,
					)
				}
			},
		)
	}
}

// A working-set save failure must surface to the caller, not report a result.
func TestSessionServiceSubmitQuizOptionSaveFailure(t *testing.T) {
	svc, store := newQuizTestSession(
		t,
		submitTestState(model.Question{
			ID:            1,
			Type:          model.QuestionMultipleChoice,
			CorrectAnswer: "a",
			Options:       json.RawMessage(`["a","b"]`),
		}),
		nil,
	)
	saveErr := errors.New("redis unavailable")
	store.setErr = saveErr

	result, err := svc.SubmitQuizOption(
		context.Background(),
		1,
		10,
		1,
		0,
	)
	if !errors.Is(
		err,
		saveErr,
	) {
		t.Fatalf(
			"error = %v, want save error",
			err,
		)
	}
	if result != nil {
		t.Fatalf(
			"result = %+v, want nil",
			result,
		)
	}
}
