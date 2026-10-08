package service

import (
	"context"
	"errors"
	"testing"

	"github.com/lsj/copylingo/internal/external"
	"github.com/lsj/copylingo/internal/model"
)

type mockGraderQuizActiveSession struct {
	recordAnswerFn func(
		ctx context.Context,
		sessionID,
		questionID int,
		userAnswer string,
		isCorrect bool,
	) error
}

func (m *mockGraderQuizActiveSession) RecordAnswer(
	ctx context.Context,
	sessionID,
	questionID int,
	userAnswer string,
	isCorrect bool,
) error {
	return m.recordAnswerFn(
		ctx,
		sessionID,
		questionID,
		userAnswer,
		isCorrect,
	)
}

type mockSRS struct {
	getDueReviewsForCategoriesFn func(
		ctx context.Context,
		userID int64,
		limit,
		kanjiRecallLimit int,
		categories ...model.QuestionCategory,
	) ([]model.Question, error)
	getDueReviewsFn func(
		ctx context.Context,
		userID int64,
		limit,
		kanjiRecallLimit int,
	) ([]model.Question, error)
	getDueCountFn func(ctx context.Context) (int, error)
	gotLanguage   string
	gotLevel      string
}

func (m *mockSRS) GetDueReviews(
	ctx context.Context,
	userID int64,
	language,
	level string,
	limit,
	kanjiRecallLimit int,
	categories ...model.QuestionCategory,
) ([]model.Question, error) {
	m.gotLanguage = language
	m.gotLevel = level
	if len(categories) > 0 {
		if m.getDueReviewsForCategoriesFn != nil {
			return m.getDueReviewsForCategoriesFn(
				ctx,
				userID,
				limit,
				kanjiRecallLimit,
				categories...,
			)
		}
		return nil, nil
	}
	return m.getDueReviewsFn(
		ctx,
		userID,
		limit,
		kanjiRecallLimit,
	)
}
func (m *mockSRS) GetDueCount(
	ctx context.Context,
	userID int64,
	language,
	level string,
) (int, error) {
	return m.getDueCountFn(ctx)
}

type mockLLM struct {
	gradeAnswerFn func(
		ctx context.Context,
		prompt,
		correctAnswer,
		userAnswer string,
	) (external.GradeResult, error)
	gradeHandwritingFn func(
		ctx context.Context,
		prompt,
		correctAnswer string,
		image []byte,
	) (external.GradeResult, error)
}

func (m *mockLLM) GradeAnswer(
	ctx context.Context,
	prompt,
	correctAnswer,
	userAnswer string,
) (external.GradeResult, error) {
	return m.gradeAnswerFn(
		ctx,
		prompt,
		correctAnswer,
		userAnswer,
	)
}

func (m *mockLLM) GradeHandwriting(
	ctx context.Context,
	prompt,
	correctAnswer string,
	image []byte,
) (external.GradeResult, error) {
	return m.gradeHandwritingFn(
		ctx,
		prompt,
		correctAnswer,
		image,
	)
}
func (m *mockLLM) AnswerLearningQuestion(
	ctx context.Context,
	question string,
) (string, error) {
	return "answer", nil
}

func TestGradeAnswer_Correct(t *testing.T) {
	ctx := context.Background()
	sessionID := 10
	questionID := 1

	question := &model.Question{
		ID:            questionID,
		CorrectAnswer: "apple",
		Type:          model.QuestionMultipleChoice,
	}

	active := &mockGraderQuizActiveSession{
		recordAnswerFn: func(
			ctx context.Context,
			sid,
			qid int,
			ans string,
			correct bool,
		) error {
			if sid != sessionID || qid != questionID || ans != "apple" || !correct {
				t.Fatalf(
					"unexpected record args sid=%d qid=%d ans=%q correct=%t",
					sid,
					qid,
					ans,
					correct,
				)
			}
			return nil
		},
	}

	grader := newGraderService(
		active,
		nil,
	)
	isCorrect, feedback, err := grader.GradeAnswerWithQuestion(
		ctx,
		sessionID,
		questionID,
		question,
		"apple",
	)
	if err != nil {
		t.Fatalf(
			"GradeAnswer failed: %v",
			err,
		)
	}
	if !isCorrect {
		t.Error("expected isCorrect true")
	}
	if feedback != "" {
		t.Errorf(
			"expected empty feedback, got %q",
			feedback,
		)
	}
}

func TestGradeAnswer_Wrong(t *testing.T) {
	ctx := context.Background()
	sessionID := 10
	questionID := 1

	question := &model.Question{
		ID:            questionID,
		CorrectAnswer: "apple",
		Type:          model.QuestionMultipleChoice,
	}

	active := &mockGraderQuizActiveSession{
		recordAnswerFn: func(
			ctx context.Context,
			sid,
			qid int,
			ans string,
			correct bool,
		) error {
			if correct {
				t.Fatal("expected wrong answer to be recorded")
			}
			return nil
		},
	}

	grader := newGraderService(
		active,
		nil,
	)
	isCorrect, _, err := grader.GradeAnswerWithQuestion(
		ctx,
		sessionID,
		questionID,
		question,
		"banana",
	)
	if err != nil {
		t.Fatalf(
			"GradeAnswer failed: %v",
			err,
		)
	}
	if isCorrect {
		t.Error("expected isCorrect false")
	}
}

