package sessionid

import (
	"context"
	"time"
)

type Provider interface {
	// Create new session
	Create(ctx context.Context,
		id, sessionid, token string,
		expiration time.Duration,
		pair ...interface{},
	) (e error)
	// Destroy a session by id
	Destroy(ctx context.Context, id string) (e error)
	// Destroy a session by token
	DestroyByToken(ctx context.Context, token string) (e error)
	// IsValid return true if token not expired
	IsValid(ctx context.Context, token string) (yes bool, e error)
	// SetExpiry set the token expiration time.
	SetExpiry(ctx context.Context, token string, expiration time.Duration) (exists bool, e error)
	// Set key value for token
	Set(ctx context.Context, token string, pair [][]byte) (e error)
	// Get key's value from token
	Get(ctx context.Context, token string, key [][]byte) (vals []Value, e error)
}
