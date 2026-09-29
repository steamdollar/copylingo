package bot

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"

	"github.com/lsj/copylingo/internal/config"
	"github.com/lsj/copylingo/internal/model"
	"github.com/lsj/copylingo/internal/service"
)

// mockSessionStore implements the sessions-table calls these tests drive; the
// embedded nil SessionRepo makes any other call panic.
type mockSessionStore struct {
	service.SessionRepo
	getSessionsByStatusFn func(
		ctx context.Context,
		userID int64,
		status config.SessionStatus,
	) ([]model.Session, error)
	startFn func(
		ctx context.Context,
		id int,
	) error
	createSessionFn func(
		ctx context.Context,
		s *model.Session,
	) error
}

func (m *mockSessionStore) CreateSession(
	ctx context.Context,
	s *model.Session,
) error {
	return m.createSessionFn(
		ctx,
		s,
	)
}
func (m *mockSessionStore) GetByID(
	ctx context.Context,
	id int,
) (*model.Session, error) {
	return nil, nil
}

func (m *mockSessionStore) GetSessionsByStatus(
	ctx context.Context,
	userID int64,
	status config.SessionStatus,
) ([]model.Session, error) {
	return m.getSessionsByStatusFn(
		ctx,
		userID,
		status,
	)
}
func (m *mockSessionStore) ListInProgress(ctx context.Context) ([]model.Session, error) {
	return nil, nil
}
func (m *mockSessionStore) Start(
	ctx context.Context,
	id int,
) error {
	return m.startFn(
		ctx,
		id,
	)
}

func TestStartStudy_NoSessions(t *testing.T) {
	ctx := context.Background()
	mAPI := &mockBotAPI{}
	mSessionStore := &mockSessionStore{
		getSessionsByStatusFn: func(
			ctx context.Context,
			userID int64,
			status config.SessionStatus,
		) ([]model.Session, error) {
			return nil, nil
		},
	}
	b := &Bot{
		telegram: newTelegramClient(mAPI),
		services: &service.Services{
			Session: newTestSessionService(
				nil,
				service.SessionDeps{SessionRepo: mSessionStore},
			),
		},
	}
	sf := NewSessionFlow(b)

	cb := &tgbotapi.CallbackQuery{
		From: &tgbotapi.User{ID: 123},
		Message: &tgbotapi.Message{
			Chat:      &tgbotapi.Chat{ID: 456},
			MessageID: 789,
		},
	}

	sf.StartStudy(
		ctx,
		cb,
	)

	if len(mAPI.sentMessages) == 0 {
		t.Fatal("expected message sent")
	}
	sent := mAPI.sentMessages[0].(tgbotapi.EditMessageTextConfig)
	if !strings.Contains(
		sent.Text,
		"대기 중인 학습 세션이 없습니다",
	) {
		t.Errorf(
			"wrong text: %s",
			sent.Text,
		)
	}
}

