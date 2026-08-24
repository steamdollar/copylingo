package bot

import (
	"context"
	"encoding/json"
	"fmt"
	"hash/fnv"
	"math/rand"
	"strconv"
	"strings"
	"time"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
	"github.com/redis/go-redis/v9"

	"github.com/lsj/copylingo/internal/config"
	"github.com/lsj/copylingo/internal/model"
)

// Keep drafts alive for exactly as long as the active-session working set.
// Drafts are intentionally separate keys so a tap does not rewrite the full
// ActiveSessionState blob.
const wordOrderDraftTTL = 24 * time.Hour

func wordOrderDraftKey(sessionID, questionID int) string {
	return config.WordOrderDraftRedisKey.Format(sessionID, questionID)
}

func wordOrderShuffleOrder(sessionID, questionID, count int) []int {
	if count <= 0 {
		return nil
	}
	h := fnv.New64a()
	_, _ = h.Write([]byte(fmt.Sprintf("%d:%d", sessionID, questionID)))
	order := rand.New(rand.NewSource(int64(h.Sum64()))).Perm(count)
	return order
}

func validWordOrderSelection(selection []int, optionCount int) bool {
	seen := make(map[int]struct{}, len(selection))
	for _, idx := range selection {
		if idx < 0 || idx >= optionCount {
			return false
		}
		if _, ok := seen[idx]; ok {
			return false
		}
		seen[idx] = struct{}{}
	}
	return true
}

func (sf *SessionFlow) getWordOrderDraft(ctx context.Context, sessionID, questionID, optionCount int) ([]int, error) {
	if sf.bot == nil || sf.bot.rdb == nil {
		return nil, nil
	}
	raw, err := sf.bot.rdb.Get(ctx, wordOrderDraftKey(sessionID, questionID)).Result()
	if err != nil {
		if err == redis.Nil {
			return nil, nil
		}
		return nil, err
	}
	var selection []int
	if err := json.Unmarshal([]byte(raw), &selection); err != nil || !validWordOrderSelection(selection, optionCount) {
		// A corrupt/stale draft must not make a question impossible to answer.
		_ = sf.bot.rdb.Del(ctx, wordOrderDraftKey(sessionID, questionID)).Err()
		return nil, nil
	}
	return selection, nil
}

func (sf *SessionFlow) setWordOrderDraft(ctx context.Context, sessionID, questionID int, selection []int) error {
	if sf.bot == nil || sf.bot.rdb == nil {
		return fmt.Errorf("word order draft redis is unavailable")
	}
	raw, err := json.Marshal(selection)
	if err != nil {
		return err
	}
	return sf.bot.rdb.Set(ctx, wordOrderDraftKey(sessionID, questionID), raw, wordOrderDraftTTL).Err()
}

func (sf *SessionFlow) deleteWordOrderDraft(ctx context.Context, sessionID, questionID int) {
	if sf.bot != nil && sf.bot.rdb != nil {
		_ = sf.bot.rdb.Del(ctx, wordOrderDraftKey(sessionID, questionID)).Err()
	}
}

func wordOrderDisplayText(prompt string, options []string, selection []int) string {
	assembled := "(아직 선택한 조각 없음)"
	if len(selection) > 0 {
		chunks := make([]string, 0, len(selection))
		for _, idx := range selection {
			if idx >= 0 && idx < len(options) {
				chunks = append(chunks, options[idx])
			}
		}
		if len(chunks) > 0 {
			assembled = strings.Join(chunks, "")
		}
	}
	return fmt.Sprintf("%s\n\n🧩 조립: <b>%s</b>", prompt, assembled)
}

func wordOrderKeyboard(sessionID, questionID int, options []string, selection []int) *tgbotapi.InlineKeyboardMarkup {
	selected := make(map[int]struct{}, len(selection))
	for _, idx := range selection {
		selected[idx] = struct{}{}
	}
	order := wordOrderShuffleOrder(sessionID, questionID, len(options))
	rows := make([][]tgbotapi.InlineKeyboardButton, 0, len(options)+1)
	for _, idx := range order {
		if _, ok := selected[idx]; ok {
			continue
		}
		rows = append(rows, tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData(
				options[idx],
				fmt.Sprintf(config.FormatWordOrderSelect, sessionID, questionID, idx),
			),
		))
	}
	controls := tgbotapi.NewInlineKeyboardRow(
		tgbotapi.NewInlineKeyboardButtonData("↩️ 되돌리기", fmt.Sprintf(config.FormatWordOrderUndo, sessionID, questionID)),
		tgbotapi.NewInlineKeyboardButtonData("초기화", fmt.Sprintf(config.FormatWordOrderReset, sessionID, questionID)),
	)
	if len(selection) == len(options) {
		controls = append(
			controls,
			tgbotapi.NewInlineKeyboardButtonData(
				"제출",
				fmt.Sprintf(config.FormatWordOrderSubmit, sessionID, questionID),
			),
		)
	}
	rows = append(rows, controls)
	return &tgbotapi.InlineKeyboardMarkup{InlineKeyboard: rows}
}

