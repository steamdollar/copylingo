package bot

import (
	"context"
	"fmt"
	"log/slog"
	"strconv"
	"strings"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"

	"github.com/lsj/copylingo/internal/callback"
	"github.com/lsj/copylingo/internal/model"
	"github.com/lsj/copylingo/internal/service"
)

type materialPreferences interface {
	Get(
		ctx context.Context,
		userID int64,
		materialID int,
	) (*model.MaterialPreference, error)
	Set(
		ctx context.Context,
		userID int64,
		materialID int,
		mode model.MaterialReviewMode,
	) error
}

type materialPreferenceList interface {
	List(
		ctx context.Context,
		userID int64,
		page int,
	) ([]model.MaterialPreference, bool, error)
	Set(
		ctx context.Context,
		userID int64,
		materialID int,
		mode model.MaterialReviewMode,
	) error
}

// Quiz callbacks carry a question ID, never a trusted material ID: resolve it from the owner's session.
// The menu is a separate message so the question keeps its answer buttons.
func (flow *SessionFlow) handleQuizMaterialPreference(
	ctx context.Context,
	callbackQuery *tgbotapi.CallbackQuery,
	parts []string,
) {
	if callbackQuery == nil || callbackQuery.From == nil ||
		callbackQuery.Message == nil || callbackQuery.Message.Chat == nil || len(callbackQuery.Data) > 64 ||
		len(parts) < 4 || len(parts) > 5 || parts[0] != callback.QuestionRoot ||
		(parts[2] != callback.QuestionActionPolicy &&
			(parts[2] != callbackActionExclude || len(parts) != 4)) {
		return
	}
	sessionID, sessionOK := materialPreferenceInt(
		parts[1],
		false,
	)
	questionID, questionOK := materialPreferenceInt(
		parts[3],
		false,
	)
	if !sessionOK || !questionOK {
		return
	}
	if len(parts) == 5 && parts[4] == callbackActionClose {
		// Closing reads no session, so it still works after the Quiz has finished.
		flow.telegram.DeleteMessage(
			callbackQuery.Message.Chat.ID,
			callbackQuery.Message.MessageID,
		)
		return
	}
	mode, modeOK := materialPreferenceMode(parts)
	if !modeOK {
		return
	}
	state, err := flow.session.QuizProgress(
		ctx,
		sessionID,
	)
	if err != nil || state == nil || state.Session.ID != sessionID || state.Session.UserID != callbackQuery.From.ID ||
		state.Session.Mode != model.SessionModeQuiz {
		slog.WarnContext(
			ctx,
			"Rejected quiz material preference callback",
			"session_id",
			sessionID,
			"question_id",
			questionID,
			"user_id",
			callbackQuery.From.ID,
			"error",
			err,
		)
		return
	}
	item, exists := state.ItemByQuestionID(questionID)
	if !exists || item.Question.MaterialID == nil {
		return
	}
	mode, ok := applyMaterialPreference(
		ctx,
		flow.telegram,
		flow.materialPreference,
		callbackQuery,
		*item.Question.MaterialID,
		mode,
	)
	if !ok {
		return
	}
	text, keyboard := quizMaterialPreferenceMenu(
		sessionID,
		questionID,
		mode,
	)
	if len(parts) == 5 {
		flow.telegram.EditMessage(
			callbackQuery.Message.Chat.ID,
			callbackQuery.Message.MessageID,
			text,
			&keyboard,
		)
		return
	}
	if err := flow.telegram.SendMessageWithKeyboard(
		callbackQuery.Message.Chat.ID,
		text,
		keyboard,
	); err != nil {
		slog.ErrorContext(
			ctx,
			"Failed to show linked quiz material preference menu",
			"session_id",
			sessionID,
			"question_id",
			questionID,
			"user_id",
			callbackQuery.From.ID,
			"error",
			err,
		)
	}
}

