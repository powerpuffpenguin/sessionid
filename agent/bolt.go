package agent

import (
	"bytes"
	"context"
	"encoding/gob"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/RussellLuo/timingwheel"
	"github.com/boltdb/bolt"
	"github.com/powerpuffpenguin/sessionid/cryptoer"
)

var bucketName = []byte(`sessionid`)

type boltElement struct {
	SessionID string
	Timer     *timingwheel.Timer
}
type boltData struct {
	Data     string
	Deadline time.Time
}

func (d *boltData) Marshal() (b []byte, e error) {
	var w bytes.Buffer
	e = gob.NewEncoder(&w).Encode(d)
	if e != nil {
		return
	}
	b = w.Bytes()
	return
}
func (d *boltData) Unmarshal(b []byte) (e error) {
	e = gob.NewDecoder(bytes.NewBuffer(b)).Decode(d)
	if e != nil {
		return
	}
	return
}

// BoltAgent like MemoryAgent but store on filesystem
type BoltAgent struct {
	opts  options
	token map[string]boltElement
	wheel *timingwheel.TimingWheel
	db    *bolt.DB
	done  uint32
	m     sync.Mutex
}

func join(id, sessionid string) string {
	return id + `.` + sessionid
}
func split(value string) (id, sessionid string, ok bool) {
	index := strings.LastIndex(value, `.`)
	if index < 1 {
		return
	}
	id = value[:index]
	sessionid = value[index+1:]
	ok = true
	return
}

func NewBoltAgent(db *bolt.DB, opt ...Option) (agent *BoltAgent, e error) {
	opts := defaultOptions
	for _, o := range opt {
		o.apply(&opts)
	}

	wheel := timingwheel.NewTimingWheel(opts.wheelTick, opts.wheelSize)
	wheel.Start()
	agent = &BoltAgent{
		opts:  opts,
		token: make(map[string]boltElement),
		wheel: wheel,
		db:    db,
	}
	e = agent.restore()
	if e != nil {
		agent.Close()
		agent = nil
	}
	return
}
func (a *BoltAgent) restore() (e error) {
	return a.doSlow(func() error {
		return a.db.Update(func(t *bolt.Tx) error {
			bucket, e := t.CreateBucketIfNotExists(bucketName)
			if e != nil {
				return e
			}
			cursor := bucket.Cursor()
			for k, v := cursor.First(); k != nil; k, v = cursor.Next() {
				if v == nil {
					e = bucket.Delete(k)
					if e != nil {
						return e
					}
					continue
				}
				var tmp boltData
				e = tmp.Unmarshal(v)
				if e != nil {
					e = bucket.Delete(k)
					if e != nil {
						return e
					}
					continue
				}
				var duration time.Duration
				if !tmp.Deadline.IsZero() {
					duration = time.Since(tmp.Deadline)
				}
				if duration < time.Second {
					e = bucket.Delete(k)
					if e != nil {
						return e
					}
					continue
				}
				id, sessionid, ok := split(string(k))
				if !ok {
					e = bucket.Delete(k)
					if e != nil {
						return e
					}
					continue
				}
				ele := boltElement{
					SessionID: sessionid,
				}
				ele.Timer = a.wheel.AfterFunc(duration, func() {
					a.remove(id, sessionid, false)
				})
				a.token[id] = ele
			}
			return nil
		})
	})
}

// Create a token for id.
// userdata is the custom data associated with the token.
// expiration is the token expiration time.
func (a *BoltAgent) Create(ctx context.Context, id, userdata string, expiration time.Duration) (token string, e error) {
	sessionid, e := a.opts.sessionid()
	if e != nil {
		return
	}
	token, e = cryptoer.Encode(a.opts.signingMethod, a.opts.signingKey, id, sessionid)
	if e != nil {
		return
	}

	ele := boltElement{
		SessionID: sessionid,
	}
	e = a.doSlow(func() error {
		tmp := boltData{
			Data:     userdata,
			Deadline: time.Now().Add(expiration),
		}
		v, e := tmp.Marshal()
		if e != nil {
			return e
		}
		e = a.db.Update(func(t *bolt.Tx) error {
			bucket := t.Bucket(bucketName)
			if bucket == nil {
				return bolt.ErrBucketNotFound
			}
			return bucket.Put([]byte(join(id, sessionid)), v)
		})
		if e != nil {
			return e
		}
		ele.Timer = a.wheel.AfterFunc(expiration, func() {
			a.remove(id, ele.SessionID, false)
		})
		a.token[id] = ele
		return nil
	})
	return
}

