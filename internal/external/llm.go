package external

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/lsj/copylingo/internal/config"
	"github.com/sashabaranov/go-openai"
)

// LLMClient defines the interface for interacting with Language Models.
type LLMClient interface {
	GradeAnswer(ctx context.Context, questionPrompt, correctAnswer, userAnswer string) (bool, string, error)
}

// GradeResult represents the structured JSON output from the LLM.
type GradeResult struct {
	IsCorrect bool   `json:"is_correct"`
	Feedback  string `json:"feedback"`
}

type DefaultLLMClient struct {
	client *openai.Client
	model  string
}

// NewLLMClient initializes an LLMClient using the OpenAI compatible API.
func NewLLMClient(cfg *config.Config) LLMClient {
	config := openai.DefaultConfig(cfg.LLM.APIKey)
	if cfg.LLM.BaseURL != "" {
		config.BaseURL = cfg.LLM.BaseURL
	}

	return &DefaultLLMClient{
		client: openai.NewClientWithConfig(config),
		model:  cfg.LLM.Model,
	}
}

// GradeAnswer evaluates a subjective similarity question using the LLM.
func (c *DefaultLLMClient) GradeAnswer(ctx context.Context, questionPrompt, correctAnswer, userAnswer string) (bool, string, error) {
	if c.client == nil || c.model == "" {
		return false, "", config.ErrAIConfigMissing
	}

	systemPrompt := `You are an expert language teacher grading a student's answer.
You must return your evaluation in strict JSON format. Do not use markdown blocks.

JSON schema:
{
  "is_correct": boolean,
  "feedback": "string (Short Korean feedback explaining the result)"
}

Rules for grading:
1. 'is_correct' should be true if the user's answer demonstrates the correct knowledge, even with minor typos, as long as it doesn't change the meaning.
2. If it is completely wrong or conceptually incorrect, set 'is_correct' to false.
3. The 'feedback' should be encouraging but direct in Korean.`

	userPrompt := fmt.Sprintf(`Question Context: %s
Expected Correct Answer: %s
User's Answer: %s

Evaluate the User's Answer against the Expected Correct Answer and output JSON.`, questionPrompt, correctAnswer, userAnswer)

	resp, err := c.client.CreateChatCompletion(ctx, openai.ChatCompletionRequest{
		Model: c.model,
		Messages: []openai.ChatCompletionMessage{
			{
				Role:    openai.ChatMessageRoleSystem,
				Content: systemPrompt,
			},
			{
				Role:    openai.ChatMessageRoleUser,
				Content: userPrompt,
			},
		},
		ResponseFormat: &openai.ChatCompletionResponseFormat{
			Type: openai.ChatCompletionResponseFormatTypeJSONObject,
		},
	})

	if err != nil {
		return false, "", fmt.Errorf("llm grading request failed: %w", err)
	}

	if len(resp.Choices) == 0 {
		return false, "", fmt.Errorf("empty llm response")
	}

	rawContent := resp.Choices[0].Message.Content

	var result GradeResult
	if err := json.Unmarshal([]byte(rawContent), &result); err != nil {
		return false, "", fmt.Errorf("failed to parse llm output (%s): %w", rawContent, err)
	}

	return result.IsCorrect, result.Feedback, nil
}
