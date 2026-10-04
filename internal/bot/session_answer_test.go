package bot

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"testing"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"

	"github.com/lsj/copylingo/internal/callback"
	"github.com/lsj/copylingo/internal/model"
	"github.com/lsj/copylingo/internal/service"
)

func TestHandleTextInput(t *testing.T) {
	ctx := context.Background()
	stateStores := newTestInteractionStores()
	mLLM := &mockLLM{}

	mAPI := &mockBotAPI{}
	sf := newTestSessionFlow(
		mAPI,
		stateStores,
		SessionFlowDeps{
			Session: newTestSessionService(
				stateStores,
				service.SessionDeps{LLM: mLLM},
			),
		},
	)

	chatID := int64(123)
	msg := &tgbotapi.Message{
		Chat: &tgbotapi.Chat{ID: chatID},
		Text: "apple",
	}

	t.Run(
		"no active question state",
		func(t *testing.T) {
			if sf.HandleTextInput(
				ctx,
				msg,
			) {
				t.Error("expected HandleTextInput to return false")
			}
		},
	)

	t.Run(
		"active question state exists",
		func(t *testing.T) {
			sessionID := 10
			_ = stateStores.SetActiveQuestion(
				ctx,
				chatID,
				model.ActiveQuestionRef{SessionID: sessionID, QuestionIndex: 0},
			)

			state := &model.QuizActiveSessionState{
				Version: model.QuizActiveSessionStateVersion,
				Session: model.Session{ID: sessionID},
				Items: []model.QuizActiveSessionQuestion{
					{
						SessionQuestion: model.SessionQuestion{QuestionID: 1},
						Question: model.Question{
							ID:            1,
							CorrectAnswer: "apple",
							Type:          model.QuestionMultipleChoice,
						},
					},
				},
			}
			seedQuizState(
				stateStores,
				state,
			)

			if !sf.HandleTextInput(
				ctx,
				msg,
			) {
				t.Error("expected HandleTextInput to return true")
			}

			if question, _ := stateStores.GetActiveQuestion(
				ctx,
				chatID,
			); question != nil {
				t.Error("expected active question key to be deleted")
			}

			// Verify message sent
			if len(mAPI.sentMessages) == 0 {
				t.Fatal("expected message sent")
			}
		},
	)
}

