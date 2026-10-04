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
	var mode model.MaterialReviewMode
	if len(parts) == 5 {
		mode = model.MaterialReviewMode(parts[4])
		if mode != model.MaterialReviewMaintenance && mode != model.MaterialReviewExcluded {
			return
		}
		if err := sf.materialPreference.Set(
			ctx,
			cb.From.ID,
			*item.Question.MaterialID,
			mode,
		); err != nil {
			slog.ErrorContext(
				ctx,
				"Failed to set linked quiz material preference",
				"session_id",
				sessionID,
				"question_id",
				questionID,
				"user_id",
				cb.From.ID,
				"error",
				err,
			)
			sf.telegram.SendMessage(
				cb.Message.Chat.ID,
				botMessagesByLocale[botDefaultLocale].materialPreferenceSaveFailed,
			)
			return
		}
		sf.telegram.EditMessage(
			cb.Message.Chat.ID,
			cb.Message.MessageID,
			fmt.Sprintf(
				botMessagesByLocale[botDefaultLocale].materialPreferenceSavedFormat,
				materialReviewModeLabel(mode),
				botMessagesByLocale[botDefaultLocale].materialPreferenceNotice,
			),
			nil,
		)
		return
	}
	preference, err := sf.materialPreference.Get(
		ctx,
		cb.From.ID,
		*item.Question.MaterialID,
	)
	if err != nil {
		slog.ErrorContext(
			ctx,
			"Failed to get linked quiz material preference",
			"session_id",
			sessionID,
			"question_id",
			questionID,
			"user_id",
			cb.From.ID,
			"error",
			err,
		)
		sf.telegram.SendMessage(
			cb.Message.Chat.ID,
			botMessagesByLocale[botDefaultLocale].materialPreferenceLoadFailed,
		)
		return
	}
	mode = model.MaterialReviewNormal
	if preference != nil {
		mode = preference.ReviewMode
	}
	text := materialPreferenceMenuText(
		botMessagesByLocale[botDefaultLocale].linkedQuizMaterialTitle,
		mode,
	)
	keyboard := quizMaterialPreferenceKeyboard(
		sessionID,
		questionID,
		mode,
	)
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

func quizMaterialPreferenceKeyboard(
	sessionID,
	questionID int,
	selected model.MaterialReviewMode,
) tgbotapi.InlineKeyboardMarkup {
	rows := make(
		[][]tgbotapi.InlineKeyboardButton,
		0,
		2,
	)
	for _, mode := range []model.MaterialReviewMode{model.MaterialReviewMaintenance, model.MaterialReviewExcluded} {
		label := botMessagesByLocale[botDefaultLocale].materialMaintenanceButtonLabel
		if mode == model.MaterialReviewExcluded {
			label = botMessagesByLocale[botDefaultLocale].materialExcludedLabel
		}
		if mode == selected {
			label = "✓ " + label
		}
		rows = append(
			rows,
			tgbotapi.NewInlineKeyboardRow(tgbotapi.NewInlineKeyboardButtonData(
				label,
				fmt.Sprintf(
					formatQuestionPolicySet,
					sessionID,
					questionID,
					mode,
				),
			)),
		)
	}
	return tgbotapi.NewInlineKeyboardMarkup(rows...)
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
	var mode model.MaterialReviewMode
	if len(parts) == 5 {
		mode = model.MaterialReviewMode(parts[4])
		if !mode.Valid() {
			return
		}
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
	var item *model.StudySessionMaterial
	for i := range state.Items {
		if state.Items[i].SessionMaterial.MaterialID == materialID {
			item = &state.Items[i]
			break
		}
	}
	if item == nil {
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
	if len(parts) == 5 {
		if err := sf.materialPreference.Set(
			ctx,
			cb.From.ID,
			materialID,
			mode,
		); err != nil {
			materialPreferenceError(
				ctx,
				sf.telegram,
				cb,
				"set",
				err,
			)
			return
		}
		sf.telegram.AnswerCallback(
			cb.ID,
			botMessagesByLocale[botDefaultLocale].materialPreferenceAppliedNotice,
		)
	} else {
		preference, err := sf.materialPreference.Get(
			ctx,
			cb.From.ID,
			materialID,
		)
		if err != nil {
			materialPreferenceError(
				ctx,
				sf.telegram,
				cb,
				"get",
				err,
			)
			return
		}
		mode = model.MaterialReviewNormal
		if preference != nil {
			mode = preference.ReviewMode
		}
	}
	text := materialPreferenceMenuText(
		item.Material.Title,
		mode,
	)
	keyboard := materialPreferenceKeyboard(
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

func materialPreferenceMenuText(
	subject string,
	mode model.MaterialReviewMode,
) string {
	return fmt.Sprintf(
		botMessagesByLocale[botDefaultLocale].materialPreferenceMenuFormat,
		escapeHTML(subject),
		materialReviewModeLabel(mode),
		botMessagesByLocale[botDefaultLocale].materialPreferenceNotice,
	)
}

func materialPreferenceKeyboard(
	sessionID,
	materialID int,
	selected model.MaterialReviewMode,
) tgbotapi.InlineKeyboardMarkup {
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
				fmt.Sprintf(
					formatStudyPolicySet,
					sessionID,
					materialID,
					mode,
				),
			)),
		)
	}
	rows = append(
		rows,
		tgbotapi.NewInlineKeyboardRow(tgbotapi.NewInlineKeyboardButtonData(
			botMessagesByLocale[botDefaultLocale].materialBackToCardButton,
			fmt.Sprintf(
				formatStudyCard,
				sessionID,
				materialID,
			),
		)),
	)
	return tgbotapi.NewInlineKeyboardMarkup(rows...)
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
	telegram *telegramClient,
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