// Remove a exists token
func (a *BoltAgent) Remove(ctx context.Context, token string) (exists bool, e error) {
	id, sessionid, e := cryptoer.Decode(a.opts.signingMethod, a.opts.signingKey, token)
	if e != nil {
		return
	}
	exists, e = a.remove(id, sessionid, true)
	return
}
func (a *BoltAgent) remove(id, sessionid string, stop bool) (exists bool, e error) {
	e = a.doSlow(func() error {
		if current, ok := a.token[id]; ok && current.SessionID == sessionid {
			err := a.db.Update(func(t *bolt.Tx) error {
				bucket := t.Bucket(bucketName)
				if bucket == nil {
					return bolt.ErrBucketNotFound
				}
				return bucket.Delete([]byte(join(id, sessionid)))
			})
			if err != nil {
				return err
			}
			if stop {
				current.Timer.Stop()
			}
			exists = true
			delete(a.token, id)
		}
		return nil
	})
	return
}

// RemoveByID remove token by id
func (a *BoltAgent) RemoveByID(ctx context.Context, id string) (exists bool, e error) {
	e = a.doSlow(func() error {
		if current, ok := a.token[id]; ok {
			err := a.db.Update(func(t *bolt.Tx) error {
				bucket := t.Bucket(bucketName)
				if bucket == nil {
					return bolt.ErrBucketNotFound
				}
				return bucket.Delete([]byte(join(id, current.SessionID)))
			})
			if err != nil {
				return err
			}

			current.Timer.Stop()
			exists = true
			delete(a.token, id)
		}
		return nil
	})
	return
}

// Get the userdata associated with the token
func (a *BoltAgent) Get(ctx context.Context, token string) (id, userdata string, exists bool, e error) {
	id, sessionid, e := cryptoer.Decode(a.opts.signingMethod, a.opts.signingKey, token)
	if e != nil {
		return
	}
	a.doSlow(func() error {
		current, ok := a.token[id]
		if ok && current.SessionID == sessionid {
			e = a.db.View(func(t *bolt.Tx) error {
				bucket := t.Bucket(bucketName)
				if bucket == nil {
					return bolt.ErrBucketNotFound
				}
				key := bucket.Get([]byte(join(id, current.SessionID)))
				if key == nil {
					return nil
				}
				var tmp boltData
				err := tmp.Unmarshal(key)
				if err != nil {
					return err
				}
				exists = true
				userdata = tmp.Data
				return nil
			})
		}
		return nil
	})
	return
}

// // SetExpiry set the token expiration time.
// func (a *MemoryAgent) SetExpiry(ctx context.Context, token string, expiration time.Duration) (exists bool, e error) {
// 	id, sessionid, e := cryptoer.Decode(a.opts.signingMethod, a.opts.signingKey, token)
// 	if e != nil {
// 		return
// 	}
// 	a.doSlow(func() error {
// 		t, ok := a.token[id]
// 		if ok && t.SessionID == sessionid {
// 			exists = true

// 			if expiration <= 0 {
// 				t.Timer.Stop()
// 				delete(a.token, id)
// 			} else {
// 				t.Timer.Stop()
// 				t.Timer = a.wheel.AfterFunc(expiration, func() {
// 					e = a.doSlow(func() error {
// 						if current, exists := a.token[id]; exists && current.SessionID == sessionid {
// 							delete(a.token, id)
// 						}
// 						return nil
// 					})
// 				})
// 				a.token[id] = t
// 			}
// 		}
// 		return nil
// 	})
// 	return
// }

func (a *BoltAgent) doSlow(f func() error) error {
	if atomic.LoadUint32(&a.done) == 0 {
		a.m.Lock()
		defer a.m.Unlock()
		if a.done == 0 {
			return f()
		}
	}
	return ErrAgentClosed
}
func (a *BoltAgent) Close() error {
	if atomic.LoadUint32(&a.done) == 0 {
		a.m.Lock()
		defer a.m.Unlock()
		if a.done == 0 {
			defer atomic.StoreUint32(&a.done, 1)
			a.wheel.Stop()
			a.db.Close()
			return nil
		}
	}
	return ErrAgentClosed
}
