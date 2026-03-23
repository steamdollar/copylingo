package pipeline

import (
	"context"
	"fmt"
	"log"
	"sort"

	"github.com/lsj/copylingo/internal/external"
)

const (
	nhkFetcherName       = "nhk"
	nhkDefaultMaxArticle = 20
	nhkLanguage          = "ja"
	nhkLevel             = "N4" // NHK Easy is suitable for N4~N5 learners
	nhkDifficulty        = 3
)

// NHKAPIClient defines the interface for NHK HTTP operations.
// This allows for mocking in tests.
type NHKAPIClient interface {
	FetchArticleList(ctx context.Context) (map[string][]external.NHKArticleMeta, error)
	FetchArticleBody(ctx context.Context, newsID string) (string, error)
}

// NHKFetcher fetches articles from NHK News Easy.
type NHKFetcher struct {
	client      NHKAPIClient
	maxArticles int
}

// NHKFetcherOption configures NHKFetcher.
type NHKFetcherOption func(*NHKFetcher)

// WithMaxArticles sets the maximum number of articles to fetch.
func WithMaxArticles(max int) NHKFetcherOption {
	return func(f *NHKFetcher) {
		f.maxArticles = max
	}
}

// NewNHKFetcher creates a new NHK News Easy fetcher.
func NewNHKFetcher(client NHKAPIClient, opts ...NHKFetcherOption) *NHKFetcher {
	f := &NHKFetcher{
		client:      client,
		maxArticles: nhkDefaultMaxArticle,
	}
	for _, opt := range opts {
		opt(f)
	}
	return f
}

// Name returns the fetcher identifier.
func (f *NHKFetcher) Name() string {
	return nhkFetcherName
}

// Fetch retrieves articles from NHK News Easy.
func (f *NHKFetcher) Fetch(ctx context.Context) ([]RawContent, error) {
	articleMap, err := f.client.FetchArticleList(ctx)
	if err != nil {
		return nil, fmt.Errorf("fetch article list: %w", err)
	}

	// Sort dates descending to get newest articles first
	dates := make([]string, 0, len(articleMap))
	for date := range articleMap {
		dates = append(dates, date)
	}
	sort.Sort(sort.Reverse(sort.StringSlice(dates)))

	var contents []RawContent
	for _, date := range dates {
		if len(contents) >= f.maxArticles {
			break
		}

		for _, article := range articleMap[date] {
			if len(contents) >= f.maxArticles {
				break
			}

			body, err := f.client.FetchArticleBody(ctx, article.NewsID)
			if err != nil {
				log.Printf("[NHKFetcher] WARN: failed to fetch body for %s: %v", article.NewsID, err)
				continue
			}

			if body == "" {
				log.Printf("[NHKFetcher] WARN: empty body for %s", article.NewsID)
				continue
			}

			contents = append(contents, RawContent{
				SourceURL:  article.NewsEasyURL,
				Title:      article.Title,
				Body:       body,
				Language:   nhkLanguage,
				Level:      nhkLevel,
				SourceType: "news",
				Difficulty: nhkDifficulty,
				Tags:       []string{"news", "nhk"},
				IsArticle:  true,
			})
		}
	}

	log.Printf("[NHKFetcher] INFO: fetched %d articles", len(contents))
	return contents, nil
}
