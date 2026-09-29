package service

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/lsj/copylingo/internal/external"
	"github.com/lsj/copylingo/internal/model"
)

type mockRenderer struct {
	renderPNGFn func(strokes []Stroke) ([]byte, error)
}

func (m *mockRenderer) RenderPNG(strokes []Stroke) ([]byte, error) {
	return m.renderPNGFn(strokes)
}

func fakeImageRenderer() *mockRenderer {
	return &mockRenderer{renderPNGFn: func([]Stroke) ([]byte, error) {
		return []byte("fake-image"), nil
	}}
}

// newHandwritingTestSession is newQuizTestSession with a stroke renderer override.
func newHandwritingTestSession(
	t *testing.T,
	state *model.QuizActiveSessionState,
	llm QuizGradingLLM,
	renderer StrokeRenderer,
) (*SessionService, *fakeQuizSessionStore) {
	t.Helper()
	svc, store := newQuizTestSession(
		t,
		state,
		llm,
	)
	if renderer != nil {
		svc.strokeRenderer = renderer
	}
	return svc, store
}

func handwritingGradeLLM(
	result external.GradeResult,
	err error,
) *mockLLM {
	return &mockLLM{gradeHandwritingFn: func(
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

	llm := &mockLLM{gradeHandwritingFn: func(
		_ context.Context,
		_,
		correctAnswer string,
		img []byte,
	) (external.GradeResult, error) {
		if correctAnswer != "あ" || string(img) != "fake-image" {
			t.Fatalf(
				"unexpected grade args answer=%q img=%q",
				correctAnswer,
				string(img),
			)
		}
		return external.GradeResult{IsCorrect: true, Feedback: "Correct!"}, nil
	}}
	svc, store := newHandwritingTestSession(
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
		fakeImageRenderer(),
	)

	res, err := svc.SubmitHandwriting(
		ctx,
		HandwritingSubmitRequest{
			UserID:     userID,
			SessionID:  sessionID,
			QuestionID: questionID,
			Strokes:    []Stroke{{Points: []StrokePoint{{X: 0, Y: 0}}}},
		},
	)
	if err != nil {
		t.Fatalf(
			"SubmitHandwriting failed: %v",
			err,
		)
	}
	if !res.IsCorrect || res.Feedback != "Correct!" {
		t.Fatalf(
			"unexpected grade result: %+v",
			res,
		)
	}
	if res.CorrectAnswer != "あ" || res.Explanation != "hiragana a" {
		t.Fatalf(
			"unexpected public result: %+v",
			res,
		)
	}
	if recorded := store.values[sessionID].Items[0].SessionQuestion.IsCorrect; recorded == nil || !*recorded {
		t.Fatalf(
			"recorded IsCorrect = %v, want true",
			recorded,
		)
	}
}

func TestSubmitHandwriting_WrongSavesRenderedImage(t *testing.T) {
	ctx := context.Background()
	userID := int64(123)
	sessionID := 10
	questionID := 1
	imageDir := t.TempDir()
	previousImageDir := failedHandwritingImageDir
	failedHandwritingImageDir = imageDir
	defer func() { failedHandwritingImageDir = previousImageDir }()

	svc, _ := newHandwritingTestSession(
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
		fakeImageRenderer(),
	)

	res, err := svc.SubmitHandwriting(
		ctx,
		HandwritingSubmitRequest{
			UserID:     userID,
			SessionID:  sessionID,
			QuestionID: questionID,
			Strokes:    []Stroke{{Points: []StrokePoint{{X: 0, Y: 0}}}},
		},
	)
	if err != nil {
		t.Fatalf(
			"SubmitHandwriting failed: %v",
			err,
		)
	}
	if res.IsCorrect {
		t.Fatal("expected wrong handwriting result")
	}
	got, err := os.ReadFile(filepath.Join(
		imageDir,
		"100.png",
	))
	if err != nil {
		t.Fatalf(
			"failed to read saved image: %v",
			err,
		)
	}
	if string(got) != "fake-image" {
		t.Fatalf(
			"saved image = %q, want fake-image",
			string(got),
		)
	}
}

// Handwriting failure policy: unlike subjective text answers, an AI grading
// failure is returned and nothing is recorded, so the user can resubmit.
func TestSubmitHandwriting_AIUnavailableReturnsErrorWithoutRecording(t *testing.T) {
	ctx := context.Background()
	userID := int64(123)
	sessionID := 10
	svc, store := newHandwritingTestSession(
		t,
		handwritingState(
			userID,
			sessionID,
			model.Question{ID: 1, Type: model.QuestionKanaHandwriting, CorrectAnswer: "あ"},
			false,
		),
		handwritingGradeLLM(
			external.GradeResult{},
			external.ErrAIConfigMissing,
		),
		fakeImageRenderer(),
	)

	_, err := svc.SubmitHandwriting(
		ctx,
		HandwritingSubmitRequest{UserID: userID, SessionID: sessionID, QuestionID: 1},
	)
	if !errors.Is(
		err,
		ErrAIUnavailable,
	) {
		t.Fatalf(
			"error = %v, want ErrAIUnavailable",
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
				// A nil LLM and renderer make any grading attempt panic.
				svc, _ := newHandwritingTestSession(
					t,
					handwritingState(
						tt.ownerID,
						10,
						tt.question,
						tt.answered,
					),
					nil,
					nil,
				)
				svc.strokeRenderer = nil
				_, err := svc.SubmitHandwriting(
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

	svc, store := newHandwritingTestSession(
		t,
		state,
		handwritingGradeLLM(
			external.GradeResult{IsCorrect: true},
			nil,
		),
		fakeImageRenderer(),
	)
	if _, err := svc.SubmitHandwriting(
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
