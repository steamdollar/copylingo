package bot

import (
	"context"
	"fmt"
	"log/slog"
	"strings"
	"time"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"

	"github.com/lsj/copylingo/internal/model"
)

type settingsUser interface {
	GetUser(
		ctx context.Context,
		telegramID int64,
		username string,
	) (*model.User, error)
	UpdateSlotTime(
		ctx context.Context,
		userID int64,
		slot model.SessionSlot,
		timeValue *string,
	) error
	UpdateTimezone(
		ctx context.Context,
		userID int64,
		timezone string,
	) error
}

type SettingsFlowDependencies struct {
	Telegram           *TelegramClient
	User               settingsUser
	MaterialPreference materialPreferenceList
}

type SettingsFlow struct {
	telegram           *TelegramClient
	user               settingsUser
	materialPreference materialPreferenceList
}

func NewSettingsFlow(dependencies SettingsFlowDependencies) *SettingsFlow {
	return &SettingsFlow{
		telegram:           dependencies.Telegram,
		user:               dependencies.User,
		materialPreference: dependencies.MaterialPreference,
	}
}

func (flow *SettingsFlow) handleSettingsCommand(
	ctx context.Context,
	message *tgbotapi.Message,
) {
	user, err := flow.user.GetUser(
		ctx,
		message.From.ID,
		message.From.UserName,
	)
	if err != nil {
		slog.ErrorContext(
			ctx,
			"Failed to get user for settings command",
			slog.Any(
				"error",
				err,
			),
		)
		flow.telegram.SendMessage(
			message.Chat.ID,
			botMessagesByLocale[botDefaultLocale].settingsLoadFailed,
		)
		return
	}

	text := buildSettingsOverviewText(user)
	keyboard := buildSettingsKeyboard(user)
	flow.telegram.SendMessageWithKeyboard(
		message.Chat.ID,
		text,
		keyboard,
	)
}

