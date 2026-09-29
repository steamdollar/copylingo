package bot

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"html"
	"log/slog"
	"strconv"
	"strings"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"

	"github.com/lsj/copylingo/internal/callback"
	"github.com/lsj/copylingo/internal/model"
)

// StudyFlow handles material-based study sessions.
type StudyFlow struct {
	bot      *Bot
	telegram *telegramClient
}

func NewStudyFlow(bot *Bot) *StudyFlow {
	return &StudyFlow{
		bot:      bot,
		telegram: bot.telegram,
	}
}

func (sf *StudyFlow) PushSession(
	ctx context.Context,
	chatID int64,
	sessionID int,
) error {
	message := botMessagesByLocale[botDefaultLocale]
	keyboard := tgbotapi.NewInlineKeyboardMarkup(
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData(
				message.startButton,
				fmt.Sprintf(
					formatStudyStart,
					sessionID,
				),
			),
		),
	)
	return sf.telegram.SendMessageWithKeyboard(
		chatID,
		message.sessionPush,
		keyboard,
	)
}

func (sf *StudyFlow) HandleCallback(
	ctx context.Context,
	cb *tgbotapi.CallbackQuery,
) {
	if cb == nil || cb.From == nil || cb.Message == nil || cb.Message.Chat == nil {
		return
	}

	// e.g. "study:42:next:3"
	parts := strings.Split(
		cb.Data,
		":",
	)
	if len(parts) < 3 {
		return
	}

	sessionID, err := strconv.Atoi(parts[1])
	if err != nil {
		slog.WarnContext(
			ctx,
			"Invalid study session ID in callback",
			"event",
			"telegram.study.invalid_session_id",
			"callback_type",
			parts[0],
		)
		return
	}

	switch parts[2] {
	case callback.QuestionActionPolicy, callbackActionCard:
		sf.handleMaterialPreference(
			ctx,
			cb,
			parts,
		)
	case callbackActionStart:
		sf.startSession(
			ctx,
			cb,
			sessionID,
		)
	case callbackActionAsk:
		if len(parts) < 4 {
			return
		}
		sf.handleAskLLMQuestion(
			ctx,
			cb,
			sessionID,
			parts[3],
		)
	case callback.QuestionActionNext:
		if len(parts) < 4 {
			return
		}
		currentOrder, err := strconv.Atoi(parts[3])
		if err != nil {
			return
		}
		sf.nextMaterial(
			ctx,
			cb,
			sessionID,
			currentOrder,
		)
	case callbackActionPrev:
		if len(parts) < 4 {
			return
		}
		currentOrder, err := strconv.Atoi(parts[3])
		if err != nil {
			return
		}
		sf.prevMaterial(
			ctx,
			cb,
			sessionID,
			currentOrder,
		)
	case callbackActionFinish:
		if len(parts) < 4 {
			return
		}
		currentOrder, err := strconv.Atoi(parts[3])
		if err != nil {
			return
		}
		sf.finishSession(
			ctx,
			cb,
			sessionID,
			currentOrder,
		)
	}
}

func (sf *StudyFlow) startSession(
	ctx context.Context,
	cb *tgbotapi.CallbackQuery,
	sessionID int,
) {
	messages := botMessagesByLocale[botDefaultLocale]
	state, err := sf.bot.services.StudyActiveSession.Start(
		ctx,
		sessionID,
		cb.From.ID,
	)
	if err != nil {
		slog.ErrorContext(
			ctx,
			"Failed to start study active session",
			"event",
			"telegram.study.active_start_failed",
			"session_id",
			sessionID,
			"error",
			err,
		)
		sf.telegram.EditMessage(
			cb.Message.Chat.ID,
			cb.Message.MessageID,
			messages.startFailed,
			mainMenuKeyboard(),
		)
		return
	}
	if state.Session.Status == model.SessionCompleted {
		sf.telegram.EditMessage(
			cb.Message.Chat.ID,
			cb.Message.MessageID,
			messages.alreadyCompleted,
			mainMenuKeyboard(),
		)
		return
	}

	nextOrder := nextUnstudiedMaterialOrder(state)
	sf.showMaterial(
		ctx,
		cb.Message.Chat.ID,
		&cb.Message.MessageID,
		state,
		nextOrder,
	)
}

