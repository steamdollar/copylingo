package service

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/lsj/copylingo/internal/config"
	"github.com/lsj/copylingo/internal/model"
)

const activeSessionWorkingSetTTL = 24 * time.Hour

var (
	ErrActiveSessionNotFound          = errors.New("active session state not found")
	ErrActiveSessionCorrupt           = errors.New("active session state is corrupt")
	ErrActiveSessionQuestionNotFound  = errors.New("active session question not found")
	ErrActiveSessionAlreadyAnswered   = errors.New("active session question already answered")
	ErrActiveSessionIncomplete        = errors.New("active session is not fully answered")
	ErrActiveSessionUserMismatch      = errors.New("active session user mismatch")
	ErrActiveSessionDependencyMissing = errors.New("active session dependency missing")
)

type activeSessionRepository interface {
	LoadActiveSession(ctx context.Context, sessionID int) (*model.ActiveSessionState, error)
	FlushActiveSession(ctx context.Context, state *model.ActiveSessionState) error
}

type activeSessionScheduler interface {
	ScheduleAnswer(question *model.Question, isCorrect bool)
}

// SessionWrongAnswer contains enough data to render a completed wrong-answer summary without DB reads.
type SessionWrongAnswer struct {
	SessionQuestion model.SessionQuestion
	Question        model.Question
}

// SessionResult contains the summary of a completed session.
type SessionResult struct {
	TotalQuestions int
	CorrectCount   int
	WrongAnswers   []SessionWrongAnswer
}

// ActiveSessionService owns the Redis working set for in-progress learning sessions.
type ActiveSessionService struct {
	repo  activeSessionRepository
	store *workingSetStore[model.ActiveSessionState]
	srs   activeSessionScheduler
}

func NewActiveSessionService(
	repo activeSessionRepository,
	rdb workingSetRedis,
	srs activeSessionScheduler,
) *ActiveSessionService {
	return &ActiveSessionService{
		repo: repo,
		store: newWorkingSetStore[model.ActiveSessionState](
			rdb,
			activeSessionWorkingSetKey,
			activeSessionWorkingSetTTL,
			validateActiveSessionState,
			workingSetErrors{
				DependencyMissing: ErrActiveSessionDependencyMissing,
				NotFound:          ErrActiveSessionNotFound,
				Corrupt:           ErrActiveSessionCorrupt,
			},
		),
		srs: srs,
	}
}

func (s *ActiveSessionService) CreateFromDB(ctx context.Context, sessionID int) (*model.ActiveSessionState, error) {
	if s.repo == nil {
		return nil, ErrActiveSessionDependencyMissing
	}

	// retrieve target session from db (session - sessionQuestions - questions)
	state, err := s.repo.LoadActiveSession(ctx, sessionID)
	if err != nil {
		return nil, fmt.Errorf("create active session from db session_id=%d: %w", sessionID, err)
	}
	state.Version = model.ActiveSessionStateVersion
	state.UpdatedAt = time.Now()
	state.RecountAnswered()
	state.CurrentIndex = state.NextUnansweredIndex()

	// set at redis
	if err := s.save(ctx, state); err != nil {
		return nil, fmt.Errorf("store active session working set session_id=%d: %w", sessionID, err)
	}
	return state, nil
}

// Get retrieves the active session working set from Redis. If not found, it attempts to recover from DB and store in Redis.
func (s *ActiveSessionService) Get(ctx context.Context, sessionID int) (*model.ActiveSessionState, error) {
	state, err := s.store.get(ctx, sessionID)
	if err != nil {
		if errors.Is(err, ErrActiveSessionNotFound) {
			state, err := s.CreateFromDB(ctx, sessionID)
			if err != nil {
				return nil, fmt.Errorf("%w session_id=%d: %v", ErrActiveSessionNotFound, sessionID, err)
			}
			return state, nil
		}
		return nil, err
	}

	state.RecountAnswered()
	return state, nil
}

