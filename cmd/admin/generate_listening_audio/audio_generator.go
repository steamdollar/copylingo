package main

import (
	"context"
	"log/slog"

	"github.com/lsj/copylingo/internal/external"
	"github.com/lsj/copylingo/internal/model"
)

// audioGeneratePerCycle bounds one TopUpAudio call so the caller can pace TTS
// requests against provider rate limits.
const audioGeneratePerCycle = 5

type audioGeneratorRepository interface {
	GetListeningNeedingAudio(
		ctx context.Context,
		language string,
		limit int,
	) ([]model.Question, error)
	SetAudioPath(
		ctx context.Context,
		id int,
		audioPath string,
	) error
}

// audioSynthesizer turns text into Telegram-ready OGG/Opus bytes.
type audioSynthesizer interface {
	Synthesize(
		ctx context.Context,
		text string,
	) ([]byte, error)
}

type audioObjectStore interface {
	Exists(
		ctx context.Context,
		key string,
	) (bool, error)
	Put(
		ctx context.Context,
		key string,
		body []byte,
		contentType string,
	) error
}

// audioGenerator lives in this admin tool because the server never calls TTS (ADR-082).
type audioGenerator struct {
	questions audioGeneratorRepository
	tts       audioSynthesizer
	store     audioObjectStore
	voiceKey  string // both configured voices identify newly generated audio
}

// newAudioGenerator: the second voice changes the key for future clips only.
func newAudioGenerator(
	questions audioGeneratorRepository,
	tts audioSynthesizer,
	store audioObjectStore,
	voiceA,
	voiceB string,
) *audioGenerator {
	return &audioGenerator{questions: questions, tts: tts, store: store, voiceKey: voiceA + "-" + voiceB}
}

// TopUpAudio fills up to audioGeneratePerCycle questions; identical scripts dedup to one object, and a
// per-question failure is logged and skipped.
func (generator *audioGenerator) TopUpAudio(
	ctx context.Context,
	language string,
) error {
	pending, err := generator.questions.GetListeningNeedingAudio(
		ctx,
		language,
		audioGeneratePerCycle,
	)
	if err != nil {
		return err
	}
	if len(pending) == 0 {
		return nil
	}

	var generated, reused, failed int
	for _, question := range pending {
		if question.AudioScript == nil || *question.AudioScript == "" {
			continue
		}
		script := *question.AudioScript
		key := external.AudioKey(
			question.Language,
			generator.voiceKey,
			script,
		)

		exists, err := generator.store.Exists(
			ctx,
			key,
		)
		if err != nil {
			failed++
			slog.WarnContext(
				ctx,
				"Audio dedup check failed",
				"event",
				"audiogen.exists_failed",
				"source",
				"service.audio",
				"question_id",
				question.ID,
				"key",
				key,
				"error",
				err,
			)
			continue
		}

		if !exists {
			ogg, err := generator.tts.Synthesize(
				ctx,
				script,
			)
			if err != nil {
				failed++
				slog.WarnContext(
					ctx,
					"Audio synthesis failed",
					"event",
					"audiogen.synthesize_failed",
					"source",
					"service.audio",
					"question_id",
					question.ID,
					"error",
					err,
				)
				continue
			}
			if err := generator.store.Put(
				ctx,
				key,
				ogg,
				external.AudioContentType,
			); err != nil {
				failed++
				slog.WarnContext(
					ctx,
					"Audio store put failed",
					"event",
					"audiogen.put_failed",
					"source",
					"service.audio",
					"question_id",
					question.ID,
					"key",
					key,
					"error",
					err,
				)
				continue
			}
			generated++
		} else {
			reused++
		}

		if err := generator.questions.SetAudioPath(
			ctx,
			question.ID,
			key,
		); err != nil {
			failed++
			slog.WarnContext(
				ctx,
				"Audio path persist failed",
				"event",
				"audiogen.set_path_failed",
				"source",
				"service.audio",
				"question_id",
				question.ID,
				"key",
				key,
				"error",
				err,
			)
			continue
		}
	}

	slog.InfoContext(
		ctx,
		"Listening audio topped up",
		"event",
		"audiogen.topped_up",
		"source",
		"service.audio",
		"language",
		language,
		"pending",
		len(pending),
		"generated",
		generated,
		"reused",
		reused,
		"failed",
		failed,
	)
	return nil
}
