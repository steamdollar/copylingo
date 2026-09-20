package service

import (
	"context"
	"errors"
	"slices"
	"testing"

	"github.com/lsj/copylingo/internal/config"
	"github.com/lsj/copylingo/internal/model"
)

type mockQuestionFetcher struct {
	getNewQuestionsFn func(ctx context.Context, userID int64, language string, levels []string, category string, excludeIDs []int, limit, kanjiRecallLimit int) ([]model.Question, error)
	getByIDFn         func(ctx context.Context, id int) (*model.Question, error)
}

func (m *mockQuestionFetcher) GetNewQuestions(
	ctx context.Context,
	userID int64,
	lang string, levels []string, cat string,
	excludeIDs []int,
	limit, kanjiRecallLimit int,
) ([]model.Question, error) {
	return m.getNewQuestionsFn(ctx, userID, lang, levels, cat, excludeIDs, limit, kanjiRecallLimit)
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
			if limit != 19 {
				t.Errorf("expected bounded due pool limit 19, got %d", limit)
			}
			return []model.Question{
				{ID: 1, ProficiencyLevel: "N5"},
				{ID: 2, ProficiencyLevel: "N5"},
				{ID: 3, ProficiencyLevel: "N5"},
				{ID: 4, ProficiencyLevel: "N5"},
			}, nil // only 4 available
		},
	}

	collectedNewCount := 0
	getNewQuestionsCalls := 0
	qFetcher := &mockQuestionFetcher{
		getNewQuestionsFn: func(ctx context.Context, gotUserID int64, lang string, levels []string, cat string, excludeIDs []int, limit, kanjiRecallLimit int) ([]model.Question, error) {
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
			if cat == string(model.CategoryGrammar) {
				// Fill up to 9 new questions (since we already have 4 reviews, total goal 15, need 11 new)
				// But we'll return 9 to match the original test's 13 total.
				need := min(9-collectedNewCount, limit)
				if need > 0 {
					for i := 0; i < need; i++ {
						qs = append(
							qs,
							model.Question{
								ID:               1000 + collectedNewCount + i,
								ProficiencyLevel: "N5",
								Category:         model.CategoryGrammar,
							},
						)
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
				questions[i] = model.Question{ID: i + 1, ProficiencyLevel: "N5"}
			}
			return questions, nil
		},
	}
	qFetcher := &mockQuestionFetcher{
		getNewQuestionsFn: func(
			ctx context.Context,
			gotUserID int64,
			language string, levels []string, category string,
			excludeIDs []int,
			limit, kanjiRecallLimit int,
		) ([]model.Question, error) {
			if !slices.Equal(levels, []string{"N5"}) && !slices.Equal(levels, []string{"N5", "N4"}) {
				t.Fatalf("levels = %v, want [N5]", levels)
			}
			if category == string(model.CategoryReading) && !slices.Equal(levels, []string{"N5"}) &&
				!slices.Equal(levels, []string{"N5", "N4"}) {
				t.Fatalf("reading levels = %v, want [N5] or adjacent fallback", levels)
			}
			switch category {
			case string(model.CategoryVocabulary):
				if limit == 6 {
					questions := make([]model.Question, 6)
					for i := range questions {
						questions[i] = model.Question{
							ID:               101 + i,
							ProficiencyLevel: "N5",
							Category:         model.CategoryVocabulary,
						}
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
					return []model.Question{{ID: 201, ProficiencyLevel: "N5", Category: model.CategoryListening}}, nil
				}
				return nil, nil
			case string(model.CategoryGrammar):
				questions := make([]model.Question, 0, limit)
				for id := 301; len(questions) < limit; id++ {
					if !slices.Contains(excludeIDs, id) {
						questions = append(
							questions,
							model.Question{ID: id, ProficiencyLevel: "N5", Category: model.CategoryGrammar},
						)
					}
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
			if limit != 14 {
				t.Fatalf("expected bounded due pool limit 14, got %d", limit)
			}
			return []model.Question{
				{ID: 1, ProficiencyLevel: "N5"},
				{ID: 2, ProficiencyLevel: "N5"},
				{ID: 3, ProficiencyLevel: "N5"},
				{ID: 4, ProficiencyLevel: "N5"},
				{ID: 5, ProficiencyLevel: "N5"},
				{ID: 6, ProficiencyLevel: "N5"},
				{ID: 7, ProficiencyLevel: "N5"},
			}, nil
		},
	}
	qFetcher := &mockQuestionFetcher{
		getNewQuestionsFn: func(ctx context.Context, gotUserID int64, lang string, levels []string, cat string, excludeIDs []int, limit, kanjiRecallLimit int) ([]model.Question, error) {
			if gotUserID != userID {
				t.Fatalf("expected userID %d, got %d", userID, gotUserID)
			}
			switch cat {
			case string(model.CategoryVocabulary):
				if !slices.Contains(excludeIDs, 101) && limit != 4 {
					t.Fatalf("expected 4 reserved vocabulary slots, got %d", limit)
				}
				var result []model.Question
				for id := 101; id <= 104 && len(result) < limit; id++ {
					if !slices.Contains(excludeIDs, id) {
						result = append(
							result,
							model.Question{ID: id, ProficiencyLevel: "N5", Category: model.CategoryVocabulary},
						)
					}
				}
				return result, nil
			case string(model.CategoryListening):
				if limit != 1 {
					t.Fatalf("expected 1 reserved listening slot, got %d", limit)
				}
				return []model.Question{{ID: 105, ProficiencyLevel: "N5", Category: model.CategoryListening}}, nil
			case string(model.CategoryReading):
				return nil, nil
			default:
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
			for _, wantID := range []int{101, 102, 103, 104, 105} {
				found := slices.ContainsFunc(sqs, func(sq model.SessionQuestion) bool {
					return sq.QuestionID == wantID && !sq.IsReview
				})
				if !found {
					t.Fatalf("reserved new question %d missing", wantID)
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
		getDueReviewsFn: func(ctx context.Context, gotUserID int64, limit, kanjiRecallLimit int) ([]model.Question, error) {
			return []model.Question{
				{ID: 1, ProficiencyLevel: "N5"},
				{ID: 2, ProficiencyLevel: "N5"},
				{ID: 3, ProficiencyLevel: "N5"},
				{ID: 4, ProficiencyLevel: "N5"},
				{ID: 5, ProficiencyLevel: "N5"},
				{ID: 6, ProficiencyLevel: "N5"},
			}, nil
		},
	}
	qFetcher := &mockQuestionFetcher{
		getNewQuestionsFn: func(ctx context.Context, gotUserID int64, lang string, levels []string, cat string, excludeIDs []int, limit, kanjiRecallLimit int) ([]model.Question, error) {
			if cat == string(model.CategoryVocabulary) {
				vocabularyCalls++
				if vocabularyCalls == 1 {
					return []model.Question{{ID: 101, ProficiencyLevel: "N5"}, {ID: 102, ProficiencyLevel: "N5"}}, nil
				}
			}
			if cat == string(model.CategoryGrammar) {
				return []model.Question{
					{ID: 201, ProficiencyLevel: "N5"},
					{ID: 202, ProficiencyLevel: "N5"},
					{ID: 203, ProficiencyLevel: "N5"},
					{ID: 204, ProficiencyLevel: "N5"},
				}, nil
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
				{ID: 1, ProficiencyLevel: "N5", Skill: &kanjiSkill},
				{ID: 2, ProficiencyLevel: "N5", Skill: &kanjiSkill},
				{ID: 3, ProficiencyLevel: "N5"},
				{ID: 4, ProficiencyLevel: "N5"},
				{ID: 5, ProficiencyLevel: "N5"},
				{ID: 6, ProficiencyLevel: "N5"},
			}, nil
		},
	}
	qFetcher := &mockQuestionFetcher{
		getNewQuestionsFn: func(
			ctx context.Context,
			userID int64,
			language string, levels []string, category string,
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
					{ID: 101, ProficiencyLevel: "N5", Skill: &kanjiSkill},
					{ID: 102, ProficiencyLevel: "N5"},
					{ID: 103, ProficiencyLevel: "N5"},
					{ID: 104, ProficiencyLevel: "N5"},
					{ID: 105, ProficiencyLevel: "N5"},
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
		getNewQuestionsFn: func(ctx context.Context, gotUserID int64, lang string, levels []string, cat string, excludeIDs []int, limit, kanjiRecallLimit int) ([]model.Question, error) {
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
			return []model.Question{{ID: 1, ProficiencyLevel: "N5"}, {ID: 1, ProficiencyLevel: "N5"}}, nil
		},
	}
	qFetcher := &mockQuestionFetcher{
		getNewQuestionsFn: func(ctx context.Context, gotUserID int64, lang string, levels []string, cat string, excludeIDs []int, limit, kanjiRecallLimit int) ([]model.Question, error) {
			return []model.Question{
				{ID: 1, ProficiencyLevel: "N5"},
				{ID: 2, ProficiencyLevel: "N5"},
				{ID: 2, ProficiencyLevel: "N5"},
			}, nil
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
		return model.Question{ID: id, ProficiencyLevel: "N5", Category: model.CategoryReading}
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
		getNewQuestionsFn: func(ctx context.Context, gotUserID int64, lang string, levels []string, cat string, excludeIDs []int, limit, kanjiRecallLimit int) ([]model.Question, error) {
			switch cat {
			case string(model.CategoryReading):
				// The review already consumed the reading budget, so the relay
				// must clamp the reading allocation to zero and never fetch.
				t.Errorf("relay fetched reading questions with limit %d despite exhausted cap", limit)
				return nil, nil
			case string(model.CategoryVocabulary):
				// The generic fallback may still return reading rows; appendQuestion
				// must reject them while admitting other categories.
				return []model.Question{
					readingQuestion(100),
					readingQuestion(101),
					{ID: 200, ProficiencyLevel: "N5", Category: model.CategoryVocabulary},
					{ID: 201, ProficiencyLevel: "N5", Category: model.CategoryVocabulary},
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

func TestBuildMorningSession_CurrentLevelMinimumAndLowerDueCap(t *testing.T) {
	ctx := context.Background()
	const userID int64 = 123
	const currentLevel = "N3"

	due := []model.Question{
		{ID: 1, ProficiencyLevel: currentLevel, Category: model.CategoryVocabulary},
		{ID: 2, ProficiencyLevel: currentLevel, Category: model.CategoryVocabulary},
		{ID: 3, ProficiencyLevel: currentLevel, Category: model.CategoryVocabulary},
	}
	for id := 4; id <= 10; id++ {
		due = append(due, model.Question{ID: id, ProficiencyLevel: "N4", Category: model.CategoryVocabulary})
	}

	srsMock := &mockSRS{
		getDueReviewsFn: func(ctx context.Context, gotUserID int64, limit, kanjiRecallLimit int) ([]model.Question, error) {
			if limit != 19 {
				t.Fatalf("general due limit = %d, want 19", limit)
			}
			return due, nil
		},
	}
	nextID := 100
	qFetcher := &mockQuestionFetcher{
		getNewQuestionsFn: func(
			ctx context.Context,
			gotUserID int64,
			language string,
			levels []string,
			category string,
			excludeIDs []int,
			limit, kanjiRecallLimit int,
		) ([]model.Question, error) {
			if !slices.Equal(levels, []string{currentLevel}) {
				t.Fatalf("new levels = %v, want current-only [%s]", levels, currentLevel)
			}
			if category == string(model.CategoryListening) || category == string(model.CategoryReading) {
				q := model.Question{
					ID:               nextID,
					ProficiencyLevel: currentLevel,
					Category:         model.QuestionCategory(category),
				}
				nextID++
				return []model.Question{q}, nil
			}
			questions := make([]model.Question, 0, limit)
			for i := 0; i < limit; i++ {
				questions = append(questions, model.Question{
					ID:               nextID,
					ProficiencyLevel: currentLevel,
					Category:         model.CategoryGrammar,
				})
				nextID++
			}
			return questions, nil
		},
	}
	var sessionQuestions []model.SessionQuestion
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
			sessionQuestions = append([]model.SessionQuestion(nil), questions...)
			return nil
		},
	}

	builder := NewSessionBuilderService(qFetcher, sStore, sqStore, srsMock)
	if _, err := builder.BuildMorningSession(ctx, userID, "ja", currentLevel); err != nil {
		t.Fatalf("BuildMorningSession failed: %v", err)
	}
	if len(sessionQuestions) != 17 {
		t.Fatalf("session question count = %d, want 17", len(sessionQuestions))
	}
	currentCount := 0
	lowerDueCount := 0
	for _, sq := range sessionQuestions {
		if sq.QuestionID >= 1 && sq.QuestionID <= 3 || sq.QuestionID >= 100 {
			currentCount++
		}
		if sq.QuestionID >= 4 && sq.QuestionID <= 10 {
			lowerDueCount++
		}
	}
	if currentCount < 14 {
		t.Fatalf("current-level questions = %d, want at least 14", currentCount)
	}
	if lowerDueCount != 3 {
		t.Fatalf("lower-level due questions = %d, want 3", lowerDueCount)
	}
}

func TestBuildMorningSession_PrioritizesCurrentDueListeningAndReading(t *testing.T) {
	ctx := context.Background()
	const currentLevel = "N5"
	due := make([]model.Question, 0, 12)
	for id := 1; id <= 10; id++ {
		due = append(due, model.Question{ID: id, ProficiencyLevel: currentLevel, Category: model.CategoryVocabulary})
	}
	due = append(due,
		model.Question{ID: 11, ProficiencyLevel: currentLevel, Category: model.CategoryListening},
		model.Question{ID: 12, ProficiencyLevel: currentLevel, Category: model.CategoryReading},
	)
	srsMock := &mockSRS{
		getDueReviewsFn: func(ctx context.Context, userID int64, limit, kanjiRecallLimit int) ([]model.Question, error) {
			if limit != 19 {
				t.Fatalf("general due limit = %d, want 19", limit)
			}
			return due, nil
		},
		getDueReviewsForCategoriesFn: func(ctx context.Context, userID int64, limit, kanjiRecallLimit int, categories ...model.QuestionCategory) ([]model.Question, error) {
			if limit != 1 || len(categories) != 1 {
				t.Fatalf("category due query limit/categories = %d/%v, want 1/one", limit, categories)
			}
			for _, q := range due {
				if q.Category == categories[0] {
					return []model.Question{q}, nil
				}
			}
			return nil, nil
		},
	}
	nextID := 100
	qFetcher := &mockQuestionFetcher{
		getNewQuestionsFn: func(
			ctx context.Context,
			userID int64,
			language string,
			levels []string,
			category string,
			excludeIDs []int,
			limit, kanjiRecallLimit int,
		) ([]model.Question, error) {
			if !slices.Equal(levels, []string{currentLevel}) {
				t.Fatalf("new levels = %v, want current-only [%s]", levels, currentLevel)
			}
			if category == string(model.CategoryReading) || category == string(model.CategoryListening) {
				return nil, nil
			}
			questions := make([]model.Question, 0, limit)
			for i := 0; i < limit; i++ {
				questions = append(
					questions,
					model.Question{ID: nextID, ProficiencyLevel: currentLevel, Category: model.CategoryGrammar},
				)
				nextID++
			}
			return questions, nil
		},
	}
	var sessionQuestions []model.SessionQuestion
	sStore := &mockSessionStore{createSessionFn: func(ctx context.Context, session *model.Session) error {
		session.ID = 10
		return nil
	}}
	sqStore := &mockSessionQuestionStore{
		createSessionQuestionsFn: func(ctx context.Context, questions []model.SessionQuestion) error {
			sessionQuestions = append([]model.SessionQuestion(nil), questions...)
			return nil
		},
	}

	builder := NewSessionBuilderService(qFetcher, sStore, sqStore, srsMock)
	if _, err := builder.BuildMorningSession(ctx, 123, "ja", currentLevel); err != nil {
		t.Fatalf("BuildMorningSession failed: %v", err)
	}
	listeningCount := 0
	readingCount := 0
	for _, sq := range sessionQuestions {
		if sq.QuestionID == 11 {
			listeningCount++
		}
		if sq.QuestionID == 12 {
			readingCount++
		}
	}
	if listeningCount != 1 || readingCount != 1 {
		t.Fatalf(
			"reserved due listening/reading = %d/%d, want 1/1; questions=%+v",
			listeningCount,
			readingCount,
			sessionQuestions,
		)
	}
}

func TestBuildEveningSession_UsesN3CurrentScopeBeforeAdjacentFallback(t *testing.T) {
	ctx := context.Background()
	const currentLevel = "N3"
	var currentFetches, adjacentFetches int
	nextID := 100
	srsMock := &mockSRS{getDueReviewsFn: func(context.Context, int64, int, int) ([]model.Question, error) {
		return nil, nil
	}}
	qFetcher := &mockQuestionFetcher{
		getNewQuestionsFn: func(
			ctx context.Context,
			userID int64,
			language string,
			levels []string,
			category string,
			excludeIDs []int,
			limit, kanjiRecallLimit int,
		) ([]model.Question, error) {
			switch {
			case slices.Equal(levels, []string{currentLevel}):
				currentFetches++
			case slices.Equal(levels, []string{"N4", "N3", "N2"}):
				adjacentFetches++
			default:
				t.Fatalf("unexpected level scope %v", levels)
			}
			if limit == 0 {
				return nil, nil
			}
			if category == string(model.CategoryVocabulary) {
				q := model.Question{ID: nextID, ProficiencyLevel: currentLevel, Category: model.CategoryVocabulary}
				nextID++
				return []model.Question{q}, nil
			}
			questionCategory := model.CategoryGrammar
			if category == string(model.CategoryListening) || category == string(model.CategoryReading) {
				questionCategory = model.QuestionCategory(category)
			}
			questions := make([]model.Question, 0, limit)
			for i := 0; i < limit; i++ {
				questions = append(
					questions,
					model.Question{ID: nextID, ProficiencyLevel: currentLevel, Category: questionCategory},
				)
				nextID++
			}
			return questions, nil
		},
	}
	var sessionQuestions []model.SessionQuestion
	sStore := &mockSessionStore{createSessionFn: func(ctx context.Context, session *model.Session) error {
		if session.TotalQuestions != 12 {
			t.Fatalf("TotalQuestions = %d, want 12", session.TotalQuestions)
		}
		session.ID = 10
		return nil
	}}
	sqStore := &mockSessionQuestionStore{
		createSessionQuestionsFn: func(ctx context.Context, questions []model.SessionQuestion) error {
			sessionQuestions = append([]model.SessionQuestion(nil), questions...)
			return nil
		},
	}

	builder := NewSessionBuilderService(qFetcher, sStore, sqStore, srsMock)
	if _, err := builder.BuildEveningSession(ctx, 123, "ja", currentLevel); err != nil {
		t.Fatalf("BuildEveningSession failed: %v", err)
	}
	if currentFetches == 0 {
		t.Fatal("expected current-level fetches")
	}
	if adjacentFetches != 0 {
		t.Fatalf("adjacent fetches = %d, want 0 while current supply is available", adjacentFetches)
	}
	if len(sessionQuestions) != 12 {
		t.Fatalf("session question count = %d, want 12", len(sessionQuestions))
	}
}

func TestBuildEveningSession_FallsBackToAdjacentWhenCurrentSupplyIsShort(t *testing.T) {
	ctx := context.Background()
	const currentLevel = "N3"
	var currentFetches, adjacentFetches int
	nextID := 200
	srsMock := &mockSRS{getDueReviewsFn: func(context.Context, int64, int, int) ([]model.Question, error) {
		return nil, nil
	}}
	qFetcher := &mockQuestionFetcher{
		getNewQuestionsFn: func(
			ctx context.Context,
			userID int64,
			language string,
			levels []string,
			category string,
			excludeIDs []int,
			limit, kanjiRecallLimit int,
		) ([]model.Question, error) {
			switch {
			case slices.Equal(levels, []string{currentLevel}):
				currentFetches++
				return nil, nil
			case slices.Equal(levels, []string{"N4", "N3", "N2"}):
				adjacentFetches++
			default:
				t.Fatalf("unexpected level scope %v", levels)
			}
			questionCategory := model.CategoryGrammar
			if category == string(model.CategoryListening) || category == string(model.CategoryReading) {
				questionCategory = model.QuestionCategory(category)
			}
			questions := make([]model.Question, 0, limit)
			for i := 0; i < limit; i++ {
				questions = append(
					questions,
					model.Question{ID: nextID, ProficiencyLevel: "N4", Category: questionCategory},
				)
				nextID++
			}
			return questions, nil
		},
	}
	sStore := &mockSessionStore{createSessionFn: func(ctx context.Context, session *model.Session) error {
		session.ID = 10
		return nil
	}}
	sqStore := &mockSessionQuestionStore{
		createSessionQuestionsFn: func(context.Context, []model.SessionQuestion) error {
			return nil
		},
	}

	builder := NewSessionBuilderService(qFetcher, sStore, sqStore, srsMock)
	if session, err := builder.BuildEveningSession(ctx, 123, "ja", currentLevel); err != nil {
		t.Fatalf("BuildEveningSession failed: %v", err)
	} else if session == nil || session.TotalQuestions != 12 {
		t.Fatalf("session = %+v, want 12 questions", session)
	}
	if currentFetches == 0 || adjacentFetches == 0 {
		t.Fatalf("fetches current=%d adjacent=%d, want both", currentFetches, adjacentFetches)
	}
}
