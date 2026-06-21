package main

import (
	"strings"
	"testing"
)

func TestBuildResetLearningDataQueryPreservesUsers(t *testing.T) {
	t.Parallel()

	query := buildResetLearningDataQuery(learningDataTables)

	if !strings.Contains(query, "TRUNCATE TABLE") {
		t.Fatalf("query = %q, want truncate", query)
	}
	if !strings.Contains(query, "RESTART IDENTITY CASCADE") {
		t.Fatalf("query = %q, want restart identity cascade", query)
	}
	if strings.Contains(query, "users") {
		t.Fatalf("query = %q, must not truncate users", query)
	}
	for _, table := range learningDataTables {
		if !strings.Contains(query, table) {
			t.Fatalf("query = %q, missing table %q", query, table)
		}
	}
}
