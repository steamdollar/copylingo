package external

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"log"
	"time"

	"github.com/lsj/copylingo/internal/config"
	"github.com/sashabaranov/go-openai"
)

// LLMClient defines AI-backed grading paths that cannot be handled by exact string matching.
type LLMClient interface {
	// GradeAnswer is for QuestionSubjective only: free-text semantic grading such as translated meaning or paraphrased answers.
	GradeAnswer(ctx context.Context, questionPrompt, correctAnswer, userAnswer string) (bool, string, error)
	// GradeHandwriting is for QuestionKanaHandwriting only: binary visual verification of a rendered handwriting PNG.
	GradeHandwriting(ctx context.Context, questionPrompt, correctAnswer string, pngImage []byte) (bool, string, error)
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

// GradeAnswer evaluates a QuestionSubjective free-text answer by semantic similarity.
// Fill-blank and multiple-choice answers are graded by exact string matching in GraderService.
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

// GradeHandwriting verifies whether a rendered handwriting image matches the expected Japanese text.
func (c *DefaultLLMClient) GradeHandwriting(ctx context.Context, questionPrompt, correctAnswer string, pngImage []byte) (bool, string, error) {
	startedAt := time.Now()

	if c.client == nil || c.model == "" {
		return false, "", config.ErrAIConfigMissing
	}

	systemPrompt := buildHandwritingSystemPrompt()
	userPrompt := buildHandwritingUserPrompt(questionPrompt, correctAnswer)

	imageURL := "data:image/png;base64," + base64.StdEncoding.EncodeToString(pngImage)
	resp, err := c.client.CreateChatCompletion(ctx, openai.ChatCompletionRequest{
		Model: c.model,
		Messages: []openai.ChatCompletionMessage{
			{
				Role:    openai.ChatMessageRoleSystem,
				Content: systemPrompt,
			},
			{
				Role: openai.ChatMessageRoleUser,
				MultiContent: []openai.ChatMessagePart{
					{
						Type: openai.ChatMessagePartTypeText,
						Text: userPrompt,
					},
					{
						Type: openai.ChatMessagePartTypeImageURL,
						ImageURL: &openai.ChatMessageImageURL{
							URL:    imageURL,
							Detail: openai.ImageURLDetailLow,
						},
					},
				},
			},
		},
		ResponseFormat: buildHandwritingResponseFormat(),
	})
	if err != nil {
		return false, "", fmt.Errorf("llm handwriting grading request failed: %w", err)
	}
	if len(resp.Choices) == 0 {
		return false, "", fmt.Errorf("empty llm handwriting response")
	}

	rawContent := resp.Choices[0].Message.Content
	var result GradeResult
	if err := json.Unmarshal([]byte(rawContent), &result); err != nil {
		return false, "", fmt.Errorf("failed to parse llm handwriting output (%s): %w", rawContent, err)
	}
	log.Printf("[Handwriting] llm model=%s elapsed=%s image_bytes=%d is_correct=%t", c.model, time.Since(startedAt), len(pngImage), result.IsCorrect)

	return result.IsCorrect, result.Feedback, nil
}

func buildHandwritingSystemPrompt() string {
	return `You are an expert Japanese handwriting grader.
You must return strict JSON only. Do not use markdown blocks.

JSON schema:
{
  "is_correct": boolean,
  "feedback": "string (empty when correct; optional short Korean correction note when incorrect)"
}

Rules:
1. This is binary verification, not open-ended OCR.
2. The image contains one centered handwritten Japanese kana character or short kana word in black on white.
3. Decide whether the handwritten image can reasonably be accepted as the expected text.
4. Accept minor wobble, uneven stroke width, rounded corners, and imperfect mobile handwriting.
5. Do not reject only because the drawing is large, small, slightly tilted, or not aesthetically neat.
6. For short words, compare the full expected string in order; reject missing, extra, swapped, or clearly different characters.
7. When the shape is close enough for a human teacher to accept in beginner practice, return true.
8. Feedback policy:
   - If is_correct is true, feedback must be an empty string.
   - If is_correct is false, do not repeat the expected text; the client already shows the correct answer.
   - For incorrect answers, feedback may be an empty string, or one short Korean sentence only when there is a useful correction note.
   - Do not praise, encourage, or add filler such as "잘 썼어요" or "아주 좋아요".`
}

func buildHandwritingResponseFormat() *openai.ChatCompletionResponseFormat {
	return &openai.ChatCompletionResponseFormat{
		Type: openai.ChatCompletionResponseFormatTypeJSONSchema,
		JSONSchema: &openai.ChatCompletionResponseFormatJSONSchema{
			Name:        "handwriting_grade_result",
			Description: "Binary Japanese kana handwriting grading result.",
			Strict:      true,
			Schema: json.RawMessage(`{
				"type": "object",
				"properties": {
					"is_correct": { "type": "boolean" },
					"feedback": {
						"type": "string",
						"description": "Empty when correct. When incorrect, optional short Korean correction note without repeating the expected text."
					}
				},
				"required": ["is_correct", "feedback"],
				"additionalProperties": false
			}`),
		},
	}
}

func buildHandwritingUserPrompt(questionPrompt, correctAnswer string) string {
	return fmt.Sprintf(`Question Context: %s
Expected Text: %s

Evaluate whether the handwriting image matches the Expected Text and output JSON.`, questionPrompt, correctAnswer)
}
