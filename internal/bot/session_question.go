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

func (sf *SessionFlow) showQuestion(ctx context.Context, chatID int64,
	editMessageID *int, sessionID, questionIdx int) {

	// get active session from redis
	state, err := sf.bot.services.ActiveSession.Get(ctx, sessionID)
	if err != nil {
		slog.ErrorContext(ctx, "Failed to get active session state",
			"event", "telegram.question.session_lookup_failed",
			"session_id", sessionID,
			"error", err,
		)
		sf.showActiveSessionUnavailable(chatID, editMessageID)
		return
	}

	// All questions answered, show finish button
	if questionIdx >= len(state.Items) {
		keyboard := tgbotapi.NewInlineKeyboardMarkup(
			tgbotapi.NewInlineKeyboardRow(
				tgbotapi.NewInlineKeyboardButtonData("📊 결과 보기", fmt.Sprintf(config.FormatSessionFinish, sessionID)),
			),
		)
		if editMessageID != nil {
			sf.bot.EditMessage(chatID, *editMessageID, "✅ 모든 문제를 풀었습니다!", &keyboard)
		} else {
			sf.bot.SendMessageWithKeyboard(chatID, "✅ 모든 문제를 풀었습니다!", keyboard)
		}
		return
	}

	// set current question index at redis
	if err := sf.bot.services.ActiveSession.SetCurrentIndex(ctx, sessionID, questionIdx); err != nil {
		slog.ErrorContext(ctx, "Failed to set active session index",
			"event", "telegram.question.index_update_failed",
			"session_id", sessionID,
			"question_index", questionIdx,
			"error", err,
		)
		sf.showActiveSessionUnavailable(chatID, editMessageID)
		return
	}

	item := state.Items[questionIdx]
	question := item.Question

	// TODO: err handling
	// render question text and keyboard by question type
	text, keyboard, done := sf.renderByType(ctx,
		chatID, editMessageID,
		sessionID, questionIdx, len(state.Items),
		question,
		item.SessionQuestion.IsReview)
	if done {
		return
	}

	// record question start time at redis for session timeout handling
	sf.bot.rdb.Set(ctx,
		config.SessionQuestionStartRedisKey.Format(sessionID),
		time.Now().UnixMilli(), 30*time.Minute)

	// err handling after rendering question
	if editMessageID != nil {
		sf.bot.EditMessage(chatID, *editMessageID, text, keyboard)
	} else {
		if keyboard != nil {
			sf.bot.SendMessageWithKeyboard(chatID, text, *keyboard)
		} else {
			sf.bot.SendMessage(chatID, text)
		}
	}
}

