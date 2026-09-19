package bot

import (
	"context"
	"fmt"
	"log/slog"
	"strings"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"

	"github.com/lsj/copylingo/internal/config"
	"github.com/lsj/copylingo/internal/model"
)

// handleSettingsCommand handles /settings command by displaying the schedule configuration menu.
func (b *Bot) handleSettingsCommand(ctx context.Context, msg *tgbotapi.Message) {
	user, err := b.services.User.GetUser(ctx, msg.From.ID, msg.From.UserName)
	if err != nil {
		slog.ErrorContext(ctx, "Failed to get user for settings command", slog.Any("error", err))
		b.SendMessage(msg.Chat.ID, "❌ 설정 정보를 불러오지 못했습니다.")
		return
	}

	text := buildSettingsOverviewText(user)
	keyboard := buildSettingsKeyboard(user)
	b.SendMessageWithKeyboard(msg.Chat.ID, text, keyboard)
}

// handleSettingsCallback routes callbacks related to push schedule & timezone settings.
func (b *Bot) handleSettingsCallback(ctx context.Context, cb *tgbotapi.CallbackQuery) {
	user, err := b.services.User.GetUser(ctx, cb.From.ID, cb.From.UserName)
	if err != nil {
		slog.ErrorContext(ctx, "Failed to get user for settings callback", slog.Any("error", err))
		b.api.Request(tgbotapi.NewCallbackWithAlert(cb.ID, "❌ 사용자 정보를 불러오지 못했습니다."))
		return
	}

	data := cb.Data
	switch {
	case data == config.ActionMenuSettings || data == config.ActionSettingsView:
		b.renderSettingsView(ctx, cb, user)

	case data == config.ActionSettingsTimezone:
		b.renderTimezoneView(ctx, cb, user)

	case strings.HasPrefix(data, "settings:set_tz:"):
		tz := strings.TrimPrefix(data, "settings:set_tz:")
		if err := b.services.User.UpdateTimezone(ctx, user.ID, tz); err != nil {
			slog.ErrorContext(ctx, "Failed to update timezone", slog.String("tz", tz), slog.Any("error", err))
			b.api.Request(tgbotapi.NewCallbackWithAlert(cb.ID, "❌ 유효하지 않은 시간대입니다."))
			return
		}
		user.Timezone = tz
		b.api.Request(tgbotapi.NewCallback(cb.ID, "✅ 시간대가 변경되었습니다."))
		b.renderSettingsView(ctx, cb, user)

	case strings.HasPrefix(data, "settings:slot:"):
		slotStr := strings.TrimPrefix(data, "settings:slot:")
		isAll := false
		if strings.HasSuffix(slotStr, ":all") {
			isAll = true
			slotStr = strings.TrimSuffix(slotStr, ":all")
		}
		slot := model.SessionSlot(slotStr)
		if !isValidSlot(slot) {
			slog.WarnContext(ctx, "Invalid slot in callback", slog.String("slot", slotStr))
			return
		}
		b.renderSlotPickerView(ctx, cb, user, slot, isAll)

	case strings.HasPrefix(data, "settings:set:"):
		payload := strings.TrimPrefix(data, "settings:set:")
		parts := strings.SplitN(payload, ":", 2)
		if len(parts) != 2 {
			slog.WarnContext(ctx, "Malformed settings:set callback", slog.String("data", data))
			return
		}
		slot := model.SessionSlot(parts[0])
		timeVal := parts[1]
		if !isValidSlot(slot) {
			slog.WarnContext(ctx, "Invalid slot in settings:set", slog.String("slot", string(slot)))
			return
		}

		if timeVal == "off" {
			if err := b.services.User.UpdateSlotTime(ctx, user.ID, slot, nil); err != nil {
				slog.ErrorContext(
					ctx,
					"Failed to disable slot time",
					slog.String("slot", string(slot)),
					slog.Any("error", err),
				)
				b.api.Request(tgbotapi.NewCallbackWithAlert(cb.ID, "❌ 설정 변경에 실패했습니다."))
				return
			}
			setUserSlotTime(user, slot, nil)
			b.api.Request(tgbotapi.NewCallback(cb.ID, "🔕 알림이 비활성화되었습니다."))
		} else {
			if err := b.services.User.UpdateSlotTime(ctx, user.ID, slot, &timeVal); err != nil {
				slog.ErrorContext(
					ctx,
					"Failed to update slot time",
					slog.String("slot", string(slot)),
					slog.String("time", timeVal),
					slog.Any("error", err),
				)
				b.api.Request(tgbotapi.NewCallbackWithAlert(cb.ID, "❌ 설정 변경에 실패했습니다."))
				return
			}
			setUserSlotTime(user, slot, &timeVal)
			b.api.Request(tgbotapi.NewCallback(cb.ID, fmt.Sprintf("✅ %s(으)로 설정되었습니다.", timeVal)))
		}
		b.renderSettingsView(ctx, cb, user)
	}
}

