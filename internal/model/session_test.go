package model

import "testing"

func TestSessionMode_IsValid(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		mode SessionMode
		want bool
	}{
		{"quiz", SessionModeQuiz, true},
		{"study", SessionModeStudy, true},
		{"empty", SessionMode(""), false},
		{"unknown", SessionMode("listening"), false},
	}
	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			if got := tt.mode.IsValid(); got != tt.want {
				t.Fatalf("SessionMode(%q).IsValid() = %v, want %v", tt.mode, got, tt.want)
			}
		})
	}
}
