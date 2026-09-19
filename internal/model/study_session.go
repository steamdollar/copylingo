package model

// StudyMaterialQuota describes how many new and review materials to request
// for one material category in a study session.
type StudyMaterialQuota struct {
	Category    MaterialCategory `json:"category"`
	NewCount    int              `json:"new_count"`
	ReviewCount int              `json:"review_count"`
}

// StudySessionPlan contains the per-category material quotas for a study
// session. The repository may return fewer materials when eligible candidates
// are unavailable. Fallback due reviews may shift between categories, then new
// vocabulary fills remaining slots. Total count, new grammar/reading quotas,
// and reading caps remain bounded by the plan.
type StudySessionPlan struct {
	Quotas []StudyMaterialQuota `json:"quotas"`
}

// TotalMaterialCount returns the total number of requested materials.
func (p StudySessionPlan) TotalMaterialCount() int {
	total := 0
	for _, quota := range p.Quotas {
		total += quota.NewCount + quota.ReviewCount
	}
	return total
}
