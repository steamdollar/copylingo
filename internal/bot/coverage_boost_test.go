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

// --- helpers ---------------------------------------------------------------

// storeActiveState marshals state into the testRedis working-set key.
func storeActiveState(t *testing.T, rdb *testRedis, sessionID int, state *model.ActiveSessionState) {
	t.Helper()
	raw, err := json.Marshal(state)
	if err != nil {
		t.Fatalf("marshal active state: %v", err)
	}
	rdb.values[config.ActiveSessionWorkingSetRedisKey.Format(sessionID)] = string(raw)
}

// graderUserRepoStub satisfies the grader's user repo (UpdateStreak).
type graderUserRepoStub struct {
	updated bool
}

func (g *graderUserRepoStub) UpdateStreak(ctx context.Context, userID int64) error {
	g.updated = true
	return nil
}

// activeRepoStub satisfies the active session repository for Flush.
type activeRepoStub struct {
	flushed bool
}

func (a *activeRepoStub) LoadActiveSession(ctx context.Context, sessionID int) (*model.ActiveSessionState, error) {
	return nil, nil
}
func (a *activeRepoStub) FlushActiveSession(ctx context.Context, state *model.ActiveSessionState) error {
	a.flushed = true
	return nil
}

// botWithActive wires a Bot whose ActiveSession + Grader use the given repo/redis.
func botWithActive(rdb *testRedis, repo *activeRepoStub, userRepo *graderUserRepoStub) (*Bot, *mockBotAPI) {
	mAPI := &mockBotAPI{}
	active := service.NewActiveSessionService(repo, rdb, &mockSRS{})
	grader := service.NewGraderService(userRepo, active, &mockLLM{})
	b := &Bot{
		api: mAPI,
		rdb: rdb,
		cfg: &config.Config{},
		services: &service.Services{
			ActiveSession: active,
			Grader:        grader,
		},
	}
	return b, mAPI
}

func cbWithMessage(data string, chatID int64, msgID int, userID int64) *tgbotapi.CallbackQuery {
	return &tgbotapi.CallbackQuery{
		Data: data,
		From: &tgbotapi.User{ID: userID},
		Message: &tgbotapi.Message{
			Chat:      &tgbotapi.Chat{ID: chatID},
			MessageID: msgID,
		},
	}
}

func collectText(msgs []tgbotapi.Chattable) string {
	var sb strings.Builder
	for _, m := range msgs {
		switch v := m.(type) {
		case tgbotapi.MessageConfig:
			sb.WriteString(v.Text)
			sb.WriteString("\n")
		case tgbotapi.EditMessageTextConfig:
			sb.WriteString(v.Text)
			sb.WriteString("\n")
		}
	}
	return sb.String()
}

// --- finishSession ---------------------------------------------------------

func TestFinishSession_Summary(t *testing.T) {
	ctx := context.Background()
	rdb := &testRedis{values: map[string]string{}}
	repo := &activeRepoStub{}
	userRepo := &graderUserRepoStub{}
	b, mAPI := botWithActive(rdb, repo, userRepo)
	sf := NewSessionFlow(b)

	sessionID := 10
	userID := int64(999)
	correct := true
	wrong := false
	state := &model.ActiveSessionState{
		Version: model.ActiveSessionStateVersion,
		Session: model.Session{ID: sessionID, UserID: userID},
		Items: []model.ActiveSessionQuestion{
			{
				SessionQuestion: model.SessionQuestion{QuestionID: 1, IsCorrect: &correct},
				Question:        model.Question{ID: 1, Type: model.QuestionMultipleChoice},
			},
			{
				SessionQuestion: model.SessionQuestion{QuestionID: 2, IsCorrect: &wrong, UserAnswer: ptr("banana")},
				Question:        model.Question{ID: 2, Type: model.QuestionMultipleChoice, Prompt: "fruit?", CorrectAnswer: "apple"},
			},
		},
	}
	storeActiveState(t, rdb, sessionID, state)

	cb := cbWithMessage("session:10:finish", 123, 456, userID)
	sf.finishSession(ctx, cb, sessionID)

	if !repo.flushed {
		t.Error("expected FlushActiveSession to be called")
	}
	if !userRepo.updated {
		t.Error("expected UpdateStreak to be called")
	}
	text := collectText(mAPI.sentMessages)
	if !strings.Contains(text, "세션 완료") {
		t.Errorf("expected completion summary, got %q", text)
	}
	if !strings.Contains(text, "1/2") {
		t.Errorf("expected score 1/2 in summary, got %q", text)
	}
	if !strings.Contains(text, "틀린 문제") {
		t.Errorf("expected wrong-answer section, got %q", text)
	}
}