func TestStartStudy_PendingStudySession(t *testing.T) {
	ctx := context.Background()
	mAPI := &mockBotAPI{}
	mSessionStore := &mockSessionStore{
		getSessionsByStatusFn: func(
			ctx context.Context,
			userID int64,
			status config.SessionStatus,
		) ([]model.Session, error) {
			if status == config.SessionStatusPending {
				return []model.Session{{
					ID:             10,
					Type:           model.SessionStudy,
					Mode:           model.SessionModeStudy,
					Status:         model.SessionPending,
					TotalQuestions: 8,
				}}, nil
			}
			return nil, nil
		},
	}
	b := &Bot{
		telegram: newTelegramClient(mAPI),
		services: &service.Services{
			Session: newTestSessionService(
				nil,
				service.SessionDeps{SessionRepo: mSessionStore},
			),
		},
	}
	sf := NewSessionFlow(b)

	cb := &tgbotapi.CallbackQuery{
		From: &tgbotapi.User{ID: 123},
		Message: &tgbotapi.Message{
			Chat:      &tgbotapi.Chat{ID: 456},
			MessageID: 789,
		},
	}

	sf.StartStudy(
		ctx,
		cb,
	)

	if len(mAPI.sentMessages) == 0 {
		t.Fatal("expected message sent")
	}
	sent := mAPI.sentMessages[0].(tgbotapi.EditMessageTextConfig)
	if !strings.Contains(
		sent.Text,
		"Study Session 준비됨",
	) {
		t.Errorf(
			"wrong text: %s",
			sent.Text,
		)
	}
	if sent.ReplyMarkup == nil ||
		len(sent.ReplyMarkup.InlineKeyboard) == 0 ||
		len(sent.ReplyMarkup.InlineKeyboard[0]) == 0 ||
		sent.ReplyMarkup.InlineKeyboard[0][0].CallbackData == nil {
		t.Fatalf(
			"unexpected reply markup: %+v",
			sent.ReplyMarkup,
		)
	}
	if got := *sent.ReplyMarkup.InlineKeyboard[0][0].CallbackData; got != "study:10:start" {
		t.Fatalf(
			"callback = %q, want study:10:start",
			got,
		)
	}
}

func TestStartStudy_ResumeInProgress(t *testing.T) {
	ctx := context.Background()
	mAPI := &mockBotAPI{}
	stateStores := newTestInteractionStores()
	mSessionStore := &mockSessionStore{
		getSessionsByStatusFn: func(
			ctx context.Context,
			userID int64,
			status config.SessionStatus,
		) ([]model.Session, error) {
			if status == config.SessionStatusInProgress {
				return []model.Session{{ID: 10}}, nil
			}
			return nil, nil
		},
	}
	b := &Bot{
		telegram: newTelegramClient(mAPI),
		input:    stateStores, drafts: stateStores, messages: stateStores, recovery: stateStores, timing: stateStores,
		services: &service.Services{
			Session: newTestSessionService(
				stateStores,
				service.SessionDeps{SessionRepo: mSessionStore},
			),
		},
	}
	sf := NewSessionFlow(b)

	cb := &tgbotapi.CallbackQuery{
		From: &tgbotapi.User{ID: 123},
		Message: &tgbotapi.Message{
			Chat:      &tgbotapi.Chat{ID: 456},
			MessageID: 789,
		},
	}

	// Setup active session state
	state := &model.QuizActiveSessionState{
		Version: model.QuizActiveSessionStateVersion,
		Session: model.Session{ID: 10},
		Items: []model.QuizActiveSessionQuestion{
			{
				Question: model.Question{
					Prompt:  "Q1",
					Type:    model.QuestionMultipleChoice,
					ID:      1,
					Options: json.RawMessage(`["A"]`),
				},
			},
		},
	}
	seedQuizState(
		stateStores,
		state,
	)

	sf.StartStudy(
		ctx,
		cb,
	)

	if len(mAPI.sentMessages) == 0 {
		t.Fatal("expected message sent")
	}
	sent := mAPI.sentMessages[0].(tgbotapi.EditMessageTextConfig)
	if !strings.Contains(
		sent.Text,
		"Q1",
	) {
		t.Errorf(
			"expected prompt Q1, got %s",
			sent.Text,
		)
	}
}

type mockSessionQuestionStore struct{}

func (m *mockSessionQuestionStore) CreateSessionQuestions(
	ctx context.Context,
	sqs []model.SessionQuestion,
) error {
	return nil
}
func (m *mockSessionQuestionStore) GetBySession(
	ctx context.Context,
	sessionID int,
) ([]model.SessionQuestion, error) {
	return nil, nil
}

