package bot

import (
	"context"
	"fmt"
	"testing"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
	"github.com/redis/go-redis/v9"

	"github.com/lsj/copylingo/internal/config"
)

type mockBotAPI struct {
	sentMessages []tgbotapi.Chattable
}

func (m *mockBotAPI) Send(c tgbotapi.Chattable) (tgbotapi.Message, error) {
	m.sentMessages = append(m.sentMessages, c)
	return tgbotapi.Message{}, nil
}

func (m *mockBotAPI) Request(c tgbotapi.Chattable) (*tgbotapi.APIResponse, error) {
	return &tgbotapi.APIResponse{}, nil
}

func (m *mockBotAPI) GetUpdatesChan(config tgbotapi.UpdateConfig) tgbotapi.UpdatesChannel {
	return nil
}

func (m *mockBotAPI) StopReceivingUpdates() {}

type mockRedis struct {
	redis.Cmdable
	deletedKeys []string
}

func (m *mockRedis) Del(ctx context.Context, keys ...string) *redis.IntCmd {
	m.deletedKeys = append(m.deletedKeys, keys...)
	return redis.NewIntCmd(ctx)
}

func TestHandleExit(t *testing.T) {
	mAPI := &mockBotAPI{}
	mRdb := &mockRedis{}
	b := &Bot{
		api: mAPI,
		rdb: mRdb,
	}

	chatID := int64(12345)
	msg := &tgbotapi.Message{
		Chat: &tgbotapi.Chat{ID: chatID},
	}

	ctx := context.Background()
	b.handleExit(ctx, msg)

	// Verify Redis key deletion
	expectedKey := fmt.Sprintf(config.KeyUserActiveQuestion, chatID)
	found := false
	for _, k := range mRdb.deletedKeys {
		if k == expectedKey {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("expected Redis key %s to be deleted, but it wasn't", expectedKey)
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
		t.Errorf("expected message text %q, got %q", expectedText, sentMsg.Text)
	}
}

func TestClearInlineKeyboardOmitsReplyMarkup(t *testing.T) {
	mAPI := &mockBotAPI{}
	b := &Bot{api: mAPI}

	if err := b.ClearInlineKeyboard(12345, 678); err != nil {
		t.Fatalf("ClearInlineKeyboard() error = %v", err)
	}

	if len(mAPI.sentMessages) != 1 {
		t.Fatalf("sent messages = %d, want 1", len(mAPI.sentMessages))
	}

	edit, ok := mAPI.sentMessages[0].(tgbotapi.EditMessageReplyMarkupConfig)
	if !ok {
		t.Fatalf("sent message type = %T, want EditMessageReplyMarkupConfig", mAPI.sentMessages[0])
	}
	if edit.ChatID != 12345 || edit.MessageID != 678 {
		t.Fatalf("target = (%d, %d), want (12345, 678)", edit.ChatID, edit.MessageID)
	}
	if edit.ReplyMarkup != nil {
		t.Fatalf("ReplyMarkup = %#v, want nil", edit.ReplyMarkup)
	}
}
