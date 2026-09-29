package bot

import (
	"context"
	"fmt"
	"log/slog"
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
func (sf *SessionFlow) StartStudy(
	ctx context.Context,
	cb *tgbotapi.CallbackQuery,
) {
	// get in-progess session for user, if exists, and resume
	resumed, err := sf.getInProgressSessions(
		ctx,
		cb,
	)
	if err != nil {
		slog.ErrorContext(
			ctx,
			"Failed to fetch in-progress sessions",
			"event",
			"telegram.session.in_progress_lookup_failed",
			"user_id",
			cb.From.ID,
			"error",
			err,
		)
		sf.showSessionFetchError(cb)
		return
	}
	if resumed {
		return
	}

	// get pending session for user, if exists, and show start button
	if err := sf.getPendingSessions(
		ctx,
		cb,
	); err != nil {
		slog.ErrorContext(
			ctx,
			"Failed to fetch pending sessions",
			"event",
			"telegram.session.pending_lookup_failed",
			"user_id",
			cb.From.ID,
			"error",
			err,
		)
		sf.showSessionFetchError(cb)
	}
}

func (sf *SessionFlow) getPendingSessions(
	ctx context.Context,
	cb *tgbotapi.CallbackQuery,
) error {
	chatID := cb.Message.Chat.ID
	sessions, err := sf.bot.services.SessionBuilder.GetSessionsByStatus(
		ctx,
		cb.From.ID,
		config.SessionStatusPending,
	)
	if err != nil {
		return err
	}
	if len(sessions) == 0 {
		sf.bot.EditMessage(
			chatID,
			cb.Message.MessageID,
			botMessagesByLocale[botDefaultLocale].pendingSessionsEmpty,
			mainMenuKeyboard(),
		)
		return nil
	}
	if studySession, ok := firstStudySession(sessions); ok {
		keyboard := tgbotapi.NewInlineKeyboardMarkup(
			tgbotapi.NewInlineKeyboardRow(
				tgbotapi.NewInlineKeyboardButtonData(
					botMessagesByLocale[botDefaultLocale].startButton,
					fmt.Sprintf(
						formatStudyStart,
						studySession.ID,
					),
				),
			),
			tgbotapi.NewInlineKeyboardRow(
				tgbotapi.NewInlineKeyboardButtonData(
					botMessagesByLocale[botDefaultLocale].menuHomeButton,
					callbackMenuMain,
				),
			),
		)
		text := fmt.Sprintf(
			botMessagesByLocale[botDefaultLocale].studySessionReadyFormat,
			studySession.TotalQuestions,
		)
		sf.bot.EditMessage(
			chatID,
			cb.Message.MessageID,
			text,
			&keyboard,
		)
		return nil
	}
	session, ok := firstQuizSession(sessions)
	if !ok {
		sf.bot.EditMessage(
			chatID,
			cb.Message.MessageID,
			botMessagesByLocale[botDefaultLocale].pendingSessionsEmpty,
			mainMenuKeyboard(),
		)
		return nil
	}
	keyboard := tgbotapi.NewInlineKeyboardMarkup(
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData(
				botMessagesByLocale[botDefaultLocale].startButton,
				fmt.Sprintf(
					formatSessionStart,
					session.ID,
				),
			),
		),
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData(
				botMessagesByLocale[botDefaultLocale].menuHomeButton,
				callbackMenuMain,
			),
		),
	)
	text := fmt.Sprintf(
		botMessagesByLocale[botDefaultLocale].quizSessionReadyFormat,
		session.TotalQuestions,
		sessionTypeLabel(string(session.Type)),
	)
	sf.bot.EditMessage(
		chatID,
		cb.Message.MessageID,
		text,
		&keyboard,
	)
	return nil
}

