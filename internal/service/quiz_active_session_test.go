package service

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"testing"

	"github.com/lsj/copylingo/internal/model"
)

type fakeQuizSessionStore struct {
	values map[int]*model.QuizActiveSessionState
	getErr error
	setErr error
	delErr error
}

func snapshotSessionTestState[T any](state *T) (*T, error) {
	raw, err := json.Marshal(state)
	if err != nil {
		return nil, fmt.Errorf(
			"marshal session test snapshot: %w",
			err,
		)
	}
	var snapshot T
	if err := json.Unmarshal(
		raw,
		&snapshot,
	); err != nil {
		return nil, fmt.Errorf(
			"unmarshal session test snapshot: %w",
			err,
		)
	}
	return &snapshot, nil
}

func newFakeQuizSessionStore() *fakeQuizSessionStore {
	return &fakeQuizSessionStore{values: make(map[int]*model.QuizActiveSessionState)}
}

func (f *fakeQuizSessionStore) Load(
	_ context.Context,
	sessionID int,
) (*model.QuizActiveSessionState, error) {
	if f.getErr != nil {
		return nil, f.getErr
	}
	state, ok := f.values[sessionID]
	if !ok {
		return nil, model.ErrSessionStoreNotFound
	}
	return snapshotSessionTestState(state)
}

func (f *fakeQuizSessionStore) Save(
	_ context.Context,
	sessionID int,
	state *model.QuizActiveSessionState,
) error {
	if f.setErr != nil {
		return f.setErr
	}
	snapshot, err := snapshotSessionTestState(state)
	if err != nil {
		return err
	}
	f.values[sessionID] = snapshot
	return nil
}

func (f *fakeQuizSessionStore) Delete(
	_ context.Context,
	sessionID int,
) error {
	if f.delErr != nil {
		return f.delErr
	}
	delete(
		f.values,
		sessionID,
	)
	return nil
}

type fakeQuizActiveSessionRepo struct {
	loadFn func(
		ctx context.Context,
		sessionID int,
	) (*model.QuizActiveSessionState, error)
	flushFn func(
		ctx context.Context,
		state *model.QuizActiveSessionState,
	) error
}

func (f *fakeQuizActiveSessionRepo) LoadQuestionSessionWithStateBySessionID(
	ctx context.Context,
	sessionID int,
) (*model.QuizActiveSessionState, error) {
	return f.loadFn(
		ctx,
		sessionID,
	)
}

func (f *fakeQuizActiveSessionRepo) FlushQuizActiveSession(
	ctx context.Context,
	state *model.QuizActiveSessionState,
) error {
	return f.flushFn(
		ctx,
		state,
	)
}

func TestQuizActiveSessionCreateFromDBAndGet(t *testing.T) {
	ctx := context.Background()
	sessionID := 10
	store := newFakeQuizSessionStore()
	repo := &fakeQuizActiveSessionRepo{
		loadFn: func(
			ctx context.Context,
			sid int,
		) (*model.QuizActiveSessionState, error) {
			if sid != sessionID {
				t.Fatalf(
					"unexpected session id %d",
					sid,
				)
			}
			// One answered, one unanswered
			state := quizActiveSessionTestState(
				sessionID,
				123,
				true,
			)
			state.Items = append(
				state.Items,
				model.QuizActiveSessionQuestion{
					SessionQuestion: model.SessionQuestion{ID: 101, QuestionID: 2, IsCorrect: nil},
					Question:        model.Question{ID: 2},
				},
			)
			return state, nil
		},
	}
	svc := NewQuizActiveSessionService(
		repo,
		store,
		NewSRSService(nil),
	)

	if _, err := svc.CreateFromDB(
		ctx,
		sessionID,
	); err != nil {
		t.Fatalf(
			"CreateFromDB failed: %v",
			err,
		)
	}
	got, err := svc.Get(
		ctx,
		sessionID,
	)
	if err != nil {
		t.Fatalf(
			"Get failed: %v",
			err,
		)
	}
	if got.Session.ID != sessionID || len(got.Items) != 2 {
		t.Fatalf(
			"unexpected state: %+v",
			got,
		)
	}
	if got.CurrentIndex != 1 {
		t.Fatalf(
			"expected CurrentIndex 1 (first unanswered), got %d",
			got.CurrentIndex,
		)
	}
}

