package service

import (
	"context"
	"errors"
	"slices"
	"strings"
	"testing"

	"github.com/lsj/copylingo/internal/model"
)

// newQuery and dueQuery record the arguments of one QuestionRepository call.
type newQuery struct {
	levels           []string
	category         string
	limit            int
	kanjiRecallLimit int
}

type dueQuery struct {
	userID           int64
	language         string
	currentLevel     string
	levels           []string
	limit            int
	kanjiRecallLimit int
	categories       []model.QuestionCategory
}

// builderPool is an in-memory QuestionRepository for the session builder. Due rows
// are filtered by category and LIMIT; new rows by level scope, category,
// exclusions and LIMIT, so a selected question is never offered twice. newFunc
// replaces the new-row lookup for tests that need a misbehaving repository.
// Every call is recorded so tests can assert on the arguments after the build.
type builderPool struct {
	QuestionRepository
	due        []model.Question
	new        []model.Question
	newFunc    func(newQuery) []model.Question
	dueQueries []dueQuery
	newQueries []newQuery
}

func (pool *builderPool) GetDueReviews(
	_ context.Context,
	userID int64,
	language,
	currentLevel string,
	levels []string,
	limit,
	kanjiRecallLimit int,
	categories ...model.QuestionCategory,
) ([]model.Question, error) {
	pool.dueQueries = append(
		pool.dueQueries,
		dueQuery{
			userID:           userID,
			language:         language,
			currentLevel:     currentLevel,
			levels:           levels,
			limit:            limit,
			kanjiRecallLimit: kanjiRecallLimit,
			categories:       categories,
		},
	)
	var rows []model.Question
	for _, question := range pool.due {
		if len(rows) == limit {
			break
		}
		if len(categories) == 0 || slices.Contains(
			categories,
			question.Category,
		) {
			rows = append(
				rows,
				question,
			)
		}
	}
	return rows, nil
}

func (pool *builderPool) GetNewQuestions(
	_ context.Context,
	_ int64,
	_ string,
	levels []string,
	category string,
	excludeIDs []int,
	limit,
	kanjiRecallLimit int,
) ([]model.Question, error) {
	query := newQuery{levels: levels, category: category, limit: limit, kanjiRecallLimit: kanjiRecallLimit}
	pool.newQueries = append(
		pool.newQueries,
		query,
	)
	if pool.newFunc != nil {
		return pool.newFunc(query), nil
	}
	var rows []model.Question
	for _, question := range pool.new {
		if len(rows) == limit {
			break
		}
		if slices.Contains(
			levels,
			question.ProficiencyLevel,
		) && string(question.Category) == category && !slices.Contains(
			excludeIDs,
			question.ID,
		) {
			rows = append(
				rows,
				question,
			)
		}
	}
	return rows, nil
}

func (pool *builderPool) generalDueQuery(t *testing.T) dueQuery {
	t.Helper()
	for _, query := range pool.dueQueries {
		if len(query.categories) == 0 {
			return query
		}
	}
	t.Fatal("no general due query")
	return dueQuery{}
}

// builderStores captures what the builder persists. CreateSession assigns ID
// 10 unless createErr is set; CreateSessionQuestions returns rowsErr.
type builderStores struct {
	SessionRepository
	createErr error
	rowsErr   error
	session   *model.Session
	rows      []model.SessionQuestion
}

func (stores *builderStores) CreateSession(
	_ context.Context,
	session *model.Session,
) error {
	if stores.createErr != nil {
		return stores.createErr
	}
	session.ID = 10
	stores.session = session
	return nil
}

func (stores *builderStores) CreateSessionQuestions(
	_ context.Context,
	rows []model.SessionQuestion,
) error {
	stores.rows = rows
	return stores.rowsErr
}

func newTestBuilder(pool *builderPool) (*sessionBuilderService, *builderStores) {
	stores := &builderStores{}
	return newSessionBuilderService(
		pool,
		stores,
		stores,
	), stores
}

func makeQuestions(
	start,
	count int,
	level string,
	category model.QuestionCategory,
) []model.Question {
	questions := make(
		[]model.Question,
		count,
	)
	for i := range questions {
		questions[i] = model.Question{ID: start + i, ProficiencyLevel: level, Category: category}
	}
	return questions
}

