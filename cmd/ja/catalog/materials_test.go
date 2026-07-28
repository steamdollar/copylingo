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
	if len(materials) != 540 {
		t.Fatalf("len(materials) = %d, want 540", len(materials))
	}

	keys := materialKeys(materials)
	if !keys["ja:vocab:n5_word_024"] {
		t.Fatal("missing vocabulary word_024 material key")
	}
	if !keys["ja:vocab:n5_word_500"] {
		t.Fatal("missing vocabulary word_500 material key")
	}
}

func TestBuildVocabularyMaterialsPayload(t *testing.T) {
	t.Parallel()

	for _, material := range BuildVocabularyMaterials(N5Words) {
		if material.MaterialKey != "ja:vocab:n5_word_024" {
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
	if len(materials) != 80 {
		t.Fatalf("len(materials) = %d, want 80", len(materials))
	}

	keys := materialKeys(materials)
	if !keys["ja:grammar:n5_grammar_001"] {
		t.Fatal("missing grammar 001 material key")
	}
	if !keys["ja:grammar:n5_grammar_060"] {
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
		if material.MaterialKey != "ja:grammar:n5_grammar_009" {
			continue
		}

		var payload GrammarMaterialPayload
		if err := json.Unmarshal(material.Payload, &payload); err != nil {
			t.Fatalf("Unmarshal: %v", err)
		}
		if payload.Pattern != "があります" ||
			payload.MeaningKo != "사물의 존재" ||
			payload.Example != "机の上に本があります。" ||
			payload.ExampleReading != "つくえのうえにほんがあります。" ||
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
	want := len(KanaMap) + len(N5Words) + len(N5GrammarPoints) + len(N5ReadingPassages)
	if len(materials) != want {
		t.Fatalf("len(materials) = %d, want %d", len(materials), want)
	}

	keys := materialKeys(materials)
	for _, key := range []string{
		"ja:kana:u3042",
		"ja:vocab:n5_word_024",
		"ja:grammar:n5_grammar_001",
		"ja:reading:n5_reading_0001",
	} {
		if !keys[key] {
			t.Fatalf("missing material key %q", key)
		}
	}
}

func TestBuildReadingMaterials(t *testing.T) {
	t.Parallel()

	passages := []ReadingPassage{
		{
			ID:      "n5_reading_0001",
			Skill:   model.SkillReadingShort,
			Title:   "図書館のお知らせ",
			Passage: "図書館は毎週月曜日が休みです。",
			Reading: "としょかんはまいしゅうげつようびがやすみです。",
			KeyVocabulary: []ReadingVocabulary{
				{Surface: "図書館", Reading: "としょかん", MeaningKo: "도서관"},
			},
			Difficulty: 2,
		},
	}

	materials := BuildReadingMaterials(passages)
	if len(materials) != 1 {
		t.Fatalf("len(materials) = %d, want 1", len(materials))
	}

	material := materials[0]
	if material.MaterialKey != "ja:reading:n5_reading_0001" ||
		material.Category != model.MaterialCategoryReading ||
		material.Language != VocabLanguage ||
		material.ProficiencyLevel != VocabProficiencyLevel ||
		material.Title != "図書館のお知らせ" ||
		material.Difficulty != 2 {
		t.Fatalf("unexpected reading material metadata: %+v", material)
	}
}

func TestBuildReadingMaterialsPayload(t *testing.T) {
	t.Parallel()

	passages := []ReadingPassage{
		{
			ID:      "n5_reading_0002",
			Skill:   model.SkillReadingShort,
			Title:   "田中さんの朝",
			Passage: "田中さんは毎朝6時に起きます。",
			Reading: "たなかさんはまいあさろくじにおきます。",
			KeyVocabulary: []ReadingVocabulary{
				{Surface: "起きる", Reading: "おきる", MeaningKo: "일어나다"},
			},
			// Quiz-only fields must never leak into the study payload.
			Prompt:        "田中さんは何時に起きますか。",
			Options:       []string{"6時", "7時", "8時", "9時"},
			CorrectAnswer: "6時",
			Explanation:   "「毎朝6時に起きます」라고 했습니다.",
			Difficulty:    1,
		},
	}

	materials := BuildReadingMaterials(passages)
	if len(materials) != 1 {
		t.Fatalf("len(materials) = %d, want 1", len(materials))
	}

	var payload ReadingMaterialPayload
	if err := json.Unmarshal(materials[0].Payload, &payload); err != nil {
		t.Fatalf("unmarshal reading payload: %v", err)
	}
	if payload.Passage != "田中さんは毎朝6時に起きます。" ||
		payload.Reading != "たなかさんはまいあさろくじにおきます。" ||
		len(payload.KeyVocabulary) != 1 ||
		payload.KeyVocabulary[0].Surface != "起きる" {
		t.Fatalf("unexpected reading payload: %+v", payload)
	}

	raw := string(materials[0].Payload)
	// "7時" appears only in the quiz options; "何時に" only in the quiz prompt.
	for _, leaked := range []string{"prompt", "correct_answer", "explanation", "7時", "何時に"} {
		if strings.Contains(raw, leaked) {
			t.Fatalf("reading payload leaks quiz field %q: %s", leaked, raw)
		}
	}
}

func TestN5WordsIntegrity(t *testing.T) {
	t.Parallel()

	if len(N5Words) != 540 {
		t.Fatalf("len(N5Words) = %d, want 540", len(N5Words))
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

	if len(N5GrammarPoints) != 80 {
		t.Fatalf("len(N5GrammarPoints) = %d, want 80", len(N5GrammarPoints))
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

func TestN5VocabContextIntegrity(t *testing.T) {
	t.Parallel()

	if len(N5VocabContext) != 15 {
		t.Fatalf("len(N5VocabContext) = %d, want 15", len(N5VocabContext))
	}

	wordIDs := make(map[string]bool, len(N5Words))
	for _, word := range N5Words {
		wordIDs[word.ID] = true
	}

	totalClozes := 0
	seenWords := make(map[string]bool, len(N5VocabContext))
	for _, vc := range N5VocabContext {
		if !wordIDs[vc.WordID] {
			t.Fatalf("vocab context references unknown word_id %q", vc.WordID)
		}
		if seenWords[vc.WordID] {
			t.Fatalf("duplicate vocab context word_id %q", vc.WordID)
		}
		seenWords[vc.WordID] = true

		if vc.CorrectAnswer == "" {
			t.Fatalf("empty correct_answer for %+v", vc)
		}
		if len(vc.FormOptions) != 4 {
			t.Fatalf("len(FormOptions) = %d for %+v, want 4", len(vc.FormOptions), vc)
		}
		hasAnswer := false
		options := make(map[string]bool, len(vc.FormOptions))
		for _, option := range vc.FormOptions {
			if options[option] {
				t.Fatalf("duplicate FormOptions value %q for %+v", option, vc)
			}
			options[option] = true
			if option == vc.CorrectAnswer {
				hasAnswer = true
			}
		}
		if !hasAnswer {
			t.Fatalf("FormOptions for %+v do not contain correct answer", vc)
		}

		// >= 2 clozes is a decided constraint: a single example would repeat
		// verbatim on every SRS re-serve, defeating the reading-comprehension goal.
		if len(vc.Clozes) < 2 {
			t.Fatalf("vocab context %q must have >= 2 clozes, got %d", vc.WordID, len(vc.Clozes))
		}
		for _, cloze := range vc.Clozes {
			totalClozes++
			if !strings.Contains(cloze, "__") {
				t.Fatalf("cloze for %q must contain blank marker: %q", vc.WordID, cloze)
			}
			if strings.Contains(cloze, vc.CorrectAnswer) {
				t.Fatalf("cloze for %q reveals the correct answer: %q", vc.WordID, cloze)
			}
		}
	}
	if totalClozes != 45 {
		t.Fatalf("total clozes = %d, want 45", totalClozes)
	}
}

func materialKeys(materials []*model.Material) map[string]bool {
	keys := make(map[string]bool, len(materials))
	for _, material := range materials {
		keys[material.MaterialKey] = true
	}
	return keys
}
