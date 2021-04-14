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

	access:      time.Hour,
	refresh:     time.Hour * 24,
	maxSize:     -1,
	checkbuffer: 128,
	clear:       time.Minute * 10,
}

type options struct {
	filename string
	mode     os.FileMode
	timeout  time.Duration
	bucket   []byte
	db       *bolt.DB

	access      time.Duration
	refresh     time.Duration
	maxSize     int
	checkbuffer int
	clear       time.Duration
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
