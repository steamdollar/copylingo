package bot

import "testing"

func TestSanitizeTelegramHTML(t *testing.T) {
	cases := []struct {
		name  string
		input string
		want  string
	}{
		{"br lowercase", "입력하세요<br>힌트", "입력하세요\n힌트"},
		{"br self closing", "a<br/>b", "a\nb"},
		{"br with space", "a<br />b", "a\nb"},
		{"br uppercase", "a<BR>b", "a\nb"},
		{"keeps bold", "<b>정답</b>", "<b>정답</b>"},
		{"no br", "plain text", "plain text"},
	}
	for _, testCase := range cases {
		t.Run(
			testCase.name,
			func(t *testing.T) {
				if got := sanitizeTelegramHTML(testCase.input); got != testCase.want {
					t.Fatalf(
						"sanitizeTelegramHTML(%q) = %q, want %q",
						testCase.input,
						got,
						testCase.want,
					)
				}
			},
		)
	}
}
