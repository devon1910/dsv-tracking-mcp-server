package obs

import (
	"context"
	"crypto/rand"
	"encoding/hex"
)

// requestIDKey is the unexported context key for request IDs.
type requestIDKey struct{}

// WithRequestID returns a new context carrying a randomly generated request ID.
func WithRequestID(ctx context.Context) context.Context {
	b := make([]byte, 8)
	_, _ = rand.Read(b)
	return context.WithValue(ctx, requestIDKey{}, hex.EncodeToString(b))
}

// RequestIDFromContext returns the request ID stored in ctx, or "" if none.
func RequestIDFromContext(ctx context.Context) string {
	id, _ := ctx.Value(requestIDKey{}).(string)
	return id
}
