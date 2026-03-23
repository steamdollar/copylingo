package service

import (
	"context"

	"github.com/lsj/copylingo/internal/model"
	"github.com/lsj/copylingo/internal/repository"
)

// GraderService handles answer grading and result processing.
type GraderService struct {
	repos *repository.Repositories
	srs   *SRSService
}

func NewGraderService(repos *repository.Repositories, srs *SRSService) *GraderService {
	return &GraderService{repos: repos, srs: srs}
}

// GradeAnswer grades a single answer and updates SRS accordingly.
func (g *GraderService) GradeAnswer(ctx context.Context, sessionID, questionID int, userAnswer string) (bool, error) {
	// Get the question to check correct answer
	question, err := g.repos.Question.GetByID(ctx, questionID)
	if err != nil {
		return false, err
	}

	isCorrect := userAnswer == question.CorrectAnswer

	// Record the answer in session_questions
	if err := g.repos.SessionQuestion.RecordAnswer(ctx, sessionID, questionID, userAnswer, isCorrect); err != nil {
		return false, err
	}

	// Update question statistics
	if err := g.repos.Question.IncrementServed(ctx, questionID); err != nil {
		return false, err
	}
	if isCorrect {
		if err := g.repos.Question.IncrementCorrect(ctx, questionID); err != nil {
			return false, err
		}
	}

	// Update SRS schedule on the question itself
	if err := g.srs.ProcessAnswer(ctx, question, isCorrect); err != nil {
		return false, err
	}

	return isCorrect, nil
}

// CompleteSession finalizes a session with results.
func (g *GraderService) CompleteSession(ctx context.Context, sessionID int, userID int64) (*SessionResult, error) {
	sqs, err := g.repos.SessionQuestion.GetBySession(ctx, sessionID)
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
	if err := g.repos.Session.Complete(ctx, sessionID, correctCount); err != nil {
		return nil, err
	}

	// Update streak
	if err := g.repos.User.UpdateStreak(ctx, userID); err != nil {
		return nil, err
	}

	// Get wrong answers for result display
	wrongSQs, _ := g.repos.SessionQuestion.GetWrongAnswers(ctx, sessionID)

	return &SessionResult{
		TotalQuestions: len(sqs),
		CorrectCount:  correctCount,
		WrongAnswers:  wrongSQs,
	}, nil
}

// SessionResult contains the summary of a completed session.
type SessionResult struct {
	TotalQuestions int
	CorrectCount   int
	WrongAnswers   []model.SessionQuestion
}