// --- HandleSessionCallback dispatch to finish ------------------------------

func TestHandleSessionCallback_Finish(t *testing.T) {
	ctx := context.Background()
	rdb := &testRedis{values: map[string]string{}}
	repo := &activeRepoStub{}
	userRepo := &graderUserRepoStub{}
	b, mAPI := botWithActive(rdb, repo, userRepo)
	sf := NewSessionFlow(b)

	sessionID := 11
	userID := int64(7)
	correct := true
	state := &model.ActiveSessionState{
		Version: model.ActiveSessionStateVersion,
		Session: model.Session{ID: sessionID, UserID: userID},
		Items: []model.ActiveSessionQuestion{
			{
				SessionQuestion: model.SessionQuestion{QuestionID: 1, IsCorrect: &correct},
				Question:        model.Question{ID: 1, Type: model.QuestionMultipleChoice},
			},
		},
	}
	storeActiveState(t, rdb, sessionID, state)

	sf.HandleSessionCallback(ctx, cbWithMessage("session:11:finish", 123, 456, userID))

	if !repo.flushed {
		t.Error("expected finish action to flush the session")
	}
	if len(mAPI.sentMessages) == 0 {
		t.Fatal("expected summary message")
	}
}

func TestHandleSessionCallback_BadData(t *testing.T) {
	ctx := context.Background()
	b, _ := botWithActive(&testRedis{values: map[string]string{}}, &activeRepoStub{}, &graderUserRepoStub{})
	sf := NewSessionFlow(b)

	// fewer than 3 parts -> early return, must not panic
	sf.HandleSessionCallback(ctx, cbWithMessage("session:onlytwo", 1, 2, 3))
	// non-numeric session id -> early return
	sf.HandleSessionCallback(ctx, cbWithMessage("session:abc:start", 1, 2, 3))
}

// --- HandleAnswerCallback --------------------------------------------------

func TestHandleAnswerCallback_OptionSelected(t *testing.T) {
	ctx := context.Background()
	rdb := &testRedis{values: map[string]string{}}
	repo := &activeRepoStub{}
	b, mAPI := botWithActive(rdb, repo, &graderUserRepoStub{})
	sf := NewSessionFlow(b)

	sessionID := 20
	questionID := 5
	opts, _ := json.Marshal([]string{"apple", "banana"})
	state := &model.ActiveSessionState{
		Version: model.ActiveSessionStateVersion,
		Session: model.Session{ID: sessionID},
		Items: []model.ActiveSessionQuestion{
			{
				SessionQuestion: model.SessionQuestion{QuestionID: questionID},
				Question: model.Question{
					ID: questionID, Type: model.QuestionMultipleChoice,
					Options: opts, CorrectAnswer: "apple", Explanation: "fruit",
				},
			},
		},
	}
	storeActiveState(t, rdb, sessionID, state)

	// q:{session}:{question}:{optionIdx} -> select option 0 ("apple", correct)
	data := "q:20:5:0"
	sf.HandleAnswerCallback(ctx, cbWithMessage(data, 123, 456, 1))

	text := collectText(mAPI.sentMessages)
	if !strings.Contains(text, "정답") {
		t.Errorf("expected correct-answer feedback, got %q", text)
	}
}

func TestHandleAnswerCallback_NextBeforeAnswering(t *testing.T) {
	ctx := context.Background()
	rdb := &testRedis{values: map[string]string{}}
	b, mAPI := botWithActive(rdb, &activeRepoStub{}, &graderUserRepoStub{})
	sf := NewSessionFlow(b)

	sessionID := 21
	state := &model.ActiveSessionState{
		Version: model.ActiveSessionStateVersion,
		Session: model.Session{ID: sessionID},
		Items: []model.ActiveSessionQuestion{
			{
				SessionQuestion: model.SessionQuestion{QuestionID: 1}, // unanswered
				Question:        model.Question{ID: 1, Type: model.QuestionKanaHandwriting},
			},
		},
	}
	storeActiveState(t, rdb, sessionID, state)

	// q:{session}:next:{idx} with current question unanswered -> prompt to submit first
	sf.HandleAnswerCallback(ctx, cbWithMessage("q:21:next:0", 123, 456, 1))

	text := collectText(mAPI.sentMessages)
	if !strings.Contains(text, "손글씨") {
		t.Errorf("expected handwriting submit prompt, got %q", text)
	}
}

