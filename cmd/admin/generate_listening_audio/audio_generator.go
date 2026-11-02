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

// audioGeneratorRepo is the narrow question-repository contract audioGenerator needs.
type audioGeneratorRepo interface {
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

// audioObjectStore is the object-store contract audioGenerator writes through.
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

// audioGenerator pre-generates listening audio into the object store (ADR-031).
// It lives in this admin tool because nothing else runs it: the server never
// calls TTS and only serves stored clips through service.AudioService (ADR-082).
type audioGenerator struct {
	questions audioGeneratorRepo
	tts       audioSynthesizer
	store     audioObjectStore
	voiceKey  string // both configured voices identify newly generated audio
}

// newAudioGenerator wires the generator with its repo, TTS client, object store,
// and configured voices. The second voice changes the key for future clips only.
func newAudioGenerator(
	questions audioGeneratorRepo,
	tts audioSynthesizer,
	store audioObjectStore,
	voiceA,
	voiceB string,
) *audioGenerator {
	return &audioGenerator{questions: questions, tts: tts, store: store, voiceKey: voiceA + "-" + voiceB}
}

// TopUpAudio synthesizes audio for up to audioGeneratePerCycle listening questions
// of the language that still lack it, across every level. Content-addressing
// dedups identical scripts to one object; a per-question failure is logged and
// skipped.
func (g *audioGenerator) TopUpAudio(
	ctx context.Context,
	language string,
) error {
	pending, err := g.questions.GetListeningNeedingAudio(
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
	for _, q := range pending {
		if q.AudioScript == nil || *q.AudioScript == "" {
			continue
		}
		script := *q.AudioScript
		key := external.AudioKey(
			q.Language,
			g.voiceKey,
			script,
		)

		exists, err := g.store.Exists(
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
				q.ID,
				"key",
				key,
				"error",
				err,
			)
			continue
		}

		if !exists {
			ogg, err := g.tts.Synthesize(
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
					q.ID,
					"error",
					err,
				)
				continue
			}
			if err := g.store.Put(
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
					q.ID,
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

		if err := g.questions.SetAudioPath(
			ctx,
			q.ID,
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
				q.ID,
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
