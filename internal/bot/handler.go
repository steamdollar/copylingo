package bot

import (
	"context"
	"fmt"
	"log"
	"log/slog"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"

	"github.com/lsj/copylingo/internal/callback"
	"github.com/lsj/copylingo/internal/model"
	"github.com/lsj/copylingo/internal/observability"
	"github.com/lsj/copylingo/internal/service"
)

// brTagPattern matches <br> variants. Telegram HTML parse mode rejects them ("Unsupported start tag").
var brTagPattern = regexp.MustCompile(`(?i)<br\s*/?>`)

func sanitizeTelegramHTML(text string) string {
	return brTagPattern.ReplaceAllString(
		text,
		"\n",
	)
}

type BotAPI interface {
	Send(chattable tgbotapi.Chattable) (tgbotapi.Message, error)
	Request(chattable tgbotapi.Chattable) (*tgbotapi.APIResponse, error)
	GetUpdatesChan(config tgbotapi.UpdateConfig) tgbotapi.UpdatesChannel
	StopReceivingUpdates()
}

type commandSession interface {
	DueReviewCount(
		ctx context.Context,
		userID int64,
		language,
		level string,
	) (int, error)
	BuildStudy(
		ctx context.Context,
		user model.User,
		profile service.StudySessionProfile,
		limit int,
	) (*model.Session, error)
	BuildMorningQuiz(
		ctx context.Context,
		user model.User,
	) (*model.Session, error)
}

type userStatsReader interface {
	GetUserStats(
		ctx context.Context,
		userID int64,
	) (*model.UserStats, error)
	GetLevelProgress(
		ctx context.Context,
		userID int64,
	) ([]model.LevelProgress, error)
}

type inputClearer interface {
	ClearInput(
		ctx context.Context,
		chatID int64,
		userID *int64,
	) error
}

type BotDependencies struct {
	Telegram        *TelegramClient
	User            userReader
	Session         commandSession
	Analyzer        userStatsReader
	Input           inputClearer
	SessionFlow     *SessionFlow
	StudyFlow       *StudyFlow
	SettingsFlow    *SettingsFlow
	LLMQuestionFlow *LLMQuestionFlow
}

type Bot struct {
	telegram        *TelegramClient
	user            userReader
	session         commandSession
	analyzer        userStatsReader
	input           inputClearer
	sessionFlow     *SessionFlow
	studyFlow       *StudyFlow
	settingsFlow    *SettingsFlow
	llmQuestionFlow *LLMQuestionFlow
	stopChannel     chan struct{}
	// loopDone closes when Start's polling loop returns. Stop waits on it before handlers so none is added late.
	loopDone chan struct{}
	handlers sync.WaitGroup
}

func NewBot(dependencies BotDependencies) *Bot {
	return &Bot{
		telegram:        dependencies.Telegram,
		user:            dependencies.User,
		session:         dependencies.Session,
		analyzer:        dependencies.Analyzer,
		input:           dependencies.Input,
		sessionFlow:     dependencies.SessionFlow,
		studyFlow:       dependencies.StudyFlow,
		settingsFlow:    dependencies.SettingsFlow,
		llmQuestionFlow: dependencies.LLMQuestionFlow,
		stopChannel:     make(chan struct{}),
		loopDone:        make(chan struct{}),
	}
}

func (bot *Bot) Start() {
	defer close(bot.loopDone)
	defer log.Println("Telegram bot stopped")

	pollConfig := tgbotapi.NewUpdate(0)
	pollConfig.Timeout = 60

	updates := bot.telegram.Updates(pollConfig)

	for {
		select {
		case update, ok := <-updates:
			// StopUpdates closes the channel; stop instead of handling empty updates.
			if !ok {
				return
			}
			bot.handlers.Add(1)
			go func() {
				defer bot.handlers.Done()
				bot.handleUpdate(update)
			}()
		case <-bot.stopChannel:
			return
		}
	}
}

// Stop waits for the polling loop and in-flight handlers so callers can close DB/Redis afterwards.
// It must follow Start.
func (bot *Bot) Stop(ctx context.Context) error {
	close(bot.stopChannel)
	bot.telegram.StopUpdates()

	handlersDone := make(chan struct{})
	go func() {
		<-bot.loopDone
		bot.handlers.Wait()
		close(handlersDone)
	}()
	select {
	case <-handlersDone:
		return nil
	case <-ctx.Done():
		return fmt.Errorf(
			"wait for in-flight Telegram updates: %w",
			ctx.Err(),
		)
	}
}

