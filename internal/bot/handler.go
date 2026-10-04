package bot

import (
	"context"
	"fmt"
	"log"
	"log/slog"
	"regexp"
	"strconv"
	"strings"
	"time"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"

	"github.com/lsj/copylingo/internal/callback"
	"github.com/lsj/copylingo/internal/config"
	"github.com/lsj/copylingo/internal/model"
	"github.com/lsj/copylingo/internal/observability"
	"github.com/lsj/copylingo/internal/service"
)

// brTagPattern matches <br>, <br/>, <br /> (any case). Telegram's HTML parse mode
// rejects <br> ("Unsupported start tag"), so we convert it to a literal newline
// before sending. This protects every outgoing message, including question prompts
// stored in the DB with <br> hints.
var brTagPattern = regexp.MustCompile(`(?i)<br\s*/?>`)

// sanitizeTelegramHTML makes text safe for ParseMode=HTML messages.
func sanitizeTelegramHTML(text string) string {
	return brTagPattern.ReplaceAllString(
		text,
		"\n",
	)
}

// BotAPI defines the interface for Telegram bot interactions to allow mocking.
type BotAPI interface {
	Send(c tgbotapi.Chattable) (tgbotapi.Message, error)
	Request(c tgbotapi.Chattable) (*tgbotapi.APIResponse, error)
	GetUpdatesChan(config tgbotapi.UpdateConfig) tgbotapi.UpdatesChannel
	StopReceivingUpdates()
}

// commandSession is the part of service.SessionService the main menu and
// the /study, /test commands use.
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

// userStatsReader computes the numbers behind /stats and /streak.
type userStatsReader interface {
	GetUserStats(
		ctx context.Context,
		userID int64,
	) (*model.UserStats, error)
}

// inputClearer drops a chat's pending typed answer and LLM mode on /exit.
type inputClearer interface {
	ClearInput(
		ctx context.Context,
		chatID int64,
		userID *int64,
	) error
}

// BotDeps wires the router; every dependency is required.
type BotDeps struct {
	Telegram        *telegramClient
	User            userReader
	Session         commandSession
	Analyzer        userStatsReader
	Input           inputClearer
	SessionFlow     *SessionFlow
	StudyFlow       *StudyFlow
	SettingsFlow    *SettingsFlow
	LLMQuestionFlow *LLMQuestionFlow
}

// Bot routes Telegram updates to the feature flows and serves the menu,
// stats, streak, help, exit, study and test commands itself.
type Bot struct {
	telegram        *telegramClient
	user            userReader
	session         commandSession
	analyzer        userStatsReader
	input           inputClearer
	sessionFlow     *SessionFlow
	studyFlow       *StudyFlow
	settingsFlow    *SettingsFlow
	llmQuestionFlow *LLMQuestionFlow
	stopCh          chan struct{}
}

func NewBot(
	cfg *config.Config,
	services *service.Services,
	stores StateStores,
) (*Bot, error) {
	api, err := tgbotapi.NewBotAPI(cfg.Telegram.Token)
	if err != nil {
		return nil, fmt.Errorf(
			"failed to create Telegram bot: %w",
			err,
		)
	}

	api.Debug = cfg.Telegram.Debug
	log.Printf(
		"Telegram bot authorized as @%s",
		api.Self.UserName,
	)

	return newBot(
		newTelegramClient(api),
		cfg,
		services,
		stores,
	), nil
}

