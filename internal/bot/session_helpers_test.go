package bot

import (
	"testing"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
	"github.com/lsj/copylingo/internal/config"
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
		t.Run(tt.name, func(t *testing.T) {
			if got := formatSessionAnswer(tt.answer); got != tt.want {
				t.Errorf("formatSessionAnswer() = %v, want %v", got, tt.want)
			}
		})
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
		t.Run(tt.input, func(t *testing.T) {
			if got := sessionTypeLabel(tt.input); got != tt.want {
				t.Errorf("sessionTypeLabel() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestTruncate(t *testing.T) {
	t.Parallel()
	tests := []struct {
		s      string
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
		t.Run(tt.s, func(t *testing.T) {
			if got := truncate(tt.s, tt.maxLen); got != tt.want {
				t.Errorf("truncate(%q, %d) = %q, want %q", tt.s, tt.maxLen, got, tt.want)
			}
		})
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
		t.Run(tt.input, func(t *testing.T) {
			if got := stripHTML(tt.input); got != tt.want {
				t.Errorf("stripHTML() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestMainMenuKeyboard(t *testing.T) {
	t.Parallel()
	kb := mainMenuKeyboard()
	if len(kb.InlineKeyboard) != 1 {
		t.Fatalf("expected 1 row, got %d", len(kb.InlineKeyboard))
	}
	if len(kb.InlineKeyboard[0]) != 1 {
		t.Fatalf("expected 1 button, got %d", len(kb.InlineKeyboard[0]))
	}
	btn := kb.InlineKeyboard[0][0]
	if btn.Text != "🏠 메뉴로" {
		t.Errorf("expected text '🏠 메뉴로', got %q", btn.Text)
	}
	if *btn.CallbackData != config.ActionMenuMain {
		t.Errorf("expected callback data %q, got %q", config.ActionMenuMain, *btn.CallbackData)
	}
}

func TestShowSessionFetchError(t *testing.T) {
	mAPI := &mockBotAPI{}
	b := &Bot{api: mAPI}
	sf := NewSessionFlow(b)

	cb := &tgbotapi.CallbackQuery{
		Message: &tgbotapi.Message{
			Chat:      &tgbotapi.Chat{ID: 123},
			MessageID: 456,
		},
	}

	sf.showSessionFetchError(cb)

	if len(mAPI.sentMessages) != 1 {
		t.Fatalf("expected 1 message, got %d", len(mAPI.sentMessages))
	}
	edit, ok := mAPI.sentMessages[0].(tgbotapi.EditMessageTextConfig)
	if !ok {
		t.Fatalf("expected EditMessageTextConfig, got %T", mAPI.sentMessages[0])
	}
	if edit.ChatID != 123 || edit.MessageID != 456 {
		t.Errorf("wrong target: chat=%d msg=%d", edit.ChatID, edit.MessageID)
	}
}

func TestShowActiveSessionUnavailable(t *testing.T) {
	mAPI := &mockBotAPI{}
	b := &Bot{api: mAPI}
	sf := NewSessionFlow(b)

	t.Run("with editMessageID", func(t *testing.T) {
		mAPI.sentMessages = nil
		editID := 789
		sf.showActiveSessionUnavailable(123, &editID)

		if len(mAPI.sentMessages) != 1 {
			t.Fatalf("expected 1 message, got %d", len(mAPI.sentMessages))
		}
		edit, ok := mAPI.sentMessages[0].(tgbotapi.EditMessageTextConfig)
		if !ok {
			t.Fatalf("expected EditMessageTextConfig, got %T", mAPI.sentMessages[0])
		}
		if edit.MessageID != 789 {
			t.Errorf("expected MessageID 789, got %d", edit.MessageID)
		}
	})

	t.Run("without editMessageID", func(t *testing.T) {
		mAPI.sentMessages = nil
		sf.showActiveSessionUnavailable(123, nil)

		if len(mAPI.sentMessages) != 1 {
			t.Fatalf("expected 1 message, got %d", len(mAPI.sentMessages))
		}
		msg, ok := mAPI.sentMessages[0].(tgbotapi.MessageConfig)
		if !ok {
			t.Fatalf("expected MessageConfig, got %T", mAPI.sentMessages[0])
		}
		if msg.ChatID != 123 {
			t.Errorf("expected ChatID 123, got %d", msg.ChatID)
		}
	})
}
