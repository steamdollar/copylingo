package catalog

import (
	"fmt"
	"strings"
	"testing"

	"github.com/lsj/copylingo/internal/model"
)

func levelCatalogForTest(t *testing.T, level string) LevelCatalog {
	t.Helper()
	catalog, ok := LevelCatalogFor(level)
	if !ok {
		t.Fatalf("catalog for level %q not found", level)
	}
	return catalog
}

func TestLevelCatalogRegistry(t *testing.T) {
	t.Parallel()

	catalogs := LevelCatalogs()
	if len(catalogs) < 2 {
		t.Fatalf("catalog count = %d, want at least 2", len(catalogs))
	}
	if catalogs[0].Level != DefaultProficiencyLevel() {
		t.Fatalf("default level = %q, first catalog = %q", DefaultProficiencyLevel(), catalogs[0].Level)
	}
	for _, catalog := range catalogs {
		got, ok := LevelCatalogFor("  " + strings.ToLower(catalog.Level) + "  ")
		if !ok || got.Level != catalog.Level {
			t.Fatalf("normalized lookup for %q = (%q, %v)", catalog.Level, got.Level, ok)
		}
	}
	if _, ok := LevelCatalogFor("N0"); ok {
		t.Fatal("unknown level unexpectedly resolved")
	}
}

// Integrity regressions for the embedded datasets. The JSON files under data/
// are the source of truth, so these checks catch a malformed or regressed file
// at test time instead of at runtime. They inspect the package vars directly,
// i.e. exactly what the seeder consumes.

