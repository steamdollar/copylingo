package bot

import (
	"context"
	"strings"
	"testing"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"

	"github.com/lsj/copylingo/internal/model"
	"github.com/lsj/copylingo/internal/service"
)

// sessionListStore drives ListInProgressQuizzes for refresh tests.
type sessionListStore struct {
	mockSessionStore
	inProgress []model.Session
}

func (store *sessionListStore) ListInProgress(ctx context.Context) ([]model.Session, error) {
	return store.inProgress, nil
}

func TestRefreshStaleMiniAppMessages_EmptyBaseURL(t *testing.T) {
	ctx := context.Background()
	mockAPI := &mockBotAPI{}
	sessionFlow := newTestSessionFlow(
		mockAPI,
		nil,
		SessionFlowDependencies{},
	) // PublicBaseURL empty

	sessionFlow.RefreshStaleMiniAppMessages(ctx)

	if len(mockAPI.sentMessages) != 0 {
		t.Errorf(
			"expected no messages when base URL empty, got %d",
			len(mockAPI.sentMessages),
		)
	}
}

func TestRefreshStaleMiniAppMessages_NoSessions(t *testing.T) {
	ctx := context.Background()
	mockAPI := &mockBotAPI{}
	stateStores := newTestInteractionStores()
	store := &sessionListStore{inProgress: nil}
	sessionFlow := newTestSessionFlow(
		mockAPI,
		stateStores,
		SessionFlowDependencies{
			Session: newTestSessionService(
				stateStores,
				service.SessionDependencies{SessionRepository: store},
			),
			PublicBaseURL: "https://x.trycloudflare.com",
		},
	)

	sessionFlow.RefreshStaleMiniAppMessages(ctx)

	if len(mockAPI.sentMessages) != 0 {
		t.Errorf(
			"expected no messages with no in-progress sessions, got %d",
			len(mockAPI.sentMessages),
		)
	}
}

// fingerprintSpy counts fingerprint lookups, the first per-session step of
// restart recovery.
type fingerprintSpy struct {
	MiniAppRecoveryStore
	lookups int
}

func (spy *fingerprintSpy) GetMiniAppFingerprint(
	context.Context,
	int,
) (string, error) {
	spy.lookups++
	return "", nil
}

func TestRefreshStaleMiniAppMessages_StopsWhenContextCancelled(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	mockAPI := &mockBotAPI{}
	stateStores := newTestInteractionStores()
	recovery := &fingerprintSpy{}
	sessionFlow := newTestSessionFlow(
		mockAPI,
		stateStores,
		SessionFlowDependencies{
			Session: newTestSessionService(
				stateStores,
				service.SessionDependencies{SessionRepository: &sessionListStore{
					inProgress: []model.Session{{ID: 1, UserID: 2, Mode: model.SessionModeQuiz}},
				}},
			),
			Recovery:      recovery,
			PublicBaseURL: "https://x.trycloudflare.com",
		},
	)

	sessionFlow.RefreshStaleMiniAppMessages(ctx)

	if recovery.lookups != 0 || len(mockAPI.sentMessages) != 0 {
		t.Fatalf(
			"lookups = %d, sent = %d; want no per-session work after shutdown",
			recovery.lookups,
			len(mockAPI.sentMessages),
		)
	}
}

// emptyQuestionFetcher has no new questions; due-review calls come from mockSRSRepository.
type emptyQuestionFetcher struct {
	mockSRSRepository
}

func (fetcher *emptyQuestionFetcher) GetNewQuestions(
	ctx context.Context,
	userID int64,
	language string,
	levels []string,
	category string,
	excludeIDs []int,
	limit,
	kanjiRecallLimit int,
) ([]model.Question, error) {
	return nil, nil
}

func TestHandleTest_NoQuestions(t *testing.T) {
	ctx := context.Background()
	mockAPI := &mockBotAPI{}
	userService := service.NewUserService(&mockUserRepository{
		getOrCreateFunc: func(
			ctx context.Context,
			id int64,
			username string,
		) (*model.User, error) {
			return &model.User{ID: id, Language: "ja", ProficiencyLevel: "N5"}, nil
		},
	})
	bot := newTestBot(
		mockAPI,
		nil,
		&testServices{
			User: userService,
			Session: newTestSessionService(
				nil,
				service.SessionDependencies{
					QuestionRepository:        &emptyQuestionFetcher{},
					SessionRepository:         &sessionListStore{},
					SessionQuestionRepository: &mockSessionQuestionStore{},
				},
			),
		},
	)

	message := &tgbotapi.Message{Chat: &tgbotapi.Chat{ID: 1}, From: &tgbotapi.User{ID: 2}}
	bot.handleTest(
		ctx,
		message,
	)

	text := collectText(mockAPI.sentMessages)
	if !strings.Contains(
		text,
		"사용 가능한 문제가 없습니다",
	) {
		t.Errorf(
			"expected no-questions notice, got %q",
			text,
		)
	}
}
