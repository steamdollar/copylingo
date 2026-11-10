package external

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/sashabaranov/go-openai"
)

// newTestLLMClient builds a DefaultLLMClient pointed at the given httptest server URL.
func newTestLLMClient(serverURL string) *DefaultLLMClient {
	clientConfig := openai.DefaultConfig("test-api-key")
	clientConfig.BaseURL = serverURL + "/v1"
	return &DefaultLLMClient{
		client: openai.NewClientWithConfig(clientConfig),
		model:  "test-model",
	}
}

func TestNewLLMClient(t *testing.T) {
	t.Parallel()

	t.Run(
		"uses configured model and custom base URL",
		func(t *testing.T) {
			t.Parallel()
			client := NewLLMClient(LLMOptions{
				APIKey:  "key",
				BaseURL: "https://example.com/v1",
				Model:   "my-model",
			})
			if client.model != "my-model" {
				t.Fatalf(
					"model = %q, want my-model",
					client.model,
				)
			}
			if client.client == nil {
				t.Fatal("client is nil")
			}
		},
	)

	t.Run(
		"empty base URL keeps default and is still usable",
		func(t *testing.T) {
			t.Parallel()
			client := NewLLMClient(LLMOptions{
				APIKey: "key",
				Model:  "m",
			})
			if client.client == nil {
				t.Fatal("NewLLMClient with empty base URL produced unusable client")
			}
		},
	)
}

func TestAnswerLearningQuestion_Errors(t *testing.T) {
	t.Parallel()

	t.Run(
		"HTTP 500 wraps request error",
		func(t *testing.T) {
			t.Parallel()
			server := httptest.NewServer(http.HandlerFunc(func(
				responseWriter http.ResponseWriter,
				request *http.Request,
			) {
				responseWriter.WriteHeader(http.StatusInternalServerError)
			}))
			defer server.Close()

			_, err := newTestLLMClient(server.URL).AnswerLearningQuestion(
				context.Background(),
				"q",
			)
			if err == nil || !strings.Contains(
				err.Error(),
				"llm learning question request failed",
			) {
				t.Fatalf(
					"err = %v, want wrapped request error",
					err,
				)
			}
		},
	)

	t.Run(
		"empty choices",
		func(t *testing.T) {
			t.Parallel()
			server := httptest.NewServer(http.HandlerFunc(func(
				responseWriter http.ResponseWriter,
				request *http.Request,
			) {
				responseWriter.Header().Set(
					"Content-Type",
					"application/json",
				)
				_, _ = responseWriter.Write([]byte(`{"choices": []}`))
			}))
			defer server.Close()

			_, err := newTestLLMClient(server.URL).AnswerLearningQuestion(
				context.Background(),
				"q",
			)
			if err == nil || !strings.Contains(
				err.Error(),
				"empty llm learning question response",
			) {
				t.Fatalf(
					"err = %v, want empty response error",
					err,
				)
			}
		},
	)
}

func TestGradeHandwriting_Errors(t *testing.T) {
	t.Parallel()

	t.Run(
		"HTTP 500 wraps request error",
		func(t *testing.T) {
			t.Parallel()
			server := httptest.NewServer(http.HandlerFunc(func(
				responseWriter http.ResponseWriter,
				request *http.Request,
			) {
				responseWriter.WriteHeader(http.StatusInternalServerError)
			}))
			defer server.Close()

			_, err := newTestLLMClient(server.URL).GradeHandwriting(
				context.Background(),
				"p",
				"オ",
				[]byte("png"),
			)
			if err == nil || !strings.Contains(
				err.Error(),
				"llm handwriting grading request failed",
			) {
				t.Fatalf(
					"err = %v, want wrapped request error",
					err,
				)
			}
		},
	)

	t.Run(
		"empty choices",
		func(t *testing.T) {
			t.Parallel()
			server := httptest.NewServer(http.HandlerFunc(func(
				responseWriter http.ResponseWriter,
				request *http.Request,
			) {
				responseWriter.Header().Set(
					"Content-Type",
					"application/json",
				)
				_, _ = responseWriter.Write([]byte(`{"choices": []}`))
			}))
			defer server.Close()

			_, err := newTestLLMClient(server.URL).GradeHandwriting(
				context.Background(),
				"p",
				"オ",
				[]byte("png"),
			)
			if err == nil || !strings.Contains(
				err.Error(),
				"empty llm handwriting response",
			) {
				t.Fatalf(
					"err = %v, want empty response error",
					err,
				)
			}
		},
	)

	t.Run(
		"invalid JSON content",
		func(t *testing.T) {
			t.Parallel()
			server := httptest.NewServer(http.HandlerFunc(func(
				responseWriter http.ResponseWriter,
				request *http.Request,
			) {
				responseWriter.Header().Set(
					"Content-Type",
					"application/json",
				)
				_, _ = responseWriter.Write([]byte(`{"choices":[{"message":{"content":"not json"}}]}`))
			}))
			defer server.Close()

			_, err := newTestLLMClient(server.URL).GradeHandwriting(
				context.Background(),
				"p",
				"オ",
				[]byte("png"),
			)
			if err == nil || !strings.Contains(
				err.Error(),
				"failed to parse llm handwriting output",
			) {
				t.Fatalf(
					"err = %v, want parse error",
					err,
				)
			}
		},
	)
}