func TestN5Grammar_Integrity(t *testing.T) {
	grammarPoints := levelCatalogForTest(t, "N5").GrammarPoints
	if len(grammarPoints) != 80 {
		t.Fatalf("expected 80 grammar points, got %d", len(grammarPoints))
	}
	seen := make(map[string]bool, len(grammarPoints))
	for i, p := range grammarPoints {
		if p.ID == "" || p.Pattern == "" || p.MeaningKo == "" ||
			p.Example == "" || p.ExampleReading == "" || p.ClozePrompt == "" || p.CorrectAnswer == "" {
			t.Errorf("point %d (%q) has an empty required field", i, p.ID)
		}
		if seen[p.ID] {
			t.Errorf("duplicate grammar ID %q", p.ID)
		}
		seen[p.ID] = true

		if len(p.FormOptions) < 2 {
			t.Errorf("point %q: expected >=2 form options, got %d", p.ID, len(p.FormOptions))
		}
		found := false
		for _, opt := range p.FormOptions {
			if opt == p.CorrectAnswer {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("point %q: correct answer %q not in form options %v", p.ID, p.CorrectAnswer, p.FormOptions)
		}
	}
}

func TestN5Vocab_Integrity(t *testing.T) {
	words := levelCatalogForTest(t, "N5").Words
	if len(words) != 540 {
		t.Fatalf("expected 540 vocab words, got %d", len(words))
	}
	seen := make(map[string]bool, len(words))
	for i, w := range words {
		if w.ID == "" || w.Kana == "" || w.Kanji == "" || w.MeaningKo == "" || w.PartOfSpeech == "" {
			t.Errorf("word %d (%q) has an empty required field", i, w.ID)
		}
		if seen[w.ID] {
			t.Errorf("duplicate vocab ID %q", w.ID)
		}
		seen[w.ID] = true
	}
}

func TestKana_Integrity(t *testing.T) {
	if len(KanaMap) != 208 {
		t.Fatalf("expected 208 kana entries, got %d", len(KanaMap))
	}
	for k, romaji := range KanaMap {
		if k == "" || romaji == "" {
			t.Errorf("kana entry %q→%q has an empty key or value", k, romaji)
		}
	}
}

func TestN5Listening_Integrity(t *testing.T) {
	listeningQuestions := levelCatalogForTest(t, "N5").ListeningQuestions
	if len(listeningQuestions) != 50 {
		t.Fatalf("expected 50 listening questions, got %d", len(listeningQuestions))
	}

	allowedSkills := map[model.Skill]bool{
		model.SkillListeningTask:     true,
		model.SkillListeningKeyPoint: true,
		model.SkillListeningOutline:  true,
	}
	seenIDs := make(map[string]bool, len(listeningQuestions))
	seenScripts := make(map[string]bool, len(listeningQuestions))
	seenPrompts := make(map[string]bool, len(listeningQuestions))
	for i, question := range listeningQuestions {
		if wantID := fmt.Sprintf("n5_listening_%04d", i+1); question.ID != wantID {
			t.Errorf("listening question %d ID = %q, want %q", i, question.ID, wantID)
		}
		if question.ID == "" || question.Script == "" || question.Prompt == "" ||
			question.CorrectAnswer == "" || question.Explanation == "" {
			t.Errorf("listening question %d (%q) has an empty required field", i, question.ID)
		}
		if seenIDs[question.ID] {
			t.Errorf("duplicate listening ID %q", question.ID)
		}
		seenIDs[question.ID] = true
		if seenScripts[question.Script] {
			t.Errorf("duplicate listening script for %q", question.ID)
		}
		seenScripts[question.Script] = true
		if seenPrompts[question.Prompt] {
			t.Errorf("duplicate listening prompt for %q", question.ID)
		}
		seenPrompts[question.Prompt] = true
		if !allowedSkills[question.Skill] {
			t.Errorf("question %q has unsupported skill %q", question.ID, question.Skill)
		}
		if question.Difficulty < 1 || question.Difficulty > 3 {
			t.Errorf("question %q has difficulty %d, want 1..3", question.ID, question.Difficulty)
		}
		if len(question.Options) != 4 {
			t.Errorf("question %q has %d options, want 4", question.ID, len(question.Options))
		}

		seenOptions := make(map[string]bool, len(question.Options))
		for _, option := range question.Options {
			if option == "" {
				t.Errorf("question %q has an empty option", question.ID)
			}
			if seenOptions[option] {
				t.Errorf("question %q has duplicate option %q", question.ID, option)
			}
			seenOptions[option] = true
		}
		if !seenOptions[question.CorrectAnswer] {
			t.Errorf("question %q correct answer %q is not in options", question.ID, question.CorrectAnswer)
		}
	}
}

func TestN5Reading_Integrity(t *testing.T) {
	readingPassages := levelCatalogForTest(t, "N5").ReadingPassages
	if len(readingPassages) != 40 {
		t.Fatalf("expected 40 reading passages, got %d", len(readingPassages))
	}

	// The seeder inlines passage/prompt/options into an HTML prompt without
	// escaping, so the dataset itself must stay free of HTML-special characters.
	assertNoHTMLSpecials := func(id, field, value string) {
		if strings.ContainsAny(value, "<>&") {
			t.Errorf("passage %q field %s contains HTML-special characters: %q", id, field, value)
		}
	}

	seenIDs := make(map[string]bool, len(readingPassages))
	seenPassages := make(map[string]bool, len(readingPassages))
	seenPrompts := make(map[string]bool, len(readingPassages))
	for i, passage := range readingPassages {
		if wantID := fmt.Sprintf("n5_reading_%04d", i+1); passage.ID != wantID {
			t.Errorf("reading passage %d ID = %q, want %q", i, passage.ID, wantID)
		}
		if passage.ID == "" || passage.Title == "" || passage.Passage == "" ||
			passage.Reading == "" || passage.Prompt == "" ||
			passage.CorrectAnswer == "" || passage.Explanation == "" {
			t.Errorf("reading passage %d (%q) has an empty required field", i, passage.ID)
		}
		if seenIDs[passage.ID] {
			t.Errorf("duplicate reading ID %q", passage.ID)
		}
		seenIDs[passage.ID] = true
		if seenPassages[passage.Passage] {
			t.Errorf("duplicate reading passage text for %q", passage.ID)
		}
		seenPassages[passage.Passage] = true
		if seenPrompts[passage.Prompt] {
			t.Errorf("duplicate reading prompt for %q", passage.ID)
		}
		seenPrompts[passage.Prompt] = true
		// reading_short only for the initial 10-passage corpus; other reading
		// skills need a separate distribution decision before the 50 expansion.
		if passage.Skill != model.SkillReadingShort {
			t.Errorf("passage %q has unsupported skill %q", passage.ID, passage.Skill)
		}
		if passage.Difficulty < 1 || passage.Difficulty > 3 {
			t.Errorf("passage %q has difficulty %d, want 1..3", passage.ID, passage.Difficulty)
		}
		if len(passage.KeyVocabulary) == 0 {
			t.Errorf("passage %q has no key vocabulary", passage.ID)
		}
		for _, vocab := range passage.KeyVocabulary {
			if vocab.Surface == "" || vocab.Reading == "" || vocab.MeaningKo == "" {
				t.Errorf("passage %q has an incomplete key vocabulary entry: %+v", passage.ID, vocab)
			}
		}
		assertNoHTMLSpecials(passage.ID, "passage", passage.Passage)
		assertNoHTMLSpecials(passage.ID, "prompt", passage.Prompt)

		if len(passage.Options) != 4 {
			t.Errorf("passage %q has %d options, want 4", passage.ID, len(passage.Options))
		}
		seenOptions := make(map[string]bool, len(passage.Options))
		for _, option := range passage.Options {
			if option == "" {
				t.Errorf("passage %q has an empty option", passage.ID)
			}
			if seenOptions[option] {
				t.Errorf("passage %q has duplicate option %q", passage.ID, option)
			}
			seenOptions[option] = true
			assertNoHTMLSpecials(passage.ID, "option", option)
		}
		if !seenOptions[passage.CorrectAnswer] {
			t.Errorf("passage %q correct answer %q is not in options", passage.ID, passage.CorrectAnswer)
		}
	}
}

func TestN4CatalogFixturesCoverOfficialItemTypes(t *testing.T) {
	t.Parallel()

	catalog := levelCatalogForTest(t, "N4")
	if len(catalog.Words) == 0 || len(catalog.GrammarPoints) == 0 || len(catalog.ReadingPassages) == 0 ||
		len(catalog.ListeningQuestions) == 0 ||
		len(catalog.QuestionSeeds) == 0 {
		t.Fatalf(
			"N4 catalog datasets must be non-empty: vocab=%d grammar=%d reading=%d listening=%d seeds=%d",
			len(catalog.Words),
			len(catalog.GrammarPoints),
			len(catalog.ReadingPassages),
			len(catalog.ListeningQuestions),
			len(catalog.QuestionSeeds),
		)
	}
	seenWordIDs := make(map[string]bool, len(catalog.Words))
	for _, word := range catalog.Words {
		if word.Level != catalog.Level || word.ID == "" || word.Kana == "" || word.Kanji == "" || word.MeaningKo == "" {
			t.Fatalf("invalid N4 vocabulary row: %+v", word)
		}
		if seenWordIDs[word.ID] {
			t.Fatalf("duplicate N4 vocabulary ID %q", word.ID)
		}
		seenWordIDs[word.ID] = true
	}
	seenGrammarIDs := make(map[string]bool, len(catalog.GrammarPoints))
	for _, point := range catalog.GrammarPoints {
		if point.Level != catalog.Level || point.ID == "" || point.Pattern == "" || point.CorrectAnswer == "" {
			t.Fatalf("invalid N4 grammar row: %+v", point)
		}
		if seenGrammarIDs[point.ID] {
			t.Fatalf("duplicate N4 grammar ID %q", point.ID)
		}
		seenGrammarIDs[point.ID] = true
		if len(point.FormOptions) == 0 || !contains(point.FormOptions, point.CorrectAnswer) {
			t.Fatalf("N4 grammar %q has invalid form options", point.ID)
		}
	}
	seenReadingIDs := make(map[string]bool, len(catalog.ReadingPassages))
	readingSkills := make([]model.Skill, 0, len(catalog.ReadingPassages))
	for _, passage := range catalog.ReadingPassages {
		if passage.Level != catalog.Level || passage.ID == "" || passage.Skill == "" || passage.Title == "" ||
			passage.Passage == "" ||
			passage.Reading == "" ||
			passage.Prompt == "" ||
			len(passage.Options) == 0 ||
			!contains(passage.Options, passage.CorrectAnswer) {
			t.Fatalf("invalid N4 reading row: %+v", passage)
		}
		if seenReadingIDs[passage.ID] {
			t.Fatalf("duplicate N4 reading ID %q", passage.ID)
		}
		seenReadingIDs[passage.ID] = true
		readingSkills = append(readingSkills, passage.Skill)
	}

	want := map[model.Skill]bool{
		model.SkillVocabKanjiReading:      true,
		model.SkillVocabOrthography:       true,
		model.SkillVocabContext:           true,
		model.SkillVocabParaphrase:        true,
		model.SkillVocabUsage:             true,
		model.SkillGrammarForm:            true,
		model.SkillSentenceComposition:    true,
		model.SkillGrammarText:            true,
		model.SkillReadingShort:           true,
		model.SkillReadingMedium:          true,
		model.SkillReadingInformation:     true,
		model.SkillListeningTask:          true,
		model.SkillListeningKeyPoint:      true,
		model.SkillListeningVerbal:        true,
		model.SkillListeningQuickResponse: true,
	}
	got := make(map[model.Skill]bool, len(catalog.QuestionSeeds)+len(catalog.ListeningQuestions)+len(readingSkills))
	for _, skill := range readingSkills {
		got[skill] = true
	}
	seenSeedIDs := make(map[string]bool, len(catalog.QuestionSeeds))
	for _, seed := range catalog.QuestionSeeds {
		seedID := seed.SourceID
		if seedID == "" {
			seedID = seed.ID
		}
		if seedID == "" || seed.ItemType == "" || seed.Type == "" || seed.Category == "" || seed.Prompt == "" ||
			seed.CorrectAnswer == "" {
			t.Fatalf("invalid N4 question seed %q: %+v", seedID, seed)
		}
		if seenSeedIDs[seedID] {
			t.Fatalf("duplicate N4 question seed ID %q", seedID)
		}
		seenSeedIDs[seedID] = true
		if seed.Category == model.CategoryListening && seed.MaterialKey != "" {
			t.Fatalf("N4 listening seed %q must be material-less", seedID)
		}
		if seed.Type == model.QuestionMultipleChoice || seed.Type == model.QuestionReadingComp {
			if len(seed.Options) == 0 || !contains(seed.Options, seed.CorrectAnswer) {
				t.Fatalf("N4 seed %q answer %q is not in options %v", seedID, seed.CorrectAnswer, seed.Options)
			}
		}
		if seed.Type == model.QuestionWordOrder &&
			(len(seed.Options) == 0 || strings.Join(seed.Options, "") != seed.CorrectAnswer) {
			t.Fatalf("N4 word-order seed %q does not join to answer", seedID)
		}
		got[seed.ItemType] = true
	}
	seenListeningIDs := make(map[string]bool, len(catalog.ListeningQuestions))
	for _, question := range catalog.ListeningQuestions {
		if question.Level != catalog.Level || question.ID == "" || question.Script == "" || question.Prompt == "" ||
			len(question.Options) == 0 ||
			!contains(question.Options, question.CorrectAnswer) {
			t.Fatalf("invalid N4 listening row: %+v", question)
		}
		if seenListeningIDs[question.ID] {
			t.Fatalf("duplicate N4 listening ID %q", question.ID)
		}
		seenListeningIDs[question.ID] = true
		got[question.Skill] = true
	}
	if len(got) != len(want) {
		t.Fatalf("N4 item type count = %d, want %d (%v)", len(got), len(want), got)
	}
	for itemType := range want {
		if !got[itemType] {
			t.Fatalf("N4 item type %q is not represented", itemType)
		}
	}
}

func contains(values []string, want string) bool {
	for _, value := range values {
		if value == want {
			return true
		}
	}
	return false
}
