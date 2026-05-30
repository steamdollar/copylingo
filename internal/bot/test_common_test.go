package bot

import (
	"context"
	"fmt"
	"time"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
	"github.com/lsj/copylingo/internal/model"
	"github.com/lsj/copylingo/internal/service"
	"github.com/redis/go-redis/v9"
)

type mockBotAPI struct {
	sentMessages []tgbotapi.Chattable
}

func (m *mockBotAPI) Send(c tgbotapi.Chattable) (tgbotapi.Message, error) {
	m.sentMessages = append(m.sentMessages, c)
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

func (m *mockSRS) ScheduleAnswer(q *model.Question, isCorrect bool) {}
func (m *mockSRS) GetDueCount(ctx context.Context) (int, error)       { return 0, nil }

type mockLLM struct {
	gradeFn func(ctx context.Context, prompt, correctAnswer, userAnswer string) (bool, string, error)
}

func (m *mockLLM) GradeAnswer(ctx context.Context, prompt, correctAnswer, userAnswer string) (bool, string, error) {
	if m.gradeFn != nil {
		return m.gradeFn(ctx, prompt, correctAnswer, userAnswer)
	}
	return true, "", nil
}
func (m *mockLLM) GradeHandwriting(ctx context.Context, prompt, correctAnswer string, image []byte) (bool, string, error) {
	return false, "", nil
}
func (m *mockLLM) Translate(ctx context.Context, text, targetLang string) (string, error) {
	return "", nil
}
