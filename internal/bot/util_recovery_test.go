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

func (e *emptyQuestionFetcher) GetNewQuestions(
	ctx context.Context,
	userID int64,
	language, level, category string,
	excludeIDs []int,
	limit int,
) ([]model.Question, error) {
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
