package main

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"
	"unicode/utf8"

	"github.com/lsj/copylingo/internal/model"
)

const japanese = "ja"

// The data/<language>/<level>.json files are the source of truth, so
// these checks catch a malformed or regressed file at test time instead of at
// seed time. They inspect the loaded records, i.e. exactly what the seeder
// consumes.

// TestRecordIntegrity checks every level file (ADR-066). Keys are the
// upsert identity, so they must be unique across levels. Only listening
// questions may sit outside a material.
func TestRecordIntegrity(t *testing.T) {
	t.Parallel()

	materials, looseQuestions, err := loadRecords(japanese)
	if err != nil {
		t.Fatalf(
			"loadRecords: %v",
			err,
		)
	}
	if len(materials) == 0 {
		t.Fatal("no material records loaded")
	}
	materialKeys := make(map[string]bool)
	questionKeys := make(map[string]bool)
	checkQuestionKey := func(question model.Question) {
		if questionKeys[*question.QuestionKey] {
			t.Fatalf(
				"duplicate question key %q",
				*question.QuestionKey,
			)
		}
		questionKeys[*question.QuestionKey] = true
	}
	for _, material := range materials {
		// Language and level are stamped from the file path; ID,
		// content_id, and created_at belong to the database.
		if !strings.HasPrefix(
			material.MaterialKey,
			japanese+":",
		) || material.Category == "" || material.Title == "" || !validDifficulty(material.Difficulty) ||
			material.Language != japanese ||
			material.ProficiencyLevel == "" ||
			material.ID != 0 ||
			material.ContentID != nil ||
			!material.CreatedAt.IsZero() {
			t.Fatalf(
				"invalid material record %q",
				material.MaterialKey,
			)
		}
		assertMaterialPayload(
			t,
			*material,
		)
		if materialKeys[material.MaterialKey] {
			t.Fatalf(
				"duplicate material key %q",
				material.MaterialKey,
			)
		}
		materialKeys[material.MaterialKey] = true
		for _, question := range material.Questions {
			assertQuestionRecord(
				t,
				question,
			)
			checkQuestionKey(question)
		}
	}
	for _, question := range looseQuestions {
		assertQuestionRecord(
			t,
			*question,
		)
		if question.Category != model.CategoryListening {
			t.Fatalf(
				"material-less question %q must be listening, got %q",
				*question.QuestionKey,
				question.Category,
			)
		}
		checkQuestionKey(*question)
	}
}

// validDifficulty mirrors the materials/questions CHECK constraint in
// migrations/001_init.sql, so a bad value fails here instead of mid-seed.
func validDifficulty(difficulty int) bool {
	return difficulty >= 1 && difficulty <= 10
}

// requiredPayloadKeys lists, per category, the payload keys the bot's study
// cards read. Each must hold a non-empty string or a non-empty array. A new
// category must add its row here.
var requiredPayloadKeys = map[model.MaterialCategory][]string{
	model.MaterialCategoryKana:       {"kana", "romaji", "script"},
	model.MaterialCategoryVocabulary: {"kana", "kanji", "meaning_ko", "part_of_speech"},
	model.MaterialCategoryGrammar: {
		"pattern",
		"meaning_ko",
		"explanation_ko",
		"example",
		"example_reading",
		"translation_ko",
	},
	model.MaterialCategoryReading: {"passage", "reading", "key_vocabulary"},
}

func assertMaterialPayload(
	t *testing.T,
	material materialRecord,
) {
	t.Helper()
	keys, ok := requiredPayloadKeys[material.Category]
	if !ok {
		t.Fatalf(
			"material %q: no payload rules for category %q",
			material.MaterialKey,
			material.Category,
		)
	}
	var payload map[string]any
	if err := json.Unmarshal(
		material.Payload,
		&payload,
	); err != nil {
		t.Fatalf(
			"material %q payload: %v",
			material.MaterialKey,
			err,
		)
	}
	for _, key := range keys {
		var present bool
		switch value := payload[key].(type) {
		case string:
			present = value != ""
		case []any:
			present = len(value) > 0
		}
		if !present {
			t.Fatalf(
				"material %q payload has no %q: %s",
				material.MaterialKey,
				key,
				material.Payload,
			)
		}
	}
}

