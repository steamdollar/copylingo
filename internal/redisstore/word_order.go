package redisstore

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/redis/go-redis/v9"
)

const (
	wordOrderDraftKey = "session:%d:word_order:%d:draft"
	wordOrderDraftTTL = 24 * time.Hour
)

// GetWordOrderDraft restores the selected word indices for an unfinished answer.
func (s *Interactions) GetWordOrderDraft(ctx context.Context, sessionID, questionID int) ([]int, error) {
	key := fmt.Sprintf(wordOrderDraftKey, sessionID, questionID)
	raw, err := s.rdb.Get(ctx, key).Result()
	if err == redis.Nil {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("get word-order draft session_id=%d question_id=%d: %w", sessionID, questionID, err)
	}
	var selection []int
	if err := json.Unmarshal([]byte(raw), &selection); err != nil {
		// Corrupt drafts are disposable, so a read can clean them without blocking the question.
		_ = s.rdb.Del(ctx, key).Err()
		return nil, nil
	}
	return selection, nil
}

func (s *Interactions) SetWordOrderDraft(ctx context.Context, sessionID, questionID int, selection []int) error {
	raw, err := json.Marshal(selection)
	if err != nil {
		return err
	}
	return s.rdb.Set(ctx, fmt.Sprintf(wordOrderDraftKey, sessionID, questionID), raw, wordOrderDraftTTL).Err()
}

func (s *Interactions) DeleteWordOrderDraft(ctx context.Context, sessionID, questionID int) error {
	return s.rdb.Del(ctx, fmt.Sprintf(wordOrderDraftKey, sessionID, questionID)).Err()
}
