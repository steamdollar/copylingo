package bot

import (
	"context"
	"errors"
	"fmt"
	"log"
	"net/url"
	"strconv"
	"strings"
	"time"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
	"github.com/lsj/copylingo/internal/callback"
	"github.com/lsj/copylingo/internal/config"
	"github.com/lsj/copylingo/internal/model"
)

// SessionFlow handles the question-answering interaction flow.
type SessionFlow struct {
	bot *Bot
}

func NewSessionFlow(bot *Bot) *SessionFlow {
	return &SessionFlow{bot: bot}
}

// StartStudy begins a new study session or resumes a pending one.
func (sf *SessionFlow) StartStudy(ctx context.Context, cb *tgbotapi.CallbackQuery) {
	userID := cb.From.ID
	chatID := cb.Message.Chat.ID

	// Resume in-progress session first.
	inProgressSessions, err := sf.bot.services.SessionBuilder.GetInProgressSessions(ctx, userID)
	if err == nil && len(inProgressSessions) > 0 {
		session := inProgressSessions[0]
		nextIdx, err := sf.nextUnansweredQuestionIndex(ctx, session.ID)
		if err != nil {
			log.Printf("Error finding next unanswered question for session %d: %v", session.ID, err)
			nextIdx = 0
		}
		editMessageID := cb.Message.MessageID
		sf.showQuestion(ctx, chatID, &editMessageID, session.ID, nextIdx)
		return
	}

	// Check for pending sessions
	sessions, err := sf.bot.services.SessionBuilder.GetPendingSessions(ctx, userID)

	// 진행중인 세션이 없으면 리턴
	if err != nil || len(sessions) == 0 {
		sf.bot.EditMessage(chatID, cb.Message.MessageID,
			"📚 현재 대기 중인 학습 세션이 없습니다.\n다음 세션이 자동으로 전송될 예정입니다!",
			&tgbotapi.InlineKeyboardMarkup{
				InlineKeyboard: [][]tgbotapi.InlineKeyboardButton{
					{tgbotapi.NewInlineKeyboardButtonData("🏠 메뉴로", config.ActionMenuMain)},
				},
			},
		)
		return
	}

	// 순차적으로 세션 시작
	session := sessions[0]

	keyboard := tgbotapi.NewInlineKeyboardMarkup(
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData("▶️ 시작하기", fmt.Sprintf(config.FormatSessionStart, session.ID)),
		),
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData("🏠 메뉴로", config.ActionMenuMain),
		),
	)

	text := fmt.Sprintf("📚 <b>학습 세션 준비됨</b>\n\n총 %d문제\n유형: %s\n\n준비되면 시작 버튼을 누르세요!",
		session.TotalQuestions, sessionTypeLabel(string(session.Type)))

	sf.bot.EditMessage(chatID, cb.Message.MessageID, text, &keyboard)
}

// StartReview starts an on-demand review session.
func (sf *SessionFlow) StartReview(ctx context.Context, cb *tgbotapi.CallbackQuery) {
	userID := cb.From.ID
	chatID := cb.Message.Chat.ID

	count, _ := sf.bot.services.SRS.GetDueCount(ctx)
	if count == 0 {
		sf.bot.EditMessage(chatID, cb.Message.MessageID,
			"✅ 복습할 문제가 없습니다! 훌륭합니다 🎉",
			&tgbotapi.InlineKeyboardMarkup{
				InlineKeyboard: [][]tgbotapi.InlineKeyboardButton{
					{tgbotapi.NewInlineKeyboardButtonData("🏠 메뉴로", config.ActionMenuMain)},
				},
			},
		)
		return
	}

	limit := count
	if limit > 15 {
		limit = 15
	}

	session, err := sf.bot.services.SessionBuilder.BuildReviewSession(ctx, userID, limit)
	if err != nil || session == nil {
		sf.bot.SendMessage(chatID, "❌ 복습 세션 생성에 실패했습니다.")
		return
	}

	keyboard := tgbotapi.NewInlineKeyboardMarkup(
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData("▶️ 복습 시작", fmt.Sprintf(config.FormatSessionStart, session.ID)),
		),
	)

	text := fmt.Sprintf("🔄 <b>복습 세션</b>\n\n복습 대상: %d문제\n\n준비되면 시작!",
		session.TotalQuestions)

	sf.bot.EditMessage(chatID, cb.Message.MessageID, text, &keyboard)
}

