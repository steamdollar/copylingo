package catalog

import (
	"encoding/json"
	"strings"
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

func TestBuildGrammarMaterials(t *testing.T) {
	t.Parallel()

	materials := BuildGrammarMaterials(N5GrammarPoints)
	if len(materials) != 60 {
		t.Fatalf("len(materials) = %d, want 60", len(materials))
	}

	keys := materialKeys(materials)
	if !keys["ja:grammar:001"] {
		t.Fatal("missing grammar 001 material key")
	}
	if !keys["ja:grammar:060"] {
		t.Fatal("missing grammar 060 material key")
	}

	for _, material := range materials {
		if material.Category != model.MaterialCategoryGrammar ||
			material.Language != VocabLanguage ||
			material.ProficiencyLevel != VocabProficiencyLevel ||
			material.Difficulty != GrammarDifficulty {
			t.Fatalf("unexpected grammar material metadata: %+v", material)
		}
	}
}

func TestBuildGrammarMaterialsPayload(t *testing.T) {
	t.Parallel()

	for _, material := range BuildGrammarMaterials(N5GrammarPoints) {
		if material.MaterialKey != "ja:grammar:009" {
			continue
		}

		var payload GrammarMaterialPayload
		if err := json.Unmarshal(material.Payload, &payload); err != nil {
			t.Fatalf("Unmarshal: %v", err)
		}
		if payload.Pattern != "があります" ||
			payload.MeaningKo != "사물의 존재" ||
			payload.Example != "机の上に本があります。" ||
			payload.TranslationKo != "책상 위에 책이 있습니다." {
			t.Fatalf("payload = %+v", payload)
		}
		return
	}
	t.Fatal("missing grammar 009 material")
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

func TestBuildAllMaterialsIncludesGrammar(t *testing.T) {
	t.Parallel()

	materials := BuildAllMaterials()
	want := len(KanaMap) + len(N5Words) + len(N5GrammarPoints)
	if len(materials) != want {
		t.Fatalf("len(materials) = %d, want %d", len(materials), want)
	}

	keys := materialKeys(materials)
	for _, key := range []string{"ja:kana:u3042", "ja:vocab:word_024", "ja:grammar:001"} {
		if !keys[key] {
			t.Fatalf("missing material key %q", key)
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

func TestN5GrammarPointsIntegrity(t *testing.T) {
	t.Parallel()

	if len(N5GrammarPoints) != 60 {
		t.Fatalf("len(N5GrammarPoints) = %d, want 60", len(N5GrammarPoints))
	}

	ids := make(map[string]bool, len(N5GrammarPoints))
	for _, point := range N5GrammarPoints {
		if point.ID == "" || point.Pattern == "" || point.MeaningKo == "" ||
			point.ExplanationKo == "" || point.Example == "" || point.TranslationKo == "" ||
			point.ClozePrompt == "" || point.CorrectAnswer == "" {
			t.Fatalf("incomplete grammar point: %+v", point)
		}
		if ids[point.ID] {
			t.Fatalf("duplicate ID %q", point.ID)
		}
		ids[point.ID] = true
		if len(point.FormOptions) != 4 {
			t.Fatalf("len(FormOptions) = %d for %+v, want 4", len(point.FormOptions), point)
		}
		if !strings.Contains(point.ClozePrompt, "__") {
			t.Fatalf("ClozePrompt for %+v must contain blank marker", point)
		}
		if strings.Contains(point.ClozePrompt, point.CorrectAnswer) {
			t.Fatalf("ClozePrompt for %+v reveals the correct answer", point)
		}
		hasAnswer := false
		options := make(map[string]bool, len(point.FormOptions))
		for _, option := range point.FormOptions {
			if options[option] {
				t.Fatalf("duplicate FormOptions value %q for %+v", option, point)
			}
			options[option] = true
			if option == point.CorrectAnswer {
				hasAnswer = true
			}
		}
		if !hasAnswer {
			t.Fatalf("FormOptions for %+v do not contain correct answer", point)
		}
	}
}

func materialKeys(materials []*model.Material) map[string]bool {
	keys := make(map[string]bool, len(materials))
	for _, material := range materials {
		keys[material.MaterialKey] = true
	}
	return keys
}
