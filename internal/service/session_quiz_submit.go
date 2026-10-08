package service

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/lsj/copylingo/internal/model"
)

var (
	// ErrQuizStateUnavailable means the Quiz working set could not be loaded.
	ErrQuizStateUnavailable = errors.New("quiz working set unavailable")
	// ErrQuizAnswerStale means the answer targets a question that is already
	// answered or no longer current; the caller should show the next unanswered one.
	ErrQuizAnswerStale = errors.New("quiz answer targets a stale question")
	// ErrQuizInvalidOption means the chosen option index does not exist.
	ErrQuizInvalidOption = errors.New("quiz option out of range")
)

// QuizAnswerResult is a recorded Quiz answer with what the caller needs to render it.
type QuizAnswerResult struct {
	Question model.Question
	// Answer is the answer as graded (text answers trimmed, fill-blank lowercased).
	Answer    string
	IsCorrect bool
	Feedback  string
	// GradingUnavailable means AI grading failed and the answer was recorded
	// as wrong instead (subjective failure policy).
	GradingUnavailable bool
	QuestionIndex      int
	TotalQuestions     int
}

// QuizTextAnswer is a typed answer to the question at QuestionIndex.
type QuizTextAnswer struct {
	// UserID is the owner of the session; mismatches are rejected.
	UserID        int64
	SessionID     int
	QuestionIndex int
	Text          string
	// OnAIGrading, when set, runs once right before a slow AI grading call
	// (subjective questions), e.g. to show a typing indicator.
	OnAIGrading func()
}

// SubmitQuizOption grades the chosen option of the current question.
// userID must be the session owner, otherwise ErrQuizActiveSessionUserMismatch.
func (s *SessionService) SubmitQuizOption(
	ctx context.Context,
	userID int64,
	sessionID,
	questionID,
	optionIdx int,
) (*QuizAnswerResult, error) {
	state, err := s.loadQuizForAnswer(
		ctx,
		userID,
		sessionID,
	)
	if err != nil {
		return nil, err
	}
	item, idx, err := currentUnansweredQuizItem(
		state,
		questionID,
	)
	if err != nil {
		return nil, err
	}
	options, err := item.Question.GetOptions()
	if err != nil || optionIdx < 0 || optionIdx >= len(options) {
		return nil, fmt.Errorf(
			"%w session_id=%d question_id=%d option=%d",
			ErrQuizInvalidOption,
			sessionID,
			questionID,
			optionIdx,
		)
	}
	return s.gradeQuizAnswer(
		ctx,
		sessionID,
		state,
		item,
		idx,
		options[optionIdx],
		nil,
	)
}

// SubmitQuizWordOrder grades a word-order answer given as option indices in
// the chosen order. The selection must use every option exactly once.
// userID must be the session owner, otherwise ErrQuizActiveSessionUserMismatch.
func (s *SessionService) SubmitQuizWordOrder(
	ctx context.Context,
	userID int64,
	sessionID,
	questionID int,
	selection []int,
) (*QuizAnswerResult, error) {
	state, err := s.loadQuizForAnswer(
		ctx,
		userID,
		sessionID,
	)
	if err != nil {
		return nil, err
	}
	item, idx, err := currentUnansweredQuizItem(
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
	for _, optionIdx := range selection {
		answer.WriteString(options[optionIdx])
	}
	return s.gradeQuizAnswer(
		ctx,
		sessionID,
		state,
		item,
		idx,
		answer.String(),
		nil,
	)
}

// SubmitQuizText grades a typed answer to the question at answer.QuestionIndex.
// An out-of-range index reports ErrQuizActiveSessionQuestionNotFound.
// answer.UserID must be the session owner, otherwise ErrQuizActiveSessionUserMismatch.
func (s *SessionService) SubmitQuizText(
	ctx context.Context,
	answer QuizTextAnswer,
) (*QuizAnswerResult, error) {
	state, err := s.loadQuizForAnswer(
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
	item, idx, err := currentUnansweredQuizItem(
		state,
		state.Items[answer.QuestionIndex].SessionQuestion.QuestionID,
	)
	if err != nil {
		return nil, err
	}
	return s.gradeQuizAnswer(
		ctx,
		answer.SessionID,
		state,
		item,
		idx,
		strings.TrimSpace(answer.Text),
		answer.OnAIGrading,
	)
}

func (s *SessionService) loadQuizForAnswer(
	ctx context.Context,
	userID int64,
	sessionID int,
) (*model.QuizActiveSessionState, error) {
	state, err := s.quizProgress.Get(
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
	item, idx, ok := state.CurrentItemByQuestionID(questionID)
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
	return item, idx, nil
}

// isOptionPermutation reports whether selection orders all optionCount options exactly once.
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
	for _, optionIdx := range selection {
		if optionIdx < 0 || optionIdx >= optionCount || seen[optionIdx] {
			return false
		}
		seen[optionIdx] = true
	}
	return true
}

// gradeQuizAnswer grades and records one answer. When AI grading is
// unavailable, the attempt is recorded as wrong so the Quiz can continue
// (subjective failure policy; handwriting returns the error instead).
func (s *SessionService) gradeQuizAnswer(
	ctx context.Context,
	sessionID int,
	state *model.QuizActiveSessionState,
	item *model.QuizActiveSessionQuestion,
	idx int,
	answer string,
	onAIGrading func(),
) (*QuizAnswerResult, error) {
	question := item.Question
	switch question.Type {
	case model.QuestionFillBlank:
		answer = strings.ToLower(answer) // Kana fill-in-the-blank is case-insensitive.
	case model.QuestionSubjective:
		if onAIGrading != nil {
			onAIGrading()
		}
	}
	result := &QuizAnswerResult{
		Question:       question,
		Answer:         answer,
		QuestionIndex:  idx,
		TotalQuestions: len(state.Items),
	}

	isCorrect, feedback, err := s.grader.GradeAnswerWithQuestion(
		ctx,
		sessionID,
		question.ID,
		&question,
		answer,
	)
	switch {
	case err == nil:
		result.IsCorrect = isCorrect
		result.Feedback = feedback
		return result, nil
	case errors.Is(
		err,
		ErrQuizActiveSessionAlreadyAnswered,
	):
		return nil, fmt.Errorf(
			"%w: %w",
			ErrQuizAnswerStale,
			err,
		)
	case !errors.Is(
		err,
		ErrAIUnavailable,
	):
		return nil, fmt.Errorf(
			"grade quiz answer session_id=%d question_id=%d: %w",
			sessionID,
			question.ID,
			err,
		)
	}

	if err := s.quizProgress.RecordAnswer(
		ctx,
		sessionID,
		question.ID,
		answer,
		false,
	); err != nil {
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
		return nil, fmt.Errorf(
			"record fallback wrong answer session_id=%d question_id=%d: %w",
			sessionID,
			question.ID,
			err,
		)
	}
	result.GradingUnavailable = true
	return result, nil
}
