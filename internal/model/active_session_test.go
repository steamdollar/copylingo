package model

import (
	"testing"
)

// boolPtr is a local helper for building *bool fields.
func boolPtr(b bool) *bool { return &b }

// activeStateWith builds an ActiveSessionState from is_correct markers.
// nil = unanswered, true/false = answered correct/wrong. QuestionID is index+1.
func activeStateWith(marks ...*bool) *ActiveSessionState {
	items := make([]ActiveSessionQuestion, 0, len(marks))
	for i, m := range marks {
		items = append(items, ActiveSessionQuestion{
			SessionQuestion: SessionQuestion{
				QuestionID: i + 1,
				IsCorrect:  m,
			},
		})
	}
	return &ActiveSessionState{Items: items}
}

func TestActiveSessionState_RecountAnswered(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name  string
		marks []*bool
		want  int
	}{
		{"all unanswered", []*bool{nil, nil}, 0},
		{"mixed", []*bool{boolPtr(true), nil, boolPtr(false)}, 2},
		{"all answered", []*bool{boolPtr(true), boolPtr(false)}, 2},
		{"empty", nil, 0},
	}
	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			s := activeStateWith(tt.marks...)
			if got := s.RecountAnswered(); got != tt.want {
				t.Fatalf("RecountAnswered() = %d, want %d", got, tt.want)
			}
			// RecountAnswered must also persist the count onto the struct.
			if s.AnsweredCount != tt.want {
				t.Fatalf("AnsweredCount = %d, want %d", s.AnsweredCount, tt.want)
			}
		})
	}
}

func TestActiveSessionState_CurrentItemByQuestionID(t *testing.T) {
	t.Parallel()

	s := activeStateWith(nil, nil, nil) // QuestionIDs 1,2,3
	s.CurrentIndex = 1                  // current question is QuestionID 2

	t.Run("matches current index", func(t *testing.T) {
		item, idx, ok := s.CurrentItemByQuestionID(2)
		if !ok || idx != 1 || item == nil || item.SessionQuestion.QuestionID != 2 {
			t.Fatalf("CurrentItemByQuestionID(2) = %v, %d, %v", item, idx, ok)
		}
	})

	t.Run("question id not at current index", func(t *testing.T) {
		item, idx, ok := s.CurrentItemByQuestionID(3)
		if ok || idx != -1 || item != nil {
			t.Fatalf("CurrentItemByQuestionID(3) = %v, %d, %v; want nil,-1,false", item, idx, ok)
		}
	})

	t.Run("current index out of range", func(t *testing.T) {
		s2 := activeStateWith(nil)
		s2.CurrentIndex = 5
		item, idx, ok := s2.CurrentItemByQuestionID(1)
		if ok || idx != -1 || item != nil {
			t.Fatalf("out of range = %v, %d, %v; want nil,-1,false", item, idx, ok)
		}
	})
}

func TestActiveSessionState_ItemByQuestionID(t *testing.T) {
	t.Parallel()

	s := activeStateWith(nil, nil, nil)

	t.Run("found regardless of current index", func(t *testing.T) {
		item, ok := s.ItemByQuestionID(3)
		if !ok || item == nil || item.SessionQuestion.QuestionID != 3 {
			t.Fatalf("ItemByQuestionID(3) = %v, %v", item, ok)
		}
	})

	t.Run("not found", func(t *testing.T) {
		item, ok := s.ItemByQuestionID(99)
		if ok || item != nil {
			t.Fatalf("ItemByQuestionID(99) = %v, %v; want nil,false", item, ok)
		}
	})
}

func TestActiveSessionState_ItemAt(t *testing.T) {
	t.Parallel()

	s := activeStateWith(nil, nil)

	tests := []struct {
		name   string
		idx    int
		wantOK bool
	}{
		{"first", 0, true},
		{"last", 1, true},
		{"negative", -1, false},
		{"past end", 2, false},
	}
	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			item, ok := s.ItemAt(tt.idx)
			if ok != tt.wantOK {
				t.Fatalf("ItemAt(%d) ok = %v, want %v", tt.idx, ok, tt.wantOK)
			}
			if tt.wantOK && item == nil {
				t.Fatalf("ItemAt(%d) returned nil item with ok=true", tt.idx)
			}
			if !tt.wantOK && item != nil {
				t.Fatalf("ItemAt(%d) returned non-nil item with ok=false", tt.idx)
			}
		})
	}
}

func TestActiveSessionState_NextUnansweredIndex(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name  string
		marks []*bool
		want  int
	}{
		{"first unanswered", []*bool{nil, nil}, 0},
		{"second unanswered", []*bool{boolPtr(true), nil}, 1},
		{"all answered returns len", []*bool{boolPtr(true), boolPtr(false)}, 2},
		{"empty returns 0", nil, 0},
	}
	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			s := activeStateWith(tt.marks...)
			if got := s.NextUnansweredIndex(); got != tt.want {
				t.Fatalf("NextUnansweredIndex() = %d, want %d", got, tt.want)
			}
		})
	}
}

func TestActiveSessionState_CorrectCount(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name  string
		marks []*bool
		want  int
	}{
		{"none correct", []*bool{boolPtr(false), nil}, 0},
		{"some correct", []*bool{boolPtr(true), boolPtr(false), boolPtr(true)}, 2},
		{"unanswered not counted", []*bool{nil, nil}, 0},
	}
	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			s := activeStateWith(tt.marks...)
			if got := s.CorrectCount(); got != tt.want {
				t.Fatalf("CorrectCount() = %d, want %d", got, tt.want)
			}
		})
	}
}

func TestActiveSessionState_WrongAnswers(t *testing.T) {
	t.Parallel()

	t.Run("collects only wrong answers in order", func(t *testing.T) {
		s := activeStateWith(boolPtr(true), boolPtr(false), nil, boolPtr(false))
		wrong := s.WrongAnswers()
		if len(wrong) != 2 {
			t.Fatalf("WrongAnswers() len = %d, want 2", len(wrong))
		}
		if wrong[0].SessionQuestion.QuestionID != 2 || wrong[1].SessionQuestion.QuestionID != 4 {
			t.Fatalf("WrongAnswers() ids = %d,%d; want 2,4",
				wrong[0].SessionQuestion.QuestionID, wrong[1].SessionQuestion.QuestionID)
		}
	})

	t.Run("none wrong returns empty non-nil slice", func(t *testing.T) {
		s := activeStateWith(boolPtr(true), nil)
		wrong := s.WrongAnswers()
		if wrong == nil {
			t.Fatal("WrongAnswers() = nil, want empty slice")
		}
		if len(wrong) != 0 {
			t.Fatalf("WrongAnswers() len = %d, want 0", len(wrong))
		}
	})
}
