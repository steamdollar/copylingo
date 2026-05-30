package bot

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	"github.com/lsj/copylingo/internal/callback"
	"github.com/lsj/copylingo/internal/config"
	"github.com/lsj/copylingo/internal/model"
	"github.com/lsj/copylingo/internal/service"
)

func TestMiniAppURLFingerprint(t *testing.T) {
	t.Parallel()

	a := callback.MiniAppURLFingerprint("https://example.trycloudflare.com")
	b := callback.MiniAppURLFingerprint("https://EXAMPLE.trycloudflare.com/path")
	c := callback.MiniAppURLFingerprint("https://other.trycloudflare.com")

	if a == "" {
		t.Fatal("expected fingerprint")
	}
	if a != b {
		t.Fatalf("expected host-only case-insensitive fingerprint, got %q and %q", a, b)
	}
	if a == c {
		t.Fatalf("expected different hosts to produce different fingerprints, got %q", a)
	}
	if got := callback.MiniAppURLFingerprint("not a url"); got != "" {
		t.Fatalf("expected invalid URL to return empty fingerprint, got %q", got)
	}
}

func TestFormatHandwritingNextCallback(t *testing.T) {
	t.Parallel()

	got := callback.FormatHandwritingNext(55, 6, "https://example.trycloudflare.com")
	if !strings.HasPrefix(got, "q:55:next:6:u:") {
		t.Fatalf("expected next callback with URL token, got %q", got)
	}
	if len(got) > 64 {
		t.Fatalf("callback data exceeds Telegram limit: len=%d data=%q", len(got), got)
	}

	withoutURL := callback.FormatHandwritingNext(55, 6, "")
	if withoutURL != "q:55:next:6" {
		t.Fatalf("expected legacy callback without URL, got %q", withoutURL)
	}
}

func TestIsStaleMiniAppCallback(t *testing.T) {
	t.Parallel()

	currentURL := "https://current.trycloudflare.com"
	currentToken := callback.MiniAppURLFingerprint(currentURL)

	tests := []struct {
		name  string
		parts []string
		want  bool
	}{
		{
			name:  "legacy callback without token is stale",
			parts: []string{"q", "55", "next", "6"},
			want:  true,
		},
		{
			name:  "same token is not stale",
			parts: []string{"q", "55", "next", "6", "u", currentToken},
			want:  false,
		},
		{
			name:  "different token is stale",
			parts: []string{"q", "55", "next", "6", "u", "deadbeef"},
			want:  true,
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			if got := callback.IsStaleMiniAppCallback(tt.parts, currentURL); got != tt.want {
				t.Fatalf("isStaleMiniAppCallback()=%v, want %v", got, tt.want)
			}
		})
	}
}

func TestSessionFlowUsesActiveSessionProgress(t *testing.T) {
	ctx := context.Background()
	sessionID := 77
	trueVal := true
	state := &model.ActiveSessionState{
		Version: model.ActiveSessionStateVersion,
		Session: model.Session{
			ID: sessionID,
		},
		Items: []model.ActiveSessionQuestion{
			{
				SessionQuestion: model.SessionQuestion{QuestionID: 1, IsCorrect: &trueVal},
				Question:        model.Question{ID: 1},
			},
			{
				SessionQuestion: model.SessionQuestion{QuestionID: 2},
				Question:        model.Question{ID: 2},
			},
		},
	}
	raw, err := json.Marshal(state)
	if err != nil {
		t.Fatalf("marshal state: %v", err)
	}
	rdb := &testRedis{values: map[string]string{
		config.ActiveSessionWorkingSetRedisKey.Format(sessionID): string(raw),
	}}
	active := service.NewActiveSessionService(nil, rdb, nil)
	sf := NewSessionFlow(&Bot{services: &service.Services{ActiveSession: active}})

	idx, err := sf.nextUnansweredQuestionIndex(ctx, sessionID)
	if err != nil {
		t.Fatalf("nextUnansweredQuestionIndex failed: %v", err)
	}
	if idx != 1 {
		t.Fatalf("expected next unanswered index 1, got %d", idx)
	}
	if !sf.isQuestionAnswered(ctx, sessionID, 0) {
		t.Fatal("expected first question to be answered")
	}
	if sf.isQuestionAnswered(ctx, sessionID, 1) {
		t.Fatal("expected second question to be unanswered")
	}
}