// Never marks the material studied.
func (flow *StudyFlow) handleMaterialPreference(
	ctx context.Context,
	callbackQuery *tgbotapi.CallbackQuery,
	parts []string,
) {
	if len(callbackQuery.Data) > 64 || len(parts) < 4 || len(parts) > 5 || parts[0] != callbackRootStudy {
		return
	}
	if parts[2] == callbackActionCard && len(parts) != 4 {
		return
	}
	sessionID, sessionOK := materialPreferenceInt(
		parts[1],
		false,
	)
	materialID, materialOK := materialPreferenceInt(
		parts[3],
		false,
	)
	if !sessionOK || !materialOK {
		return
	}
	mode, modeOK := materialPreferenceMode(parts)
	if !modeOK {
		return
	}
	state, err := flow.session.StudyProgress(
		ctx,
		sessionID,
		callbackQuery.From.ID,
	)
	if err != nil || state == nil || state.Session.ID != sessionID {
		slog.WarnContext(
			ctx,
			"Rejected study material preference callback",
			"session_id",
			sessionID,
			"user_id",
			callbackQuery.From.ID,
			"error",
			err,
		)
		flow.telegram.AnswerCallbackAlert(
			callbackQuery.ID,
			botMessagesByLocale[botDefaultLocale].materialQuizSessionRejected,
		)
		return
	}
	item, ok := state.ItemByMaterialID(materialID)
	if !ok {
		flow.telegram.AnswerCallbackAlert(
			callbackQuery.ID,
			botMessagesByLocale[botDefaultLocale].materialNotInSession,
		)
		return
	}
	if parts[2] == callbackActionCard {
		flow.showMaterial(
			ctx,
			callbackQuery.Message.Chat.ID,
			&callbackQuery.Message.MessageID,
			state,
			item.SessionMaterial.MaterialOrder,
		)
		return
	}
	mode, ok = applyMaterialPreference(
		ctx,
		flow.telegram,
		flow.materialPreference,
		callbackQuery,
		materialID,
		mode,
	)
	if !ok {
		return
	}
	text, keyboard := studyMaterialPreferenceMenu(
		item.Material.Title,
		sessionID,
		materialID,
		mode,
	)
	flow.telegram.EditMessage(
		callbackQuery.Message.Chat.ID,
		callbackQuery.Message.MessageID,
		text,
		&keyboard,
	)
}

// Returns "" when the callback only opens the menu, false for unknown modes.
func materialPreferenceMode(parts []string) (model.MaterialReviewMode, bool) {
	if len(parts) != 5 {
		return "", true
	}
	mode := model.MaterialReviewMode(parts[4])
	return mode, mode.Valid()
}

// chosen "" (menu just opened) reads the current mode instead of storing one.
func applyMaterialPreference(
	ctx context.Context,
	telegram *TelegramClient,
	preferences materialPreferences,
	callbackQuery *tgbotapi.CallbackQuery,
	materialID int,
	chosen model.MaterialReviewMode,
) (model.MaterialReviewMode, bool) {
	if chosen != "" {
		if err := preferences.Set(
			ctx,
			callbackQuery.From.ID,
			materialID,
			chosen,
		); err != nil {
			materialPreferenceError(
				ctx,
				telegram,
				callbackQuery,
				"set",
				err,
			)
			return "", false
		}
		telegram.AnswerCallback(
			callbackQuery.ID,
			botMessagesByLocale[botDefaultLocale].materialPreferenceAppliedNotice,
		)
		return chosen, true
	}
	preference, err := preferences.Get(
		ctx,
		callbackQuery.From.ID,
		materialID,
	)
	if err != nil {
		materialPreferenceError(
			ctx,
			telegram,
			callbackQuery,
			"get",
			err,
		)
		return "", false
	}
	if preference == nil {
		return model.MaterialReviewNormal, true
	}
	return preference.ReviewMode, true
}

func quizMaterialPreferenceMenu(
	sessionID,
	questionID int,
	selected model.MaterialReviewMode,
) (string, tgbotapi.InlineKeyboardMarkup) {
	callbackData := func(action string) string {
		return fmt.Sprintf(
			formatQuestionPolicySet,
			sessionID,
			questionID,
			action,
		)
	}
	return materialPreferenceMenu(
		botMessagesByLocale[botDefaultLocale].linkedQuizMaterialTitle,
		selected,
		func(mode model.MaterialReviewMode) string { return callbackData(string(mode)) },
		tgbotapi.NewInlineKeyboardButtonData(
			botMessagesByLocale[botDefaultLocale].materialPreferenceCloseButton,
			callbackData(callbackActionClose),
		),
	)
}