func rowIDs(rows []model.SessionQuestion) []int {
	ids := make(
		[]int,
		len(rows),
	)
	for i, row := range rows {
		ids[i] = row.QuestionID
	}
	return ids
}

func TestBuildMorningSession_MixesReviewAndNew(t *testing.T) {
	pool := &builderPool{
		due: makeQuestions(
			1,
			4,
			"N5",
			"",
		),
		new: makeQuestions(
			1000,
			9,
			"N5",
			model.CategoryGrammar,
		),
	}
	builder, stores := newTestBuilder(pool)
	if _, err := builder.BuildMorningSession(
		context.Background(),
		123,
		"ja",
		"N5",
	); err != nil {
		t.Fatalf(
			"BuildMorningSession failed: %v",
			err,
		)
	}

	// The vocabulary reservation runs first and the general due pool is bounded.
	if first := pool.newQueries[0]; first.category != string(model.CategoryVocabulary) || first.limit != 6 {
		t.Fatalf(
			"first new query = %+v, want 6 reserved vocabulary slots",
			first,
		)
	}
	if limit := pool.generalDueQuery(t).limit; limit != 19 {
		t.Fatalf(
			"general due limit = %d, want 19",
			limit,
		)
	}
	// 4 reviews + 9 new grammar questions.
	if stores.session.TotalQuestions != 13 || len(stores.rows) != 13 {
		t.Fatalf(
			"total = %d, rows = %d, want 13",
			stores.session.TotalQuestions,
			len(stores.rows),
		)
	}
}

func TestBuildMorningSession_ReservesListeningAndBuildsSeventeenQuestions(t *testing.T) {
	pool := &builderPool{
		due: makeQuestions(
			1,
			6,
			"N5",
			"",
		),
		new: slices.Concat(
			makeQuestions(
				101,
				6,
				"N5",
				model.CategoryVocabulary,
			),
			makeQuestions(
				201,
				1,
				"N5",
				model.CategoryListening,
			),
			makeQuestions(
				301,
				20,
				"N5",
				model.CategoryGrammar,
			),
		),
	}
	builder, stores := newTestBuilder(pool)
	if _, err := builder.BuildMorningSession(
		context.Background(),
		123,
		"ja",
		"N5",
	); err != nil {
		t.Fatalf(
			"BuildMorningSession failed: %v",
			err,
		)
	}

	for _, query := range pool.newQueries {
		// Reserved listening/reading slots may probe one adjacent level once the
		// current level has no new supply (ADR-079); everything else stays current.
		reservedProbe := len(query.levels) == 1 &&
			(query.category == string(model.CategoryListening) || query.category == string(model.CategoryReading))
		if !reservedProbe && !slices.Equal(
			query.levels,
			[]string{"N5"},
		) {
			t.Fatalf(
				"new query levels = %v, want [N5] or a reserved adjacent probe",
				query.levels,
			)
		}
	}
	listening := slices.IndexFunc(
		pool.newQueries,
		func(query newQuery) bool { return query.category == string(model.CategoryListening) },
	)
	if listening < 0 || pool.newQueries[listening].limit != 1 {
		t.Fatalf(
			"reserved listening query index = %d, want limit 1",
			listening,
		)
	}
	if stores.session.TotalQuestions != 17 || !slices.Contains(
		rowIDs(stores.rows),
		201,
	) {
		t.Fatalf(
			"total = %d, rows = %v, want 17 with listening 201",
			stores.session.TotalQuestions,
			rowIDs(stores.rows),
		)
	}
}

