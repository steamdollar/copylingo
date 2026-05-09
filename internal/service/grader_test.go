package service

import (
	"context"
	"errors"
	"testing"

	"github.com/lsj/copylingo/internal/model"
)

// Mocks for GraderService
type mockGraderUserRepo struct {
	updateStreakFn func(ctx context.Context, userID int64) error
}

func (m *mockGraderUserRepo) UpdateStreak(ctx context.Context, userID int64) error {
	return m.updateStreakFn(ctx, userID)
}

type mockGraderQuestionRepo struct {
	getByIDFn          func(ctx context.Context, id int) (*model.Question, error)
	incrementServedFn  func(ctx context.Context, id int) error
	incrementCorrectFn func(ctx context.Context, id int) error
}

func (m *mockGraderQuestionRepo) GetByID(ctx context.Context, id int) (*model.Question, error) {
	return m.getByIDFn(ctx, id)
}
func (m *mockGraderQuestionRepo) IncrementServed(ctx context.Context, id int) error {
	return m.incrementServedFn(ctx, id)
}
func (m *mockGraderQuestionRepo) IncrementCorrect(ctx context.Context, id int) error {
	return m.incrementCorrectFn(ctx, id)
}

type mockGraderSessionRepo struct {
	completeFn func(ctx context.Context, id int, correctCount int) error
}

func (m *mockGraderSessionRepo) Complete(ctx context.Context, id int, correctCount int) error {
	return m.completeFn(ctx, id, correctCount)
}

type mockGraderSessionQuestionRepo struct {
	recordAnswerFn    func(ctx context.Context, sessionID, questionID int, userAnswer string, isCorrect bool) error
	getBySessionFn    func(ctx context.Context, sessionID int) ([]model.SessionQuestion, error)
	getWrongAnswersFn func(ctx context.Context, sessionID int) ([]model.SessionQuestion, error)
}

func (m *mockGraderSessionQuestionRepo) RecordAnswer(ctx context.Context, sessionID, questionID int, userAnswer string, isCorrect bool) error {
	return m.recordAnswerFn(ctx, sessionID, questionID, userAnswer, isCorrect)
}
func (m *mockGraderSessionQuestionRepo) GetBySession(ctx context.Context, sessionID int) ([]model.SessionQuestion, error) {
	return m.getBySessionFn(ctx, sessionID)
}
func (m *mockGraderSessionQuestionRepo) GetWrongAnswers(ctx context.Context, sessionID int) ([]model.SessionQuestion, error) {
	return m.getWrongAnswersFn(ctx, sessionID)
}

type mockSRS struct {
	getDueReviewsFn func(ctx context.Context, limit int) ([]model.Question, error)
	getDueCountFn   func(ctx context.Context) (int, error)
	processAnswerFn func(ctx context.Context, q *model.Question, isCorrect bool) error
}

func (m *mockSRS) GetDueReviews(ctx context.Context, limit int) ([]model.Question, error) {
	return m.getDueReviewsFn(ctx, limit)
}
func (m *mockSRS) GetDueCount(ctx context.Context) (int, error) {
	return m.getDueCountFn(ctx)
}
func (m *mockSRS) ProcessAnswer(ctx context.Context, q *model.Question, isCorrect bool) error {
	return m.processAnswerFn(ctx, q, isCorrect)
}

type mockLLM struct {
	gradeAnswerFn      func(ctx context.Context, prompt, correctAnswer, userAnswer string) (bool, string, error)
	gradeHandwritingFn func(ctx context.Context, prompt, correctAnswer string, image []byte) (bool, string, error)
	translateFn        func(ctx context.Context, text, targetLang string) (string, error)
}