func TestQuizActiveSessionCreateFromDB_ShufflesQuestionOptions(t *testing.T) {
	ctx := context.Background()
	sessionID := 101
	store := newFakeQuizSessionStore()

	rawOptions := json.RawMessage(`["A","B","C","D","E"]`)
	repo := &fakeQuizActiveSessionRepo{
		loadFn: func(
			ctx context.Context,
			sid int,
		) (*model.QuizActiveSessionState, error) {
			state := quizActiveSessionTestState(
				sid,
				123,
				false,
			)
			state.Items[0].Question.ID = 42
			state.Items[0].Question.Options = rawOptions
			return state, nil
		},
	}
	svc := NewQuizActiveSessionService(
		repo,
		store,
		nil,
	)

	created, err := svc.CreateFromDB(
		ctx,
		sessionID,
	)
	if err != nil {
		t.Fatalf(
			"CreateFromDB failed: %v",
			err,
		)
	}

	opts, err := created.Items[0].Question.GetOptions()
	if err != nil {
		t.Fatalf(
			"GetOptions failed: %v",
			err,
		)
	}
	if len(opts) != 5 {
		t.Fatalf(
			"expected 5 options, got %d",
			len(opts),
		)
	}

	// Verify deterministic: same sessionID + questionID yields identical order
	store2 := newFakeQuizSessionStore()
	svc2 := NewQuizActiveSessionService(
		repo,
		store2,
		nil,
	)
	created2, err := svc2.CreateFromDB(
		ctx,
		sessionID,
	)
	if err != nil {
		t.Fatalf(
			"CreateFromDB 2 failed: %v",
			err,
		)
	}
	opts2, _ := created2.Items[0].Question.GetOptions()
	for i := range opts {
		if opts[i] != opts2[i] {
			t.Fatalf(
				"expected identical order for same session, mismatch at %d: %s vs %s",
				i,
				opts[i],
				opts2[i],
			)
		}
	}

	// Verify different sessionID yields different permutation across sessions
	differentPermutationFound := false
	for nextSID := sessionID + 1; nextSID <= sessionID+20; nextSID++ {
		storeN := newFakeQuizSessionStore()
		svcN := NewQuizActiveSessionService(
			repo,
			storeN,
			nil,
		)
		createdN, err := svcN.CreateFromDB(
			ctx,
			nextSID,
		)
		if err != nil {
			t.Fatalf(
				"CreateFromDB N failed: %v",
				err,
			)
		}
		if string(createdN.Items[0].Question.Options) != string(created.Items[0].Question.Options) {
			differentPermutationFound = true
			break
		}
	}
	if !differentPermutationFound {
		t.Fatal("expected at least one different permutation across 20 sessions")
	}
}

func TestQuizActiveSessionGetAutoRecover(t *testing.T) {
	ctx := context.Background()
	sessionID := 10
	store := newFakeQuizSessionStore() // Empty Redis
	repo := &fakeQuizActiveSessionRepo{
		loadFn: func(
			ctx context.Context,
			sid int,
		) (*model.QuizActiveSessionState, error) {
			return quizActiveSessionTestState(
				sessionID,
				123,
				false,
			), nil
		},
	}
	svc := NewQuizActiveSessionService(
		repo,
		store,
		nil,
	)

	// Get should trigger auto-recovery because it's missing in Redis
	got, err := svc.Get(
		ctx,
		sessionID,
	)
	if err != nil {
		t.Fatalf(
			"Get failed to auto-recover: %v",
			err,
		)
	}
	if got.Session.ID != sessionID {
		t.Fatalf(
			"expected session id %d, got %d",
			sessionID,
			got.Session.ID,
		)
	}

	// Verify it's now in Redis
	stored, err := store.Load(
		ctx,
		sessionID,
	)
	if err != nil {
		t.Fatalf(
			"expected state in Redis after auto-recovery: %v",
			err,
		)
	}
	if stored == nil {
		t.Fatal("expected state in store after auto-recovery")
	}
}

