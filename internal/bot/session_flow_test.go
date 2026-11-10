package bot

import (
	"context"
	"strings"
	"testing"

	"github.com/lsj/copylingo/internal/callback"
	"github.com/lsj/copylingo/internal/model"
	"github.com/lsj/copylingo/internal/service"
)

func TestMiniAppURLFingerprint(t *testing.T) {
	t.Parallel()

	lowercaseHostFingerprint := callback.MiniAppURLFingerprint("https://example.trycloudflare.com")
	uppercaseHostFingerprint := callback.MiniAppURLFingerprint("https://EXAMPLE.trycloudflare.com/path")
	otherHostFingerprint := callback.MiniAppURLFingerprint("https://other.trycloudflare.com")

	if lowercaseHostFingerprint == "" {
		t.Fatal("expected fingerprint")
	}
	if lowercaseHostFingerprint != uppercaseHostFingerprint {
		t.Fatalf(
			"expected host-only case-insensitive fingerprint, got %q and %q",
			lowercaseHostFingerprint,
			uppercaseHostFingerprint,
		)
	}
	if lowercaseHostFingerprint == otherHostFingerprint {
		t.Fatalf(
			"expected different hosts to produce different fingerprints, got %q",
			lowercaseHostFingerprint,
		)
	}
	if got := callback.MiniAppURLFingerprint("not a url"); got != "" {
		t.Fatalf(
			"expected invalid URL to return empty fingerprint, got %q",
			got,
		)
	}
}

func TestFormatHandwritingNextCallback(t *testing.T) {
	t.Parallel()

	got := callback.FormatHandwritingNext(
		55,
		6,
		"https://example.trycloudflare.com",
	)
	if !strings.HasPrefix(
		got,
		"q:55:next:6:u:",
	) {
		t.Fatalf(
			"expected next callback with URL token, got %q",
			got,
		)
	}
	if len(got) > 64 {
		t.Fatalf(
			"callback data exceeds Telegram limit: len=%d data=%q",
			len(got),
			got,
		)
	}

	withoutURL := callback.FormatHandwritingNext(
		55,
		6,
		"",
	)
	if withoutURL != "q:55:next:6" {
		t.Fatalf(
			"expected legacy callback without URL, got %q",
			withoutURL,
		)
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
		t.Run(
			tt.name,
			func(t *testing.T) {
				t.Parallel()

				if got := callback.IsStaleMiniAppCallback(
					tt.parts,
					currentURL,
				); got != tt.want {
					t.Fatalf(
						"isStaleMiniAppCallback()=%v, want %v",
						got,
						tt.want,
					)
				}
			},
		)
	}
}

func TestSessionFlowUsesQuizActiveSessionProgress(t *testing.T) {
	ctx := context.Background()
	sessionID := 77
	trueValue := true
	state := &model.QuizActiveSessionState{
		Version: model.QuizActiveSessionStateVersion,
		Session: model.Session{
			ID: sessionID,
		},
		Items: []model.QuizActiveSessionQuestion{
			{
				SessionQuestion: model.SessionQuestion{QuestionID: 1, IsCorrect: &trueValue},
				Question:        model.Question{ID: 1},
			},
			{
				SessionQuestion: model.SessionQuestion{QuestionID: 2},
				Question:        model.Question{ID: 2},
			},
		},
	}
	stateStores := newTestInteractionStores()
	seedQuizState(
		stateStores,
		state,
	)
	sessionFlow := newTestSessionFlow(
		nil,
		nil,
		SessionFlowDependencies{
			Session: newTestSessionService(
				stateStores,
				service.SessionDependencies{},
			),
		},
	)

	index, err := sessionFlow.nextUnansweredQuestionIndex(
		ctx,
		sessionID,
	)
	if err != nil {
		t.Fatalf(
			"nextUnansweredQuestionIndex failed: %v",
			err,
		)
	}
	if index != 1 {
		t.Fatalf(
			"expected next unanswered index 1, got %d",
			index,
		)
	}
	if !sessionFlow.isQuestionAnswered(
		ctx,
		sessionID,
		0,
	) {
		t.Fatal("expected first question to be answered")
	}
	if sessionFlow.isQuestionAnswered(
		ctx,
		sessionID,
		1,
	) {
		t.Fatal("expected second question to be unanswered")
	}
}