// newBot assembles the feature flows over one Telegram client. It moves to
// cmd/server once every flow has its own Deps (ADR-059 §8 step C).
func newBot(
	telegram *telegramClient,
	cfg *config.Config,
	services *service.Services,
	stores StateStores,
) *Bot {
	study := NewStudyFlow(StudyFlowDeps{
		Telegram:           telegram,
		Session:            services.Session,
		MaterialPreference: services.MaterialPreference,
		Input:              stores.Input,
	})
	sessionDeps := SessionFlowDeps{
		Telegram:           telegram,
		Session:            services.Session,
		User:               services.User,
		MaterialPreference: services.MaterialPreference,
		Input:              stores.Input,
		Drafts:             stores.Drafts,
		Messages:           stores.Messages,
		Recovery:           stores.Recovery,
		Timing:             stores.Timing,
		Study:              study,
		PublicBaseURL:      cfg.Server.PublicBaseURL,
	}
	// Audio is nil without a TTS key; assigning a nil *AudioService would make
	// a non-nil interface and bypass SessionFlow's "audio unavailable" path.
	if services.Audio != nil {
		sessionDeps.Audio = services.Audio
	}
	return newRouter(BotDeps{
		Telegram:    telegram,
		User:        services.User,
		Session:     services.Session,
		Analyzer:    services.Analyzer,
		Input:       stores.Input,
		SessionFlow: NewSessionFlow(sessionDeps),
		StudyFlow:   study,
		SettingsFlow: NewSettingsFlow(SettingsFlowDeps{
			Telegram:           telegram,
			User:               services.User,
			MaterialPreference: services.MaterialPreference,
		}),
		LLMQuestionFlow: NewLLMQuestionFlow(LLMQuestionFlowDeps{
			Telegram:    telegram,
			User:        services.User,
			LLMQuestion: services.LLMQuestion,
			Session:     services.Session,
			Input:       stores.Input,
		}),
	})
}

// newRouter builds the router over already-assembled flows. It becomes
// NewBot once cmd/server assembles the flows (ADR-059 §8 step C).
func newRouter(deps BotDeps) *Bot {
	return &Bot{
		telegram:        deps.Telegram,
		user:            deps.User,
		session:         deps.Session,
		analyzer:        deps.Analyzer,
		input:           deps.Input,
		sessionFlow:     deps.SessionFlow,
		studyFlow:       deps.StudyFlow,
		settingsFlow:    deps.SettingsFlow,
		llmQuestionFlow: deps.LLMQuestionFlow,
		stopCh:          make(chan struct{}),
	}
}

// RefreshStaleMiniAppMessages re-sends handwriting questions whose Mini App
// link went stale across a restart; SessionFlow owns the work.
func (b *Bot) RefreshStaleMiniAppMessages(ctx context.Context) {
	b.sessionFlow.RefreshStaleMiniAppMessages(ctx)
}

// Start begins listening for Telegram updates.
func (b *Bot) Start() {
	// when bot starts, it fetches messges from user that was stored in telegram while bot was offline,
	// and handle them with handleUpdate function.
	pollConfig := tgbotapi.NewUpdate(0)
	pollConfig.Timeout = 60

	// 업데이트 받는 go chan 생성
	updates := b.telegram.Updates(pollConfig)

	for {
		select {
		case update := <-updates:
			go b.handleUpdate(update)
		case <-b.stopCh:
			log.Println("Telegram bot stopped")
			return
		}
	}
}

// Stop signals the bot to stop listening.
func (b *Bot) Stop() {
	close(b.stopCh)
	b.telegram.StopUpdates()
}

// PushSession: push session container message to user
func (b *Bot) PushSession(
	ctx context.Context,
	chatID int64,
	sessionID int,
	sessionType string,
) error {
	return b.sessionFlow.PushSession(
		ctx,
		chatID,
		sessionID,
		sessionType,
	)
}

// PushStudySession: sends a material-based study session start message.
func (b *Bot) PushStudySession(
	ctx context.Context,
	chatID int64,
	sessionID int,
) error {
	return b.studyFlow.PushSession(
		ctx,
		chatID,
		sessionID,
	)
}

// EditMessageReplyMarkup satisfies the Mini App's TelegramMessenger contract.
// It moves to a narrow Flow contract when flows are split (ADR-059 §8 step C).
func (b *Bot) EditMessageReplyMarkup(
	chatID int64,
	messageID int,
	markup tgbotapi.InlineKeyboardMarkup,
) error {
	return b.telegram.EditMessageReplyMarkup(
		chatID,
		messageID,
		markup,
	)
}

