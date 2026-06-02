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

type handwritingActiveSession interface {
	Get(ctx context.Context, sessionID int) (*model.ActiveSessionState, error)
}

type graderClient interface {
	GradeHandwritingWithQuestion(ctx context.Context, sessionID, questionID int, question *model.Question, renderedImage []byte) (bool, string, error)
}

var (
	ErrHandwritingUnauthorized     = errors.New("handwriting submission is not owned by user")
	ErrHandwritingQuestionMismatch = errors.New("handwriting question is not part of session")
	ErrHandwritingAlreadyAnswered  = errors.New("handwriting question is already answered")
	ErrHandwritingInvalidQuestion  = errors.New("question is not a handwriting question")
)

// StrokePoint represents one sampled drawing point from the Mini App canvas.
type StrokePoint struct {
	X       float64 `json:"x"`
	Y       float64 `json:"y"`
	TimeMS  int64   `json:"time_ms,omitempty"`
	Drawing bool    `json:"drawing"`
}

// Stroke represents one continuous pen-down path.
type Stroke struct {
	Points []StrokePoint `json:"points"`
}

// HandwritingSubmitRequest is the service-level request after HTTP auth is verified.
type HandwritingSubmitRequest struct {
	UserID     int64    `json:"user_id"`
	SessionID  int      `json:"session_id"`
	QuestionID int      `json:"question_id"`
	Strokes    []Stroke `json:"strokes"`
}

// HandwritingSubmitResult is returned to the Mini App after grading.
type HandwritingSubmitResult struct {
	IsCorrect     bool   `json:"is_correct"`
	Feedback      string `json:"feedback"`
	CorrectAnswer string `json:"correct_answer"`
	Explanation   string `json:"explanation"`
}

// HandwritingService coordinates Mini App submissions without coupling HTTP and Bot flows.
type HandwritingService struct {
	activeSession handwritingActiveSession
	grader        graderClient
	renderer      StrokeRenderer
}

func NewHandwritingService(
	activeSession handwritingActiveSession,
	grader graderClient,
	renderer StrokeRenderer,
) *HandwritingService {
	if renderer == nil {
		renderer = NewDefaultPNGStrokeRenderer()
	}
	return &HandwritingService{
		activeSession: activeSession,
		grader:        grader,
		renderer:      renderer,
	}
}

func (s *HandwritingService) SubmitAnswer(ctx context.Context, req HandwritingSubmitRequest) (*HandwritingSubmitResult, error) {
	startedAt := time.Now()
	ctx = observability.WithAttrs(ctx,
		slog.String("source", "service.handwriting"),
		slog.Int64("user_id", req.UserID),
		slog.Int("session_id", req.SessionID),
		slog.Int("question_id", req.QuestionID),
	)

	state, err := s.activeSession.Get(ctx, req.SessionID)
	if err != nil {
		return nil, fmt.Errorf("get active session for handwriting submission: %w", err)
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

	renderedImage, err := s.renderer.RenderPNG(req.Strokes)
	if err != nil {
		return nil, fmt.Errorf("render handwriting strokes: %w", err)
	}
	renderedAt := time.Now()

	isCorrect, feedback, err := s.grader.GradeHandwritingWithQuestion(ctx, req.SessionID, req.QuestionID, question, renderedImage)
	if err != nil {
		if errors.Is(err, ErrActiveSessionAlreadyAnswered) {
			return nil, ErrHandwritingAlreadyAnswered
		}
		return nil, fmt.Errorf("grade handwriting answer: %w", err)
	}
	slog.InfoContext(ctx, "Handwriting service completed",
		"event", "handwriting.service.completed",
		"duration_ms", time.Since(startedAt).Milliseconds(),
		"render_duration_ms", renderedAt.Sub(startedAt).Milliseconds(),
		"grade_duration_ms", time.Since(renderedAt).Milliseconds(),
		"image_bytes", len(renderedImage),
	)

	return &HandwritingSubmitResult{
		IsCorrect:     isCorrect,
		Feedback:      feedback,
		CorrectAnswer: question.CorrectAnswer,
		Explanation:   question.Explanation,
	}, nil
}
