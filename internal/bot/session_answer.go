package bot

import (
	"context"
	"errors"
	"fmt"
	"log/slog"

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
	editMessageID := cb.Message.MessageID
	result, err := sf.session.SubmitQuizOption(
		ctx,
		cb.From.ID,
		sessionID,
		questionID,
		optionIdx,
	)
	if err != nil {
		sf.handleQuizAnswerError(
			ctx,
			cb.Message.Chat.ID,
			sessionID,
			&editMessageID,
			err,
		)
		return
	}
	sf.renderQuizAnswerResult(
		cb.Message.Chat.ID,
		cb.From,
		sessionID,
		result,
		&editMessageID,
	)
}

// HandleTextInput intercepts text messages if there is an active text question.
// It returns false (message not consumed) when the active question can no
// longer be resolved from the Quiz working set.
func (sf *SessionFlow) HandleTextInput(
	ctx context.Context,
	msg *tgbotapi.Message,
) bool {
	if sf.input == nil {
		return false
	}
	activeQuestion, err := sf.input.GetActiveQuestion(
		ctx,
		msg.Chat.ID,
	)
	if err != nil || activeQuestion == nil {
		return false
	}
	_ = sf.input.DeleteActiveQuestion(
		ctx,
		msg.Chat.ID,
	)

	sessionID := activeQuestion.SessionID
	ctx = observability.WithAttrs(
		ctx,
		slog.Int(
			"session_id",
			sessionID,
		),
	)
	result, err := sf.session.SubmitQuizText(
		ctx,
		service.QuizTextAnswer{
			UserID:        msg.From.ID,
			SessionID:     sessionID,
			QuestionIndex: activeQuestion.QuestionIndex,
			Text:          msg.Text,
			// Show typing status while AI grades a subjective answer.
			OnAIGrading: func() {
				sf.telegram.SendChatAction(
					msg.Chat.ID,
					tgbotapi.ChatTyping,
				)
			},
		},
	)
	if errors.Is(
		err,
		service.ErrQuizStateUnavailable,
	) || errors.Is(
		err,
		service.ErrQuizActiveSessionQuestionNotFound,
	) {
		return false
	}
	if err != nil {
		sf.handleQuizAnswerError(
			ctx,
			msg.Chat.ID,
			sessionID,
			nil,
			err,
		)
		return true
	}
	sf.renderQuizAnswerResult(
		msg.Chat.ID,
		msg.From,
		sessionID,
		result,
		nil,
	)
	return true
}

// handleQuizAnswerError maps a rejected answer to its screen: a stale answer
// re-renders the next unanswered question, unexpected grading failures are
// logged, and unresolvable targets are ignored.
func (sf *SessionFlow) handleQuizAnswerError(
	ctx context.Context,
	chatID int64,
	sessionID int,
	editMessageID *int,
	err error,
) {
	switch {
	case errors.Is(
		err,
		service.ErrQuizAnswerStale,
	):
		sf.redirectToNextUnansweredQuestion(
			ctx,
			chatID,
			sessionID,
			editMessageID,
		)
	case errors.Is(
		err,
		service.ErrQuizStateUnavailable,
	), errors.Is(
		err,
		service.ErrQuizActiveSessionQuestionNotFound,
	), errors.Is(
		err,
		service.ErrQuizInvalidOption,
	):
		return
	case errors.Is(
		err,
		service.ErrQuizActiveSessionUserMismatch,
	):
		// Forged callback for another user's session: drop silently.
		slog.WarnContext(
			ctx,
			"Quiz answer rejected: session owner mismatch",
			"event",
			"telegram.answer.owner_mismatch",
			"error",
			err,
		)
		return
	default:
		slog.ErrorContext(
			ctx,
			"Failed to grade answer",
			"event",
			"telegram.answer.grading_failed",
			"error",
			err,
		)
	}
}