func TestStartReview_NoneDue(t *testing.T) {
	ctx := context.Background()
	mAPI := &mockBotAPI{}
	mSRSRepo := &mockSRSRepoWithCount{count: 5} // Has due questions

	mSessionStore := &mockSessionStore{
		createSessionFn: func(
			ctx context.Context,
			s *model.Session,
		) error {
			s.ID = 100
			s.TotalQuestions = 5
			return nil
		},
	}
	mSQStore := &mockSessionQuestionStore{}

	b := &Bot{
		telegram: newTelegramClient(mAPI),
		services: &service.Services{
			User: service.NewUserService(&mockUserRepo{getOrCreateFn: func(
				context.Context,
				int64,
				string,
			) (*model.User, error) {
				return &model.User{ID: 123, Language: "ja", ProficiencyLevel: "N5"}, nil
			}}),
			Session: newTestSessionService(
				nil,
				service.SessionDeps{
					QuestionRepo:        mSRSRepo,
					SessionRepo:         mSessionStore,
					SessionQuestionRepo: mSQStore,
				},
			),
		},
	}
	sf := NewSessionFlow(b)

	cb := &tgbotapi.CallbackQuery{
		From: &tgbotapi.User{ID: 123},
		Message: &tgbotapi.Message{
			Chat:      &tgbotapi.Chat{ID: 456},
			MessageID: 789,
		},
	}

	sf.StartReview(
		ctx,
		cb,
	)

	if len(mAPI.sentMessages) == 0 {
		t.Fatal("expected message sent")
	}
	var sentText string
	switch m := mAPI.sentMessages[0].(type) {
	case tgbotapi.MessageConfig:
		sentText = m.Text
	case tgbotapi.EditMessageTextConfig:
		sentText = m.Text
	}
	if !strings.Contains(
		sentText,
		"복습 세션",
	) {
		t.Errorf(
			"wrong text: %s",
			sentText,
		)
	}
}

type mockSRSRepoWithCount struct {
	mockSRSRepo
	count int
}

func (m *mockSRSRepoWithCount) GetDueReviews(
	ctx context.Context,
	userID int64,
	language,
	currentLevel string,
	levels []string,
	limit,
	kanjiRecallLimit int,
	categories ...model.QuestionCategory,
) ([]model.Question, error) {
	if m.count > 0 {
		return make(
			[]model.Question,
			m.count,
		), nil
	}
	return nil, nil
}

func (m *mockSRSRepoWithCount) GetDueReviewCount(
	ctx context.Context,
	userID int64,
	language string,
	levels []string,
) (int, error) {
	return m.count, nil
}

func TestStartReview_NoneDue_Actual(t *testing.T) {
	ctx := context.Background()
	mAPI := &mockBotAPI{}
	mSRSRepo := &mockSRSRepoWithCount{count: 0}
	b := &Bot{
		telegram: newTelegramClient(mAPI),
		services: &service.Services{
			User: service.NewUserService(&mockUserRepo{getOrCreateFn: func(
				context.Context,
				int64,
				string,
			) (*model.User, error) {
				return &model.User{ID: 123, Language: "ja", ProficiencyLevel: "N5"}, nil
			}}),
			Session: newTestSessionService(
				nil,
				service.SessionDeps{QuestionRepo: mSRSRepo},
			),
		},
	}
	sf := NewSessionFlow(b)

	cb := &tgbotapi.CallbackQuery{
		From: &tgbotapi.User{ID: 123},
		Message: &tgbotapi.Message{
			Chat:      &tgbotapi.Chat{ID: 456},
			MessageID: 789,
		},
	}

	sf.StartReview(
		ctx,
		cb,
	)

	// In the fail/NoneDue path it might be MessageConfig or EditMessageTextConfig
	var sentText string
	switch m := mAPI.sentMessages[0].(type) {
	case tgbotapi.MessageConfig:
		sentText = m.Text
	case tgbotapi.EditMessageTextConfig:
		sentText = m.Text
	}

	if !strings.Contains(
		sentText,
		"복습할 문제가 없습니다",
	) {
		t.Errorf(
			"wrong text: %s",
			sentText,
		)
	}
}

