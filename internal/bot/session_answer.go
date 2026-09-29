package bot

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"strings"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"

	"github.com/lsj/copylingo/internal/callback"
	"github.com/lsj/copylingo/internal/model"
	"github.com/lsj/copylingo/internal/observability"
	"github.com/lsj/copylingo/internal/service"
)

func (sf *SessionFlow) processAnswer(
	ctx context.Context,
	cb *tgbotapi.CallbackQuery,
	sessionID,
	questionID int,
	optionIdx int,
) {
	state, err := sf.bot.services.QuizActiveSession.Get(
		ctx,
		sessionID,
	)
	if err != nil {
		return
	}
	item, _, ok := state.CurrentItemByQuestionID(questionID)
	if !ok {
		// A delayed button can point at a question that is no longer current
		// (including one already answered). Re-render the first unanswered item
		// instead of silently dropping the callback.
		if _, exists := state.ItemByQuestionID(questionID); exists {
			sf.redirectToNextUnansweredQuestion(
				ctx,
				cb.Message.Chat.ID,
				sessionID,
				&cb.Message.MessageID,
			)
		}
		return
	}
	if item.SessionQuestion.IsCorrect != nil {
		sf.redirectToNextUnansweredQuestion(
			ctx,
			cb.Message.Chat.ID,
			sessionID,
			&cb.Message.MessageID,
		)
		return
	}
	question := item.Question

	options, err := question.GetOptions()
	if err != nil || optionIdx >= len(options) {
		return
	}

	selectedAnswer := options[optionIdx]
	editMessageID := cb.Message.MessageID
	sf.processAnswerText(
		ctx,
		cb.Message.Chat.ID,
		cb.From,
		sessionID,
		questionID,
		selectedAnswer,
		&editMessageID,
	)
}

// HandleTextInput intercepts text messages if there is an active text question.
func (sf *SessionFlow) HandleTextInput(
	ctx context.Context,
	msg *tgbotapi.Message,
) bool {
	if sf.bot.input == nil {
		return false
	}
	activeQuestion, err := sf.bot.input.GetActiveQuestion(
		ctx,
		msg.Chat.ID,
	)
	if err != nil || activeQuestion == nil {
		return false
	}
	_ = sf.bot.input.DeleteActiveQuestion(
		ctx,
		msg.Chat.ID,
	)

	state, err := sf.bot.services.QuizActiveSession.Get(
		ctx,
		activeQuestion.SessionID,
	)
	if err != nil || activeQuestion.QuestionIndex >= len(state.Items) {
		return false
	}
	questionID := state.Items[activeQuestion.QuestionIndex].SessionQuestion.QuestionID

	sf.processAnswerText(
		ctx,
		msg.Chat.ID,
		msg.From,
		activeQuestion.SessionID,
		questionID,
		strings.TrimSpace(msg.Text),
		nil,
	)
	return true
}

