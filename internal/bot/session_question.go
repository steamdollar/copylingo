package bot

import (
	"context"
	"fmt"
	"log/slog"
	"net/url"
	"strconv"
	"strings"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"

	"github.com/lsj/copylingo/internal/callback"
	"github.com/lsj/copylingo/internal/config"
	"github.com/lsj/copylingo/internal/model"
)

func (flow *SessionFlow) showQuestion(
	ctx context.Context,
	chatID int64,
	editMessageID *int,
	sessionID,
	questionIndex int,
) {
	// Loads the working set and moves its cursor to questionIndex in one call.
	state, err := flow.session.ShowQuizQuestion(
		ctx,
		sessionID,
		questionIndex,
	)
	if err != nil {
		slog.ErrorContext(
			ctx,
			"Failed to get active session state",
			"event",
			"telegram.question.session_lookup_failed",
			"session_id",
			sessionID,
			"error",
			err,
		)
		flow.showQuizActiveSessionUnavailable(
			chatID,
			editMessageID,
		)
		return
	}

	if questionIndex >= len(state.Items) {
		keyboard := tgbotapi.NewInlineKeyboardMarkup(
			tgbotapi.NewInlineKeyboardRow(
				tgbotapi.NewInlineKeyboardButtonData(
					botMessagesByLocale[botDefaultLocale].resultsButton,
					fmt.Sprintf(
						formatSessionFinish,
						sessionID,
					),
				),
			),
		)
		if editMessageID != nil {
			flow.telegram.EditMessage(
				chatID,
				*editMessageID,
				botMessagesByLocale[botDefaultLocale].questionCompleted,
				&keyboard,
			)
		} else {
			flow.telegram.SendMessageWithKeyboard(
				chatID,
				botMessagesByLocale[botDefaultLocale].questionCompleted,
				keyboard,
			)
		}
		return
	}

	item := state.Items[questionIndex]
	question := item.Question

	// TODO: err handling
	text, keyboard, done := flow.renderByType(
		ctx,
		chatID,
		editMessageID,
		sessionID,
		questionIndex,
		len(state.Items),
		question,
		item.SessionQuestion.IsReview,
	)
	if done {
		return
	}
	// Keep the current question intact. The material setting only affects which questions future sessions may select.
	if question.MaterialID != nil {
		if keyboard == nil {
			keyboard = &tgbotapi.InlineKeyboardMarkup{}
		}
		keyboard.InlineKeyboard = append(
			keyboard.InlineKeyboard,
			tgbotapi.NewInlineKeyboardRow(
				tgbotapi.NewInlineKeyboardButtonData(
					botMessagesByLocale[botDefaultLocale].linkedMaterialSettingsButton,
					fmt.Sprintf(
						callback.FormatQuestionPolicy,
						sessionID,
						question.ID,
					),
				),
			),
		)
	}

	if editMessageID != nil {
		flow.telegram.EditMessage(
			chatID,
			*editMessageID,
			text,
			keyboard,
		)
	} else {
		if keyboard != nil {
			flow.telegram.SendMessageWithKeyboard(
				chatID,
				text,
				*keyboard,
			)
		} else {
			flow.telegram.SendMessage(
				chatID,
				text,
			)
		}
	}
}

