package model

import "errors"

var ErrInvalidTelegramMessageRef = errors.New("invalid Telegram message reference")

// ActiveQuestionRef identifies the text-answer question waiting for a chat reply.
type ActiveQuestionRef struct {
	SessionID     int
	QuestionIndex int
}

// PendingLLMInput describes the context attached to the next LLM text message.
type PendingLLMInput struct {
	Kind          PendingLLMInputKind
	SessionID     int
	QuestionID    int
	MaterialOrder int
}

type PendingLLMInputKind uint8

const (
	PendingLLMPlain PendingLLMInputKind = iota
	PendingLLMQuizQuestion
	PendingLLMStudyMaterial
)

// TelegramMessageRef identifies the Telegram message containing a Mini App link.
type TelegramMessageRef struct {
	ChatID    int64
	MessageID int
}
