package model

import "testing"

func TestStudySessionPlanTotalMaterialCount(t *testing.T) {
	plan := StudySessionPlan{Quotas: []StudyMaterialQuota{
		{Category: MaterialCategoryVocabulary, NewCount: 8, ReviewCount: 7},
		{Category: MaterialCategoryGrammar, NewCount: 1, ReviewCount: 3},
		{Category: MaterialCategoryReading, NewCount: 1},
	}}
	if got := plan.TotalMaterialCount(); got != 20 {
		t.Fatalf("TotalMaterialCount() = %d, want 20", got)
	}
}