func (sf *SessionFlow) renderQuizAnswerResult(
	chatID int64,
	from *tgbotapi.User,
	sessionID int,
	result *service.QuizAnswerResult,
	editMessageID *int,
) {
	if result.GradingUnavailable {
		sf.telegram.SendMessage(
			chatID,
			botMessagesByLocale[botDefaultLocale].subjectiveGradingUnavailable,
		)
	}
	question := result.Question
	questionID := question.ID
	currentIdx := result.QuestionIndex

	// 원본 문제 메시지는 editMessage로 덮어써지므로, 결과에 문제 원문을 다시 실어 맥락을 보존한다.
	messages := botMessagesByLocale[botDefaultLocale]
	promptLine := fmt.Sprintf(
		messages.questionAnswerPromptFormat,
		question.Prompt,
	)
	var text string
	if result.IsCorrect {
		text = promptLine + fmt.Sprintf(
			messages.correctAnswerResultFormat,
			question.Explanation,
		)
	} else {
		text = promptLine + fmt.Sprintf(
			messages.wrongAnswerResultFormat,
			result.Answer,
			question.CorrectAnswer,
			question.Explanation,
		)
	}

	if result.Feedback != "" {
		text += fmt.Sprintf(
			messages.aiFeedbackFormat,
			result.Feedback,
		)
	}

	nextLabel := messages.nextQuestionButton
	if currentIdx+1 >= result.TotalQuestions {
		nextLabel = messages.resultsButton
	}

	var nextData string
	if currentIdx+1 >= result.TotalQuestions {
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
	if isLLMAllowed(from) {
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
	if question.MaterialID != nil {
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
		sf.telegram.EditMessage(
			chatID,
			*editMessageID,
			text,
			&keyboard,
		)
	} else {
		// 텍스트 답변은 사용자 메시지로 들어오므로 편집할 봇 문제 메시지가 없다.
		sf.telegram.SendMessageWithKeyboard(
			chatID,
			text,
			keyboard,
		)
	}
}

// ShowHandwritingGraded swaps the keyboard of a handwriting question message
// once the Mini App has graded it: "next question" plus the linked-material
// action. The Mini App calls it in the background, so failures are logged
// under handwriting.cleanup.* instead of returned.
func (sf *SessionFlow) ShowHandwritingGraded(
	ctx context.Context,
	sessionID,
	questionID int,
) {
	message, err := sf.messages.GetHandwritingMessage(
		ctx,
		sessionID,
		questionID,
	)
	if err != nil {
		if errors.Is(
			err,
			model.ErrInvalidTelegramMessageRef,
		) {
			slog.ErrorContext(
				ctx,
				"Invalid handwriting message ID format",
				"event",
				"handwriting.cleanup.invalid_message_id",
				"error",
				err,
			)
		} else {
			slog.ErrorContext(
				ctx,
				"Failed to get handwriting message ID",
				"event",
				"handwriting.cleanup.message_lookup_failed",
				"error",
				err,
			)
		}
		return
	}
	if message == nil {
		return
	}

	// The "next" callback carries the question position, so re-read progress
	// instead of trusting the Mini App request.
	state, err := sf.session.QuizProgress(
		ctx,
		sessionID,
	)
	if err != nil {
		slog.ErrorContext(
			ctx,
			"Failed to get active session state for handwriting cleanup",
			"event",
			"handwriting.cleanup.session_lookup_failed",
			"error",
			err,
		)
		return
	}
	item, questionIdx, ok := state.CurrentItemByQuestionID(questionID)
	if !ok {
		slog.WarnContext(
			ctx,
			"Question not found in session for handwriting cleanup",
			"event",
			"handwriting.cleanup.question_not_found",
		)
		return
	}

	markup := sf.handwritingGradedKeyboard(
		sessionID,
		questionIdx,
		item.Question,
	)
	if err := sf.telegram.EditMessageReplyMarkup(
		message.ChatID,
		message.MessageID,
		markup,
	); err != nil {
		slog.ErrorContext(
			ctx,
			"Failed to edit handwriting message reply markup",
			"event",
			"handwriting.cleanup.reply_markup_failed",
			"chat_id",
			message.ChatID,
			"message_id",
			message.MessageID,
			"error",
			err,
		)
	}
}

// handwritingGradedKeyboard replaces the Mini App answer button after
// grading. The linked-material action stays available.
func (sf *SessionFlow) handwritingGradedKeyboard(
	sessionID,
	questionIdx int,
	question model.Question,
) tgbotapi.InlineKeyboardMarkup {
	messages := botMessagesByLocale[botDefaultLocale]
	markup := tgbotapi.NewInlineKeyboardMarkup(
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData(
				messages.nextQuestionButton,
				callback.FormatHandwritingNext(
					sessionID,
					questionIdx,
					sf.publicBaseURL,
				),
			),
		),
	)
	if question.MaterialID != nil {
		markup.InlineKeyboard = append(
			markup.InlineKeyboard,
			tgbotapi.NewInlineKeyboardRow(
				tgbotapi.NewInlineKeyboardButtonData(
					messages.linkedMaterialSettingsButton,
					fmt.Sprintf(
						callback.FormatQuestionPolicy,
						sessionID,
						question.ID,
					),
				),
			),
		)
	}
	return markup
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