func studyMaterialPreferenceMenu(
	title string,
	sessionID,
	materialID int,
	selected model.MaterialReviewMode,
) (string, tgbotapi.InlineKeyboardMarkup) {
	return materialPreferenceMenu(
		title,
		selected,
		func(mode model.MaterialReviewMode) string {
			return fmt.Sprintf(
				formatStudyPolicySet,
				sessionID,
				materialID,
				mode,
			)
		},
		tgbotapi.NewInlineKeyboardButtonData(
			botMessagesByLocale[botDefaultLocale].materialBackToCardButton,
			fmt.Sprintf(
				formatStudyCard,
				sessionID,
				materialID,
			),
		),
	)
}

func materialPreferenceMenu(
	subject string,
	selected model.MaterialReviewMode,
	choiceCallbackData func(model.MaterialReviewMode) string,
	navigation tgbotapi.InlineKeyboardButton,
) (string, tgbotapi.InlineKeyboardMarkup) {
	rows := make(
		[][]tgbotapi.InlineKeyboardButton,
		0,
		4,
	)
	for _, mode := range []model.MaterialReviewMode{model.MaterialReviewNormal, model.MaterialReviewMaintenance, model.MaterialReviewExcluded} {
		label := materialReviewModeLabel(mode)
		if mode == selected {
			label = "✓ " + label
		}
		rows = append(
			rows,
			tgbotapi.NewInlineKeyboardRow(tgbotapi.NewInlineKeyboardButtonData(
				label,
				choiceCallbackData(mode),
			)),
		)
	}
	rows = append(
		rows,
		tgbotapi.NewInlineKeyboardRow(navigation),
	)
	text := fmt.Sprintf(
		botMessagesByLocale[botDefaultLocale].materialPreferenceMenuFormat,
		escapeHTML(subject),
		materialReviewModeLabel(selected),
		botMessagesByLocale[botDefaultLocale].materialPreferenceNotice,
	)
	return text, tgbotapi.NewInlineKeyboardMarkup(rows...)
}

func materialReviewModeLabel(mode model.MaterialReviewMode) string {
	switch mode {
	case model.MaterialReviewMaintenance:
		return botMessagesByLocale[botDefaultLocale].materialAlreadyKnowLabel
	case model.MaterialReviewExcluded:
		return botMessagesByLocale[botDefaultLocale].materialExcludedLabel
	default:
		return botMessagesByLocale[botDefaultLocale].materialNormalLabel
	}
}

func (flow *SettingsFlow) handleMaterialPreferencesCallback(
	ctx context.Context,
	callbackQuery *tgbotapi.CallbackQuery,
) {
	if len(callbackQuery.Data) > 64 {
		return
	}
	parts := strings.Split(
		callbackQuery.Data,
		":",
	)
	if len(parts) < 3 || parts[0] != callbackRootSettings {
		return
	}
	var page, materialID int
	var ok bool
	switch parts[1] {
	case callbackActionMaterials:
		if len(parts) != 3 {
			return
		}
		page, ok = materialPreferenceInt(
			parts[2],
			true,
		)
	case callbackActionRestore:
		if len(parts) != 4 {
			return
		}
		materialID, ok = materialPreferenceInt(
			parts[2],
			false,
		)
		if !ok {
			return
		}
		page, ok = materialPreferenceInt(
			parts[3],
			true,
		)
	default:
		return
	}
	if !ok || page > (int(^uint(0)>>1)-service.MaterialPreferencePageSize-1)/service.MaterialPreferencePageSize {
		return
	}
	if parts[1] == callbackActionRestore {
		// The service treats an already restored or absent preference as normal, so repeated callbacks are safe.
		if err := flow.materialPreference.Set(
			ctx,
			callbackQuery.From.ID,
			materialID,
			model.MaterialReviewNormal,
		); err != nil {
			materialPreferenceError(
				ctx,
				flow.telegram,
				callbackQuery,
				"restore",
				err,
			)
			return
		}
		flow.telegram.AnswerCallback(
			callbackQuery.ID,
			botMessagesByLocale[botDefaultLocale].materialPreferenceRestoredNotice,
		)
	}
	items, hasNext, err := flow.materialPreference.List(
		ctx,
		callbackQuery.From.ID,
		page,
	)
	if err != nil {
		materialPreferenceError(
			ctx,
			flow.telegram,
			callbackQuery,
			"list",
			err,
		)
		return
	}
	text, keyboard := materialPreferencesView(
		items,
		page,
		hasNext,
	)
	flow.telegram.EditMessage(
		callbackQuery.Message.Chat.ID,
		callbackQuery.Message.MessageID,
		text,
		&keyboard,
	)
}

