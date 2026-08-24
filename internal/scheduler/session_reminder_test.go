package scheduler

import (
	"context"
	"errors"
	"testing"

	"github.com/lsj/copylingo/internal/model"
	"github.com/lsj/copylingo/internal/service"
)

type schedulerUserRepoStub struct {
	users []model.User
}

func (r *schedulerUserRepoStub) GetOrCreate(context.Context, int64, string) (*model.User, error) {
	return nil, nil
}

func (r *schedulerUserRepoStub) GetAllUsers(context.Context) ([]model.User, error) {
	return r.users, nil
}

type schedulerSessionQueryRepoStub struct {
	session *model.Session
	err     error
}

func (r *schedulerSessionQueryRepoStub) GetOldestUnfinished(context.Context, int64) (*model.Session, error) {
	return r.session, r.err
}

type schedulerPusherStub struct {
	quizCalls  []quizPushCall
	studyCalls []studyPushCall
	quizErr    error
	studyErr   error
}

type quizPushCall struct {
	userID      int64
	sessionID   int
	sessionType string
}

type studyPushCall struct {
	userID    int64
	sessionID int
}

func (p *schedulerPusherStub) PushSession(_ context.Context, userID int64, sessionID int, sessionType string) error {
	p.quizCalls = append(p.quizCalls, quizPushCall{userID: userID, sessionID: sessionID, sessionType: sessionType})
	return p.quizErr
}

func (p *schedulerPusherStub) PushStudySession(_ context.Context, userID int64, sessionID int) error {
	p.studyCalls = append(p.studyCalls, studyPushCall{userID: userID, sessionID: sessionID})
	return p.studyErr
}

func newSchedulerForReminderTest(
	user model.User,
	session *model.Session,
	queryErr error,
	pusher sessionPusher,
) *Scheduler {
	return &Scheduler{
		services: &service.Services{
			User: service.NewUserService(&schedulerUserRepoStub{users: []model.User{user}}),
			SessionQuery: service.NewSessionQueryService(&schedulerSessionQueryRepoStub{
				session: session,
				err:     queryErr,
			}),
		},
		bot: pusher,
	}
}

func TestBuildAndPushSessionsRemindsExistingLegacyQuizWithoutBuilding(t *testing.T) {
	pusher := &schedulerPusherStub{}
	scheduler := newSchedulerForReminderTest(
		model.User{ID: 123},
		&model.Session{ID: 77, Type: model.SessionEvening, Mode: "", Status: model.SessionPending},
		nil,
		pusher,
	)

	if err := scheduler.buildAndPushSessions(context.Background(), model.SessionMorning); err != nil {
		t.Fatalf("buildAndPushSessions failed: %v", err)
	}
	if len(pusher.quizCalls) != 1 {
		t.Fatalf("quiz push calls = %d, want 1", len(pusher.quizCalls))
	}
	call := pusher.quizCalls[0]
	if call.userID != 123 || call.sessionID != 77 || call.sessionType != string(model.SessionEvening) {
		t.Fatalf("quiz push call = %+v, want user/session/type (123,77,%q)", call, model.SessionEvening)
	}
	if len(pusher.studyCalls) != 0 {
		t.Fatalf("study push calls = %d, want 0", len(pusher.studyCalls))
	}
}

func TestBuildAndPushStudySessionsRemindsExistingStudyWithoutBuilding(t *testing.T) {
	pusher := &schedulerPusherStub{}
	scheduler := newSchedulerForReminderTest(
		model.User{ID: 456},
		&model.Session{ID: 88, Type: model.SessionStudy, Mode: model.SessionModeStudy, Status: model.SessionInProgress},
		nil,
		pusher,
	)

	if err := scheduler.buildAndPushStudySessions(context.Background()); err != nil {
		t.Fatalf("buildAndPushStudySessions failed: %v", err)
	}
	if len(pusher.studyCalls) != 1 {
		t.Fatalf("study push calls = %d, want 1", len(pusher.studyCalls))
	}
	call := pusher.studyCalls[0]
	if call.userID != 456 || call.sessionID != 88 {
		t.Fatalf("study push call = %+v, want user/session (456,88)", call)
	}
	if len(pusher.quizCalls) != 0 {
		t.Fatalf("quiz push calls = %d, want 0", len(pusher.quizCalls))
	}
}

func TestBuildAndPushSessionsQueryFailureSkipsBuild(t *testing.T) {
	pusher := &schedulerPusherStub{}
	scheduler := newSchedulerForReminderTest(model.User{ID: 789}, nil, errors.New("query failed"), pusher)

	if err := scheduler.buildAndPushSessions(context.Background(), model.SessionMorning); err == nil {
		t.Fatal("buildAndPushSessions error = nil, want query failure")
	}
	if len(pusher.quizCalls) != 0 || len(pusher.studyCalls) != 0 {
		t.Fatalf("push calls = quiz %d, study %d; want none", len(pusher.quizCalls), len(pusher.studyCalls))
	}
}

func TestBuildAndPushSessionsReminderPushFailureSkipsBuild(t *testing.T) {
	pusher := &schedulerPusherStub{quizErr: errors.New("push failed")}
	scheduler := newSchedulerForReminderTest(
		model.User{ID: 321},
		&model.Session{ID: 99, Type: model.SessionMorning, Mode: model.SessionModeQuiz, Status: model.SessionPending},
		nil,
		pusher,
	)

	if err := scheduler.buildAndPushSessions(context.Background(), model.SessionMorning); err == nil {
		t.Fatal("buildAndPushSessions error = nil, want reminder push failure")
	}
	if len(pusher.quizCalls) != 1 || len(pusher.studyCalls) != 0 {
		t.Fatalf("push calls = quiz %d, study %d; want quiz 1/study 0", len(pusher.quizCalls), len(pusher.studyCalls))
	}
}

func TestBuildAndPushSessionsInvalidModeSkipsBuildAndPush(t *testing.T) {
	pusher := &schedulerPusherStub{}
	scheduler := newSchedulerForReminderTest(
		model.User{ID: 654},
		&model.Session{ID: 100, Type: model.SessionMorning, Mode: "unknown", Status: model.SessionPending},
		nil,
		pusher,
	)

	if err := scheduler.buildAndPushSessions(context.Background(), model.SessionMorning); err == nil {
		t.Fatal("buildAndPushSessions error = nil, want unsupported mode error")
	}
	if len(pusher.quizCalls) != 0 || len(pusher.studyCalls) != 0 {
		t.Fatalf("push calls = quiz %d, study %d; want none", len(pusher.quizCalls), len(pusher.studyCalls))
	}
}