func (flow *SettingsFlow) handleSettingsCallback(
	ctx context.Context,
	callbackQuery *tgbotapi.CallbackQuery,
) {
	if callbackQuery == nil || callbackQuery.From == nil || callbackQuery.Message == nil ||
		callbackQuery.Message.Chat == nil {
		return
	}
	parts := strings.Split(
		callbackQuery.Data,
		":",
	)
	if len(parts) >= 2 && (parts[1] == callbackActionMaterials || parts[1] == callbackActionRestore) {
		flow.handleMaterialPreferencesCallback(
			ctx,
			callbackQuery,
		)
		return
	}
	user, err := flow.user.GetUser(
		ctx,
		callbackQuery.From.ID,
		callbackQuery.From.UserName,
	)
	if err != nil {
		slog.ErrorContext(
			ctx,
			"Failed to get user for settings callback",
			slog.Any(
				"error",
				err,
			),
		)
		flow.telegram.AnswerCallbackAlert(
			callbackQuery.ID,
			botMessagesByLocale[botDefaultLocale].settingsUserLoadFailed,
		)
		return
	}

	data := callbackQuery.Data
	switch {
	case data == callbackMenuSettings || data == callbackSettingsView:
		flow.renderSettingsView(
			ctx,
			callbackQuery,
			user,
		)

	case data == callbackSettingsTimezone:
		flow.renderTimezoneView(
			ctx,
			callbackQuery,
			user,
		)

	case strings.HasPrefix(
		data,
		callbackPrefixSettingsTimezone,
	):
		timezone := strings.TrimPrefix(
			data,
			callbackPrefixSettingsTimezone,
		)
		if err := flow.user.UpdateTimezone(
			ctx,
			user.ID,
			timezone,
		); err != nil {
			slog.ErrorContext(
				ctx,
				"Failed to update timezone",
				slog.String(
					"tz",
					timezone,
				),
				slog.Any(
					"error",
					err,
				),
			)
			flow.telegram.AnswerCallbackAlert(
				callbackQuery.ID,
				botMessagesByLocale[botDefaultLocale].settingsTimezoneInvalid,
			)
			return
		}
		user.Timezone = timezone
		flow.telegram.AnswerCallback(
			callbackQuery.ID,
			botMessagesByLocale[botDefaultLocale].settingsTimezoneChanged,
		)
		flow.renderSettingsView(
			ctx,
			callbackQuery,
			user,
		)

	case strings.HasPrefix(
		data,
		callbackPrefixSettingsSlot,
	):
		slotText := strings.TrimPrefix(
			data,
			callbackPrefixSettingsSlot,
		)
		isAll := false
		if strings.HasSuffix(
			slotText,
			":"+callbackActionAllTimes,
		) {
			isAll = true
			slotText = strings.TrimSuffix(
				slotText,
				":"+callbackActionAllTimes,
			)
		}
		slot := model.SessionSlot(slotText)
		if !isValidSlot(slot) {
			slog.WarnContext(
				ctx,
				"Invalid slot in callback",
				slog.String(
					"slot",
					slotText,
				),
			)
			return
		}
		flow.renderSlotPickerView(
			ctx,
			callbackQuery,
			user,
			slot,
			isAll,
		)

	case strings.HasPrefix(
		data,
		callbackPrefixSettingsSet,
	):
		payload := strings.TrimPrefix(
			data,
			callbackPrefixSettingsSet,
		)
		parts := strings.SplitN(
			payload,
			":",
			2,
		)
		if len(parts) != 2 {
			slog.WarnContext(
				ctx,
				"Malformed settings:set callback",
				slog.String(
					"data",
					data,
				),
			)
			return
		}
		slot := model.SessionSlot(parts[0])
		timeValue := parts[1]
		if !isValidSlot(slot) {
			slog.WarnContext(
				ctx,
				"Invalid slot in settings:set",
				slog.String(
					"slot",
					string(slot),
				),
			)
			return
		}

		if timeValue == callbackActionOff {
			if err := flow.user.UpdateSlotTime(
				ctx,
				user.ID,
				slot,
				nil,
			); err != nil {
				slog.ErrorContext(
					ctx,
					"Failed to disable slot time",
					slog.String(
						"slot",
						string(slot),
					),
					slog.Any(
						"error",
						err,
					),
				)
				flow.telegram.AnswerCallbackAlert(
					callbackQuery.ID,
					botMessagesByLocale[botDefaultLocale].settingsChangeFailed,
				)
				return
			}
			setUserSlotTime(
				user,
				slot,
				nil,
			)
			flow.telegram.AnswerCallback(
				callbackQuery.ID,
				botMessagesByLocale[botDefaultLocale].settingsNotificationsDisabled,
			)
		} else {
			if err := flow.user.UpdateSlotTime(
				ctx,
				user.ID,
				slot,
				&timeValue,
			); err != nil {
				slog.ErrorContext(
					ctx,
					"Failed to update slot time",
					slog.String(
						"slot",
						string(slot),
					),
					slog.String(
						"time",
						timeValue,
					),
					slog.Any(
						"error",
						err,
					),
				)
				flow.telegram.AnswerCallbackAlert(
					callbackQuery.ID,
					botMessagesByLocale[botDefaultLocale].settingsChangeFailed,
				)
				return
			}
			setUserSlotTime(
				user,
				slot,
				&timeValue,
			)
			flow.telegram.AnswerCallback(
				callbackQuery.ID,
				fmt.Sprintf(
					botMessagesByLocale[botDefaultLocale].settingsNotificationTimeSetFormat,
					timeValue,
				),
			)
		}
		flow.renderSettingsView(
			ctx,
			callbackQuery,
			user,
		)
	}
}

func (flow *SettingsFlow) renderSettingsView(
	ctx context.Context,
	callbackQuery *tgbotapi.CallbackQuery,
	user *model.User,
) {
	text := buildSettingsOverviewText(user)
	keyboard := buildSettingsKeyboard(user)
	if callbackQuery.Message != nil {
		flow.telegram.EditMessage(
			callbackQuery.Message.Chat.ID,
			callbackQuery.Message.MessageID,
			text,
			&keyboard,
		)
	}
}

