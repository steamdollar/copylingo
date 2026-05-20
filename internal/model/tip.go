package model

import "time"

// TipCategory is a code-level whitelist for tips.category.
// DB stores VARCHAR; validity is enforced in Go.
type TipCategory string

const (
	// Japanese kana handwriting categories.
	TipCategoryKanaYoon     TipCategory = "kana_youon"        // 요음 (small ya/yu/yo)
	TipCategoryKanaSokuon   TipCategory = "kana_sokuon"       // 촉음 (small tsu)
	TipCategoryKanaDakuten  TipCategory = "kana_dakuten"      // 탁점/반탁점
	TipCategoryKanaChouon   TipCategory = "kana_chouon"       // 장음
	TipCategoryKanaShape    TipCategory = "kana_shape"        // 비슷한 모양 구분 (シ vs ツ)
	TipCategoryKanaStroke   TipCategory = "kana_stroke"       // 획순/필기 기본
	TipCategoryKanaHiraKata TipCategory = "kana_hira_vs_kata" // 히라가나 vs 가타카나
)

// tipCategoryDisplay maps the internal enum key to the user-facing eyebrow
// label shown on tip cards. Update here when adding/renaming a category.
var tipCategoryDisplay = map[TipCategory]string{
	TipCategoryKanaYoon:     "요음",
	TipCategoryKanaSokuon:   "촉음",
	TipCategoryKanaDakuten:  "탁점/반탁점",
	TipCategoryKanaChouon:   "장음",
	TipCategoryKanaShape:    "비슷한 모양",
	TipCategoryKanaStroke:   "획순",
	TipCategoryKanaHiraKata: "히라가나/가타카나",
}

// DisplayName returns the user-facing eyebrow label for the tip card.
// Falls back to the raw enum string if the category is unknown (drift guard).
func (c TipCategory) DisplayName() string {
	if name, ok := tipCategoryDisplay[c]; ok {
		return name
	}
	return string(c)
}

// AllTipCategories returns every whitelisted category — used by the tip
// generation pipeline to iterate when filling buckets.
func AllTipCategories() []TipCategory {
	out := make([]TipCategory, 0, len(tipCategoryDisplay))
	for c := range tipCategoryDisplay {
		out = append(out, c)
	}
	return out
}

type Tip struct {
	ID               int         `db:"id" json:"id"`
	Language         string      `db:"language" json:"language"`
	ProficiencyLevel string      `db:"proficiency_level" json:"proficiency_level"`
	Category         TipCategory `db:"category" json:"category"`
	Body             string      `db:"body" json:"body"`

	SourceModel     *string `db:"source_model" json:"-"`
	SourcePromptVer *string `db:"source_prompt_ver" json:"-"`
	IsActive        bool    `db:"is_active" json:"-"`

	CreatedAt time.Time `db:"created_at" json:"-"`
}