func TestHandleAnswerCallback_BadData(t *testing.T) {
	ctx := context.Background()
	b, _ := botWithActive(&testRedis{values: map[string]string{}}, &activeRepoStub{}, &graderUserRepoStub{})
	sf := NewSessionFlow(b)

	// fewer than 4 parts -> early return, no panic
	sf.HandleAnswerCallback(ctx, cbWithMessage("q:20:5", 1, 2, 3))
	// non-numeric session id
	sf.HandleAnswerCallback(ctx, cbWithMessage("q:x:5:0", 1, 2, 3))
}

// --- showQuestion / renderByType -------------------------------------------

func TestShowQuestion_MultipleChoiceKeyboard(t *testing.T) {
	ctx := context.Background()
	rdb := &testRedis{values: map[string]string{}}
	b, mAPI := botWithActive(rdb, &activeRepoStub{}, &graderUserRepoStub{})
	sf := NewSessionFlow(b)

	sessionID := 30
	opts, _ := json.Marshal([]string{"A", "B", "C", "D"})
	state := &model.ActiveSessionState{
		Version: model.ActiveSessionStateVersion,
		Session: model.Session{ID: sessionID},
		Items: []model.ActiveSessionQuestion{
			{
				SessionQuestion: model.SessionQuestion{QuestionID: 1},
				Question:        model.Question{ID: 1, Type: model.QuestionMultipleChoice, Prompt: "Pick one", Options: opts},
			},
		},
	}
	storeActiveState(t, rdb, sessionID, state)

	sf.showQuestion(ctx, 123, nil, sessionID, 0)

	if len(mAPI.sentMessages) == 0 {
		t.Fatal("expected a question message")
	}
	msg, ok := mAPI.sentMessages[0].(tgbotapi.MessageConfig)
	if !ok {
		t.Fatalf("expected MessageConfig, got %T", mAPI.sentMessages[0])
	}
	if !strings.Contains(msg.Text, "Pick one") {
		t.Errorf("expected prompt text, got %q", msg.Text)
	}
	kb, ok := msg.ReplyMarkup.(tgbotapi.InlineKeyboardMarkup)
	if !ok {
		t.Fatalf("expected inline keyboard, got %T", msg.ReplyMarkup)
	}
	// 4 options -> 2 rows of 2
	if len(kb.InlineKeyboard) != 2 {
		t.Errorf("expected 2 option rows, got %d", len(kb.InlineKeyboard))
	}
}

func TestShowQuestion_AllAnsweredShowsFinish(t *testing.T) {
	ctx := context.Background()
	rdb := &testRedis{values: map[string]string{}}
	b, mAPI := botWithActive(rdb, &activeRepoStub{}, &graderUserRepoStub{})
	sf := NewSessionFlow(b)

	sessionID := 31
	state := &model.ActiveSessionState{
		Version: model.ActiveSessionStateVersion,
		Session: model.Session{ID: sessionID},
		Items: []model.ActiveSessionQuestion{
			{SessionQuestion: model.SessionQuestion{QuestionID: 1}, Question: model.Question{ID: 1}},
		},
	}
	storeActiveState(t, rdb, sessionID, state)

	// questionIdx beyond items -> finish button branch
	sf.showQuestion(ctx, 123, nil, sessionID, 5)

	text := collectText(mAPI.sentMessages)
	if !strings.Contains(text, "모든 문제를 풀었습니다") {
		t.Errorf("expected all-answered message, got %q", text)
	}
}

func TestShowQuestion_SubjectivePrompt(t *testing.T) {
	ctx := context.Background()
	rdb := &testRedis{values: map[string]string{}}
	b, mAPI := botWithActive(rdb, &activeRepoStub{}, &graderUserRepoStub{})
	sf := NewSessionFlow(b)

	sessionID := 32
	state := &model.ActiveSessionState{
		Version: model.ActiveSessionStateVersion,
		Session: model.Session{ID: sessionID},
		Items: []model.ActiveSessionQuestion{
			{
				SessionQuestion: model.SessionQuestion{QuestionID: 1},
				Question:        model.Question{ID: 1, Type: model.QuestionSubjective, Prompt: "Translate"},
			},
		},
	}
	storeActiveState(t, rdb, sessionID, state)

	sf.showQuestion(ctx, 123, nil, sessionID, 0)

	text := collectText(mAPI.sentMessages)
	if !strings.Contains(text, "텍스트로 입력") {
		t.Errorf("expected subjective input prompt, got %q", text)
	}
	// active-question marker should be stored in redis for text routing
	if _, ok := rdb.values[config.UserActiveQuestionRedisKey.Format(123)]; !ok {
		t.Error("expected active question key to be stored for subjective question")
	}
}

