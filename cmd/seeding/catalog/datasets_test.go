package catalog

import (
	"encoding/json"
	"fmt"
	"reflect"
	"strings"
	"testing"

	"github.com/lsj/copylingo/internal/model"
)

func levelCatalogForTest(
	t *testing.T,
	level string,
) LevelCatalog {
	t.Helper()
	catalog, ok := LevelCatalogFor(
		Japanese,
		level,
	)
	if !ok {
		t.Fatalf(
			"catalog for level %q not found",
			level,
		)
	}
	return catalog
}

func TestLevelCatalogRegistry(t *testing.T) {
	t.Parallel()

	catalogs := LevelCatalogsFor(Japanese)
	if len(catalogs) < 2 {
		t.Fatalf(
			"catalog count = %d, want at least 2",
			len(catalogs),
		)
	}
	if catalogs[0].Level != DefaultProficiencyLevel(Japanese) {
		t.Fatalf(
			"default level = %q, first catalog = %q",
			DefaultProficiencyLevel(Japanese),
			catalogs[0].Level,
		)
	}
	for _, catalog := range catalogs {
		if catalog.Language != Japanese {
			t.Fatalf(
				"catalog %q language = %q, want %q",
				catalog.Level,
				catalog.Language,
				Japanese,
			)
		}
		got, ok := LevelCatalogFor(
			" JA ",
			"  "+strings.ToLower(catalog.Level)+"  ",
		)
		if !ok || got.Level != catalog.Level {
			t.Fatalf(
				"normalized lookup for %q = (%q, %v)",
				catalog.Level,
				got.Level,
				ok,
			)
		}
	}
	if _, ok := LevelCatalogFor(
		Japanese,
		"N0",
	); ok {
		t.Fatal("unknown level unexpectedly resolved")
	}
	// A language without a data/<code>/ registry entry must not borrow
	// another language's catalogs.
	if got := LevelCatalogsFor("el"); len(got) != 0 {
		t.Fatalf(
			"unregistered language catalogs = %d, want 0",
			len(got),
		)
	}
	if _, ok := LevelCatalogFor(
		"el",
		catalogs[0].Level,
	); ok {
		t.Fatal("level resolved for an unregistered language")
	}
}

// Integrity regressions for the embedded datasets. The JSON files under data/<language>/
// are the source of truth, so these checks catch a malformed or regressed file
// at test time instead of at runtime. They inspect the package vars directly,
// i.e. exactly what the seeder consumes.

func TestN5Grammar_Integrity(t *testing.T) {
	grammarPoints := levelCatalogForTest(
		t,
		"N5",
	).GrammarPoints
	if len(grammarPoints) != 80 {
		t.Fatalf(
			"expected 80 grammar points, got %d",
			len(grammarPoints),
		)
	}
	seen := make(
		map[string]bool,
		len(grammarPoints),
	)
	for i, p := range grammarPoints {
		if p.ID == "" || p.Pattern == "" || p.MeaningKo == "" ||
			p.Example == "" || p.ExampleReading == "" || p.ClozePrompt == "" || p.CorrectAnswer == "" {
			t.Errorf(
				"point %d (%q) has an empty required field",
				i,
				p.ID,
			)
		}
		if seen[p.ID] {
			t.Errorf(
				"duplicate grammar ID %q",
				p.ID,
			)
		}
		seen[p.ID] = true

		if len(p.FormOptions) < 2 {
			t.Errorf(
				"point %q: expected >=2 form options, got %d",
				p.ID,
				len(p.FormOptions),
			)
		}
		found := false
		for _, opt := range p.FormOptions {
			if opt == p.CorrectAnswer {
				found = true
				break
			}
		}
		if !found {
			t.Errorf(
				"point %q: correct answer %q not in form options %v",
				p.ID,
				p.CorrectAnswer,
				p.FormOptions,
			)
		}
	}
}

