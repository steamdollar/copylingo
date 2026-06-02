package observability

import (
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"
)

func TestDailyWriterConcurrentAppend(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	now := time.Date(2026, time.June, 1, 10, 0, 0, 0, time.FixedZone("KST", 9*60*60))
	writer, err := NewDailyWriter(DailyWriterOptions{
		Dir:           dir,
		RetentionDays: 30,
		Location:      now.Location(),
		Now:           func() time.Time { return now },
	})
	if err != nil {
		t.Fatalf("NewDailyWriter() error = %v", err)
	}
	defer writer.Close()

	const writes = 50
	var wg sync.WaitGroup
	for i := 0; i < writes; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			if _, err := writer.Write([]byte("{\"event\":\"test\"}\n")); err != nil {
				t.Errorf("Write() error = %v", err)
			}
		}()
	}
	wg.Wait()

	body, err := os.ReadFile(filepath.Join(dir, "copylingo-2026-06-01.jsonl"))
	if err != nil {
		t.Fatalf("ReadFile() error = %v", err)
	}
	if got := strings.Count(string(body), "\n"); got != writes {
		t.Fatalf("log line count = %d, want %d", got, writes)
	}
}

func TestDailyWriterRotatesByConfiguredLocation(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	location := time.FixedZone("KST", 9*60*60)
	now := time.Date(2026, time.June, 1, 23, 59, 0, 0, location)
	writer, err := NewDailyWriter(DailyWriterOptions{
		Dir:           dir,
		RetentionDays: 30,
		Location:      location,
		Now:           func() time.Time { return now },
	})
	if err != nil {
		t.Fatalf("NewDailyWriter() error = %v", err)
	}
	defer writer.Close()

	if _, err := writer.Write([]byte("first\n")); err != nil {
		t.Fatalf("first Write() error = %v", err)
	}
	now = now.Add(2 * time.Minute)
	if _, err := writer.Write([]byte("second\n")); err != nil {
		t.Fatalf("second Write() error = %v", err)
	}

	assertFileContent(t, filepath.Join(dir, "copylingo-2026-06-01.jsonl"), "first\n")
	assertFileContent(t, filepath.Join(dir, "copylingo-2026-06-02.jsonl"), "second\n")
}

func TestDailyWriterCleansExpiredMatchingFilesOnly(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	for name, body := range map[string]string{
		"copylingo-2026-04-30.jsonl": "expired",
		"copylingo-2026-05-02.jsonl": "cutoff-kept",
		"notes.jsonl":                "unrelated",
	} {
		if err := os.WriteFile(filepath.Join(dir, name), []byte(body), 0o644); err != nil {
			t.Fatalf("WriteFile(%s) error = %v", name, err)
		}
	}
	now := time.Date(2026, time.June, 1, 10, 0, 0, 0, time.FixedZone("KST", 9*60*60))
	writer, err := NewDailyWriter(DailyWriterOptions{
		Dir:           dir,
		RetentionDays: 30,
		Location:      now.Location(),
		Now:           func() time.Time { return now },
	})
	if err != nil {
		t.Fatalf("NewDailyWriter() error = %v", err)
	}
	defer writer.Close()

	if _, err := os.Stat(filepath.Join(dir, "copylingo-2026-04-30.jsonl")); !os.IsNotExist(err) {
		t.Fatalf("expired file still exists, stat error = %v", err)
	}
	for _, name := range []string{"copylingo-2026-05-02.jsonl", "notes.jsonl"} {
		if _, err := os.Stat(filepath.Join(dir, name)); err != nil {
			t.Fatalf("expected %s to remain: %v", name, err)
		}
	}
}

func assertFileContent(t *testing.T, path, want string) {
	t.Helper()
	body, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile(%s) error = %v", path, err)
	}
	if got := string(body); got != want {
		t.Fatalf("ReadFile(%s) = %q, want %q", path, got, want)
	}
}
