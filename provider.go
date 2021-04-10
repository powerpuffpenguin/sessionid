package sessionid

import (
	"container/list"
	"context"
	"sync"
	"sync/atomic"
	"time"
)

type Provider interface {
	// Create new session
	Create(ctx context.Context,
		token string, // id.sessionid.signature
		expiration time.Duration,
		pair [][]byte,
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
	// Close provider
	Close() (e error)
}

type tokenValue struct {
	id       string
	data     map[string][]byte
	deadline time.Time
}

func (t *tokenValue) setCopy(k string, v []byte) {
	if v == nil {
		t.data[k] = nil
	} else {
		val := make([]byte, len(v))
		t.data[k] = val
	}
}

// MemoryProvider a provider store data on local memory
type MemoryProvider struct {
	size   int
	tokens map[string]tokenValue
	ids    map[string][]string
	lru    *list.List // token lru

	done uint32
	m    sync.RWMutex
}

func NewMemoryProvider(size int) (provider *MemoryProvider) {
	if size < 1 {
		panic(`MemoryProvider size < 1`)
	}
	return &MemoryProvider{
		size:   size,
		tokens: make(map[string]tokenValue),
		ids:    make(map[string][]string),
		lru:    list.New(),
	}
}

// Create new session
func (p *MemoryProvider) Create(ctx context.Context,
	token string,
	expiration time.Duration,
	pair [][]byte,
) (e error) {
	id, _, _, e := Split(token)
	if e != nil {
		return
	}
	t := tokenValue{
		deadline: time.Now().Add(expiration),
		data:     make(map[string][]byte),
	}
	for i := 0; i < len(pair); i += 2 {
		k := string(pair[i])
		var v []byte
		if i+1 < len(pair) {
			v = pair[i+1]
		}
		t.setCopy(k, v)
	}
	if expiration <= 0 {
		return
	}

	p.m.Lock()
	defer p.m.Unlock()
	if p.done == 0 {
		e = ErrProviderClosed
		return
	}
	p.unsafeDestroyByToken(token)
	if p.lru.Len() >= p.size {
		p.unsafeDestroyByToken(p.lru.Back().Value.(string))
	}

	p.tokens[token] = t
	p.ids[id] = append(p.ids[id], token)
	p.lru.PushFront(token)
	return
}

func (p *MemoryProvider) unsafeDestroyByToken(token string) {

}

// Destroy a session by id
func (p *MemoryProvider) Destroy(ctx context.Context, id string) (e error) {
	return
}

// Destroy a session by token
func (p *MemoryProvider) DestroyByToken(ctx context.Context, token string) (e error) {
	return
}

// IsValid return true if token not expired
func (p *MemoryProvider) IsValid(ctx context.Context, token string) (yes bool, e error) {
	return
}

// SetExpiry set the token expiration time.
func (p *MemoryProvider) SetExpiry(ctx context.Context, token string, expiration time.Duration) (exists bool, e error) {
	return
}

// Set key value for token
func (p *MemoryProvider) Set(ctx context.Context, token string, pair [][]byte) (e error) {
	return
}

// Get key's value from token
func (p *MemoryProvider) Get(ctx context.Context, token string, key [][]byte) (vals []Value, e error) {
	return
}

// Close provider
func (p *MemoryProvider) Close() (e error) {
	if atomic.LoadUint32(&p.done) == 0 {
		p.m.Lock()
		defer p.m.Unlock()
		if p.done == 0 {
			defer atomic.StoreUint32(&p.done, 1)
			p.tokens = nil
			p.ids = nil
			p.lru = nil
			return nil
		}
	}
	return ErrProviderClosed
}
