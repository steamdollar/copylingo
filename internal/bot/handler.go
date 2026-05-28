package bot

import (
	"context"
	"fmt"
	"log"
	"strings"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
	"github.com/redis/go-redis/v9"

	"github.com/lsj/copylingo/internal/config"
	"github.com/lsj/copylingo/internal/service"
)

// BotAPI defines the interface for Telegram bot interactions to allow mocking.
type BotAPI interface {
	Send(c tgbotapi.Chattable) (tgbotapi.Message, error)
	Request(c tgbotapi.Chattable) (*tgbotapi.APIResponse, error)
	GetUpdatesChan(config tgbotapi.UpdateConfig) tgbotapi.UpdatesChannel
	StopReceivingUpdates()
}

// Bot wraps the Telegram bot API with CopyLingo business logic.
type Bot struct {
	api      BotAPI
	cfg      *config.Config
	services *service.Services
	rdb      redis.Cmdable
	flow     *SessionFlow
	stopCh   chan struct{}
}

func New(cfg *config.Config, services *service.Services, rdb redis.Cmdable) (*Bot, error) {
	api, err := tgbotapi.NewBotAPI(cfg.Telegram.Token)
	if err != nil {
		return nil, fmt.Errorf("failed to create Telegram bot: %w", err)
	}

	api.Debug = cfg.Telegram.Debug
	log.Printf("Telegram bot authorized as @%s", api.Self.UserName)

	bot := &Bot{
		api:      api,
		cfg:      cfg,
		services: services,
		rdb:      rdb,
		stopCh:   make(chan struct{}),
	}
	bot.flow = NewSessionFlow(bot)

	return bot, nil
}

