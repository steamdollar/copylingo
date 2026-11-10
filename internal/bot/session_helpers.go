package bot

import (
	"strings"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"

	"github.com/lsj/copylingo/internal/model"
)

func (flow *SessionFlow) showSessionFetchError(callbackQuery *tgbotapi.CallbackQuery) {
	flow.telegram.EditMessage(
		callbackQuery.Message.Chat.ID,
		callbackQuery.Message.MessageID,
		botMessagesByLocale[botDefaultLocale].sessionFetchFailed,
		mainMenuKeyboard(),
	)
}

func (flow *SessionFlow) showQuizActiveSessionUnavailable(
	chatID int64,
	editMessageID *int,
) {
	text := botMessagesByLocale[botDefaultLocale].activeSessionUnavailable
	if editMessageID != nil {
		flow.telegram.EditMessage(
			chatID,
			*editMessageID,
			text,
			nil,
		)
		return
	}
	flow.telegram.SendMessage(
		chatID,
		text,
	)
}

func mainMenuKeyboard() *tgbotapi.InlineKeyboardMarkup {
	keyboard := tgbotapi.NewInlineKeyboardMarkup(
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData(
				botMessagesByLocale[botDefaultLocale].menuHomeButton,
				callbackMenuMain,
			),
		),
	)
	return &keyboard
}

func formatSessionAnswer(userAnswer *string) string {
	if userAnswer == nil || *userAnswer == "" {
		return botMessagesByLocale[botDefaultLocale].unansweredLabel
	}
	return *userAnswer
}

func sessionTypeLabel(sessionType string) string {
	messages := botMessagesByLocale[botDefaultLocale]
	switch sessionType {
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
		return sessionType
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
	text string,
	maxLen int,
) string {
	runes := []rune(text)
	if len(runes) <= maxLen {
		return text
	}
	return string(runes[:maxLen]) + "..."
}

func stripHTML(text string) string {
	// Simple tag stripping to avoid Telegram API errors on truncated HTML
	var result strings.Builder
	inTag := false
	for _, character := range text {
		if character == '<' {
			inTag = true
			continue
		}
		if character == '>' {
			inTag = false
			continue
		}
		if !inTag {
			result.WriteRune(character)
		}
	}
	return result.String()
}