func (sf *StudyFlow) nextMaterial(
	ctx context.Context,
	cb *tgbotapi.CallbackQuery,
	sessionID,
	currentOrder int,
) {
	messages := botMessagesByLocale[botDefaultLocale]
	// update study proceeding state
	state, err := sf.bot.services.StudyActiveSession.MarkStudied(
		ctx,
		sessionID,
		cb.From.ID,
		currentOrder,
	)
	if err != nil {
		slog.ErrorContext(
			ctx,
			"Failed to mark study material",
			"event",
			"telegram.study.material_mark_failed",
			"session_id",
			sessionID,
			"material_order",
			currentOrder,
			"error",
			err,
		)
		sf.telegram.SendMessage(
			cb.Message.Chat.ID,
			messages.saveProgressFailed,
		)
		return
	}

	sf.showMaterial(
		ctx,
		cb.Message.Chat.ID,
		&cb.Message.MessageID,
		state,
		currentOrder+1,
	)
}

// prevMaterial re-shows an already-seen card; it never mutates studied state.
func (sf *StudyFlow) prevMaterial(
	ctx context.Context,
	cb *tgbotapi.CallbackQuery,
	sessionID,
	currentOrder int,
) {
	messages := botMessagesByLocale[botDefaultLocale]
	state, err := sf.bot.services.StudyActiveSession.LoadOwnedStudySessionState(
		ctx,
		sessionID,
		cb.From.ID,
	)
	if err != nil {
		slog.ErrorContext(
			ctx,
			"Failed to load study session for prev material",
			"event",
			"telegram.study.material_prev_failed",
			"session_id",
			sessionID,
			"material_order",
			currentOrder,
			"error",
			err,
		)
		sf.telegram.SendMessage(
			cb.Message.Chat.ID,
			messages.loadProgressFailed,
		)
		return
	}

	sf.showMaterial(
		ctx,
		cb.Message.Chat.ID,
		&cb.Message.MessageID,
		state,
		currentOrder-1,
	)
}

func (sf *StudyFlow) finishSession(
	ctx context.Context,
	cb *tgbotapi.CallbackQuery,
	sessionID,
	currentOrder int,
) {
	messages := botMessagesByLocale[botDefaultLocale]
	if _, err := sf.bot.services.StudyActiveSession.MarkStudied(
		ctx,
		sessionID,
		cb.From.ID,
		currentOrder,
	); err != nil {
		slog.ErrorContext(
			ctx,
			"Failed to mark final study material",
			"event",
			"telegram.study.final_material_mark_failed",
			"session_id",
			sessionID,
			"material_order",
			currentOrder,
			"error",
			err,
		)
		sf.telegram.SendMessage(
			cb.Message.Chat.ID,
			messages.saveCompletionFailed,
		)
		return
	}
	if err := sf.bot.services.StudyActiveSession.Complete(
		ctx,
		sessionID,
		cb.From.ID,
	); err != nil {
		slog.ErrorContext(
			ctx,
			"Failed to complete study session",
			"event",
			"telegram.study.complete_failed",
			"session_id",
			sessionID,
			"error",
			err,
		)
		sf.telegram.SendMessage(
			cb.Message.Chat.ID,
			messages.completeFailed,
		)
		return
	}

	sf.telegram.EditMessage(
		cb.Message.Chat.ID,
		cb.Message.MessageID,
		messages.sessionCompleted,
		mainMenuKeyboard(),
	)
}

func (sf *StudyFlow) showMaterial(
	ctx context.Context,
	chatID int64,
	editMessageID *int,
	state *model.StudyActiveSessionState,
	materialOrder int,
) {
	messages := botMessagesByLocale[botDefaultLocale]
	items := state.Items
	if len(items) == 0 {
		sf.telegram.SendMessage(
			chatID,
			messages.noMaterials,
		)
		return
	}
	if materialOrder < 0 {
		materialOrder = 0
	}
	if materialOrder >= len(items) {
		if err := sf.bot.services.StudyActiveSession.Complete(
			ctx,
			state.Session.ID,
			state.Session.UserID,
		); err != nil {
			slog.ErrorContext(
				ctx,
				"Failed to auto-complete study session",
				"event",
				"telegram.study.auto_complete_failed",
				"session_id",
				state.Session.ID,
				"error",
				err,
			)
		}
		sf.telegram.SendMessage(
			chatID,
			messages.autoCompleted,
		)
		return
	}
	idx := studyMaterialIndexByOrder(
		items,
		materialOrder,
	)
	if idx == -1 {
		slog.WarnContext(
			ctx,
			"Study material order not found",
			"event",
			"telegram.study.material_order_missing",
			"session_id",
			state.Session.ID,
			"material_order",
			materialOrder,
		)
		sf.telegram.SendMessage(
			chatID,
			messages.materialOrderNotFound,
		)
		return
	}

	item := items[idx]
	text := renderStudyMaterial(
		item.Material,
		idx,
		len(items),
	)
	keyboard := studyMaterialKeyboard(
		state.Session.ID,
		item.SessionMaterial.MaterialOrder,
		idx == 0,
		idx == len(items)-1,
		sf.bot.isLLMAllowed(&tgbotapi.User{ID: state.Session.UserID}),
	)
	if sf.bot.services != nil && sf.bot.services.MaterialPreference != nil {
		keyboard.InlineKeyboard = append(
			keyboard.InlineKeyboard,
			tgbotapi.NewInlineKeyboardRow(
				tgbotapi.NewInlineKeyboardButtonData(
					messages.studySettingsButton,
					fmt.Sprintf(
						formatStudyPolicy,
						state.Session.ID,
						item.SessionMaterial.MaterialID,
					),
				),
			),
		)
	}

	if editMessageID != nil {
		sf.telegram.EditMessage(
			chatID,
			*editMessageID,
			text,
			&keyboard,
		)
		return
	}
	sf.telegram.SendMessageWithKeyboard(
		chatID,
		text,
		keyboard,
	)
}

