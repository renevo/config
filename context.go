package config

import (
	"context"
)

type contextKey string

var (
	configSetContextKey = contextKey("config-set")
)

// FromContext returns the set stored in ctx, or nil when no set is stored.
func FromContext(ctx context.Context) *Set {
	set := ctx.Value(configSetContextKey)
	if set == nil {
		return nil
	}

	return set.(*Set)
}

// WithContext returns a child context carrying set for use with FromContext.
func WithContext(ctx context.Context, set *Set) context.Context {
	return context.WithValue(ctx, configSetContextKey, set)
}
