package service

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/lsj/copylingo/internal/model"
)

var (
	ErrQuizStateUnavailable = errors.New("quiz working set unavailable")
	// ErrQuizAnswerStale means the answer targets a question that is already
	// answered or no longer current; the caller should show the next unanswered one.
	ErrQuizAnswerStale   = errors.New("quiz answer targets a stale question")
	ErrQuizInvalidOption = errors.New("quiz option out of range")
)

type QuizAnswerResult struct {
	Question model.Question
	// Answer is the answer as graded (text answers trimmed, fill-blank lowercased).
	Answer         string
	IsCorrect      bool
	QuestionIndex  int
	TotalQuestions int
}

type QuizTextAnswer struct {
	// UserID is the owner of the session; mismatches are rejected.
	UserID        int64
	SessionID     int
	QuestionIndex int
	Text          string
}

// userID must be the session owner, otherwise ErrQuizActiveSessionUserMismatch.
func (service *SessionService) SubmitQuizOption(
	ctx context.Context,
	userID int64,
	sessionID,
	questionID,
	optionIndex int,
) (*QuizAnswerResult, error) {
	state, err := service.loadQuizForAnswer(
		ctx,
		userID,
		sessionID,
	)
	if err != nil {
		return nil, err
	}
	item, index, err := currentUnansweredQuizItem(
		state,
		questionID,
	)
	if err != nil {
		return nil, err
	}
	options, err := item.Question.GetOptions()
	if err != nil || optionIndex < 0 || optionIndex >= len(options) {
		return nil, fmt.Errorf(
			"%w session_id=%d question_id=%d option=%d",
			ErrQuizInvalidOption,
			sessionID,
			questionID,
			optionIndex,
		)
	}
	return service.gradeQuizAnswer(
		ctx,
		sessionID,
		state,
		item,
		index,
		options[optionIndex],
	)
}

// the chosen order. The selection must use every option exactly once.
// userID must be the session owner, otherwise ErrQuizActiveSessionUserMismatch.
func (service *SessionService) SubmitQuizWordOrder(
	ctx context.Context,
	userID int64,
	sessionID,
	questionID int,
	selection []int,
) (*QuizAnswerResult, error) {
	state, err := service.loadQuizForAnswer(
		ctx,
		userID,
		sessionID,
	)
	if err != nil {
		return nil, err
	}
	item, index, err := currentUnansweredQuizItem(
		state,
		questionID,
	)
	if err != nil {
		return nil, err
	}
	options, err := item.Question.GetOptions()
	if err != nil || !isOptionPermutation(
		selection,
		len(options),
	) {
		return nil, fmt.Errorf(
			"%w session_id=%d question_id=%d selection=%v",
			ErrQuizInvalidOption,
			sessionID,
			questionID,
			selection,
		)
	}
	var answer strings.Builder
	for _, optionIndex := range selection {
		answer.WriteString(options[optionIndex])
	}
	return service.gradeQuizAnswer(
		ctx,
		sessionID,
		state,
		item,
		index,
		answer.String(),
	)
}

// An out-of-range index reports ErrQuizActiveSessionQuestionNotFound.
// answer.UserID must be the session owner, otherwise ErrQuizActiveSessionUserMismatch.
func (service *SessionService) SubmitQuizText(
	ctx context.Context,
	answer QuizTextAnswer,
) (*QuizAnswerResult, error) {
	state, err := service.loadQuizForAnswer(
		ctx,
		answer.UserID,
		answer.SessionID,
	)
	if err != nil {
		return nil, err
	}
	if answer.QuestionIndex < 0 || answer.QuestionIndex >= len(state.Items) {
		return nil, fmt.Errorf(
			"%w session_id=%d idx=%d",
			ErrQuizActiveSessionQuestionNotFound,
			answer.SessionID,
			answer.QuestionIndex,
		)
	}
	item, index, err := currentUnansweredQuizItem(
		state,
		state.Items[answer.QuestionIndex].SessionQuestion.QuestionID,
	)
	if err != nil {
		return nil, err
	}
	return service.gradeQuizAnswer(
		ctx,
		answer.SessionID,
		state,
		item,
		index,
		strings.TrimSpace(answer.Text),
	)
}

