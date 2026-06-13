package model

import (
	"encoding/json"
	"time"
)

// MaterialCategory defines the kind of concept shown in a study session.
type MaterialCategory string

const (
	MaterialCategoryKana       MaterialCategory = "kana"
	MaterialCategoryVocabulary MaterialCategory = "vocabulary"
	MaterialCategoryGrammar    MaterialCategory = "grammar"
	MaterialCategorySentence   MaterialCategory = "sentence"
)

// Material represents a single concept that a user can study.
type Material struct {
	ID               int              `db:"id"                json:"id"`
	MaterialKey      string           `db:"material_key"      json:"material_key"`
	ContentID        *int             `db:"content_id"        json:"content_id"`
	Category         MaterialCategory `db:"category"          json:"category"`
	Language         string           `db:"language"          json:"language"`
	ProficiencyLevel string           `db:"proficiency_level" json:"proficiency_level"`
	Title            string           `db:"title"             json:"title"`
	Payload          json.RawMessage  `db:"payload"           json:"payload"`
	Difficulty       int              `db:"difficulty"        json:"difficulty"`
	CreatedAt        time.Time        `db:"created_at"        json:"created_at"`
}

// UserMaterialProgress stores user-specific spaced repetition state for a material.
type UserMaterialProgress struct {
	UserID        int64      `db:"user_id"         json:"user_id"`
	MaterialID    int        `db:"material_id"     json:"material_id"`
	EaseFactor    float64    `db:"ease_factor"     json:"ease_factor"`
	IntervalDays  int        `db:"interval_days"   json:"interval_days"`
	Repetitions   int        `db:"repetitions"     json:"repetitions"`
	NextReviewAt  *time.Time `db:"next_review_at"  json:"next_review_at"`
	LastStudiedAt *time.Time `db:"last_studied_at" json:"last_studied_at"`
	TimesStudied  int        `db:"times_studied"   json:"times_studied"`
	CreatedAt     time.Time  `db:"created_at"      json:"created_at"`
	UpdatedAt     time.Time  `db:"updated_at"      json:"updated_at"`
}
