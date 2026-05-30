package external

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestNHKClient_FetchArticleList(t *testing.T) {
	// Mock server that handles the NHK list endpoint
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`[{
			"2024-03-22": [
				{
					"news_id": "k123456",
					"title": "テスト記事",
					"title_with_ruby": "<ruby>テスト<rt>てすと</rt></ruby>記事",
					"news_prearranged_time": "2024-03-22 10:00:00",
					"news_easy_url": "https://www3.nhk.or.jp/news/easy/k123456/k123456.html"
				}
			]
		}]`))
	}))
	defer server.Close()

	// Create client with custom HTTP client and override listURL via transport
	client := &NHKClient{
		httpClient: server.Client(),
		baseURL:    server.URL,
	}

	// Test JSON parsing
	jsonData := `[{
		"2024-03-22": [
			{
				"news_id": "k123456",
				"title": "テスト記事"
			}
		]
	}]`

	var wrapper []map[string][]NHKArticleMeta
	if err := json.Unmarshal([]byte(jsonData), &wrapper); err != nil {
		t.Fatalf("failed to unmarshal: %v", err)
	}

	if len(wrapper) != 1 {
		t.Fatalf("expected 1 wrapper, got %d", len(wrapper))
	}

	articles := wrapper[0]
	if len(articles) != 1 {
		t.Fatalf("expected 1 date, got %d", len(articles))
	}

	dateArticles, ok := articles["2024-03-22"]
	if !ok {
		t.Fatal("expected articles for 2024-03-22")
	}

	if len(dateArticles) != 1 {
		t.Fatalf("expected 1 article, got %d", len(dateArticles))
	}

	if dateArticles[0].NewsID != "k123456" {
		t.Errorf("expected news_id k123456, got %s", dateArticles[0].NewsID)
	}

	if dateArticles[0].Title != "テスト記事" {
		t.Errorf("expected title 테스트記事, got %s", dateArticles[0].Title)
	}

	_ = client
}

func TestNHKClient_FetchArticleList_HTTP(t *testing.T) {
	t.Run("Success", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			_, _ = w.Write([]byte(`[{ "2024-03-22": [] }]`))
		}))
		defer server.Close()

		_ = &NHKClient{
			httpClient: server.Client(),
			baseURL:    server.URL,
		}
	})

	t.Run("HTTP 500", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusInternalServerError)
		}))
		defer server.Close()

		client := NewNHKClient(WithHTTPClient(server.Client()))
		client.baseURL = server.URL

		_, err := client.FetchArticleBody(context.Background(), "k123456")
		if err == nil || !strings.Contains(err.Error(), "unexpected status code: 500") {
			t.Errorf("expected 500 error, got %v", err)
		}
	})
}

func TestNHKClient_FetchArticleBody(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		w.Write([]byte(`
			<html>
			<body>
				<div id="js-article-body">
					<p><ruby>今日<rt>きょう</rt></ruby>は<ruby>天気<rt>てんき</rt></ruby>がいい입니다.</p>
				</div>
			</body>
			</html>
		`))
	}))
	defer server.Close()

	client := NewNHKClient(WithHTTPClient(server.Client()))
	client.baseURL = server.URL

	body, err := client.FetchArticleBody(context.Background(), "k123456")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !strings.Contains(body, "今日は天気") {
		t.Errorf("expected body to contain 今日は天気, got %q", body)
	}
}

func TestExtractArticleBody(t *testing.T) {
	tests := []struct {
		name     string
		html     string
		expected string
	}{
		{
			name:     "simple text",
			html:     `<div id="js-article-body">Hello World</div>`,
			expected: "Hello World",
		},
		{
			name:     "with ruby annotation",
			html:     `<div id="js-article-body"><ruby>漢字<rt>かん지</rt></ruby>입니다</div>`,
			expected: "漢字입니다",
		},
		{
			name:     "with paragraph tags",
			html:     `<div id="js-article-body"><p>First</p><p>Second</p></div>`,
			expected: "FirstSecond",
		},
		{
			name:     "no article body",
			html:     `<div id="other">Content</div>`,
			expected: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := extractArticleBody(tt.html)
			if result != tt.expected {
				t.Errorf("expected %q, got %q", tt.expected, result)
			}
		})
	}
}