// --- processAnswerText: wrong answer & AI unavailable ----------------------

func TestProcessAnswerText_Wrong(t *testing.T) {
	ctx := context.Background()
	rdb := &testRedis{values: map[string]string{}}
	b, mAPI := botWithActive(rdb, &activeRepoStub{}, &graderUserRepoStub{})
	sf := NewSessionFlow(b)

	sessionID, questionID := 40, 1
	state := &model.ActiveSessionState{
		Version: model.ActiveSessionStateVersion,
		Session: model.Session{ID: sessionID},
		Items: []model.ActiveSessionQuestion{
			{
				SessionQuestion: model.SessionQuestion{QuestionID: questionID},
				Question:        model.Question{ID: questionID, Type: model.QuestionMultipleChoice, CorrectAnswer: "apple", Explanation: "fruit"},
			},
		},
	}
	storeActiveState(t, rdb, sessionID, state)

	sf.processAnswerText(ctx, 123, sessionID, questionID, "banana", nil)

	text := collectText(mAPI.sentMessages)
	if !strings.Contains(text, "오답") {
		t.Errorf("expected wrong-answer feedback, got %q", text)
	}
	if !strings.Contains(text, "apple") {
		t.Errorf("expected correct answer shown, got %q", text)
	}
}

func TestProcessAnswerText_SubjectiveAIUnavailable(t *testing.T) {
	ctx := context.Background()
	rdb := &testRedis{values: map[string]string{}}
	mAPI := &mockBotAPI{}
	active := service.NewActiveSessionService(&activeRepoStub{}, rdb, &mockSRS{})
	// LLM returns AI-unavailable error
	llm := &mockLLM{gradeFn: func(ctx context.Context, prompt, correctAnswer, userAnswer string) (bool, string, error) {
		return false, "", service.ErrAIUnavailable
	}}
	grader := service.NewGraderService(&graderUserRepoStub{}, active, llm)
	b := &Bot{
		api: mAPI, rdb: rdb, cfg: &config.Config{},
		services: &service.Services{ActiveSession: active, Grader: grader},
	}
	sf := NewSessionFlow(b)

	sessionID, questionID := 41, 1
	state := &model.ActiveSessionState{
		Version: model.ActiveSessionStateVersion,
		Session: model.Session{ID: sessionID},
		Items: []model.ActiveSessionQuestion{
			{
				SessionQuestion: model.SessionQuestion{QuestionID: questionID},
				Question:        model.Question{ID: questionID, Type: model.QuestionSubjective, Prompt: "Translate", CorrectAnswer: "x"},
			},
		},
	}
	storeActiveState(t, rdb, sessionID, state)

	sf.processAnswerText(ctx, 123, sessionID, questionID, "answer", nil)

	text := collectText(mAPI.sentMessages)
	if !strings.Contains(text, "AI 주관식 채점이 불가능") {
		t.Errorf("expected AI-unavailable notice, got %q", text)
	}
}

// --- handler.go stats/streak/menu paths ------------------------------------

type analyzerUserRepoStub struct{ streak int }

func (a *analyzerUserRepoStub) GetByID(ctx context.Context, id int64) (*model.User, error) {
	return &model.User{ID: id, StreakDays: a.streak, Language: "ja", ProficiencyLevel: "N5"}, nil
}

type statRepoStub struct{}

func (s *statRepoStub) GetTodayStats(ctx context.Context) (int, int, error) { return 10, 7, nil }
func (s *statRepoStub) GetCategoryAccuracy(ctx context.Context) (map[string]float64, error) {
	return map[string]float64{"vocabulary": 80, "grammar": 60}, nil
}

