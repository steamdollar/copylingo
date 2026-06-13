package service

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/redis/go-redis/v9"
)

type workingSetRedis interface {
	Get(ctx context.Context, key string) *redis.StringCmd
	Set(ctx context.Context, key string, value any, expiration time.Duration) *redis.StatusCmd
	Del(ctx context.Context, keys ...string) *redis.IntCmd
}

type workingSetErrors struct {
	DependencyMissing error
	NotFound          error
	Corrupt           error
}

type workingSetStore[T any] struct {
	rdb      workingSetRedis
	key      func(sessionID int) string
	ttl      time.Duration
	validate func(state *T, sessionID int) error
	errs     workingSetErrors
}

func newWorkingSetStore[T any](
	rdb workingSetRedis,
	key func(sessionID int) string,
	ttl time.Duration,
	validate func(state *T, sessionID int) error,
	errs workingSetErrors,
) *workingSetStore[T] {
	return &workingSetStore[T]{
		rdb:      rdb,
		key:      key,
		ttl:      ttl,
		validate: validate,
		errs:     errs,
	}
}

// get: get session (study | quiz) from redis
func (s *workingSetStore[T]) get(ctx context.Context, sessionID int) (*T, error) {
	if s.rdb == nil {
		return nil, s.errs.DependencyMissing
	}

	key := s.key(sessionID)
	raw, err := s.rdb.Get(ctx, key).Result()
	if err != nil {
		if errors.Is(err, redis.Nil) {
			return nil, fmt.Errorf("%w session_id=%d", s.errs.NotFound, sessionID)
		}
		return nil, fmt.Errorf("get working set session_id=%d: %w", sessionID, err)
	}

	var state T
	if err := json.Unmarshal([]byte(raw), &state); err != nil {
		corruptErr := fmt.Errorf("%w session_id=%d: %v", s.errs.Corrupt, sessionID, err)
		if deleteErr := s.delete(ctx, sessionID); deleteErr != nil {
			return nil, errors.Join(corruptErr, deleteErr)
		}
		return nil, corruptErr
	}
	if s.validate != nil {
		if err := s.validate(&state, sessionID); err != nil {
			corruptErr := fmt.Errorf("%w session_id=%d: %v", s.errs.Corrupt, sessionID, err)
			if deleteErr := s.delete(ctx, sessionID); deleteErr != nil {
				return nil, errors.Join(corruptErr, deleteErr)
			}
			return nil, corruptErr
		}
	}

	return &state, nil
}

func (s *workingSetStore[T]) save(ctx context.Context, sessionID int, state *T) error {
	if s.rdb == nil {
		return s.errs.DependencyMissing
	}
	raw, err := json.Marshal(state)
	if err != nil {
		return fmt.Errorf("marshal working set session_id=%d: %w", sessionID, err)
	}
	if err := s.rdb.Set(ctx, s.key(sessionID), raw, s.ttl).Err(); err != nil {
		return fmt.Errorf("set working set session_id=%d: %w", sessionID, err)
	}
	return nil
}

func (s *workingSetStore[T]) delete(ctx context.Context, sessionID int) error {
	if s.rdb == nil {
		return s.errs.DependencyMissing
	}
	if err := s.rdb.Del(ctx, s.key(sessionID)).Err(); err != nil {
		return fmt.Errorf("delete working set session_id=%d: %w", sessionID, err)
	}
	return nil
}