func TestBuildEveningSession_ReservesOneThirdForVocabulary(t *testing.T) {
	pool := &builderPool{
		due: makeQuestions(
			1,
			10,
			"N5",
			"",
		),
		new: slices.Concat(
			makeQuestions(
				101,
				6,
				"N5",
				model.CategoryVocabulary,
			),
			makeQuestions(
				150,
				1,
				"N5",
				model.CategoryListening,
			),
		),
	}
	builder, stores := newTestBuilder(pool)
	if _, err := builder.BuildEveningSession(
		context.Background(),
		123,
		"ja",
		"N5",
	); err != nil {
		t.Fatalf(
			"BuildEveningSession failed: %v",
			err,
		)
	}

	if first := pool.newQueries[0]; first.category != string(model.CategoryVocabulary) || first.limit != 6 {
		t.Fatalf(
			"first new query = %+v, want 6 reserved vocabulary slots",
			first,
		)
	}
	if limit := pool.generalDueQuery(t).limit; limit != 19 {
		t.Fatalf(
			"general due limit = %d, want 19",
			limit,
		)
	}
	if stores.session.TotalQuestions != 17 || len(stores.rows) != 17 {
		t.Fatalf(
			"total = %d, rows = %d, want 17",
			stores.session.TotalQuestions,
			len(stores.rows),
		)
	}
	// Reserved vocabulary and listening enter as new questions despite the
	// due backlog.
	for _, wantID := range []int{101, 102, 103, 104, 105, 106, 150} {
		if !slices.ContainsFunc(
			stores.rows,
			func(row model.SessionQuestion) bool {
				return row.QuestionID == wantID && !row.IsReview
			},
		) {
			t.Fatalf(
				"reserved new question %d missing",
				wantID,
			)
		}
	}
}

func TestBuildEveningSession_FillsVocabularyShortageWithRelay(t *testing.T) {
	pool := &builderPool{
		due: makeQuestions(
			1,
			6,
			"N5",
			"",
		),
		new: slices.Concat(
			makeQuestions(
				101,
				2,
				"N5",
				model.CategoryVocabulary,
			),
			makeQuestions(
				201,
				4,
				"N5",
				model.CategoryGrammar,
			),
		),
	}
	builder, stores := newTestBuilder(pool)
	if _, err := builder.BuildEveningSession(
		context.Background(),
		123,
		"ja",
		"N5",
	); err != nil {
		t.Fatalf(
			"BuildEveningSession failed: %v",
			err,
		)
	}

	// 6 reviews + 2 vocabulary; the relay fills the shortage with 4 grammar.
	if stores.session.TotalQuestions != 12 {
		t.Fatalf(
			"total = %d, want 12",
			stores.session.TotalQuestions,
		)
	}
}

func TestBuildReviewSession_OnlySRS(t *testing.T) {
	pool := &builderPool{due: makeQuestions(
		1,
		2,
		"",
		"",
	)}
	builder, _ := newTestBuilder(pool)
	session, err := builder.BuildReviewSession(
		context.Background(),
		123,
		"ja",
		"N5",
		5,
	)
	if err != nil {
		t.Fatalf(
			"BuildReviewSession failed: %v",
			err,
		)
	}

	if session.TotalQuestions != 2 || len(pool.newQueries) != 0 {
		t.Fatalf(
			"total = %d, new queries = %d, want 2 due only",
			session.TotalQuestions,
			len(pool.newQueries),
		)
	}
	// Due reviews span the same level scope as new questions.
	query := pool.dueQueries[0]
	if query.userID != 123 || query.language != "ja" || query.currentLevel != "N5" || !slices.Equal(
		query.levels,
		sessionLevelsFor(
			"ja",
			"N5",
		),
	) {
		t.Fatalf(
			"due query = %+v, want user 123 ja/N5 with session levels",
			query,
		)
	}
}

func TestBuildReviewSession_CapsKanjiRecallAdmissionAtThree(t *testing.T) {
	kanjiSkill := model.SkillVocabularyKanjiRecall
	pool := &builderPool{due: []model.Question{
		{ID: 1, Skill: &kanjiSkill},
		{ID: 2, Skill: &kanjiSkill},
		{ID: 3, Skill: &kanjiSkill},
		{ID: 4, Skill: &kanjiSkill},
		{ID: 5, Skill: &kanjiSkill},
		{ID: 6},
		{ID: 7},
	}}
	builder, stores := newTestBuilder(pool)
	if _, err := builder.BuildReviewSession(
		context.Background(),
		123,
		"ja",
		"N5",
		10,
	); err != nil {
		t.Fatalf(
			"BuildReviewSession failed: %v",
			err,
		)
	}

	if query := pool.dueQueries[0]; query.limit != 10 || query.kanjiRecallLimit != maxKanjiRecallPerSession {
		t.Fatalf(
			"due query limit = %d, kanjiRecallLimit = %d, want 10, %d",
			query.limit,
			query.kanjiRecallLimit,
			maxKanjiRecallPerSession,
		)
	}
	// The repository returned five kanji-recall rows; the builder admits three.
	if got := rowIDs(stores.rows); !slices.Equal(
		got,
		[]int{1, 2, 3, 6, 7},
	) || stores.session.TotalQuestions != 5 {
		t.Fatalf(
			"selected = %v, total = %d, want [1 2 3 6 7]",
			got,
			stores.session.TotalQuestions,
		)
	}
}