func materialPreferencesView(
	items []model.MaterialPreference,
	page int,
	hasNext bool,
) (string, tgbotapi.InlineKeyboardMarkup) {
	var text strings.Builder
	fmt.Fprintf(
		&text,
		botMessagesByLocale[botDefaultLocale].materialPreferencesHeadingFormat,
		botMessagesByLocale[botDefaultLocale].materialPreferenceNotice,
	)
	rows := make(
		[][]tgbotapi.InlineKeyboardButton,
		0,
		len(items)+2,
	)
	if len(items) == 0 {
		text.WriteString(botMessagesByLocale[botDefaultLocale].materialPreferencesEmpty)
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
		rows = append(
			rows,
			tgbotapi.NewInlineKeyboardRow(tgbotapi.NewInlineKeyboardButtonData(
				fmt.Sprintf(
					botMessagesByLocale[botDefaultLocale].materialRestoreButtonFormat,
					number,
				),
				fmt.Sprintf(
					formatMaterialRestore,
					preference.MaterialID,
					page,
				),
			)),
		)
	}
	fmt.Fprintf(
		&text,
		botMessagesByLocale[botDefaultLocale].materialPageFormat,
		page+1,
	)
	navigation := make(
		[]tgbotapi.InlineKeyboardButton,
		0,
		2,
	)
	if page > 0 {
		navigation = append(
			navigation,
			tgbotapi.NewInlineKeyboardButtonData(
				botMessagesByLocale[botDefaultLocale].previousButton,
				fmt.Sprintf(
					formatMaterialPreferences,
					page-1,
				),
			),
		)
	}
	if hasNext {
		navigation = append(
			navigation,
			tgbotapi.NewInlineKeyboardButtonData(
				botMessagesByLocale[botDefaultLocale].nextButton,
				fmt.Sprintf(
					formatMaterialPreferences,
					page+1,
				),
			),
		)
	}
	if len(navigation) > 0 {
		rows = append(
			rows,
			navigation,
		)
	}
	rows = append(
		rows,
		tgbotapi.NewInlineKeyboardRow(tgbotapi.NewInlineKeyboardButtonData(
			botMessagesByLocale[botDefaultLocale].settingsBackButton,
			callbackSettingsView,
		)),
	)
	return text.String(), tgbotapi.NewInlineKeyboardMarkup(rows...)
}

func materialPreferenceInt(
	raw string,
	allowZero bool,
) (int, bool) {
	value, err := strconv.Atoi(raw)
	return value, err == nil && value >= 0 && (allowZero || value > 0) && strconv.Itoa(value) == raw
}

func materialPreferenceError(
	ctx context.Context,
	telegram *TelegramClient,
	callbackQuery *tgbotapi.CallbackQuery,
	action string,
	err error,
) {
	slog.ErrorContext(
		ctx,
		"Material preference request failed",
		"user_id",
		callbackQuery.From.ID,
		"action",
		action,
		"error",
		err,
	)
	telegram.AnswerCallbackAlert(
		callbackQuery.ID,
		botMessagesByLocale[botDefaultLocale].materialPreferenceFailed,
	)
}
