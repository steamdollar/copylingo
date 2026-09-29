package bot

import (
	"context"
	"fmt"
	"log/slog"
	"net/url"
	"strconv"
	"strings"
	"time"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"

	"github.com/lsj/copylingo/internal/callback"
	"github.com/lsj/copylingo/internal/config"
	"github.com/lsj/copylingo/internal/model"
)

func (sf *SessionFlow) showQuestion(
	ctx context.Context,
	chatID int64,
	editMessageID *int,
	sessionID,
	questionIdx int,
) {

	// get active session from redis
	state, err := sf.bot.services.QuizActiveSession.Get(
		ctx,
		sessionID,
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
		sf.showQuizActiveSessionUnavailable(
			chatID,
			editMessageID,
		)
		return
	}

	// All questions answered, show finish button
	if questionIdx >= len(state.Items) {
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
			sf.bot.EditMessage(
				chatID,
				*editMessageID,
				botMessagesByLocale[botDefaultLocale].questionCompleted,
				&keyboard,
			)
		} else {
			sf.bot.SendMessageWithKeyboard(
				chatID,
				botMessagesByLocale[botDefaultLocale].questionCompleted,
				keyboard,
			)
		}
		return
	}

	// set current question index at redis
	if err := sf.bot.services.QuizActiveSession.SetCurrentIndex(
		ctx,
		sessionID,
		questionIdx,
	); err != nil {
		slog.ErrorContext(
			ctx,
			"Failed to set active session index",
			"event",
			"telegram.question.index_update_failed",
			"session_id",
			sessionID,
			"question_index",
			questionIdx,
			"error",
			err,
		)
		sf.showQuizActiveSessionUnavailable(
			chatID,
			editMessageID,
		)
		return
	}

	item := state.Items[questionIdx]
	question := item.Question

	// TODO: err handling
	// render question text and keyboard by question type
	text, keyboard, done := sf.renderByType(
		ctx,
		chatID,
		editMessageID,
		sessionID,
		questionIdx,
		len(state.Items),
		question,
		item.SessionQuestion.IsReview,
	)
	if done {
		return
	}
	// Keep the current question intact; changing its linked material setting
	// only affects which questions future sessions may select.
	if question.MaterialID != nil && sf.bot.services.MaterialPreference != nil {
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

	if sf.bot.timing != nil {
		_ = sf.bot.timing.RecordQuestionStart(
			ctx,
			sessionID,
			time.Now(),
		)
	}

	// err handling after rendering question
	if editMessageID != nil {
		sf.bot.EditMessage(
			chatID,
			*editMessageID,
			text,
			keyboard,
		)
	} else {
		if keyboard != nil {
			sf.bot.SendMessageWithKeyboard(
				chatID,
				text,
				*keyboard,
			)
		} else {
			sf.bot.SendMessage(
				chatID,
				text,
			)
		}
	}
}

