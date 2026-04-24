package miniapp

import (
	"errors"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/lsj/copylingo/internal/config"
	"github.com/lsj/copylingo/internal/service"
)

type Handler struct {
	services *service.Services
	verifier *InitDataVerifier
}

type handwritingSubmitRequest struct {
	InitData   string           `json:"init_data" binding:"required"`
	SessionID  int              `json:"session_id" binding:"required"`
	QuestionID int              `json:"question_id" binding:"required"`
	Strokes    []service.Stroke `json:"strokes" binding:"required"`
}

func NewHandler(services *service.Services, verifier *InitDataVerifier) *Handler {
	return &Handler{services: services, verifier: verifier}
}

func RegisterRoutes(r *gin.Engine, cfg *config.Config, services *service.Services) {
	handler := NewHandler(services, NewInitDataVerifier(cfg.Telegram.Token, 24*time.Hour))

	r.Static("/miniapp/handwriting/assets", "./web/miniapp/handwriting")
	r.GET(config.PathHandwritingMiniApp, handler.ShowHandwriting)
	r.POST(config.PathHandwritingSubmit, handler.SubmitHandwriting)
}

func (h *Handler) ShowHandwriting(c *gin.Context) {
	c.File("./web/miniapp/handwriting/index.html")
}

func (h *Handler) SubmitHandwriting(c *gin.Context) {
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

	c.JSON(http.StatusOK, result)
}
