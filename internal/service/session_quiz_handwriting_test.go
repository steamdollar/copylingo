package service

import (
	"context"
	"errors"
	"testing"

	"github.com/lsj/copylingo/internal/external"
	"github.com/lsj/copylingo/internal/model"
)

type mockLLM struct {
	gradeHandwritingFunc func(
		ctx context.Context,
		prompt,
		correctAnswer string,
		image []byte,
	) (external.GradeResult, error)
}

func (mock *mockLLM) GradeHandwriting(
	ctx context.Context,
	prompt,
	correctAnswer string,
	image []byte,
) (external.GradeResult, error) {
	return mock.gradeHandwritingFunc(
		ctx,
		prompt,
		correctAnswer,
		image,
	)
}

func handwritingGradeLLM(
	result external.GradeResult,
	err error,
) *mockLLM {
	return &mockLLM{gradeHandwritingFunc: func(
		context.Context,
		string,
		string,
		[]byte,
	) (external.GradeResult, error) {
		return result, err
	}}
}

func TestSubmitHandwriting_Success(t *testing.T) {
	ctx := context.Background()
	userID := int64(123)
	sessionID := 10
	questionID := 1

	llm := &mockLLM{gradeHandwritingFunc: func(
		_ context.Context,
		_,
		correctAnswer string,
		image []byte,
	) (external.GradeResult, error) {
		if correctAnswer != "あ" || string(image) != "fake-image" {
			t.Fatalf(
				"unexpected grade args answer=%q img=%q",
				correctAnswer,
				string(image),
			)
		}
		return external.GradeResult{IsCorrect: true, Feedback: "Correct!"}, nil
	}}
	sessionService, store := newQuizTestSession(
		t,
		handwritingState(
			userID,
			sessionID,
			model.Question{
				ID:            questionID,
				Type:          model.QuestionKanaHandwriting,
				CorrectAnswer: "あ",
				Explanation:   "hiragana a",
			},
			false,
		),
		llm,
	)

	result, err := sessionService.SubmitHandwriting(
		ctx,
		HandwritingSubmitRequest{
			UserID:     userID,
			SessionID:  sessionID,
			QuestionID: questionID,
			Image:      []byte("fake-image"),
		},
	)
	if err != nil {
		t.Fatalf(
			"SubmitHandwriting failed: %v",
			err,
		)
	}
	if !result.IsCorrect || result.Feedback != "Correct!" {
		t.Fatalf(
			"unexpected grade result: %+v",
			result,
		)
	}
	if result.CorrectAnswer != "あ" || result.Explanation != "hiragana a" {
		t.Fatalf(
			"unexpected public result: %+v",
			result,
		)
	}
	if recorded := store.values[sessionID].Items[0].SessionQuestion.IsCorrect; recorded == nil || !*recorded {
		t.Fatalf(
			"recorded IsCorrect = %v, want true",
			recorded,
		)
	}
}

func TestSubmitHandwriting_WrongRecordsIncorrect(t *testing.T) {
	ctx := context.Background()
	userID := int64(123)
	sessionID := 10
	questionID := 1

	sessionService, store := newQuizTestSession(
		t,
		handwritingState(
			userID,
			sessionID,
			model.Question{
				ID:            questionID,
				Type:          model.QuestionKanaHandwriting,
				CorrectAnswer: "あ",
			},
			false,
		),
		handwritingGradeLLM(
			external.GradeResult{IsCorrect: false, Feedback: "Try again"},
			nil,
		),
	)

	result, err := sessionService.SubmitHandwriting(
		ctx,
		HandwritingSubmitRequest{
			UserID:     userID,
			SessionID:  sessionID,
			QuestionID: questionID,
			Image:      []byte("fake-image"),
		},
	)
	if err != nil {
		t.Fatalf(
			"SubmitHandwriting failed: %v",
			err,
		)
	}
	if result.IsCorrect {
		t.Fatal("expected wrong handwriting result")
	}
	if recorded := store.values[sessionID].Items[0].SessionQuestion.IsCorrect; recorded == nil || *recorded {
		t.Fatalf(
			"recorded IsCorrect = %v, want false",
			recorded,
		)
	}
}

