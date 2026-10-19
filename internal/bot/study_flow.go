package bot

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"html"
	"log/slog"
	"slices"
	"strconv"
	"strings"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"

	"github.com/lsj/copylingo/internal/callback"
	"github.com/lsj/copylingo/internal/model"
	"github.com/lsj/copylingo/internal/service"
)

// studySession is the Study part of service.SessionService that StudyFlow
// drives: start, per-card progress, finish/complete, and owner-checked reads.
type studySession interface {
	StartStudy(
		ctx context.Context,
		sessionID int,
		userID int64,
	) (*model.StudyActiveSessionState, error)
	MarkStudied(
		ctx context.Context,
		sessionID int,
		userID int64,
		materialOrder int,
	) (*model.StudyActiveSessionState, error)
	FinishStudy(
		ctx context.Context,
		sessionID int,
		userID int64,
		lastMaterialOrder int,
	) error
	CompleteStudy(
		ctx context.Context,
		sessionID int,
		userID int64,
	) error
	StudyProgress(
		ctx context.Context,
		sessionID int,
		userID int64,
	) (*model.StudyActiveSessionState, error)
}

// studyInputStore arms the per-card "ask the LLM" pending token.
type studyInputStore interface {
	SetLLMPending(
		ctx context.Context,
		userID int64,
		input model.PendingLLMInput,
	) error
}

// StudyFlowDeps wires StudyFlow; every dependency is required.
type StudyFlowDeps struct {
	Telegram           *TelegramClient
	Session            studySession
	MaterialPreference materialPreferences
	Input              studyInputStore
}

// StudyFlow handles material-based study sessions.
type StudyFlow struct {
	telegram           *TelegramClient
	session            studySession
	materialPreference materialPreferences
	input              studyInputStore
}

