package bot

import (
	"context"
	"encoding/json"
	"fmt"
	"net/url"
	"strings"
	"testing"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"

	"github.com/lsj/copylingo/internal/model"
	"github.com/lsj/copylingo/internal/service"
)

func TestIsStaleMiniAppCallbackWrapper(t *testing.T) {
	t.Parallel()
	if !isStaleMiniAppCallback(
		[]string{"q", "1"},
		"http://new.com",
	) {
		t.Error("expected legacy callback to be stale")
	}
}

func TestHandwritingMiniAppURL(t *testing.T) {
	t.Parallel()
	sessionFlow := newTestSessionFlow(
		nil,
		nil,
		SessionFlowDependencies{},
	)

	t.Run(
		"empty base url",
		func(t *testing.T) {
			sessionFlow.publicBaseURL = ""
			_, err := sessionFlow.handwritingMiniAppURL(
				1,
				1,
				"jp",
				"n5",
				"prompt",
				1,
			)
			if err == nil {
				t.Error("expected error for empty base URL")
			}
		},
	)

	t.Run(
		"valid url",
		func(t *testing.T) {
			sessionFlow.publicBaseURL = "https://api.example.com/"
			prompt := "뜻 <b>'학교'</b>에 해당하는 일본어 단어를 손글씨로 쓰세요"
			got, err := sessionFlow.handwritingMiniAppURL(
				123,
				456,
				"jp",
				"n5",
				prompt,
				2,
			)
			if err != nil {
				t.Fatalf(
					"unexpected error: %v",
					err,
				)
			}
			if !strings.Contains(
				got,
				"session_id=123",
			) {
				t.Errorf(
					"missing session_id in %s",
					got,
				)
			}
			if !strings.Contains(
				got,
				"question_id=456",
			) {
				t.Errorf(
					"missing question_id in %s",
					got,
				)
			}
			parsedURL, err := url.Parse(got)
			if err != nil {
				t.Fatalf(
					"parse url: %v",
					err,
				)
			}
			if gotPrompt := parsedURL.Query().Get("prompt"); gotPrompt != prompt {
				t.Errorf(
					"prompt = %q, want %q",
					gotPrompt,
					prompt,
				)
			}
			if gotCells := parsedURL.Query().Get("cells"); gotCells != "2" {
				t.Errorf(
					"cells = %q, want %q",
					gotCells,
					"2",
				)
			}
		},
	)
}

func TestHandwritingCellCountExcludesSokuon(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name   string
		answer string
		want   int
	}{
		{name: "hiragana", answer: "がっこう", want: 3},
		{name: "katakana", answer: "カッコ", want: 2},
		{name: "without sokuon", answer: "かな", want: 2},
	}
	for _, tt := range tests {
		t.Run(
			tt.name,
			func(t *testing.T) {
				if got := handwritingCellCount(tt.answer); got != tt.want {
					t.Errorf(
						"handwritingCellCount(%q) = %d, want %d",
						tt.answer,
						got,
						tt.want,
					)
				}
			},
		)
	}
}

func TestQuestionNavigation(t *testing.T) {
	ctx := context.Background()
	stateStores := newTestInteractionStores()
	sessionFlow := newTestSessionFlow(
		nil,
		nil,
		SessionFlowDependencies{
			Session: newTestSessionService(
				stateStores,
				service.SessionDependencies{},
			),
		},
	)

	trueValue := true
	state := &model.QuizActiveSessionState{
		Version: model.QuizActiveSessionStateVersion,
		Session: model.Session{ID: 10},
		Items: []model.QuizActiveSessionQuestion{
			{SessionQuestion: model.SessionQuestion{QuestionID: 1, IsCorrect: &trueValue}},
			{SessionQuestion: model.SessionQuestion{QuestionID: 2}},
			{SessionQuestion: model.SessionQuestion{QuestionID: 3}},
		},
	}
	seedQuizState(
		stateStores,
		state,
	)

	t.Run(
		"isQuestionAnswered",
		func(t *testing.T) {
			if !sessionFlow.isQuestionAnswered(
				ctx,
				10,
				0,
			) {
				t.Error("expected index 0 to be answered")
			}
			if sessionFlow.isQuestionAnswered(
				ctx,
				10,
				1,
			) {
				t.Error("expected index 1 to be unanswered")
			}
			if sessionFlow.isQuestionAnswered(
				ctx,
				10,
				5,
			) {
				t.Error("expected out of bounds to be false")
			}
		},
	)

	t.Run(
		"nextUnansweredQuestionIndex",
		func(t *testing.T) {
			index, err := sessionFlow.nextUnansweredQuestionIndex(
				ctx,
				10,
			)
			if err != nil {
				t.Fatalf(
					"failed: %v",
					err,
				)
			}
			if index != 1 {
				t.Errorf(
					"expected idx 1, got %d",
					index,
				)
			}
		},
	)
}

