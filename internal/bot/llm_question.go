package bot

import (
	"context"
	"fmt"
	"html"
	"log/slog"
	"strings"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"

	"github.com/lsj/copylingo/internal/model"
)

func (b *Bot) handleLLM(
	ctx context.Context,
	msg *tgbotapi.Message,
) {
	messages := botMessagesByLocale[botDefaultLocale]
	if !b.isLLMAllowed(msg.From) {
		slog.WarnContext(
			ctx,
			"Unauthorized LLM command ignored",
			"event",
			"telegram.llm.unauthorized",
			"user_id",
			telegramUserID(msg.From),
			"username",
			telegramUsername(msg.From),
		)
		return
	}
	if b.input == nil {
		b.telegram.SendMessage(
			msg.Chat.ID,
			messages.activationUnavailable,
		)
		return
	}

	if err := b.input.SetLLMPending(
		ctx,
		msg.From.ID,
		model.PendingLLMInput{Kind: model.PendingLLMPlain},
	); err != nil {
		slog.ErrorContext(
			ctx,
			"Failed to activate LLM mode",
			"event",
			"telegram.llm.activate_failed",
			"user_id",
			msg.From.ID,
			"error",
			err,
		)
		b.telegram.SendMessage(
			msg.Chat.ID,
			messages.activationUnavailable,
		)
		return
	}
	b.telegram.SendMessageWithKeyboard(
		msg.Chat.ID,
		messages.modeActivated,
		llmCancelKeyboard(),
	)
}

func llmCancelKeyboard() tgbotapi.InlineKeyboardMarkup {
	messages := botMessagesByLocale[botDefaultLocale]
	return tgbotapi.NewInlineKeyboardMarkup(
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData(
				messages.cancelButton,
				callbackLLMCancel,
			),
		),
	)
}

func (b *Bot) handleLLMCancel(
	ctx context.Context,
	cb *tgbotapi.CallbackQuery,
) {
	messages := botMessagesByLocale[botDefaultLocale]
	if cb == nil || cb.From == nil || cb.Message == nil || cb.Message.Chat == nil {
		return
	}
	if b.input == nil {
		b.telegram.SendMessage(
			cb.Message.Chat.ID,
			messages.cancelUnavailable,
		)
		return
	}

	if err := b.input.DeleteLLMPending(
		ctx,
		cb.From.ID,
	); err != nil {
		slog.ErrorContext(
			ctx,
			"Failed to cancel LLM mode",
			"event",
			"telegram.llm.cancel_failed",
			"user_id",
			cb.From.ID,
			"error",
			err,
		)
		b.telegram.SendMessage(
			cb.Message.Chat.ID,
			messages.cancelUnavailable,
		)
		return
	}
	b.telegram.SendMessage(
		cb.Message.Chat.ID,
		messages.modeCancelled,
	)
}

