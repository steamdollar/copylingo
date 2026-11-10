package model

import "errors"

var ErrInvalidTelegramMessageReference = errors.New("invalid Telegram message reference")

type ActiveQuestionReference struct {
	SessionID     int
	QuestionIndex int
}

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

type TelegramMessageReference struct {
	ChatID    int64
	MessageID int
}
