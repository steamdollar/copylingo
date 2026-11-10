package bot

import (
	"context"
	"strings"
	"testing"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"

	"github.com/lsj/copylingo/internal/model"
	"github.com/lsj/copylingo/internal/service"
)

type settingsMockUserRepository struct {
	user         *model.User
	slotUpdates  map[model.SessionSlot]*string
	lastTimezone string
}

func (mock *settingsMockUserRepository) GetOrCreate(
	ctx context.Context,
	telegramID int64,
	username string,
) (*model.User, error) {
	return mock.user, nil
}
func (mock *settingsMockUserRepository) GetByID(
	ctx context.Context,
	id int64,
) (*model.User, error) {
	return mock.user, nil
}
func (mock *settingsMockUserRepository) GetAllUsers(ctx context.Context) ([]model.User, error) {
	return []model.User{*mock.user}, nil
}
func (mock *settingsMockUserRepository) GetActiveTimezones(ctx context.Context) ([]string, error) {
	return []string{mock.user.Timezone}, nil
}

func (mock *settingsMockUserRepository) GetUsersBySlot(
	ctx context.Context,
	slot model.SessionSlot,
	localTime string,
	timezone string,
) ([]model.User, error) {
	return []model.User{*mock.user}, nil
}

func (mock *settingsMockUserRepository) UpdateSlotTime(
	ctx context.Context,
	userID int64,
	slot model.SessionSlot,
	timeValue *string,
) error {
	mock.slotUpdates[slot] = timeValue
	return nil
}
func (mock *settingsMockUserRepository) UpdateTimezone(
	ctx context.Context,
	userID int64,
	timezone string,
) error {
	mock.lastTimezone = timezone
	mock.user.Timezone = timezone
	return nil
}

func newSettingsTestFlow() (*SettingsFlow, *settingsMockUserRepository, *mockBotAPI) {
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

	repository := &settingsMockUserRepository{
		user:        user,
		slotUpdates: make(map[model.SessionSlot]*string),
	}

	mockAPI := &mockBotAPI{}
	flow := newTestSettingsFlow(
		mockAPI,
		SettingsFlowDependencies{User: service.NewUserService(repository)},
	)
	return flow, repository, mockAPI
}

func TestHandleSettingsCommand(t *testing.T) {
	flow, _, mockAPI := newSettingsTestFlow()

	message := &tgbotapi.Message{
		MessageID: 100,
		Chat:      &tgbotapi.Chat{ID: 12345},
		From:      &tgbotapi.User{ID: 12345, UserName: "tester"},
		Text:      "/settings",
	}

	flow.handleSettingsCommand(
		context.Background(),
		message,
	)

	if len(mockAPI.sentMessages) == 0 {
		t.Fatal("expected message to be sent")
	}

	sentMessage, ok := mockAPI.sentMessages[0].(tgbotapi.MessageConfig)
	if !ok {
		t.Fatalf(
			"expected MessageConfig, got %T",
			mockAPI.sentMessages[0],
		)
	}

	if !strings.Contains(
		sentMessage.Text,
		"푸시 알림 및 스케줄 설정",
	) {
		t.Errorf(
			"expected title in message, got: %s",
			sentMessage.Text,
		)
	}
	if !strings.Contains(
		sentMessage.Text,
		"Asia/Seoul",
	) {
		t.Errorf(
			"expected timezone in message, got: %s",
			sentMessage.Text,
		)
	}
	if !strings.Contains(
		sentMessage.Text,
		"08:00",
	) {
		t.Errorf(
			"expected morning study time in message, got: %s",
			sentMessage.Text,
		)
	}

	// Verify keyboard buttons
	markup, ok := sentMessage.ReplyMarkup.(tgbotapi.InlineKeyboardMarkup)
	if !ok {
		t.Fatalf(
			"expected InlineKeyboardMarkup, got %T",
			sentMessage.ReplyMarkup,
		)
	}
	// 4 slots, timezone, linked-material list, main menu.
	if len(markup.InlineKeyboard) != 7 {
		t.Fatalf(
			"expected 7 rows of buttons, got %d",
			len(markup.InlineKeyboard),
		)
	}
	if *markup.InlineKeyboard[0][0].CallbackData != "settings:slot:morning_study" {
		t.Errorf(
			"unexpected callback data: %s",
			*markup.InlineKeyboard[0][0].CallbackData,
		)
	}
}

