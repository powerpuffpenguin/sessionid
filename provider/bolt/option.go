package bolt

import (
	"os"
	"time"

	"github.com/boltdb/bolt"
)

var defaultOptions = options{
	filename: "sessionid.db",
	mode:     0600,
	timeout:  time.Second * 30,
	bucket:   []byte(`sessionid`),

	access:  time.Hour,
	refresh: time.Hour * 24,
	maxSize: -1,
	batch:   128,
	clear:   time.Minute * 10,
}

type options struct {
	filename string
	mode     os.FileMode
	timeout  time.Duration
	bucket   []byte
	db       *bolt.DB

	access  time.Duration
	refresh time.Duration
	maxSize int
	batch   int
	clear   time.Duration
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
func WithFilename(filename string) Option {
	return newFuncOption(func(o *options) {
		o.filename = filename
	})
}
func WithFileMode(mode os.FileMode) Option {
	return newFuncOption(func(o *options) {
		o.mode = mode
	})
}
func WithTimeout(timeout time.Duration) Option {
	return newFuncOption(func(o *options) {
		o.timeout = timeout
	})
}
func WithDB(db *bolt.DB) Option {
	return newFuncOption(func(o *options) {
		o.db = db
	})
}
func WithBucket(bucket []byte) Option {
	return newFuncOption(func(o *options) {
		o.bucket = bucket
	})
}

// WithProviderAccess set the valid time of access token, at least one second.
func WithProviderAccess(duration time.Duration) Option {
	return newFuncOption(func(po *options) {
		po.access = duration
		if po.refresh < duration {
			po.refresh = duration
		}
	})
}

// WithProviderRefresh set the valid time of refresh token, at least one second.
func WithProviderRefresh(duration time.Duration) Option {
	return newFuncOption(func(po *options) {
		po.refresh = duration
		if po.access > duration {
			po.access = duration
		}
	})
}

// WithProviderMaxSize maximum number of tokens saved, if <= 0 not limit
func WithProviderMaxSize(maxSize int) Option {
	return newFuncOption(func(po *options) {
		po.maxSize = maxSize
	})
}

// WithProviderCheckBatch set batch check.
func WithProviderCheckBatch(batch int) Option {
	return newFuncOption(func(po *options) {
		po.batch = batch
	})
}

// WithProviderClear timer clear invalid token, if <=0 not start timer
func WithProviderClear(duration time.Duration) Option {
	return newFuncOption(func(po *options) {
		po.clear = duration
	})
}