func studyMaterialKeyboard(
	sessionID,
	materialOrder int,
	isFirst,
	isLast,
	showAskLLM bool,
) tgbotapi.InlineKeyboardMarkup {
	messages := botMessagesByLocale[botDefaultLocale]
	buttons := make(
		[]tgbotapi.InlineKeyboardButton,
		0,
		2,
	)
	if !isFirst {
		buttons = append(
			buttons,
			tgbotapi.NewInlineKeyboardButtonData(
				messages.previousButton,
				fmt.Sprintf(
					formatStudyPrev,
					sessionID,
					materialOrder,
				),
			),
		)
	}
	if isLast {
		buttons = append(
			buttons,
			tgbotapi.NewInlineKeyboardButtonData(
				messages.completeButton,
				fmt.Sprintf(
					formatStudyFinish,
					sessionID,
					materialOrder,
				),
			),
		)
	} else {
		buttons = append(
			buttons,
			tgbotapi.NewInlineKeyboardButtonData(
				messages.nextButton,
				fmt.Sprintf(
					formatStudyNext,
					sessionID,
					materialOrder,
				),
			),
		)
	}
	rows := [][]tgbotapi.InlineKeyboardButton{
		tgbotapi.NewInlineKeyboardRow(buttons...),
	}
	if showAskLLM {
		rows = append(
			rows,
			tgbotapi.NewInlineKeyboardRow(
				tgbotapi.NewInlineKeyboardButtonData(
					messages.askButton,
					fmt.Sprintf(
						formatStudyAskLLM,
						sessionID,
						materialOrder,
					),
				),
			),
		)
	}
	return tgbotapi.NewInlineKeyboardMarkup(rows...)
}

// handleAskLLMQuestion arms one contextual LLM question for a Study material.
// Ownership and material membership are checked before accepting the callback;
// they are checked again when resolving the context after the text reply.
func (sf *StudyFlow) handleAskLLMQuestion(
	ctx context.Context,
	cb *tgbotapi.CallbackQuery,
	sessionID int,
	materialOrderStr string,
) {
	messages := botMessagesByLocale[botDefaultLocale]
	if cb.Message == nil || !sf.bot.isLLMAllowed(cb.From) {
		return
	}
	materialOrder, err := strconv.Atoi(materialOrderStr)
	if err != nil {
		return
	}
	if sf.bot.input == nil || sf.bot.services == nil || sf.bot.services.StudyActiveSession == nil {
		sf.telegram.SendMessage(
			cb.Message.Chat.ID,
			messages.llmQuestionActivationFailed,
		)
		return
	}

	state, err := sf.bot.services.StudyActiveSession.LoadOwnedStudySessionState(
		ctx,
		sessionID,
		cb.From.ID,
	)
	if err != nil || state.Session.Status == model.SessionCompleted {
		slog.WarnContext(
			ctx,
			"Rejected study LLM question activation",
			"event",
			"telegram.llm.study_activate_rejected",
			"session_id",
			sessionID,
			"material_order",
			materialOrder,
			"error",
			err,
		)
		sf.telegram.SendMessage(
			cb.Message.Chat.ID,
			messages.currentMaterialQuestionUnavailable,
		)
		return
	}
	if _, _, ok := state.ItemByOrder(materialOrder); !ok {
		sf.telegram.SendMessage(
			cb.Message.Chat.ID,
			messages.materialNotFound,
		)
		return
	}

	input := model.PendingLLMInput{
		Kind:          model.PendingLLMStudyMaterial,
		SessionID:     sessionID,
		MaterialOrder: materialOrder,
	}
	if err := sf.bot.input.SetLLMPending(
		ctx,
		cb.From.ID,
		input,
	); err != nil {
		slog.ErrorContext(
			ctx,
			"Failed to activate in-study LLM mode",
			"event",
			"telegram.llm.study_activate_failed",
			"session_id",
			sessionID,
			"material_order",
			materialOrder,
			"error",
			err,
		)
		sf.telegram.SendMessage(
			cb.Message.Chat.ID,
			messages.llmQuestionActivationFailed,
		)
		return
	}
	sf.telegram.SendMessageWithKeyboard(
		cb.Message.Chat.ID,
		messages.llmQuestionPrompt,
		llmCancelKeyboard(),
	)
}

