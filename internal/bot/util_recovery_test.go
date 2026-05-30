package bot

import (
	"context"
	"strings"
	"testing"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
	"github.com/lsj/copylingo/internal/config"
	"github.com/lsj/copylingo/internal/model"
	"github.com/lsj/copylingo/internal/service"
)

func TestParseHandwritingMessageRef(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name     string
		raw      string
		wantChat int64
		wantMsg  int
		wantOK   bool
	}{
		{"valid", "12345:678", 12345, 678, true},
		{"negative chat", "-100:5", -100, 5, true},
		{"too few parts", "12345", 0, 0, false},
		{"too many parts", "1:2:3", 0, 0, false},
		{"bad chat", "abc:5", 0, 0, false},
		{"bad message", "5:abc", 0, 0, false},
		{"zero chat", "0:5", 0, 0, false},
		{"non-positive message", "5:0", 0, 0, false},
		{"empty", "", 0, 0, false},
	}
	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			chat, msg, err := ParseHandwritingMessageRef(tt.raw)
			ok := err == nil
			if ok != tt.wantOK || chat != tt.wantChat || msg != tt.wantMsg {
				t.Fatalf("ParseHandwritingMessageRef(%q) = (%d, %d, ok=%t), want (%d, %d, ok=%t)",
					tt.raw, chat, msg, ok, tt.wantChat, tt.wantMsg, tt.wantOK)
			}
		})
	}
}

// sessionListStore drives GetAllInProgressSessions for refresh tests.
type sessionListStore struct {
	mockSessionStore
	inProgress []model.Session
}

func (s *sessionListStore) ListInProgress(ctx context.Context) ([]model.Session, error) {
	return s.inProgress, nil
}

func TestRefreshStaleMiniAppMessages_EmptyBaseURL(t *testing.T) {
	ctx := context.Background()
	mAPI := &mockBotAPI{}
	b := &Bot{api: mAPI, cfg: &config.Config{}} // PublicBaseURL empty

	b.RefreshStaleMiniAppMessages(ctx)

	if len(mAPI.sentMessages) != 0 {
		t.Errorf("expected no messages when base URL empty, got %d", len(mAPI.sentMessages))
	}
}

func TestRefreshStaleMiniAppMessages_NoSessions(t *testing.T) {
	ctx := context.Background()
	mAPI := &mockBotAPI{}
	rdb := &testRedis{values: map[string]string{}}
	store := &sessionListStore{inProgress: nil}
	sb := service.NewSessionBuilderService(nil, store, nil, nil)
	cfg := &config.Config{}
	cfg.Server.PublicBaseURL = "https://x.trycloudflare.com"
	b := &Bot{
		api: mAPI, rdb: rdb, cfg: cfg,
		services: &service.Services{SessionBuilder: sb},
	}

	b.RefreshStaleMiniAppMessages(ctx)

	if len(mAPI.sentMessages) != 0 {
		t.Errorf("expected no messages with no in-progress sessions, got %d", len(mAPI.sentMessages))
	}
}

type emptyQuestionFetcher struct{}

func (e *emptyQuestionFetcher) GetNewQuestions(ctx context.Context, language, level, category string, excludeIDs []int, limit int) ([]model.Question, error) {
	return nil, nil
}
func (e *emptyQuestionFetcher) GetByID(ctx context.Context, id int) (*model.Question, error) {
	return nil, nil
}

func TestHandleTest_NoQuestions(t *testing.T) {
	ctx := context.Background()
	mAPI := &mockBotAPI{}
	userSvc := service.NewUserService(&mockUserRepo{
		getOrCreateFn: func(ctx context.Context, id int64, username string) (*model.User, error) {
			return &model.User{ID: id, Language: "ja", ProficiencyLevel: "N5"}, nil
		},
	})
	srs := service.NewSRSService(&mockSRSRepo{})
	store := &sessionListStore{}
	sb := service.NewSessionBuilderService(&emptyQuestionFetcher{}, store, &mockSessionQuestionStore{}, srs)
	b := &Bot{
		api: mAPI, cfg: &config.Config{},
		services: &service.Services{User: userSvc, SessionBuilder: sb},
	}

	msg := &tgbotapi.Message{Chat: &tgbotapi.Chat{ID: 1}, From: &tgbotapi.User{ID: 2}}
	b.handleTest(ctx, msg)

	text := collectText(mAPI.sentMessages)
	if !strings.Contains(text, "사용 가능한 문제가 없습니다") {
		t.Errorf("expected no-questions notice, got %q", text)
	}
}
