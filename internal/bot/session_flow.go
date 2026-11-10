package bot

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"strconv"
	"strings"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"

	"github.com/lsj/copylingo/internal/callback"
	"github.com/lsj/copylingo/internal/model"
	"github.com/lsj/copylingo/internal/service"
)

type quizSession interface {
	StartQuiz(
		ctx context.Context,
		sessionID int,
		userID int64,
	) (*model.QuizActiveSessionState, error)
	ShowQuizQuestion(
		ctx context.Context,
		sessionID,
		questionIndex int,
	) (*model.QuizActiveSessionState, error)
	SubmitQuizOption(
		ctx context.Context,
		userID int64,
		sessionID,
		questionID,
		optionIndex int,
	) (*service.QuizAnswerResult, error)
	SubmitQuizText(
		ctx context.Context,
		answer service.QuizTextAnswer,
	) (*service.QuizAnswerResult, error)
	SubmitQuizWordOrder(
		ctx context.Context,
		userID int64,
		sessionID,
		questionID int,
		selection []int,
	) (*service.QuizAnswerResult, error)
	CompleteQuiz(
		ctx context.Context,
		sessionID int,
		userID int64,
	) (*service.QuizCompletion, error)
	BuildReviewQuiz(
		ctx context.Context,
		user model.User,
	) (*model.Session, error)
	ListByStatus(
		ctx context.Context,
		userID int64,
		status model.SessionStatus,
	) ([]model.Session, error)
	QuizProgress(
		ctx context.Context,
		sessionID int,
	) (*model.QuizActiveSessionState, error)
	ListInProgressQuizzes(ctx context.Context) ([]model.Session, error)
}

type userReader interface {
	GetUser(
		ctx context.Context,
		telegramID int64,
		username string,
	) (*model.User, error)
}

// listeningAudio may be nil, then listening questions go out as text only (ADR-082).
type listeningAudio interface {
	GetClip(
		ctx context.Context,
		key string,
	) ([]byte, error)
	CacheFileID(
		ctx context.Context,
		questionID int,
		fileID string,
	) error
}

type quizInputStore interface {
	GetActiveQuestion(
		ctx context.Context,
		chatID int64,
	) (*model.ActiveQuestionReference, error)
	SetActiveQuestion(
		ctx context.Context,
		chatID int64,
		question model.ActiveQuestionReference,
	) error
	DeleteActiveQuestion(
		ctx context.Context,
		chatID int64,
	) error
	SetLLMPending(
		ctx context.Context,
		userID int64,
		input model.PendingLLMInput,
	) error
}

// Audio may be nil (see listeningAudio), every other dependency is required.
type SessionFlowDependencies struct {
	Telegram           *TelegramClient
	Session            quizSession
	User               userReader
	MaterialPreference materialPreferences
	Audio              listeningAudio
	Input              quizInputStore
	Drafts             WordOrderDraftStore
	Messages           HandwritingMessageStore
	Recovery           MiniAppRecoveryStore
	Study              *StudyFlow
	PublicBaseURL      string
}

type SessionFlow struct {
	telegram           *TelegramClient
	session            quizSession
	user               userReader
	materialPreference materialPreferences
	audio              listeningAudio
	input              quizInputStore
	drafts             WordOrderDraftStore
	messages           HandwritingMessageStore
	recovery           MiniAppRecoveryStore
	study              *StudyFlow
	publicBaseURL      string
}

func NewSessionFlow(dependencies SessionFlowDependencies) *SessionFlow {
	return &SessionFlow{
		telegram:           dependencies.Telegram,
		session:            dependencies.Session,
		user:               dependencies.User,
		materialPreference: dependencies.MaterialPreference,
		audio:              dependencies.Audio,
		input:              dependencies.Input,
		drafts:             dependencies.Drafts,
		messages:           dependencies.Messages,
		recovery:           dependencies.Recovery,
		study:              dependencies.Study,
		publicBaseURL:      dependencies.PublicBaseURL,
	}
}