func (b *Bot) handleLLMQuestion(
	ctx context.Context,
	msg *tgbotapi.Message,
) bool {
	messages := botMessagesByLocale[botDefaultLocale]
	if msg.From == nil || b.input == nil {
		return false
	}
	pendingInput, found, err := b.input.TakeLLMPending(
		ctx,
		msg.From.ID,
	)
	if err != nil || !found {
		return false
	}

	if !b.isLLMAllowed(msg.From) {
		slog.WarnContext(
			ctx,
			"Unauthorized LLM question ignored",
			"event",
			"telegram.llm.question_unauthorized",
			"user_id",
			telegramUserID(msg.From),
			"username",
			telegramUsername(msg.From),
		)
		return true
	}
	question := strings.TrimSpace(msg.Text)
	if question == "" {
		b.telegram.SendMessage(
			msg.Chat.ID,
			messages.emptyQuestion,
		)
		return true
	}
	if b.services == nil || b.services.User == nil || b.services.LLMQuestion == nil {
		b.telegram.SendMessage(
			msg.Chat.ID,
			messages.questionUnavailable,
		)
		return true
	}

	user, err := b.services.User.GetUser(
		ctx,
		msg.From.ID,
		msg.From.UserName,
	)
	if err != nil {
		slog.ErrorContext(
			ctx,
			"Failed to get user for LLM question",
			"event",
			"telegram.llm.user_lookup_failed",
			"user_id",
			msg.From.ID,
			"error",
			err,
		)
		b.telegram.SendMessage(
			msg.Chat.ID,
			messages.userUnavailable,
		)
		return true
	}

	b.telegram.SendMessage(
		msg.Chat.ID,
		messages.answerGenerating,
	)
	// in-quiz "ask" 버튼 경로면 그 문제의 원문/정답/해설/사용자 답을 프롬프트에 실어준다.
	llmPrompt := question
	if quizContext := b.loadQuizQuestionContext(
		ctx,
		pendingInput,
	); quizContext != "" {
		llmPrompt = quizContext + fmt.Sprintf(
			messages.quizQuestionPromptFormat,
			question,
		)
	} else if studyContext := b.loadStudyMaterialContext(
		ctx,
		pendingInput,
		msg.From.ID,
	); studyContext != "" {
		llmPrompt = studyContext + fmt.Sprintf(
			messages.studyQuestionPromptFormat,
			question,
		)
	}
	// The service also keeps the Q&A as a tip candidate for curation.
	answer, err := b.services.LLMQuestion.Answer(
		ctx,
		*user,
		msg.From.UserName,
		llmPrompt,
		question,
	)
	if err != nil {
		slog.ErrorContext(
			ctx,
			"Failed to answer LLM question",
			"event",
			"telegram.llm.answer_failed",
			"user_id",
			user.ID,
			"error",
			err,
		)
		b.telegram.SendMessage(
			msg.Chat.ID,
			messages.answerFailed,
		)
		return true
	}
	b.telegram.SendMessage(
		msg.Chat.ID,
		fmt.Sprintf(
			messages.answerFormat,
			html.EscapeString(answer),
		),
	)
	return true
}

// The Redis adapter turns stored tokens into typed input; invalid tokens remain plain questions.
func (b *Bot) loadQuizQuestionContext(
	ctx context.Context,
	input model.PendingLLMInput,
) string {
	if input.Kind != model.PendingLLMQuizQuestion {
		return ""
	}
	if b.services == nil || b.services.Session == nil {
		return ""
	}
	state, err := b.services.Session.QuizProgress(
		ctx,
		input.SessionID,
	)
	if err != nil {
		return ""
	}
	item, ok := state.ItemByQuestionID(input.QuestionID)
	if !ok {
		return ""
	}
	q := item.Question
	messages := botMessagesByLocale[botDefaultLocale]
	return fmt.Sprintf(
		messages.quizContextFormat,
		stripHTML(q.Prompt),
		q.CorrectAnswer,
		q.Explanation,
		formatSessionAnswer(item.SessionQuestion.UserAnswer),
	)
}

// loadStudyMaterialContext resolves a pending Study Material token into a
// user-owned active-session card. Invalid, stale, or completed session tokens
// intentionally fall back to a plain LLM question.
func (b *Bot) loadStudyMaterialContext(
	ctx context.Context,
	input model.PendingLLMInput,
	userID int64,
) string {
	if input.Kind != model.PendingLLMStudyMaterial || b.services == nil || b.services.Session == nil {
		return ""
	}

	state, err := b.services.Session.StudyProgress(
		ctx,
		input.SessionID,
		userID,
	)
	if err != nil || state.Session.Status == model.SessionCompleted {
		return ""
	}
	item, idx, ok := state.ItemByOrder(input.MaterialOrder)
	if !ok {
		return ""
	}
	materialText := stripHTML(renderStudyMaterial(
		item.Material,
		idx,
		len(state.Items),
	))
	messages := botMessagesByLocale[botDefaultLocale]
	return fmt.Sprintf(
		messages.studyContextFormat,
		item.Material.Category,
		item.Material.Title,
		materialText,
	)
}

func (b *Bot) isLLMAllowed(from *tgbotapi.User) bool {
	if from == nil {
		return false
	}
	for _, allowedUserID := range llmAllowedTelegramUserIDs {
		if from.ID == allowedUserID {
			return true
		}
	}
	return false
}

func telegramUserID(from *tgbotapi.User) int64 {
	if from == nil {
		return 0
	}
	return from.ID
}

func telegramUsername(from *tgbotapi.User) string {
	if from == nil {
		return ""
	}
	return from.UserName
}
