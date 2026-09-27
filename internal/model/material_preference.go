package model

import "time"

// MaterialReviewMode is a user's explicit learning preference, not evidence of mastery.
type MaterialReviewMode string

const (
	MaterialReviewNormal      MaterialReviewMode = "normal"
	MaterialReviewMaintenance MaterialReviewMode = "maintenance"
	MaterialReviewExcluded    MaterialReviewMode = "excluded"

	InitialMaintenanceDays = 30
	MaxMaintenanceDays     = 180
)

func (m MaterialReviewMode) Valid() bool {
	return m == MaterialReviewNormal || m == MaterialReviewMaintenance || m == MaterialReviewExcluded
}

// MaterialPreference is stored separately from observed Study/Quiz progress.
// Normal mode is represented by an absent row; NextCheckAt gates new sessions only.
type MaterialPreference struct {
	UserID            int64              `db:"user_id"             json:"user_id"`
	MaterialID        int                `db:"material_id"         json:"material_id"`
	ReviewMode        MaterialReviewMode `db:"review_mode"         json:"review_mode"`
	NextCheckAt       *time.Time         `db:"next_check_at"       json:"next_check_at"`
	CheckIntervalDays int                `db:"check_interval_days" json:"check_interval_days"`
	CreatedAt         time.Time          `db:"created_at"          json:"created_at"`
	UpdatedAt         time.Time          `db:"updated_at"          json:"updated_at"`
	MaterialTitle     string             `db:"material_title"      json:"material_title"`
}
