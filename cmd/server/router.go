package main

import (
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/jmoiron/sqlx"
	"github.com/redis/go-redis/v9"

	"github.com/lsj/copylingo/internal/miniapp"
)

// setupRouter forces gin release mode: debug only adds route dumps and requestLoggingMiddleware logs requests.
func setupRouter(
	health gin.HandlerFunc,
	miniappHandler *miniapp.Handler,
) *gin.Engine {
	gin.SetMode(gin.ReleaseMode)

	router := gin.New()
	router.Use(
		requestLoggingMiddleware(),
		structuredRecoveryMiddleware(),
	)

	router.GET(
		"/health",
		health,
	)
	miniapp.RegisterRoutes(
		router,
		miniappHandler,
	)

	return router
}

func healthHandler(
	db *sqlx.DB,
	redisClient *redis.Client,
) gin.HandlerFunc {
	return func(ginContext *gin.Context) {
		if err := db.Ping(); err != nil {
			ginContext.JSON(
				http.StatusServiceUnavailable,
				gin.H{
					"status": "unhealthy",
					"error":  "database connection failed",
				},
			)
			return
		}

		if err := redisClient.Ping(ginContext.Request.Context()).Err(); err != nil {
			ginContext.JSON(
				http.StatusServiceUnavailable,
				gin.H{
					"status": "unhealthy",
					"error":  "redis connection failed",
				},
			)
			return
		}

		ginContext.JSON(
			http.StatusOK,
			gin.H{
				"status": "healthy",
				"time":   time.Now().Format(time.RFC3339),
			},
		)
	}
}
