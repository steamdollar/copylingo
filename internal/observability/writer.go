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
	Directory     string
	RetentionDays int
	Location      *time.Location
	Now           func() time.Time
	Stderr        io.Writer
}

// DailyWriter appends logs to a date-scoped JSONL file and rotates at local midnight.
type DailyWriter struct {
	mu            sync.Mutex
	directory     string
	retentionDays int
	location      *time.Location
	now           func() time.Time
	stderr        io.Writer
	file          *os.File
	currentDate   string
	lastWarningAt time.Time
}

func NewDailyWriter(options DailyWriterOptions) (*DailyWriter, error) {
	if options.Directory == "" {
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

	if err := os.MkdirAll(
		options.Directory,
		0o755,
	); err != nil {
		return nil, fmt.Errorf(
			"create log directory: %w",
			err,
		)
	}

	writer := &DailyWriter{
		directory:     options.Directory,
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

func (writer *DailyWriter) Write(data []byte) (int, error) {
	writer.mu.Lock()
	defer writer.mu.Unlock()

	now := writer.now().In(writer.location)
	if date := now.Format(time.DateOnly); date != writer.currentDate {
		if err := writer.rotateLocked(now); err != nil {
			writer.warnLocked(
				"rotate daily log file: %v",
				err,
			)
			return len(data), nil
		}
		writer.cleanupLocked(now)
	}
	if _, err := writer.file.Write(data); err != nil {
		writer.warnLocked(
			"append daily log file: %v",
			err,
		)
		return len(data), nil
	}
	return len(data), nil
}

func (writer *DailyWriter) Close() error {
	writer.mu.Lock()
	defer writer.mu.Unlock()
	if writer.file == nil {
		return nil
	}
	return writer.file.Close()
}

func (writer *DailyWriter) rotateLocked(now time.Time) error {
	date := now.Format(time.DateOnly)
	path := filepath.Join(
		writer.directory,
		"copylingo-"+date+".jsonl",
	)
	file, err := os.OpenFile(
		path,
		os.O_APPEND|os.O_CREATE|os.O_WRONLY,
		0o644,
	)
	if err != nil {
		return fmt.Errorf(
			"open %s: %w",
			path,
			err,
		)
	}
	if writer.file != nil {
		if err := writer.file.Close(); err != nil {
			writer.warnLocked(
				"close previous daily log file: %v",
				err,
			)
		}
	}
	writer.file = file
	writer.currentDate = date
	return nil
}

func (writer *DailyWriter) cleanupLocked(now time.Time) {
	entries, err := os.ReadDir(writer.directory)
	if err != nil {
		writer.warnLocked(
			"list daily log files: %v",
			err,
		)
		return
	}

	cutoff := startOfDay(
		now,
		writer.location,
	).AddDate(
		0,
		0,
		-writer.retentionDays,
	)
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		match := dailyLogPattern.FindStringSubmatch(entry.Name())
		if len(match) != 2 {
			continue
		}
		fileDate, err := time.ParseInLocation(
			time.DateOnly,
			match[1],
			writer.location,
		)
		if err != nil || !fileDate.Before(cutoff) {
			continue
		}
		if err := os.Remove(filepath.Join(
			writer.directory,
			entry.Name(),
		)); err != nil {
			writer.warnLocked(
				"remove expired daily log file %s: %v",
				entry.Name(),
				err,
			)
		}
	}
}

func (writer *DailyWriter) warnLocked(
	format string,
	arguments ...any,
) {
	now := writer.now()
	if !writer.lastWarningAt.IsZero() && now.Sub(writer.lastWarningAt) < warningInterval {
		return
	}
	writer.lastWarningAt = now
	fmt.Fprintf(
		writer.stderr,
		"[copylingo logging] "+format+"\n",
		arguments...,
	)
}

func startOfDay(
	now time.Time,
	location *time.Location,
) time.Time {
	local := now.In(location)
	return time.Date(
		local.Year(),
		local.Month(),
		local.Day(),
		0,
		0,
		0,
		0,
		location,
	)
}
