package model

type KanjiNoteType string

const (
	KanjiNotePhonoSemantic KanjiNoteType = "phono_semantic" // 形声
	KanjiNoteCompound      KanjiNoteType = "compound"       // 会意
	KanjiNoteIndicative    KanjiNoteType = "indicative"     // 指事
	KanjiNotePictograph    KanjiNoteType = "pictograph"     // 象形
)

func (noteType KanjiNoteType) Valid() bool {
	switch noteType {
	case KanjiNotePhonoSemantic, KanjiNoteCompound, KanjiNoteIndicative, KanjiNotePictograph:
		return true
	default:
		return false
	}
}

type KanjiPartRole string

const (
	KanjiPartMeaning KanjiPartRole = "meaning"
	KanjiPartSound   KanjiPartRole = "sound"
	// KanjiPartForm marks a visible shape whose original role was lost to shinjitai simplification, such as 売 in 読.
	KanjiPartForm KanjiPartRole = "form"
)

func (role KanjiPartRole) Valid() bool {
	switch role {
	case KanjiPartMeaning, KanjiPartSound, KanjiPartForm:
		return true
	default:
		return false
	}
}

// Every word using the kanji carries the same note; ExplanationKo is empty when the link needs its history (ADR-080).
type KanjiNote struct {
	Character     string        `json:"char"`
	Type          KanjiNoteType `json:"type"`
	Parts         []KanjiPart   `json:"parts"`
	ExplanationKo string        `json:"explanation_ko"`
}

// KanjiPart is a visible component. GlossKo is what a meaning or form part means in this kanji; Base is the kanji a
// radical variant stands for (手 for 扌, ADR-080). A sound part has no gloss: HunEum is its own 훈음, On the
// on'yomi it shares with the kanji, empty when the Japanese readings diverged.
type KanjiPart struct {
	Part    string        `json:"part"`
	Role    KanjiPartRole `json:"role"`
	GlossKo string        `json:"gloss_ko,omitempty"`
	Base    string        `json:"base,omitempty"`
	HunEum  string        `json:"hun_eum,omitempty"`
	On      string        `json:"on,omitempty"`
}

type KanjiReadingType string

const (
	KanjiReadingOn  KanjiReadingType = "on"  // 音読み
	KanjiReadingKun KanjiReadingType = "kun" // 訓読み
	// KanjiReadingJukuji is a reading owned by a kanji group as a whole, such as 今日 きょう, not a single kanji.
	KanjiReadingJukuji KanjiReadingType = "jukuji"
)

func (readingType KanjiReadingType) Valid() bool {
	switch readingType {
	case KanjiReadingOn, KanjiReadingKun, KanjiReadingJukuji:
		return true
	default:
		return false
	}
}

// KanjiReading is one entry of a vocabulary payload's optional "kanji_readings" list, in word order: Text is one kanji
// or the whole jukuji group, Reading the hiragana it takes here. On (common on'yomi, most used first) and HunEum
// (Korean 훈음 such as 펼 신, ADR-076) are the same in every word; a jukuji group has no On and joins its kanji's
// HunEum, as 今 이제 금 · 日 날 일.
type KanjiReading struct {
	Text    string           `json:"text"`
	Reading string           `json:"reading"`
	Type    KanjiReadingType `json:"type"`
	On      []string         `json:"on,omitempty"`
	HunEum  string           `json:"hun_eum,omitempty"`
}