// HandleSessionCallback handles session-level callbacks (start, finish).
func (sf *SessionFlow) HandleSessionCallback(ctx context.Context, cb *tgbotapi.CallbackQuery) {
	// e.g. session:50:start
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
		sf.startSession(ctx, cb, sessionID)
	case "finish":
		sf.finishSession(ctx, cb, sessionID)
	}
}

// HandleAnswerCallback handles question answer callbacks.
func (sf *SessionFlow) HandleAnswerCallback(ctx context.Context, cb *tgbotapi.CallbackQuery) {
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
		if cb.Message == nil {
			return
		}
		currentIdx := 0
		fmt.Sscanf(parts[3], "%d", &currentIdx)
		if !sf.isQuestionAnswered(ctx, sessionID, currentIdx) {
			if isStaleMiniAppCallback(parts, sf.bot.cfg.Server.PublicBaseURL) {
				sf.bot.ClearInlineKeyboard(cb.Message.Chat.ID, cb.Message.MessageID)
				sf.bot.SendMessage(cb.Message.Chat.ID, "🔄 손글씨 링크가 만료되어 같은 문제를 새 링크로 다시 보냅니다.")
				sf.showQuestion(ctx, cb.Message.Chat.ID, nil, sessionID, currentIdx)
				return
			}
			sf.bot.SendMessage(cb.Message.Chat.ID, "✍️ 먼저 손글씨 답안을 제출해 주세요.")
			return
		}
		// 같은 손글씨 제출 결과로 다음 문제를 중복 진행하지 못하게 먼저 버튼을 제거한다.
		sf.bot.ClearInlineKeyboard(cb.Message.Chat.ID, cb.Message.MessageID)
		// 손글씨 문항은 Mini App HTTP로 제출되므로 원래 메시지 맥락을 남기고,
		// 다음 문제는 새 Telegram 메시지로 렌더링한다.
		sf.showQuestion(ctx, cb.Message.Chat.ID, nil, sessionID, currentIdx+1)
		return
	}

	questionID, err := strconv.Atoi(parts[2])
	if err != nil {
		return
	}
	optionIdx := 0
	fmt.Sscanf(parts[3], "%d", &optionIdx)

	sf.processAnswer(ctx, cb, sessionID, questionID, optionIdx)
}

func (sf *SessionFlow) startSession(ctx context.Context, cb *tgbotapi.CallbackQuery, sessionID int) {
	// 세션 상태 update at db
	if err := sf.bot.
		services.SessionBuilder.StartSession(ctx, sessionID); err != nil {
		log.Printf("Error starting session: %v", err)
		return
	}

	// redis에 k-v로 시작 시간 기록
	key := fmt.Sprintf(config.KeySessionQuestionStart, sessionID)
	sf.bot.rdb.Set(ctx, key, time.Now().UnixMilli(), 30*time.Minute)

	editMessageID := cb.Message.MessageID
	sf.showQuestion(ctx, cb.Message.Chat.ID, &editMessageID, sessionID, 0)
}