func TestQuizActiveSessionGetMissing(t *testing.T) {
	ctx := context.Background()
	svc := NewQuizActiveSessionService(
		nil,
		newFakeQuizSessionStore(),
		nil,
	)

	_, err := svc.Get(
		ctx,
		10,
	)
	if !errors.Is(
		err,
		model.ErrSessionStoreNotFound,
	) {
		t.Fatalf(
			"expected ErrActiveSessionNotFound, got %v",
			err,
		)
	}
}

func TestQuizActiveSessionMissingStoreDependency(t *testing.T) {
	_, err := NewQuizActiveSessionService(
		nil,
		nil,
		nil,
	).Get(
		context.Background(),
		10,
	)
	if !errors.Is(
		err,
		ErrQuizActiveSessionDependencyMissing,
	) {
		t.Fatalf(
			"Get without store = %v, want ErrActiveSessionDependencyMissing",
			err,
		)
	}
}

func TestQuizActiveSessionRecordAnswerUpdatesProgressAndSRS(t *testing.T) {
	ctx := context.Background()
	sessionID := 10
	store := newFakeQuizSessionStore()
	svc := NewQuizActiveSessionService(
		nil,
		store,
		NewSRSService(nil),
	)
	if err := svc.save(
		ctx,
		quizActiveSessionTestState(
			sessionID,
			123,
			false,
		),
	); err != nil {
		t.Fatalf(
			"save failed: %v",
			err,
		)
	}

	if err := svc.RecordAnswer(
		ctx,
		sessionID,
		1,
		"apple",
		true,
	); err != nil {
		t.Fatalf(
			"RecordAnswer failed: %v",
			err,
		)
	}

	got, err := svc.Get(
		ctx,
		sessionID,
	)
	if err != nil {
		t.Fatalf(
			"Get failed: %v",
			err,
		)
	}
	item := got.Items[0]
	if got.AnsweredCount != 1 {
		t.Fatalf(
			"expected answered count 1, got %d",
			got.AnsweredCount,
		)
	}
	if item.SessionQuestion.UserAnswer == nil || *item.SessionQuestion.UserAnswer != "apple" {
		t.Fatalf(
			"unexpected answer: %+v",
			item.SessionQuestion.UserAnswer,
		)
	}
	if item.SessionQuestion.IsCorrect == nil || !*item.SessionQuestion.IsCorrect {
		t.Fatalf(
			"expected correct answer, got %+v",
			item.SessionQuestion.IsCorrect,
		)
	}
	if item.Progress.TimesServed != 1 || item.Progress.TimesCorrect != 1 {
		t.Fatalf(
			"expected stats to increment, got served=%d correct=%d",
			item.Progress.TimesServed,
			item.Progress.TimesCorrect,
		)
	}
	if item.Progress.NextReviewAt == nil || item.Progress.LastReviewedAt == nil {
		t.Fatal("expected SRS timestamps to be set")
	}
}

func TestQuizActiveSessionRecordAnswerRejectsDuplicate(t *testing.T) {
	ctx := context.Background()
	sessionID := 10
	store := newFakeQuizSessionStore()
	svc := NewQuizActiveSessionService(
		nil,
		store,
		NewSRSService(nil),
	)
	if err := svc.save(
		ctx,
		quizActiveSessionTestState(
			sessionID,
			123,
			true,
		),
	); err != nil {
		t.Fatalf(
			"save failed: %v",
			err,
		)
	}

	err := svc.RecordAnswer(
		ctx,
		sessionID,
		1,
		"apple",
		true,
	)
	if !errors.Is(
		err,
		ErrQuizActiveSessionAlreadyAnswered,
	) {
		t.Fatalf(
			"expected ErrActiveSessionAlreadyAnswered, got %v",
			err,
		)
	}
}

