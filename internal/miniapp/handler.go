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
	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
	"github.com/redis/go-redis/v9"

	"github.com/lsj/copylingo/internal/bot"
	"github.com/lsj/copylingo/internal/callback"
	"github.com/lsj/copylingo/internal/config"
	"github.com/lsj/copylingo/internal/model"
	"github.com/lsj/copylingo/internal/observability"
	"github.com/lsj/copylingo/internal/service"
)

type TelegramMessenger interface {
	EditMessageReplyMarkup(chatID int64, messageID int, markup tgbotapi.InlineKeyboardMarkup) error
}

type handwritingService interface {
	SubmitAnswer(ctx context.Context, req service.HandwritingSubmitRequest) (*service.HandwritingSubmitResult, error)
}

type tipService interface {
	ListActive(ctx context.Context, language, level string, limit int) ([]model.Tip, error)
}

type activeSessionService interface {
	Get(ctx context.Context, sessionID int) (*model.ActiveSessionState, error)
}

type verifier interface {
	Verify(initData string) (*TelegramUser, error)
}

type Handler struct {
	handwriting   handwritingService
	tip           tipService
	activeSession activeSessionService
	verifier      verifier
	rdb           *redis.Client
	messenger     TelegramMessenger
	cfg           *config.Config
}

type HandlerDeps struct {
	Handwriting   handwritingService
	Tip           tipService
	ActiveSession activeSessionService
	Verifier      verifier
	Redis         *redis.Client
	Messenger     TelegramMessenger
	Config        *config.Config
}

type handwritingSubmitRequest struct {
	InitData   string           `json:"init_data" binding:"required"`
	SessionID  int              `json:"session_id" binding:"required"`
	QuestionID int              `json:"question_id" binding:"required"`
	Strokes    []service.Stroke `json:"strokes" binding:"required"`
}

func NewHandler(deps HandlerDeps) *Handler {
	return &Handler{
		handwriting:   deps.Handwriting,
		tip:           deps.Tip,
		activeSession: deps.ActiveSession,
		verifier:      deps.Verifier,
		rdb:           deps.Redis,
		messenger:     deps.Messenger,
		cfg:           deps.Config,
	}
}

func RegisterRoutes(r *gin.Engine, cfg *config.Config, services *service.Services, rdb *redis.Client, messenger TelegramMessenger) {
	handler := NewHandler(HandlerDeps{
		Handwriting:   services.Handwriting,
		Tip:           services.Tip,
		ActiveSession: services.ActiveSession,
		Verifier:      NewInitDataVerifier(cfg.Telegram.Token, 24*time.Hour),
		Redis:         rdb,
		Messenger:     messenger,
		Config:        cfg,
	})

	r.Static("/miniapp/handwriting/assets", "./web/miniapp/handwriting")
	r.GET(config.PathHandwritingMiniApp, func(c *gin.Context) {
		c.File("./web/miniapp/handwriting/index.html")
	})
	r.POST(config.PathHandwritingSubmit, handler.SubmitHandwriting)
	r.GET(config.PathMiniAppTips, handler.ListTips)
}

func (h *Handler) ListTips(c *gin.Context) {
	ctx := observability.WithAttrs(c.Request.Context(), slog.String("source", "miniapp.tips"))
	language := strings.TrimSpace(c.Query("language"))
	level := strings.TrimSpace(c.Query("level"))
	if language == "" || level == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "language and level required"})
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

	tips, err := h.tip.ListActive(ctx, language, level, limit)
	if err != nil {
		slog.ErrorContext(ctx, "Failed to list tips",
			"event", "miniapp.tips.list_failed",
			"language", language,
			"level", level,
			"error", err,
		)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to load tips"})
		return
	}
	if tips == nil {
		tips = []model.Tip{}
	}

	// model.Tip 의 json 태그가 source_* / is_active / created_at 를 "-" 로 두었으므로 그대로 직렬화하면 public 필드만 나간다.
	c.JSON(http.StatusOK, tips)
}

