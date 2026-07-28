package bot

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"testing"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"

	"github.com/lsj/copylingo/internal/config"
	"github.com/lsj/copylingo/internal/model"
	"github.com/lsj/copylingo/internal/service"
)

// storeQuizState stores a single-item active session working set in the test redis.
func storeQuizState(
	t *testing.T,
	rdb *testRedis,
	sessionID, questionID int,
	q model.Question,
	sq model.SessionQuestion,
) {
	t.Helper()
	state := &model.ActiveSessionState{
		Version: model.ActiveSessionStateVersion,
		Session: model.Session{ID: sessionID},
		Items: []model.ActiveSessionQuestion{
			{SessionQuestion: sq, Question: q},
		},
	}
	raw, _ := json.Marshal(state)
	rdb.values[config.ActiveSessionWorkingSetRedisKey.Format(sessionID)] = string(raw)
}

func storeStudyState(
	t *testing.T,
	rdb *testRedis,
	state *model.StudyActiveSessionState,
) {
	t.Helper()
	raw, err := json.Marshal(state)
	if err != nil {
		t.Fatalf("marshal study state: %v", err)
	}
	rdb.values[config.StudySessionWorkingSetRedisKey.Format(state.Session.ID)] = string(raw)
}

// keyboardHasCallback reports whether any inline button in a sent message carries the callback data.
func keyboardHasCallback(c tgbotapi.Chattable, data string) bool {
	msg, ok := c.(tgbotapi.MessageConfig)
	if !ok {
		return false
	}
	markup, ok := msg.ReplyMarkup.(tgbotapi.InlineKeyboardMarkup)
	if !ok {
		return false
	}
	for _, row := range markup.InlineKeyboard {
		for _, btn := range row {
			if btn.CallbackData != nil && *btn.CallbackData == data {
				return true
			}
		}
	}
	return false
}

func TestLoadQuizQuestionContext(t *testing.T) {
	ctx := context.Background()
	rdb := &testRedis{values: map[string]string{}}
	active := service.NewActiveSessionService(nil, rdb, &mockSRS{})
	b := &Bot{rdb: rdb, services: &service.Services{ActiveSession: active}}

	sessionID, questionID := 10, 1
	userAnswer := "が"
	storeQuizState(t, rdb, sessionID, questionID,
		model.Question{
			ID:            questionID,
			Prompt:        "わたし___がくせいです。",
			CorrectAnswer: "は",
			Explanation:   "主題を示す助詞は「は」。",
		},
		model.SessionQuestion{QuestionID: questionID, UserAnswer: &userAnswer},
	)

	t.Run("plain /llm token yields no context", func(t *testing.T) {
		if got := b.loadQuizQuestionContext(ctx, "1"); got != "" {
			t.Errorf("expected empty context for plain token, got %q", got)
		}
	})

	t.Run("malformed token yields no context", func(t *testing.T) {
		for _, tok := range []string{"", "q:abc:1", "q:10", "x:10:1"} {
			if got := b.loadQuizQuestionContext(ctx, tok); got != "" {
				t.Errorf("expected empty context for %q, got %q", tok, got)
			}
		}
	})

	t.Run("missing session yields no context", func(t *testing.T) {
		if got := b.loadQuizQuestionContext(ctx, "q:999:1"); got != "" {
			t.Errorf("expected empty context for missing session, got %q", got)
		}
	})

	t.Run("valid token includes question fields", func(t *testing.T) {
		got := b.loadQuizQuestionContext(ctx, "q:10:1")
		for _, want := range []string{"わたし___がくせいです。", "は", "主題を示す助詞", userAnswer} {
			if !strings.Contains(got, want) {
				t.Errorf("context missing %q, got %q", want, got)
			}
		}
	})
}

