package callback

import "testing"

func TestParseHandwritingMessageRef(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		raw      string
		wantChat int64
		wantMsg  int
		wantOK   bool
	}{
		{"valid private chat", "12345:678", 12345, 678, true},
		{"valid group chat", "-10012345:678", -10012345, 678, true},
		{"too few parts", "12345", 0, 0, false},
		{"too many parts", "1:2:3", 0, 0, false},
		{"bad chat", "abc:5", 0, 0, false},
		{"bad message", "5:abc", 0, 0, false},
		{"zero chat", "0:5", 0, 0, false},
		{"zero message", "5:0", 0, 0, false},
		{"negative message", "5:-1", 0, 0, false},
		{"empty", "", 0, 0, false},
	}
	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			chat, msg, err := ParseHandwritingMessageRef(tt.raw)
			ok := err == nil
			if ok != tt.wantOK || chat != tt.wantChat || msg != tt.wantMsg {
				t.Fatalf("ParseHandwritingMessageRef(%q) = (%d, %d, ok=%t), want (%d, %d, ok=%t)",
					tt.raw, chat, msg, ok, tt.wantChat, tt.wantMsg, tt.wantOK)
			}
		})
	}
}
