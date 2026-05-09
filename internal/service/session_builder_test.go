package service

import (
	"context"
	"errors"
	"testing"

	"github.com/lsj/copylingo/internal/model"
)

type mockQuestionFetcher struct {
	getNewQuestionsFn func(ctx context.Context, language, level, category string, limit int) ([]model.Question, error)
	getByIDFn         func(ctx context.Context, id int) (*model.Question, error)
}

func (m *mockQuestionFetcher) GetNewQuestions(ctx context.Context, lang, level, cat string, limit int) ([]model.Question, error) {
	return m.getNewQuestionsFn(ctx, lang, level, cat, limit)
}
func (m *mockQuestionFetcher) GetByID(ctx context.Context, id int) (*model.Question, error) {
	return m.getByIDFn(ctx, id)
}

type mockSessionStore struct {
	createSessionFn         func(ctx context.Context, s *model.Session) error
	getByIDFn               func(ctx context.Context, id int) (*model.Session, error)
	getPendingSessionsFn    func(ctx context.Context, userID int64) ([]model.Session, error)
	getInProgressSessionsFn func(ctx context.Context, userID int64) ([]model.Session, error)
	startFn                 func(ctx context.Context, id int) error
}

func (m *mockSessionStore) CreateSession(ctx context.Context, s *model.Session) error {
	return m.createSessionFn(ctx, s)
}
func (m *mockSessionStore) GetByID(ctx context.Context, id int) (*model.Session, error) {
	return m.getByIDFn(ctx, id)
}
func (m *mockSessionStore) GetPendingSessions(ctx context.Context, userID int64) ([]model.Session, error) {
	return m.getPendingSessionsFn(ctx, userID)
}
func (m *mockSessionStore) GetInProgressSessions(ctx context.Context, userID int64) ([]model.Session, error) {
	return m.getInProgressSessionsFn(ctx, userID)
}
func (m *mockSessionStore) Start(ctx context.Context, id int) error {
	return m.startFn(ctx, id)
}

type mockSessionQuestionStore struct {
	createSessionQuestionsFn func(ctx context.Context, sqs []model.SessionQuestion) error
	getBySessionFn           func(ctx context.Context, sessionID int) ([]model.SessionQuestion, error)
}

func (m *mockSessionQuestionStore) CreateSessionQuestions(ctx context.Context, sqs []model.SessionQuestion) error {
	return m.createSessionQuestionsFn(ctx, sqs)
}
func (m *mockSessionQuestionStore) GetBySession(ctx context.Context, sessionID int) ([]model.SessionQuestion, error) {
	return m.getBySessionFn(ctx, sessionID)
}

func TestBuildMorningSession_MixesReviewAndNew(t *testing.T) {
	ctx := context.Background()
	userID := int64(123)

	srsMock := &mockSRS{
		getDueReviewsFn: func(ctx context.Context, limit int) ([]model.Question, error) {
			if limit != 6 {
				t.Errorf("expected review limit 6, got %d", limit)
			}
			return []model.Question{{ID: 1}, {ID: 2}, {ID: 3}, {ID: 4}}, nil // only 4 available
		},
	}

	qFetcher := &mockQuestionFetcher{
		getNewQuestionsFn: func(ctx context.Context, lang, level, cat string, limit int) ([]model.Question, error) {
			// total 15, reviews 4, so need 11 new
			if limit != 11 {
				t.Errorf("expected new limit 11, got %d", limit)
			}
			return []model.Question{{ID: 101}, {ID: 102}, {ID: 103}, {ID: 104}, {ID: 105}, {ID: 106}, {ID: 107}, {ID: 108}, {ID: 109}}, nil // only 9 available
		},
	}

	sStore := &mockSessionStore{
		createSessionFn: func(ctx context.Context, s *model.Session) error {
			if s.TotalQuestions != 13 {
				t.Errorf("expected total 13, got %d", s.TotalQuestions)
			}
			s.ID = 10
			return nil
		},
	}

	sqStore := &mockSessionQuestionStore{
		createSessionQuestionsFn: func(ctx context.Context, sqs []model.SessionQuestion) error {
			if len(sqs) != 13 {
				t.Errorf("expected 13 sqs, got %d", len(sqs))
			}
			return nil
		},
	}

	builder := NewSessionBuilderService(qFetcher, sStore, sqStore, srsMock)
	session, err := builder.BuildMorningSession(ctx, userID, "jp", "n5")

	if err != nil {
		t.Fatalf("BuildMorningSession failed: %v", err)
	}
	if session == nil {
		t.Fatal("expected session to be created")
	}
}

