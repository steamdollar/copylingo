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

// SessionStore directly implements typed session persistence for Quiz and Study.
// Use NewQuizSessions or NewStudySessions to configure its state type and key.
// The zero value is not ready for use.
type SessionStore[T any] struct {
	rdb      sessionRedis
	key      func(sessionID int) string
	name     string
	validate func(
		state *T,
		sessionID int,
	) bool
}

func newSessionStore[T any](
	rdb sessionRedis,
	key func(int) string,
	name string,
	validate func(
		*T,
		int,
	) bool,
) *SessionStore[T] {
	return &SessionStore[T]{rdb: rdb, key: key, name: name, validate: validate}
}

// Load rejects malformed JSON and state that fails the configured validator.
func (s *SessionStore[T]) Load(
	ctx context.Context,
	sessionID int,
) (*T, error) {
	raw, err := s.rdb.Get(
		ctx,
		s.key(sessionID),
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
			s.name,
			sessionID,
			err,
		)
	}

	var state T
	if err := json.Unmarshal(
		[]byte(raw),
		&state,
	); err != nil {
		return nil, s.deleteCorrupt(
			ctx,
			sessionID,
			err,
		)
	}
	if s.validate != nil && !s.validate(
		&state,
		sessionID,
	) {
		return nil, s.deleteCorrupt(
			ctx,
			sessionID,
			nil,
		)
	}
	return &state, nil
}

func (s *SessionStore[T]) deleteCorrupt(
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
	if deleteErr := s.Delete(
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
func (s *SessionStore[T]) Save(
	ctx context.Context,
	sessionID int,
	state *T,
) error {
	raw, err := json.Marshal(state)
	if err != nil {
		return fmt.Errorf(
			"marshal %s session state session_id=%d: %w",
			s.name,
			sessionID,
			err,
		)
	}
	if err := s.rdb.Set(
		ctx,
		s.key(sessionID),
		raw,
		sessionWorkingSetTTL,
	).Err(); err != nil {
		return fmt.Errorf(
			"save %s session state session_id=%d: %w",
			s.name,
			sessionID,
			err,
		)
	}
	return nil
}

// Delete removes working state after completion or corruption handling.
func (s *SessionStore[T]) Delete(
	ctx context.Context,
	sessionID int,
) error {
	if err := s.rdb.Del(
		ctx,
		s.key(sessionID),
	).Err(); err != nil {
		return fmt.Errorf(
			"delete %s session state session_id=%d: %w",
			s.name,
			sessionID,
			err,
		)
	}
	return nil
}
