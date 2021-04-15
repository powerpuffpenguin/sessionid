package bolt

import (
	"sync"
	"sync/atomic"
	"time"

	"github.com/boltdb/bolt"
	"github.com/powerpuffpenguin/sessionid"
)

var (
	bucketMetadata = []byte(`metadata`)
	bucketData     = []byte(`data`)
	bucketIDS      = []byte(`ids`)
	bucketLRU      = []byte(`lru`)
	bucketCount    = []byte(`count`)
)

// Provider a provider store data on local bolt database
type Provider struct {
	opts options
	db   *bolt.DB

	ticker *time.Ticker
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
	go provider.check(opts.batch)
	if opts.clear > 0 {
		ticker := time.NewTicker(opts.clear)
		provider.ticker = ticker
		go provider.clear(ticker.C)
	}
	return
}
func (p *Provider) clear(ch <-chan time.Time) {
	for {
		select {
		case <-p.closed:
			return
		case <-ch:
			p.m.Lock()
			p.doClear()
			p.m.Unlock()
		}
	}
}
func (p *Provider) doClear() {
	p.db.Update(func(t *bolt.Tx) (e error) {
		store := t.Bucket(p.opts.bucket)
		if store == nil {
			return
		}
		bucket := store.Bucket(bucketLRU)
		if bucket == nil {
			return
		}
		var (
			cursor = bucket.Cursor()
			bt     *bolt.Bucket
			md     *_Metadata
		)
		for k, v := cursor.First(); k != nil; k, v = cursor.Next() {
			bt, md, e = p.getMetadata(store, v)
			if e != nil {
				return
			} else if bt == nil || md == nil {
				e = bt.Delete(k)
				if e != nil {
					return
				}
				continue
			} else if md.IsDeleted() {
				e = bt.Delete(v)
				if e != nil {
					return
				}
				e = p.deleteData(store, v)
				if e != nil {
					return
				}
				e = p.deleteIDS(store, []byte(md.id), string(v))
				if e != nil {
					return
				}
				e = bucket.Delete(k)
				if e != nil {
					return
				}
				e = p.decrement(store)
				if e != nil {
					return
				}
			}
		}
		return
	})
}
func (p *Provider) check(batch int) {
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
		// do task
		if p.doCheck(m) {
			break
		}
		// clear task
		for token = range m {
			delete(m, token)
		}
	}
}
func (p *Provider) doCheck(m map[string]bool) (closed bool) {
	p.m.Lock()
	defer p.m.Unlock()
	if p.done != 0 {
		closed = true
		return
	}
	p.db.Update(func(t *bolt.Tx) (e error) {
		store := t.Bucket(p.opts.bucket)
		if store == nil {
			return
		}
		var (
			bucket *bolt.Bucket
			md     *_Metadata
		)
		for token := range m {
			key := []byte(token)
			bucket, md, e = p.getMetadata(store, key)
			if e != nil {
				return
			}
			if md == nil {
				continue
			}
			if md.IsDeleted() {
				e = bucket.Delete(key)
				if e != nil {
					return
				}
				e = p.deleteData(store, key)
				if e != nil {
					return
				}
				e = p.deleteIDS(store, []byte(md.id), token)
				if e != nil {
					return
				}
				e = p.deleteLRU(store, md.lru)
				if e != nil {
					return
				}
				e = p.decrement(store)
				if e != nil {
					return
				}
			}
		}
		return
	})
	return
}
func (p *Provider) tryRead() (token string, yes, closed bool) {
	select {
	case <-p.closed:
		closed = true
	case token = <-p.ch:
		yes = true
	default:
	}
	return
}
func (p *Provider) read() (token string, closed bool) {
	select {
	case <-p.closed:
		closed = true
	case token = <-p.ch:
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
			if p.ticker != nil {
				p.ticker.Stop()
			}
			if p.opts.db == nil {
				p.db.Close()
			}
			return nil
		}
	}
	return sessionid.ErrProviderClosed
}
