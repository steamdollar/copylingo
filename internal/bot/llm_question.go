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

// llmQuestionAnswerer answers a learning question; the service also keeps the
// Q&A as a tip candidate.
type llmQuestionAnswerer interface {
	Answer(
		ctx context.Context,
		user model.User,
		username,
		prompt,
		question string,
	) (string, error)
}

// llmContextSession reads the Quiz question or Study card that an in-session
// "ask the LLM" token points at, to prepend it to the prompt.
type llmContextSession interface {
	QuizProgress(
		ctx context.Context,
		sessionID int,
	) (*model.QuizActiveSessionState, error)
	StudyProgress(
		ctx context.Context,
		sessionID int,
		userID int64,
	) (*model.StudyActiveSessionState, error)
}

// llmInputStore holds the one-shot "next message is an LLM question" token.
type llmInputStore interface {
	SetLLMPending(
		ctx context.Context,
		userID int64,
		input model.PendingLLMInput,
	) error
	TakeLLMPending(
		ctx context.Context,
		userID int64,
	) (model.PendingLLMInput, bool, error)
	DeleteLLMPending(
		ctx context.Context,
		userID int64,
	) error
}

// LLMQuestionFlowDependencies wires LLMQuestionFlow; every dependency is required.
type LLMQuestionFlowDependencies struct {
	Telegram    *TelegramClient
	User        userReader
	LLMQuestion llmQuestionAnswerer
	Session     llmContextSession
	Input       llmInputStore
}

// LLMQuestionFlow handles the owner-only /llm mode: arming it, cancelling it,
// and answering the next message with optional Quiz/Study context.
type LLMQuestionFlow struct {
	telegram    *TelegramClient
	user        userReader
	llmQuestion llmQuestionAnswerer
	session     llmContextSession
	input       llmInputStore
}

func NewLLMQuestionFlow(dependencies LLMQuestionFlowDependencies) *LLMQuestionFlow {
	return &LLMQuestionFlow{
		telegram:    dependencies.Telegram,
		user:        dependencies.User,
		llmQuestion: dependencies.LLMQuestion,
		session:     dependencies.Session,
		input:       dependencies.Input,
	}
}

