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

// settingsUser loads the user behind an update and saves their push
// schedule and timezone.
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
		timeVal *string,
	) error
	UpdateTimezone(
		ctx context.Context,
		userID int64,
		tz string,
	) error
}

// SettingsFlowDeps wires SettingsFlow; every dependency is required.
type SettingsFlowDeps struct {
	Telegram           *TelegramClient
	User               settingsUser
	MaterialPreference materialPreferenceList
}

// SettingsFlow handles the /settings screens: push schedule, timezone, and
// the list of materials whose review mode the user changed.
type SettingsFlow struct {
	telegram           *TelegramClient
	user               settingsUser
	materialPreference materialPreferenceList
}

func NewSettingsFlow(deps SettingsFlowDeps) *SettingsFlow {
	return &SettingsFlow{
		telegram:           deps.Telegram,
		user:               deps.User,
		materialPreference: deps.MaterialPreference,
	}
}

// handleSettingsCommand handles /settings command by displaying the schedule configuration menu.
func (sf *SettingsFlow) handleSettingsCommand(
	ctx context.Context,
	msg *tgbotapi.Message,
) {
	user, err := sf.user.GetUser(
		ctx,
		msg.From.ID,
		msg.From.UserName,
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
		sf.telegram.SendMessage(
			msg.Chat.ID,
			botMessagesByLocale[botDefaultLocale].settingsLoadFailed,
		)
		return
	}

	text := buildSettingsOverviewText(user)
	keyboard := buildSettingsKeyboard(user)
	sf.telegram.SendMessageWithKeyboard(
		msg.Chat.ID,
		text,
		keyboard,
	)
}

// handleSettingsCallback routes callbacks related to push schedule & timezone settings.
func (sf *SettingsFlow) handleSettingsCallback(
	ctx context.Context,
	cb *tgbotapi.CallbackQuery,
) {
	if cb == nil || cb.From == nil || cb.Message == nil || cb.Message.Chat == nil {
		return
	}
	parts := strings.Split(
		cb.Data,
		":",
	)
	if len(parts) >= 2 && (parts[1] == callbackActionMaterials || parts[1] == callbackActionRestore) {
		sf.handleMaterialPreferencesCallback(
			ctx,
			cb,
		)
		return
	}
	user, err := sf.user.GetUser(
		ctx,
		cb.From.ID,
		cb.From.UserName,
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
		sf.telegram.AnswerCallbackAlert(
			cb.ID,
			botMessagesByLocale[botDefaultLocale].settingsUserLoadFailed,
		)
		return
	}

	data := cb.Data
	switch {
	case data == callbackMenuSettings || data == callbackSettingsView:
		sf.renderSettingsView(
			ctx,
			cb,
			user,
		)

	case data == callbackSettingsTimezone:
		sf.renderTimezoneView(
			ctx,
			cb,
			user,
		)

	case strings.HasPrefix(
		data,
		callbackPrefixSettingsTimezone,
	):
		tz := strings.TrimPrefix(
			data,
			callbackPrefixSettingsTimezone,
		)
		if err := sf.user.UpdateTimezone(
			ctx,
			user.ID,
			tz,
		); err != nil {
			slog.ErrorContext(
				ctx,
				"Failed to update timezone",
				slog.String(
					"tz",
					tz,
				),
				slog.Any(
					"error",
					err,
				),
			)
			sf.telegram.AnswerCallbackAlert(
				cb.ID,
				botMessagesByLocale[botDefaultLocale].settingsTimezoneInvalid,
			)
			return
		}
		user.Timezone = tz
		sf.telegram.AnswerCallback(
			cb.ID,
			botMessagesByLocale[botDefaultLocale].settingsTimezoneChanged,
		)
		sf.renderSettingsView(
			ctx,
			cb,
			user,
		)

	case strings.HasPrefix(
		data,
		callbackPrefixSettingsSlot,
	):
		slotStr := strings.TrimPrefix(
			data,
			callbackPrefixSettingsSlot,
		)
		isAll := false
		if strings.HasSuffix(
			slotStr,
			":"+callbackActionAllTimes,
		) {
			isAll = true
			slotStr = strings.TrimSuffix(
				slotStr,
				":"+callbackActionAllTimes,
			)
		}
		slot := model.SessionSlot(slotStr)
		if !isValidSlot(slot) {
			slog.WarnContext(
				ctx,
				"Invalid slot in callback",
				slog.String(
					"slot",
					slotStr,
				),
			)
			return
		}
		sf.renderSlotPickerView(
			ctx,
			cb,
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
		timeVal := parts[1]
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

		if timeVal == callbackActionOff {
			if err := sf.user.UpdateSlotTime(
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
				sf.telegram.AnswerCallbackAlert(
					cb.ID,
					botMessagesByLocale[botDefaultLocale].settingsChangeFailed,
				)
				return
			}
			setUserSlotTime(
				user,
				slot,
				nil,
			)
			sf.telegram.AnswerCallback(
				cb.ID,
				botMessagesByLocale[botDefaultLocale].settingsNotificationsDisabled,
			)
		} else {
			if err := sf.user.UpdateSlotTime(
				ctx,
				user.ID,
				slot,
				&timeVal,
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
						timeVal,
					),
					slog.Any(
						"error",
						err,
					),
				)
				sf.telegram.AnswerCallbackAlert(
					cb.ID,
					botMessagesByLocale[botDefaultLocale].settingsChangeFailed,
				)
				return
			}
			setUserSlotTime(
				user,
				slot,
				&timeVal,
			)
			sf.telegram.AnswerCallback(
				cb.ID,
				fmt.Sprintf(
					botMessagesByLocale[botDefaultLocale].settingsNotificationTimeSetFormat,
					timeVal,
				),
			)
		}
		sf.renderSettingsView(
			ctx,
			cb,
			user,
		)
	}
}

