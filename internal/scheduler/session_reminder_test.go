package scheduler

import (
	"context"
	"errors"
	"reflect"
	"sort"
	"testing"

	"github.com/robfig/cron/v3"

	"github.com/lsj/copylingo/internal/config"
	"github.com/lsj/copylingo/internal/model"
	"github.com/lsj/copylingo/internal/service"
)

type schedulerUserRepoStub struct {
	users []model.User
}

func (r *schedulerUserRepoStub) GetOrCreate(context.Context, int64, string) (*model.User, error) {
	return nil, nil
}

func (r *schedulerUserRepoStub) GetByID(context.Context, int64) (*model.User, error) {
	return nil, nil
}

func (r *schedulerUserRepoStub) GetAllUsers(context.Context) ([]model.User, error) {
	return r.users, nil
}

func (r *schedulerUserRepoStub) GetActiveTimezones(context.Context) ([]string, error) {
	return nil, nil
}

func (r *schedulerUserRepoStub) GetUsersBySlot(
	context.Context,
	model.SessionSlot,
	string,
	string,
) ([]model.User, error) {
	return r.users, nil
}

func (r *schedulerUserRepoStub) UpdateSlotTime(context.Context, int64, model.SessionSlot, *string) error {
	return nil
}

func (r *schedulerUserRepoStub) UpdateTimezone(context.Context, int64, string) error {
	return nil
}

type schedulerSessionQueryRepoStub struct {
	session         *model.Session
	err             error
	unfinishedCount int
}

func (r *schedulerSessionQueryRepoStub) GetOldestUnfinished(context.Context, int64) (*model.Session, error) {
	return r.session, r.err
}

func (r *schedulerSessionQueryRepoStub) CountUnfinished(context.Context, int64) (int, error) {
	return r.unfinishedCount, r.err
}

func (r *schedulerSessionQueryRepoStub) CountUnfinishedBatch(context.Context, []int64) (map[int64]int, error) {
	return nil, r.err
}

type schedulerPusherStub struct {
	quizCalls  []quizPushCall
	studyCalls []studyPushCall
	quizErr    error
	studyErr   error
}

type schedulerQuestionRepoStub struct{}

func (schedulerQuestionRepoStub) GetNewQuestions(
	context.Context,
	int64,
	string,
	[]string,
	string,
	[]int,
	int,
	int,
) ([]model.Question, error) {
	return nil, nil
}

func (schedulerQuestionRepoStub) GetByID(context.Context, int) (*model.Question, error) {
	return nil, nil
}

type schedulerSessionStoreStub struct {
	nextID int
}

func (r *schedulerSessionStoreStub) CreateSession(_ context.Context, session *model.Session) error {
	if session.ID == 0 {
		session.ID = r.nextID
	}
	return nil
}

func (schedulerSessionStoreStub) GetByID(context.Context, int) (*model.Session, error) {
	return nil, nil
}

func (schedulerSessionStoreStub) GetSessionsByStatus(
	context.Context,
	int64,
	config.SessionStatus,
) ([]model.Session, error) {
	return nil, nil
}

func (schedulerSessionStoreStub) ListInProgress(context.Context) ([]model.Session, error) {
	return nil, nil
}

func (schedulerSessionStoreStub) Start(context.Context, int) error {
	return nil
}

type schedulerSessionQuestionStoreStub struct{}

func (schedulerSessionQuestionStoreStub) CreateSessionQuestions(context.Context, []model.SessionQuestion) error {
	return nil
}

func (schedulerSessionQuestionStoreStub) GetBySession(context.Context, int) ([]model.SessionQuestion, error) {
	return nil, nil
}

type schedulerSRSSStub struct{}

func (schedulerSRSSStub) GetDueReviews(
	context.Context,
	int64,
	string,
	string,
	int,
	int,
) ([]model.Question, error) {
	return []model.Question{{ID: 1}}, nil
}

func (schedulerSRSSStub) GetDueCount(context.Context, int64, string, string) (int, error) {
	return 0, nil
}

type schedulerMaterialRepoStub struct{}

func (schedulerMaterialRepoStub) GetForStudySession(
	_ context.Context,
	_ int64,
	_ string,
	_ string,
	_ []string,
	_ model.StudySessionPlan,
) ([]model.Material, error) {
	return []model.Material{{ID: 1}}, nil
}

type schedulerMaterialRepoRecorder struct {
	plan  model.StudySessionPlan
	plans []model.StudySessionPlan
}