func (sf *SessionFlow) showQuestion(ctx context.Context, chatID int64,
	editMessageID *int, sessionID, questionIdx int) {
	sqs, err := sf.bot.services.SessionBuilder.GetSessionQuestions(ctx, sessionID)
	if err != nil {
		log.Printf("Error getting session questions for session %d: %v", sessionID, err)
		if editMessageID != nil {
			sf.bot.EditMessage(chatID, *editMessageID, "❌ 문제를 불러오지 못했습니다. 잠시 후 다시 시도해 주세요.", nil)
		} else {
			sf.bot.SendMessage(chatID, "❌ 문제를 불러오지 못했습니다. 잠시 후 다시 시도해 주세요.")
		}
		return
	}

	if questionIdx >= len(sqs) {
		// All questions answered, show finish button
		keyboard := tgbotapi.NewInlineKeyboardMarkup(
			tgbotapi.NewInlineKeyboardRow(
				tgbotapi.NewInlineKeyboardButtonData("📊 결과 보기", fmt.Sprintf(config.FormatSessionFinish, sessionID)),
			),
		)
		if editMessageID != nil {
			sf.bot.EditMessage(chatID, *editMessageID, "✅ 모든 문제를 풀었습니다!", &keyboard)
		} else {
			sf.bot.SendMessageWithKeyboard(chatID, "✅ 모든 문제를 풀었습니다!", keyboard)
		}
		return
	}

	sq := sqs[questionIdx]
	// TODO: 매 showQuestion 함수 호출마다 db hit > cache 도입
	question, err := sf.bot.services.SessionBuilder.GetQuestion(ctx, sq.QuestionID)
	if err != nil {
		log.Printf("Error getting question for session %d question_idx=%d question_id=%d: %v",
			sessionID, questionIdx, sq.QuestionID, err)
		if editMessageID != nil {
			sf.bot.EditMessage(chatID, *editMessageID, "❌ 문제를 불러오지 못했습니다. 잠시 후 다시 시도해 주세요.", nil)
		} else {
			sf.bot.SendMessage(chatID, "❌ 문제를 불러오지 못했습니다. 잠시 후 다시 시도해 주세요.")
		}
		return
	}

	reviewTag := ""
	if sq.IsReview {
		reviewTag = " 🔄"
	}

	text := fmt.Sprintf("📝 <b>문제 %d/%d</b>%s\n\n%s",
		questionIdx+1, len(sqs), reviewTag, question.Prompt)

	var keyboard *tgbotapi.InlineKeyboardMarkup

	switch question.Type {
	case model.QuestionKanaHandwriting:
		miniAppURL, err := sf.handwritingMiniAppURL(sessionID, question.ID, question.Language, question.ProficiencyLevel)
		if err != nil {
			text += "\n\n⚠️ 손글씨 Mini App URL 설정이 필요합니다. `COPYLINGO_SERVER_PUBLIC_BASE_URL`을 설정해 주세요."
			break
		}
		nextData := callback.FormatHandwritingNext(sessionID, questionIdx, sf.bot.cfg.Server.PublicBaseURL)

		text += "\n\n✍️ 아래 버튼을 눌러 화면에 글자를 써 주세요.\n제출 후 이 채팅으로 돌아와 다음 문제를 진행하면 됩니다."
		replyMarkup := webAppKeyboardMarkup{
			InlineKeyboard: [][]webAppButton{
				{newWebAppButton("✍️ 손글씨로 답하기", miniAppURL)},
				{newCallbackButton("제출 후 다음 문제 →", nextData)},
			},
		}
		if editMessageID != nil {
			// Web App 버튼은 별도 메시지로 두는 편이 Mini App 왕복 흐름을 추적하기 쉽다.
			// 이전 메시지는 재사용하지 않고 짧은 안내 문구로 축약한다.
			sf.bot.EditMessage(chatID, *editMessageID, "✍️ 손글씨 문항을 새 메시지로 보냈습니다.", nil)
		}
		msgID, err := sf.bot.SendMessageWithReplyMarkup(chatID, text, replyMarkup)
		if err != nil {
			log.Printf("Error sending handwriting message chat=%d session=%d question=%d: %v", chatID, sessionID, question.ID, err)
			return
		}

		// Store message ID to clean up buttons after Mini App submission
		key := fmt.Sprintf(config.KeyHandwritingMessage, sessionID, question.ID)
		val := fmt.Sprintf("%d:%d", chatID, msgID)
		if err := sf.bot.rdb.Set(ctx, key, val, time.Hour).Err(); err != nil {
			log.Printf("Error caching handwriting message id session=%d question=%d: %v", sessionID, question.ID, err)
		}
		return
	case model.QuestionFillBlank, model.QuestionSubjective:
		// Set active question in Redis for 1 hour
		key := fmt.Sprintf(config.KeyUserActiveQuestion, chatID)
		state := fmt.Sprintf("%d:%d", sessionID, questionIdx)
		sf.bot.rdb.Set(ctx, key, state, 1*time.Hour)

		if question.Type == model.QuestionSubjective {
			text += "\n\n⌨️ 정답을 자유롭게 텍스트로 입력해 주세요"
		} else {
			text += "\n\n⌨️ 채팅창에 답안을 입력해 주세요"
		}
	default:
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
				fmt.Sprintf(config.FormatQuestionAnswer, sessionID, question.ID, i),
			))
			if i+1 < len(options) {
				row = append(row, tgbotapi.NewInlineKeyboardButtonData(
					options[i+1],
					fmt.Sprintf(config.FormatQuestionAnswer, sessionID, question.ID, i+1),
				))
			}
			rows = append(rows, row)
		}
		keyboard = &tgbotapi.InlineKeyboardMarkup{InlineKeyboard: rows}
	}

	key := fmt.Sprintf(config.KeySessionQuestionStart, sessionID)
	sf.bot.rdb.Set(ctx, key, time.Now().UnixMilli(), 30*time.Minute)

	if editMessageID != nil {
		sf.bot.EditMessage(chatID, *editMessageID, text, keyboard)
	} else {
		if keyboard != nil {
			sf.bot.SendMessageWithKeyboard(chatID, text, *keyboard)
		} else {
			sf.bot.SendMessage(chatID, text)
		}
	}
}

