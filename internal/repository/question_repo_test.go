package repository

import (
	"strings"
	"testing"

	"github.com/lsj/copylingo/internal/model"
)

func TestBuildQuestionBatchInsertQuery(t *testing.T) {
	materialID := 24
	questionKey := "ja:kana:u3042:recall"
	questions := []*model.Question{
		{
			QuestionKey:      &questionKey,
			MaterialID:       &materialID,
			Type:             model.QuestionFillBlank,
			Skill:            model.SkillPtr(model.SkillKanaRecall),
			Language:         "ja",
			ProficiencyLevel: "N5",
			Category:         "kana",
			Prompt:           "prompt-1",
			Options:          []byte("[]"),
			CorrectAnswer:    "a",
			Explanation:      "exp-1",
			Difficulty:       1,
		},
		{
			Type:             model.QuestionKanaHandwriting,
			Language:         "ja",
			ProficiencyLevel: "N5",
			Category:         "kana",
			Prompt:           "prompt-2",
			Options:          []byte("[]"),
			CorrectAnswer:    "あ",
			Explanation:      "exp-2",
			Difficulty:       1,
		},
	}

	query, args := buildQuestionBatchInsertQuery(questions)

	if !strings.Contains(query, "INSERT INTO questions") {
		t.Fatalf("query = %q, want insert statement", query)
	}
	if !strings.Contains(query, "question_key") {
		t.Fatalf("query = %q, want question_key column", query)
	}
	if !strings.Contains(query, "item_type") {
		t.Fatalf("query = %q, want item_type column", query)
	}
	if !strings.Contains(query, "material_id") {
		t.Fatalf("query = %q, want material_id column", query)
	}
	if !strings.Contains(query, "audio_script") {
		t.Fatalf("query = %q, want audio_script column", query)
	}
	if !strings.Contains(query, "($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15)") {
		t.Fatalf("query = %q, want first placeholder group", query)
	}
	if !strings.Contains(query, "($16, $17, $18, $19, $20, $21, $22, $23, $24, $25, $26, $27, $28, $29, $30)") {
		t.Fatalf("query = %q, want second placeholder group", query)
	}
	if strings.Contains(query, "RETURNING id") {
		t.Fatalf("query = %q, did not expect returning id clause", query)
	}
	if len(args) != 30 {
		t.Fatalf("len(args) = %d, want 30", len(args))
	}
	gotQuestionKey, ok := args[0].(*string)
	if !ok || gotQuestionKey == nil || *gotQuestionKey != questionKey {
		t.Fatalf("args[0] = %#v, want question_key %q", args[0], questionKey)
	}
	gotMaterialID, ok := args[2].(*int)
	if !ok || gotMaterialID == nil || *gotMaterialID != materialID {
		t.Fatalf("args[2] = %#v, want material_id %d", args[2], materialID)
	}
	gotSkill, ok := args[4].(*model.Skill)
	if !ok || gotSkill == nil || *gotSkill != model.SkillKanaRecall {
		t.Fatalf("args[4] = %#v, want %q", args[4], model.SkillKanaRecall)
	}
}

func TestBuildQuestionBatchUpsertQuery(t *testing.T) {
	questionKey := "ja:kana:u3042:reading"
	questions := []*model.Question{
		{
			QuestionKey:      &questionKey,
			Type:             model.QuestionFillBlank,
			Skill:            model.SkillPtr(model.SkillKanaReading),
			Language:         "ja",
			ProficiencyLevel: "N5",
			Category:         "kana",
			Prompt:           "prompt",
			Options:          []byte("[]"),
			CorrectAnswer:    "a",
			Explanation:      "exp",
			Difficulty:       1,
		},
	}

	query, args := buildQuestionBatchUpsertQuery(questions)

	if !strings.Contains(query, "ON CONFLICT (question_key) DO UPDATE SET") {
		t.Fatalf("query = %q, want question_key upsert", query)
	}
	for _, column := range []string{
		"content_id",
		"material_id",
		"type",
		"item_type",
		"language",
		"proficiency_level",
		"category",
		"prompt",
		"options",
		"correct_answer",
		"explanation",
		"audio_path",
		"audio_script",
		"difficulty",
	} {
		want := column + " = EXCLUDED." + column
		if !strings.Contains(query, want) {
			t.Fatalf("query = %q, want update %q", query, want)
		}
	}
	// audio_file_id is set post-hoc (SetAudioFileID), never via the seed upsert.
	if strings.Contains(query, "audio_file_id = EXCLUDED.audio_file_id") {
		t.Fatalf("query = %q, did not expect audio_file_id in upsert", query)
	}
	for _, runtimeColumn := range []string{
		"times_served",
		"times_correct",
		"ease_factor",
		"interval_days",
		"repetitions",
		"next_review_at",
		"last_reviewed_at",
	} {
		if strings.Contains(query, runtimeColumn+" = EXCLUDED."+runtimeColumn) {
			t.Fatalf("query = %q, did not expect runtime column update %q", query, runtimeColumn)
		}
	}
	if len(args) != 15 {
		t.Fatalf("len(args) = %d, want 15", len(args))
	}
}

func TestNewQuestionsQueryPrioritizesStudiedMaterialsWithFallback(t *testing.T) {
	for _, want := range []string{
		"LEFT JOIN user_material_progress ump",
		"ump.material_id = q.material_id",
		"ump.user_id = $1",
		"ump.times_studied > 0",
		"q.next_review_at IS NULL",
		"CASE WHEN ump.material_id IS NOT NULL THEN 0 ELSE 1 END",
		// Listening questions must not be scheduled before their audio exists.
		"q.category <> 'listening' OR q.audio_path IS NOT NULL",
		"q.item_type IS DISTINCT FROM 'vocab_kanji_recall'",
		"candidate.kanji_recall_rank <= $7",
		"LIMIT $6",
	} {
		if !strings.Contains(newQuestionsForStudiedMaterialsQuery, want) {
			t.Fatalf("query = %q, want %q", newQuestionsForStudiedMaterialsQuery, want)
		}
	}
}

func TestDueReviewsQueryPrioritizesStudiedMaterialsWithFallback(t *testing.T) {
	for _, want := range []string{
		"LEFT JOIN user_material_progress ump",
		"ump.material_id = q.material_id",
		"ump.user_id = $1",
		"ump.times_studied > 0",
		"q.next_review_at IS NOT NULL",
		"CASE WHEN ump.material_id IS NOT NULL THEN 0 ELSE 1 END",
		"q.item_type IS DISTINCT FROM 'vocab_kanji_recall'",
		"candidate.kanji_recall_rank <= $3",
		"LIMIT $2",
	} {
		if !strings.Contains(dueReviewsForStudiedMaterialsQuery, want) {
			t.Fatalf("query = %q, want %q", dueReviewsForStudiedMaterialsQuery, want)
		}
	}
}
