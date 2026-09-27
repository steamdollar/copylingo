package bot

import (
	"context"
	"fmt"
	"html"
	"log/slog"
	"strings"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"

	"github.com/lsj/copylingo/internal/config"
	"github.com/lsj/copylingo/internal/model"
)

func (b *Bot) handleLLM(ctx context.Context, msg *tgbotapi.Message) {
	if !b.isLLMAllowed(msg.From) {
		slog.WarnContext(ctx, "Unauthorized LLM command ignored",
			"event", "telegram.llm.unauthorized",
			"user_id", telegramUserID(msg.From),
			"username", telegramUsername(msg.From),
		)
		return
	}
	if b.input == nil {
		b.SendMessage(msg.Chat.ID, "❌ LLM mode를 활성화할 수 없습니다.")
		return
	}

	if err := b.input.SetLLMPending(ctx, msg.From.ID, model.PendingLLMInput{Kind: model.PendingLLMPlain}); err != nil {
		slog.ErrorContext(ctx, "Failed to activate LLM mode",
			"event", "telegram.llm.activate_failed",
			"user_id", msg.From.ID,
			"error", err,
		)
		b.SendMessage(msg.Chat.ID, "❌ LLM mode를 활성화할 수 없습니다.")
		return
	}
	b.SendMessageWithKeyboard(msg.Chat.ID,
		"🤖 <b>LLM mode 활성화</b>\n질문을 입력해 주세요. 다음 메시지 1개를 AI에게 보냅니다.", llmCancelKeyboard())
}

func llmCancelKeyboard() tgbotapi.InlineKeyboardMarkup {
	return tgbotapi.NewInlineKeyboardMarkup(
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData("❌ 취소", config.ActionLLMCancel),
		),
	)
}

func (b *Bot) handleLLMCancel(ctx context.Context, cb *tgbotapi.CallbackQuery) {
	if cb == nil || cb.From == nil || cb.Message == nil || cb.Message.Chat == nil {
		return
	}
	if b.input == nil {
		b.SendMessage(cb.Message.Chat.ID, "❌ LLM mode를 취소할 수 없습니다.")
		return
	}

	if err := b.input.DeleteLLMPending(ctx, cb.From.ID); err != nil {
		slog.ErrorContext(ctx, "Failed to cancel LLM mode",
			"event", "telegram.llm.cancel_failed",
			"user_id", cb.From.ID,
			"error", err,
		)
		b.SendMessage(cb.Message.Chat.ID, "❌ LLM mode를 취소할 수 없습니다.")
		return
	}
	b.SendMessage(cb.Message.Chat.ID, "✅ LLM mode를 취소했습니다.")
}

func (b *Bot) handleLLMQuestion(ctx context.Context, msg *tgbotapi.Message) bool {
	if msg.From == nil || b.input == nil {
		return false
	}
	pendingInput, found, err := b.input.TakeLLMPending(ctx, msg.From.ID)
	if err != nil || !found {
		return false
	}

	if !b.isLLMAllowed(msg.From) {
		slog.WarnContext(ctx, "Unauthorized LLM question ignored",
			"event", "telegram.llm.question_unauthorized",
			"user_id", telegramUserID(msg.From),
			"username", telegramUsername(msg.From),
		)
		return true
	}
	question := strings.TrimSpace(msg.Text)
	if question == "" {
		b.SendMessage(msg.Chat.ID, "⚠️ 질문 내용이 비어 있습니다. /llm 으로 다시 시작해 주세요.")
		return true
	}
	if b.services == nil || b.services.User == nil || b.services.LLM == nil || b.services.Tip == nil {
		b.SendMessage(msg.Chat.ID, "❌ LLM 질문 기능이 준비되지 않았습니다.")
		return true
	}

	user, err := b.services.User.GetUser(ctx, msg.From.ID, msg.From.UserName)
	if err != nil {
		slog.ErrorContext(ctx, "Failed to get user for LLM question",
			"event", "telegram.llm.user_lookup_failed",
			"user_id", msg.From.ID,
			"error", err,
		)
		b.SendMessage(msg.Chat.ID, "❌ 사용자 정보를 확인할 수 없습니다.")
		return true
	}

	b.SendMessage(msg.Chat.ID, "🤖 AI가 답변을 생성 중입니다...")
	// in-quiz "ask" 버튼 경로면 그 문제의 원문/정답/해설/사용자 답을 프롬프트에 실어준다.
	llmPrompt := question
	if quizContext := b.loadQuizQuestionContext(ctx, pendingInput); quizContext != "" {
		llmPrompt = quizContext + "\n\n위 문제에 대한 사용자의 질문에 답해 주세요.\n[질문] " + question
	} else if studyContext := b.loadStudyMaterialContext(ctx, pendingInput, msg.From.ID); studyContext != "" {
		llmPrompt = studyContext + "\n\n위 Study Material에 대한 사용자의 질문에 답해 주세요.\n[질문] " + question
	}
	answer, err := b.services.LLM.AnswerLearningQuestion(ctx, llmPrompt)
	if err != nil {
		slog.ErrorContext(ctx, "Failed to answer LLM question",
			"event", "telegram.llm.answer_failed",
			"user_id", user.ID,
			"error", err,
		)
		b.SendMessage(msg.Chat.ID, "❌ AI 답변 생성 중 오류가 발생했습니다. 다시 질문하려면 /llm 을 입력해 주세요.")
		return true
	}
	b.createTipCandidate(ctx, user, msg.From.UserName, question, answer)

	b.SendMessage(msg.Chat.ID, "<b>AI 답변</b>\n\n"+html.EscapeString(answer))
	return true
}

