package bot

import (
	"context"
	"time"

	"github.com/lsj/copylingo/internal/model"
)

type InputStateStore interface {
	SetLLMPending(ctx context.Context, userID int64, input model.PendingLLMInput) error
	TakeLLMPending(ctx context.Context, userID int64) (model.PendingLLMInput, bool, error)
	DeleteLLMPending(ctx context.Context, userID int64) error
	GetActiveQuestion(ctx context.Context, chatID int64) (*model.ActiveQuestionRef, error)
	SetActiveQuestion(ctx context.Context, chatID int64, question model.ActiveQuestionRef) error
	DeleteActiveQuestion(ctx context.Context, chatID int64) error
	ClearInput(ctx context.Context, chatID int64, userID *int64) error
}

type WordOrderDraftStore interface {
	GetWordOrderDraft(ctx context.Context, sessionID, questionID int) ([]int, error)
	SetWordOrderDraft(ctx context.Context, sessionID, questionID int, selection []int) error
	DeleteWordOrderDraft(ctx context.Context, sessionID, questionID int) error
}

type HandwritingMessageStore interface {
	SaveHandwritingMessage(ctx context.Context, sessionID, questionID int, ref model.TelegramMessageRef) error
	GetHandwritingMessage(ctx context.Context, sessionID, questionID int) (*model.TelegramMessageRef, error)
}

type MiniAppRecoveryStore interface {
	GetMiniAppFingerprint(ctx context.Context, sessionID int) (string, error)
	SetMiniAppFingerprint(ctx context.Context, sessionID int, fingerprint string) error
}

type QuestionTimingStore interface {
	RecordQuestionStart(ctx context.Context, sessionID int, startedAt time.Time) error
}

// StateStores keeps each bot use of transient Redis state behind its own narrow contract.
type StateStores struct {
	Input    InputStateStore
	Drafts   WordOrderDraftStore
	Messages HandwritingMessageStore
	Recovery MiniAppRecoveryStore
	Timing   QuestionTimingStore
}
