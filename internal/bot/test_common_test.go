package bot

import (
	"context"
	"fmt"
	"time"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
	"github.com/redis/go-redis/v9"

	"github.com/lsj/copylingo/internal/external"
	"github.com/lsj/copylingo/internal/model"
	"github.com/lsj/copylingo/internal/service"
)

type mockBotAPI struct {
	sentMessages []tgbotapi.Chattable
	sendErr      error
	// returnVoiceFileID, when set, is echoed back as the file_id of a sent voice
	// message so file_id-caching paths can be exercised.
	returnVoiceFileID string
}

func (m *mockBotAPI) Send(c tgbotapi.Chattable) (tgbotapi.Message, error) {
	m.sentMessages = append(m.sentMessages, c)
	if m.sendErr != nil {
		return tgbotapi.Message{}, m.sendErr
	}
	if _, ok := c.(tgbotapi.VoiceConfig); ok && m.returnVoiceFileID != "" {
		return tgbotapi.Message{MessageID: 1001, Voice: &tgbotapi.Voice{FileID: m.returnVoiceFileID}}, nil
	}
	return tgbotapi.Message{MessageID: 1001}, nil
}

func (m *mockBotAPI) Request(c tgbotapi.Chattable) (*tgbotapi.APIResponse, error) {
	return &tgbotapi.APIResponse{}, nil
}

func (m *mockBotAPI) GetUpdatesChan(config tgbotapi.UpdateConfig) tgbotapi.UpdatesChannel {
	return nil
}

func (m *mockBotAPI) StopReceivingUpdates() {}

type testRedis struct {
	redis.Cmdable
	values map[string]string
}

func (f *testRedis) Get(ctx context.Context, key string) *redis.StringCmd {
	val, ok := f.values[key]
	if !ok {
		return redis.NewStringResult("", redis.Nil)
	}
	return redis.NewStringResult(val, nil)
}

func (f *testRedis) GetDel(ctx context.Context, key string) *redis.StringCmd {
	val, ok := f.values[key]
	if !ok {
		return redis.NewStringResult("", redis.Nil)
	}
	delete(f.values, key)
	return redis.NewStringResult(val, nil)
}

func (f *testRedis) Set(ctx context.Context, key string, value any, expiration time.Duration) *redis.StatusCmd {
	switch v := value.(type) {
	case []byte:
		f.values[key] = string(v)
	case string:
		f.values[key] = v
	default:
		f.values[key] = fmt.Sprint(v)
	}
	return redis.NewStatusResult("OK", nil)
}

func (f *testRedis) Del(ctx context.Context, keys ...string) *redis.IntCmd {
	var deleted int64
	for _, key := range keys {
		if _, ok := f.values[key]; ok {
			delete(f.values, key)
			deleted++
		}
	}
	return redis.NewIntResult(deleted, nil)
}

type mockSRS struct {
	service.SRSService
}

func (m *mockSRS) ScheduleAnswer(q *model.UserQuestionProgress, isCorrect bool) {}
func (m *mockSRS) GetDueCount(ctx context.Context, userID int64, language, level string) (int, error) {
	return 0, nil
}

type mockLLM struct {
	gradeFn  func(ctx context.Context, prompt, correctAnswer, userAnswer string) (external.GradeResult, error)
	answerFn func(ctx context.Context, question string) (string, error)
}

func (m *mockLLM) GradeAnswer(
	ctx context.Context,
	prompt, correctAnswer, userAnswer string,
) (external.GradeResult, error) {
	if m.gradeFn != nil {
		return m.gradeFn(ctx, prompt, correctAnswer, userAnswer)
	}
	return external.GradeResult{IsCorrect: true}, nil
}

func (m *mockLLM) GradeHandwriting(
	ctx context.Context,
	prompt, correctAnswer string,
	image []byte,
) (external.GradeResult, error) {
	return external.GradeResult{}, nil
}
func (m *mockLLM) AnswerLearningQuestion(ctx context.Context, question string) (string, error) {
	if m.answerFn != nil {
		return m.answerFn(ctx, question)
	}
	return "answer", nil
}