func TestFormatSlotTimeNormalizesDatabaseTimestamp(t *testing.T) {
	value := "0000-01-01T08:00:00Z"

	if got, want := formatSlotTime(&value), "08:00"; got != want {
		t.Fatalf(
			"formatSlotTime() = %q, want %q",
			got,
			want,
		)
	}
}

func TestHandleSettingsCallback_View(t *testing.T) {
	flow, _, mockAPI := newSettingsTestFlow()

	callbackQuery := &tgbotapi.CallbackQuery{
		ID:   "cb_1",
		Data: callbackMenuSettings,
		From: &tgbotapi.User{ID: 12345, UserName: "tester"},
		Message: &tgbotapi.Message{
			MessageID: 200,
			Chat:      &tgbotapi.Chat{ID: 12345},
		},
	}

	flow.handleSettingsCallback(
		context.Background(),
		callbackQuery,
	)

	if len(mockAPI.sentMessages) == 0 {
		t.Fatal("expected message edit to be sent")
	}

	editMessage, ok := mockAPI.sentMessages[0].(tgbotapi.EditMessageTextConfig)
	if !ok {
		t.Fatalf(
			"expected EditMessageTextConfig, got %T",
			mockAPI.sentMessages[0],
		)
	}
	if !strings.Contains(
		editMessage.Text,
		"푸시 알림 및 스케줄 설정",
	) {
		t.Errorf(
			"unexpected edit text: %s",
			editMessage.Text,
		)
	}
}

