package service

import (
	"context"
	"fmt"

	"github.com/lsj/copylingo/internal/model"
)

// QuizCompletion is the completed-Quiz summary plus the word-order question IDs
// whose ephemeral drafts the caller should now discard.
type QuizCompletion struct {
	QuizSessionResult
	WordOrderQuestionIDs []int
}

// CompleteQuiz flushes the Quiz working set to DB, updates the user's streak
// (Quiz completion only; Study does not count) and deletes the working set.
//
// Word-order IDs are captured first because Delete removes the working set;
// they are collected only when the caller owns the session, and a failed
// capture lookup is left for Flush to report.
func (s *SessionService) CompleteQuiz(
	ctx context.Context,
	sessionID int,
	userID int64,
) (*QuizCompletion, error) {
	var wordOrderQuestionIDs []int
	if state, err := s.quizProgress.Get(
		ctx,
		sessionID,
	); err == nil && state.Session.UserID == userID {
		for _, item := range state.Items {
			if item.Question.Type == model.QuestionWordOrder {
				wordOrderQuestionIDs = append(
					wordOrderQuestionIDs,
					item.Question.ID,
				)
			}
		}
	}

	result, err := s.quizProgress.Flush(
		ctx,
		sessionID,
		userID,
	)
	if err != nil {
		return nil, err
	}
	if err := s.userRepo.UpdateStreak(
		ctx,
		userID,
	); err != nil {
		return nil, fmt.Errorf(
			"update streak after quiz session_id=%d user_id=%d: %w",
			sessionID,
			userID,
			err,
		)
	}
	if err := s.quizProgress.Delete(
		ctx,
		sessionID,
	); err != nil {
		return nil, err
	}
	return &QuizCompletion{
		QuizSessionResult:    *result,
		WordOrderQuestionIDs: wordOrderQuestionIDs,
	}, nil
}
