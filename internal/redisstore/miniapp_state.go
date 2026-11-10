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
func (interactions *Interactions) SaveHandwritingMessage(
	ctx context.Context,
	sessionID,
	questionID int,
	messageReference model.TelegramMessageReference,
) error {
	value := fmt.Sprintf(
		"%d:%d",
		messageReference.ChatID,
		messageReference.MessageID,
	)
	return interactions.redisClient.Set(
		ctx,
		fmt.Sprintf(
			handwritingMessageKey,
			sessionID,
			questionID,
		),
		value,
		handwritingMessageTTL,
	).Err()
}

func (interactions *Interactions) GetHandwritingMessage(
	ctx context.Context,
	sessionID,
	questionID int,
) (*model.TelegramMessageReference, error) {
	key := fmt.Sprintf(
		handwritingMessageKey,
		sessionID,
		questionID,
	)
	raw, err := interactions.redisClient.Get(
		ctx,
		key,
	).Result()
	if err == redis.Nil {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf(
			"get handwriting message session_id=%d question_id=%d: %w",
			sessionID,
			questionID,
			err,
		)
	}
	messageReference, err := parseHandwritingMessageReference(raw)
	if err != nil {
		return nil, fmt.Errorf(
			"%w: %v",
			model.ErrInvalidTelegramMessageReference,
			err,
		)
	}
	return &messageReference, nil
}

func (interactions *Interactions) GetMiniAppFingerprint(
	ctx context.Context,
	sessionID int,
) (string, error) {
	fingerprint, err := interactions.redisClient.Get(
		ctx,
		fmt.Sprintf(
			miniAppFingerprintKey,
			sessionID,
		),
	).Result()
	if err == redis.Nil {
		return "", nil
	}
	if err != nil {
		return "", fmt.Errorf(
			"get Mini App fingerprint session_id=%d: %w",
			sessionID,
			err,
		)
	}
	return fingerprint, nil
}

func (interactions *Interactions) SetMiniAppFingerprint(
	ctx context.Context,
	sessionID int,
	fingerprint string,
) error {
	return interactions.redisClient.Set(
		ctx,
		fmt.Sprintf(
			miniAppFingerprintKey,
			sessionID,
		),
		fingerprint,
		miniAppFingerprintTTL,
	).Err()
}

func parseHandwritingMessageReference(raw string) (model.TelegramMessageReference, error) {
	parts := strings.Split(
		raw,
		":",
	)
	if len(parts) != 2 {
		return model.TelegramMessageReference{}, fmt.Errorf("expected chat_id:message_id")
	}
	chatID, err := strconv.ParseInt(
		parts[0],
		10,
		64,
	)
	if err != nil {
		return model.TelegramMessageReference{}, fmt.Errorf(
			"parse chat_id: %w",
			err,
		)
	}
	if chatID == 0 {
		return model.TelegramMessageReference{}, fmt.Errorf("chat_id is zero")
	}
	messageID, err := strconv.Atoi(parts[1])
	if err != nil {
		return model.TelegramMessageReference{}, fmt.Errorf(
			"parse message_id: %w",
			err,
		)
	}
	if messageID <= 0 {
		return model.TelegramMessageReference{}, fmt.Errorf("message_id must be positive")
	}
	return model.TelegramMessageReference{ChatID: chatID, MessageID: messageID}, nil
}
