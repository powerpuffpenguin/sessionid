package redis

import (
	"context"
	"errors"
	"sync"
	"sync/atomic"
	"time"

	"github.com/go-redis/redis/v8"
	"github.com/powerpuffpenguin/sessionid"
)

// Provider a provider store data on local bolt database
type Provider struct {
	opts options

	ticker *time.Ticker
	ch     chan string

	ctx    context.Context
	cancel context.CancelFunc

	done uint32
	m    sync.RWMutex
}

func New(opt ...Option) (provider *Provider, e error) {
	opts := defaultOptions
	for _, o := range opt {
		o.apply(&opts)
	}

	if opts.client == nil {
		var opt *redis.Options
		opt, e = redis.ParseURL(opts.url)
		if e != nil {
			return
		}
		opts.client = redis.NewClient(opt)
	}
	var ch chan string
	if opts.batch > 0 {
		ch = make(chan string, opts.batch)
	} else {
		ch = make(chan string)
	}
	ctx, cancel := context.WithCancel(context.Background())
	provider = &Provider{
		opts:   opts,
		ch:     ch,
		ctx:    ctx,
		cancel: cancel,
	}
	go provider.delete(opts.batch)
	return
}
func (p *Provider) delete(batch int) {
	if batch < 1 {
		batch = 1
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
		for len(m) < batch {
			token, yes, closed = p.tryRead()
			if closed {
				return
			} else if yes {
				m[token] = true
			} else {
				break
			}
		}
		// do delete
		keys := make([]string, 0, len(m))
		for key := range m {
			keys = append(keys, key)
		}
		p.opts.client.Del(p.ctx, keys...)
		// clear task
		for token = range m {
			delete(m, token)
		}
	}
}
func (p *Provider) tryRead() (token string, yes, closed bool) {
	select {
	case <-p.ctx.Done():
		closed = true
	case token = <-p.ch:
		yes = true
	default:
	}
	return
}
func (p *Provider) read() (token string, closed bool) {
	select {
	case <-p.ctx.Done():
		closed = true
	case token = <-p.ch:
	}
	return
}
func (p *Provider) toError(e error) error {
	if errors.Is(e, redis.ErrClosed) {
		return sessionid.ErrProviderClosed
	}
	return e
}

// Create new session
func (p *Provider) Create(ctx context.Context,
	access, refresh string,
	pair []sessionid.PairBytes,
) (e error) {
	e = p.checkID(access, refresh)
	if e != nil {
		return
	}

	md := newMetadata(access, refresh, p.opts.access, p.opts.refresh)
	if md.IsDeleted() {
		return
	}
	b, e := p.opts.coder.Encode(md)
	if e != nil {
		return
	}

	fields := make([]string, 0, (len(pair)+1)*2)
	fields = append(fields, p.opts.metadataKey, string(b))
	for _, field := range pair {
		e = p.checkKey(field.Key)
		if e != nil {
			return
		}
		fields = append(fields, field.Key, string(field.Value))
	}

	key := p.keyName(access)
	tx := p.opts.client.TxPipeline()
	tx.HSet(ctx, key, fields)
	tx.ExpireAt(ctx, key, md.RefreshDeadline)
	_, e = tx.Exec(ctx)
	if e != nil {
		e = p.toError(e)
		return
	}
	return
}
func (p *Provider) checkKey(key string) (e error) {
	if key == p.opts.metadataKey {
		e = errors.New(`key is reserved : ` + key)
		return
	}
	return
}
func (p *Provider) keyName(key string) string {
	return p.opts.keyPrefix + key
}

// Destroy a session by id
func (p *Provider) Destroy(ctx context.Context, id string) (e error) {
	result := p.opts.client.Keys(ctx, p.keyName(id+`.*`))
	e = result.Err()
	if e != nil {
		e = p.toError(e)
		return
	}
	keys := result.Val()
	if len(keys) == 0 {
		return
	}
	e = p.opts.client.Del(ctx, keys...).Err()
	if e != nil {
		e = p.toError(e)
		return
	}
	return
}

// Destroy a session by token
func (p *Provider) DestroyByToken(ctx context.Context, token string) (e error) {
	e = p.checkID(token)
	if e != nil {
		return
	}

	e = p.opts.client.Del(ctx, p.keyName(token)).Err()
	if e != nil {
		e = p.toError(e)
		return
	}
	return
}
func (p *Provider) getMetadata(ctx context.Context, key string, md *_Metadata) (val string, e error) {
	val, e = p.opts.client.HGet(ctx, key, p.opts.metadataKey).Result()
	if e != nil {
		e = p.toError(e)
		return
	}
	e = p.opts.coder.Decode([]byte(val), md)
	if e != nil {
		return
	}
	return
}

// Check return true if token not expired
func (p *Provider) Check(ctx context.Context, token string) (e error) {
	e = p.checkID(token)
	if e != nil {
		return
	}

	var md _Metadata
	key := p.keyName(token)
	_, e = p.getMetadata(ctx, key, &md)
	if e != nil {
		return
	} else if md.IsDeleted() {
		e = sessionid.ErrTokenNotExists
		p.pushDelete(key)
		return
	} else if md.IsExpired() {
		e = sessionid.ErrTokenExpired
		return
	}
	return
}
func (p *Provider) pushDelete(token string) {
	select {
	case <-p.ctx.Done():
	case p.ch <- token:
	}
}

