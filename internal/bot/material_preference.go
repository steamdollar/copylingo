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

// materialPreferences reads and changes one user's review mode for a linked
// material from the Quiz and Study screens.
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

// materialPreferenceList pages one user's changed material preferences and
// restores one to normal from the Settings screen.
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

// Quiz callbacks carry a question ID, never a trusted material ID. Resolve it
// from the owner's session before showing or changing the linked preference.
// The menu is a separate message so the question keeps its answer buttons;
// its close button deletes only that menu message.
func (sf *SessionFlow) handleQuizMaterialPreference(
	ctx context.Context,
	cb *tgbotapi.CallbackQuery,
	parts []string,
) {
	if cb == nil || cb.From == nil || cb.Message == nil || cb.Message.Chat == nil || len(cb.Data) > 64 ||
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
		// Closing reads no session: it removes only the menu message the
		// button sits on, so it still works after the Quiz has finished.
		sf.telegram.DeleteMessage(
			cb.Message.Chat.ID,
			cb.Message.MessageID,
		)
		return
	}
	mode, modeOK := materialPreferenceMode(parts)
	if !modeOK {
		return
	}
	state, err := sf.session.QuizProgress(
		ctx,
		sessionID,
	)
	if err != nil || state == nil || state.Session.ID != sessionID || state.Session.UserID != cb.From.ID ||
		state.Session.Mode != model.SessionModeQuiz {
		slog.WarnContext(
			ctx,
			"Rejected quiz material preference callback",
			"session_id",
			sessionID,
			"question_id",
			questionID,
			"user_id",
			cb.From.ID,
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
		sf.telegram,
		sf.materialPreference,
		cb,
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
		// A choice re-renders the menu message in place, as on the Study card.
		sf.telegram.EditMessage(
			cb.Message.Chat.ID,
			cb.Message.MessageID,
			text,
			&keyboard,
		)
		return
	}
	if err := sf.telegram.SendMessageWithKeyboard(
		cb.Message.Chat.ID,
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
			cb.From.ID,
			"error",
			err,
		)
	}
}

// handleMaterialPreference only changes selection policy. Opening the menu,
// changing a policy, and returning to the card never mark material studied.
func (sf *StudyFlow) handleMaterialPreference(
	ctx context.Context,
	cb *tgbotapi.CallbackQuery,
	parts []string,
) {
	if len(cb.Data) > 64 || len(parts) < 4 || len(parts) > 5 || parts[0] != callbackRootStudy {
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
	state, err := sf.session.StudyProgress(
		ctx,
		sessionID,
		cb.From.ID,
	)
	if err != nil || state == nil || state.Session.ID != sessionID {
		slog.WarnContext(
			ctx,
			"Rejected study material preference callback",
			"session_id",
			sessionID,
			"user_id",
			cb.From.ID,
			"error",
			err,
		)
		sf.telegram.AnswerCallbackAlert(
			cb.ID,
			botMessagesByLocale[botDefaultLocale].materialQuizSessionRejected,
		)
		return
	}
	item, ok := state.ItemByMaterialID(materialID)
	if !ok {
		sf.telegram.AnswerCallbackAlert(
			cb.ID,
			botMessagesByLocale[botDefaultLocale].materialNotInSession,
		)
		return
	}
	if parts[2] == callbackActionCard {
		sf.showMaterial(
			ctx,
			cb.Message.Chat.ID,
			&cb.Message.MessageID,
			state,
			item.SessionMaterial.MaterialOrder,
		)
		return
	}
	mode, ok = applyMaterialPreference(
		ctx,
		sf.telegram,
		sf.materialPreference,
		cb,
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
	sf.telegram.EditMessage(
		cb.Message.Chat.ID,
		cb.Message.MessageID,
		text,
		&keyboard,
	)
}

// materialPreferenceMode returns the review mode a menu callback chose, or ""
// when the callback only opens the menu. It reports false for unknown modes.
func materialPreferenceMode(parts []string) (model.MaterialReviewMode, bool) {
	if len(parts) != 5 {
		return "", true
	}
	mode := model.MaterialReviewMode(parts[4])
	return mode, mode.Valid()
}

// applyMaterialPreference stores the chosen mode, or reads the current one when
// chosen is "" (menu just opened). On failure it alerts the user and reports false.
func applyMaterialPreference(
	ctx context.Context,
	telegram *TelegramClient,
	preferences materialPreferences,
	cb *tgbotapi.CallbackQuery,
	materialID int,
	chosen model.MaterialReviewMode,
) (model.MaterialReviewMode, bool) {
	if chosen != "" {
		if err := preferences.Set(
			ctx,
			cb.From.ID,
			materialID,
			chosen,
		); err != nil {
			materialPreferenceError(
				ctx,
				telegram,
				cb,
				"set",
				err,
			)
			return "", false
		}
		telegram.AnswerCallback(
			cb.ID,
			botMessagesByLocale[botDefaultLocale].materialPreferenceAppliedNotice,
		)
		return chosen, true
	}
	preference, err := preferences.Get(
		ctx,
		cb.From.ID,
		materialID,
	)
	if err != nil {
		materialPreferenceError(
			ctx,
			telegram,
			cb,
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

// quizMaterialPreferenceMenu is the shared menu for a Quiz question's linked
// material. Its close button deletes the separate menu message.
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

// studyMaterialPreferenceMenu is the shared menu for a Study card. Its back
// button re-renders the same card in place.
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

// materialPreferenceMenu renders the menu shared by the Quiz and Study screens:
// the subject and current mode, one button per review mode with the current
// one checked, and a final navigation button chosen by the caller.
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

func (sf *SettingsFlow) handleMaterialPreferencesCallback(
	ctx context.Context,
	cb *tgbotapi.CallbackQuery,
) {
	if len(cb.Data) > 64 {
		return
	}
	parts := strings.Split(
		cb.Data,
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
		// The service restores only this user's preference and treats an already
		// restored or absent preference as normal, so repeated callbacks are safe.
		if err := sf.materialPreference.Set(
			ctx,
			cb.From.ID,
			materialID,
			model.MaterialReviewNormal,
		); err != nil {
			materialPreferenceError(
				ctx,
				sf.telegram,
				cb,
				"restore",
				err,
			)
			return
		}
		sf.telegram.AnswerCallback(
			cb.ID,
			botMessagesByLocale[botDefaultLocale].materialPreferenceRestoredNotice,
		)
	}
	items, hasNext, err := sf.materialPreference.List(
		ctx,
		cb.From.ID,
		page,
	)
	if err != nil {
		materialPreferenceError(
			ctx,
			sf.telegram,
			cb,
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
	sf.telegram.EditMessage(
		cb.Message.Chat.ID,
		cb.Message.MessageID,
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

// materialPreferenceError logs a failed preference read/write and tells the
// user through a callback alert, shared by the Quiz, Study and Settings screens.
func materialPreferenceError(
	ctx context.Context,
	telegram *TelegramClient,
	cb *tgbotapi.CallbackQuery,
	action string,
	err error,
) {
	slog.ErrorContext(
		ctx,
		"Material preference request failed",
		"user_id",
		cb.From.ID,
		"action",
		action,
		"error",
		err,
	)
	telegram.AnswerCallbackAlert(
		cb.ID,
		botMessagesByLocale[botDefaultLocale].materialPreferenceFailed,
	)
}
