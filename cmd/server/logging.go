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
	return func(ginContext *gin.Context) {
		startedAt := time.Now()
		interactionID := observability.NewInteractionID("http")
		ctx := observability.WithAttributes(
			ginContext.Request.Context(),
			slog.String(
				"interaction_id",
				interactionID,
			),
			slog.String(
				"source",
				"http",
			),
		)
		ginContext.Request = ginContext.Request.WithContext(ctx)
		ginContext.Header(
			"X-Request-ID",
			interactionID,
		)

		ginContext.Next()

		if !shouldLogHTTPRequest(ginContext.Request.URL.Path) {
			return
		}
		slog.InfoContext(
			ctx,
			"HTTP request completed",
			"event",
			"http.completed",
			"method",
			ginContext.Request.Method,
			"path",
			matchedRoutePath(ginContext),
			"status",
			ginContext.Writer.Status(),
			"duration_ms",
			time.Since(startedAt).Milliseconds(),
		)
	}
}

func structuredRecoveryMiddleware() gin.HandlerFunc {
	return func(ginContext *gin.Context) {
		defer func() {
			if recovered := recover(); recovered != nil {
				slog.ErrorContext(
					ginContext.Request.Context(),
					"HTTP panic recovered",
					"event",
					"http.panic",
					"error",
					fmt.Sprint(recovered),
					"stack",
					string(debug.Stack()),
				)
				ginContext.AbortWithStatus(http.StatusInternalServerError)
			}
		}()
		ginContext.Next()
	}
}

func shouldLogHTTPRequest(path string) bool {
	return strings.HasPrefix(
		path,
		"/api/",
	) || path == "/miniapp/handwriting"
}

func matchedRoutePath(ginContext *gin.Context) string {
	if path := ginContext.FullPath(); path != "" {
		return path
	}
	return ginContext.Request.URL.Path
}
