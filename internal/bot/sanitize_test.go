package bot

import "testing"

func TestSanitizeTelegramHTML(t *testing.T) {
	cases := []struct {
		name string
		in   string
		want string
	}{
		{"br lowercase", "입력하세요<br>힌트", "입력하세요\n힌트"},
		{"br self closing", "a<br/>b", "a\nb"},
		{"br with space", "a<br />b", "a\nb"},
		{"br uppercase", "a<BR>b", "a\nb"},
		{"keeps bold", "<b>정답</b>", "<b>정답</b>"},
		{"no br", "plain text", "plain text"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := sanitizeTelegramHTML(c.in); got != c.want {
				t.Fatalf("sanitizeTelegramHTML(%q) = %q, want %q", c.in, got, c.want)
			}
		})
	}
}
