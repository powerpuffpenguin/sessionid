package agent

import (
	"context"
	"sync"
	"sync/atomic"
	"time"

	"github.com/RussellLuo/timingwheel"
	"github.com/powerpuffpenguin/sessionid/cryptoer"
)

type memoryElement struct {
	SessionID string
	Data      string
	Timer     *timingwheel.Timer
}

// MemoryAgent sessionid agent store on memory
type MemoryAgent struct {
	opts  options
	token map[string]memoryElement
	wheel *timingwheel.TimingWheel
	done  uint32
	m     sync.Mutex
}

func NewMemoryAgent(opt ...Option) *MemoryAgent {
	opts := defaultOptions
	for _, o := range opt {
		o.apply(&opts)
	}

	wheel := timingwheel.NewTimingWheel(opts.wheelTick, opts.wheelSize)
	wheel.Start()
	return &MemoryAgent{
		opts:  opts,
		token: make(map[string]memoryElement),
		wheel: wheel,
	}
}

// Create a token for id.
// userdata is the custom data associated with the token.
// expiry is the token expiry  duration.
func (a *MemoryAgent) Create(ctx context.Context, id, userdata string, expiry time.Duration) (token string, e error) {
	sessionid, e := a.opts.sessionid()
	if e != nil {
		return
	}
	token, e = cryptoer.Encode(a.opts.signingMethod, a.opts.signingKey, id, sessionid)
	if e != nil {
		return
	}

	ele := memoryElement{
		SessionID: sessionid,
		Data:      userdata,
	}
	e = a.doSlow(func() error {
		a.token[id] = ele
		ele.Timer = a.wheel.AfterFunc(expiry, func() {
			a.doSlow(func() error {
				if current, exists := a.token[id]; exists && current.SessionID == ele.SessionID {
					delete(a.token, id)
				}
				return nil
			})
		})
		return nil
	})
	return
}

// Remove a exists token
func (a *MemoryAgent) Remove(ctx context.Context, token string) (exists bool, e error) {
	id, sessionid, e := cryptoer.Decode(a.opts.signingMethod, a.opts.signingKey, token)
	if e != nil {
		return
	}
	e = a.doSlow(func() error {
		if t, ok := a.token[id]; ok && t.SessionID == sessionid {
			t.Timer.Stop()
			exists = true
			delete(a.token, id)
		}
		return nil
	})
	return
}

// RemoveByID remove token by id
func (a *MemoryAgent) RemoveByID(ctx context.Context, id string) (exists bool, e error) {
	e = a.doSlow(func() error {
		if t, ok := a.token[id]; ok {
			t.Timer.Stop()
			exists = true
			delete(a.token, id)
		}
		return nil
	})
	return
}

// Get the userdata associated with the token
//
// if expiry > 0 then reset the expiration time
//
// if expiry < 0 then expire immediately after returning
func (a *MemoryAgent) Get(ctx context.Context, token string, expiry time.Duration) (id, userdata string, exists bool, e error) {
	id, sessionid, e := cryptoer.Decode(a.opts.signingMethod, a.opts.signingKey, token)
	if e != nil {
		return
	}
	var (
		t  memoryElement
		ok bool
	)
	e = a.doSlow(func() error {
		t, ok = a.token[id]
		return nil
	})

	if ok && t.SessionID == sessionid {
		if expiry != 0 {
			t.Timer.Stop()
		}
		exists = true
		userdata = t.Data
		if expiry < 0 {
			delete(a.token, id)
		} else if expiry > 0 {
			t.Timer = a.wheel.AfterFunc(expiry, func() {
				e = a.doSlow(func() error {
					if current, exists := a.token[id]; exists && current.SessionID == sessionid {
						delete(a.token, id)
					}
					return nil
				})
			})
		}
	}
	return
}
func (a *MemoryAgent) doSlow(f func() error) error {
	if atomic.LoadUint32(&a.done) == 0 {
		a.m.Lock()
		defer a.m.Unlock()
		if a.done == 0 {
			return f()
		}
	}
	return ErrAgentClosed
}
func (a *MemoryAgent) Close() (e error) {
	closed := true
	if atomic.LoadUint32(&a.done) == 0 {
		a.m.Lock()
		defer a.m.Unlock()
		if a.done == 0 {
			defer atomic.StoreUint32(&a.done, 1)
			a.wheel.Stop()
			closed = false
		}
	}
	if closed {
		e = ErrAgentClosed
	}
	return
}