func TestBuildMorningSession_PassesRemainingKanjiRecallBudgetToNewFetches(t *testing.T) {
	kanjiSkill := model.SkillVocabularyKanjiRecall
	due := makeQuestions(
		1,
		6,
		"N5",
		"",
	)
	due[0].Skill, due[1].Skill = &kanjiSkill, &kanjiSkill
	fresh := makeQuestions(
		101,
		5,
		"N5",
		model.CategoryVocabulary,
	)
	fresh[0].Skill = &kanjiSkill
	pool := &builderPool{due: due, new: fresh}
	builder, stores := newTestBuilder(pool)
	if _, err := builder.BuildMorningSession(
		context.Background(),
		123,
		"ja",
		"N5",
	); err != nil {
		t.Fatalf(
			"BuildMorningSession failed: %v",
			err,
		)
	}

	// Two due kanji-recall reviews leave one slot for the first new fetch; the
	// new kanji-recall question then exhausts the budget for later fetches.
	if first := pool.newQueries[0]; first.category != string(model.CategoryVocabulary) || first.kanjiRecallLimit != 1 {
		t.Fatalf(
			"first new query = %+v, want vocabulary with kanjiRecallLimit 1",
			first,
		)
	}
	for _, query := range pool.newQueries[1:] {
		if query.kanjiRecallLimit != 0 {
			t.Fatalf(
				"later new query = %+v, want kanjiRecallLimit 0",
				query,
			)
		}
	}
	if stores.session.TotalQuestions != 11 {
		t.Fatalf(
			"total = %d, want 11",
			stores.session.TotalQuestions,
		)
	}
}

func TestBuildSession_NoQuestions(t *testing.T) {
	builder, stores := newTestBuilder(&builderPool{})
	session, err := builder.BuildMorningSession(
		context.Background(),
		123,
		"ja",
		"N5",
	)
	if err != nil {
		t.Fatalf(
			"BuildMorningSession failed: %v",
			err,
		)
	}
	if session != nil || stores.session != nil {
		t.Fatalf(
			"session = %+v, want nothing created when no questions are found",
			session,
		)
	}
}

func TestBuildSession_CreateFails(t *testing.T) {
	builder, stores := newTestBuilder(&builderPool{due: makeQuestions(
		1,
		1,
		"",
		"",
	)})
	stores.createErr = errors.New("db error")
	_, err := builder.BuildReviewSession(
		context.Background(),
		123,
		"ja",
		"N5",
		5,
	)
	if !errors.Is(
		err,
		stores.createErr,
	) || stores.rows != nil {
		t.Fatalf(
			"error = %v, rows = %v, want CreateSession error and no rows",
			err,
			stores.rows,
		)
	}
}

func TestBuildSession_CreateSessionQuestionsFails(t *testing.T) {
	builder, stores := newTestBuilder(&builderPool{due: makeQuestions(
		1,
		2,
		"",
		"",
	)})
	stores.rowsErr = errors.New("create session questions failed")
	_, err := builder.BuildReviewSession(
		context.Background(),
		123,
		"ja",
		"N5",
		5,
	)
	if !errors.Is(
		err,
		stores.rowsErr,
	) {
		t.Fatalf(
			"error = %v, want %v",
			err,
			stores.rowsErr,
		)
	}
	// Rows carry the new session's ID before they are stored.
	if len(stores.rows) != 2 || stores.rows[0].SessionID != 10 || stores.rows[1].SessionID != 10 {
		t.Fatalf(
			"rows = %+v, want 2 rows of session 10",
			stores.rows,
		)
	}
}