func (m *mockLLM) GradeAnswer(ctx context.Context, prompt, correctAnswer, userAnswer string) (bool, string, error) {
	return m.gradeAnswerFn(ctx, prompt, correctAnswer, userAnswer)
}
func (m *mockLLM) GradeHandwriting(ctx context.Context, prompt, correctAnswer string, image []byte) (bool, string, error) {
	return m.gradeHandwritingFn(ctx, prompt, correctAnswer, image)
}
func (m *mockLLM) Translate(ctx context.Context, text, targetLang string) (string, error) {
	return m.translateFn(ctx, text, targetLang)
}

func TestGradeAnswer_Correct(t *testing.T) {
	ctx := context.Background()
	questionID := 1
	sessionID := 10
	userAnswer := "apple"

	qRepo := &mockGraderQuestionRepo{
		getByIDFn: func(ctx context.Context, id int) (*model.Question, error) {
			return &model.Question{ID: id, CorrectAnswer: "apple", Type: model.QuestionMultipleChoice}, nil
		},
		incrementServedFn:  func(ctx context.Context, id int) error { return nil },
		incrementCorrectFn: func(ctx context.Context, id int) error { return nil },
	}
	sqRepo := &mockGraderSessionQuestionRepo{
		recordAnswerFn: func(ctx context.Context, sid, qid int, ans string, correct bool) error {
			if !correct {
				t.Errorf("expected isCorrect true")
			}
			return nil
		},
	}
	srsMock := &mockSRS{
		processAnswerFn: func(ctx context.Context, q *model.Question, correct bool) error {
			if !correct {
				t.Errorf("expected srs processAnswer isCorrect true")
			}
			return nil
		},
	}

	grader := NewGraderService(nil, qRepo, nil, sqRepo, srsMock, nil)
	isCorrect, feedback, err := grader.GradeAnswer(ctx, sessionID, questionID, userAnswer)

	if err != nil {
		t.Fatalf("GradeAnswer failed: %v", err)
	}
	if !isCorrect {
		t.Error("expected isCorrect true")
	}
	if feedback != "" {
		t.Errorf("expected empty feedback for multiple choice, got %s", feedback)
	}
}

func TestGradeAnswer_Wrong(t *testing.T) {
	ctx := context.Background()
	questionID := 1
	sessionID := 10
	userAnswer := "banana"

	qRepo := &mockGraderQuestionRepo{
		getByIDFn: func(ctx context.Context, id int) (*model.Question, error) {
			return &model.Question{ID: id, CorrectAnswer: "apple", Type: model.QuestionMultipleChoice}, nil
		},
		incrementServedFn: func(ctx context.Context, id int) error { return nil },
	}
	sqRepo := &mockGraderSessionQuestionRepo{
		recordAnswerFn: func(ctx context.Context, sid, qid int, ans string, correct bool) error {
			if correct {
				t.Error("expected isCorrect false")
			}
			return nil
		},
	}
	srsMock := &mockSRS{
		processAnswerFn: func(ctx context.Context, q *model.Question, correct bool) error {
			if correct {
				t.Error("expected srs processAnswer isCorrect false")
			}
			return nil
		},
	}

	grader := NewGraderService(nil, qRepo, nil, sqRepo, srsMock, nil)
	isCorrect, _, err := grader.GradeAnswer(ctx, sessionID, questionID, userAnswer)

	if err != nil {
		t.Fatalf("GradeAnswer failed: %v", err)
	}
	if isCorrect {
		t.Error("expected isCorrect false")
	}
}