func (sf *SessionFlow) isQuestionAnswered(ctx context.Context, sessionID, questionIdx int) bool {
	sqs, err := sf.bot.services.SessionBuilder.GetSessionQuestions(ctx, sessionID)
	if err != nil || questionIdx < 0 || questionIdx >= len(sqs) {
		return false
	}
	return sqs[questionIdx].IsCorrect != nil
}

func (sf *SessionFlow) nextUnansweredQuestionIndex(ctx context.Context, sessionID int) (int, error) {
	sqs, err := sf.bot.services.SessionBuilder.GetSessionQuestions(ctx, sessionID)
	if err != nil {
		return 0, err
	}

	for idx, sq := range sqs {
		if sq.IsCorrect == nil {
			return idx, nil
		}
	}

	return len(sqs), nil
}

// TODO: 이 함수 굳이 이렇게 복잡하게 짜야 함?
func (sf *SessionFlow) handwritingMiniAppURL(sessionID, questionID int, language, level string) (string, error) {
	baseURL := strings.TrimRight(sf.bot.cfg.Server.PublicBaseURL, "/")
	if baseURL == "" {
		return "", fmt.Errorf("server public base url is empty")
	}
	u, err := url.Parse(baseURL + config.PathHandwritingMiniApp)
	if err != nil {
		return "", err
	}
	q := u.Query()
	q.Set("session_id", strconv.Itoa(sessionID))
	q.Set("question_id", strconv.Itoa(questionID))
	q.Set("language", language)
	q.Set("level", level)
	u.RawQuery = q.Encode()
	return u.String(), nil
}

func isStaleMiniAppCallback(parts []string, currentPublicBaseURL string) bool {
	return callback.IsStaleMiniAppCallback(parts, currentPublicBaseURL)
}

func (sf *SessionFlow) processAnswer(ctx context.Context, cb *tgbotapi.CallbackQuery, sessionID, questionID int, optionIdx int) {
	question, err := sf.bot.services.SessionBuilder.GetQuestion(ctx, questionID)
	if err != nil {
		return
	}

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
	key := fmt.Sprintf(config.KeyUserActiveQuestion, msg.Chat.ID)
	state, err := sf.bot.rdb.Get(ctx, key).Result()
	if err != nil {
		return false
	}

	parts := strings.Split(state, ":")
	if len(parts) != 2 {
		return false
	}
	sessionID, _ := strconv.Atoi(parts[0])
	questionIdx, _ := strconv.Atoi(parts[1])

	sf.bot.rdb.Del(ctx, key)

	sqs, err := sf.bot.services.SessionBuilder.GetSessionQuestions(ctx, sessionID)
	if err != nil || questionIdx >= len(sqs) {
		return false
	}
	questionID := sqs[questionIdx].QuestionID

	sf.processAnswerText(ctx, msg.Chat.ID, sessionID, questionID, strings.TrimSpace(msg.Text), nil)
	return true
}