func TestBuildSession_DeduplicatesQuestionIDs(t *testing.T) {
	pool := &builderPool{
		due: []model.Question{{ID: 1, ProficiencyLevel: "N5"}, {ID: 1, ProficiencyLevel: "N5"}},
		// This repository ignores exclusions, so the builder must drop repeated
		// and already-selected IDs itself.
		newFunc: func(newQuery) []model.Question {
			return []model.Question{
				{ID: 1, ProficiencyLevel: "N5"},
				{ID: 2, ProficiencyLevel: "N5"},
				{ID: 2, ProficiencyLevel: "N5"},
			}
		},
	}
	builder, stores := newTestBuilder(pool)
	if _, err := builder.BuildMorningSession(
		context.Background(),
		123,
		"ja",
		"N5",
	); err != nil {
		t.Fatalf(
			"BuildMorningSession failed: %v",
			err,
		)
	}
	if got := rowIDs(stores.rows); !slices.Equal(
		got,
		[]int{1, 2},
	) || stores.session.TotalQuestions != 2 {
		t.Fatalf(
			"selected = %v, total = %d, want [1 2]",
			got,
			stores.session.TotalQuestions,
		)
	}
}

// Reading is capped at one question per session across every admission path:
// due reviews, the relay, and the final fallback (ADR-036).
func TestBuildSession_CapsReadingAtOne(t *testing.T) {
	pool := &builderPool{
		// Two due reading reviews: only the first may enter the session.
		due: makeQuestions(
			1,
			2,
			"N5",
			model.CategoryReading,
		),
		// A vocabulary query may still return reading rows; appendQuestion must
		// reject them while admitting other categories.
		newFunc: func(query newQuery) []model.Question {
			if query.category != string(model.CategoryVocabulary) {
				return nil
			}
			return slices.Concat(
				makeQuestions(
					100,
					2,
					"N5",
					model.CategoryReading,
				),
				makeQuestions(
					200,
					2,
					"N5",
					model.CategoryVocabulary,
				),
			)
		},
	}
	builder, stores := newTestBuilder(pool)
	if _, err := builder.BuildMorningSession(
		context.Background(),
		123,
		"ja",
		"N5",
	); err != nil {
		t.Fatalf(
			"BuildMorningSession failed: %v",
			err,
		)
	}

	// The review already consumed the reading budget, so the relay must clamp
	// the reading allocation to zero and never fetch.
	for _, query := range pool.newQueries {
		if query.category == string(model.CategoryReading) {
			t.Fatalf(
				"fetched reading with limit %d despite exhausted cap",
				query.limit,
			)
		}
	}
	readingCount := 0
	for _, id := range rowIDs(stores.rows) {
		if slices.Contains(
			[]int{1, 2, 100, 101},
			id,
		) {
			readingCount++
		}
	}
	// 1 reading review + 2 fallback vocabulary.
	if readingCount != 1 || len(stores.rows) != 3 {
		t.Fatalf(
			"reading = %d, rows = %v, want 1 reading in 3 rows",
			readingCount,
			rowIDs(stores.rows),
		)
	}
}

func TestBuildMorningSession_CurrentLevelMinimumAndLowerDueCap(t *testing.T) {
	const currentLevel = "N3"
	pool := &builderPool{
		due: slices.Concat(
			makeQuestions(
				1,
				3,
				currentLevel,
				model.CategoryVocabulary,
			),
			makeQuestions(
				4,
				7,
				"N4",
				model.CategoryVocabulary,
			),
		),
		new: slices.Concat(
			makeQuestions(
				100,
				20,
				currentLevel,
				model.CategoryVocabulary,
			),
			makeQuestions(
				200,
				20,
				currentLevel,
				model.CategoryGrammar,
			),
			makeQuestions(
				300,
				2,
				currentLevel,
				model.CategoryListening,
			),
			makeQuestions(
				310,
				2,
				currentLevel,
				model.CategoryReading,
			),
		),
	}
	builder, stores := newTestBuilder(pool)
	if _, err := builder.BuildMorningSession(
		context.Background(),
		123,
		"ja",
		currentLevel,
	); err != nil {
		t.Fatalf(
			"BuildMorningSession failed: %v",
			err,
		)
	}

	if limit := pool.generalDueQuery(t).limit; limit != 19 {
		t.Fatalf(
			"general due limit = %d, want 19",
			limit,
		)
	}
	for _, query := range pool.newQueries {
		if !slices.Equal(
			query.levels,
			[]string{currentLevel},
		) {
			t.Fatalf(
				"new query levels = %v, want current-only [%s]",
				query.levels,
				currentLevel,
			)
		}
	}
	if stores.session.TotalQuestions != 17 {
		t.Fatalf(
			"total = %d, want 17",
			stores.session.TotalQuestions,
		)
	}
	currentCount, lowerDueCount := 0, 0
	for _, id := range rowIDs(stores.rows) {
		if id <= 3 || id >= 100 {
			currentCount++
		} else {
			lowerDueCount++
		}
	}
	if currentCount < 14 || lowerDueCount != 3 {
		t.Fatalf(
			"current = %d, lower due = %d, want at least 14 and exactly 3",
			currentCount,
			lowerDueCount,
		)
	}
}