// The Redis adapter turns stored tokens into typed input; invalid tokens remain plain questions.
func (b *Bot) loadQuizQuestionContext(ctx context.Context, input model.PendingLLMInput) string {
	if input.Kind != model.PendingLLMQuizQuestion {
		return ""
	}
	if b.services == nil || b.services.QuizActiveSession == nil {
		return ""
	}
	state, err := b.services.QuizActiveSession.Get(ctx, input.SessionID)
	if err != nil {
		return ""
	}
	item, ok := state.ItemByQuestionID(input.QuestionID)
	if !ok {
		return ""
	}
	q := item.Question
	return fmt.Sprintf(`다음은 사용자가 방금 푼 일본어 학습 문제입니다.
[문제] %s
[정답] %s
[해설] %s
[사용자가 제출한 답] %s`,
		stripHTML(q.Prompt), q.CorrectAnswer, q.Explanation,
		formatSessionAnswer(item.SessionQuestion.UserAnswer))
}

// loadStudyMaterialContext resolves a pending Study Material token into a
// user-owned active-session card. Invalid, stale, or completed session tokens
// intentionally fall back to a plain LLM question.
func (b *Bot) loadStudyMaterialContext(ctx context.Context, input model.PendingLLMInput, userID int64) string {
	if input.Kind != model.PendingLLMStudyMaterial || b.services == nil || b.services.StudyActiveSession == nil {
		return ""
	}

	state, err := b.services.StudyActiveSession.GetOwned(ctx, input.SessionID, userID)
	if err != nil || state.Session.Status == model.SessionCompleted {
		return ""
	}
	item, idx, ok := state.ItemByOrder(input.MaterialOrder)
	if !ok {
		return ""
	}
	materialText := stripHTML(renderStudyMaterial(item.Material, idx, len(state.Items)))
	return fmt.Sprintf(`다음은 사용자가 방금 보고 있는 일본어 Study Material입니다.
[카테고리] %s
[제목] %s
[학습 내용] %s`, item.Material.Category, item.Material.Title, materialText)
}

func (b *Bot) createTipCandidate(ctx context.Context, user *model.User, username, question, answer string) {
	if b.services == nil || b.services.Tip == nil {
		return
	}

	var sourceModel *string
	if b.cfg != nil && strings.TrimSpace(b.cfg.LLM.Model) != "" {
		modelName := b.cfg.LLM.Model
		sourceModel = &modelName
	}
	candidate := &model.TipCandidate{
		UserID:           user.ID,
		Username:         username,
		Language:         user.Language,
		ProficiencyLevel: user.ProficiencyLevel,
		Question:         question,
		Answer:           answer,
		SourceModel:      sourceModel,
	}
	if err := b.services.Tip.CreateCandidate(ctx, candidate); err != nil {
		slog.ErrorContext(ctx, "Failed to create tip candidate",
			"event", "telegram.llm.tip_candidate_create_failed",
			"user_id", user.ID,
			"language", user.Language,
			"level", user.ProficiencyLevel,
			"error", err,
		)
	}
}

func (b *Bot) isLLMAllowed(from *tgbotapi.User) bool {
	if from == nil {
		return false
	}
	for _, allowedUserID := range config.LLMAllowedTelegramUserIDs {
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