func (flow *SettingsFlow) renderTimezoneView(
	ctx context.Context,
	callbackQuery *tgbotapi.CallbackQuery,
	user *model.User,
) {
	text := buildTimezoneText(user)
	keyboard := buildTimezoneKeyboard()
	if callbackQuery.Message != nil {
		flow.telegram.EditMessage(
			callbackQuery.Message.Chat.ID,
			callbackQuery.Message.MessageID,
			text,
			&keyboard,
		)
	}
}

func (flow *SettingsFlow) renderSlotPickerView(
	ctx context.Context,
	callbackQuery *tgbotapi.CallbackQuery,
	user *model.User,
	slot model.SessionSlot,
	isAll bool,
) {
	text := buildSlotPickerText(
		user,
		slot,
	)
	keyboard := buildSlotPickerKeyboard(
		slot,
		isAll,
	)
	if callbackQuery.Message != nil {
		flow.telegram.EditMessage(
			callbackQuery.Message.Chat.ID,
			callbackQuery.Message.MessageID,
			text,
			&keyboard,
		)
	}
}

func buildSettingsOverviewText(user *model.User) string {
	messages := botMessagesByLocale[botDefaultLocale]
	timezone := user.Timezone
	if timezone == "" {
		timezone = "Asia/Seoul"
	}
	mStudy := formatSlotTime(user.MorningStudyTime)
	mQuiz := formatSlotTime(user.MorningQuizTime)
	eStudy := formatSlotTime(user.EveningStudyTime)
	eQuiz := formatSlotTime(user.EveningQuizTime)

	return fmt.Sprintf(
		messages.settingsOverviewFormat,
		timezone,
		messages.settingsMorningStudyLabel,
		mStudy,
		messages.settingsMorningQuizLabel,
		mQuiz,
		messages.settingsEveningStudyLabel,
		eStudy,
		messages.settingsEveningQuizLabel,
		eQuiz,
	)
}