func TestHandleSettingsCallback_SlotPicker(t *testing.T) {
	flow, _, mockAPI := newSettingsTestFlow()

	callbackQuery := &tgbotapi.CallbackQuery{
		ID:   "cb_2",
		Data: "settings:slot:morning_study",
		From: &tgbotapi.User{ID: 12345, UserName: "tester"},
		Message: &tgbotapi.Message{
			MessageID: 200,
			Chat:      &tgbotapi.Chat{ID: 12345},
		},
	}

	flow.handleSettingsCallback(
		context.Background(),
		callbackQuery,
	)

	if len(mockAPI.sentMessages) == 0 {
		t.Fatal("expected message edit to be sent")
	}

	editMessage, ok := mockAPI.sentMessages[0].(tgbotapi.EditMessageTextConfig)
	if !ok {
		t.Fatalf(
			"expected EditMessageTextConfig, got %T",
			mockAPI.sentMessages[0],
		)
	}
	if !strings.Contains(
		editMessage.Text,
		"오전 학습 시각 설정",
	) {
		t.Errorf(
			"unexpected slot picker text: %s",
			editMessage.Text,
		)
	}

	markup := editMessage.ReplyMarkup
	if markup == nil {
		t.Fatal("expected reply markup")
	}

	// Verify time buttons exist
	foundTime := false
	foundOff := false
	foundAllToggle := false
	for _, row := range markup.InlineKeyboard {
		for _, button := range row {
			if *button.CallbackData == "settings:set:morning_study:08:30" {
				foundTime = true
			}
			if *button.CallbackData == "settings:set:morning_study:off" {
				foundOff = true
			}
			if *button.CallbackData == "settings:slot:morning_study:all" {
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
	flow, _, mockAPI := newSettingsTestFlow()

	callbackQuery := &tgbotapi.CallbackQuery{
		ID:   "cb_3",
		Data: "settings:slot:evening_quiz:all",
		From: &tgbotapi.User{ID: 12345, UserName: "tester"},
		Message: &tgbotapi.Message{
			MessageID: 200,
			Chat:      &tgbotapi.Chat{ID: 12345},
		},
	}

	flow.handleSettingsCallback(
		context.Background(),
		callbackQuery,
	)

	editMessage, ok := mockAPI.sentMessages[0].(tgbotapi.EditMessageTextConfig)
	if !ok {
		t.Fatalf(
			"expected EditMessageTextConfig, got %T",
			mockAPI.sentMessages[0],
		)
	}

	markup := editMessage.ReplyMarkup
	foundMidnight := false
	foundLateNight := false
	for _, row := range markup.InlineKeyboard {
		for _, button := range row {
			if *button.CallbackData == "settings:set:evening_quiz:00:00" {
				foundMidnight = true
			}
			if *button.CallbackData == "settings:set:evening_quiz:23:30" {
				foundLateNight = true
			}
		}
	}

	if !foundMidnight || !foundLateNight {
		t.Error("expected full 24h options in all mode")
	}
}

func TestHandleSettingsCallback_SetSlotTime(t *testing.T) {
	flow, repository, mockAPI := newSettingsTestFlow()

	callbackQuery := &tgbotapi.CallbackQuery{
		ID:   "cb_4",
		Data: "settings:set:morning_study:07:30",
		From: &tgbotapi.User{ID: 12345, UserName: "tester"},
		Message: &tgbotapi.Message{
			MessageID: 200,
			Chat:      &tgbotapi.Chat{ID: 12345},
		},
	}

	flow.handleSettingsCallback(
		context.Background(),
		callbackQuery,
	)

	value, ok := repository.slotUpdates[model.SessionSlotMorningStudy]
	if !ok || value == nil || *value != "07:30" {
		t.Errorf(
			"expected slot update to 07:30, got %v",
			value,
		)
	}

	// Verify message was re-rendered with new time
	editMessage, ok := mockAPI.sentMessages[0].(tgbotapi.EditMessageTextConfig)
	if !ok {
		t.Fatalf(
			"expected EditMessageTextConfig, got %T",
			mockAPI.sentMessages[0],
		)
	}
	if !strings.Contains(
		editMessage.Text,
		"07:30",
	) {
		t.Errorf(
			"expected updated time 07:30 in message, got: %s",
			editMessage.Text,
		)
	}
}

func TestHandleSettingsCallback_DisableSlotTime(t *testing.T) {
	flow, repository, mockAPI := newSettingsTestFlow()

	callbackQuery := &tgbotapi.CallbackQuery{
		ID:   "cb_5",
		Data: "settings:set:evening_quiz:off",
		From: &tgbotapi.User{ID: 12345, UserName: "tester"},
		Message: &tgbotapi.Message{
			MessageID: 200,
			Chat:      &tgbotapi.Chat{ID: 12345},
		},
	}

	flow.handleSettingsCallback(
		context.Background(),
		callbackQuery,
	)

	value, ok := repository.slotUpdates[model.SessionSlotEveningQuiz]
	if !ok {
		t.Fatal("expected slot update entry")
	}
	if value != nil {
		t.Errorf(
			"expected nil for disabled slot, got %v",
			*value,
		)
	}

	// Verify message displays 꺼짐
	editMessage, ok := mockAPI.sentMessages[0].(tgbotapi.EditMessageTextConfig)
	if !ok {
		t.Fatalf(
			"expected EditMessageTextConfig, got %T",
			mockAPI.sentMessages[0],
		)
	}
	if !strings.Contains(
		editMessage.Text,
		"🔕 꺼짐",
	) {
		t.Errorf(
			"expected 🔕 꺼짐 in message, got: %s",
			editMessage.Text,
		)
	}
}

func TestHandleSettingsCallback_Timezone(t *testing.T) {
	flow, _, mockAPI := newSettingsTestFlow()

	callbackQuery := &tgbotapi.CallbackQuery{
		ID:   "cb_6",
		Data: callbackSettingsTimezone,
		From: &tgbotapi.User{ID: 12345, UserName: "tester"},
		Message: &tgbotapi.Message{
			MessageID: 200,
			Chat:      &tgbotapi.Chat{ID: 12345},
		},
	}

	flow.handleSettingsCallback(
		context.Background(),
		callbackQuery,
	)

	editMessage, ok := mockAPI.sentMessages[0].(tgbotapi.EditMessageTextConfig)
	if !ok {
		t.Fatalf(
			"expected EditMessageTextConfig, got %T",
			mockAPI.sentMessages[0],
		)
	}
	if !strings.Contains(
		editMessage.Text,
		"시간대(Timezone) 설정",
	) {
		t.Errorf(
			"unexpected timezone view text: %s",
			editMessage.Text,
		)
	}

	// Verify timezone options
	markup := editMessage.ReplyMarkup
	foundTokyo := false
	foundNY := false
	for _, row := range markup.InlineKeyboard {
		for _, button := range row {
			if *button.CallbackData == "settings:set_tz:Asia/Tokyo" {
				foundTokyo = true
			}
			if *button.CallbackData == "settings:set_tz:America/New_York" {
				foundNY = true
			}
		}
	}
	if !foundTokyo || !foundNY {
		t.Error("expected timezone buttons in keyboard")
	}
}

func TestHandleSettingsCallback_SetTimezone(t *testing.T) {
	flow, repository, mockAPI := newSettingsTestFlow()

	callbackQuery := &tgbotapi.CallbackQuery{
		ID:   "cb_7",
		Data: "settings:set_tz:Asia/Tokyo",
		From: &tgbotapi.User{ID: 12345, UserName: "tester"},
		Message: &tgbotapi.Message{
			MessageID: 200,
			Chat:      &tgbotapi.Chat{ID: 12345},
		},
	}

	flow.handleSettingsCallback(
		context.Background(),
		callbackQuery,
	)

	if repository.lastTimezone != "Asia/Tokyo" {
		t.Errorf(
			"expected timezone updated to Asia/Tokyo, got %s",
			repository.lastTimezone,
		)
	}

	// Verify settings overview shows updated timezone
	editMessage, ok := mockAPI.sentMessages[0].(tgbotapi.EditMessageTextConfig)
	if !ok {
		t.Fatalf(
			"expected EditMessageTextConfig, got %T",
			mockAPI.sentMessages[0],
		)
	}
	if !strings.Contains(
		editMessage.Text,
		"Asia/Tokyo",
	) {
		t.Errorf(
			"expected Asia/Tokyo in updated overview, got: %s",
			editMessage.Text,
		)
	}
}

func TestHandleSettingsCallback_InvalidInputs(t *testing.T) {
	flow, repository, _ := newSettingsTestFlow()

	// Invalid slot
	callbackQuery := &tgbotapi.CallbackQuery{
		ID:   "cb_inv_slot",
		Data: "settings:slot:invalid_slot",
		From: &tgbotapi.User{ID: 12345, UserName: "tester"},
		Message: &tgbotapi.Message{
			MessageID: 200,
			Chat:      &tgbotapi.Chat{ID: 12345},
		},
	}
	flow.handleSettingsCallback(
		context.Background(),
		callbackQuery,
	)

	// Invalid timezone
	timezoneCallback := &tgbotapi.CallbackQuery{
		ID:   "cb_inv_tz",
		Data: "settings:set_tz:Invalid/Location_999",
		From: &tgbotapi.User{ID: 12345, UserName: "tester"},
		Message: &tgbotapi.Message{
			MessageID: 200,
			Chat:      &tgbotapi.Chat{ID: 12345},
		},
	}
	flow.handleSettingsCallback(
		context.Background(),
		timezoneCallback,
	)
	if repository.lastTimezone == "Invalid/Location_999" {
		t.Error("expected invalid timezone to not be saved")
	}
}
