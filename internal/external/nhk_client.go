package external

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"regexp"
	"strings"
	"time"
)

const (
	nhkBaseURL     = "https://www3.nhk.or.jp/news/easy"
	nhkListURL     = nhkBaseURL + "/news-list.json"
	defaultTimeout = 30 * time.Second
)

// NHKArticleMeta represents metadata for a single article from news-list.json.
type NHKArticleMeta struct {
	NewsID          string `json:"news_id"`
	Title           string `json:"title"`
	TitleWithRuby   string `json:"title_with_ruby"`
	PublishDate     string `json:"news_prearranged_time"`
	NewsEasyURL     string `json:"news_easy_url"`
	NewsWebURL      string `json:"news_web_url"`
	NewsWebImageURI string `json:"news_web_image_uri"`
}

// NHKClient handles HTTP communication with NHK News Easy API.
type NHKClient struct {
	httpClient *http.Client
	baseURL    string
}

// NHKClientOption configures NHKClient.
type NHKClientOption func(*NHKClient)

// WithHTTPClient sets a custom HTTP client.
func WithHTTPClient(client *http.Client) NHKClientOption {
	return func(nhkClient *NHKClient) {
		nhkClient.httpClient = client
	}
}

// NewNHKClient creates a new NHK News Easy API client.
func NewNHKClient(options ...NHKClientOption) *NHKClient {
	client := &NHKClient{
		httpClient: &http.Client{Timeout: defaultTimeout},
		baseURL:    nhkBaseURL,
	}
	for _, option := range options {
		option(client)
	}
	return client
}

// FetchArticleList fetches the news list JSON.
// Returns a map where keys are dates (YYYY-MM-DD) and values are article lists.
func (client *NHKClient) FetchArticleList(ctx context.Context) (map[string][]NHKArticleMeta, error) {
	request, err := http.NewRequestWithContext(
		ctx,
		http.MethodGet,
		nhkListURL,
		nil,
	)
	if err != nil {
		return nil, fmt.Errorf(
			"create request: %w",
			err,
		)
	}

	response, err := client.httpClient.Do(request)
	if err != nil {
		return nil, fmt.Errorf(
			"fetch article list: %w",
			err,
		)
	}
	defer response.Body.Close()

	if response.StatusCode != http.StatusOK {
		return nil, fmt.Errorf(
			"unexpected status code: %d",
			response.StatusCode,
		)
	}

	body, err := io.ReadAll(response.Body)
	if err != nil {
		return nil, fmt.Errorf(
			"read response body: %w",
			err,
		)
	}

	// NHK returns array with single object: [{ "date": [...], "date2": [...] }]
	var wrapper []map[string][]NHKArticleMeta
	if err := json.Unmarshal(
		body,
		&wrapper,
	); err != nil {
		return nil, fmt.Errorf(
			"unmarshal article list: %w",
			err,
		)
	}

	if len(wrapper) == 0 {
		return nil, fmt.Errorf("empty article list response")
	}

	return wrapper[0], nil
}

// FetchArticleBody fetches and extracts the article body from HTML.
func (client *NHKClient) FetchArticleBody(
	ctx context.Context,
	newsID string,
) (string, error) {
	url := fmt.Sprintf(
		"%s/%s/%s.html",
		client.baseURL,
		newsID,
		newsID,
	)

	request, err := http.NewRequestWithContext(
		ctx,
		http.MethodGet,
		url,
		nil,
	)
	if err != nil {
		return "", fmt.Errorf(
			"create request: %w",
			err,
		)
	}

	response, err := client.httpClient.Do(request)
	if err != nil {
		return "", fmt.Errorf(
			"fetch article body: %w",
			err,
		)
	}
	defer response.Body.Close()

	if response.StatusCode != http.StatusOK {
		return "", fmt.Errorf(
			"unexpected status code: %d",
			response.StatusCode,
		)
	}

	body, err := io.ReadAll(response.Body)
	if err != nil {
		return "", fmt.Errorf(
			"read response body: %w",
			err,
		)
	}

	return extractArticleBody(string(body)), nil
}

// extractArticleBody extracts the main article text from NHK Easy HTML.
// Uses regex to find content within <div id="js-article-body"> tags.
func extractArticleBody(html string) string {
	// Pattern to match article body div
	pattern := regexp.MustCompile(`(?s)<div[^>]*id="js-article-body"[^>]*>(.*?)</div>`)
	matches := pattern.FindStringSubmatch(html)
	if len(matches) < 2 {
		return ""
	}

	content := matches[1]

	// Remove ruby annotations but keep the base text
	// <ruby>漢字<rt>かんじ</rt></ruby> -> 漢字
	rubyPattern := regexp.MustCompile(`<ruby>([^<]*)<rt>[^<]*</rt></ruby>`)
	content = rubyPattern.ReplaceAllString(
		content,
		"$1",
	)

	// Remove remaining HTML tags
	tagPattern := regexp.MustCompile(`<[^>]+>`)
	content = tagPattern.ReplaceAllString(
		content,
		"",
	)

	// Clean up whitespace
	content = strings.TrimSpace(content)
	content = regexp.MustCompile(`\s+`).ReplaceAllString(
		content,
		" ",
	)

	return content
}
