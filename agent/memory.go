package agent

import (
	"context"
	"sync"
	"time"

	"github.com/RussellLuo/timingwheel"
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

func NewMemory(opt ...Option) *MemoryAgent {
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
func (a *MemoryAgent) Create(ctx context.Context, id, userdata string, maxage int32) (token string, e error) {
	sessionid, e := a.opts.cryptoer.SessionID()
	if e != nil {
		return
	}
	token, e = a.opts.cryptoer.Encode(id, sessionid)
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

func (a *MemoryAgent) Remove(ctx context.Context, token string) (exists bool, e error) {
	id, sessionid, e := a.opts.cryptoer.Decode(token)
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
func (a *MemoryAgent) Get(ctx context.Context, token string, maxage int32) (userdata string, exists bool, e error) {
	id, sessionid, e := a.opts.cryptoer.Decode(token)
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
