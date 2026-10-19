package service

import (
	"context"

	"github.com/lsj/copylingo/internal/model"
)

// KanjiSeriesWordsPerPart caps the studied words listed per sound part.
const KanjiSeriesWordsPerPart = 3

type kanjiSeriesRepo interface {
	ListStudiedSoundSeriesWords(
		ctx context.Context,
		userID int64,
		language string,
		excludeMaterialID int,
		soundParts []string,
		excludeChars []string,
		perPart int,
	) ([]model.StudiedSeriesWord, error)
}

// KanjiNoteService links a card's kanji notes to the words the user already
// studied: a sound part seen today (青 in 清) is shown next to earlier words
// whose kanji share it (晴れ, 精神), so the series is noticed (ADR-073).
type KanjiNoteService struct {
	repo kanjiSeriesRepo
}

func NewKanjiNoteService(repo kanjiSeriesRepo) *KanjiNoteService {
	return &KanjiNoteService{repo: repo}
}

// StudiedSeriesWords returns, per sound part of notes, the user's studied
// words that use the same sound part in another kanji. Notes without a sound
// part need no lookup.
func (s *KanjiNoteService) StudiedSeriesWords(
	ctx context.Context,
	userID int64,
	material model.Material,
	notes []model.KanjiNote,
) ([]model.StudiedSeriesWord, error) {
	soundParts := make(
		[]string,
		0,
		len(notes),
	)
	chars := make(
		[]string,
		0,
		len(notes),
	)
	for _, note := range notes {
		chars = append(
			chars,
			note.Char,
		)
		if part, ok := note.SoundPart(); ok {
			soundParts = append(
				soundParts,
				part.Part,
			)
		}
	}
	if len(soundParts) == 0 {
		return nil, nil
	}
	return s.repo.ListStudiedSoundSeriesWords(
		ctx,
		userID,
		material.Language,
		material.ID,
		soundParts,
		chars,
		KanjiSeriesWordsPerPart,
	)
}