func (sf *SessionFlow) getInProgressSessions(
	ctx context.Context,
	cb *tgbotapi.CallbackQuery,
) (bool, error) {
	chatID := cb.Message.Chat.ID
	inProgressSessions, err := sf.bot.services.SessionBuilder.
		GetSessionsByStatus(
			ctx,
			cb.From.ID,
			config.SessionStatusInProgress,
		)
	if err != nil {
		return false, err
	}
	if studySession, ok := firstStudySession(inProgressSessions); ok {
		if sf.bot.study == nil {
			sf.bot.study = NewStudyFlow(sf.bot)
		}
		sf.bot.study.startSession(
			ctx,
			cb,
			studySession.ID,
		)
		return true, nil
	}
	session, ok := firstQuizSession(inProgressSessions)
	if !ok {
		return false, nil
	}

	nextIdx, err := sf.nextUnansweredQuestionIndex(
		ctx,
		session.ID,
	)
	if err != nil {
		slog.ErrorContext(
			ctx,
			"Failed to find next unanswered question",
			"event",
			"telegram.session.next_question_lookup_failed",
			"session_id",
			session.ID,
			"error",
			err,
		)
		sf.bot.EditMessage(
			chatID,
			cb.Message.MessageID,
			botMessagesByLocale[botDefaultLocale].activeSessionUnavailable,
			mainMenuKeyboard(),
		)
		return true, nil
	}
	editMessageID := cb.Message.MessageID
	sf.showQuestion(
		ctx,
		chatID,
		&editMessageID,
		session.ID,
		nextIdx,
	)
	return true, nil
}

// StartReview starts an on-demand review session.
func (sf *SessionFlow) StartReview(
	ctx context.Context,
	cb *tgbotapi.CallbackQuery,
) {
	userID := cb.From.ID
	chatID := cb.Message.Chat.ID

	user, err := sf.bot.services.User.GetUser(
		ctx,
		userID,
		cb.From.UserName,
	)
	if err != nil {
		sf.bot.SendMessage(
			chatID,
			botMessagesByLocale[botDefaultLocale].reviewUserLoadFailed,
		)
		return
	}

	count, _ := sf.bot.services.SRS.GetDueCount(
		ctx,
		userID,
		user.Language,
		user.ProficiencyLevel,
	)
	if count == 0 {
		sf.bot.EditMessage(
			chatID,
			cb.Message.MessageID,
			botMessagesByLocale[botDefaultLocale].noReviewQuestions,
			mainMenuKeyboard(),
		)
		return
	}

	limit := count
	if limit > 15 {
		limit = 15
	}

	session, err := sf.bot.services.SessionBuilder.BuildReviewSession(
		ctx,
		userID,
		user.Language,
		user.ProficiencyLevel,
		limit,
	)
	if err != nil || session == nil {
		sf.bot.SendMessage(
			chatID,
			botMessagesByLocale[botDefaultLocale].reviewSessionBuildFailed,
		)
		return
	}

	keyboard := tgbotapi.NewInlineKeyboardMarkup(
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData(
				botMessagesByLocale[botDefaultLocale].reviewStartButton,
				fmt.Sprintf(
					formatSessionStart,
					session.ID,
				),
			),
		),
	)

	text := fmt.Sprintf(
		botMessagesByLocale[botDefaultLocale].reviewSessionFormat,
		session.TotalQuestions,
	)

	sf.bot.EditMessage(
		chatID,
		cb.Message.MessageID,
		text,
		&keyboard,
	)
}

// HandleSessionCallback handles session-level callbacks (start, finish).
func (sf *SessionFlow) HandleSessionCallback(
	ctx context.Context,
	cb *tgbotapi.CallbackQuery,
) {
	// e.g. session:50:start
	parts := strings.Split(
		cb.Data,
		":",
	)
	if len(parts) < 3 {
		return
	}

	sessionID, err := strconv.Atoi(parts[1])
	if err != nil {
		slog.WarnContext(
			ctx,
			"Invalid session ID in callback",
			"event",
			"telegram.callback.invalid_session_id",
		)
		return
	}
	action := parts[2]

	switch action {
	case callbackActionStart:
		sf.startSession(
			ctx,
			cb,
			sessionID,
		)
	case callbackActionFinish:
		sf.finishSession(
			ctx,
			cb,
			sessionID,
		)
	}
}

