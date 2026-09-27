package service

import (
	"context"
	"slices"
	"testing"

	"github.com/lsj/copylingo/internal/model"
)

func TestMaintenanceMaterialCapAcrossNewCurrentAndDueAdjacentQueries(t *testing.T) {
	for _, maintenance := range []bool{true, false} {
		name := "normal variants remain available"
		if maintenance {
			name = "maintenance variants share one slot"
		}
		t.Run(name, func(t *testing.T) {
			shared, normal := 1, 2
			newQuestions := []model.Question{
				{
					ID:                 1,
					MaterialID:         &shared,
					IsMaintenanceCheck: maintenance,
					ProficiencyLevel:   "N4",
					Category:           model.CategoryVocabulary,
				},
				{ID: 2, MaterialID: &normal, ProficiencyLevel: "N4", Category: model.CategoryVocabulary},
				{ID: 3, MaterialID: &normal, ProficiencyLevel: "N4", Category: model.CategoryVocabulary},
			}
			due := []model.Question{
				{
					ID:                 10,
					MaterialID:         &shared,
					IsMaintenanceCheck: maintenance,
					ProficiencyLevel:   "N5",
					Category:           model.CategoryVocabulary,
				},
				{ID: 11, ProficiencyLevel: "N5", Category: model.CategoryVocabulary},
			}
			getDue := func(_ context.Context, _ int64, limit, _ int, categories ...model.QuestionCategory) ([]model.Question, error) {
				var result []model.Question
				for _, q := range due {
					if len(categories) > 0 && !slices.Contains(categories, q.Category) {
						continue
					}
					result = append(result, q)
					if len(result) >= limit {
						break
					}
				}
				return result, nil
			}
			fetcher := &mockQuestionFetcher{getNewQuestionsFn: func(_ context.Context, _ int64, _ string,
				levels []string, category string, excluded []int, limit, _ int,
			) ([]model.Question, error) {
				var result []model.Question
				for _, q := range newQuestions {
					if !slices.Contains(levels, q.ProficiencyLevel) || slices.Contains(excluded, q.ID) ||
						string(q.Category) != category {
						continue
					}
					result = append(result, q)
					if len(result) >= limit {
						break
					}
				}
				return result, nil
			}}
			var selected []int
			builder := NewSessionBuilderService(
				fetcher,
				&mockSessionStore{
					createSessionFn: func(_ context.Context, s *model.Session) error { s.ID = 77; return nil },
				},
				&mockSessionQuestionStore{
					createSessionQuestionsFn: func(_ context.Context, rows []model.SessionQuestion) error {
						for _, row := range rows {
							selected = append(selected, row.QuestionID)
						}
						return nil
					},
				},
				&mockSRS{
					getDueReviewsFn: func(ctx context.Context, userID int64, limit, recall int) ([]model.Question, error) {
						return getDue(ctx, userID, limit, recall)
					},
					getDueReviewsForCategoriesFn: getDue,
				},
			)
			session, err := builder.BuildEveningSession(context.Background(), 42, "ja", "N4")
			if err != nil || session == nil {
				t.Fatalf("build: session=%+v err=%v", session, err)
			}
			for _, id := range []int{1, 2, 3, 11} {
				if !slices.Contains(selected, id) {
					t.Fatalf("expected available question %d in %v", id, selected)
				}
			}
			if slices.Contains(selected, 10) == maintenance {
				t.Fatalf("adjacent variant admission maintenance=%v selected=%v", maintenance, selected)
			}
			if session.TotalQuestions != len(selected) {
				t.Fatalf("total=%d selected=%v", session.TotalQuestions, selected)
			}
		})
	}
}
