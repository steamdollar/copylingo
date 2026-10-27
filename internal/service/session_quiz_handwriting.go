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

// HandwritingSubmitRequest is the service-level request after HTTP auth is
// verified. Image is the PNG the Mini App rendered from the learner's strokes.
type HandwritingSubmitRequest struct {
	UserID     int64
	SessionID  int
	QuestionID int
	Image      []byte
}

// HandwritingSubmitResult is returned to the Mini App after grading.
type HandwritingSubmitResult struct {
	IsCorrect     bool   `json:"is_correct"`
	Feedback      string `json:"feedback"`
	CorrectAnswer string `json:"correct_answer"`
	Explanation   string `json:"explanation"`
}

// SubmitHandwriting grades a Mini App handwriting submission for the current
// question. An AI grading failure is returned as an error and nothing is
// recorded, so the user can resubmit.
func (s *SessionService) SubmitHandwriting(
	ctx context.Context,
	req HandwritingSubmitRequest,
) (*HandwritingSubmitResult, error) {
	startedAt := time.Now()
	ctx = observability.WithAttrs(
		ctx,
		slog.String(
			"source",
			"service.handwriting",
		),
		slog.Int64(
			"user_id",
			req.UserID,
		),
		slog.Int(
			"session_id",
			req.SessionID,
		),
		slog.Int(
			"question_id",
			req.QuestionID,
		),
	)

	state, err := s.quizProgress.Get(
		ctx,
		req.SessionID,
	)
	if err != nil {
		return nil, fmt.Errorf(
			"get active session for handwriting submission: %w",
			err,
		)
	}
	if state.Session.UserID != req.UserID {
		return nil, ErrHandwritingUnauthorized
	}

	item, _, ok := state.CurrentItemByQuestionID(req.QuestionID)
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
	// An LLM failure returns before recording, so the learner can resubmit.
	result, err := s.llm.GradeHandwriting(
		ctx,
		question.Prompt,
		question.CorrectAnswer,
		req.Image,
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
	if err := s.quizProgress.RecordAnswer(
		ctx,
		req.SessionID,
		req.QuestionID,
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
		len(req.Image),
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
