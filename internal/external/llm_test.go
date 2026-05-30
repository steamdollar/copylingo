package external

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/sashabaranov/go-openai"
)

func TestBuildHandwritingSystemPromptSupportsShortWords(t *testing.T) {
	t.Parallel()

	prompt := buildHandwritingSystemPrompt()

	if strings.Contains(prompt, "single kana") {
		t.Fatalf("system prompt still assumes only single kana: %q", prompt)
	}
	if !strings.Contains(prompt, "short kana word") {
		t.Fatalf("system prompt does not mention short kana word: %q", prompt)
	}
	if !strings.Contains(prompt, "full expected string") {
		t.Fatalf("system prompt does not require full string comparison: %q", prompt)
	}
	if !strings.Contains(prompt, "not open-ended OCR") {
		t.Fatalf("system prompt does not state binary verification boundary: %q", prompt)
	}
}

func TestBuildHandwritingSystemPromptDefinesFeedbackPolicy(t *testing.T) {
	t.Parallel()

	prompt := buildHandwritingSystemPrompt()

	for _, want := range []string{
		"If is_correct is true, feedback must be an empty string",
		"one short Korean correction note only when a reliable note exists",
		"Explain only which expected feature is clearly missing or wrong",
		"Do not propose, transcribe, or mention an alternative character",
		"If no reliable correction note exists, return an empty string",
	} {
		if !strings.Contains(prompt, want) {
			t.Fatalf("system prompt does not contain feedback policy %q: %q", want, prompt)
		}
	}
}

func TestBuildHandwritingSystemPromptDefinesConditionalVerificationPolicy(t *testing.T) {
	t.Parallel()

	prompt := buildHandwritingSystemPrompt()

	for _, want := range []string{
		"conditional verification against the provided Expected Text, not open-ended OCR",
		"Default to true when the Expected Text is a plausible reading",
		"Do not search for or prefer an alternative transcription",
		"another kana or kanji",
		"ambiguous small kana or diacritic marks when plausibly present",
		"full expected string in order",
		"cannot plausibly be read as the Expected Text",
		"Apply this principle generally, not only to this example",
		"Expected Text: オ",
		"visually similar kanji 才",
		"Since オ remains a plausible reading, return true",
	} {
		if !strings.Contains(prompt, want) {
			t.Fatalf("system prompt does not contain conditional verification policy %q: %q", want, prompt)
		}
	}
}

func TestBuildHandwritingUserPromptIncludesContextAndExpectedText(t *testing.T) {
	t.Parallel()

	questionPrompt := "뜻 <b>'학교'</b>에 해당하는 일본어 단어를 손글씨로 쓰세요"
	correctAnswer := "がっこう"
	prompt := buildHandwritingUserPrompt(questionPrompt, correctAnswer)

	if !strings.Contains(prompt, "Expected Text") {
		t.Fatalf("user prompt does not include Expected Text label: %q", prompt)
	}
	if !strings.Contains(prompt, questionPrompt) {
		t.Fatalf("user prompt does not include question prompt: %q", prompt)
	}
	if !strings.Contains(prompt, correctAnswer) {
		t.Fatalf("user prompt does not include correct answer: %q", prompt)
	}
}

func TestBuildHandwritingResponseFormatUsesStrictJSONSchema(t *testing.T) {
	t.Parallel()

	format := buildHandwritingResponseFormat()

	if format.Type != openai.ChatCompletionResponseFormatTypeJSONSchema {
		t.Fatalf("response format type = %q, want %q", format.Type, openai.ChatCompletionResponseFormatTypeJSONSchema)
	}
	if format.JSONSchema == nil {
		t.Fatal("response format JSONSchema is nil")
	}
	if !format.JSONSchema.Strict {
		t.Fatal("response format JSONSchema Strict = false, want true")
	}
	if format.JSONSchema.Name != "handwriting_grade_result" {
		t.Fatalf("response format schema name = %q", format.JSONSchema.Name)
	}

	schemaBytes, err := format.JSONSchema.Schema.MarshalJSON()
	if err != nil {
		t.Fatalf("marshal response format schema: %v", err)
	}

	var schema struct {
		Type                 string                    `json:"type"`
		Properties           map[string]map[string]any `json:"properties"`
		Required             []string                  `json:"required"`
		AdditionalProperties bool                      `json:"additionalProperties"`
	}
	if err := json.Unmarshal(schemaBytes, &schema); err != nil {
		t.Fatalf("unmarshal response format schema: %v", err)
	}
	if schema.Type != "object" {
		t.Fatalf("schema type = %q, want object", schema.Type)
	}
	if schema.AdditionalProperties {
		t.Fatal("schema additionalProperties = true, want false")
	}
	if got := schema.Properties["is_correct"]["type"]; got != "boolean" {
		t.Fatalf("is_correct type = %v, want boolean", got)
	}
	if got := schema.Properties["feedback"]["type"]; got != "string" {
		t.Fatalf("feedback type = %v, want string", got)
	}
	if description, ok := schema.Properties["feedback"]["description"].(string); !ok || !strings.Contains(description, "Empty when correct") {
		t.Fatalf("feedback description = %v, want Empty when correct policy", schema.Properties["feedback"]["description"])
	}
	if len(schema.Required) != 2 || schema.Required[0] != "is_correct" || schema.Required[1] != "feedback" {
		t.Fatalf("schema required = %v, want [is_correct feedback]", schema.Required)
	}
}