func (bot *Bot) handleUpdate(update tgbotapi.Update) {
	startedAt := time.Now()
	ctx := observability.WithAttributes(
		context.Background(),
		telegramUpdateAttributes(update)...,
	)

	if update.Message != nil {
		bot.handleMessage(
			ctx,
			update.Message,
		)
	} else if update.CallbackQuery != nil {
		bot.handleCallback(
			ctx,
			update.CallbackQuery,
		)
	}

	slog.InfoContext(
		ctx,
		"Telegram update completed",
		"event",
		"telegram.update.completed",
		"duration_ms",
		time.Since(startedAt).Milliseconds(),
	)
}

func telegramUpdateAttributes(update tgbotapi.Update) []slog.Attr {
	interactionID := observability.NewInteractionID("tg")
	if update.UpdateID > 0 {
		interactionID = fmt.Sprintf(
			"tg-%d",
			update.UpdateID,
		)
	}
	attributes := []slog.Attr{
		slog.String(
			"interaction_id",
			interactionID,
		),
		slog.String(
			"source",
			"telegram",
		),
		slog.Int(
			"update_id",
			update.UpdateID,
		),
	}

	switch {
	case update.Message != nil:
		attributes = append(
			attributes,
			slog.String(
				"update_type",
				"message",
			),
		)
		if update.Message.From != nil {
			attributes = append(
				attributes,
				slog.Int64(
					"user_id",
					update.Message.From.ID,
				),
			)
		}
		if update.Message.Chat != nil {
			attributes = append(
				attributes,
				slog.Int64(
					"chat_id",
					update.Message.Chat.ID,
				),
			)
		}
		if update.Message.IsCommand() {
			attributes = append(
				attributes,
				slog.String(
					"command",
					update.Message.Command(),
				),
			)
		}
	case update.CallbackQuery != nil:
		attributes = append(
			attributes,
			slog.String(
				"update_type",
				"callback",
			),
			slog.String(
				"callback_type",
				callbackType(update.CallbackQuery.Data),
			),
		)
		if update.CallbackQuery.From != nil {
			attributes = append(
				attributes,
				slog.Int64(
					"user_id",
					update.CallbackQuery.From.ID,
				),
			)
		}
		if update.CallbackQuery.Message != nil && update.CallbackQuery.Message.Chat != nil {
			attributes = append(
				attributes,
				slog.Int64(
					"chat_id",
					update.CallbackQuery.Message.Chat.ID,
				),
			)
		}
		attributes = append(
			attributes,
			callbackIDAttributes(update.CallbackQuery.Data)...,
		)
	default:
		attributes = append(
			attributes,
			slog.String(
				"update_type",
				"unknown",
			),
		)
	}
	return attributes
}

func callbackType(data string) string {
	switch {
	case data == callbackLLMCancel:
		return "llm"
	case strings.HasPrefix(
		data,
		callbackPrefixMenu,
	):
		return "menu"
	case strings.HasPrefix(
		data,
		callbackPrefixSettings,
	):
		return "settings"
	case strings.HasPrefix(
		data,
		callbackPrefixSession,
	):
		return "session"
	case strings.HasPrefix(
		data,
		callback.QuestionPrefix,
	):
		return "question"
	case strings.HasPrefix(
		data,
		callbackPrefixStudy,
	):
		return "study"
	default:
		return "unknown"
	}
}

func callbackIDAttributes(data string) []slog.Attr {
	parts := strings.Split(
		data,
		":",
	)
	if len(parts) < 2 {
		return nil
	}
	sessionID, err := strconv.Atoi(parts[1])
	if err != nil {
		return nil
	}
	attributes := []slog.Attr{slog.Int(
		"session_id",
		sessionID,
	)}
	if strings.HasPrefix(
		data,
		callback.QuestionPrefix,
	) && len(parts) >= 3 && parts[2] != callback.QuestionActionNext {
		if questionID, err := strconv.Atoi(parts[2]); err == nil {
			attributes = append(
				attributes,
				slog.Int(
					"question_id",
					questionID,
				),
			)
		}
	}
	return attributes
}

