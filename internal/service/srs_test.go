package service

import (
	"testing"

	"github.com/lsj/copylingo/internal/model"
)

func TestScheduleAnswer(t *testing.T) {
	tests := []struct {
		name              string
		initialRepetition int
		initialInt        int
		initialEase       float64
		isCorrect         bool
		expectRepetition  int
		expectInt         int
		expectEaseGte     float64
	}{
		{
			name:             "CorrectFirstRepetition",
			initialEase:      2.5,
			isCorrect:        true,
			expectRepetition: 1,
			expectInt:        3,
			expectEaseGte:    1.3,
		},
		{
			name:              "CorrectSecondRepetition",
			initialRepetition: 1,
			initialInt:        1,
			initialEase:       2.5,
			isCorrect:         true,
			expectRepetition:  2,
			expectInt:         6,
			expectEaseGte:     1.3,
		},
		{
			name:              "CorrectSubsequentUsesFactor",
			initialRepetition: 2,
			initialInt:        6,
			initialEase:       2.5,
			isCorrect:         true,
			expectRepetition:  3,
			expectInt:         15,
			expectEaseGte:     1.3,
		},
		{
			name:              "WrongResetsRepetitions",
			initialRepetition: 3,
			initialInt:        20,
			initialEase:       2.5,
			expectRepetition:  0,
			expectInt:         1,
			expectEaseGte:     1.3,
		},
	}

	for _, tt := range tests {
		t.Run(
			tt.name,
			func(t *testing.T) {
				progress := &model.UserQuestionProgress{
					Repetitions:  tt.initialRepetition,
					IntervalDays: tt.initialInt,
					EaseFactor:   tt.initialEase,
				}
				scheduleAnswer(
					progress,
					tt.isCorrect,
				)

				if progress.Repetitions != tt.expectRepetition {
					t.Errorf(
						"Repetitions = %d, want %d",
						progress.Repetitions,
						tt.expectRepetition,
					)
				}
				if progress.IntervalDays != tt.expectInt {
					t.Errorf(
						"IntervalDays = %d, want %d",
						progress.IntervalDays,
						tt.expectInt,
					)
				}
				if progress.EaseFactor < tt.expectEaseGte {
					t.Errorf(
						"EaseFactor = %f, want >= %f",
						progress.EaseFactor,
						tt.expectEaseGte,
					)
				}
				if progress.NextReviewAt == nil || progress.LastReviewedAt == nil {
					t.Fatal("expected review timestamps")
				}
			},
		)
	}
}

func TestEaseFactorFloorAt1_3(t *testing.T) {
	progress := &model.UserQuestionProgress{Repetitions: 1, IntervalDays: 1, EaseFactor: 1.3}
	for range 5 {
		scheduleAnswer(
			progress,
			false,
		)
		if progress.EaseFactor < 1.3 {
			t.Fatalf(
				"EaseFactor dropped below 1.3: %f",
				progress.EaseFactor,
			)
		}
	}
}