func TestQuizActiveSessionRecordAnswerUsesCurrentDuplicateOccurrence(t *testing.T) {
	ctx := context.Background()
	sessionID := 10
	store := newFakeQuizSessionStore()
	svc := NewQuizActiveSessionService(
		nil,
		store,
		NewSRSService(nil),
	)

	state := quizActiveSessionTestState(
		sessionID,
		123,
		true,
	)
	state.Items = append(
		state.Items,
		model.QuizActiveSessionQuestion{
			SessionQuestion: model.SessionQuestion{ID: 101, SessionID: sessionID, QuestionID: 1},
			Question: model.Question{
				ID:            1,
				Type:          model.QuestionMultipleChoice,
				CorrectAnswer: "apple",
			},
			Progress: model.NewUserQuestionProgress(
				123,
				1,
			),
		},
	)
	state.CurrentIndex = 1
	if err := svc.save(
		ctx,
		state,
	); err != nil {
		t.Fatalf(
			"save failed: %v",
			err,
		)
	}

	if err := svc.RecordAnswer(
		ctx,
		sessionID,
		1,
		"apple",
		true,
	); err != nil {
		t.Fatalf(
			"RecordAnswer failed: %v",
			err,
		)
	}

	got, err := svc.Get(
		ctx,
		sessionID,
	)
	if err != nil {
		t.Fatalf(
			"Get failed: %v",
			err,
		)
	}
	if got.Items[1].SessionQuestion.IsCorrect == nil || !*got.Items[1].SessionQuestion.IsCorrect {
		t.Fatalf(
			"expected current duplicate occurrence to be answered: %+v",
			got.Items[1].SessionQuestion,
		)
	}
}

func TestQuizActiveSessionRecordAnswerRejectsStaleDuplicateCallback(t *testing.T) {
	ctx := context.Background()
	sessionID := 10
	store := newFakeQuizSessionStore()
	svc := NewQuizActiveSessionService(
		nil,
		store,
		NewSRSService(nil),
	)

	state := quizActiveSessionTestState(
		sessionID,
		123,
		true,
	)
	state.Items = append(
		state.Items,
		model.QuizActiveSessionQuestion{
			SessionQuestion: model.SessionQuestion{ID: 101, SessionID: sessionID, QuestionID: 1},
			Question: model.Question{
				ID:            1,
				Type:          model.QuestionMultipleChoice,
				CorrectAnswer: "apple",
			},
			Progress: model.NewUserQuestionProgress(
				123,
				1,
			),
		},
	)
	if err := svc.save(
		ctx,
		state,
	); err != nil {
		t.Fatalf(
			"save failed: %v",
			err,
		)
	}

	err := svc.RecordAnswer(
		ctx,
		sessionID,
		1,
		"apple",
		true,
	)
	if !errors.Is(
		err,
		ErrQuizActiveSessionAlreadyAnswered,
	) {
		t.Fatalf(
			"expected ErrActiveSessionAlreadyAnswered, got %v",
			err,
		)
	}

	got, getErr := svc.Get(
		ctx,
		sessionID,
	)
	if getErr != nil {
		t.Fatalf(
			"Get failed: %v",
			getErr,
		)
	}
	if got.Items[1].SessionQuestion.IsCorrect != nil {
		t.Fatalf(
			"expected later duplicate occurrence to remain unanswered: %+v",
			got.Items[1].SessionQuestion,
		)
	}
}

func TestQuizActiveSessionFlushSuccess(t *testing.T) {
	ctx := context.Background()
	sessionID := 10
	userID := int64(123)
	store := newFakeQuizSessionStore()
	flushed := false
	repo := &fakeQuizActiveSessionRepo{
		flushFn: func(
			ctx context.Context,
			state *model.QuizActiveSessionState,
		) error {
			flushed = true
			if state.Session.CorrectCount != 1 {
				t.Fatalf(
					"expected correct count 1, got %d",
					state.Session.CorrectCount,
				)
			}
			return nil
		},
	}
	svc := NewQuizActiveSessionService(
		repo,
		store,
		NewSRSService(nil),
	)
	if err := svc.save(
		ctx,
		quizActiveSessionTestState(
			sessionID,
			userID,
			true,
		),
	); err != nil {
		t.Fatalf(
			"save failed: %v",
			err,
		)
	}

	result, err := svc.Flush(
		ctx,
		sessionID,
		userID,
	)
	if err != nil {
		t.Fatalf(
			"Flush failed: %v",
			err,
		)
	}
	if !flushed {
		t.Fatal("expected repository flush")
	}
	if result.TotalQuestions != 1 || result.CorrectCount != 1 {
		t.Fatalf(
			"unexpected result: %+v",
			result,
		)
	}
}

