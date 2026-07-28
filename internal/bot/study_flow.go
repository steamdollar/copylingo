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

	"github.com/lsj/copylingo/internal/config"
	"github.com/lsj/copylingo/internal/model"
)

// StudyFlow handles material-based study sessions.
type StudyFlow struct {
	bot *Bot
}

func NewStudyFlow(bot *Bot) *StudyFlow {
	return &StudyFlow{bot: bot}
}

func (sf *StudyFlow) PushSession(ctx context.Context, chatID int64, sessionID int) error {
	text := "☀️ <b>정오 학습 세션이 도착했습니다!</b>\n\n오늘 레벨에 맞춘 Study Material을 짧게 훑고 가세요."
	keyboard := tgbotapi.NewInlineKeyboardMarkup(
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData("▶️ 시작하기", fmt.Sprintf(config.FormatStudyStart, sessionID)),
		),
	)
	return sf.bot.SendMessageWithKeyboard(chatID, text, keyboard)
}

func (sf *StudyFlow) HandleCallback(ctx context.Context, cb *tgbotapi.CallbackQuery) {
	if cb.Message == nil {
		return
	}

	parts := strings.Split(cb.Data, ":")
	if len(parts) < 3 {
		return
	}

	sessionID, err := strconv.Atoi(parts[1])
	if err != nil {
		slog.WarnContext(ctx, "Invalid study session ID in callback",
			"event", "telegram.study.invalid_session_id",
			"callback_type", parts[0],
		)
		return
	}

	switch parts[2] {
	case "start":
		sf.startSession(ctx, cb, sessionID)
	case "next":
		if len(parts) < 4 {
			return
		}
		currentOrder, err := strconv.Atoi(parts[3])
		if err != nil {
			return
		}
		sf.nextMaterial(ctx, cb, sessionID, currentOrder)
	case "prev":
		if len(parts) < 4 {
			return
		}
		currentOrder, err := strconv.Atoi(parts[3])
		if err != nil {
			return
		}
		sf.prevMaterial(ctx, cb, sessionID, currentOrder)
	case "finish":
		if len(parts) < 4 {
			return
		}
		currentOrder, err := strconv.Atoi(parts[3])
		if err != nil {
			return
		}
		sf.finishSession(ctx, cb, sessionID, currentOrder)
	}
}

func (sf *StudyFlow) startSession(ctx context.Context, cb *tgbotapi.CallbackQuery, sessionID int) {
	state, err := sf.bot.services.StudyActiveSession.Start(ctx, sessionID, cb.From.ID)
	if err != nil {
		slog.ErrorContext(ctx, "Failed to start study active session",
			"event", "telegram.study.active_start_failed",
			"session_id", sessionID,
			"error", err,
		)
		sf.bot.EditMessage(cb.Message.Chat.ID, cb.Message.MessageID,
			"❌ Study Session을 시작하지 못했습니다.",
			mainMenuKeyboard(),
		)
		return
	}
	if state.Session.Status == model.SessionCompleted {
		sf.bot.EditMessage(cb.Message.Chat.ID, cb.Message.MessageID,
			"✅ 이미 완료한 Study Session입니다.",
			mainMenuKeyboard(),
		)
		return
	}

	nextOrder := nextUnstudiedMaterialOrder(state)
	sf.showMaterial(ctx, cb.Message.Chat.ID, &cb.Message.MessageID, state, nextOrder)
}

func (sf *StudyFlow) nextMaterial(ctx context.Context, cb *tgbotapi.CallbackQuery, sessionID, currentOrder int) {
	state, err := sf.bot.services.StudyActiveSession.MarkStudied(ctx, sessionID, cb.From.ID, currentOrder)
	if err != nil {
		slog.ErrorContext(ctx, "Failed to mark study material",
			"event", "telegram.study.material_mark_failed",
			"session_id", sessionID,
			"material_order", currentOrder,
			"error", err,
		)
		sf.bot.SendMessage(cb.Message.Chat.ID, "❌ Study 진행 상태를 저장하지 못했습니다.")
		return
	}

	sf.showMaterial(ctx, cb.Message.Chat.ID, &cb.Message.MessageID, state, currentOrder+1)
}

