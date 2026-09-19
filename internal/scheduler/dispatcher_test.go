package scheduler

import (
	"context"
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/redis/go-redis/v9"

	"github.com/lsj/copylingo/internal/config"
	"github.com/lsj/copylingo/internal/model"
	"github.com/lsj/copylingo/internal/service"
)

type mockRedisForDispatcher struct {
	redis.Cmdable
	mu     sync.Mutex
	values map[string]string
}

func newMockRedis() *mockRedisForDispatcher {
	return &mockRedisForDispatcher{values: make(map[string]string)}
}

func (m *mockRedisForDispatcher) SetNX(
	ctx context.Context,
	key string,
	value any,
	expiration time.Duration,
) *redis.BoolCmd {
	m.mu.Lock()
	defer m.mu.Unlock()
	if _, ok := m.values[key]; ok {
		return redis.NewBoolResult(false, nil)
	}
	m.values[key] = fmt.Sprint(value)
	return redis.NewBoolResult(true, nil)
}

type mockDispatcherPusher struct {
	mu          sync.Mutex
	quizPushes  []int64
	studyPushes []int64
}

func (p *mockDispatcherPusher) PushSession(ctx context.Context, chatID int64, sessionID int, sessionType string) error {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.quizPushes = append(p.quizPushes, chatID)
	return nil
}

func (p *mockDispatcherPusher) PushStudySession(ctx context.Context, chatID int64, sessionID int) error {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.studyPushes = append(p.studyPushes, chatID)
	return nil
}

func TestDispatcher_RedisIdempotency(t *testing.T) {
	ctx := context.Background()
	rdb := newMockRedis()
	pusher := &mockDispatcherPusher{}

	// Stub session query returning an unfinished session so reminder succeeds
	unfinishedSession := &model.Session{ID: 10, Mode: model.SessionModeStudy}
	sqStub := &schedulerSessionQueryRepoStub{session: unfinishedSession, unfinishedCount: 3}
	services := &service.Services{
		SessionQuery: service.NewSessionQueryService(sqStub),
	}

	cfg := &config.Config{
		Schedule: config.ScheduleConfig{MaxUnfinishedSessions: 3},
	}

	d := newSessionDispatcher(cfg, services, pusher, rdb)
	user := model.User{ID: 1001, Language: "ja", ProficiencyLevel: "N5"}

	// 1st dispatch: Should acquire lock and push
	if err := d.dispatchUser(ctx, user, model.SessionSlotMorningStudy, 3); err != nil {
		t.Fatalf("first dispatch failed: %v", err)
	}
	if len(pusher.studyPushes) != 1 {
		t.Fatalf("studyPushes = %d, want 1", len(pusher.studyPushes))
	}

	// 2nd dispatch: Redis lock already exists, should skip
	if err := d.dispatchUser(ctx, user, model.SessionSlotMorningStudy, 3); err != nil {
		t.Fatalf("second dispatch failed: %v", err)
	}
	if len(pusher.studyPushes) != 1 {
		t.Fatalf("studyPushes = %d after duplicate, want 1 (skipped)", len(pusher.studyPushes))
	}
}

func TestDispatcher_BacklogRemind(t *testing.T) {
	ctx := context.Background()
	pusher := &mockDispatcherPusher{}

	unfinishedSession := &model.Session{ID: 99, Type: model.SessionMorning, Mode: model.SessionModeQuiz}
	sqStub := &schedulerSessionQueryRepoStub{session: unfinishedSession, unfinishedCount: 3}
	services := &service.Services{
		SessionQuery: service.NewSessionQueryService(sqStub),
	}

	cfg := &config.Config{
		Schedule: config.ScheduleConfig{MaxUnfinishedSessions: 3},
	}

	d := newSessionDispatcher(cfg, services, pusher, nil)
	user := model.User{ID: 2002, Language: "ja", ProficiencyLevel: "N5"}

	// Unfinished count = 3 >= max (3): Should remind rather than build
	if err := d.dispatchUser(ctx, user, model.SessionSlotMorningQuiz, 3); err != nil {
		t.Fatalf("dispatch failed: %v", err)
	}

	if len(pusher.quizPushes) != 1 || pusher.quizPushes[0] != 2002 {
		t.Fatalf("quizPushes = %+v, want [2002]", pusher.quizPushes)
	}
}

func TestDispatcher_RateLimiter(t *testing.T) {
	rl := newRateLimiter(100) // 100 msgs/sec
	defer rl.Stop()

	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()

	if err := rl.Wait(ctx); err != nil {
		t.Fatalf("Wait failed: %v", err)
	}
}

func TestDispatcher_BatchConcurrent(t *testing.T) {
	ctx := context.Background()
	pusher := &mockDispatcherPusher{}

	unfinishedSession := &model.Session{ID: 1, Mode: model.SessionModeStudy}
	sqStub := &schedulerSessionQueryRepoStub{session: unfinishedSession, unfinishedCount: 3}
	services := &service.Services{
		SessionQuery: service.NewSessionQueryService(sqStub),
	}

	cfg := &config.Config{
		Schedule: config.ScheduleConfig{MaxUnfinishedSessions: 3},
	}

	d := newSessionDispatcher(cfg, services, pusher, nil)
	// Higher limiter rate for fast testing
	d.limiter = newRateLimiter(500)
	defer d.limiter.Stop()

	var users []model.User
	unfinishedCounts := make(map[int64]int)
	for i := int64(1); i <= 20; i++ {
		users = append(users, model.User{ID: i, Language: "ja", ProficiencyLevel: "N5"})
		unfinishedCounts[i] = 3
	}

	if err := d.dispatchBatch(ctx, model.SessionSlotMorningStudy, users, unfinishedCounts); err != nil {
		t.Fatalf("dispatchBatch failed: %v", err)
	}

	pusher.mu.Lock()
	count := len(pusher.studyPushes)
	pusher.mu.Unlock()

	if count != 20 {
		t.Fatalf("dispatched = %d, want 20", count)
	}
}
