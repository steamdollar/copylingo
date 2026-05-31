package main

import (
	"strings"
	"testing"
)

func TestKanaScriptLabel(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		kana string
		want string
	}{
		{name: "hiragana basic", kana: "あ", want: "히라가나"},
		{name: "hiragana yoon", kana: "きゃ", want: "히라가나"},
		{name: "katakana basic", kana: "ア", want: "가타카나"},
		{name: "katakana yoon", kana: "キャ", want: "가타카나"},
		{name: "unknown", kana: "a", want: "가나"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			if got := kanaScriptLabel(tt.kana); got != tt.want {
				t.Fatalf("kanaScriptLabel(%q) = %q, want %q", tt.kana, got, tt.want)
			}
		})
	}
}

func TestBuildQuestionType2PromptIncludesScriptLabel(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name   string
		answer string
		want   string
	}{
		{name: "hiragana answer", answer: "あ", want: "히라가나 문자"},
		{name: "katakana answer", answer: "ア", want: "가타카나 문자"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			q := buildQuestion("a", tt.answer, []string{"い", "う", "え", "お"}, false)
			if !strings.Contains(q.Prompt, tt.want) {
				t.Fatalf("prompt %q does not contain %q", q.Prompt, tt.want)
			}
			if !strings.Contains(q.Explanation, tt.want) {
				t.Fatalf("explanation %q does not contain %q", q.Explanation, tt.want)
			}
		})
	}
}

func TestKanaDisambiguationHint(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		kana string
		want string
	}{
		{name: "hiragana ji sa row", kana: "じ", want: "さ행에 탁점"},
		{name: "hiragana zu sa row", kana: "ず", want: "さ행에 탁점"},
		{name: "hiragana ji ta row", kana: "ぢ", want: "た행에 탁점"},
		{name: "hiragana zu ta row", kana: "づ", want: "た행에 탁점"},
		{name: "katakana ji sa row", kana: "ジ", want: "サ행에 탁점"},
		{name: "katakana zu sa row", kana: "ズ", want: "サ행에 탁점"},
		{name: "katakana ji ta row", kana: "ヂ", want: "タ행에 탁점"},
		{name: "katakana zu ta row", kana: "ヅ", want: "タ행에 탁점"},
		{name: "unambiguous kana", kana: "が", want: ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			if got := kanaDisambiguationHint(tt.kana); got != tt.want {
				t.Fatalf("kanaDisambiguationHint(%q) = %q, want %q", tt.kana, got, tt.want)
			}
		})
	}
}

func TestAmbiguousReverseKanaQuestionsIncludeHint(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		kana string
		want string
	}{
		{name: "hiragana ji sa row", kana: "じ", want: "さ행에 탁점"},
		{name: "hiragana zu sa row", kana: "ず", want: "さ행에 탁점"},
		{name: "hiragana ji ta row", kana: "ぢ", want: "た행에 탁점"},
		{name: "hiragana zu ta row", kana: "づ", want: "た행에 탁점"},
		{name: "katakana ji sa row", kana: "ジ", want: "サ행에 탁점"},
		{name: "katakana zu sa row", kana: "ズ", want: "サ행에 탁점"},
		{name: "katakana ji ta row", kana: "ヂ", want: "タ행에 탁점"},
		{name: "katakana zu ta row", kana: "ヅ", want: "タ행에 탁점"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			romaji := kanaMap[tt.kana]
			type2 := buildQuestion(romaji, tt.kana, []string{"あ", "い", "う", "え", "お"}, false)
			if !strings.Contains(type2.Prompt, tt.want) {
				t.Fatalf("Type 2 prompt %q does not contain %q", type2.Prompt, tt.want)
			}

			handwriting := buildHandwritingQuestion(romaji, tt.kana)
			if !strings.Contains(handwriting.Prompt, tt.want) {
				t.Fatalf("handwriting prompt %q does not contain %q", handwriting.Prompt, tt.want)
			}
		})
	}
}

func TestUnambiguousReverseKanaQuestionsDoNotIncludeHint(t *testing.T) {
	t.Parallel()

	type2 := buildQuestion("ga", "が", []string{"あ", "い", "う", "え", "お"}, false)
	if strings.Contains(type2.Prompt, "힌트:") {
		t.Fatalf("Type 2 prompt unexpectedly contains hint: %q", type2.Prompt)
	}

	handwriting := buildHandwritingQuestion("ga", "が")
	if strings.Contains(handwriting.Prompt, "힌트:") {
		t.Fatalf("handwriting prompt unexpectedly contains hint: %q", handwriting.Prompt)
	}
}
