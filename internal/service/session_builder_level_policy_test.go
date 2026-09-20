package service

import (
	"context"
	"slices"
	"sort"
	"testing"

	"github.com/lsj/copylingo/internal/model"
)

func TestDailyQuizCurrentLevelPolicy(t *testing.T) {
	questions := func(start, count int, level string, category model.QuestionCategory) []model.Question {
		rows := make([]model.Question, count)
		for i := range rows {
			rows[i] = model.Question{ID: start + i, ProficiencyLevel: level, Category: category}
		}
		return rows
	}
	for _, tt := range []struct {
		name        string
		level       string
		due         []model.Question
		new         []model.Question
		wantReading bool
		wantAudio   bool
	}{
		{"current due alone meets the target", "N4", questions(1, 30, "N4", model.CategoryVocabulary), nil, false, false},
		{"current grammar precedes adjacent new vocabulary", "N4", questions(1, 30, "N5", model.CategoryVocabulary),
			append(questions(101, 30, "N4", model.CategoryGrammar), questions(201, 30, "N5", model.CategoryVocabulary)...), false, false},
		{"reading cap cannot hide current grammar", "N4", nil,
			append(questions(101, 30, "N4", model.CategoryReading), questions(201, 30, "N4", model.CategoryGrammar)...), true, false},
		{"reserved due beyond the general pool remains reachable", "N4",
			append(questions(1, 30, "N4", model.CategoryVocabulary),
				model.Question{ID: 50, ProficiencyLevel: "N4", Category: model.CategoryReading},
				model.Question{ID: 51, ProficiencyLevel: "N4", Category: model.CategoryListening}), nil, true, true},
		{"upper due cannot take N3 slots", "N3", questions(1, 30, "N2", model.CategoryVocabulary),
			questions(101, 30, "N3", model.CategoryGrammar), false, false},
	} {
		for _, profile := range []struct {
			name  string
			total int
			floor int
		}{{"morning", 17, 14}, {"evening", 12, 10}} {
			t.Run(tt.name+"/"+profile.name, func(t *testing.T) {
				byID := make(map[int]model.Question)
				for _, q := range append(slices.Clone(tt.due), tt.new...) {
					byID[q.ID] = q
				}
				// Fake the repository contract: level-first due ordering and caps
				// apply before LIMIT, with category filtering before selection.
				getDue := func(_ context.Context, _ int64, limit, recall int, categories ...model.QuestionCategory) ([]model.Question, error) {
					if limit > profile.total+2 {
						t.Fatalf("unbounded due query: %d", limit)
					}
					pool := slices.Clone(tt.due)
					sort.SliceStable(pool, func(i, j int) bool {
						return pool[i].ProficiencyLevel == tt.level && pool[j].ProficiencyLevel != tt.level
					})
					var result []model.Question
					reading := 0
					for _, q := range pool {
						if len(categories) > 0 && !slices.Contains(categories, q.Category) {
							continue
						}
						if q.Category == model.CategoryReading {
							if reading == 1 {
								continue
							}
							reading++
						}
						result = append(result, q)
						if len(result) == limit {
							break
						}
					}
					return result, nil
				}
				srs := &mockSRS{
					getDueReviewsFn: func(ctx context.Context, userID int64, limit, recall int) ([]model.Question, error) {
						return getDue(ctx, userID, limit, recall)
					},
					getDueReviewsForCategoriesFn: getDue,
				}
				fetcher := &mockQuestionFetcher{getNewQuestionsFn: func(_ context.Context, _ int64, _ string,
					levels []string, category string, exclude []int, limit, recall int,
				) ([]model.Question, error) {
					if limit > profile.total {
						t.Fatalf("unbounded new query: %d", limit)
					}
					var result []model.Question
					for _, q := range tt.new {
						if !slices.Contains(levels, q.ProficiencyLevel) || slices.Contains(exclude, q.ID) ||
							(category != "" && string(q.Category) != category) {
							continue
						}
						result = append(result, q)
						if len(result) == limit {
							break
						}
					}
					return result, nil
				}}
				var selected []model.SessionQuestion
				builder := NewSessionBuilderService(
					fetcher,
					&mockSessionStore{
						createSessionFn: func(_ context.Context, s *model.Session) error { s.ID = 99; return nil },
					},
					&mockSessionQuestionStore{
						createSessionQuestionsFn: func(_ context.Context, rows []model.SessionQuestion) error {
							selected = rows
							return nil
						},
					},
					srs,
				)
				var session *model.Session
				var err error
				if profile.name == "morning" {
					session, err = builder.BuildMorningSession(context.Background(), 42, "ja", tt.level)
				} else {
					session, err = builder.BuildEveningSession(context.Background(), 42, "ja", tt.level)
				}
				if err != nil || session == nil || session.TotalQuestions != profile.total ||
					len(selected) != profile.total {
					t.Fatalf("session = %+v, selected = %d, error = %v", session, len(selected), err)
				}
				current, reading, audio := 0, 0, 0
				seen := make(map[int]bool)
				for _, row := range selected {
					q := byID[row.QuestionID]
					if seen[q.ID] {
						t.Fatalf("duplicate question %d", q.ID)
					}
					seen[q.ID] = true
					if q.ProficiencyLevel == tt.level {
						current++
					} else if !row.IsReview || !isLowerAdjacentQuestion(q, "ja", tt.level) {
						t.Fatalf("non-current question admitted despite sufficient current supply: %+v", q)
					}
					if q.ProficiencyLevel == tt.level && q.Category == model.CategoryReading {
						reading++
					}
					if q.ProficiencyLevel == tt.level && q.Category == model.CategoryListening {
						audio++
					}
				}
				if current < profile.floor || reading > 1 || (tt.wantReading && reading != 1) ||
					(tt.wantAudio && audio < 1) {
					t.Fatalf("current/reading/listening = %d/%d/%d", current, reading, audio)
				}
			})
		}
	}
}
