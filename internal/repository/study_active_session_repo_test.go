package repository

import (
	"testing"
	"time"

	"github.com/lsj/copylingo/internal/model"
)

func TestStudySessionFromRowMapsParentSession(t *testing.T) {
	now := time.Now()
	startedAt := now.Add(-time.Minute)
	completedAt := now

	session := studySessionFromRow(studyActiveSessionRow{
		SessionIDForParent: 77,
		UserID:             123,
		SessionType:        model.SessionStudy,
		Mode:               model.SessionModeStudy,
		Status:             model.SessionCompleted,
		TotalQuestions:     8,
		CorrectCount:       0,
		StartedAt:          &startedAt,
		CompletedAt:        &completedAt,
		SessionCreatedAt:   now.Add(-time.Hour),
	})

	if session.ID != 77 ||
		session.UserID != 123 ||
		session.Type != model.SessionStudy ||
		session.Mode != model.SessionModeStudy ||
		session.Status != model.SessionCompleted ||
		session.TotalQuestions != 8 ||
		session.CorrectCount != 0 ||
		session.StartedAt != &startedAt ||
		session.CompletedAt != &completedAt {
		t.Fatalf("unexpected session mapping: %+v", session)
	}
}
