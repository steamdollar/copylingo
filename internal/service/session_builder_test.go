package service

import (
	"context"
	"errors"
	"testing"

	"github.com/lsj/copylingo/internal/config"
	"github.com/lsj/copylingo/internal/model"
)

type mockQuestionFetcher struct {
	getNewQuestionsFn func(ctx context.Context, language, level, category string, excludeIDs []int, limit int) ([]model.Question, error)
	getByIDFn         func(ctx context.Context, id int) (*model.Question, error)
}

func (m *mockQuestionFetcher) GetNewQuestions(ctx context.Context, lang, level, cat string, excludeIDs []int, limit int) ([]model.Question, error) {
	return m.getNewQuestionsFn(ctx, lang, level, cat, excludeIDs, limit)
}
func (m *mockQuestionFetcher) GetByID(ctx context.Context, id int) (*model.Question, error) {
	return m.getByIDFn(ctx, id)
}

type mockSessionStore struct {
	createSessionFn       func(ctx context.Context, s *model.Session) error
	getByIDFn             func(ctx context.Context, id int) (*model.Session, error)
	getSessionsByStatusFn func(ctx context.Context, userID int64, status config.SessionStatus) ([]model.Session, error)
	listInProgressFn      func(ctx context.Context) ([]model.Session, error)
	startFn               func(ctx context.Context, id int) error
}

