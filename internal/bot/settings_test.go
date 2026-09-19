package bot

import (
	"context"
	"strings"
	"testing"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"

	"github.com/lsj/copylingo/internal/config"
	"github.com/lsj/copylingo/internal/model"
	"github.com/lsj/copylingo/internal/service"
)

type settingsMockUserRepo struct {
	user         *model.User
	slotUpdates  map[model.SessionSlot]*string
	lastTimezone string
}

func (m *settingsMockUserRepo) GetOrCreate(
	ctx context.Context,
	telegramID int64,
	username string,
) (*model.User, error) {
	return m.user, nil
}
func (m *settingsMockUserRepo) GetByID(ctx context.Context, id int64) (*model.User, error) {
	return m.user, nil
}
func (m *settingsMockUserRepo) GetAllUsers(ctx context.Context) ([]model.User, error) {
	return []model.User{*m.user}, nil
}
func (m *settingsMockUserRepo) GetActiveTimezones(ctx context.Context) ([]string, error) {
	return []string{m.user.Timezone}, nil
}

func (m *settingsMockUserRepo) GetUsersBySlot(
	ctx context.Context,
	slot model.SessionSlot,
	localTime string,
	timezone string,
) ([]model.User, error) {
	return []model.User{*m.user}, nil
}

func (m *settingsMockUserRepo) UpdateSlotTime(
	ctx context.Context,
	userID int64,
	slot model.SessionSlot,
	timeVal *string,
) error {
	m.slotUpdates[slot] = timeVal
	return nil
}
func (m *settingsMockUserRepo) UpdateTimezone(ctx context.Context, userID int64, timezone string) error {
	m.lastTimezone = timezone
	m.user.Timezone = timezone
	return nil
}

func newSettingsTestBot() (*Bot, *settingsMockUserRepo, *mockBotAPI) {
	mStudy := "08:00"
	mQuiz := "12:00"
	eStudy := "16:30"
	eQuiz := "21:00"

	user := &model.User{
		ID:                 42,
		Username:           "tester",
		Timezone:           "Asia/Seoul",
		MorningStudyTime:   &mStudy,
		MorningQuizTime:    &mQuiz,
		EveningStudyTime:   &eStudy,
		EveningQuizTime:    &eQuiz,
		MorningSessionTime: "08:00",
		EveningSessionTime: "21:00",
	}

	repo := &settingsMockUserRepo{
		user:        user,
		slotUpdates: make(map[model.SessionSlot]*string),
	}

	userSvc := service.NewUserService(repo)
	services := &service.Services{
		User: userSvc,
	}

	mockAPI := &mockBotAPI{}
	bot := &Bot{
		api:      mockAPI,
		services: services,
	}
	return bot, repo, mockAPI
}

func TestHandleSettingsCommand(t *testing.T) {
	bot, _, mockAPI := newSettingsTestBot()

	msg := &tgbotapi.Message{
		MessageID: 100,
		Chat:      &tgbotapi.Chat{ID: 12345},
		From:      &tgbotapi.User{ID: 12345, UserName: "tester"},
		Text:      "/settings",
	}

	bot.handleSettingsCommand(context.Background(), msg)

	if len(mockAPI.sentMessages) == 0 {
		t.Fatal("expected message to be sent")
	}

	sentMsg, ok := mockAPI.sentMessages[0].(tgbotapi.MessageConfig)
	if !ok {
		t.Fatalf("expected MessageConfig, got %T", mockAPI.sentMessages[0])
	}

	if !strings.Contains(sentMsg.Text, "푸시 알림 및 스케줄 설정") {
		t.Errorf("expected title in message, got: %s", sentMsg.Text)
	}
	if !strings.Contains(sentMsg.Text, "Asia/Seoul") {
		t.Errorf("expected timezone in message, got: %s", sentMsg.Text)
	}
	if !strings.Contains(sentMsg.Text, "08:00") {
		t.Errorf("expected morning study time in message, got: %s", sentMsg.Text)
	}

	// Verify keyboard buttons
	markup, ok := sentMsg.ReplyMarkup.(tgbotapi.InlineKeyboardMarkup)
	if !ok {
		t.Fatalf("expected InlineKeyboardMarkup, got %T", sentMsg.ReplyMarkup)
	}
	if len(markup.InlineKeyboard) != 6 {
		t.Fatalf("expected 6 rows of buttons, got %d", len(markup.InlineKeyboard))
	}
	if *markup.InlineKeyboard[0][0].CallbackData != "settings:slot:morning_study" {
		t.Errorf("unexpected callback data: %s", *markup.InlineKeyboard[0][0].CallbackData)
	}
}