func (flow *LLMQuestionFlow) handleLLM(
	ctx context.Context,
	message *tgbotapi.Message,
) {
	messages := botMessagesByLocale[botDefaultLocale]
	if !isLLMAllowed(message.From) {
		slog.WarnContext(
			ctx,
			"Unauthorized LLM command ignored",
			"event",
			"telegram.llm.unauthorized",
			"user_id",
			telegramUserID(message.From),
			"username",
			telegramUsername(message.From),
		)
		return
	}

	if err := flow.input.SetLLMPending(
		ctx,
		message.From.ID,
		model.PendingLLMInput{Kind: model.PendingLLMPlain},
	); err != nil {
		slog.ErrorContext(
			ctx,
			"Failed to activate LLM mode",
			"event",
			"telegram.llm.activate_failed",
			"user_id",
			message.From.ID,
			"error",
			err,
		)
		flow.telegram.SendMessage(
			message.Chat.ID,
			messages.activationUnavailable,
		)
		return
	}
	flow.telegram.SendMessageWithKeyboard(
		message.Chat.ID,
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

func (flow *LLMQuestionFlow) handleLLMCancel(
	ctx context.Context,
	callbackQuery *tgbotapi.CallbackQuery,
) {
	messages := botMessagesByLocale[botDefaultLocale]
	if callbackQuery == nil || callbackQuery.From == nil || callbackQuery.Message == nil ||
		callbackQuery.Message.Chat == nil {
		return
	}
	if err := flow.input.DeleteLLMPending(
		ctx,
		callbackQuery.From.ID,
	); err != nil {
		slog.ErrorContext(
			ctx,
			"Failed to cancel LLM mode",
			"event",
			"telegram.llm.cancel_failed",
			"user_id",
			callbackQuery.From.ID,
			"error",
			err,
		)
		flow.telegram.SendMessage(
			callbackQuery.Message.Chat.ID,
			messages.cancelUnavailable,
		)
		return
	}
	flow.telegram.SendMessage(
		callbackQuery.Message.Chat.ID,
		messages.modeCancelled,
	)
}

func (flow *LLMQuestionFlow) handleLLMQuestion(
	ctx context.Context,
	message *tgbotapi.Message,
) bool {
	messages := botMessagesByLocale[botDefaultLocale]
	if message.From == nil {
		return false
	}
	pendingInput, found, err := flow.input.TakeLLMPending(
		ctx,
		message.From.ID,
	)
	if err != nil || !found {
		return false
	}

	if !isLLMAllowed(message.From) {
		slog.WarnContext(
			ctx,
			"Unauthorized LLM question ignored",
			"event",
			"telegram.llm.question_unauthorized",
			"user_id",
			telegramUserID(message.From),
			"username",
			telegramUsername(message.From),
		)
		return true
	}
	question := strings.TrimSpace(message.Text)
	if question == "" {
		flow.telegram.SendMessage(
			message.Chat.ID,
			messages.emptyQuestion,
		)
		return true
	}
	user, err := flow.user.GetUser(
		ctx,
		message.From.ID,
		message.From.UserName,
	)
	if err != nil {
		slog.ErrorContext(
			ctx,
			"Failed to get user for LLM question",
			"event",
			"telegram.llm.user_lookup_failed",
			"user_id",
			message.From.ID,
			"error",
			err,
		)
		flow.telegram.SendMessage(
			message.Chat.ID,
			messages.userUnavailable,
		)
		return true
	}

	flow.telegram.SendMessage(
		message.Chat.ID,
		messages.answerGenerating,
	)
	// in-quiz "ask" 버튼 경로면 그 문제의 원문/정답/해설/사용자 답을 프롬프트에 실어준다.
	llmPrompt := question
	if quizContext := flow.loadQuizQuestionContext(
		ctx,
		pendingInput,
	); quizContext != "" {
		llmPrompt = quizContext + fmt.Sprintf(
			messages.quizQuestionPromptFormat,
			question,
		)
	} else if studyContext := flow.loadStudyMaterialContext(
		ctx,
		pendingInput,
		message.From.ID,
	); studyContext != "" {
		llmPrompt = studyContext + fmt.Sprintf(
			messages.studyQuestionPromptFormat,
			question,
		)
	}
	// The service also keeps the Q&A as a tip candidate for curation.
	answer, err := flow.llmQuestion.Answer(
		ctx,
		*user,
		message.From.UserName,
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
		flow.telegram.SendMessage(
			message.Chat.ID,
			messages.answerFailed,
		)
		return true
	}
	flow.telegram.SendMessage(
		message.Chat.ID,
		fmt.Sprintf(
			messages.answerFormat,
			html.EscapeString(answer),
		),
	)
	return true
}

// The Redis adapter turns stored tokens into typed input; invalid tokens remain plain questions.
func (flow *LLMQuestionFlow) loadQuizQuestionContext(
	ctx context.Context,
	input model.PendingLLMInput,
) string {
	if input.Kind != model.PendingLLMQuizQuestion {
		return ""
	}
	state, err := flow.session.QuizProgress(
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
	question := item.Question
	messages := botMessagesByLocale[botDefaultLocale]
	return fmt.Sprintf(
		messages.quizContextFormat,
		stripHTML(question.Prompt),
		question.CorrectAnswer,
		question.Explanation,
		formatSessionAnswer(item.SessionQuestion.UserAnswer),
	)
}

// loadStudyMaterialContext resolves a pending Study Material token into a
// user-owned active-session card. Invalid, stale, or completed session tokens
// intentionally fall back to a plain LLM question.
func (flow *LLMQuestionFlow) loadStudyMaterialContext(
	ctx context.Context,
	input model.PendingLLMInput,
	userID int64,
) string {
	if input.Kind != model.PendingLLMStudyMaterial {
		return ""
	}

	state, err := flow.session.StudyProgress(
		ctx,
		input.SessionID,
		userID,
	)
	if err != nil || state.Session.Status == model.SessionCompleted {
		return ""
	}
	item, index, ok := state.ItemByOrder(input.MaterialOrder)
	if !ok {
		return ""
	}
	materialText := stripHTML(renderStudyMaterial(
		item.Material,
		index,
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

// isLLMAllowed gates the owner-only LLM features to the allowlisted Telegram users.
func isLLMAllowed(from *tgbotapi.User) bool {
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