func TestN5Vocab_Integrity(t *testing.T) {
	words := levelCatalogForTest(
		t,
		"N5",
	).Words
	if len(words) != 789 {
		t.Fatalf(
			"expected 789 vocab words, got %d",
			len(words),
		)
	}
	seen := make(
		map[string]bool,
		len(words),
	)
	for i, w := range words {
		if w.ID == "" || w.Kana == "" || w.Kanji == "" || w.MeaningKo == "" || w.PartOfSpeech == "" {
			t.Errorf(
				"word %d (%q) has an empty required field",
				i,
				w.ID,
			)
		}
		if seen[w.ID] {
			t.Errorf(
				"duplicate vocab ID %q",
				w.ID,
			)
		}
		seen[w.ID] = true
	}
}

func TestKana_Integrity(t *testing.T) {
	if len(KanaMap) != 208 {
		t.Fatalf(
			"expected 208 kana entries, got %d",
			len(KanaMap),
		)
	}
	for k, romaji := range KanaMap {
		if k == "" || romaji == "" {
			t.Errorf(
				"kana entry %q→%q has an empty key or value",
				k,
				romaji,
			)
		}
	}
}

func TestN5Listening_Integrity(t *testing.T) {
	listeningQuestions := levelCatalogForTest(
		t,
		"N5",
	).ListeningQuestions
	if len(listeningQuestions) != 50 {
		t.Fatalf(
			"expected 50 listening questions, got %d",
			len(listeningQuestions),
		)
	}

	allowedSkills := map[model.Skill]bool{
		model.SkillListeningTask:     true,
		model.SkillListeningKeyPoint: true,
		model.SkillListeningOutline:  true,
	}
	seenIDs := make(
		map[string]bool,
		len(listeningQuestions),
	)
	seenScripts := make(
		map[string]bool,
		len(listeningQuestions),
	)
	seenPrompts := make(
		map[string]bool,
		len(listeningQuestions),
	)
	for i, question := range listeningQuestions {
		if wantID := fmt.Sprintf(
			"n5_listening_%04d",
			i+1,
		); question.ID != wantID {
			t.Errorf(
				"listening question %d ID = %q, want %q",
				i,
				question.ID,
				wantID,
			)
		}
		if question.ID == "" || question.Script == "" || question.Prompt == "" ||
			question.CorrectAnswer == "" || question.Explanation == "" {
			t.Errorf(
				"listening question %d (%q) has an empty required field",
				i,
				question.ID,
			)
		}
		if seenIDs[question.ID] {
			t.Errorf(
				"duplicate listening ID %q",
				question.ID,
			)
		}
		seenIDs[question.ID] = true
		if seenScripts[question.Script] {
			t.Errorf(
				"duplicate listening script for %q",
				question.ID,
			)
		}
		seenScripts[question.Script] = true
		if seenPrompts[question.Prompt] {
			t.Errorf(
				"duplicate listening prompt for %q",
				question.ID,
			)
		}
		seenPrompts[question.Prompt] = true
		if !allowedSkills[question.Skill] {
			t.Errorf(
				"question %q has unsupported skill %q",
				question.ID,
				question.Skill,
			)
		}
		if question.Difficulty < 1 || question.Difficulty > 3 {
			t.Errorf(
				"question %q has difficulty %d, want 1..3",
				question.ID,
				question.Difficulty,
			)
		}
		if len(question.Options) != 4 {
			t.Errorf(
				"question %q has %d options, want 4",
				question.ID,
				len(question.Options),
			)
		}

		seenOptions := make(
			map[string]bool,
			len(question.Options),
		)
		for _, option := range question.Options {
			if option == "" {
				t.Errorf(
					"question %q has an empty option",
					question.ID,
				)
			}
			if seenOptions[option] {
				t.Errorf(
					"question %q has duplicate option %q",
					question.ID,
					option,
				)
			}
			seenOptions[option] = true
		}
		if !seenOptions[question.CorrectAnswer] {
			t.Errorf(
				"question %q correct answer %q is not in options",
				question.ID,
				question.CorrectAnswer,
			)
		}
	}
}

