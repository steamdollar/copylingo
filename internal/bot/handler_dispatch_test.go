package bot

import (
	"context"
	"strings"
	"testing"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
	"github.com/lsj/copylingo/internal/model"
	"github.com/lsj/copylingo/internal/service"
)

type mockUserRepo struct {
	getOrCreateFn func(ctx context.Context, id int64, username string) (*model.User, error)
}

func (m *mockUserRepo) GetOrCreate(ctx context.Context, id int64, username string) (*model.User, error) {
	return m.getOrCreateFn(ctx, id, username)
}
func (m *mockUserRepo) GetAllUsers(ctx context.Context) ([]model.User, error) { return nil, nil }

type mockSRSRepo struct{}

func (m *mockSRSRepo) GetDueReviews(ctx context.Context, limit int) ([]model.Question, error) {
	return nil, nil
}
func (m *mockSRSRepo) GetDueReviewCount(ctx context.Context) (int, error) {
	return 5, nil // Return 5 for main menu display test
}
func (m *mockSRSRepo) UpdateSRS(ctx context.Context, q *model.Question) error { return nil }

type mockStatsRepo struct {
	getTodayStatsFn func(ctx context.Context, userID int64) (*model.UserStats, error)
}

func (m *mockStatsRepo) GetTodayStats(ctx context.Context, userID int64) (*model.UserStats, error) {
	return m.getTodayStatsFn(ctx, userID)
}
func (m *mockStatsRepo) SaveDailyStats(ctx context.Context, stats *model.UserStats) error { return nil }

func TestLanguageDisplayName(t *testing.T) {
	t.Parallel()
	tests := []struct {
		code string
		want string
	}{
		{"ja", "일본어"},
		{"en", "영어"},
		{"ko", "한국어"},
		{"fr", "fr"},
	}

	for _, tt := range tests {
		if got := languageDisplayName(tt.code); got != tt.want {
			t.Errorf("languageDisplayName(%q) = %q, want %q", tt.code, got, tt.want)
		}
	}
}

func TestHandleUpdate_Dispatch(t *testing.T) {
	mAPI := &mockBotAPI{}
	b := &Bot{api: mAPI}

	t.Run("Message update", func(t *testing.T) {
		update := tgbotapi.Update{
			Message: &tgbotapi.Message{
				Text: "/start",
				Entities: []tgbotapi.MessageEntity{
					{Type: "bot_command", Offset: 0, Length: 6},
				},
				Chat: &tgbotapi.Chat{ID: 123},
			},
		}
		b.handleUpdate(update)
		if len(mAPI.sentMessages) == 0 {
			t.Fatal("expected message sent for /start")
		}
	})

	t.Run("Callback update", func(t *testing.T) {
		mAPI.sentMessages = nil
		update := tgbotapi.Update{
			CallbackQuery: &tgbotapi.CallbackQuery{
				ID:   "1",
				Data: "menu:main",
				Message: &tgbotapi.Message{
					Chat: &tgbotapi.Chat{ID: 123},
				},
				From: &tgbotapi.User{ID: 456},
			},
		}
		
		// Setup dependencies for showMainMenu
		mUserRepo := &mockUserRepo{
			getOrCreateFn: func(ctx context.Context, id int64, username string) (*model.User, error) {
				return &model.User{ID: id, Language: "jp", ProficiencyLevel: "n5"}, nil
			},
		}
		mSRSRepo := &mockSRSRepo{}
		b.services = &service.Services{
			User: service.NewUserService(mUserRepo),
			SRS:  service.NewSRSService(mSRSRepo),
		}

		b.handleUpdate(update)
		if len(mAPI.sentMessages) == 0 {
			t.Fatal("expected message sent for callback menu:main")
		}
	})
}

func TestHandleHelp(t *testing.T) {
	mAPI := &mockBotAPI{}
	b := &Bot{api: mAPI}
	ctx := context.Background()
	msg := &tgbotapi.Message{
		Chat: &tgbotapi.Chat{ID: 123},
	}

	b.handleHelp(ctx, msg)

	if len(mAPI.sentMessages) != 1 {
		t.Fatalf("expected 1 message, got %d", len(mAPI.sentMessages))
	}
	sent := mAPI.sentMessages[0].(tgbotapi.MessageConfig)
	if !strings.Contains(sent.Text, "도움말") {
		t.Errorf("expected help text, got %q", sent.Text)
	}
}

func TestHandleMessage_UnknownCommand(t *testing.T) {
	mAPI := &mockBotAPI{}
	b := &Bot{api: mAPI}
	ctx := context.Background()
	msg := &tgbotapi.Message{
		Chat: &tgbotapi.Chat{ID: 123},
		Text: "/unknown",
		Entities: []tgbotapi.MessageEntity{
			{Type: "bot_command", Offset: 0, Length: 8},
		},
	}

	b.handleMessage(ctx, msg)

	sent := mAPI.sentMessages[0].(tgbotapi.MessageConfig)
	if !strings.Contains(sent.Text, "알 수 없는 명령어") {
		t.Errorf("expected unknown command message, got %q", sent.Text)
	}
}
