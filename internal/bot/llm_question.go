package bot

import (
	"context"
	"html"
	"log/slog"
	"strings"
	"time"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"

	"github.com/lsj/copylingo/internal/config"
	"github.com/lsj/copylingo/internal/model"
)

const llmModeTTL = 10 * time.Minute

func (b *Bot) handleLLM(ctx context.Context, msg *tgbotapi.Message) {
	if !b.isLLMAllowed(msg.From) {
		slog.WarnContext(ctx, "Unauthorized LLM command ignored",
			"event", "telegram.llm.unauthorized",
			"user_id", telegramUserID(msg.From),
			"username", telegramUsername(msg.From),
		)
		return
	}
	if b.rdb == nil {
		b.SendMessage(msg.Chat.ID, "❌ LLM mode를 활성화할 수 없습니다.")
		return
	}

	key := config.UserLLMPendingRedisKey.Format(msg.From.ID)
	if err := b.rdb.Set(ctx, key, "1", llmModeTTL).Err(); err != nil {
		slog.ErrorContext(ctx, "Failed to activate LLM mode",
			"event", "telegram.llm.activate_failed",
			"user_id", msg.From.ID,
			"error", err,
		)
		b.SendMessage(msg.Chat.ID, "❌ LLM mode를 활성화할 수 없습니다.")
		return
	}
	b.SendMessage(msg.Chat.ID, "🤖 <b>LLM mode 활성화</b>\n질문을 입력해 주세요. 다음 메시지 1개를 AI에게 보냅니다.")
}

func (b *Bot) handleLLMQuestion(ctx context.Context, msg *tgbotapi.Message) bool {
	if msg.From == nil || b.rdb == nil {
		return false
	}
	key := config.UserLLMPendingRedisKey.Format(msg.From.ID)
	if err := b.rdb.GetDel(ctx, key).Err(); err != nil {
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
	answer, err := b.services.LLM.AnswerLearningQuestion(ctx, question)
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
