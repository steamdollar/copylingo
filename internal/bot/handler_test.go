package bot

import (
	"context"
	"testing"
	"time"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"

	"github.com/lsj/copylingo/internal/model"
)

func TestHandleExit(t *testing.T) {
	mAPI := &mockBotAPI{}
	mRdb := newTestInteractionStores()
	chatID := int64(12345)
	_ = mRdb.SetActiveQuestion(
		context.Background(),
		chatID,
		model.ActiveQuestionRef{SessionID: 10, QuestionIndex: 2},
	)
	b := &Bot{
		telegram: newTelegramClient(mAPI),
		input:    mRdb,
	}

	msg := &tgbotapi.Message{
		Chat: &tgbotapi.Chat{ID: chatID},
	}

	ctx := context.Background()
	b.handleExit(
		ctx,
		msg,
	)

	if question, _ := mRdb.GetActiveQuestion(
		context.Background(),
		chatID,
	); question != nil {
		t.Error("active text question still exists")
	}

	// Verify message sent
	if len(mAPI.sentMessages) == 0 {
		t.Fatal("expected a message to be sent, but none were")
	}

	sentMsg, ok := mAPI.sentMessages[0].(tgbotapi.MessageConfig)
	if !ok {
		t.Fatal("expected sent message to be tgbotapi.MessageConfig")
	}

	expectedText := "🚪 현재 입력을 취소했습니다. /menu 에서 언제든 이어서 진행할 수 있어요."
	if sentMsg.Text != expectedText {
		t.Errorf(
			"expected message text %q, got %q",
			expectedText,
			sentMsg.Text,
		)
	}
}

func TestClearInlineKeyboardOmitsReplyMarkup(t *testing.T) {
	mAPI := &mockBotAPI{}
	b := &Bot{telegram: newTelegramClient(mAPI)}

	if err := b.telegram.ClearInlineKeyboard(
		12345,
		678,
	); err != nil {
		t.Fatalf(
			"ClearInlineKeyboard() error = %v",
			err,
		)
	}

	if len(mAPI.sentMessages) != 1 {
		t.Fatalf(
			"sent messages = %d, want 1",
			len(mAPI.sentMessages),
		)
	}

	edit, ok := mAPI.sentMessages[0].(tgbotapi.EditMessageReplyMarkupConfig)
	if !ok {
		t.Fatalf(
			"sent message type = %T, want EditMessageReplyMarkupConfig",
			mAPI.sentMessages[0],
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

func (a *blockingUpdatesAPI) GetUpdatesChan(tgbotapi.UpdateConfig) tgbotapi.UpdatesChannel {
	return a.updates
}

func (a *blockingUpdatesAPI) Send(tgbotapi.Chattable) (tgbotapi.Message, error) {
	close(a.sending)
	<-a.release
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
	b := NewBot(BotDeps{Telegram: newTelegramClient(api)})
	go b.Start()

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
	go func() { stopErr <- b.Stop(context.Background()) }()
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