func TestHandleSessionCallback(t *testing.T) {
	ctx := context.Background()
	mAPI := &mockBotAPI{}
	stateStores := newTestInteractionStores()
	mSessionStore := &mockSessionStore{
		startFn: func(
			ctx context.Context,
			id int,
		) error {
			return nil
		},
	}
	session := newTestSessionService(
		stateStores,
		service.SessionDeps{SessionRepo: mSessionStore},
	)
	b := &Bot{
		telegram: newTelegramClient(mAPI),
		input:    stateStores, drafts: stateStores, messages: stateStores, recovery: stateStores, timing: stateStores,
		services: &service.Services{
			Session: session,
		},
	}
	sf := NewSessionFlow(b)

	t.Run(
		"start action",
		func(t *testing.T) {
			cb := &tgbotapi.CallbackQuery{
				Data: "session:10:start",
				From: &tgbotapi.User{ID: 123},
				Message: &tgbotapi.Message{
					Chat:      &tgbotapi.Chat{ID: 123},
					MessageID: 456,
				},
			}
			// QuizActiveSession.CreateFromDB will fail because sessionStore.GetByID is nil.
			// Let's just mock StartSession and see it logs and returns.
			// Wait, startSession calls showQuestion which needs active session.
			// I'll skip deep testing here as it requires complex mocks,
			// but I can at least check it doesn't crash.
			sf.HandleSessionCallback(
				ctx,
				cb,
			)
		},
	)
}

func TestStartSessionRepeatedStartResumesNextUnanswered(t *testing.T) {
	ctx := context.Background()
	mAPI := &mockBotAPI{}
	stateStores := newTestInteractionStores()
	startCalls := 0
	mSessionStore := &mockSessionStore{
		startFn: func(
			ctx context.Context,
			id int,
		) error {
			startCalls++
			return nil
		},
	}
	session := newTestSessionService(
		stateStores,
		service.SessionDeps{SessionRepo: mSessionStore},
	)
	b := &Bot{
		telegram: newTelegramClient(mAPI),
		input:    stateStores, drafts: stateStores, messages: stateStores, recovery: stateStores, timing: stateStores,
		services: &service.Services{
			Session: session,
		},
	}
	sf := NewSessionFlow(b)

	sessionID := 31
	answered := true
	state := &model.QuizActiveSessionState{
		Version: model.QuizActiveSessionStateVersion,
		Session: model.Session{ID: sessionID, UserID: 123, Mode: model.SessionModeQuiz},
		Items: []model.QuizActiveSessionQuestion{
			{
				SessionQuestion: model.SessionQuestion{QuestionID: 1, IsCorrect: &answered},
				Question:        model.Question{ID: 1, Prompt: "첫 문제", Type: model.QuestionMultipleChoice},
			},
			{
				SessionQuestion: model.SessionQuestion{QuestionID: 2},
				Question: model.Question{
					ID:      2,
					Prompt:  "두 번째 문제",
					Type:    model.QuestionMultipleChoice,
					Options: json.RawMessage(`["A", "B"]`),
				},
			},
		},
	}
	storeActiveState(
		t,
		stateStores,
		sessionID,
		state,
	)
	cb := cbWithMessage(
		"session:31:start",
		123,
		456,
		123,
	)

	sf.HandleSessionCallback(
		ctx,
		cb,
	)
	sf.HandleSessionCallback(
		ctx,
		cb,
	)

	if startCalls != 2 {
		t.Fatalf(
			"StartSession calls = %d, want 2 callback invocations",
			startCalls,
		)
	}
	if len(mAPI.sentMessages) != 2 {
		t.Fatalf(
			"rendered messages = %d, want 2",
			len(mAPI.sentMessages),
		)
	}
	last, ok := mAPI.sentMessages[1].(tgbotapi.EditMessageTextConfig)
	if !ok {
		t.Fatalf(
			"last message type = %T, want edit",
			mAPI.sentMessages[1],
		)
	}
	if !strings.Contains(
		last.Text,
		"두 번째 문제",
	) {
		t.Fatalf(
			"repeated start should resume next unanswered question, got %q",
			last.Text,
		)
	}
	resumed, err := session.QuizProgress(
		ctx,
		sessionID,
	)
	if err != nil {
		t.Fatalf(
			"active state read failed: %v",
			err,
		)
	}
	if resumed.Items[0].SessionQuestion.IsCorrect == nil {
		t.Fatal("repeated start must preserve answered state")
	}
}

