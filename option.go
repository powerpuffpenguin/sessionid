package sessionid

import (
	"github.com/powerpuffpenguin/sessionid/cryptoer"
)

var defaultOptions = options{
	method: cryptoer.SigningMethodHMD5,
	key:    []byte(`github.com/powerpuffpenguin/sessionid`),
	coder:  JSONCoder{},
}

type options struct {
	method   cryptoer.SigningMethod
	key      []byte
	provider Provider
	coder    Coder
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
func WithMethod(method cryptoer.SigningMethod) Option {
	return newFuncOption(func(o *options) {
		o.method = method
	})
}
func WithKey(key []byte) Option {
	return newFuncOption(func(o *options) {
		o.key = key
	})
}
func WithProvider(provider Provider) Option {
	return newFuncOption(func(o *options) {
		o.provider = provider
	})
}
func WithCoder(coder Coder) Option {
	return newFuncOption(func(o *options) {
		o.coder = coder
	})
}
