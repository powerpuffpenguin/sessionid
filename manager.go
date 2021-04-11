package sessionid

import (
	"context"
	"strings"
	"time"
)

const (
	Separator    = '.'
	sessionidLen = 16 + 4 // uuid + 4bytes rand
)

func Join(elem ...string) string {
	return strings.Join(elem, string(Separator))
}
func Split(token string) (id, sessionid, signature string, e error) {
	sep := string(Separator)
	index := strings.LastIndex(token, sep)
	if index == -1 {
		e = ErrInvalidToken
		return
	}
	signature = token[index+1:]
	signingString := token[:index]
	index = strings.Index(signingString, sep)
	if index == -1 {
		e = ErrInvalidToken
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
// * expiration how long does the session expire
//
// * pair session init key value
func (m *Manager) Create(ctx context.Context,
	id interface{},
	expiration time.Duration,
	pair ...Pair,
) (session *Session, token string, e error) {
	sessionid, e := newSessionID()
	if e != nil {
		return
	}
	opts := m.opts
	provider := opts.provider
	coder := opts.coder
	b, e := coder.Encode(id)
	if e != nil {
		return
	}
	eid := encode(b)
	signingString := Join(eid, sessionid)
	signature, e := opts.method.Sign(signingString, opts.key)
	if e != nil {
		return
	}
	t := Join(signingString, signature)
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
	e = provider.Create(ctx, token, expiration, kv)
	if e != nil {
		return
	}
	session = newSession(eid, sessionid, t, provider, coder)
	token = t
	return
}

// Destroy a session by id
func (m *Manager) Destroy(ctx context.Context, id interface{}) error {
	b, e := m.opts.coder.Encode(id)
	if e != nil {
		return e
	}
	return m.opts.provider.Destroy(ctx, encode(b))
}

// Destroy a session by token
func (m *Manager) DestroyByToken(ctx context.Context, token string) error {
	return m.opts.provider.DestroyByToken(ctx, token)
}

func (m *Manager) Get(token string) (s *Session, e error) {
	id, sessionid, signature, e := Split(token)
	if e != nil {
		return
	}
	e = m.opts.method.Verify(token[:len(token)-len(signature)-1], signature, m.opts.key)
	if e != nil {
		return
	}
	s = newSession(id, sessionid, token, m.opts.provider, m.opts.coder)
	return
}
