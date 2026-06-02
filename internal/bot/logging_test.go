package bot

import (
	"bytes"
	"encoding/json"
	"log/slog"
	"strings"
	"testing"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"

	"github.com/lsj/copylingo/internal/observability"
)

func TestHandleUpdateLogsTelegramCorrelationWithoutMessageBody(t *testing.T) {
	var output bytes.Buffer
	previous := slog.Default()
	slog.SetDefault(slog.New(observability.NewContextHandler(slog.NewJSONHandler(&output, nil))))
	defer slog.SetDefault(previous)

	b := &Bot{api: &mockBotAPI{}}
	b.handleUpdate(tgbotapi.Update{
		UpdateID: 42,
		Message: &tgbotapi.Message{
			Text: "/start secret-payload",
			Entities: []tgbotapi.MessageEntity{
				{Type: "bot_command", Offset: 0, Length: 6},
			},
			From: &tgbotapi.User{ID: 123},
			Chat: &tgbotapi.Chat{ID: 456},
		},
	})

	var entry map[string]any
	if err := json.Unmarshal(output.Bytes(), &entry); err != nil {
		t.Fatalf("json.Unmarshal(%q) error = %v", output.Bytes(), err)
	}
	for key, want := range map[string]any{
		"interaction_id": "tg-42",
		"event":          "telegram.update.completed",
		"update_type":    "message",
		"command":        "start",
		"user_id":        float64(123),
		"chat_id":        float64(456),
	} {
		if got := entry[key]; got != want {
			t.Fatalf("entry[%q] = %#v, want %#v", key, got, want)
		}
	}
	if strings.Contains(output.String(), "secret-payload") {
		t.Fatalf("Telegram log leaked message body: %s", output.String())
	}
}

func TestTelegramUpdateAttrsExtractCallbackIDsWithoutRawData(t *testing.T) {
	attrs := telegramUpdateAttrs(tgbotapi.Update{
		UpdateID: 43,
		CallbackQuery: &tgbotapi.CallbackQuery{
			Data: "q:7:11:3",
			From: &tgbotapi.User{ID: 123},
			Message: &tgbotapi.Message{
				Chat: &tgbotapi.Chat{ID: 456},
			},
		},
	})

	got := make(map[string]any)
	for _, attr := range attrs {
		got[attr.Key] = attr.Value.Any()
	}
	for key, want := range map[string]any{
		"interaction_id": "tg-43",
		"callback_type":  "question",
		"session_id":     int64(7),
		"question_id":    int64(11),
	} {
		if value := got[key]; value != want {
			t.Fatalf("attrs[%q] = %#v, want %#v", key, value, want)
		}
	}
}
