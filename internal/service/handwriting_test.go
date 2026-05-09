package service

import (
	"context"
	"testing"

	"github.com/lsj/copylingo/internal/model"
)

type mockHandwritingSessionRepo struct {
	getByIDFn func(ctx context.Context, id int) (*model.Session, error)
}

func (m *mockHandwritingSessionRepo) GetByID(ctx context.Context, id int) (*model.Session, error) {
	return m.getByIDFn(ctx, id)
}

type mockHandwritingQuestionRepo struct {
	getByIDFn func(ctx context.Context, id int) (*model.Question, error)
}

func (m *mockHandwritingQuestionRepo) GetByID(ctx context.Context, id int) (*model.Question, error) {
	return m.getByIDFn(ctx, id)
}

type mockHandwritingSessionQuestionRepo struct {
	getBySessionFn func(ctx context.Context, sessionID int) ([]model.SessionQuestion, error)
}

func (m *mockHandwritingSessionQuestionRepo) GetBySession(ctx context.Context, sid int) ([]model.SessionQuestion, error) {
	return m.getBySessionFn(ctx, sid)
}

type mockGraderClient struct {
	gradeHandwritingFn func(ctx context.Context, sid, qid int, img []byte) (bool, string, error)
}

func (m *mockGraderClient) GradeHandwriting(ctx context.Context, sid, qid int, img []byte) (bool, string, error) {
	return m.gradeHandwritingFn(ctx, sid, qid, img)
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

	sRepo := &mockHandwritingSessionRepo{
		getByIDFn: func(ctx context.Context, id int) (*model.Session, error) {
			return &model.Session{ID: id, UserID: userID}, nil
		},
	}
	qRepo := &mockHandwritingQuestionRepo{
		getByIDFn: func(ctx context.Context, id int) (*model.Question, error) {
			return &model.Question{ID: id, Type: model.QuestionKanaHandwriting, CorrectAnswer: "あ"}, nil
		},
	}
	sqRepo := &mockHandwritingSessionQuestionRepo{
		getBySessionFn: func(ctx context.Context, sid int) ([]model.SessionQuestion, error) {
			return []model.SessionQuestion{{QuestionID: questionID, SessionID: sid}}, nil
		},
	}
	grader := &mockGraderClient{
		gradeHandwritingFn: func(ctx context.Context, sid, qid int, img []byte) (bool, string, error) {
			return true, "Correct!", nil
		},
	}
	renderer := &mockRenderer{
		renderPNGFn: func(strokes []Stroke) ([]byte, error) {
			return []byte("fake-image"), nil
		},
	}

	svc := NewHandwritingService(sRepo, qRepo, sqRepo, grader, renderer)
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
}

func TestSubmitAnswer_Unauthorized(t *testing.T) {
	ctx := context.Background()
	sRepo := &mockHandwritingSessionRepo{
		getByIDFn: func(ctx context.Context, id int) (*model.Session, error) {
			return &model.Session{ID: id, UserID: 456}, nil // Different user
		},
	}

	svc := NewHandwritingService(sRepo, nil, nil, nil, nil)
	_, err := svc.SubmitAnswer(ctx, HandwritingSubmitRequest{UserID: 123, SessionID: 10})

	if err != ErrHandwritingUnauthorized {
		t.Errorf("expected ErrHandwritingUnauthorized, got %v", err)
	}
}

func TestSubmitAnswer_InvalidQuestionType(t *testing.T) {
	ctx := context.Background()
	userID := int64(123)
	sRepo := &mockHandwritingSessionRepo{
		getByIDFn: func(ctx context.Context, id int) (*model.Session, error) {
			return &model.Session{ID: id, UserID: userID}, nil
		},
	}
	qRepo := &mockHandwritingQuestionRepo{
		getByIDFn: func(ctx context.Context, id int) (*model.Question, error) {
			return &model.Question{ID: id, Type: model.QuestionMultipleChoice}, nil // Wrong type
		},
	}

	svc := NewHandwritingService(sRepo, qRepo, nil, nil, nil)
	_, err := svc.SubmitAnswer(ctx, HandwritingSubmitRequest{UserID: userID, SessionID: 10, QuestionID: 1})

	if err != ErrHandwritingInvalidQuestion {
		t.Errorf("expected ErrHandwritingInvalidQuestion, got %v", err)
	}
}
