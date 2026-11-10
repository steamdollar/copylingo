package bot

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"

	"github.com/lsj/copylingo/internal/model"
	"github.com/lsj/copylingo/internal/service"
)

func storeActiveState(
	t *testing.T,
	stateStores *testInteractionStores,
	sessionID int,
	state *model.QuizActiveSessionState,
) {
	t.Helper()
	state.Session.ID = sessionID
	seedQuizState(
		stateStores,
		state,
	)
}

type streakRepositoryStub struct {
	updated bool
}

func (stub *streakRepositoryStub) UpdateStreak(
	ctx context.Context,
	userID int64,
) error {
	stub.updated = true
	return nil
}

type activeRepositoryStub struct {
	flushed bool
}

func (stub *activeRepositoryStub) LoadQuestionSessionWithStateBySessionID(
	ctx context.Context,
	sessionID int,
) (*model.QuizActiveSessionState, error) {
	return nil, nil
}
func (stub *activeRepositoryStub) FlushQuizActiveSession(
	ctx context.Context,
	state *model.QuizActiveSessionState,
) error {
	stub.flushed = true
	return nil
}

func botWithActive(
	stateStores *testInteractionStores,
	repository *activeRepositoryStub,
	userRepository *streakRepositoryStub,
) (*SessionFlow, *mockBotAPI) {
	mockAPI := &mockBotAPI{}
	sessionFlow := newTestSessionFlow(
		mockAPI,
		stateStores,
		SessionFlowDependencies{
			Session: newTestSessionService(
				stateStores,
				service.SessionDependencies{
					QuizActiveSessionRepository: repository,
					UserRepository:              userRepository,
					LLM:                         &mockLLM{},
				},
			),
		},
	)
	return sessionFlow, mockAPI
}

func callbackWithMessage(
	data string,
	chatID int64,
	messageID int,
	userID int64,
) *tgbotapi.CallbackQuery {
	return &tgbotapi.CallbackQuery{
		Data: data,
		From: &tgbotapi.User{ID: userID},
		Message: &tgbotapi.Message{
			Chat:      &tgbotapi.Chat{ID: chatID},
			MessageID: messageID,
		},
	}
}

func collectText(messages []tgbotapi.Chattable) string {
	var builder strings.Builder
	for _, chattable := range messages {
		switch chattable := chattable.(type) {
		case tgbotapi.MessageConfig:
			builder.WriteString(chattable.Text)
			builder.WriteString("\n")
		case tgbotapi.EditMessageTextConfig:
			builder.WriteString(chattable.Text)
			builder.WriteString("\n")
		}
	}
	return builder.String()
}