// HandleAnswerCallback handles question answer callbacks.
func (sf *SessionFlow) HandleAnswerCallback(
	ctx context.Context,
	cb *tgbotapi.CallbackQuery,
) {
	// Format: q:{sessionID}:{questionID}:{optionIndex} or q:{sessionID}:next:{currentIndex}
	parts := strings.Split(
		cb.Data,
		":",
	)
	if len(parts) < 4 {
		return
	}

	sessionID, err := strconv.Atoi(parts[1])
	if err != nil {
		return
	}

	if parts[2] == callbackActionWordOrder {
		sf.handleWordOrderCallback(
			ctx,
			cb,
		)
		return
	}

	if parts[2] == callback.QuestionActionNext {
		if cb.Message == nil {
			return
		}
		currentIdx := 0
		fmt.Sscanf(
			parts[3],
			"%d",
			&currentIdx,
		)
		if !sf.isQuestionAnswered(
			ctx,
			sessionID,
			currentIdx,
		) {
			if isStaleMiniAppCallback(
				parts,
				sf.bot.cfg.Server.PublicBaseURL,
			) {
				sf.bot.ClearInlineKeyboard(
					cb.Message.Chat.ID,
					cb.Message.MessageID,
				)
				sf.bot.SendMessage(
					cb.Message.Chat.ID,
					botMessagesByLocale[botDefaultLocale].handwritingLinkExpired,
				)
				sf.showQuestion(
					ctx,
					cb.Message.Chat.ID,
					nil,
					sessionID,
					currentIdx,
				)
				return
			}
			sf.bot.SendMessage(
				cb.Message.Chat.ID,
				botMessagesByLocale[botDefaultLocale].handwritingSubmitFirst,
			)
			return
		}
		// 같은 손글씨 제출 결과로 다음 문제를 중복 진행하지 못하게 먼저 버튼을 제거한다.
		sf.bot.ClearInlineKeyboard(
			cb.Message.Chat.ID,
			cb.Message.MessageID,
		)
		// 손글씨 문항은 Mini App HTTP로 제출되므로 원래 메시지 맥락을 남기고,
		// 다음 문제는 새 Telegram 메시지로 렌더링한다.
		sf.showQuestion(
			ctx,
			cb.Message.Chat.ID,
			nil,
			sessionID,
			currentIdx+1,
		)
		return
	}

	if parts[2] == callbackActionAsk {
		sf.handleAskLLMQuestion(
			ctx,
			cb,
			sessionID,
			parts[3],
		)
		return
	}
	if parts[2] == callback.QuestionActionPolicy || parts[2] == callbackActionExclude {
		sf.handleQuizMaterialPreference(
			ctx,
			cb,
			parts,
		)
		return
	}

	questionID, err := strconv.Atoi(parts[2])
	if err != nil {
		return
	}
	optionIdx := 0
	fmt.Sscanf(
		parts[3],
		"%d",
		&optionIdx,
	)

	sf.processAnswer(
		ctx,
		cb,
		sessionID,
		questionID,
		optionIdx,
	)
}