func studyMaterialIndexByOrder(
	items []model.StudySessionMaterial,
	materialOrder int,
) int {
	for idx, item := range items {
		if item.SessionMaterial.MaterialOrder == materialOrder {
			return idx
		}
	}
	return -1
}

func nextUnstudiedMaterialOrder(state *model.StudyActiveSessionState) int {
	idx := state.NextUnstudiedIndex()
	if idx >= len(state.Items) {
		return len(state.Items)
	}
	return state.Items[idx].SessionMaterial.MaterialOrder
}

type vocabularyStudyPayload struct {
	Kana         string `json:"kana"`
	Kanji        string `json:"kanji"`
	MeaningKo    string `json:"meaning_ko"`
	PartOfSpeech string `json:"part_of_speech"`
}

func renderStudyMaterial(
	material model.Material,
	idx,
	total int,
) string {
	header := fmt.Sprintf(
		"📚 <b>Study Session</b>\n\n<b>%d/%d · %s</b>\n\n",
		idx+1,
		total,
		escapeHTML(materialCategoryLabel(material.Category)),
	)
	title := fmt.Sprintf(
		"<b>%s</b>",
		escapeHTML(material.Title),
	)

	switch material.Category {
	case model.MaterialCategoryVocabulary:
		return header + title + renderVocabularyPayload(material.Payload)
	case model.MaterialCategoryGrammar:
		return header + title + renderGrammarPayload(material.Payload)
	case model.MaterialCategoryReading:
		return header + title + renderReadingPayload(material.Payload)
	default:
		return header + title + renderGenericPayload(material.Payload)
	}
}

func renderVocabularyPayload(payload json.RawMessage) string {
	messages := botMessagesByLocale[botDefaultLocale]
	var vocab vocabularyStudyPayload
	if err := json.Unmarshal(
		payload,
		&vocab,
	); err != nil {
		return renderGenericPayload(payload)
	}

	lines := make(
		[]string,
		0,
		4,
	)
	if strings.TrimSpace(vocab.Kana) != "" {
		lines = append(
			lines,
			fmt.Sprintf(
				messages.vocabularyReadingFormat,
				escapeHTML(vocab.Kana),
			),
		)
	}
	if strings.TrimSpace(vocab.Kanji) != "" {
		lines = append(
			lines,
			fmt.Sprintf(
				messages.vocabularyWritingFormat,
				escapeHTML(vocab.Kanji),
			),
		)
	}
	if strings.TrimSpace(vocab.MeaningKo) != "" {
		lines = append(
			lines,
			fmt.Sprintf(
				messages.meaningFormat,
				escapeHTML(vocab.MeaningKo),
			),
		)
	}
	if strings.TrimSpace(vocab.PartOfSpeech) != "" {
		lines = append(
			lines,
			fmt.Sprintf(
				messages.partOfSpeechFormat,
				escapeHTML(vocab.PartOfSpeech),
			),
		)
	}
	if len(lines) == 0 {
		return ""
	}
	return "\n\n" + strings.Join(
		lines,
		"\n",
	)
}

type grammarStudyPayload struct {
	Pattern        string `json:"pattern"`
	MeaningKo      string `json:"meaning_ko"`
	ExplanationKo  string `json:"explanation_ko"`
	Example        string `json:"example"`
	ExampleReading string `json:"example_reading"`
	TranslationKo  string `json:"translation_ko"`
}

