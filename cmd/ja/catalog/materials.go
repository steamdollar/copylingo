package catalog

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

type GrammarMaterialPayload struct {
	Pattern        string `json:"pattern"`
	MeaningKo      string `json:"meaning_ko"`
	ExplanationKo  string `json:"explanation_ko"`
	Example        string `json:"example"`
	ExampleReading string `json:"example_reading"`
	TranslationKo  string `json:"translation_ko"`
}

func BuildAllMaterials() []*model.Material {
	kanaMaterials := BuildKanaMaterials(KanaMap)
	vocabMaterials := BuildVocabularyMaterials(N5Words)
	grammarMaterials := BuildGrammarMaterials(N5GrammarPoints)

	materials := make([]*model.Material, 0, len(kanaMaterials)+len(vocabMaterials)+len(grammarMaterials))
	materials = append(materials, kanaMaterials...)
	materials = append(materials, vocabMaterials...)
	materials = append(materials, grammarMaterials...)
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

func BuildGrammarMaterials(points []GrammarPoint) []*model.Material {
	materials := make([]*model.Material, 0, len(points))
	for _, point := range points {
		materials = append(materials, &model.Material{
			MaterialKey:      MaterialKeyForGrammar(point),
			Category:         model.MaterialCategoryGrammar,
			Language:         VocabLanguage,
			ProficiencyLevel: VocabProficiencyLevel,
			Title:            point.Pattern,
			Payload: mustMaterialJSON(GrammarMaterialPayload{
				Pattern:        point.Pattern,
				MeaningKo:      point.MeaningKo,
				ExplanationKo:  point.ExplanationKo,
				Example:        point.Example,
				ExampleReading: point.ExampleReading,
				TranslationKo:  point.TranslationKo,
			}),
			Difficulty: GrammarDifficulty,
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

// Material keys embed the dataset ID verbatim so the proficiency level stays
// part of the key; trimming a level prefix here would collide IDs across levels.
func MaterialKeyForVocab(word VocabWord) string {
	return "ja:vocab:" + word.ID
}

func MaterialKeyForGrammar(point GrammarPoint) string {
	return "ja:grammar:" + point.ID
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