// Handwriting failure policy: unlike subjective text answers, an AI grading
// failure is returned and nothing is recorded, so the user can resubmit.
func TestSubmitHandwriting_LLMErrorReturnsErrorWithoutRecording(t *testing.T) {
	ctx := context.Background()
	llmErr := errors.New("llm unauthorized")
	userID := int64(123)
	sessionID := 10
	sessionService, store := newQuizTestSession(
		t,
		handwritingState(
			userID,
			sessionID,
			model.Question{ID: 1, Type: model.QuestionKanaHandwriting, CorrectAnswer: "あ"},
			false,
		),
		handwritingGradeLLM(
			external.GradeResult{},
			llmErr,
		),
	)

	_, err := sessionService.SubmitHandwriting(
		ctx,
		HandwritingSubmitRequest{UserID: userID, SessionID: sessionID, QuestionID: 1},
	)
	if !errors.Is(
		err,
		llmErr,
	) {
		t.Fatalf(
			"error = %v, want wrapped LLM error",
			err,
		)
	}
	if recorded := store.values[sessionID].Items[0].SessionQuestion.IsCorrect; recorded != nil {
		t.Fatalf(
			"handwriting AI failure must not record an answer, got %v",
			*recorded,
		)
	}
}

func TestSubmitHandwriting_RejectsBeforeGrading(t *testing.T) {
	tests := []struct {
		name     string
		ownerID  int64
		question model.Question
		answered bool
		wantErr  error
	}{
		{
			name:     "non-owner",
			ownerID:  456,
			question: model.Question{ID: 1, Type: model.QuestionKanaHandwriting},
			wantErr:  ErrHandwritingUnauthorized,
		},
		{
			name:     "not a handwriting question",
			ownerID:  123,
			question: model.Question{ID: 1, Type: model.QuestionMultipleChoice},
			wantErr:  ErrHandwritingInvalidQuestion,
		},
		{
			name:     "already answered",
			ownerID:  123,
			question: model.Question{ID: 1, Type: model.QuestionKanaHandwriting},
			answered: true,
			wantErr:  ErrHandwritingAlreadyAnswered,
		},
	}
	for _, tt := range tests {
		t.Run(
			tt.name,
			func(t *testing.T) {
				// A nil LLM makes any grading attempt panic.
				sessionService, _ := newQuizTestSession(
					t,
					handwritingState(
						tt.ownerID,
						10,
						tt.question,
						tt.answered,
					),
					nil,
				)
				_, err := sessionService.SubmitHandwriting(
					context.Background(),
					HandwritingSubmitRequest{UserID: 123, SessionID: 10, QuestionID: 1},
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
}

func TestSubmitHandwriting_UsesCurrentDuplicateOccurrence(t *testing.T) {
	ctx := context.Background()
	userID := int64(123)
	sessionID := 10
	questionID := 1

	state := handwritingState(
		userID,
		sessionID,
		model.Question{
			ID:            questionID,
			Type:          model.QuestionKanaHandwriting,
			CorrectAnswer: "あ",
		},
		true,
	)
	state.Items = append(
		state.Items,
		model.QuizActiveSessionQuestion{
			SessionQuestion: model.SessionQuestion{ID: 101, SessionID: sessionID, QuestionID: questionID},
			Question: model.Question{
				ID:            questionID,
				Type:          model.QuestionKanaHandwriting,
				CorrectAnswer: "あ",
			},
		},
	)
	state.CurrentIndex = 1

	sessionService, store := newQuizTestSession(
		t,
		state,
		handwritingGradeLLM(
			external.GradeResult{IsCorrect: true},
			nil,
		),
	)
	if _, err := sessionService.SubmitHandwriting(
		ctx,
		HandwritingSubmitRequest{
			UserID:     userID,
			SessionID:  sessionID,
			QuestionID: questionID,
		},
	); err != nil {
		t.Fatalf(
			"SubmitHandwriting failed: %v",
			err,
		)
	}
	if store.values[sessionID].Items[1].SessionQuestion.IsCorrect == nil {
		t.Fatal("the current (second) occurrence must be recorded")
	}
}

func handwritingState(
	userID int64,
	sessionID int,
	question model.Question,
	answered bool,
) *model.QuizActiveSessionState {
	state := activeStateForQuestion(
		sessionID,
		question,
		answered,
	)
	state.Session.UserID = userID
	return state
}
