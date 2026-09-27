package repository

import (
	"context"
	"reflect"
	"testing"
	"time"

	"github.com/lsj/copylingo/internal/model"
)

func TestStudySessionFromRowMapsParentSession(t *testing.T) {
	now := time.Now()
	startedAt := now.Add(-time.Minute)
	completedAt := now

	session := studySessionFromRow(studySessionWithStateRow{
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

func TestStudyMaterialPreferenceCompletionPostgres(t *testing.T) {
	tx := preferenceTestTransaction(t)
	ctx := context.Background()
	for _, query := range []string{
		`INSERT INTO materials (id,material_key) SELECT id,'material-'||id FROM generate_series(1,5) id`,
		`INSERT INTO user_material_preferences (user_id,material_id,review_mode,next_check_at,check_interval_days,updated_at)
		 SELECT 42,id,'maintenance',NOW()-INTERVAL '1 day',60,NOW()-INTERVAL '2 days' FROM materials`,
		`UPDATE user_material_preferences SET review_mode='excluded',next_check_at=NULL WHERE material_id=2`,
		`UPDATE user_material_preferences SET next_check_at=NOW()+INTERVAL '1 day' WHERE material_id=3`,
		`UPDATE user_material_preferences SET updated_at=NOW()-INTERVAL '30 minutes' WHERE material_id=4`,
		`INSERT INTO user_material_preferences (user_id,material_id,review_mode,next_check_at) VALUES (99,1,'maintenance',NOW()-INTERVAL '1 day')`,
		`INSERT INTO sessions (id,user_id,mode,created_at) VALUES (1,42,'study',NOW()-INTERVAL '1 hour')`,
		`INSERT INTO session_materials (id,session_id,material_id) SELECT id,1,id FROM materials`,
	} {
		mustPreferenceExec(t, tx, query)
	}
	studied := time.Now()
	state := &model.StudyActiveSessionState{Session: model.Session{ID: 1, UserID: 42}}
	for _, id := range []int{1, 2, 3, 4, 5} {
		at := &studied
		if id == 5 {
			at = nil
		}
		state.Items = append(state.Items, model.StudySessionMaterial{
			SessionMaterial: model.SessionMaterial{ID: id, SessionID: 1, MaterialID: id, StudiedAt: at},
			Material:        model.Material{ID: id},
		})
	}
	flushed, err := markStudySessionCompleted(ctx, tx, state)
	if err != nil || !flushed {
		t.Fatalf("mark complete: %v, %v", flushed, err)
	}
	if err := flushStudySessionMaterials(ctx, tx, state.Items); err != nil {
		t.Fatal(err)
	}
	if err := flushUserMaterialProgress(ctx, tx, state); err != nil {
		t.Fatal(err)
	}
	if err := flushStudyMaterialPreferences(ctx, tx, 1); err != nil {
		t.Fatal(err)
	}
	repo := &MaterialPreferenceRepository{db: tx}
	got, err := repo.Get(ctx, 42, 1)
	if err != nil || got.CheckIntervalDays != 60 {
		t.Fatalf("study granted mastery: %+v, %v", got, err)
	}
	var delta float64
	if err := tx.GetContext(
		ctx,
		&delta,
		`SELECT EXTRACT(EPOCH FROM p.next_check_at-s.completed_at)/86400 FROM user_material_preferences p CROSS JOIN sessions s WHERE p.user_id=42 AND p.material_id=1 AND s.id=1`,
	); err != nil {
		t.Fatal(err)
	}
	if delta != 60 {
		t.Fatalf("study interval=%v, want unchanged 60", delta)
	}
	for _, id := range []int{2, 3, 4, 5} {
		var untouched bool
		if err := tx.GetContext(
			ctx,
			&untouched,
			`SELECT updated_at < NOW() FROM user_material_preferences WHERE user_id=42 AND material_id=$1`,
			id,
		); err != nil {
			t.Fatal(err)
		}
		if !untouched {
			t.Fatalf("study changed excluded/stale/unviewed preference %d", id)
		}
	}
	other, err := repo.Get(ctx, 99, 1)
	if err != nil || other.CheckIntervalDays != 30 || other.NextCheckAt == nil || !other.NextCheckAt.Before(studied) {
		t.Fatalf("other user changed: %+v, %v", other, err)
	}
	flushed, err = markStudySessionCompleted(ctx, tx, state)
	if err != nil || flushed {
		t.Fatalf("duplicate study completion: %v, %v", flushed, err)
	}
	if err := flushStudyMaterialPreferences(ctx, tx, 1); err != nil {
		t.Fatal(err)
	}
	after, err := repo.Get(ctx, 42, 1)
	if err != nil || !reflect.DeepEqual(got, after) {
		t.Fatalf("study replay changed preference: %+v -> %+v, %v", got, after, err)
	}
}