func TestGradeAnswer_Subjective_Correct(t *testing.T) {
	ctx := context.Background()
	questionID := 1
	sessionID := 10
	userAnswer := "I am a student"

	qRepo := &mockGraderQuestionRepo{
		getByIDFn: func(ctx context.Context, id int) (*model.Question, error) {
			return &model.Question{
				ID:            id,
				CorrectAnswer: "I'm a student",
				Type:          model.QuestionSubjective,
				Prompt:        "Translate: 私は学生です",
			}, nil
		},
		incrementServedFn:  func(ctx context.Context, id int) error { return nil },
		incrementCorrectFn: func(ctx context.Context, id int) error { return nil },
	}
	sqRepo := &mockGraderSessionQuestionRepo{
		recordAnswerFn: func(ctx context.Context, sid, qid int, ans string, correct bool) error { return nil },
	}
	srsMock := &mockSRS{
		processAnswerFn: func(ctx context.Context, q *model.Question, correct bool) error { return nil },
	}
	llmMock := &mockLLM{
		gradeAnswerFn: func(ctx context.Context, prompt, correct, user string) (bool, string, error) {
			return true, "Good job", nil
		},
	}

	grader := NewGraderService(nil, qRepo, nil, sqRepo, srsMock, llmMock)
	isCorrect, feedback, err := grader.GradeAnswer(ctx, sessionID, questionID, userAnswer)

	if err != nil {
		t.Fatalf("GradeAnswer failed: %v", err)
	}
	if !isCorrect {
		t.Error("expected isCorrect true from LLM")
	}
	if feedback != "Good job" {
		t.Errorf("expected feedback 'Good job', got %s", feedback)
	}
}

func TestCompleteSession_CorrectCountCalculation(t *testing.T) {
	ctx := context.Background()
	sessionID := 10
	userID := int64(12345)

	trueVal := true
	falseVal := false
	sqRepo := &mockGraderSessionQuestionRepo{
		getBySessionFn: func(ctx context.Context, sid int) ([]model.SessionQuestion, error) {
			return []model.SessionQuestion{
				{QuestionID: 1, IsCorrect: &trueVal},
				{QuestionID: 2, IsCorrect: &falseVal},
				{QuestionID: 3, IsCorrect: &trueVal},
			}, nil
		},
		getWrongAnswersFn: func(ctx context.Context, sid int) ([]model.SessionQuestion, error) {
			return []model.SessionQuestion{{QuestionID: 2, IsCorrect: &falseVal}}, nil
		},
	}

	sRepo := &mockGraderSessionRepo{
		completeFn: func(ctx context.Context, id int, count int) error {
			if count != 2 {
				t.Errorf("expected correctCount 2, got %d", count)
			}
			return nil
		},
	}

	uRepo := &mockGraderUserRepo{
		updateStreakFn: func(ctx context.Context, uid int64) error {
			if uid != userID {
				t.Errorf("expected userID %d, got %d", userID, uid)
			}
			return nil
		},
	}

	grader := NewGraderService(uRepo, nil, sRepo, sqRepo, nil, nil)
	result, err := grader.CompleteSession(ctx, sessionID, userID)

	if err != nil {
		t.Fatalf("CompleteSession failed: %v", err)
	}
	if result.CorrectCount != 2 {
		t.Errorf("expected result.CorrectCount 2, got %d", result.CorrectCount)
	}
	if len(result.WrongAnswers) != 1 {
		t.Errorf("expected 1 wrong answer, got %d", len(result.WrongAnswers))
	}
}

func TestGradeAnswer_QuestionNotFound(t *testing.T) {
	ctx := context.Background()
	qRepo := &mockGraderQuestionRepo{
		getByIDFn: func(ctx context.Context, id int) (*model.Question, error) {
			return nil, errors.New("not found")
		},
	}

	grader := NewGraderService(nil, qRepo, nil, nil, nil, nil)
	_, _, err := grader.GradeAnswer(ctx, 10, 1, "ans")

	if err == nil {
		t.Error("expected error when question not found")
	}
}

