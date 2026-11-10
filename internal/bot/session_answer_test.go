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
	llmMock := &mockLLM{}

	mockAPI := &mockBotAPI{}
	sessionFlow := newTestSessionFlow(
		mockAPI,
		stateStores,
		SessionFlowDependencies{
			Session: newTestSessionService(
				stateStores,
				service.SessionDependencies{LLM: llmMock},
			),
		},
	)

	chatID := int64(123)
	message := &tgbotapi.Message{
		Chat: &tgbotapi.Chat{ID: chatID},
		From: &tgbotapi.User{ID: 1},
		Text: "apple",
	}

	t.Run(
		"no active question state",
		func(t *testing.T) {
			if sessionFlow.HandleTextInput(
				ctx,
				message,
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
				model.ActiveQuestionReference{SessionID: sessionID, QuestionIndex: 0},
			)

			state := &model.QuizActiveSessionState{
				Version: model.QuizActiveSessionStateVersion,
				Session: model.Session{ID: sessionID, UserID: 1},
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

			if !sessionFlow.HandleTextInput(
				ctx,
				message,
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
			if len(mockAPI.sentMessages) == 0 {
				t.Fatal("expected message sent")
			}
		},
	)
}

func TestProcessAnswerText_Correct(t *testing.T) {
	stateStores := newTestInteractionStores()
	llmMock := &mockLLM{}
	mockAPI := &mockBotAPI{}
	sessionFlow := newTestSessionFlow(
		mockAPI,
		stateStores,
		SessionFlowDependencies{
			Session: newTestSessionService(
				stateStores,
				service.SessionDependencies{LLM: llmMock},
			),
		},
	)

	sessionID := 10
	questionID := 1
	state := &model.QuizActiveSessionState{
		Version: model.QuizActiveSessionStateVersion,
		Session: model.Session{ID: sessionID, UserID: 1},
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
		sessionFlow,
		stateStores,
		123,
		&tgbotapi.User{ID: 1},
		sessionID,
		0,
		"apple",
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
		"정답!",
	) {
		t.Errorf(
			"wrong text: %s",
			message.Text,
		)
	}
}

func TestProcessAnswerText_AlreadyAnsweredRedirectsToResult(t *testing.T) {
	stateStores := newTestInteractionStores()
	mockAPI := &mockBotAPI{}
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

	sessionID := 10
	questionID := 1
	trueValue := true
	state := &model.QuizActiveSessionState{
		Version: model.QuizActiveSessionStateVersion,
		Session: model.Session{ID: sessionID, UserID: 1},
		Items: []model.QuizActiveSessionQuestion{
			{
				SessionQuestion: model.SessionQuestion{QuestionID: questionID, IsCorrect: &trueValue},
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
		sessionFlow,
		stateStores,
		123,
		&tgbotapi.User{ID: 1},
		sessionID,
		0,
		"apple",
	)

	message := mockAPI.sentMessages[0].(tgbotapi.MessageConfig)
	if strings.Contains(
		message.Text,
		"이미 답변한 문제입니다",
	) {
		t.Fatalf(
			"stale answer should not return already-answered error: %s",
			message.Text,
		)
	}
	if !strings.Contains(
		message.Text,
		"모든 문제를 풀었습니다",
	) {
		t.Errorf(
			"expected result redirect, got: %s",
			message.Text,
		)
	}
}

func TestProcessAnswer_AlreadyAnsweredRedirectsToNextQuestion(t *testing.T) {
	ctx := context.Background()
	stateStores := newTestInteractionStores()
	mockAPI := &mockBotAPI{}
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

	sessionID := 11
	firstAnswered := true
	state := &model.QuizActiveSessionState{
		Version: model.QuizActiveSessionStateVersion,
		Session: model.Session{ID: sessionID, UserID: 123},
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

	sessionFlow.processAnswer(
		ctx,
		callbackWithMessage(
			"q:11:1:0",
			123,
			456,
			123,
		),
		sessionID,
		1,
		0,
	)

	if len(mockAPI.sentMessages) != 1 {
		t.Fatalf(
			"expected one redirected message, got %d",
			len(mockAPI.sentMessages),
		)
	}
	message, ok := mockAPI.sentMessages[0].(tgbotapi.EditMessageTextConfig)
	if !ok {
		t.Fatalf(
			"expected edited message, got %T",
			mockAPI.sentMessages[0],
		)
	}
	if strings.Contains(
		message.Text,
		"이미 답변한 문제입니다",
	) || !strings.Contains(
		message.Text,
		"두 번째 문제",
	) {
		t.Fatalf(
			"expected redirect to next question, got %q",
			message.Text,
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
						Session: model.Session{ID: sessionID, UserID: 1},
						Items: []model.QuizActiveSessionQuestion{{
							SessionQuestion: model.SessionQuestion{QuestionID: questionID},
							Question:        model.Question{ID: questionID, MaterialID: &materialID},
						}},
					},
				)
				if tt.storeMessage {
					stateStores.messages[handwritingMessageKey{sessionID, tt.gradedQuestion}] = model.TelegramMessageReference{
						ChatID:    55,
						MessageID: 66,
					}
				}
				mockAPI := &mockBotAPI{}
				sessionFlow := newTestSessionFlow(
					mockAPI,
					stateStores,
					SessionFlowDependencies{
						Session: newTestSessionService(
							stateStores,
							service.SessionDependencies{},
						),
						PublicBaseURL: publicBaseURL,
					},
				)

				sessionFlow.ShowHandwritingGraded(
					context.Background(),
					sessionID,
					tt.gradedQuestion,
				)

				if !tt.wantEdit {
					if len(mockAPI.sentMessages) != 0 {
						t.Fatalf(
							"sent %d Telegram calls, want none",
							len(mockAPI.sentMessages),
						)
					}
					return
				}
				if len(mockAPI.sentMessages) != 1 {
					t.Fatalf(
						"sent %d Telegram calls, want 1",
						len(mockAPI.sentMessages),
					)
				}
				edit, ok := mockAPI.sentMessages[0].(tgbotapi.EditMessageReplyMarkupConfig)
				if !ok {
					t.Fatalf(
						"sent %T, want EditMessageReplyMarkupConfig",
						mockAPI.sentMessages[0],
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

// A malformed option index must be dropped instead of being read as option 0.
func TestHandleAnswerCallback_NonNumericOptionIndexIsIgnored(t *testing.T) {
	ctx := context.Background()
	stateStores := newTestInteractionStores()
	mockAPI := &mockBotAPI{}
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

	sessionID, questionID := 12, 1
	seedQuizState(
		stateStores,
		&model.QuizActiveSessionState{
			Version: model.QuizActiveSessionStateVersion,
			Session: model.Session{ID: sessionID, UserID: 123},
			Items: []model.QuizActiveSessionQuestion{
				{
					SessionQuestion: model.SessionQuestion{QuestionID: questionID},
					Question: model.Question{
						ID:            questionID,
						Type:          model.QuestionMultipleChoice,
						CorrectAnswer: "A",
						Options:       json.RawMessage(`["A", "B"]`),
					},
				},
			},
		},
	)

	sessionFlow.HandleAnswerCallback(
		ctx,
		callbackWithMessage(
			fmt.Sprintf(
				"q:%d:%d:x",
				sessionID,
				questionID,
			),
			123,
			456,
			123,
		),
	)

	if len(mockAPI.sentMessages) != 0 {
		t.Fatalf(
			"malformed callback produced messages: %d",
			len(mockAPI.sentMessages),
		)
	}
	state, err := stateStores.quiz.Load(
		ctx,
		sessionID,
	)
	if err != nil {
		t.Fatalf(
			"load state: %v",
			err,
		)
	}
	if state.Items[0].SessionQuestion.IsCorrect != nil {
		t.Fatal("malformed callback recorded an answer")
	}
}

// newResumeTestSessionFlow wires a SessionFlow whose user has the given
// in-progress sessions and whose Quiz working set is state (nil = expired).
func newResumeTestSessionFlow(
	api *mockBotAPI,
	inProgress []model.Session,
	state *model.QuizActiveSessionState,
) *SessionFlow {
	stores := newTestInteractionStores()
	if state != nil {
		seedQuizState(
			stores,
			state,
		)
	}
	sessionStore := &mockSessionStore{
		getSessionsByStatusFunc: func(
			ctx context.Context,
			userID int64,
			status model.SessionStatus,
		) ([]model.Session, error) {
			if status == model.SessionInProgress {
				return inProgress, nil
			}
			return nil, nil
		},
	}
	return newTestSessionFlow(
		api,
		stores,
		SessionFlowDependencies{
			Session: newTestSessionService(
				stores,
				service.SessionDependencies{SessionRepository: sessionStore},
			),
		},
	)
}

func TestResumeQuizFromTextReportsLastResultAndResendsNextQuestion(t *testing.T) {
	correct, wrong := true, false
	tests := []struct {
		name      string
		isCorrect *bool
		wantLine  string
	}{
		{
			name:      "last answer correct",
			isCorrect: &correct,
			wantLine:  "직전 문제 1/2: ✅ 정답",
		},
		{
			name:      "last answer wrong shows correct answer",
			isCorrect: &wrong,
			wantLine:  "직전 문제 1/2: ❌ 오답 · 정답 <b>すばらしい</b>",
		},
	}
	for _, tt := range tests {
		t.Run(
			tt.name,
			func(t *testing.T) {
				api := &mockBotAPI{}
				sessionFlow := newResumeTestSessionFlow(
					api,
					[]model.Session{{ID: 10, Mode: model.SessionModeQuiz}},
					&model.QuizActiveSessionState{
						Version: model.QuizActiveSessionStateVersion,
						Session: model.Session{ID: 10},
						Items: []model.QuizActiveSessionQuestion{
							{
								SessionQuestion: model.SessionQuestion{IsCorrect: tt.isCorrect},
								Question: model.Question{
									ID:            1,
									Prompt:        "Q1",
									Type:          model.QuestionFillBlank,
									CorrectAnswer: "すばらしい",
								},
							},
							{
								Question: model.Question{
									ID:      2,
									Prompt:  "Q2",
									Type:    model.QuestionMultipleChoice,
									Options: json.RawMessage(`["A","B"]`),
								},
							},
						},
					},
				)

				resumed := sessionFlow.ResumeQuizFromText(
					context.Background(),
					&tgbotapi.Message{
						Text: "すばらしい",
						From: &tgbotapi.User{ID: 123},
						Chat: &tgbotapi.Chat{ID: 456},
					},
				)

				if !resumed {
					t.Fatal("ResumeQuizFromText = false, want true")
				}
				if len(api.sentMessages) != 2 {
					t.Fatalf(
						"sent messages = %d, want 2 (notice + next question)",
						len(api.sentMessages),
					)
				}
				notice := api.sentMessages[0].(tgbotapi.MessageConfig)
				if !strings.Contains(
					notice.Text,
					tt.wantLine,
				) {
					t.Fatalf(
						"notice = %q, want line %q",
						notice.Text,
						tt.wantLine,
					)
				}
				next := api.sentMessages[1].(tgbotapi.MessageConfig)
				if !strings.Contains(
					next.Text,
					"Q2",
				) {
					t.Fatalf(
						"next question = %q, want Q2",
						next.Text,
					)
				}
			},
		)
	}
}

func TestResumeQuizFromTextWithoutInProgressQuizSendsNothing(t *testing.T) {
	api := &mockBotAPI{}
	sessionFlow := newResumeTestSessionFlow(
		api,
		nil,
		nil,
	)

	if sessionFlow.ResumeQuizFromText(
		context.Background(),
		&tgbotapi.Message{
			Text: "hello",
			From: &tgbotapi.User{ID: 123},
			Chat: &tgbotapi.Chat{ID: 456},
		},
	) {
		t.Fatal("ResumeQuizFromText = true, want false")
	}
	if len(api.sentMessages) != 0 {
		t.Fatalf(
			"sent messages = %d, want 0",
			len(api.sentMessages),
		)
	}
}

func TestResumeQuizFromTextWithExpiredStateShowsUnavailable(t *testing.T) {
	api := &mockBotAPI{}
	sessionFlow := newResumeTestSessionFlow(
		api,
		[]model.Session{{ID: 10, Mode: model.SessionModeQuiz}},
		nil,
	)

	if !sessionFlow.ResumeQuizFromText(
		context.Background(),
		&tgbotapi.Message{
			Text: "hello",
			From: &tgbotapi.User{ID: 123},
			Chat: &tgbotapi.Chat{ID: 456},
		},
	) {
		t.Fatal("ResumeQuizFromText = false, want true")
	}
	if len(api.sentMessages) != 1 {
		t.Fatalf(
			"sent messages = %d, want 1",
			len(api.sentMessages),
		)
	}
	sent := api.sentMessages[0].(tgbotapi.MessageConfig)
	if sent.Text != botMessagesByLocale[botDefaultLocale].activeSessionUnavailable {
		t.Fatalf(
			"sent = %q, want active session unavailable",
			sent.Text,
		)
	}
}