func TestBuildMCQLayout(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name           string
		options        []string
		widths         []int
		wantLabels     []string
		wantOptionList string
	}{
		{"short options", []string{"A", "B", "C", "D"}, []int{2, 2}, []string{"A", "B", "C", "D"}, ""},
		{
			"two-column boundary",
			[]string{"あいうえおかきく", "B", "C", "D"},
			[]int{2, 2},
			[]string{"あいうえおかきく", "B", "C", "D"},
			"",
		},
		{
			"long Japanese option",
			[]string{"あいうえおかきくけ", "B", "C", "D"},
			[]int{4},
			[]string{"1", "2", "3", "4"},
			"\n\n1. あいうえおかきくけ\n2. B\n3. C\n4. D",
		},
		{
			"long option is HTML-escaped",
			[]string{"A", "1 < 2 && 3 > 2 is true", "C"},
			[]int{3},
			[]string{"1", "2", "3"},
			"\n\n1. A\n2. 1 &lt; 2 &amp;&amp; 3 &gt; 2 is true\n3. C",
		},
		{"odd option count", []string{"A", "B", "C"}, []int{2, 1}, []string{"A", "B", "C"}, ""},
	}
	for _, tt := range tests {
		t.Run(
			tt.name,
			func(t *testing.T) {
				optionList, keyboard := buildMCQ(
					10,
					20,
					tt.options,
				)
				if optionList != tt.wantOptionList {
					t.Errorf(
						"optionList = %q, want %q",
						optionList,
						tt.wantOptionList,
					)
				}
				if len(keyboard.InlineKeyboard) != len(tt.widths) {
					t.Fatalf(
						"rows = %d, want %d",
						len(keyboard.InlineKeyboard),
						len(tt.widths),
					)
				}
				index := 0
				for rowIndex, row := range keyboard.InlineKeyboard {
					if len(row) != tt.widths[rowIndex] {
						t.Fatalf(
							"row %d has %d buttons, want %d",
							rowIndex,
							len(row),
							tt.widths[rowIndex],
						)
					}
					for _, button := range row {
						if button.Text != tt.wantLabels[index] {
							t.Errorf(
								"button %d text = %q, want %q",
								index,
								button.Text,
								tt.wantLabels[index],
							)
						}
						wantData := fmt.Sprintf(
							formatQuestionAnswer,
							10,
							20,
							index,
						)
						if button.CallbackData == nil || *button.CallbackData != wantData {
							t.Errorf(
								"button %d callback = %v, want %q",
								index,
								button.CallbackData,
								wantData,
							)
						}
						index++
					}
				}
			},
		)
	}
}

