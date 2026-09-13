package service

import (
	"reflect"
	"testing"
)

func TestSessionLevelsForJapaneseJLPT(t *testing.T) {
	tests := []struct {
		target string
		want   []string
	}{
		{target: "N5", want: []string{"N5", "N4"}},
		{target: "N4", want: []string{"N5", "N4", "N3"}},
		{target: "N3", want: []string{"N4", "N3", "N2"}},
		{target: "N2", want: []string{"N3", "N2", "N1"}},
		{target: "N1", want: []string{"N2", "N1"}},
	}

	for _, tt := range tests {
		t.Run(tt.target, func(t *testing.T) {
			if got := sessionLevelsFor("ja", tt.target); !reflect.DeepEqual(got, tt.want) {
				t.Fatalf("sessionLevelsFor(ja, %s) = %v, want %v", tt.target, got, tt.want)
			}
		})
	}
}

func TestSessionLevelsForUsesExactFallbackOutsideKnownJapaneseJLPT(t *testing.T) {
	tests := []struct {
		language string
		target   string
	}{
		{language: "ja", target: "N0"},
		{language: "en", target: "B1"},
		{language: "", target: "N4"},
	}

	for _, tt := range tests {
		t.Run(tt.language+"/"+tt.target, func(t *testing.T) {
			want := []string{tt.target}
			if got := sessionLevelsFor(tt.language, tt.target); !reflect.DeepEqual(got, want) {
				t.Fatalf("sessionLevelsFor(%s, %s) = %v, want %v", tt.language, tt.target, got, want)
			}
		})
	}
}

func TestSessionLevelsForReturnsIndependentScope(t *testing.T) {
	got := sessionLevelsFor("ja", "N4")
	got[0] = "N1"

	if want := []string{"N5", "N4", "N3"}; !reflect.DeepEqual(sessionLevelsFor("ja", "N4"), want) {
		t.Fatalf("sessionLevelsFor returned mutable shared scope: got %v, want %v", sessionLevelsFor("ja", "N4"), want)
	}
}