// TestKanjiNotes checks the optional vocabulary "kanji_notes" (ADR-073). The
// key is omitted rather than null or empty, notes decode strictly, and each
// kanji's note is identical in every word that uses it, so a fix made in one
// word but not the others fails here.
func TestKanjiNotes(t *testing.T) {
	t.Parallel()

	materials, _, err := loadRecords(japanese)
	if err != nil {
		t.Fatalf(
			"loadRecords: %v",
			err,
		)
	}
	noteByChar := make(map[string]string)
	keyByChar := make(map[string]string)
	for _, material := range materials {
		if material.Category != model.MaterialCategoryVocabulary {
			continue
		}
		var payload struct {
			Kanji      string           `json:"kanji"`
			KanjiNotes *json.RawMessage `json:"kanji_notes"`
		}
		if err := json.Unmarshal(
			material.Payload,
			&payload,
		); err != nil {
			t.Fatalf(
				"material %q payload: %v",
				material.MaterialKey,
				err,
			)
		}
		if payload.KanjiNotes == nil {
			continue
		}
		var notes []model.KanjiNote
		decoder := json.NewDecoder(bytes.NewReader(*payload.KanjiNotes))
		decoder.DisallowUnknownFields()
		if err := decoder.Decode(&notes); err != nil || len(notes) == 0 {
			t.Fatalf(
				"material %q kanji_notes must be a non-empty note list: %v",
				material.MaterialKey,
				err,
			)
		}
		seen := make(map[string]bool)
		for _, note := range notes {
			if utf8.RuneCountInString(note.Char) != 1 || !strings.Contains(
				payload.Kanji,
				note.Char,
			) || seen[note.Char] {
				t.Fatalf(
					"material %q: note char %q must be one kanji of %q, once",
					material.MaterialKey,
					note.Char,
					payload.Kanji,
				)
			}
			seen[note.Char] = true
			if problem := kanjiNoteProblem(note); problem != "" {
				t.Fatalf(
					"material %q kanji %q: %s",
					material.MaterialKey,
					note.Char,
					problem,
				)
			}
			encoded, _ := json.Marshal(note)
			if previous, ok := noteByChar[note.Char]; ok && previous != string(encoded) {
				t.Fatalf(
					"kanji %q has different notes in %q and %q",
					note.Char,
					keyByChar[note.Char],
					material.MaterialKey,
				)
			}
			noteByChar[note.Char] = string(encoded)
			keyByChar[note.Char] = material.MaterialKey
		}
	}
}

// kanjiNoteProblem returns why a note breaks the schema rules, or "".
func kanjiNoteProblem(note model.KanjiNote) string {
	if !note.Type.Valid() || strings.TrimSpace(note.ExplanationKo) == "" {
		return "needs a valid type and explanation_ko"
	}
	var sound *model.KanjiPart
	hasForm := false
	for i, part := range note.Parts {
		if utf8.RuneCountInString(part.Part) != 1 || !part.Role.Valid() {
			return "each part needs one character and a valid role"
		}
		switch part.Role {
		case model.KanjiPartSound:
			if sound != nil || part.GlossKo != "" {
				return "at most one sound part, without gloss_ko"
			}
			sound = &note.Parts[i]
		default:
			if part.GlossKo == "" || part.On != "" {
				return "meaning and form parts need gloss_ko and no on"
			}
			hasForm = hasForm || part.Role == model.KanjiPartForm
		}
	}
	if note.Type == model.KanjiNotePhonoSemantic {
		if sound == nil && !hasForm {
			return "phono_semantic needs a sound part, or a form part when simplified"
		}
	} else if sound != nil {
		return "only phono_semantic may have a sound part"
	}
	return ""
}

