package redisstore

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/redis/go-redis/v9"

	"github.com/lsj/copylingo/internal/model"
)

const sessionWorkingSetTTL = 24 * time.Hour

type sessionRedis interface {
	Get(
		ctx context.Context,
		key string,
	) *redis.StringCmd
	Set(
		ctx context.Context,
		key string,
		value any,
		expiration time.Duration,
	) *redis.StatusCmd
	Del(
		ctx context.Context,
		keys ...string,
	) *redis.IntCmd
}

// SessionStore persists typed Quiz and Study session state; use NewQuizSessions or NewStudySessions.
type SessionStore[T any] struct {
	redisClient sessionRedis
	key         func(sessionID int) string
	name        string
	validate    func(
		state *T,
		sessionID int,
	) bool
}

func newSessionStore[T any](
	redisClient sessionRedis,
	key func(int) string,
	name string,
	validate func(
		*T,
		int,
	) bool,
) *SessionStore[T] {
	return &SessionStore[T]{redisClient: redisClient, key: key, name: name, validate: validate}
}

func (store *SessionStore[T]) Load(
	ctx context.Context,
	sessionID int,
) (*T, error) {
	raw, err := store.redisClient.Get(
		ctx,
		store.key(sessionID),
	).Result()
	if errors.Is(
		err,
		redis.Nil,
	) {
		return nil, fmt.Errorf(
			"%w session_id=%d",
			model.ErrSessionStoreNotFound,
			sessionID,
		)
	}
	if err != nil {
		return nil, fmt.Errorf(
			"load %s session state session_id=%d: %w",
			store.name,
			sessionID,
			err,
		)
	}

	var state T
	if err := json.Unmarshal(
		[]byte(raw),
		&state,
	); err != nil {
		return nil, store.deleteCorrupt(
			ctx,
			sessionID,
			err,
		)
	}
	if store.validate != nil && !store.validate(
		&state,
		sessionID,
	) {
		return nil, store.deleteCorrupt(
			ctx,
			sessionID,
			nil,
		)
	}
	return &state, nil
}

func (store *SessionStore[T]) deleteCorrupt(
	ctx context.Context,
	sessionID int,
	cause error,
) error {
	corruptErr := fmt.Errorf(
		"%w session_id=%d",
		model.ErrSessionStoreCorrupt,
		sessionID,
	)
	if cause != nil {
		corruptErr = fmt.Errorf(
			"%w: %v",
			corruptErr,
			cause,
		)
	}
	if deleteErr := store.Delete(
		ctx,
		sessionID,
	); deleteErr != nil {
		return errors.Join(
			corruptErr,
			deleteErr,
		)
	}
	return corruptErr
}

// Save writes a JSON snapshot and renews the existing session TTL.
func (store *SessionStore[T]) Save(
	ctx context.Context,
	sessionID int,
	state *T,
) error {
	raw, err := json.Marshal(state)
	if err != nil {
		return fmt.Errorf(
			"marshal %s session state session_id=%d: %w",
			store.name,
			sessionID,
			err,
		)
	}
	if err := store.redisClient.Set(
		ctx,
		store.key(sessionID),
		raw,
		sessionWorkingSetTTL,
	).Err(); err != nil {
		return fmt.Errorf(
			"save %s session state session_id=%d: %w",
			store.name,
			sessionID,
			err,
		)
	}
	return nil
}

func (store *SessionStore[T]) Delete(
	ctx context.Context,
	sessionID int,
) error {
	if err := store.redisClient.Del(
		ctx,
		store.key(sessionID),
	).Err(); err != nil {
		return fmt.Errorf(
			"delete %s session state session_id=%d: %w",
			store.name,
			sessionID,
			err,
		)
	}
	return nil
}