func (m *mockSessionStore) CreateSession(ctx context.Context, s *model.Session) error {
	return m.createSessionFn(ctx, s)
}
func (m *mockSessionStore) GetByID(ctx context.Context, id int) (*model.Session, error) {
	return m.getByIDFn(ctx, id)
}
func (m *mockSessionStore) GetSessionsByStatus(ctx context.Context, userID int64, status config.SessionStatus) ([]model.Session, error) {
	return m.getSessionsByStatusFn(ctx, userID, status)
}
func (m *mockSessionStore) ListInProgress(ctx context.Context) ([]model.Session, error) {
	if m.listInProgressFn != nil {
		return m.listInProgressFn(ctx)
	}
	return nil, nil
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

	collectedNewCount := 0
	getNewQuestionsCalls := 0
	qFetcher := &mockQuestionFetcher{
		getNewQuestionsFn: func(ctx context.Context, lang, level, cat string, excludeIDs []int, limit int) ([]model.Question, error) {
			if getNewQuestionsCalls == 0 {
				if cat != string(model.CategoryVocabulary) {
					t.Fatalf("expected first category %q, got %q", model.CategoryVocabulary, cat)
				}
				if limit != 5 {
					t.Fatalf("expected 5 reserved vocabulary slots, got %d", limit)
				}
			}
			getNewQuestionsCalls++

			// Random Slot Relay will call this multiple times for different categories.
			// Each call should have a reasonable limit.
			if limit < 0 {
				t.Errorf("unexpected negative limit %d", limit)
			}

			// We simulate returning a few questions for some categories to test relay.
			// If cat is empty (final fallback), we return some to fill the gap.
			var qs []model.Question
			if cat == "" {
				// Fill up to 9 new questions (since we already have 4 reviews, total goal 15, need 11 new)
				// But we'll return 9 to match the original test's 13 total.
				need := 9 - collectedNewCount
				if need > 0 {
					for i := 0; i < need; i++ {
						qs = append(qs, model.Question{ID: 1000 + collectedNewCount + i})
					}
				}
			}
			collectedNewCount += len(qs)
			return qs, nil
		},
	}

	sStore := &mockSessionStore{
		createSessionFn: func(ctx context.Context, s *model.Session) error {
			// 4 reviews + 9 new = 13 total
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

func TestBuildEveningSession_ReservesOneThirdForVocabulary(t *testing.T) {
	ctx := context.Background()
	userID := int64(123)

	srsMock := &mockSRS{
		getDueReviewsFn: func(ctx context.Context, limit int) ([]model.Question, error) {
			if limit != 6 {
				t.Fatalf("expected review limit 6 after vocabulary reservation, got %d", limit)
			}
			return []model.Question{{ID: 1}, {ID: 2}, {ID: 3}, {ID: 4}, {ID: 5}, {ID: 6}}, nil
		},
	}
	qFetcher := &mockQuestionFetcher{
		getNewQuestionsFn: func(ctx context.Context, lang, level, cat string, excludeIDs []int, limit int) ([]model.Question, error) {
			if cat != string(model.CategoryVocabulary) {
				t.Fatalf("expected reserved vocabulary fetch, got category %q", cat)
			}
			if limit != 4 {
				t.Fatalf("expected 4 reserved vocabulary slots, got %d", limit)
			}
			return []model.Question{{ID: 101}, {ID: 102}, {ID: 103}, {ID: 104}}, nil
		},
	}
	sStore := &mockSessionStore{
		createSessionFn: func(ctx context.Context, s *model.Session) error {
			if s.TotalQuestions != 10 {
				t.Fatalf("expected total 10, got %d", s.TotalQuestions)
			}
			s.ID = 10
			return nil
		},
	}
	sqStore := &mockSessionQuestionStore{
		createSessionQuestionsFn: func(ctx context.Context, sqs []model.SessionQuestion) error {
			if len(sqs) != 10 {
				t.Fatalf("expected 10 session questions, got %d", len(sqs))
			}
			for i, wantID := range []int{101, 102, 103, 104} {
				if got := sqs[6+i].QuestionID; got != wantID {
					t.Fatalf("session question %d id = %d, want %d", 6+i, got, wantID)
				}
			}
			return nil
		},
	}

	builder := NewSessionBuilderService(qFetcher, sStore, sqStore, srsMock)
	session, err := builder.BuildEveningSession(ctx, userID, "ja", "N5")
	if err != nil {
		t.Fatalf("BuildEveningSession failed: %v", err)
	}
	if session == nil {
		t.Fatal("expected session")
	}
}

func TestBuildEveningSession_FillsVocabularyShortageWithRelay(t *testing.T) {
	ctx := context.Background()
	userID := int64(123)
	vocabularyCalls := 0

	srsMock := &mockSRS{
		getDueReviewsFn: func(ctx context.Context, limit int) ([]model.Question, error) {
			return []model.Question{{ID: 1}, {ID: 2}, {ID: 3}, {ID: 4}, {ID: 5}, {ID: 6}}, nil
		},
	}
	qFetcher := &mockQuestionFetcher{
		getNewQuestionsFn: func(ctx context.Context, lang, level, cat string, excludeIDs []int, limit int) ([]model.Question, error) {
			if cat == string(model.CategoryVocabulary) {
				vocabularyCalls++
				if vocabularyCalls == 1 {
					return []model.Question{{ID: 101}, {ID: 102}}, nil
				}
			}
			if cat == "" {
				return []model.Question{{ID: 201}, {ID: 202}}, nil
			}
			return nil, nil
		},
	}
	sStore := &mockSessionStore{
		createSessionFn: func(ctx context.Context, s *model.Session) error {
			if s.TotalQuestions != 10 {
				t.Fatalf("expected total 10, got %d", s.TotalQuestions)
			}
			s.ID = 10
			return nil
		},
	}
	sqStore := &mockSessionQuestionStore{
		createSessionQuestionsFn: func(ctx context.Context, sqs []model.SessionQuestion) error {
			return nil
		},
	}

	builder := NewSessionBuilderService(qFetcher, sStore, sqStore, srsMock)
	session, err := builder.BuildEveningSession(ctx, userID, "ja", "N5")
	if err != nil {
		t.Fatalf("BuildEveningSession failed: %v", err)
	}
	if session == nil {
		t.Fatal("expected session")
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
		getNewQuestionsFn: func(ctx context.Context, lang, level, cat string, excludeIDs []int, limit int) ([]model.Question, error) {
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

func TestBuildSession_DeduplicatesQuestionIDs(t *testing.T) {
	ctx := context.Background()
	userID := int64(123)

	srsMock := &mockSRS{
		getDueReviewsFn: func(ctx context.Context, limit int) ([]model.Question, error) {
			return []model.Question{{ID: 1}, {ID: 1}}, nil
		},
	}
	qFetcher := &mockQuestionFetcher{
		getNewQuestionsFn: func(ctx context.Context, lang, level, cat string, excludeIDs []int, limit int) ([]model.Question, error) {
			return []model.Question{{ID: 1}, {ID: 2}, {ID: 2}}, nil
		},
	}
	sStore := &mockSessionStore{
		createSessionFn: func(ctx context.Context, s *model.Session) error {
			if s.TotalQuestions != 2 {
				t.Fatalf("expected 2 unique questions, got %d", s.TotalQuestions)
			}
			s.ID = 10
			return nil
		},
	}
	sqStore := &mockSessionQuestionStore{
		createSessionQuestionsFn: func(ctx context.Context, sqs []model.SessionQuestion) error {
			if len(sqs) != 2 {
				t.Fatalf("expected 2 unique session questions, got %d", len(sqs))
			}
			if sqs[0].QuestionID != 1 || sqs[1].QuestionID != 2 {
				t.Fatalf("unexpected question ids: %+v", sqs)
			}
			return nil
		},
	}

	builder := NewSessionBuilderService(qFetcher, sStore, sqStore, srsMock)
	session, err := builder.BuildMorningSession(ctx, userID, "ja", "N5")
	if err != nil {
		t.Fatalf("BuildMorningSession failed: %v", err)
	}
	if session == nil {
		t.Fatal("expected session")
	}
}
