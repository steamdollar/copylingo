package model

import "time"

// ContentSourceType defines the origin of learning content.
type ContentSourceType string

const (
	ContentSourceNews     ContentSourceType = "news"
	ContentSourceExamPrep ContentSourceType = "exam_prep"
)

// Content represents collected learning material from external sources.
type Content struct {
	ID               int               `db:"id" json:"id"`
	SourceType       ContentSourceType `db:"source_type" json:"source_type"`
	SourceURL        string            `db:"source_url" json:"source_url"`
	Title            string            `db:"title" json:"title"`
	Body             string            `db:"body" json:"body"`
	Language         string            `db:"language" json:"language"`                   // ISO 639-1: 'ja', 'el', 'en'
	ProficiencyLevel string            `db:"proficiency_level" json:"proficiency_level"` // JLPT: N5-N1, CEFR: A1-C2
	Difficulty       int               `db:"difficulty" json:"difficulty"`
	Tags             []string          `db:"tags" json:"tags"`
	IsArticle        bool              `db:"is_article" json:"is_article"`
	CollectedAt      time.Time         `db:"collected_at" json:"collected_at"`
}
