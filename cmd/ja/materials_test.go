package ja

import (
	"encoding/json"
	"testing"

	"github.com/lsj/copylingo/internal/model"
)

func TestBuildVocabularyMaterials(t *testing.T) {
	t.Parallel()

	materials := BuildVocabularyMaterials(N5Words)
	if len(materials) != 500 {
		t.Fatalf("len(materials) = %d, want 500", len(materials))
	}

	keys := materialKeys(materials)
	if !keys["ja:vocab:word_024"] {
		t.Fatal("missing vocabulary word_024 material key")
	}
	if !keys["ja:vocab:word_500"] {
		t.Fatal("missing vocabulary word_500 material key")
	}
}

func TestBuildVocabularyMaterialsPayload(t *testing.T) {
	t.Parallel()

	for _, material := range BuildVocabularyMaterials(N5Words) {
		if material.MaterialKey != "ja:vocab:word_024" {
			continue
		}

		var payload VocabularyMaterialPayload
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

func TestBuildKanaMaterials(t *testing.T) {
	t.Parallel()

	materials := BuildKanaMaterials(KanaMap)
	if len(materials) != len(KanaMap) {
		t.Fatalf("len(materials) = %d, want %d", len(materials), len(KanaMap))
	}

	keys := materialKeys(materials)
	if !keys["ja:kana:u3042"] {
		t.Fatal("missing hiragana a material")
	}
	if !keys["ja:kana:u304d_u3083"] {
		t.Fatal("missing hiragana kya material")
	}

	for _, material := range materials {
		if material.Category != model.MaterialCategoryKana ||
			material.Language != VocabLanguage ||
			material.ProficiencyLevel != VocabProficiencyLevel ||
			material.Difficulty != 1 {
			t.Fatalf("unexpected kana material metadata: %+v", material)
		}
	}
}

func TestN5WordsIntegrity(t *testing.T) {
	t.Parallel()

	if len(N5Words) != 500 {
		t.Fatalf("len(N5Words) = %d, want 500", len(N5Words))
	}

	ids := make(map[string]bool, len(N5Words))
	for _, word := range N5Words {
		if word.ID == "" || word.Kana == "" || word.Kanji == "" || word.MeaningKo == "" || word.PartOfSpeech == "" {
			t.Fatalf("incomplete word: %+v", word)
		}
		if ids[word.ID] {
			t.Fatalf("duplicate ID %q", word.ID)
		}
		ids[word.ID] = true
	}
}

func materialKeys(materials []*model.Material) map[string]bool {
	keys := make(map[string]bool, len(materials))
	for _, material := range materials {
		keys[material.MaterialKey] = true
	}
	return keys
}
