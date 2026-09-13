package catalog

import (
	"encoding/json"
	"reflect"
	"strings"
	"testing"

	"github.com/lsj/copylingo/internal/model"
)

func TestBuildVocabularyMaterials(t *testing.T) {
	t.Parallel()

	materials := BuildVocabularyMaterials(levelCatalogForTest(t, DefaultProficiencyLevel()).Words)
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

	for _, material := range BuildVocabularyMaterials(levelCatalogForTest(t, DefaultProficiencyLevel()).Words) {
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

	materials := BuildGrammarMaterials(levelCatalogForTest(t, DefaultProficiencyLevel()).GrammarPoints)
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
			material.ProficiencyLevel != DefaultProficiencyLevel() ||
			material.Difficulty != GrammarDifficulty {
			t.Fatalf("unexpected grammar material metadata: %+v", material)
		}
	}
}

func TestBuildGrammarMaterialsPayload(t *testing.T) {
	t.Parallel()

	for _, material := range BuildGrammarMaterials(levelCatalogForTest(t, DefaultProficiencyLevel()).GrammarPoints) {
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
			material.ProficiencyLevel != DefaultProficiencyLevel() ||
			material.Difficulty != 1 {
			t.Fatalf("unexpected kana material metadata: %+v", material)
		}
	}
}

func TestBuildAllMaterialsIncludesGrammar(t *testing.T) {
	t.Parallel()

	materials := BuildAllMaterials()
	catalog := levelCatalogForTest(t, DefaultProficiencyLevel())
	want := len(KanaMap) + len(catalog.Words) + len(catalog.GrammarPoints) + len(catalog.ReadingPassages)
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
		material.ProficiencyLevel != DefaultProficiencyLevel() ||
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

	words := levelCatalogForTest(t, DefaultProficiencyLevel()).Words
	if len(words) != 540 {
		t.Fatalf("len(words) = %d, want 540", len(words))
	}

	ids := make(map[string]bool, len(words))
	for _, word := range words {
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

	grammarPoints := levelCatalogForTest(t, DefaultProficiencyLevel()).GrammarPoints
	if len(grammarPoints) != 80 {
		t.Fatalf("len(grammarPoints) = %d, want 80", len(grammarPoints))
	}

	ids := make(map[string]bool, len(grammarPoints))
	for _, point := range grammarPoints {
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

	catalog := levelCatalogForTest(t, DefaultProficiencyLevel())
	if len(catalog.VocabContexts) != 15 {
		t.Fatalf("len(vocab contexts) = %d, want 15", len(catalog.VocabContexts))
	}

	wordIDs := make(map[string]bool, len(catalog.Words))
	for _, word := range catalog.Words {
		wordIDs[word.ID] = true
	}

	totalClozes := 0
	seenWords := make(map[string]bool, len(catalog.VocabContexts))
	for _, vc := range catalog.VocabContexts {
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

func TestBuildAdditionalLevelMaterialsAreLevelAwareAndDoNotCollide(t *testing.T) {
	t.Parallel()

	defaultCatalog := levelCatalogForTest(t, DefaultProficiencyLevel())
	additionalCatalog := levelCatalogForTest(t, "N4")
	defaultMaterials := BuildAllMaterialsForLevels(defaultCatalog.Level)
	combined := BuildAllMaterialsForLevels(defaultCatalog.Level, additionalCatalog.Level)
	defaultKeys := materialKeys(defaultMaterials)
	seen := make(map[string]bool, len(combined))
	additionalCount := 0
	for _, material := range combined {
		if seen[material.MaterialKey] {
			t.Fatalf("duplicate material key %q", material.MaterialKey)
		}
		seen[material.MaterialKey] = true
		if material.ProficiencyLevel != additionalCatalog.Level {
			continue
		}
		additionalCount++
		if defaultKeys[material.MaterialKey] {
			t.Fatalf("additional material key collides with default: %q", material.MaterialKey)
		}
		if !strings.Contains(strings.ToLower(material.MaterialKey), "n4") {
			t.Fatalf("N4 material key %q does not include level", material.MaterialKey)
		}
	}
	wantAdditional := len(
		additionalCatalog.Words,
	) + len(
		additionalCatalog.GrammarPoints,
	) + len(
		additionalCatalog.ReadingPassages,
	)
	if additionalCount != wantAdditional {
		t.Fatalf("additional material count = %d, want %d", additionalCount, wantAdditional)
	}

	second := BuildAllMaterialsForLevels(defaultCatalog.Level, additionalCatalog.Level)
	if !reflect.DeepEqual(second, combined) {
		t.Fatal("combined material output is not deterministic")
	}
}
