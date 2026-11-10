package observability

import (
	"fmt"
	"io"
	"log/slog"
	"os"
	"strings"
	"time"
)

type LoggerOptions struct {
	Directory     string
	Level         string
	RetentionDays int
	Timezone      string
	Stdout        io.Writer
	Stderr        io.Writer
}

// NewLogger falls back to stdout-only logging when file setup fails.
func NewLogger(options LoggerOptions) (*slog.Logger, func(), error) {
	level, err := ParseLevel(options.Level)
	if err != nil {
		return nil, nil, err
	}
	location, err := time.LoadLocation(options.Timezone)
	if err != nil {
		return nil, nil, fmt.Errorf(
			"load logging timezone %q: %w",
			options.Timezone,
			err,
		)
	}
	if options.Stdout == nil {
		options.Stdout = os.Stdout
	}
	if options.Stderr == nil {
		options.Stderr = os.Stderr
	}

	output := options.Stdout
	closeWriter := func() {}
	dailyWriter, err := NewDailyWriter(DailyWriterOptions{
		Directory:     options.Directory,
		RetentionDays: options.RetentionDays,
		Location:      location,
		Stderr:        options.Stderr,
	})
	if err != nil {
		fmt.Fprintf(
			options.Stderr,
			"[copylingo logging] file sink unavailable; using stdout only: %v\n",
			err,
		)
	} else {
		output = io.MultiWriter(
			options.Stdout,
			dailyWriter,
		)
		closeWriter = func() {
			if err := dailyWriter.Close(); err != nil {
				fmt.Fprintf(
					options.Stderr,
					"[copylingo logging] close daily log file: %v\n",
					err,
				)
			}
		}
	}

	handler := slog.NewJSONHandler(
		output,
		&slog.HandlerOptions{
			Level: level,
			ReplaceAttr: func(
				_ []string,
				attribute slog.Attr,
			) slog.Attr {
				if attribute.Key == slog.TimeKey && attribute.Value.Kind() == slog.KindTime {
					return slog.Time(
						attribute.Key,
						attribute.Value.Time().In(location),
					)
				}
				return attribute
			},
		},
	)
	return slog.New(NewContextHandler(handler)), closeWriter, nil
}

func ParseLevel(raw string) (slog.Level, error) {
	switch strings.ToUpper(strings.TrimSpace(raw)) {
	case "DEBUG":
		return slog.LevelDebug, nil
	case "INFO":
		return slog.LevelInfo, nil
	case "WARN":
		return slog.LevelWarn, nil
	case "ERROR":
		return slog.LevelError, nil
	default:
		return 0, fmt.Errorf(
			"unsupported logging level %q",
			raw,
		)
	}
}
