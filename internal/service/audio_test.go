package service

import (
	"context"
	"errors"
	"testing"

	"github.com/lsj/copylingo/internal/external"
	"github.com/lsj/copylingo/internal/model"
)

const testVoice = "Kore"

type mockAudioRepo struct {
	pending     []model.Question
	pendingErr  error
	setPathErr  error
	setPathCall []struct {
		id  int
		key string
	}
	setFileIDCall []struct {
		id     int
		fileID string
	}
}

func (m *mockAudioRepo) GetListeningNeedingAudio(_ context.Context, _, _ string, _ int) ([]model.Question, error) {
	return m.pending, m.pendingErr
}

func (m *mockAudioRepo) SetAudioPath(_ context.Context, id int, key string) error {
	if m.setPathErr != nil {
		return m.setPathErr
	}
	m.setPathCall = append(m.setPathCall, struct {
		id  int
		key string
	}{id, key})
	return nil
}

func (m *mockAudioRepo) SetAudioFileID(_ context.Context, id int, fileID string) error {
	m.setFileIDCall = append(m.setFileIDCall, struct {
		id     int
		fileID string
	}{id, fileID})
	return nil
}

type mockSynth struct {
	calls    int
	synthErr error
}

func (m *mockSynth) Synthesize(_ context.Context, _ string) ([]byte, error) {
	m.calls++
	if m.synthErr != nil {
		return nil, m.synthErr
	}
	return []byte("ogg-bytes"), nil
}

type mockStore struct {
	existing map[string]bool // keys that already exist
	putKeys  []string
	existErr error
	putErr   error
	getBytes []byte
	getErr   error
}

func (m *mockStore) Exists(_ context.Context, key string) (bool, error) {
	if m.existErr != nil {
		return false, m.existErr
	}
	return m.existing[key], nil
}

func (m *mockStore) Put(_ context.Context, key string, _ []byte, _ string) error {
	if m.putErr != nil {
		return m.putErr
	}
	m.putKeys = append(m.putKeys, key)
	return nil
}

func (m *mockStore) Get(_ context.Context, _ string) ([]byte, error) {
	return m.getBytes, m.getErr
}

func scriptPtr(s string) *string { return &s }

// New object → Synthesize + Put + SetAudioPath with the content-addressed key.
func TestTopUpAudio_GeneratesMissing(t *testing.T) {
	script := "すみません、駅はどこですか。"
	repo := &mockAudioRepo{pending: []model.Question{
		{ID: 7, Language: "ja", AudioScript: scriptPtr(script)},
	}}
	synth := &mockSynth{}
	store := &mockStore{existing: map[string]bool{}}
	s := NewAudioService(repo, synth, store, testVoice)

	if err := s.TopUpAudio(context.Background(), "ja", "N5"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	wantKey := external.AudioKey("ja", testVoice, script)
	if synth.calls != 1 {
		t.Errorf("expected 1 synth call, got %d", synth.calls)
	}
	if len(store.putKeys) != 1 || store.putKeys[0] != wantKey {
		t.Errorf("put keys = %v, want [%s]", store.putKeys, wantKey)
	}
	if len(repo.setPathCall) != 1 || repo.setPathCall[0].id != 7 || repo.setPathCall[0].key != wantKey {
		t.Errorf("setPath = %+v, want id=7 key=%s", repo.setPathCall, wantKey)
	}
}

// Object already present (dedup) → no Synthesize, no Put, but audio_path still set.
func TestTopUpAudio_DedupSkipsSynthesis(t *testing.T) {
	script := "同じスクリプト"
	key := external.AudioKey("ja", testVoice, script)
	repo := &mockAudioRepo{pending: []model.Question{
		{ID: 1, Language: "ja", AudioScript: scriptPtr(script)},
	}}
	synth := &mockSynth{}
	store := &mockStore{existing: map[string]bool{key: true}}
	s := NewAudioService(repo, synth, store, testVoice)

	if err := s.TopUpAudio(context.Background(), "ja", "N5"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if synth.calls != 0 {
		t.Errorf("expected 0 synth calls on dedup, got %d", synth.calls)
	}
	if len(store.putKeys) != 0 {
		t.Errorf("expected 0 puts on dedup, got %v", store.putKeys)
	}
	if len(repo.setPathCall) != 1 || repo.setPathCall[0].key != key {
		t.Errorf("expected audio_path set to existing key, got %+v", repo.setPathCall)
	}
}

// A synth failure on one question does not abort the batch or return an error.
func TestTopUpAudio_PerQuestionFailureIsolated(t *testing.T) {
	repo := &mockAudioRepo{pending: []model.Question{
		{ID: 1, Language: "ja", AudioScript: scriptPtr("a")},
		{ID: 2, Language: "ja", AudioScript: scriptPtr("b")},
	}}
	synth := &mockSynth{synthErr: errors.New("tts down")}
	store := &mockStore{existing: map[string]bool{}}
	s := NewAudioService(repo, synth, store, testVoice)

	if err := s.TopUpAudio(context.Background(), "ja", "N5"); err != nil {
		t.Fatalf("batch should not error on per-question synth failure, got %v", err)
	}
	if synth.calls != 2 {
		t.Errorf("expected both questions attempted, got %d synth calls", synth.calls)
	}
	if len(repo.setPathCall) != 0 {
		t.Errorf("expected no audio_path set when synth fails, got %+v", repo.setPathCall)
	}
}

// Questions with no script are skipped entirely.
func TestTopUpAudio_SkipsEmptyScript(t *testing.T) {
	repo := &mockAudioRepo{pending: []model.Question{
		{ID: 1, Language: "ja", AudioScript: nil},
		{ID: 2, Language: "ja", AudioScript: scriptPtr("")},
	}}
	synth := &mockSynth{}
	store := &mockStore{existing: map[string]bool{}}
	s := NewAudioService(repo, synth, store, testVoice)

	if err := s.TopUpAudio(context.Background(), "ja", "N5"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if synth.calls != 0 {
		t.Errorf("expected 0 synth calls for empty scripts, got %d", synth.calls)
	}
}

// Empty backlog → no-op, no error.
func TestTopUpAudio_EmptyPending(t *testing.T) {
	repo := &mockAudioRepo{pending: nil}
	synth := &mockSynth{}
	store := &mockStore{existing: map[string]bool{}}
	s := NewAudioService(repo, synth, store, testVoice)

	if err := s.TopUpAudio(context.Background(), "ja", "N5"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if synth.calls != 0 {
		t.Errorf("expected 0 synth calls on empty pending, got %d", synth.calls)
	}
}

// nil dependencies → ErrTTSConfigMissing (scheduler tolerates and skips).
func TestTopUpAudio_NilDepsReturnsConfigMissing(t *testing.T) {
	repo := &mockAudioRepo{}
	s := NewAudioService(repo, nil, nil, testVoice)
	if err := s.TopUpAudio(context.Background(), "ja", "N5"); !errors.Is(err, external.ErrTTSConfigMissing) {
		t.Fatalf("expected ErrTTSConfigMissing, got %v", err)
	}
}

// CacheFileID delegates to the repo.
func TestCacheFileID(t *testing.T) {
	repo := &mockAudioRepo{}
	s := NewAudioService(repo, &mockSynth{}, &mockStore{}, testVoice)
	if err := s.CacheFileID(context.Background(), 42, "file-xyz"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(repo.setFileIDCall) != 1 || repo.setFileIDCall[0].id != 42 || repo.setFileIDCall[0].fileID != "file-xyz" {
		t.Errorf("setFileID = %+v, want id=42 fileID=file-xyz", repo.setFileIDCall)
	}
}
