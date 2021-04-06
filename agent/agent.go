package agent

import (
	"context"
	"time"
)

type Agent interface {
	// Create a token for id.
	// userdata is the custom data associated with the token.
	// expiry is the token expiry  duration.
	Create(ctx context.Context, id, userdata string, expiry time.Duration) (token string, e error)
	// Remove a exists token
	Remove(ctx context.Context, token string) (exists bool, e error)
	// RemoveByID remove token by id
	RemoveByID(ctx context.Context, id string) (exists bool, e error)
	// Get the userdata associated with the token
	//
	// if expiry > 0 then reset the expiration time
	//
	// if expiry < 0 then expire immediately after returning
	Get(ctx context.Context, token string, expiry time.Duration) (id, userdata string, exists bool, e error)
	Close() (e error)
}
