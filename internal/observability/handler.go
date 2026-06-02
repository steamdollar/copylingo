package observability

import (
	"context"
	"log/slog"
)

// ContextHandler injects boundary attributes carried through context.Context.
type ContextHandler struct {
	next slog.Handler
}

func NewContextHandler(next slog.Handler) *ContextHandler {
	return &ContextHandler{next: next}
}

func (h *ContextHandler) Enabled(ctx context.Context, level slog.Level) bool {
	return h.next.Enabled(ctx, level)
}

func (h *ContextHandler) Handle(ctx context.Context, record slog.Record) error {
	keys := make(map[string]struct{})
	record.Attrs(func(attr slog.Attr) bool {
		keys[attr.Key] = struct{}{}
		return true
	})

	for _, attr := range attrsFromContext(ctx) {
		if _, exists := keys[attr.Key]; exists {
			continue
		}
		record.AddAttrs(attr)
		keys[attr.Key] = struct{}{}
	}
	if _, exists := keys["source"]; !exists {
		record.AddAttrs(slog.String("source", "app"))
	}
	if _, exists := keys["event"]; !exists {
		record.AddAttrs(slog.String("event", "legacy.log"))
	}

	return h.next.Handle(ctx, record)
}

func (h *ContextHandler) WithAttrs(attrs []slog.Attr) slog.Handler {
	return &ContextHandler{next: h.next.WithAttrs(attrs)}
}

func (h *ContextHandler) WithGroup(name string) slog.Handler {
	return &ContextHandler{next: h.next.WithGroup(name)}
}
