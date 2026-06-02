package observability

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"log/slog"
	"slices"
	"sync/atomic"
	"time"
)

// study: docs/study/go_context_value_key.md

type contextAttrsKey struct{}

var fallbackIDCounter atomic.Uint64

// WithAttrs returns a context carrying structured logging attributes.
// Attributes with the same key replace older values so nested layers can refine context.
func WithAttrs(ctx context.Context, attrs ...slog.Attr) context.Context {
	// copy existing array to avoid modifying parent context's attributes
	existing := slices.Clone(attrsFromContext(ctx))
	for _, attr := range attrs {
		if attr.Key == "" {
			continue
		}
		replaced := false
		for i := range existing {
			if existing[i].Key == attr.Key {
				existing[i] = attr
				replaced = true
				break
			}
		}
		if !replaced {
			existing = append(existing, attr)
		}
	}
	return context.WithValue(ctx, contextAttrsKey{}, existing)
}

// InteractionID returns the current correlation identifier, if one exists.
func InteractionID(ctx context.Context) string {
	for _, attr := range attrsFromContext(ctx) {
		if attr.Key == "interaction_id" && attr.Value.Kind() == slog.KindString {
			return attr.Value.String()
		}
	}
	return ""
}

// NewInteractionID returns a compact correlation identifier for a boundary operation.
func NewInteractionID(prefix string) string {
	var raw [16]byte
	if _, err := rand.Read(raw[:]); err == nil {
		return prefix + "-" + hex.EncodeToString(raw[:])
	}
	return fmt.Sprintf("%s-%x-%x", prefix, time.Now().UnixNano(), fallbackIDCounter.Add(1))
}

func attrsFromContext(ctx context.Context) []slog.Attr {
	if ctx == nil {
		return nil
	}
	attrs, _ := ctx.Value(contextAttrsKey{}).([]slog.Attr)
	return attrs
}
