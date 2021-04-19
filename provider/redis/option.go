package redis

import (
	"time"

	"github.com/go-redis/redis/v8"
)

var defaultOptions = options{
	access:  time.Hour,
	refresh: time.Hour * 24,
}

type options struct {
	url     string
	client  *redis.Client
	access  time.Duration
	refresh time.Duration
}
type Option interface {
	apply(*options)
}
type funcOption struct {
	f func(*options)
}

func (fdo *funcOption) apply(do *options) {
	fdo.f(do)
}
func newFuncOption(f func(*options)) *funcOption {
	return &funcOption{
		f: f,
	}
}

// WithAccess set the valid time of access token, at least one second.
func WithAccess(duration time.Duration) Option {
	return newFuncOption(func(po *options) {
		po.access = duration
		if po.refresh < duration {
			po.refresh = duration
		}
	})
}

// WithRefresh set the valid time of refresh token, at least one second.
func WithRefresh(duration time.Duration) Option {
	return newFuncOption(func(po *options) {
		po.refresh = duration
		if po.access > duration {
			po.access = duration
		}
	})
}

// WithClient set redis client.
func WithClient(client *redis.Client) Option {
	return newFuncOption(func(po *options) {
		po.client = client
	})
}

// WithURL set redis client url.
func WithURL(url string) Option {
	return newFuncOption(func(po *options) {
		po.url = url
	})
}
