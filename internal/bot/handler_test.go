package bot

import (
	"context"
	"testing"
	"time"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"

	"github.com/lsj/copylingo/internal/model"
)

func TestHandleExit(t *testing.T) {
	mockAPI := &mockBotAPI{}
	interactionStores := newTestInteractionStores()
	chatID := int64(12345)
	_ = interactionStores.SetActiveQuestion(
		context.Background(),
		chatID,
		model.ActiveQuestionReference{SessionID: 10, QuestionIndex: 2},
	)
	bot := &Bot{
		telegram: newTelegramClient(mockAPI),
		input:    interactionStores,
	}

	message := &tgbotapi.Message{
		Chat: &tgbotapi.Chat{ID: chatID},
	}

	ctx := context.Background()
	bot.handleExit(
		ctx,
		message,
	)

	if question, _ := interactionStores.GetActiveQuestion(
		context.Background(),
		chatID,
	); question != nil {
		t.Error("active text question still exists")
	}

	// Verify message sent
	if len(mockAPI.sentMessages) == 0 {
		t.Fatal("expected a message to be sent, but none were")
	}

	sentMessage, ok := mockAPI.sentMessages[0].(tgbotapi.MessageConfig)
	if !ok {
		t.Fatal("expected sent message to be tgbotapi.MessageConfig")
	}

	expectedText := "🚪 현재 입력을 취소했습니다. /menu 에서 언제든 이어서 진행할 수 있어요."
	if sentMessage.Text != expectedText {
		t.Errorf(
			"expected message text %q, got %q",
			expectedText,
			sentMessage.Text,
		)
	}
}

func TestClearInlineKeyboardOmitsReplyMarkup(t *testing.T) {
	mockAPI := &mockBotAPI{}
	bot := &Bot{telegram: newTelegramClient(mockAPI)}

	if err := bot.telegram.ClearInlineKeyboard(
		12345,
		678,
	); err != nil {
		t.Fatalf(
			"ClearInlineKeyboard() error = %v",
			err,
		)
	}

	if len(mockAPI.sentMessages) != 1 {
		t.Fatalf(
			"sent messages = %d, want 1",
			len(mockAPI.sentMessages),
		)
	}

	edit, ok := mockAPI.sentMessages[0].(tgbotapi.EditMessageReplyMarkupConfig)
	if !ok {
		t.Fatalf(
			"sent message type = %T, want EditMessageReplyMarkupConfig",
			mockAPI.sentMessages[0],
		)
	}
	if edit.ChatID != 12345 || edit.MessageID != 678 {
		t.Fatalf(
			"target = (%d, %d), want (12345, 678)",
			edit.ChatID,
			edit.MessageID,
		)
	}
	if edit.ReplyMarkup != nil {
		t.Fatalf(
			"ReplyMarkup = %#v, want nil",
			edit.ReplyMarkup,
		)
	}
}

// blockingUpdatesAPI feeds updates from a test channel and holds every Send
// until release closes, so the handler of a delivered update stays in flight.
type blockingUpdatesAPI struct {
	mockBotAPI
	updates chan tgbotapi.Update
	sending chan struct{}
	release chan struct{}
}

func (api *blockingUpdatesAPI) GetUpdatesChan(tgbotapi.UpdateConfig) tgbotapi.UpdatesChannel {
	return api.updates
}

func (api *blockingUpdatesAPI) Send(tgbotapi.Chattable) (tgbotapi.Message, error) {
	close(api.sending)
	<-api.release
	return tgbotapi.Message{MessageID: 1}, nil
}

func TestStopWaitsForInFlightUpdateHandler(t *testing.T) {
	api := &blockingUpdatesAPI{
		updates: make(
			chan tgbotapi.Update,
			1,
		),
		sending: make(chan struct{}),
		release: make(chan struct{}),
	}
	bot := NewBot(BotDependencies{Telegram: newTelegramClient(api)})
	go bot.Start()

	// /help sends one message, which blocks in Send until release.
	api.updates <- tgbotapi.Update{
		UpdateID: 1,
		Message: commandMessage(
			"/help",
			1,
			1,
			"learner",
		),
	}
	select {
	case <-api.sending:
	case <-time.After(5 * time.Second):
		t.Fatal("timed out waiting for the /help handler to send")
	}

	stopErr := make(
		chan error,
		1,
	)
	go func() { stopErr <- bot.Stop(context.Background()) }()
	select {
	case err := <-stopErr:
		t.Fatalf(
			"Stop returned %v while an update handler was still running",
			err,
		)
	case <-time.After(50 * time.Millisecond):
	}

	close(api.release)
	select {
	case err := <-stopErr:
		if err != nil {
			t.Fatalf(
				"Stop error = %v, want nil once the handler finished",
				err,
			)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("Stop did not return after the handler finished")
	}
}