func TestGradeAnswer_RecordAnswerFails(t *testing.T) {
	ctx := context.Background()
	expectedErr := errors.New("record answer failed")
	incrementServedCalled := false
	processAnswerCalled := false

	qRepo := &mockGraderQuestionRepo{
		getByIDFn: func(ctx context.Context, id int) (*model.Question, error) {
			return &model.Question{ID: id, CorrectAnswer: "apple", Type: model.QuestionMultipleChoice}, nil
		},
		incrementServedFn: func(ctx context.Context, id int) error {
			incrementServedCalled = true
			return nil
		},
	}
	sqRepo := &mockGraderSessionQuestionRepo{
		recordAnswerFn: func(ctx context.Context, sid, qid int, ans string, correct bool) error {
			return expectedErr
		},
	}
	srsMock := &mockSRS{
		processAnswerFn: func(ctx context.Context, q *model.Question, correct bool) error {
			processAnswerCalled = true
			return nil
		},
	}

	grader := NewGraderService(nil, qRepo, nil, sqRepo, srsMock, nil)
	_, _, err := grader.GradeAnswer(ctx, 10, 1, "apple")

	if !errors.Is(err, expectedErr) {
		t.Fatalf("expected RecordAnswer error %v, got %v", expectedErr, err)
	}
	if incrementServedCalled {
		t.Error("IncrementServed should not be called after RecordAnswer failure")
	}
	if processAnswerCalled {
		t.Error("ProcessAnswer should not be called after RecordAnswer failure")
	}
}

func TestGradeAnswer_IncrementServedFails(t *testing.T) {
	ctx := context.Background()
	expectedErr := errors.New("increment served failed")
	incrementCorrectCalled := false
	processAnswerCalled := false

	qRepo := &mockGraderQuestionRepo{
		getByIDFn: func(ctx context.Context, id int) (*model.Question, error) {
			return &model.Question{ID: id, CorrectAnswer: "apple", Type: model.QuestionMultipleChoice}, nil
		},
		incrementServedFn: func(ctx context.Context, id int) error {
			return expectedErr
		},
		incrementCorrectFn: func(ctx context.Context, id int) error {
			incrementCorrectCalled = true
			return nil
		},
	}
	sqRepo := &mockGraderSessionQuestionRepo{
		recordAnswerFn: func(ctx context.Context, sid, qid int, ans string, correct bool) error {
			return nil
		},
	}
	srsMock := &mockSRS{
		processAnswerFn: func(ctx context.Context, q *model.Question, correct bool) error {
			processAnswerCalled = true
			return nil
		},
	}

	grader := NewGraderService(nil, qRepo, nil, sqRepo, srsMock, nil)
	_, _, err := grader.GradeAnswer(ctx, 10, 1, "apple")

	if !errors.Is(err, expectedErr) {
		t.Fatalf("expected IncrementServed error %v, got %v", expectedErr, err)
	}
	if incrementCorrectCalled {
		t.Error("IncrementCorrect should not be called after IncrementServed failure")
	}
	if processAnswerCalled {
		t.Error("ProcessAnswer should not be called after IncrementServed failure")
	}
}

func TestGradeAnswer_ProcessAnswerFails(t *testing.T) {
	ctx := context.Background()
	expectedErr := errors.New("process answer failed")

	qRepo := &mockGraderQuestionRepo{
		getByIDFn: func(ctx context.Context, id int) (*model.Question, error) {
			return &model.Question{ID: id, CorrectAnswer: "apple", Type: model.QuestionMultipleChoice}, nil
		},
		incrementServedFn:  func(ctx context.Context, id int) error { return nil },
		incrementCorrectFn: func(ctx context.Context, id int) error { return nil },
	}
	sqRepo := &mockGraderSessionQuestionRepo{
		recordAnswerFn: func(ctx context.Context, sid, qid int, ans string, correct bool) error {
			return nil
		},
	}
	srsMock := &mockSRS{
		processAnswerFn: func(ctx context.Context, q *model.Question, correct bool) error {
			return expectedErr
		},
	}

	grader := NewGraderService(nil, qRepo, nil, sqRepo, srsMock, nil)
	_, _, err := grader.GradeAnswer(ctx, 10, 1, "apple")

	if !errors.Is(err, expectedErr) {
		t.Fatalf("expected ProcessAnswer error %v, got %v", expectedErr, err)
	}
}