func TestN5Reading_Integrity(t *testing.T) {
	readingPassages := levelCatalogForTest(
		t,
		"N5",
	).ReadingPassages
	if len(readingPassages) != 40 {
		t.Fatalf(
			"expected 40 reading passages, got %d",
			len(readingPassages),
		)
	}

	// The seeder inlines passage/prompt/options into an HTML prompt without
	// escaping, so the dataset itself must stay free of HTML-special characters.
	assertNoHTMLSpecials := func(
		id,
		field,
		value string,
	) {
		if strings.ContainsAny(
			value,
			"<>&",
		) {
			t.Errorf(
				"passage %q field %s contains HTML-special characters: %q",
				id,
				field,
				value,
			)
		}
	}

	seenIDs := make(
		map[string]bool,
		len(readingPassages),
	)
	seenPassages := make(
		map[string]bool,
		len(readingPassages),
	)
	seenPrompts := make(
		map[string]bool,
		len(readingPassages),
	)
	for i, passage := range readingPassages {
		if wantID := fmt.Sprintf(
			"n5_reading_%04d",
			i+1,
		); passage.ID != wantID {
			t.Errorf(
				"reading passage %d ID = %q, want %q",
				i,
				passage.ID,
				wantID,
			)
		}
		if passage.ID == "" || passage.Title == "" || passage.Passage == "" ||
			passage.Reading == "" || passage.Prompt == "" ||
			passage.CorrectAnswer == "" || passage.Explanation == "" {
			t.Errorf(
				"reading passage %d (%q) has an empty required field",
				i,
				passage.ID,
			)
		}
		if seenIDs[passage.ID] {
			t.Errorf(
				"duplicate reading ID %q",
				passage.ID,
			)
		}
		seenIDs[passage.ID] = true
		if seenPassages[passage.Passage] {
			t.Errorf(
				"duplicate reading passage text for %q",
				passage.ID,
			)
		}
		seenPassages[passage.Passage] = true
		if seenPrompts[passage.Prompt] {
			t.Errorf(
				"duplicate reading prompt for %q",
				passage.ID,
			)
		}
		seenPrompts[passage.Prompt] = true
		// reading_short only for the initial 10-passage corpus; other reading
		// skills need a separate distribution decision before the 50 expansion.
		if passage.Skill != model.SkillReadingShort {
			t.Errorf(
				"passage %q has unsupported skill %q",
				passage.ID,
				passage.Skill,
			)
		}
		if passage.Difficulty < 1 || passage.Difficulty > 3 {
			t.Errorf(
				"passage %q has difficulty %d, want 1..3",
				passage.ID,
				passage.Difficulty,
			)
		}
		if len(passage.KeyVocabulary) == 0 {
			t.Errorf(
				"passage %q has no key vocabulary",
				passage.ID,
			)
		}
		for _, vocab := range passage.KeyVocabulary {
			if vocab.Surface == "" || vocab.Reading == "" || vocab.MeaningKo == "" {
				t.Errorf(
					"passage %q has an incomplete key vocabulary entry: %+v",
					passage.ID,
					vocab,
				)
			}
		}
		assertNoHTMLSpecials(
			passage.ID,
			"passage",
			passage.Passage,
		)
		assertNoHTMLSpecials(
			passage.ID,
			"prompt",
			passage.Prompt,
		)

		if len(passage.Options) != 4 {
			t.Errorf(
				"passage %q has %d options, want 4",
				passage.ID,
				len(passage.Options),
			)
		}
		seenOptions := make(
			map[string]bool,
			len(passage.Options),
		)
		for _, option := range passage.Options {
			if option == "" {
				t.Errorf(
					"passage %q has an empty option",
					passage.ID,
				)
			}
			if seenOptions[option] {
				t.Errorf(
					"passage %q has duplicate option %q",
					passage.ID,
					option,
				)
			}
			seenOptions[option] = true
			assertNoHTMLSpecials(
				passage.ID,
				"option",
				option,
			)
		}
		if !seenOptions[passage.CorrectAnswer] {
			t.Errorf(
				"passage %q correct answer %q is not in options",
				passage.ID,
				passage.CorrectAnswer,
			)
		}
	}
}

func contains(
	values []string,
	want string,
) bool {
	for _, value := range values {
		if value == want {
			return true
		}
	}
	return false
}

