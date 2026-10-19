package model

// KanjiNoteType classifies how a kanji was formed (ADR-073).
type KanjiNoteType string

const (
	KanjiNotePhonoSemantic KanjiNoteType = "phono_semantic" // 形声
	KanjiNoteCompound      KanjiNoteType = "compound"       // 会意
	KanjiNoteIndicative    KanjiNoteType = "indicative"     // 指事
	KanjiNotePictograph    KanjiNoteType = "pictograph"     // 象形
)

func (t KanjiNoteType) Valid() bool {
	switch t {
	case KanjiNotePhonoSemantic, KanjiNoteCompound, KanjiNoteIndicative, KanjiNotePictograph:
		return true
	default:
		return false
	}
}

// KanjiPartRole says what a visible component contributes to its kanji.
type KanjiPartRole string

const (
	KanjiPartMeaning KanjiPartRole = "meaning"
	KanjiPartSound   KanjiPartRole = "sound"
	// KanjiPartForm marks a visible shape whose original role was lost by the
	// shinjitai simplification, such as 売 in 読.
	KanjiPartForm KanjiPartRole = "form"
)

func (r KanjiPartRole) Valid() bool {
	switch r {
	case KanjiPartMeaning, KanjiPartSound, KanjiPartForm:
		return true
	default:
		return false
	}
}

// KanjiNote is one kanji's structure note in a vocabulary payload's optional
// "kanji_notes" list. Every word that uses the kanji carries the same note.
type KanjiNote struct {
	Char          string        `json:"char"`
	Type          KanjiNoteType `json:"type"`
	Parts         []KanjiPart   `json:"parts"`
	ExplanationKo string        `json:"explanation_ko"`
}

// KanjiPart is one visible component. GlossKo explains meaning and form
// parts; On is the on'yomi a sound part shares with the kanji, empty when the
// Japanese readings diverged.
type KanjiPart struct {
	Part    string        `json:"part"`
	Role    KanjiPartRole `json:"role"`
	GlossKo string        `json:"gloss_ko,omitempty"`
	On      string        `json:"on,omitempty"`
}

// KanjiReadingType says how a word reads one of its kanji (ADR-074).
type KanjiReadingType string

const (
	KanjiReadingOn  KanjiReadingType = "on"  // 音読み
	KanjiReadingKun KanjiReadingType = "kun" // 訓読み
	// KanjiReadingJukuji is a reading that belongs to a kanji group as a
	// whole, such as 今日 きょう, so no single kanji owns a part of it.
	KanjiReadingJukuji KanjiReadingType = "jukuji"
)

func (t KanjiReadingType) Valid() bool {
	switch t {
	case KanjiReadingOn, KanjiReadingKun, KanjiReadingJukuji:
		return true
	default:
		return false
	}
}

// KanjiReading is one entry of a vocabulary payload's optional
// "kanji_readings" list, in word order: Text is one kanji, or the whole group
// for jukuji, and Reading is the hiragana it takes in this word. On lists the
// kanji's on'yomi used in common words, most used first; like a note it is
// the same in every word, and a jukuji group has none.
type KanjiReading struct {
	Text    string           `json:"text"`
	Reading string           `json:"reading"`
	Type    KanjiReadingType `json:"type"`
	On      []string         `json:"on,omitempty"`
}
