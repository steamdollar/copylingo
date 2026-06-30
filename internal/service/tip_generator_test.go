package service

import (
	"context"
	"errors"
	"testing"

	"github.com/lsj/copylingo/internal/external"
	"github.com/lsj/copylingo/internal/model"
)

type mockTipGenRepo struct {
	countActiveFn func(ctx context.Context, language, level string) (int, error)
	createFn      func(ctx context.Context, tip *model.Tip) error

	createCalls int
	created     []*model.Tip
}

func (m *mockTipGenRepo) CountActive(ctx context.Context, language, level string) (int, error) {
	return m.countActiveFn(ctx, language, level)
}

func (m *mockTipGenRepo) Create(ctx context.Context, tip *model.Tip) error {
	m.createCalls++
	m.created = append(m.created, tip)
	return m.createFn(ctx, tip)
}

type mockTipGenLLM struct {
	generateTipsFn func(ctx context.Context, language, level string, category model.TipCategory, n int) ([]external.GeneratedTip, error)

	generateCalls int
}

func (m *mockTipGenLLM) GenerateTips(
	ctx context.Context,
	language, level string,
	category model.TipCategory,
	n int,
) ([]external.GeneratedTip, error) {
	m.generateCalls++
	return m.generateTipsFn(ctx, language, level, category, n)
}

// bucket already at/above target → no LLM call, no Create.
func TestTopUpBucket_FullBucketSkipsLLM(t *testing.T) {
	repo := &mockTipGenRepo{
		countActiveFn: func(_ context.Context, _, _ string) (int, error) {
			return TipBucketTarget, nil
		},
		createFn: func(_ context.Context, _ *model.Tip) error { return nil },
	}
	llm := &mockTipGenLLM{
		generateTipsFn: func(_ context.Context, _, _ string, _ model.TipCategory, _ int) ([]external.GeneratedTip, error) {
			return nil, nil
		},
	}
	g := NewTipGenerator(repo, llm, "test-model")

	if err := g.TopUpBucket(context.Background(), "ja", "N5"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if llm.generateCalls != 0 {
		t.Errorf("expected 0 LLM calls when bucket full, got %d", llm.generateCalls)
	}
	if repo.createCalls != 0 {
		t.Errorf("expected 0 Create calls when bucket full, got %d", repo.createCalls)
	}
}

// bucket below target → one LLM call + Create for each returned tip, with
// source_model / source_prompt_ver / is_active populated from code.
func TestTopUpBucket_BelowTargetGeneratesAndCreates(t *testing.T) {
	repo := &mockTipGenRepo{
		countActiveFn: func(_ context.Context, _, _ string) (int, error) {
			return 10, nil
		},
		createFn: func(_ context.Context, _ *model.Tip) error { return nil },
	}
	llm := &mockTipGenLLM{
		generateTipsFn: func(_ context.Context, _, _ string, _ model.TipCategory, n int) ([]external.GeneratedTip, error) {
			if n != TipGeneratePerCycle {
				t.Errorf("expected n=%d, got %d", TipGeneratePerCycle, n)
			}
			return []external.GeneratedTip{{Body: "tip a"}, {Body: "tip b"}}, nil
		},
	}
	g := NewTipGenerator(repo, llm, "test-model")

	if err := g.TopUpBucket(context.Background(), "ja", "N5"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if llm.generateCalls != 1 {
		t.Errorf("expected 1 LLM call, got %d", llm.generateCalls)
	}
	if repo.createCalls != 2 {
		t.Errorf("expected 2 Create calls, got %d", repo.createCalls)
	}
	first := repo.created[0]
	if first.Language != "ja" || first.ProficiencyLevel != "N5" {
		t.Errorf("unexpected (language, level): %s/%s", first.Language, first.ProficiencyLevel)
	}
	if first.Body != "tip a" {
		t.Errorf("unexpected body: %q", first.Body)
	}
	if !first.IsActive {
		t.Error("expected IsActive=true")
	}
	if first.SourceModel == nil || *first.SourceModel != "test-model" {
		t.Errorf("expected source_model=test-model, got %v", first.SourceModel)
	}
	if first.SourcePromptVer == nil || *first.SourcePromptVer != TipPromptVersion {
		t.Errorf("expected source_prompt_ver=%s, got %v", TipPromptVersion, first.SourcePromptVer)
	}
}

// LLM returns an empty array → no Create, no error.
func TestTopUpBucket_EmptyLLMResultNoError(t *testing.T) {
	repo := &mockTipGenRepo{
		countActiveFn: func(_ context.Context, _, _ string) (int, error) { return 0, nil },
		createFn:      func(_ context.Context, _ *model.Tip) error { return nil },
	}
	llm := &mockTipGenLLM{
		generateTipsFn: func(_ context.Context, _, _ string, _ model.TipCategory, _ int) ([]external.GeneratedTip, error) {
			return []external.GeneratedTip{}, nil
		},
	}
	g := NewTipGenerator(repo, llm, "test-model")

	if err := g.TopUpBucket(context.Background(), "ja", "N5"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if repo.createCalls != 0 {
		t.Errorf("expected 0 Create calls on empty result, got %d", repo.createCalls)
	}
}

// LLM error → propagated to caller (scheduler skips the pair).
func TestTopUpBucket_LLMErrorPropagates(t *testing.T) {
	wantErr := errors.New("llm down")
	repo := &mockTipGenRepo{
		countActiveFn: func(_ context.Context, _, _ string) (int, error) { return 0, nil },
		createFn:      func(_ context.Context, _ *model.Tip) error { return nil },
	}
	llm := &mockTipGenLLM{
		generateTipsFn: func(_ context.Context, _, _ string, _ model.TipCategory, _ int) ([]external.GeneratedTip, error) {
			return nil, wantErr
		},
	}
	g := NewTipGenerator(repo, llm, "test-model")

	err := g.TopUpBucket(context.Background(), "ja", "N5")
	if !errors.Is(err, wantErr) {
		t.Fatalf("expected llm error to propagate, got %v", err)
	}
	if repo.createCalls != 0 {
		t.Errorf("expected 0 Create calls when LLM errors, got %d", repo.createCalls)
	}
}

// One Create fails → remaining tips still persisted (partial success), no error.
func TestTopUpBucket_PartialCreateFailureContinues(t *testing.T) {
	createErr := errors.New("db write failed")
	var nth int
	repo := &mockTipGenRepo{
		countActiveFn: func(_ context.Context, _, _ string) (int, error) { return 0, nil },
		createFn: func(_ context.Context, _ *model.Tip) error {
			nth++
			if nth == 1 {
				return createErr
			}
			return nil
		},
	}
	llm := &mockTipGenLLM{
		generateTipsFn: func(_ context.Context, _, _ string, _ model.TipCategory, _ int) ([]external.GeneratedTip, error) {
			return []external.GeneratedTip{{Body: "a"}, {Body: "b"}, {Body: "c"}}, nil
		},
	}
	g := NewTipGenerator(repo, llm, "test-model")

	if err := g.TopUpBucket(context.Background(), "ja", "N5"); err != nil {
		t.Fatalf("partial Create failure should not error, got %v", err)
	}
	if repo.createCalls != 3 {
		t.Errorf("expected 3 Create attempts (continue past failure), got %d", repo.createCalls)
	}
}