func (b *Bot) renderSettingsView(ctx context.Context, cb *tgbotapi.CallbackQuery, u *model.User) {
	text := buildSettingsOverviewText(u)
	keyboard := buildSettingsKeyboard(u)
	if cb.Message != nil {
		b.EditMessage(cb.Message.Chat.ID, cb.Message.MessageID, text, &keyboard)
	}
}

func (b *Bot) renderTimezoneView(ctx context.Context, cb *tgbotapi.CallbackQuery, u *model.User) {
	text := buildTimezoneText(u)
	keyboard := buildTimezoneKeyboard()
	if cb.Message != nil {
		b.EditMessage(cb.Message.Chat.ID, cb.Message.MessageID, text, &keyboard)
	}
}

func (b *Bot) renderSlotPickerView(
	ctx context.Context,
	cb *tgbotapi.CallbackQuery,
	u *model.User,
	slot model.SessionSlot,
	isAll bool,
) {
	text := buildSlotPickerText(u, slot)
	keyboard := buildSlotPickerKeyboard(slot, isAll)
	if cb.Message != nil {
		b.EditMessage(cb.Message.Chat.ID, cb.Message.MessageID, text, &keyboard)
	}
}

func buildSettingsOverviewText(u *model.User) string {
	tz := u.Timezone
	if tz == "" {
		tz = "Asia/Seoul"
	}
	mStudy := formatSlotTime(u.MorningStudyTime)
	mQuiz := formatSlotTime(u.MorningQuizTime)
	eStudy := formatSlotTime(u.EveningStudyTime)
	eQuiz := formatSlotTime(u.EveningQuizTime)

	return fmt.Sprintf(`⚙️ <b>푸시 알림 및 스케줄 설정</b>

현재 시간대: <b>%s</b>
각 슬롯별 알림 시각을 변경하거나 끌 수 있습니다.

🌅 오전 학습: <b>%s</b>
📝 오전 퀴즈: <b>%s</b>
🌆 오후 학습: <b>%s</b>
🌙 저녁 퀴즈: <b>%s</b>

💡 30분 단위로 시각을 지정할 수 있습니다.`,
		tz, mStudy, mQuiz, eStudy, eQuiz,
	)
}

func buildSettingsKeyboard(u *model.User) tgbotapi.InlineKeyboardMarkup {
	mStudy := formatSlotTime(u.MorningStudyTime)
	mQuiz := formatSlotTime(u.MorningQuizTime)
	eStudy := formatSlotTime(u.EveningStudyTime)
	eQuiz := formatSlotTime(u.EveningQuizTime)

	return tgbotapi.NewInlineKeyboardMarkup(
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData(fmt.Sprintf("🌅 오전 학습 (%s)", mStudy), "settings:slot:morning_study"),
		),
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData(fmt.Sprintf("📝 오전 퀴즈 (%s)", mQuiz), "settings:slot:morning_quiz"),
		),
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData(fmt.Sprintf("🌆 오후 학습 (%s)", eStudy), "settings:slot:evening_study"),
		),
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData(fmt.Sprintf("🌙 저녁 퀴즈 (%s)", eQuiz), "settings:slot:evening_quiz"),
		),
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData("🌍 시간대 변경", config.ActionSettingsTimezone),
		),
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData("🏠 메뉴로", config.ActionMenuMain),
		),
	)
}

