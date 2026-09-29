package repository

import (
	"context"
	"os"
	"reflect"
	"strings"
	"testing"

	"github.com/jmoiron/sqlx"
	_ "github.com/lib/pq"

	"github.com/lsj/copylingo/internal/model"
)

func TestWithinTxRollsBackWhenStudyMaterialInsertFails(t *testing.T) {
	databaseURL := os.Getenv("COPYLINGO_TEST_DATABASE_URL")
	if databaseURL == "" {
		t.Skip("COPYLINGO_TEST_DATABASE_URL is not set")
	}

	db, err := sqlx.Open(
		"postgres",
		databaseURL,
	)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = db.Close() })
	db.SetMaxOpenConns(1)
	ctx := context.Background()
	for _, statement := range []string{
		`CREATE TEMP TABLE sessions (
			id SERIAL PRIMARY KEY, user_id BIGINT NOT NULL, type TEXT NOT NULL,
			mode TEXT NOT NULL, status TEXT NOT NULL, total_questions INT NOT NULL
		)`,
		`CREATE TEMP TABLE session_materials (
			session_id INT NOT NULL REFERENCES sessions(id),
			material_id INT NOT NULL CHECK (material_id > 0), material_order INT NOT NULL
		)`,
	} {
		if _, err := db.ExecContext(
			ctx,
			statement,
		); err != nil {
			t.Fatal(err)
		}
	}

	repo := NewSessionRepository(db)
	createStudySession := func(
		session *model.Session,
		materialIDs []int,
	) error {
		var sessionID int
		err := WithinTx(
			ctx,
			db,
			func(tx *sqlx.Tx) error {
				var err error
				sessionID, err = repo.CreateSessionInTx(
					ctx,
					tx,
					session,
				)
				if err != nil {
					return err
				}
				return repo.CreateSessionMaterialsInTx(
					ctx,
					tx,
					sessionID,
					materialIDs,
				)
			},
		)
		if err != nil {
			return err
		}
		session.ID = sessionID
		return nil
	}
	success := &model.Session{UserID: 42, Type: model.SessionStudy, Mode: model.SessionModeStudy,
		Status: model.SessionPending, TotalQuestions: 2}
	if err := createStudySession(
		success,
		[]int{10, 11},
	); err != nil {
		t.Fatal(err)
	}
	var savedMaterialIDs []int
	if err := db.SelectContext(
		ctx,
		&savedMaterialIDs,
		`SELECT material_id FROM session_materials WHERE session_id = $1 ORDER BY material_order`,
		success.ID,
	); err != nil {
		t.Fatal(err)
	}
	if success.ID == 0 || !reflect.DeepEqual(
		savedMaterialIDs,
		[]int{10, 11},
	) {
		t.Fatalf(
			"saved session ID=%d, material IDs=%v",
			success.ID,
			savedMaterialIDs,
		)
	}

	// The second material violates the constraint; the parent row must disappear too.
	failure := &model.Session{UserID: 43, Type: model.SessionStudy, Mode: model.SessionModeStudy,
		Status: model.SessionPending, TotalQuestions: 2}
	if err := createStudySession(
		failure,
		[]int{20, -1},
	); err == nil {
		t.Fatal("expected material insert failure")
	}
	if failure.ID != 0 {
		t.Fatalf(
			"failed session ID = %d, want 0",
			failure.ID,
		)
	}
	var remaining int
	if err := db.GetContext(
		ctx,
		&remaining,
		`SELECT COUNT(*) FROM sessions WHERE user_id = $1`,
		failure.UserID,
	); err != nil {
		t.Fatal(err)
	}
	if remaining != 0 {
		t.Fatalf(
			"sessions left after rollback = %d, want 0",
			remaining,
		)
	}
}

func TestGetOldestUnfinishedQueryPrioritizesActiveBacklog(t *testing.T) {
	for _, want := range []string{
		"user_id = $1",
		"status IN ('in_progress', 'pending')",
		"CASE WHEN status = 'in_progress' THEN 0 ELSE 1 END",
		"created_at ASC",
		"LIMIT 1",
	} {
		if !strings.Contains(
			getOldestUnfinishedQuery,
			want,
		) {
			t.Fatalf(
				"getOldestUnfinishedQuery does not contain %q:\n%s",
				want,
				getOldestUnfinishedQuery,
			)
		}
	}
	if strings.Contains(
		getOldestUnfinishedQuery,
		"expired",
	) {
		t.Fatalf(
			"getOldestUnfinishedQuery must exclude expired sessions through status filter:\n%s",
			getOldestUnfinishedQuery,
		)
	}
}
