package service

import (
	"context"
	"errors"
	"testing"

	"github.com/lsj/copylingo/internal/model"
)

type mockHandwritingActiveSession struct {
	getFn func(ctx context.Context, sessionID int) (*model.ActiveSessionState, error)
}

func (m *mockHandwritingActiveSession) Get(ctx context.Context, sessionID int) (*model.ActiveSessionState, error) {
	return m.getFn(ctx, sessionID)
}

type mockGraderClient struct {
	gradeHandwritingFn func(ctx context.Context, sid, qid int, q *model.Question, img []byte) (bool, string, error)
}

func (m *mockGraderClient) GradeHandwritingWithQuestion(ctx context.Context, sid, qid int, q *model.Question, img []byte) (bool, string, error) {
	return m.gradeHandwritingFn(ctx, sid, qid, q, img)
}

type mockRenderer struct {
	renderPNGFn func(strokes []Stroke) ([]byte, error)
}

func (m *mockRenderer) RenderPNG(strokes []Stroke) ([]byte, error) {
	return m.renderPNGFn(strokes)
}

func TestSubmitAnswer_Success(t *testing.T) {
	ctx := context.Background()
	userID := int64(123)
	sessionID := 10
	questionID := 1

	active := &mockHandwritingActiveSession{
		getFn: func(ctx context.Context, id int) (*model.ActiveSessionState, error) {
			return handwritingState(userID, sessionID, model.Question{
				ID:            questionID,
				Type:          model.QuestionKanaHandwriting,
				CorrectAnswer: "あ",
				Explanation:   "hiragana a",
			}, false), nil
		},
	}
	grader := &mockGraderClient{
		gradeHandwritingFn: func(ctx context.Context, sid, qid int, q *model.Question, img []byte) (bool, string, error) {
			if sid != sessionID || qid != questionID || q.CorrectAnswer != "あ" || string(img) != "fake-image" {
				t.Fatalf("unexpected grade args sid=%d qid=%d q=%+v img=%q", sid, qid, q, string(img))
			}
			return true, "Correct!", nil
		},
	}
	renderer := &mockRenderer{
		renderPNGFn: func(strokes []Stroke) ([]byte, error) {
			return []byte("fake-image"), nil
		},
	}

	svc := NewHandwritingService(active, grader, renderer)
	res, err := svc.SubmitAnswer(ctx, HandwritingSubmitRequest{
		UserID:     userID,
		SessionID:  sessionID,
		QuestionID: questionID,
		Strokes:    []Stroke{{Points: []StrokePoint{{X: 0, Y: 0}}}},
	})

	if err != nil {
		t.Fatalf("SubmitAnswer failed: %v", err)
	}
	if !res.IsCorrect {
		t.Error("expected IsCorrect true")
	}
	if res.Feedback != "Correct!" {
		t.Errorf("expected feedback Correct!, got %s", res.Feedback)
	}
	if res.CorrectAnswer != "あ" || res.Explanation != "hiragana a" {
		t.Fatalf("unexpected public result: %+v", res)
	}
}

func TestSubmitAnswer_Unauthorized(t *testing.T) {
	ctx := context.Background()
	active := &mockHandwritingActiveSession{
		getFn: func(ctx context.Context, id int) (*model.ActiveSessionState, error) {
			return handwritingState(456, 10, model.Question{ID: 1, Type: model.QuestionKanaHandwriting}, false), nil
		},
	}

	svc := NewHandwritingService(active, nil, nil)
	_, err := svc.SubmitAnswer(ctx, HandwritingSubmitRequest{UserID: 123, SessionID: 10, QuestionID: 1})
	if !errors.Is(err, ErrHandwritingUnauthorized) {
		t.Errorf("expected ErrHandwritingUnauthorized, got %v", err)
	}
}

func TestSubmitAnswer_InvalidQuestionType(t *testing.T) {
	ctx := context.Background()
	userID := int64(123)
	active := &mockHandwritingActiveSession{
		getFn: func(ctx context.Context, id int) (*model.ActiveSessionState, error) {
			return handwritingState(userID, 10, model.Question{ID: 1, Type: model.QuestionMultipleChoice}, false), nil
		},
	}

	svc := NewHandwritingService(active, nil, nil)
	_, err := svc.SubmitAnswer(ctx, HandwritingSubmitRequest{UserID: userID, SessionID: 10, QuestionID: 1})
	if !errors.Is(err, ErrHandwritingInvalidQuestion) {
		t.Errorf("expected ErrHandwritingInvalidQuestion, got %v", err)
	}
}

func TestSubmitAnswer_AlreadyAnswered(t *testing.T) {
	ctx := context.Background()
	userID := int64(123)
	active := &mockHandwritingActiveSession{
		getFn: func(ctx context.Context, id int) (*model.ActiveSessionState, error) {
			return handwritingState(userID, 10, model.Question{ID: 1, Type: model.QuestionKanaHandwriting}, true), nil
		},
	}

	svc := NewHandwritingService(active, nil, nil)
	_, err := svc.SubmitAnswer(ctx, HandwritingSubmitRequest{UserID: userID, SessionID: 10, QuestionID: 1})
	if !errors.Is(err, ErrHandwritingAlreadyAnswered) {
		t.Errorf("expected ErrHandwritingAlreadyAnswered, got %v", err)
	}
}

func handwritingState(userID int64, sessionID int, question model.Question, answered bool) *model.ActiveSessionState {
	state := activeStateForQuestion(sessionID, question, answered)
	state.Session.UserID = userID
	return state
}
