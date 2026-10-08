package scheduler

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/robfig/cron/v3"

	"github.com/lsj/copylingo/internal/model"
	"github.com/lsj/copylingo/internal/observability"
)

// userPushCron matches the 30-minute choices in each user's schedule settings.
const userPushCron = "*/30 * * * *"

// Scheduler runs the periodic user-session dispatch.
type Scheduler struct {
	user       slotUsers
	session    slotSessions
	tip        tipTopUp
	audio      audioTopUp
	cron       *cron.Cron
	dispatcher *sessionDispatcher
}

// slotUsers finds the timezones in use and the users due at a slot time.
type slotUsers interface {
	GetActiveTimezones(ctx context.Context) ([]string, error)
	GetUsersBySlot(
		ctx context.Context,
		slot model.SessionSlot,
		localTime string,
		timezone string,
	) ([]model.User, error)
}

// slotSessions counts unfinished sessions for the backlog limit and serves the
// dispatcher's build/remind calls.
type slotSessions interface {
	CountUnfinishedBatch(
		ctx context.Context,
		userIDs []int64,
	) (map[int64]int, error)
	dispatchSessions
}

// tipTopUp fills a (language, level) tip bucket.
type tipTopUp interface {
	TopUpBucket(
		ctx context.Context,
		language,
		level string,
	) error
}

// audioTopUp fills a (language, level) bucket's missing listening clips.
type audioTopUp interface {
	TopUpAudio(
		ctx context.Context,
		language,
		level string,
	) error
}

// quizPusher sends the "Quiz session arrived" message (bot SessionFlow).
type quizPusher interface {
	PushSession(
		ctx context.Context,
		chatID int64,
		sessionID int,
		sessionType string,
	) error
}

// studyPusher sends the "Study session arrived" message (bot StudyFlow).
type studyPusher interface {
	PushSession(
		ctx context.Context,
		chatID int64,
		sessionID int,
	) error
}

type pushClaims interface {
	TryClaim(
		ctx context.Context,
		userID int64,
		slot model.SessionSlot,
		today string,
	) (bool, error)
}

// Deps wires Scheduler with the service calls it makes. Audio is optional:
// it stays nil without a TTS key and the audio top-up is skipped.
type Deps struct {
	User        slotUsers
	Session     slotSessions
	Tip         tipTopUp
	Audio       audioTopUp
	QuizPusher  quizPusher
	StudyPusher studyPusher
	Cron        *cron.Cron
	Claims      pushClaims
}

func New(deps Deps) *Scheduler {
	s := &Scheduler{
		user:    deps.User,
		session: deps.Session,
		tip:     deps.Tip,
		audio:   deps.Audio,
		cron:    deps.Cron,
	}
	s.dispatcher = newSessionDispatcher(
		deps.Session,
		deps.QuizPusher,
		deps.StudyPusher,
		deps.Claims,
	)
	return s
}

// Start registers the 30-minute user dispatch and starts the scheduler.
func (s *Scheduler) Start() {
	if _, err := s.cron.AddFunc(
		userPushCron,
		func() {
			s.runJob(
				"dynamic_user_push",
				10*time.Minute,
				s.tick,
			)
		},
	); err != nil {
		slog.Error(
			"Failed to register scheduler job",
			"event",
			"scheduler.job.registration_failed",
			"source",
			"scheduler",
			"job",
			"dynamic_user_push",
			"error",
			err,
		)
	} else {
		slog.Info(
			"Scheduler job registered",
			"event",
			"scheduler.job.registered",
			"source",
			"scheduler",
			"job",
			"dynamic_user_push",
			"cron",
			userPushCron,
		)
	}

	s.cron.Start()
	slog.Info(
		"Scheduler started",
		"event",
		"scheduler.started",
		"source",
		"scheduler",
	)
}

// Stop gracefully stops the scheduler.
func (s *Scheduler) Stop() {
	s.cron.Stop()
	if s.dispatcher != nil && s.dispatcher.limiter != nil {
		s.dispatcher.limiter.Stop()
	}
	slog.Info(
		"Scheduler stopped",
		"event",
		"scheduler.stopped",
		"source",
		"scheduler",
	)
}

func (s *Scheduler) runJob(
	name string,
	timeout time.Duration,
	run func(context.Context) error,
) {
	ctx := observability.WithAttrs(
		context.Background(),
		slog.String(
			"interaction_id",
			observability.NewInteractionID("job-"+name),
		),
		slog.String(
			"source",
			"scheduler",
		),
		slog.String(
			"job",
			name,
		),
	)
	if timeout > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(
			ctx,
			timeout,
		)
		defer cancel()
	}
	startedAt := time.Now()
	slog.InfoContext(
		ctx,
		"Scheduler job started",
		"event",
		"scheduler.job.started",
	)
	if err := run(ctx); err != nil {
		slog.ErrorContext(
			ctx,
			"Scheduler job failed",
			"event",
			"scheduler.job.failed",
			"duration_ms",
			time.Since(startedAt).Milliseconds(),
			"error",
			err,
		)
		return
	}
	slog.InfoContext(
		ctx,
		"Scheduler job completed",
		"event",
		"scheduler.job.completed",
		"duration_ms",
		time.Since(startedAt).Milliseconds(),
	)
}

