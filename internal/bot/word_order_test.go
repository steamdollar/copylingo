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

func newWordOrderFixture(t *testing.T, options []string) (*SessionFlow, *testRedis, *mockBotAPI) {
	t.Helper()
	rdb := &testRedis{values: map[string]string{}}
	active := service.NewActiveSessionService(nil, rdb, &mockSRS{})
	grader := service.NewGraderService(nil, active, &mockLLM{})
	api := &mockBotAPI{}
	b := &Bot{
		api: api,
		rdb: rdb,
		services: &service.Services{
			ActiveSession: active,
			Grader:        grader,
		},
	}
	state := &model.ActiveSessionState{
		Version: model.ActiveSessionStateVersion,
		Session: model.Session{ID: 7, UserID: 42},
		Items: []model.ActiveSessionQuestion{{
			SessionQuestion: model.SessionQuestion{QuestionID: 99},
			Question: model.Question{
				ID:            99,
				Type:          model.QuestionWordOrder,
				Prompt:        "문장을 배열하세요.",
				Options:       mustTestJSON(t, options),
				CorrectAnswer: strings.Join(options, ""),
			},
		}},
	}
	storeActiveState(t, rdb, 7, state)
	return NewSessionFlow(b), rdb, api
}

func mustTestJSON(t *testing.T, value any) []byte {
	t.Helper()
	raw, err := json.Marshal(value)
	if err != nil {
		t.Fatalf("marshal test JSON: %v", err)
	}
	return raw
}

func draftSelection(t *testing.T, rdb *testRedis) []int {
	t.Helper()
	raw := rdb.values[config.WordOrderDraftRedisKey.Format(7, 99)]
	if raw == "" {
		return nil
	}
	var selection []int
	if err := json.Unmarshal([]byte(raw), &selection); err != nil {
		t.Fatalf("decode draft: %v", err)
	}
	return selection
}

func TestWordOrderShuffleDeterministic(t *testing.T) {
	t.Parallel()
	first := wordOrderShuffleOrder(7, 99, 8)
	for i := 0; i < 5; i++ {
		if got := wordOrderShuffleOrder(7, 99, 8); !sameInts(got, first) {
			t.Fatalf("shuffle changed between callbacks: %v vs %v", got, first)
		}
	}
	if sameInts(first, wordOrderShuffleOrder(8, 99, 8)) {
		t.Fatalf("different session unexpectedly produced same order: %v", first)
	}

	kb := wordOrderKeyboard(7, 99, []string{"私", "は", "私", "です。"}, nil)
	for _, row := range kb.InlineKeyboard {
		for _, button := range row {
			if button.CallbackData != nil && len(*button.CallbackData) > 64 {
				t.Fatalf("callback exceeds Telegram limit: %q", *button.CallbackData)
			}
		}
	}
}

func TestWordOrderRepeatedChunksUndoReset(t *testing.T) {
	sf, rdb, api := newWordOrderFixture(t, []string{"私", "は", "私", "です。"})
	ctx := context.Background()
	text, _, done := sf.renderByType(ctx, 42, nil, 7, 0, 1, model.Question{
		ID:      99,
		Type:    model.QuestionWordOrder,
		Prompt:  "문장을 배열하세요.",
		Options: mustTestJSON(t, []string{"私", "は", "私", "です。"}),
	}, false)
	if done || !strings.Contains(text, "조립") {
		t.Fatalf("word-order renderer output invalid: done=%t text=%q", done, text)
	}
	selectChunk := func(index int) {
		sf.handleWordOrderCallback(ctx, cbWithMessage(
			fmtWordOrderSelect(7, 99, index), 42, 1, 42,
		))
	}
	selectChunk(0)
	selectChunk(2)
	if got := draftSelection(t, rdb); !sameInts(got, []int{0, 2}) {
		t.Fatalf("repeated chunks must use original indices, got %v", got)
	}
	selectChunk(0)
	if got := draftSelection(t, rdb); !sameInts(got, []int{0, 2}) {
		t.Fatalf("duplicate tap changed draft: %v", got)
	}
	sf.handleWordOrderCallback(ctx, cbWithMessage(fmt.Sprintf(config.FormatWordOrderUndo, 7, 99), 42, 1, 42))
	if got := draftSelection(t, rdb); !sameInts(got, []int{0}) {
		t.Fatalf("undo selection = %v, want [0]", got)
	}
	sf.handleWordOrderCallback(ctx, cbWithMessage(fmt.Sprintf(config.FormatWordOrderReset, 7, 99), 42, 1, 42))
	if _, ok := rdb.values[wordOrderDraftKey(7, 99)]; ok {
		t.Fatal("reset must delete the draft key")
	}
	if len(api.sentMessages) == 0 {
		t.Fatal("tap actions should edit the Telegram message")
	}
}

func TestWordOrderRejectsStaleOwnerAndIndex(t *testing.T) {
	sf, rdb, _ := newWordOrderFixture(t, []string{"私", "は", "です。"})
	ctx := context.Background()
	for _, cb := range []*tgbotapi.CallbackQuery{
		cbWithMessage(fmtWordOrderSelect(7, 99, 99), 42, 1, 42), // invalid index
		cbWithMessage(fmtWordOrderSelect(7, 99, 0), 42, 1, 999), // wrong owner
		cbWithMessage(fmtWordOrderSelect(7, 98, 0), 42, 1, 42),  // stale question
	} {
		sf.handleWordOrderCallback(ctx, cb)
	}
	if len(rdb.values) != 1 {
		t.Fatalf("stale/invalid callbacks changed Redis: %#v", rdb.values)
	}
}

func TestWordOrderSubmitExactAndIdempotent(t *testing.T) {
	sf, rdb, api := newWordOrderFixture(t, []string{"私", "は", "学生", "です。"})
	ctx := context.Background()
	for i := 0; i < 4; i++ {
		sf.handleWordOrderCallback(ctx, cbWithMessage(fmtWordOrderSelect(7, 99, i), 42, 1, 42))
	}
	if got := draftSelection(t, rdb); !sameInts(got, []int{0, 1, 2, 3}) {
		t.Fatalf("complete draft = %v", got)
	}
	sf.handleWordOrderCallback(ctx, cbWithMessage(fmt.Sprintf(config.FormatWordOrderSubmit, 7, 99), 42, 1, 42))
	state, err := sf.bot.services.ActiveSession.Get(ctx, 7)
	if err != nil {
		t.Fatalf("active session get: %v", err)
	}
	if state.Items[0].SessionQuestion.UserAnswer == nil || *state.Items[0].SessionQuestion.UserAnswer != "私は学生です。" {
		t.Fatalf("submitted answer = %v", state.Items[0].SessionQuestion.UserAnswer)
	}
	if _, ok := rdb.values[wordOrderDraftKey(7, 99)]; ok {
		t.Fatal("submit must delete the draft")
	}
	messageCount := len(api.sentMessages)
	sf.handleWordOrderCallback(ctx, cbWithMessage(fmt.Sprintf(config.FormatWordOrderSubmit, 7, 99), 42, 1, 42))
	if len(api.sentMessages) != messageCount+1 {
		t.Fatal("duplicate submit should be acknowledged without grading twice")
	}
}

func fmtWordOrderSelect(sessionID, questionID, index int) string {
	return fmt.Sprintf(config.FormatWordOrderSelect, sessionID, questionID, index)
}

func sameInts(a, b []int) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}
