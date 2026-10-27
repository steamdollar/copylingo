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
		t.Run(
			name,
			func(t *testing.T) {
				shared, normal := 1, 2
				pool := &builderPool{
					new: []model.Question{
						{
							ID:                 1,
							MaterialID:         &shared,
							IsMaintenanceCheck: maintenance,
							ProficiencyLevel:   "N4",
							Category:           model.CategoryVocabulary,
						},
						{ID: 2, MaterialID: &normal, ProficiencyLevel: "N4", Category: model.CategoryVocabulary},
						{ID: 3, MaterialID: &normal, ProficiencyLevel: "N4", Category: model.CategoryVocabulary},
					},
					due: []model.Question{
						{
							ID:                 10,
							MaterialID:         &shared,
							IsMaintenanceCheck: maintenance,
							ProficiencyLevel:   "N5",
							Category:           model.CategoryVocabulary,
						},
						{ID: 11, ProficiencyLevel: "N5", Category: model.CategoryVocabulary},
					},
				}
				builder, stores := newTestBuilder(pool)
				session, err := builder.BuildEveningSession(
					context.Background(),
					42,
					"ja",
					"N4",
				)
				if err != nil || session == nil {
					t.Fatalf(
						"build: session=%+v err=%v",
						session,
						err,
					)
				}
				selected := rowIDs(stores.rows)
				for _, id := range []int{1, 2, 3, 11} {
					if !slices.Contains(
						selected,
						id,
					) {
						t.Fatalf(
							"expected available question %d in %v",
							id,
							selected,
						)
					}
				}
				if slices.Contains(
					selected,
					10,
				) == maintenance {
					t.Fatalf(
						"adjacent variant admission maintenance=%v selected=%v",
						maintenance,
						selected,
					)
				}
				if session.TotalQuestions != len(selected) {
					t.Fatalf(
						"total=%d selected=%v",
						session.TotalQuestions,
						selected,
					)
				}
			},
		)
	}
}
