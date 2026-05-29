package service

import (
	"context"
	"errors"
	"testing"

	"github.com/lsj/copylingo/internal/external"
	"github.com/lsj/copylingo/internal/model"
)

type mockGraderUserRepo struct {
	updateStreakFn func(ctx context.Context, userID int64) error
}

func (m *mockGraderUserRepo) UpdateStreak(ctx context.Context, userID int64) error {
	return m.updateStreakFn(ctx, userID)
}

type mockGraderActiveSession struct {
	getFn          func(ctx context.Context, sessionID int) (*model.ActiveSessionState, error)
	recordAnswerFn func(ctx context.Context, sessionID, questionID int, userAnswer string, isCorrect bool) error
	flushFn        func(ctx context.Context, sessionID int, userID int64) (*SessionResult, error)
	deleteFn       func(ctx context.Context, sessionID int) error
}

func (m *mockGraderActiveSession) Get(ctx context.Context, sessionID int) (*model.ActiveSessionState, error) {
	return m.getFn(ctx, sessionID)
}

func (m *mockGraderActiveSession) RecordAnswer(ctx context.Context, sessionID, questionID int, userAnswer string, isCorrect bool) error {
	return m.recordAnswerFn(ctx, sessionID, questionID, userAnswer, isCorrect)
}

func (m *mockGraderActiveSession) Flush(ctx context.Context, sessionID int, userID int64) (*SessionResult, error) {
	return m.flushFn(ctx, sessionID, userID)
}

