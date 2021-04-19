package redis

import (
	"context"
	"time"

	"github.com/go-redis/redis/v8"
	"github.com/powerpuffpenguin/sessionid"
)

// Provider a provider store data on local bolt database
type Provider struct {
	opts options
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
	provider = &Provider{
		opts: opts,
	}
	return
}

// Close provider
func (p *Provider) Close() (e error) {
	return p.opts.client.Close()
}

// Create new session
func (p *Provider) Create(ctx context.Context,
	access, refresh string,
	pair []sessionid.PairBytes,
) (e error) {
	_, _, _, e = sessionid.Split(access)
	if e != nil {
		return
	}
	_, _, _, e = sessionid.Split(refresh)
	if e != nil {
		return
	}
	md := newMetadata(access, refresh, p.opts.access, p.opts.refresh)
	if md.IsDeleted() {
		return
	}
	md.Marshal()

	fields := make(map[string]interface{}, len(pair)+1)
	fields[metadataKey] = md
	for _, field := range pair {
		e = checkKey(field.Key)
		if e != nil {
			return
		}
		fields[field.Key] = string(field.Value)
	}
	key := p.prepareKey(access)
	e = p.opts.client.HSet(ctx, key, fields).Err()
	if e != nil {
		return
	}
	e = p.fix(ctx, key, p.readyKey(access), md.RefreshDeadline)
	if e != nil {
		p.opts.client.Del(ctx, key)
		return
	}
	return
}

func (p *Provider) fix(ctx context.Context, key, newkey string, deadline time.Time) (e error) {
	e = p.opts.client.ExpireAt(ctx, key, deadline).Err()
	if e != nil {
		return
	}
	e = p.opts.client.RenameNX(ctx, key, newkey).Err()
	if e != nil {
		return
	}
	return
}
func (p *Provider) prepareKey(key string) string {
	return preparePrefix + key
}

func (p *Provider) readyKey(key string) string {
	return readyPrefix + key
}
func (p *Provider) preparePattern(key string) string {
	return preparePrefix + key + `*`
}

func (p *Provider) readyPattern(key string) string {
	return readyPrefix + key + `*`
}

// Destroy a session by id
func (p *Provider) Destroy(ctx context.Context, id string) (e error) {
	result := p.opts.client.Keys(ctx, p.readyPattern(id+`.`))
	e = result.Err()
	if e != nil {
		return
	}
	keys := result.Val()
	if len(keys) == 0 {
		return
	}
	e = p.opts.client.Del(ctx, keys...).Err()
	if e != nil {
		return
	}
	return
}

// Destroy a session by token
func (p *Provider) DestroyByToken(ctx context.Context, token string) (e error) {
	_, _, _, e = sessionid.Split(token)
	if e != nil {
		return
	}
	e = p.opts.client.Del(ctx, p.readyKey(token)).Err()
	if e != nil {
		return
	}
	return
}

// Check return true if token not expired
func (p *Provider) Check(ctx context.Context, token string) (e error) {
	_, _, _, e = sessionid.Split(token)
	if e != nil {
		return
	}
	_, e = p.opts.client.HGet(ctx, p.readyKey(token), metadataKey).Result()
	if e != nil {
		return
	}

	return
}
