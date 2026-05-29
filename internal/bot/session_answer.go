package bot

import (
	"context"
	"errors"
	"fmt"
	"log"
	"strconv"
	"strings"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
	"github.com/lsj/copylingo/internal/config"
	"github.com/lsj/copylingo/internal/model"
	"github.com/lsj/copylingo/internal/service"
)

func (sf *SessionFlow) processAnswer(ctx context.Context, cb *tgbotapi.CallbackQuery, sessionID, questionID int, optionIdx int) {
	state, err := sf.bot.services.ActiveSession.Get(ctx, sessionID)
	if err != nil {
		return
	}
	item, _, ok := state.FindItemByQuestionID(questionID)
	if !ok {
		return
	}
	question := item.Question

	options, err := question.GetOptions()
	if err != nil || optionIdx >= len(options) {
		return
	}

	selectedAnswer := options[optionIdx]
	editMessageID := cb.Message.MessageID
	sf.processAnswerText(ctx, cb.Message.Chat.ID, sessionID, questionID, selectedAnswer, &editMessageID)
}

// HandleTextInput intercepts text messages if there is an active text question.
func (sf *SessionFlow) HandleTextInput(ctx context.Context, msg *tgbotapi.Message) bool {
	key := config.UserActiveQuestionRedisKey.Format(msg.Chat.ID)
	activeQuestionState, err := sf.bot.rdb.Get(ctx, key).Result()
	if err != nil {
		return false
	}

	parts := strings.Split(activeQuestionState, ":")
	if len(parts) != 2 {
		return false
	}
	sessionID, _ := strconv.Atoi(parts[0])
	questionIdx, _ := strconv.Atoi(parts[1])

	sf.bot.rdb.Del(ctx, key)

	state, err := sf.bot.services.ActiveSession.Get(ctx, sessionID)
	if err != nil || questionIdx >= len(state.Items) {
		return false
	}
	questionID := state.Items[questionIdx].SessionQuestion.QuestionID

	sf.processAnswerText(ctx, msg.Chat.ID, sessionID, questionID, strings.TrimSpace(msg.Text), nil)
	return true
}

func (sf *SessionFlow) processAnswerText(ctx context.Context, chatID int64, sessionID, questionID int, selectedAnswer string, editMessageID *int) {
	state, err := sf.bot.services.ActiveSession.Get(ctx, sessionID)
	if err != nil {
		log.Printf("Error getting active session for answer: %v", err)
		sf.showActiveSessionUnavailable(chatID, editMessageID)
		return
	}
	item, currentIdx, ok := state.FindItemByQuestionID(questionID)
	if !ok {
		log.Printf("Question not found in active session session=%d question=%d", sessionID, questionID)
		return
	}
	if item.SessionQuestion.IsCorrect != nil {
		sf.bot.SendMessage(chatID, "이미 답변한 문제입니다.")
		return
	}
	question := item.Question

	// Grade the answer
	switch question.Type {
	case model.QuestionFillBlank:
		selectedAnswer = strings.ToLower(selectedAnswer) // For Kana fill in the blank
	case model.QuestionSubjective:
		// Show typing status for AI grading UX
		sf.bot.api.Request(tgbotapi.NewChatAction(chatID, tgbotapi.ChatTyping))
	}

	isCorrect, feedback, err := sf.bot.services.Grader.GradeAnswerWithQuestion(ctx, sessionID, questionID, &question, selectedAnswer)
	if err != nil {
		if errors.Is(err, config.ErrAIConfigMissing) {
			errMsg := tgbotapi.NewMessage(chatID, "⚠️ 시스템 설정 문제로 현재 AI 주관식 채점이 불가능합니다. 임시로 오답 처리하고 넘어갑니다.")
			sf.bot.api.Send(errMsg)
			isCorrect = false
			if recordErr := sf.bot.services.ActiveSession.RecordAnswer(ctx, sessionID, questionID, selectedAnswer, false); recordErr != nil {
				log.Printf("Error recording fallback wrong answer: %v", recordErr)
				return
			}
		} else if errors.Is(err, service.ErrActiveSessionAlreadyAnswered) {
			sf.bot.SendMessage(chatID, "이미 답변한 문제입니다.")
			return
		} else {
			log.Printf("Error grading answer: %v", err)
			return
		}
	}

	var text string
	if isCorrect {
		text = fmt.Sprintf("✅ <b>정답!</b>\n\n%s", question.Explanation)
	} else {
		text = fmt.Sprintf("❌ <b>오답</b>\n\n입력/선택: %s\n정답: <b>%s</b>\n\n%s",
			selectedAnswer, question.CorrectAnswer, question.Explanation)
	}

	if feedback != "" {
		text += fmt.Sprintf("\n\n🤖 <b>AI 피드백:</b>\n%s", feedback)
	}

	nextLabel := "다음 문제 →"
	if currentIdx+1 >= len(state.Items) {
		nextLabel = "📊 결과 보기"
	}

	var nextData string
	if currentIdx+1 >= len(state.Items) {
		nextData = fmt.Sprintf(config.FormatSessionFinish, sessionID)
	} else {
		nextData = fmt.Sprintf(config.FormatQuestionNext, sessionID, currentIdx)
	}

	keyboard := tgbotapi.NewInlineKeyboardMarkup(
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData(nextLabel, nextData),
		),
	)

	if editMessageID != nil {
		sf.bot.EditMessage(chatID, *editMessageID, text, &keyboard)
	} else {
		// 텍스트 답변은 사용자 메시지로 들어오므로 편집할 봇 문제 메시지가 없다.
		sf.bot.SendMessageWithKeyboard(chatID, text, keyboard)
	}
}
