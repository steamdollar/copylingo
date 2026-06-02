package main

import (
	"encoding/json"
	"fmt"
	"testing"

	"github.com/lsj/copylingo/internal/model"
)

func TestBuildVocabularyMaterials(t *testing.T) {
	t.Parallel()

	materials := buildVocabularyMaterials(n5Words)
	if len(materials) != len(n5Words) {
		t.Fatalf("len(materials) = %d, want %d", len(materials), len(n5Words))
	}

	keys := make(map[string]bool, len(materials))
	for _, material := range materials {
		if keys[material.MaterialKey] {
			t.Fatalf("duplicate material key %q", material.MaterialKey)
		}
		keys[material.MaterialKey] = true
		if material.Category != model.MaterialCategoryVocabulary ||
			material.Language != vocabLanguage ||
			material.ProficiencyLevel != vocabProficiencyLevel ||
			material.Difficulty != vocabDifficulty {
			t.Fatalf("unexpected material metadata: %+v", material)
		}
	}

	if !keys["ja:vocab:word_024"] {
		t.Fatal("missing vocabulary word_024 material key")
	}
}

func TestBuildVocabularyMaterialsPayload(t *testing.T) {
	t.Parallel()

	for _, material := range buildVocabularyMaterials(n5Words) {
		if material.MaterialKey != "ja:vocab:word_024" {
			continue
		}

		var payload vocabularyMaterialPayload
		if err := json.Unmarshal(material.Payload, &payload); err != nil {
			t.Fatalf("Unmarshal: %v", err)
		}
		if payload.Kana != "みず" ||
			payload.Kanji != "水" ||
			payload.MeaningKo != "물" ||
			payload.PartOfSpeech != "noun" {
			t.Fatalf("payload = %+v", payload)
		}
		return
	}
	t.Fatal("missing vocabulary word_024 material")
}

func TestN5WordsIntegrity(t *testing.T) {
	t.Parallel()

	// 1. len(n5Words)==500
	if len(n5Words) != 500 {
		t.Fatalf("len(n5Words) = %d, want 500", len(n5Words))
	}

	encounteredIDs := make(map[string]bool)
	encounteredKanaKanjiMeaning := make(map[string]bool)
	partOfSpeechWhitelist := map[string]bool{
		"noun":        true,
		"verb":        true,
		"adjective":   true,
		"adverb":      true,
		"pronoun":     true,
		"numeral":     true,
		"expression":  true,
		"counter":     true,
		"conjunction": true,
	}

	for i, word := range n5Words {
		// 2. ID가 n5_word_001부터 n5_word_500까지 slice 순서대로 정확히 연속
		expectedID := fmt.Sprintf("n5_word_%03d", i+1)

		if word.ID != expectedID {
			t.Errorf("n5Words[%d].ID = %q, want %q", i, word.ID, expectedID)
		}

		// 3. ID 중복 없음
		if encounteredIDs[word.ID] {
			t.Errorf("duplicate ID found: %q at index %d", word.ID, i)
		}
		encounteredIDs[word.ID] = true

		// 4. Kana/Kanji/MeaningKo/PartOfSpeech 빈 문자열 없음
		if word.Kana == "" {
			t.Errorf("n5Words[%d].Kana is empty", i)
		}
		if word.Kanji == "" {
			t.Errorf("n5Words[%d].Kanji is empty", i)
		}
		if word.MeaningKo == "" {
			t.Errorf("n5Words[%d].MeaningKo is empty", i)
		}
		if word.PartOfSpeech == "" {
			t.Errorf("n5Words[%d].PartOfSpeech is empty", i)
		}

		// 5. (Kana,Kanji,MeaningKo) 완전 중복 없음
		compositeKey := word.Kana + "|" + word.Kanji + "|" + word.MeaningKo
		if encounteredKanaKanjiMeaning[compositeKey] {
			t.Errorf("duplicate (Kana, Kanji, MeaningKo) found: %q at index %d", compositeKey, i)
		}
		encounteredKanaKanjiMeaning[compositeKey] = true

		// 6. PartOfSpeech가 whitelist 중 하나
		if !partOfSpeechWhitelist[word.PartOfSpeech] {
			t.Errorf("n5Words[%d].PartOfSpeech = %q is not in whitelist", i, word.PartOfSpeech)
		}
	}

	// 7. buildVocabularyMaterials(n5Words) 결과 500개
	materials := buildVocabularyMaterials(n5Words)
	if len(materials) != 500 {
		t.Fatalf("len(materials) after buildVocabularyMaterials = %d, want 500", len(materials))
	}

	// 8. MaterialKey 중복 없음
	materialKeys := make(map[string]bool)
	for _, material := range materials {
		if materialKeys[material.MaterialKey] {
			t.Errorf("duplicate MaterialKey found: %q", material.MaterialKey)
		}
		materialKeys[material.MaterialKey] = true
	}

	// 9. ja:vocab:word_500 존재
	if !materialKeys["ja:vocab:word_500"] {
		t.Fatal("missing material key ja:vocab:word_500")
	}
}