func (h *Handler) SubmitHandwriting(c *gin.Context) {
	startedAt := time.Now()
	ctx := observability.WithAttrs(c.Request.Context(), slog.String("source", "miniapp.handwriting"))

	var req handwritingSubmitRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request"})
		return
	}

	user, err := h.verifier.Verify(req.InitData)
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "invalid telegram init data"})
		return
	}
	ctx = observability.WithAttrs(ctx,
		slog.Int64("user_id", user.ID),
		slog.Int("session_id", req.SessionID),
		slog.Int("question_id", req.QuestionID),
	)

	result, err := h.handwriting.SubmitAnswer(ctx, service.HandwritingSubmitRequest{
		UserID:     user.ID,
		SessionID:  req.SessionID,
		QuestionID: req.QuestionID,
		Strokes:    req.Strokes,
	})
	if err != nil {
		publicErr := handwritingPublicError(err)
		slog.ErrorContext(ctx, "Handwriting submission failed",
			"event", "handwriting.submit.failed",
			"status", publicErr.status,
			"error", err,
		)
		c.JSON(publicErr.status, gin.H{"error": publicErr.message})
		return
	}

	slog.InfoContext(ctx, "Handwriting submission completed",
		"event", "handwriting.submit.completed",
		"duration_ms", time.Since(startedAt).Milliseconds(),
		"is_correct", result.IsCorrect,
	)

	// Refresh Telegram message buttons in background
	go h.refreshHandwritingMessage(context.WithoutCancel(ctx), req.SessionID, req.QuestionID)

	c.JSON(http.StatusOK, result)
}

type publicError struct {
	status  int
	message string
}

func handwritingPublicError(err error) publicError {
	switch {
	case errors.Is(err, service.ErrHandwritingUnauthorized):
		return publicError{status: http.StatusForbidden, message: "권한이 없습니다."}
	case errors.Is(err, service.ErrHandwritingQuestionMismatch),
		errors.Is(err, service.ErrHandwritingInvalidQuestion),
		errors.Is(err, service.ErrEmptyStrokes):
		return publicError{status: http.StatusBadRequest, message: "손글씨 제출 정보를 확인할 수 없습니다."}
	case errors.Is(err, service.ErrHandwritingAlreadyAnswered):
		return publicError{status: http.StatusConflict, message: "이미 채점된 문항입니다."}
	case errors.Is(err, service.ErrAIUnavailable):
		return publicError{status: http.StatusServiceUnavailable, message: "현재 AI 채점 설정을 사용할 수 없습니다."}
	default:
		return publicError{status: http.StatusServiceUnavailable, message: "현재 AI 채점이 지연되고 있습니다. 잠시 후 다시 시도해 주세요."}
	}
}

func (h *Handler) refreshHandwritingMessage(parent context.Context, sessionID, questionID int) {
	parent = observability.WithAttrs(parent,
		slog.String("source", "miniapp.handwriting.cleanup"),
		slog.Int("session_id", sessionID),
		slog.Int("question_id", questionID),
	)
	if h.rdb == nil || h.messenger == nil || h.cfg == nil {
		slog.WarnContext(parent, "Handwriting cleanup skipped because dependency is missing",
			"event", "handwriting.cleanup.skipped",
		)
		return
	}

	ctx, cancel := context.WithTimeout(parent, 15*time.Second)
	defer cancel()

	key := config.HandwritingMessageRedisKey.Format(sessionID, questionID)
	val, err := h.rdb.Get(ctx, key).Result()
	if err != nil {
		if !errors.Is(err, redis.Nil) {
			slog.ErrorContext(ctx, "Failed to get handwriting message ID",
				"event", "handwriting.cleanup.message_lookup_failed",
				"error", err,
			)
		}
		return
	}

	chatID, msgID, err := bot.ParseHandwritingMessageRef(val)
	if err != nil {
		slog.ErrorContext(ctx, "Invalid handwriting message ID format",
			"event", "handwriting.cleanup.invalid_message_id",
			"error", err,
		)
		return
	}

	// We need to know the question index to format the "Next" button.
	state, err := h.activeSession.Get(ctx, sessionID)
	if err != nil {
		slog.ErrorContext(ctx, "Failed to get active session state for handwriting cleanup",
			"event", "handwriting.cleanup.session_lookup_failed",
			"error", err,
		)
		return
	}

	_, questionIdx, ok := state.CurrentItemByQuestionID(questionID)
	if !ok {
		slog.WarnContext(ctx, "Question not found in session for handwriting cleanup",
			"event", "handwriting.cleanup.question_not_found",
		)
		return
	}

	nextData := callback.FormatHandwritingNext(sessionID, questionIdx, h.cfg.Server.PublicBaseURL)

	markup := tgbotapi.NewInlineKeyboardMarkup(
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData("다음 문제 →", nextData),
		),
	)

	if err := h.messenger.EditMessageReplyMarkup(chatID, msgID, markup); err != nil {
		slog.ErrorContext(ctx, "Failed to edit handwriting message reply markup",
			"event", "handwriting.cleanup.reply_markup_failed",
			"chat_id", chatID,
			"message_id", msgID,
			"error", err,
		)
	}
}