// done=true means the message was already sent (or should be skipped), so the caller must return.
func (flow *SessionFlow) renderByType(
	ctx context.Context,
	chatID int64,
	editMessageID *int,
	sessionID,
	questionIndex,
	totalQuestions int,
	question model.Question,
	isReview bool,
) (string, *tgbotapi.InlineKeyboardMarkup, bool) {
	messages := botMessagesByLocale[botDefaultLocale]
	// Sessions mix adjacent levels (ADR-046), so the header names each question's level when it has one.
	levelTag := ""
	if question.ProficiencyLevel != "" {
		levelTag = fmt.Sprintf(
			messages.questionLevelMarkerFormat,
			question.ProficiencyLevel,
		)
	}
	reviewTag := ""
	if isReview {
		reviewTag = messages.reviewQuestionMarker
	}
	text := fmt.Sprintf(
		messages.questionFormat,
		questionIndex+1,
		totalQuestions,
		levelTag,
		reviewTag,
		question.Prompt,
	)

	switch question.Type {
	case model.QuestionKanaHandwriting:
		// cells = 정답 글자 수(촉음은 다음 글자와 같은 셀). 정답 문자열은 cheat 방지를 위해 client로 보내지 않고 길이만 전달한다.
		cells := handwritingCellCount(question.CorrectAnswer)
		miniAppURL, err := flow.handwritingMiniAppURL(
			sessionID,
			question.ID,
			question.Language,
			question.ProficiencyLevel,
			question.Prompt,
			cells,
		)
		if err != nil {
			text += messages.handwritingURLUnavailable
			return text, nil, false
		}
		nextData := callback.FormatHandwritingNext(
			sessionID,
			questionIndex,
			flow.publicBaseURL,
		)
		text += messages.handwritingPrompt
		replyMarkup := webAppKeyboardMarkup{
			InlineKeyboard: [][]webAppButton{
				{newWebAppButton(
					messages.handwritingAnswerButton,
					miniAppURL,
				)},
				{newCallbackButton(
					messages.handwritingNextButton,
					nextData,
				)},
			},
		}
		if question.MaterialID != nil {
			replyMarkup.InlineKeyboard = append(
				replyMarkup.InlineKeyboard,
				[]webAppButton{
					newCallbackButton(
						messages.linkedMaterialSettingsButton,
						fmt.Sprintf(
							callback.FormatQuestionPolicy,
							sessionID,
							question.ID,
						),
					),
				},
			)
		}
		if editMessageID != nil {
			// Web App 버튼은 별도 메시지로 두고, 이전 메시지는 짧은 안내 문구로 축약한다.
			flow.telegram.EditMessage(
				chatID,
				*editMessageID,
				messages.handwritingSentNotice,
				nil,
			)
		}
		messageID, err := flow.telegram.SendMessageWithReplyMarkup(
			chatID,
			text,
			replyMarkup,
		)
		if err != nil {
			slog.ErrorContext(
				ctx,
				"Failed to send handwriting message",
				"event",
				"telegram.question.handwriting_send_failed",
				"chat_id",
				chatID,
				"session_id",
				sessionID,
				"question_id",
				question.ID,
				"error",
				err,
			)
			return "", nil, true
		}
		if err := flow.messages.SaveHandwritingMessage(
			ctx,
			sessionID,
			question.ID,
			model.TelegramMessageReference{ChatID: chatID, MessageID: messageID},
		); err != nil {
			slog.ErrorContext(
				ctx,
				"Failed to cache handwriting message ID",
				"event",
				"telegram.question.handwriting_cache_failed",
				"session_id",
				sessionID,
				"question_id",
				question.ID,
				"error",
				err,
			)
		}
		return "", nil, true

	case model.QuestionListening:
		// Voice message first, then the question via the shared MCQ path. Grading reuses the option flow (ADR-031).
		if !flow.sendListeningAudio(
			ctx,
			chatID,
			&question,
		) {
			text += messages.listeningAudioUnavailable
			return text, nil, false
		}
		text += messages.listeningAnswerPrompt
		options, err := question.GetOptions()
		if err != nil || len(options) == 0 {
			return "", nil, true
		}
		optionList, keyboard := buildMCQ(
			sessionID,
			question.ID,
			options,
		)
		return text + optionList, keyboard, false

	case model.QuestionWordOrder:
		wordOrderText, keyboard, done := flow.renderWordOrder(
			ctx,
			sessionID,
			question,
		)
		if done {
			return "", nil, true
		}
		return text + "\n\n" + wordOrderText, keyboard, false

	case model.QuestionFillBlank:
		_ = flow.input.SetActiveQuestion(
			ctx,
			chatID,
			model.ActiveQuestionReference{SessionID: sessionID, QuestionIndex: questionIndex},
		)
		text += messages.chatAnswerPrompt
		return text, nil, false

	default:
		options, err := question.GetOptions()
		if err != nil || len(options) == 0 {
			return "", nil, true
		}
		optionList, keyboard := buildMCQ(
			sessionID,
			question.ID,
			options,
		)
		return text + optionList, keyboard, false
	}
}

// Sokuon (small っ/ッ) shares the following kana's cell.
func handwritingCellCount(answer string) int {
	cells := 0
	for _, character := range answer {
		if character == 'っ' || character == 'ッ' {
			continue
		}
		cells++
	}
	return cells
}

// Short options use a two-column keyboard and optionList is empty. Telegram truncates long button labels,
// so wide options are listed in the message text and the buttons carry only numbers.
func buildMCQ(
	sessionID,
	questionID int,
	options []string,
) (string, *tgbotapi.InlineKeyboardMarkup) {
	const maxTwoColumnOptionWidth = 16
	fitsTwoColumns := true
	for _, option := range options {
		width := 0
		for _, character := range option {
			width++
			if character > 127 { // Japanese and Korean characters take roughly twice the space.
				width++
			}
		}
		if width > maxTwoColumnOptionWidth {
			fitsTwoColumns = false
			break
		}
	}
	answerButton := func(
		label string,
		optionIndex int,
	) tgbotapi.InlineKeyboardButton {
		return tgbotapi.NewInlineKeyboardButtonData(
			label,
			fmt.Sprintf(
				formatQuestionAnswer,
				sessionID,
				questionID,
				optionIndex,
			),
		)
	}

	if !fitsTwoColumns {
		var optionList strings.Builder
		optionList.WriteString("\n")
		numberRow := make(
			[]tgbotapi.InlineKeyboardButton,
			0,
			len(options),
		)
		for i, option := range options {
			number := strconv.Itoa(i + 1)
			fmt.Fprintf(
				&optionList,
				"\n%s. %s",
				number,
				escapeHTML(option),
			)
			numberRow = append(
				numberRow,
				answerButton(
					number,
					i,
				),
			)
		}
		return optionList.String(), &tgbotapi.InlineKeyboardMarkup{
			InlineKeyboard: [][]tgbotapi.InlineKeyboardButton{numberRow},
		}
	}

	var rows [][]tgbotapi.InlineKeyboardButton
	for i := 0; i < len(options); i += 2 {
		var row []tgbotapi.InlineKeyboardButton
		for j := i; j < i+2 && j < len(options); j++ {
			row = append(
				row,
				answerButton(
					options[j],
					j,
				),
			)
		}
		rows = append(
			rows,
			row,
		)
	}
	return "", &tgbotapi.InlineKeyboardMarkup{InlineKeyboard: rows}
}

