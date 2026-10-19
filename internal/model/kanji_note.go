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
	Char          string             `json:"char"`
	Type          KanjiNoteType      `json:"type"`
	Parts         []KanjiPart        `json:"parts"`
	ExplanationKo string             `json:"explanation_ko"`
	Series        []KanjiSeriesKanji `json:"series,omitempty"`
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

// KanjiSeriesKanji is another kanji with the same sound part and on'yomi.
type KanjiSeriesKanji struct {
	Char string `json:"char"`
	On   string `json:"on"`
}

// SoundPart returns the note's sound component, if it has one.
func (n KanjiNote) SoundPart() (KanjiPart, bool) {
	for _, part := range n.Parts {
		if part.Role == KanjiPartSound {
			return part, true
		}
	}
	return KanjiPart{}, false
}

// StudiedSeriesWord is a word the user has studied whose kanji shares
// SoundPart with a kanji on the current card.
type StudiedSeriesWord struct {
	SoundPart string `db:"sound_part"`
	Word      string `db:"word"`
}
