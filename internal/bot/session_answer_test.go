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

func TestHandleTextInput(t *testing.T) {
	ctx := context.Background()
	rdb := &testRedis{values: map[string]string{}}
	mSRS := &mockSRS{}
	active := service.NewActiveSessionService(nil, rdb, mSRS)
	mLLM := &mockLLM{}
	grader := service.NewGraderService(nil, active, mLLM)

	mAPI := &mockBotAPI{}
	b := &Bot{
		api: mAPI,
		rdb: rdb,
		services: &service.Services{
			ActiveSession: active,
			Grader:        grader,
		},
	}
	sf := NewSessionFlow(b)

	chatID := int64(123)
	msg := &tgbotapi.Message{
		Chat: &tgbotapi.Chat{ID: chatID},
		Text: "apple",
	}

	t.Run("no active question state", func(t *testing.T) {
		if sf.HandleTextInput(ctx, msg) {
			t.Error("expected HandleTextInput to return false")
		}
	})

	t.Run("active question state exists", func(t *testing.T) {
		sessionID := 10
		rdb.values[config.UserActiveQuestionRedisKey.Format(chatID)] = "10:0"

		state := &model.ActiveSessionState{
			Version: model.ActiveSessionStateVersion,
			Session: model.Session{ID: sessionID},
			Items: []model.ActiveSessionQuestion{
				{
					SessionQuestion: model.SessionQuestion{QuestionID: 1},
					Question:        model.Question{ID: 1, CorrectAnswer: "apple", Type: model.QuestionMultipleChoice},
				},
			},
		}
		raw, _ := json.Marshal(state)
		rdb.values[config.ActiveSessionWorkingSetRedisKey.Format(sessionID)] = string(raw)

		if !sf.HandleTextInput(ctx, msg) {
			t.Error("expected HandleTextInput to return true")
		}

		// Verify Redis state cleared
		if _, ok := rdb.values[config.UserActiveQuestionRedisKey.Format(chatID)]; ok {
			t.Error("expected active question key to be deleted")
		}

		// Verify message sent
		if len(mAPI.sentMessages) == 0 {
			t.Fatal("expected message sent")
		}
	})
}

func TestProcessAnswerText_Correct(t *testing.T) {
	ctx := context.Background()
	rdb := &testRedis{values: map[string]string{}}
	mSRS := &mockSRS{}
	active := service.NewActiveSessionService(nil, rdb, mSRS)
	mLLM := &mockLLM{}
	grader := service.NewGraderService(nil, active, mLLM)
	mAPI := &mockBotAPI{}
	b := &Bot{
		api: mAPI,
		rdb: rdb,
		services: &service.Services{
			ActiveSession: active,
			Grader:        grader,
		},
	}
	sf := NewSessionFlow(b)

	sessionID := 10
	questionID := 1
	state := &model.ActiveSessionState{
		Version: model.ActiveSessionStateVersion,
		Session: model.Session{ID: sessionID},
		Items: []model.ActiveSessionQuestion{
			{
				SessionQuestion: model.SessionQuestion{QuestionID: questionID},
				Question: model.Question{
					ID:            questionID,
					CorrectAnswer: "apple",
					Explanation:   "It's a fruit",
					Type:          model.QuestionMultipleChoice,
				},
			},
		},
	}
	raw, _ := json.Marshal(state)
	rdb.values[config.ActiveSessionWorkingSetRedisKey.Format(sessionID)] = string(raw)

	sf.processAnswerText(ctx, 123, nil, sessionID, questionID, "apple", nil)

	if len(mAPI.sentMessages) != 1 {
		t.Fatalf("expected 1 message, got %d", len(mAPI.sentMessages))
	}
	msg := mAPI.sentMessages[0].(tgbotapi.MessageConfig)
	if !strings.Contains(msg.Text, "정답!") {
		t.Errorf("wrong text: %s", msg.Text)
	}
}

func TestProcessAnswerText_AlreadyAnsweredRedirectsToResult(t *testing.T) {
	ctx := context.Background()
	rdb := &testRedis{values: map[string]string{}}
	active := service.NewActiveSessionService(nil, rdb, nil)
	mAPI := &mockBotAPI{}
	b := &Bot{
		api: mAPI,
		rdb: rdb,
		services: &service.Services{
			ActiveSession: active,
		},
	}
	sf := NewSessionFlow(b)

	sessionID := 10
	questionID := 1
	trueVal := true
	state := &model.ActiveSessionState{
		Version: model.ActiveSessionStateVersion,
		Session: model.Session{ID: sessionID},
		Items: []model.ActiveSessionQuestion{
			{
				SessionQuestion: model.SessionQuestion{QuestionID: questionID, IsCorrect: &trueVal},
				Question:        model.Question{ID: questionID},
			},
		},
	}
	raw, _ := json.Marshal(state)
	rdb.values[config.ActiveSessionWorkingSetRedisKey.Format(sessionID)] = string(raw)

	sf.processAnswerText(ctx, 123, nil, sessionID, questionID, "apple", nil)

	msg := mAPI.sentMessages[0].(tgbotapi.MessageConfig)
	if strings.Contains(msg.Text, "이미 답변한 문제입니다") {
		t.Fatalf("stale answer should not return already-answered error: %s", msg.Text)
	}
	if !strings.Contains(msg.Text, "모든 문제를 풀었습니다") {
		t.Errorf("expected result redirect, got: %s", msg.Text)
	}
}

func TestProcessAnswer_AlreadyAnsweredRedirectsToNextQuestion(t *testing.T) {
	ctx := context.Background()
	rdb := &testRedis{values: map[string]string{}}
	active := service.NewActiveSessionService(nil, rdb, nil)
	mAPI := &mockBotAPI{}
	b := &Bot{
		api: mAPI,
		rdb: rdb,
		cfg: &config.Config{},
		services: &service.Services{
			ActiveSession: active,
		},
	}
	sf := NewSessionFlow(b)

	sessionID := 11
	firstAnswered := true
	state := &model.ActiveSessionState{
		Version: model.ActiveSessionStateVersion,
		Session: model.Session{ID: sessionID},
		Items: []model.ActiveSessionQuestion{
			{
				SessionQuestion: model.SessionQuestion{QuestionID: 1, IsCorrect: &firstAnswered},
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
	raw, _ := json.Marshal(state)
	rdb.values[config.ActiveSessionWorkingSetRedisKey.Format(sessionID)] = string(raw)

	sf.processAnswer(ctx, cbWithMessage("q:11:1:0", 123, 456, 123), sessionID, 1, 0)

	if len(mAPI.sentMessages) != 1 {
		t.Fatalf("expected one redirected message, got %d", len(mAPI.sentMessages))
	}
	msg, ok := mAPI.sentMessages[0].(tgbotapi.EditMessageTextConfig)
	if !ok {
		t.Fatalf("expected edited message, got %T", mAPI.sentMessages[0])
	}
	if strings.Contains(msg.Text, "이미 답변한 문제입니다") || !strings.Contains(msg.Text, "두 번째 문제") {
		t.Fatalf("expected redirect to next question, got %q", msg.Text)
	}
}
