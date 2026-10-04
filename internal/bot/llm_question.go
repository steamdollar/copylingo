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

// LLMQuestionFlowDeps wires LLMQuestionFlow; every dependency is required.
type LLMQuestionFlowDeps struct {
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

func NewLLMQuestionFlow(deps LLMQuestionFlowDeps) *LLMQuestionFlow {
	return &LLMQuestionFlow{
		telegram:    deps.Telegram,
		user:        deps.User,
		llmQuestion: deps.LLMQuestion,
		session:     deps.Session,
		input:       deps.Input,
	}
}

func (lf *LLMQuestionFlow) handleLLM(
	ctx context.Context,
	msg *tgbotapi.Message,
) {
	messages := botMessagesByLocale[botDefaultLocale]
	if !isLLMAllowed(msg.From) {
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
	if lf.input == nil {
		lf.telegram.SendMessage(
			msg.Chat.ID,
			messages.activationUnavailable,
		)
		return
	}

	if err := lf.input.SetLLMPending(
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
		lf.telegram.SendMessage(
			msg.Chat.ID,
			messages.activationUnavailable,
		)
		return
	}
	lf.telegram.SendMessageWithKeyboard(
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

func (lf *LLMQuestionFlow) handleLLMCancel(
	ctx context.Context,
	cb *tgbotapi.CallbackQuery,
) {
	messages := botMessagesByLocale[botDefaultLocale]
	if cb == nil || cb.From == nil || cb.Message == nil || cb.Message.Chat == nil {
		return
	}
	if lf.input == nil {
		lf.telegram.SendMessage(
			cb.Message.Chat.ID,
			messages.cancelUnavailable,
		)
		return
	}

	if err := lf.input.DeleteLLMPending(
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
		lf.telegram.SendMessage(
			cb.Message.Chat.ID,
			messages.cancelUnavailable,
		)
		return
	}
	lf.telegram.SendMessage(
		cb.Message.Chat.ID,
		messages.modeCancelled,
	)
}

func (lf *LLMQuestionFlow) handleLLMQuestion(
	ctx context.Context,
	msg *tgbotapi.Message,
) bool {
	messages := botMessagesByLocale[botDefaultLocale]
	if msg.From == nil || lf.input == nil {
		return false
	}
	pendingInput, found, err := lf.input.TakeLLMPending(
		ctx,
		msg.From.ID,
	)
	if err != nil || !found {
		return false
	}

	if !isLLMAllowed(msg.From) {
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
		lf.telegram.SendMessage(
			msg.Chat.ID,
			messages.emptyQuestion,
		)
		return true
	}
	user, err := lf.user.GetUser(
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
		lf.telegram.SendMessage(
			msg.Chat.ID,
			messages.userUnavailable,
		)
		return true
	}

	lf.telegram.SendMessage(
		msg.Chat.ID,
		messages.answerGenerating,
	)
	// in-quiz "ask" 버튼 경로면 그 문제의 원문/정답/해설/사용자 답을 프롬프트에 실어준다.
	llmPrompt := question
	if quizContext := lf.loadQuizQuestionContext(
		ctx,
		pendingInput,
	); quizContext != "" {
		llmPrompt = quizContext + fmt.Sprintf(
			messages.quizQuestionPromptFormat,
			question,
		)
	} else if studyContext := lf.loadStudyMaterialContext(
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
	answer, err := lf.llmQuestion.Answer(
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
		lf.telegram.SendMessage(
			msg.Chat.ID,
			messages.answerFailed,
		)
		return true
	}
	lf.telegram.SendMessage(
		msg.Chat.ID,
		fmt.Sprintf(
			messages.answerFormat,
			html.EscapeString(answer),
		),
	)
	return true
}

// The Redis adapter turns stored tokens into typed input; invalid tokens remain plain questions.
func (lf *LLMQuestionFlow) loadQuizQuestionContext(
	ctx context.Context,
	input model.PendingLLMInput,
) string {
	if input.Kind != model.PendingLLMQuizQuestion {
		return ""
	}
	state, err := lf.session.QuizProgress(
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
func (lf *LLMQuestionFlow) loadStudyMaterialContext(
	ctx context.Context,
	input model.PendingLLMInput,
	userID int64,
) string {
	if input.Kind != model.PendingLLMStudyMaterial {
		return ""
	}

	state, err := lf.session.StudyProgress(
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
