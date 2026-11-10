package model

import "time"

// TipCandidate keeps its pre-ADR-075 name to match the tip_candidates table.
type TipCandidate struct {
	ID               int       `db:"id"                json:"id"`
	UserID           int64     `db:"user_id"           json:"user_id"`
	Username         string    `db:"username"          json:"username"`
	Language         string    `db:"language"          json:"language"`
	ProficiencyLevel string    `db:"proficiency_level" json:"proficiency_level"`
	Question         string    `db:"question"          json:"question"`
	Answer           string    `db:"answer"            json:"answer"`
	SourceModel      *string   `db:"source_model"      json:"-"`
	CreatedAt        time.Time `db:"created_at"        json:"-"`
}