func TestBuildHandwritingChatCompletionRequestConstrainsGeneration(t *testing.T) {
	t.Parallel()

	req := buildHandwritingChatCompletionRequest(
		"gemini-3.1-flash-lite",
		"system prompt",
		"user prompt",
		"data:image/png;base64,abc",
	)

	if req.MaxCompletionTokens != handwritingMaxCompletionTokens {
		t.Fatalf("MaxCompletionTokens = %d, want %d", req.MaxCompletionTokens, handwritingMaxCompletionTokens)
	}
	if req.ReasoningEffort != "" {
		t.Fatalf("ReasoningEffort = %q, want empty", req.ReasoningEffort)
	}
	if req.Temperature != handwritingTemperature {
		t.Fatalf("Temperature = %v, want %v", req.Temperature, handwritingTemperature)
	}
	if req.ResponseFormat == nil || req.ResponseFormat.Type != openai.ChatCompletionResponseFormatTypeJSONSchema {
		t.Fatalf("ResponseFormat = %#v, want JSON schema", req.ResponseFormat)
	}
	if len(req.Messages) != 2 {
		t.Fatalf("messages length = %d, want 2", len(req.Messages))
	}
	if len(req.Messages[1].MultiContent) != 2 {
		t.Fatalf("user multi content length = %d, want 2", len(req.Messages[1].MultiContent))
	}
	imagePart := req.Messages[1].MultiContent[1]
	if imagePart.ImageURL == nil {
		t.Fatal("image part ImageURL is nil")
	}
	if imagePart.ImageURL.Detail != openai.ImageURLDetailLow {
		t.Fatalf("image detail = %q, want %q", imagePart.ImageURL.Detail, openai.ImageURLDetailLow)
	}
}

func TestGradeHandwritingReturnsProviderFeedback(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/chat/completions" {
			t.Fatalf("request path = %q, want /v1/chat/completions", r.URL.Path)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{
			"choices": [{
				"message": {
					"content": "{\"is_correct\":false,\"feedback\":\"탁점이 빠졌습니다.\"}"
				}
			}]
		}`))
	}))
	defer server.Close()

	cfg := openai.DefaultConfig("test-api-key")
	cfg.BaseURL = server.URL + "/v1"
	client := &DefaultLLMClient{
		client: openai.NewClientWithConfig(cfg),
		model:  "test-model",
	}

	isCorrect, feedback, err := client.GradeHandwriting(context.Background(), "prompt", "オ", []byte("png"))
	if err != nil {
		t.Fatalf("GradeHandwriting() error = %v", err)
	}
	if isCorrect {
		t.Fatal("GradeHandwriting() isCorrect = true, want false")
	}
	if feedback != "탁점이 빠졌습니다." {
		t.Fatalf("GradeHandwriting() feedback = %q, want correction note", feedback)
	}
}

func TestGradeAnswer_Success(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{
			"choices": [{
				"message": {
					"content": "{\"is_correct\":true,\"feedback\":\"잘 하셨습니다.\"}"
				}
			}]
		}`))
	}))
	defer server.Close()

	cfg := openai.DefaultConfig("test-api-key")
	cfg.BaseURL = server.URL + "/v1"
	client := &DefaultLLMClient{
		client: openai.NewClientWithConfig(cfg),
		model:  "test-model",
	}

	isCorrect, feedback, err := client.GradeAnswer(context.Background(), "prompt", "apple", "apple")
	if err != nil {
		t.Fatalf("GradeAnswer() error = %v", err)
	}
	if !isCorrect {
		t.Fatal("GradeAnswer() isCorrect = false, want true")
	}
	if !strings.Contains(feedback, "잘 하셨습니다") {
		t.Fatalf("GradeAnswer() feedback = %q", feedback)
	}
}

func TestDefaultLLMClient_Errors(t *testing.T) {
	t.Parallel()

	t.Run("Missing config", func(t *testing.T) {
		client := &DefaultLLMClient{}
		_, _, err := client.GradeAnswer(context.Background(), "p", "a", "u")
		if !strings.Contains(err.Error(), "ai system is not configured") {
			t.Errorf("expected ErrAIConfigMissing, got %v", err)
		}
	})

	t.Run("HTTP 500", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusInternalServerError)
		}))
		defer server.Close()

		cfg := openai.DefaultConfig("test-api-key")
		cfg.BaseURL = server.URL + "/v1"
		client := &DefaultLLMClient{
			client: openai.NewClientWithConfig(cfg),
			model:  "test-model",
		}

		_, _, err := client.GradeAnswer(context.Background(), "p", "a", "u")
		if err == nil || !strings.Contains(err.Error(), "llm grading request failed") {
			t.Errorf("expected HTTP error, got %v", err)
		}
	})

	t.Run("Invalid JSON response", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{
				"choices": [{
					"message": {
						"content": "invalid json"
					}
				}]
			}`))
		}))
		defer server.Close()

		cfg := openai.DefaultConfig("test-api-key")
		cfg.BaseURL = server.URL + "/v1"
		client := &DefaultLLMClient{
			client: openai.NewClientWithConfig(cfg),
			model:  "test-model",
		}

		_, _, err := client.GradeAnswer(context.Background(), "p", "a", "u")
		if err == nil || !strings.Contains(err.Error(), "failed to parse llm output") {
			t.Errorf("expected parsing error, got %v", err)
		}
	})
}