// renderByType builds the question text and keyboard for the given question type.
// Returns (text, keyboard, done): done=true means the message was already sent (or should be skipped) — caller must return.
func (sf *SessionFlow) renderByType(
	ctx context.Context,
	chatID int64,
	editMessageID *int,
	sessionID,
	questionIdx,
	totalQuestions int,
	question model.Question,
	isReview bool,
) (string, *tgbotapi.InlineKeyboardMarkup, bool) {
	messages := botMessagesByLocale[botDefaultLocale]
	reviewTag := ""
	if isReview {
		reviewTag = messages.reviewQuestionMarker
	}
	text := fmt.Sprintf(
		messages.questionFormat,
		questionIdx+1,
		totalQuestions,
		reviewTag,
		question.Prompt,
	)

	switch question.Type {
	case model.QuestionKanaHandwriting:
		// cells = answer 글자 수(촉음은 다음 글자와 같은 셀). 정답 문자열 자체는 cheat 방지를 위해 client로 보내지 않고, 길이만 전달해 캔버스 폭을 글자 수에 비례시킨다.
		cells := handwritingCellCount(question.CorrectAnswer)
		miniAppURL, err := sf.handwritingMiniAppURL(
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
			questionIdx,
			sf.bot.cfg.Server.PublicBaseURL,
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
		if question.MaterialID != nil && sf.bot.services != nil && sf.bot.services.MaterialPreference != nil {
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
			// Web App 버튼은 별도 메시지로 두는 편이 Mini App 왕복 흐름을 추적하기 쉽다.
			// 이전 메시지는 재사용하지 않고 짧은 안내 문구로 축약한다.
			sf.bot.EditMessage(
				chatID,
				*editMessageID,
				messages.handwritingSentNotice,
				nil,
			)
		}
		msgID, err := sf.bot.SendMessageWithReplyMarkup(
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
		if sf.bot.messages != nil {
			err := sf.bot.messages.SaveHandwritingMessage(
				ctx,
				sessionID,
				question.ID,
				model.TelegramMessageRef{ChatID: chatID, MessageID: msgID},
			)
			if err != nil {
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
		}
		return "", nil, true

	case model.QuestionListening:
		// Deliver the audio as a separate voice message, then render the
		// comprehension question + options through the shared MCQ path (grading
		// reuses the exact-match option flow — no new grader, ADR-031).
		if !sf.sendListeningAudio(
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
		return text, buildMCQKeyboard(
			sessionID,
			question.ID,
			options,
		), false

	case model.QuestionWordOrder:
		wordOrderText, keyboard, done := sf.renderWordOrder(
			ctx,
			sessionID,
			question,
		)
		if done {
			return "", nil, true
		}
		return text + "\n\n" + wordOrderText, keyboard, false

	case model.QuestionFillBlank, model.QuestionSubjective:
		if sf.bot.input != nil {
			_ = sf.bot.input.SetActiveQuestion(
				ctx,
				chatID,
				model.ActiveQuestionRef{SessionID: sessionID, QuestionIndex: questionIdx},
			)
		}
		if question.Type == model.QuestionSubjective {
			text += messages.freeTextAnswerPrompt
		} else {
			text += messages.chatAnswerPrompt
		}
		return text, nil, false

	default:
		options, err := question.GetOptions()
		if err != nil || len(options) == 0 {
			return "", nil, true
		}
		return text, buildMCQKeyboard(
			sessionID,
			question.ID,
			options,
		), false
	}
}

// handwritingCellCount returns the number of writing cells for a Japanese answer.
// Sokuon (small っ/ッ) is written within the following kana's cell, so it does
// not receive a separate slot in the handwriting pad.
func handwritingCellCount(answer string) int {
	cells := 0
	for _, r := range answer {
		if r == 'っ' || r == 'ッ' {
			continue
		}
		cells++
	}
	return cells
}

// buildMCQKeyboard uses one button per row when an option is too wide for a
// two-column layout. Shared by plain multiple-choice and listening comprehension.
func buildMCQKeyboard(
	sessionID,
	questionID int,
	options []string,
) *tgbotapi.InlineKeyboardMarkup {
	const maxTwoColumnOptionWidth = 16
	buttonsPerRow := 2
	for _, option := range options {
		width := 0
		for _, r := range option {
			width++
			if r > 127 { // Japanese and Korean characters take roughly twice the space.
				width++
			}
		}
		if width > maxTwoColumnOptionWidth {
			buttonsPerRow = 1
			break
		}
	}

	var rows [][]tgbotapi.InlineKeyboardButton
	for i := 0; i < len(options); i += buttonsPerRow {
		var row []tgbotapi.InlineKeyboardButton
		for j := i; j < i+buttonsPerRow && j < len(options); j++ {
			row = append(
				row,
				tgbotapi.NewInlineKeyboardButtonData(
					options[j],
					fmt.Sprintf(
						formatQuestionAnswer,
						sessionID,
						questionID,
						j,
					),
				),
			)
		}
		rows = append(
			rows,
			row,
		)
	}
	return &tgbotapi.InlineKeyboardMarkup{InlineKeyboard: rows}
}

// sendListeningAudio delivers a listening question's clip as a Telegram voice
// message. It prefers the cached file_id and falls back to fetching the object
// from the store and uploading it, caching the returned file_id (ADR-032).
// Returns false when audio is unavailable so the caller can degrade gracefully.
func (sf *SessionFlow) sendListeningAudio(
	ctx context.Context,
	chatID int64,
	q *model.Question,
) bool {
	audio := sf.bot.services.Audio
	if audio == nil || q.AudioPath == nil || *q.AudioPath == "" {
		return false
	}

	// Fast path: re-send by cached file_id (no store fetch, no re-upload).
	if q.AudioFileID != nil && *q.AudioFileID != "" {
		if err := sf.bot.SendVoiceFileID(
			chatID,
			*q.AudioFileID,
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
			q.ID,
		)
	}

	clip, err := audio.GetClip(
		ctx,
		*q.AudioPath,
	)
	if err != nil {
		slog.ErrorContext(
			ctx,
			"Failed to fetch listening clip from store",
			"event",
			"telegram.listening.fetch_failed",
			"question_id",
			q.ID,
			"key",
			*q.AudioPath,
			"error",
			err,
		)
		return false
	}

	fileID, err := sf.bot.SendVoiceBytes(
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
			q.ID,
			"error",
			err,
		)
		return false
	}

	if fileID != "" {
		if err := audio.CacheFileID(
			ctx,
			q.ID,
			fileID,
		); err != nil {
			slog.WarnContext(
				ctx,
				"Failed to cache listening voice file_id",
				"event",
				"telegram.listening.cache_failed",
				"question_id",
				q.ID,
				"error",
				err,
			)
		}
	}
	return true
}

func (sf *SessionFlow) isQuestionAnswered(
	ctx context.Context,
	sessionID,
	questionIdx int,
) bool {
	state, err := sf.bot.services.QuizActiveSession.Get(
		ctx,
		sessionID,
	)
	if err != nil || questionIdx < 0 || questionIdx >= len(state.Items) {
		return false
	}
	return state.Items[questionIdx].SessionQuestion.IsCorrect != nil
}

func (sf *SessionFlow) nextUnansweredQuestionIndex(
	ctx context.Context,
	sessionID int,
) (int, error) {
	state, err := sf.bot.services.QuizActiveSession.Get(
		ctx,
		sessionID,
	)
	if err != nil {
		return 0, err
	}

	return state.NextUnansweredIndex(), nil
}

// TODO: 이 함수 굳이 이렇게 복잡하게 짜야 함?
func (sf *SessionFlow) handwritingMiniAppURL(
	sessionID,
	questionID int,
	language,
	level,
	prompt string,
	cells int,
) (string, error) {
	baseURL := strings.TrimRight(
		sf.bot.cfg.Server.PublicBaseURL,
		"/",
	)
	if baseURL == "" {
		return "", fmt.Errorf("server public base url is empty")
	}
	u, err := url.Parse(baseURL + config.PathHandwritingMiniApp)
	if err != nil {
		return "", err
	}
	q := u.Query()
	q.Set(
		"session_id",
		strconv.Itoa(sessionID),
	)
	q.Set(
		"question_id",
		strconv.Itoa(questionID),
	)
	q.Set(
		"language",
		language,
	)
	q.Set(
		"level",
		level,
	)
	q.Set(
		"prompt",
		prompt,
	)
	if cells < 1 {
		cells = 1
	}
	q.Set(
		"cells",
		strconv.Itoa(cells),
	)
	u.RawQuery = q.Encode()
	return u.String(), nil
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
