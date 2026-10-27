package service

import (
	"context"
	"slices"
	"testing"

	"github.com/lsj/copylingo/internal/model"
)

func TestDailyQuizCurrentLevelPolicy(t *testing.T) {
	for _, tt := range []struct {
		name        string
		level       string
		due         []model.Question
		new         []model.Question
		wantReading bool
		wantAudio   bool
	}{
		{"current due alone meets the target", "N4", makeQuestions(
			1,
			30,
			"N4",
			model.CategoryVocabulary,
		), nil, false, false},
		{"current grammar precedes adjacent new vocabulary", "N4", makeQuestions(
			1,
			30,
			"N5",
			model.CategoryVocabulary,
		),
			append(
				makeQuestions(
					101,
					30,
					"N4",
					model.CategoryGrammar,
				),
				makeQuestions(
					201,
					30,
					"N5",
					model.CategoryVocabulary,
				)...,
			), false, false},
		{"reading cap cannot hide current grammar", "N4", nil,
			append(
				makeQuestions(
					101,
					30,
					"N4",
					model.CategoryReading,
				),
				makeQuestions(
					201,
					30,
					"N4",
					model.CategoryGrammar,
				)...,
			), true, false},
		{"reserved due beyond the general pool remains reachable", "N4",
			append(
				makeQuestions(
					1,
					30,
					"N4",
					model.CategoryVocabulary,
				),
				model.Question{ID: 50, ProficiencyLevel: "N4", Category: model.CategoryReading},
				model.Question{ID: 51, ProficiencyLevel: "N4", Category: model.CategoryListening},
			), nil, true, true},
		{"upper due cannot take N3 slots", "N3", makeQuestions(
			1,
			30,
			"N2",
			model.CategoryVocabulary,
		),
			makeQuestions(
				101,
				30,
				"N3",
				model.CategoryGrammar,
			), false, false},
	} {
		for _, profile := range []struct {
			name  string
			total int
			floor int
		}{{"morning", 17, 14}, {"evening", 17, 14}} {
			t.Run(
				tt.name+"/"+profile.name,
				func(t *testing.T) {
					pool := &builderPool{due: tt.due, new: tt.new}
					builder, stores := newTestBuilder(pool)
					build := builder.BuildMorningSession
					if profile.name == "evening" {
						build = builder.BuildEveningSession
					}
					session, err := build(
						context.Background(),
						42,
						"ja",
						tt.level,
					)
					if err != nil || session == nil || session.TotalQuestions != profile.total ||
						len(stores.rows) != profile.total {
						t.Fatalf(
							"session = %+v, selected = %d, error = %v",
							session,
							len(stores.rows),
							err,
						)
					}
					// Every query stays bounded by the session size.
					for _, query := range pool.dueQueries {
						if query.limit > profile.total+2 {
							t.Fatalf(
								"unbounded due query: %d",
								query.limit,
							)
						}
					}
					for _, query := range pool.newQueries {
						if query.limit > profile.total {
							t.Fatalf(
								"unbounded new query: %d",
								query.limit,
							)
						}
					}

					byID := make(map[int]model.Question)
					for _, q := range append(
						slices.Clone(tt.due),
						tt.new...,
					) {
						byID[q.ID] = q
					}
					current, reading, audio := 0, 0, 0
					seen := make(map[int]bool)
					for _, row := range stores.rows {
						q := byID[row.QuestionID]
						if seen[q.ID] {
							t.Fatalf(
								"duplicate question %d",
								q.ID,
							)
						}
						seen[q.ID] = true
						if q.ProficiencyLevel == tt.level {
							current++
						} else if !row.IsReview || !isLowerAdjacentQuestion(
							q,
							"ja",
							tt.level,
						) {
							t.Fatalf(
								"non-current question admitted despite sufficient current supply: %+v",
								q,
							)
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
						t.Fatalf(
							"current/reading/listening = %d/%d/%d",
							current,
							reading,
							audio,
						)
					}
				},
			)
		}
	}
}
