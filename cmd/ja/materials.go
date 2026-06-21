package ja

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/lsj/copylingo/internal/model"
)

type KanaMaterialPayload struct {
	Kana   string `json:"kana"`
	Romaji string `json:"romaji"`
	Script string `json:"script"`
}

type VocabularyMaterialPayload struct {
	Kana         string `json:"kana"`
	Kanji        string `json:"kanji"`
	MeaningKo    string `json:"meaning_ko"`
	PartOfSpeech string `json:"part_of_speech"`
}

func BuildAllMaterials() []*model.Material {
	kanaMaterials := BuildKanaMaterials(KanaMap)
	vocabMaterials := BuildVocabularyMaterials(N5Words)

	materials := make([]*model.Material, 0, len(kanaMaterials)+len(vocabMaterials))
	materials = append(materials, kanaMaterials...)
	materials = append(materials, vocabMaterials...)
	return materials
}

func BuildKanaMaterials(kanaMap map[string]string) []*model.Material {
	materials := make([]*model.Material, 0, len(kanaMap))
	for kana, romaji := range kanaMap {
		materials = append(materials, &model.Material{
			MaterialKey:      MaterialKeyForKana(kana),
			Category:         model.MaterialCategoryKana,
			Language:         VocabLanguage,
			ProficiencyLevel: VocabProficiencyLevel,
			Title:            kana,
			Payload: mustMaterialJSON(KanaMaterialPayload{
				Kana:   kana,
				Romaji: romaji,
				Script: ScriptLabel(kana),
			}),
			Difficulty: 1,
		})
	}
	return materials
}

func BuildVocabularyMaterials(words []VocabWord) []*model.Material {
	materials := make([]*model.Material, 0, len(words))
	for _, word := range words {
		materials = append(materials, &model.Material{
			MaterialKey:      MaterialKeyForVocab(word),
			Category:         model.MaterialCategoryVocabulary,
			Language:         VocabLanguage,
			ProficiencyLevel: VocabProficiencyLevel,
			Title:            word.Kana,
			Payload: mustMaterialJSON(VocabularyMaterialPayload{
				Kana:         word.Kana,
				Kanji:        word.Kanji,
				MeaningKo:    word.MeaningKo,
				PartOfSpeech: word.PartOfSpeech,
			}),
			Difficulty: VocabDifficulty,
		})
	}
	return materials
}

func MaterialKeyForKana(kana string) string {
	parts := make([]string, 0, len([]rune(kana)))
	for _, r := range kana {
		parts = append(parts, fmt.Sprintf("u%04x", r))
	}
	return "ja:kana:" + strings.Join(parts, "_")
}

func MaterialKeyForVocab(word VocabWord) string {
	return "ja:vocab:" + strings.TrimPrefix(word.ID, "n5_")
}

func ScriptLabel(kana string) string {
	for _, r := range kana {
		switch {
		case r >= 'ぁ' && r <= 'ゖ':
			return "히라가나"
		case r >= 'ァ' && r <= 'ヺ':
			return "가타카나"
		}
	}

	return "가나"
}

func mustMaterialJSON(value any) json.RawMessage {
	payload, err := json.Marshal(value)
	if err != nil {
		panic(fmt.Sprintf("marshal material payload: %v", err))
	}
	return payload
}
