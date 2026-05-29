package service

import (
	"context"
	"errors"
	"fmt"
	"log"
	"time"

	"github.com/lsj/copylingo/internal/external"
	"github.com/lsj/copylingo/internal/model"
)

type graderUserRepo interface {
	UpdateStreak(ctx context.Context, userID int64) error
}

type graderActiveSession interface {
	Get(ctx context.Context, sessionID int) (*model.ActiveSessionState, error)
	RecordAnswer(ctx context.Context, sessionID, questionID int, userAnswer string, isCorrect bool) error
	Flush(ctx context.Context, sessionID int, userID int64) (*SessionResult, error)
	Delete(ctx context.Context, sessionID int) error
}

// GraderService handles answer grading and result processing.
type GraderService struct {
	userRepo      graderUserRepo
	activeSession graderActiveSession
	llm           external.LLMClient
}

func NewGraderService(
	userRepo graderUserRepo,
	activeSession graderActiveSession,
	llm external.LLMClient,
) *GraderService {
	return &GraderService{
		userRepo:      userRepo,
		activeSession: activeSession,
		llm:           llm,
	}
}

// GradeAnswer grades a single answer and updates SRS accordingly.
func (g *GraderService) GradeAnswer(ctx context.Context, sessionID, questionID int, userAnswer string) (bool, string, error) {
	question, err := g.questionFromActiveSession(ctx, sessionID, questionID)
	if err != nil {
		return false, "", err
	}
	return g.GradeAnswerWithQuestion(ctx, sessionID, questionID, question, userAnswer)
}

func (g *GraderService) GradeAnswerWithQuestion(ctx context.Context, sessionID, questionID int, question *model.Question, userAnswer string) (bool, string, error) {
	if question == nil || question.ID != questionID {
		return false, "", fmt.Errorf("grade answer question mismatch session_id=%d question_id=%d", sessionID, questionID)
	}
	var isCorrect bool
	var feedback string
	var err error

	// QuestionSubjective is the only text-answer path that uses LLM semantic grading.
	// FillBlank and MultipleChoice remain exact-match to avoid unnecessary latency and nondeterminism.
	if question.Type == model.QuestionSubjective {
		isCorrect, feedback, err = g.llm.GradeAnswer(ctx, question.Prompt, question.CorrectAnswer, userAnswer)
		if err != nil {
			return false, "", mapAIUnavailableError(err)
		}
	} else {
		isCorrect = userAnswer == question.CorrectAnswer
	}

	if err := g.recordGradingResult(ctx, sessionID, questionID, userAnswer, isCorrect); err != nil {
		return false, "", err
	}

	return isCorrect, feedback, nil
}

func (g *GraderService) GradeHandwriting(ctx context.Context, sessionID, questionID int, renderedImage []byte) (bool, string, error) {
	question, err := g.questionFromActiveSession(ctx, sessionID, questionID)
	if err != nil {
		return false, "", err
	}
	return g.GradeHandwritingWithQuestion(ctx, sessionID, questionID, question, renderedImage)
}

func (g *GraderService) GradeHandwritingWithQuestion(ctx context.Context, sessionID, questionID int, question *model.Question, renderedImage []byte) (bool, string, error) {
	startedAt := time.Now()

	if question == nil || question.ID != questionID {
		return false, "", fmt.Errorf("grade handwriting question mismatch session_id=%d question_id=%d", sessionID, questionID)
	}
	if question.Type != model.QuestionKanaHandwriting {
		return false, "", ErrHandwritingInvalidQuestion
	}

	isCorrect, feedback, err := g.llm.GradeHandwriting(ctx, question.Prompt, question.CorrectAnswer, renderedImage)
	if err != nil {
		return false, "", mapAIUnavailableError(err)
	}
	gradedAt := time.Now()

	userAnswer := "handwriting:submitted"
	if err := g.recordGradingResult(ctx, sessionID, questionID, userAnswer, isCorrect); err != nil {
		return false, "", err
	}
	log.Printf("[Handwriting] grader total=%s llm=%s record=%s session_id=%d question_id=%d is_correct=%t",
		time.Since(startedAt), gradedAt.Sub(startedAt), time.Since(gradedAt), sessionID, questionID, isCorrect)

	return isCorrect, feedback, nil
}

func mapAIUnavailableError(err error) error {
	if errors.Is(err, external.ErrAIConfigMissing) {
		return fmt.Errorf("%w: %w", ErrAIUnavailable, err)
	}
	return err
}

func (g *GraderService) recordGradingResult(ctx context.Context, sessionID, questionID int, userAnswer string, isCorrect bool) error {
	if g.activeSession == nil {
		return ErrActiveSessionDependencyMissing
	}
	return g.activeSession.RecordAnswer(ctx, sessionID, questionID, userAnswer, isCorrect)
}

// CompleteSession finalizes a session with results.
func (g *GraderService) CompleteSession(ctx context.Context, sessionID int, userID int64) (*SessionResult, error) {
	if g.activeSession == nil {
		return nil, ErrActiveSessionDependencyMissing
	}
	result, err := g.activeSession.Flush(ctx, sessionID, userID)
	if err != nil {
		return nil, err
	}

	// Update streak
	if err := g.userRepo.UpdateStreak(ctx, userID); err != nil {
		return nil, err
	}

	if err := g.activeSession.Delete(ctx, sessionID); err != nil {
		return nil, err
	}

	return result, nil
}

func (g *GraderService) questionFromActiveSession(ctx context.Context, sessionID, questionID int) (*model.Question, error) {
	if g.activeSession == nil {
		return nil, ErrActiveSessionDependencyMissing
	}
	state, err := g.activeSession.Get(ctx, sessionID)
	if err != nil {
		return nil, err
	}
	item, _, ok := state.FindItemByQuestionID(questionID)
	if !ok {
		return nil, fmt.Errorf("%w session_id=%d question_id=%d", ErrActiveSessionQuestionNotFound, sessionID, questionID)
	}
	if item.SessionQuestion.IsCorrect != nil {
		return nil, fmt.Errorf("%w session_id=%d question_id=%d", ErrActiveSessionAlreadyAnswered, sessionID, questionID)
	}
	return &item.Question, nil
}
