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

type contextAttributesKey struct{}

var fallbackIDCounter atomic.Uint64

// WithAttributes replaces same-key attributes so nested layers can refine context.
func WithAttributes(
	ctx context.Context,
	attributes ...slog.Attr,
) context.Context {
	// copy existing array to avoid modifying parent context's attributes
	existing := slices.Clone(attributesFromContext(ctx))
	for _, attribute := range attributes {
		if attribute.Key == "" {
			continue
		}
		replaced := false
		for i := range existing {
			if existing[i].Key == attribute.Key {
				existing[i] = attribute
				replaced = true
				break
			}
		}
		if !replaced {
			existing = append(
				existing,
				attribute,
			)
		}
	}
	return context.WithValue(
		ctx,
		contextAttributesKey{},
		existing,
	)
}

func InteractionID(ctx context.Context) string {
	for _, attribute := range attributesFromContext(ctx) {
		if attribute.Key == "interaction_id" && attribute.Value.Kind() == slog.KindString {
			return attribute.Value.String()
		}
	}
	return ""
}

func NewInteractionID(prefix string) string {
	var raw [16]byte
	if _, err := rand.Read(raw[:]); err == nil {
		return prefix + "-" + hex.EncodeToString(raw[:])
	}
	return fmt.Sprintf(
		"%s-%x-%x",
		prefix,
		time.Now().UnixNano(),
		fallbackIDCounter.Add(1),
	)
}

func attributesFromContext(ctx context.Context) []slog.Attr {
	if ctx == nil {
		return nil
	}
	attributes, _ := ctx.Value(contextAttributesKey{}).([]slog.Attr)
	return attributes
}
