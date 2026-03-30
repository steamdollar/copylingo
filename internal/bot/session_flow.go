package bot

import (
	"context"
	"fmt"
	"log"
	"strconv"
	"strings"
	"time"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
)

// SessionFlow handles the question-answering interaction flow.
type SessionFlow struct {
	bot *Bot
}

func NewSessionFlow(bot *Bot) *SessionFlow {
	return &SessionFlow{bot: bot}
}

// StartStudy begins a new study session or resumes a pending one.
func (f *SessionFlow) StartStudy(ctx context.Context, cb *tgbotapi.CallbackQuery) {
	userID := cb.From.ID
	chatID := cb.Message.Chat.ID

	// Check for pending sessions
	sessions, err := f.bot.services.SessionBuilder.GetPendingSessions(ctx, userID)
	if err != nil || len(sessions) == 0 {
		f.bot.EditMessage(chatID, cb.Message.MessageID,
			"📚 현재 대기 중인 학습 세션이 없습니다.\n다음 세션이 자동으로 전송될 예정입니다!",
			&tgbotapi.InlineKeyboardMarkup{
				InlineKeyboard: [][]tgbotapi.InlineKeyboardButton{
					{tgbotapi.NewInlineKeyboardButtonData("🏠 메뉴로", "menu:main")},
				},
			},
		)
		return
	}

	session := sessions[0]

	keyboard := tgbotapi.NewInlineKeyboardMarkup(
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData("▶️ 시작하기", fmt.Sprintf("session:%d:start", session.ID)),
		),
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData("🏠 메뉴로", "menu:main"),
		),
	)

	text := fmt.Sprintf("📚 <b>학습 세션 준비됨</b>\n\n총 %d문제\n유형: %s\n\n준비되면 시작 버튼을 누르세요!",
		session.TotalQuestions, sessionTypeLabel(string(session.Type)))

	f.bot.EditMessage(chatID, cb.Message.MessageID, text, &keyboard)
}

// StartReview starts an on-demand review session.
func (f *SessionFlow) StartReview(ctx context.Context, cb *tgbotapi.CallbackQuery) {
	userID := cb.From.ID
	chatID := cb.Message.Chat.ID

	count, _ := f.bot.services.SRS.GetDueCount(ctx)
	if count == 0 {
		f.bot.EditMessage(chatID, cb.Message.MessageID,
			"✅ 복습할 문제가 없습니다! 훌륭합니다 🎉",
			&tgbotapi.InlineKeyboardMarkup{
				InlineKeyboard: [][]tgbotapi.InlineKeyboardButton{
					{tgbotapi.NewInlineKeyboardButtonData("🏠 메뉴로", "menu:main")},
				},
			},
		)
		return
	}

	limit := count
	if limit > 15 {
		limit = 15
	}

	session, err := f.bot.services.SessionBuilder.BuildReviewSession(ctx, userID, limit)
	if err != nil || session == nil {
		f.bot.SendMessage(chatID, "❌ 복습 세션 생성에 실패했습니다.")
		return
	}

	keyboard := tgbotapi.NewInlineKeyboardMarkup(
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData("▶️ 복습 시작", fmt.Sprintf("session:%d:start", session.ID)),
		),
	)

	text := fmt.Sprintf("🔄 <b>복습 세션</b>\n\n복습 대상: %d문제\n\n준비되면 시작!",
		session.TotalQuestions)

	f.bot.EditMessage(chatID, cb.Message.MessageID, text, &keyboard)
}

// HandleSessionCallback handles session-level callbacks (start, finish).
func (f *SessionFlow) HandleSessionCallback(ctx context.Context, cb *tgbotapi.CallbackQuery) {
	parts := strings.Split(cb.Data, ":")
	if len(parts) < 3 {
		return
	}

	sessionID, err := strconv.Atoi(parts[1])
	if err != nil {
		log.Printf("Invalid session ID in callback: %s", parts[1])
		return
	}
	action := parts[2]

	switch action {
	case "start":
		f.startSession(ctx, cb, sessionID)
	case "finish":
		f.finishSession(ctx, cb, sessionID)
	}
}

// HandleAnswerCallback handles question answer callbacks.
func (f *SessionFlow) HandleAnswerCallback(ctx context.Context, cb *tgbotapi.CallbackQuery) {
	// Format: q:{sessionID}:{questionID}:{optionIndex} or q:{sessionID}:next:{currentIndex}
	parts := strings.Split(cb.Data, ":")
	if len(parts) < 4 {
		return
	}

	sessionID, err := strconv.Atoi(parts[1])
	if err != nil {
		return
	}

	if parts[2] == "next" {
		currentIdx := 0
		fmt.Sscanf(parts[3], "%d", &currentIdx)
		// 1. Remove the "Next" button from current feedback to avoid confusion
		f.bot.EditMessage(cb.Message.Chat.ID, cb.Message.MessageID, cb.Message.Text, nil)
		// 2. Show next question as a NEW message (0 messageID)
		f.showQuestion(ctx, cb.Message.Chat.ID, 0, sessionID, currentIdx+1)
		return
	}

	questionID, err := strconv.Atoi(parts[2])
	if err != nil {
		return
	}
	optionIdx := 0
	fmt.Sscanf(parts[3], "%d", &optionIdx)

	f.processAnswer(ctx, cb, sessionID, questionID, optionIdx)
}