func buildSettingsKeyboard(user *model.User) tgbotapi.InlineKeyboardMarkup {
	messages := botMessagesByLocale[botDefaultLocale]
	mStudy := formatSlotTime(user.MorningStudyTime)
	mQuiz := formatSlotTime(user.MorningQuizTime)
	eStudy := formatSlotTime(user.EveningStudyTime)
	eQuiz := formatSlotTime(user.EveningQuizTime)

	return tgbotapi.NewInlineKeyboardMarkup(
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData(
				fmt.Sprintf(
					messages.settingsSlotButtonFormat,
					"🌅",
					messages.settingsMorningStudyLabel,
					mStudy,
				),
				fmt.Sprintf(
					formatSettingsSlot,
					model.SessionSlotMorningStudy,
				),
			),
		),
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData(
				fmt.Sprintf(
					messages.settingsSlotButtonFormat,
					"📝",
					messages.settingsMorningQuizLabel,
					mQuiz,
				),
				fmt.Sprintf(
					formatSettingsSlot,
					model.SessionSlotMorningQuiz,
				),
			),
		),
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData(
				fmt.Sprintf(
					messages.settingsSlotButtonFormat,
					"🌆",
					messages.settingsEveningStudyLabel,
					eStudy,
				),
				fmt.Sprintf(
					formatSettingsSlot,
					model.SessionSlotEveningStudy,
				),
			),
		),
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData(
				fmt.Sprintf(
					messages.settingsSlotButtonFormat,
					"🌙",
					messages.settingsEveningQuizLabel,
					eQuiz,
				),
				fmt.Sprintf(
					formatSettingsSlot,
					model.SessionSlotEveningQuiz,
				),
			),
		),
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData(
				messages.settingsTimezoneChangeButton,
				callbackSettingsTimezone,
			),
		),
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData(
				messages.materialSettingsListButton,
				fmt.Sprintf(
					formatMaterialPreferences,
					0,
				),
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

func buildSlotPickerText(
	user *model.User,
	slot model.SessionSlot,
) string {
	icon := slotDisplayIcon(slot)
	name := slotDisplayName(slot)
	current := formatSlotTime(getUserSlotTime(
		user,
		slot,
	))

	return fmt.Sprintf(
		botMessagesByLocale[botDefaultLocale].settingsSlotPickerFormat,
		icon,
		name,
		current,
	)
}

func buildSlotPickerKeyboard(
	slot model.SessionSlot,
	isAll bool,
) tgbotapi.InlineKeyboardMarkup {
	var times []string
	if !isAll {
		switch slot {
		case model.SessionSlotMorningStudy, model.SessionSlotMorningQuiz:
			times = []string{
				"06:00", "06:30", "07:00", "07:30",
				"08:00", "08:30", "09:00", "09:30",
				"10:00", "10:30", "11:00", "11:30",
				"12:00", "12:30", "13:00", "13:30",
			}
		case model.SessionSlotEveningStudy, model.SessionSlotEveningQuiz:
			times = []string{
				"15:00", "15:30", "16:00", "16:30",
				"17:00", "17:30", "18:00", "18:30",
				"19:00", "19:30", "20:00", "20:30",
				"21:00", "21:30", "22:00", "22:30",
			}
		default:
			isAll = true
		}
	}

	if isAll {
		times = make(
			[]string,
			0,
			48,
		)
		for hour := 0; hour < 24; hour++ {
			times = append(
				times,
				fmt.Sprintf(
					"%02d:00",
					hour,
				),
				fmt.Sprintf(
					"%02d:30",
					hour,
				),
			)
		}
	}

	var rows [][]tgbotapi.InlineKeyboardButton
	for i := 0; i < len(times); i += 4 {
		end := i + 4
		if end > len(times) {
			end = len(times)
		}
		var row []tgbotapi.InlineKeyboardButton
		for _, slotTime := range times[i:end] {
			callbackData := fmt.Sprintf(
				formatSettingsSet,
				slot,
				slotTime,
			)
			row = append(
				row,
				tgbotapi.NewInlineKeyboardButtonData(
					slotTime,
					callbackData,
				),
			)
		}
		rows = append(
			rows,
			row,
		)
	}

	rows = append(
		rows,
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData(
				botMessagesByLocale[botDefaultLocale].settingsDisableNotificationsButton,
				fmt.Sprintf(
					formatSettingsSet,
					slot,
					callbackActionOff,
				),
			),
		),
	)

	var navigationRow []tgbotapi.InlineKeyboardButton
	if !isAll {
		navigationRow = append(
			navigationRow,
			tgbotapi.NewInlineKeyboardButtonData(
				botMessagesByLocale[botDefaultLocale].settingsAllTimesButton,
				fmt.Sprintf(
					formatSettingsSlotAll,
					slot,
				),
			),
		)
	} else {
		navigationRow = append(
			navigationRow,
			tgbotapi.NewInlineKeyboardButtonData(
				botMessagesByLocale[botDefaultLocale].settingsDefaultTimesButton,
				fmt.Sprintf(
					formatSettingsSlot,
					slot,
				),
			),
		)
	}
	navigationRow = append(
		navigationRow,
		tgbotapi.NewInlineKeyboardButtonData(
			botMessagesByLocale[botDefaultLocale].settingsBackButton,
			callbackSettingsView,
		),
	)
	rows = append(
		rows,
		navigationRow,
	)

	return tgbotapi.NewInlineKeyboardMarkup(rows...)
}

func buildTimezoneText(user *model.User) string {
	current := user.Timezone
	if current == "" {
		current = "Asia/Seoul"
	}
	return fmt.Sprintf(
		botMessagesByLocale[botDefaultLocale].settingsTimezoneFormat,
		current,
	)
}