func (sf *SessionFlow) renderWordOrder(
	ctx context.Context,
	sessionID int,
	question model.Question,
) (string, *tgbotapi.InlineKeyboardMarkup, bool) {
	options, err := question.GetOptions()
	if err != nil || len(options) == 0 {
		return "", nil, true
	}
	selection, err := sf.getWordOrderDraft(ctx, sessionID, question.ID, len(options))
	if err != nil {
		return "", nil, true
	}
	return wordOrderDisplayText(
			question.Prompt,
			options,
			selection,
		), wordOrderKeyboard(
			sessionID,
			question.ID,
			options,
			selection,
		), false
}

func (sf *SessionFlow) wordOrderCurrentItem(
	ctx context.Context,
	cb *tgbotapi.CallbackQuery,
	sessionID, questionID int,
) (*model.ActiveSessionState, *model.ActiveSessionQuestion, bool) {
	if cb == nil || cb.From == nil || sf.bot == nil || sf.bot.services == nil || sf.bot.services.ActiveSession == nil {
		return nil, nil, false
	}
	state, err := sf.bot.services.ActiveSession.Get(ctx, sessionID)
	if err != nil || state.Session.UserID != cb.From.ID {
		return nil, nil, false
	}
	item, _, ok := state.CurrentItemByQuestionID(questionID)
	if !ok || item.Question.Type != model.QuestionWordOrder {
		return nil, nil, false
	}
	return state, item, true
}

func wordOrderUpdatedText(existing, prompt string, options []string, selection []int) string {
	if idx := strings.Index(existing, "\n\n🧩 조립:"); idx >= 0 {
		existing = existing[:idx]
	}
	if strings.TrimSpace(existing) == "" {
		existing = prompt
	}
	return existing + wordOrderDisplayText("", options, selection)
}

func (sf *SessionFlow) handleWordOrderCallback(ctx context.Context, cb *tgbotapi.CallbackQuery) {
	if cb == nil || cb.Message == nil {
		return
	}
	parts := strings.Split(cb.Data, ":")
	if len(parts) < 5 || parts[0] != "q" || parts[2] != "wo" {
		return
	}
	sessionID, err1 := strconv.Atoi(parts[1])
	questionID, err2 := strconv.Atoi(parts[3])
	if err1 != nil || err2 != nil {
		return
	}
	_, item, ok := sf.wordOrderCurrentItem(ctx, cb, sessionID, questionID)
	if !ok {
		return
	}
	action := parts[4]
	if item.SessionQuestion.IsCorrect != nil {
		if action == "s" && len(parts) == 5 {
			sf.bot.SendMessage(cb.Message.Chat.ID, "이미 답변한 문제입니다.")
		}
		return
	}
	options, err := item.Question.GetOptions()
	if err != nil || len(options) == 0 {
		return
	}
	selection, err := sf.getWordOrderDraft(ctx, sessionID, questionID, len(options))
	if err != nil {
		return
	}

	switch action {
	case "a":
		if len(parts) != 6 {
			return
		}
		idx, err := strconv.Atoi(parts[5])
		if err != nil || idx < 0 || idx >= len(options) {
			return
		}
		for _, selected := range selection {
			if selected == idx {
				return
			}
		}
		selection = append(selection, idx)
	case "u":
		if len(parts) != 5 || len(selection) == 0 {
			return
		}
		selection = selection[:len(selection)-1]
	case "r":
		if len(parts) != 5 {
			return
		}
		selection = nil
	case "s":
		if len(parts) != 5 || len(selection) != len(options) {
			return
		}
		answerParts := make([]string, 0, len(selection))
		for _, idx := range selection {
			answerParts = append(answerParts, options[idx])
		}
		answer := strings.Join(answerParts, "")
		if item.SessionQuestion.IsCorrect != nil {
			sf.bot.SendMessage(cb.Message.Chat.ID, "이미 답변한 문제입니다.")
			return
		}
		editMessageID := cb.Message.MessageID
		sf.processAnswerText(ctx, cb.Message.Chat.ID, cb.From, sessionID, questionID, answer, &editMessageID)
		if state, _, valid := sf.wordOrderCurrentItem(ctx, cb, sessionID, questionID); valid {
			if current, _, exists := state.CurrentItemByQuestionID(
				questionID,
			); exists &&
				current.SessionQuestion.IsCorrect != nil {
				sf.deleteWordOrderDraft(ctx, sessionID, questionID)
			}
		}
		return
	default:
		return
	}
	if !validWordOrderSelection(selection, len(options)) {
		return
	}
	if action == "r" {
		sf.deleteWordOrderDraft(ctx, sessionID, questionID)
	} else {
		if err := sf.setWordOrderDraft(ctx, sessionID, questionID, selection); err != nil {
			return
		}
	}
	text := wordOrderUpdatedText(cb.Message.Text, item.Question.Prompt, options, selection)
	kb := wordOrderKeyboard(sessionID, questionID, options, selection)
	sf.bot.EditMessage(cb.Message.Chat.ID, cb.Message.MessageID, text, kb)
}
