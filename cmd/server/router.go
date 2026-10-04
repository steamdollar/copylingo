package main

import (
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/jmoiron/sqlx"
	"github.com/redis/go-redis/v9"

	"github.com/lsj/copylingo/internal/miniapp"
)

// setupRouter registers the prebuilt health probe and Mini App routes.
func setupRouter(
	releaseMode bool,
	health gin.HandlerFunc,
	miniappHandler *miniapp.Handler,
) *gin.Engine {
	if releaseMode {
		gin.SetMode(gin.ReleaseMode)
	}

	r := gin.New()
	r.Use(
		requestLoggingMiddleware(),
		structuredRecoveryMiddleware(),
	)

	r.GET(
		"/health",
		health,
	)
	miniapp.RegisterRoutes(
		r,
		miniappHandler,
	)

	return r
}

// healthHandler reports unhealthy when the DB or Redis connection fails a ping.
func healthHandler(
	db *sqlx.DB,
	rdb *redis.Client,
) gin.HandlerFunc {
	return func(c *gin.Context) {
		// Check DB
		if err := db.Ping(); err != nil {
			c.JSON(
				http.StatusServiceUnavailable,
				gin.H{
					"status": "unhealthy",
					"error":  "database connection failed",
				},
			)
			return
		}

		// Check Redis
		if err := rdb.Ping(c.Request.Context()).Err(); err != nil {
			c.JSON(
				http.StatusServiceUnavailable,
				gin.H{
					"status": "unhealthy",
					"error":  "redis connection failed",
				},
			)
			return
		}

		c.JSON(
			http.StatusOK,
			gin.H{
				"status": "healthy",
				"time":   time.Now().Format(time.RFC3339),
			},
		)
	}
}