func (sf *SessionFlow) processAnswerText(ctx context.Context, chatID int64, sessionID, questionID int, selectedAnswer string, editMessageID *int) {
	question, err := sf.bot.services.SessionBuilder.GetQuestion(ctx, questionID)
	if err != nil {
		return
	}

	// Grade the answer
	switch question.Type {
	case model.QuestionFillBlank:
		selectedAnswer = strings.ToLower(selectedAnswer) // For Kana fill in the blank
	case model.QuestionSubjective:
		// Show typing status for AI grading UX
		sf.bot.api.Request(tgbotapi.NewChatAction(chatID, tgbotapi.ChatTyping))
	}

	isCorrect, feedback, err := sf.bot.services.Grader.GradeAnswer(ctx, sessionID, questionID, selectedAnswer)
	if err != nil {
		if errors.Is(err, config.ErrAIConfigMissing) {
			errMsg := tgbotapi.NewMessage(chatID, "⚠️ 시스템 설정 문제로 현재 AI 주관식 채점이 불가능합니다. 임시로 오답 처리하고 넘어갑니다.")
			sf.bot.api.Send(errMsg)
		} else {
			log.Printf("Error grading answer: %v", err)
			return
		}
	}

	// Determine current question index for "next" button
	sqs, _ := sf.bot.services.SessionBuilder.GetSessionQuestions(ctx, sessionID)
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

	if feedback != "" {
		text += fmt.Sprintf("\n\n🤖 <b>AI 피드백:</b>\n%s", feedback)
	}

	nextLabel := "다음 문제 →"
	if currentIdx+1 >= len(sqs) {
		nextLabel = "📊 결과 보기"
	}

	var nextData string
	if currentIdx+1 >= len(sqs) {
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

func (sf *SessionFlow) finishSession(ctx context.Context, cb *tgbotapi.CallbackQuery, sessionID int) {
	result, err := sf.bot.services.Grader.CompleteSession(ctx, sessionID, cb.From.ID)
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
			if wa.IsCorrect == nil || *wa.IsCorrect {
				continue
			}
			q, err := sf.bot.services.SessionBuilder.GetQuestion(ctx, wa.QuestionID)
			if err != nil {
				continue
			}
			if q.Type == model.QuestionKanaHandwriting {
				wrongSummary += fmt.Sprintf("❌ %s (정답: %s)\n",
					truncate(stripHTML(q.Prompt), 30), q.CorrectAnswer)
				continue
			}
			answer := formatSessionAnswer(wa.UserAnswer)
			wrongSummary += fmt.Sprintf("❌ %s → %s (정답: %s)\n",
				truncate(stripHTML(q.Prompt), 30), answer, q.CorrectAnswer)
		}
	}

	text := fmt.Sprintf(`🎉 <b>세션 완료!</b>

정답률: <b>%d/%d (%.0f%%)</b>%s`,
		result.CorrectCount, result.TotalQuestions, accuracy, wrongSummary)

	keyboard := tgbotapi.NewInlineKeyboardMarkup(
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData("🏠 메뉴로", config.ActionMenuMain),
		),
	)

	sf.bot.EditMessage(cb.Message.Chat.ID, cb.Message.MessageID, text, &keyboard)
}

func formatSessionAnswer(userAnswer *string) string {
	if userAnswer == nil || *userAnswer == "" {
		return "미응답"
	}
	return *userAnswer
}

func (sf *SessionFlow) PushSession(ctx context.Context, chatID int64, sessionID int, sessionType string) error {
	emoji := "📚"
	label := "학습"
	if sessionType == "evening" {
		emoji = "🌙"
		label = "복습"
	}

	text := fmt.Sprintf("%s <b>%s 세션이 도착했습니다!</b>\n\n아래 버튼을 눌러 시작하세요.", emoji, label)

	// 여러 row를 합쳐 줌
	keyboard := tgbotapi.NewInlineKeyboardMarkup(
		// 버튼을 가로로 배치할 row 생성
		tgbotapi.NewInlineKeyboardRow(
			// 버튼 1개 만들기
			tgbotapi.NewInlineKeyboardButtonData("▶️ 시작하기", fmt.Sprintf(config.FormatSessionStart, sessionID)),
		),
	)

	// 위해서 만든 keyboard를 send
	return sf.bot.SendMessageWithKeyboard(chatID, text, keyboard)
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