// Start begins listening for Telegram updates.
func (b *Bot) Start() {
	// 사용자가 보낸 메시지, 버튼 클릭 이벤트 등을 poll하는 것 관련 config
	pollConfig := tgbotapi.NewUpdate(0)
	pollConfig.Timeout = 60

	// 업데이트 받는 go chan 생성
	updates := b.api.GetUpdatesChan(pollConfig)

	for {
		select {
		case update := <-updates:
			// update listen
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
	b.api.StopReceivingUpdates()
}

// PushSession: 시간마다 세션 시작 메시지 전송
func (b *Bot) PushSession(ctx context.Context, chatID int64, sessionID int, sessionType string) error {
	return b.flow.PushSession(ctx, chatID, sessionID, sessionType)
}

// SendMessage sends a text message to a chat.
func (b *Bot) SendMessage(chatID int64, text string) error {
	msg := tgbotapi.NewMessage(chatID, text)
	msg.ParseMode = "HTML"
	_, err := b.api.Send(msg)
	return err
}

// SendMessageWithKeyboard sends a message with an inline keyboard.
func (b *Bot) SendMessageWithKeyboard(chatID int64, text string, keyboard tgbotapi.InlineKeyboardMarkup) error {
	msg := tgbotapi.NewMessage(chatID, text)
	msg.ParseMode = "HTML"
	if len(keyboard.InlineKeyboard) > 0 {
		msg.ReplyMarkup = keyboard
	}
	_, err := b.api.Send(msg)
	return err
}

// SendMessageWithReplyMarkup sends a message with custom Telegram reply markup.
func (b *Bot) SendMessageWithReplyMarkup(chatID int64, text string, replyMarkup interface{}) (int, error) {
	msg := tgbotapi.NewMessage(chatID, text)
	msg.ParseMode = "HTML"
	msg.ReplyMarkup = replyMarkup
	sent, err := b.api.Send(msg)
	if err != nil {
		return 0, err
	}
	return sent.MessageID, nil
}

// EditMessageReplyMarkup updates the inline keyboard of an existing message.
func (b *Bot) EditMessageReplyMarkup(chatID int64, messageID int, markup tgbotapi.InlineKeyboardMarkup) error {
	edit := tgbotapi.NewEditMessageReplyMarkup(chatID, messageID, markup)
	_, err := b.api.Send(edit)
	return err
}

// EditMessage edits an existing message.
func (b *Bot) EditMessage(chatID int64, messageID int, text string, keyboard *tgbotapi.InlineKeyboardMarkup) error {
	edit := tgbotapi.NewEditMessageText(chatID, messageID, text)
	edit.ParseMode = "HTML"
	if keyboard != nil && len(keyboard.InlineKeyboard) > 0 {
		edit.ReplyMarkup = keyboard
	}
	_, err := b.api.Send(edit)
	return err
}

// ClearInlineKeyboard removes inline buttons from an existing bot message.
func (b *Bot) ClearInlineKeyboard(chatID int64, messageID int) error {
	edit := tgbotapi.NewEditMessageReplyMarkup(chatID, messageID, tgbotapi.InlineKeyboardMarkup{})
	_, err := b.api.Send(edit)
	return err
}

func (b *Bot) handleUpdate(update tgbotapi.Update) {
	ctx := context.Background()

	if update.Message != nil {
		// text input handle
		b.handleMessage(ctx, update.Message)
	} else if update.CallbackQuery != nil {
		// button click handle
		b.handleCallback(ctx, update.CallbackQuery)
	}
}

func (b *Bot) handleMessage(ctx context.Context, msg *tgbotapi.Message) {
	if !msg.IsCommand() {
		// Route plain text to session flow for FillBlank questions
		if handled := b.flow.HandleTextInput(ctx, msg); !handled {
			// Optional: Fallback to chat or ignore
		}
		return
	}

	switch msg.Command() {
	case config.CommandStart:
		b.handleStart(ctx, msg)
	case config.CommandMenu:
		b.handleMenu(ctx, msg)
	case config.CommandStats:
		b.handleStats(ctx, msg)
	case config.CommandStreak:
		b.handleStreak(ctx, msg)
	case config.CommandTest:
		b.handleTest(ctx, msg)
	case config.CommandHelp:
		b.handleHelp(ctx, msg)
	case config.CommandExit:
		b.handleExit(ctx, msg)
	default:
		b.SendMessage(msg.Chat.ID, "❓ 알 수 없는 명령어입니다. /help 를 입력해 보세요.")
	}
}

func (b *Bot) handleCallback(ctx context.Context, cb *tgbotapi.CallbackQuery) {
	// Acknowledge callback to remove loading indicator
	callback := tgbotapi.NewCallback(cb.ID, "")
	b.api.Request(callback)

	data := cb.Data

	switch {
	case data == config.ActionMenuMain:
		b.showMainMenu(ctx, cb.Message.Chat.ID, cb.From)
	case data == config.ActionMenuStudy:
		b.flow.StartStudy(ctx, cb)
	case data == config.ActionMenuReview:
		b.flow.StartReview(ctx, cb)
	case data == config.ActionMenuStats:
		b.handleStatsCallback(ctx, cb)
		// 학습 세션 시작
		// e.g. session:50:start
	case strings.HasPrefix(data, config.PrefixSession):
		b.flow.HandleSessionCallback(ctx, cb)
	case strings.HasPrefix(data, config.PrefixQuestion):
		b.flow.HandleAnswerCallback(ctx, cb)
	}
}

func (b *Bot) handleStart(ctx context.Context, msg *tgbotapi.Message) {
	welcome := `🎌 <b>CopyLingo에 오신 것을 환영합니다!</b>

일본어를 마스터하기 위한 여정을 시작합니다.
JLPT N5부터 N1까지, 매일 조금씩 실력을 키워갑니다.

📚 <b>학습 방식:</b>
• 매일 오전/오후 학습 세션이 전송됩니다
• 뉴스, 시험 대비 자료를 기반으로 문제가 생성됩니다
• 틀린 문제는 간격 반복(SRS)으로 자동 복습됩니다
• 주말에는 아티클 읽기 + AI 대화도 제공됩니다

/menu 를 눌러 시작하세요! 🚀`

	b.SendMessage(msg.Chat.ID, welcome)
}

func (b *Bot) handleMenu(ctx context.Context, msg *tgbotapi.Message) {
	b.showMainMenu(ctx, msg.Chat.ID, msg.From)
}

func (b *Bot) showMainMenu(ctx context.Context, chatID int64, from *tgbotapi.User) {
	user, err := b.services.User.GetUser(ctx, from.ID, from.UserName)
	if err != nil {
		log.Printf("Error getting user: %v", err)
	}

	reviewCount, _ := b.services.SRS.GetDueCount(ctx)

	streakEmoji := "🔥"
	if user != nil && user.StreakDays == 0 {
		streakEmoji = "💤"
	}

	streakDays := 0
	lang := "ja"
	level := "N5"
	if user != nil {
		streakDays = user.StreakDays
		lang = user.Language
		level = user.ProficiencyLevel
	}

	langName := languageDisplayName(lang)
	text := fmt.Sprintf(`🎌 <b>CopyLingo</b>

%s 스트릭: <b>%d일</b> 연속
🌐 언어: <b>%s</b>
📈 레벨: <b>%s</b>`, streakEmoji, streakDays, langName, level)

	reviewLabel := fmt.Sprintf("🔄 복습하기 (%d개)", reviewCount)

	keyboard := tgbotapi.NewInlineKeyboardMarkup(
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData("📚 학습하기", config.ActionMenuStudy),
			tgbotapi.NewInlineKeyboardButtonData(reviewLabel, config.ActionMenuReview),
		),
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData("📊 내 통계", config.ActionMenuStats),
			tgbotapi.NewInlineKeyboardButtonData("⚙️ 설정", config.ActionMenuSettings),
		),
	)

	b.SendMessageWithKeyboard(chatID, text, keyboard)
}