func (m *mockGraderActiveSession) Delete(ctx context.Context, sessionID int) error {
	return m.deleteFn(ctx, sessionID)
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
func (m *mockSRS) ProcessAnswer(ctx context.Context, q *model.Question, correct bool) error {
	return m.processAnswerFn(ctx, q, correct)
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
	sessionID := 10
	questionID := 1

	active := &mockGraderActiveSession{
		getFn: func(ctx context.Context, sid int) (*model.ActiveSessionState, error) {
			return activeStateForQuestion(sessionID, model.Question{
				ID:            questionID,
				CorrectAnswer: "apple",
				Type:          model.QuestionMultipleChoice,
			}, false), nil
		},
		recordAnswerFn: func(ctx context.Context, sid, qid int, ans string, correct bool) error {
			if sid != sessionID || qid != questionID || ans != "apple" || !correct {
				t.Fatalf("unexpected record args sid=%d qid=%d ans=%q correct=%t", sid, qid, ans, correct)
			}
			return nil
		},
	}

	grader := NewGraderService(nil, active, nil)
	isCorrect, feedback, err := grader.GradeAnswer(ctx, sessionID, questionID, "apple")
	if err != nil {
		t.Fatalf("GradeAnswer failed: %v", err)
	}
	if !isCorrect {
		t.Error("expected isCorrect true")
	}
	if feedback != "" {
		t.Errorf("expected empty feedback, got %q", feedback)
	}
}

func TestGradeAnswer_Wrong(t *testing.T) {
	ctx := context.Background()
	sessionID := 10
	questionID := 1

	active := &mockGraderActiveSession{
		getFn: func(ctx context.Context, sid int) (*model.ActiveSessionState, error) {
			return activeStateForQuestion(sessionID, model.Question{
				ID:            questionID,
				CorrectAnswer: "apple",
				Type:          model.QuestionMultipleChoice,
			}, false), nil
		},
		recordAnswerFn: func(ctx context.Context, sid, qid int, ans string, correct bool) error {
			if correct {
				t.Fatal("expected wrong answer to be recorded")
			}
			return nil
		},
	}

	grader := NewGraderService(nil, active, nil)
	isCorrect, _, err := grader.GradeAnswer(ctx, sessionID, questionID, "banana")
	if err != nil {
		t.Fatalf("GradeAnswer failed: %v", err)
	}
	if isCorrect {
		t.Error("expected isCorrect false")
	}
}

func TestGradeAnswer_Subjective_Correct(t *testing.T) {
	ctx := context.Background()
	sessionID := 10
	questionID := 1

	active := &mockGraderActiveSession{
		getFn: func(ctx context.Context, sid int) (*model.ActiveSessionState, error) {
			return activeStateForQuestion(sessionID, model.Question{
				ID:            questionID,
				CorrectAnswer: "I'm a student",
				Type:          model.QuestionSubjective,
				Prompt:        "Translate: 私は学生です",
			}, false), nil
		},
		recordAnswerFn: func(ctx context.Context, sid, qid int, ans string, correct bool) error {
			if !correct {
				t.Fatal("expected subjective answer to be recorded as correct")
			}
			return nil
		},
	}
	llm := &mockLLM{
		gradeAnswerFn: func(ctx context.Context, prompt, correct, user string) (bool, string, error) {
			return true, "Good job", nil
		},
	}

	grader := NewGraderService(nil, active, llm)
	isCorrect, feedback, err := grader.GradeAnswer(ctx, sessionID, questionID, "I am a student")
	if err != nil {
		t.Fatalf("GradeAnswer failed: %v", err)
	}
	if !isCorrect {
		t.Error("expected isCorrect true from LLM")
	}
	if feedback != "Good job" {
		t.Errorf("expected feedback 'Good job', got %q", feedback)
	}
}

func TestGradeAnswer_Subjective_AIUnavailable(t *testing.T) {
	ctx := context.Background()
	sessionID := 10
	questionID := 1

	active := &mockGraderActiveSession{
		getFn: func(ctx context.Context, sid int) (*model.ActiveSessionState, error) {
			return activeStateForQuestion(sessionID, model.Question{
				ID:            questionID,
				CorrectAnswer: "I'm a student",
				Type:          model.QuestionSubjective,
				Prompt:        "Translate: 私は学生です",
			}, false), nil
		},
	}
	llm := &mockLLM{
		gradeAnswerFn: func(ctx context.Context, prompt, correct, user string) (bool, string, error) {
			return false, "", external.ErrAIConfigMissing
		},
	}

	grader := NewGraderService(nil, active, llm)
	_, _, err := grader.GradeAnswer(ctx, sessionID, questionID, "I am a student")
	if !errors.Is(err, ErrAIUnavailable) {
		t.Fatalf("expected ErrAIUnavailable, got %v", err)
	}
	if !errors.Is(err, external.ErrAIConfigMissing) {
		t.Fatalf("expected wrapped external.ErrAIConfigMissing, got %v", err)
	}
}

func TestGradeHandwriting_AIUnavailable(t *testing.T) {
	ctx := context.Background()
	sessionID := 10
	questionID := 1

	active := &mockGraderActiveSession{
		getFn: func(ctx context.Context, sid int) (*model.ActiveSessionState, error) {
			return activeStateForQuestion(sessionID, model.Question{
				ID:            questionID,
				CorrectAnswer: "あ",
				Type:          model.QuestionKanaHandwriting,
				Prompt:        "Write あ",
			}, false), nil
		},
	}
	llm := &mockLLM{
		gradeHandwritingFn: func(ctx context.Context, prompt, correctAnswer string, image []byte) (bool, string, error) {
			return false, "", external.ErrAIConfigMissing
		},
	}

	grader := NewGraderService(nil, active, llm)
	_, _, err := grader.GradeHandwriting(ctx, sessionID, questionID, []byte("png"))
	if !errors.Is(err, ErrAIUnavailable) {
		t.Fatalf("expected ErrAIUnavailable, got %v", err)
	}
	if !errors.Is(err, external.ErrAIConfigMissing) {
		t.Fatalf("expected wrapped external.ErrAIConfigMissing, got %v", err)
	}
}

func TestGradeAnswer_AlreadyAnswered(t *testing.T) {
	ctx := context.Background()
	sessionID := 10
	questionID := 1

	active := &mockGraderActiveSession{
		getFn: func(ctx context.Context, sid int) (*model.ActiveSessionState, error) {
			return activeStateForQuestion(sessionID, model.Question{
				ID:            questionID,
				CorrectAnswer: "apple",
				Type:          model.QuestionMultipleChoice,
			}, true), nil
		},
	}

	grader := NewGraderService(nil, active, nil)
	_, _, err := grader.GradeAnswer(ctx, sessionID, questionID, "apple")
	if !errors.Is(err, ErrActiveSessionAlreadyAnswered) {
		t.Fatalf("expected ErrActiveSessionAlreadyAnswered, got %v", err)
	}
}

func TestGradeAnswer_RecordAnswerFails(t *testing.T) {
	ctx := context.Background()
	sessionID := 10
	questionID := 1
	expectedErr := errors.New("record answer failed")

	active := &mockGraderActiveSession{
		getFn: func(ctx context.Context, sid int) (*model.ActiveSessionState, error) {
			return activeStateForQuestion(sessionID, model.Question{
				ID:            questionID,
				CorrectAnswer: "apple",
				Type:          model.QuestionMultipleChoice,
			}, false), nil
		},
		recordAnswerFn: func(ctx context.Context, sid, qid int, ans string, correct bool) error {
			return expectedErr
		},
	}

	grader := NewGraderService(nil, active, nil)
	_, _, err := grader.GradeAnswer(ctx, sessionID, questionID, "apple")
	if !errors.Is(err, expectedErr) {
		t.Fatalf("expected RecordAnswer error %v, got %v", expectedErr, err)
	}
}

func TestCompleteSession_FlushStreakAndDelete(t *testing.T) {
	ctx := context.Background()
	sessionID := 10
	userID := int64(12345)
	deleteCalled := false

	active := &mockGraderActiveSession{
		flushFn: func(ctx context.Context, sid int, uid int64) (*SessionResult, error) {
			if sid != sessionID || uid != userID {
				t.Fatalf("unexpected flush args sid=%d uid=%d", sid, uid)
			}
			return &SessionResult{TotalQuestions: 3, CorrectCount: 2}, nil
		},
		deleteFn: func(ctx context.Context, sid int) error {
			if sid != sessionID {
				t.Fatalf("unexpected delete session id %d", sid)
			}
			deleteCalled = true
			return nil
		},
	}
	userRepo := &mockGraderUserRepo{
		updateStreakFn: func(ctx context.Context, uid int64) error {
			if uid != userID {
				t.Fatalf("expected userID %d, got %d", userID, uid)
			}
			return nil
		},
	}

	grader := NewGraderService(userRepo, active, nil)
	result, err := grader.CompleteSession(ctx, sessionID, userID)
	if err != nil {
		t.Fatalf("CompleteSession failed: %v", err)
	}
	if result.CorrectCount != 2 || result.TotalQuestions != 3 {
		t.Fatalf("unexpected result: %+v", result)
	}
	if !deleteCalled {
		t.Fatal("expected active session state to be deleted")
	}
}

func activeStateForQuestion(sessionID int, question model.Question, answered bool) *model.ActiveSessionState {
	var userAnswer *string
	var isCorrect *bool
	if answered {
		answer := "answered"
		correct := true
		userAnswer = &answer
		isCorrect = &correct
	}
	return &model.ActiveSessionState{
		Version: model.ActiveSessionStateVersion,
		Session: model.Session{
			ID:     sessionID,
			UserID: 1,
		},
		Items: []model.ActiveSessionQuestion{
			{
				SessionQuestion: model.SessionQuestion{
					ID:         100,
					SessionID:  sessionID,
					QuestionID: question.ID,
					UserAnswer: userAnswer,
					IsCorrect:  isCorrect,
				},
				Question: question,
			},
		},
	}
}
