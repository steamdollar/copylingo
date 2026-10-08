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
	llmPendingKey     = "user:%d:llm_pending"
	activeQuestionKey = "user:%d:active_question"

	llmPendingTTL     = 10 * time.Minute
	activeQuestionTTL = time.Hour
)

// Interactions shares one Redis client across input, draft, and Mini App state.
type Interactions struct {
	rdb redis.Cmdable
}

func NewInteractions(rdb redis.Cmdable) *Interactions {
	return &Interactions{rdb: rdb}
}

func (s *Interactions) SetLLMPending(
	ctx context.Context,
	userID int64,
	input model.PendingLLMInput,
) error {
	return s.rdb.Set(
		ctx,
		fmt.Sprintf(
			llmPendingKey,
			userID,
		),
		encodePendingLLMInput(input),
		llmPendingTTL,
	).Err()
}

func (s *Interactions) TakeLLMPending(
	ctx context.Context,
	userID int64,
) (model.PendingLLMInput, bool, error) {
	// GETDEL preserves one-shot consumption, including malformed legacy values.
	raw, err := s.rdb.GetDel(
		ctx,
		fmt.Sprintf(
			llmPendingKey,
			userID,
		),
	).Result()
	if err == redis.Nil {
		return model.PendingLLMInput{}, false, nil
	}
	if err != nil {
		return model.PendingLLMInput{}, false, fmt.Errorf(
			"take pending LLM input user_id=%d: %w",
			userID,
			err,
		)
	}
	return decodePendingLLMInput(raw), true, nil
}

func (s *Interactions) DeleteLLMPending(
	ctx context.Context,
	userID int64,
) error {
	return s.rdb.Del(
		ctx,
		fmt.Sprintf(
			llmPendingKey,
			userID,
		),
	).Err()
}

func (s *Interactions) GetActiveQuestion(
	ctx context.Context,
	chatID int64,
) (*model.ActiveQuestionRef, error) {
	raw, err := s.rdb.Get(
		ctx,
		fmt.Sprintf(
			activeQuestionKey,
			chatID,
		),
	).Result()
	if err == redis.Nil {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf(
			"get active text question chat_id=%d: %w",
			chatID,
			err,
		)
	}
	parts := strings.Split(
		raw,
		":",
	)
	if len(parts) != 2 {
		return nil, nil
	}
	// Match the prior caller's permissive Atoi handling for existing marker bytes.
	sessionID, _ := strconv.Atoi(parts[0])
	questionIndex, _ := strconv.Atoi(parts[1])
	return &model.ActiveQuestionRef{SessionID: sessionID, QuestionIndex: questionIndex}, nil
}

func (s *Interactions) SetActiveQuestion(
	ctx context.Context,
	chatID int64,
	question model.ActiveQuestionRef,
) error {
	value := fmt.Sprintf(
		"%d:%d",
		question.SessionID,
		question.QuestionIndex,
	)
	return s.rdb.Set(
		ctx,
		fmt.Sprintf(
			activeQuestionKey,
			chatID,
		),
		value,
		activeQuestionTTL,
	).Err()
}

func (s *Interactions) DeleteActiveQuestion(
	ctx context.Context,
	chatID int64,
) error {
	return s.rdb.Del(
		ctx,
		fmt.Sprintf(
			activeQuestionKey,
			chatID,
		),
	).Err()
}

func (s *Interactions) ClearInput(
	ctx context.Context,
	chatID int64,
	userID *int64,
) error {
	keys := []string{fmt.Sprintf(
		activeQuestionKey,
		chatID,
	)}
	if userID != nil {
		keys = append(
			keys,
			fmt.Sprintf(
				llmPendingKey,
				*userID,
			),
		)
	}
	return s.rdb.Del(
		ctx,
		keys...,
	).Err()
}

func encodePendingLLMInput(input model.PendingLLMInput) string {
	switch input.Kind {
	case model.PendingLLMQuizQuestion:
		return fmt.Sprintf(
			"q:%d:%d",
			input.SessionID,
			input.QuestionID,
		)
	case model.PendingLLMStudyMaterial:
		return fmt.Sprintf(
			"study:%d:%d",
			input.SessionID,
			input.MaterialOrder,
		)
	default:
		return "1"
	}
}

func decodePendingLLMInput(raw string) model.PendingLLMInput {
	parts := strings.Split(
		raw,
		":",
	)
	if len(parts) != 3 {
		return model.PendingLLMInput{}
	}
	sessionID, sessionErr := strconv.Atoi(parts[1])
	itemID, itemErr := strconv.Atoi(parts[2])
	if sessionErr != nil || itemErr != nil {
		return model.PendingLLMInput{}
	}
	switch parts[0] {
	case "q":
		return model.PendingLLMInput{Kind: model.PendingLLMQuizQuestion, SessionID: sessionID, QuestionID: itemID}
	case "study":
		return model.PendingLLMInput{Kind: model.PendingLLMStudyMaterial, SessionID: sessionID, MaterialOrder: itemID}
	default:
		return model.PendingLLMInput{}
	}
}
