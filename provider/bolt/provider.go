package bolt

import (
	"sync"
	"sync/atomic"

	"github.com/boltdb/bolt"
	"github.com/powerpuffpenguin/sessionid"
)

// Provider a provider store data on local bolt database
type Provider struct {
	opts options
	db   *bolt.DB

	// tokens map[string]*list.Element
	// ids    map[string][]string
	// lru    *list.List // token lru

	ch     chan string
	closed chan struct{}
	done   uint32
	m      sync.RWMutex
}

func New(opt ...Option) (provider *Provider, e error) {
	opts := defaultOptions
	for _, o := range opt {
		o.apply(&opts)
	}
	var db *bolt.DB
	if opts.db == nil {
		var o *bolt.Options
		if opts.timeout > 0 {
			o = &bolt.Options{Timeout: opts.timeout}
		}
		db, e = bolt.Open(opts.filename, opts.mode, o)
		if e != nil {
			return
		}
	} else {
		db = opts.db
	}

	provider = &Provider{
		opts: opts,
		db:   db,
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
			close(p.closed)
			if p.opts.db == nil {
				p.db.Close()
			}
			return nil
		}
	}
	return sessionid.ErrProviderClosed
}