func TestGradeAnswer_Subjective_Correct(t *testing.T) {
	ctx := context.Background()
	sessionID := 10
	questionID := 1

	question := &model.Question{
		ID:            questionID,
		CorrectAnswer: "I'm a student",
		Type:          model.QuestionSubjective,
		Prompt:        "Translate: 私は学生です",
	}

	active := &mockGraderQuizActiveSession{
		recordAnswerFn: func(
			ctx context.Context,
			sid,
			qid int,
			ans string,
			correct bool,
		) error {
			if !correct {
				t.Fatal("expected subjective answer to be recorded as correct")
			}
			return nil
		},
	}
	llm := &mockLLM{
		gradeAnswerFn: func(
			ctx context.Context,
			prompt,
			correct,
			user string,
		) (external.GradeResult, error) {
			return external.GradeResult{IsCorrect: true, Feedback: "Good job"}, nil
		},
	}

	grader := newGraderService(
		active,
		llm,
	)
	isCorrect, feedback, err := grader.GradeAnswerWithQuestion(
		ctx,
		sessionID,
		questionID,
		question,
		"I am a student",
	)
	if err != nil {
		t.Fatalf(
			"GradeAnswer failed: %v",
			err,
		)
	}
	if !isCorrect {
		t.Error("expected isCorrect true from LLM")
	}
	if feedback != "Good job" {
		t.Errorf(
			"expected feedback 'Good job', got %q",
			feedback,
		)
	}
}

func TestGradeAnswer_Subjective_AIUnavailable(t *testing.T) {
	ctx := context.Background()
	sessionID := 10
	questionID := 1

	question := &model.Question{
		ID:            questionID,
		CorrectAnswer: "I'm a student",
		Type:          model.QuestionSubjective,
		Prompt:        "Translate: 私は学生です",
	}

	active := &mockGraderQuizActiveSession{}
	llm := &mockLLM{
		gradeAnswerFn: func(
			ctx context.Context,
			prompt,
			correct,
			user string,
		) (external.GradeResult, error) {
			return external.GradeResult{}, external.ErrAIConfigMissing
		},
	}

	grader := newGraderService(
		active,
		llm,
	)
	_, _, err := grader.GradeAnswerWithQuestion(
		ctx,
		sessionID,
		questionID,
		question,
		"I am a student",
	)
	if !errors.Is(
		err,
		ErrAIUnavailable,
	) {
		t.Fatalf(
			"expected ErrAIUnavailable, got %v",
			err,
		)
	}
	if !errors.Is(
		err,
		external.ErrAIConfigMissing,
	) {
		t.Fatalf(
			"expected wrapped external.ErrAIConfigMissing, got %v",
			err,
		)
	}
}

func TestGradeHandwriting_AIUnavailable(t *testing.T) {
	ctx := context.Background()
	sessionID := 10
	questionID := 1

	question := &model.Question{
		ID:            questionID,
		CorrectAnswer: "あ",
		Type:          model.QuestionKanaHandwriting,
		Prompt:        "Write あ",
	}

	active := &mockGraderQuizActiveSession{}
	llm := &mockLLM{
		gradeHandwritingFn: func(
			ctx context.Context,
			prompt,
			correctAnswer string,
			image []byte,
		) (external.GradeResult, error) {
			return external.GradeResult{}, external.ErrAIConfigMissing
		},
	}

	grader := newGraderService(
		active,
		llm,
	)
	_, _, err := grader.GradeHandwritingWithQuestion(
		ctx,
		sessionID,
		questionID,
		question,
		[]byte("png"),
	)
	if !errors.Is(
		err,
		ErrAIUnavailable,
	) {
		t.Fatalf(
			"expected ErrAIUnavailable, got %v",
			err,
		)
	}
	if !errors.Is(
		err,
		external.ErrAIConfigMissing,
	) {
		t.Fatalf(
			"expected wrapped external.ErrAIConfigMissing, got %v",
			err,
		)
	}
}

func TestGradeAnswer_RecordAnswerFails(t *testing.T) {
	ctx := context.Background()
	sessionID := 10
	questionID := 1
	expectedErr := errors.New("record answer failed")

	question := &model.Question{
		ID:            questionID,
		CorrectAnswer: "apple",
		Type:          model.QuestionMultipleChoice,
	}

	active := &mockGraderQuizActiveSession{
		recordAnswerFn: func(
			ctx context.Context,
			sid,
			qid int,
			ans string,
			correct bool,
		) error {
			return expectedErr
		},
	}

	grader := newGraderService(
		active,
		nil,
	)
	_, _, err := grader.GradeAnswerWithQuestion(
		ctx,
		sessionID,
		questionID,
		question,
		"apple",
	)
	if !errors.Is(
		err,
		expectedErr,
	) {
		t.Fatalf(
			"expected RecordAnswer error %v, got %v",
			expectedErr,
			err,
		)
	}
}

func activeStateForQuestion(
	sessionID int,
	question model.Question,
	answered bool,
) *model.QuizActiveSessionState {
	var userAnswer *string
	var isCorrect *bool
	if answered {
		answer := "answered"
		correct := true
		userAnswer = &answer
		isCorrect = &correct
	}
	return &model.QuizActiveSessionState{
		Version: model.QuizActiveSessionStateVersion,
		Session: model.Session{
			ID:     sessionID,
			UserID: 1,
		},
		Items: []model.QuizActiveSessionQuestion{
			{
				SessionQuestion: model.SessionQuestion{
					ID:         100,
					SessionID:  sessionID,
					QuestionID: question.ID,
					UserAnswer: userAnswer,
					IsCorrect:  isCorrect,
				},
				Question: question,
				Progress: model.NewUserQuestionProgress(
					1,
					question.ID,
				),
			},
		},
	}
}