func (b *Bot) handleStats(ctx context.Context, msg *tgbotapi.Message) {
	stats, err := b.services.Analyzer.GetUserStats(ctx, msg.From.ID)
	if err != nil {
		b.SendMessage(msg.Chat.ID, "❌ 통계를 불러오는 데 실패했습니다.")
		return
	}

	text := fmt.Sprintf(`📊 <b>학습 통계</b>

📅 오늘: %d문제 풀음 (정답률 %.0f%%)
🔥 스트릭: %d일 연속

<b>카테고리별 정답률:</b>
📝 어휘: %.0f%%
📖 문법: %.0f%%
🈲 한자: %.0f%%
📚 독해: %.0f%%
🎧 청해: %.0f%%`,
		stats.TodayQuestions, stats.OverallAccuracy,
		stats.CurrentStreak,
		stats.VocabularyAccuracy,
		stats.GrammarAccuracy,
		stats.KanjiAccuracy,
		stats.ReadingAccuracy,
		stats.ListeningAccuracy,
	)

	b.SendMessage(msg.Chat.ID, text)
}

func (b *Bot) handleStatsCallback(ctx context.Context, cb *tgbotapi.CallbackQuery) {
	stats, err := b.services.Analyzer.GetUserStats(ctx, cb.From.ID)
	if err != nil {
		return
	}

	text := fmt.Sprintf(`📊 <b>학습 통계</b>

📅 오늘: %d문제 (정답률 %.0f%%)
🔥 스트릭: %d일

📝 어휘: %.0f%% | 📖 문법: %.0f%%
🈲 한자: %.0f%% | 📚 독해: %.0f%%
🎧 청해: %.0f%%`,
		stats.TodayQuestions, stats.OverallAccuracy,
		stats.CurrentStreak,
		stats.VocabularyAccuracy, stats.GrammarAccuracy,
		stats.KanjiAccuracy, stats.ReadingAccuracy,
		stats.ListeningAccuracy,
	)

	backBtn := tgbotapi.NewInlineKeyboardMarkup(
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData("🏠 메뉴로", config.ActionMenuMain),
		),
	)

	b.EditMessage(cb.Message.Chat.ID, cb.Message.MessageID, text, &backBtn)
}

func (b *Bot) handleStreak(ctx context.Context, msg *tgbotapi.Message) {
	stats, err := b.services.Analyzer.GetUserStats(ctx, msg.From.ID)
	if err != nil {
		b.SendMessage(msg.Chat.ID, "❌ 스트릭 정보를 불러올 수 없습니다.")
		return
	}

	text := fmt.Sprintf("🔥 현재 스트릭: <b>%d일</b> 연속 학습 중!", stats.CurrentStreak)
	b.SendMessage(msg.Chat.ID, text)
}

func (b *Bot) handleHelp(_ context.Context, msg *tgbotapi.Message) {
	help := `📖 <b>CopyLingo 도움말</b>

<b>명령어:</b>
/menu - 메인 메뉴
/stats - 학습 통계
/streak - 스트릭 확인
/exit - 현재 입력 취소 (세션은 보존, /menu 에서 재개)
/help - 도움말

<b>학습 흐름:</b>
1. 매일 오전 8시 / 오후 9시에 학습 세션이 전송됩니다
2. 인라인 버튼으로 문제를 풀어주세요
3. 틀린 문제는 SRS로 자동 복습됩니다
4. /menu → 복습하기로 수동 복습도 가능합니다`

	b.SendMessage(msg.Chat.ID, help)
}

func (b *Bot) handleExit(ctx context.Context, msg *tgbotapi.Message) {
	key := fmt.Sprintf(config.KeyUserActiveQuestion, msg.Chat.ID)
	b.rdb.Del(ctx, key)
	b.SendMessage(msg.Chat.ID, "🚪 현재 입력을 취소했습니다. /menu 에서 언제든 이어서 진행할 수 있어요.")
}

func (b *Bot) handleTest(ctx context.Context, msg *tgbotapi.Message) {
	// 1. Ensure user exists
	user, err := b.services.User.GetUser(ctx, msg.From.ID, msg.From.UserName)
	if err != nil {
		log.Printf("Error getting user for /test: %v", err)
		b.SendMessage(msg.Chat.ID, "❌ 사용자 정보를 확인할 수 없습니다.")
		return
	}

	// 2. Build a morning session (9 new + 6 review)
	session, err := b.services.SessionBuilder.BuildMorningSession(ctx, user.ID, user.Language, user.ProficiencyLevel)
	if err != nil {
		log.Printf("Error building session for /test: %v", err)
		b.SendMessage(msg.Chat.ID, "❌ 세션 생성 중 오류가 발생했습니다.")
		return
	}

	if session == nil {
		b.SendMessage(msg.Chat.ID, "⚠️ 현재 사용 가능한 문제가 없습니다. 컨텐츠가 수집되었는지 확인해 주세요.")
		return
	}

	// 3. Push the session immediately
	if err := b.PushSession(ctx, user.ID, session.ID, "morning"); err != nil {
		log.Printf("Error pushing session for /test: %v", err)
		b.SendMessage(msg.Chat.ID, "❌ 세션 발송에 실패했습니다.")
		return
	}

	log.Printf("[Test] Manually triggered morning session for user %d", user.ID)
}

// languageDisplayName returns a human-readable name for the language code.
func languageDisplayName(code string) string {
	switch code {
	case "ja":
		return "일본어"
	case "el":
		return "그리스어"
	case "en":
		return "영어"
	case "ko":
		return "한국어"
	default:
		return code
	}
}
