package model

import "errors"

var ErrInvalidTelegramMessageReference = errors.New("invalid Telegram message reference")

// ActiveQuestionReference identifies the text-answer question waiting for a chat reply.
type ActiveQuestionReference struct {
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

// TelegramMessageReference identifies the Telegram message containing a Mini App link.
type TelegramMessageReference struct {
	ChatID    int64
	MessageID int
}
