package bot

import (
	"context"
	"fmt"
	"log/slog"
	"strconv"
	"strings"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"

	"github.com/lsj/copylingo/internal/config"
	"github.com/lsj/copylingo/internal/model"
	"github.com/lsj/copylingo/internal/service"
)

const materialPreferenceNotice = "새로 만드는 세션부터 적용됩니다. 현재 세션은 그대로 진행합니다."

// Quiz callbacks carry a question ID, never a trusted material ID. Resolve it
// from the owner's session before showing or changing the linked preference.
func (sf *SessionFlow) handleQuizMaterialPreference(ctx context.Context, cb *tgbotapi.CallbackQuery, parts []string) {
	if cb == nil || cb.From == nil || cb.Message == nil || cb.Message.Chat == nil || len(cb.Data) > 64 ||
		len(parts) < 4 || len(parts) > 5 || parts[0] != "q" ||
		(parts[2] != "policy" && (parts[2] != "exclude" || len(parts) != 4)) ||
		sf.bot.services == nil || sf.bot.services.QuizActiveSession == nil || sf.bot.services.MaterialPreference == nil {
		return
	}
	sessionID, sessionOK := materialPreferenceInt(parts[1], false)
	questionID, questionOK := materialPreferenceInt(parts[3], false)
	if !sessionOK || !questionOK {
		return
	}
	state, err := sf.bot.services.QuizActiveSession.Get(ctx, sessionID)
	if err != nil || state == nil || state.Session.ID != sessionID || state.Session.UserID != cb.From.ID ||
		state.Session.Mode != model.SessionModeQuiz {
		slog.WarnContext(ctx, "Rejected quiz material preference callback",
			"session_id", sessionID, "question_id", questionID, "user_id", cb.From.ID, "error", err)
		return
	}
	item, exists := state.ItemByQuestionID(questionID)
	if !exists || item.Question.MaterialID == nil {
		return
	}
	var mode model.MaterialReviewMode
	if len(parts) == 5 {
		mode = model.MaterialReviewMode(parts[4])
		if mode != model.MaterialReviewMaintenance && mode != model.MaterialReviewExcluded {
			return
		}
		if err := sf.bot.services.MaterialPreference.Set(ctx, cb.From.ID, *item.Question.MaterialID, mode); err != nil {
			slog.ErrorContext(ctx, "Failed to set linked quiz material preference",
				"session_id", sessionID, "question_id", questionID, "user_id", cb.From.ID, "error", err)
			sf.bot.SendMessage(cb.Message.Chat.ID, "❌ 연결 자료 설정을 저장하지 못했습니다. 다시 시도해 주세요.")
			return
		}
		sf.bot.EditMessage(
			cb.Message.Chat.ID,
			cb.Message.MessageID,
			fmt.Sprintf(
				"✅ %s로 설정했습니다.\n\n%s\n원래 Quiz 메시지에서 계속하세요.",
				materialReviewModeLabel(mode),
				materialPreferenceNotice,
			),
			nil,
		)
		return
	}
	preference, err := sf.bot.services.MaterialPreference.Get(ctx, cb.From.ID, *item.Question.MaterialID)
	if err != nil {
		slog.ErrorContext(ctx, "Failed to get linked quiz material preference",
			"session_id", sessionID, "question_id", questionID, "user_id", cb.From.ID, "error", err)
		sf.bot.SendMessage(cb.Message.Chat.ID, "❌ 연결 자료 설정을 불러오지 못했습니다. 다시 시도해 주세요.")
		return
	}
	mode = model.MaterialReviewNormal
	if preference != nil {
		mode = preference.ReviewMode
	}
	text := materialPreferenceMenuText("이 문제에 연결된 학습 자료", mode)
	keyboard := quizMaterialPreferenceKeyboard(sessionID, questionID, mode)
	if err := sf.bot.SendMessageWithKeyboard(cb.Message.Chat.ID, text, keyboard); err != nil {
		slog.ErrorContext(ctx, "Failed to show linked quiz material preference menu",
			"session_id", sessionID, "question_id", questionID, "user_id", cb.From.ID, "error", err)
	}
}

