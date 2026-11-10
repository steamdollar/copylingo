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
	hasher := fnv.New64a()
	_, _ = hasher.Write([]byte(fmt.Sprintf(
		"%d:%d",
		sessionID,
		questionID,
	)))
	order := rand.New(rand.NewSource(int64(hasher.Sum64()))).Perm(count)
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
	for _, index := range selection {
		if index < 0 || index >= optionCount {
			return false
		}
		if _, ok := seen[index]; ok {
			return false
		}
		seen[index] = struct{}{}
	}
	return true
}

func (flow *SessionFlow) getWordOrderDraft(
	ctx context.Context,
	sessionID,
	questionID,
	optionCount int,
) ([]int, error) {
	selection, err := flow.drafts.GetWordOrderDraft(
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
		_ = flow.drafts.DeleteWordOrderDraft(
			ctx,
			sessionID,
			questionID,
		)
		return nil, nil
	}
	return selection, nil
}

func (flow *SessionFlow) setWordOrderDraft(
	ctx context.Context,
	sessionID,
	questionID int,
	selection []int,
) error {
	return flow.drafts.SetWordOrderDraft(
		ctx,
		sessionID,
		questionID,
		selection,
	)
}

func (flow *SessionFlow) deleteWordOrderDraft(
	ctx context.Context,
	sessionID,
	questionID int,
) {
	_ = flow.drafts.DeleteWordOrderDraft(
		ctx,
		sessionID,
		questionID,
	)
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
		for _, index := range selection {
			if index >= 0 && index < len(options) {
				chunks = append(
					chunks,
					options[index],
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
	for _, index := range selection {
		selected[index] = struct{}{}
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
	for _, index := range order {
		if _, ok := selected[index]; ok {
			continue
		}
		rows = append(
			rows,
			tgbotapi.NewInlineKeyboardRow(
				tgbotapi.NewInlineKeyboardButtonData(
					options[index],
					fmt.Sprintf(
						formatWordOrderSelect,
						sessionID,
						questionID,
						index,
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

func (flow *SessionFlow) renderWordOrder(
	ctx context.Context,
	sessionID int,
	question model.Question,
) (string, *tgbotapi.InlineKeyboardMarkup, bool) {
	options, err := question.GetOptions()
	if err != nil || len(options) == 0 {
		return "", nil, true
	}
	selection, err := flow.getWordOrderDraft(
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

func (flow *SessionFlow) wordOrderCurrentItem(
	ctx context.Context,
	callbackQuery *tgbotapi.CallbackQuery,
	sessionID,
	questionID int,
) (*model.QuizActiveSessionState, *model.QuizActiveSessionQuestion, bool) {
	if callbackQuery == nil || callbackQuery.From == nil {
		return nil, nil, false
	}
	state, err := flow.session.QuizProgress(
		ctx,
		sessionID,
	)
	if err != nil || state.Session.UserID != callbackQuery.From.ID {
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
	if index := strings.Index(
		existing,
		botMessagesByLocale[botDefaultLocale].wordOrderAssembledPrefix,
	); index >= 0 {
		existing = existing[:index]
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

func (flow *SessionFlow) handleWordOrderCallback(
	ctx context.Context,
	callbackQuery *tgbotapi.CallbackQuery,
) {
	if callbackQuery == nil || callbackQuery.Message == nil {
		return
	}
	parts := strings.Split(
		callbackQuery.Data,
		":",
	)
	if len(parts) < 5 || parts[0] != callback.QuestionRoot || parts[2] != callbackActionWordOrder {
		return
	}
	sessionID, sessionIDErr := strconv.Atoi(parts[1])
	questionID, questionIDErr := strconv.Atoi(parts[3])
	if sessionIDErr != nil || questionIDErr != nil {
		return
	}
	_, item, ok := flow.wordOrderCurrentItem(
		ctx,
		callbackQuery,
		sessionID,
		questionID,
	)
	if !ok {
		return
	}
	action := parts[4]
	if item.SessionQuestion.IsCorrect != nil {
		if action == callbackActionWordOrderSubmit && len(parts) == 5 {
			flow.telegram.SendMessage(
				callbackQuery.Message.Chat.ID,
				botMessagesByLocale[botDefaultLocale].alreadyAnswered,
			)
		}
		return
	}
	options, err := item.Question.GetOptions()
	if err != nil || len(options) == 0 {
		return
	}
	selection, err := flow.getWordOrderDraft(
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
		index, err := strconv.Atoi(parts[5])
		if err != nil || index < 0 || index >= len(options) {
			return
		}
		for _, selected := range selection {
			if selected == index {
				return
			}
		}
		selection = append(
			selection,
			index,
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
		if item.SessionQuestion.IsCorrect != nil {
			flow.telegram.SendMessage(
				callbackQuery.Message.Chat.ID,
				botMessagesByLocale[botDefaultLocale].alreadyAnswered,
			)
			return
		}
		editMessageID := callbackQuery.Message.MessageID
		result, err := flow.session.SubmitQuizWordOrder(
			ctx,
			callbackQuery.From.ID,
			sessionID,
			questionID,
			selection,
		)
		if err != nil {
			flow.handleQuizAnswerError(
				ctx,
				callbackQuery.Message.Chat.ID,
				sessionID,
				&editMessageID,
				err,
			)
		} else {
			flow.renderQuizAnswerResult(
				callbackQuery.Message.Chat.ID,
				callbackQuery.From,
				sessionID,
				result,
				&editMessageID,
			)
		}
		if state, _, valid := flow.wordOrderCurrentItem(
			ctx,
			callbackQuery,
			sessionID,
			questionID,
		); valid {
			if current, _, exists := state.CurrentItemByQuestionID(
				questionID,
			); exists &&
				current.SessionQuestion.IsCorrect != nil {
				flow.deleteWordOrderDraft(
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
		flow.deleteWordOrderDraft(
			ctx,
			sessionID,
			questionID,
		)
	} else {
		if err := flow.setWordOrderDraft(
			ctx,
			sessionID,
			questionID,
			selection,
		); err != nil {
			return
		}
	}
	text := wordOrderUpdatedText(
		callbackQuery.Message.Text,
		item.Question.Prompt,
		options,
		selection,
	)
	keyboard := wordOrderKeyboard(
		sessionID,
		questionID,
		options,
		selection,
	)
	if item.Question.MaterialID != nil {
		keyboard.InlineKeyboard = append(
			keyboard.InlineKeyboard,
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
	flow.telegram.EditMessage(
		callbackQuery.Message.Chat.ID,
		callbackQuery.Message.MessageID,
		text,
		keyboard,
	)
}