func TestLoadStudyMaterialContext(t *testing.T) {
	ctx := context.Background()
	owner := config.LLMAllowedTelegramUserIDs[0]
	sessionID := 20
	state := &model.StudyActiveSessionState{
		Version: model.StudyActiveSessionStateVersion,
		Session: model.Session{
			ID:     sessionID,
			UserID: owner,
			Mode:   model.SessionModeStudy,
			Status: model.SessionInProgress,
		},
		Items: []model.StudySessionMaterial{
			studyItem(sessionID, 40, 0, "水", vocabularyPayload("みず", "水", "물", "noun")),
		},
	}

	newBot := func() *Bot {
		rdb := &testRedis{values: map[string]string{}}
		storeStudyState(t, rdb, state)
		return &Bot{
			rdb: rdb,
			services: &service.Services{
				StudyActiveSession: service.NewStudyActiveSessionService(nil, nil, rdb),
			},
		}
	}

	t.Run("valid token includes rendered material and requires ownership", func(t *testing.T) {
		b := newBot()
		got := b.loadStudyMaterialContext(ctx, "study:20:0", owner)
		for _, want := range []string{"Study Material", "Vocabulary", "水", "읽기: みず", "의미: 물"} {
			if !strings.Contains(got, want) {
				t.Errorf("context missing %q: %q", want, got)
			}
		}
		if got := b.loadStudyMaterialContext(ctx, "study:20:0", owner+1); got != "" {
			t.Errorf("foreign user context = %q, want empty", got)
		}
	})

	t.Run("malformed or unknown material token yields no context", func(t *testing.T) {
		b := newBot()
		for _, token := range []string{"", "study:abc:0", "study:20", "study:20:1", "q:20:0"} {
			if got := b.loadStudyMaterialContext(ctx, token, owner); got != "" {
				t.Errorf("context for %q = %q, want empty", token, got)
			}
		}
	})
}

func TestProcessAnswerText_AskButtonOwnerGate(t *testing.T) {
	owner := &tgbotapi.User{ID: config.LLMAllowedTelegramUserIDs[0]}
	sessionID, questionID := 10, 1

	newFlow := func(rdb *testRedis, mAPI *mockBotAPI) *SessionFlow {
		active := service.NewActiveSessionService(nil, rdb, &mockSRS{})
		grader := service.NewGraderService(nil, active, &mockLLM{})
		b := &Bot{api: mAPI, rdb: rdb, services: &service.Services{ActiveSession: active, Grader: grader}}
		return NewSessionFlow(b)
	}

	mcQuestion := model.Question{
		ID:            questionID,
		Prompt:        "りんごは？",
		CorrectAnswer: "apple",
		Explanation:   "りんご = apple",
		Type:          model.QuestionMultipleChoice,
	}
	askData := fmt.Sprintf(config.FormatQuestionAskLLM, sessionID, questionID)

	t.Run("owner sees ask button and prompt", func(t *testing.T) {
		rdb := &testRedis{values: map[string]string{}}
		mAPI := &mockBotAPI{}
		sf := newFlow(rdb, mAPI)
		storeQuizState(t, rdb, sessionID, questionID, mcQuestion, model.SessionQuestion{QuestionID: questionID})

		sf.processAnswerText(context.Background(), 123, owner, sessionID, questionID, "apple", nil)

		if len(mAPI.sentMessages) != 1 {
			t.Fatalf("expected 1 message, got %d", len(mAPI.sentMessages))
		}
		msg := mAPI.sentMessages[0].(tgbotapi.MessageConfig)
		if !strings.Contains(msg.Text, "りんごは？") {
			t.Errorf("expected prompt preserved in result, got %q", msg.Text)
		}
		if !keyboardHasCallback(mAPI.sentMessages[0], askData) {
			t.Errorf("expected ask button %q for owner", askData)
		}
	})

	t.Run("non-owner sees no ask button", func(t *testing.T) {
		rdb := &testRedis{values: map[string]string{}}
		mAPI := &mockBotAPI{}
		sf := newFlow(rdb, mAPI)
		storeQuizState(t, rdb, sessionID, questionID, mcQuestion, model.SessionQuestion{QuestionID: questionID})

		sf.processAnswerText(context.Background(), 123, nil, sessionID, questionID, "apple", nil)

		if keyboardHasCallback(mAPI.sentMessages[0], askData) {
			t.Error("did not expect ask button for non-owner")
		}
	})
}