func TestBuildMorningSession_PrioritizesCurrentDueListeningAndReading(t *testing.T) {
	const currentLevel = "N5"
	pool := &builderPool{
		due: slices.Concat(
			makeQuestions(
				1,
				10,
				currentLevel,
				model.CategoryVocabulary,
			),
			makeQuestions(
				11,
				1,
				currentLevel,
				model.CategoryListening,
			),
			makeQuestions(
				12,
				1,
				currentLevel,
				model.CategoryReading,
			),
		),
		new: makeQuestions(
			100,
			20,
			currentLevel,
			model.CategoryGrammar,
		),
	}
	builder, stores := newTestBuilder(pool)
	if _, err := builder.BuildMorningSession(
		context.Background(),
		123,
		"ja",
		currentLevel,
	); err != nil {
		t.Fatalf(
			"BuildMorningSession failed: %v",
			err,
		)
	}

	if limit := pool.generalDueQuery(t).limit; limit != 19 {
		t.Fatalf(
			"general due limit = %d, want 19",
			limit,
		)
	}
	for _, query := range pool.dueQueries {
		if len(query.categories) > 0 && (query.limit != 1 || len(query.categories) != 1) {
			t.Fatalf(
				"category due query = %+v, want limit 1 with one category",
				query,
			)
		}
	}
	for _, query := range pool.newQueries {
		// The new-listening reservation still runs after a due listening item
		// and may probe single adjacent levels; other new queries stay current-only.
		listeningProbe := query.category == string(model.CategoryListening) && len(query.levels) == 1
		if !listeningProbe && !slices.Equal(
			query.levels,
			[]string{currentLevel},
		) {
			t.Fatalf(
				"new query levels = %v, want current-only [%s]",
				query.levels,
				currentLevel,
			)
		}
	}
	// Due listening 11 and reading 12 sit behind ten vocabulary reviews, so
	// only the reserved queries can bring them in.
	ids := rowIDs(stores.rows)
	if !slices.Contains(
		ids,
		11,
	) || !slices.Contains(
		ids,
		12,
	) {
		t.Fatalf(
			"selected = %v, want reserved due listening 11 and reading 12",
			ids,
		)
	}
}

func TestBuildEveningSession_UsesN3CurrentScopeBeforeAdjacentFallback(t *testing.T) {
	const currentLevel = "N3"
	pool := &builderPool{new: slices.Concat(
		makeQuestions(
			100,
			6,
			currentLevel,
			model.CategoryVocabulary,
		),
		makeQuestions(
			200,
			20,
			currentLevel,
			model.CategoryGrammar,
		),
		makeQuestions(
			300,
			1,
			currentLevel,
			model.CategoryListening,
		),
		makeQuestions(
			310,
			1,
			currentLevel,
			model.CategoryReading,
		),
	)}
	builder, stores := newTestBuilder(pool)
	if _, err := builder.BuildEveningSession(
		context.Background(),
		123,
		"ja",
		currentLevel,
	); err != nil {
		t.Fatalf(
			"BuildEveningSession failed: %v",
			err,
		)
	}

	// Current supply fills the session, so no query reaches an adjacent level.
	for _, query := range pool.newQueries {
		if !slices.Equal(
			query.levels,
			[]string{currentLevel},
		) {
			t.Fatalf(
				"new query levels = %v while current supply is available",
				query.levels,
			)
		}
	}
	if len(pool.newQueries) == 0 || stores.session.TotalQuestions != 17 {
		t.Fatalf(
			"new queries = %d, total = %d, want current fetches and 17",
			len(pool.newQueries),
			stores.session.TotalQuestions,
		)
	}
}

