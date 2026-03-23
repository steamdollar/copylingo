package pipeline

import (
	"context"
	"errors"
	"testing"

	"github.com/lsj/copylingo/internal/external"
	"github.com/lsj/copylingo/internal/model"
)

// Mock NHK API Client
type mockNHKClient struct {
	articles map[string][]external.NHKArticleMeta
	bodies   map[string]string
	fetchErr error
	bodyErr  error
}

func (m *mockNHKClient) FetchArticleList(ctx context.Context) (map[string][]external.NHKArticleMeta, error) {
	if m.fetchErr != nil {
		return nil, m.fetchErr
	}
	return m.articles, nil
}

func (m *mockNHKClient) FetchArticleBody(ctx context.Context, newsID string) (string, error) {
	if m.bodyErr != nil {
		return "", m.bodyErr
	}
	body, ok := m.bodies[newsID]
	if !ok {
		return "", errors.New("not found")
	}
	return body, nil
}

// Mock Content Repository
type mockContentRepo struct {
	existing map[string]bool
	saved    []model.Content
	saveErr  error
}

func (m *mockContentRepo) ExistsByURL(ctx context.Context, url string) (bool, error) {
	return m.existing[url], nil
}

func (m *mockContentRepo) Create(ctx context.Context, content *model.Content) error {
	if m.saveErr != nil {
		return m.saveErr
	}
	m.saved = append(m.saved, *content)
	return nil
}

// Mock Fetcher
type mockFetcher struct {
	name     string
	contents []RawContent
	err      error
}

func (m *mockFetcher) Name() string { return m.name }
func (m *mockFetcher) Fetch(ctx context.Context) ([]RawContent, error) {
	return m.contents, m.err
}

// Mock Processor
type mockProcessor struct {
	err error
}

func (m *mockProcessor) Process(ctx context.Context, raw []RawContent) ([]model.Content, error) {
	if m.err != nil {
		return nil, m.err
	}
	contents := make([]model.Content, len(raw))
	for i, r := range raw {
		contents[i] = model.Content{
			SourceURL: r.SourceURL,
			Title:     r.Title,
			Body:      r.Body,
		}
	}
	return contents, nil
}

// Mock Saver
type mockSaver struct {
	result SaveResult
	err    error
}

func (m *mockSaver) Save(ctx context.Context, contents []model.Content) (SaveResult, error) {
	return m.result, m.err
}

// Tests

func TestNHKFetcher_Fetch(t *testing.T) {
	client := &mockNHKClient{
		articles: map[string][]external.NHKArticleMeta{
			"2024-03-22": {
				{NewsID: "k123", Title: "テスト1", NewsEasyURL: "https://test1.com"},
				{NewsID: "k456", Title: "テスト2", NewsEasyURL: "https://test2.com"},
			},
		},
		bodies: map[string]string{
			"k123": "本文1",
			"k456": "本文2",
		},
	}

	fetcher := NewNHKFetcher(client, WithMaxArticles(10))
	contents, err := fetcher.Fetch(context.Background())

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(contents) != 2 {
		t.Fatalf("expected 2 contents, got %d", len(contents))
	}

	if contents[0].Title != "テスト1" {
		t.Errorf("expected title テスト1, got %s", contents[0].Title)
	}

	if contents[0].Language != "ja" {
		t.Errorf("expected language ja, got %s", contents[0].Language)
	}

	if contents[0].Level != "N4" {
		t.Errorf("expected level N4, got %s", contents[0].Level)
	}
}

func TestNHKFetcher_FetchWithLimit(t *testing.T) {
	client := &mockNHKClient{
		articles: map[string][]external.NHKArticleMeta{
			"2024-03-22": {
				{NewsID: "k1", Title: "Test1", NewsEasyURL: "https://1.com"},
				{NewsID: "k2", Title: "Test2", NewsEasyURL: "https://2.com"},
				{NewsID: "k3", Title: "Test3", NewsEasyURL: "https://3.com"},
			},
		},
		bodies: map[string]string{
			"k1": "Body1",
			"k2": "Body2",
			"k3": "Body3",
		},
	}

	fetcher := NewNHKFetcher(client, WithMaxArticles(2))
	contents, err := fetcher.Fetch(context.Background())

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(contents) != 2 {
		t.Fatalf("expected 2 contents (limited), got %d", len(contents))
	}
}

func TestNHKFetcher_FetchError(t *testing.T) {
	client := &mockNHKClient{
		fetchErr: errors.New("network error"),
	}

	fetcher := NewNHKFetcher(client)
	_, err := fetcher.Fetch(context.Background())

	if err == nil {
		t.Fatal("expected error, got nil")
	}
}

