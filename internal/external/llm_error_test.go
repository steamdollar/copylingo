package external

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/sashabaranov/go-openai"

	"github.com/lsj/copylingo/internal/config"
)

// newTestLLMClient builds a DefaultLLMClient pointed at the given httptest server URL.
func newTestLLMClient(serverURL string) *DefaultLLMClient {
	cfg := openai.DefaultConfig("test-api-key")
	cfg.BaseURL = serverURL + "/v1"
	return &DefaultLLMClient{
		client: openai.NewClientWithConfig(cfg),
		model:  "test-model",
	}
}

func TestNewLLMClient(t *testing.T) {
	t.Parallel()

	t.Run("uses configured model and custom base URL", func(t *testing.T) {
		t.Parallel()
		cfg := &config.Config{}
		cfg.LLM.APIKey = "key"
		cfg.LLM.BaseURL = "https://example.com/v1"
		cfg.LLM.Model = "my-model"

		client := NewLLMClient(cfg)
		dc, ok := client.(*DefaultLLMClient)
		if !ok {
			t.Fatalf("NewLLMClient returned %T, want *DefaultLLMClient", client)
		}
		if dc.model != "my-model" {
			t.Fatalf("model = %q, want my-model", dc.model)
		}
		if dc.client == nil {
			t.Fatal("client is nil")
		}
	})

	t.Run("empty base URL keeps default and is still usable", func(t *testing.T) {
		t.Parallel()
		cfg := &config.Config{}
		cfg.LLM.APIKey = "key"
		cfg.LLM.Model = "m"

		dc, ok := NewLLMClient(cfg).(*DefaultLLMClient)
		if !ok || dc.client == nil {
			t.Fatal("NewLLMClient with empty base URL produced unusable client")
		}
	})
}

func TestAnswerLearningQuestion_Errors(t *testing.T) {
	t.Parallel()

	t.Run("missing config", func(t *testing.T) {
		t.Parallel()
		client := &DefaultLLMClient{}
		_, err := client.AnswerLearningQuestion(context.Background(), "q")
		if !errors.Is(err, ErrAIConfigMissing) {
			t.Fatalf("err = %v, want ErrAIConfigMissing", err)
		}
	})

	t.Run("HTTP 500 wraps request error", func(t *testing.T) {
		t.Parallel()
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusInternalServerError)
		}))
		defer server.Close()

		_, err := newTestLLMClient(server.URL).AnswerLearningQuestion(context.Background(), "q")
		if err == nil || !strings.Contains(err.Error(), "llm learning question request failed") {
			t.Fatalf("err = %v, want wrapped request error", err)
		}
	})

	t.Run("empty choices", func(t *testing.T) {
		t.Parallel()
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"choices": []}`))
		}))
		defer server.Close()

		_, err := newTestLLMClient(server.URL).AnswerLearningQuestion(context.Background(), "q")
		if err == nil || !strings.Contains(err.Error(), "empty llm learning question response") {
			t.Fatalf("err = %v, want empty response error", err)
		}
	})
}

func TestGradeHandwriting_Errors(t *testing.T) {
	t.Parallel()

	t.Run("missing config", func(t *testing.T) {
		t.Parallel()
		client := &DefaultLLMClient{}
		_, err := client.GradeHandwriting(context.Background(), "p", "オ", []byte("png"))
		if !errors.Is(err, ErrAIConfigMissing) {
			t.Fatalf("err = %v, want ErrAIConfigMissing", err)
		}
	})

	t.Run("HTTP 500 wraps request error", func(t *testing.T) {
		t.Parallel()
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusInternalServerError)
		}))
		defer server.Close()

		_, err := newTestLLMClient(server.URL).GradeHandwriting(context.Background(), "p", "オ", []byte("png"))
		if err == nil || !strings.Contains(err.Error(), "llm handwriting grading request failed") {
			t.Fatalf("err = %v, want wrapped request error", err)
		}
	})

	t.Run("empty choices", func(t *testing.T) {
		t.Parallel()
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"choices": []}`))
		}))
		defer server.Close()

		_, err := newTestLLMClient(server.URL).GradeHandwriting(context.Background(), "p", "オ", []byte("png"))
		if err == nil || !strings.Contains(err.Error(), "empty llm handwriting response") {
			t.Fatalf("err = %v, want empty response error", err)
		}
	})

	t.Run("invalid JSON content", func(t *testing.T) {
		t.Parallel()
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"choices":[{"message":{"content":"not json"}}]}`))
		}))
		defer server.Close()

		_, err := newTestLLMClient(server.URL).GradeHandwriting(context.Background(), "p", "オ", []byte("png"))
		if err == nil || !strings.Contains(err.Error(), "failed to parse llm handwriting output") {
			t.Fatalf("err = %v, want parse error", err)
		}
	})
}

func TestGradeAnswer_EmptyChoices(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"choices": []}`))
	}))
	defer server.Close()

	_, err := newTestLLMClient(server.URL).GradeAnswer(context.Background(), "p", "a", "u")
	if err == nil || !strings.Contains(err.Error(), "empty llm response") {
		t.Fatalf("err = %v, want empty llm response error", err)
	}
}
