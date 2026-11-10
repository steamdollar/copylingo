package main

import (
	"context"
	"errors"
	"testing"

	"github.com/lsj/copylingo/internal/external"
	"github.com/lsj/copylingo/internal/model"
)

const (
	testVoice    = "Kore"
	testVoiceB   = "Puck"
	testVoiceKey = "Kore-Puck"
)

type mockAudioRepository struct {
	pending     []model.Question
	pendingErr  error
	setPathErr  error
	setPathCall []struct {
		id  int
		key string
	}
}

func (mock *mockAudioRepository) GetListeningNeedingAudio(
	_ context.Context,
	_ string,
	_ int,
) ([]model.Question, error) {
	return mock.pending, mock.pendingErr
}

func (mock *mockAudioRepository) SetAudioPath(
	_ context.Context,
	id int,
	key string,
) error {
	if mock.setPathErr != nil {
		return mock.setPathErr
	}
	mock.setPathCall = append(
		mock.setPathCall,
		struct {
			id  int
			key string
		}{id, key},
	)
	return nil
}

type mockSynthesizer struct {
	calls          int
	synthesizerErr error
}

func (mock *mockSynthesizer) Synthesize(
	_ context.Context,
	_ string,
) ([]byte, error) {
	mock.calls++
	if mock.synthesizerErr != nil {
		return nil, mock.synthesizerErr
	}
	return []byte("ogg-bytes"), nil
}

type mockStore struct {
	existing map[string]bool
	putKeys  []string
	existErr error
	putErr   error
}

func (mock *mockStore) Exists(
	_ context.Context,
	key string,
) (bool, error) {
	if mock.existErr != nil {
		return false, mock.existErr
	}
	return mock.existing[key], nil
}

func (mock *mockStore) Put(
	_ context.Context,
	key string,
	_ []byte,
	_ string,
) error {
	if mock.putErr != nil {
		return mock.putErr
	}
	mock.putKeys = append(
		mock.putKeys,
		key,
	)
	return nil
}

func scriptPointer(value string) *string { return &value }

func TestTopUpAudio_GeneratesMissing(t *testing.T) {
	script := "すみません、駅はどこですか。"
	repository := &mockAudioRepository{pending: []model.Question{
		{ID: 7, Language: "ja", AudioScript: scriptPointer(script)},
	}}
	synthesizer := &mockSynthesizer{}
	store := &mockStore{existing: map[string]bool{}}
	generator := newAudioGenerator(
		repository,
		synthesizer,
		store,
		testVoice,
		testVoiceB,
	)

	if err := generator.TopUpAudio(
		context.Background(),
		"ja",
	); err != nil {
		t.Fatalf(
			"unexpected error: %v",
			err,
		)
	}

	wantKey := external.AudioKey(
		"ja",
		testVoiceKey,
		script,
	)
	if synthesizer.calls != 1 {
		t.Errorf(
			"expected 1 synth call, got %d",
			synthesizer.calls,
		)
	}
	if len(store.putKeys) != 1 || store.putKeys[0] != wantKey {
		t.Errorf(
			"put keys = %v, want [%s]",
			store.putKeys,
			wantKey,
		)
	}
	if len(repository.setPathCall) != 1 || repository.setPathCall[0].id != 7 ||
		repository.setPathCall[0].key != wantKey {
		t.Errorf(
			"setPath = %+v, want id=7 key=%s",
			repository.setPathCall,
			wantKey,
		)
	}
}

// Object already present (dedup) → no Synthesize, no Put, but audio_path still set.
func TestTopUpAudio_DedupSkipsSynthesis(t *testing.T) {
	script := "同じスクリプト"
	key := external.AudioKey(
		"ja",
		testVoiceKey,
		script,
	)
	repository := &mockAudioRepository{pending: []model.Question{
		{ID: 1, Language: "ja", AudioScript: scriptPointer(script)},
	}}
	synthesizer := &mockSynthesizer{}
	store := &mockStore{existing: map[string]bool{key: true}}
	generator := newAudioGenerator(
		repository,
		synthesizer,
		store,
		testVoice,
		testVoiceB,
	)

	if err := generator.TopUpAudio(
		context.Background(),
		"ja",
	); err != nil {
		t.Fatalf(
			"unexpected error: %v",
			err,
		)
	}
	if synthesizer.calls != 0 {
		t.Errorf(
			"expected 0 synth calls on dedup, got %d",
			synthesizer.calls,
		)
	}
	if len(store.putKeys) != 0 {
		t.Errorf(
			"expected 0 puts on dedup, got %v",
			store.putKeys,
		)
	}
	if len(repository.setPathCall) != 1 || repository.setPathCall[0].key != key {
		t.Errorf(
			"expected audio_path set to existing key, got %+v",
			repository.setPathCall,
		)
	}
}

// A synth failure on one question does not abort the batch or return an error.
func TestTopUpAudio_PerQuestionFailureIsolated(t *testing.T) {
	repository := &mockAudioRepository{pending: []model.Question{
		{ID: 1, Language: "ja", AudioScript: scriptPointer("a")},
		{ID: 2, Language: "ja", AudioScript: scriptPointer("b")},
	}}
	synthesizer := &mockSynthesizer{synthesizerErr: errors.New("tts down")}
	store := &mockStore{existing: map[string]bool{}}
	generator := newAudioGenerator(
		repository,
		synthesizer,
		store,
		testVoice,
		testVoiceB,
	)

	if err := generator.TopUpAudio(
		context.Background(),
		"ja",
	); err != nil {
		t.Fatalf(
			"batch should not error on per-question synth failure, got %v",
			err,
		)
	}
	if synthesizer.calls != 2 {
		t.Errorf(
			"expected both questions attempted, got %d synth calls",
			synthesizer.calls,
		)
	}
	if len(repository.setPathCall) != 0 {
		t.Errorf(
			"expected no audio_path set when synth fails, got %+v",
			repository.setPathCall,
		)
	}
}

// Questions with no script are skipped entirely.
func TestTopUpAudio_SkipsEmptyScript(t *testing.T) {
	repository := &mockAudioRepository{pending: []model.Question{
		{ID: 1, Language: "ja", AudioScript: nil},
		{ID: 2, Language: "ja", AudioScript: scriptPointer("")},
	}}
	synthesizer := &mockSynthesizer{}
	store := &mockStore{existing: map[string]bool{}}
	generator := newAudioGenerator(
		repository,
		synthesizer,
		store,
		testVoice,
		testVoiceB,
	)

	if err := generator.TopUpAudio(
		context.Background(),
		"ja",
	); err != nil {
		t.Fatalf(
			"unexpected error: %v",
			err,
		)
	}
	if synthesizer.calls != 0 {
		t.Errorf(
			"expected 0 synth calls for empty scripts, got %d",
			synthesizer.calls,
		)
	}
}

func TestTopUpAudio_EmptyPending(t *testing.T) {
	repository := &mockAudioRepository{pending: nil}
	synthesizer := &mockSynthesizer{}
	store := &mockStore{existing: map[string]bool{}}
	generator := newAudioGenerator(
		repository,
		synthesizer,
		store,
		testVoice,
		testVoiceB,
	)

	if err := generator.TopUpAudio(
		context.Background(),
		"ja",
	); err != nil {
		t.Fatalf(
			"unexpected error: %v",
			err,
		)
	}
	if synthesizer.calls != 0 {
		t.Errorf(
			"expected 0 synth calls on empty pending, got %d",
			synthesizer.calls,
		)
	}
}