func (f *SessionFlow) startSession(ctx context.Context, cb *tgbotapi.CallbackQuery, sessionID int) {
	if err := f.bot.services.SessionBuilder.StartSession(ctx, sessionID); err != nil {
		log.Printf("Error starting session: %v", err)
		return
	}

	// Store session start time in Redis for timing
	key := fmt.Sprintf("session:%d:question_start", sessionID)
	f.bot.rdb.Set(ctx, key, time.Now().UnixMilli(), 30*time.Minute)

	f.showQuestion(ctx, cb.Message.Chat.ID, cb.Message.MessageID, sessionID, 0)
}

func (f *SessionFlow) showQuestion(ctx context.Context, chatID int64, messageID int, sessionID int, questionIdx int) {
	sqs, err := f.bot.services.SessionBuilder.GetSessionQuestions(ctx, sessionID)
	if err != nil || questionIdx >= len(sqs) {
		// All questions answered, show finish button
		keyboard := tgbotapi.NewInlineKeyboardMarkup(
			tgbotapi.NewInlineKeyboardRow(
				tgbotapi.NewInlineKeyboardButtonData("📊 결과 보기", fmt.Sprintf("session:%d:finish", sessionID)),
			),
		)
		if messageID > 0 {
			f.bot.EditMessage(chatID, messageID, "✅ 모든 문제를 풀었습니다!", &keyboard)
		} else {
			f.bot.SendMessageWithKeyboard(chatID, "✅ 모든 문제를 풀었습니다!", keyboard)
		}
		return
	}

	sq := sqs[questionIdx]
	question, err := f.bot.services.SessionBuilder.GetQuestion(ctx, sq.QuestionID)
	if err != nil {
		return
	}

	reviewTag := ""
	if sq.IsReview {
		reviewTag = " 🔄"
	}

	text := fmt.Sprintf("📝 <b>문제 %d/%d</b>%s\n\n%s",
		questionIdx+1, len(sqs), reviewTag, question.Prompt)

	var keyboard *tgbotapi.InlineKeyboardMarkup

	if string(question.Type) == "fill_blank" {
		// Set active question in Redis for 1 hour
		key := fmt.Sprintf("user:%d:active_question", chatID)
		state := fmt.Sprintf("%d:%d", sessionID, questionIdx)
		f.bot.rdb.Set(ctx, key, state, 1*time.Hour)
		text += "\n\n⌨️ 채팅창에 답안을 영어로 입력해 주세요 (예: a, ka)"
	} else {
		options, err := question.GetOptions()
		if err != nil || len(options) == 0 {
			return
		}

		// Build inline keyboard with options (2 per row)
		var rows [][]tgbotapi.InlineKeyboardButton
		for i := 0; i < len(options); i += 2 {
			var row []tgbotapi.InlineKeyboardButton
			row = append(row, tgbotapi.NewInlineKeyboardButtonData(
				options[i],
				fmt.Sprintf("q:%d:%d:%d", sessionID, question.ID, i),
			))
			if i+1 < len(options) {
				row = append(row, tgbotapi.NewInlineKeyboardButtonData(
					options[i+1],
					fmt.Sprintf("q:%d:%d:%d", sessionID, question.ID, i+1),
				))
			}
			rows = append(rows, row)
		}
		keyboard = &tgbotapi.InlineKeyboardMarkup{InlineKeyboard: rows}
	}

	key := fmt.Sprintf("session:%d:question_start", sessionID)
	f.bot.rdb.Set(ctx, key, time.Now().UnixMilli(), 30*time.Minute)

	if messageID > 0 {
		f.bot.EditMessage(chatID, messageID, text, keyboard)
	} else {
		if keyboard != nil {
			f.bot.SendMessageWithKeyboard(chatID, text, *keyboard)
		} else {
			f.bot.SendMessage(chatID, text)
		}
	}
}

func (f *SessionFlow) processAnswer(ctx context.Context, cb *tgbotapi.CallbackQuery, sessionID, questionID int, optionIdx int) {
	question, err := f.bot.services.SessionBuilder.GetQuestion(ctx, questionID)
	if err != nil {
		return
	}

	options, err := question.GetOptions()
	if err != nil || optionIdx >= len(options) {
		return
	}

	selectedAnswer := options[optionIdx]
	f.processAnswerText(ctx, cb.Message.Chat.ID, sessionID, questionID, selectedAnswer, cb.Message.MessageID)
}