func TestProcessAnswerText_Correct(t *testing.T) {
	stateStores := newTestInteractionStores()
	mLLM := &mockLLM{}
	mAPI := &mockBotAPI{}
	sf := newTestSessionFlow(
		mAPI,
		stateStores,
		SessionFlowDeps{
			Session: newTestSessionService(
				stateStores,
				service.SessionDeps{LLM: mLLM},
			),
		},
	)

	sessionID := 10
	questionID := 1
	state := &model.QuizActiveSessionState{
		Version: model.QuizActiveSessionStateVersion,
		Session: model.Session{ID: sessionID},
		Items: []model.QuizActiveSessionQuestion{
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
	seedQuizState(
		stateStores,
		state,
	)

	answerByText(
		t,
		sf,
		stateStores,
		123,
		nil,
		sessionID,
		0,
		"apple",
	)

	if len(mAPI.sentMessages) != 1 {
		t.Fatalf(
			"expected 1 message, got %d",
			len(mAPI.sentMessages),
		)
	}
	msg := mAPI.sentMessages[0].(tgbotapi.MessageConfig)
	if !strings.Contains(
		msg.Text,
		"정답!",
	) {
		t.Errorf(
			"wrong text: %s",
			msg.Text,
		)
	}
}

func TestProcessAnswerText_AlreadyAnsweredRedirectsToResult(t *testing.T) {
	stateStores := newTestInteractionStores()
	mAPI := &mockBotAPI{}
	sf := newTestSessionFlow(
		mAPI,
		stateStores,
		SessionFlowDeps{
			Session: newTestSessionService(
				stateStores,
				service.SessionDeps{},
			),
		},
	)

	sessionID := 10
	questionID := 1
	trueVal := true
	state := &model.QuizActiveSessionState{
		Version: model.QuizActiveSessionStateVersion,
		Session: model.Session{ID: sessionID},
		Items: []model.QuizActiveSessionQuestion{
			{
				SessionQuestion: model.SessionQuestion{QuestionID: questionID, IsCorrect: &trueVal},
				Question:        model.Question{ID: questionID},
			},
		},
	}
	seedQuizState(
		stateStores,
		state,
	)

	answerByText(
		t,
		sf,
		stateStores,
		123,
		nil,
		sessionID,
		0,
		"apple",
	)

	msg := mAPI.sentMessages[0].(tgbotapi.MessageConfig)
	if strings.Contains(
		msg.Text,
		"이미 답변한 문제입니다",
	) {
		t.Fatalf(
			"stale answer should not return already-answered error: %s",
			msg.Text,
		)
	}
	if !strings.Contains(
		msg.Text,
		"모든 문제를 풀었습니다",
	) {
		t.Errorf(
			"expected result redirect, got: %s",
			msg.Text,
		)
	}
}

func TestProcessAnswer_AlreadyAnsweredRedirectsToNextQuestion(t *testing.T) {
	ctx := context.Background()
	stateStores := newTestInteractionStores()
	mAPI := &mockBotAPI{}
	sf := newTestSessionFlow(
		mAPI,
		stateStores,
		SessionFlowDeps{
			Session: newTestSessionService(
				stateStores,
				service.SessionDeps{},
			),
		},
	)

	sessionID := 11
	firstAnswered := true
	state := &model.QuizActiveSessionState{
		Version: model.QuizActiveSessionStateVersion,
		Session: model.Session{ID: sessionID},
		Items: []model.QuizActiveSessionQuestion{
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
	seedQuizState(
		stateStores,
		state,
	)

	sf.processAnswer(
		ctx,
		cbWithMessage(
			"q:11:1:0",
			123,
			456,
			123,
		),
		sessionID,
		1,
		0,
	)

	if len(mAPI.sentMessages) != 1 {
		t.Fatalf(
			"expected one redirected message, got %d",
			len(mAPI.sentMessages),
		)
	}
	msg, ok := mAPI.sentMessages[0].(tgbotapi.EditMessageTextConfig)
	if !ok {
		t.Fatalf(
			"expected edited message, got %T",
			mAPI.sentMessages[0],
		)
	}
	if strings.Contains(
		msg.Text,
		"이미 답변한 문제입니다",
	) || !strings.Contains(
		msg.Text,
		"두 번째 문제",
	) {
		t.Fatalf(
			"expected redirect to next question, got %q",
			msg.Text,
		)
	}
}

// ShowHandwritingGraded edits the stored handwriting message only while the
// graded question is still current, taking its position from progress.
func TestShowHandwritingGraded(t *testing.T) {
	const (
		sessionID     = 7
		questionID    = 3
		publicBaseURL = "https://x.trycloudflare.com"
	)
	materialID := 11
	tests := []struct {
		name           string
		gradedQuestion int
		storeMessage   bool
		wantEdit       bool
	}{
		{
			name:           "current question gets next and material buttons",
			gradedQuestion: questionID,
			storeMessage:   true,
			wantEdit:       true,
		},
		{name: "question no longer current is left alone", gradedQuestion: questionID + 1, storeMessage: true},
		{name: "missing message is left alone", gradedQuestion: questionID},
	}
	for _, tt := range tests {
		t.Run(
			tt.name,
			func(t *testing.T) {
				stateStores := newTestInteractionStores()
				seedQuizState(
					stateStores,
					&model.QuizActiveSessionState{
						Version: model.QuizActiveSessionStateVersion,
						Session: model.Session{ID: sessionID},
						Items: []model.QuizActiveSessionQuestion{{
							SessionQuestion: model.SessionQuestion{QuestionID: questionID},
							Question:        model.Question{ID: questionID, MaterialID: &materialID},
						}},
					},
				)
				if tt.storeMessage {
					stateStores.messages[handwritingMessageKey{sessionID, tt.gradedQuestion}] = model.TelegramMessageRef{
						ChatID:    55,
						MessageID: 66,
					}
				}
				mAPI := &mockBotAPI{}
				sf := newTestSessionFlow(
					mAPI,
					stateStores,
					SessionFlowDeps{
						Session: newTestSessionService(
							stateStores,
							service.SessionDeps{},
						),
						PublicBaseURL: publicBaseURL,
					},
				)

				sf.ShowHandwritingGraded(
					context.Background(),
					sessionID,
					tt.gradedQuestion,
				)

				if !tt.wantEdit {
					if len(mAPI.sentMessages) != 0 {
						t.Fatalf(
							"sent %d Telegram calls, want none",
							len(mAPI.sentMessages),
						)
					}
					return
				}
				if len(mAPI.sentMessages) != 1 {
					t.Fatalf(
						"sent %d Telegram calls, want 1",
						len(mAPI.sentMessages),
					)
				}
				edit, ok := mAPI.sentMessages[0].(tgbotapi.EditMessageReplyMarkupConfig)
				if !ok {
					t.Fatalf(
						"sent %T, want EditMessageReplyMarkupConfig",
						mAPI.sentMessages[0],
					)
				}
				if edit.ChatID != 55 || edit.MessageID != 66 {
					t.Fatalf(
						"edited chat %d message %d, want 55/66",
						edit.ChatID,
						edit.MessageID,
					)
				}
				wantCallbacks := []string{
					callback.FormatHandwritingNext(
						sessionID,
						0,
						publicBaseURL,
					),
					fmt.Sprintf(
						callback.FormatQuestionPolicy,
						sessionID,
						questionID,
					),
				}
				rows := edit.ReplyMarkup.InlineKeyboard
				if len(rows) != len(wantCallbacks) {
					t.Fatalf(
						"keyboard rows = %d, want %d",
						len(rows),
						len(wantCallbacks),
					)
				}
				for i, want := range wantCallbacks {
					if got := *rows[i][0].CallbackData; got != want {
						t.Errorf(
							"row %d callback = %q, want %q",
							i,
							got,
							want,
						)
					}
				}
			},
		)
	}
}