// renderByType builds the question text and keyboard for the given question type.
// Returns (text, keyboard, done): done=true means the message was already sent (or should be skipped) — caller must return.
func (sf *SessionFlow) renderByType(ctx context.Context,
	chatID int64, editMessageID *int,
	sessionID, questionIdx, totalQuestions int,
	question model.Question,
	isReview bool) (string, *tgbotapi.InlineKeyboardMarkup, bool) {
	reviewTag := ""
	if isReview {
		reviewTag = " 🔄"
	}
	text := fmt.Sprintf("📝 <b>문제 %d/%d</b>%s\n\n%s", questionIdx+1, totalQuestions, reviewTag, question.Prompt)

	switch question.Type {
	case model.QuestionKanaHandwriting:
		// cells = answer 글자 수. 정답 문자열 자체는 cheat 방지를 위해 client로 보내지 않고, 길이만 전달해 캔버스 폭을 글자 수에 비례시킨다.
		cells := len([]rune(question.CorrectAnswer))
		miniAppURL, err := sf.handwritingMiniAppURL(sessionID, question.ID, question.Language, question.ProficiencyLevel, question.Prompt, cells)
		if err != nil {
			text += "\n\n⚠️ 손글씨 Mini App URL 설정이 필요합니다. `COPYLINGO_SERVER_PUBLIC_BASE_URL`을 설정해 주세요."
			return text, nil, false
		}
		nextData := callback.FormatHandwritingNext(sessionID, questionIdx, sf.bot.cfg.Server.PublicBaseURL)
		text += "\n\n✍️ 아래 버튼을 눌러 화면에 글자를 써 주세요.\n제출 후 이 채팅으로 돌아와 다음 문제를 진행하면 됩니다."
		replyMarkup := webAppKeyboardMarkup{
			InlineKeyboard: [][]webAppButton{
				{newWebAppButton("✍️ 손글씨로 답하기", miniAppURL)},
				{newCallbackButton("제출 후 다음 문제 →", nextData)},
			},
		}
		if editMessageID != nil {
			// Web App 버튼은 별도 메시지로 두는 편이 Mini App 왕복 흐름을 추적하기 쉽다.
			// 이전 메시지는 재사용하지 않고 짧은 안내 문구로 축약한다.
			sf.bot.EditMessage(chatID, *editMessageID, "✍️ 손글씨 문항을 새 메시지로 보냈습니다.", nil)
		}
		msgID, err := sf.bot.SendMessageWithReplyMarkup(chatID, text, replyMarkup)
		if err != nil {
			slog.ErrorContext(ctx, "Failed to send handwriting message",
				"event", "telegram.question.handwriting_send_failed",
				"chat_id", chatID,
				"session_id", sessionID,
				"question_id", question.ID,
				"error", err,
			)
			return "", nil, true
		}
		key := config.HandwritingMessageRedisKey.Format(sessionID, question.ID)
		val := fmt.Sprintf("%d:%d", chatID, msgID)
		if err := sf.bot.rdb.Set(ctx, key, val, time.Hour).Err(); err != nil {
			slog.ErrorContext(ctx, "Failed to cache handwriting message ID",
				"event", "telegram.question.handwriting_cache_failed",
				"session_id", sessionID,
				"question_id", question.ID,
				"error", err,
			)
		}
		return "", nil, true

	case model.QuestionFillBlank, model.QuestionSubjective:
		sf.bot.rdb.Set(ctx, config.UserActiveQuestionRedisKey.Format(chatID),
			fmt.Sprintf("%d:%d", sessionID, questionIdx), 1*time.Hour)
		if question.Type == model.QuestionSubjective {
			text += "\n\n⌨️ 정답을 자유롭게 텍스트로 입력해 주세요"
		} else {
			text += "\n\n⌨️ 채팅창에 답안을 입력해 주세요"
		}
		return text, nil, false

	default:
		options, err := question.GetOptions()
		if err != nil || len(options) == 0 {
			return "", nil, true
		}
		var rows [][]tgbotapi.InlineKeyboardButton
		for i := 0; i < len(options); i += 2 {
			var row []tgbotapi.InlineKeyboardButton
			row = append(row, tgbotapi.NewInlineKeyboardButtonData(
				options[i],
				fmt.Sprintf(config.FormatQuestionAnswer, sessionID, question.ID, i),
			))
			if i+1 < len(options) {
				row = append(row, tgbotapi.NewInlineKeyboardButtonData(
					options[i+1],
					fmt.Sprintf(config.FormatQuestionAnswer, sessionID, question.ID, i+1),
				))
			}
			rows = append(rows, row)
		}
		return text, &tgbotapi.InlineKeyboardMarkup{InlineKeyboard: rows}, false
	}
}

func (sf *SessionFlow) isQuestionAnswered(ctx context.Context, sessionID, questionIdx int) bool {
	state, err := sf.bot.services.ActiveSession.Get(ctx, sessionID)
	if err != nil || questionIdx < 0 || questionIdx >= len(state.Items) {
		return false
	}
	return state.Items[questionIdx].SessionQuestion.IsCorrect != nil
}

func (sf *SessionFlow) nextUnansweredQuestionIndex(ctx context.Context, sessionID int) (int, error) {
	state, err := sf.bot.services.ActiveSession.Get(ctx, sessionID)
	if err != nil {
		return 0, err
	}

	return state.NextUnansweredIndex(), nil
}

// TODO: 이 함수 굳이 이렇게 복잡하게 짜야 함?
func (sf *SessionFlow) handwritingMiniAppURL(sessionID, questionID int, language, level, prompt string, cells int) (string, error) {
	baseURL := strings.TrimRight(sf.bot.cfg.Server.PublicBaseURL, "/")
	if baseURL == "" {
		return "", fmt.Errorf("server public base url is empty")
	}
	u, err := url.Parse(baseURL + config.PathHandwritingMiniApp)
	if err != nil {
		return "", err
	}
	q := u.Query()
	q.Set("session_id", strconv.Itoa(sessionID))
	q.Set("question_id", strconv.Itoa(questionID))
	q.Set("language", language)
	q.Set("level", level)
	q.Set("prompt", prompt)
	if cells < 1 {
		cells = 1
	}
	q.Set("cells", strconv.Itoa(cells))
	u.RawQuery = q.Encode()
	return u.String(), nil
}

func isStaleMiniAppCallback(parts []string, currentPublicBaseURL string) bool {
	return callback.IsStaleMiniAppCallback(parts, currentPublicBaseURL)
}
