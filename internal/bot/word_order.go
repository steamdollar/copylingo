package bot

import (
	"context"
	"fmt"
	"hash/fnv"
	"math/rand"
	"strconv"
	"strings"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"

	"github.com/lsj/copylingo/internal/callback"
	"github.com/lsj/copylingo/internal/model"
)

func wordOrderShuffleOrder(
	sessionID,
	questionID,
	count int,
) []int {
	if count <= 0 {
		return nil
	}
	h := fnv.New64a()
	_, _ = h.Write([]byte(fmt.Sprintf(
		"%d:%d",
		sessionID,
		questionID,
	)))
	order := rand.New(rand.NewSource(int64(h.Sum64()))).Perm(count)
	return order
}

func validWordOrderSelection(
	selection []int,
	optionCount int,
) bool {
	seen := make(
		map[int]struct{},
		len(selection),
	)
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

func (sf *SessionFlow) getWordOrderDraft(
	ctx context.Context,
	sessionID,
	questionID,
	optionCount int,
) ([]int, error) {
	if sf.bot == nil || sf.bot.drafts == nil {
		return nil, nil
	}
	selection, err := sf.bot.drafts.GetWordOrderDraft(
		ctx,
		sessionID,
		questionID,
	)
	if err != nil {
		return nil, err
	}
	if !validWordOrderSelection(
		selection,
		optionCount,
	) {
		// A corrupt/stale draft must not make a question impossible to answer.
		_ = sf.bot.drafts.DeleteWordOrderDraft(
			ctx,
			sessionID,
			questionID,
		)
		return nil, nil
	}
	return selection, nil
}

func (sf *SessionFlow) setWordOrderDraft(
	ctx context.Context,
	sessionID,
	questionID int,
	selection []int,
) error {
	if sf.bot == nil || sf.bot.drafts == nil {
		return fmt.Errorf("word order draft redis is unavailable")
	}
	return sf.bot.drafts.SetWordOrderDraft(
		ctx,
		sessionID,
		questionID,
		selection,
	)
}

func (sf *SessionFlow) deleteWordOrderDraft(
	ctx context.Context,
	sessionID,
	questionID int,
) {
	if sf.bot != nil && sf.bot.drafts != nil {
		_ = sf.bot.drafts.DeleteWordOrderDraft(
			ctx,
			sessionID,
			questionID,
		)
	}
}

func wordOrderDisplayText(
	prompt string,
	options []string,
	selection []int,
) string {
	messages := botMessagesByLocale[botDefaultLocale]
	assembled := messages.wordOrderEmptySelection
	if len(selection) > 0 {
		chunks := make(
			[]string,
			0,
			len(selection),
		)
		for _, idx := range selection {
			if idx >= 0 && idx < len(options) {
				chunks = append(
					chunks,
					options[idx],
				)
			}
		}
		if len(chunks) > 0 {
			assembled = strings.Join(
				chunks,
				"",
			)
		}
	}
	return fmt.Sprintf(
		"%s%s <b>%s</b>",
		prompt,
		messages.wordOrderAssembledPrefix,
		assembled,
	)
}

func wordOrderKeyboard(
	sessionID,
	questionID int,
	options []string,
	selection []int,
) *tgbotapi.InlineKeyboardMarkup {
	selected := make(
		map[int]struct{},
		len(selection),
	)
	for _, idx := range selection {
		selected[idx] = struct{}{}
	}
	order := wordOrderShuffleOrder(
		sessionID,
		questionID,
		len(options),
	)
	rows := make(
		[][]tgbotapi.InlineKeyboardButton,
		0,
		len(options)+1,
	)
	for _, idx := range order {
		if _, ok := selected[idx]; ok {
			continue
		}
		rows = append(
			rows,
			tgbotapi.NewInlineKeyboardRow(
				tgbotapi.NewInlineKeyboardButtonData(
					options[idx],
					fmt.Sprintf(
						formatWordOrderSelect,
						sessionID,
						questionID,
						idx,
					),
				),
			),
		)
	}
	controls := tgbotapi.NewInlineKeyboardRow(
		tgbotapi.NewInlineKeyboardButtonData(
			botMessagesByLocale[botDefaultLocale].wordOrderUndoButton,
			fmt.Sprintf(
				formatWordOrderUndo,
				sessionID,
				questionID,
			),
		),
		tgbotapi.NewInlineKeyboardButtonData(
			botMessagesByLocale[botDefaultLocale].wordOrderResetButton,
			fmt.Sprintf(
				formatWordOrderReset,
				sessionID,
				questionID,
			),
		),
	)
	if len(selection) == len(options) {
		controls = append(
			controls,
			tgbotapi.NewInlineKeyboardButtonData(
				botMessagesByLocale[botDefaultLocale].wordOrderSubmitButton,
				fmt.Sprintf(
					formatWordOrderSubmit,
					sessionID,
					questionID,
				),
			),
		)
	}
	rows = append(
		rows,
		controls,
	)
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
	selection, err := sf.getWordOrderDraft(
		ctx,
		sessionID,
		question.ID,
		len(options),
	)
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
	sessionID,
	questionID int,
) (*model.QuizActiveSessionState, *model.QuizActiveSessionQuestion, bool) {
	if cb == nil || cb.From == nil || sf.bot == nil || sf.bot.services == nil ||
		sf.bot.services.QuizActiveSession == nil {
		return nil, nil, false
	}
	state, err := sf.bot.services.QuizActiveSession.Get(
		ctx,
		sessionID,
	)
	if err != nil || state.Session.UserID != cb.From.ID {
		return nil, nil, false
	}
	item, _, ok := state.CurrentItemByQuestionID(questionID)
	if !ok || item.Question.Type != model.QuestionWordOrder {
		return nil, nil, false
	}
	return state, item, true
}

func wordOrderUpdatedText(
	existing,
	prompt string,
	options []string,
	selection []int,
) string {
	if idx := strings.Index(
		existing,
		botMessagesByLocale[botDefaultLocale].wordOrderAssembledPrefix,
	); idx >= 0 {
		existing = existing[:idx]
	}
	if strings.TrimSpace(existing) == "" {
		existing = prompt
	}
	return existing + wordOrderDisplayText(
		"",
		options,
		selection,
	)
}

func (sf *SessionFlow) handleWordOrderCallback(
	ctx context.Context,
	cb *tgbotapi.CallbackQuery,
) {
	if cb == nil || cb.Message == nil {
		return
	}
	parts := strings.Split(
		cb.Data,
		":",
	)
	if len(parts) < 5 || parts[0] != callback.QuestionRoot || parts[2] != callbackActionWordOrder {
		return
	}
	sessionID, err1 := strconv.Atoi(parts[1])
	questionID, err2 := strconv.Atoi(parts[3])
	if err1 != nil || err2 != nil {
		return
	}
	_, item, ok := sf.wordOrderCurrentItem(
		ctx,
		cb,
		sessionID,
		questionID,
	)
	if !ok {
		return
	}
	action := parts[4]
	if item.SessionQuestion.IsCorrect != nil {
		if action == callbackActionWordOrderSubmit && len(parts) == 5 {
			sf.telegram.SendMessage(
				cb.Message.Chat.ID,
				botMessagesByLocale[botDefaultLocale].alreadyAnswered,
			)
		}
		return
	}
	options, err := item.Question.GetOptions()
	if err != nil || len(options) == 0 {
		return
	}
	selection, err := sf.getWordOrderDraft(
		ctx,
		sessionID,
		questionID,
		len(options),
	)
	if err != nil {
		return
	}

	switch action {
	case callbackActionWordOrderSelect:
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
		selection = append(
			selection,
			idx,
		)
	case callbackActionWordOrderUndo:
		if len(parts) != 5 || len(selection) == 0 {
			return
		}
		selection = selection[:len(selection)-1]
	case callbackActionWordOrderReset:
		if len(parts) != 5 {
			return
		}
		selection = nil
	case callbackActionWordOrderSubmit:
		if len(parts) != 5 || len(selection) != len(options) {
			return
		}
		answerParts := make(
			[]string,
			0,
			len(selection),
		)
		for _, idx := range selection {
			answerParts = append(
				answerParts,
				options[idx],
			)
		}
		answer := strings.Join(
			answerParts,
			"",
		)
		if item.SessionQuestion.IsCorrect != nil {
			sf.telegram.SendMessage(
				cb.Message.Chat.ID,
				botMessagesByLocale[botDefaultLocale].alreadyAnswered,
			)
			return
		}
		editMessageID := cb.Message.MessageID
		sf.processAnswerText(
			ctx,
			cb.Message.Chat.ID,
			cb.From,
			sessionID,
			questionID,
			answer,
			&editMessageID,
		)
		if state, _, valid := sf.wordOrderCurrentItem(
			ctx,
			cb,
			sessionID,
			questionID,
		); valid {
			if current, _, exists := state.CurrentItemByQuestionID(
				questionID,
			); exists &&
				current.SessionQuestion.IsCorrect != nil {
				sf.deleteWordOrderDraft(
					ctx,
					sessionID,
					questionID,
				)
			}
		}
		return
	default:
		return
	}
	if !validWordOrderSelection(
		selection,
		len(options),
	) {
		return
	}
	if action == callbackActionWordOrderReset {
		sf.deleteWordOrderDraft(
			ctx,
			sessionID,
			questionID,
		)
	} else {
		if err := sf.setWordOrderDraft(
			ctx,
			sessionID,
			questionID,
			selection,
		); err != nil {
			return
		}
	}
	text := wordOrderUpdatedText(
		cb.Message.Text,
		item.Question.Prompt,
		options,
		selection,
	)
	kb := wordOrderKeyboard(
		sessionID,
		questionID,
		options,
		selection,
	)
	if item.Question.MaterialID != nil && sf.bot.services.MaterialPreference != nil {
		kb.InlineKeyboard = append(
			kb.InlineKeyboard,
			tgbotapi.NewInlineKeyboardRow(
				tgbotapi.NewInlineKeyboardButtonData(
					botMessagesByLocale[botDefaultLocale].linkedMaterialSettingsButton,
					fmt.Sprintf(
						callback.FormatQuestionPolicy,
						sessionID,
						questionID,
					),
				),
			),
		)
	}
	sf.telegram.EditMessage(
		cb.Message.Chat.ID,
		cb.Message.MessageID,
		text,
		kb,
	)
}