// prevMaterial re-shows an already-seen card; it never mutates studied state.
func (sf *StudyFlow) prevMaterial(ctx context.Context, cb *tgbotapi.CallbackQuery, sessionID, currentOrder int) {
	state, err := sf.bot.services.StudyActiveSession.GetOwned(ctx, sessionID, cb.From.ID)
	if err != nil {
		slog.ErrorContext(ctx, "Failed to load study session for prev material",
			"event", "telegram.study.material_prev_failed",
			"session_id", sessionID,
			"material_order", currentOrder,
			"error", err,
		)
		sf.bot.SendMessage(cb.Message.Chat.ID, "❌ Study 진행 상태를 불러오지 못했습니다.")
		return
	}

	sf.showMaterial(ctx, cb.Message.Chat.ID, &cb.Message.MessageID, state, currentOrder-1)
}

func (sf *StudyFlow) finishSession(ctx context.Context, cb *tgbotapi.CallbackQuery, sessionID, currentOrder int) {
	if _, err := sf.bot.services.StudyActiveSession.MarkStudied(ctx, sessionID, cb.From.ID, currentOrder); err != nil {
		slog.ErrorContext(ctx, "Failed to mark final study material",
			"event", "telegram.study.final_material_mark_failed",
			"session_id", sessionID,
			"material_order", currentOrder,
			"error", err,
		)
		sf.bot.SendMessage(cb.Message.Chat.ID, "❌ Study 완료 상태를 저장하지 못했습니다.")
		return
	}
	if err := sf.bot.services.StudyActiveSession.Complete(ctx, sessionID, cb.From.ID); err != nil {
		slog.ErrorContext(ctx, "Failed to complete study session",
			"event", "telegram.study.complete_failed",
			"session_id", sessionID,
			"error", err,
		)
		sf.bot.SendMessage(cb.Message.Chat.ID, "❌ Study Session을 완료하지 못했습니다.")
		return
	}

	sf.bot.EditMessage(cb.Message.Chat.ID, cb.Message.MessageID,
		"✅ <b>정오 Study Session 완료!</b>\n\n오늘 학습한 Material 이력이 저장됐습니다.",
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
	items := state.Items
	if len(items) == 0 {
		sf.bot.SendMessage(chatID, "⚠️ 표시할 Study Material이 없습니다.")
		return
	}
	if materialOrder < 0 {
		materialOrder = 0
	}
	if materialOrder >= len(items) {
		if err := sf.bot.services.StudyActiveSession.Complete(ctx, state.Session.ID, state.Session.UserID); err != nil {
			slog.ErrorContext(ctx, "Failed to auto-complete study session",
				"event", "telegram.study.auto_complete_failed",
				"session_id", state.Session.ID,
				"error", err,
			)
		}
		sf.bot.SendMessage(chatID, "✅ Study Session을 완료했습니다.")
		return
	}
	idx := studyMaterialIndexByOrder(items, materialOrder)
	if idx == -1 {
		slog.WarnContext(ctx, "Study material order not found",
			"event", "telegram.study.material_order_missing",
			"session_id", state.Session.ID,
			"material_order", materialOrder,
		)
		sf.bot.SendMessage(chatID, "⚠️ Study Material 순서를 찾지 못했습니다.")
		return
	}

	item := items[idx]
	text := renderStudyMaterial(item.Material, idx, len(items))
	keyboard := studyMaterialKeyboard(
		state.Session.ID,
		item.SessionMaterial.MaterialOrder,
		idx == 0,
		idx == len(items)-1,
	)

	if editMessageID != nil {
		sf.bot.EditMessage(chatID, *editMessageID, text, &keyboard)
		return
	}
	sf.bot.SendMessageWithKeyboard(chatID, text, keyboard)
}

func studyMaterialKeyboard(sessionID, materialOrder int, isFirst, isLast bool) tgbotapi.InlineKeyboardMarkup {
	buttons := make([]tgbotapi.InlineKeyboardButton, 0, 2)
	if !isFirst {
		buttons = append(buttons, tgbotapi.NewInlineKeyboardButtonData(
			"← 이전",
			fmt.Sprintf(config.FormatStudyPrev, sessionID, materialOrder),
		))
	}
	if isLast {
		buttons = append(buttons, tgbotapi.NewInlineKeyboardButtonData(
			"✅ 완료",
			fmt.Sprintf(config.FormatStudyFinish, sessionID, materialOrder),
		))
	} else {
		buttons = append(buttons, tgbotapi.NewInlineKeyboardButtonData(
			"다음 →",
			fmt.Sprintf(config.FormatStudyNext, sessionID, materialOrder),
		))
	}
	return tgbotapi.NewInlineKeyboardMarkup(tgbotapi.NewInlineKeyboardRow(buttons...))
}

func studyMaterialIndexByOrder(items []model.StudySessionMaterial, materialOrder int) int {
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

func renderStudyMaterial(material model.Material, idx, total int) string {
	header := fmt.Sprintf("☀️ <b>정오 Study</b>\n\n<b>%d/%d · %s</b>\n\n",
		idx+1, total, escapeHTML(materialCategoryLabel(material.Category)))
	title := fmt.Sprintf("<b>%s</b>", escapeHTML(material.Title))

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
	var vocab vocabularyStudyPayload
	if err := json.Unmarshal(payload, &vocab); err != nil {
		return renderGenericPayload(payload)
	}

	lines := make([]string, 0, 4)
	if strings.TrimSpace(vocab.Kana) != "" {
		lines = append(lines, fmt.Sprintf("읽기: <b>%s</b>", escapeHTML(vocab.Kana)))
	}
	if strings.TrimSpace(vocab.Kanji) != "" {
		lines = append(lines, fmt.Sprintf("표기: <b>%s</b>", escapeHTML(vocab.Kanji)))
	}
	if strings.TrimSpace(vocab.MeaningKo) != "" {
		lines = append(lines, fmt.Sprintf("의미: <b>%s</b>", escapeHTML(vocab.MeaningKo)))
	}
	if strings.TrimSpace(vocab.PartOfSpeech) != "" {
		lines = append(lines, fmt.Sprintf("품사: <b>%s</b>", escapeHTML(vocab.PartOfSpeech)))
	}
	if len(lines) == 0 {
		return ""
	}
	return "\n\n" + strings.Join(lines, "\n")
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
	var grammar grammarStudyPayload
	if err := json.Unmarshal(payload, &grammar); err != nil {
		return renderGenericPayload(payload)
	}

	lines := make([]string, 0, 4)
	if strings.TrimSpace(grammar.MeaningKo) != "" {
		lines = append(lines, fmt.Sprintf("의미: <b>%s</b>", escapeHTML(grammar.MeaningKo)))
	}
	if strings.TrimSpace(grammar.ExplanationKo) != "" {
		lines = append(lines, fmt.Sprintf("설명: %s", escapeHTML(grammar.ExplanationKo)))
	}
	if strings.TrimSpace(grammar.Example) != "" {
		lines = append(lines, fmt.Sprintf("예문: <b>%s</b>", escapeHTML(grammar.Example)))
	}
	if strings.TrimSpace(grammar.ExampleReading) != "" {
		lines = append(lines, fmt.Sprintf("읽기: %s", escapeHTML(grammar.ExampleReading)))
	}
	if strings.TrimSpace(grammar.TranslationKo) != "" {
		lines = append(lines, fmt.Sprintf("해석: %s", escapeHTML(grammar.TranslationKo)))
	}
	if len(lines) == 0 {
		return ""
	}
	return "\n\n" + strings.Join(lines, "\n")
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
	var reading readingStudyPayload
	if err := json.Unmarshal(payload, &reading); err != nil {
		return renderGenericPayload(payload)
	}

	sections := make([]string, 0, 3)
	if strings.TrimSpace(reading.Passage) != "" {
		sections = append(sections, fmt.Sprintf("<b>%s</b>", escapeHTML(reading.Passage)))
	}
	if strings.TrimSpace(reading.Reading) != "" {
		sections = append(sections, fmt.Sprintf("읽기: %s", escapeHTML(reading.Reading)))
	}
	vocabLines := make([]string, 0, len(reading.KeyVocabulary))
	for _, vocab := range reading.KeyVocabulary {
		if strings.TrimSpace(vocab.Surface) == "" {
			continue
		}
		line := fmt.Sprintf("・<b>%s</b>", escapeHTML(vocab.Surface))
		if strings.TrimSpace(vocab.Reading) != "" && vocab.Reading != vocab.Surface {
			line += fmt.Sprintf(" (%s)", escapeHTML(vocab.Reading))
		}
		if strings.TrimSpace(vocab.MeaningKo) != "" {
			line += fmt.Sprintf(" — %s", escapeHTML(vocab.MeaningKo))
		}
		vocabLines = append(vocabLines, line)
	}
	if len(vocabLines) > 0 {
		sections = append(sections, "핵심 어휘:\n"+strings.Join(vocabLines, "\n"))
	}
	if len(sections) == 0 {
		return ""
	}
	return "\n\n" + strings.Join(sections, "\n\n")
}

func renderGenericPayload(payload json.RawMessage) string {
	if len(payload) == 0 || string(payload) == "null" {
		return ""
	}
	var out bytes.Buffer
	if err := json.Indent(&out, payload, "", "  "); err != nil {
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