// Prefers the cached file_id, else uploads from the store and caches the new one (ADR-032).
// False if audio is unavailable.
func (flow *SessionFlow) sendListeningAudio(
	ctx context.Context,
	chatID int64,
	question *model.Question,
) bool {
	if question.AudioPath == nil || *question.AudioPath == "" {
		return false
	}

	if question.AudioFileID != nil && *question.AudioFileID != "" {
		if err := flow.telegram.SendVoiceFileID(
			chatID,
			*question.AudioFileID,
		); err == nil {
			return true
		}
		// A purged/invalid file_id falls through to a fresh upload (ADR-032).
		slog.WarnContext(
			ctx,
			"Cached voice file_id failed; re-uploading from store",
			"event",
			"telegram.listening.file_id_stale",
			"question_id",
			question.ID,
		)
	}

	clip, err := flow.audio.GetClip(
		ctx,
		*question.AudioPath,
	)
	if err != nil {
		slog.ErrorContext(
			ctx,
			"Failed to fetch listening clip from store",
			"event",
			"telegram.listening.fetch_failed",
			"question_id",
			question.ID,
			"key",
			*question.AudioPath,
			"error",
			err,
		)
		return false
	}

	fileID, err := flow.telegram.SendVoiceBytes(
		chatID,
		clip,
	)
	if err != nil {
		slog.ErrorContext(
			ctx,
			"Failed to send listening voice",
			"event",
			"telegram.listening.send_failed",
			"question_id",
			question.ID,
			"error",
			err,
		)
		return false
	}

	if fileID != "" {
		if err := flow.audio.CacheFileID(
			ctx,
			question.ID,
			fileID,
		); err != nil {
			slog.WarnContext(
				ctx,
				"Failed to cache listening voice file_id",
				"event",
				"telegram.listening.cache_failed",
				"question_id",
				question.ID,
				"error",
				err,
			)
		}
	}
	return true
}

func (flow *SessionFlow) isQuestionAnswered(
	ctx context.Context,
	sessionID,
	questionIndex int,
) bool {
	state, err := flow.session.QuizProgress(
		ctx,
		sessionID,
	)
	if err != nil || questionIndex < 0 || questionIndex >= len(state.Items) {
		return false
	}
	return state.Items[questionIndex].SessionQuestion.IsCorrect != nil
}

func (flow *SessionFlow) nextUnansweredQuestionIndex(
	ctx context.Context,
	sessionID int,
) (int, error) {
	state, err := flow.session.QuizProgress(
		ctx,
		sessionID,
	)
	if err != nil {
		return 0, err
	}

	return state.NextUnansweredIndex(), nil
}

// TODO: 이 함수 굳이 이렇게 복잡하게 짜야 함?
func (flow *SessionFlow) handwritingMiniAppURL(
	sessionID,
	questionID int,
	language,
	level,
	prompt string,
	cells int,
) (string, error) {
	baseURL := strings.TrimRight(
		flow.publicBaseURL,
		"/",
	)
	if baseURL == "" {
		return "", fmt.Errorf("server public base url is empty")
	}
	parsedURL, err := url.Parse(baseURL + config.PathHandwritingMiniApp)
	if err != nil {
		return "", err
	}
	query := parsedURL.Query()
	query.Set(
		"session_id",
		strconv.Itoa(sessionID),
	)
	query.Set(
		"question_id",
		strconv.Itoa(questionID),
	)
	query.Set(
		"language",
		language,
	)
	query.Set(
		"level",
		level,
	)
	query.Set(
		"prompt",
		prompt,
	)
	if cells < 1 {
		cells = 1
	}
	query.Set(
		"cells",
		strconv.Itoa(cells),
	)
	parsedURL.RawQuery = query.Encode()
	return parsedURL.String(), nil
}

func isStaleMiniAppCallback(
	parts []string,
	currentPublicBaseURL string,
) bool {
	return callback.IsStaleMiniAppCallback(
		parts,
		currentPublicBaseURL,
	)
}
