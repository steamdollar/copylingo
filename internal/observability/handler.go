package observability

import (
	"context"
	"log/slog"
)

type ContextHandler struct {
	next slog.Handler
}

func NewContextHandler(next slog.Handler) *ContextHandler {
	return &ContextHandler{next: next}
}

func (handler *ContextHandler) Enabled(
	ctx context.Context,
	level slog.Level,
) bool {
	return handler.next.Enabled(
		ctx,
		level,
	)
}

func (handler *ContextHandler) Handle(
	ctx context.Context,
	record slog.Record,
) error {
	keys := make(map[string]struct{})
	record.Attrs(func(attribute slog.Attr) bool {
		keys[attribute.Key] = struct{}{}
		return true
	})

	for _, attribute := range attributesFromContext(ctx) {
		if _, exists := keys[attribute.Key]; exists {
			continue
		}
		record.AddAttrs(attribute)
		keys[attribute.Key] = struct{}{}
	}
	if _, exists := keys["source"]; !exists {
		record.AddAttrs(slog.String(
			"source",
			"app",
		))
	}
	if _, exists := keys["event"]; !exists {
		record.AddAttrs(slog.String(
			"event",
			"legacy.log",
		))
	}

	return handler.next.Handle(
		ctx,
		record,
	)
}

func (handler *ContextHandler) WithAttrs(attributes []slog.Attr) slog.Handler {
	return &ContextHandler{next: handler.next.WithAttrs(attributes)}
}

func (handler *ContextHandler) WithGroup(name string) slog.Handler {
	return &ContextHandler{next: handler.next.WithGroup(name)}
}
