package service

import (
	"context"
	"errors"
	"fmt"
	"log"
	"time"

	"github.com/lsj/copylingo/internal/model"
)

type handwritingSessionRepo interface {
	GetByID(ctx context.Context, id int) (*model.Session, error)
}

type handwritingQuestionRepo interface {
	GetByID(ctx context.Context, id int) (*model.Question, error)
}

type handwritingSessionQuestionRepo interface {
	GetBySession(ctx context.Context, sessionID int) ([]model.SessionQuestion, error)
}

type graderClient interface {
	GradeHandwriting(ctx context.Context, sessionID, questionID int, renderedImage []byte) (bool, string, error)
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
	sessionRepo         handwritingSessionRepo
	questionRepo        handwritingQuestionRepo
	sessionQuestionRepo handwritingSessionQuestionRepo
	grader              graderClient
	renderer            StrokeRenderer
}

func NewHandwritingService(
	sessionRepo handwritingSessionRepo,
	questionRepo handwritingQuestionRepo,
	sessionQuestionRepo handwritingSessionQuestionRepo,
	grader graderClient,
	renderer StrokeRenderer,
) *HandwritingService {
	if renderer == nil {
		renderer = NewPNGStrokeRenderer(256, 24)
	}
	return &HandwritingService{
		sessionRepo:         sessionRepo,
		questionRepo:        questionRepo,
		sessionQuestionRepo: sessionQuestionRepo,
		grader:              grader,
		renderer:            renderer,
	}
}

func (s *HandwritingService) SubmitAnswer(ctx context.Context, req HandwritingSubmitRequest) (*HandwritingSubmitResult, error) {
	startedAt := time.Now()

	session, err := s.sessionRepo.GetByID(ctx, req.SessionID)
	if err != nil {
		return nil, fmt.Errorf("get session for handwriting submission: %w", err)
	}
	if session.UserID != req.UserID {
		return nil, ErrHandwritingUnauthorized
	}

	question, err := s.questionRepo.GetByID(ctx, req.QuestionID)
	if err != nil {
		return nil, fmt.Errorf("get question for handwriting submission: %w", err)
	}
	if question.Type != model.QuestionKanaHandwriting {
		return nil, ErrHandwritingInvalidQuestion
	}

	sessionQuestions, err := s.sessionQuestionRepo.GetBySession(ctx, req.SessionID)
	if err != nil {
		return nil, fmt.Errorf("get session questions for handwriting submission: %w", err)
	}

	var matched *model.SessionQuestion
	for i := range sessionQuestions {
		if sessionQuestions[i].QuestionID == req.QuestionID {
			matched = &sessionQuestions[i]
			break
		}
	}
	if matched == nil {
		return nil, ErrHandwritingQuestionMismatch
	}
	if matched.IsCorrect != nil {
		return nil, ErrHandwritingAlreadyAnswered
	}

	renderedImage, err := s.renderer.RenderPNG(req.Strokes)
	if err != nil {
		return nil, fmt.Errorf("render handwriting strokes: %w", err)
	}
	renderedAt := time.Now()

	isCorrect, feedback, err := s.grader.GradeHandwriting(ctx, req.SessionID, req.QuestionID, renderedImage)
	if err != nil {
		return nil, fmt.Errorf("grade handwriting answer: %w", err)
	}
	log.Printf("[Handwriting] service total=%s render=%s grade=%s session_id=%d question_id=%d image_bytes=%d",
		time.Since(startedAt), renderedAt.Sub(startedAt), time.Since(renderedAt), req.SessionID, req.QuestionID, len(renderedImage))

	return &HandwritingSubmitResult{
		IsCorrect:     isCorrect,
		Feedback:      feedback,
		CorrectAnswer: question.CorrectAnswer,
		Explanation:   question.Explanation,
	}, nil
}
