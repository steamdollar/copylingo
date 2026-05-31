package miniapp

import (
	"context"
	"errors"
	"log"
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

	tips, err := h.tip.ListActive(c.Request.Context(), language, level, limit)
	if err != nil {
		log.Printf("[Tips] list failed language=%s level=%s: %v", language, level, err)
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

	result, err := h.handwriting.SubmitAnswer(c.Request.Context(), service.HandwritingSubmitRequest{
		UserID:     user.ID,
		SessionID:  req.SessionID,
		QuestionID: req.QuestionID,
		Strokes:    req.Strokes,
	})
	if err != nil {
		publicErr := handwritingPublicError(err)
		log.Printf("[Handwriting] submit failed status=%d session_id=%d question_id=%d: %v", publicErr.status, req.SessionID, req.QuestionID, err)
		c.JSON(publicErr.status, gin.H{"error": publicErr.message})
		return
	}

	log.Printf("[Handwriting] submit total=%s session_id=%d question_id=%d is_correct=%t", time.Since(startedAt), req.SessionID, req.QuestionID, result.IsCorrect)

	// Refresh Telegram message buttons in background
	go h.refreshHandwritingMessage(req.SessionID, req.QuestionID)

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

func (h *Handler) refreshHandwritingMessage(sessionID, questionID int) {
	if h.rdb == nil || h.messenger == nil || h.cfg == nil {
		log.Printf("[Handwriting] cleanup skipped: dependency missing session=%d question=%d", sessionID, questionID)
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	key := config.HandwritingMessageRedisKey.Format(sessionID, questionID)
	val, err := h.rdb.Get(ctx, key).Result()
	if err != nil {
		if !errors.Is(err, redis.Nil) {
			log.Printf("[Handwriting] failed to get message id from redis session=%d question=%d: %v", sessionID, questionID, err)
		}
		return
	}

	chatID, msgID, err := bot.ParseHandwritingMessageRef(val)
	if err != nil {
		log.Printf("[Handwriting] invalid message id format in redis value=%q: %v", val, err)
		return
	}

	// We need to know the question index to format the "Next" button.
	state, err := h.activeSession.Get(ctx, sessionID)
	if err != nil {
		log.Printf("[Handwriting] failed to get active session state for cleanup session=%d: %v", sessionID, err)
		return
	}

	_, questionIdx, ok := state.CurrentItemByQuestionID(questionID)
	if !ok {
		log.Printf("[Handwriting] question not found in session for cleanup session=%d question=%d", sessionID, questionID)
		return
	}

	nextData := callback.FormatHandwritingNext(sessionID, questionIdx, h.cfg.Server.PublicBaseURL)

	markup := tgbotapi.NewInlineKeyboardMarkup(
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData("다음 문제 →", nextData),
		),
	)

	if err := h.messenger.EditMessageReplyMarkup(chatID, msgID, markup); err != nil {
		log.Printf("[Handwriting] failed to edit message reply markup chat=%d msg=%d: %v", chatID, msgID, err)
	}
}
