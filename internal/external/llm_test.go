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
		"Feedback is empty by default, including for an incorrect result",
		"set feedback to an empty string. The app will show the Expected Text without an additional correction",
		"Never write speculative or hedged feedback",
		"Do not propose, transcribe, or mention an alternative character",
		"Never mention stroke order, starting point, writing direction, or pen movement",
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
		"This is an acceptance-first decision. The default is is_correct=true",
		"If the image is ambiguous, low-resolution, partially clipped, rough, or plausibly matches the Expected Text, return true",
		"Do not search for or prefer an alternative transcription",
		"another kana or kanji",
		"ambiguous small kana or diacritic marks when plausibly present",
		"compare the full expected string, but do not reject for spacing",
		"Return false ONLY when you can name one concrete, observable defect in the final bitmap with high confidence",
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

func TestBuildHandwritingSystemPromptDefinesStaticPNGEvidenceBoundary(t *testing.T) {
	t.Parallel()

	prompt := buildHandwritingSystemPrompt()

	for _, want := range []string{
		"student drew with a finger on a mobile canvas",
		"server collected sampled stroke points, rebuilt them as a static PNG, and sent only that PNG",
		"Temporal pen-movement information is not available",
		"Evaluate only the final visible bitmap",
		"Do not infer or grade stroke order, starting point, writing direction, or pen movement",
		"require knowing stroke direction or pen movement",
		"When script identity or diacritic type is visually ambiguous in rough mobile handwriting",
	} {
		if !strings.Contains(prompt, want) {
			t.Fatalf("system prompt does not contain static PNG evidence boundary %q: %q", want, prompt)
		}
	}
}

func TestBuildHandwritingSystemPromptDefinesYoonTolerance(t *testing.T) {
	t.Parallel()

	prompt := buildHandwritingSystemPrompt()

	for _, want := range []string{
		"Contracted sounds (yoon; small ゃ / ゅ / ょ or ャ / ュ / ョ)",
		"Do NOT require textbook size, proportions, or exact shape",
		"a plausible second small mark is present in the expected position",
		"ONLY when the small kana is clearly absent or clearly replaced by an unrelated shape",
		"Do NOT claim that a small kana is full-sized, malformed, or wrong unless that is unambiguous from the final bitmap",
	} {
		if !strings.Contains(prompt, want) {
			t.Fatalf("system prompt does not contain yoon tolerance policy %q: %q", want, prompt)
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
	if description, ok := schema.Properties["feedback"]["description"].(string); !ok ||
		!strings.Contains(description, "Empty by default") {
		t.Fatalf(
			"feedback description = %v, want empty-by-default policy",
			schema.Properties["feedback"]["description"],
		)
	}
	if description := schema.Properties["feedback"]["description"].(string); !strings.Contains(
		description,
		"stroke order",
	) {
		t.Fatalf("feedback description = %q, want static PNG evidence boundary", description)
	}
	if len(schema.Required) != 2 || schema.Required[0] != "is_correct" || schema.Required[1] != "feedback" {
		t.Fatalf("schema required = %v, want [is_correct feedback]", schema.Required)
	}
}

func TestBuildHandwritingChatCompletionRequestConstrainsGeneration(t *testing.T) {
	t.Parallel()

	req := buildHandwritingChatCompletionRequest(
		"gemini-3.5-flash-lite",
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
	if req.Temperature != 0 {
		t.Fatalf("Temperature = %v, want omitted zero value", req.Temperature)
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
	if imagePart.ImageURL.Detail != openai.ImageURLDetailHigh {
		t.Fatalf("image detail = %q, want %q", imagePart.ImageURL.Detail, openai.ImageURLDetailHigh)
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

	result, err := client.GradeHandwriting(context.Background(), "prompt", "オ", []byte("png"))
	if err != nil {
		t.Fatalf("GradeHandwriting() error = %v", err)
	}
	if result.IsCorrect {
		t.Fatal("GradeHandwriting() isCorrect = true, want false")
	}
	if result.Feedback != "탁점이 빠졌습니다." {
		t.Fatalf("GradeHandwriting() feedback = %q, want correction note", result.Feedback)
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

	result, err := client.GradeAnswer(context.Background(), "prompt", "apple", "apple")
	if err != nil {
		t.Fatalf("GradeAnswer() error = %v", err)
	}
	if !result.IsCorrect {
		t.Fatal("GradeAnswer() isCorrect = false, want true")
	}
	if !strings.Contains(result.Feedback, "잘 하셨습니다") {
		t.Fatalf("GradeAnswer() feedback = %q", result.Feedback)
	}
}

func TestAnswerLearningQuestionSuccess(t *testing.T) {
	t.Parallel()

	var req openai.ChatCompletionRequest
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			t.Fatalf("decode request: %v", err)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{
			"choices": [{
				"message": {
					"content": "honoo는 일본어로 불꽃이라는 뜻입니다."
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

	answer, err := client.AnswerLearningQuestion(context.Background(), "honoo가 뭐야?")
	if err != nil {
		t.Fatalf("AnswerLearningQuestion() error = %v", err)
	}
	if !strings.Contains(answer, "불꽃") {
		t.Fatalf("AnswerLearningQuestion() answer = %q", answer)
	}
	if req.Model != "test-model" {
		t.Fatalf("request model = %q, want test-model", req.Model)
	}
	if req.MaxCompletionTokens != learningQuestionMaxTokens {
		t.Fatalf("MaxCompletionTokens = %d, want %d", req.MaxCompletionTokens, learningQuestionMaxTokens)
	}
	if len(req.Messages) != 2 || req.Messages[1].Content != "honoo가 뭐야?" {
		t.Fatalf("request messages = %+v", req.Messages)
	}
}

func TestDefaultLLMClient_Errors(t *testing.T) {
	t.Parallel()

	t.Run("Missing config", func(t *testing.T) {
		client := &DefaultLLMClient{}
		_, err := client.GradeAnswer(context.Background(), "p", "a", "u")
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

		_, err := client.GradeAnswer(context.Background(), "p", "a", "u")
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

		_, err := client.GradeAnswer(context.Background(), "p", "a", "u")
		if err == nil || !strings.Contains(err.Error(), "failed to parse llm output") {
			t.Errorf("expected parsing error, got %v", err)
		}
	})
}
