package agent

import (
	"context"
	"sync"
	"sync/atomic"
	"time"

	"github.com/go-redis/redis/v8"
	"github.com/powerpuffpenguin/sessionid/cryptoer"
)

// MemoryAgent sessionid agent store on memory
type RedisAgent struct {
	opts   options
	client *redis.Client
	done   uint32
	m      sync.Mutex
}

func NewRedisAgent(client *redis.Client, opt ...Option) *RedisAgent {
	opts := defaultOptions
	for _, o := range opt {
		o.apply(&opts)
	}
	return &RedisAgent{
		opts:   opts,
		client: client,
	}
}

// Create a token for id.
// userdata is the custom data associated with the token.
// expiry is the token expiry  duration.
func (a *RedisAgent) Create(ctx context.Context, id, userdata string, expiry time.Duration) (token string, e error) {
	sessionid, e := a.opts.sessionid()
	if e != nil {
		return
	}
	token, e = cryptoer.Encode(a.opts.signingMethod, a.opts.signingKey, id, sessionid)
	if e != nil {
		return
	}
	e = a.client.Set(ctx, token, userdata, expiry).Err()
	return
}

// Remove a exists token
func (a *RedisAgent) Remove(ctx context.Context, token string) (exists bool, e error) {
	_, _, e = cryptoer.Decode(a.opts.signingMethod, a.opts.signingKey, token)
	if e != nil {
		return
	}
	e = a.client.Del(ctx, token).Err()
	return
}

// RemoveByID remove token by id
func (a *RedisAgent) RemoveByID(ctx context.Context, id string) (exists bool, e error) {
	result := a.client.Keys(ctx, id+`.`)
	e = result.Err()
	if e != nil {
		return
	}
	keys := result.Val()
	if len(keys) != 0 {
		e = a.client.Del(ctx, keys...).Err()
	}
	return
}

// Get the userdata associated with the token
//
// if expiry > 0 then reset the expiration time
//
// if expiry < 0 then expire immediately after returning
func (a *RedisAgent) Get(ctx context.Context, token string, expiry time.Duration) (id, userdata string, exists bool, e error) {
	id, _, e = cryptoer.Decode(a.opts.signingMethod, a.opts.signingKey, token)
	if e != nil {
		return
	}
	cmd := a.client.Get(ctx, token)
	userdata, e = cmd.Result()
	if e != nil {
		if e == redis.Nil {
			e = nil
		}
		return
	}
	exists = true
	if expiry < 0 {
		e = a.client.Del(ctx, token).Err()
	} else if expiry > 0 {
		e = a.client.Set(ctx, token, userdata, expiry).Err()
	}
	return
}
func (a *RedisAgent) Close() error {
	if atomic.LoadUint32(&a.done) == 0 {
		a.m.Lock()
		defer a.m.Unlock()
		if a.done == 0 {
			defer atomic.StoreUint32(&a.done, 1)
			a.client.Close()
			return nil
		}
	}
	return ErrAgentClosed
}
