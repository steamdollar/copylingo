package main

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/lsj/copylingo/internal/model"
)

type vocabularyMaterialPayload struct {
	Kana         string `json:"kana"`
	Kanji        string `json:"kanji"`
	MeaningKo    string `json:"meaning_ko"`
	PartOfSpeech string `json:"part_of_speech"`
}

func buildVocabularyMaterials(words []vocabWord) []*model.Material {
	materials := make([]*model.Material, 0, len(words))
	for _, word := range words {
		materials = append(materials, &model.Material{
			MaterialKey:      "ja:vocab:" + strings.TrimPrefix(word.ID, "n5_"),
			Category:         model.MaterialCategoryVocabulary,
			Language:         vocabLanguage,
			ProficiencyLevel: vocabProficiencyLevel,
			Title:            word.Kana,
			Payload: mustMaterialJSON(vocabularyMaterialPayload{
				Kana:         word.Kana,
				Kanji:        word.Kanji,
				MeaningKo:    word.MeaningKo,
				PartOfSpeech: word.PartOfSpeech,
			}),
			Difficulty: vocabDifficulty,
		})
	}
	return materials
}

func mustMaterialJSON(value any) json.RawMessage {
	payload, err := json.Marshal(value)
	if err != nil {
		panic(fmt.Sprintf("marshal material payload: %v", err))
	}
	return payload
}
