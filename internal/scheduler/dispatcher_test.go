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

// mockPushClaims grants each user/slot/date claim once. Workers call it
// concurrently.
type mockPushClaims struct {
	mu      sync.Mutex
	claimed map[pushClaimCall]bool
	err     error
	calls   []pushClaimCall
}

type pushClaimCall struct {
	userID int64
	slot   model.SessionSlot
	today  string
}

func (mock *mockPushClaims) TryClaim(
	_ context.Context,
	userID int64,
	slot model.SessionSlot,
	today string,
) (bool, error) {
	mock.mu.Lock()
	defer mock.mu.Unlock()
	call := pushClaimCall{userID: userID, slot: slot, today: today}
	mock.calls = append(
		mock.calls,
		call,
	)
	if mock.err != nil {
		return false, mock.err
	}
	if mock.claimed[call] {
		return false, nil
	}
	if mock.claimed == nil {
		mock.claimed = make(map[pushClaimCall]bool)
	}
	mock.claimed[call] = true
	return true, nil
}

type mockDispatcherPusher struct {
	mu          sync.Mutex
	quizPushes  []int64
	studyPushes []int64
}

// schedulerSessionQueryRepositoryStub serves the unfinished-session queries; the
// embedded nil SessionRepository makes any other repository call panic.
type schedulerSessionQueryRepositoryStub struct {
	service.SessionRepository
	session *model.Session
	err     error
}

func (stub *schedulerSessionQueryRepositoryStub) GetOldestUnfinished(
	context.Context,
	int64,
) (*model.Session, error) {
	return stub.session, stub.err
}

func (stub *schedulerSessionQueryRepositoryStub) CountUnfinishedBatch(
	context.Context,
	[]int64,
) (map[int64]int, error) {
	return nil, stub.err
}

func newSchedulerSessionService(repository *schedulerSessionQueryRepositoryStub) *service.SessionService {
	return service.NewSessionService(service.SessionDependencies{SessionRepository: repository})
}

func (mock *mockDispatcherPusher) PushSession(
	ctx context.Context,
	chatID int64,
	sessionID int,
	sessionType string,
) error {
	mock.mu.Lock()
	defer mock.mu.Unlock()
	mock.quizPushes = append(
		mock.quizPushes,
		chatID,
	)
	return nil
}

// study returns the Study push contract over the same recorder, so a test
// can assert which kind of push each user received.
func (mock *mockDispatcherPusher) study() mockStudyPusher {
	return mockStudyPusher{recorder: mock}
}

type mockStudyPusher struct {
	recorder *mockDispatcherPusher
}

func (mock mockStudyPusher) PushSession(
	ctx context.Context,
	chatID int64,
	sessionID int,
) error {
	mock.recorder.mu.Lock()
	defer mock.recorder.mu.Unlock()
	mock.recorder.studyPushes = append(
		mock.recorder.studyPushes,
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
	sessionQueryStub := &schedulerSessionQueryRepositoryStub{session: unfinishedSession}
	sessions := newSchedulerSessionService(sessionQueryStub)

	dispatcher := newSessionDispatcher(
		sessions,
		pusher,
		pusher.study(),
		claims,
	)
	user := model.User{ID: 1001, Language: "ja", ProficiencyLevel: "N5"}

	// 1st dispatch: Should acquire lock and push
	if err := dispatcher.dispatchUser(
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
	if err := dispatcher.dispatchUser(
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
	sessions := newSchedulerSessionService(&schedulerSessionQueryRepositoryStub{
		session: &model.Session{ID: 10, Mode: model.SessionModeStudy},
	})
	dispatcher := newSessionDispatcher(
		sessions,
		pusher,
		pusher.study(),
		claims,
	)

	if err := dispatcher.dispatchUser(
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
	sessionQueryStub := &schedulerSessionQueryRepositoryStub{session: unfinishedSession}
	sessions := newSchedulerSessionService(sessionQueryStub)

	dispatcher := newSessionDispatcher(
		sessions,
		pusher,
		pusher.study(),
		&mockPushClaims{},
	)
	user := model.User{ID: 2002, Language: "ja", ProficiencyLevel: "N5"}

	// Unfinished count = 3 >= max (3): Should remind rather than build
	if err := dispatcher.dispatchUser(
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
	limiter := newRateLimiter(100) // 100 msgs/sec
	defer limiter.Stop()

	ctx, cancel := context.WithTimeout(
		context.Background(),
		50*time.Millisecond,
	)
	defer cancel()

	if err := limiter.Wait(ctx); err != nil {
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
	sessionQueryStub := &schedulerSessionQueryRepositoryStub{session: unfinishedSession}
	sessions := newSchedulerSessionService(sessionQueryStub)

	dispatcher := newSessionDispatcher(
		sessions,
		pusher,
		pusher.study(),
		&mockPushClaims{},
	)
	// Higher limiter rate for fast testing
	dispatcher.limiter = newRateLimiter(500)
	defer dispatcher.limiter.Stop()

	var users []model.User
	unfinishedCounts := make(map[int64]int)
	for i := int64(1); i <= 20; i++ {
		users = append(
			users,
			model.User{ID: i, Language: "ja", ProficiencyLevel: "N5"},
		)
		unfinishedCounts[i] = 3
	}

	if err := dispatcher.dispatchBatch(
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