// tick executes 30-minute interval dynamic dispatch across distinct timezones and slots.
func (s *Scheduler) tick(ctx context.Context) error {
	timezones, err := s.user.GetActiveTimezones(ctx)
	if err != nil {
		slog.WarnContext(
			ctx,
			"Failed to get active timezones, using Asia/Seoul fallback",
			"error",
			err,
		)
		timezones = []string{"Asia/Seoul"}
	}
	if len(timezones) == 0 {
		timezones = []string{"Asia/Seoul"}
	}

	slots := []model.SessionSlot{
		model.SessionSlotMorningStudy,
		model.SessionSlotMorningQuiz,
		model.SessionSlotEveningStudy,
		model.SessionSlotEveningQuiz,
	}

	var allUsers []model.User
	var failures int

	for _, tz := range timezones {
		loc, err := time.LoadLocation(tz)
		if err != nil {
			slog.WarnContext(
				ctx,
				"Failed to load timezone",
				"timezone",
				tz,
				"error",
				err,
			)
			continue
		}

		nowLocal := time.Now().In(loc)
		minuteSlot := (nowLocal.Minute() / 30) * 30
		localTime := fmt.Sprintf(
			"%02d:%02d",
			nowLocal.Hour(),
			minuteSlot,
		)

		for _, slot := range slots {
			users, err := s.user.GetUsersBySlot(
				ctx,
				slot,
				localTime,
				tz,
			)
			if err != nil {
				failures++
				slog.ErrorContext(
					ctx,
					"Failed to query users for slot",
					"event",
					"scheduler.slot.query_failed",
					"slot",
					slot,
					"timezone",
					tz,
					"time",
					localTime,
					"error",
					err,
				)
				continue
			}
			if len(users) == 0 {
				continue
			}

			allUsers = append(
				allUsers,
				users...,
			)

			userIDs := make(
				[]int64,
				len(users),
			)
			for i, u := range users {
				userIDs[i] = u.ID
			}

			var unfinishedCounts map[int64]int
			counts, err := s.session.CountUnfinishedBatch(
				ctx,
				userIDs,
			)
			if err != nil {
				slog.WarnContext(
					ctx,
					"Failed to batch count unfinished sessions",
					"error",
					err,
				)
			} else {
				unfinishedCounts = counts
			}

			if err := s.dispatcher.dispatchBatch(
				ctx,
				slot,
				users,
				unfinishedCounts,
			); err != nil {
				failures++
			}
		}
	}

	if len(allUsers) > 0 {
		s.topUpTips(
			ctx,
			allUsers,
		)
		s.topUpAudio(
			ctx,
			allUsers,
		)
	}

	if failures > 0 {
		return fmt.Errorf(
			"%d slot operations failed during tick",
			failures,
		)
	}
	return nil
}

// langLevelPair identifies a distinct (language, proficiency level) bucket.
type langLevelPair struct {
	Language string
	Level    string
}

// distinctLangLevelPairs collapses the user slice into the unique set of
// (language, level) pairs in a single pass.
func distinctLangLevelPairs(users []model.User) []langLevelPair {
	seen := make(
		map[langLevelPair]struct{},
		len(users),
	)
	pairs := make(
		[]langLevelPair,
		0,
		len(users),
	)
	for _, user := range users {
		p := langLevelPair{Language: user.Language, Level: user.ProficiencyLevel}
		if _, ok := seen[p]; ok {
			continue
		}
		seen[p] = struct{}{}
		pairs = append(
			pairs,
			p,
		)
	}
	return pairs
}

// topUpTips fills each distinct (language, level) tip bucket toward the target.
func (s *Scheduler) topUpTips(
	ctx context.Context,
	users []model.User,
) {
	for _, p := range distinctLangLevelPairs(users) {
		if err := s.tip.TopUpBucket(
			ctx,
			p.Language,
			p.Level,
		); err != nil {
			slog.WarnContext(
				ctx,
				"Tip top-up failed",
				"event",
				"scheduler.tip.topup_failed",
				"language",
				p.Language,
				"level",
				p.Level,
				"error",
				err,
			)
			continue
		}
	}
}

// topUpAudio fills each distinct (language, level) bucket's missing listening clips.
func (s *Scheduler) topUpAudio(
	ctx context.Context,
	users []model.User,
) {
	if s.audio == nil {
		return
	}
	for _, p := range distinctLangLevelPairs(users) {
		if err := s.audio.TopUpAudio(
			ctx,
			p.Language,
			p.Level,
		); err != nil {
			slog.WarnContext(
				ctx,
				"Listening audio top-up failed",
				"event",
				"scheduler.audio.topup_failed",
				"language",
				p.Language,
				"level",
				p.Level,
				"error",
				err,
			)
			continue
		}
	}
}
