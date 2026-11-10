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

type NHKAPIClient interface {
	FetchArticleList(ctx context.Context) (map[string][]external.NHKArticleMeta, error)
	FetchArticleBody(
		ctx context.Context,
		newsID string,
	) (string, error)
}

type NHKFetcher struct {
	client      NHKAPIClient
	maxArticles int
}

type NHKFetcherOption func(*NHKFetcher)

func WithMaxArticles(max int) NHKFetcherOption {
	return func(fetcher *NHKFetcher) {
		fetcher.maxArticles = max
	}
}

func NewNHKFetcher(
	client NHKAPIClient,
	options ...NHKFetcherOption,
) *NHKFetcher {
	fetcher := &NHKFetcher{
		client:      client,
		maxArticles: nhkDefaultMaxArticle,
	}
	for _, option := range options {
		option(fetcher)
	}
	return fetcher
}

func (fetcher *NHKFetcher) Name() string {
	return nhkFetcherName
}

func (fetcher *NHKFetcher) Fetch(ctx context.Context) ([]RawContent, error) {
	articleMap, err := fetcher.client.FetchArticleList(ctx)
	if err != nil {
		return nil, fmt.Errorf(
			"fetch article list: %w",
			err,
		)
	}

	dates := make(
		[]string,
		0,
		len(articleMap),
	)
	for date := range articleMap {
		dates = append(
			dates,
			date,
		)
	}
	sort.Sort(sort.Reverse(sort.StringSlice(dates)))

	var contents []RawContent
	for _, date := range dates {
		if len(contents) >= fetcher.maxArticles {
			break
		}

		for _, article := range articleMap[date] {
			if len(contents) >= fetcher.maxArticles {
				break
			}

			body, err := fetcher.client.FetchArticleBody(
				ctx,
				article.NewsID,
			)
			if err != nil {
				log.Printf(
					"[NHKFetcher] WARN: failed to fetch body for %s: %v",
					article.NewsID,
					err,
				)
				continue
			}

			if body == "" {
				log.Printf(
					"[NHKFetcher] WARN: empty body for %s",
					article.NewsID,
				)
				continue
			}

			contents = append(
				contents,
				RawContent{
					SourceURL:  article.NewsEasyURL,
					Title:      article.Title,
					Body:       body,
					Language:   nhkLanguage,
					Level:      nhkLevel,
					SourceType: "news",
					Difficulty: nhkDifficulty,
					Tags:       []string{"news", "nhk"},
					IsArticle:  true,
				},
			)
		}
	}

	log.Printf(
		"[NHKFetcher] INFO: fetched %d articles",
		len(contents),
	)
	return contents, nil
}
