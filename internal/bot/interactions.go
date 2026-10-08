package bot

import (
	"context"

	"github.com/lsj/copylingo/internal/model"
)

type WordOrderDraftStore interface {
	GetWordOrderDraft(
		ctx context.Context,
		sessionID,
		questionID int,
	) ([]int, error)
	SetWordOrderDraft(
		ctx context.Context,
		sessionID,
		questionID int,
		selection []int,
	) error
	DeleteWordOrderDraft(
		ctx context.Context,
		sessionID,
		questionID int,
	) error
}

type HandwritingMessageStore interface {
	SaveHandwritingMessage(
		ctx context.Context,
		sessionID,
		questionID int,
		ref model.TelegramMessageRef,
	) error
	GetHandwritingMessage(
		ctx context.Context,
		sessionID,
		questionID int,
	) (*model.TelegramMessageRef, error)
}

type MiniAppRecoveryStore interface {
	GetMiniAppFingerprint(
		ctx context.Context,
		sessionID int,
	) (string, error)
	SetMiniAppFingerprint(
		ctx context.Context,
		sessionID int,
		fingerprint string,
	) error
}