func (sf *SessionFlow) processAnswerText(
	ctx context.Context,
	chatID int64,
	from *tgbotapi.User,
	sessionID,
	questionID int,
	selectedAnswer string,
	editMessageID *int,
) {
	ctx = observability.WithAttrs(
		ctx,
		slog.Int(
			"session_id",
			sessionID,
		),
		slog.Int(
			"question_id",
			questionID,
		),
	)
	state, err := sf.bot.services.QuizActiveSession.Get(
		ctx,
		sessionID,
	)
	if err != nil {
		slog.ErrorContext(
			ctx,
			"Failed to get active session for answer",
			"event",
			"telegram.answer.session_lookup_failed",
			"error",
			err,
		)
		sf.showQuizActiveSessionUnavailable(
			chatID,
			editMessageID,
		)
		return
	}
	item, currentIdx, ok := state.CurrentItemByQuestionID(questionID)
	if !ok {
		slog.WarnContext(
			ctx,
			"Question not found in active session",
			"event",
			"telegram.answer.question_not_found",
		)
		if _, exists := state.ItemByQuestionID(questionID); exists {
			sf.redirectToNextUnansweredQuestion(
				ctx,
				chatID,
				sessionID,
				editMessageID,
			)
		}
		return
	}
	if item.SessionQuestion.IsCorrect != nil {
		sf.redirectToNextUnansweredQuestion(
			ctx,
			chatID,
			sessionID,
			editMessageID,
		)
		return
	}
	question := item.Question

	// Grade the answer
	switch question.Type {
	case model.QuestionFillBlank:
		selectedAnswer = strings.ToLower(selectedAnswer) // For Kana fill in the blank
	case model.QuestionSubjective:
		// Show typing status for AI grading UX
		sf.bot.api.Request(tgbotapi.NewChatAction(
			chatID,
			tgbotapi.ChatTyping,
		))
	}

	isCorrect, feedback, err := sf.bot.services.Grader.GradeAnswerWithQuestion(
		ctx,
		sessionID,
		questionID,
		&question,
		selectedAnswer,
	)
	if err != nil {
		if errors.Is(
			err,
			service.ErrAIUnavailable,
		) {
			errMsg := tgbotapi.NewMessage(
				chatID,
				botMessagesByLocale[botDefaultLocale].subjectiveGradingUnavailable,
			)
			sf.bot.api.Send(errMsg)
			isCorrect = false
			if recordErr := sf.bot.services.QuizActiveSession.RecordAnswer(
				ctx,
				sessionID,
				questionID,
				selectedAnswer,
				false,
			); recordErr != nil {
				if errors.Is(
					recordErr,
					service.ErrQuizActiveSessionAlreadyAnswered,
				) {
					sf.redirectToNextUnansweredQuestion(
						ctx,
						chatID,
						sessionID,
						editMessageID,
					)
					return
				}
				slog.ErrorContext(
					ctx,
					"Failed to record fallback wrong answer",
					"event",
					"telegram.answer.fallback_record_failed",
					"error",
					recordErr,
				)
				return
			}
		} else if errors.Is(
			err,
			service.ErrQuizActiveSessionAlreadyAnswered,
		) {
			sf.redirectToNextUnansweredQuestion(
				ctx,
				chatID,
				sessionID,
				editMessageID,
			)
			return
		} else {
			slog.ErrorContext(
				ctx,
				"Failed to grade answer",
				"event",
				"telegram.answer.grading_failed",
				"error",
				err,
			)
			return
		}
	}

	// 원본 문제 메시지는 editMessage로 덮어써지므로, 결과에 문제 원문을 다시 실어 맥락을 보존한다.
	messages := botMessagesByLocale[botDefaultLocale]
	promptLine := fmt.Sprintf(
		messages.questionAnswerPromptFormat,
		question.Prompt,
	)
	var text string
	if isCorrect {
		text = promptLine + fmt.Sprintf(
			messages.correctAnswerResultFormat,
			question.Explanation,
		)
	} else {
		text = promptLine + fmt.Sprintf(
			messages.wrongAnswerResultFormat,
			selectedAnswer,
			question.CorrectAnswer,
			question.Explanation,
		)
	}

	if feedback != "" {
		text += fmt.Sprintf(
			messages.aiFeedbackFormat,
			feedback,
		)
	}

	nextLabel := messages.nextQuestionButton
	if currentIdx+1 >= len(state.Items) {
		nextLabel = messages.resultsButton
	}

	var nextData string
	if currentIdx+1 >= len(state.Items) {
		nextData = fmt.Sprintf(
			formatSessionFinish,
			sessionID,
		)
	} else {
		nextData = fmt.Sprintf(
			callback.FormatQuestionNext,
			sessionID,
			currentIdx,
		)
	}

	// "다음/결과" 버튼과 owner 전용 "질문" 버튼을 한 row에 나란히 둔다.
	row := tgbotapi.NewInlineKeyboardRow(
		tgbotapi.NewInlineKeyboardButtonData(
			nextLabel,
			nextData,
		),
	)
	// owner에게만 "이 문제 질문" 버튼을 노출한다 (LLM 비용/abuse gate, ADR-028·029).
	if sf.bot.isLLMAllowed(from) {
		row = append(
			row,
			tgbotapi.NewInlineKeyboardButtonData(
				messages.askButton,
				fmt.Sprintf(
					formatQuestionAskLLM,
					sessionID,
					questionID,
				),
			),
		)
	}
	keyboard := tgbotapi.NewInlineKeyboardMarkup(row)
	if question.MaterialID != nil && sf.bot.services.MaterialPreference != nil {
		keyboard.InlineKeyboard = append(
			keyboard.InlineKeyboard,
			tgbotapi.NewInlineKeyboardRow(
				tgbotapi.NewInlineKeyboardButtonData(
					messages.linkedMaterialSettingsButton,
					fmt.Sprintf(
						callback.FormatQuestionPolicy,
						sessionID,
						questionID,
					),
				),
			),
		)
	}

	if editMessageID != nil {
		sf.bot.EditMessage(
			chatID,
			*editMessageID,
			text,
			&keyboard,
		)
	} else {
		// 텍스트 답변은 사용자 메시지로 들어오므로 편집할 봇 문제 메시지가 없다.
		sf.bot.SendMessageWithKeyboard(
			chatID,
			text,
			keyboard,
		)
	}
}

// redirectToNextUnansweredQuestion repairs delayed answer callbacks by
// rendering the current working-set position. When every item is answered,
// showQuestion renders the existing result button.
func (sf *SessionFlow) redirectToNextUnansweredQuestion(
	ctx context.Context,
	chatID int64,
	sessionID int,
	editMessageID *int,
) {
	nextIdx, err := sf.nextUnansweredQuestionIndex(
		ctx,
		sessionID,
	)
	if err != nil {
		sf.showQuizActiveSessionUnavailable(
			chatID,
			editMessageID,
		)
		return
	}
	sf.showQuestion(
		ctx,
		chatID,
		editMessageID,
		sessionID,
		nextIdx,
	)
}
