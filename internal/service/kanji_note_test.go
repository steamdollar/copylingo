package service

import (
	"context"
	"slices"
	"testing"

	"github.com/lsj/copylingo/internal/model"
)

type fakeKanjiSeriesRepo struct {
	calls      int
	userID     int64
	language   string
	materialID int
	soundParts []string
	chars      []string
	perPart    int
}

func (r *fakeKanjiSeriesRepo) ListStudiedSoundSeriesWords(
	ctx context.Context,
	userID int64,
	language string,
	excludeMaterialID int,
	soundParts []string,
	excludeChars []string,
	perPart int,
) ([]model.StudiedSeriesWord, error) {
	r.calls++
	r.userID = userID
	r.language = language
	r.materialID = excludeMaterialID
	r.soundParts = soundParts
	r.chars = excludeChars
	r.perPart = perPart
	return []model.StudiedSeriesWord{{SoundPart: "青", Word: "晴れ"}}, nil
}

func TestKanjiNoteServiceStudiedSeriesWords(t *testing.T) {
	t.Parallel()

	material := model.Material{ID: 10, Language: "ja"}
	sound := model.KanjiNote{
		Char: "清",
		Type: model.KanjiNotePhonoSemantic,
		Parts: []model.KanjiPart{
			{Part: "氵", Role: model.KanjiPartMeaning, GlossKo: "물"},
			{Part: "青", Role: model.KanjiPartSound, On: "セイ"},
		},
	}
	compound := model.KanjiNote{
		Char: "休",
		Type: model.KanjiNoteCompound,
		Parts: []model.KanjiPart{
			{Part: "亻", Role: model.KanjiPartMeaning, GlossKo: "사람"},
			{Part: "木", Role: model.KanjiPartMeaning, GlossKo: "나무"},
		},
	}

	t.Run(
		"sound parts are looked up excluding the card's kanji",
		func(t *testing.T) {
			repo := &fakeKanjiSeriesRepo{}
			words, err := NewKanjiNoteService(repo).StudiedSeriesWords(
				context.Background(),
				42,
				material,
				[]model.KanjiNote{sound, compound},
			)
			if err != nil || len(words) != 1 {
				t.Fatalf(
					"words = %+v, err = %v",
					words,
					err,
				)
			}
			if repo.userID != 42 || repo.language != "ja" || repo.materialID != 10 ||
				!slices.Equal(
					repo.soundParts,
					[]string{"青"},
				) ||
				!slices.Equal(
					repo.chars,
					[]string{"清", "休"},
				) ||
				repo.perPart != KanjiSeriesWordsPerPart {
				t.Errorf(
					"repo call = %+v",
					repo,
				)
			}
		},
	)
	t.Run(
		"notes without a sound part skip the lookup",
		func(t *testing.T) {
			repo := &fakeKanjiSeriesRepo{}
			words, err := NewKanjiNoteService(repo).StudiedSeriesWords(
				context.Background(),
				42,
				material,
				[]model.KanjiNote{compound},
			)
			if err != nil || words != nil || repo.calls != 0 {
				t.Errorf(
					"words = %+v, err = %v, calls = %d",
					words,
					err,
					repo.calls,
				)
			}
		},
	)
}
