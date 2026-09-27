package service

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"time"

	"github.com/lsj/copylingo/internal/external"
	"github.com/lsj/copylingo/internal/model"
	"github.com/lsj/copylingo/internal/observability"
)

type graderLLM interface {
	GradeAnswer(ctx context.Context, questionPrompt, correctAnswer, userAnswer string) (external.GradeResult, error)
	GradeHandwriting(
		ctx context.Context,
		questionPrompt, correctAnswer string,
		pngImage []byte,
	) (external.GradeResult, error)
}

type graderUserRepo interface {
	UpdateStreak(ctx context.Context, userID int64) error
}

type graderQuizActiveSession interface {
	Get(ctx context.Context, sessionID int) (*model.QuizActiveSessionState, error)
	RecordAnswer(ctx context.Context, sessionID, questionID int, userAnswer string, isCorrect bool) error
	Flush(ctx context.Context, sessionID int, userID int64) (*QuizSessionResult, error)
	Delete(ctx context.Context, sessionID int) error
}

// GraderService handles answer grading and result processing.
type GraderService struct {
	userRepo          graderUserRepo
	quizActiveSession graderQuizActiveSession
	llm               graderLLM
}

func NewGraderService(
	userRepo graderUserRepo,
	quizActiveSession graderQuizActiveSession,
	llm graderLLM,
) *GraderService {
	return &GraderService{
		userRepo:          userRepo,
		quizActiveSession: quizActiveSession,
		llm:               llm,
	}
}

// GradeAnswer grades a single answer and updates SRS accordingly.
func (g *GraderService) GradeAnswer(
	ctx context.Context,
	sessionID, questionID int,
	userAnswer string,
) (bool, string, error) {
	question, err := g.questionFromQuizActiveSession(ctx, sessionID, questionID)
	if err != nil {
		return false, "", err
	}
	return g.GradeAnswerWithQuestion(ctx, sessionID, questionID, question, userAnswer)
}

func (g *GraderService) GradeAnswerWithQuestion(
	ctx context.Context,
	sessionID, questionID int,
	question *model.Question,
	userAnswer string,
) (bool, string, error) {
	if question == nil || question.ID != questionID {
		return false, "", fmt.Errorf(
			"grade answer question mismatch session_id=%d question_id=%d",
			sessionID,
			questionID,
		)
	}
	var isCorrect bool
	var feedback string
	var err error

	// QuestionSubjective is the only text-answer path that uses LLM semantic grading.
	// FillBlank and MultipleChoice remain exact-match to avoid unnecessary latency and nondeterminism.
	if question.Type == model.QuestionSubjective {
		var result external.GradeResult
		result, err = g.llm.GradeAnswer(ctx, question.Prompt, question.CorrectAnswer, userAnswer)
		if err != nil {
			return false, "", mapAIUnavailableError(err)
		}
		isCorrect = result.IsCorrect
		feedback = result.Feedback
	} else {
		isCorrect = userAnswer == question.CorrectAnswer
	}

	if err := g.recordGradingResult(ctx, sessionID, questionID, userAnswer, isCorrect); err != nil {
		return false, "", err
	}

	return isCorrect, feedback, nil
}

func (g *GraderService) GradeHandwriting(
	ctx context.Context,
	sessionID, questionID int,
	renderedImage []byte,
) (bool, string, error) {
	question, err := g.questionFromQuizActiveSession(ctx, sessionID, questionID)
	if err != nil {
		return false, "", err
	}
	return g.GradeHandwritingWithQuestion(ctx, sessionID, questionID, question, renderedImage)
}

func (g *GraderService) GradeHandwritingWithQuestion(
	ctx context.Context,
	sessionID, questionID int,
	question *model.Question,
	renderedImage []byte,
) (bool, string, error) {
	startedAt := time.Now()
	ctx = observability.WithAttrs(ctx,
		slog.String("source", "service.grader"),
		slog.Int("session_id", sessionID),
		slog.Int("question_id", questionID),
	)

	if question == nil || question.ID != questionID {
		return false, "", fmt.Errorf(
			"grade handwriting question mismatch session_id=%d question_id=%d",
			sessionID,
			questionID,
		)
	}
	if question.Type != model.QuestionKanaHandwriting {
		return false, "", ErrHandwritingInvalidQuestion
	}

	result, err := g.llm.GradeHandwriting(ctx, question.Prompt, question.CorrectAnswer, renderedImage)
	if err != nil {
		return false, "", mapAIUnavailableError(err)
	}
	isCorrect := result.IsCorrect
	feedback := result.Feedback
	gradedAt := time.Now()

	userAnswer := "handwriting:submitted"
	if err := g.recordGradingResult(ctx, sessionID, questionID, userAnswer, isCorrect); err != nil {
		return false, "", err
	}
	slog.InfoContext(ctx, "Handwriting grader completed",
		"event", "handwriting.grader.completed",
		"duration_ms", time.Since(startedAt).Milliseconds(),
		"llm_duration_ms", gradedAt.Sub(startedAt).Milliseconds(),
		"record_duration_ms", time.Since(gradedAt).Milliseconds(),
		"is_correct", isCorrect,
	)

	return isCorrect, feedback, nil
}

func mapAIUnavailableError(err error) error {
	if errors.Is(err, external.ErrAIConfigMissing) {
		return fmt.Errorf("%w: %w", ErrAIUnavailable, err)
	}
	return err
}

func (g *GraderService) recordGradingResult(
	ctx context.Context,
	sessionID, questionID int,
	userAnswer string,
	isCorrect bool,
) error {
	if g.quizActiveSession == nil {
		return ErrQuizActiveSessionDependencyMissing
	}
	return g.quizActiveSession.RecordAnswer(ctx, sessionID, questionID, userAnswer, isCorrect)
}

// CompleteSession finalizes a session with results.
func (g *GraderService) CompleteSession(ctx context.Context, sessionID int, userID int64) (*QuizSessionResult, error) {
	if g.quizActiveSession == nil {
		return nil, ErrQuizActiveSessionDependencyMissing
	}
	result, err := g.quizActiveSession.Flush(ctx, sessionID, userID)
	if err != nil {
		return nil, err
	}

	// Update streak
	if err := g.userRepo.UpdateStreak(ctx, userID); err != nil {
		return nil, err
	}

	if err := g.quizActiveSession.Delete(ctx, sessionID); err != nil {
		return nil, err
	}

	return result, nil
}

func (g *GraderService) questionFromQuizActiveSession(
	ctx context.Context,
	sessionID, questionID int,
) (*model.Question, error) {
	if g.quizActiveSession == nil {
		return nil, ErrQuizActiveSessionDependencyMissing
	}
	state, err := g.quizActiveSession.Get(ctx, sessionID)
	if err != nil {
		return nil, err
	}
	item, _, ok := state.CurrentItemByQuestionID(questionID)
	if !ok {
		return nil, fmt.Errorf(
			"%w session_id=%d question_id=%d",
			ErrQuizActiveSessionQuestionNotFound,
			sessionID,
			questionID,
		)
	}
	if item.SessionQuestion.IsCorrect != nil {
		return nil, fmt.Errorf(
			"%w session_id=%d question_id=%d",
			ErrQuizActiveSessionAlreadyAnswered,
			sessionID,
			questionID,
		)
	}
	return &item.Question, nil
}