// TestRecordCatalogIntegrity checks every level authored in the unified
// record format (ADR-066). Keys are the upsert identity, so they must be
// unique across levels. Only listening questions may sit outside a material.
func TestRecordCatalogIntegrity(t *testing.T) {
	t.Parallel()

	materialKeys := make(map[string]bool)
	questionKeys := make(map[string]bool)
	checkQuestionKey := func(record QuestionRecord) {
		if questionKeys[record.QuestionKey] {
			t.Fatalf(
				"duplicate question key %q",
				record.QuestionKey,
			)
		}
		questionKeys[record.QuestionKey] = true
	}
	recordLevels := 0
	for _, catalog := range LevelCatalogsFor(Japanese) {
		if len(catalog.Materials) == 0 && len(catalog.Questions) == 0 {
			continue
		}
		recordLevels++
		for _, material := range catalog.Materials {
			var payload map[string]any
			if !strings.HasPrefix(
				material.MaterialKey,
				catalog.Language+":",
			) || material.Category == "" || material.Title == "" || material.Difficulty < 1 ||
				json.Unmarshal(
					material.Payload,
					&payload,
				) != nil ||
				len(payload) == 0 {
				t.Fatalf(
					"invalid %s material record %q",
					catalog.Level,
					material.MaterialKey,
				)
			}
			if materialKeys[material.MaterialKey] {
				t.Fatalf(
					"duplicate material key %q",
					material.MaterialKey,
				)
			}
			materialKeys[material.MaterialKey] = true
			for _, record := range material.Questions {
				assertQuestionRecord(
					t,
					catalog,
					record,
				)
				if record.Category == model.CategoryListening {
					t.Fatalf(
						"listening question %q must not belong to material %q",
						record.QuestionKey,
						material.MaterialKey,
					)
				}
				checkQuestionKey(record)
			}
		}
		for _, record := range catalog.Questions {
			assertQuestionRecord(
				t,
				catalog,
				record,
			)
			if record.Category != model.CategoryListening {
				t.Fatalf(
					"material-less question %q must be listening, got %q",
					record.QuestionKey,
					record.Category,
				)
			}
			checkQuestionKey(record)
		}
	}
	if recordLevels == 0 {
		t.Fatal("no level is authored in the unified record format")
	}
}

func assertQuestionRecord(
	t *testing.T,
	catalog LevelCatalog,
	record QuestionRecord,
) {
	t.Helper()
	if !strings.HasPrefix(
		record.QuestionKey,
		catalog.Language+":",
	) || record.ItemType == "" || record.Type == "" || record.Category == "" || record.Prompt == "" ||
		record.CorrectAnswer == "" ||
		record.Difficulty < 1 {
		t.Fatalf(
			"invalid %s question record: %+v",
			catalog.Level,
			record,
		)
	}
	switch record.Type {
	case model.QuestionMultipleChoice, model.QuestionReadingComp, model.QuestionListening:
		if !contains(
			record.Options,
			record.CorrectAnswer,
		) {
			t.Fatalf(
				"question %q answer %q is not in options %v",
				record.QuestionKey,
				record.CorrectAnswer,
				record.Options,
			)
		}
	case model.QuestionWordOrder:
		if len(record.Options) == 0 || strings.Join(
			record.Options,
			"",
		) != record.CorrectAnswer {
			t.Fatalf(
				"word-order question %q chunks do not join to the answer",
				record.QuestionKey,
			)
		}
	}
	if record.Category == model.CategoryListening && record.AudioScript == "" {
		t.Fatalf(
			"listening question %q has no audio script",
			record.QuestionKey,
		)
	}
}

// TestN4RecordsCoverOfficialItemTypes pins N4 to the JLPT item types the
// app models, so a dropped record group under data/ja/n4/ is caught.
func TestN4RecordsCoverOfficialItemTypes(t *testing.T) {
	t.Parallel()

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
	catalog := levelCatalogForTest(
		t,
		"N4",
	)
	got := make(map[model.Skill]bool)
	for _, material := range catalog.Materials {
		for _, record := range material.Questions {
			got[record.ItemType] = true
		}
	}
	for _, record := range catalog.Questions {
		got[record.ItemType] = true
	}
	if !reflect.DeepEqual(
		got,
		want,
	) {
		t.Fatalf(
			"N4 item types = %v, want %v",
			got,
			want,
		)
	}
}
