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
	ID               int              `db:"id" json:"id"`
	MaterialKey      string           `db:"material_key" json:"material_key"`
	ContentID        *int             `db:"content_id" json:"content_id"`
	Category         MaterialCategory `db:"category" json:"category"`
	Language         string           `db:"language" json:"language"`
	ProficiencyLevel string           `db:"proficiency_level" json:"proficiency_level"`
	Title            string           `db:"title" json:"title"`
	Payload          json.RawMessage  `db:"payload" json:"payload"`
	Difficulty       int              `db:"difficulty" json:"difficulty"`
	CreatedAt        time.Time        `db:"created_at" json:"created_at"`
}
