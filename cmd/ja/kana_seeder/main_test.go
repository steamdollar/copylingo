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
