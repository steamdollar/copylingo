package catalog

import (
	"encoding/json"
	"fmt"
	"sort"
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

type ReadingMaterialPayload struct {
	Passage       string              `json:"passage"`
	Reading       string              `json:"reading"`
	KeyVocabulary []ReadingVocabulary `json:"key_vocabulary"`
}

func BuildAllMaterials() []*model.Material {
	return BuildAllMaterialsForLevels(DefaultProficiencyLevel())
}

// BuildAllMaterialsForLevels builds the shared material catalog for the
// requested levels. The registry default remains the single-level compatibility
// callers retain their exact catalog scope.
func BuildAllMaterialsForLevels(levels ...string) []*model.Material {
	if len(levels) == 0 {
		levels = []string{DefaultProficiencyLevel()}
	}

	kanaMaterials := BuildKanaMaterials(KanaMap)
	vocabMaterials := make([]*model.Material, 0)
	grammarMaterials := make([]*model.Material, 0)
	readingMaterials := make([]*model.Material, 0)
	seenLevels := make(map[string]struct{}, len(levels))
	for _, level := range levels {
		level = strings.ToUpper(strings.TrimSpace(level))
		if _, seen := seenLevels[level]; seen {
			continue
		}
		seenLevels[level] = struct{}{}
		catalog, ok := LevelCatalogFor(level)
		if !ok {
			continue
		}
		vocabMaterials = append(vocabMaterials, BuildVocabularyMaterialsForLevel(level, catalog.Words)...)
		grammarMaterials = append(grammarMaterials, BuildGrammarMaterialsForLevel(level, catalog.GrammarPoints)...)
		readingMaterials = append(readingMaterials, BuildReadingMaterialsForLevel(level, catalog.ReadingPassages)...)
	}

	materials := make(
		[]*model.Material,
		0,
		len(kanaMaterials)+len(vocabMaterials)+len(grammarMaterials)+len(readingMaterials),
	)
	materials = append(materials, kanaMaterials...)
	materials = append(materials, vocabMaterials...)
	materials = append(materials, grammarMaterials...)
	materials = append(materials, readingMaterials...)
	return materials
}

func BuildKanaMaterials(kanaMap map[string]string) []*model.Material {
	materials := make([]*model.Material, 0, len(kanaMap))
	kanas := make([]string, 0, len(kanaMap))
	for kana := range kanaMap {
		kanas = append(kanas, kana)
	}
	sort.Strings(kanas)
	for _, kana := range kanas {
		romaji := kanaMap[kana]
		materials = append(materials, &model.Material{
			MaterialKey:      MaterialKeyForKana(kana),
			Category:         model.MaterialCategoryKana,
			Language:         VocabLanguage,
			ProficiencyLevel: DefaultProficiencyLevel(),
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
	return BuildVocabularyMaterialsForLevel(datasetLevel(words, DefaultProficiencyLevel()), words)
}

// BuildVocabularyMaterialsForLevel maps authored vocabulary to materials with
// an explicit proficiency level. The level is part of every material key.
func BuildVocabularyMaterialsForLevel(level string, words []VocabWord) []*model.Material {
	level = strings.ToUpper(strings.TrimSpace(level))
	materials := make([]*model.Material, 0, len(words))
	for _, word := range words {
		materials = append(materials, &model.Material{
			MaterialKey:      MaterialKeyForVocabAtLevel(level, word),
			Category:         model.MaterialCategoryVocabulary,
			Language:         VocabLanguage,
			ProficiencyLevel: level,
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
	return BuildGrammarMaterialsForLevel(datasetLevelGrammar(points, DefaultProficiencyLevel()), points)
}

// BuildGrammarMaterialsForLevel maps authored grammar to materials with an
// explicit proficiency level. The level is part of every material key.
func BuildGrammarMaterialsForLevel(level string, points []GrammarPoint) []*model.Material {
	level = strings.ToUpper(strings.TrimSpace(level))
	materials := make([]*model.Material, 0, len(points))
	for _, point := range points {
		materials = append(materials, &model.Material{
			MaterialKey:      MaterialKeyForGrammarAtLevel(level, point),
			Category:         model.MaterialCategoryGrammar,
			Language:         VocabLanguage,
			ProficiencyLevel: level,
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

// BuildReadingMaterials maps each reading passage to a study material. The
// quiz-only fields (Prompt/Options/CorrectAnswer/Explanation) stay out of the
// payload so the study card never leaks the answer rationale (ADR-036).
func BuildReadingMaterials(passages []ReadingPassage) []*model.Material {
	return BuildReadingMaterialsForLevel(datasetLevelReading(passages, DefaultProficiencyLevel()), passages)
}

// BuildReadingMaterialsForLevel maps authored reading to materials with an
// explicit proficiency level. The level is part of every material key.
func BuildReadingMaterialsForLevel(level string, passages []ReadingPassage) []*model.Material {
	level = strings.ToUpper(strings.TrimSpace(level))
	materials := make([]*model.Material, 0, len(passages))
	for _, passage := range passages {
		materials = append(materials, &model.Material{
			MaterialKey:      MaterialKeyForReadingAtLevel(level, passage),
			Category:         model.MaterialCategoryReading,
			Language:         VocabLanguage,
			ProficiencyLevel: level,
			Title:            passage.Title,
			Payload: mustMaterialJSON(ReadingMaterialPayload{
				Passage:       passage.Passage,
				Reading:       passage.Reading,
				KeyVocabulary: passage.KeyVocabulary,
			}),
			Difficulty: passage.Difficulty,
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
	return MaterialKeyForVocabAtLevel(word.Level, word)
}

func MaterialKeyForGrammar(point GrammarPoint) string {
	return MaterialKeyForGrammarAtLevel(point.Level, point)
}

func MaterialKeyForReading(passage ReadingPassage) string {
	return MaterialKeyForReadingAtLevel(passage.Level, passage)
}

func MaterialKeyForVocabAtLevel(level string, word VocabWord) string {
	return "ja:vocab:" + sourceIDWithLevel(level, word.ID)
}

func MaterialKeyForGrammarAtLevel(level string, point GrammarPoint) string {
	return "ja:grammar:" + sourceIDWithLevel(level, point.ID)
}

func MaterialKeyForReadingAtLevel(level string, passage ReadingPassage) string {
	return "ja:reading:" + sourceIDWithLevel(level, passage.ID)
}

func sourceIDWithLevel(level, sourceID string) string {
	level = strings.ToLower(strings.TrimSpace(level))
	sourceID = strings.TrimSpace(sourceID)
	if level == "" || strings.HasPrefix(strings.ToLower(sourceID), level+"_") {
		return sourceID
	}
	return level + "_" + sourceID
}

func datasetLevel(words []VocabWord, fallback string) string {
	for _, word := range words {
		if strings.TrimSpace(word.Level) != "" {
			return strings.ToUpper(strings.TrimSpace(word.Level))
		}
	}
	return fallback
}

func datasetLevelGrammar(points []GrammarPoint, fallback string) string {
	for _, point := range points {
		if strings.TrimSpace(point.Level) != "" {
			return strings.ToUpper(strings.TrimSpace(point.Level))
		}
	}
	return fallback
}

func datasetLevelReading(passages []ReadingPassage, fallback string) string {
	for _, passage := range passages {
		if strings.TrimSpace(passage.Level) != "" {
			return strings.ToUpper(strings.TrimSpace(passage.Level))
		}
	}
	return fallback
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
