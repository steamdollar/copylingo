package redisstore

import (
	"context"
	"fmt"
	"time"

	"github.com/redis/go-redis/v9"

	"github.com/lsj/copylingo/internal/model"
)

// PushClaims stores the scheduler's per-user, per-slot daily dispatch claim.
type PushClaims struct {
	redisClient redis.Cmdable
}

// NewPushClaims adapts Redis to the scheduler push-claim contract.
func NewPushClaims(redisClient redis.Cmdable) *PushClaims {
	return &PushClaims{redisClient: redisClient}
}

// TryClaim creates the existing daily lock only when it does not already exist.
func (claims *PushClaims) TryClaim(
	ctx context.Context,
	userID int64,
	slot model.SessionSlot,
	today string,
) (bool, error) {
	key := fmt.Sprintf(
		"copylingo:push:lock:%d:%s:%s",
		userID,
		slot,
		today,
	)
	return claims.redisClient.SetNX(
		ctx,
		key,
		"1",
		24*time.Hour,
	).Result()
}