func botWithAnalyzer() (*Bot, *mockBotAPI) {
	mAPI := &mockBotAPI{}
	analyzer := service.NewAnalyzerService(&analyzerUserRepoStub{streak: 5}, &statRepoStub{})
	b := &Bot{
		api: mAPI, cfg: &config.Config{},
		services: &service.Services{Analyzer: analyzer},
	}
	return b, mAPI
}

func TestHandleStats(t *testing.T) {
	b, mAPI := botWithAnalyzer()
	msg := &tgbotapi.Message{Chat: &tgbotapi.Chat{ID: 1}, From: &tgbotapi.User{ID: 2}}

	b.handleStats(context.Background(), msg)

	text := collectText(mAPI.sentMessages)
	if !strings.Contains(text, "학습 통계") {
		t.Errorf("expected stats text, got %q", text)
	}
	if !strings.Contains(text, "5일") {
		t.Errorf("expected streak in stats, got %q", text)
	}
}

func TestHandleStreak(t *testing.T) {
	b, mAPI := botWithAnalyzer()
	msg := &tgbotapi.Message{Chat: &tgbotapi.Chat{ID: 1}, From: &tgbotapi.User{ID: 2}}

	b.handleStreak(context.Background(), msg)

	text := collectText(mAPI.sentMessages)
	if !strings.Contains(text, "스트릭") {
		t.Errorf("expected streak text, got %q", text)
	}
}

func TestHandleStatsCallback(t *testing.T) {
	b, mAPI := botWithAnalyzer()
	cb := cbWithMessage("menu:stats", 1, 2, 3)

	b.handleStatsCallback(context.Background(), cb)

	if len(mAPI.sentMessages) == 0 {
		t.Fatal("expected stats edit message")
	}
	edit, ok := mAPI.sentMessages[0].(tgbotapi.EditMessageTextConfig)
	if !ok {
		t.Fatalf("expected EditMessageTextConfig, got %T", mAPI.sentMessages[0])
	}
	if !strings.Contains(edit.Text, "학습 통계") {
		t.Errorf("expected stats text, got %q", edit.Text)
	}
}

func TestHandleMenu(t *testing.T) {
	mAPI := &mockBotAPI{}
	userSvc := service.NewUserService(&mockUserRepo{
		getOrCreateFn: func(ctx context.Context, id int64, username string) (*model.User, error) {
			return &model.User{ID: id, StreakDays: 3, Language: "ja", ProficiencyLevel: "N5"}, nil
		},
	})
	srs := service.NewSRSService(&mockSRSRepo{})
	b := &Bot{
		api: mAPI, cfg: &config.Config{},
		services: &service.Services{User: userSvc, SRS: srs},
	}

	msg := &tgbotapi.Message{Chat: &tgbotapi.Chat{ID: 1}, From: &tgbotapi.User{ID: 2}}
	b.handleMenu(context.Background(), msg)

	if len(mAPI.sentMessages) == 0 {
		t.Fatal("expected menu message")
	}
	sent := mAPI.sentMessages[0].(tgbotapi.MessageConfig)
	if !strings.Contains(sent.Text, "CopyLingo") {
		t.Errorf("expected menu text, got %q", sent.Text)
	}
}

func ptr(s string) *string { return &s }

func TestEditMessageReplyMarkup(t *testing.T) {
	mAPI := &mockBotAPI{}
	b := &Bot{api: mAPI}

	markup := tgbotapi.NewInlineKeyboardMarkup(
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData("x", "data"),
		),
	)
	if err := b.EditMessageReplyMarkup(123, 456, markup); err != nil {
		t.Fatalf("EditMessageReplyMarkup error = %v", err)
	}
	if len(mAPI.sentMessages) != 1 {
		t.Fatalf("expected 1 sent, got %d", len(mAPI.sentMessages))
	}
	if _, ok := mAPI.sentMessages[0].(tgbotapi.EditMessageReplyMarkupConfig); !ok {
		t.Fatalf("expected EditMessageReplyMarkupConfig, got %T", mAPI.sentMessages[0])
	}
}

func TestBotPushSession(t *testing.T) {
	mAPI := &mockBotAPI{}
	b := &Bot{api: mAPI}
	b.flow = NewSessionFlow(b)

	if err := b.PushSession(context.Background(), 123, 10, "evening"); err != nil {
		t.Fatalf("PushSession error = %v", err)
	}
	text := collectText(mAPI.sentMessages)
	if !strings.Contains(text, "세션이 도착했습니다") {
		t.Errorf("expected push message, got %q", text)
	}
}
