package sessionid

import (
	"container/list"
	"context"
	"sync"
	"sync/atomic"
	"time"
)

type Pair struct {
	Key   string
	Value interface{}
}
type PairBytes struct {
	Key   string
	Value []byte
}
type Provider interface {
	// Create new session
	Create(ctx context.Context,
		access, refresh string, // id.sessionid.signature
		pair []PairBytes,
	) (e error)
	// Destroy a session by id
	Destroy(ctx context.Context, id string) (e error)
	// Destroy a session by token
	DestroyByToken(ctx context.Context, token string) (e error)
	// Check token status
	Check(ctx context.Context, token string) (e error)
	// Put key value for token
	Put(ctx context.Context, token string, pair []PairBytes) (e error)
	// Get key's value from token
	Get(ctx context.Context, token string, keys []string) (vals []Value, e error)
	// Keys return all token's key
	Keys(ctx context.Context, token string) (keys []string, e error)
	// Delete keys
	Delete(ctx context.Context, token string, keys []string) (e error)
	// Refresh a new access token
	Refresh(ctx context.Context, access, refresh, newAccess, newRefresh string) (e error)
	// Close provider
	Close() (e error)
}

type tokenValue struct {
	id              string
	access          string
	refresh         string
	accessDeadline  time.Time
	refreshDeadline time.Time
	data            map[string][]byte
}

func newTokenValue(id, access, refresh string, accessDuration, refreshDuration time.Duration) *tokenValue {
	at := time.Now()
	return &tokenValue{
		id:              id,
		access:          access,
		refresh:         refresh,
		accessDeadline:  at.Add(accessDuration),
		refreshDeadline: at.Add(refreshDuration),
		data:            make(map[string][]byte),
	}
}
func (t *tokenValue) Refresh(access, refresh string, accessDuration, refreshDuration time.Duration) {
	at := time.Now()
	t.access = access
	t.refresh = refresh
	t.accessDeadline = at.Add(accessDuration)
	t.refreshDeadline = at.Add(refreshDuration)
}