func renderGrammarPayload(payload json.RawMessage) string {
	messages := botMessagesByLocale[botDefaultLocale]
	var grammar grammarStudyPayload
	if err := json.Unmarshal(
		payload,
		&grammar,
	); err != nil {
		return renderGenericPayload(payload)
	}

	lines := make(
		[]string,
		0,
		4,
	)
	if strings.TrimSpace(grammar.MeaningKo) != "" {
		lines = append(
			lines,
			fmt.Sprintf(
				messages.meaningFormat,
				escapeHTML(grammar.MeaningKo),
			),
		)
	}
	if strings.TrimSpace(grammar.ExplanationKo) != "" {
		lines = append(
			lines,
			fmt.Sprintf(
				messages.explanationFormat,
				escapeHTML(grammar.ExplanationKo),
			),
		)
	}
	if strings.TrimSpace(grammar.Example) != "" {
		lines = append(
			lines,
			fmt.Sprintf(
				messages.exampleFormat,
				escapeHTML(grammar.Example),
			),
		)
	}
	if strings.TrimSpace(grammar.ExampleReading) != "" {
		lines = append(
			lines,
			fmt.Sprintf(
				messages.readingFormat,
				escapeHTML(grammar.ExampleReading),
			),
		)
	}
	if strings.TrimSpace(grammar.TranslationKo) != "" {
		lines = append(
			lines,
			fmt.Sprintf(
				messages.translationFormat,
				escapeHTML(grammar.TranslationKo),
			),
		)
	}
	if len(lines) == 0 {
		return ""
	}
	return "\n\n" + strings.Join(
		lines,
		"\n",
	)
}

type readingStudyVocabulary struct {
	Surface   string `json:"surface"`
	Reading   string `json:"reading"`
	MeaningKo string `json:"meaning_ko"`
}

type readingStudyPayload struct {
	Passage       string                   `json:"passage"`
	Reading       string                   `json:"reading"`
	KeyVocabulary []readingStudyVocabulary `json:"key_vocabulary"`
}

// renderReadingPayload shows the passage, its full-hiragana reading aid, and
// key vocabulary. The Korean translation and answer rationale intentionally
// stay out — they surface only in the quiz explanation (ADR-036).
func renderReadingPayload(payload json.RawMessage) string {
	messages := botMessagesByLocale[botDefaultLocale]
	var reading readingStudyPayload
	if err := json.Unmarshal(
		payload,
		&reading,
	); err != nil {
		return renderGenericPayload(payload)
	}

	sections := make(
		[]string,
		0,
		3,
	)
	if strings.TrimSpace(reading.Passage) != "" {
		sections = append(
			sections,
			fmt.Sprintf(
				"<b>%s</b>",
				escapeHTML(reading.Passage),
			),
		)
	}
	if strings.TrimSpace(reading.Reading) != "" {
		sections = append(
			sections,
			fmt.Sprintf(
				messages.readingFormat,
				escapeHTML(reading.Reading),
			),
		)
	}
	vocabLines := make(
		[]string,
		0,
		len(reading.KeyVocabulary),
	)
	for _, vocab := range reading.KeyVocabulary {
		if strings.TrimSpace(vocab.Surface) == "" {
			continue
		}
		line := fmt.Sprintf(
			"・<b>%s</b>",
			escapeHTML(vocab.Surface),
		)
		if strings.TrimSpace(vocab.Reading) != "" && vocab.Reading != vocab.Surface {
			line += fmt.Sprintf(
				" (%s)",
				escapeHTML(vocab.Reading),
			)
		}
		if strings.TrimSpace(vocab.MeaningKo) != "" {
			line += fmt.Sprintf(
				" — %s",
				escapeHTML(vocab.MeaningKo),
			)
		}
		vocabLines = append(
			vocabLines,
			line,
		)
	}
	if len(vocabLines) > 0 {
		sections = append(
			sections,
			fmt.Sprintf(
				messages.keyVocabularyFormat,
				strings.Join(
					vocabLines,
					"\n",
				),
			),
		)
	}
	if len(sections) == 0 {
		return ""
	}
	return "\n\n" + strings.Join(
		sections,
		"\n\n",
	)
}

func renderGenericPayload(payload json.RawMessage) string {
	if len(payload) == 0 || string(payload) == "null" {
		return ""
	}
	var out bytes.Buffer
	if err := json.Indent(
		&out,
		payload,
		"",
		"  ",
	); err != nil {
		return ""
	}
	return "\n\n<pre>" + escapeHTML(out.String()) + "</pre>"
}

func materialCategoryLabel(category model.MaterialCategory) string {
	switch category {
	case model.MaterialCategoryKana:
		return "Kana"
	case model.MaterialCategoryVocabulary:
		return "Vocabulary"
	case model.MaterialCategoryGrammar:
		return "Grammar"
	case model.MaterialCategoryReading:
		return "Reading"
	case model.MaterialCategorySentence:
		return "Sentence"
	default:
		return string(category)
	}
}

func escapeHTML(s string) string {
	return html.EscapeString(s)
}
