package bot

import (
	"testing"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
)

func TestFormatSessionAnswer(t *testing.T) {
	t.Parallel()
	apple := "apple"
	empty := ""

	tests := []struct {
		name   string
		answer *string
		want   string
	}{
		{"nil answer", nil, "미응답"},
		{"empty string", &empty, "미응답"},
		{"valid answer", &apple, "apple"},
	}

	for _, tt := range tests {
		t.Run(
			tt.name,
			func(t *testing.T) {
				if got := formatSessionAnswer(tt.answer); got != tt.want {
					t.Errorf(
						"formatSessionAnswer() = %v, want %v",
						got,
						tt.want,
					)
				}
			},
		)
	}
}

func TestSessionTypeLabel(t *testing.T) {
	t.Parallel()
	tests := []struct {
		input string
		want  string
	}{
		{"morning", "🌅 오전 학습"},
		{"evening", "🌙 오후 복습"},
		{"review", "🔄 복습"},
		{"article", "📖 아티클"},
		{"unknown", "unknown"},
	}

	for _, tt := range tests {
		t.Run(
			tt.input,
			func(t *testing.T) {
				if got := sessionTypeLabel(tt.input); got != tt.want {
					t.Errorf(
						"sessionTypeLabel() = %v, want %v",
						got,
						tt.want,
					)
				}
			},
		)
	}
}

func TestTruncate(t *testing.T) {
	t.Parallel()
	tests := []struct {
		text   string
		maxLen int
		want   string
	}{
		{"hello", 10, "hello"},
		{"hello", 5, "hello"},
		{"hello world", 5, "hello..."},
		{"안녕하세요", 2, "안녕..."},
		{"안녕하세요", 5, "안녕하세요"},
		{"日本語です", 3, "日本語..."},
	}

	for _, tt := range tests {
		t.Run(
			tt.text,
			func(t *testing.T) {
				if got := truncate(
					tt.text,
					tt.maxLen,
				); got != tt.want {
					t.Errorf(
						"truncate(%q, %d) = %q, want %q",
						tt.text,
						tt.maxLen,
						got,
						tt.want,
					)
				}
			},
		)
	}
}

func TestStripHTML(t *testing.T) {
	t.Parallel()
	tests := []struct {
		input string
		want  string
	}{
		{"hello", "hello"},
		{"<b>hello</b>", "hello"},
		{"<div><p>nested</p></div>", "nested"},
		{"unfinished <tag", "unfinished "},
		{"multiple <tags> for <b>test</b>", "multiple  for test"},
	}

	for _, tt := range tests {
		t.Run(
			tt.input,
			func(t *testing.T) {
				if got := stripHTML(tt.input); got != tt.want {
					t.Errorf(
						"stripHTML() = %v, want %v",
						got,
						tt.want,
					)
				}
			},
		)
	}
}

func TestMainMenuKeyboard(t *testing.T) {
	t.Parallel()
	keyboard := mainMenuKeyboard()
	if len(keyboard.InlineKeyboard) != 1 {
		t.Fatalf(
			"expected 1 row, got %d",
			len(keyboard.InlineKeyboard),
		)
	}
	if len(keyboard.InlineKeyboard[0]) != 1 {
		t.Fatalf(
			"expected 1 button, got %d",
			len(keyboard.InlineKeyboard[0]),
		)
	}
	button := keyboard.InlineKeyboard[0][0]
	if button.Text != "🏠 메뉴로" {
		t.Errorf(
			"expected text '🏠 메뉴로', got %q",
			button.Text,
		)
	}
	if *button.CallbackData != callbackMenuMain {
		t.Errorf(
			"expected callback data %q, got %q",
			callbackMenuMain,
			*button.CallbackData,
		)
	}
}

func TestShowSessionFetchError(t *testing.T) {
	mockAPI := &mockBotAPI{}
	sessionFlow := newTestSessionFlow(
		mockAPI,
		nil,
		SessionFlowDependencies{},
	)

	callbackQuery := &tgbotapi.CallbackQuery{
		Message: &tgbotapi.Message{
			Chat:      &tgbotapi.Chat{ID: 123},
			MessageID: 456,
		},
	}

	sessionFlow.showSessionFetchError(callbackQuery)

	if len(mockAPI.sentMessages) != 1 {
		t.Fatalf(
			"expected 1 message, got %d",
			len(mockAPI.sentMessages),
		)
	}
	edit, ok := mockAPI.sentMessages[0].(tgbotapi.EditMessageTextConfig)
	if !ok {
		t.Fatalf(
			"expected EditMessageTextConfig, got %T",
			mockAPI.sentMessages[0],
		)
	}
	if edit.ChatID != 123 || edit.MessageID != 456 {
		t.Errorf(
			"wrong target: chat=%d msg=%d",
			edit.ChatID,
			edit.MessageID,
		)
	}
}

func TestShowQuizActiveSessionUnavailable(t *testing.T) {
	mockAPI := &mockBotAPI{}
	sessionFlow := newTestSessionFlow(
		mockAPI,
		nil,
		SessionFlowDependencies{},
	)

	t.Run(
		"with editMessageID",
		func(t *testing.T) {
			mockAPI.sentMessages = nil
			editID := 789
			sessionFlow.showQuizActiveSessionUnavailable(
				123,
				&editID,
			)

			if len(mockAPI.sentMessages) != 1 {
				t.Fatalf(
					"expected 1 message, got %d",
					len(mockAPI.sentMessages),
				)
			}
			edit, ok := mockAPI.sentMessages[0].(tgbotapi.EditMessageTextConfig)
			if !ok {
				t.Fatalf(
					"expected EditMessageTextConfig, got %T",
					mockAPI.sentMessages[0],
				)
			}
			if edit.MessageID != 789 {
				t.Errorf(
					"expected MessageID 789, got %d",
					edit.MessageID,
				)
			}
		},
	)

	t.Run(
		"without editMessageID",
		func(t *testing.T) {
			mockAPI.sentMessages = nil
			sessionFlow.showQuizActiveSessionUnavailable(
				123,
				nil,
			)

			if len(mockAPI.sentMessages) != 1 {
				t.Fatalf(
					"expected 1 message, got %d",
					len(mockAPI.sentMessages),
				)
			}
			message, ok := mockAPI.sentMessages[0].(tgbotapi.MessageConfig)
			if !ok {
				t.Fatalf(
					"expected MessageConfig, got %T",
					mockAPI.sentMessages[0],
				)
			}
			if message.ChatID != 123 {
				t.Errorf(
					"expected ChatID 123, got %d",
					message.ChatID,
				)
			}
		},
	)
}
