package redisstore

import (
	"context"
	"fmt"
	"time"

	"github.com/redis/go-redis/v9"

	"github.com/lsj/copylingo/internal/model"
)

type PushClaims struct {
	redisClient redis.Cmdable
}

func NewPushClaims(redisClient redis.Cmdable) *PushClaims {
	return &PushClaims{redisClient: redisClient}
}

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
