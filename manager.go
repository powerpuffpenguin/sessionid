package sessionid

import (
	"context"
)

type Manager interface {
	// Create a session for the user
	//
	// * id uid or web-uid or mobile-uid or ...
	//
	// * pair session init key value
	Create(ctx context.Context,
		id string,
		pair ...Pair,
	) (session *Session, refresh string, e error)
	// Destroy a session by id
	Destroy(ctx context.Context, id string) error
	// Destroy a session by token
	DestroyByToken(ctx context.Context, token string) error
	// Get session from token
	Get(ctx context.Context, token string) (s *Session, e error)
	// Refresh a new access token
	Refresh(ctx context.Context, access, refresh string) (newAccess, newRefresh string, e error)
}

type LocalManager struct {
	opts options
}

func NewManager(opt ...Option) *LocalManager {
	opts := defaultOptions
	for _, o := range opt {
		o.apply(&opts)
	}

	return &LocalManager{
		opts: opts,
	}
}

// Create a session for the user
//
// * id uid or web-uid or mobile-uid or ...
//
// * pair session init key value
func (m *LocalManager) Create(ctx context.Context,
	id string,
	pair ...Pair,
) (session *Session, refresh string, e error) {
	eid := encode([]byte(id))
	access, refresh, e := CreateToken(m.opts.method, m.opts.key, eid)
	if e != nil {
		return
	}
	opts := m.opts
	provider := opts.provider
	coder := opts.coder
	var kv []PairBytes
	count := len(pair)
	if count != 0 {
		kv = make([]PairBytes, count)
		var v []byte
		for i := 0; i < count; i++ {
			v, e = coder.Encode(pair[i].Value)
			if e != nil {
				return
			}
			kv[i] = PairBytes{
				Key:   pair[i].Key,
				Value: v,
			}
		}
	}
	e = provider.Create(ctx, access, refresh, kv)
	if e != nil {
		return
	}
	session = newSession(eid, access, provider, coder)
	return
}

// Destroy a session by id
func (m *LocalManager) Destroy(ctx context.Context, id string) error {
	return m.opts.provider.Destroy(ctx, encode([]byte(id)))
}

// Destroy a session by token
func (m *LocalManager) DestroyByToken(ctx context.Context, token string) error {
	return m.opts.provider.DestroyByToken(ctx, token)
}

// Get session from token
func (m *LocalManager) Get(ctx context.Context, token string) (s *Session, e error) {
	id, _, signature, e := Split(token)
	if e != nil {
		return
	}
	e = m.opts.method.Verify(token[:len(token)-len(signature)-1], signature, m.opts.key)
	if e != nil {
		return
	}
	s = newSession(id, token, m.opts.provider, m.opts.coder)
	return
}

// Refresh a new access token
func (m *LocalManager) Refresh(ctx context.Context, access, refresh string) (newAccess, newRefresh string, e error) {
	id, _, signature, e := Split(access)
	if e != nil {
		return
	}
	e = m.opts.method.Verify(access[:len(access)-len(signature)-1], signature, m.opts.key)
	if e != nil {
		return
	}
	id1, _, signature, e := Split(refresh)
	if e != nil {
		return
	}
	e = m.opts.method.Verify(refresh[:len(refresh)-len(signature)-1], signature, m.opts.key)
	if e != nil {
		return
	}
	if id != id1 {
		e = ErrTokenIDNotMatched
		return
	}

	newAccess, newRefresh, e = CreateToken(m.opts.method, m.opts.key, id)
	if e != nil {
		return
	}
	e = m.opts.provider.Refresh(ctx, access, refresh, newAccess, newRefresh)
	return
}