// Put key value for token
func (p *Provider) Put(ctx context.Context, token string, pair []sessionid.PairBytes) (e error) {
	e = p.checkID(token)
	if e != nil {
		return
	}

	fields := make([]string, (len(pair)+1)*2)
	for _, field := range pair {
		e = p.checkKey(field.Key)
		if e != nil {
			return
		}
		fields = append(fields, string(field.Value))
	}

	var md _Metadata
	key := p.keyName(token)
	mdStr, e := p.getMetadata(ctx, key, &md)
	if e != nil {
		return
	} else if md.IsDeleted() {
		e = sessionid.ErrTokenNotExists
		p.pushDelete(key)
		return
	} else if md.IsExpired() {
		e = sessionid.ErrTokenExpired
		return
	}
	fields = append(fields, p.opts.metadataKey, mdStr)
	e = p.opts.client.HSet(ctx, key, fields).Err()
	if e != nil {
		e = p.toError(e)
		return
	}
	return
}

// Get key's value from token
func (p *Provider) Get(ctx context.Context, token string, keys []string) (vals []sessionid.Value, e error) {
	e = p.checkID(token)
	if e != nil {
		return
	}

	fields := make([]string, 1+len(keys))
	fields[0] = p.opts.metadataKey
	copy(fields[1:], keys)
	key := p.keyName(token)
	results, e := p.opts.client.HMGet(ctx, key, fields...).Result()
	if e != nil {
		e = p.toError(e)
		return
	}
	if len(results) != len(fields) {
		e = errors.New(`redis result not match`)
		return
	}
	vals = make([]sessionid.Value, len(keys))
	for i, v := range results {
		if i == 0 {
			if str, ok := v.(string); ok && len(str) != 0 {
				var md _Metadata
				e = p.opts.coder.Decode([]byte(str), &md)
				if e != nil {
					p.pushDelete(key)
					break
				} else if md.IsDeleted() {
					e = sessionid.ErrTokenNotExists
					p.pushDelete(key)
					break
				} else if md.IsExpired() {
					e = sessionid.ErrTokenExpired
					break
				}
			} else {
				e = sessionid.ErrTokenNotExists
				p.pushDelete(key)
				break
			}
		} else {
			if str, ok := v.(string); ok {
				vals[i-1] = sessionid.Value{
					Exists: true,
					Bytes:  []byte(str),
				}
			}
		}
	}
	return
}

// Keys return all token's key
func (p *Provider) Keys(ctx context.Context, token string) (keys []string, e error) {
	e = p.checkID(token)
	if e != nil {
		return
	}

	keys, e = p.opts.client.HKeys(ctx, p.keyName(token)).Result()
	if e != nil {
		e = p.toError(e)
		return
	}
	return
}

// Delete keys
func (p *Provider) Delete(ctx context.Context, token string, keys []string) (e error) {
	e = p.checkID(token)
	if e != nil {
		return
	}
	if len(keys) == 0 {
		return
	}
	_, e = p.opts.client.HDel(ctx, p.keyName(token), keys...).Result()
	if e != nil {
		e = p.toError(e)
		return
	}
	return
}
func (p *Provider) checkID(token ...string) (e error) {
	var id, s string
	for i, str := range token {
		s, _, _, e = sessionid.Split(str)
		if e != nil {
			return
		}
		if i == 0 {
			id = s
		} else if s != id {
			e = sessionid.ErrTokenIDNotMatched
			return
		}
	}
	return
}

// Refresh a new access token
func (p *Provider) Refresh(ctx context.Context, access, refresh, newAccess, newRefresh string) (err error) {
	e := p.checkID(access, refresh, newAccess, newRefresh)
	if e != nil {
		return
	}
	var md _Metadata
	key := p.keyName(access)
	_, e = p.getMetadata(ctx, key, &md)
	if e != nil {
		return
	} else if md.IsDeleted() {
		p.pushDelete(key)
		return
	} else if md.Refresh != refresh {
		err = sessionid.ErrRefreshTokenNotMatched
		return
	}

	md.DoRefresh(newAccess, newRefresh, p.opts.access, p.opts.refresh)
	b, e := p.opts.coder.Encode(&md)
	if e != nil {
		return
	}

	newkey := p.keyName(newAccess)
	tx := p.opts.client.TxPipeline()
	tx.HSet(ctx, key, p.opts.metadataKey, string(b))
	tx.ExpireAt(ctx, key, md.RefreshDeadline)
	tx.Rename(ctx, key, newkey)
	_, e = tx.Exec(ctx)
	if e != nil {
		e = p.toError(e)
		return
	}
	return
}

// Close provider
func (p *Provider) Close() (e error) {
	if atomic.LoadUint32(&p.done) == 0 {
		p.m.Lock()
		defer p.m.Unlock()
		if p.done == 0 {
			defer atomic.StoreUint32(&p.done, 1)
			p.cancel()
			if p.ticker != nil {
				p.ticker.Stop()
			}
			p.opts.client.Close()
			return nil
		}
	}
	return p.opts.client.Close()
}
