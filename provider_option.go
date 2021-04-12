package sessionid

import (
	"time"
)

var defaultProviderOptions = providerOptions{
	access:      time.Hour,
	refresh:     time.Hour * 24,
	maxSize:     -1,
	checkbuffer: 128,
	clear:       time.Minute * 10,
}

type providerOptions struct {
	access      time.Duration
	refresh     time.Duration
	maxSize     int
	checkbuffer int
	clear       time.Duration
}
type ProviderOption interface {
	apply(*providerOptions)
}
type funcProviderOption struct {
	f func(*providerOptions)
}

func (fdo *funcProviderOption) apply(do *providerOptions) {
	fdo.f(do)
}
func newFuncProviderOption(f func(*providerOptions)) *funcProviderOption {
	return &funcProviderOption{
		f: f,
	}
}

// WithProviderAccess set the valid time of access token, at least one second.
func WithProviderAccess(duration time.Duration) ProviderOption {
	return newFuncProviderOption(func(po *providerOptions) {
		po.access = duration
		if po.refresh < duration {
			po.refresh = duration
		}
	})
}

// WithProviderRefresh set the valid time of refresh token, at least one second.
func WithProviderRefresh(duration time.Duration) ProviderOption {
	return newFuncProviderOption(func(po *providerOptions) {
		po.refresh = duration
		if po.access > duration {
			po.access = duration
		}
	})
}

// WithProviderMaxSize maximum number of tokens saved, if <= 0 not limit
func WithProviderMaxSize(maxSize int) ProviderOption {
	return newFuncProviderOption(func(po *providerOptions) {
		po.maxSize = maxSize
	})
}

// WithProviderClear timer clear invalid token, if <=0 not start timer
func WithProviderClear(duration time.Duration) ProviderOption {
	return newFuncProviderOption(func(po *providerOptions) {
		po.clear = duration
	})
}
