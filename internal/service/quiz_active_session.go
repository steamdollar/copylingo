package service

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/lsj/copylingo/internal/model"
)

var (
	ErrQuizActiveSessionQuestionNotFound  = errors.New("active session question not found")
	ErrQuizActiveSessionAlreadyAnswered   = errors.New("active session question already answered")
	ErrQuizActiveSessionIncomplete        = errors.New("active session is not fully answered")
	ErrQuizActiveSessionUserMismatch      = errors.New("active session user mismatch")
	ErrQuizActiveSessionDependencyMissing = errors.New("active session dependency missing")
)

type QuizSessionStore interface {
	Load(
		ctx context.Context,
		sessionID int,
	) (*model.QuizActiveSessionState, error)
	Save(
		ctx context.Context,
		sessionID int,
		state *model.QuizActiveSessionState,
	) error
	Delete(
		ctx context.Context,
		sessionID int,
	) error
}

// QuizSessionWrongAnswer contains enough data to render a completed wrong-answer summary without DB reads.
type QuizSessionWrongAnswer struct {
	SessionQuestion model.SessionQuestion
	Question        model.Question
}

// QuizSessionResult contains the summary of a completed session.
type QuizSessionResult struct {
	TotalQuestions int
	CorrectCount   int
	WrongAnswers   []QuizSessionWrongAnswer
}

// quizActiveSessionService owns the Redis working set for in-progress learning sessions.
type quizActiveSessionService struct {
	repo  QuizActiveSessionRepo
	store QuizSessionStore
	srs   *srsService
}

func newQuizActiveSessionService(
	repo QuizActiveSessionRepo,
	store QuizSessionStore,
	srs *srsService,
) *quizActiveSessionService {
	return &quizActiveSessionService{
		repo:  repo,
		store: store,
		srs:   srs,
	}
}

func (s *quizActiveSessionService) CreateFromDB(
	ctx context.Context,
	sessionID int,
) (*model.QuizActiveSessionState, error) {
	if s.repo == nil {
		return nil, ErrQuizActiveSessionDependencyMissing
	}

	// retrieve target session from db (session - sessionQuestions - questions)
	state, err := s.repo.LoadQuestionSessionWithStateBySessionID(
		ctx,
		sessionID,
	)
	if err != nil {
		return nil, fmt.Errorf(
			"create active session from db session_id=%d: %w",
			sessionID,
			err,
		)
	}
	state.Version = model.QuizActiveSessionStateVersion
	state.UpdatedAt = time.Now()
	state.RecountAnswered()
	state.CurrentIndex = state.NextUnansweredIndex()

	for i := range state.Items {
		if err := state.Items[i].Question.ShuffleOptions(sessionID); err != nil {
			return nil, fmt.Errorf(
				"shuffle options session_id=%d question_id=%d: %w",
				sessionID,
				state.Items[i].Question.ID,
				err,
			)
		}
	}

	// set at redis
	if err := s.save(
		ctx,
		state,
	); err != nil {
		return nil, fmt.Errorf(
			"store active session working set session_id=%d: %w",
			sessionID,
			err,
		)
	}
	return state, nil
}

// Get retrieves the active session working set from Redis. If not found, it attempts to recover from DB and store in Redis.
func (s *quizActiveSessionService) Get(
	ctx context.Context,
	sessionID int,
) (*model.QuizActiveSessionState, error) {
	if s.store == nil {
		return nil, ErrQuizActiveSessionDependencyMissing
	}
	state, err := s.store.Load(
		ctx,
		sessionID,
	)
	if err != nil {
		if errors.Is(
			err,
			model.ErrSessionStoreNotFound,
		) {
			state, err := s.CreateFromDB(
				ctx,
				sessionID,
			)
			if err != nil {
				return nil, fmt.Errorf(
					"%w session_id=%d: %v",
					model.ErrSessionStoreNotFound,
					sessionID,
					err,
				)
			}
			return state, nil
		}
		return nil, err
	}

	state.RecountAnswered()
	return state, nil
}