func quizMaterialPreferenceKeyboard(
	sessionID, questionID int,
	selected model.MaterialReviewMode,
) tgbotapi.InlineKeyboardMarkup {
	rows := make([][]tgbotapi.InlineKeyboardButton, 0, 2)
	for _, mode := range []model.MaterialReviewMode{model.MaterialReviewMaintenance, model.MaterialReviewExcluded} {
		label := "유지 복습"
		if mode == model.MaterialReviewExcluded {
			label = "학습에서 제외"
		}
		if mode == selected {
			label = "✓ " + label
		}
		rows = append(rows, tgbotapi.NewInlineKeyboardRow(tgbotapi.NewInlineKeyboardButtonData(
			label, fmt.Sprintf(config.FormatQuestionPolicySet, sessionID, questionID, mode))))
	}
	return tgbotapi.NewInlineKeyboardMarkup(rows...)
}

// handleMaterialPreference only changes selection policy. Opening the menu,
// changing a policy, and returning to the card never mark material studied.
func (sf *StudyFlow) handleMaterialPreference(ctx context.Context, cb *tgbotapi.CallbackQuery, parts []string) {
	if len(cb.Data) > 64 || len(parts) < 4 || len(parts) > 5 || parts[0] != "study" {
		return
	}
	if parts[2] == "card" && len(parts) != 4 {
		return
	}
	sessionID, sessionOK := materialPreferenceInt(parts[1], false)
	materialID, materialOK := materialPreferenceInt(parts[3], false)
	if !sessionOK || !materialOK {
		return
	}
	var mode model.MaterialReviewMode
	if len(parts) == 5 {
		mode = model.MaterialReviewMode(parts[4])
		if !mode.Valid() {
			return
		}
	}
	if sf.bot.services == nil || sf.bot.services.MaterialPreference == nil ||
		sf.bot.services.StudyActiveSession == nil {
		return
	}
	state, err := sf.bot.services.StudyActiveSession.GetOwned(ctx, sessionID, cb.From.ID)
	if err != nil || state == nil || state.Session.ID != sessionID {
		slog.WarnContext(ctx, "Rejected study material preference callback",
			"session_id", sessionID, "user_id", cb.From.ID, "error", err)
		sf.bot.api.Request(tgbotapi.NewCallbackWithAlert(cb.ID, "❌ 이 Study Session의 설정을 변경할 수 없습니다."))
		return
	}
	var item *model.StudySessionMaterial
	for i := range state.Items {
		if state.Items[i].SessionMaterial.MaterialID == materialID {
			item = &state.Items[i]
			break
		}
	}
	if item == nil {
		sf.bot.api.Request(tgbotapi.NewCallbackWithAlert(cb.ID, "❌ 이 세션에 없는 학습 항목입니다."))
		return
	}
	if parts[2] == "card" {
		sf.showMaterial(ctx, cb.Message.Chat.ID, &cb.Message.MessageID, state, item.SessionMaterial.MaterialOrder)
		return
	}
	if len(parts) == 5 {
		if err := sf.bot.services.MaterialPreference.Set(ctx, cb.From.ID, materialID, mode); err != nil {
			sf.bot.materialPreferenceError(ctx, cb, "set", err)
			return
		}
		sf.bot.api.Request(tgbotapi.NewCallback(cb.ID, "✅ 새로 만드는 세션부터 적용됩니다."))
	} else {
		preference, err := sf.bot.services.MaterialPreference.Get(ctx, cb.From.ID, materialID)
		if err != nil {
			sf.bot.materialPreferenceError(ctx, cb, "get", err)
			return
		}
		mode = model.MaterialReviewNormal
		if preference != nil {
			mode = preference.ReviewMode
		}
	}
	text := materialPreferenceMenuText(item.Material.Title, mode)
	keyboard := materialPreferenceKeyboard(sessionID, materialID, mode)
	sf.bot.EditMessage(cb.Message.Chat.ID, cb.Message.MessageID, text, &keyboard)
}

