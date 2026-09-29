package bot

import (
	"strings"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"

	"github.com/lsj/copylingo/internal/model"
)

func (sf *SessionFlow) showSessionFetchError(cb *tgbotapi.CallbackQuery) {
	sf.bot.EditMessage(
		cb.Message.Chat.ID,
		cb.Message.MessageID,
		botMessagesByLocale[botDefaultLocale].sessionFetchFailed,
		mainMenuKeyboard(),
	)
}

func (sf *SessionFlow) showQuizActiveSessionUnavailable(
	chatID int64,
	editMessageID *int,
) {
	text := botMessagesByLocale[botDefaultLocale].activeSessionUnavailable
	if editMessageID != nil {
		sf.bot.EditMessage(
			chatID,
			*editMessageID,
			text,
			nil,
		)
		return
	}
	sf.bot.SendMessage(
		chatID,
		text,
	)
}

func mainMenuKeyboard() *tgbotapi.InlineKeyboardMarkup {
	kb := tgbotapi.NewInlineKeyboardMarkup(
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData(
				botMessagesByLocale[botDefaultLocale].menuHomeButton,
				callbackMenuMain,
			),
		),
	)
	return &kb
}

func formatSessionAnswer(userAnswer *string) string {
	if userAnswer == nil || *userAnswer == "" {
		return botMessagesByLocale[botDefaultLocale].unansweredLabel
	}
	return *userAnswer
}

func sessionTypeLabel(t string) string {
	messages := botMessagesByLocale[botDefaultLocale]
	switch t {
	case "morning":
		return messages.morningStudySessionLabel
	case "evening":
		return messages.eveningReviewSessionLabel
	case "review":
		return messages.reviewSessionLabel
	case "article":
		return messages.articleSessionLabel
	case "study":
		return messages.middayStudySessionLabel
	default:
		return t
	}
}

func firstQuizSession(sessions []model.Session) (model.Session, bool) {
	for _, session := range sessions {
		if session.Mode == "" || session.Mode == model.SessionModeQuiz {
			return session, true
		}
	}
	return model.Session{}, false
}

func firstStudySession(sessions []model.Session) (model.Session, bool) {
	for _, session := range sessions {
		if session.Mode == model.SessionModeStudy {
			return session, true
		}
	}
	return model.Session{}, false
}

func truncate(
	s string,
	maxLen int,
) string {
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
