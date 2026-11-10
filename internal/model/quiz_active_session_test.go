package model

import (
	"testing"
)

func boolPointer(value bool) *bool { return &value }

// activeStateWith builds state from is_correct markers: nil = unanswered, true/false = answered. QuestionID is index+1.
func activeStateWith(marks ...*bool) *QuizActiveSessionState {
	items := make(
		[]QuizActiveSessionQuestion,
		0,
		len(marks),
	)
	for i, mark := range marks {
		items = append(
			items,
			QuizActiveSessionQuestion{
				SessionQuestion: SessionQuestion{
					QuestionID: i + 1,
					IsCorrect:  mark,
				},
			},
		)
	}
	return &QuizActiveSessionState{Items: items}
}

func TestQuizActiveSessionState_RecountAnswered(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name  string
		marks []*bool
		want  int
	}{
		{"all unanswered", []*bool{nil, nil}, 0},
		{"mixed", []*bool{boolPointer(true), nil, boolPointer(false)}, 2},
		{"all answered", []*bool{boolPointer(true), boolPointer(false)}, 2},
		{"empty", nil, 0},
	}
	for _, tt := range tests {
		tt := tt
		t.Run(
			tt.name,
			func(t *testing.T) {
				t.Parallel()
				state := activeStateWith(tt.marks...)
				if got := state.RecountAnswered(); got != tt.want {
					t.Fatalf(
						"RecountAnswered() = %d, want %d",
						got,
						tt.want,
					)
				}
				// RecountAnswered must also persist the count onto the struct.
				if state.AnsweredCount != tt.want {
					t.Fatalf(
						"AnsweredCount = %d, want %d",
						state.AnsweredCount,
						tt.want,
					)
				}
			},
		)
	}
}

func TestQuizActiveSessionState_CurrentItemByQuestionID(t *testing.T) {
	t.Parallel()

	state := activeStateWith(
		nil,
		nil,
		nil,
	) // QuestionIDs 1,2,3
	state.CurrentIndex = 1 // current question is QuestionID 2

	t.Run(
		"matches current index",
		func(t *testing.T) {
			item, index, ok := state.CurrentItemByQuestionID(2)
			if !ok || index != 1 || item == nil || item.SessionQuestion.QuestionID != 2 {
				t.Fatalf(
					"CurrentItemByQuestionID(2) = %v, %d, %v",
					item,
					index,
					ok,
				)
			}
		},
	)

	t.Run(
		"question id not at current index",
		func(t *testing.T) {
			item, index, ok := state.CurrentItemByQuestionID(3)
			if ok || index != -1 || item != nil {
				t.Fatalf(
					"CurrentItemByQuestionID(3) = %v, %d, %v; want nil,-1,false",
					item,
					index,
					ok,
				)
			}
		},
	)

	t.Run(
		"current index out of range",
		func(t *testing.T) {
			outOfRangeState := activeStateWith(nil)
			outOfRangeState.CurrentIndex = 5
			item, index, ok := outOfRangeState.CurrentItemByQuestionID(1)
			if ok || index != -1 || item != nil {
				t.Fatalf(
					"out of range = %v, %d, %v; want nil,-1,false",
					item,
					index,
					ok,
				)
			}
		},
	)
}

func TestQuizActiveSessionState_ItemByQuestionID(t *testing.T) {
	t.Parallel()

	state := activeStateWith(
		nil,
		nil,
		nil,
	)

	t.Run(
		"found regardless of current index",
		func(t *testing.T) {
			item, ok := state.ItemByQuestionID(3)
			if !ok || item == nil || item.SessionQuestion.QuestionID != 3 {
				t.Fatalf(
					"ItemByQuestionID(3) = %v, %v",
					item,
					ok,
				)
			}
		},
	)

	t.Run(
		"not found",
		func(t *testing.T) {
			item, ok := state.ItemByQuestionID(99)
			if ok || item != nil {
				t.Fatalf(
					"ItemByQuestionID(99) = %v, %v; want nil,false",
					item,
					ok,
				)
			}
		},
	)
}

