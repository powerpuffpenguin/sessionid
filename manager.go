package sessionid

import (
	"context"
	"strings"
)

const (
	Separator    = '.'
	sessionidLen = 16 + 4 // uuid + 4bytes rand
)

func Join(elem ...string) string {
	return strings.Join(elem, string(Separator))
}
func IsToken(token ...string) (e error) {
	for _, str := range token {
		_, _, _, e = Split(str)
		if e != nil {
			break
		}
	}
	return
}
func Split(token string) (id, sessionid, signature string, e error) {
	sep := string(Separator)
	index := strings.LastIndex(token, sep)
	if index == -1 {
		e = ErrTokenInvalid
		return
	}
	signature = token[index+1:]
	signingString := token[:index]
	index = strings.Index(signingString, sep)
	if index == -1 {
		e = ErrTokenInvalid
		return
	}
	id = signingString[:index]
	sessionid = signingString[index+1:]
	return
}

type Manager struct {
	opts options
}

func NewManager(opt ...Option) *Manager {
	opts := defaultOptions
	for _, o := range opt {
		o.apply(&opts)
	}

	return &Manager{
		opts: opts,
	}
}

// Create a session for the user
//
// * id uid or web-uid or mobile-uid or ...
//
// * pair session init key value
func (m *Manager) Create(ctx context.Context,
	id string,
	pair ...Pair,
) (session *Session, refresh string, e error) {
	eid := encode([]byte(id))
	access, refresh, e := m.create(eid)
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
func (m *Manager) create(id string) (access, refresh string, e error) {
	access, e = m.createToken(id)
	if e != nil {
		return
	}
	refresh, e = m.createToken(id)
	if e != nil {
		return
	}
	return
}
func (m *Manager) createToken(id string) (token string, e error) {
	sessionid, e := newSessionID()
	if e != nil {
		return
	}
	signingString := Join(id, sessionid)
	signature, e := m.opts.method.Sign(signingString, m.opts.key)
	if e != nil {
		return
	}
	token = Join(signingString, signature)
	return
}

// Destroy a session by id
func (m *Manager) Destroy(ctx context.Context, id string) error {
	return m.opts.provider.Destroy(ctx, encode([]byte(id)))
}

// Destroy a session by token
func (m *Manager) DestroyByToken(ctx context.Context, token string) error {
	return m.opts.provider.DestroyByToken(ctx, token)
}

// Get session from token
func (m *Manager) Get(token string) (s *Session, e error) {
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
func (m *Manager) Refresh(ctx context.Context, access, refresh string) (newAccess, newRefresh string, e error) {
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

	newAccess, newRefresh, e = m.create(id)
	if e != nil {
		return
	}
	e = m.opts.provider.Refresh(ctx, access, refresh, newAccess, newRefresh)
	return
}