func TestHandleAskLLMQuestion(t *testing.T) {
	sessionID, questionID := 10, 1
	owner := &tgbotapi.User{ID: config.LLMAllowedTelegramUserIDs[0]}
	pendingKey := config.UserLLMPendingRedisKey.Format(owner.ID)

	newFlow := func(rdb *testRedis, mAPI *mockBotAPI) *SessionFlow {
		b := &Bot{api: mAPI, rdb: rdb, services: &service.Services{}}
		return NewSessionFlow(b)
	}
	cbFor := func(from *tgbotapi.User) *tgbotapi.CallbackQuery {
		return &tgbotapi.CallbackQuery{From: from, Message: &tgbotapi.Message{Chat: &tgbotapi.Chat{ID: 123}}}
	}

	t.Run("owner arms context-scoped pending value", func(t *testing.T) {
		rdb := &testRedis{values: map[string]string{}}
		mAPI := &mockBotAPI{}
		sf := newFlow(rdb, mAPI)

		sf.handleAskLLMQuestion(context.Background(), cbFor(owner), sessionID, "1")

		want := fmt.Sprintf("q:%d:%d", sessionID, questionID)
		if got := rdb.values[pendingKey]; got != want {
			t.Errorf("pending value = %q, want %q", got, want)
		}
		if len(mAPI.sentMessages) != 1 {
			t.Fatalf("expected 1 instruction message, got %d", len(mAPI.sentMessages))
		}
	})

	t.Run("non-owner is ignored", func(t *testing.T) {
		rdb := &testRedis{values: map[string]string{}}
		mAPI := &mockBotAPI{}
		sf := newFlow(rdb, mAPI)

		sf.handleAskLLMQuestion(context.Background(), cbFor(&tgbotapi.User{ID: 999}), sessionID, "1")

		if _, ok := rdb.values[config.UserLLMPendingRedisKey.Format(999)]; ok {
			t.Error("expected no pending value for non-owner")
		}
		if len(mAPI.sentMessages) != 0 {
			t.Errorf("expected no message for non-owner, got %d", len(mAPI.sentMessages))
		}
	})
}

func TestHandleStudyAskLLMQuestion(t *testing.T) {
	ctx := context.Background()
	owner := &tgbotapi.User{ID: config.LLMAllowedTelegramUserIDs[0]}
	sessionID := 30

	newFlow := func() (*StudyFlow, *testRedis, *mockBotAPI) {
		rdb := &testRedis{values: map[string]string{}}
		storeStudyState(t, rdb, &model.StudyActiveSessionState{
			Version: model.StudyActiveSessionStateVersion,
			Session: model.Session{
				ID:     sessionID,
				UserID: owner.ID,
				Mode:   model.SessionModeStudy,
				Status: model.SessionInProgress,
			},
			Items: []model.StudySessionMaterial{
				studyItem(sessionID, 50, 0, "山", vocabularyPayload("やま", "山", "산", "noun")),
			},
		})
		api := &mockBotAPI{}
		b := &Bot{
			api: api,
			rdb: rdb,
			services: &service.Services{
				StudyActiveSession: service.NewStudyActiveSessionService(nil, nil, rdb),
			},
		}
		return NewStudyFlow(b), rdb, api
	}
	callback := func(from *tgbotapi.User, order int) *tgbotapi.CallbackQuery {
		return &tgbotapi.CallbackQuery{
			Data: fmt.Sprintf(config.FormatStudyAskLLM, sessionID, order),
			From: from,
			Message: &tgbotapi.Message{
				Chat: &tgbotapi.Chat{ID: 123},
			},
		}
	}

	t.Run("owner arms a material-scoped token", func(t *testing.T) {
		flow, rdb, api := newFlow()
		flow.HandleCallback(ctx, callback(owner, 0))

		key := config.UserLLMPendingRedisKey.Format(owner.ID)
		if got, want := rdb.values[key], "study:30:0"; got != want {
			t.Errorf("pending value = %q, want %q", got, want)
		}
		if len(api.sentMessages) != 1 {
			t.Fatalf("instruction messages = %d, want 1", len(api.sentMessages))
		}
	})

	t.Run("unknown material is rejected", func(t *testing.T) {
		flow, rdb, api := newFlow()
		flow.HandleCallback(ctx, callback(owner, 1))

		if _, ok := rdb.values[config.UserLLMPendingRedisKey.Format(owner.ID)]; ok {
			t.Error("unexpected pending token for unknown material")
		}
		if len(api.sentMessages) != 1 {
			t.Fatalf("messages = %d, want rejection message", len(api.sentMessages))
		}
	})

	t.Run("non-owner callback is ignored", func(t *testing.T) {
		flow, rdb, api := newFlow()
		flow.HandleCallback(ctx, callback(&tgbotapi.User{ID: 999}, 0))

		if _, ok := rdb.values[config.UserLLMPendingRedisKey.Format(999)]; ok {
			t.Error("unexpected pending token for non-owner")
		}
		if len(api.sentMessages) != 0 {
			t.Errorf("messages = %d, want 0", len(api.sentMessages))
		}
	})
}
