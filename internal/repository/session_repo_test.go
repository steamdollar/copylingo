package repository

import (
	"strings"
	"testing"
)

func TestGetOldestUnfinishedQueryPrioritizesActiveBacklog(t *testing.T) {
	for _, want := range []string{
		"user_id = $1",
		"status IN ('in_progress', 'pending')",
		"CASE WHEN status = 'in_progress' THEN 0 ELSE 1 END",
		"created_at ASC",
		"LIMIT 1",
	} {
		if !strings.Contains(getOldestUnfinishedQuery, want) {
			t.Fatalf("getOldestUnfinishedQuery does not contain %q:\n%s", want, getOldestUnfinishedQuery)
		}
	}
	if strings.Contains(getOldestUnfinishedQuery, "expired") {
		t.Fatalf(
			"getOldestUnfinishedQuery must exclude expired sessions through status filter:\n%s",
			getOldestUnfinishedQuery,
		)
	}
}