func buildTimezoneKeyboard() tgbotapi.InlineKeyboardMarkup {
	return tgbotapi.NewInlineKeyboardMarkup(
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData(
				botMessagesByLocale[botDefaultLocale].settingsTimezoneSeoulButton,
				fmt.Sprintf(
					formatSettingsTimezone,
					"Asia/Seoul",
				),
			),
			tgbotapi.NewInlineKeyboardButtonData(
				botMessagesByLocale[botDefaultLocale].settingsTimezoneTokyoButton,
				fmt.Sprintf(
					formatSettingsTimezone,
					"Asia/Tokyo",
				),
			),
		),
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData(
				botMessagesByLocale[botDefaultLocale].settingsTimezoneNewYorkButton,
				fmt.Sprintf(
					formatSettingsTimezone,
					"America/New_York",
				),
			),
			tgbotapi.NewInlineKeyboardButtonData(
				botMessagesByLocale[botDefaultLocale].settingsTimezoneLosAngelesButton,
				fmt.Sprintf(
					formatSettingsTimezone,
					"America/Los_Angeles",
				),
			),
		),
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData(
				botMessagesByLocale[botDefaultLocale].settingsTimezoneLondonButton,
				fmt.Sprintf(
					formatSettingsTimezone,
					"Europe/London",
				),
			),
			tgbotapi.NewInlineKeyboardButtonData(
				botMessagesByLocale[botDefaultLocale].settingsTimezoneUTCButton,
				fmt.Sprintf(
					formatSettingsTimezone,
					"UTC",
				),
			),
		),
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData(
				botMessagesByLocale[botDefaultLocale].settingsBackButton,
				callbackSettingsView,
			),
		),
	)
}

func formatSlotTime(slotTime *string) string {
	if slotTime == nil || *slotTime == "" {
		return botMessagesByLocale[botDefaultLocale].settingsDisabledSlotLabel
	}
	if parsed, err := time.Parse(
		time.RFC3339,
		*slotTime,
	); err == nil {
		return parsed.Format("15:04")
	}
	return *slotTime
}

func slotDisplayName(slot model.SessionSlot) string {
	messages := botMessagesByLocale[botDefaultLocale]
	switch slot {
	case model.SessionSlotMorningStudy:
		return messages.settingsMorningStudyLabel
	case model.SessionSlotMorningQuiz:
		return messages.settingsMorningQuizLabel
	case model.SessionSlotEveningStudy:
		return messages.settingsEveningStudyLabel
	case model.SessionSlotEveningQuiz:
		return messages.settingsEveningQuizLabel
	default:
		return string(slot)
	}
}

func slotDisplayIcon(slot model.SessionSlot) string {
	switch slot {
	case model.SessionSlotMorningStudy:
		return "🌅"
	case model.SessionSlotMorningQuiz:
		return "📝"
	case model.SessionSlotEveningStudy:
		return "🌆"
	case model.SessionSlotEveningQuiz:
		return "🌙"
	default:
		return "⏰"
	}
}

func getUserSlotTime(
	user *model.User,
	slot model.SessionSlot,
) *string {
	switch slot {
	case model.SessionSlotMorningStudy:
		return user.MorningStudyTime
	case model.SessionSlotMorningQuiz:
		return user.MorningQuizTime
	case model.SessionSlotEveningStudy:
		return user.EveningStudyTime
	case model.SessionSlotEveningQuiz:
		return user.EveningQuizTime
	default:
		return nil
	}
}

func setUserSlotTime(
	user *model.User,
	slot model.SessionSlot,
	slotTime *string,
) {
	switch slot {
	case model.SessionSlotMorningStudy:
		user.MorningStudyTime = slotTime
	case model.SessionSlotMorningQuiz:
		user.MorningQuizTime = slotTime
	case model.SessionSlotEveningStudy:
		user.EveningStudyTime = slotTime
	case model.SessionSlotEveningQuiz:
		user.EveningQuizTime = slotTime
	}
}

func isValidSlot(slot model.SessionSlot) bool {
	switch slot {
	case model.SessionSlotMorningStudy,
		model.SessionSlotMorningQuiz,
		model.SessionSlotEveningStudy,
		model.SessionSlotEveningQuiz:
		return true
	default:
		return false
	}
}