func (flow *SessionFlow) StartStudy(
	ctx context.Context,
	callbackQuery *tgbotapi.CallbackQuery,
) {
	resumed, err := flow.getInProgressSessions(
		ctx,
		callbackQuery,
	)
	if err != nil {
		slog.ErrorContext(
			ctx,
			"Failed to fetch in-progress sessions",
			"event",
			"telegram.session.in_progress_lookup_failed",
			"user_id",
			callbackQuery.From.ID,
			"error",
			err,
		)
		flow.showSessionFetchError(callbackQuery)
		return
	}
	if resumed {
		return
	}

	if err := flow.getPendingSessions(
		ctx,
		callbackQuery,
	); err != nil {
		slog.ErrorContext(
			ctx,
			"Failed to fetch pending sessions",
			"event",
			"telegram.session.pending_lookup_failed",
			"user_id",
			callbackQuery.From.ID,
			"error",
			err,
		)
		flow.showSessionFetchError(callbackQuery)
	}
}

func (flow *SessionFlow) getPendingSessions(
	ctx context.Context,
	callbackQuery *tgbotapi.CallbackQuery,
) error {
	chatID := callbackQuery.Message.Chat.ID
	sessions, err := flow.session.ListByStatus(
		ctx,
		callbackQuery.From.ID,
		model.SessionPending,
	)
	if err != nil {
		return err
	}
	if len(sessions) == 0 {
		flow.telegram.EditMessage(
			chatID,
			callbackQuery.Message.MessageID,
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
		flow.telegram.EditMessage(
			chatID,
			callbackQuery.Message.MessageID,
			text,
			&keyboard,
		)
		return nil
	}
	session, ok := firstQuizSession(sessions)
	if !ok {
		flow.telegram.EditMessage(
			chatID,
			callbackQuery.Message.MessageID,
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
	flow.telegram.EditMessage(
		chatID,
		callbackQuery.Message.MessageID,
		text,
		&keyboard,
	)
	return nil
}

func (flow *SessionFlow) getInProgressSessions(
	ctx context.Context,
	callbackQuery *tgbotapi.CallbackQuery,
) (bool, error) {
	chatID := callbackQuery.Message.Chat.ID
	inProgressSessions, err := flow.session.ListByStatus(
		ctx,
		callbackQuery.From.ID,
		model.SessionInProgress,
	)
	if err != nil {
		return false, err
	}
	if studySession, ok := firstStudySession(inProgressSessions); ok {
		flow.study.startSession(
			ctx,
			callbackQuery,
			studySession.ID,
		)
		return true, nil
	}
	session, ok := firstQuizSession(inProgressSessions)
	if !ok {
		return false, nil
	}

	nextIndex, err := flow.nextUnansweredQuestionIndex(
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
		flow.telegram.EditMessage(
			chatID,
			callbackQuery.Message.MessageID,
			botMessagesByLocale[botDefaultLocale].activeSessionUnavailable,
			mainMenuKeyboard(),
		)
		return true, nil
	}
	editMessageID := callbackQuery.Message.MessageID
	flow.showQuestion(
		ctx,
		chatID,
		&editMessageID,
		session.ID,
		nextIndex,
	)
	return true, nil
}

func (flow *SessionFlow) StartReview(
	ctx context.Context,
	callbackQuery *tgbotapi.CallbackQuery,
) {
	userID := callbackQuery.From.ID
	chatID := callbackQuery.Message.Chat.ID

	user, err := flow.user.GetUser(
		ctx,
		userID,
		callbackQuery.From.UserName,
	)
	if err != nil {
		flow.telegram.SendMessage(
			chatID,
			botMessagesByLocale[botDefaultLocale].reviewUserLoadFailed,
		)
		return
	}

	session, err := flow.session.BuildReviewQuiz(
		ctx,
		*user,
	)
	if errors.Is(
		err,
		service.ErrNoDueReviews,
	) {
		flow.telegram.EditMessage(
			chatID,
			callbackQuery.Message.MessageID,
			botMessagesByLocale[botDefaultLocale].noReviewQuestions,
			mainMenuKeyboard(),
		)
		return
	}
	if err != nil || session == nil {
		flow.telegram.SendMessage(
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

	flow.telegram.EditMessage(
		chatID,
		callbackQuery.Message.MessageID,
		text,
		&keyboard,
	)
}

func (flow *SessionFlow) HandleSessionCallback(
	ctx context.Context,
	callbackQuery *tgbotapi.CallbackQuery,
) {
	parts := strings.Split(
		callbackQuery.Data,
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
		flow.startSession(
			ctx,
			callbackQuery,
			sessionID,
		)
	case callbackActionFinish:
		flow.finishSession(
			ctx,
			callbackQuery,
			sessionID,
		)
	}
}

func (flow *SessionFlow) HandleAnswerCallback(
	ctx context.Context,
	callbackQuery *tgbotapi.CallbackQuery,
) {
	// Format: q:{sessionID}:{questionID}:{optionIndex} or q:{sessionID}:next:{currentIndex}
	parts := strings.Split(
		callbackQuery.Data,
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
		flow.handleWordOrderCallback(
			ctx,
			callbackQuery,
		)
		return
	}

	if parts[2] == callback.QuestionActionNext {
		if callbackQuery.Message == nil {
			return
		}
		currentIndex, err := strconv.Atoi(parts[3])
		if err != nil {
			return
		}
		if !flow.isQuestionAnswered(
			ctx,
			sessionID,
			currentIndex,
		) {
			if isStaleMiniAppCallback(
				parts,
				flow.publicBaseURL,
			) {
				flow.telegram.ClearInlineKeyboard(
					callbackQuery.Message.Chat.ID,
					callbackQuery.Message.MessageID,
				)
				flow.telegram.SendMessage(
					callbackQuery.Message.Chat.ID,
					botMessagesByLocale[botDefaultLocale].handwritingLinkExpired,
				)
				flow.showQuestion(
					ctx,
					callbackQuery.Message.Chat.ID,
					nil,
					sessionID,
					currentIndex,
				)
				return
			}
			flow.telegram.SendMessage(
				callbackQuery.Message.Chat.ID,
				botMessagesByLocale[botDefaultLocale].handwritingSubmitFirst,
			)
			return
		}
		// 같은 손글씨 제출 결과로 다음 문제가 중복 진행되지 않도록 먼저 버튼을 제거한다.
		flow.telegram.ClearInlineKeyboard(
			callbackQuery.Message.Chat.ID,
			callbackQuery.Message.MessageID,
		)
		// 손글씨는 Mini App HTTP로 제출되므로 원래 메시지를 남기고, 다음 문제는 새 메시지로 렌더링한다.
		flow.showQuestion(
			ctx,
			callbackQuery.Message.Chat.ID,
			nil,
			sessionID,
			currentIndex+1,
		)
		return
	}

	if parts[2] == callbackActionAsk {
		flow.handleAskLLMQuestion(
			ctx,
			callbackQuery,
			sessionID,
			parts[3],
		)
		return
	}
	if parts[2] == callback.QuestionActionPolicy || parts[2] == callbackActionExclude {
		flow.handleQuizMaterialPreference(
			ctx,
			callbackQuery,
			parts,
		)
		return
	}

	questionID, err := strconv.Atoi(parts[2])
	if err != nil {
		return
	}
	optionIndex, err := strconv.Atoi(parts[3])
	if err != nil {
		return
	}

	flow.processAnswer(
		ctx,
		callbackQuery,
		sessionID,
		questionID,
		optionIndex,
	)
}

// One-shot LLM mode scoped to a just-answered question. The next plain-text message is answered with its context.
func (flow *SessionFlow) handleAskLLMQuestion(
	ctx context.Context,
	callbackQuery *tgbotapi.CallbackQuery,
	sessionID int,
	questionIDText string,
) {
	if callbackQuery.Message == nil {
		return
	}
	// owner gate를 callback에서도 재검증한다 (버튼은 owner에게만 렌더되지만 callback은 위조 가능).
	if !isLLMAllowed(callbackQuery.From) {
		return
	}
	questionID, err := strconv.Atoi(questionIDText)
	if err != nil {
		return
	}
	input := model.PendingLLMInput{Kind: model.PendingLLMQuizQuestion, SessionID: sessionID, QuestionID: questionID}
	if err := flow.input.SetLLMPending(
		ctx,
		callbackQuery.From.ID,
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
		flow.telegram.SendMessage(
			callbackQuery.Message.Chat.ID,
			botMessagesByLocale[botDefaultLocale].llmQuestionActivationFailed,
		)
		return
	}
	flow.telegram.SendMessageWithKeyboard(
		callbackQuery.Message.Chat.ID,
		botMessagesByLocale[botDefaultLocale].quizLLMQuestionPrompt,
		llmCancelKeyboard(),
	)
}

func (flow *SessionFlow) startSession(
	ctx context.Context,
	callbackQuery *tgbotapi.CallbackQuery,
	sessionID int,
) {
	state, err := flow.session.StartQuiz(
		ctx,
		sessionID,
		callbackQuery.From.ID,
	)
	if err != nil {
		switch {
		case errors.Is(
			err,
			service.ErrQuizActiveSessionUserMismatch,
		):
			slog.WarnContext(
				ctx,
				"Rejected session start for non-owner",
				"event",
				"telegram.session.start_owner_mismatch",
				"session_id",
				sessionID,
				"user_id",
				callbackQuery.From.ID,
			)
		case errors.Is(
			err,
			service.ErrQuizStatePrepareFailed,
		):
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
			flow.telegram.SendMessage(
				callbackQuery.Message.Chat.ID,
				botMessagesByLocale[botDefaultLocale].sessionStatePrepareFailed,
			)
		default:
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
		}
		return
	}

	editMessageID := callbackQuery.Message.MessageID
	flow.showQuestion(
		ctx,
		callbackQuery.Message.Chat.ID,
		&editMessageID,
		sessionID,
		state.NextUnansweredIndex(),
	)
}

func (flow *SessionFlow) finishSession(
	ctx context.Context,
	callbackQuery *tgbotapi.CallbackQuery,
	sessionID int,
) {
	result, err := flow.session.CompleteQuiz(
		ctx,
		sessionID,
		callbackQuery.From.ID,
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
	// Word-order drafts are ephemeral and must not survive a finished session.
	for _, questionID := range result.WordOrderQuestionIDs {
		flow.deleteWordOrderDraft(
			ctx,
			sessionID,
			questionID,
		)
	}

	accuracy := float64(0)
	if result.TotalQuestions > 0 {
		accuracy = float64(result.CorrectCount) / float64(result.TotalQuestions) * 100
	}

	wrongSummary := ""
	if len(result.WrongAnswers) > 0 {
		wrongSummary = botMessagesByLocale[botDefaultLocale].sessionCompletionWrongAnswerHeading
		for _, wrongAnswer := range result.WrongAnswers {
			if wrongAnswer.SessionQuestion.IsCorrect == nil || *wrongAnswer.SessionQuestion.IsCorrect {
				continue
			}
			question := wrongAnswer.Question
			if question.Type == model.QuestionKanaHandwriting {
				wrongSummary += fmt.Sprintf(
					botMessagesByLocale[botDefaultLocale].sessionCompletionKanaAnswerFormat,
					truncate(
						stripHTML(question.Prompt),
						30,
					),
					question.CorrectAnswer,
				)
				continue
			}
			answer := formatSessionAnswer(wrongAnswer.SessionQuestion.UserAnswer)
			wrongSummary += fmt.Sprintf(
				botMessagesByLocale[botDefaultLocale].sessionCompletionAnswerFormat,
				truncate(
					stripHTML(question.Prompt),
					30,
				),
				answer,
				question.CorrectAnswer,
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

	flow.telegram.EditMessage(
		callbackQuery.Message.Chat.ID,
		callbackQuery.Message.MessageID,
		text,
		mainMenuKeyboard(),
	)
}

func (flow *SessionFlow) PushSession(
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

	keyboard := tgbotapi.NewInlineKeyboardMarkup(
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData(
				botMessagesByLocale[botDefaultLocale].startButton,
				fmt.Sprintf(
					formatSessionStart,
					sessionID,
				),
			),
		),
	)

	return flow.telegram.SendMessageWithKeyboard(
		chatID,
		text,
		keyboard,
	)
}