func (s *quizActiveSessionService) RecordAnswer(
	ctx context.Context,
	sessionID,
	questionID int,
	userAnswer string,
	isCorrect bool,
) error {
	state, err := s.Get(
		ctx,
		sessionID,
	)
	if err != nil {
		return err
	}

	item, idx, ok := state.CurrentItemByQuestionID(questionID)
	if !ok {
		return fmt.Errorf(
			"%w session_id=%d question_id=%d",
			ErrQuizActiveSessionQuestionNotFound,
			sessionID,
			questionID,
		)
	}
	if item.SessionQuestion.IsCorrect != nil {
		return fmt.Errorf(
			"%w session_id=%d question_id=%d",
			ErrQuizActiveSessionAlreadyAnswered,
			sessionID,
			questionID,
		)
	}

	answer := userAnswer
	correct := isCorrect
	state.Items[idx].SessionQuestion.UserAnswer = &answer
	state.Items[idx].SessionQuestion.IsCorrect = &correct
	state.Items[idx].Progress.TimesServed++
	if isCorrect {
		state.Items[idx].Progress.TimesCorrect++
	}
	if s.srs == nil {
		return ErrQuizActiveSessionDependencyMissing
	}
	s.srs.ScheduleAnswer(
		&state.Items[idx].Progress,
		isCorrect,
	)

	state.CurrentIndex = idx
	state.UpdatedAt = time.Now()
	state.RecountAnswered()

	return s.save(
		ctx,
		state,
	)
}

func (s *quizActiveSessionService) Flush(
	ctx context.Context,
	sessionID int,
	userID int64,
) (*QuizSessionResult, error) {
	state, err := s.Get(
		ctx,
		sessionID,
	)
	if err != nil {
		return nil, err
	}
	if state.Session.UserID != userID {
		return nil, fmt.Errorf(
			"%w session_id=%d user_id=%d",
			ErrQuizActiveSessionUserMismatch,
			sessionID,
			userID,
		)
	}
	if state.NextUnansweredIndex() != len(state.Items) {
		return nil, fmt.Errorf(
			"%w session_id=%d",
			ErrQuizActiveSessionIncomplete,
			sessionID,
		)
	}
	if s.repo == nil {
		return nil, ErrQuizActiveSessionDependencyMissing
	}

	state.Session.CorrectCount = state.CorrectCount()
	if err := s.repo.FlushQuizActiveSession(
		ctx,
		state,
	); err != nil {
		return nil, fmt.Errorf(
			"flush active session state session_id=%d: %w",
			sessionID,
			err,
		)
	}

	return quizSessionResultFromState(state), nil
}

func (s *quizActiveSessionService) Delete(
	ctx context.Context,
	sessionID int,
) error {
	if s.store == nil {
		return ErrQuizActiveSessionDependencyMissing
	}
	if err := s.store.Delete(
		ctx,
		sessionID,
	); err != nil {
		return fmt.Errorf(
			"delete active session working set session_id=%d: %w",
			sessionID,
			err,
		)
	}
	return nil
}

func (s *quizActiveSessionService) save(
	ctx context.Context,
	state *model.QuizActiveSessionState,
) error {
	if s.store == nil {
		return ErrQuizActiveSessionDependencyMissing
	}
	if err := s.store.Save(
		ctx,
		state.Session.ID,
		state,
	); err != nil {
		return fmt.Errorf(
			"set active session working set session_id=%d: %w",
			state.Session.ID,
			err,
		)
	}
	return nil
}

func quizSessionResultFromState(state *model.QuizActiveSessionState) *QuizSessionResult {
	wrongItems := state.WrongAnswers()
	wrongAnswers := make(
		[]QuizSessionWrongAnswer,
		0,
		len(wrongItems),
	)
	for _, item := range wrongItems {
		wrongAnswers = append(
			wrongAnswers,
			QuizSessionWrongAnswer{
				SessionQuestion: item.SessionQuestion,
				Question:        item.Question,
			},
		)
	}

	return &QuizSessionResult{
		TotalQuestions: len(state.Items),
		CorrectCount:   state.CorrectCount(),
		WrongAnswers:   wrongAnswers,
	}
}