func (t *tokenValue) SetCopy(k string, v []byte) {
	if v == nil {
		t.data[k] = nil
	} else {
		val := make([]byte, len(v))
		copy(val, v)
		t.data[k] = val
	}
}
func (t *tokenValue) IsExpired() bool {
	return !t.accessDeadline.After(time.Now())
}
func (t *tokenValue) IsDeleted() bool {
	return !t.refreshDeadline.After(time.Now())
}
func (t *tokenValue) CopyKeys(keys []string) (vals []Value) {
	m := t.data
	vals = make([]Value, len(keys))
	if m == nil {
		return
	}
	for i, k := range keys {
		v, exists := m[k]
		if !exists {
			// vals[i] = Value{
			// 	Exists: false,
			// 	Bytes:  nil,
			// }
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
	opts providerOptions

	tokens map[string]*list.Element
	ids    map[string][]string
	lru    *list.List // token lru

	ticker *time.Ticker
	ch     chan string
	closed chan struct{}
	done   uint32
	m      sync.RWMutex
}

func NewMemoryProvider(opt ...ProviderOption) (provider *MemoryProvider) {
	opts := defaultProviderOptions
	for _, o := range opt {
		o.apply(&opts)
	}
	provider = &MemoryProvider{
		opts:   opts,
		tokens: make(map[string]*list.Element),
		ids:    make(map[string][]string),
		lru:    list.New(),
		ch:     make(chan string, opts.checkbuffer),
		closed: make(chan struct{}),
	}
	go provider.check(opts.checkbuffer)
	if opts.clear > 0 {
		ticker := time.NewTicker(opts.clear)
		provider.ticker = ticker
		go provider.clear(ticker.C)
	}
	return
}
func (p *MemoryProvider) clear(ch <-chan time.Time) {
	for {
		select {
		case <-p.closed:
			return
		case <-ch:
			p.m.Lock()
			p.doClear()
			p.m.Unlock()
		}
	}
}
func (p *MemoryProvider) doClear() {
	var (
		ele *list.Element
		t   *tokenValue
	)
	for {
		ele = p.lru.Front()
		if ele == nil {
			break
		}
		t = ele.Value.(*tokenValue)
		if t.IsDeleted() {
			strs := p.ids[t.id]
			for i, str := range strs {
				if str != t.access {
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
			delete(p.tokens, t.access)
		} else {
			break
		}
	}
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
		t      *tokenValue
	)
	for token = range m {
		ele, exists = p.tokens[token]
		if !exists {
			continue
		}
		t = ele.Value.(*tokenValue)
		if t.IsDeleted() {
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
	access, refresh string,
	pair []PairBytes,
) (e error) {
	id, _, _, e := Split(access)
	if e != nil {
		return
	}
	_, _, _, e = Split(refresh)
	if e != nil {
		return
	}
	t := newTokenValue(id, access, refresh, p.opts.access, p.opts.refresh)
	for i := 0; i < len(pair); i++ {
		t.SetCopy(pair[i].Key, pair[i].Value)
	}
	if t.IsDeleted() {
		return
	}

	p.m.Lock()
	defer p.m.Unlock()
	if p.done != 0 {
		e = ErrProviderClosed
		return
	}
	p.unsafeDestroyByToken(access)
	if p.opts.maxSize > 0 &&
		p.lru.Len() >= p.opts.maxSize {
		token := p.unsafePop(p.lru.Front())
		delete(p.tokens, token)
	}

	p.tokens[access] = p.lru.PushBack(t)
	p.ids[id] = append(p.ids[id], access)
	return
}
func (p *MemoryProvider) unsafePop(ele *list.Element) string {
	t := ele.Value.(*tokenValue)
	strs := p.ids[t.id]
	for i, str := range strs {
		if str != t.access {
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
	return t.access
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

// Check return true if token not expired
func (p *MemoryProvider) Check(ctx context.Context, token string) (e error) {
	_, _, _, e = Split(token)
	if e != nil {
		return
	}
	deleted := false
	p.m.RLock()
	if p.done != 0 {
		p.m.RUnlock()
		e = ErrProviderClosed
		return
	}
	ele, exists := p.tokens[token]
	if exists {
		t := ele.Value.(*tokenValue)
		if t.IsDeleted() {
			deleted = true
			e = ErrTokenNotExists
		} else if t.IsExpired() {
			e = ErrTokenExpired
		}
	} else {
		e = ErrTokenNotExists
	}
	p.m.RUnlock()

	if deleted {
		select {
		case <-p.closed:
		case p.ch <- token:
		}
	}
	return
}

func (p *MemoryProvider) unsafeGet(token string) (t *tokenValue, e error) {
	ele, exists := p.tokens[token]
	if !exists {
		e = ErrTokenNotExists
		return
	}
	t = ele.Value.(*tokenValue)
	if t.IsDeleted() {
		e = ErrTokenNotExists
		p.unsafePop(ele)
		delete(p.tokens, token)
		return
	} else if t.IsExpired() {
		e = ErrTokenExpired
		return
	}
	return
}

// Put key value for token
func (p *MemoryProvider) Put(ctx context.Context, token string, pair []PairBytes) (e error) {
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
	t, e := p.unsafeGet(token)
	if e != nil {
		return
	}
	if len(pair) == 0 {
		return
	}
	for i := 0; i < len(pair); i++ {
		t.SetCopy(pair[i].Key, pair[i].Value)
	}
	return
}

// Get key's value from token
func (p *MemoryProvider) Get(ctx context.Context, token string, keys []string) (vals []Value, e error) {
	_, _, _, e = Split(token)
	if e != nil {
		return
	}
	deleted := false
	p.m.RLock()
	if p.done != 0 {
		p.m.RUnlock()
		e = ErrProviderClosed
		return
	}
	ele, exists := p.tokens[token]
	if exists {
		t := ele.Value.(*tokenValue)
		if t.IsDeleted() {
			e = ErrTokenNotExists
			deleted = true
		} else if t.IsExpired() {
			e = ErrTokenExpired
		} else {
			vals = t.CopyKeys(keys)
		}
	} else {
		e = ErrTokenNotExists
	}
	p.m.RUnlock()

	if deleted {
		select {
		case <-p.closed:
		case p.ch <- token:
		}
	}
	return
}

// Keys return all token's key
func (p *MemoryProvider) Keys(ctx context.Context, token string) (keys []string, e error) {
	_, _, _, e = Split(token)
	if e != nil {
		return
	}
	deleted := false
	p.m.RLock()
	if p.done != 0 {
		p.m.RUnlock()
		e = ErrProviderClosed
		return
	}
	ele, exists := p.tokens[token]
	if exists {
		t := ele.Value.(*tokenValue)
		if t.IsDeleted() {
			e = ErrTokenNotExists
			deleted = true
		} else if t.IsExpired() {
			e = ErrTokenExpired
		} else if len(t.data) != 0 {
			keys = make([]string, len(t.data))
			for k := range t.data {
				keys = append(keys, k)
			}
		}
	} else {
		e = ErrTokenNotExists
	}
	p.m.RUnlock()
	if deleted {
		select {
		case <-p.closed:
		case p.ch <- token:
		}
	}
	return
}

// Delete keys
func (p *MemoryProvider) Delete(ctx context.Context, token string, keys []string) (e error) {
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
	t, e := p.unsafeGet(token)
	if e != nil {
		return
	}
	if len(keys) == 0 {
		return
	}
	for i := 0; i < len(keys); i++ {
		delete(t.data, keys[i])
	}
	return
}
func (p *MemoryProvider) checkID(id string, token ...string) (e error) {
	var s string
	for _, str := range token {
		s, _, _, e = Split(str)
		if e != nil {
			return
		}
		if s != id {
			e = ErrTokenIDNotMatched
			return
		}
	}
	return
}

// Refresh a new access token
func (p *MemoryProvider) Refresh(ctx context.Context, access, refresh, newAccess, newRefresh string) (e error) {
	id, _, _, e := Split(access)
	if e != nil {
		return
	}
	e = p.checkID(id, refresh, newAccess, newRefresh)
	if e != nil {
		return
	}

	p.m.Lock()
	defer p.m.Unlock()
	if p.done != 0 {
		e = ErrProviderClosed
		return
	}

	ele, exists := p.tokens[access]
	if !exists {
		e = ErrTokenNotExists
		return
	}
	t := ele.Value.(*tokenValue)
	if t.IsDeleted() {
		e = ErrTokenNotExists
		return
	} else if t.refresh != refresh {
		e = ErrRefreshTokenNotMatched
		return
	}
	// remove old
	p.unsafePop(ele)
	delete(p.tokens, access)
	// push new
	t.Refresh(newAccess, newRefresh, p.opts.access, p.opts.refresh)
	p.tokens[newAccess] = p.lru.PushBack(t)
	p.ids[id] = append(p.ids[id], newAccess)
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
			if p.ticker != nil {
				p.ticker.Stop()
			}
			p.tokens = nil
			p.ids = nil
			p.lru = nil
			return nil
		}
	}
	return ErrProviderClosed
}
