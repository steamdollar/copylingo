package service

import (
	"context"

	"github.com/lsj/copylingo/internal/external"
	"github.com/lsj/copylingo/internal/model"
	"github.com/lsj/copylingo/internal/repository"
)

// GraderService handles answer grading and result processing.
type GraderService struct {
	userRepo            *repository.UserRepository
	questionRepo        *repository.QuestionRepository
	sessionRepo         *repository.SessionRepository
	sessionQuestionRepo *repository.SessionQuestionRepository
	srs                 *SRSService
	llm                 external.LLMClient
}

func NewGraderService(
	userRepo *repository.UserRepository,
	questionRepo *repository.QuestionRepository,
	sessionRepo *repository.SessionRepository,
	sessionQuestionRepo *repository.SessionQuestionRepository,
	srs *SRSService,
	llm external.LLMClient,
) *GraderService {
	return &GraderService{
		userRepo:            userRepo,
		questionRepo:        questionRepo,
		sessionRepo:         sessionRepo,
		sessionQuestionRepo: sessionQuestionRepo,
		srs:                 srs,
		llm:                 llm,
	}
}

// SessionResult contains the summary of a completed session.
type SessionResult struct {
	TotalQuestions int
	CorrectCount   int
	WrongAnswers   []model.SessionQuestion
}

// GradeAnswer grades a single answer and updates SRS accordingly.
func (g *GraderService) GradeAnswer(ctx context.Context, sessionID, questionID int, userAnswer string) (bool, string, error) {
	// Get the question to check correct answer
	question, err := g.questionRepo.GetByID(ctx, questionID)
	if err != nil {
		return false, "", err
	}

	var isCorrect bool
	var feedback string

	// 쓰기 문제인 경우 llm이 유사도를 확인해 채점
	if question.Type == model.QuestionSubjective {
		isCorrect, feedback, err = g.llm.GradeAnswer(ctx, question.Prompt, question.CorrectAnswer, userAnswer)
		if err != nil {
			return false, "", err
		}
	} else {
		isCorrect = userAnswer == question.CorrectAnswer
	}

	if err := g.recordGradingResult(ctx, sessionID, questionID, question, userAnswer, isCorrect); err != nil {
		return false, "", err
	}

	return isCorrect, feedback, nil
}

func (g *GraderService) GradeHandwriting(ctx context.Context, sessionID, questionID int, renderedImage []byte) (bool, string, error) {
	question, err := g.questionRepo.GetByID(ctx, questionID)
	if err != nil {
		return false, "", err
	}
	if question.Type != model.QuestionKanaHandwriting {
		return false, "", ErrHandwritingInvalidQuestion
	}

	isCorrect, feedback, err := g.llm.GradeHandwriting(ctx, question.Prompt, question.CorrectAnswer, renderedImage)
	if err != nil {
		return false, "", err
	}

	userAnswer := "handwriting:" + question.CorrectAnswer
	if err := g.recordGradingResult(ctx, sessionID, questionID, question, userAnswer, isCorrect); err != nil {
		return false, "", err
	}

	return isCorrect, feedback, nil
}

func (g *GraderService) recordGradingResult(ctx context.Context, sessionID, questionID int, question *model.Question, userAnswer string, isCorrect bool) error {
	// Record the answer in session_questions
	if err := g.sessionQuestionRepo.RecordAnswer(ctx, sessionID, questionID, userAnswer, isCorrect); err != nil {
		return err
	}

	// Update question statistics
	if err := g.questionRepo.IncrementServed(ctx, questionID); err != nil {
		return err
	}
	if isCorrect {
		if err := g.questionRepo.IncrementCorrect(ctx, questionID); err != nil {
			return err
		}
	}

	// Update SRS schedule on the question itself
	if err := g.srs.ProcessAnswer(ctx, question, isCorrect); err != nil {
		return err
	}

	return nil
}

// CompleteSession finalizes a session with results.
func (g *GraderService) CompleteSession(ctx context.Context, sessionID int, userID int64) (*SessionResult, error) {
	sqs, err := g.sessionQuestionRepo.GetBySession(ctx, sessionID)
	if err != nil {
		return nil, err
	}

	correctCount := 0
	for _, sq := range sqs {
		if sq.IsCorrect != nil && *sq.IsCorrect {
			correctCount++
		}
	}

	// Update session
	if err := g.sessionRepo.Complete(ctx, sessionID, correctCount); err != nil {
		return nil, err
	}

	// Update streak
	if err := g.userRepo.UpdateStreak(ctx, userID); err != nil {
		return nil, err
	}

	// Get wrong answers for result display
	wrongSQs, _ := g.sessionQuestionRepo.GetWrongAnswers(ctx, sessionID)

	return &SessionResult{
		TotalQuestions: len(sqs),
		CorrectCount:   correctCount,
		WrongAnswers:   wrongSQs,
	}, nil
}
