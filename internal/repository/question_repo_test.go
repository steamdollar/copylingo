package repository

import (
	"strings"
	"testing"

	"github.com/lsj/copylingo/internal/model"
)

func TestBuildQuestionBatchInsertQuery(t *testing.T) {
	questions := []*model.Question{
		{
			Type:             model.QuestionFillBlank,
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
	if !strings.Contains(query, "($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11)") {
		t.Fatalf("query = %q, want first placeholder group", query)
	}
	if !strings.Contains(query, "($12, $13, $14, $15, $16, $17, $18, $19, $20, $21, $22)") {
		t.Fatalf("query = %q, want second placeholder group", query)
	}
	if strings.Contains(query, "RETURNING id") {
		t.Fatalf("query = %q, did not expect returning id clause", query)
	}
	if len(args) != 22 {
		t.Fatalf("len(args) = %d, want 22", len(args))
	}
}
