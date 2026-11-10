package miniapp

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/lsj/copylingo/internal/config"
	"github.com/lsj/copylingo/internal/observability"
	"github.com/lsj/copylingo/internal/service"
)

// quizSession is the Quiz part of service.SessionService used by the Mini App:
// grading a handwriting submission.
type quizSession interface {
	SubmitHandwriting(
		ctx context.Context,
		request service.HandwritingSubmitRequest,
	) (*service.HandwritingSubmitResult, error)
}

// handwritingScreen is the bot-side Telegram message of a handwriting
// question. The bot owns finding the message, re-reading progress, and
// building the keyboard; the Mini App only reports that grading finished.
type handwritingScreen interface {
	ShowHandwritingGraded(
		ctx context.Context,
		sessionID,
		questionID int,
	)
}

type verifier interface {
	Verify(initData string) (*TelegramUser, error)
}

type Handler struct {
	session           quizSession
	verifier          verifier
	handwritingScreen handwritingScreen
}

type HandlerDependencies struct {
	Session           quizSession
	Verifier          verifier
	HandwritingScreen handwritingScreen
}

type handwritingSubmitRequest struct {
	InitData   string   `json:"init_data"   binding:"required"`
	SessionID  int      `json:"session_id"  binding:"required"`
	QuestionID int      `json:"question_id" binding:"required"`
	Strokes    []Stroke `json:"strokes"     binding:"required"`
}

func NewHandler(dependencies HandlerDependencies) *Handler {
	return &Handler{
		session:           dependencies.Session,
		verifier:          dependencies.Verifier,
		handwritingScreen: dependencies.HandwritingScreen,
	}
}

// RegisterRoutes serves the handwriting Mini App assets and its API on handler,
// which cmd/server builds with its dependencies.
func RegisterRoutes(
	router *gin.Engine,
	handler *Handler,
) {
	router.Static(
		"/miniapp/handwriting/assets",
		"./web/miniapp/handwriting",
	)
	router.GET(
		config.PathHandwritingMiniApp,
		func(ginContext *gin.Context) {
			ginContext.File("./web/miniapp/handwriting/index.html")
		},
	)
	router.POST(
		config.PathHandwritingSubmit,
		handler.SubmitHandwriting,
	)
}

func (handler *Handler) SubmitHandwriting(ginContext *gin.Context) {
	startedAt := time.Now()
	ctx := observability.WithAttributes(
		ginContext.Request.Context(),
		slog.String(
			"source",
			"miniapp.handwriting",
		),
	)

	var request handwritingSubmitRequest
	if err := ginContext.ShouldBindJSON(&request); err != nil {
		ginContext.JSON(
			http.StatusBadRequest,
			gin.H{"error": "invalid request"},
		)
		return
	}

	user, err := handler.verifier.Verify(request.InitData)
	if err != nil {
		ginContext.JSON(
			http.StatusUnauthorized,
			gin.H{"error": "invalid telegram init data"},
		)
		return
	}
	ctx = observability.WithAttributes(
		ctx,
		slog.Int64(
			"user_id",
			user.ID,
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

	result, err := handler.renderAndSubmit(
		ctx,
		user.ID,
		request,
	)
	if err != nil {
		publicErr := handwritingPublicError(err)
		slog.ErrorContext(
			ctx,
			"Handwriting submission failed",
			"event",
			"handwriting.submit.failed",
			"status",
			publicErr.status,
			"error",
			err,
		)
		ginContext.JSON(
			publicErr.status,
			gin.H{"error": publicErr.message},
		)
		return
	}

	slog.InfoContext(
		ctx,
		"Handwriting submission completed",
		"event",
		"handwriting.submit.completed",
		"duration_ms",
		time.Since(startedAt).Milliseconds(),
		"is_correct",
		result.IsCorrect,
	)

	// Refresh Telegram message buttons in background
	go handler.refreshHandwritingMessage(
		context.WithoutCancel(ctx),
		request.SessionID,
		request.QuestionID,
	)

	ginContext.JSON(
		http.StatusOK,
		result,
	)
}

// renderAndSubmit turns the strokes into the PNG the service grades. Strokes
// are this Mini App's input format, so the service only ever sees the image.
func (handler *Handler) renderAndSubmit(
	ctx context.Context,
	userID int64,
	request handwritingSubmitRequest,
) (*service.HandwritingSubmitResult, error) {
	image, err := RenderStrokesPNG(request.Strokes)
	if err != nil {
		return nil, fmt.Errorf(
			"render handwriting strokes: %w",
			err,
		)
	}
	return handler.session.SubmitHandwriting(
		ctx,
		service.HandwritingSubmitRequest{
			UserID:     userID,
			SessionID:  request.SessionID,
			QuestionID: request.QuestionID,
			Image:      image,
		},
	)
}

type publicError struct {
	status  int
	message string
}

func handwritingPublicError(err error) publicError {
	switch {
	case errors.Is(
		err,
		service.ErrHandwritingUnauthorized,
	):
		return publicError{status: http.StatusForbidden, message: "권한이 없습니다."}
	case errors.Is(
		err,
		service.ErrHandwritingQuestionMismatch,
	),
		errors.Is(
			err,
			service.ErrHandwritingInvalidQuestion,
		),
		errors.Is(
			err,
			ErrEmptyStrokes,
		):
		return publicError{status: http.StatusBadRequest, message: "손글씨 제출 정보를 확인할 수 없습니다."}
	case errors.Is(
		err,
		service.ErrHandwritingAlreadyAnswered,
	):
		return publicError{status: http.StatusConflict, message: "이미 채점된 문항입니다."}
	default:
		return publicError{status: http.StatusServiceUnavailable, message: "현재 AI 채점이 지연되고 있습니다. 잠시 후 다시 시도해 주세요."}
	}
}

func (handler *Handler) refreshHandwritingMessage(
	parent context.Context,
	sessionID,
	questionID int,
) {
	parent = observability.WithAttributes(
		parent,
		slog.String(
			"source",
			"miniapp.handwriting.cleanup",
		),
		slog.Int(
			"session_id",
			sessionID,
		),
		slog.Int(
			"question_id",
			questionID,
		),
	)
	ctx, cancel := context.WithTimeout(
		parent,
		15*time.Second,
	)
	defer cancel()

	handler.handwritingScreen.ShowHandwritingGraded(
		ctx,
		sessionID,
		questionID,
	)
}
