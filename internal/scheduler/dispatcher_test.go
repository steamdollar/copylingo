package scheduler

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/lsj/copylingo/internal/model"
	"github.com/lsj/copylingo/internal/service"
)

type mockPushClaims struct {
	claimed bool
	err     error
	calls   []pushClaimCall
}

type pushClaimCall struct {
	userID int64
	slot   model.SessionSlot
	today  string
}

func (m *mockPushClaims) TryClaim(
	_ context.Context,
	userID int64,
	slot model.SessionSlot,
	today string,
) (bool, error) {
	m.calls = append(
		m.calls,
		pushClaimCall{userID: userID, slot: slot, today: today},
	)
	if m.err != nil {
		return false, m.err
	}
	if m.claimed {
		return false, nil
	}
	m.claimed = true
	return true, nil
}

type mockDispatcherPusher struct {
	mu          sync.Mutex
	quizPushes  []int64
	studyPushes []int64
}

// schedulerSessionQueryRepoStub serves the unfinished-session queries; the
// embedded nil SessionRepo makes any other repository call panic.
type schedulerSessionQueryRepoStub struct {
	service.SessionRepo
	session *model.Session
	err     error
}

func (r *schedulerSessionQueryRepoStub) GetOldestUnfinished(
	context.Context,
	int64,
) (*model.Session, error) {
	return r.session, r.err
}

func (r *schedulerSessionQueryRepoStub) CountUnfinishedBatch(
	context.Context,
	[]int64,
) (map[int64]int, error) {
	return nil, r.err
}

func newSchedulerSessionService(repo *schedulerSessionQueryRepoStub) *service.SessionService {
	return service.NewSessionService(service.SessionDeps{SessionRepo: repo})
}

func (p *mockDispatcherPusher) PushSession(
	ctx context.Context,
	chatID int64,
	sessionID int,
	sessionType string,
) error {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.quizPushes = append(
		p.quizPushes,
		chatID,
	)
	return nil
}

// study returns the Study push contract over the same recorder, so a test
// can assert which kind of push each user received.
func (p *mockDispatcherPusher) study() mockStudyPusher {
	return mockStudyPusher{recorder: p}
}

type mockStudyPusher struct {
	recorder *mockDispatcherPusher
}

func (p mockStudyPusher) PushSession(
	ctx context.Context,
	chatID int64,
	sessionID int,
) error {
	p.recorder.mu.Lock()
	defer p.recorder.mu.Unlock()
	p.recorder.studyPushes = append(
		p.recorder.studyPushes,
		chatID,
	)
	return nil
}

func TestDispatcher_RedisIdempotency(t *testing.T) {
	ctx := context.Background()
	claims := &mockPushClaims{}
	pusher := &mockDispatcherPusher{}

	// Stub session query returning an unfinished session so reminder succeeds
	unfinishedSession := &model.Session{ID: 10, Mode: model.SessionModeStudy}
	sqStub := &schedulerSessionQueryRepoStub{session: unfinishedSession}
	services := &service.Services{
		Session: newSchedulerSessionService(sqStub),
	}

	d := newSessionDispatcher(
		services,
		pusher,
		pusher.study(),
		claims,
	)
	user := model.User{ID: 1001, Language: "ja", ProficiencyLevel: "N5"}

	// 1st dispatch: Should acquire lock and push
	if err := d.dispatchUser(
		ctx,
		user,
		model.SessionSlotMorningStudy,
		3,
	); err != nil {
		t.Fatalf(
			"first dispatch failed: %v",
			err,
		)
	}
	if len(pusher.studyPushes) != 1 {
		t.Fatalf(
			"studyPushes = %d, want 1",
			len(pusher.studyPushes),
		)
	}

	// 2nd dispatch: Redis lock already exists, should skip
	if err := d.dispatchUser(
		ctx,
		user,
		model.SessionSlotMorningStudy,
		3,
	); err != nil {
		t.Fatalf(
			"second dispatch failed: %v",
			err,
		)
	}
	if len(pusher.studyPushes) != 1 {
		t.Fatalf(
			"studyPushes = %d after duplicate, want 1 (skipped)",
			len(pusher.studyPushes),
		)
	}
	if len(claims.calls) != 2 || claims.calls[0] != claims.calls[1] {
		t.Fatalf(
			"claim calls = %+v, want two identical user/slot/date claims",
			claims.calls,
		)
	}
	if claims.calls[0].userID != user.ID || claims.calls[0].slot != model.SessionSlotMorningStudy {
		t.Fatalf(
			"claim call = %+v, want user %d and slot %s",
			claims.calls[0],
			user.ID,
			model.SessionSlotMorningStudy,
		)
	}
	if _, err := time.Parse(
		"2006-01-02",
		claims.calls[0].today,
	); err != nil {
		t.Fatalf(
			"claim date = %q, want YYYY-MM-DD: %v",
			claims.calls[0].today,
			err,
		)
	}
}

