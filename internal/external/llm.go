package external

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/sashabaranov/go-openai"

	"github.com/lsj/copylingo/internal/config"
	"github.com/lsj/copylingo/internal/model"
	"github.com/lsj/copylingo/internal/observability"
)

// LLMClient defines AI-backed grading paths that cannot be handled by exact string matching.
type LLMClient interface {
	// GradeAnswer is for QuestionSubjective only: free-text semantic grading such as translated meaning or paraphrased answers.
	GradeAnswer(ctx context.Context, questionPrompt, correctAnswer, userAnswer string) (GradeResult, error)
	// GradeHandwriting is for QuestionKanaHandwriting only: binary visual verification of a rendered handwriting PNG.
	GradeHandwriting(ctx context.Context, questionPrompt, correctAnswer string, pngImage []byte) (GradeResult, error)
	// AnswerLearningQuestion answers an ad-hoc language-learning question from Telegram.
	AnswerLearningQuestion(ctx context.Context, question string) (string, error)
}

// GradeResult represents the structured JSON output from the LLM.
type GradeResult struct {
	IsCorrect bool   `json:"is_correct"`
	Feedback  string `json:"feedback"`
}

// GeneratedTip is a single LLM-generated tip body. The eyebrow label is filled
// in by code via TipCategory.DisplayName(), so the model only returns the body.
type GeneratedTip struct {
	Body string `json:"body"`
}

