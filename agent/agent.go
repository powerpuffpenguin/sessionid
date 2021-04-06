package agent

import (
	"context"
	"time"
)

type Agent interface {
	// Create a token for id.
	// userdata is the custom data associated with the token.
	// expiration is the token expiration time.
	Create(ctx context.Context, id, userdata string, expiration time.Duration) (token string, e error)
	// Remove a exists token
	Remove(ctx context.Context, token string) (exists bool, e error)
	// RemoveByID remove token by id
	RemoveByID(ctx context.Context, id string) (exists bool, e error)
	// Get the userdata associated with the token
	Get(ctx context.Context, token string) (id, userdata string, exists bool, e error)
	// SetExpiry set the token expiration time.
	SetExpiry(ctx context.Context, token string, expiration time.Duration) (exists bool, e error)
	Close() (e error)
}
