package service

import (
	"context"
	"fmt"

	"github.com/lsj/copylingo/internal/model"
)

// QuizCompletion carries the word-order question IDs whose ephemeral drafts the caller should discard.
type QuizCompletion struct {
	QuizSessionResult
	WordOrderQuestionIDs []int
}

// CompleteQuiz updates the streak (Quiz completion only; Study does not count).
// Word-order IDs are captured before Delete removes the working set, only when the caller owns the session.
func (service *SessionService) CompleteQuiz(
	ctx context.Context,
	sessionID int,
	userID int64,
) (*QuizCompletion, error) {
	var wordOrderQuestionIDs []int
	if state, err := service.quizProgress.Get(
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

	result, err := service.quizProgress.Flush(
		ctx,
		sessionID,
		userID,
	)
	if err != nil {
		return nil, err
	}
	if err := service.userRepository.UpdateStreak(
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
	if err := service.quizProgress.Delete(
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
