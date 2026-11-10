package service

import (
	"context"
)

type audioFileIDStore interface {
	SetAudioFileID(
		ctx context.Context,
		id int,
		fileID string,
	) error
}

type audioClipReader interface {
	Get(
		ctx context.Context,
		key string,
	) ([]byte, error)
}

// AudioService serves clips generated offline by cmd/admin/generate_listening_audio (ADR-082).
type AudioService struct {
	questions audioFileIDStore
	store     audioClipReader
}

func NewAudioService(
	questions audioFileIDStore,
	store audioClipReader,
) *AudioService {
	return &AudioService{questions: questions, store: store}
}

// GetClip is the fallback used on the first send or after a Telegram file_id is invalidated.
func (service *AudioService) GetClip(
	ctx context.Context,
	key string,
) ([]byte, error) {
	return service.store.Get(
		ctx,
		key,
	)
}

// CacheFileID stores the file_id so later sends can reuse it; the object store stays the SSOT.
func (service *AudioService) CacheFileID(
	ctx context.Context,
	questionID int,
	fileID string,
) error {
	return service.questions.SetAudioFileID(
		ctx,
		questionID,
		fileID,
	)
}
