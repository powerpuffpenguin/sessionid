package agent

import (
	"time"

	"github.com/powerpuffpenguin/sessionid/cryptoer"
)

var defaultOptions = options{
	cryptoer:  nil,
	wheelTick: time.Second,
	wheelSize: 60 * 10,
}

type options struct {
	cryptoer  cryptoer.Cryptoer
	wheelTick time.Duration
	wheelSize int64
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
func WithCryptoer(cryptoer cryptoer.Cryptoer) Option {
	return newFuncOption(func(o *options) {
		o.cryptoer = cryptoer
	})
}
func WithWheel(tick time.Duration, wheelSize int64) Option {
	return newFuncOption(func(o *options) {
		o.wheelTick = tick
		o.wheelSize = wheelSize
	})
}