func TestRenderByType(t *testing.T) {
	ctx := context.Background()
	mockAPI := &mockBotAPI{}
	stateStores := newTestInteractionStores()
	sessionFlow := newTestSessionFlow(
		mockAPI,
		stateStores,
		SessionFlowDependencies{
			PublicBaseURL: "https://ex.com",
		},
	)

	t.Run(
		"MultipleChoice",
		func(t *testing.T) {
			question := model.Question{
				Type:    model.QuestionMultipleChoice,
				Prompt:  "Choose one",
				Options: json.RawMessage(`["A", "B", "C"]`),
			}
			text, keyboard, done := sessionFlow.renderByType(
				ctx,
				1,
				nil,
				10,
				0,
				5,
				question,
				false,
			)
			if done {
				t.Fatal("expected not done")
			}
			if !strings.Contains(
				text,
				"Choose one",
			) {
				t.Errorf(
					"text missing prompt: %s",
					text,
				)
			}
			if keyboard == nil || len(keyboard.InlineKeyboard) != 2 {
				t.Fatalf(
					"expected 2 rows for 3 options, got %v",
					keyboard,
				)
			}
		},
	)

	t.Run(
		"HeaderLevel",
		func(t *testing.T) {
			tests := []struct {
				name       string
				level      string
				isReview   bool
				wantHeader string
			}{
				{"with level", "N4", false, "📝 <b>문제 1/5</b> · N4\n\n"},
				{"with level and review", "N5", true, "📝 <b>문제 1/5</b> · N5 🔄\n\n"},
				{"without level", "", false, "📝 <b>문제 1/5</b>\n\n"},
			}
			for _, tt := range tests {
				t.Run(
					tt.name,
					func(t *testing.T) {
						question := model.Question{
							Type:             model.QuestionMultipleChoice,
							ProficiencyLevel: tt.level,
							Prompt:           "Choose one",
							Options:          json.RawMessage(`["A", "B"]`),
						}
						text, _, done := sessionFlow.renderByType(
							ctx,
							1,
							nil,
							10,
							0,
							5,
							question,
							tt.isReview,
						)
						if done {
							t.Fatal("expected not done")
						}
						if !strings.HasPrefix(
							text,
							tt.wantHeader,
						) {
							t.Errorf(
								"header = %q, want prefix %q",
								text,
								tt.wantHeader,
							)
						}
					},
				)
			}
		},
	)

	t.Run(
		"FillBlank",
		func(t *testing.T) {
			question := model.Question{
				Type:   model.QuestionFillBlank,
				Prompt: "Write something",
			}
			_, keyboard, done := sessionFlow.renderByType(
				ctx,
				123,
				nil,
				10,
				0,
				5,
				question,
				false,
			)
			if done {
				t.Fatal("expected not done")
			}
			if keyboard != nil {
				t.Error("expected no keyboard for fill-blank")
			}
			if got, ok := stateStores.active[123]; !ok ||
				got != (model.ActiveQuestionReference{SessionID: 10, QuestionIndex: 0}) {
				t.Errorf(
					"active question = %+v, want session 10 index 0",
					got,
				)
			}
		},
	)

	t.Run(
		"Handwriting",
		func(t *testing.T) {
			question := model.Question{
				ID:   456,
				Type: model.QuestionKanaHandwriting,
			}
			mockAPI.sentMessages = nil
			_, _, done := sessionFlow.renderByType(
				ctx,
				123,
				nil,
				10,
				0,
				5,
				question,
				false,
			)
			if !done {
				t.Fatal("expected done for handwriting (it sends message internally)")
			}
			if len(mockAPI.sentMessages) != 1 {
				t.Fatalf(
					"expected 1 message sent, got %d",
					len(mockAPI.sentMessages),
				)
			}
		},
	)
}

type fakeAudioRepository struct {
	setFileID    int
	setFileValue string
}

func (fake *fakeAudioRepository) SetAudioFileID(
	_ context.Context,
	id int,
	fileID string,
) error {
	fake.setFileID = id
	fake.setFileValue = fileID
	return nil
}

type fakeStore struct {
	getCalls int
	bytes    []byte
}

func (fake *fakeStore) Get(
	_ context.Context,
	_ string,
) ([]byte, error) {
	fake.getCalls++
	return fake.bytes, nil
}

