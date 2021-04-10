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
	SetExpiry(ctx context.Context, token string, expiration time.Duration) (e error)
	// Set key value for token
	Set(ctx context.Context, token string, pair [][]byte) (e error)
	// Get key's value from token
	Get(ctx context.Context, token string, key [][]byte) (vals []Value, e error)
	// Delete keys
	Delete(ctx context.Context, token string, key [][]byte) (e error)
	// Close provider
	Close() (e error)
}

type tokenValue struct {
	id       string
	token    string
	data     map[string][]byte
	deadline time.Time
}

func (t *tokenValue) SetCopy(k string, v []byte) {
	if v == nil {
		t.data[k] = nil
	} else {
		val := make([]byte, len(v))
		t.data[k] = val
	}
}
func (t *tokenValue) IsExpired() bool {
	return !t.deadline.Before(time.Now())
}
func (t *tokenValue) CopyKey(key [][]byte) (vals []Value, expired bool) {
	expired = t.IsExpired()
	m := t.data
	vals = make([]Value, len(key))
	if m == nil {
		return
	}
	for i, k := range key {
		v, exists := m[string(k)]
		if !exists {
			continue
		}
		if v == nil {
			vals[i] = Value{
				Exists: true,
				Bytes:  nil,
			}
		} else {
			b := make([]byte, len(v))
			copy(b, v)
			vals[i] = Value{
				Exists: true,
				Bytes:  b,
			}
		}
	}
	return
}

// MemoryProvider a provider store data on local memory
type MemoryProvider struct {
	maxsize int
	tokens  map[string]*list.Element
	ids     map[string][]string
	lru     *list.List // token lru

	ch     chan string
	closed chan struct{}
	done   uint32
	m      sync.RWMutex
}

func NewMemoryProvider(maxsize, checkbuffer int) (provider *MemoryProvider) {
	if maxsize < 1 {
		panic(`MemoryProvider size < 1`)
	}
	if checkbuffer < 128 {
		checkbuffer = 128
	}
	provider = &MemoryProvider{
		maxsize: maxsize,
		tokens:  make(map[string]*list.Element),
		ids:     make(map[string][]string),
		lru:     list.New(),
		ch:      make(chan string, checkbuffer),
		closed:  make(chan struct{}),
	}
	go provider.check(checkbuffer)
	return provider
}

func (p *MemoryProvider) check(checkbuffer int) {
	if checkbuffer < 128 {
		checkbuffer = 128
	}
	var (
		m           = make(map[string]bool)
		closed, yes bool
		token       string
	)
	for {
		// first task
		token, closed = p.read()
		if closed {
			break
		}
		m[token] = true
		// merge task
		for {
			token, yes, closed = p.tryRead()
			if closed {
				return
			} else if yes {
				m[token] = true
				if len(m) >= checkbuffer {
					break
				}
			}
		}
		// do task
		if p.doCheck(m) {
			break
		}
		// clear task
		for token = range m {
			delete(m, token)
		}
	}
}
func (p *MemoryProvider) doCheck(m map[string]bool) (closed bool) {
	p.m.Lock()
	defer p.m.Unlock()
	if p.done != 0 {
		closed = true
		return
	}
	var (
		token  string
		ele    *list.Element
		exists bool
		t      tokenValue
	)
	for token = range m {
		ele, exists = p.tokens[token]
		if !exists {
			continue
		}
		t = ele.Value.(tokenValue)
		if t.IsExpired() {
			p.unsafePop(ele)
			delete(p.tokens, token)
		}
	}
	return
}
func (p *MemoryProvider) tryRead() (token string, yes, closed bool) {
	select {
	case <-p.closed:
		closed = true
	case token = <-p.ch:
		yes = true
	default:
	}
	return
}
func (p *MemoryProvider) read() (token string, closed bool) {
	select {
	case <-p.closed:
		closed = true
	case token = <-p.ch:
	}
	return
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
		id:       id,
		token:    token,
		deadline: time.Now().Add(expiration),
		data:     make(map[string][]byte),
	}
	for i := 0; i < len(pair); i += 2 {
		k := string(pair[i])
		var v []byte
		if i+1 < len(pair) {
			v = pair[i+1]
		}
		t.SetCopy(k, v)
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
	if p.lru.Len() >= p.maxsize {
		token := p.unsafePop(p.lru.Back())
		delete(p.tokens, token)
	}

	p.tokens[token] = p.lru.PushFront(token)
	p.ids[id] = append(p.ids[id], token)
	return
}
func (p *MemoryProvider) unsafePop(ele *list.Element) string {
	t := ele.Value.(tokenValue)
	strs := p.ids[t.id]
	for i, str := range strs {
		if str != t.token {
			continue
		}
		if len(strs) == 0 {
			delete(p.ids, t.id)
		} else {
			copy(strs[i:], strs[i+1:])
			p.ids[t.id] = strs[:len(strs)-1]
		}
		break
	}
	p.lru.Remove(ele)
	return t.token
}
func (p *MemoryProvider) unsafeDestroyByToken(token string) {
	ele, exists := p.tokens[token]
	if !exists {
		return
	}
	p.unsafePop(ele)
	delete(p.tokens, token)
}