func (sf *SettingsFlow) renderSettingsView(
	ctx context.Context,
	cb *tgbotapi.CallbackQuery,
	u *model.User,
) {
	text := buildSettingsOverviewText(u)
	keyboard := buildSettingsKeyboard(u)
	if cb.Message != nil {
		sf.telegram.EditMessage(
			cb.Message.Chat.ID,
			cb.Message.MessageID,
			text,
			&keyboard,
		)
	}
}

func (sf *SettingsFlow) renderTimezoneView(
	ctx context.Context,
	cb *tgbotapi.CallbackQuery,
	u *model.User,
) {
	text := buildTimezoneText(u)
	keyboard := buildTimezoneKeyboard()
	if cb.Message != nil {
		sf.telegram.EditMessage(
			cb.Message.Chat.ID,
			cb.Message.MessageID,
			text,
			&keyboard,
		)
	}
}

func (sf *SettingsFlow) renderSlotPickerView(
	ctx context.Context,
	cb *tgbotapi.CallbackQuery,
	u *model.User,
	slot model.SessionSlot,
	isAll bool,
) {
	text := buildSlotPickerText(
		u,
		slot,
	)
	keyboard := buildSlotPickerKeyboard(
		slot,
		isAll,
	)
	if cb.Message != nil {
		sf.telegram.EditMessage(
			cb.Message.Chat.ID,
			cb.Message.MessageID,
			text,
			&keyboard,
		)
	}
}

