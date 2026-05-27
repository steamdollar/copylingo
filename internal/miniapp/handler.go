package miniapp

import (
	"context"
	"errors"
	"fmt"
	"log"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
	"github.com/redis/go-redis/v9"

	"github.com/lsj/copylingo/internal/callback"
	"github.com/lsj/copylingo/internal/config"
	"github.com/lsj/copylingo/internal/model"
	"github.com/lsj/copylingo/internal/service"
)

type TelegramMessenger interface {
	EditMessageReplyMarkup(chatID int64, messageID int, markup tgbotapi.InlineKeyboardMarkup) error
}

type Handler struct {
	services  *service.Services
	verifier  *InitDataVerifier
	rdb       *redis.Client
	messenger TelegramMessenger
	cfg       *config.Config
}

type handwritingSubmitRequest struct {
	InitData   string           `json:"init_data" binding:"required"`
	SessionID  int              `json:"session_id" binding:"required"`
	QuestionID int              `json:"question_id" binding:"required"`
	Strokes    []service.Stroke `json:"strokes" binding:"required"`
}

func NewHandler(services *service.Services, verifier *InitDataVerifier, rdb *redis.Client, messenger TelegramMessenger, cfg *config.Config) *Handler {
	return &Handler{
		services:  services,
		verifier:  verifier,
		rdb:       rdb,
		messenger: messenger,
		cfg:       cfg,
	}
}

func RegisterRoutes(r *gin.Engine, cfg *config.Config, services *service.Services, rdb *redis.Client, messenger TelegramMessenger) {
	handler := NewHandler(services, NewInitDataVerifier(cfg.Telegram.Token, 24*time.Hour), rdb, messenger, cfg)

	r.Static("/miniapp/handwriting/assets", "./web/miniapp/handwriting")
	r.GET(config.PathHandwritingMiniApp, handler.ShowHandwriting)
	r.POST(config.PathHandwritingSubmit, handler.SubmitHandwriting)
	r.GET(config.PathMiniAppTips, handler.ListTips)
}

func (h *Handler) ShowHandwriting(c *gin.Context) {
	c.File("./web/miniapp/handwriting/index.html")
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

	tips, err := h.services.Tip.ListActive(c.Request.Context(), language, level, limit)
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

	result, err := h.services.Handwriting.SubmitAnswer(c.Request.Context(), service.HandwritingSubmitRequest{
		UserID:     user.ID,
		SessionID:  req.SessionID,
		QuestionID: req.QuestionID,
		Strokes:    req.Strokes,
	})
	if err != nil {
		status := http.StatusInternalServerError
		switch {
		case errors.Is(err, service.ErrHandwritingUnauthorized):
			status = http.StatusForbidden
		case errors.Is(err, service.ErrHandwritingQuestionMismatch),
			errors.Is(err, service.ErrHandwritingInvalidQuestion),
			errors.Is(err, service.ErrEmptyStrokes):
			status = http.StatusBadRequest
		case errors.Is(err, service.ErrHandwritingAlreadyAnswered):
			status = http.StatusConflict
		case errors.Is(err, config.ErrAIConfigMissing):
			status = http.StatusServiceUnavailable
		}
		c.JSON(status, gin.H{"error": err.Error()})
		return
	}

	log.Printf("[Handwriting] submit total=%s session_id=%d question_id=%d is_correct=%t", time.Since(startedAt), req.SessionID, req.QuestionID, result.IsCorrect)

	// Refresh Telegram message buttons in background
	go h.refreshHandwritingMessage(req.SessionID, req.QuestionID)

	c.JSON(http.StatusOK, result)
}

func (h *Handler) refreshHandwritingMessage(sessionID, questionID int) {
	if h.rdb == nil || h.messenger == nil || h.cfg == nil {
		log.Printf("[Handwriting] cleanup skipped: dependency missing session=%d question=%d", sessionID, questionID)
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	key := fmt.Sprintf(config.KeyHandwritingMessage, sessionID, questionID)
	val, err := h.rdb.Get(ctx, key).Result()
	if err != nil {
		if !errors.Is(err, redis.Nil) {
			log.Printf("[Handwriting] failed to get message id from redis session=%d question=%d: %v", sessionID, questionID, err)
		}
		return
	}

	chatID, msgID, err := parseHandwritingMessageRef(val)
	if err != nil {
		log.Printf("[Handwriting] invalid message id format in redis value=%q: %v", val, err)
		return
	}

	// We need to know the question index to format the "Next" button.
	sqs, err := h.services.SessionBuilder.GetSessionQuestions(ctx, sessionID)
	if err != nil {
		log.Printf("[Handwriting] failed to get session questions for cleanup session=%d: %v", sessionID, err)
		return
	}

	questionIdx := -1
	for i, sq := range sqs {
		if sq.QuestionID == questionID {
			questionIdx = i
			break
		}
	}

	if questionIdx == -1 {
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

func parseHandwritingMessageRef(raw string) (int64, int, error) {
	parts := strings.Split(raw, ":")
	if len(parts) != 2 {
		return 0, 0, fmt.Errorf("expected chat_id:message_id")
	}

	chatID, err := strconv.ParseInt(parts[0], 10, 64)
	if err != nil {
		return 0, 0, fmt.Errorf("parse chat_id: %w", err)
	}
	if chatID == 0 {
		return 0, 0, fmt.Errorf("chat_id is zero")
	}

	msgID, err := strconv.Atoi(parts[1])
	if err != nil {
		return 0, 0, fmt.Errorf("parse message_id: %w", err)
	}
	if msgID <= 0 {
		return 0, 0, fmt.Errorf("message_id must be positive")
	}

	return chatID, msgID, nil
}
