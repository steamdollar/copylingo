package catalog

import (
	"github.com/lsj/copylingo/internal/model"
)

// Material payload shapes per category. The seeder copies payloads verbatim;
// these types document the study-card contract that the catalog tests check.

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

// ReadingVocabulary is one key-vocabulary entry surfaced on a reading study card.
type ReadingVocabulary struct {
	Surface   string `json:"surface"`
	Reading   string `json:"reading"`
	MeaningKo string `json:"meaning_ko"`
}

type ReadingMaterialPayload struct {
	Passage       string              `json:"passage"`
	Reading       string              `json:"reading"`
	KeyVocabulary []ReadingVocabulary `json:"key_vocabulary"`
}

// BuildRecordMaterials maps unified material records to rows. Every
// category takes this one path: the record already carries its key, title,
// and payload, so only language and level are stamped from the registry.
func BuildRecordMaterials(
	language,
	level string,
	records []MaterialRecord,
) []*model.Material {
	materials := make(
		[]*model.Material,
		0,
		len(records),
	)
	for _, record := range records {
		materials = append(
			materials,
			&model.Material{
				MaterialKey:      record.MaterialKey,
				Category:         record.Category,
				Language:         language,
				ProficiencyLevel: level,
				Title:            record.Title,
				Payload:          record.Payload,
				Difficulty:       record.Difficulty,
			},
		)
	}
	return materials
}