func (bot *Bot) handleMessage(
	ctx context.Context,
	message *tgbotapi.Message,
) {
	if !message.IsCommand() {
		if handled := bot.llmQuestionFlow.handleLLMQuestion(
			ctx,
			message,
		); handled {
			return
		}
		if bot.sessionFlow.HandleTextInput(
			ctx,
			message,
		) {
			return
		}
		// No pending text input: resume an in-progress Quiz instead of leaving the user stuck.
		if bot.sessionFlow.ResumeQuizFromText(
			ctx,
			message,
		) {
			return
		}
		// No reply is sent. Log it to tell "bot went silent" from a failed send.
		slog.InfoContext(
			ctx,
			"Text message ignored: no pending text input",
			"event",
			"telegram.message.ignored",
		)
		return
	}

	switch botCommand(message.Command()) {
	case commandStart:
		bot.handleStart(message)
	case commandMenu:
		bot.handleMenu(
			ctx,
			message,
		)
	case commandStats:
		bot.handleStats(
			ctx,
			message,
		)
	case commandStreak:
		bot.handleStreak(
			ctx,
			message,
		)
	case commandStudy:
		bot.handleStudy(
			ctx,
			message,
		)
	case commandLLM:
		bot.llmQuestionFlow.handleLLM(
			ctx,
			message,
		)
	case commandTest:
		bot.handleTest(
			ctx,
			message,
		)
	case commandHelp:
		bot.handleHelp(
			ctx,
			message,
		)
	case commandExit:
		bot.handleExit(
			ctx,
			message,
		)
	case commandSettings:
		bot.settingsFlow.handleSettingsCommand(
			ctx,
			message,
		)
	default:
		bot.telegram.SendMessage(
			message.Chat.ID,
			botMessagesByLocale[botDefaultLocale].unknownCommand,
		)
	}
}

func (bot *Bot) handleCallback(
	ctx context.Context,
	callbackQuery *tgbotapi.CallbackQuery,
) {
	// Acknowledge callback to remove loading indicator
	bot.telegram.AnswerCallback(
		callbackQuery.ID,
		"",
	)

	data := callbackQuery.Data

	switch {
	case data == callbackLLMCancel:
		bot.llmQuestionFlow.handleLLMCancel(
			ctx,
			callbackQuery,
		)
	case data == callbackMenuMain:
		bot.showMainMenu(
			ctx,
			callbackQuery.Message.Chat.ID,
			callbackQuery.From,
		)
	case data == callbackMenuStudy:
		bot.sessionFlow.StartStudy(
			ctx,
			callbackQuery,
		)
	case data == callbackMenuReview:
		bot.sessionFlow.StartReview(
			ctx,
			callbackQuery,
		)
	case data == callbackMenuStats:
		bot.handleStatsCallback(
			ctx,
			callbackQuery,
		)
	case data == callbackStatsProgress:
		bot.handleStatsProgressCallback(
			ctx,
			callbackQuery,
		)
	case data == callbackMenuSettings || strings.HasPrefix(
		data,
		callbackPrefixSettings,
	):
		bot.settingsFlow.handleSettingsCallback(
			ctx,
			callbackQuery,
		)
	case strings.HasPrefix(
		data,
		callbackPrefixSession,
	):
		bot.sessionFlow.HandleSessionCallback(
			ctx,
			callbackQuery,
		)
	case strings.HasPrefix(
		data,
		callback.QuestionPrefix,
	):
		bot.sessionFlow.HandleAnswerCallback(
			ctx,
			callbackQuery,
		)
	case strings.HasPrefix(
		data,
		callbackPrefixStudy,
	):
		bot.studyFlow.HandleCallback(
			ctx,
			callbackQuery,
		)
	}
}

func (bot *Bot) handleStart(message *tgbotapi.Message) {
	bot.telegram.SendMessage(
		message.Chat.ID,
		botMessagesByLocale[botDefaultLocale].welcomeMessage,
	)
}

func (bot *Bot) handleMenu(
	ctx context.Context,
	message *tgbotapi.Message,
) {
	bot.showMainMenu(
		ctx,
		message.Chat.ID,
		message.From,
	)
}