func materialPreferenceMenuText(subject string, mode model.MaterialReviewMode) string {
	return fmt.Sprintf(
		"⚙️ <b>학습 설정</b>\n\n<b>%s</b>\n현재: %s\n\n유지 복습은 30일 간격으로 시작합니다. 정답이면 60→120→최대 180일, 오답이면 일반 학습으로 돌아갑니다.\n학습에서 제외: 새 학습·연결 퀴즈에서 제외합니다.\n\n%s",
		escapeHTML(subject),
		materialReviewModeLabel(mode),
		materialPreferenceNotice,
	)
}

func materialPreferenceKeyboard(
	sessionID, materialID int,
	selected model.MaterialReviewMode,
) tgbotapi.InlineKeyboardMarkup {
	rows := make([][]tgbotapi.InlineKeyboardButton, 0, 4)
	for _, mode := range []model.MaterialReviewMode{model.MaterialReviewNormal, model.MaterialReviewMaintenance, model.MaterialReviewExcluded} {
		label := materialReviewModeLabel(mode)
		if mode == selected {
			label = "✓ " + label
		}
		rows = append(rows, tgbotapi.NewInlineKeyboardRow(tgbotapi.NewInlineKeyboardButtonData(
			label, fmt.Sprintf(config.FormatStudyPolicySet, sessionID, materialID, mode))))
	}
	rows = append(rows, tgbotapi.NewInlineKeyboardRow(tgbotapi.NewInlineKeyboardButtonData(
		"← 학습 카드로", fmt.Sprintf(config.FormatStudyCard, sessionID, materialID))))
	return tgbotapi.NewInlineKeyboardMarkup(rows...)
}

func materialReviewModeLabel(mode model.MaterialReviewMode) string {
	switch mode {
	case model.MaterialReviewMaintenance:
		return "이미 알아요 · 드물게 복습"
	case model.MaterialReviewExcluded:
		return "학습에서 제외"
	default:
		return "일반 학습"
	}
}

func (b *Bot) settingsKeyboard(user *model.User) tgbotapi.InlineKeyboardMarkup {
	keyboard := buildSettingsKeyboard(user)
	if b.services != nil && b.services.MaterialPreference != nil {
		row := tgbotapi.NewInlineKeyboardRow(tgbotapi.NewInlineKeyboardButtonData(
			"📚 이미 아는 항목 · 제외 목록", fmt.Sprintf(config.FormatMaterialPreferences, 0)))
		last := len(keyboard.InlineKeyboard) - 1
		keyboard.InlineKeyboard = append(keyboard.InlineKeyboard[:last], row, keyboard.InlineKeyboard[last])
	}
	return keyboard
}

