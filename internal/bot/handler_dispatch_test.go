package bot

import (
	"context"
	"errors"
	"strings"
	"testing"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"

	"github.com/lsj/copylingo/internal/config"
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

func (m *mockSRSRepo) GetDueReviews(
	ctx context.Context,
	userID int64,
	language, level string,
	limit, kanjiRecallLimit int,
) ([]model.Question, error) {
	return nil, nil
}
func (m *mockSRSRepo) GetDueReviewCount(ctx context.Context, userID int64, language, level string) (int, error) {
	return 5, nil // Return 5 for main menu display test
}

type mockStatsRepo struct {
	getTodayStatsFn func(ctx context.Context, userID int64) (*model.UserStats, error)
}

func (m *mockStatsRepo) GetTodayStats(ctx context.Context, userID int64) (*model.UserStats, error) {
	return m.getTodayStatsFn(ctx, userID)
}
func (m *mockStatsRepo) SaveDailyStats(ctx context.Context, stats *model.UserStats) error { return nil }

type commandStudyMaterialStore struct {
	materials []model.Material
	err       error
	userID    int64
	language  string
	level     string
	limit     int
}

func (s *commandStudyMaterialStore) GetForStudySession(
	ctx context.Context,
	userID int64,
	language, level string,
	limit int,
) ([]model.Material, error) {
	s.userID = userID
	s.language = language
	s.level = level
	s.limit = limit
	if s.err != nil {
		return nil, s.err
	}
	return s.materials, nil
}

type commandStudySessionStore struct {
	nextID  int
	created []*model.Session
	err     error
}

func (s *commandStudySessionStore) CreateSession(ctx context.Context, session *model.Session) error {
	if s.err != nil {
		return s.err
	}
	session.ID = s.nextID
	s.created = append(s.created, session)
	return nil
}

type commandStudySessionMaterialStore struct {
	created []model.SessionMaterial
	err     error
}

func (s *commandStudySessionMaterialStore) CreateSessionMaterials(
	ctx context.Context,
	sms []model.SessionMaterial,
) error {
	if s.err != nil {
		return s.err
	}
	s.created = append(s.created, sms...)
	return nil
}

type llmTipCandidateStore struct {
	created []*model.TipCandidate
	err     error
}

func (s *llmTipCandidateStore) Create(ctx context.Context, candidate *model.TipCandidate) error {
	return s.CreateCandidate(ctx, candidate)
}

func (s *llmTipCandidateStore) CreateCandidate(ctx context.Context, candidate *model.TipCandidate) error {
	if s.err != nil {
		return s.err
	}
	s.created = append(s.created, candidate)
	return nil
}

func (s *llmTipCandidateStore) ListActive(
	ctx context.Context,
	language, level string,
	limit int,
) ([]model.Tip, error) {
	return nil, nil
}

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

func TestHandleLLMCommandAllowedActivatesMode(t *testing.T) {
	api := &mockBotAPI{}
	rdb := &testRedis{values: map[string]string{}}
	b := &Bot{
		api: api,
		rdb: rdb,
	}

	allowedUserID := config.LLMAllowedTelegramUserIDs[0]
	b.handleMessage(context.Background(), commandMessage("/llm", allowedUserID, 456, "learner"))

	if got := rdb.values[config.UserLLMPendingRedisKey.Format(allowedUserID)]; got != "1" {
		t.Fatalf("LLM pending key = %q, want 1", got)
	}
	if len(api.sentMessages) != 1 {
		t.Fatalf("sent messages = %d, want 1", len(api.sentMessages))
	}
	sent := api.sentMessages[0].(tgbotapi.MessageConfig)
	if !strings.Contains(sent.Text, "LLM mode 활성화") {
		t.Fatalf("message text = %q", sent.Text)
	}
}

func TestHandleLLMCommandUnauthorizedReturnsWithoutMessage(t *testing.T) {
	api := &mockBotAPI{}
	rdb := &testRedis{values: map[string]string{}}
	b := &Bot{
		api: api,
		rdb: rdb,
	}

	b.handleMessage(context.Background(), commandMessage("/llm", config.LLMAllowedTelegramUserIDs[0]+1, 456, "learner"))

	if len(rdb.values) != 0 {
		t.Fatalf("redis values = %+v, want empty", rdb.values)
	}
	if len(api.sentMessages) != 0 {
		t.Fatalf("sent messages = %d, want 0", len(api.sentMessages))
	}
}

func TestHandleLLMQuestionAnswersAndCreatesTipCandidateWithUserLevel(t *testing.T) {
	api := &mockBotAPI{}
	allowedUserID := config.LLMAllowedTelegramUserIDs[0]
	rdb := &testRedis{values: map[string]string{
		config.UserLLMPendingRedisKey.Format(allowedUserID): "1",
	}}
	tipStore := &llmTipCandidateStore{}
	userRepo := &mockUserRepo{
		getOrCreateFn: func(ctx context.Context, id int64, username string) (*model.User, error) {
			if id != allowedUserID || username != "learner" {
				t.Fatalf("GetUser args = (%d, %s), want (%d, learner)",
					id, username, allowedUserID)
			}
			return &model.User{ID: id, Username: username, Language: "ja", ProficiencyLevel: "N4"}, nil
		},
	}
	var gotQuestion string
	b := &Bot{
		api: api,
		cfg: &config.Config{LLM: config.LLMConfig{
			Model: "test-model",
		}},
		rdb: rdb,
		services: &service.Services{
			User: service.NewUserService(userRepo),
			LLM: service.NewLLMService(&mockLLM{
				answerFn: func(ctx context.Context, question string) (string, error) {
					gotQuestion = question
					return "honoo는 불꽃이고 <tag>는 escape 대상입니다.", nil
				},
			}),
			Tip: service.NewTipService(tipStore),
		},
	}

	b.handleMessage(context.Background(),
		plainMessage("hi, honowo의 차이가 뭐야?", allowedUserID, 456, "learner"))

	if gotQuestion != "hi, honowo의 차이가 뭐야?" {
		t.Fatalf("question = %q", gotQuestion)
	}
	if _, ok := rdb.values[config.UserLLMPendingRedisKey.Format(allowedUserID)]; ok {
		t.Fatal("LLM pending key still exists")
	}
	if len(tipStore.created) != 1 {
		t.Fatalf("tip candidates = %d, want 1", len(tipStore.created))
	}
	candidate := tipStore.created[0]
	if candidate.UserID != allowedUserID || candidate.Username != "learner" ||
		candidate.Language != "ja" || candidate.ProficiencyLevel != "N4" ||
		candidate.Question != "hi, honowo의 차이가 뭐야?" {
		t.Fatalf("candidate = %+v", candidate)
	}
	if candidate.SourceModel == nil || *candidate.SourceModel != "test-model" {
		t.Fatalf("source model = %#v, want test-model", candidate.SourceModel)
	}
	if len(api.sentMessages) != 2 {
		t.Fatalf("sent messages = %d, want 2", len(api.sentMessages))
	}
	answer := api.sentMessages[1].(tgbotapi.MessageConfig)
	if !strings.Contains(answer.Text, "&lt;tag&gt;") {
		t.Fatalf("answer text was not escaped: %q", answer.Text)
	}
}

func TestHandleLLMQuestionConsumesModeOnAnswerFailure(t *testing.T) {
	api := &mockBotAPI{}
	allowedUserID := config.LLMAllowedTelegramUserIDs[0]
	rdb := &testRedis{values: map[string]string{
		config.UserLLMPendingRedisKey.Format(allowedUserID): "1",
	}}
	userRepo := &mockUserRepo{
		getOrCreateFn: func(ctx context.Context, id int64, username string) (*model.User, error) {
			return &model.User{ID: id, Username: username, Language: "ja", ProficiencyLevel: "N5"}, nil
		},
	}
	b := &Bot{
		api: api,
		rdb: rdb,
		services: &service.Services{
			User: service.NewUserService(userRepo),
			LLM: service.NewLLMService(&mockLLM{
				answerFn: func(ctx context.Context, question string) (string, error) {
					return "", errors.New("provider failed")
				},
			}),
			Tip: service.NewTipService(&llmTipCandidateStore{}),
		},
	}

	b.handleMessage(context.Background(), plainMessage("honoo가 뭐야?", allowedUserID, 456, "learner"))

	if _, ok := rdb.values[config.UserLLMPendingRedisKey.Format(allowedUserID)]; ok {
		t.Fatal("LLM pending key still exists after answer failure")
	}
	if len(api.sentMessages) != 2 {
		t.Fatalf("sent messages = %d, want 2", len(api.sentMessages))
	}
	failure := api.sentMessages[1].(tgbotapi.MessageConfig)
	if !strings.Contains(failure.Text, "다시 질문하려면 /llm") {
		t.Fatalf("failure text = %q", failure.Text)
	}
}

func TestHandleMessage_StudyCommandBuildsAndPushesStudySession(t *testing.T) {
	ctx := context.Background()
	api := &mockBotAPI{}
	userRepo := &mockUserRepo{
		getOrCreateFn: func(ctx context.Context, id int64, username string) (*model.User, error) {
			if id != 123 || username != "learner" {
				t.Fatalf("GetUser args = (%d, %s), want (123, learner)", id, username)
			}
			return &model.User{ID: id, Username: username, Language: "ja", ProficiencyLevel: "N5"}, nil
		},
	}
	materialStore := &commandStudyMaterialStore{
		materials: []model.Material{
			{ID: 10, Category: model.MaterialCategoryVocabulary, Language: "ja", ProficiencyLevel: "N5", Title: "みず"},
			{ID: 11, Category: model.MaterialCategoryVocabulary, Language: "ja", ProficiencyLevel: "N5", Title: "ひと"},
		},
	}
	sessionStore := &commandStudySessionStore{nextID: 321}
	sessionMaterialStore := &commandStudySessionMaterialStore{}
	b := &Bot{
		api: api,
		services: &service.Services{
			User:         service.NewUserService(userRepo),
			StudySession: service.NewStudySessionService(materialStore, sessionStore, sessionMaterialStore),
		},
	}

	b.handleMessage(ctx, commandMessage("/study", 123, 456, "learner"))

	if materialStore.userID != 123 || materialStore.language != "ja" ||
		materialStore.level != "N5" || materialStore.limit != service.DefaultStudySessionMaterialCount {
		t.Fatalf("GetForStudySession args = (%d, %s, %s, %d), want user/language/level and default limit %d",
			materialStore.userID, materialStore.language, materialStore.level, materialStore.limit,
			service.DefaultStudySessionMaterialCount)
	}
	if len(sessionStore.created) != 1 {
		t.Fatalf("created sessions = %d, want 1", len(sessionStore.created))
	}
	session := sessionStore.created[0]
	if session.UserID != 123 || session.Type != model.SessionStudy ||
		session.Mode != model.SessionModeStudy || session.Status != model.SessionPending ||
		session.TotalQuestions != 2 {
		t.Fatalf("created session = %+v", session)
	}
	if len(sessionMaterialStore.created) != 2 {
		t.Fatalf("created session materials = %d, want 2", len(sessionMaterialStore.created))
	}
	if sessionMaterialStore.created[0].SessionID != 321 ||
		sessionMaterialStore.created[0].MaterialID != 10 ||
		sessionMaterialStore.created[0].MaterialOrder != 0 ||
		sessionMaterialStore.created[1].MaterialID != 11 ||
		sessionMaterialStore.created[1].MaterialOrder != 1 {
		t.Fatalf("created session materials = %+v", sessionMaterialStore.created)
	}

	if len(api.sentMessages) != 1 {
		t.Fatalf("sent messages = %d, want 1", len(api.sentMessages))
	}
	msg, ok := api.sentMessages[0].(tgbotapi.MessageConfig)
	if !ok {
		t.Fatalf("sent message type = %T, want MessageConfig", api.sentMessages[0])
	}
	if !strings.Contains(msg.Text, "정오 학습 세션") {
		t.Fatalf("message text = %q", msg.Text)
	}
	if got := onlyMessageCallbackData(t, msg); got != "study:321:start" {
		t.Fatalf("callback data = %q, want study:321:start", got)
	}
	if msg.ChatID != 456 {
		t.Fatalf("chat ID = %d, want 456", msg.ChatID)
	}
}

func TestHandleStudyCommandUsesRequestedLimit(t *testing.T) {
	api := &mockBotAPI{}
	materialStore := &commandStudyMaterialStore{
		materials: []model.Material{
			{ID: 10, Category: model.MaterialCategoryVocabulary, Language: "ja", ProficiencyLevel: "N5", Title: "みず"},
			{ID: 11, Category: model.MaterialCategoryVocabulary, Language: "ja", ProficiencyLevel: "N5", Title: "ひと"},
		},
	}
	sessionStore := &commandStudySessionStore{nextID: 321}
	b := botWithStudyCommandDeps(api, nil, materialStore, sessionStore, &commandStudySessionMaterialStore{})

	b.handleStudy(context.Background(), commandMessage("/study 20", 123, 456, "learner"))

	if materialStore.limit != 20 {
		t.Fatalf("limit = %d, want 20", materialStore.limit)
	}
	if len(sessionStore.created) != 1 {
		t.Fatalf("created sessions = %d, want 1", len(sessionStore.created))
	}
}

func TestHandleStudyCommandRejectsInvalidLimit(t *testing.T) {
	api := &mockBotAPI{}
	materialStore := &commandStudyMaterialStore{
		materials: []model.Material{{ID: 10, Category: model.MaterialCategoryVocabulary}},
	}
	sessionStore := &commandStudySessionStore{nextID: 321}
	b := botWithStudyCommandDeps(api, nil, materialStore, sessionStore, &commandStudySessionMaterialStore{})

	b.handleStudy(context.Background(), commandMessage("/study 999", 123, 456, "learner"))

	if materialStore.limit != 0 {
		t.Fatalf("limit = %d, want 0 because material lookup should not run", materialStore.limit)
	}
	if len(sessionStore.created) != 0 {
		t.Fatalf("created sessions = %d, want 0", len(sessionStore.created))
	}
	sent := api.sentMessages[0].(tgbotapi.MessageConfig)
	if !strings.Contains(sent.Text, "사용법: /study") {
		t.Fatalf("message text = %q", sent.Text)
	}
}

func TestHandleStudyCommandNoMaterials(t *testing.T) {
	api := &mockBotAPI{}
	sessionStore := &commandStudySessionStore{nextID: 321}
	b := botWithStudyCommandDeps(api, nil, &commandStudyMaterialStore{}, sessionStore, nil)

	b.handleStudy(context.Background(), commandMessage("/study", 123, 456, "learner"))

	if len(sessionStore.created) != 0 {
		t.Fatalf("created sessions = %d, want 0", len(sessionStore.created))
	}
	sent := api.sentMessages[0].(tgbotapi.MessageConfig)
	if !strings.Contains(sent.Text, "학습 가능한 Study Material이 없습니다") {
		t.Fatalf("message text = %q", sent.Text)
	}
}

func TestHandleStudyCommandUserLookupFailure(t *testing.T) {
	api := &mockBotAPI{}
	b := botWithStudyCommandDeps(api, errors.New("lookup failed"), &commandStudyMaterialStore{}, nil, nil)

	b.handleStudy(context.Background(), commandMessage("/study", 123, 456, "learner"))

	sent := api.sentMessages[0].(tgbotapi.MessageConfig)
	if !strings.Contains(sent.Text, "사용자 정보를 확인할 수 없습니다") {
		t.Fatalf("message text = %q", sent.Text)
	}
}

func TestHandleStudyCommandBuildFailure(t *testing.T) {
	api := &mockBotAPI{}
	materialStore := &commandStudyMaterialStore{err: errors.New("material failed")}
	b := botWithStudyCommandDeps(api, nil, materialStore, nil, nil)

	b.handleStudy(context.Background(), commandMessage("/study", 123, 456, "learner"))

	sent := api.sentMessages[0].(tgbotapi.MessageConfig)
	if !strings.Contains(sent.Text, "Study Session 생성 중 오류") {
		t.Fatalf("message text = %q", sent.Text)
	}
}

func TestHandleStudyCommandPushFailure(t *testing.T) {
	api := &mockBotAPI{sendErr: errors.New("telegram failed")}
	materialStore := &commandStudyMaterialStore{
		materials: []model.Material{
			{ID: 10, Category: model.MaterialCategoryVocabulary, Language: "ja", ProficiencyLevel: "N5", Title: "みず"},
		},
	}
	sessionStore := &commandStudySessionStore{nextID: 321}
	b := botWithStudyCommandDeps(api, nil, materialStore, sessionStore, &commandStudySessionMaterialStore{})

	b.handleStudy(context.Background(), commandMessage("/study", 123, 456, "learner"))

	if len(api.sentMessages) != 2 {
		t.Fatalf("sent messages = %d, want 2", len(api.sentMessages))
	}
	fallback := api.sentMessages[1].(tgbotapi.MessageConfig)
	if !strings.Contains(fallback.Text, "Study Session 발송에 실패했습니다") {
		t.Fatalf("fallback text = %q", fallback.Text)
	}
}

func botWithStudyCommandDeps(
	api *mockBotAPI,
	userErr error,
	materialStore *commandStudyMaterialStore,
	sessionStore *commandStudySessionStore,
	sessionMaterialStore *commandStudySessionMaterialStore,
) *Bot {
	if materialStore == nil {
		materialStore = &commandStudyMaterialStore{}
	}
	if sessionStore == nil {
		sessionStore = &commandStudySessionStore{nextID: 321}
	}
	if sessionMaterialStore == nil {
		sessionMaterialStore = &commandStudySessionMaterialStore{}
	}

	userRepo := &mockUserRepo{
		getOrCreateFn: func(ctx context.Context, id int64, username string) (*model.User, error) {
			if userErr != nil {
				return nil, userErr
			}
			return &model.User{ID: id, Username: username, Language: "ja", ProficiencyLevel: "N5"}, nil
		},
	}
	return &Bot{
		api: api,
		services: &service.Services{
			User:         service.NewUserService(userRepo),
			StudySession: service.NewStudySessionService(materialStore, sessionStore, sessionMaterialStore),
		},
	}
}

func commandMessage(text string, userID, chatID int64, username string) *tgbotapi.Message {
	commandLength := len(text)
	if idx := strings.IndexByte(text, ' '); idx >= 0 {
		commandLength = idx
	}
	return &tgbotapi.Message{
		Text: text,
		Entities: []tgbotapi.MessageEntity{
			{Type: "bot_command", Offset: 0, Length: commandLength},
		},
		From: &tgbotapi.User{ID: userID, UserName: username},
		Chat: &tgbotapi.Chat{ID: chatID},
	}
}

func plainMessage(text string, userID, chatID int64, username string) *tgbotapi.Message {
	return &tgbotapi.Message{
		Text: text,
		From: &tgbotapi.User{ID: userID, UserName: username},
		Chat: &tgbotapi.Chat{ID: chatID},
	}
}

func onlyMessageCallbackData(t *testing.T, msg tgbotapi.MessageConfig) string {
	t.Helper()
	markup, ok := msg.ReplyMarkup.(tgbotapi.InlineKeyboardMarkup)
	if !ok {
		t.Fatalf("reply markup type = %T, want InlineKeyboardMarkup", msg.ReplyMarkup)
	}
	if len(markup.InlineKeyboard) != 1 || len(markup.InlineKeyboard[0]) != 1 {
		t.Fatalf("unexpected reply markup: %+v", markup)
	}
	data := markup.InlineKeyboard[0][0].CallbackData
	if data == nil {
		t.Fatal("callback data is nil")
	}
	return *data
}