func TestHandleSettingsCallback_View(t *testing.T) {
	bot, _, mockAPI := newSettingsTestBot()

	cb := &tgbotapi.CallbackQuery{
		ID:   "cb_1",
		Data: config.ActionMenuSettings,
		From: &tgbotapi.User{ID: 12345, UserName: "tester"},
		Message: &tgbotapi.Message{
			MessageID: 200,
			Chat:      &tgbotapi.Chat{ID: 12345},
		},
	}

	bot.handleSettingsCallback(context.Background(), cb)

	if len(mockAPI.sentMessages) == 0 {
		t.Fatal("expected message edit to be sent")
	}

	editMsg, ok := mockAPI.sentMessages[0].(tgbotapi.EditMessageTextConfig)
	if !ok {
		t.Fatalf("expected EditMessageTextConfig, got %T", mockAPI.sentMessages[0])
	}
	if !strings.Contains(editMsg.Text, "푸시 알림 및 스케줄 설정") {
		t.Errorf("unexpected edit text: %s", editMsg.Text)
	}
}

func TestHandleSettingsCallback_SlotPicker(t *testing.T) {
	bot, _, mockAPI := newSettingsTestBot()

	cb := &tgbotapi.CallbackQuery{
		ID:   "cb_2",
		Data: "settings:slot:morning_study",
		From: &tgbotapi.User{ID: 12345, UserName: "tester"},
		Message: &tgbotapi.Message{
			MessageID: 200,
			Chat:      &tgbotapi.Chat{ID: 12345},
		},
	}

	bot.handleSettingsCallback(context.Background(), cb)

	if len(mockAPI.sentMessages) == 0 {
		t.Fatal("expected message edit to be sent")
	}

	editMsg, ok := mockAPI.sentMessages[0].(tgbotapi.EditMessageTextConfig)
	if !ok {
		t.Fatalf("expected EditMessageTextConfig, got %T", mockAPI.sentMessages[0])
	}
	if !strings.Contains(editMsg.Text, "오전 학습 시각 설정") {
		t.Errorf("unexpected slot picker text: %s", editMsg.Text)
	}

	markup := editMsg.ReplyMarkup
	if markup == nil {
		t.Fatal("expected reply markup")
	}

	// Verify time buttons exist
	foundTime := false
	foundOff := false
	foundAllToggle := false
	for _, row := range markup.InlineKeyboard {
		for _, btn := range row {
			if *btn.CallbackData == "settings:set:morning_study:08:30" {
				foundTime = true
			}
			if *btn.CallbackData == "settings:set:morning_study:off" {
				foundOff = true
			}
			if *btn.CallbackData == "settings:slot:morning_study:all" {
				foundAllToggle = true
			}
		}
	}

	if !foundTime {
		t.Error("expected 08:30 time button in slot picker")
	}
	if !foundOff {
		t.Error("expected off button in slot picker")
	}
	if !foundAllToggle {
		t.Error("expected 24h toggle button in slot picker")
	}
}

func TestHandleSettingsCallback_SlotPickerAll(t *testing.T) {
	bot, _, mockAPI := newSettingsTestBot()

	cb := &tgbotapi.CallbackQuery{
		ID:   "cb_3",
		Data: "settings:slot:evening_quiz:all",
		From: &tgbotapi.User{ID: 12345, UserName: "tester"},
		Message: &tgbotapi.Message{
			MessageID: 200,
			Chat:      &tgbotapi.Chat{ID: 12345},
		},
	}

	bot.handleSettingsCallback(context.Background(), cb)

	editMsg, ok := mockAPI.sentMessages[0].(tgbotapi.EditMessageTextConfig)
	if !ok {
		t.Fatalf("expected EditMessageTextConfig, got %T", mockAPI.sentMessages[0])
	}

	markup := editMsg.ReplyMarkup
	foundMidnight := false
	foundLateNight := false
	for _, row := range markup.InlineKeyboard {
		for _, btn := range row {
			if *btn.CallbackData == "settings:set:evening_quiz:00:00" {
				foundMidnight = true
			}
			if *btn.CallbackData == "settings:set:evening_quiz:23:30" {
				foundLateNight = true
			}
		}
	}

	if !foundMidnight || !foundLateNight {
		t.Error("expected full 24h options in all mode")
	}
}

func TestHandleSettingsCallback_SetSlotTime(t *testing.T) {
	bot, repo, mockAPI := newSettingsTestBot()

	cb := &tgbotapi.CallbackQuery{
		ID:   "cb_4",
		Data: "settings:set:morning_study:07:30",
		From: &tgbotapi.User{ID: 12345, UserName: "tester"},
		Message: &tgbotapi.Message{
			MessageID: 200,
			Chat:      &tgbotapi.Chat{ID: 12345},
		},
	}

	bot.handleSettingsCallback(context.Background(), cb)

	val, ok := repo.slotUpdates[model.SessionSlotMorningStudy]
	if !ok || val == nil || *val != "07:30" {
		t.Errorf("expected slot update to 07:30, got %v", val)
	}

	// Verify message was re-rendered with new time
	editMsg, ok := mockAPI.sentMessages[0].(tgbotapi.EditMessageTextConfig)
	if !ok {
		t.Fatalf("expected EditMessageTextConfig, got %T", mockAPI.sentMessages[0])
	}
	if !strings.Contains(editMsg.Text, "07:30") {
		t.Errorf("expected updated time 07:30 in message, got: %s", editMsg.Text)
	}
}