func TestPassThroughProcessor_Process(t *testing.T) {
	processor := NewPassThroughProcessor()

	raw := []RawContent{
		{
			SourceURL:  "https://test.com",
			Title:      "Test Title",
			Body:       "Test Body",
			Language:   "ja",
			Level:      "N4",
			SourceType: "news",
			Difficulty: 3,
			Tags:       []string{"news", "nhk"},
			IsArticle:  true,
		},
	}

	contents, err := processor.Process(context.Background(), raw)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(contents) != 1 {
		t.Fatalf("expected 1 content, got %d", len(contents))
	}

	c := contents[0]
	if c.SourceURL != "https://test.com" {
		t.Errorf("expected URL https://test.com, got %s", c.SourceURL)
	}
	if c.Title != "Test Title" {
		t.Errorf("expected title Test Title, got %s", c.Title)
	}
	if c.Language != "ja" {
		t.Errorf("expected language ja, got %s", c.Language)
	}
	if c.ProficiencyLevel != "N4" {
		t.Errorf("expected level N4, got %s", c.ProficiencyLevel)
	}
	if c.SourceType != model.ContentSourceNews {
		t.Errorf("expected source type news, got %s", c.SourceType)
	}
}

func TestContentSaver_Save(t *testing.T) {
	repo := &mockContentRepo{
		existing: map[string]bool{
			"https://existing.com": true,
		},
		saved: make([]model.Content, 0),
	}

	saver := NewContentSaver(repo)

	contents := []model.Content{
		{SourceURL: "https://new1.com", Title: "New 1"},
		{SourceURL: "https://existing.com", Title: "Existing"},
		{SourceURL: "https://new2.com", Title: "New 2"},
	}

	result, err := saver.Save(context.Background(), contents)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result.Saved != 2 {
		t.Errorf("expected 2 saved, got %d", result.Saved)
	}

	if result.Duplicates != 1 {
		t.Errorf("expected 1 duplicate, got %d", result.Duplicates)
	}

	if len(repo.saved) != 2 {
		t.Errorf("expected 2 items in repo, got %d", len(repo.saved))
	}
}

func TestOrchestrator_RunAll(t *testing.T) {
	orchestrator := NewOrchestrator()

	fetcher := &mockFetcher{
		name: "test",
		contents: []RawContent{
			{SourceURL: "https://test.com", Title: "Test", Body: "Body"},
		},
	}

	processor := &mockProcessor{}

	saver := &mockSaver{
		result: SaveResult{Saved: 1, Duplicates: 0},
	}

	orchestrator.Register(fetcher, processor, saver)

	if orchestrator.PipelineCount() != 1 {
		t.Errorf("expected 1 pipeline, got %d", orchestrator.PipelineCount())
	}

	results := orchestrator.RunAll(context.Background())

	if len(results) != 1 {
		t.Fatalf("expected 1 result, got %d", len(results))
	}

	r := results[0]
	if r.FetcherName != "test" {
		t.Errorf("expected fetcher name test, got %s", r.FetcherName)
	}

	if r.FetchedCount != 1 {
		t.Errorf("expected 1 fetched, got %d", r.FetchedCount)
	}

	if r.SaveResult.Saved != 1 {
		t.Errorf("expected 1 saved, got %d", r.SaveResult.Saved)
	}

	if r.Err != nil {
		t.Errorf("unexpected error: %v", r.Err)
	}
}

func TestOrchestrator_RunAll_FetcherError(t *testing.T) {
	orchestrator := NewOrchestrator()

	fetcher := &mockFetcher{
		name: "failing",
		err:  errors.New("fetch error"),
	}

	orchestrator.Register(fetcher, &mockProcessor{}, &mockSaver{})

	results := orchestrator.RunAll(context.Background())

	if len(results) != 1 {
		t.Fatalf("expected 1 result, got %d", len(results))
	}

	if results[0].Err == nil {
		t.Error("expected error, got nil")
	}
}

func TestOrchestrator_RunAll_MultiplePipelines(t *testing.T) {
	orchestrator := NewOrchestrator()

	// First pipeline - success
	orchestrator.Register(
		&mockFetcher{name: "p1", contents: []RawContent{{SourceURL: "https://1.com"}}},
		&mockProcessor{},
		&mockSaver{result: SaveResult{Saved: 1}},
	)

	// Second pipeline - fetch error (should not stop first)
	orchestrator.Register(
		&mockFetcher{name: "p2", err: errors.New("error")},
		&mockProcessor{},
		&mockSaver{},
	)

	results := orchestrator.RunAll(context.Background())

	if len(results) != 2 {
		t.Fatalf("expected 2 results, got %d", len(results))
	}

	// First should succeed
	if results[0].Err != nil {
		t.Errorf("first pipeline should succeed, got error: %v", results[0].Err)
	}

	// Second should fail
	if results[1].Err == nil {
		t.Error("second pipeline should fail")
	}
}