// handleAskLLMQuestion arms one-shot LLM mode scoped to a just-answered question.
// The next plain-text message is answered with that question's context (see handleLLMQuestion).
func (sf *SessionFlow) handleAskLLMQuestion(
	ctx context.Context,
	cb *tgbotapi.CallbackQuery,
	sessionID int,
	questionIDStr string,
) {
	if cb.Message == nil {
		return
	}
	// owner gate를 callback에서도 재검증한다 (버튼은 owner에게만 렌더되지만 callback은 위조 가능).
	if !sf.bot.isLLMAllowed(cb.From) {
		return
	}
	questionID, err := strconv.Atoi(questionIDStr)
	if err != nil {
		return
	}
	if sf.bot.input == nil {
		sf.bot.SendMessage(
			cb.Message.Chat.ID,
			botMessagesByLocale[botDefaultLocale].llmQuestionActivationFailed,
		)
		return
	}

	input := model.PendingLLMInput{Kind: model.PendingLLMQuizQuestion, SessionID: sessionID, QuestionID: questionID}
	if err := sf.bot.input.SetLLMPending(
		ctx,
		cb.From.ID,
		input,
	); err != nil {
		slog.ErrorContext(
			ctx,
			"Failed to activate in-quiz LLM mode",
			"event",
			"telegram.llm.quiz_activate_failed",
			"session_id",
			sessionID,
			"question_id",
			questionID,
			"error",
			err,
		)
		sf.bot.SendMessage(
			cb.Message.Chat.ID,
			botMessagesByLocale[botDefaultLocale].llmQuestionActivationFailed,
		)
		return
	}
	sf.bot.SendMessageWithKeyboard(
		cb.Message.Chat.ID,
		botMessagesByLocale[botDefaultLocale].quizLLMQuestionPrompt,
		llmCancelKeyboard(),
	)
}

func (sf *SessionFlow) startSession(
	ctx context.Context,
	cb *tgbotapi.CallbackQuery,
	sessionID int,
) {
	// Redis is the source of truth while a quiz is in progress. Recovering from
	// DB unconditionally here would overwrite answers already recorded in the
	// working set when the user presses an old/repeated start button.
	state, err := sf.bot.services.QuizActiveSession.Get(
		ctx,
		sessionID,
	)
	if err != nil {
		slog.ErrorContext(
			ctx,
			"Failed to load active session state before start",
			"event",
			"telegram.session.active_state_lookup_failed",
			"session_id",
			sessionID,
			"error",
			err,
		)
		return
	}
	if cb.From != nil && state.Session.UserID != 0 && state.Session.UserID != cb.From.ID {
		slog.WarnContext(
			ctx,
			"Rejected session start for non-owner",
			"event",
			"telegram.session.start_owner_mismatch",
			"session_id",
			sessionID,
			"user_id",
			cb.From.ID,
		)
		return
	}
	wasPending := state.Session.Status == model.SessionPending

	// update session status at db (pending > in progress)
	if err := sf.bot.
		services.SessionBuilder.StartSession(
		ctx,
		sessionID,
	); err != nil {
		slog.ErrorContext(
			ctx,
			"Failed to start session",
			"event",
			"telegram.session.start_failed",
			"session_id",
			sessionID,
			"error",
			err,
		)
		return
	}
	// A DB-backed pending state has no progress to preserve. Reload it after
	// StartSession so the Redis copy also reflects the in_progress transition;
	// an already in-progress working set must never be replaced from DB.
	if wasPending {
		state, err = sf.bot.services.QuizActiveSession.CreateFromDB(
			ctx,
			sessionID,
		)
		if err != nil {
			slog.ErrorContext(
				ctx,
				"Failed to refresh active session after start",
				"event",
				"telegram.session.active_state_refresh_failed",
				"session_id",
				sessionID,
				"error",
				err,
			)
			sf.bot.SendMessage(
				cb.Message.Chat.ID,
				botMessagesByLocale[botDefaultLocale].sessionStatePrepareFailed,
			)
			return
		}
	}

	if sf.bot.timing != nil {
		_ = sf.bot.timing.RecordQuestionStart(
			ctx,
			sessionID,
			time.Now(),
		)
	}

	editMessageID := cb.Message.MessageID
	sf.showQuestion(
		ctx,
		cb.Message.Chat.ID,
		&editMessageID,
		sessionID,
		state.NextUnansweredIndex(),
	)
}

