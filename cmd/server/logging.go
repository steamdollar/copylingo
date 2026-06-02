package main

import (
	"fmt"
	"log/slog"
	"net/http"
	"runtime/debug"
	"strings"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/lsj/copylingo/internal/observability"
)

func requestLoggingMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		startedAt := time.Now()
		interactionID := observability.NewInteractionID("http")
		ctx := observability.WithAttrs(c.Request.Context(),
			slog.String("interaction_id", interactionID),
			slog.String("source", "http"),
		)
		c.Request = c.Request.WithContext(ctx)
		c.Header("X-Request-ID", interactionID)

		c.Next()

		if !shouldLogHTTPRequest(c.Request.URL.Path) {
			return
		}
		slog.InfoContext(ctx, "HTTP request completed",
			"event", "http.completed",
			"method", c.Request.Method,
			"path", matchedRoutePath(c),
			"status", c.Writer.Status(),
			"duration_ms", time.Since(startedAt).Milliseconds(),
		)
	}
}

func structuredRecoveryMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		defer func() {
			if recovered := recover(); recovered != nil {
				slog.ErrorContext(c.Request.Context(), "HTTP panic recovered",
					"event", "http.panic",
					"error", fmt.Sprint(recovered),
					"stack", string(debug.Stack()),
				)
				c.AbortWithStatus(http.StatusInternalServerError)
			}
		}()
		c.Next()
	}
}

func shouldLogHTTPRequest(path string) bool {
	return strings.HasPrefix(path, "/api/") || path == "/miniapp/handwriting"
}

func matchedRoutePath(c *gin.Context) string {
	if path := c.FullPath(); path != "" {
		return path
	}
	return c.Request.URL.Path
}