func buildSlotPickerText(u *model.User, slot model.SessionSlot) string {
	icon := slotDisplayIcon(slot)
	name := slotDisplayName(slot)
	curr := formatSlotTime(getUserSlotTime(u, slot))

	return fmt.Sprintf(`⚙️ <b>%s %s 시각 설정</b>

현재 설정: <b>%s</b>
원하는 시각을 선택하세요 (30분 단위):`,
		icon, name, curr,
	)
}

func buildSlotPickerKeyboard(slot model.SessionSlot, isAll bool) tgbotapi.InlineKeyboardMarkup {
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
		times = make([]string, 0, 48)
		for h := 0; h < 24; h++ {
			times = append(times, fmt.Sprintf("%02d:00", h), fmt.Sprintf("%02d:30", h))
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
			cbData := fmt.Sprintf("settings:set:%s:%s", slot, t)
			row = append(row, tgbotapi.NewInlineKeyboardButtonData(t, cbData))
		}
		rows = append(rows, row)
	}

	// Action row: OFF button
	rows = append(rows, tgbotapi.NewInlineKeyboardRow(
		tgbotapi.NewInlineKeyboardButtonData("🔕 알림 끄기 (OFF)", fmt.Sprintf("settings:set:%s:off", slot)),
	))

	// Navigation row
	var navRow []tgbotapi.InlineKeyboardButton
	if !isAll {
		navRow = append(
			navRow,
			tgbotapi.NewInlineKeyboardButtonData("🕒 전체 시간 (24h)", fmt.Sprintf("settings:slot:%s:all", slot)),
		)
	} else {
		navRow = append(navRow, tgbotapi.NewInlineKeyboardButtonData("🕒 기본 시간대", fmt.Sprintf("settings:slot:%s", slot)))
	}
	navRow = append(navRow, tgbotapi.NewInlineKeyboardButtonData("⬅️ 설정 목록으로", config.ActionSettingsView))
	rows = append(rows, navRow)

	return tgbotapi.NewInlineKeyboardMarkup(rows...)
}

func buildTimezoneText(u *model.User) string {
	curr := u.Timezone
	if curr == "" {
		curr = "Asia/Seoul"
	}
	return fmt.Sprintf(`🌍 <b>시간대(Timezone) 설정</b>

현재 설정: <b>%s</b>
알림 발송 기준이 되는 시간대를 선택하세요:`, curr)
}

func buildTimezoneKeyboard() tgbotapi.InlineKeyboardMarkup {
	return tgbotapi.NewInlineKeyboardMarkup(
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData("🇰🇷 서울 (Asia/Seoul)", "settings:set_tz:Asia/Seoul"),
			tgbotapi.NewInlineKeyboardButtonData("🇯🇵 도쿄 (Asia/Tokyo)", "settings:set_tz:Asia/Tokyo"),
		),
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData("🇺🇸 뉴욕 (America/New_York)", "settings:set_tz:America/New_York"),
			tgbotapi.NewInlineKeyboardButtonData("🇺🇸 LA (America/Los_Angeles)", "settings:set_tz:America/Los_Angeles"),
		),
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData("🇬🇧 런던 (Europe/London)", "settings:set_tz:Europe/London"),
			tgbotapi.NewInlineKeyboardButtonData("🌐 UTC", "settings:set_tz:UTC"),
		),
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData("⬅️ 설정 목록으로", config.ActionSettingsView),
		),
	)
}

func formatSlotTime(t *string) string {
	if t == nil || *t == "" {
		return "🔕 꺼짐"
	}
	return *t
}

func slotDisplayName(slot model.SessionSlot) string {
	switch slot {
	case model.SessionSlotMorningStudy:
		return "오전 학습"
	case model.SessionSlotMorningQuiz:
		return "오전 퀴즈"
	case model.SessionSlotEveningStudy:
		return "오후 학습"
	case model.SessionSlotEveningQuiz:
		return "저녁 퀴즈"
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

func getUserSlotTime(u *model.User, slot model.SessionSlot) *string {
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

func setUserSlotTime(u *model.User, slot model.SessionSlot, t *string) {
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