func TestQuizActiveSessionFlushRejectsIncomplete(t *testing.T) {
	ctx := context.Background()
	sessionID := 10
	userID := int64(123)
	store := newFakeQuizSessionStore()
	svc := NewQuizActiveSessionService(
		&fakeQuizActiveSessionRepo{},
		store,
		NewSRSService(nil),
	)
	if err := svc.save(
		ctx,
		quizActiveSessionTestState(
			sessionID,
			userID,
			false,
		),
	); err != nil {
		t.Fatalf(
			"save failed: %v",
			err,
		)
	}

	_, err := svc.Flush(
		ctx,
		sessionID,
		userID,
	)
	if !errors.Is(
		err,
		ErrQuizActiveSessionIncomplete,
	) {
		t.Fatalf(
			"expected ErrActiveSessionIncomplete, got %v",
			err,
		)
	}
}

func TestQuizActiveSessionFlushRejectsUserMismatch(t *testing.T) {
	ctx := context.Background()
	sessionID := 10
	store := newFakeQuizSessionStore()
	svc := NewQuizActiveSessionService(
		&fakeQuizActiveSessionRepo{},
		store,
		NewSRSService(nil),
	)
	if err := svc.save(
		ctx,
		quizActiveSessionTestState(
			sessionID,
			123,
			true,
		),
	); err != nil {
		t.Fatalf(
			"save failed: %v",
			err,
		)
	}

	// Flush with a different user must be rejected before touching the repo.
	_, err := svc.Flush(
		ctx,
		sessionID,
		999,
	)
	if !errors.Is(
		err,
		ErrQuizActiveSessionUserMismatch,
	) {
		t.Fatalf(
			"Flush user mismatch = %v, want ErrActiveSessionUserMismatch",
			err,
		)
	}
}

func TestQuizActiveSessionFlushRejectsNilRepo(t *testing.T) {
	ctx := context.Background()
	sessionID := 10
	userID := int64(123)
	store := newFakeQuizSessionStore()
	// repo nil but state already in Redis (complete) — dependency missing at flush time.
	svc := NewQuizActiveSessionService(
		nil,
		store,
		NewSRSService(nil),
	)
	if err := svc.save(
		ctx,
		quizActiveSessionTestState(
			sessionID,
			userID,
			true,
		),
	); err != nil {
		t.Fatalf(
			"save failed: %v",
			err,
		)
	}

	_, err := svc.Flush(
		ctx,
		sessionID,
		userID,
	)
	if !errors.Is(
		err,
		ErrQuizActiveSessionDependencyMissing,
	) {
		t.Fatalf(
			"Flush nil repo = %v, want ErrActiveSessionDependencyMissing",
			err,
		)
	}
}

func TestQuizActiveSessionFlushRepoError(t *testing.T) {
	ctx := context.Background()
	sessionID := 10
	userID := int64(123)
	flushErr := errors.New("db flush failed")
	store := newFakeQuizSessionStore()
	repo := &fakeQuizActiveSessionRepo{
		flushFn: func(
			ctx context.Context,
			state *model.QuizActiveSessionState,
		) error {
			return flushErr
		},
	}
	svc := NewQuizActiveSessionService(
		repo,
		store,
		NewSRSService(nil),
	)
	if err := svc.save(
		ctx,
		quizActiveSessionTestState(
			sessionID,
			userID,
			true,
		),
	); err != nil {
		t.Fatalf(
			"save failed: %v",
			err,
		)
	}

	_, err := svc.Flush(
		ctx,
		sessionID,
		userID,
	)
	if !errors.Is(
		err,
		flushErr,
	) {
		t.Fatalf(
			"Flush repo error = %v, want %v in chain",
			err,
			flushErr,
		)
	}
}