func TestDispatcher_ClaimErrorFailsOpen(t *testing.T) {
	claims := &mockPushClaims{err: errors.New("redis unavailable")}
	pusher := &mockDispatcherPusher{}
	services := &service.Services{
		Session: newSchedulerSessionService(&schedulerSessionQueryRepoStub{
			session: &model.Session{ID: 10, Mode: model.SessionModeStudy},
		}),
	}
	d := newSessionDispatcher(
		services,
		pusher,
		pusher.study(),
		claims,
	)

	if err := d.dispatchUser(
		context.Background(),
		model.User{ID: 1001},
		model.SessionSlotMorningStudy,
		3,
	); err != nil {
		t.Fatalf(
			"dispatchUser() error = %v, want fail-open success",
			err,
		)
	}
	if len(pusher.studyPushes) != 1 {
		t.Fatalf(
			"study pushes = %d, want 1 after claim error",
			len(pusher.studyPushes),
		)
	}
}

func TestDispatcher_BacklogRemind(t *testing.T) {
	ctx := context.Background()
	pusher := &mockDispatcherPusher{}

	unfinishedSession := &model.Session{ID: 99, Type: model.SessionMorning, Mode: model.SessionModeQuiz}
	sqStub := &schedulerSessionQueryRepoStub{session: unfinishedSession}
	services := &service.Services{
		Session: newSchedulerSessionService(sqStub),
	}

	d := newSessionDispatcher(
		services,
		pusher,
		pusher.study(),
		nil,
	)
	user := model.User{ID: 2002, Language: "ja", ProficiencyLevel: "N5"}

	// Unfinished count = 3 >= max (3): Should remind rather than build
	if err := d.dispatchUser(
		ctx,
		user,
		model.SessionSlotMorningQuiz,
		3,
	); err != nil {
		t.Fatalf(
			"dispatch failed: %v",
			err,
		)
	}

	if len(pusher.quizPushes) != 1 || pusher.quizPushes[0] != 2002 {
		t.Fatalf(
			"quizPushes = %+v, want [2002]",
			pusher.quizPushes,
		)
	}
}

func TestDispatcher_RateLimiter(t *testing.T) {
	rl := newRateLimiter(100) // 100 msgs/sec
	defer rl.Stop()

	ctx, cancel := context.WithTimeout(
		context.Background(),
		50*time.Millisecond,
	)
	defer cancel()

	if err := rl.Wait(ctx); err != nil {
		t.Fatalf(
			"Wait failed: %v",
			err,
		)
	}
}

func TestDispatcher_BatchConcurrent(t *testing.T) {
	ctx := context.Background()
	pusher := &mockDispatcherPusher{}

	unfinishedSession := &model.Session{ID: 1, Mode: model.SessionModeStudy}
	sqStub := &schedulerSessionQueryRepoStub{session: unfinishedSession}
	services := &service.Services{
		Session: newSchedulerSessionService(sqStub),
	}

	d := newSessionDispatcher(
		services,
		pusher,
		pusher.study(),
		nil,
	)
	// Higher limiter rate for fast testing
	d.limiter = newRateLimiter(500)
	defer d.limiter.Stop()

	var users []model.User
	unfinishedCounts := make(map[int64]int)
	for i := int64(1); i <= 20; i++ {
		users = append(
			users,
			model.User{ID: i, Language: "ja", ProficiencyLevel: "N5"},
		)
		unfinishedCounts[i] = 3
	}

	if err := d.dispatchBatch(
		ctx,
		model.SessionSlotMorningStudy,
		users,
		unfinishedCounts,
	); err != nil {
		t.Fatalf(
			"dispatchBatch failed: %v",
			err,
		)
	}

	pusher.mu.Lock()
	count := len(pusher.studyPushes)
	pusher.mu.Unlock()

	if count != 20 {
		t.Fatalf(
			"dispatched = %d, want 20",
			count,
		)
	}
}
