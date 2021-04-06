package agent

import (
	"sync"

	"github.com/RussellLuo/timingwheel"
	"github.com/boltdb/bolt"
)

type boltElement struct {
	SessionID string
	Data      string

	Timer *timingwheel.Timer
}

// BoltAgent sessionid agent store on bolt
type BoltAgent struct {
	opts   options
	db     *bolt.DB
	wheel  *timingwheel.TimingWheel
	locker sync.Mutex
}

func NewBoltAgent(path string, opt ...Option) (a *BoltAgent, e error) {
	opts := defaultOptions
	for _, o := range opt {
		o.apply(&opts)
	}
	db, e := bolt.Open(path, 0600, nil)
	if e != nil {
		return
	}

	wheel := timingwheel.NewTimingWheel(opts.wheelTick, opts.wheelSize)
	wheel.Start()
	a = &BoltAgent{
		opts:  opts,
		db:    db,
		wheel: wheel,
	}
// sync.Once
	return
}

// // Create a token for id.
// // userdata is the custom data associated with the token.
// // expiry is the token expiry  duration.
// func (a *MemoryAgent) Create(ctx context.Context, id, userdata string, expiry time.Duration) (token string, e error) {
// 	sessionid, e := a.opts.sessionid()
// 	if e != nil {
// 		return
// 	}
// 	token, e = cryptoer.Encode(a.opts.signingMethod, a.opts.signingKey, id, sessionid)
// 	if e != nil {
// 		return
// 	}

// 	ele := memoryElement{
// 		SessionID: sessionid,
// 		Data:      userdata,
// 	}
// 	a.locker.Lock()
// 	defer a.locker.Unlock()
// 	a.token[id] = ele
// 	ele.Timer = a.wheel.AfterFunc(expiry, func() {
// 		a.locker.Lock()
// 		defer a.locker.Unlock()
// 		if current, exists := a.token[id]; exists && current.SessionID == ele.SessionID {
// 			delete(a.token, id)
// 		}
// 	})
// 	return
// }

// // Remove a exists token
// func (a *MemoryAgent) Remove(ctx context.Context, token string) (exists bool, e error) {
// 	id, sessionid, e := cryptoer.Decode(a.opts.signingMethod, a.opts.signingKey, token)
// 	if e != nil {
// 		return
// 	}
// 	a.locker.Lock()
// 	defer a.locker.Unlock()
// 	if t, ok := a.token[id]; ok && t.SessionID == sessionid {
// 		t.Timer.Stop()
// 		exists = true
// 		delete(a.token, id)
// 	}
// 	return
// }

// // RemoveByID remove token by id
// func (a *MemoryAgent) RemoveByID(ctx context.Context, id string) (exists bool, e error) {
// 	a.locker.Lock()
// 	defer a.locker.Unlock()
// 	if t, ok := a.token[id]; ok {
// 		t.Timer.Stop()
// 		exists = true
// 		delete(a.token, id)
// 	}
// 	return
// }

// // Get the userdata associated with the token
// //
// // if expiry > 0 then reset the expiration time
// //
// // if expiry < 0 then expire immediately after returning
// func (a *MemoryAgent) Get(ctx context.Context, token string, expiry time.Duration) (id, userdata string, exists bool, e error) {
// 	id, sessionid, e := cryptoer.Decode(a.opts.signingMethod, a.opts.signingKey, token)
// 	if e != nil {
// 		return
// 	}
// 	a.locker.Lock()
// 	defer a.locker.Unlock()

// 	if t, ok := a.token[id]; ok && t.SessionID == sessionid {
// 		if expiry != 0 {
// 			t.Timer.Stop()
// 		}
// 		exists = true
// 		userdata = t.Data
// 		if expiry < 0 {
// 			delete(a.token, id)
// 		} else if expiry > 0 {
// 			t.Timer = a.wheel.AfterFunc(expiry, func() {
// 				a.locker.Lock()
// 				defer a.locker.Unlock()
// 				if current, exists := a.token[id]; exists && current.SessionID == sessionid {
// 					delete(a.token, id)
// 				}
// 			})
// 		}
// 	}
// 	return
// }
// func (a *MemoryAgent) Wheel() *timingwheel.TimingWheel {
// 	return a.wheel
// }
