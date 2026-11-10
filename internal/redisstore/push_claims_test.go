package redisstore

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/redis/go-redis/v9"

	"github.com/lsj/copylingo/internal/model"
)

type pushClaimsRedisMock struct {
	redis.Cmdable
	values     map[string]struct{}
	key        string
	value      any
	expiration time.Duration
	calls      int
	err        error
}

func (mock *pushClaimsRedisMock) SetNX(
	_ context.Context,
	key string,
	value any,
	expiration time.Duration,
) *redis.BoolCmd {
	mock.calls++
	mock.key = key
	mock.value = value
	mock.expiration = expiration
	if mock.err != nil {
		return redis.NewBoolResult(
			false,
			mock.err,
		)
	}
	if _, exists := mock.values[key]; exists {
		return redis.NewBoolResult(
			false,
			nil,
		)
	}
	mock.values[key] = struct{}{}
	return redis.NewBoolResult(
		true,
		nil,
	)
}

func TestPushClaimsTryClaimUsesAtomicDailyLock(t *testing.T) {
	redisClient := &pushClaimsRedisMock{values: make(map[string]struct{})}
	claims := NewPushClaims(redisClient)

	for _, want := range []bool{true, false} {
		got, err := claims.TryClaim(
			context.Background(),
			42,
			model.SessionSlotMorningQuiz,
			"2026-09-26",
		)
		if err != nil {
			t.Fatalf(
				"TryClaim() error = %v",
				err,
			)
		}
		if got != want {
			t.Fatalf(
				"TryClaim() = %t, want %t",
				got,
				want,
			)
		}
	}

	if got, want := redisClient.key, "copylingo:push:lock:42:morning_quiz:2026-09-26"; got != want {
		t.Fatalf(
			"SetNX key = %q, want %q",
			got,
			want,
		)
	}
	if got, ok := redisClient.value.(string); !ok || got != "1" {
		t.Fatalf(
			"SetNX value = %v, want %q",
			redisClient.value,
			"1",
		)
	}
	if redisClient.expiration != 24*time.Hour {
		t.Fatalf(
			"SetNX expiration = %s, want 24h",
			redisClient.expiration,
		)
	}
	if redisClient.calls != 2 {
		t.Fatalf(
			"SetNX calls = %d, want 2",
			redisClient.calls,
		)
	}
}

func TestPushClaimsTryClaimReturnsRedisError(t *testing.T) {
	wantErr := errors.New("redis unavailable")
	claims := NewPushClaims(&pushClaimsRedisMock{err: wantErr})

	if _, err := claims.TryClaim(
		context.Background(),
		42,
		model.SessionSlotMorningQuiz,
		"2026-09-26",
	); !errors.Is(
		err,
		wantErr,
	) {
		t.Fatalf(
			"TryClaim() error = %v, want %v",
			err,
			wantErr,
		)
	}
}
