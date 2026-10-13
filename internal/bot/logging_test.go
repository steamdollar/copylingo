package bot

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"strings"
	"testing"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"

	"github.com/lsj/copylingo/internal/model"
	"github.com/lsj/copylingo/internal/observability"
	"github.com/lsj/copylingo/internal/service"
)

// captureJSONLogs routes the default slog logger into a buffer for one test.
func captureJSONLogs(t *testing.T) *bytes.Buffer {
	t.Helper()
	output := &bytes.Buffer{}
	previous := slog.Default()
	slog.SetDefault(slog.New(observability.NewContextHandler(slog.NewJSONHandler(
		output,
		nil,
	))))
	t.Cleanup(func() { slog.SetDefault(previous) })
	return output
}

// findLogEntry returns the first JSON log line whose event matches.
func findLogEntry(
	t *testing.T,
	output *bytes.Buffer,
	event string,
) map[string]any {
	t.Helper()
	for _, line := range strings.Split(
		strings.TrimSpace(output.String()),
		"\n",
	) {
		var entry map[string]any
		if err := json.Unmarshal(
			[]byte(line),
			&entry,
		); err != nil {
			t.Fatalf(
				"json.Unmarshal(%q) error = %v",
				line,
				err,
			)
		}
		if entry["event"] == event {
			return entry
		}
	}
	t.Fatalf(
		"no %q log entry in %s",
		event,
		output.String(),
	)
	return nil
}

func assertLogFields(
	t *testing.T,
	entry map[string]any,
	want map[string]any,
) {
	t.Helper()
	for key, wantValue := range want {
		if got := entry[key]; got != wantValue {
			t.Fatalf(
				"entry[%q] = %#v, want %#v",
				key,
				got,
				wantValue,
			)
		}
	}
}

func TestHandleUpdateLogsTelegramCorrelationWithoutMessageBody(t *testing.T) {
	output := captureJSONLogs(t)

	b := &Bot{telegram: newTelegramClient(&mockBotAPI{})}
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

	assertLogFields(
		t,
		findLogEntry(
			t,
			output,
			"telegram.update.completed",
		),
		map[string]any{
			"interaction_id": "tg-42",
			"event":          "telegram.update.completed",
			"update_type":    "message",
			"command":        "start",
			"user_id":        float64(123),
			"chat_id":        float64(456),
		},
	)
	if strings.Contains(
		output.String(),
		"secret-payload",
	) {
		t.Fatalf(
			"Telegram log leaked message body: %s",
			output.String(),
		)
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
			t.Fatalf(
				"attrs[%q] = %#v, want %#v",
				key,
				value,
				want,
			)
		}
	}
}

func TestTelegramClientLogsRejectedSendWithRateLimit(t *testing.T) {
	output := captureJSONLogs(t)
	rateLimited := &tgbotapi.Error{
		Code:               429,
		Message:            "Too Many Requests: retry after 5",
		ResponseParameters: tgbotapi.ResponseParameters{RetryAfter: 5},
	}
	client := newTelegramClient(&mockBotAPI{sendErr: rateLimited})

	err := client.SendMessageWithKeyboard(
		456,
		"result",
		tgbotapi.NewInlineKeyboardMarkup(),
	)

	if !errors.Is(
		err,
		rateLimited,
	) {
		t.Fatalf(
			"SendMessageWithKeyboard error = %v, want %v",
			err,
			rateLimited,
		)
	}
	assertLogFields(
		t,
		findLogEntry(
			t,
			output,
			"telegram.api.failed",
		),
		map[string]any{
			"level":           "WARN",
			"method":          "sendMessage",
			"chat_id":         float64(456),
			"error_code":      float64(429),
			"retry_after_sec": float64(5),
		},
	)
}

func TestTelegramClientDoesNotLogSuccessfulSend(t *testing.T) {
	output := captureJSONLogs(t)
	client := newTelegramClient(&mockBotAPI{})

	if err := client.SendMessage(
		456,
		"ok",
	); err != nil {
		t.Fatalf(
			"SendMessage error = %v",
			err,
		)
	}
	if output.Len() != 0 {
		t.Fatalf(
			"successful send logged: %s",
			output.String(),
		)
	}
}

func TestHandleMessageLogsPlainTextWithoutPendingInput(t *testing.T) {
	output := captureJSONLogs(t)
	api := &mockBotAPI{}
	stores := newTestInteractionStores()
	noSessions := &mockSessionStore{
		getSessionsByStatusFn: func(
			ctx context.Context,
			userID int64,
			status model.SessionStatus,
		) ([]model.Session, error) {
			return nil, nil
		},
	}
	b := newTestBot(
		api,
		stores,
		&testServices{
			Session: newTestSessionService(
				stores,
				service.SessionDeps{SessionRepo: noSessions},
			),
		},
	)

	b.handleMessage(
		observability.WithAttrs(
			context.Background(),
			slog.String(
				"interaction_id",
				"tg-7",
			),
		),
		&tgbotapi.Message{
			Text: "すばらしい",
			From: &tgbotapi.User{ID: 123},
			Chat: &tgbotapi.Chat{ID: 456},
		},
	)

	if len(api.sentMessages) != 0 {
		t.Fatalf(
			"sent messages = %d, want 0",
			len(api.sentMessages),
		)
	}
	assertLogFields(
		t,
		findLogEntry(
			t,
			output,
			"telegram.message.ignored",
		),
		map[string]any{
			"level":          "INFO",
			"interaction_id": "tg-7",
		},
	)
}

func TestHandleMessageResumesInProgressQuizInsteadOfIgnoring(t *testing.T) {
	output := captureJSONLogs(t)
	api := &mockBotAPI{}
	stores := newTestInteractionStores()
	seedQuizState(
		stores,
		&model.QuizActiveSessionState{
			Version: model.QuizActiveSessionStateVersion,
			Session: model.Session{ID: 10},
			Items: []model.QuizActiveSessionQuestion{
				{
					Question: model.Question{
						ID:      1,
						Prompt:  "Q1",
						Type:    model.QuestionMultipleChoice,
						Options: json.RawMessage(`["A"]`),
					},
				},
			},
		},
	)
	inProgressQuiz := &mockSessionStore{
		getSessionsByStatusFn: func(
			ctx context.Context,
			userID int64,
			status model.SessionStatus,
		) ([]model.Session, error) {
			return []model.Session{{ID: 10, Mode: model.SessionModeQuiz}}, nil
		},
	}
	b := newTestBot(
		api,
		stores,
		&testServices{
			Session: newTestSessionService(
				stores,
				service.SessionDeps{SessionRepo: inProgressQuiz},
			),
		},
	)

	b.handleMessage(
		context.Background(),
		&tgbotapi.Message{
			Text: "hello",
			From: &tgbotapi.User{ID: 123},
			Chat: &tgbotapi.Chat{ID: 456},
		},
	)

	if len(api.sentMessages) != 2 {
		t.Fatalf(
			"sent messages = %d, want 2 (notice + current question)",
			len(api.sentMessages),
		)
	}
	findLogEntry(
		t,
		output,
		"telegram.session.resumed_from_text",
	)
	if strings.Contains(
		output.String(),
		"telegram.message.ignored",
	) {
		t.Fatalf(
			"resumed text was also logged as ignored: %s",
			output.String(),
		)
	}
}
