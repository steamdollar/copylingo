package external

import (
	"strings"
	"testing"
)

func TestAudioKey_ContentAddressed(t *testing.T) {
	firstKey := AudioKey(
		"ja",
		"Kore",
		"同じ台本",
	)
	secondKey := AudioKey(
		"ja",
		"Kore",
		"同じ台本",
	)
	if firstKey != secondKey {
		t.Errorf(
			"same inputs must yield same key: %q vs %q",
			firstKey,
			secondKey,
		)
	}

	if diff := AudioKey(
		"ja",
		"Kore",
		"別の台本",
	); diff == firstKey {
		t.Errorf(
			"different script must yield a different key, both = %q",
			diff,
		)
	}
	if otherVoiceKey := AudioKey(
		"ja",
		"Puck",
		"同じ台本",
	); otherVoiceKey == firstKey {
		t.Errorf(
			"different voice must yield a different key, both = %q",
			otherVoiceKey,
		)
	}
	if otherLanguageKey := AudioKey(
		"en",
		"Kore",
		"同じ台本",
	); otherLanguageKey == firstKey {
		t.Errorf(
			"different language must yield a different key, both = %q",
			otherLanguageKey,
		)
	}
}

func TestAudioKey_Shape(t *testing.T) {
	key := AudioKey(
		"JA",
		"Kore",
		"hello",
	)
	if !strings.HasPrefix(
		key,
		"tts/ja/kore/",
	) {
		t.Errorf(
			"key %q should start with tts/{lang}/{voice}/ (lowercased)",
			key,
		)
	}
	if !strings.HasSuffix(
		key,
		".ogg",
	) {
		t.Errorf(
			"key %q should end with .ogg",
			key,
		)
	}
	// tts/<lang>/<voice>/<64-hex>.ogg
	parts := strings.Split(
		key,
		"/",
	)
	if len(parts) != 4 {
		t.Fatalf(
			"key %q should have 4 path segments, got %d",
			key,
			len(parts),
		)
	}
	hexName := strings.TrimSuffix(
		parts[3],
		".ogg",
	)
	if len(hexName) != 64 {
		t.Errorf(
			"sha256 hex segment should be 64 chars, got %d (%q)",
			len(hexName),
			hexName,
		)
	}
}

func TestKeySegment(t *testing.T) {
	if got := keySegment("  Kore  "); got != "kore" {
		t.Errorf(
			"keySegment trimmed/lowered = %q, want kore",
			got,
		)
	}
	if got := keySegment(""); got != "unknown" {
		t.Errorf(
			"empty keySegment = %q, want unknown",
			got,
		)
	}
}
