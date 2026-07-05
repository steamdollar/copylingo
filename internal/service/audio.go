package service

import (
	"context"
	"log/slog"

	"github.com/lsj/copylingo/internal/external"
	"github.com/lsj/copylingo/internal/model"
)

const (
	// AudioGeneratePerCycle bounds how many listening clips one TopUpAudio call
	// synthesizes. The Gemini preview TTS has a tight rate-limit, and audio is
	// pre-generated (ADR-031), so a small per-cycle batch fills the backlog over
	// several scheduler runs without bursting the quota.
	AudioGeneratePerCycle = 5
)

// audioServiceRepo is the narrow question-repository contract AudioService needs.
type audioServiceRepo interface {
	GetListeningNeedingAudio(ctx context.Context, language, level string, limit int) ([]model.Question, error)
	SetAudioPath(ctx context.Context, id int, audioPath string) error
	SetAudioFileID(ctx context.Context, id int, fileID string) error
}

// audioSynthesizer turns text into Telegram-ready OGG/Opus bytes.
type audioSynthesizer interface {
	Synthesize(ctx context.Context, text string) ([]byte, error)
}

// audioObjectStore is the object-store contract AudioService depends on.
type audioObjectStore interface {
	Exists(ctx context.Context, key string) (bool, error)
	Put(ctx context.Context, key string, body []byte, contentType string) error
	Get(ctx context.Context, key string) ([]byte, error)
}

// AudioService owns the listening-audio lifecycle: pre-generation into the object
// store (push model, ADR-031) and serve-time retrieval for Telegram. It holds no
// transaction with session building — a generation failure never blocks a push.
type AudioService struct {
	questions audioServiceRepo
	tts       audioSynthesizer
	store     audioObjectStore
	voice     string // recorded into the content-addressed key
}

// NewAudioService wires the service with its repo, TTS client, object store, and
// the configured voice name.
func NewAudioService(
	questions audioServiceRepo,
	tts audioSynthesizer,
	store audioObjectStore,
	voice string,
) *AudioService {
	return &AudioService{questions: questions, tts: tts, store: store, voice: voice}
}

// TopUpAudio synthesizes audio for up to AudioGeneratePerCycle listening questions
// in the (language, level) bucket that still lack it. Content-addressing dedups
// identical scripts to one object; a per-question failure is logged and skipped.
func (s *AudioService) TopUpAudio(ctx context.Context, language, level string) error {
	if s.tts == nil || s.store == nil {
		return external.ErrTTSConfigMissing
	}

	pending, err := s.questions.GetListeningNeedingAudio(ctx, language, level, AudioGeneratePerCycle)
	if err != nil {
		return err
	}
	if len(pending) == 0 {
		return nil
	}

	var generated, reused, failed int
	for _, q := range pending {
		if q.AudioScript == nil || *q.AudioScript == "" {
			continue
		}
		script := *q.AudioScript
		key := external.AudioKey(q.Language, s.voice, script)

		exists, err := s.store.Exists(ctx, key)
		if err != nil {
			failed++
			slog.WarnContext(ctx, "Audio dedup check failed",
				"event", "audiogen.exists_failed",
				"source", "service.audio",
				"question_id", q.ID, "key", key, "error", err,
			)
			continue
		}

		if !exists {
			ogg, err := s.tts.Synthesize(ctx, script)
			if err != nil {
				failed++
				slog.WarnContext(ctx, "Audio synthesis failed",
					"event", "audiogen.synthesize_failed",
					"source", "service.audio",
					"question_id", q.ID, "error", err,
				)
				continue
			}
			if err := s.store.Put(ctx, key, ogg, external.AudioContentType); err != nil {
				failed++
				slog.WarnContext(ctx, "Audio store put failed",
					"event", "audiogen.put_failed",
					"source", "service.audio",
					"question_id", q.ID, "key", key, "error", err,
				)
				continue
			}
			generated++
		} else {
			reused++
		}

		if err := s.questions.SetAudioPath(ctx, q.ID, key); err != nil {
			failed++
			slog.WarnContext(ctx, "Audio path persist failed",
				"event", "audiogen.set_path_failed",
				"source", "service.audio",
				"question_id", q.ID, "key", key, "error", err,
			)
			continue
		}
	}

	slog.InfoContext(ctx, "Listening audio topped up",
		"event", "audiogen.topped_up",
		"source", "service.audio",
		"language", language, "level", level,
		"pending", len(pending),
		"generated", generated, "reused", reused, "failed", failed,
	)
	return nil
}

// GetClip fetches the stored OGG bytes for an object key (serve-time fallback used
// on the first send, or after a Telegram file_id is invalidated).
func (s *AudioService) GetClip(ctx context.Context, key string) ([]byte, error) {
	if s.store == nil {
		return nil, external.ErrTTSConfigMissing
	}
	return s.store.Get(ctx, key)
}

// CacheFileID stores the Telegram file_id returned after the first upload so later
// sends can reuse it (ADR-032). The object store stays the SSOT.
func (s *AudioService) CacheFileID(ctx context.Context, questionID int, fileID string) error {
	return s.questions.SetAudioFileID(ctx, questionID, fileID)
}