func buildSettingsOverviewText(u *model.User) string {
	messages := botMessagesByLocale[botDefaultLocale]
	tz := u.Timezone
	if tz == "" {
		tz = "Asia/Seoul"
	}
	mStudy := formatSlotTime(u.MorningStudyTime)
	mQuiz := formatSlotTime(u.MorningQuizTime)
	eStudy := formatSlotTime(u.EveningStudyTime)
	eQuiz := formatSlotTime(u.EveningQuizTime)

	return fmt.Sprintf(
		messages.settingsOverviewFormat,
		tz,
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

func buildSettingsKeyboard(u *model.User) tgbotapi.InlineKeyboardMarkup {
	messages := botMessagesByLocale[botDefaultLocale]
	mStudy := formatSlotTime(u.MorningStudyTime)
	mQuiz := formatSlotTime(u.MorningQuizTime)
	eStudy := formatSlotTime(u.EveningStudyTime)
	eQuiz := formatSlotTime(u.EveningQuizTime)

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
	u *model.User,
	slot model.SessionSlot,
) string {
	icon := slotDisplayIcon(slot)
	name := slotDisplayName(slot)
	curr := formatSlotTime(getUserSlotTime(
		u,
		slot,
	))

	return fmt.Sprintf(
		botMessagesByLocale[botDefaultLocale].settingsSlotPickerFormat,
		icon,
		name,
		curr,
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
		for h := 0; h < 24; h++ {
			times = append(
				times,
				fmt.Sprintf(
					"%02d:00",
					h,
				),
				fmt.Sprintf(
					"%02d:30",
					h,
				),
			)
		}
	}

	var rows [][]tgbotapi.InlineKeyboardButton
	// Group into rows of 4
	for i := 0; i < len(times); i += 4 {
		end := i + 4
		if end > len(times) {
			end = len(times)
		}
		var row []tgbotapi.InlineKeyboardButton
		for _, t := range times[i:end] {
			cbData := fmt.Sprintf(
				formatSettingsSet,
				slot,
				t,
			)
			row = append(
				row,
				tgbotapi.NewInlineKeyboardButtonData(
					t,
					cbData,
				),
			)
		}
		rows = append(
			rows,
			row,
		)
	}

	// Action row: OFF button
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

	// Navigation row
	var navRow []tgbotapi.InlineKeyboardButton
	if !isAll {
		navRow = append(
			navRow,
			tgbotapi.NewInlineKeyboardButtonData(
				botMessagesByLocale[botDefaultLocale].settingsAllTimesButton,
				fmt.Sprintf(
					formatSettingsSlotAll,
					slot,
				),
			),
		)
	} else {
		navRow = append(
			navRow,
			tgbotapi.NewInlineKeyboardButtonData(
				botMessagesByLocale[botDefaultLocale].settingsDefaultTimesButton,
				fmt.Sprintf(
					formatSettingsSlot,
					slot,
				),
			),
		)
	}
	navRow = append(
		navRow,
		tgbotapi.NewInlineKeyboardButtonData(
			botMessagesByLocale[botDefaultLocale].settingsBackButton,
			callbackSettingsView,
		),
	)
	rows = append(
		rows,
		navRow,
	)

	return tgbotapi.NewInlineKeyboardMarkup(rows...)
}

func buildTimezoneText(u *model.User) string {
	curr := u.Timezone
	if curr == "" {
		curr = "Asia/Seoul"
	}
	return fmt.Sprintf(
		botMessagesByLocale[botDefaultLocale].settingsTimezoneFormat,
		curr,
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

func formatSlotTime(t *string) string {
	if t == nil || *t == "" {
		return botMessagesByLocale[botDefaultLocale].settingsDisabledSlotLabel
	}
	if parsed, err := time.Parse(
		time.RFC3339,
		*t,
	); err == nil {
		return parsed.Format("15:04")
	}
	return *t
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
	u *model.User,
	slot model.SessionSlot,
) *string {
	switch slot {
	case model.SessionSlotMorningStudy:
		return u.MorningStudyTime
	case model.SessionSlotMorningQuiz:
		return u.MorningQuizTime
	case model.SessionSlotEveningStudy:
		return u.EveningStudyTime
	case model.SessionSlotEveningQuiz:
		return u.EveningQuizTime
	default:
		return nil
	}
}

func setUserSlotTime(
	u *model.User,
	slot model.SessionSlot,
	t *string,
) {
	switch slot {
	case model.SessionSlotMorningStudy:
		u.MorningStudyTime = t
	case model.SessionSlotMorningQuiz:
		u.MorningQuizTime = t
	case model.SessionSlotEveningStudy:
		u.EveningStudyTime = t
	case model.SessionSlotEveningQuiz:
		u.EveningQuizTime = t
	}
}

func isValidSlot(s model.SessionSlot) bool {
	switch s {
	case model.SessionSlotMorningStudy,
		model.SessionSlotMorningQuiz,
		model.SessionSlotEveningStudy,
		model.SessionSlotEveningQuiz:
		return true
	default:
		return false
	}
}
