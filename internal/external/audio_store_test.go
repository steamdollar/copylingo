package external

import (
	"strings"
	"testing"
)

func TestAudioKey_ContentAddressed(t *testing.T) {
	k1 := AudioKey("ja", "Kore", "同じ台本")
	k2 := AudioKey("ja", "Kore", "同じ台本")
	if k1 != k2 {
		t.Errorf("same inputs must yield same key: %q vs %q", k1, k2)
	}

	if diff := AudioKey("ja", "Kore", "別の台本"); diff == k1 {
		t.Errorf("different script must yield a different key, both = %q", diff)
	}
	if v := AudioKey("ja", "Puck", "同じ台本"); v == k1 {
		t.Errorf("different voice must yield a different key, both = %q", v)
	}
	if l := AudioKey("en", "Kore", "同じ台本"); l == k1 {
		t.Errorf("different language must yield a different key, both = %q", l)
	}
}

func TestAudioKey_Shape(t *testing.T) {
	k := AudioKey("JA", "Kore", "hello")
	if !strings.HasPrefix(k, "tts/ja/kore/") {
		t.Errorf("key %q should start with tts/{lang}/{voice}/ (lowercased)", k)
	}
	if !strings.HasSuffix(k, ".ogg") {
		t.Errorf("key %q should end with .ogg", k)
	}
	// tts/<lang>/<voice>/<64-hex>.ogg
	parts := strings.Split(k, "/")
	if len(parts) != 4 {
		t.Fatalf("key %q should have 4 path segments, got %d", k, len(parts))
	}
	hexName := strings.TrimSuffix(parts[3], ".ogg")
	if len(hexName) != 64 {
		t.Errorf("sha256 hex segment should be 64 chars, got %d (%q)", len(hexName), hexName)
	}
}

func TestKeySegment(t *testing.T) {
	if got := keySegment("  Kore  "); got != "kore" {
		t.Errorf("keySegment trimmed/lowered = %q, want kore", got)
	}
	if got := keySegment(""); got != "unknown" {
		t.Errorf("empty keySegment = %q, want unknown", got)
	}
}
