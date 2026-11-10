package service

import (
	"context"
)

// audioFileIDStore records the Telegram file_id of an uploaded listening clip.
type audioFileIDStore interface {
	SetAudioFileID(
		ctx context.Context,
		id int,
		fileID string,
	) error
}

// audioClipReader reads a stored listening clip from the object store.
type audioClipReader interface {
	Get(
		ctx context.Context,
		key string,
	) ([]byte, error)
}

// AudioService serves pre-generated listening clips to Telegram (ADR-032).
// Clips are generated outside the server by cmd/admin/generate_listening_audio (ADR-082).
type AudioService struct {
	questions audioFileIDStore
	store     audioClipReader
}

// NewAudioService wires the service with the question repo and object store.
func NewAudioService(
	questions audioFileIDStore,
	store audioClipReader,
) *AudioService {
	return &AudioService{questions: questions, store: store}
}

// GetClip fetches the stored OGG bytes for an object key (serve-time fallback used
// on the first send, or after a Telegram file_id is invalidated).
func (service *AudioService) GetClip(
	ctx context.Context,
	key string,
) ([]byte, error) {
	return service.store.Get(
		ctx,
		key,
	)
}

// CacheFileID stores the Telegram file_id returned after the first upload so later
// sends can reuse it (ADR-032). The object store stays the SSOT.
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