func (bot *Bot) showMainMenu(
	ctx context.Context,
	chatID int64,
	from *tgbotapi.User,
) {
	user, err := bot.user.GetUser(
		ctx,
		from.ID,
		from.UserName,
	)
	if err != nil {
		slog.ErrorContext(
			ctx,
			"Failed to get user for main menu",
			"event",
			"telegram.menu.user_lookup_failed",
			"error",
			err,
		)
	}

	streakEmoji := "🔥"
	if user != nil && user.StreakDays == 0 {
		streakEmoji = "💤"
	}

	var language, level string

	streakDays := 0
	if user != nil {
		streakDays = user.StreakDays
		language = user.Language
		level = user.ProficiencyLevel
	}
	reviewCount, _ := bot.session.DueReviewCount(
		ctx,
		from.ID,
		language,
		level,
	)

	languageName := languageDisplayName(language)
	text := fmt.Sprintf(
		botMessagesByLocale[botDefaultLocale].mainMenuFormat,
		streakEmoji,
		streakDays,
		languageName,
		level,
	)

	messages := botMessagesByLocale[botDefaultLocale]
	reviewLabel := fmt.Sprintf(
		messages.reviewMenuButtonFormat,
		reviewCount,
	)

	keyboard := tgbotapi.NewInlineKeyboardMarkup(
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData(
				messages.studyMenuButton,
				callbackMenuStudy,
			),
			tgbotapi.NewInlineKeyboardButtonData(
				reviewLabel,
				callbackMenuReview,
			),
		),
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData(
				messages.statsMenuButton,
				callbackMenuStats,
			),
			tgbotapi.NewInlineKeyboardButtonData(
				messages.settingsMenuButton,
				callbackMenuSettings,
			),
		),
	)

	bot.telegram.SendMessageWithKeyboard(
		chatID,
		text,
		keyboard,
	)
}

func (bot *Bot) handleStats(
	ctx context.Context,
	message *tgbotapi.Message,
) {
	stats, err := bot.analyzer.GetUserStats(
		ctx,
		message.From.ID,
	)
	if err != nil {
		bot.telegram.SendMessage(
			message.Chat.ID,
			botMessagesByLocale[botDefaultLocale].statsCommandFailed,
		)
		return
	}

	text := fmt.Sprintf(
		botMessagesByLocale[botDefaultLocale].statsOverviewFormat,
		stats.TodayQuestions,
		stats.OverallAccuracy,
		stats.CurrentStreak,
		stats.VocabularyAccuracy,
		stats.GrammarAccuracy,
		stats.KanjiAccuracy,
		stats.ReadingAccuracy,
		stats.ListeningAccuracy,
	)

	bot.telegram.SendMessage(
		message.Chat.ID,
		text,
	)
}

func (bot *Bot) handleStatsCallback(
	ctx context.Context,
	callbackQuery *tgbotapi.CallbackQuery,
) {
	stats, err := bot.analyzer.GetUserStats(
		ctx,
		callbackQuery.From.ID,
	)
	if err != nil {
		return
	}

	text := fmt.Sprintf(
		botMessagesByLocale[botDefaultLocale].statsMenuFormat,
		stats.TodayQuestions,
		stats.OverallAccuracy,
		stats.CurrentStreak,
		stats.VocabularyAccuracy,
		stats.GrammarAccuracy,
		stats.KanjiAccuracy,
		stats.ReadingAccuracy,
		stats.ListeningAccuracy,
	)

	keyboard := statsTabKeyboard(callbackMenuStats)
	bot.telegram.EditMessage(
		callbackQuery.Message.Chat.ID,
		callbackQuery.Message.MessageID,
		text,
		&keyboard,
	)
}