func (b *Bot) handleMaterialPreferencesCallback(ctx context.Context, cb *tgbotapi.CallbackQuery) {
	if len(cb.Data) > 64 || b.services == nil || b.services.MaterialPreference == nil {
		return
	}
	parts := strings.Split(cb.Data, ":")
	if len(parts) < 3 || parts[0] != "settings" {
		return
	}
	var page, materialID int
	var ok bool
	switch parts[1] {
	case "materials":
		if len(parts) != 3 {
			return
		}
		page, ok = materialPreferenceInt(parts[2], true)
	case "restore":
		if len(parts) != 4 {
			return
		}
		materialID, ok = materialPreferenceInt(parts[2], false)
		if !ok {
			return
		}
		page, ok = materialPreferenceInt(parts[3], true)
	default:
		return
	}
	if !ok || page > (int(^uint(0)>>1)-service.MaterialPreferencePageSize-1)/service.MaterialPreferencePageSize {
		return
	}
	if parts[1] == "restore" {
		// The service restores only this user's preference and treats an already
		// restored or absent preference as normal, so repeated callbacks are safe.
		if err := b.services.MaterialPreference.Set(
			ctx,
			cb.From.ID,
			materialID,
			model.MaterialReviewNormal,
		); err != nil {
			b.materialPreferenceError(ctx, cb, "restore", err)
			return
		}
		b.api.Request(tgbotapi.NewCallback(cb.ID, "✅ 일반 학습으로 복원했습니다. 새로 만드는 세션부터 적용됩니다."))
	}
	items, hasNext, err := b.services.MaterialPreference.List(ctx, cb.From.ID, page)
	if err != nil {
		b.materialPreferenceError(ctx, cb, "list", err)
		return
	}
	text, keyboard := materialPreferencesView(items, page, hasNext)
	b.EditMessage(cb.Message.Chat.ID, cb.Message.MessageID, text, &keyboard)
}

func materialPreferencesView(
	items []model.MaterialPreference,
	page int,
	hasNext bool,
) (string, tgbotapi.InlineKeyboardMarkup) {
	var text strings.Builder
	fmt.Fprintf(&text, "📚 <b>이미 아는 항목 · 제외 목록</b>\n\n%s\n\n", materialPreferenceNotice)
	rows := make([][]tgbotapi.InlineKeyboardButton, 0, len(items)+2)
	if len(items) == 0 {
		text.WriteString("이 페이지에 설정한 항목이 없습니다.\n")
	}
	for i, preference := range items {
		number := page*service.MaterialPreferencePageSize + i + 1
		fmt.Fprintf(
			&text,
			"%d. <b>%s</b> — %s\n",
			number,
			escapeHTML(preference.MaterialTitle),
			materialReviewModeLabel(preference.ReviewMode),
		)
		rows = append(rows, tgbotapi.NewInlineKeyboardRow(tgbotapi.NewInlineKeyboardButtonData(
			fmt.Sprintf(
				"↩️ %d번 일반 학습으로 복원",
				number,
			),
			fmt.Sprintf(config.FormatMaterialRestore, preference.MaterialID, page),
		)))
	}
	fmt.Fprintf(&text, "\n%d페이지", page+1)
	navigation := make([]tgbotapi.InlineKeyboardButton, 0, 2)
	if page > 0 {
		navigation = append(
			navigation,
			tgbotapi.NewInlineKeyboardButtonData("← 이전", fmt.Sprintf(config.FormatMaterialPreferences, page-1)),
		)
	}
	if hasNext {
		navigation = append(
			navigation,
			tgbotapi.NewInlineKeyboardButtonData("다음 →", fmt.Sprintf(config.FormatMaterialPreferences, page+1)),
		)
	}
	if len(navigation) > 0 {
		rows = append(rows, navigation)
	}
	rows = append(
		rows,
		tgbotapi.NewInlineKeyboardRow(tgbotapi.NewInlineKeyboardButtonData("⬅️ 설정 목록으로", config.ActionSettingsView)),
	)
	return text.String(), tgbotapi.NewInlineKeyboardMarkup(rows...)
}

func materialPreferenceInt(raw string, allowZero bool) (int, bool) {
	value, err := strconv.Atoi(raw)
	return value, err == nil && value >= 0 && (allowZero || value > 0) && strconv.Itoa(value) == raw
}

func (b *Bot) materialPreferenceError(ctx context.Context, cb *tgbotapi.CallbackQuery, action string, err error) {
	slog.ErrorContext(ctx, "Material preference request failed", "user_id", cb.From.ID, "action", action, "error", err)
	b.api.Request(tgbotapi.NewCallbackWithAlert(cb.ID, "❌ 학습 설정을 처리하지 못했습니다. 다시 시도해 주세요."))
}