// TestKanjiReadings checks the optional vocabulary "kanji_readings"
// (ADR-074). Entries decode strictly and follow the word's kanji in order, and
// each kanji lists the same on'yomi and 훈음 (ADR-076) in every word, so its
// title and "이 한자의 음독" line never change between cards.
func TestKanjiReadings(t *testing.T) {
	t.Parallel()

	materials, _, err := loadRecords(japanese)
	if err != nil {
		t.Fatalf(
			"loadRecords: %v",
			err,
		)
	}
	kanjiLevelByChar := make(map[string]string)
	keyByChar := make(map[string]string)
	for _, material := range materials {
		if material.Category != model.MaterialCategoryVocabulary {
			continue
		}
		var payload struct {
			Kanji         string           `json:"kanji"`
			KanjiReadings *json.RawMessage `json:"kanji_readings"`
		}
		if err := json.Unmarshal(
			material.Payload,
			&payload,
		); err != nil {
			t.Fatalf(
				"material %q payload: %v",
				material.MaterialKey,
				err,
			)
		}
		if payload.KanjiReadings == nil {
			continue
		}
		var readings []model.KanjiReading
		decoder := json.NewDecoder(bytes.NewReader(*payload.KanjiReadings))
		decoder.DisallowUnknownFields()
		if err := decoder.Decode(&readings); err != nil || len(readings) == 0 {
			t.Fatalf(
				"material %q kanji_readings must be a non-empty list: %v",
				material.MaterialKey,
				err,
			)
		}
		rest := payload.Kanji
		for _, reading := range readings {
			at := strings.Index(
				rest,
				reading.Text,
			)
			if at < 0 {
				t.Fatalf(
					"material %q: reading text %q is not next in %q",
					material.MaterialKey,
					reading.Text,
					payload.Kanji,
				)
			}
			rest = rest[at+len(reading.Text):]
			if problem := kanjiReadingProblem(reading); problem != "" {
				t.Fatalf(
					"material %q reading %q: %s",
					material.MaterialKey,
					reading.Text,
					problem,
				)
			}
			if reading.Type == model.KanjiReadingJukuji {
				continue
			}
			kanjiLevel := strings.Join(
				reading.On,
				"·",
			) + " / " + reading.HunEum
			if previous, ok := kanjiLevelByChar[reading.Text]; ok && previous != kanjiLevel {
				t.Fatalf(
					"kanji %q lists on'yomi / 훈음 %q in %q but %q in %q",
					reading.Text,
					previous,
					keyByChar[reading.Text],
					kanjiLevel,
					material.MaterialKey,
				)
			}
			kanjiLevelByChar[reading.Text] = kanjiLevel
			keyByChar[reading.Text] = material.MaterialKey
		}
	}
}

// kanjiReadingProblem returns why a reading breaks the schema rules, or "".
func kanjiReadingProblem(reading model.KanjiReading) string {
	if !reading.Type.Valid() || reading.Reading == "" || strings.TrimFunc(
		reading.Reading,
		isHiragana,
	) != "" {
		return "needs a valid type and a hiragana reading"
	}
	if strings.TrimSpace(reading.HunEum) == "" {
		return "needs a 훈음"
	}
	for _, on := range reading.On {
		if on == "" || strings.TrimFunc(
			on,
			isKatakana,
		) != "" {
			return "each on'yomi must be katakana"
		}
	}
	single := utf8.RuneCountInString(reading.Text) == 1
	switch reading.Type {
	case model.KanjiReadingJukuji:
		if single || len(reading.On) > 0 {
			return "a jukuji group spans several characters and has no on'yomi"
		}
	case model.KanjiReadingOn:
		if !single || len(reading.On) == 0 {
			return "an on reading is one kanji that lists its on'yomi"
		}
	default:
		if !single {
			return "a kun reading is one kanji"
		}
	}
	return ""
}

func isHiragana(r rune) bool {
	return r >= 'ぁ' && r <= 'ゖ'
}

func isKatakana(r rune) bool {
	return r >= 'ァ' && r <= 'ヶ'
}

