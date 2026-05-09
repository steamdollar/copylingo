package bot

import (
	"strings"
	"testing"
)

func TestMiniAppURLFingerprint(t *testing.T) {
	t.Parallel()

	a := miniAppURLFingerprint("https://example.trycloudflare.com")
	b := miniAppURLFingerprint("https://EXAMPLE.trycloudflare.com/path")
	c := miniAppURLFingerprint("https://other.trycloudflare.com")

	if a == "" {
		t.Fatal("expected fingerprint")
	}
	if a != b {
		t.Fatalf("expected host-only case-insensitive fingerprint, got %q and %q", a, b)
	}
	if a == c {
		t.Fatalf("expected different hosts to produce different fingerprints, got %q", a)
	}
	if got := miniAppURLFingerprint("not a url"); got != "" {
		t.Fatalf("expected invalid URL to return empty fingerprint, got %q", got)
	}
}

func TestFormatHandwritingNextCallback(t *testing.T) {
	t.Parallel()

	got := formatHandwritingNextCallback(55, 6, "https://example.trycloudflare.com")
	if !strings.HasPrefix(got, "q:55:next:6:u:") {
		t.Fatalf("expected next callback with URL token, got %q", got)
	}
	if len(got) > 64 {
		t.Fatalf("callback data exceeds Telegram limit: len=%d data=%q", len(got), got)
	}

	withoutURL := formatHandwritingNextCallback(55, 6, "")
	if withoutURL != "q:55:next:6" {
		t.Fatalf("expected legacy callback without URL, got %q", withoutURL)
	}
}

func TestIsStaleMiniAppCallback(t *testing.T) {
	t.Parallel()

	currentURL := "https://current.trycloudflare.com"
	currentToken := miniAppURLFingerprint(currentURL)

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

			if got := isStaleMiniAppCallback(tt.parts, currentURL); got != tt.want {
				t.Fatalf("isStaleMiniAppCallback()=%v, want %v", got, tt.want)
			}
		})
	}
}