func (sf *SessionFlow) finishSession(
	ctx context.Context,
	cb *tgbotapi.CallbackQuery,
	sessionID int,
) {
	// Capture word-order draft keys before CompleteSession deletes the active
	// session working set. Drafts are ephemeral and must not survive a finished
	// session, including when the user reached the result screen via a retry.
	var wordOrderQuestionIDs []int
	if sf.bot.services != nil && sf.bot.services.QuizActiveSession != nil {
		if state, err := sf.bot.services.QuizActiveSession.Get(
			ctx,
			sessionID,
		); err == nil &&
			cb != nil && cb.From != nil && state.Session.UserID == cb.From.ID {
			for _, item := range state.Items {
				if item.Question.Type == model.QuestionWordOrder {
					wordOrderQuestionIDs = append(
						wordOrderQuestionIDs,
						item.Question.ID,
					)
				}
			}
		}
	}
	result, err := sf.bot.services.Grader.CompleteSession(
		ctx,
		sessionID,
		cb.From.ID,
	)
	if err != nil {
		slog.ErrorContext(
			ctx,
			"Failed to complete session",
			"event",
			"telegram.session.complete_failed",
			"session_id",
			sessionID,
			"error",
			err,
		)
		return
	}
	for _, questionID := range wordOrderQuestionIDs {
		sf.deleteWordOrderDraft(
			ctx,
			sessionID,
			questionID,
		)
	}

	accuracy := float64(0)
	if result.TotalQuestions > 0 {
		accuracy = float64(result.CorrectCount) / float64(result.TotalQuestions) * 100
	}

	// Build wrong answers summary
	wrongSummary := ""
	if len(result.WrongAnswers) > 0 {
		wrongSummary = botMessagesByLocale[botDefaultLocale].sessionCompletionWrongAnswerHeading
		for _, wa := range result.WrongAnswers {
			if wa.SessionQuestion.IsCorrect == nil || *wa.SessionQuestion.IsCorrect {
				continue
			}
			q := wa.Question
			if q.Type == model.QuestionKanaHandwriting {
				wrongSummary += fmt.Sprintf(
					botMessagesByLocale[botDefaultLocale].sessionCompletionKanaAnswerFormat,
					truncate(
						stripHTML(q.Prompt),
						30,
					),
					q.CorrectAnswer,
				)
				continue
			}
			answer := formatSessionAnswer(wa.SessionQuestion.UserAnswer)
			wrongSummary += fmt.Sprintf(
				botMessagesByLocale[botDefaultLocale].sessionCompletionAnswerFormat,
				truncate(
					stripHTML(q.Prompt),
					30,
				),
				answer,
				q.CorrectAnswer,
			)
		}
	}

	text := fmt.Sprintf(
		botMessagesByLocale[botDefaultLocale].sessionCompletionFormat,
		result.CorrectCount,
		result.TotalQuestions,
		accuracy,
		wrongSummary,
	)

	sf.bot.EditMessage(
		cb.Message.Chat.ID,
		cb.Message.MessageID,
		text,
		mainMenuKeyboard(),
	)
}

func (sf *SessionFlow) PushSession(
	ctx context.Context,
	chatID int64,
	sessionID int,
	sessionType string,
) error {
	emoji := "📚"
	label := botMessagesByLocale[botDefaultLocale].sessionPushStudyLabel
	if sessionType == "evening" {
		emoji = "🌙"
		label = botMessagesByLocale[botDefaultLocale].sessionPushReviewLabel
	}

	text := fmt.Sprintf(
		botMessagesByLocale[botDefaultLocale].sessionPushFormat,
		emoji,
		label,
	)

	// 여러 row를 합쳐 줌
	keyboard := tgbotapi.NewInlineKeyboardMarkup(
		// 버튼을 가로로 배치할 row 생성
		tgbotapi.NewInlineKeyboardRow(
			// 버튼 1개 만들기
			tgbotapi.NewInlineKeyboardButtonData(
				botMessagesByLocale[botDefaultLocale].startButton,
				fmt.Sprintf(
					formatSessionStart,
					sessionID,
				),
			),
		),
	)

	// 위해서 만든 keyboard를 send
	return sf.bot.SendMessageWithKeyboard(
		chatID,
		text,
		keyboard,
	)
}