func (s *ActiveSessionService) SetCurrentIndex(ctx context.Context, sessionID, idx int) error {
	state, err := s.Get(ctx, sessionID)
	if err != nil {
		return err
	}
	if idx < 0 || idx > len(state.Items) {
		return fmt.Errorf("set active session current index session_id=%d idx=%d: %w",
			sessionID, idx, ErrActiveSessionQuestionNotFound)
	}
	state.CurrentIndex = idx
	state.UpdatedAt = time.Now()
	return s.save(ctx, state)
}

func (s *ActiveSessionService) RecordAnswer(
	ctx context.Context,
	sessionID, questionID int,
	userAnswer string,
	isCorrect bool,
) error {
	state, err := s.Get(ctx, sessionID)
	if err != nil {
		return err
	}

	item, idx, ok := state.CurrentItemByQuestionID(questionID)
	if !ok {
		return fmt.Errorf("%w session_id=%d question_id=%d", ErrActiveSessionQuestionNotFound, sessionID, questionID)
	}
	if item.SessionQuestion.IsCorrect != nil {
		return fmt.Errorf("%w session_id=%d question_id=%d", ErrActiveSessionAlreadyAnswered, sessionID, questionID)
	}

	answer := userAnswer
	correct := isCorrect
	state.Items[idx].SessionQuestion.UserAnswer = &answer
	state.Items[idx].SessionQuestion.IsCorrect = &correct
	state.Items[idx].Question.TimesServed++
	if isCorrect {
		state.Items[idx].Question.TimesCorrect++
	}
	if s.srs == nil {
		return ErrActiveSessionDependencyMissing
	}
	s.srs.ScheduleAnswer(&state.Items[idx].Question, isCorrect)

	state.CurrentIndex = idx
	state.UpdatedAt = time.Now()
	state.RecountAnswered()

	return s.save(ctx, state)
}

func (s *ActiveSessionService) Flush(ctx context.Context, sessionID int, userID int64) (*SessionResult, error) {
	state, err := s.Get(ctx, sessionID)
	if err != nil {
		return nil, err
	}
	if state.Session.UserID != userID {
		return nil, fmt.Errorf("%w session_id=%d user_id=%d", ErrActiveSessionUserMismatch, sessionID, userID)
	}
	if state.NextUnansweredIndex() != len(state.Items) {
		return nil, fmt.Errorf("%w session_id=%d", ErrActiveSessionIncomplete, sessionID)
	}
	if s.repo == nil {
		return nil, ErrActiveSessionDependencyMissing
	}

	state.Session.CorrectCount = state.CorrectCount()
	if err := s.repo.FlushActiveSession(ctx, state); err != nil {
		return nil, fmt.Errorf("flush active session state session_id=%d: %w", sessionID, err)
	}

	return sessionResultFromState(state), nil
}

func (s *ActiveSessionService) Delete(ctx context.Context, sessionID int) error {
	if err := s.store.delete(ctx, sessionID); err != nil {
		return fmt.Errorf("delete active session working set session_id=%d: %w", sessionID, err)
	}
	return nil
}

func (s *ActiveSessionService) save(ctx context.Context, state *model.ActiveSessionState) error {
	if err := s.store.save(ctx, state.Session.ID, state); err != nil {
		return fmt.Errorf("set active session working set session_id=%d: %w", state.Session.ID, err)
	}
	return nil
}

func sessionResultFromState(state *model.ActiveSessionState) *SessionResult {
	wrongItems := state.WrongAnswers()
	wrongAnswers := make([]SessionWrongAnswer, 0, len(wrongItems))
	for _, item := range wrongItems {
		wrongAnswers = append(wrongAnswers, SessionWrongAnswer{
			SessionQuestion: item.SessionQuestion,
			Question:        item.Question,
		})
	}

	return &SessionResult{
		TotalQuestions: len(state.Items),
		CorrectCount:   state.CorrectCount(),
		WrongAnswers:   wrongAnswers,
	}
}

func activeSessionWorkingSetKey(sessionID int) string {
	return config.ActiveSessionWorkingSetRedisKey.Format(sessionID)
}

func validateActiveSessionState(state *model.ActiveSessionState, sessionID int) error {
	if state.Version != model.ActiveSessionStateVersion || state.Session.ID != sessionID {
		return ErrActiveSessionCorrupt
	}
	return nil
}