func TestQuizActiveSessionState_ItemAt(t *testing.T) {
	t.Parallel()

	state := activeStateWith(
		nil,
		nil,
	)

	tests := []struct {
		name   string
		index  int
		wantOK bool
	}{
		{"first", 0, true},
		{"last", 1, true},
		{"negative", -1, false},
		{"past end", 2, false},
	}
	for _, tt := range tests {
		tt := tt
		t.Run(
			tt.name,
			func(t *testing.T) {
				t.Parallel()
				item, ok := state.ItemAt(tt.index)
				if ok != tt.wantOK {
					t.Fatalf(
						"ItemAt(%d) ok = %v, want %v",
						tt.index,
						ok,
						tt.wantOK,
					)
				}
				if tt.wantOK && item == nil {
					t.Fatalf(
						"ItemAt(%d) returned nil item with ok=true",
						tt.index,
					)
				}
				if !tt.wantOK && item != nil {
					t.Fatalf(
						"ItemAt(%d) returned non-nil item with ok=false",
						tt.index,
					)
				}
			},
		)
	}
}

func TestQuizActiveSessionState_NextUnansweredIndex(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name  string
		marks []*bool
		want  int
	}{
		{"first unanswered", []*bool{nil, nil}, 0},
		{"second unanswered", []*bool{boolPointer(true), nil}, 1},
		{"all answered returns len", []*bool{boolPointer(true), boolPointer(false)}, 2},
		{"empty returns 0", nil, 0},
	}
	for _, tt := range tests {
		tt := tt
		t.Run(
			tt.name,
			func(t *testing.T) {
				t.Parallel()
				state := activeStateWith(tt.marks...)
				if got := state.NextUnansweredIndex(); got != tt.want {
					t.Fatalf(
						"NextUnansweredIndex() = %d, want %d",
						got,
						tt.want,
					)
				}
			},
		)
	}
}

func TestQuizActiveSessionState_CorrectCount(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name  string
		marks []*bool
		want  int
	}{
		{"none correct", []*bool{boolPointer(false), nil}, 0},
		{"some correct", []*bool{boolPointer(true), boolPointer(false), boolPointer(true)}, 2},
		{"unanswered not counted", []*bool{nil, nil}, 0},
	}
	for _, tt := range tests {
		tt := tt
		t.Run(
			tt.name,
			func(t *testing.T) {
				t.Parallel()
				state := activeStateWith(tt.marks...)
				if got := state.CorrectCount(); got != tt.want {
					t.Fatalf(
						"CorrectCount() = %d, want %d",
						got,
						tt.want,
					)
				}
			},
		)
	}
}

func TestQuizActiveSessionState_WrongAnswers(t *testing.T) {
	t.Parallel()

	t.Run(
		"collects only wrong answers in order",
		func(t *testing.T) {
			state := activeStateWith(
				boolPointer(true),
				boolPointer(false),
				nil,
				boolPointer(false),
			)
			wrong := state.WrongAnswers()
			if len(wrong) != 2 {
				t.Fatalf(
					"WrongAnswers() len = %d, want 2",
					len(wrong),
				)
			}
			if wrong[0].SessionQuestion.QuestionID != 2 || wrong[1].SessionQuestion.QuestionID != 4 {
				t.Fatalf(
					"WrongAnswers() ids = %d,%d; want 2,4",
					wrong[0].SessionQuestion.QuestionID,
					wrong[1].SessionQuestion.QuestionID,
				)
			}
		},
	)

	t.Run(
		"none wrong returns empty non-nil slice",
		func(t *testing.T) {
			state := activeStateWith(
				boolPointer(true),
				nil,
			)
			wrong := state.WrongAnswers()
			if wrong == nil {
				t.Fatal("WrongAnswers() = nil, want empty slice")
			}
			if len(wrong) != 0 {
				t.Fatalf(
					"WrongAnswers() len = %d, want 0",
					len(wrong),
				)
			}
		},
	)
}