// Destroy a session by id
func (p *MemoryProvider) Destroy(ctx context.Context, id string) (e error) {
	p.m.Lock()
	if p.done == 0 {
		if strs, exists := p.ids[id]; exists {
			delete(p.ids, id)
			for _, token := range strs {
				ele := p.tokens[token]
				p.lru.Remove(ele)
				delete(p.tokens, token)
			}
		}
	} else {
		e = ErrProviderClosed
	}
	p.m.Unlock()
	return
}

// Destroy a session by token
func (p *MemoryProvider) DestroyByToken(ctx context.Context, token string) (e error) {
	_, _, _, e = Split(token)
	if e != nil {
		return
	}
	p.m.Lock()
	if p.done == 0 {
		p.unsafeDestroyByToken(token)
	} else {
		e = ErrProviderClosed
	}
	p.m.Unlock()
	return
}

// IsValid return true if token not expired
func (p *MemoryProvider) IsValid(ctx context.Context, token string) (yes bool, e error) {
	_, _, _, e = Split(token)
	if e != nil {
		return
	}
	expired := false
	p.m.RLock()
	ele, exists := p.tokens[token]
	if exists {
		t := ele.Value.(tokenValue)
		expired = t.IsExpired()
		e = ErrExpiredToken
	} else {
		expired = true
	}
	p.m.RUnlock()

	if expired {
		select {
		case <-p.closed:
			if e == nil {
				e = ErrProviderClosed
			}
		case p.ch <- token:
			if e == nil {
				e = ErrExpiredToken
			}
		}
	}
	return
}

// SetExpiry set the token expiration time.
func (p *MemoryProvider) SetExpiry(ctx context.Context, token string, expiration time.Duration) (e error) {
	_, _, _, e = Split(token)
	if e != nil {
		return
	}
	p.m.Lock()
	defer p.m.Unlock()
	if p.done != 0 {
		e = ErrProviderClosed
		return
	}
	t, ele, e := p.unsafeGet(token)
	if e != nil {
		return
	}
	if expiration <= 0 {
		p.unsafePop(ele)
		delete(p.tokens, token)
		return
	}
	t.deadline = time.Now().Add(expiration)
	ele.Value = t
	if p.lru.Front() != ele {
		p.lru.MoveToFront(ele)
	}
	return
}
func (p *MemoryProvider) unsafeGet(token string) (t tokenValue, ele *list.Element, e error) {
	ele, exists := p.tokens[token]
	if !exists {
		e = ErrExpiredToken
		return
	}
	t = ele.Value.(tokenValue)
	if t.IsExpired() {
		e = ErrExpiredToken
		p.unsafePop(ele)
		delete(p.tokens, token)
		return
	}
	return
}

// Set key value for token
func (p *MemoryProvider) Set(ctx context.Context, token string, pair [][]byte) (e error) {
	_, _, _, e = Split(token)
	if e != nil {
		return
	}
	p.m.Lock()
	defer p.m.Unlock()
	if p.done != 0 {
		e = ErrProviderClosed
		return
	}
	t, ele, e := p.unsafeGet(token)
	if e != nil {
		return
	}
	if len(pair) == 0 {
		return
	}
	for i := 0; i < len(pair); i += 2 {
		k := string(pair[i])
		var v []byte
		if i+1 < len(pair) {
			v = pair[i+1]
		}
		t.SetCopy(k, v)
	}
	ele.Value = t
	return
}

// Get key's value from token
func (p *MemoryProvider) Get(ctx context.Context, token string, key [][]byte) (vals []Value, e error) {
	_, _, _, e = Split(token)
	if e != nil {
		return
	}
	expired := false
	p.m.RLock()
	ele, exists := p.tokens[token]
	if exists {
		t := ele.Value.(tokenValue)
		vals, expired = t.CopyKey(key)
		if expired {
			e = ErrExpiredToken
		}
	} else {
		expired = true
	}
	p.m.RUnlock()

	if expired {
		select {
		case <-p.closed:
			if e == nil {
				e = ErrProviderClosed
			}
		case p.ch <- token:
			if e == nil {
				e = ErrExpiredToken
			}
		}
	}
	return
}

// Delete keys
func (p *MemoryProvider) Delete(ctx context.Context, token string, key [][]byte) (e error) {
	_, _, _, e = Split(token)
	if e != nil {
		return
	}
	p.m.Lock()
	defer p.m.Unlock()
	if p.done != 0 {
		e = ErrProviderClosed
		return
	}
	t, ele, e := p.unsafeGet(token)
	if e != nil {
		return
	}
	if len(key) == 0 {
		return
	}
	for i := 0; i < len(key); i += 2 {
		delete(t.data, string(key[i]))
	}
	ele.Value = t
	return
}

// Close provider
func (p *MemoryProvider) Close() (e error) {
	if atomic.LoadUint32(&p.done) == 0 {
		p.m.Lock()
		defer p.m.Unlock()
		if p.done == 0 {
			defer atomic.StoreUint32(&p.done, 1)
			close(p.closed)
			p.tokens = nil
			p.ids = nil
			p.lru = nil
			return nil
		}
	}
	return ErrProviderClosed
}
