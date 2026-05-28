package external

import (
	"strings"
	"testing"
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