func assertQuestionRecord(
	t *testing.T,
	question model.Question,
) {
	t.Helper()
	// Language and level are stamped from the file path; the remaining
	// pointer fields and ID belong to the database or the seeder.
	if question.QuestionKey == nil || !strings.HasPrefix(
		*question.QuestionKey,
		japanese+":",
	) || question.Skill == nil || *question.Skill == "" || question.Type == "" || question.Category == "" ||
		question.Prompt == "" ||
		question.CorrectAnswer == "" ||
		!validDifficulty(question.Difficulty) ||
		question.Language != japanese ||
		question.ProficiencyLevel == "" ||
		question.ID != 0 ||
		question.ContentID != nil ||
		question.MaterialID != nil ||
		question.AudioPath != nil ||
		question.AudioFileID != nil ||
		!question.CreatedAt.IsZero() {
		t.Fatalf(
			"invalid question record: %+v",
			question,
		)
	}
	var options []string
	if err := json.Unmarshal(
		question.Options,
		&options,
	); err != nil {
		t.Fatalf(
			"question %q options: %v",
			*question.QuestionKey,
			err,
		)
	}
	switch question.Type {
	case model.QuestionMultipleChoice, model.QuestionReadingComp, model.QuestionListening:
		distinct := make(map[string]bool)
		for _, option := range options {
			distinct[option] = true
		}
		if len(options) != 4 || len(distinct) != 4 || !distinct[question.CorrectAnswer] {
			t.Fatalf(
				"question %q needs 4 distinct options including answer %q, got %v",
				*question.QuestionKey,
				question.CorrectAnswer,
				options,
			)
		}
	case model.QuestionWordOrder:
		if len(options) < 2 || strings.Join(
			options,
			"",
		) != question.CorrectAnswer {
			t.Fatalf(
				"word-order question %q chunks do not join to the answer",
				*question.QuestionKey,
			)
		}
	}
	if question.Category == model.CategoryListening &&
		(question.AudioScript == nil || *question.AudioScript == "") {
		t.Fatalf(
			"listening question %q has no audio script",
			*question.QuestionKey,
		)
	}
}

func questionKey(key string) *string {
	return &key
}

func TestCollectQuestionsLinksNestedQuestions(t *testing.T) {
	t.Parallel()

	vocab := model.Question{
		QuestionKey:   questionKey("ja:question:n4:sample_vocab"),
		Skill:         model.SkillPtr(model.SkillVocabKanjiReading),
		Type:          model.QuestionFillBlank,
		Category:      model.CategoryVocabulary,
		Prompt:        "「経験」의 읽기를 쓰세요.",
		Options:       []byte(`[]`),
		CorrectAnswer: "けいけん",
		Difficulty:    2,
	}
	listening := model.Question{
		QuestionKey:   questionKey("ja:listening:n4:sample_listening"),
		Skill:         model.SkillPtr(model.SkillListeningTask),
		Type:          model.QuestionListening,
		Category:      model.CategoryListening,
		Prompt:        "몇 시에 갑니까?",
		CorrectAnswer: "9시",
		Difficulty:    1,
	}
	materials := []*materialRecord{
		{Material: model.Material{ID: 42, MaterialKey: "ja:vocab:n4_word_0001"}, Questions: []model.Question{vocab}},
		{Material: model.Material{ID: 43, MaterialKey: "ja:vocab:n4_word_0002"}},
	}
	questions := collectQuestions(
		materials,
		[]*model.Question{&listening},
	)
	if len(questions) != 2 {
		t.Fatalf(
			"question count = %d, want 2",
			len(questions),
		)
	}
	if nested := questions[0]; *nested.QuestionKey != *vocab.QuestionKey || nested.MaterialID == nil ||
		*nested.MaterialID != 42 {
		t.Fatalf(
			"nested question not linked to its material: %+v",
			nested,
		)
	}
	if standalone := questions[1]; *standalone.QuestionKey != *listening.QuestionKey ||
		standalone.MaterialID != nil {
		t.Fatalf(
			"material-less question linked to a material: %+v",
			standalone,
		)
	}
}

func TestParseLanguageArg(t *testing.T) {
	t.Parallel()

	languages := []string{japanese}
	language, err := parseLanguageArg(
		[]string{japanese},
		languages,
	)
	if err != nil || language != japanese {
		t.Fatalf(
			"parseLanguageArg(%q) = %q, %v; want %q, nil",
			japanese,
			language,
			err,
			japanese,
		)
	}
	// Missing, extra, unknown and non-exact arguments all fail before any
	// loading.
	for _, args := range [][]string{
		nil,
		{japanese, "n5"},
		{""},
		{"el"},
		{"JA"},
		{" ja"},
	} {
		if _, err := parseLanguageArg(
			args,
			languages,
		); err == nil {
			t.Fatalf(
				"parseLanguageArg(%q) error = nil, want error",
				args,
			)
		}
	}
}
