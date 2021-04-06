package agent

import (
	"context"
)

type Agent interface {
	// Create a token for id.
	// userdata is the custom data associated with the token.
	// maxage is the token expiration time in seconds .
	Create(ctx context.Context, id, userdata string, maxage int32) (token string, e error)
	// Remove a exists token
	Remove(ctx context.Context, token string) (exists bool, e error)
	// RemoveByID remove token by id
	RemoveByID(ctx context.Context, id string) (exists bool, e error)
	// Get the userdata associated with the token
	//
	// if maxage > 0 then reset the expiration time
	//
	// if maxage < 0 then expire immediately after returning
	Get(ctx context.Context, token string, maxage int32) (id, userdata string, exists bool, e error)
}
