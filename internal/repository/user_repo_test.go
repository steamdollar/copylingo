package repository

import (
	"testing"

	"github.com/lsj/copylingo/internal/model"
)

func TestSlotColumn_MapsCorrectly(t *testing.T) {
	tests := []struct {
		slot model.SessionSlot
		want string
	}{
		{model.SessionSlotMorningStudy, "morning_study_time"},
		{model.SessionSlotMorningQuiz, "morning_quiz_time"},
		{model.SessionSlotEveningStudy, "evening_study_time"},
		{model.SessionSlotEveningQuiz, "evening_quiz_time"},
	}

	for _, tc := range tests {
		got, err := slotColumn(tc.slot)
		if err != nil {
			t.Fatalf("slotColumn(%q) error: %v", tc.slot, err)
		}
		if got != tc.want {
			t.Errorf("slotColumn(%q) = %q, want %q", tc.slot, got, tc.want)
		}
	}

	// Invalid slot should error
	if _, err := slotColumn("unknown_slot"); err == nil {
		t.Error("expected error for unknown_slot, got nil")
	}
}