func TestQuizActiveSessionDelete(t *testing.T) {
	ctx := context.Background()
	sessionID := 10

	t.Run(
		"removes the working set key",
		func(t *testing.T) {
			store := newFakeQuizSessionStore()
			svc := NewQuizActiveSessionService(
				nil,
				store,
				NewSRSService(nil),
			)
			if err := svc.save(
				ctx,
				quizActiveSessionTestState(
					sessionID,
					123,
					true,
				),
			); err != nil {
				t.Fatalf(
					"save failed: %v",
					err,
				)
			}
			if _, ok := store.values[sessionID]; !ok {
				t.Fatal("precondition: key should exist before delete")
			}
			if err := svc.Delete(
				ctx,
				sessionID,
			); err != nil {
				t.Fatalf(
					"Delete failed: %v",
					err,
				)
			}
			if _, ok := store.values[sessionID]; ok {
				t.Fatal("expected key removed after Delete")
			}
		},
	)

	t.Run(
		"wraps redis delete error",
		func(t *testing.T) {
			delErr := errors.New("redis del failed")
			store := newFakeQuizSessionStore()
			store.delErr = delErr
			svc := NewQuizActiveSessionService(
				nil,
				store,
				NewSRSService(nil),
			)
			err := svc.Delete(
				ctx,
				sessionID,
			)
			if !errors.Is(
				err,
				delErr,
			) {
				t.Fatalf(
					"Delete error = %v, want %v in chain",
					err,
					delErr,
				)
			}
		},
	)
}

func TestQuizActiveSessionSaveError(t *testing.T) {
	ctx := context.Background()
	setErr := errors.New("redis set failed")
	store := newFakeQuizSessionStore()
	svc := NewQuizActiveSessionService(
		nil,
		store,
		NewSRSService(nil),
	)
	state := quizActiveSessionTestState(
		10,
		123,
		false,
	)
	state.Items[0].Question.Prompt = "persisted"
	if err := svc.save(
		ctx,
		state,
	); err != nil {
		t.Fatalf(
			"initial save failed: %v",
			err,
		)
	}

	loaded, err := svc.Get(
		ctx,
		10,
	)
	if err != nil {
		t.Fatalf(
			"Get failed: %v",
			err,
		)
	}
	loaded.Items[0].Question.Prompt = "unsaved mutation"
	stored, err := store.Load(
		ctx,
		10,
	)
	if err != nil {
		t.Fatalf(
			"Load after mutation failed: %v",
			err,
		)
	}
	if got := stored.Items[0].Question.Prompt; got != "persisted" {
		t.Fatalf(
			"mutation before Save changed stored snapshot to %q",
			got,
		)
	}

	store.setErr = setErr

	err = svc.save(
		ctx,
		loaded,
	)
	if !errors.Is(
		err,
		setErr,
	) {
		t.Fatalf(
			"save error = %v, want %v in chain",
			err,
			setErr,
		)
	}
	stored, err = store.Load(
		ctx,
		10,
	)
	if err != nil {
		t.Fatalf(
			"Load after failed Save failed: %v",
			err,
		)
	}
	if got := stored.Items[0].Question.Prompt; got != "persisted" {
		t.Fatalf(
			"failed Save changed stored snapshot to %q",
			got,
		)
	}
}

func quizActiveSessionTestState(
	sessionID int,
	userID int64,
	answered bool,
) *model.QuizActiveSessionState {
	state := activeStateForQuestion(
		sessionID,
		model.Question{
			ID:            1,
			Type:          model.QuestionMultipleChoice,
			CorrectAnswer: "apple",
		},
		answered,
	)
	state.Session.UserID = userID
	state.Items[0].Progress = model.NewUserQuestionProgress(
		userID,
		1,
	)
	return state
}
