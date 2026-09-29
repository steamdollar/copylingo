package service

import (
	"context"
	"encoding/json"
	"errors"
	"testing"

	"github.com/lsj/copylingo/internal/external"
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
			svc := NewSessionService(SessionDeps{Stores: SessionStores{Quiz: newFakeQuizSessionStore()}})
			_, err := svc.SubmitQuizOption(
				context.Background(),
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
	subjective := model.Question{ID: 1, Type: model.QuestionSubjective, Prompt: "p", CorrectAnswer: "I am"}

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
				QuizTextAnswer{SessionID: 10, QuestionIndex: 0, Text: "  KA "},
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

	// Subjective failure policy: AI unavailability records a wrong answer.
	t.Run(
		"subjective AI unavailable records wrong answer",
		func(t *testing.T) {
			hookCalls := 0
			llm := &mockLLM{gradeAnswerFn: func(
				context.Context,
				string,
				string,
				string,
			) (external.GradeResult, error) {
				if hookCalls != 1 {
					t.Fatalf(
						"OnAIGrading calls before grading = %d, want 1",
						hookCalls,
					)
				}
				return external.GradeResult{}, external.ErrAIConfigMissing
			}}
			svc, store := newQuizTestSession(
				t,
				submitTestState(subjective),
				llm,
			)
			got, err := svc.SubmitQuizText(
				context.Background(),
				QuizTextAnswer{
					SessionID:     10,
					QuestionIndex: 0,
					Text:          "I'm",
					OnAIGrading:   func() { hookCalls++ },
				},
			)
			if err != nil {
				t.Fatalf(
					"SubmitQuizText failed: %v",
					err,
				)
			}
			if !got.GradingUnavailable || got.IsCorrect {
				t.Fatalf(
					"result = %+v, want grading-unavailable wrong answer",
					got,
				)
			}
			if answer, correct := recordedAnswer(
				t,
				store,
			); answer != "I'm" || correct == nil || *correct {
				t.Fatalf(
					"recorded %q/%v, want I'm/false",
					answer,
					correct,
				)
			}
		},
	)

	t.Run(
		"subjective other LLM error is not recorded",
		func(t *testing.T) {
			llmErr := errors.New("llm timeout")
			llm := &mockLLM{gradeAnswerFn: func(
				context.Context,
				string,
				string,
				string,
			) (external.GradeResult, error) {
				return external.GradeResult{}, llmErr
			}}
			svc, store := newQuizTestSession(
				t,
				submitTestState(subjective),
				llm,
			)
			_, err := svc.SubmitQuizText(
				context.Background(),
				QuizTextAnswer{SessionID: 10, QuestionIndex: 0, Text: "x"},
			)
			if !errors.Is(
				err,
				llmErr,
			) || errors.Is(
				err,
				ErrQuizAnswerStale,
			) {
				t.Fatalf(
					"error = %v, want wrapped LLM error",
					err,
				)
			}
			if _, correct := recordedAnswer(
				t,
				store,
			); correct != nil {
				t.Fatal("non-availability LLM errors must not record an answer")
			}
		},
	)

	t.Run(
		"index out of range is question not found",
		func(t *testing.T) {
			svc, _ := newQuizTestSession(
				t,
				submitTestState(subjective),
				nil,
			)
			_, err := svc.SubmitQuizText(
				context.Background(),
				QuizTextAnswer{SessionID: 10, QuestionIndex: 1, Text: "x"},
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
