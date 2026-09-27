package redisstore

import (
	"context"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/redis/go-redis/v9"

	"github.com/lsj/copylingo/internal/model"
)

const (
	handwritingMessageKey = "handwriting:msg:%d:%d"
	miniAppFingerprintKey = "copylingo:miniapp:last_fingerprint:%d"

	handwritingMessageTTL = time.Hour
	miniAppFingerprintTTL = 24 * time.Hour
)

// SaveHandwritingMessage records which Telegram message to update after submission.
func (s *Interactions) SaveHandwritingMessage(
	ctx context.Context,
	sessionID, questionID int,
	ref model.TelegramMessageRef,
) error {
	value := fmt.Sprintf("%d:%d", ref.ChatID, ref.MessageID)
	return s.rdb.Set(ctx, fmt.Sprintf(handwritingMessageKey, sessionID, questionID), value, handwritingMessageTTL).Err()
}

func (s *Interactions) GetHandwritingMessage(
	ctx context.Context,
	sessionID, questionID int,
) (*model.TelegramMessageRef, error) {
	key := fmt.Sprintf(handwritingMessageKey, sessionID, questionID)
	raw, err := s.rdb.Get(ctx, key).Result()
	if err == redis.Nil {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("get handwriting message session_id=%d question_id=%d: %w", sessionID, questionID, err)
	}
	ref, err := parseHandwritingMessageRef(raw)
	if err != nil {
		return nil, fmt.Errorf("%w: %v", model.ErrInvalidTelegramMessageRef, err)
	}
	return &ref, nil
}

func (s *Interactions) GetMiniAppFingerprint(ctx context.Context, sessionID int) (string, error) {
	fingerprint, err := s.rdb.Get(ctx, fmt.Sprintf(miniAppFingerprintKey, sessionID)).Result()
	if err == redis.Nil {
		return "", nil
	}
	if err != nil {
		return "", fmt.Errorf("get Mini App fingerprint session_id=%d: %w", sessionID, err)
	}
	return fingerprint, nil
}

func (s *Interactions) SetMiniAppFingerprint(ctx context.Context, sessionID int, fingerprint string) error {
	return s.rdb.Set(ctx, fmt.Sprintf(miniAppFingerprintKey, sessionID), fingerprint, miniAppFingerprintTTL).Err()
}

func parseHandwritingMessageRef(raw string) (model.TelegramMessageRef, error) {
	parts := strings.Split(raw, ":")
	if len(parts) != 2 {
		return model.TelegramMessageRef{}, fmt.Errorf("expected chat_id:message_id")
	}
	chatID, err := strconv.ParseInt(parts[0], 10, 64)
	if err != nil {
		return model.TelegramMessageRef{}, fmt.Errorf("parse chat_id: %w", err)
	}
	if chatID == 0 {
		return model.TelegramMessageRef{}, fmt.Errorf("chat_id is zero")
	}
	messageID, err := strconv.Atoi(parts[1])
	if err != nil {
		return model.TelegramMessageRef{}, fmt.Errorf("parse message_id: %w", err)
	}
	if messageID <= 0 {
		return model.TelegramMessageRef{}, fmt.Errorf("message_id must be positive")
	}
	return model.TelegramMessageRef{ChatID: chatID, MessageID: messageID}, nil
}