const (
	handwritingMaxCompletionTokens = 80
	learningQuestionMaxTokens      = 700
	// Gemini 3.5 Flash-Lite deprecates sampling parameters; deterministic behavior comes from system prompts.
	tipGenerationMaxTokens = 800
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
func (c *DefaultLLMClient) GradeAnswer(
	ctx context.Context,
	questionPrompt, correctAnswer, userAnswer string,
) (GradeResult, error) {
	if c.client == nil || c.model == "" {
		return GradeResult{}, ErrAIConfigMissing
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
		return GradeResult{}, fmt.Errorf("llm grading request failed: %w", err)
	}

	if len(resp.Choices) == 0 {
		return GradeResult{}, fmt.Errorf("empty llm response")
	}

	rawContent := resp.Choices[0].Message.Content

	var result GradeResult
	if err := json.Unmarshal([]byte(rawContent), &result); err != nil {
		return GradeResult{}, fmt.Errorf("failed to parse llm output (%s): %w", rawContent, err)
	}

	return result, nil
}

func (c *DefaultLLMClient) AnswerLearningQuestion(ctx context.Context, question string) (string, error) {
	if c.client == nil || c.model == "" {
		return "", ErrAIConfigMissing
	}

	resp, err := c.client.CreateChatCompletion(ctx, openai.ChatCompletionRequest{
		Model:               c.model,
		MaxCompletionTokens: learningQuestionMaxTokens,
		Messages: []openai.ChatCompletionMessage{
			{
				Role: openai.ChatMessageRoleSystem,
				Content: `You are CopyLingo's language-learning assistant.
Answer in Korean by default.
Be concise, accurate, and practical for a beginner-to-intermediate language learner.
When the user asks about Japanese, include kana/romaji/meaning distinctions when useful.
Do not use HTML tags or markdown code fences.`,
			},
			{
				Role:    openai.ChatMessageRoleUser,
				Content: question,
			},
		},
	})
	if err != nil {
		return "", fmt.Errorf("llm learning question request failed: %w", err)
	}
	if len(resp.Choices) == 0 {
		return "", fmt.Errorf("empty llm learning question response")
	}
	return resp.Choices[0].Message.Content, nil
}

// GradeHandwriting verifies whether a rendered handwriting image matches the expected Japanese text.
func (c *DefaultLLMClient) GradeHandwriting(
	ctx context.Context,
	questionPrompt, correctAnswer string,
	pngImage []byte,
) (GradeResult, error) {
	startedAt := time.Now()
	ctx = observability.WithAttrs(ctx, slog.String("source", "external.llm"))

	if c.client == nil || c.model == "" {
		return GradeResult{}, ErrAIConfigMissing
	}

	systemPrompt := buildHandwritingSystemPrompt()
	userPrompt := buildHandwritingUserPrompt(questionPrompt, correctAnswer)

	imageURL := "data:image/png;base64," + base64.StdEncoding.EncodeToString(pngImage)
	req := buildHandwritingChatCompletionRequest(c.model, systemPrompt, userPrompt, imageURL)
	resp, err := c.client.CreateChatCompletion(ctx, req)
	if err != nil {
		return GradeResult{}, fmt.Errorf("llm handwriting grading request failed: %w", err)
	}
	if len(resp.Choices) == 0 {
		return GradeResult{}, fmt.Errorf("empty llm handwriting response")
	}

	rawContent := resp.Choices[0].Message.Content
	var result GradeResult
	if err := json.Unmarshal([]byte(rawContent), &result); err != nil {
		return GradeResult{}, fmt.Errorf("failed to parse llm handwriting output (%s): %w", rawContent, err)
	}
	slog.InfoContext(ctx, "Handwriting LLM grading completed",
		"event", "handwriting.llm.completed",
		"model", c.model,
		"duration_ms", time.Since(startedAt).Milliseconds(),
		"image_bytes", len(pngImage),
		"is_correct", result.IsCorrect,
	)

	return result, nil
}

func buildHandwritingChatCompletionRequest(
	model, systemPrompt, userPrompt, imageURL string,
) openai.ChatCompletionRequest {
	return openai.ChatCompletionRequest{
		Model:               model,
		MaxCompletionTokens: handwritingMaxCompletionTokens,
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
- This is an acceptance-first decision. The default is is_correct=true.
- Grade generously. This is low-stakes beginner practice; a wrong rejection discourages the learner far more than a lenient acceptance helps.
- If the image is ambiguous, low-resolution, partially clipped, rough, or plausibly matches the Expected Text, return true.
- Do not search for or prefer an alternative transcription.
- If the image resembles both the Expected Text and another kana or kanji, return true when the Expected Text remains plausible.
- If distinguishing the Expected Text from another character would require knowing stroke direction or pen movement, return true when the Expected Text remains plausible.
- Accept rough mobile handwriting, joined or separated strokes, uneven proportions, spacing, alignment, size, and ambiguous small kana or diacritic marks when plausibly present.
- For a short kana word, compare the full expected string, but do not reject for spacing, alignment, relative size, or stroke segmentation. Reject only for a clearly missing, extra, substituted, or swapped character that is visibly unambiguous.
- Return false ONLY when you can name one concrete, observable defect in the final bitmap with high confidence. If you cannot name that defect without guessing, return true.

Marks and script (do not over-reject on these):
- Diacritics (゛dakuten / ゜handakuten) render as tiny, low-resolution marks. If a diacritic is plausibly present where one is expected, accept it. NEVER reject solely because you cannot tell dakuten from handakuten, or cannot count the exact number of dots.
- Do NOT reject for hiragana-vs-katakana unless the written shape clearly and unambiguously belongs to the other script. Treat visually similar shapes as the Expected Text.
- When script identity or diacritic type is visually ambiguous in rough mobile handwriting, return true when the Expected Text remains plausible.

Contracted sounds (yoon; small ゃ / ゅ / ょ or ャ / ュ / ョ):
- Small kana handwritten with a finger often have rough proportions and simplified shapes. Do NOT require textbook size, proportions, or exact shape.
- When the Expected Text contains a small kana, return true if a plausible second small mark is present in the expected position and the full Expected Text remains plausible.
- Return false for a small-kana issue ONLY when the small kana is clearly absent or clearly replaced by an unrelated shape.
- Do NOT claim that a small kana is full-sized, malformed, or wrong unless that is unambiguous from the final bitmap. When uncertain, return true.

Apply this principle generally, not only to this example:
- Expected Text: オ
- The handwriting could also be interpreted as the visually similar kanji 才.
- Since オ remains a plausible reading, return true.

Feedback policy:
- If is_correct is true, feedback must be an empty string.
- Feedback is empty by default, including for an incorrect result.
- When is_correct is false, return a Korean correction note ONLY when one concrete visual defect is clearly visible and can be stated with high confidence.
- If the false verdict is not supported by a precise visual defect, set feedback to an empty string. The app will show the Expected Text without an additional correction, which is safer than a speculative explanation.
- Explain only which expected feature is clearly missing or wrong; do not explain why the handwriting is merely unusual.
- Do not propose, transcribe, or mention an alternative character.
- Never mention stroke order, starting point, writing direction, or pen movement.
- Never write speculative or hedged feedback such as "maybe", "probably", "looks like", or "it seems".
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
							"description": "Empty by default. Only when is_correct is false and one concrete visual defect is clearly visible, provide a short Korean note about that missing or wrong expected feature. Never speculate or mention alternative characters, stroke order, starting point, writing direction, or pen movement."
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

// GenerateTips asks the LLM for n short Korean learning tips for the given
// (language, level, category). The model returns a JSON array of {"body": "..."}
// objects; the eyebrow label is added by the caller via category.DisplayName().
//
// This is exposed only on the concrete client (not the LLMClient interface) so
// that grading mocks elsewhere stay unaffected; the tip pipeline depends on a
// narrow service-layer interface instead.
func (c *DefaultLLMClient) GenerateTips(
	ctx context.Context,
	language, level string,
	category model.TipCategory,
	n int,
) ([]GeneratedTip, error) {
	if c.client == nil || c.model == "" {
		return nil, ErrAIConfigMissing
	}
	if n <= 0 {
		return nil, nil
	}

	systemPrompt := `당신은 외국어 학습 팁 작성자입니다. JSON 배열로만 응답하세요.`
	userPrompt := buildTipGenerationUserPrompt(language, level, category, n)

	resp, err := c.client.CreateChatCompletion(ctx, openai.ChatCompletionRequest{
		Model:               c.model,
		MaxCompletionTokens: tipGenerationMaxTokens,
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
	})
	if err != nil {
		return nil, fmt.Errorf("llm tip generation request failed (language=%s level=%s category=%s): %w",
			language, level, category, err)
	}
	if len(resp.Choices) == 0 {
		return nil, fmt.Errorf("empty llm tip generation response (language=%s level=%s category=%s)",
			language, level, category)
	}

	rawContent := extractJSONArray(resp.Choices[0].Message.Content)

	var tips []GeneratedTip
	if err := json.Unmarshal([]byte(rawContent), &tips); err != nil {
		return nil, fmt.Errorf("failed to parse llm tip output (%s): %w", rawContent, err)
	}
	return tips, nil
}

func buildTipGenerationUserPrompt(language, level string, category model.TipCategory, n int) string {
	return fmt.Sprintf(`대상 언어: %s
학습 레벨: %s
카테고리: %s (%s)

위 카테고리에 대한 외국어 학습 팁을 정확히 %d개 작성하세요.
- 각 팁의 body 는 한국어로 1~2 문장, 최대 200자.
- 학습자에게 짧고 명확한 어조로 작성하세요.
- 한 카테고리 안에서 서로 다른 각도(예시·주의점·구분법 등)로 작성해 중복을 피하세요.
- 마크다운이나 코드 펜스 없이 JSON 배열만 출력하세요.

출력 스키마:
[{"body": "..."}, {"body": "..."}]`,
		language, level, category.DisplayName(), category, n)
}

// extractJSONArray trims optional markdown code fences and isolates the JSON
// array payload so json.Unmarshal can parse defensively against fenced output.
func extractJSONArray(raw string) string {
	s := strings.TrimSpace(raw)
	s = strings.TrimPrefix(s, "```json")
	s = strings.TrimPrefix(s, "```")
	s = strings.TrimSuffix(s, "```")
	s = strings.TrimSpace(s)

	start := strings.Index(s, "[")
	end := strings.LastIndex(s, "]")
	if start != -1 && end != -1 && end > start {
		return s[start : end+1]
	}
	return s
}
