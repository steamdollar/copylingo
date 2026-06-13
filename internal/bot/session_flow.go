package bot

import (
	"context"
	"fmt"
	"log/slog"
	"strconv"
	"strings"
	"time"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"

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
	// get in-progess session for user, if exists, and resume
	resumed, err := sf.getInProgressSessions(ctx, cb)
	if err != nil {
		slog.ErrorContext(ctx, "Failed to fetch in-progress sessions",
			"event", "telegram.session.in_progress_lookup_failed",
			"user_id", cb.From.ID,
			"error", err,
		)
		sf.showSessionFetchError(cb)
		return
	}
	if resumed {
		return
	}

	// get pending session for user, if exists, and show start button
	if err := sf.getPendingSessions(ctx, cb); err != nil {
		slog.ErrorContext(ctx, "Failed to fetch pending sessions",
			"event", "telegram.session.pending_lookup_failed",
			"user_id", cb.From.ID,
			"error", err,
		)
		sf.showSessionFetchError(cb)
	}
}

func (sf *SessionFlow) getPendingSessions(ctx context.Context, cb *tgbotapi.CallbackQuery) error {
	chatID := cb.Message.Chat.ID
	sessions, err := sf.bot.services.SessionBuilder.GetSessionsByStatus(ctx, cb.From.ID, config.SessionStatusPending)
	if err != nil {
		return err
	}
	if len(sessions) == 0 {
		sf.bot.EditMessage(chatID, cb.Message.MessageID,
			"📚 현재 대기 중인 학습 세션이 없습니다.\n다음 세션이 자동으로 전송될 예정입니다!",
			mainMenuKeyboard(),
		)
		return nil
	}
	session, ok := firstQuizSession(sessions)
	if !ok {
		sf.bot.EditMessage(chatID, cb.Message.MessageID,
			"📚 현재 대기 중인 문제 풀이 세션이 없습니다.\n다음 세션이 자동으로 전송될 예정입니다!",
			mainMenuKeyboard(),
		)
		return nil
	}
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
	return nil
}

func (sf *SessionFlow) getInProgressSessions(ctx context.Context, cb *tgbotapi.CallbackQuery) (bool, error) {
	chatID := cb.Message.Chat.ID
	inProgressSessions, err := sf.bot.services.SessionBuilder.
		GetSessionsByStatus(ctx, cb.From.ID, config.SessionStatusInProgress)
	if err != nil {
		return false, err
	}
	session, ok := firstQuizSession(inProgressSessions)
	if !ok {
		return false, nil
	}

	nextIdx, err := sf.nextUnansweredQuestionIndex(ctx, session.ID)
	if err != nil {
		slog.ErrorContext(ctx, "Failed to find next unanswered question",
			"event", "telegram.session.next_question_lookup_failed",
			"session_id", session.ID,
			"error", err,
		)
		sf.bot.EditMessage(chatID, cb.Message.MessageID,
			"⚠️ 진행 중 세션 상태가 만료되었습니다. 새 세션을 다시 시작해 주세요.",
			mainMenuKeyboard(),
		)
		return true, nil
	}
	editMessageID := cb.Message.MessageID
	sf.showQuestion(ctx, chatID, &editMessageID, session.ID, nextIdx)
	return true, nil
}

// StartReview starts an on-demand review session.
func (sf *SessionFlow) StartReview(ctx context.Context, cb *tgbotapi.CallbackQuery) {
	userID := cb.From.ID
	chatID := cb.Message.Chat.ID

	count, _ := sf.bot.services.SRS.GetDueCount(ctx)
	if count == 0 {
		sf.bot.EditMessage(chatID, cb.Message.MessageID, "✅ 복습할 문제가 없습니다! 훌륭합니다 🎉", mainMenuKeyboard())
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
		slog.WarnContext(ctx, "Invalid session ID in callback",
			"event", "telegram.callback.invalid_session_id",
		)
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
	// update session status at db (pending > in progress)
	if err := sf.bot.
		services.SessionBuilder.StartSession(ctx, sessionID); err != nil {
		slog.ErrorContext(ctx, "Failed to start session",
			"event", "telegram.session.start_failed",
			"session_id", sessionID,
			"error", err,
		)
		return
	}

	// session fetch by id, set at redis
	if _, err := sf.bot.services.ActiveSession.CreateFromDB(ctx, sessionID); err != nil {
		slog.ErrorContext(ctx, "Failed to create active session state",
			"event", "telegram.session.active_state_create_failed",
			"session_id", sessionID,
			"error", err,
		)
		sf.bot.SendMessage(cb.Message.Chat.ID, "❌ 세션 상태를 준비하지 못했습니다. 잠시 후 다시 시도해 주세요.")
		return
	}

	// redis에 k-v로 시작 시간 기록
	key := config.SessionQuestionStartRedisKey.Format(sessionID)
	sf.bot.rdb.Set(ctx, key, time.Now().UnixMilli(), 30*time.Minute)

	editMessageID := cb.Message.MessageID
	sf.showQuestion(ctx, cb.Message.Chat.ID, &editMessageID, sessionID, 0)
}

func (sf *SessionFlow) finishSession(ctx context.Context, cb *tgbotapi.CallbackQuery, sessionID int) {
	result, err := sf.bot.services.Grader.CompleteSession(ctx, sessionID, cb.From.ID)
	if err != nil {
		slog.ErrorContext(ctx, "Failed to complete session",
			"event", "telegram.session.complete_failed",
			"session_id", sessionID,
			"error", err,
		)
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
			if wa.SessionQuestion.IsCorrect == nil || *wa.SessionQuestion.IsCorrect {
				continue
			}
			q := wa.Question
			if q.Type == model.QuestionKanaHandwriting {
				wrongSummary += fmt.Sprintf("❌ %s (정답: %s)\n",
					truncate(stripHTML(q.Prompt), 30), q.CorrectAnswer)
				continue
			}
			answer := formatSessionAnswer(wa.SessionQuestion.UserAnswer)
			wrongSummary += fmt.Sprintf("❌ %s → %s (정답: %s)\n",
				truncate(stripHTML(q.Prompt), 30), answer, q.CorrectAnswer)
		}
	}

	text := fmt.Sprintf(`🎉 <b>세션 완료!</b>

정답률: <b>%d/%d (%.0f%%)</b>%s`,
		result.CorrectCount, result.TotalQuestions, accuracy, wrongSummary)

	sf.bot.EditMessage(cb.Message.Chat.ID, cb.Message.MessageID, text, mainMenuKeyboard())
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
