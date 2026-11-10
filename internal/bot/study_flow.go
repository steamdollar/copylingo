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

// StudyFlowDependencies wires StudyFlow; every dependency is required.
type StudyFlowDependencies struct {
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

func NewStudyFlow(dependencies StudyFlowDependencies) *StudyFlow {
	return &StudyFlow{
		telegram:           dependencies.Telegram,
		session:            dependencies.Session,
		materialPreference: dependencies.MaterialPreference,
		input:              dependencies.Input,
	}
}

func (flow *StudyFlow) PushSession(
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
	return flow.telegram.SendMessageWithKeyboard(
		chatID,
		message.sessionPush,
		keyboard,
	)
}

func (flow *StudyFlow) HandleCallback(
	ctx context.Context,
	callbackQuery *tgbotapi.CallbackQuery,
) {
	if callbackQuery == nil || callbackQuery.From == nil || callbackQuery.Message == nil ||
		callbackQuery.Message.Chat == nil {
		return
	}

	// e.g. "study:42:next:3"
	parts := strings.Split(
		callbackQuery.Data,
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
		flow.handleMaterialPreference(
			ctx,
			callbackQuery,
			parts,
		)
	case callbackActionStart:
		flow.startSession(
			ctx,
			callbackQuery,
			sessionID,
		)
	case callbackActionKanji:
		if len(parts) != 4 {
			return
		}
		flow.showKanjiNotes(
			ctx,
			callbackQuery,
			sessionID,
			parts[3],
		)
	case callbackActionAsk:
		if len(parts) < 4 {
			return
		}
		flow.handleAskLLMQuestion(
			ctx,
			callbackQuery,
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
		flow.nextMaterial(
			ctx,
			callbackQuery,
			sessionID,
			currentOrder,
		)
	case callbackActionPrevious:
		if len(parts) < 4 {
			return
		}
		currentOrder, err := strconv.Atoi(parts[3])
		if err != nil {
			return
		}
		flow.previousMaterial(
			ctx,
			callbackQuery,
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
		flow.finishSession(
			ctx,
			callbackQuery,
			sessionID,
			currentOrder,
		)
	}
}

func (flow *StudyFlow) startSession(
	ctx context.Context,
	callbackQuery *tgbotapi.CallbackQuery,
	sessionID int,
) {
	messages := botMessagesByLocale[botDefaultLocale]
	state, err := flow.session.StartStudy(
		ctx,
		sessionID,
		callbackQuery.From.ID,
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
		flow.telegram.EditMessage(
			callbackQuery.Message.Chat.ID,
			callbackQuery.Message.MessageID,
			messages.startFailed,
			mainMenuKeyboard(),
		)
		return
	}
	if state.Session.Status == model.SessionCompleted {
		flow.telegram.EditMessage(
			callbackQuery.Message.Chat.ID,
			callbackQuery.Message.MessageID,
			messages.alreadyCompleted,
			mainMenuKeyboard(),
		)
		return
	}

	nextOrder := nextUnstudiedMaterialOrder(state)
	flow.showMaterial(
		ctx,
		callbackQuery.Message.Chat.ID,
		&callbackQuery.Message.MessageID,
		state,
		nextOrder,
	)
}

func (flow *StudyFlow) nextMaterial(
	ctx context.Context,
	callbackQuery *tgbotapi.CallbackQuery,
	sessionID,
	currentOrder int,
) {
	messages := botMessagesByLocale[botDefaultLocale]
	// update study proceeding state
	state, err := flow.session.MarkStudied(
		ctx,
		sessionID,
		callbackQuery.From.ID,
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
		flow.telegram.SendMessage(
			callbackQuery.Message.Chat.ID,
			messages.saveProgressFailed,
		)
		return
	}

	flow.showMaterial(
		ctx,
		callbackQuery.Message.Chat.ID,
		&callbackQuery.Message.MessageID,
		state,
		currentOrder+1,
	)
}

// previousMaterial re-shows an already-seen card; it never mutates studied state.
func (flow *StudyFlow) previousMaterial(
	ctx context.Context,
	callbackQuery *tgbotapi.CallbackQuery,
	sessionID,
	currentOrder int,
) {
	messages := botMessagesByLocale[botDefaultLocale]
	state, err := flow.session.StudyProgress(
		ctx,
		sessionID,
		callbackQuery.From.ID,
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
		flow.telegram.SendMessage(
			callbackQuery.Message.Chat.ID,
			messages.loadProgressFailed,
		)
		return
	}

	flow.showMaterial(
		ctx,
		callbackQuery.Message.Chat.ID,
		&callbackQuery.Message.MessageID,
		state,
		currentOrder-1,
	)
}

func (flow *StudyFlow) finishSession(
	ctx context.Context,
	callbackQuery *tgbotapi.CallbackQuery,
	sessionID,
	currentOrder int,
) {
	messages := botMessagesByLocale[botDefaultLocale]
	if err := flow.session.FinishStudy(
		ctx,
		sessionID,
		callbackQuery.From.ID,
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
			flow.telegram.SendMessage(
				callbackQuery.Message.Chat.ID,
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
		flow.telegram.SendMessage(
			callbackQuery.Message.Chat.ID,
			messages.completeFailed,
		)
		return
	}

	flow.telegram.EditMessage(
		callbackQuery.Message.Chat.ID,
		callbackQuery.Message.MessageID,
		messages.sessionCompleted,
		mainMenuKeyboard(),
	)
}

func (flow *StudyFlow) showMaterial(
	ctx context.Context,
	chatID int64,
	editMessageID *int,
	state *model.StudyActiveSessionState,
	materialOrder int,
) {
	messages := botMessagesByLocale[botDefaultLocale]
	items := state.Items
	if len(items) == 0 {
		flow.telegram.SendMessage(
			chatID,
			messages.noMaterials,
		)
		return
	}
	if materialOrder < 0 {
		materialOrder = 0
	}
	if materialOrder >= len(items) {
		if err := flow.session.CompleteStudy(
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
		flow.telegram.SendMessage(
			chatID,
			messages.autoCompleted,
		)
		return
	}
	index := studyMaterialIndexByOrder(
		items,
		materialOrder,
	)
	if index == -1 {
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
		flow.telegram.SendMessage(
			chatID,
			messages.materialOrderNotFound,
		)
		return
	}

	item := items[index]
	text := renderStudyMaterial(
		item.Material,
		index,
		len(items),
	)
	keyboard := studyMaterialKeyboard(
		state.Session.ID,
		item.SessionMaterial.MaterialOrder,
		index == 0,
		index == len(items)-1,
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
		flow.telegram.EditMessage(
			chatID,
			*editMessageID,
			text,
			&keyboard,
		)
		return
	}
	flow.telegram.SendMessageWithKeyboard(
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
					formatStudyPrevious,
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
func (flow *StudyFlow) handleAskLLMQuestion(
	ctx context.Context,
	callbackQuery *tgbotapi.CallbackQuery,
	sessionID int,
	materialOrderText string,
) {
	messages := botMessagesByLocale[botDefaultLocale]
	if callbackQuery.Message == nil || !isLLMAllowed(callbackQuery.From) {
		return
	}
	materialOrder, err := strconv.Atoi(materialOrderText)
	if err != nil {
		return
	}
	state, err := flow.session.StudyProgress(
		ctx,
		sessionID,
		callbackQuery.From.ID,
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
		flow.telegram.SendMessage(
			callbackQuery.Message.Chat.ID,
			messages.currentMaterialQuestionUnavailable,
		)
		return
	}
	if _, _, ok := state.ItemByOrder(materialOrder); !ok {
		flow.telegram.SendMessage(
			callbackQuery.Message.Chat.ID,
			messages.materialNotFound,
		)
		return
	}

	input := model.PendingLLMInput{
		Kind:          model.PendingLLMStudyMaterial,
		SessionID:     sessionID,
		MaterialOrder: materialOrder,
	}
	if err := flow.input.SetLLMPending(
		ctx,
		callbackQuery.From.ID,
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
		flow.telegram.SendMessage(
			callbackQuery.Message.Chat.ID,
			messages.llmQuestionActivationFailed,
		)
		return
	}
	flow.telegram.SendMessageWithKeyboard(
		callbackQuery.Message.Chat.ID,
		messages.llmQuestionPrompt,
		llmCancelKeyboard(),
	)
}

// showKanjiNotes replaces the card with its kanji notes and a button back to
// the card. Like the settings menu, it never marks the material studied.
func (flow *StudyFlow) showKanjiNotes(
	ctx context.Context,
	callbackQuery *tgbotapi.CallbackQuery,
	sessionID int,
	materialIDText string,
) {
	messages := botMessagesByLocale[botDefaultLocale]
	materialID, err := strconv.Atoi(materialIDText)
	if err != nil {
		return
	}
	state, err := flow.session.StudyProgress(
		ctx,
		sessionID,
		callbackQuery.From.ID,
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
		flow.telegram.AnswerCallbackAlert(
			callbackQuery.ID,
			messages.kanjiNotesUnavailable,
		)
		return
	}
	item, ok := state.ItemByMaterialID(materialID)
	if !ok {
		flow.telegram.AnswerCallbackAlert(
			callbackQuery.ID,
			messages.materialNotInSession,
		)
		return
	}
	var vocabulary vocabularyStudyPayload
	if err := json.Unmarshal(
		item.Material.Payload,
		&vocabulary,
	); err != nil || len(vocabulary.KanjiNotes)+len(vocabulary.KanjiReadings) == 0 {
		flow.telegram.AnswerCallbackAlert(
			callbackQuery.ID,
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
	flow.telegram.EditMessage(
		callbackQuery.Message.Chat.ID,
		callbackQuery.Message.MessageID,
		renderKanjiNotes(vocabulary),
		&keyboard,
	)
}

func studyMaterialIndexByOrder(
	items []model.StudySessionMaterial,
	materialOrder int,
) int {
	for index, item := range items {
		if item.SessionMaterial.MaterialOrder == materialOrder {
			return index
		}
	}
	return -1
}

func nextUnstudiedMaterialOrder(state *model.StudyActiveSessionState) int {
	index := state.NextUnstudiedIndex()
	if index >= len(state.Items) {
		return len(state.Items)
	}
	return state.Items[index].SessionMaterial.MaterialOrder
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
	index,
	total int,
) string {
	// Study sessions mix adjacent levels like quiz sessions (ADR-046), so the
	// header names each material's level; materials without one omit the marker.
	levelTag := ""
	if material.ProficiencyLevel != "" {
		levelTag = fmt.Sprintf(
			botMessagesByLocale[botDefaultLocale].questionLevelMarkerFormat,
			escapeHTML(material.ProficiencyLevel),
		)
	}
	header := fmt.Sprintf(
		"📚 <b>Study Session</b>\n\n<b>%d/%d%s · %s</b>\n\n",
		index+1,
		total,
		levelTag,
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
	var vocabulary vocabularyStudyPayload
	if err := json.Unmarshal(
		payload,
		&vocabulary,
	); err != nil {
		return renderGenericPayload(payload)
	}

	lines := make(
		[]string,
		0,
		4,
	)
	if strings.TrimSpace(vocabulary.Kana) != "" {
		lines = append(
			lines,
			fmt.Sprintf(
				messages.vocabularyReadingFormat,
				escapeHTML(vocabulary.Kana),
			),
		)
	}
	if strings.TrimSpace(vocabulary.Kanji) != "" {
		lines = append(
			lines,
			fmt.Sprintf(
				messages.vocabularyWritingFormat,
				escapeHTML(vocabulary.Kanji),
			),
		)
	}
	if strings.TrimSpace(vocabulary.MeaningKo) != "" {
		lines = append(
			lines,
			fmt.Sprintf(
				messages.meaningFormat,
				escapeHTML(vocabulary.MeaningKo),
			),
		)
	}
	if strings.TrimSpace(vocabulary.PartOfSpeech) != "" {
		lines = append(
			lines,
			fmt.Sprintf(
				messages.partOfSpeechFormat,
				escapeHTML(vocabulary.PartOfSpeech),
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
	var vocabulary vocabularyStudyPayload
	if err := json.Unmarshal(
		material.Payload,
		&vocabulary,
	); err != nil {
		return false
	}
	return len(vocabulary.KanjiNotes)+len(vocabulary.KanjiReadings) > 0
}

// renderKanjiNotes shows one block per kanji in word order: its title with
// its 훈음, its structure note when it has one, then how this word reads it
// and the kanji's common on'yomi, so an on'yomi seen here carries over to
// other words (ADR-074, ADR-076). A note whose kanji has no reading entry,
// such as one inside a jukuji group, follows the others without 훈음 or
// reading lines.
func renderKanjiNotes(vocabulary vocabularyStudyPayload) string {
	messages := botMessagesByLocale[botDefaultLocale]
	unrenderedNotes := make(
		map[string]model.KanjiNote,
		len(vocabulary.KanjiNotes),
	)
	for _, note := range vocabulary.KanjiNotes {
		unrenderedNotes[note.Character] = note
	}

	blocks := make(
		[]string,
		0,
		len(vocabulary.KanjiReadings)+len(vocabulary.KanjiNotes)+1,
	)
	blocks = append(
		blocks,
		fmt.Sprintf(
			messages.kanjiNotesHeadingFormat,
			escapeHTML(vocabulary.Kanji),
			escapeHTML(vocabulary.Kana),
		),
	)
	for _, reading := range vocabulary.KanjiReadings {
		lines := []string{
			kanjiTitle(
				reading.Text,
				reading.HunEum,
				"",
			),
		}
		if note, ok := unrenderedNotes[reading.Text]; ok {
			lines = kanjiNoteLines(
				note,
				reading.HunEum,
			)
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
	for _, note := range vocabulary.KanjiNotes {
		if _, ok := unrenderedNotes[note.Character]; ok {
			blocks = append(
				blocks,
				strings.Join(
					kanjiNoteLines(
						note,
						"",
					),
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

// kanjiTitle names a block's kanji, then its 훈음 so the reader knows what
// the kanji is before its parts (ADR-076), then the note's type when it has
// one: <b>伸</b> 펼 신 · 형성문자. Empty hunEum or noteType is left out.
func kanjiTitle(
	text,
	hunEum string,
	noteType model.KanjiNoteType,
) string {
	messages := botMessagesByLocale[botDefaultLocale]
	title := fmt.Sprintf(
		messages.kanjiTitleFormat,
		escapeHTML(text),
	)
	if hunEum != "" {
		title = fmt.Sprintf(
			messages.kanjiTitleHunEumFormat,
			title,
			escapeHTML(hunEum),
		)
	}
	if noteType != "" {
		title = fmt.Sprintf(
			messages.kanjiTitleTypeFormat,
			title,
			kanjiNoteTypeLabel(noteType),
		)
	}
	return title
}

// kanjiNoteLines writes a note's title with its 훈음 and type, its parts and
// its one-line explanation. hunEum comes from the kanji's reading entry and
// is empty for a note that has none.
func kanjiNoteLines(
	note model.KanjiNote,
	hunEum string,
) []string {
	lines := []string{
		kanjiTitle(
			note.Character,
			hunEum,
			note.Type,
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

// renderKanjiPart writes a part with what it gives its kanji in brackets
// (ADR-080): a meaning or form part its gloss, after the kanji a radical
// variant stands for, as 扌(手 · 손) or 木(나무); a sound part its own 훈음
// and the on'yomi it shares with the kanji, then 발음, as
// 交(사귈 교 · コウ · 발음) or 寺(절 사 · 발음) when the readings diverged.
// Empty values are left out.
func renderKanjiPart(part model.KanjiPart) string {
	messages := botMessagesByLocale[botDefaultLocale]
	values := []string{
		part.Base,
		part.GlossKo,
	}
	if part.Role == model.KanjiPartSound {
		values = []string{
			part.HunEum,
			part.On,
			messages.kanjiSoundPartLabel,
		}
	}
	shown := make(
		[]string,
		0,
		len(values),
	)
	for _, value := range values {
		if value != "" {
			shown = append(
				shown,
				escapeHTML(value),
			)
		}
	}
	return fmt.Sprintf(
		messages.kanjiPartFormat,
		escapeHTML(part.Part),
		strings.Join(
			shown,
			" · ",
		),
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
	vocabularyLines := make(
		[]string,
		0,
		len(reading.KeyVocabulary),
	)
	for _, vocabulary := range reading.KeyVocabulary {
		if strings.TrimSpace(vocabulary.Surface) == "" {
			continue
		}
		line := fmt.Sprintf(
			"・<b>%s</b>",
			escapeHTML(vocabulary.Surface),
		)
		if strings.TrimSpace(vocabulary.Reading) != "" && vocabulary.Reading != vocabulary.Surface {
			line += fmt.Sprintf(
				" (%s)",
				escapeHTML(vocabulary.Reading),
			)
		}
		if strings.TrimSpace(vocabulary.MeaningKo) != "" {
			line += fmt.Sprintf(
				" — %s",
				escapeHTML(vocabulary.MeaningKo),
			)
		}
		vocabularyLines = append(
			vocabularyLines,
			line,
		)
	}
	if len(vocabularyLines) > 0 {
		sections = append(
			sections,
			fmt.Sprintf(
				messages.keyVocabularyFormat,
				strings.Join(
					vocabularyLines,
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

func escapeHTML(text string) string {
	return html.EscapeString(text)
}
