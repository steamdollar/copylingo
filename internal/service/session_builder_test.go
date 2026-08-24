package service

import (
	"context"
	"errors"
	"testing"

	"github.com/lsj/copylingo/internal/config"
	"github.com/lsj/copylingo/internal/model"
)

type mockQuestionFetcher struct {
	getNewQuestionsFn func(ctx context.Context, userID int64, language, level, category string, excludeIDs []int, limit, kanjiRecallLimit int) ([]model.Question, error)
	getByIDFn         func(ctx context.Context, id int) (*model.Question, error)
}

func (m *mockQuestionFetcher) GetNewQuestions(
	ctx context.Context,
	userID int64,
	lang, level, cat string,
	excludeIDs []int,
	limit, kanjiRecallLimit int,
) ([]model.Question, error) {
	return m.getNewQuestionsFn(ctx, userID, lang, level, cat, excludeIDs, limit, kanjiRecallLimit)
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

func (m *mockSessionStore) GetSessionsByStatus(
	ctx context.Context,
	userID int64,
	status config.SessionStatus,
) ([]model.Session, error) {
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
		getDueReviewsFn: func(ctx context.Context, gotUserID int64, limit, kanjiRecallLimit int) ([]model.Question, error) {
			if gotUserID != userID {
				t.Fatalf("expected userID %d, got %d", userID, gotUserID)
			}
			if limit != 6 {
				t.Errorf("expected review limit 6, got %d", limit)
			}
			return []model.Question{{ID: 1}, {ID: 2}, {ID: 3}, {ID: 4}}, nil // only 4 available
		},
	}

	collectedNewCount := 0
	getNewQuestionsCalls := 0
	qFetcher := &mockQuestionFetcher{
		getNewQuestionsFn: func(ctx context.Context, gotUserID int64, lang, level, cat string, excludeIDs []int, limit, kanjiRecallLimit int) ([]model.Question, error) {
			if gotUserID != userID {
				t.Fatalf("expected userID %d, got %d", userID, gotUserID)
			}
			if getNewQuestionsCalls == 0 {
				if cat != string(model.CategoryVocabulary) {
					t.Fatalf("expected first category %q, got %q", model.CategoryVocabulary, cat)
				}
				if limit != 6 {
					t.Fatalf("expected 6 reserved vocabulary slots, got %d", limit)
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

func TestBuildMorningSession_ReservesListeningAndBuildsSeventeenQuestions(t *testing.T) {
	ctx := context.Background()
	const userID int64 = 123
	listeningFetches := 0

	srsMock := &mockSRS{
		getDueReviewsFn: func(context.Context, int64, int, int) ([]model.Question, error) {
			questions := make([]model.Question, 6)
			for i := range questions {
				questions[i].ID = i + 1
			}
			return questions, nil
		},
	}
	qFetcher := &mockQuestionFetcher{
		getNewQuestionsFn: func(
			ctx context.Context,
			gotUserID int64,
			language, level, category string,
			excludeIDs []int,
			limit, kanjiRecallLimit int,
		) ([]model.Question, error) {
			switch category {
			case string(model.CategoryVocabulary):
				if limit == 6 {
					questions := make([]model.Question, 6)
					for i := range questions {
						questions[i] = model.Question{ID: 101 + i, Category: model.CategoryVocabulary}
					}
					return questions, nil
				}
				return nil, nil
			case string(model.CategoryListening):
				listeningFetches++
				if listeningFetches == 1 {
					if limit != 1 {
						t.Fatalf("reserved listening limit = %d, want 1", limit)
					}
					return []model.Question{{ID: 201, Category: model.CategoryListening}}, nil
				}
				return nil, nil
			case "":
				questions := make([]model.Question, limit)
				for i := range questions {
					questions[i] = model.Question{ID: 301 + i, Category: model.CategoryGrammar}
				}
				return questions, nil
			default:
				return nil, nil
			}
		},
	}
	sStore := &mockSessionStore{
		createSessionFn: func(ctx context.Context, session *model.Session) error {
			if session.TotalQuestions != 17 {
				t.Fatalf("TotalQuestions = %d, want 17", session.TotalQuestions)
			}
			session.ID = 10
			return nil
		},
	}
	sqStore := &mockSessionQuestionStore{
		createSessionQuestionsFn: func(ctx context.Context, questions []model.SessionQuestion) error {
			if len(questions) != 17 {
				t.Fatalf("len(questions) = %d, want 17", len(questions))
			}
			listeningCount := 0
			for _, question := range questions {
				if question.QuestionID == 201 {
					listeningCount++
				}
			}
			if listeningCount != 1 {
				t.Fatalf("listeningCount = %d, want 1", listeningCount)
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

func TestBuildEveningSession_ReservesOneThirdForVocabulary(t *testing.T) {
	ctx := context.Background()
	userID := int64(123)

	srsMock := &mockSRS{
		getDueReviewsFn: func(ctx context.Context, gotUserID int64, limit, kanjiRecallLimit int) ([]model.Question, error) {
			if gotUserID != userID {
				t.Fatalf("expected userID %d, got %d", userID, gotUserID)
			}
			if limit != 7 {
				t.Fatalf("expected review limit 7 after vocabulary and listening reservations, got %d", limit)
			}
			return []model.Question{{ID: 1}, {ID: 2}, {ID: 3}, {ID: 4}, {ID: 5}, {ID: 6}, {ID: 7}}, nil
		},
	}
	qFetcher := &mockQuestionFetcher{
		getNewQuestionsFn: func(ctx context.Context, gotUserID int64, lang, level, cat string, excludeIDs []int, limit, kanjiRecallLimit int) ([]model.Question, error) {
			if gotUserID != userID {
				t.Fatalf("expected userID %d, got %d", userID, gotUserID)
			}
			switch cat {
			case string(model.CategoryVocabulary):
				if limit != 4 {
					t.Fatalf("expected 4 reserved vocabulary slots, got %d", limit)
				}
				return []model.Question{{ID: 101}, {ID: 102}, {ID: 103}, {ID: 104}}, nil
			case string(model.CategoryListening):
				if limit != 1 {
					t.Fatalf("expected 1 reserved listening slot, got %d", limit)
				}
				return []model.Question{{ID: 105, Category: model.CategoryListening}}, nil
			default:
				t.Fatalf("unexpected new-question category %q", cat)
				return nil, nil
			}
		},
	}
	sStore := &mockSessionStore{
		createSessionFn: func(ctx context.Context, s *model.Session) error {
			if s.TotalQuestions != 12 {
				t.Fatalf("expected total 12, got %d", s.TotalQuestions)
			}
			s.ID = 10
			return nil
		},
	}
	sqStore := &mockSessionQuestionStore{
		createSessionQuestionsFn: func(ctx context.Context, sqs []model.SessionQuestion) error {
			if len(sqs) != 12 {
				t.Fatalf("expected 12 session questions, got %d", len(sqs))
			}
			for i, wantID := range []int{101, 102, 103, 104} {
				if got := sqs[7+i].QuestionID; got != wantID {
					t.Fatalf("session question %d id = %d, want %d", 7+i, got, wantID)
				}
			}
			if got := sqs[11].QuestionID; got != 105 {
				t.Fatalf("reserved listening id = %d, want 105", got)
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
		getDueReviewsFn: func(ctx context.Context, gotUserID int64, limit, kanjiRecallLimit int) ([]model.Question, error) {
			return []model.Question{{ID: 1}, {ID: 2}, {ID: 3}, {ID: 4}, {ID: 5}, {ID: 6}}, nil
		},
	}
	qFetcher := &mockQuestionFetcher{
		getNewQuestionsFn: func(ctx context.Context, gotUserID int64, lang, level, cat string, excludeIDs []int, limit, kanjiRecallLimit int) ([]model.Question, error) {
			if cat == string(model.CategoryVocabulary) {
				vocabularyCalls++
				if vocabularyCalls == 1 {
					return []model.Question{{ID: 101}, {ID: 102}}, nil
				}
			}
			if cat == "" {
				return []model.Question{{ID: 201}, {ID: 202}, {ID: 203}, {ID: 204}}, nil
			}
			return nil, nil
		},
	}
	sStore := &mockSessionStore{
		createSessionFn: func(ctx context.Context, s *model.Session) error {
			if s.TotalQuestions != 12 {
				t.Fatalf("expected total 12, got %d", s.TotalQuestions)
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
		getDueReviewsFn: func(ctx context.Context, gotUserID int64, limit, kanjiRecallLimit int) ([]model.Question, error) {
			if gotUserID != userID {
				t.Fatalf("expected userID %d, got %d", userID, gotUserID)
			}
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
	session, err := builder.BuildReviewSession(ctx, userID, "ja", "N5", 5)

	if err != nil {
		t.Fatalf("BuildReviewSession failed: %v", err)
	}
	if session.TotalQuestions != 2 {
		t.Errorf("expected 2 questions, got %d", session.TotalQuestions)
	}
	if srsMock.gotLanguage != "ja" || srsMock.gotLevel != "N5" {
		t.Fatalf("due scope = %s/%s, want ja/N5", srsMock.gotLanguage, srsMock.gotLevel)
	}
}

func TestBuildReviewSession_CapsKanjiRecallAdmissionAtThree(t *testing.T) {
	ctx := context.Background()
	kanjiSkill := model.SkillVocabKanjiRecall
	due := []model.Question{
		{ID: 1, Skill: &kanjiSkill},
		{ID: 2, Skill: &kanjiSkill},
		{ID: 3, Skill: &kanjiSkill},
		{ID: 4, Skill: &kanjiSkill},
		{ID: 5, Skill: &kanjiSkill},
		{ID: 6},
		{ID: 7},
	}

	srsMock := &mockSRS{
		getDueReviewsFn: func(
			ctx context.Context,
			userID int64,
			limit, kanjiRecallLimit int,
		) ([]model.Question, error) {
			if limit != 10 || kanjiRecallLimit != maxKanjiRecallPerSession {
				t.Fatalf(
					"GetDueReviews limit=%d kanjiRecallLimit=%d, want 10,%d",
					limit,
					kanjiRecallLimit,
					maxKanjiRecallPerSession,
				)
			}
			return due, nil
		},
	}
	sStore := &mockSessionStore{
		createSessionFn: func(ctx context.Context, session *model.Session) error {
			session.ID = 10
			return nil
		},
	}
	sqStore := &mockSessionQuestionStore{
		createSessionQuestionsFn: func(ctx context.Context, questions []model.SessionQuestion) error {
			wantIDs := []int{1, 2, 3, 6, 7}
			if len(questions) != len(wantIDs) {
				t.Fatalf("len(questions) = %d, want %d", len(questions), len(wantIDs))
			}
			for i, wantID := range wantIDs {
				if questions[i].QuestionID != wantID {
					t.Fatalf("questions[%d].QuestionID = %d, want %d", i, questions[i].QuestionID, wantID)
				}
			}
			return nil
		},
	}

	builder := NewSessionBuilderService(nil, sStore, sqStore, srsMock)
	session, err := builder.BuildReviewSession(ctx, 123, "ja", "N5", 10)
	if err != nil {
		t.Fatalf("BuildReviewSession failed: %v", err)
	}
	if session.TotalQuestions != 5 {
		t.Fatalf("TotalQuestions = %d, want 5", session.TotalQuestions)
	}
}

func TestBuildMorningSession_PassesRemainingKanjiRecallBudgetToNewFetches(t *testing.T) {
	ctx := context.Background()
	kanjiSkill := model.SkillVocabKanjiRecall
	newFetchCalls := 0

	srsMock := &mockSRS{
		getDueReviewsFn: func(
			ctx context.Context,
			userID int64,
			limit, kanjiRecallLimit int,
		) ([]model.Question, error) {
			return []model.Question{
				{ID: 1, Skill: &kanjiSkill},
				{ID: 2, Skill: &kanjiSkill},
				{ID: 3},
				{ID: 4},
				{ID: 5},
				{ID: 6},
			}, nil
		},
	}
	qFetcher := &mockQuestionFetcher{
		getNewQuestionsFn: func(
			ctx context.Context,
			userID int64,
			language, level, category string,
			excludeIDs []int,
			limit, kanjiRecallLimit int,
		) ([]model.Question, error) {
			newFetchCalls++
			if newFetchCalls == 1 {
				if category != string(model.CategoryVocabulary) || kanjiRecallLimit != 1 {
					t.Fatalf(
						"first new fetch category=%q kanjiRecallLimit=%d, want vocabulary,1",
						category,
						kanjiRecallLimit,
					)
				}
				return []model.Question{
					{ID: 101, Skill: &kanjiSkill},
					{ID: 102},
					{ID: 103},
					{ID: 104},
					{ID: 105},
				}, nil
			}
			if kanjiRecallLimit != 0 {
				t.Fatalf("later new fetch kanjiRecallLimit = %d, want 0", kanjiRecallLimit)
			}
			return nil, nil
		},
	}
	sStore := &mockSessionStore{
		createSessionFn: func(ctx context.Context, session *model.Session) error {
			session.ID = 10
			return nil
		},
	}
	sqStore := &mockSessionQuestionStore{
		createSessionQuestionsFn: func(ctx context.Context, questions []model.SessionQuestion) error {
			return nil
		},
	}

	builder := NewSessionBuilderService(qFetcher, sStore, sqStore, srsMock)
	session, err := builder.BuildMorningSession(ctx, 123, "ja", "N5")
	if err != nil {
		t.Fatalf("BuildMorningSession failed: %v", err)
	}
	if session == nil || session.TotalQuestions != 11 {
		t.Fatalf("session = %+v, want 11 questions", session)
	}
}

func TestBuildSession_NoQuestions(t *testing.T) {
	ctx := context.Background()
	userID := int64(123)

	srsMock := &mockSRS{
		getDueReviewsFn: func(ctx context.Context, gotUserID int64, limit, kanjiRecallLimit int) ([]model.Question, error) {
			return nil, nil
		},
	}

	qFetcher := &mockQuestionFetcher{
		getNewQuestionsFn: func(ctx context.Context, gotUserID int64, lang, level, cat string, excludeIDs []int, limit, kanjiRecallLimit int) ([]model.Question, error) {
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
		getDueReviewsFn: func(ctx context.Context, gotUserID int64, limit, kanjiRecallLimit int) ([]model.Question, error) {
			return []model.Question{{ID: 1}}, nil
		},
	}

	sStore := &mockSessionStore{
		createSessionFn: func(ctx context.Context, s *model.Session) error {
			return errors.New("db error")
		},
	}

	builder := NewSessionBuilderService(nil, sStore, nil, srsMock)
	_, err := builder.BuildReviewSession(ctx, userID, "ja", "N5", 5)

	if err == nil {
		t.Error("expected error when session creation fails")
	}
}

func TestBuildSession_CreateSessionQuestionsFails(t *testing.T) {
	ctx := context.Background()
	userID := int64(123)
	expectedErr := errors.New("create session questions failed")

	srsMock := &mockSRS{
		getDueReviewsFn: func(ctx context.Context, gotUserID int64, limit, kanjiRecallLimit int) ([]model.Question, error) {
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
	_, err := builder.BuildReviewSession(ctx, userID, "ja", "N5", 5)

	if !errors.Is(err, expectedErr) {
		t.Fatalf("expected CreateSessionQuestions error %v, got %v", expectedErr, err)
	}
}

func TestBuildSession_DeduplicatesQuestionIDs(t *testing.T) {
	ctx := context.Background()
	userID := int64(123)

	srsMock := &mockSRS{
		getDueReviewsFn: func(ctx context.Context, gotUserID int64, limit, kanjiRecallLimit int) ([]model.Question, error) {
			return []model.Question{{ID: 1}, {ID: 1}}, nil
		},
	}
	qFetcher := &mockQuestionFetcher{
		getNewQuestionsFn: func(ctx context.Context, gotUserID int64, lang, level, cat string, excludeIDs []int, limit, kanjiRecallLimit int) ([]model.Question, error) {
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

// Reading is capped at one question per session across every admission path:
// due reviews, the relay, and the final fallback (ADR-036).
func TestBuildSession_CapsReadingAtOne(t *testing.T) {
	ctx := context.Background()
	userID := int64(123)

	readingQuestion := func(id int) model.Question {
		return model.Question{ID: id, Category: model.CategoryReading}
	}
	isReadingID := func(id int) bool {
		return id == 1 || id == 2 || id == 100 || id == 101
	}

	srsMock := &mockSRS{
		getDueReviewsFn: func(ctx context.Context, gotUserID int64, limit, kanjiRecallLimit int) ([]model.Question, error) {
			// Two due reading reviews: only the first may enter the session.
			return []model.Question{readingQuestion(1), readingQuestion(2)}, nil
		},
	}
	qFetcher := &mockQuestionFetcher{
		getNewQuestionsFn: func(ctx context.Context, gotUserID int64, lang, level, cat string, excludeIDs []int, limit, kanjiRecallLimit int) ([]model.Question, error) {
			switch cat {
			case string(model.CategoryReading):
				// The review already consumed the reading budget, so the relay
				// must clamp the reading allocation to zero and never fetch.
				t.Errorf("relay fetched reading questions with limit %d despite exhausted cap", limit)
				return nil, nil
			case "":
				// The generic fallback may still return reading rows; appendQuestion
				// must reject them while admitting other categories.
				return []model.Question{
					readingQuestion(100),
					readingQuestion(101),
					{ID: 200, Category: model.CategoryVocabulary},
					{ID: 201, Category: model.CategoryVocabulary},
				}, nil
			default:
				return nil, nil
			}
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
			readingCount := 0
			for _, sq := range sqs {
				if isReadingID(sq.QuestionID) {
					readingCount++
				}
			}
			if readingCount != 1 {
				t.Fatalf("session admitted %d reading questions, want 1: %+v", readingCount, sqs)
			}
			if len(sqs) != 3 { // 1 reading review + 2 fallback vocabulary
				t.Fatalf("expected 3 session questions, got %d: %+v", len(sqs), sqs)
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
