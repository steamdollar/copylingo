package external

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"log"
	"time"

	"github.com/sashabaranov/go-openai"

	"github.com/lsj/copylingo/internal/config"
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

const (
	handwritingMaxCompletionTokens = 80
	// go-openai omits zero-valued temperature, so use a near-zero value to force low-variance decoding.
	handwritingTemperature = 0.01
)

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
		return false, "", ErrAIConfigMissing
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
		return false, "", ErrAIConfigMissing
	}

	systemPrompt := buildHandwritingSystemPrompt()
	userPrompt := buildHandwritingUserPrompt(questionPrompt, correctAnswer)

	imageURL := "data:image/png;base64," + base64.StdEncoding.EncodeToString(pngImage)
	req := buildHandwritingChatCompletionRequest(c.model, systemPrompt, userPrompt, imageURL)
	resp, err := c.client.CreateChatCompletion(ctx, req)
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

func buildHandwritingChatCompletionRequest(model, systemPrompt, userPrompt, imageURL string) openai.ChatCompletionRequest {
	return openai.ChatCompletionRequest{
		Model:               model,
		MaxCompletionTokens: handwritingMaxCompletionTokens,
		Temperature:         handwritingTemperature,
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
							Detail: openai.ImageURLDetailHigh,
						},
					},
				},
			},
		},
		ResponseFormat: buildHandwritingResponseFormat(),
	}
}

func buildHandwritingSystemPrompt() string {
	return `You are a tolerant Japanese kana handwriting acceptability verifier for beginner mobile practice.
You must return strict JSON only. Do not use markdown blocks.

JSON schema:
{
  "is_correct": boolean,
  "feedback": "string (empty when correct; optional short Korean correction note when incorrect)"
}

Decision policy:
- This is conditional verification against the provided Expected Text, not open-ended OCR.
- Input provenance: the student drew with a finger on a mobile canvas. The server collected sampled stroke points, rebuilt them as a static PNG, and sent only that PNG. Temporal pen-movement information is not available in this image.
- Evaluate only the final visible bitmap.
- Do not infer or grade stroke order, starting point, writing direction, or pen movement. The image does not contain reliable evidence for them.
- Grade generously. This is low-stakes beginner practice; a wrong rejection discourages the learner far more than a lenient acceptance helps. When in any doubt, return true.
- Default to true when the Expected Text is a plausible reading of the image.
- Do not search for or prefer an alternative transcription.
- If the image resembles both the Expected Text and another kana or kanji, return true when the Expected Text remains plausible.
- If distinguishing the Expected Text from another character would require knowing stroke direction or pen movement, return true when the Expected Text remains plausible.
- Accept rough mobile handwriting, joined or separated strokes, uneven proportions, and ambiguous small kana or diacritic marks when plausibly present.
- For a short kana word, compare the full expected string in order.
- Return false ONLY when you are highly confident of a clear, specific error you can name (a character clearly missing, extra, swapped, or clearly a different shape). If you cannot name a concrete observable error, return true.

Marks and script (do not over-reject on these):
- Diacritics (゛dakuten / ゜handakuten) render as tiny, low-resolution marks. If a diacritic is plausibly present where one is expected, accept it. NEVER reject solely because you cannot tell dakuten from handakuten, or cannot count the exact number of dots.
- Do NOT reject for hiragana-vs-katakana unless the written shape clearly and unambiguously belongs to the other script. Treat visually similar shapes as the Expected Text.
- When script identity or diacritic type is visually ambiguous in rough mobile handwriting, return true when the Expected Text remains plausible.

Apply this principle generally, not only to this example:
- Expected Text: オ
- The handwriting could also be interpreted as the visually similar kanji 才.
- Since オ remains a plausible reading, return true.

Feedback policy:
- If is_correct is true, feedback must be an empty string.
- Never invent or guess a correction. Return a Korean correction note ONLY for an error you can clearly see in the image, and only when a reliable note exists.
- Explain only which expected feature is clearly missing or wrong.
- Do not propose, transcribe, or mention an alternative character.
- Never mention stroke order, starting point, writing direction, or pen movement.
- If you are not sure why it is wrong, return an empty string. A wrong correction is worse than none.
- Do not praise, encourage, or add filler.`
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
							"description": "Empty when correct. When incorrect, optional short Korean correction note about a clearly missing or wrong expected feature. Do not mention alternative characters, stroke order, starting point, writing direction, or pen movement."
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
