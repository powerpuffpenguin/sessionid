package bolt

import (
	"bytes"
	"context"
	"encoding/gob"
	"fmt"
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
	keyCount       = []byte(`count`)
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
	e = db.Update(func(t *bolt.Tx) (e error) {
		bucket, e := t.CreateBucketIfNotExists(opts.bucket)
		if e != nil {
			return
		}
		names := [][]byte{bucketMetadata, bucketData, bucketIDS, bucketLRU}
		for _, name := range names {
			_, e = bucket.CreateBucketIfNotExists(name)
			if e != nil {
				return
			}
		}
		return
	})
	if e != nil {
		if opts.db == nil {
			db.Close()
		}
		return
	}

	var ch chan string
	if opts.batch > 0 {
		ch = make(chan string, opts.batch)
	} else {
		ch = make(chan string)
	}
	provider = &Provider{
		opts:   opts,
		db:     db,
		ch:     ch,
		closed: make(chan struct{}),
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
func (p *Provider) store(t *bolt.Tx) (bucket *bolt.Bucket) {
	bucket = t.Bucket(p.opts.bucket)
	if bucket == nil {
		panic(fmt.Sprintf(`store bucket %s not exists`, p.opts.bucket))
	}
	return
}
func (p *Provider) buckets(store *bolt.Bucket) (bMD, bData, bIDS, bLRU *bolt.Bucket) {
	bMD = store.Bucket(bucketMetadata)
	if bMD == nil {
		panic(`bucket metadata not exists`)
	}
	bData = store.Bucket(bucketData)
	if bData == nil {
		panic(`bucket data not exists`)
	}
	bIDS = store.Bucket(bucketIDS)
	if bIDS == nil {
		panic(`bucket ids not exists`)
	}
	bLRU = store.Bucket(bucketLRU)
	if bLRU == nil {
		panic(`bucket lru not exists`)
	}
	return
}
func (p *Provider) md(t *bolt.Tx) (store, md *bolt.Bucket) {
	store = t.Bucket(p.opts.bucket)
	if store == nil {
		panic(fmt.Sprintf(`store bucket %s not exists`, p.opts.bucket))
	}
	md = store.Bucket(bucketMetadata)
	if md == nil {
		panic(`bucket metadata not exists`)
	}
	return
}
func (p *Provider) data(t *bolt.Tx) (store, md, data *bolt.Bucket) {
	store = t.Bucket(p.opts.bucket)
	if store == nil {
		panic(fmt.Sprintf(`store bucket %s not exists`, p.opts.bucket))
	}
	md = store.Bucket(bucketMetadata)
	if md == nil {
		panic(`bucket metadata not exists`)
	}
	data = store.Bucket(bucketData)
	if data == nil {
		panic(`bucket data not exists`)
	}
	return
}

func (p *Provider) doClear() {
	p.db.Update(func(t *bolt.Tx) (e error) {
		var (
			store                  = p.store(t)
			bMD, bData, bIDS, bLRU = p.buckets(store)
			key                    []byte
			md                     *_Metadata
			cursor                 = bLRU.Cursor()
		)
		for k, v := cursor.First(); k != nil; k, v = cursor.Next() {
			key = v
			md, e = p.getMetadata(bMD, key)
			if e != nil {
				return
			} else if md == nil {
				e = bLRU.Delete(k)
				if e != nil {
					return
				}
				continue
			} else if !md.IsDeleted() {
				break
			}
			e = p.delete(store, bMD, bData, bIDS, bLRU, string(v), []byte(md.ID), md.LRU)
			if e != nil {
				return
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
		var (
			store                  = p.store(t)
			bMD, bData, bIDS, bLRU = p.buckets(store)
			md                     *_Metadata
		)
		for token := range m {
			md, e = p.getMetadata(bMD, []byte(token))
			if e == nil {
				return
			} else if md == nil {
				continue
			} else if md.IsDeleted() {
				e = p.delete(store, bMD, bData, bIDS, bLRU, token, []byte(md.ID), md.LRU)
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

// Create new session
func (p *Provider) Create(ctx context.Context,
	access, refresh string,
	pair []sessionid.PairBytes,
) (e error) {
	id, _, _, e := sessionid.Split(access)
	if e != nil {
		return
	}
	_, _, _, e = sessionid.Split(refresh)
	if e != nil {
		return
	}
	md := newMetadata(id, access, refresh, p.opts.access, p.opts.refresh)
	if md.IsDeleted() {
		return
	}
	p.m.Lock()
	defer p.m.Unlock()
	if p.done != 0 {
		e = sessionid.ErrProviderClosed
		return
	}
	e = p.db.Update(func(t *bolt.Tx) (e error) {
		var (
			store                  = p.store(t)
			bMD, bData, bIDS, bLRU = p.buckets(store)
			key                    = []byte(access)
		)
		e = p.destoryToken(store, bMD, bData, bIDS, bLRU, access)
		if e != nil {
			return
		}
		if p.opts.maxSize > 0 &&
			p.count(store) >= uint64(p.opts.maxSize) {
			e = p.unsafePop(store, bMD, bData, bIDS, bLRU)
			if e != nil {
				return
			}
		}

		md.LRU, e = p.putLRU(bLRU, key)
		if e != nil {
			return
		}
		e = p.putMetadata(bMD, key, md)
		if e != nil {
			return
		}
		e = p.putData(bData, key, pair)
		if e != nil {
			return
		}
		e = p.appendIDS(bIDS, []byte(id), access)
		if e != nil {
			return
		}
		e = p.increment(store)
		if e != nil {
			return
		}
		return
	})
	return
}

func (p *Provider) delete(store, bMD, bData, bIDS, bLRU *bolt.Bucket,
	token string, id []byte, lru uint64,
) (e error) {
	key := []byte(token)
	e = bMD.Delete(key)
	if e != nil {
		return
	}
	e = bData.DeleteBucket(key)
	if e != nil {
		return
	}
	if bIDS != nil && id != nil {
		e = p.deleteIDS(bIDS, id, token)
		if e != nil {
			return
		}
	}
	if bLRU != nil && lru != 0 {
		e = bLRU.Delete(toBytes(lru))
		if e != nil {
			return
		}
	}
	e = p.decrement(store)
	if e != nil {
		return
	}
	return
}
func (p *Provider) deleteTokens(store, bMD, bData, bLRU *bolt.Bucket, tokens []string) (e error) {
	var (
		key []byte
		md  *_Metadata
	)
	for _, token := range tokens {
		key = []byte(token)
		md, e = p.getMetadata(bMD, key)
		if e != nil {
			return
		} else if md == nil {
			continue
		}
		e = bMD.Delete(key)
		if e != nil {
			return
		}
		e = bData.DeleteBucket(key)
		if e != nil {
			return
		}
		e = bLRU.Delete(toBytes(md.LRU))
		if e != nil {
			return
		}
		e = p.decrement(store)
		if e != nil {
			return
		}
	}
	return
}
func (p *Provider) destoryToken(store, bMD, bData, bIDS, bLRU *bolt.Bucket, token string) (e error) {
	md, e := p.getMetadata(bMD, []byte(token))
	if e != nil {
		return
	} else if md == nil {
		return
	}
	e = p.delete(store, bMD, bData, bIDS, bLRU,
		token, []byte(md.ID), md.LRU)
	return
}
func (p *Provider) unsafeDestroyByToken(store *bolt.Bucket, token string) error {
	bMD, bData, bIDS, bLRU := p.buckets(store)
	return p.destoryToken(store, bMD, bData, bIDS, bLRU, token)
}
func (p *Provider) unsafeDestroy(store *bolt.Bucket, id string) (e error) {
	bMD, bData, bIDS, bLRU := p.buckets(store)
	key := []byte(id)
	b := bIDS.Get(key)
	if b == nil {
		return
	}
	e = bIDS.Delete(key)
	if e != nil {
		return
	}

	var strs []string
	dec := gob.NewDecoder(bytes.NewBuffer(b))
	e = dec.Decode(&strs)
	if e != nil {
		return
	}
	e = p.deleteTokens(store, bMD, bData, bLRU, strs)
	return
}
func (p *Provider) unsafePop(store, bMD, bData, bIDS, bLRU *bolt.Bucket) (e error) {
	var (
		key    []byte
		md     *_Metadata
		cursor = bLRU.Cursor()
	)
	for k, v := cursor.First(); k != nil; k, v = cursor.Next() {
		key = v
		md, e = p.getMetadata(bMD, key)
		if e != nil {
			return
		} else if md == nil {
			e = bLRU.Delete(k)
			if e != nil {
				return
			}
			continue
		}
		e = p.delete(store, bMD, bData, bIDS, nil, string(key), []byte(md.ID), md.LRU)
		if e != nil {
			return
		}
		e = bLRU.Delete(k)
		if e != nil {
			return
		}
		break
	}
	return
}

// Destroy a session by id
func (p *Provider) Destroy(ctx context.Context, id string) (e error) {
	p.m.Lock()
	if p.done == 0 {
		e = p.db.Update(func(t *bolt.Tx) (e error) {
			store := t.Bucket(p.opts.bucket)
			if store == nil {
				return
			}
			e = p.unsafeDestroy(store, id)
			return
		})
	} else {
		e = sessionid.ErrProviderClosed
	}
	p.m.Unlock()
	return
}

// Destroy a session by token
func (p *Provider) DestroyByToken(ctx context.Context, token string) (e error) {
	_, _, _, e = sessionid.Split(token)
	if e != nil {
		return
	}
	p.m.Lock()
	if p.done == 0 {
		e = p.db.Update(func(t *bolt.Tx) (e error) {
			store := t.Bucket(p.opts.bucket)
			if store == nil {
				return
			}
			e = p.unsafeDestroyByToken(store, token)
			return
		})
	} else {
		e = sessionid.ErrProviderClosed
	}
	p.m.Unlock()
	return
}

func (p *Provider) unsafeCheck(bucket *bolt.Bucket, token string) (deleted bool, e error) {
	md, e := p.getMetadata(bucket, []byte(token))
	if e != nil {
		return
	} else if md == nil {
		e = sessionid.ErrTokenNotExists
	} else if md.IsDeleted() {
		deleted = true
		e = sessionid.ErrTokenNotExists
	} else if md.IsExpired() {
		e = sessionid.ErrTokenExpired
	}
	return
}

// Check return true if token not expired
func (p *Provider) Check(ctx context.Context, token string) (e error) {
	_, _, _, e = sessionid.Split(token)
	if e != nil {
		return
	}
	deleted := false
	p.m.RLock()
	if p.done == 0 {
		e = p.db.View(func(t *bolt.Tx) (e error) {
			_, bMD := p.md(t)
			deleted, e = p.unsafeCheck(bMD, token)
			return
		})
	} else {
		e = sessionid.ErrProviderClosed
	}
	p.m.RUnlock()
	if deleted {
		select {
		case <-p.closed:
		case p.ch <- token:
		}
	}
	return
}

// Put key value for token
func (p *Provider) Put(ctx context.Context, token string, pair []sessionid.PairBytes) (err error) {
	_, _, _, err = sessionid.Split(token)
	if err != nil {
		return
	}
	p.m.Lock()
	if p.done == 0 {
		result := p.db.Update(func(t *bolt.Tx) (e error) {
			var (
				store                  = p.store(t)
				bMD, bData, bIDS, bLRU = p.buckets(store)
				md                     *_Metadata
				key                    = []byte(token)
			)
			md, e = p.getMetadata(bMD, key)
			if e != nil {
				return
			} else if md == nil {
				err = sessionid.ErrTokenNotExists
				return
			} else if md.IsDeleted() {
				e = p.delete(store, bMD, bData, bIDS, bLRU, token, []byte(md.ID), md.LRU)
				if e != nil {
					return
				}
				err = sessionid.ErrTokenNotExists
				return
			} else if md.IsExpired() {
				err = sessionid.ErrTokenExpired
				return
			}
			e = p.putData(bData, key, pair)
			if e != nil {
				return
			}
			return
		})
		if result != nil {
			err = result
		}
	} else {
		err = sessionid.ErrProviderClosed
	}
	p.m.Unlock()
	return
}

// Get key's value from token
func (p *Provider) Get(ctx context.Context, token string, keys []string) (vals []sessionid.Value, e error) {
	_, _, _, e = sessionid.Split(token)
	if e != nil {
		return
	}
	deleted := false
	p.m.RLock()
	if p.done == 0 {
		e = p.db.View(func(t *bolt.Tx) (e error) {
			_, bMD, bData := p.data(t)
			deleted, e = p.unsafeCheck(bMD, token)
			if e != nil {
				return
			}
			vals, e = p.getData(bData, token, keys)
			return
		})
	} else {
		e = sessionid.ErrProviderClosed
	}
	p.m.RUnlock()

	if deleted {
		select {
		case <-p.closed:
		case p.ch <- token:
		}
	}
	return
}

// Keys return all token's key
func (p *Provider) Keys(ctx context.Context, token string) (keys []string, e error) {
	_, _, _, e = sessionid.Split(token)
	if e != nil {
		return
	}
	deleted := false
	p.m.RLock()
	if p.done == 0 {
		e = p.db.View(func(t *bolt.Tx) (e error) {
			_, bMD, bData := p.data(t)
			deleted, e = p.unsafeCheck(bMD, token)
			if e != nil {
				return
			}
			keys, e = p.getKeys(bData, token)
			return
		})
	} else {
		e = sessionid.ErrProviderClosed
	}
	p.m.RUnlock()

	if deleted {
		select {
		case <-p.closed:
		case p.ch <- token:
		}
	}
	return
}

// Delete keys
func (p *Provider) Delete(ctx context.Context, token string, keys []string) (err error) {
	_, _, _, err = sessionid.Split(token)
	if err != nil {
		return
	}
	p.m.Lock()
	if p.done == 0 {
		result := p.db.Update(func(t *bolt.Tx) (e error) {
			var (
				store                  = p.store(t)
				bMD, bData, bIDS, bLRU = p.buckets(store)
				md                     *_Metadata
				key                    = []byte(token)
			)
			md, e = p.getMetadata(bMD, key)
			if e != nil {
				return
			} else if md == nil {
				err = sessionid.ErrTokenNotExists
				return
			} else if md.IsDeleted() {
				e = p.delete(store, bMD, bData, bIDS, bLRU, token, []byte(md.ID), md.LRU)
				if e != nil {
					return
				}
				err = sessionid.ErrTokenNotExists
				return
			} else if md.IsExpired() {
				err = sessionid.ErrTokenExpired
				return
			}
			e = p.deleteKeys(bData, token, keys)
			if e != nil {
				return
			}
			return
		})
		if result != nil {
			err = result
		}
	} else {
		err = sessionid.ErrProviderClosed
	}
	p.m.Unlock()
	return
}
func (p *Provider) checkID(id string, token ...string) (e error) {
	var s string
	for _, str := range token {
		s, _, _, e = sessionid.Split(str)
		if e != nil {
			return
		}
		if s != id {
			e = sessionid.ErrTokenIDNotMatched
			return
		}
	}
	return
}

// Refresh a new access token
func (p *Provider) Refresh(ctx context.Context, access, refresh, newAccess, newRefresh string) (err error) {
	id, _, _, err := sessionid.Split(access)
	if err != nil {
		return
	}
	err = p.checkID(id, refresh, newAccess, newRefresh)
	if err != nil {
		return
	}

	p.m.Lock()
	if p.done == 0 {
		result := p.db.Update(func(t *bolt.Tx) (e error) {
			var (
				store                  = p.store(t)
				bMD, bData, bIDS, bLRU = p.buckets(store)
				md                     *_Metadata
				key                    = []byte(access)
			)
			md, e = p.getMetadata(bMD, key)
			if e != nil {
				return
			} else if md == nil {
				err = sessionid.ErrTokenNotExists
				return
			} else if md.IsDeleted() {
				e = p.delete(store, bMD, bData, bIDS, bLRU, access, []byte(md.ID), md.LRU)
				if e != nil {
					return
				}
				err = sessionid.ErrTokenNotExists
				return
			} else if md.Refresh != refresh {
				err = sessionid.ErrRefreshTokenNotMatched
				return
			}
			pairs, e := p.getPairs(bData, access)
			if e != nil {
				return
			}
			e = p.delete(store, bMD, bData, bIDS, bLRU, access, []byte(md.ID), md.LRU)
			if e != nil {
				return
			}
			// new
			md.DoRefresh(newAccess, newRefresh, p.opts.access, p.opts.refresh)
			md.LRU, e = p.putLRU(bLRU, key)
			if e != nil {
				return
			}
			e = p.putMetadata(bMD, key, md)
			if e != nil {
				return
			}
			e = p.putData(bData, key, pairs)
			if e != nil {
				return
			}
			e = p.appendIDS(bIDS, []byte(id), newAccess)
			if e != nil {
				return
			}
			e = p.increment(store)
			if e != nil {
				return
			}
			return
		})
		if result != nil {
			err = result
		}
	} else {
		err = sessionid.ErrProviderClosed
	}
	p.m.Unlock()
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
func (p *Provider) debugCount() (count int, e error) {
	e = p.db.View(func(t *bolt.Tx) (e error) {
		store := p.store(t)
		count = int(p.count(store))
		return
	})
	return
}
func (p *Provider) deleteBucket(bucket *bolt.Bucket) (e error) {
	cursor := bucket.Cursor()
	for k, v := cursor.First(); k != nil; k, v = cursor.Next() {
		if v == nil {
			e = bucket.DeleteBucket(k)
		} else {
			e = bucket.Delete(k)
		}
		if e != nil {
			break
		}
	}
	return
}
func (p *Provider) Clear() (e error) {
	p.m.Lock()
	if p.done == 0 {
		e = p.db.Update(func(t *bolt.Tx) (e error) {
			var (
				store                  = p.store(t)
				bMD, bData, bIDS, bLRU = p.buckets(store)
			)
			e = store.Delete(keyCount)
			if e != nil {
				return
			}
			e = p.deleteBucket(bMD)
			if e != nil {
				return
			}
			e = p.deleteBucket(bData)
			if e != nil {
				return
			}
			e = p.deleteBucket(bIDS)
			if e != nil {
				return
			}
			e = p.deleteBucket(bLRU)
			if e != nil {
				return
			}
			return
		})

	} else {
		e = sessionid.ErrProviderClosed
		return
	}
	p.m.Unlock()
	return
}