// HandleTextInput intercepts text messages if there is an active text question.
func (f *SessionFlow) HandleTextInput(ctx context.Context, msg *tgbotapi.Message) bool {
	key := fmt.Sprintf("user:%d:active_question", msg.Chat.ID)
	state, err := f.bot.rdb.Get(ctx, key).Result()
	if err != nil {
		return false
	}

	parts := strings.Split(state, ":")
	if len(parts) != 2 {
		return false
	}
	sessionID, _ := strconv.Atoi(parts[0])
	questionIdx, _ := strconv.Atoi(parts[1])

	f.bot.rdb.Del(ctx, key)

	sqs, err := f.bot.services.SessionBuilder.GetSessionQuestions(ctx, sessionID)
	if err != nil || questionIdx >= len(sqs) {
		return false
	}
	questionID := sqs[questionIdx].QuestionID

	f.processAnswerText(ctx, msg.Chat.ID, sessionID, questionID, strings.TrimSpace(msg.Text), 0)
	return true
}

func (f *SessionFlow) processAnswerText(ctx context.Context, chatID int64, sessionID, questionID int, selectedAnswer string, messageID int) {
	question, err := f.bot.services.SessionBuilder.GetQuestion(ctx, questionID)
	if err != nil {
		return
	}

	// Grade the answer
	selectedAnswer = strings.ToLower(selectedAnswer) // For Kana fill in the blank
	isCorrect, err := f.bot.services.Grader.GradeAnswer(ctx, sessionID, questionID, selectedAnswer)
	if err != nil {
		log.Printf("Error grading answer: %v", err)
		return
	}

	// Determine current question index for "next" button
	sqs, _ := f.bot.services.SessionBuilder.GetSessionQuestions(ctx, sessionID)
	currentIdx := 0
	for i, sq := range sqs {
		if sq.QuestionID == questionID {
			currentIdx = i
			break
		}
	}

	var text string
	if isCorrect {
		text = fmt.Sprintf("✅ <b>정답!</b>\n\n%s", question.Explanation)
	} else {
		text = fmt.Sprintf("❌ <b>오답</b>\n\n입력/선택: %s\n정답: <b>%s</b>\n\n%s",
			selectedAnswer, question.CorrectAnswer, question.Explanation)
	}

	nextLabel := "다음 문제 →"
	if currentIdx+1 >= len(sqs) {
		nextLabel = "📊 결과 보기"
	}

	var nextData string
	if currentIdx+1 >= len(sqs) {
		nextData = fmt.Sprintf("session:%d:finish", sessionID)
	} else {
		nextData = fmt.Sprintf("q:%d:next:%d", sessionID, currentIdx)
	}

	keyboard := tgbotapi.NewInlineKeyboardMarkup(
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData(nextLabel, nextData),
		),
	)

	if messageID > 0 {
		f.bot.EditMessage(chatID, messageID, text, &keyboard)
	} else {
		// Normal message replacement
		f.bot.SendMessageWithKeyboard(chatID, text, keyboard)
	}
}

func (f *SessionFlow) finishSession(ctx context.Context, cb *tgbotapi.CallbackQuery, sessionID int) {
	result, err := f.bot.services.Grader.CompleteSession(ctx, sessionID, cb.From.ID)
	if err != nil {
		log.Printf("Error completing session: %v", err)
		return
	}

	accuracy := float64(0)
	if result.TotalQuestions > 0 {
		accuracy = float64(result.CorrectCount) / float64(result.TotalQuestions) * 100
	}

	// Build wrong answers summary
	wrongSummary := ""
	if len(result.WrongAnswers) > 0 {
		wrongSummary = "\n\n<b>틀린 문제:</b>\n"
		for _, wa := range result.WrongAnswers {
			q, err := f.bot.services.SessionBuilder.GetQuestion(ctx, wa.QuestionID)
			if err != nil {
				continue
			}
			answer := ""
			if wa.UserAnswer != nil {
				answer = *wa.UserAnswer
			}
			wrongSummary += fmt.Sprintf("❌ %s → %s (정답: %s)\n",
				truncate(stripHTML(q.Prompt), 30), answer, q.CorrectAnswer)
		}
	}

	text := fmt.Sprintf(`🎉 <b>세션 완료!</b>

정답률: <b>%d/%d (%.0f%%)</b>%s`,
		result.CorrectCount, result.TotalQuestions, accuracy, wrongSummary)

	keyboard := tgbotapi.NewInlineKeyboardMarkup(
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData("🏠 메뉴로", "menu:main"),
		),
	)

	f.bot.EditMessage(cb.Message.Chat.ID, cb.Message.MessageID, text, &keyboard)
}

// PushSession sends a session notification to a user.
func (f *SessionFlow) PushSession(ctx context.Context, chatID int64, sessionID int, sessionType string) error {
	emoji := "📚"
	label := "학습"
	if sessionType == "evening" {
		emoji = "🌙"
		label = "복습"
	}

	text := fmt.Sprintf("%s <b>%s 세션이 도착했습니다!</b>\n\n아래 버튼을 눌러 시작하세요.", emoji, label)

	keyboard := tgbotapi.NewInlineKeyboardMarkup(
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData("▶️ 시작하기", fmt.Sprintf("session:%d:start", sessionID)),
		),
	)

	return f.bot.SendMessageWithKeyboard(chatID, text, keyboard)
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