func NewStudyFlow(deps StudyFlowDeps) *StudyFlow {
	return &StudyFlow{
		telegram:           deps.Telegram,
		session:            deps.Session,
		materialPreference: deps.MaterialPreference,
		input:              deps.Input,
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
	case callbackActionKanji:
		if len(parts) != 4 {
			return
		}
		sf.showKanjiNotes(
			ctx,
			cb,
			sessionID,
			parts[3],
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
	state, err := sf.session.StartStudy(
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
	state, err := sf.session.MarkStudied(
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
	state, err := sf.session.StudyProgress(
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
	if err := sf.session.FinishStudy(
		ctx,
		sessionID,
		cb.From.ID,
		currentOrder,
	); err != nil {
		if errors.Is(
			err,
			service.ErrStudyFinalMarkFailed,
		) {
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
		if err := sf.session.CompleteStudy(
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
		isLLMAllowed(&tgbotapi.User{ID: state.Session.UserID}),
	)
	// The kanji button exists only when the payload carries notes or
	// readings, so other cards keep their previous keyboard (ADR-073,
	// ADR-074). It goes right below the navigation row (index 0), above the
	// ask-LLM row.
	if hasKanjiDetail(item.Material) {
		keyboard.InlineKeyboard = slices.Insert(
			keyboard.InlineKeyboard,
			1,
			tgbotapi.NewInlineKeyboardRow(
				tgbotapi.NewInlineKeyboardButtonData(
					messages.kanjiNotesButton,
					fmt.Sprintf(
						formatStudyKanji,
						state.Session.ID,
						item.SessionMaterial.MaterialID,
					),
				),
			),
		)
	}
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
	if cb.Message == nil || !isLLMAllowed(cb.From) {
		return
	}
	materialOrder, err := strconv.Atoi(materialOrderStr)
	if err != nil {
		return
	}
	if sf.input == nil {
		sf.telegram.SendMessage(
			cb.Message.Chat.ID,
			messages.llmQuestionActivationFailed,
		)
		return
	}

	state, err := sf.session.StudyProgress(
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
	if err := sf.input.SetLLMPending(
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

// showKanjiNotes replaces the card with its kanji notes and a button back to
// the card. Like the settings menu, it never marks the material studied.
func (sf *StudyFlow) showKanjiNotes(
	ctx context.Context,
	cb *tgbotapi.CallbackQuery,
	sessionID int,
	materialIDStr string,
) {
	messages := botMessagesByLocale[botDefaultLocale]
	materialID, err := strconv.Atoi(materialIDStr)
	if err != nil {
		return
	}
	state, err := sf.session.StudyProgress(
		ctx,
		sessionID,
		cb.From.ID,
	)
	if err != nil {
		slog.WarnContext(
			ctx,
			"Rejected study kanji notes callback",
			"event",
			"telegram.study.kanji_notes_rejected",
			"session_id",
			sessionID,
			"material_id",
			materialID,
			"error",
			err,
		)
		sf.telegram.AnswerCallbackAlert(
			cb.ID,
			messages.kanjiNotesUnavailable,
		)
		return
	}
	item, ok := state.ItemByMaterialID(materialID)
	if !ok {
		sf.telegram.AnswerCallbackAlert(
			cb.ID,
			messages.materialNotInSession,
		)
		return
	}
	var vocab vocabularyStudyPayload
	if err := json.Unmarshal(
		item.Material.Payload,
		&vocab,
	); err != nil || len(vocab.KanjiNotes)+len(vocab.KanjiReadings) == 0 {
		sf.telegram.AnswerCallbackAlert(
			cb.ID,
			messages.kanjiNotesUnavailable,
		)
		return
	}
	keyboard := tgbotapi.NewInlineKeyboardMarkup(
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData(
				messages.materialBackToCardButton,
				fmt.Sprintf(
					formatStudyCard,
					sessionID,
					materialID,
				),
			),
		),
	)
	sf.telegram.EditMessage(
		cb.Message.Chat.ID,
		cb.Message.MessageID,
		renderKanjiNotes(vocab),
		&keyboard,
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
	Kana          string               `json:"kana"`
	Kanji         string               `json:"kanji"`
	MeaningKo     string               `json:"meaning_ko"`
	PartOfSpeech  string               `json:"part_of_speech"`
	KanjiNotes    []model.KanjiNote    `json:"kanji_notes"`
	KanjiReadings []model.KanjiReading `json:"kanji_readings"`
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

// hasKanjiDetail reports whether a vocabulary card has kanji notes or
// readings to show behind the kanji button; other categories have none.
func hasKanjiDetail(material model.Material) bool {
	if material.Category != model.MaterialCategoryVocabulary {
		return false
	}
	var vocab vocabularyStudyPayload
	if err := json.Unmarshal(
		material.Payload,
		&vocab,
	); err != nil {
		return false
	}
	return len(vocab.KanjiNotes)+len(vocab.KanjiReadings) > 0
}

// renderKanjiNotes shows one block per kanji in word order: its structure
// note when it has one, then how this word reads it and the kanji's common
// on'yomi, so an on'yomi seen here carries over to other words (ADR-074). A
// note whose kanji has no reading entry, such as one inside a jukuji group,
// follows the others without reading lines.
func renderKanjiNotes(vocab vocabularyStudyPayload) string {
	messages := botMessagesByLocale[botDefaultLocale]
	unrenderedNotes := make(
		map[string]model.KanjiNote,
		len(vocab.KanjiNotes),
	)
	for _, note := range vocab.KanjiNotes {
		unrenderedNotes[note.Char] = note
	}

	blocks := make(
		[]string,
		0,
		len(vocab.KanjiReadings)+len(vocab.KanjiNotes)+1,
	)
	blocks = append(
		blocks,
		fmt.Sprintf(
			messages.kanjiNotesHeadingFormat,
			escapeHTML(vocab.Kanji),
			escapeHTML(vocab.Kana),
		),
	)
	for _, reading := range vocab.KanjiReadings {
		lines := []string{
			fmt.Sprintf(
				messages.kanjiTitleFormat,
				escapeHTML(reading.Text),
			),
		}
		if note, ok := unrenderedNotes[reading.Text]; ok {
			lines = kanjiNoteLines(note)
			delete(
				unrenderedNotes,
				reading.Text,
			)
		}
		blocks = append(
			blocks,
			strings.Join(
				append(
					lines,
					kanjiReadingLines(reading)...,
				),
				"\n",
			),
		)
	}
	for _, note := range vocab.KanjiNotes {
		if _, ok := unrenderedNotes[note.Char]; ok {
			blocks = append(
				blocks,
				strings.Join(
					kanjiNoteLines(note),
					"\n",
				),
			)
		}
	}
	return strings.Join(
		blocks,
		"\n\n",
	)
}

// kanjiNoteLines writes a note's title with its type, its parts and its
// one-line explanation.
func kanjiNoteLines(note model.KanjiNote) []string {
	messages := botMessagesByLocale[botDefaultLocale]
	lines := []string{
		fmt.Sprintf(
			messages.kanjiNoteTitleFormat,
			escapeHTML(note.Char),
			kanjiNoteTypeLabel(note.Type),
		),
	}
	parts := make(
		[]string,
		0,
		len(note.Parts),
	)
	for _, part := range note.Parts {
		parts = append(
			parts,
			renderKanjiPart(part),
		)
	}
	if len(parts) > 0 {
		lines = append(
			lines,
			strings.Join(
				parts,
				" + ",
			),
		)
	}
	if strings.TrimSpace(note.ExplanationKo) != "" {
		lines = append(
			lines,
			escapeHTML(note.ExplanationKo),
		)
	}
	return lines
}

// kanjiReadingLines writes how this word reads the kanji, as 이 단어에선: の
// (훈독), and the kanji's common on'yomi, which a jukuji group has none of.
func kanjiReadingLines(reading model.KanjiReading) []string {
	messages := botMessagesByLocale[botDefaultLocale]
	lines := []string{
		fmt.Sprintf(
			messages.kanjiReadingInWordFormat,
			escapeHTML(reading.Reading),
			kanjiReadingTypeLabel(reading.Type),
		),
	}
	if len(reading.On) > 0 {
		lines = append(
			lines,
			fmt.Sprintf(
				messages.kanjiOnReadingsFormat,
				escapeHTML(strings.Join(
					reading.On,
					" · ",
				)),
			),
		)
	}
	return lines
}

func kanjiReadingTypeLabel(readingType model.KanjiReadingType) string {
	messages := botMessagesByLocale[botDefaultLocale]
	switch readingType {
	case model.KanjiReadingOn:
		return messages.kanjiReadingOn
	case model.KanjiReadingKun:
		return messages.kanjiReadingKun
	default:
		return messages.kanjiReadingJukuji
	}
}

// renderKanjiPart writes 氵(물) for meaning/form parts, 青(セイ · 발음) for a
// sound part with a shared on'yomi, and 寺(발음) when the readings diverged.
func renderKanjiPart(part model.KanjiPart) string {
	messages := botMessagesByLocale[botDefaultLocale]
	if part.Role == model.KanjiPartSound {
		if part.On == "" {
			return fmt.Sprintf(
				messages.kanjiSoundPartNoOnFormat,
				escapeHTML(part.Part),
			)
		}
		return fmt.Sprintf(
			messages.kanjiSoundPartFormat,
			escapeHTML(part.Part),
			escapeHTML(part.On),
		)
	}
	return fmt.Sprintf(
		messages.kanjiPartGlossFormat,
		escapeHTML(part.Part),
		escapeHTML(part.GlossKo),
	)
}

func kanjiNoteTypeLabel(noteType model.KanjiNoteType) string {
	messages := botMessagesByLocale[botDefaultLocale]
	switch noteType {
	case model.KanjiNotePhonoSemantic:
		return messages.kanjiTypePhonoSemantic
	case model.KanjiNoteCompound:
		return messages.kanjiTypeCompound
	case model.KanjiNoteIndicative:
		return messages.kanjiTypeIndicative
	default:
		return messages.kanjiTypePictograph
	}
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