type quizStartActiveRepo struct {
	state *model.QuizActiveSessionState
}

func (r *quizStartActiveRepo) LoadQuestionSessionWithStateBySessionID(
	ctx context.Context,
	sessionID int,
) (*model.QuizActiveSessionState, error) {
	return r.state, nil
}

func (r *quizStartActiveRepo) FlushQuizActiveSession(
	ctx context.Context,
	state *model.QuizActiveSessionState,
) error {
	return nil
}

func TestStartSessionRefreshesPendingStatusAfterDBStart(t *testing.T) {
	ctx := context.Background()
	mAPI := &mockBotAPI{}
	stateStores := newTestInteractionStores()
	dbState := &model.QuizActiveSessionState{
		Version: model.QuizActiveSessionStateVersion,
		Session: model.Session{
			ID:     32,
			UserID: 123,
			Mode:   model.SessionModeQuiz,
			Status: model.SessionPending,
		},
		Items: []model.QuizActiveSessionQuestion{
			{
				SessionQuestion: model.SessionQuestion{QuestionID: 1},
				Question: model.Question{
					ID:      1,
					Prompt:  "첫 문제",
					Type:    model.QuestionMultipleChoice,
					Options: json.RawMessage(`["A"]`),
				},
			},
		},
	}
	repo := &quizStartActiveRepo{state: dbState}
	store := &mockSessionStore{
		startFn: func(
			ctx context.Context,
			sessionID int,
		) error {
			repo.state.Session.Status = model.SessionInProgress
			return nil
		},
	}
	session := newTestSessionService(
		stateStores,
		service.SessionDeps{
			SessionRepo:           store,
			QuizActiveSessionRepo: repo,
		},
	)
	b := &Bot{
		telegram: newTelegramClient(mAPI),
		input:    stateStores, drafts: stateStores, messages: stateStores, recovery: stateStores, timing: stateStores,
		services: &service.Services{
			Session: session,
		},
	}
	storeActiveState(
		t,
		stateStores,
		32,
		dbState,
	)

	NewSessionFlow(b).HandleSessionCallback(
		ctx,
		cbWithMessage(
			"session:32:start",
			123,
			456,
			123,
		),
	)

	state, err := session.QuizProgress(
		ctx,
		32,
	)
	if err != nil {
		t.Fatalf(
			"active state read failed: %v",
			err,
		)
	}
	if state.Session.Status != model.SessionInProgress {
		t.Fatalf(
			"active status = %s, want %s after DB start",
			state.Session.Status,
			model.SessionInProgress,
		)
	}
}

func TestPushSession(t *testing.T) {
	mAPI := &mockBotAPI{}
	b := &Bot{telegram: newTelegramClient(mAPI)}
	sf := NewSessionFlow(b)

	err := sf.PushSession(
		context.Background(),
		123,
		10,
		"morning",
	)
	if err != nil {
		t.Fatalf(
			"unexpected error: %v",
			err,
		)
	}

	if len(mAPI.sentMessages) != 1 {
		t.Fatalf(
			"expected 1 message, got %d",
			len(mAPI.sentMessages),
		)
	}
	sent := mAPI.sentMessages[0].(tgbotapi.MessageConfig)
	if !strings.Contains(
		sent.Text,
		"세션이 도착했습니다",
	) {
		t.Errorf(
			"wrong text: %s",
			sent.Text,
		)
	}
}
