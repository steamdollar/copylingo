package catalog

import (
	"reflect"
	"testing"

	"github.com/lsj/copylingo/internal/model"
)

func TestBuildRecordMaterialsStampsLanguageAndLevel(t *testing.T) {
	t.Parallel()

	records := []MaterialRecord{
		{
			MaterialKey: "ja:vocab:n5_word_0001",
			Category:    model.MaterialCategoryVocabulary,
			Title:       "わたし",
			Difficulty:  2,
			Payload:     []byte(`{"kana":"わたし"}`),
			Questions:   []QuestionRecord{{QuestionKey: "ja:vocab:n5_word_0001:meaning"}},
		},
	}
	got := BuildRecordMaterials(
		Japanese,
		"N5",
		records,
	)
	want := []*model.Material{
		{
			MaterialKey:      "ja:vocab:n5_word_0001",
			Category:         model.MaterialCategoryVocabulary,
			Language:         Japanese,
			ProficiencyLevel: "N5",
			Title:            "わたし",
			Payload:          []byte(`{"kana":"わたし"}`),
			Difficulty:       2,
		},
	}
	if !reflect.DeepEqual(
		got,
		want,
	) {
		t.Fatalf(
			"BuildRecordMaterials() = %+v, want %+v",
			got[0],
			want[0],
		)
	}
}
