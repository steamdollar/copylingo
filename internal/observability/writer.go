package observability

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"sync"
	"time"
)

const warningInterval = time.Minute

var dailyLogPattern = regexp.MustCompile(`^copylingo-(\d{4}-\d{2}-\d{2})\.jsonl$`)

type DailyWriterOptions struct {
	Dir           string
	RetentionDays int
	Location      *time.Location
	Now           func() time.Time
	Stderr        io.Writer
}

// DailyWriter appends logs to a date-scoped JSONL file and rotates at local midnight.
type DailyWriter struct {
	mu            sync.Mutex
	dir           string
	retentionDays int
	location      *time.Location
	now           func() time.Time
	stderr        io.Writer
	file          *os.File
	currentDate   string
	lastWarningAt time.Time
}

func NewDailyWriter(options DailyWriterOptions) (*DailyWriter, error) {
	if options.Dir == "" {
		return nil, fmt.Errorf("log directory is empty")
	}
	if options.RetentionDays < 1 {
		return nil, fmt.Errorf("retention days must be positive")
	}
	if options.Location == nil {
		return nil, fmt.Errorf("location is required")
	}
	if options.Now == nil {
		options.Now = time.Now
	}
	if options.Stderr == nil {
		options.Stderr = os.Stderr
	}

	if err := os.MkdirAll(options.Dir, 0o755); err != nil {
		return nil, fmt.Errorf("create log directory: %w", err)
	}

	writer := &DailyWriter{
		dir:           options.Dir,
		retentionDays: options.RetentionDays,
		location:      options.Location,
		now:           options.Now,
		stderr:        options.Stderr,
	}
	if err := writer.rotateLocked(writer.now().In(writer.location)); err != nil {
		return nil, err
	}
	writer.cleanupLocked(writer.now().In(writer.location))
	return writer, nil
}

func (w *DailyWriter) Write(p []byte) (int, error) {
	w.mu.Lock()
	defer w.mu.Unlock()

	now := w.now().In(w.location)
	if date := now.Format(time.DateOnly); date != w.currentDate {
		if err := w.rotateLocked(now); err != nil {
			w.warnLocked("rotate daily log file: %v", err)
			return len(p), nil
		}
		w.cleanupLocked(now)
	}
	if _, err := w.file.Write(p); err != nil {
		w.warnLocked("append daily log file: %v", err)
		return len(p), nil
	}
	return len(p), nil
}

func (w *DailyWriter) Close() error {
	w.mu.Lock()
	defer w.mu.Unlock()
	if w.file == nil {
		return nil
	}
	return w.file.Close()
}

func (w *DailyWriter) rotateLocked(now time.Time) error {
	date := now.Format(time.DateOnly)
	path := filepath.Join(w.dir, "copylingo-"+date+".jsonl")
	file, err := os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
	if err != nil {
		return fmt.Errorf("open %s: %w", path, err)
	}
	if w.file != nil {
		if err := w.file.Close(); err != nil {
			w.warnLocked("close previous daily log file: %v", err)
		}
	}
	w.file = file
	w.currentDate = date
	return nil
}

func (w *DailyWriter) cleanupLocked(now time.Time) {
	entries, err := os.ReadDir(w.dir)
	if err != nil {
		w.warnLocked("list daily log files: %v", err)
		return
	}

	cutoff := startOfDay(now, w.location).AddDate(0, 0, -w.retentionDays)
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		match := dailyLogPattern.FindStringSubmatch(entry.Name())
		if len(match) != 2 {
			continue
		}
		fileDate, err := time.ParseInLocation(time.DateOnly, match[1], w.location)
		if err != nil || !fileDate.Before(cutoff) {
			continue
		}
		if err := os.Remove(filepath.Join(w.dir, entry.Name())); err != nil {
			w.warnLocked("remove expired daily log file %s: %v", entry.Name(), err)
		}
	}
}

func (w *DailyWriter) warnLocked(format string, args ...any) {
	now := w.now()
	if !w.lastWarningAt.IsZero() && now.Sub(w.lastWarningAt) < warningInterval {
		return
	}
	w.lastWarningAt = now
	fmt.Fprintf(w.stderr, "[copylingo logging] "+format+"\n", args...)
}

func startOfDay(now time.Time, location *time.Location) time.Time {
	local := now.In(location)
	return time.Date(local.Year(), local.Month(), local.Day(), 0, 0, 0, 0, location)
}
