package bot

import (
	"strings"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
	"github.com/lsj/copylingo/internal/config"
)

func (sf *SessionFlow) showSessionFetchError(cb *tgbotapi.CallbackQuery) {
	sf.bot.EditMessage(cb.Message.Chat.ID, cb.Message.MessageID,
		"❌ 세션 정보를 불러오지 못했습니다. 잠시 후 다시 시도해 주세요.",
		mainMenuKeyboard(),
	)
}

func (sf *SessionFlow) showActiveSessionUnavailable(chatID int64, editMessageID *int) {
	text := "⚠️ 진행 중 세션 상태가 만료되었습니다. 새 세션을 다시 시작해 주세요."
	if editMessageID != nil {
		sf.bot.EditMessage(chatID, *editMessageID, text, nil)
		return
	}
	sf.bot.SendMessage(chatID, text)
}

func mainMenuKeyboard() *tgbotapi.InlineKeyboardMarkup {
	kb := tgbotapi.NewInlineKeyboardMarkup(
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData("🏠 메뉴로", config.ActionMenuMain),
		),
	)
	return &kb
}

func formatSessionAnswer(userAnswer *string) string {
	if userAnswer == nil || *userAnswer == "" {
		return "미응답"
	}
	return *userAnswer
}

func sessionTypeLabel(t string) string {
	switch t {
	case "morning":
		return "🌅 오전 학습"
	case "evening":
		return "🌙 오후 복습"
	case "review":
		return "🔄 복습"
	case "article":
		return "📖 아티클"
	default:
		return t
	}
}

func truncate(s string, maxLen int) string {
	runes := []rune(s)
	if len(runes) <= maxLen {
		return s
	}
	return string(runes[:maxLen]) + "..."
}

func stripHTML(s string) string {
	// Simple tag stripping to avoid Telegram API errors on truncated HTML
	var result strings.Builder
	inTag := false
	for _, r := range s {
		if r == '<' {
			inTag = true
			continue
		}
		if r == '>' {
			inTag = false
			continue
		}
		if !inTag {
			result.WriteRune(r)
		}
	}
	return result.String()
}