func TestBuildEveningSession_FallsBackToAdjacentWhenCurrentSupplyIsShort(t *testing.T) {
	// The current level N3 has no new supply at all.
	pool := &builderPool{new: slices.Concat(
		makeQuestions(
			200,
			20,
			"N4",
			model.CategoryGrammar,
		),
		makeQuestions(
			300,
			1,
			"N4",
			model.CategoryListening,
		),
		makeQuestions(
			310,
			1,
			"N4",
			model.CategoryReading,
		),
	)}
	builder, stores := newTestBuilder(pool)
	if _, err := builder.BuildEveningSession(
		context.Background(),
		123,
		"ja",
		"N3",
	); err != nil {
		t.Fatalf(
			"BuildEveningSession failed: %v",
			err,
		)
	}

	if stores.session.TotalQuestions != 17 {
		t.Fatalf(
			"total = %d, want 17",
			stores.session.TotalQuestions,
		)
	}
	scopes := make(map[string]int)
	for _, query := range pool.newQueries {
		scopes[strings.Join(
			query.levels,
			",",
		)]++
	}
	// Reserved listening/reading slots probe one adjacent level at a time; the
	// general fallback uses the full adjacent scope.
	for scope := range scopes {
		if !slices.Contains(
			[]string{"N3", "N4,N3,N2", "N2", "N4"},
			scope,
		) {
			t.Fatalf(
				"unexpected level scope %s",
				scope,
			)
		}
	}
	if scopes["N3"] == 0 || scopes["N4,N3,N2"] == 0 {
		t.Fatalf(
			"queries by scope = %v, want current and adjacent",
			scopes,
		)
	}
}

func TestBuildMorningSession_ReservedListeningFallsBackToHigherAdjacentFirst(t *testing.T) {
	const (
		currentLevel      = "N4"
		higherListeningID = 500
		lowerListeningID  = 600
	)
	tests := []struct {
		name        string
		higherEmpty bool
		wantID      int
		wantProbes  string
	}{
		{name: "higher level has new listening", wantID: higherListeningID, wantProbes: "N4;N3"},
		{name: "higher level exhausted", higherEmpty: true, wantID: lowerListeningID, wantProbes: "N4;N3;N5"},
	}

	for _, tt := range tests {
		t.Run(
			tt.name,
			func(t *testing.T) {
				// The current level has no new listening left; grammar keeps the
				// rest of the session at the current level.
				fresh := append(
					makeQuestions(
						100,
						20,
						currentLevel,
						model.CategoryGrammar,
					),
					model.Question{ID: lowerListeningID, ProficiencyLevel: "N5", Category: model.CategoryListening},
				)
				if !tt.higherEmpty {
					fresh = append(
						fresh,
						model.Question{
							ID:               higherListeningID,
							ProficiencyLevel: "N3",
							Category:         model.CategoryListening,
						},
					)
				}
				pool := &builderPool{new: fresh}
				builder, stores := newTestBuilder(pool)
				if _, err := builder.BuildMorningSession(
					context.Background(),
					123,
					"ja",
					currentLevel,
				); err != nil {
					t.Fatalf(
						"BuildMorningSession failed: %v",
						err,
					)
				}

				// The reservation probes adjacent levels one at a time, highest
				// first, and stops as soon as the slot is filled.
				var probes []string
				for _, query := range pool.newQueries {
					if query.category == string(model.CategoryListening) {
						probes = append(
							probes,
							strings.Join(
								query.levels,
								",",
							),
						)
					}
				}
				if got := strings.Join(
					probes,
					";",
				); got != tt.wantProbes {
					t.Fatalf(
						"listening probes = %s, want %s",
						got,
						tt.wantProbes,
					)
				}
				var gotIDs []int
				for _, id := range rowIDs(stores.rows) {
					if id == higherListeningID || id == lowerListeningID {
						gotIDs = append(
							gotIDs,
							id,
						)
					}
				}
				if !slices.Equal(
					gotIDs,
					[]int{tt.wantID},
				) {
					t.Fatalf(
						"adjacent listening = %v, want [%d]",
						gotIDs,
						tt.wantID,
					)
				}
			},
		)
	}
}