func (b *Bot) handleUpdate(update tgbotapi.Update) {
	startedAt := time.Now()
	ctx := observability.WithAttrs(
		context.Background(),
		telegramUpdateAttrs(update)...,
	)

	if update.Message != nil {
		// text input handle
		b.handleMessage(
			ctx,
			update.Message,
		)
	} else if update.CallbackQuery != nil {
		// button click handle
		b.handleCallback(
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

func telegramUpdateAttrs(update tgbotapi.Update) []slog.Attr {
	interactionID := observability.NewInteractionID("tg")
	// updateID: a unique ID for each update received by the bot.
	if update.UpdateID > 0 {
		interactionID = fmt.Sprintf(
			"tg-%d",
			update.UpdateID,
		)
	}
	attrs := []slog.Attr{
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
		attrs = append(
			attrs,
			slog.String(
				"update_type",
				"message",
			),
		)
		if update.Message.From != nil {
			attrs = append(
				attrs,
				slog.Int64(
					"user_id",
					update.Message.From.ID,
				),
			)
		}
		if update.Message.Chat != nil {
			attrs = append(
				attrs,
				slog.Int64(
					"chat_id",
					update.Message.Chat.ID,
				),
			)
		}
		if update.Message.IsCommand() {
			attrs = append(
				attrs,
				slog.String(
					"command",
					update.Message.Command(),
				),
			)
		}
	case update.CallbackQuery != nil:
		attrs = append(
			attrs,
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
			attrs = append(
				attrs,
				slog.Int64(
					"user_id",
					update.CallbackQuery.From.ID,
				),
			)
		}
		if update.CallbackQuery.Message != nil && update.CallbackQuery.Message.Chat != nil {
			attrs = append(
				attrs,
				slog.Int64(
					"chat_id",
					update.CallbackQuery.Message.Chat.ID,
				),
			)
		}
		attrs = append(
			attrs,
			callbackIDAttrs(update.CallbackQuery.Data)...,
		)
	default:
		attrs = append(
			attrs,
			slog.String(
				"update_type",
				"unknown",
			),
		)
	}
	return attrs
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

func callbackIDAttrs(data string) []slog.Attr {
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
	attrs := []slog.Attr{slog.Int(
		"session_id",
		sessionID,
	)}
	if strings.HasPrefix(
		data,
		callback.QuestionPrefix,
	) && len(parts) >= 3 && parts[2] != callback.QuestionActionNext {
		if questionID, err := strconv.Atoi(parts[2]); err == nil {
			attrs = append(
				attrs,
				slog.Int(
					"question_id",
					questionID,
				),
			)
		}
	}
	return attrs
}

func (b *Bot) handleMessage(
	ctx context.Context,
	msg *tgbotapi.Message,
) {
	// if it is nor bot command, check if it is active question answer, if not, ignore or route to chat.
	if !msg.IsCommand() {
		if handled := b.llmQuestionFlow.handleLLMQuestion(
			ctx,
			msg,
		); handled {
			return
		}
		// Route plain text to session flow for FillBlank questions
		b.sessionFlow.HandleTextInput(
			ctx,
			msg,
		)
		return
	}

	switch botCommand(msg.Command()) {
	case commandStart:
		b.handleStart(msg)
	case commandMenu:
		b.handleMenu(
			ctx,
			msg,
		)
	case commandStats:
		b.handleStats(
			ctx,
			msg,
		)
	case commandStreak:
		b.handleStreak(
			ctx,
			msg,
		)
	case commandStudy:
		b.handleStudy(
			ctx,
			msg,
		)
	case commandLLM:
		b.llmQuestionFlow.handleLLM(
			ctx,
			msg,
		)
	case commandTest:
		b.handleTest(
			ctx,
			msg,
		)
	case commandHelp:
		b.handleHelp(
			ctx,
			msg,
		)
	case commandExit:
		b.handleExit(
			ctx,
			msg,
		)
	case commandSettings:
		b.settingsFlow.handleSettingsCommand(
			ctx,
			msg,
		)
	default:
		b.telegram.SendMessage(
			msg.Chat.ID,
			botMessagesByLocale[botDefaultLocale].unknownCommand,
		)
	}
}

func (b *Bot) handleCallback(
	ctx context.Context,
	cb *tgbotapi.CallbackQuery,
) {
	// Acknowledge callback to remove loading indicator
	b.telegram.AnswerCallback(
		cb.ID,
		"",
	)

	data := cb.Data

	switch {
	case data == callbackLLMCancel:
		b.llmQuestionFlow.handleLLMCancel(
			ctx,
			cb,
		)
	case data == callbackMenuMain:
		b.showMainMenu(
			ctx,
			cb.Message.Chat.ID,
			cb.From,
		)
	case data == callbackMenuStudy:
		b.sessionFlow.StartStudy(
			ctx,
			cb,
		)
	case data == callbackMenuReview:
		b.sessionFlow.StartReview(
			ctx,
			cb,
		)
	case data == callbackMenuStats:
		b.handleStatsCallback(
			ctx,
			cb,
		)
	case data == callbackMenuSettings || strings.HasPrefix(
		data,
		callbackPrefixSettings,
	):
		b.settingsFlow.handleSettingsCallback(
			ctx,
			cb,
		)
		// 학습 세션 시작
		// e.g. session:50:start
	case strings.HasPrefix(
		data,
		callbackPrefixSession,
	):
		b.sessionFlow.HandleSessionCallback(
			ctx,
			cb,
		)
	case strings.HasPrefix(
		data,
		callback.QuestionPrefix,
	):
		b.sessionFlow.HandleAnswerCallback(
			ctx,
			cb,
		)
	case strings.HasPrefix(
		data,
		callbackPrefixStudy,
	):
		b.studyFlow.HandleCallback(
			ctx,
			cb,
		)
	}
}

func (b *Bot) handleStart(msg *tgbotapi.Message) {
	b.telegram.SendMessage(
		msg.Chat.ID,
		botMessagesByLocale[botDefaultLocale].welcomeMessage,
	)
}

func (b *Bot) handleMenu(
	ctx context.Context,
	msg *tgbotapi.Message,
) {
	b.showMainMenu(
		ctx,
		msg.Chat.ID,
		msg.From,
	)
}

func (b *Bot) showMainMenu(
	ctx context.Context,
	chatID int64,
	from *tgbotapi.User,
) {
	user, err := b.user.GetUser(
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

	var lang, level string

	streakDays := 0
	if user != nil {
		streakDays = user.StreakDays
		lang = user.Language
		level = user.ProficiencyLevel
	}
	reviewCount, _ := b.session.DueReviewCount(
		ctx,
		from.ID,
		lang,
		level,
	)

	langName := languageDisplayName(lang)
	text := fmt.Sprintf(
		botMessagesByLocale[botDefaultLocale].mainMenuFormat,
		streakEmoji,
		streakDays,
		langName,
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

	b.telegram.SendMessageWithKeyboard(
		chatID,
		text,
		keyboard,
	)
}

func (b *Bot) handleStats(
	ctx context.Context,
	msg *tgbotapi.Message,
) {
	stats, err := b.analyzer.GetUserStats(
		ctx,
		msg.From.ID,
	)
	if err != nil {
		b.telegram.SendMessage(
			msg.Chat.ID,
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

	b.telegram.SendMessage(
		msg.Chat.ID,
		text,
	)
}

func (b *Bot) handleStatsCallback(
	ctx context.Context,
	cb *tgbotapi.CallbackQuery,
) {
	stats, err := b.analyzer.GetUserStats(
		ctx,
		cb.From.ID,
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

	backBtn := tgbotapi.NewInlineKeyboardMarkup(
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData(
				botMessagesByLocale[botDefaultLocale].menuHomeButton,
				callbackMenuMain,
			),
		),
	)

	b.telegram.EditMessage(
		cb.Message.Chat.ID,
		cb.Message.MessageID,
		text,
		&backBtn,
	)
}

func (b *Bot) handleStreak(
	ctx context.Context,
	msg *tgbotapi.Message,
) {
	stats, err := b.analyzer.GetUserStats(
		ctx,
		msg.From.ID,
	)
	if err != nil {
		b.telegram.SendMessage(
			msg.Chat.ID,
			botMessagesByLocale[botDefaultLocale].streakCommandFailed,
		)
		return
	}

	text := fmt.Sprintf(
		botMessagesByLocale[botDefaultLocale].streakFormat,
		stats.CurrentStreak,
	)
	b.telegram.SendMessage(
		msg.Chat.ID,
		text,
	)
}

func (b *Bot) handleHelp(
	_ context.Context,
	msg *tgbotapi.Message,
) {
	b.telegram.SendMessage(
		msg.Chat.ID,
		botMessagesByLocale[botDefaultLocale].helpMessage,
	)
}

func (b *Bot) handleExit(
	ctx context.Context,
	msg *tgbotapi.Message,
) {
	var userID *int64
	if msg.From != nil {
		userID = &msg.From.ID
	}
	if b.input != nil {
		_ = b.input.ClearInput(
			ctx,
			msg.Chat.ID,
			userID,
		)
	}
	b.telegram.SendMessage(
		msg.Chat.ID,
		botMessagesByLocale[botDefaultLocale].exitMessage,
	)
}

func (b *Bot) handleStudy(
	ctx context.Context,
	msg *tgbotapi.Message,
) {
	limit, err := parseStudyCommandLimit(msg.CommandArguments())
	if err != nil {
		b.telegram.SendMessage(
			msg.Chat.ID,
			fmt.Sprintf(
				botMessagesByLocale[botDefaultLocale].studyCommandUsageFormat,
				service.MaxStudySessionMaterialCount,
			),
		)
		return
	}

	user, err := b.user.GetUser(
		ctx,
		msg.From.ID,
		msg.From.UserName,
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
		b.telegram.SendMessage(
			msg.Chat.ID,
			botMessagesByLocale[botDefaultLocale].userUnavailable,
		)
		return
	}

	session, err := b.session.BuildStudy(
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
		b.telegram.SendMessage(
			msg.Chat.ID,
			botMessagesByLocale[botDefaultLocale].studyCommandBuildFailed,
		)
		return
	}
	if session == nil {
		b.telegram.SendMessage(
			msg.Chat.ID,
			botMessagesByLocale[botDefaultLocale].studyCommandNoMaterials,
		)
		return
	}

	if err := b.PushStudySession(
		ctx,
		msg.Chat.ID,
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
		b.telegram.SendMessage(
			msg.Chat.ID,
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

func parseStudyCommandLimit(args string) (int, error) {
	args = strings.TrimSpace(args)
	if args == "" {
		return service.DefaultStudySessionMaterialCount, nil
	}
	fields := strings.Fields(args)
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

func (b *Bot) handleTest(
	ctx context.Context,
	msg *tgbotapi.Message,
) {
	// 1. Ensure user exists
	user, err := b.user.GetUser(
		ctx,
		msg.From.ID,
		msg.From.UserName,
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
		b.telegram.SendMessage(
			msg.Chat.ID,
			botMessagesByLocale[botDefaultLocale].userUnavailable,
		)
		return
	}

	// 2. Build a morning session (9 new + 6 review)
	session, err := b.session.BuildMorningQuiz(
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
		b.telegram.SendMessage(
			msg.Chat.ID,
			botMessagesByLocale[botDefaultLocale].testSessionBuildFailed,
		)
		return
	}

	if session == nil {
		b.telegram.SendMessage(
			msg.Chat.ID,
			botMessagesByLocale[botDefaultLocale].testSessionNoQuestions,
		)
		return
	}

	// 3. Push the session immediately
	if err := b.PushSession(
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
		b.telegram.SendMessage(
			msg.Chat.ID,
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

// languageDisplayName returns a human-readable name for the language code.
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