func (service *SessionService) loadQuizForAnswer(
	ctx context.Context,
	userID int64,
	sessionID int,
) (*model.QuizActiveSessionState, error) {
	state, err := service.quizProgress.Get(
		ctx,
		sessionID,
	)
	if err != nil {
		return nil, fmt.Errorf(
			"%w session_id=%d: %w",
			ErrQuizStateUnavailable,
			sessionID,
			err,
		)
	}
	// Callback data is forgeable, so the caller must own the session.
	if state.Session.UserID != userID {
		return nil, fmt.Errorf(
			"%w session_id=%d user_id=%d",
			ErrQuizActiveSessionUserMismatch,
			sessionID,
			userID,
		)
	}
	return state, nil
}

// currentUnansweredQuizItem resolves the answerable occurrence of questionID.
// A question that exists but is not current, or is already answered, is stale.
func currentUnansweredQuizItem(
	state *model.QuizActiveSessionState,
	questionID int,
) (*model.QuizActiveSessionQuestion, int, error) {
	item, index, ok := state.CurrentItemByQuestionID(questionID)
	if !ok {
		if _, exists := state.ItemByQuestionID(questionID); exists {
			return nil, 0, fmt.Errorf(
				"%w session_id=%d question_id=%d",
				ErrQuizAnswerStale,
				state.Session.ID,
				questionID,
			)
		}
		return nil, 0, fmt.Errorf(
			"%w session_id=%d question_id=%d",
			ErrQuizActiveSessionQuestionNotFound,
			state.Session.ID,
			questionID,
		)
	}
	if item.SessionQuestion.IsCorrect != nil {
		return nil, 0, fmt.Errorf(
			"%w session_id=%d question_id=%d",
			ErrQuizAnswerStale,
			state.Session.ID,
			questionID,
		)
	}
	return item, index, nil
}

func isOptionPermutation(
	selection []int,
	optionCount int,
) bool {
	if len(selection) != optionCount {
		return false
	}
	seen := make(
		[]bool,
		optionCount,
	)
	for _, optionIndex := range selection {
		if optionIndex < 0 || optionIndex >= optionCount || seen[optionIndex] {
			return false
		}
		seen[optionIndex] = true
	}
	return true
}

func (service *SessionService) gradeQuizAnswer(
	ctx context.Context,
	sessionID int,
	state *model.QuizActiveSessionState,
	item *model.QuizActiveSessionQuestion,
	index int,
	answer string,
) (*QuizAnswerResult, error) {
	question := item.Question
	if question.Type == model.QuestionFillBlank {
		answer = strings.ToLower(answer) // Kana fill-in-the-blank is case-insensitive.
	}

	// Text and option answers are exact-match; only handwriting uses the LLM.
	isCorrect := answer == question.CorrectAnswer
	err := service.quizProgress.RecordAnswer(
		ctx,
		sessionID,
		question.ID,
		answer,
		isCorrect,
	)
	if errors.Is(
		err,
		ErrQuizActiveSessionAlreadyAnswered,
	) {
		return nil, fmt.Errorf(
			"%w: %w",
			ErrQuizAnswerStale,
			err,
		)
	}
	if err != nil {
		return nil, fmt.Errorf(
			"record quiz answer session_id=%d question_id=%d: %w",
			sessionID,
			question.ID,
			err,
		)
	}
	return &QuizAnswerResult{
		Question:       question,
		Answer:         answer,
		IsCorrect:      isCorrect,
		QuestionIndex:  index,
		TotalQuestions: len(state.Items),
	}, nil
}