func TestRenderByType_Listening(t *testing.T) {
	ctx := context.Background()
	repository := &fakeAudioRepository{}
	store := &fakeStore{bytes: []byte("ogg-bytes")}
	audio := service.NewAudioService(
		repository,
		store,
	)

	t.Run(
		"cached file_id fast path",
		func(t *testing.T) {
			mockAPI := &mockBotAPI{}
			sessionFlow := newTestSessionFlow(
				mockAPI,
				nil,
				SessionFlowDependencies{
					Input: newTestInteractionStores(),
					Audio: audio,
				},
			)
			question := model.Question{
				ID:          5,
				Type:        model.QuestionListening,
				Prompt:      "何をしますか?",
				Options:     json.RawMessage(`["A","B"]`),
				AudioPath:   stringPointer("tts/ja/kore/abc.ogg"),
				AudioFileID: stringPointer("cached-fid"),
			}
			text, keyboard, done := sessionFlow.renderByType(
				ctx,
				1,
				nil,
				10,
				0,
				5,
				question,
				false,
			)
			if done {
				t.Fatal("expected not done")
			}
			if keyboard == nil || len(keyboard.InlineKeyboard) != 1 {
				t.Fatalf(
					"expected 1 option row, got %v",
					keyboard,
				)
			}
			if !strings.Contains(
				text,
				"🎧",
			) {
				t.Errorf(
					"missing listen hint: %s",
					text,
				)
			}
			if store.getCalls != 0 {
				t.Errorf(
					"cached file_id path must not fetch store, got %d gets",
					store.getCalls,
				)
			}
			if len(mockAPI.sentMessages) != 1 {
				t.Fatalf(
					"expected 1 voice message, got %d",
					len(mockAPI.sentMessages),
				)
			}
			if _, ok := mockAPI.sentMessages[0].(tgbotapi.VoiceConfig); !ok {
				t.Errorf(
					"expected a VoiceConfig, got %T",
					mockAPI.sentMessages[0],
				)
			}
		},
	)

	t.Run(
		"no file_id fetches store, uploads, caches file_id",
		func(t *testing.T) {
			store.getCalls = 0
			mockAPI := &mockBotAPI{returnVoiceFileID: "new-fid"}
			sessionFlow := newTestSessionFlow(
				mockAPI,
				nil,
				SessionFlowDependencies{
					Input: newTestInteractionStores(),
					Audio: audio,
				},
			)
			question := model.Question{
				ID:        9,
				Type:      model.QuestionListening,
				Prompt:    "?",
				Options:   json.RawMessage(`["A","B"]`),
				AudioPath: stringPointer("tts/ja/kore/def.ogg"),
			}
			_, _, done := sessionFlow.renderByType(
				ctx,
				1,
				nil,
				10,
				0,
				5,
				question,
				false,
			)
			if done {
				t.Fatal("expected not done")
			}
			if store.getCalls != 1 {
				t.Errorf(
					"expected 1 store fetch, got %d",
					store.getCalls,
				)
			}
			if repository.setFileID != 9 || repository.setFileValue != "new-fid" {
				t.Errorf(
					"expected file_id cached for q9=new-fid, got id=%d val=%q",
					repository.setFileID,
					repository.setFileValue,
				)
			}
		},
	)

	t.Run(
		"no audio available degrades softly",
		func(t *testing.T) {
			mockAPI := &mockBotAPI{}
			sessionFlow := newTestSessionFlow(
				mockAPI,
				nil,
				SessionFlowDependencies{
					Audio: audio,
					Input: newTestInteractionStores(),
				},
			)
			question := model.Question{
				ID:      1,
				Type:    model.QuestionListening,
				Prompt:  "?",
				Options: json.RawMessage(`["A","B"]`),
				// AudioPath nil => unavailable
			}
			text, keyboard, done := sessionFlow.renderByType(
				ctx,
				1,
				nil,
				10,
				0,
				5,
				question,
				false,
			)
			if done {
				t.Fatal("expected not done (soft degrade still returns text)")
			}
			if keyboard != nil {
				t.Error("expected no keyboard when audio is unavailable")
			}
			if !strings.Contains(
				text,
				"준비하지 못했",
			) {
				t.Errorf(
					"expected soft-degrade text, got %s",
					text,
				)
			}
		},
	)
}

func TestShowQuestion_Finish(t *testing.T) {
	ctx := context.Background()
	mockAPI := &mockBotAPI{}
	stateStores := newTestInteractionStores()
	sessionFlow := newTestSessionFlow(
		mockAPI,
		stateStores,
		SessionFlowDependencies{
			Session: newTestSessionService(
				stateStores,
				service.SessionDependencies{},
			),
		},
	)

	state := &model.QuizActiveSessionState{
		Version: model.QuizActiveSessionStateVersion,
		Session: model.Session{ID: 10},
		Items:   []model.QuizActiveSessionQuestion{{}}, // 1 item
	}
	seedQuizState(
		stateStores,
		state,
	)

	sessionFlow.showQuestion(
		ctx,
		123,
		nil,
		10,
		1,
	)

	if len(mockAPI.sentMessages) != 1 {
		t.Fatalf(
			"expected 1 message, got %d",
			len(mockAPI.sentMessages),
		)
	}
	message := mockAPI.sentMessages[0].(tgbotapi.MessageConfig)
	if !strings.Contains(
		message.Text,
		"모든 문제를 풀었습니다",
	) {
		t.Errorf(
			"wrong text: %s",
			message.Text,
		)
	}
}