func (bot *Bot) handleStatsProgressCallback(
	ctx context.Context,
	callbackQuery *tgbotapi.CallbackQuery,
) {
	messages := botMessagesByLocale[botDefaultLocale]
	progress, err := bot.analyzer.GetLevelProgress(
		ctx,
		callbackQuery.From.ID,
	)
	if err != nil {
		slog.ErrorContext(
			ctx,
			"Failed to get level progress for stats callback",
			slog.Any(
				"error",
				err,
			),
		)
		bot.telegram.SendMessage(
			callbackQuery.Message.Chat.ID,
			messages.statsCommandFailed,
		)
		return
	}

	lines := make(
		[]string,
		0,
		len(progress),
	)
	for _, level := range progress {
		lines = append(
			lines,
			fmt.Sprintf(
				messages.statsProgressLineFormat,
				level.Level,
				progressBar(level.Percent()),
				level.Percent(),
				level.StudiedMaterials,
				level.TotalMaterials,
			),
		)
	}
	body := messages.statsProgressEmpty
	if len(lines) > 0 {
		body = strings.Join(
			lines,
			"\n",
		)
	}

	keyboard := statsTabKeyboard(callbackStatsProgress)
	bot.telegram.EditMessage(
		callbackQuery.Message.Chat.ID,
		callbackQuery.Message.MessageID,
		fmt.Sprintf(
			messages.statsProgressFormat,
			body,
		),
		&keyboard,
	)
}

func statsTabKeyboard(activeCallback string) tgbotapi.InlineKeyboardMarkup {
	messages := botMessagesByLocale[botDefaultLocale]
	tabLabel := func(
		label string,
		callbackData string,
	) string {
		if callbackData != activeCallback {
			return label
		}
		return fmt.Sprintf(
			messages.statsActiveTabFormat,
			label,
		)
	}

	return tgbotapi.NewInlineKeyboardMarkup(
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData(
				tabLabel(
					messages.statsAccuracyTabButton,
					callbackMenuStats,
				),
				callbackMenuStats,
			),
			tgbotapi.NewInlineKeyboardButtonData(
				tabLabel(
					messages.statsProgressTabButton,
					callbackStatsProgress,
				),
				callbackStatsProgress,
			),
		),
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData(
				messages.menuHomeButton,
				callbackMenuMain,
			),
		),
	)
}

func progressBar(percent int) string {
	const progressBarCells = 10
	filledCells := percent / 10
	return strings.Repeat(
		"▓",
		filledCells,
	) + strings.Repeat(
		"░",
		progressBarCells-filledCells,
	)
}

func (bot *Bot) handleStreak(
	ctx context.Context,
	message *tgbotapi.Message,
) {
	stats, err := bot.analyzer.GetUserStats(
		ctx,
		message.From.ID,
	)
	if err != nil {
		bot.telegram.SendMessage(
			message.Chat.ID,
			botMessagesByLocale[botDefaultLocale].streakCommandFailed,
		)
		return
	}

	text := fmt.Sprintf(
		botMessagesByLocale[botDefaultLocale].streakFormat,
		stats.CurrentStreak,
	)
	bot.telegram.SendMessage(
		message.Chat.ID,
		text,
	)
}

func (bot *Bot) handleHelp(
	_ context.Context,
	message *tgbotapi.Message,
) {
	bot.telegram.SendMessage(
		message.Chat.ID,
		botMessagesByLocale[botDefaultLocale].helpMessage,
	)
}

func (bot *Bot) handleExit(
	ctx context.Context,
	message *tgbotapi.Message,
) {
	var userID *int64
	if message.From != nil {
		userID = &message.From.ID
	}
	_ = bot.input.ClearInput(
		ctx,
		message.Chat.ID,
		userID,
	)
	bot.telegram.SendMessage(
		message.Chat.ID,
		botMessagesByLocale[botDefaultLocale].exitMessage,
	)
}

