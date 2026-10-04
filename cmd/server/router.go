package main

import (
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/jmoiron/sqlx"
	"github.com/redis/go-redis/v9"

	"github.com/lsj/copylingo/internal/config"
	"github.com/lsj/copylingo/internal/miniapp"
)

// setupRouter registers the infrastructure health probe and Mini App routes.
func setupRouter(
	cfg *config.Config,
	db *sqlx.DB,
	rdb *redis.Client,
	miniappHandler *miniapp.Handler,
) *gin.Engine {
	if cfg.Server.Mode == "release" {
		gin.SetMode(gin.ReleaseMode)
	}

	r := gin.New()
	r.Use(
		requestLoggingMiddleware(),
		structuredRecoveryMiddleware(),
	)

	// Health check
	r.GET(
		"/health",
		func(c *gin.Context) {
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
		},
	)

	miniapp.RegisterRoutes(
		r,
		miniappHandler,
	)

	return r
}