func TestHandleSettingsCallback_DisableSlotTime(t *testing.T) {
	bot, repo, mockAPI := newSettingsTestBot()

	cb := &tgbotapi.CallbackQuery{
		ID:   "cb_5",
		Data: "settings:set:evening_quiz:off",
		From: &tgbotapi.User{ID: 12345, UserName: "tester"},
		Message: &tgbotapi.Message{
			MessageID: 200,
			Chat:      &tgbotapi.Chat{ID: 12345},
		},
	}

	bot.handleSettingsCallback(context.Background(), cb)

	val, ok := repo.slotUpdates[model.SessionSlotEveningQuiz]
	if !ok {
		t.Fatal("expected slot update entry")
	}
	if val != nil {
		t.Errorf("expected nil for disabled slot, got %v", *val)
	}

	// Verify message displays 꺼짐
	editMsg, ok := mockAPI.sentMessages[0].(tgbotapi.EditMessageTextConfig)
	if !ok {
		t.Fatalf("expected EditMessageTextConfig, got %T", mockAPI.sentMessages[0])
	}
	if !strings.Contains(editMsg.Text, "🔕 꺼짐") {
		t.Errorf("expected 🔕 꺼짐 in message, got: %s", editMsg.Text)
	}
}

func TestHandleSettingsCallback_Timezone(t *testing.T) {
	bot, _, mockAPI := newSettingsTestBot()

	cb := &tgbotapi.CallbackQuery{
		ID:   "cb_6",
		Data: config.ActionSettingsTimezone,
		From: &tgbotapi.User{ID: 12345, UserName: "tester"},
		Message: &tgbotapi.Message{
			MessageID: 200,
			Chat:      &tgbotapi.Chat{ID: 12345},
		},
	}

	bot.handleSettingsCallback(context.Background(), cb)

	editMsg, ok := mockAPI.sentMessages[0].(tgbotapi.EditMessageTextConfig)
	if !ok {
		t.Fatalf("expected EditMessageTextConfig, got %T", mockAPI.sentMessages[0])
	}
	if !strings.Contains(editMsg.Text, "시간대(Timezone) 설정") {
		t.Errorf("unexpected timezone view text: %s", editMsg.Text)
	}

	// Verify timezone options
	markup := editMsg.ReplyMarkup
	foundTokyo := false
	foundNY := false
	for _, row := range markup.InlineKeyboard {
		for _, btn := range row {
			if *btn.CallbackData == "settings:set_tz:Asia/Tokyo" {
				foundTokyo = true
			}
			if *btn.CallbackData == "settings:set_tz:America/New_York" {
				foundNY = true
			}
		}
	}
	if !foundTokyo || !foundNY {
		t.Error("expected timezone buttons in keyboard")
	}
}

func TestHandleSettingsCallback_SetTimezone(t *testing.T) {
	bot, repo, mockAPI := newSettingsTestBot()

	cb := &tgbotapi.CallbackQuery{
		ID:   "cb_7",
		Data: "settings:set_tz:Asia/Tokyo",
		From: &tgbotapi.User{ID: 12345, UserName: "tester"},
		Message: &tgbotapi.Message{
			MessageID: 200,
			Chat:      &tgbotapi.Chat{ID: 12345},
		},
	}

	bot.handleSettingsCallback(context.Background(), cb)

	if repo.lastTimezone != "Asia/Tokyo" {
		t.Errorf("expected timezone updated to Asia/Tokyo, got %s", repo.lastTimezone)
	}

	// Verify settings overview shows updated timezone
	editMsg, ok := mockAPI.sentMessages[0].(tgbotapi.EditMessageTextConfig)
	if !ok {
		t.Fatalf("expected EditMessageTextConfig, got %T", mockAPI.sentMessages[0])
	}
	if !strings.Contains(editMsg.Text, "Asia/Tokyo") {
		t.Errorf("expected Asia/Tokyo in updated overview, got: %s", editMsg.Text)
	}
}

func TestHandleSettingsCallback_InvalidInputs(t *testing.T) {
	bot, repo, _ := newSettingsTestBot()

	// Invalid slot
	cb := &tgbotapi.CallbackQuery{
		ID:   "cb_inv_slot",
		Data: "settings:slot:invalid_slot",
		From: &tgbotapi.User{ID: 12345, UserName: "tester"},
		Message: &tgbotapi.Message{
			MessageID: 200,
			Chat:      &tgbotapi.Chat{ID: 12345},
		},
	}
	bot.handleSettingsCallback(context.Background(), cb)

	// Invalid timezone
	cbTz := &tgbotapi.CallbackQuery{
		ID:   "cb_inv_tz",
		Data: "settings:set_tz:Invalid/Location_999",
		From: &tgbotapi.User{ID: 12345, UserName: "tester"},
		Message: &tgbotapi.Message{
			MessageID: 200,
			Chat:      &tgbotapi.Chat{ID: 12345},
		},
	}
	bot.handleSettingsCallback(context.Background(), cbTz)
	if repo.lastTimezone == "Invalid/Location_999" {
		t.Error("expected invalid timezone to not be saved")
	}
}
