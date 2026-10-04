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

func (s *sessionListStore) ListInProgress(ctx context.Context) ([]model.Session, error) {
	return s.inProgress, nil
}

func TestRefreshStaleMiniAppMessages_EmptyBaseURL(t *testing.T) {
	ctx := context.Background()
	mAPI := &mockBotAPI{}
	sf := newTestSessionFlow(
		mAPI,
		nil,
		SessionFlowDeps{},
	) // PublicBaseURL empty

	sf.RefreshStaleMiniAppMessages(ctx)

	if len(mAPI.sentMessages) != 0 {
		t.Errorf(
			"expected no messages when base URL empty, got %d",
			len(mAPI.sentMessages),
		)
	}
}

func TestRefreshStaleMiniAppMessages_NoSessions(t *testing.T) {
	ctx := context.Background()
	mAPI := &mockBotAPI{}
	stateStores := newTestInteractionStores()
	store := &sessionListStore{inProgress: nil}
	sf := newTestSessionFlow(
		mAPI,
		stateStores,
		SessionFlowDeps{
			Session: newTestSessionService(
				stateStores,
				service.SessionDeps{SessionRepo: store},
			),
			PublicBaseURL: "https://x.trycloudflare.com",
		},
	)

	sf.RefreshStaleMiniAppMessages(ctx)

	if len(mAPI.sentMessages) != 0 {
		t.Errorf(
			"expected no messages with no in-progress sessions, got %d",
			len(mAPI.sentMessages),
		)
	}
}

// emptyQuestionFetcher has no new questions; due-review calls come from mockSRSRepo.
type emptyQuestionFetcher struct {
	mockSRSRepo
}

func (e *emptyQuestionFetcher) GetNewQuestions(
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
	mAPI := &mockBotAPI{}
	userSvc := service.NewUserService(&mockUserRepo{
		getOrCreateFn: func(
			ctx context.Context,
			id int64,
			username string,
		) (*model.User, error) {
			return &model.User{ID: id, Language: "ja", ProficiencyLevel: "N5"}, nil
		},
	})
	b := newTestBot(
		mAPI,
		nil,
		&service.Services{
			User: userSvc,
			Session: newTestSessionService(
				nil,
				service.SessionDeps{
					QuestionRepo:        &emptyQuestionFetcher{},
					SessionRepo:         &sessionListStore{},
					SessionQuestionRepo: &mockSessionQuestionStore{},
				},
			),
		},
	)

	msg := &tgbotapi.Message{Chat: &tgbotapi.Chat{ID: 1}, From: &tgbotapi.User{ID: 2}}
	b.handleTest(
		ctx,
		msg,
	)

	text := collectText(mAPI.sentMessages)
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
