package service

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"time"

	"github.com/lsj/copylingo/internal/model"
	"github.com/lsj/copylingo/internal/observability"
)

var (
	ErrHandwritingUnauthorized     = errors.New("handwriting submission is not owned by user")
	ErrHandwritingQuestionMismatch = errors.New("handwriting question is not part of session")
	ErrHandwritingAlreadyAnswered  = errors.New("handwriting question is already answered")
	ErrHandwritingInvalidQuestion  = errors.New("question is not a handwriting question")
)

// HandwritingSubmitRequest.Image is the PNG the Mini App rendered from the learner's strokes.
type HandwritingSubmitRequest struct {
	UserID     int64
	SessionID  int
	QuestionID int
	Image      []byte
}

type HandwritingSubmitResult struct {
	IsCorrect     bool   `json:"is_correct"`
	Feedback      string `json:"feedback"`
	CorrectAnswer string `json:"correct_answer"`
	Explanation   string `json:"explanation"`
}

// An AI grading failure is returned as an error and nothing is recorded, so the user can resubmit.
func (service *SessionService) SubmitHandwriting(
	ctx context.Context,
	request HandwritingSubmitRequest,
) (*HandwritingSubmitResult, error) {
	startedAt := time.Now()
	ctx = observability.WithAttributes(
		ctx,
		slog.String(
			"source",
			"service.handwriting",
		),
		slog.Int64(
			"user_id",
			request.UserID,
		),
		slog.Int(
			"session_id",
			request.SessionID,
		),
		slog.Int(
			"question_id",
			request.QuestionID,
		),
	)

	state, err := service.quizProgress.Get(
		ctx,
		request.SessionID,
	)
	if err != nil {
		return nil, fmt.Errorf(
			"get active session for handwriting submission: %w",
			err,
		)
	}
	if state.Session.UserID != request.UserID {
		return nil, ErrHandwritingUnauthorized
	}

	item, _, ok := state.CurrentItemByQuestionID(request.QuestionID)
	if !ok {
		return nil, ErrHandwritingQuestionMismatch
	}
	question := &item.Question
	if question.Type != model.QuestionKanaHandwriting {
		return nil, ErrHandwritingInvalidQuestion
	}
	if item.SessionQuestion.IsCorrect != nil {
		return nil, ErrHandwritingAlreadyAnswered
	}

	llmStartedAt := time.Now()
	result, err := service.llm.GradeHandwriting(
		ctx,
		question.Prompt,
		question.CorrectAnswer,
		request.Image,
	)
	if err != nil {
		return nil, fmt.Errorf(
			"grade handwriting answer: %w",
			err,
		)
	}
	gradedAt := time.Now()

	// RecordAnswer re-reads the state, so an answer recorded while the LLM
	// was grading is rejected here instead of overwritten.
	if err := service.quizProgress.RecordAnswer(
		ctx,
		request.SessionID,
		request.QuestionID,
		"handwriting:submitted",
		result.IsCorrect,
	); err != nil {
		if errors.Is(
			err,
			ErrQuizActiveSessionAlreadyAnswered,
		) {
			return nil, ErrHandwritingAlreadyAnswered
		}
		return nil, fmt.Errorf(
			"record handwriting answer: %w",
			err,
		)
	}
	slog.InfoContext(
		ctx,
		"Handwriting service completed",
		"event",
		"handwriting.service.completed",
		"duration_ms",
		time.Since(startedAt).Milliseconds(),
		"llm_duration_ms",
		gradedAt.Sub(llmStartedAt).Milliseconds(),
		"record_duration_ms",
		time.Since(gradedAt).Milliseconds(),
		"image_bytes",
		len(request.Image),
		"is_correct",
		result.IsCorrect,
	)

	return &HandwritingSubmitResult{
		IsCorrect:     result.IsCorrect,
		Feedback:      result.Feedback,
		CorrectAnswer: question.CorrectAnswer,
		Explanation:   question.Explanation,
	}, nil
}
