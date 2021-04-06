package agent

import (
	"time"

	"github.com/powerpuffpenguin/sessionid/cryptoer"
)

var defaultOptions = options{
	signingMethod: cryptoer.SigningMethodHS256,
	signingKey:    []byte(`github.com/powerpuffpenguin/sessionid`),
	sessionid:     cryptoer.SessionID,
	wheelTick:     time.Second,
	wheelSize:     60 * 10,
}

type options struct {
	signingMethod cryptoer.SigningMethod
	signingKey    []byte
	sessionid     func() (string, error)
	wheelTick     time.Duration
	wheelSize     int64
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
func WithSigningMethod(method cryptoer.SigningMethod) Option {
	return newFuncOption(func(o *options) {
		o.signingMethod = method
	})
}
func WithSigningKey(key []byte) Option {
	return newFuncOption(func(o *options) {
		o.signingKey = key
	})
}
func WithSessionID(sessionid func() (string, error)) Option {
	return newFuncOption(func(o *options) {
		o.sessionid = sessionid
	})
}
func WithWheel(tick time.Duration, wheelSize int64) Option {
	return newFuncOption(func(o *options) {
		o.wheelTick = tick
		o.wheelSize = wheelSize
	})
}
