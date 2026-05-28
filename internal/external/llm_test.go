package external

import (
	"encoding/json"
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
}

func TestBuildHandwritingSystemPromptDefinesFeedbackPolicy(t *testing.T) {
	t.Parallel()

	prompt := buildHandwritingSystemPrompt()

	for _, want := range []string{
		"feedback must be an empty string",
		"do not repeat the expected text",
		"one short Korean sentence",
		"Do not praise",
	} {
		if !strings.Contains(prompt, want) {
			t.Fatalf("system prompt does not contain feedback policy %q: %q", want, prompt)
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
