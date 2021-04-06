package agent

import (
	"context"
	"sync"
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
	opts   options
	token  map[string]memoryElement
	wheel  *timingwheel.TimingWheel
	locker sync.Mutex
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
// maxage is the token expiration time in seconds .
func (a *MemoryAgent) Create(ctx context.Context, id, userdata string, maxage int32) (token string, e error) {
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
	a.locker.Lock()
	defer a.locker.Unlock()
	a.token[id] = ele
	ele.Timer = a.wheel.AfterFunc(time.Second*time.Duration(maxage), func() {
		a.locker.Lock()
		defer a.locker.Unlock()
		if current, exists := a.token[id]; exists && current.SessionID == ele.SessionID {
			delete(a.token, id)
		}
	})
	return
}

// Remove a exists token
func (a *MemoryAgent) Remove(ctx context.Context, token string) (exists bool, e error) {
	id, sessionid, e := cryptoer.Decode(a.opts.signingMethod, a.opts.signingKey, token)
	if e != nil {
		return
	}
	a.locker.Lock()
	defer a.locker.Unlock()
	if t, ok := a.token[id]; ok && t.SessionID == sessionid {
		t.Timer.Stop()
		exists = true
		delete(a.token, id)
	}
	return
}

// RemoveByID remove token by id
func (a *MemoryAgent) RemoveByID(ctx context.Context, id string) (exists bool, e error) {
	a.locker.Lock()
	defer a.locker.Unlock()
	if t, ok := a.token[id]; ok {
		t.Timer.Stop()
		exists = true
		delete(a.token, id)
	}
	return
}

// Get the userdata associated with the token
//
// if maxage > 0 then reset the expiration time
//
// if maxage < 0 then expire immediately after returning
func (a *MemoryAgent) Get(ctx context.Context, token string, maxage int32) (id, userdata string, exists bool, e error) {
	id, sessionid, e := cryptoer.Decode(a.opts.signingMethod, a.opts.signingKey, token)
	if e != nil {
		return
	}
	a.locker.Lock()
	defer a.locker.Unlock()

	if t, ok := a.token[id]; ok && t.SessionID == sessionid {
		if maxage != 0 {
			t.Timer.Stop()
		}
		exists = true
		userdata = t.Data
		if maxage < 0 {
			delete(a.token, id)
		} else if maxage > 0 {
			t.Timer = a.wheel.AfterFunc(time.Second*time.Duration(maxage), func() {
				a.locker.Lock()
				defer a.locker.Unlock()
				if current, exists := a.token[id]; exists && current.SessionID == sessionid {
					delete(a.token, id)
				}
			})
		}
	}
	return
}