func TestBuildReviewSession_OnlySRS(t *testing.T) {
	ctx := context.Background()
	userID := int64(123)

	srsMock := &mockSRS{
		getDueReviewsFn: func(ctx context.Context, limit int) ([]model.Question, error) {
			return []model.Question{{ID: 1}, {ID: 2}}, nil
		},
	}

	sStore := &mockSessionStore{
		createSessionFn: func(ctx context.Context, s *model.Session) error {
			return nil
		},
	}

	sqStore := &mockSessionQuestionStore{
		createSessionQuestionsFn: func(ctx context.Context, sqs []model.SessionQuestion) error {
			return nil
		},
	}

	builder := NewSessionBuilderService(nil, sStore, sqStore, srsMock)
	session, err := builder.BuildReviewSession(ctx, userID, 5)

	if err != nil {
		t.Fatalf("BuildReviewSession failed: %v", err)
	}
	if session.TotalQuestions != 2 {
		t.Errorf("expected 2 questions, got %d", session.TotalQuestions)
	}
}

func TestBuildSession_NoQuestions(t *testing.T) {
	ctx := context.Background()
	userID := int64(123)

	srsMock := &mockSRS{
		getDueReviewsFn: func(ctx context.Context, limit int) ([]model.Question, error) {
			return nil, nil
		},
	}

	qFetcher := &mockQuestionFetcher{
		getNewQuestionsFn: func(ctx context.Context, lang, level, cat string, limit int) ([]model.Question, error) {
			return nil, nil
		},
	}

	builder := NewSessionBuilderService(qFetcher, nil, nil, srsMock)
	session, err := builder.BuildMorningSession(ctx, userID, "jp", "n5")

	if err != nil {
		t.Fatalf("BuildMorningSession failed: %v", err)
	}
	if session != nil {
		t.Error("expected nil session when no questions found")
	}
}

func TestBuildSession_CreateFails(t *testing.T) {
	ctx := context.Background()
	userID := int64(123)

	srsMock := &mockSRS{
		getDueReviewsFn: func(ctx context.Context, limit int) ([]model.Question, error) {
			return []model.Question{{ID: 1}}, nil
		},
	}

	sStore := &mockSessionStore{
		createSessionFn: func(ctx context.Context, s *model.Session) error {
			return errors.New("db error")
		},
	}

	builder := NewSessionBuilderService(nil, sStore, nil, srsMock)
	_, err := builder.BuildReviewSession(ctx, userID, 5)

	if err == nil {
		t.Error("expected error when session creation fails")
	}
}

func TestBuildSession_CreateSessionQuestionsFails(t *testing.T) {
	ctx := context.Background()
	userID := int64(123)
	expectedErr := errors.New("create session questions failed")

	srsMock := &mockSRS{
		getDueReviewsFn: func(ctx context.Context, limit int) ([]model.Question, error) {
			return []model.Question{{ID: 1}, {ID: 2}}, nil
		},
	}
	sStore := &mockSessionStore{
		createSessionFn: func(ctx context.Context, s *model.Session) error {
			s.ID = 10
			return nil
		},
	}
	sqStore := &mockSessionQuestionStore{
		createSessionQuestionsFn: func(ctx context.Context, sqs []model.SessionQuestion) error {
			if len(sqs) != 2 {
				t.Errorf("expected 2 session questions, got %d", len(sqs))
			}
			for _, sq := range sqs {
				if sq.SessionID != 10 {
					t.Errorf("expected SessionID 10, got %d", sq.SessionID)
				}
			}
			return expectedErr
		},
	}

	builder := NewSessionBuilderService(nil, sStore, sqStore, srsMock)
	_, err := builder.BuildReviewSession(ctx, userID, 5)

	if !errors.Is(err, expectedErr) {
		t.Fatalf("expected CreateSessionQuestions error %v, got %v", expectedErr, err)
	}
}