func TestFinishSession_Summary(t *testing.T) {
	ctx := context.Background()
	stateStores := newTestInteractionStores()
	repository := &activeRepositoryStub{}
	userRepository := &streakRepositoryStub{}
	sessionFlow, mockAPI := botWithActive(
		stateStores,
		repository,
		userRepository,
	)

	sessionID := 10
	userID := int64(999)
	correct := true
	wrong := false
	state := &model.QuizActiveSessionState{
		Version: model.QuizActiveSessionStateVersion,
		Session: model.Session{ID: sessionID, UserID: userID},
		Items: []model.QuizActiveSessionQuestion{
			{
				SessionQuestion: model.SessionQuestion{QuestionID: 1, IsCorrect: &correct},
				Question:        model.Question{ID: 1, Type: model.QuestionMultipleChoice},
			},
			{
				SessionQuestion: model.SessionQuestion{
					QuestionID: 2,
					IsCorrect:  &wrong,
					UserAnswer: stringPointer("banana"),
				},
				Question: model.Question{
					ID:            2,
					Type:          model.QuestionMultipleChoice,
					Prompt:        "fruit?",
					CorrectAnswer: "apple",
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

	callbackQuery := callbackWithMessage(
		"session:10:finish",
		123,
		456,
		userID,
	)
	sessionFlow.finishSession(
		ctx,
		callbackQuery,
		sessionID,
	)

	if !repository.flushed {
		t.Error("expected FlushActiveSession to be called")
	}
	if !userRepository.updated {
		t.Error("expected UpdateStreak to be called")
	}
	text := collectText(mockAPI.sentMessages)
	if !strings.Contains(
		text,
		"세션 완료",
	) {
		t.Errorf(
			"expected completion summary, got %q",
			text,
		)
	}
	if !strings.Contains(
		text,
		"1/2",
	) {
		t.Errorf(
			"expected score 1/2 in summary, got %q",
			text,
		)
	}
	if !strings.Contains(
		text,
		"틀린 문제",
	) {
		t.Errorf(
			"expected wrong-answer section, got %q",
			text,
		)
	}
}

func TestHandleSessionCallback_Finish(t *testing.T) {
	ctx := context.Background()
	stateStores := newTestInteractionStores()
	repository := &activeRepositoryStub{}
	userRepository := &streakRepositoryStub{}
	sessionFlow, mockAPI := botWithActive(
		stateStores,
		repository,
		userRepository,
	)

	sessionID := 11
	userID := int64(7)
	correct := true
	state := &model.QuizActiveSessionState{
		Version: model.QuizActiveSessionStateVersion,
		Session: model.Session{ID: sessionID, UserID: userID},
		Items: []model.QuizActiveSessionQuestion{
			{
				SessionQuestion: model.SessionQuestion{QuestionID: 1, IsCorrect: &correct},
				Question:        model.Question{ID: 1, Type: model.QuestionMultipleChoice},
			},
		},
	}
	storeActiveState(
		t,
		stateStores,
		sessionID,
		state,
	)

	sessionFlow.HandleSessionCallback(
		ctx,
		callbackWithMessage(
			"session:11:finish",
			123,
			456,
			userID,
		),
	)

	if !repository.flushed {
		t.Error("expected finish action to flush the session")
	}
	if len(mockAPI.sentMessages) == 0 {
		t.Fatal("expected summary message")
	}
}

func TestHandleSessionCallback_BadData(t *testing.T) {
	ctx := context.Background()
	sessionFlow, _ := botWithActive(
		newTestInteractionStores(),
		&activeRepositoryStub{},
		&streakRepositoryStub{},
	)

	// fewer than 3 parts -> early return, must not panic
	sessionFlow.HandleSessionCallback(
		ctx,
		callbackWithMessage(
			"session:onlytwo",
			1,
			2,
			3,
		),
	)
	sessionFlow.HandleSessionCallback(
		ctx,
		callbackWithMessage(
			"session:abc:start",
			1,
			2,
			3,
		),
	)
}

func TestHandleAnswerCallback_OptionSelected(t *testing.T) {
	ctx := context.Background()
	stateStores := newTestInteractionStores()
	repository := &activeRepositoryStub{}
	sessionFlow, mockAPI := botWithActive(
		stateStores,
		repository,
		&streakRepositoryStub{},
	)

	sessionID := 20
	questionID := 5
	options, _ := json.Marshal([]string{"apple", "banana"})
	state := &model.QuizActiveSessionState{
		Version: model.QuizActiveSessionStateVersion,
		Session: model.Session{ID: sessionID, UserID: 1},
		Items: []model.QuizActiveSessionQuestion{
			{
				SessionQuestion: model.SessionQuestion{QuestionID: questionID},
				Question: model.Question{
					ID: questionID, Type: model.QuestionMultipleChoice,
					Options: options, CorrectAnswer: "apple", Explanation: "fruit",
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

	// q:{session}:{question}:{optionIndex} -> select option 0 ("apple", correct)
	data := "q:20:5:0"
	sessionFlow.HandleAnswerCallback(
		ctx,
		callbackWithMessage(
			data,
			123,
			456,
			1,
		),
	)

	text := collectText(mockAPI.sentMessages)
	if !strings.Contains(
		text,
		"정답",
	) {
		t.Errorf(
			"expected correct-answer feedback, got %q",
			text,
		)
	}
}

func TestHandleAnswerCallback_NextBeforeAnswering(t *testing.T) {
	ctx := context.Background()
	stateStores := newTestInteractionStores()
	sessionFlow, mockAPI := botWithActive(
		stateStores,
		&activeRepositoryStub{},
		&streakRepositoryStub{},
	)

	sessionID := 21
	state := &model.QuizActiveSessionState{
		Version: model.QuizActiveSessionStateVersion,
		Session: model.Session{ID: sessionID, UserID: 1},
		Items: []model.QuizActiveSessionQuestion{
			{
				SessionQuestion: model.SessionQuestion{QuestionID: 1}, // unanswered
				Question:        model.Question{ID: 1, Type: model.QuestionKanaHandwriting},
			},
		},
	}
	storeActiveState(
		t,
		stateStores,
		sessionID,
		state,
	)

	// q:{session}:next:{idx} with current question unanswered -> prompt to submit first
	sessionFlow.HandleAnswerCallback(
		ctx,
		callbackWithMessage(
			"q:21:next:0",
			123,
			456,
			1,
		),
	)

	text := collectText(mockAPI.sentMessages)
	if !strings.Contains(
		text,
		"손글씨",
	) {
		t.Errorf(
			"expected handwriting submit prompt, got %q",
			text,
		)
	}
}

func TestHandleAnswerCallback_BadData(t *testing.T) {
	ctx := context.Background()
	sessionFlow, _ := botWithActive(
		newTestInteractionStores(),
		&activeRepositoryStub{},
		&streakRepositoryStub{},
	)

	// fewer than 4 parts -> early return, no panic
	sessionFlow.HandleAnswerCallback(
		ctx,
		callbackWithMessage(
			"q:20:5",
			1,
			2,
			3,
		),
	)
	sessionFlow.HandleAnswerCallback(
		ctx,
		callbackWithMessage(
			"q:x:5:0",
			1,
			2,
			3,
		),
	)
}

func TestShowQuestion_MultipleChoiceKeyboard(t *testing.T) {
	ctx := context.Background()
	stateStores := newTestInteractionStores()
	sessionFlow, mockAPI := botWithActive(
		stateStores,
		&activeRepositoryStub{},
		&streakRepositoryStub{},
	)

	sessionID := 30
	options, _ := json.Marshal([]string{"A", "B", "C", "D"})
	state := &model.QuizActiveSessionState{
		Version: model.QuizActiveSessionStateVersion,
		Session: model.Session{ID: sessionID, UserID: 1},
		Items: []model.QuizActiveSessionQuestion{
			{
				SessionQuestion: model.SessionQuestion{QuestionID: 1},
				Question: model.Question{
					ID:      1,
					Type:    model.QuestionMultipleChoice,
					Prompt:  "Pick one",
					Options: options,
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

	sessionFlow.showQuestion(
		ctx,
		123,
		nil,
		sessionID,
		0,
	)

	if len(mockAPI.sentMessages) == 0 {
		t.Fatal("expected a question message")
	}
	message, ok := mockAPI.sentMessages[0].(tgbotapi.MessageConfig)
	if !ok {
		t.Fatalf(
			"expected MessageConfig, got %T",
			mockAPI.sentMessages[0],
		)
	}
	if !strings.Contains(
		message.Text,
		"Pick one",
	) {
		t.Errorf(
			"expected prompt text, got %q",
			message.Text,
		)
	}
	keyboard, ok := message.ReplyMarkup.(tgbotapi.InlineKeyboardMarkup)
	if !ok {
		t.Fatalf(
			"expected inline keyboard, got %T",
			message.ReplyMarkup,
		)
	}
	if len(keyboard.InlineKeyboard) != 2 {
		t.Errorf(
			"expected 2 option rows, got %d",
			len(keyboard.InlineKeyboard),
		)
	}
}

func TestShowQuestion_AllAnsweredShowsFinish(t *testing.T) {
	ctx := context.Background()
	stateStores := newTestInteractionStores()
	sessionFlow, mockAPI := botWithActive(
		stateStores,
		&activeRepositoryStub{},
		&streakRepositoryStub{},
	)

	sessionID := 31
	state := &model.QuizActiveSessionState{
		Version: model.QuizActiveSessionStateVersion,
		Session: model.Session{ID: sessionID, UserID: 1},
		Items: []model.QuizActiveSessionQuestion{
			{SessionQuestion: model.SessionQuestion{QuestionID: 1}, Question: model.Question{ID: 1}},
		},
	}
	storeActiveState(
		t,
		stateStores,
		sessionID,
		state,
	)

	// questionIndex beyond items -> finish button branch
	sessionFlow.showQuestion(
		ctx,
		123,
		nil,
		sessionID,
		5,
	)

	text := collectText(mockAPI.sentMessages)
	if !strings.Contains(
		text,
		"모든 문제를 풀었습니다",
	) {
		t.Errorf(
			"expected all-answered message, got %q",
			text,
		)
	}
}

func TestProcessAnswerText_Wrong(t *testing.T) {
	stateStores := newTestInteractionStores()
	sessionFlow, mockAPI := botWithActive(
		stateStores,
		&activeRepositoryStub{},
		&streakRepositoryStub{},
	)

	sessionID, questionID := 40, 1
	state := &model.QuizActiveSessionState{
		Version: model.QuizActiveSessionStateVersion,
		Session: model.Session{ID: sessionID, UserID: 1},
		Items: []model.QuizActiveSessionQuestion{
			{
				SessionQuestion: model.SessionQuestion{QuestionID: questionID},
				Question: model.Question{
					ID:            questionID,
					Type:          model.QuestionMultipleChoice,
					CorrectAnswer: "apple",
					Explanation:   "fruit",
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

	answerByText(
		t,
		sessionFlow,
		stateStores,
		123,
		&tgbotapi.User{ID: 1},
		sessionID,
		0,
		"banana",
	)

	text := collectText(mockAPI.sentMessages)
	if !strings.Contains(
		text,
		"오답",
	) {
		t.Errorf(
			"expected wrong-answer feedback, got %q",
			text,
		)
	}
	if !strings.Contains(
		text,
		"apple",
	) {
		t.Errorf(
			"expected correct answer shown, got %q",
			text,
		)
	}
}

type analyzerUserRepositoryStub struct{ streak int }

func (stub *analyzerUserRepositoryStub) GetByID(
	ctx context.Context,
	id int64,
) (*model.User, error) {
	return &model.User{ID: id, StreakDays: stub.streak, Language: "ja", ProficiencyLevel: "N5"}, nil
}

type statsRepositoryStub struct{}

func (stub *statsRepositoryStub) GetTodayStats(
	ctx context.Context,
	userID int64,
) (int, int, error) {
	return 10, 7, nil
}
func (stub *statsRepositoryStub) GetCategoryAccuracy(
	ctx context.Context,
	userID int64,
) (map[string]float64, error) {
	return map[string]float64{"vocabulary": 80, "grammar": 60}, nil
}

type materialProgressRepositoryStub struct{}

func (stub *materialProgressRepositoryStub) GetLevelProgress(
	ctx context.Context,
	userID int64,
	language string,
) ([]model.LevelProgress, error) {
	return []model.LevelProgress{
		{Level: "N4", TotalMaterials: 200, StudiedMaterials: 10},
		{Level: "N5", TotalMaterials: 100, StudiedMaterials: 42},
	}, nil
}

func botWithAnalyzer() (*Bot, *mockBotAPI) {
	mockAPI := &mockBotAPI{}
	analyzer := service.NewAnalyzerService(
		&analyzerUserRepositoryStub{streak: 5},
		&statsRepositoryStub{},
		&materialProgressRepositoryStub{},
	)
	bot := newTestBot(
		mockAPI,
		nil,
		&testServices{Analyzer: analyzer},
	)
	return bot, mockAPI
}

func TestHandleStats(t *testing.T) {
	bot, mockAPI := botWithAnalyzer()
	message := &tgbotapi.Message{Chat: &tgbotapi.Chat{ID: 1}, From: &tgbotapi.User{ID: 2}}

	bot.handleStats(
		context.Background(),
		message,
	)

	text := collectText(mockAPI.sentMessages)
	if !strings.Contains(
		text,
		"학습 통계",
	) {
		t.Errorf(
			"expected stats text, got %q",
			text,
		)
	}
	if !strings.Contains(
		text,
		"5일",
	) {
		t.Errorf(
			"expected streak in stats, got %q",
			text,
		)
	}
}

func TestHandleStreak(t *testing.T) {
	bot, mockAPI := botWithAnalyzer()
	message := &tgbotapi.Message{Chat: &tgbotapi.Chat{ID: 1}, From: &tgbotapi.User{ID: 2}}

	bot.handleStreak(
		context.Background(),
		message,
	)

	text := collectText(mockAPI.sentMessages)
	if !strings.Contains(
		text,
		"스트릭",
	) {
		t.Errorf(
			"expected streak text, got %q",
			text,
		)
	}
}

func TestHandleStatsCallback(t *testing.T) {
	bot, mockAPI := botWithAnalyzer()
	callbackQuery := callbackWithMessage(
		"menu:stats",
		1,
		2,
		3,
	)

	bot.handleStatsCallback(
		context.Background(),
		callbackQuery,
	)

	if len(mockAPI.sentMessages) == 0 {
		t.Fatal("expected stats edit message")
	}
	edit, ok := mockAPI.sentMessages[0].(tgbotapi.EditMessageTextConfig)
	if !ok {
		t.Fatalf(
			"expected EditMessageTextConfig, got %T",
			mockAPI.sentMessages[0],
		)
	}
	if !strings.Contains(
		edit.Text,
		"학습 통계",
	) {
		t.Errorf(
			"expected stats text, got %q",
			edit.Text,
		)
	}
	assertStatsTabs(
		t,
		edit,
		callbackMenuStats,
	)
}

func TestHandleStatsProgressCallback(t *testing.T) {
	bot, mockAPI := botWithAnalyzer()

	bot.handleCallback(
		context.Background(),
		callbackWithMessage(
			callbackStatsProgress,
			1,
			2,
			3,
		),
	)

	var edit tgbotapi.EditMessageTextConfig
	found := false
	for _, sent := range mockAPI.sentMessages {
		if candidate, ok := sent.(tgbotapi.EditMessageTextConfig); ok {
			edit = candidate
			found = true
		}
	}
	if !found {
		t.Fatal("expected progress edit message")
	}
	n5Line := "<b>N5</b>  ▓▓▓▓░░░░░░ 42% (42/100)"
	n4Line := "<b>N4</b>  ░░░░░░░░░░ 5% (10/200)"
	n5Index := strings.Index(
		edit.Text,
		n5Line,
	)
	n4Index := strings.Index(
		edit.Text,
		n4Line,
	)
	if n5Index < 0 || n4Index < 0 || n5Index > n4Index {
		t.Fatalf(
			"progress text = %q, want N5 line then N4 line",
			edit.Text,
		)
	}
	assertStatsTabs(
		t,
		edit,
		callbackStatsProgress,
	)
}

func assertStatsTabs(
	t *testing.T,
	edit tgbotapi.EditMessageTextConfig,
	activeCallback string,
) {
	t.Helper()
	if edit.ReplyMarkup == nil || len(edit.ReplyMarkup.InlineKeyboard) == 0 {
		t.Fatal("expected stats tab keyboard")
	}
	tabs := edit.ReplyMarkup.InlineKeyboard[0]
	if len(tabs) != 2 {
		t.Fatalf(
			"tab count = %d, want 2",
			len(tabs),
		)
	}
	for _, tab := range tabs {
		active := strings.HasSuffix(
			tab.Text,
			"✓",
		)
		if active != (*tab.CallbackData == activeCallback) {
			t.Errorf(
				"tab %q (%s) active=%v, want active only for %s",
				tab.Text,
				*tab.CallbackData,
				active,
				activeCallback,
			)
		}
	}
}

func TestHandleMenu(t *testing.T) {
	mockAPI := &mockBotAPI{}
	userService := service.NewUserService(&mockUserRepository{
		getOrCreateFunc: func(
			ctx context.Context,
			id int64,
			username string,
		) (*model.User, error) {
			return &model.User{ID: id, StreakDays: 3, Language: "ja", ProficiencyLevel: "N5"}, nil
		},
	})
	bot := newTestBot(
		mockAPI,
		nil,
		&testServices{
			User: userService,
			Session: newTestSessionService(
				nil,
				service.SessionDependencies{QuestionRepository: &mockSRSRepository{}},
			),
		},
	)

	message := &tgbotapi.Message{Chat: &tgbotapi.Chat{ID: 1}, From: &tgbotapi.User{ID: 2}}
	bot.handleMenu(
		context.Background(),
		message,
	)

	if len(mockAPI.sentMessages) == 0 {
		t.Fatal("expected menu message")
	}
	sent := mockAPI.sentMessages[0].(tgbotapi.MessageConfig)
	if !strings.Contains(
		sent.Text,
		"CopyLingo",
	) {
		t.Errorf(
			"expected menu text, got %q",
			sent.Text,
		)
	}
}

func stringPointer(value string) *string { return &value }

func TestTelegramClientEditMessageReplyMarkup(t *testing.T) {
	mockAPI := &mockBotAPI{}
	client := newTelegramClient(mockAPI)

	markup := tgbotapi.NewInlineKeyboardMarkup(
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData(
				"x",
				"data",
			),
		),
	)
	if err := client.EditMessageReplyMarkup(
		123,
		456,
		markup,
	); err != nil {
		t.Fatalf(
			"EditMessageReplyMarkup error = %v",
			err,
		)
	}
	if len(mockAPI.sentMessages) != 1 {
		t.Fatalf(
			"expected 1 sent, got %d",
			len(mockAPI.sentMessages),
		)
	}
	if _, ok := mockAPI.sentMessages[0].(tgbotapi.EditMessageReplyMarkupConfig); !ok {
		t.Fatalf(
			"expected EditMessageReplyMarkupConfig, got %T",
			mockAPI.sentMessages[0],
		)
	}
}
