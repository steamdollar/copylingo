package model

import "testing"

func TestNewUserQuestionProgress(t *testing.T) {
	progress := NewUserQuestionProgress(42, 7)
	if progress.UserID != 42 || progress.QuestionID != 7 {
		t.Fatalf("unexpected identity: %+v", progress)
	}
	if progress.EaseFactor != DefaultQuestionEaseFactor {
		t.Fatalf("EaseFactor = %f, want %f", progress.EaseFactor, DefaultQuestionEaseFactor)
	}
	if progress.TimesServed != 0 || progress.NextReviewAt != nil {
		t.Fatalf("expected untouched progress defaults: %+v", progress)
	}
}
