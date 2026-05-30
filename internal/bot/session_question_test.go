package bot

import (
	"context"
	"encoding/json"
	"net/url"
	"strings"
	"testing"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
	"github.com/lsj/copylingo/internal/config"
	"github.com/lsj/copylingo/internal/model"
	"github.com/lsj/copylingo/internal/service"
)

func TestIsStaleMiniAppCallbackWrapper(t *testing.T) {
	t.Parallel()
	// This is a simple wrapper, we just check it doesn't crash and delegates correctly
	if !isStaleMiniAppCallback([]string{"q", "1"}, "http://new.com") {
		t.Error("expected legacy callback to be stale")
	}
}

func TestHandwritingMiniAppURL(t *testing.T) {
	t.Parallel()
	b := &Bot{cfg: &config.Config{}}
	sf := NewSessionFlow(b)

	t.Run("empty base url", func(t *testing.T) {
		b.cfg.Server.PublicBaseURL = ""
		_, err := sf.handwritingMiniAppURL(1, 1, "jp", "n5", "prompt")
		if err == nil {
			t.Error("expected error for empty base URL")
		}
	})

	t.Run("valid url", func(t *testing.T) {
		b.cfg.Server.PublicBaseURL = "https://api.example.com/"
		prompt := "뜻 <b>'학교'</b>에 해당하는 일본어 단어를 손글씨로 쓰세요"
		got, err := sf.handwritingMiniAppURL(123, 456, "jp", "n5", prompt)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if !strings.Contains(got, "session_id=123") {
			t.Errorf("missing session_id in %s", got)
		}
		if !strings.Contains(got, "question_id=456") {
			t.Errorf("missing question_id in %s", got)
		}
		u, err := url.Parse(got)
		if err != nil {
			t.Fatalf("parse url: %v", err)
		}
		if gotPrompt := u.Query().Get("prompt"); gotPrompt != prompt {
			t.Errorf("prompt = %q, want %q", gotPrompt, prompt)
		}
	})
}

func TestQuestionNavigation(t *testing.T) {
	ctx := context.Background()
	rdb := &testRedis{values: map[string]string{}}
	active := service.NewActiveSessionService(nil, rdb, nil)
	sf := NewSessionFlow(&Bot{services: &service.Services{ActiveSession: active}})

	trueVal := true
	state := &model.ActiveSessionState{
		Version: model.ActiveSessionStateVersion,
		Session: model.Session{ID: 10},
		Items: []model.ActiveSessionQuestion{
			{SessionQuestion: model.SessionQuestion{QuestionID: 1, IsCorrect: &trueVal}},
			{SessionQuestion: model.SessionQuestion{QuestionID: 2}},
			{SessionQuestion: model.SessionQuestion{QuestionID: 3}},
		},
	}
	raw, _ := json.Marshal(state)
	rdb.values[config.ActiveSessionWorkingSetRedisKey.Format(10)] = string(raw)

	t.Run("isQuestionAnswered", func(t *testing.T) {
		if !sf.isQuestionAnswered(ctx, 10, 0) {
			t.Error("expected index 0 to be answered")
		}
		if sf.isQuestionAnswered(ctx, 10, 1) {
			t.Error("expected index 1 to be unanswered")
		}
		if sf.isQuestionAnswered(ctx, 10, 5) {
			t.Error("expected out of bounds to be false")
		}
	})

	t.Run("nextUnansweredQuestionIndex", func(t *testing.T) {
		idx, err := sf.nextUnansweredQuestionIndex(ctx, 10)
		if err != nil {
			t.Fatalf("failed: %v", err)
		}
		if idx != 1 {
			t.Errorf("expected idx 1, got %d", idx)
		}
	})
}

func TestRenderByType(t *testing.T) {
	ctx := context.Background()
	mAPI := &mockBotAPI{}
	rdb := &testRedis{values: map[string]string{}}
	b := &Bot{
		api: mAPI,
		rdb: rdb,
		cfg: &config.Config{Server: config.ServerConfig{PublicBaseURL: "https://ex.com"}},
	}
	sf := NewSessionFlow(b)

	t.Run("MultipleChoice", func(t *testing.T) {
		q := model.Question{
			Type:    model.QuestionMultipleChoice,
			Prompt:  "Choose one",
			Options: json.RawMessage(`["A", "B", "C"]`),
		}
		text, kb, done := sf.renderByType(ctx, 1, nil, 10, 0, 5, q, false)
		if done {
			t.Fatal("expected not done")
		}
		if !strings.Contains(text, "Choose one") {
			t.Errorf("text missing prompt: %s", text)
		}
		if kb == nil || len(kb.InlineKeyboard) != 2 {
			t.Fatalf("expected 2 rows for 3 options, got %v", kb)
		}
	})

	t.Run("Subjective", func(t *testing.T) {
		q := model.Question{
			Type:   model.QuestionSubjective,
			Prompt: "Write something",
		}
		_, kb, done := sf.renderByType(ctx, 123, nil, 10, 0, 5, q, false)
		if done {
			t.Fatal("expected not done")
		}
		if kb != nil {
			t.Error("expected no keyboard for subjective")
		}
		// Check Redis state for text input capture
		val := rdb.values[config.UserActiveQuestionRedisKey.Format(123)]
		if val != "10:0" {
			t.Errorf("expected Redis value 10:0, got %q", val)
		}
	})

	t.Run("Handwriting", func(t *testing.T) {
		q := model.Question{
			ID:   456,
			Type: model.QuestionKanaHandwriting,
		}
		mAPI.sentMessages = nil
		_, _, done := sf.renderByType(ctx, 123, nil, 10, 0, 5, q, false)
		if !done {
			t.Fatal("expected done for handwriting (it sends message internally)")
		}
		if len(mAPI.sentMessages) != 1 {
			t.Fatalf("expected 1 message sent, got %d", len(mAPI.sentMessages))
		}
	})
}

func TestShowQuestion_Finish(t *testing.T) {
	ctx := context.Background()
	mAPI := &mockBotAPI{}
	rdb := &testRedis{values: map[string]string{}}
	active := service.NewActiveSessionService(nil, rdb, nil)
	b := &Bot{
		api:      mAPI,
		rdb:      rdb,
		services: &service.Services{ActiveSession: active},
	}
	sf := NewSessionFlow(b)

	state := &model.ActiveSessionState{
		Version: model.ActiveSessionStateVersion,
		Session: model.Session{ID: 10},
		Items:   []model.ActiveSessionQuestion{{}}, // 1 item
	}
	raw, _ := json.Marshal(state)
	rdb.values[config.ActiveSessionWorkingSetRedisKey.Format(10)] = string(raw)

	// Index 1 on 1 item session -> should show finish
	sf.showQuestion(ctx, 123, nil, 10, 1)

	if len(mAPI.sentMessages) != 1 {
		t.Fatalf("expected 1 message, got %d", len(mAPI.sentMessages))
	}
	msg := mAPI.sentMessages[0].(tgbotapi.MessageConfig)
	if !strings.Contains(msg.Text, "모든 문제를 풀었습니다") {
		t.Errorf("wrong text: %s", msg.Text)
	}
}
