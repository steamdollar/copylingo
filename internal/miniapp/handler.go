package miniapp

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/lsj/copylingo/internal/config"
	"github.com/lsj/copylingo/internal/model"
	"github.com/lsj/copylingo/internal/observability"
	"github.com/lsj/copylingo/internal/service"
)

// quizSession is the Quiz part of service.SessionService used by the Mini App:
// grading a handwriting submission.
type quizSession interface {
	SubmitHandwriting(
		ctx context.Context,
		req service.HandwritingSubmitRequest,
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

type tipService interface {
	ListActive(
		ctx context.Context,
		language,
		level string,
		limit int,
	) ([]model.Tip, error)
}

type verifier interface {
	Verify(initData string) (*TelegramUser, error)
}

type Handler struct {
	session           quizSession
	tip               tipService
	verifier          verifier
	handwritingScreen handwritingScreen
}

type HandlerDeps struct {
	Session           quizSession
	Tip               tipService
	Verifier          verifier
	HandwritingScreen handwritingScreen
}

type handwritingSubmitRequest struct {
	InitData   string           `json:"init_data"   binding:"required"`
	SessionID  int              `json:"session_id"  binding:"required"`
	QuestionID int              `json:"question_id" binding:"required"`
	Strokes    []service.Stroke `json:"strokes"     binding:"required"`
}

func NewHandler(deps HandlerDeps) *Handler {
	return &Handler{
		session:           deps.Session,
		tip:               deps.Tip,
		verifier:          deps.Verifier,
		handwritingScreen: deps.HandwritingScreen,
	}
}

func RegisterRoutes(
	r *gin.Engine,
	cfg *config.Config,
	services *service.Services,
	handwritingScreen handwritingScreen,
) {
	handler := NewHandler(HandlerDeps{
		Session: services.Session,
		Tip:     services.Tip,
		Verifier: NewInitDataVerifier(
			cfg.Telegram.Token,
			24*time.Hour,
		),
		HandwritingScreen: handwritingScreen,
	})

	r.Static(
		"/miniapp/handwriting/assets",
		"./web/miniapp/handwriting",
	)
	r.GET(
		config.PathHandwritingMiniApp,
		func(c *gin.Context) {
			c.File("./web/miniapp/handwriting/index.html")
		},
	)
	r.POST(
		config.PathHandwritingSubmit,
		handler.SubmitHandwriting,
	)
	r.GET(
		config.PathMiniAppTips,
		handler.ListTips,
	)
}

func (h *Handler) ListTips(c *gin.Context) {
	ctx := observability.WithAttrs(
		c.Request.Context(),
		slog.String(
			"source",
			"miniapp.tips",
		),
	)
	language := strings.TrimSpace(c.Query("language"))
	level := strings.TrimSpace(c.Query("level"))
	if language == "" || level == "" {
		c.JSON(
			http.StatusBadRequest,
			gin.H{"error": "language and level required"},
		)
		return
	}

	limit := 30
	if raw := c.Query("limit"); raw != "" {
		if n, err := strconv.Atoi(raw); err == nil && n > 0 {
			if n > 50 {
				n = 50
			}
			limit = n
		}
	}

	tips, err := h.tip.ListActive(
		ctx,
		language,
		level,
		limit,
	)
	if err != nil {
		slog.ErrorContext(
			ctx,
			"Failed to list tips",
			"event",
			"miniapp.tips.list_failed",
			"language",
			language,
			"level",
			level,
			"error",
			err,
		)
		c.JSON(
			http.StatusInternalServerError,
			gin.H{"error": "failed to load tips"},
		)
		return
	}
	if tips == nil {
		tips = []model.Tip{}
	}

	// model.Tip 의 json 태그가 source_* / is_active / created_at 를 "-" 로 두었으므로 그대로 직렬화하면 public 필드만 나간다.
	c.JSON(
		http.StatusOK,
		tips,
	)
}

func (h *Handler) SubmitHandwriting(c *gin.Context) {
	startedAt := time.Now()
	ctx := observability.WithAttrs(
		c.Request.Context(),
		slog.String(
			"source",
			"miniapp.handwriting",
		),
	)

	var req handwritingSubmitRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(
			http.StatusBadRequest,
			gin.H{"error": "invalid request"},
		)
		return
	}

	user, err := h.verifier.Verify(req.InitData)
	if err != nil {
		c.JSON(
			http.StatusUnauthorized,
			gin.H{"error": "invalid telegram init data"},
		)
		return
	}
	ctx = observability.WithAttrs(
		ctx,
		slog.Int64(
			"user_id",
			user.ID,
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

	result, err := h.session.SubmitHandwriting(
		ctx,
		service.HandwritingSubmitRequest{
			UserID:     user.ID,
			SessionID:  req.SessionID,
			QuestionID: req.QuestionID,
			Strokes:    req.Strokes,
		},
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
		c.JSON(
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
	go h.refreshHandwritingMessage(
		context.WithoutCancel(ctx),
		req.SessionID,
		req.QuestionID,
	)

	c.JSON(
		http.StatusOK,
		result,
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
			service.ErrEmptyStrokes,
		):
		return publicError{status: http.StatusBadRequest, message: "손글씨 제출 정보를 확인할 수 없습니다."}
	case errors.Is(
		err,
		service.ErrHandwritingAlreadyAnswered,
	):
		return publicError{status: http.StatusConflict, message: "이미 채점된 문항입니다."}
	case errors.Is(
		err,
		service.ErrAIUnavailable,
	):
		return publicError{status: http.StatusServiceUnavailable, message: "현재 AI 채점 설정을 사용할 수 없습니다."}
	default:
		return publicError{status: http.StatusServiceUnavailable, message: "현재 AI 채점이 지연되고 있습니다. 잠시 후 다시 시도해 주세요."}
	}
}

func (h *Handler) refreshHandwritingMessage(
	parent context.Context,
	sessionID,
	questionID int,
) {
	parent = observability.WithAttrs(
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
	if h.handwritingScreen == nil {
		slog.WarnContext(
			parent,
			"Handwriting cleanup skipped because dependency is missing",
			"event",
			"handwriting.cleanup.skipped",
		)
		return
	}

	ctx, cancel := context.WithTimeout(
		parent,
		15*time.Second,
	)
	defer cancel()

	h.handwritingScreen.ShowHandwritingGraded(
		ctx,
		sessionID,
		questionID,
	)
}