func (r *schedulerMaterialRepoRecorder) GetForStudySession(
	_ context.Context,
	_ int64,
	_ string,
	_ string,
	_ []string,
	plan model.StudySessionPlan,
) ([]model.Material, error) {
	r.plan = plan
	r.plans = append(r.plans, plan)
	return []model.Material{{ID: 1}}, nil
}

type schedulerSessionMaterialStoreStub struct{}

func (schedulerSessionMaterialStoreStub) CreateSessionMaterials(context.Context, []model.SessionMaterial) error {
	return nil
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
	return newSchedulerForReminderTestWithCount(user, session, queryErr, pusher, 3)
}

func newSchedulerForReminderTestWithCount(
	user model.User,
	session *model.Session,
	queryErr error,
	pusher sessionPusher,
	unfinishedCount int,
) *Scheduler {
	return &Scheduler{
		cfg: &config.Config{Schedule: config.ScheduleConfig{MaxUnfinishedSessions: 3}},
		services: &service.Services{
			User: service.NewUserService(&schedulerUserRepoStub{users: []model.User{user}}),
			SessionQuery: service.NewSessionQueryService(&schedulerSessionQueryRepoStub{
				session:         session,
				err:             queryErr,
				unfinishedCount: unfinishedCount,
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

func TestBuildAndPushSessionsBuildsWhenBacklogBelowCap(t *testing.T) {
	for _, sessionType := range []model.SessionType{model.SessionMorning, model.SessionEvening} {
		t.Run(string(sessionType), func(t *testing.T) {
			pusher := &schedulerPusherStub{}
			scheduler := newSchedulerForReminderTestWithCount(
				model.User{ID: 123},
				&model.Session{
					ID:     77,
					Type:   model.SessionEvening,
					Mode:   model.SessionModeQuiz,
					Status: model.SessionPending,
				},
				nil,
				pusher,
				2,
			)
			scheduler.services.SessionBuilder = service.NewSessionBuilderService(
				schedulerQuestionRepoStub{},
				&schedulerSessionStoreStub{nextID: 101},
				schedulerSessionQuestionStoreStub{},
				schedulerSRSSStub{},
			)

			if err := scheduler.buildAndPushSessions(context.Background(), sessionType); err != nil {
				t.Fatalf("buildAndPushSessions failed: %v", err)
			}
			if len(pusher.quizCalls) != 1 {
				t.Fatalf("quiz push calls = %d, want 1", len(pusher.quizCalls))
			}
			call := pusher.quizCalls[0]
			if call.userID != 123 || call.sessionID != 101 || call.sessionType != string(sessionType) {
				t.Fatalf("quiz push call = %+v, want user/session/type (123,101,%q)", call, sessionType)
			}
			if len(pusher.studyCalls) != 0 {
				t.Fatalf("study push calls = %d, want 0", len(pusher.studyCalls))
			}
		})
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

	if err := scheduler.buildAndPushStudySessions(context.Background(), service.StudyProfileMorning); err != nil {
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

func TestBuildAndPushStudySessionsBuildsWhenBacklogBelowCap(t *testing.T) {
	pusher := &schedulerPusherStub{}
	materialRepo := &schedulerMaterialRepoRecorder{}
	scheduler := newSchedulerForReminderTestWithCount(
		model.User{ID: 456},
		&model.Session{ID: 88, Type: model.SessionStudy, Mode: model.SessionModeStudy, Status: model.SessionPending},
		nil,
		pusher,
		2,
	)
	scheduler.services.StudySession = service.NewStudySessionService(
		materialRepo,
		&schedulerSessionStoreStub{nextID: 202},
		schedulerSessionMaterialStoreStub{},
	)

	if err := scheduler.buildAndPushStudySessions(context.Background(), service.StudyProfileMorning); err != nil {
		t.Fatalf("buildAndPushStudySessions failed: %v", err)
	}
	if len(pusher.studyCalls) != 1 {
		t.Fatalf("study push calls = %d, want 1", len(pusher.studyCalls))
	}
	call := pusher.studyCalls[0]
	if call.userID != 456 || call.sessionID != 202 {
		t.Fatalf("study push call = %+v, want user/session (456,202)", call)
	}
	if len(pusher.quizCalls) != 0 {
		t.Fatalf("quiz push calls = %d, want 0", len(pusher.quizCalls))
	}
	want := model.StudySessionPlan{Quotas: []model.StudyMaterialQuota{
		{Category: model.MaterialCategoryVocabulary, NewCount: 8, ReviewCount: 7},
		{Category: model.MaterialCategoryGrammar, NewCount: 1, ReviewCount: 3},
		{Category: model.MaterialCategoryReading, NewCount: 1, ReviewCount: 0},
	}}
	if !reflect.DeepEqual(materialRepo.plan, want) {
		t.Fatalf("plan = %+v, want %+v", materialRepo.plan, want)
	}
}

func TestBuildAndPushStudySessionsUsesEveningProfileWhenBuilding(t *testing.T) {
	pusher := &schedulerPusherStub{}
	materialRepo := &schedulerMaterialRepoRecorder{}
	scheduler := newSchedulerForReminderTestWithCount(
		model.User{ID: 456, Language: "ja", ProficiencyLevel: "N5"},
		nil,
		nil,
		pusher,
		2,
	)
	scheduler.services.StudySession = service.NewStudySessionService(
		materialRepo,
		&schedulerSessionStoreStub{nextID: 202},
		schedulerSessionMaterialStoreStub{},
	)

	if err := scheduler.buildAndPushStudySessions(context.Background(), service.StudyProfileEvening); err != nil {
		t.Fatalf("buildAndPushStudySessions failed: %v", err)
	}
	want := model.StudySessionPlan{Quotas: []model.StudyMaterialQuota{
		{Category: model.MaterialCategoryVocabulary, NewCount: 4, ReviewCount: 14},
		{Category: model.MaterialCategoryGrammar, NewCount: 1, ReviewCount: 3},
		{Category: model.MaterialCategoryReading, NewCount: 0, ReviewCount: 2},
	}}
	if !reflect.DeepEqual(materialRepo.plan, want) {
		t.Fatalf("plan = %+v, want %+v", materialRepo.plan, want)
	}
	if len(pusher.studyCalls) != 1 || pusher.studyCalls[0].sessionID != 202 {
		t.Fatalf("study push calls = %+v, want session 202", pusher.studyCalls)
	}
}

func TestStartRunsStudyJobsWithDistinctProfiles(t *testing.T) {
	cronScheduler := cron.New()
	materialRepo := &schedulerMaterialRepoRecorder{}
	scheduler := newSchedulerForReminderTestWithCount(
		model.User{ID: 456, Language: "ja", ProficiencyLevel: "N5"},
		nil,
		nil,
		&schedulerPusherStub{},
		2,
	)
	scheduler.cfg.Schedule.StudyPushCron = "0 12 * * *"
	scheduler.cfg.Schedule.AfternoonStudyPushCron = "30 16 * * *"
	scheduler.cron = cronScheduler
	scheduler.services.StudySession = service.NewStudySessionService(
		materialRepo,
		&schedulerSessionStoreStub{nextID: 202},
		schedulerSessionMaterialStoreStub{},
	)

	scheduler.Start()
	defer scheduler.Stop()
	entries := cronScheduler.Entries()
	if len(entries) != 2 {
		t.Fatalf("registered study entries = %d, want 2", len(entries))
	}
	sort.Slice(entries, func(i, j int) bool { return entries[i].ID < entries[j].ID })
	for _, entry := range entries {
		entry.Job.Run()
	}

	wantMorning := model.StudySessionPlan{Quotas: []model.StudyMaterialQuota{
		{Category: model.MaterialCategoryVocabulary, NewCount: 8, ReviewCount: 7},
		{Category: model.MaterialCategoryGrammar, NewCount: 1, ReviewCount: 3},
		{Category: model.MaterialCategoryReading, NewCount: 1, ReviewCount: 0},
	}}
	wantEvening := model.StudySessionPlan{Quotas: []model.StudyMaterialQuota{
		{Category: model.MaterialCategoryVocabulary, NewCount: 4, ReviewCount: 14},
		{Category: model.MaterialCategoryGrammar, NewCount: 1, ReviewCount: 3},
		{Category: model.MaterialCategoryReading, NewCount: 0, ReviewCount: 2},
	}}
	if len(materialRepo.plans) != 2 ||
		!reflect.DeepEqual(materialRepo.plans[0], wantMorning) ||
		!reflect.DeepEqual(materialRepo.plans[1], wantEvening) {
		t.Fatalf("study plans = %+v, want morning then evening", materialRepo.plans)
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