func (bot *Bot) handleStudy(
	ctx context.Context,
	message *tgbotapi.Message,
) {
	limit, err := parseStudyCommandLimit(message.CommandArguments())
	if err != nil {
		bot.telegram.SendMessage(
			message.Chat.ID,
			fmt.Sprintf(
				botMessagesByLocale[botDefaultLocale].studyCommandUsageFormat,
				service.MaxStudySessionMaterialCount,
			),
		)
		return
	}

	user, err := bot.user.GetUser(
		ctx,
		message.From.ID,
		message.From.UserName,
	)
	if err != nil {
		slog.ErrorContext(
			ctx,
			"Failed to get user for study session",
			"event",
			"telegram.study.command_user_lookup_failed",
			"error",
			err,
		)
		bot.telegram.SendMessage(
			message.Chat.ID,
			botMessagesByLocale[botDefaultLocale].userUnavailable,
		)
		return
	}

	session, err := bot.session.BuildStudy(
		ctx,
		*user,
		service.StudyProfileMorning,
		limit,
	)
	if err != nil {
		slog.ErrorContext(
			ctx,
			"Failed to build study session from command",
			"event",
			"telegram.study.command_build_failed",
			"user_id",
			user.ID,
			"limit",
			limit,
			"error",
			err,
		)
		bot.telegram.SendMessage(
			message.Chat.ID,
			botMessagesByLocale[botDefaultLocale].studyCommandBuildFailed,
		)
		return
	}
	if session == nil {
		bot.telegram.SendMessage(
			message.Chat.ID,
			botMessagesByLocale[botDefaultLocale].studyCommandNoMaterials,
		)
		return
	}

	if err := bot.studyFlow.PushSession(
		ctx,
		message.Chat.ID,
		session.ID,
	); err != nil {
		slog.ErrorContext(
			ctx,
			"Failed to push study session from command",
			"event",
			"telegram.study.command_push_failed",
			"user_id",
			user.ID,
			"session_id",
			session.ID,
			"error",
			err,
		)
		bot.telegram.SendMessage(
			message.Chat.ID,
			botMessagesByLocale[botDefaultLocale].studyCommandPushFailed,
		)
		return
	}

	slog.InfoContext(
		ctx,
		"Study session triggered from command",
		"event",
		"telegram.study.command_triggered",
		"user_id",
		user.ID,
		"session_id",
		session.ID,
		"limit",
		limit,
	)
}

func parseStudyCommandLimit(arguments string) (int, error) {
	arguments = strings.TrimSpace(arguments)
	if arguments == "" {
		return service.DefaultStudySessionMaterialCount, nil
	}
	fields := strings.Fields(arguments)
	if len(fields) != 1 {
		return 0, fmt.Errorf("expected one study limit argument")
	}
	limit, err := strconv.Atoi(fields[0])
	if err != nil {
		return 0, fmt.Errorf(
			"parse study limit: %w",
			err,
		)
	}
	if limit <= 0 || limit > service.MaxStudySessionMaterialCount {
		return 0, fmt.Errorf(
			"study limit out of range: %d",
			limit,
		)
	}
	return limit, nil
}

func (bot *Bot) handleTest(
	ctx context.Context,
	message *tgbotapi.Message,
) {
	user, err := bot.user.GetUser(
		ctx,
		message.From.ID,
		message.From.UserName,
	)
	if err != nil {
		slog.ErrorContext(
			ctx,
			"Failed to get user for test session",
			"event",
			"telegram.test.user_lookup_failed",
			"error",
			err,
		)
		bot.telegram.SendMessage(
			message.Chat.ID,
			botMessagesByLocale[botDefaultLocale].userUnavailable,
		)
		return
	}

	session, err := bot.session.BuildMorningQuiz(
		ctx,
		*user,
	)
	if err != nil {
		slog.ErrorContext(
			ctx,
			"Failed to build test session",
			"event",
			"telegram.test.session_build_failed",
			"user_id",
			user.ID,
			"error",
			err,
		)
		bot.telegram.SendMessage(
			message.Chat.ID,
			botMessagesByLocale[botDefaultLocale].testSessionBuildFailed,
		)
		return
	}

	if session == nil {
		bot.telegram.SendMessage(
			message.Chat.ID,
			botMessagesByLocale[botDefaultLocale].testSessionNoQuestions,
		)
		return
	}

	if err := bot.sessionFlow.PushSession(
		ctx,
		user.ID,
		session.ID,
		"morning",
	); err != nil {
		slog.ErrorContext(
			ctx,
			"Failed to push test session",
			"event",
			"telegram.test.session_push_failed",
			"user_id",
			user.ID,
			"session_id",
			session.ID,
			"error",
			err,
		)
		bot.telegram.SendMessage(
			message.Chat.ID,
			botMessagesByLocale[botDefaultLocale].testSessionPushFailed,
		)
		return
	}

	slog.InfoContext(
		ctx,
		"Test session triggered",
		"event",
		"telegram.test.session_triggered",
		"user_id",
		user.ID,
		"session_id",
		session.ID,
	)
}

func languageDisplayName(code string) string {
	messages := botMessagesByLocale[botDefaultLocale]
	switch code {
	case "ja":
		return messages.languageJapanese
	case "el":
		return messages.languageGreek
	case "en":
		return messages.languageEnglish
	case "ko":
		return messages.languageKorean
	default:
		return code
	}
}
