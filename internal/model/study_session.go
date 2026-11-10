package model

type StudyMaterialQuota struct {
	Category    MaterialCategory `json:"category"`
	NewCount    int              `json:"new_count"`
	ReviewCount int              `json:"review_count"`
}

// May yield fewer materials; fallback due reviews can shift categories, then new vocabulary fills the rest.
type StudySessionPlan struct {
	Quotas []StudyMaterialQuota `json:"quotas"`
}

func (plan StudySessionPlan) TotalMaterialCount() int {
	total := 0
	for _, quota := range plan.Quotas {
		total += quota.NewCount + quota.ReviewCount
	}
	return total
}
