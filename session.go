package sessionid

import (
	"context"
	"reflect"
)

type Value struct {
	Bytes  []byte
	Exists bool
}
type value struct {
	Value
	Elem  reflect.Value
	Ready bool
}

type Session struct {
	id       string
	token    string
	provider Provider
	coder    Coder
	keys     map[string]value
}

func (s *Session) ID() string {
	return s.id
}
func (s *Session) Token() string {
	return s.token
}

func newSession(m *Manager, id, token string, provider Provider, coder Coder) *Session {
	return &Session{
		id:       id,
		token:    token,
		provider: provider,
		coder:    coder,
	}
}

// Destroy session
func (s *Session) Destroy(ctx context.Context) error {
	return s.provider.DestroyByToken(ctx, s.token)
}

// Check token status
func (s *Session) Check(ctx context.Context) error {
	return s.provider.Check(ctx, s.token)
}

// Put key value for token
func (s *Session) Put(ctx context.Context, pair ...Pair) (e error) {
	count := len(pair)
	if count == 0 {
		return
	}
	kv := make([]PairBytes, count)
	var b []byte
	for i := 0; i < count; i++ {
		b, e = s.coder.Encode(pair[i].Value)
		if e != nil {
			return
		}
		kv = append(kv, PairBytes{
			Key:   pair[i].Key,
			Value: b,
		})
	}
	e = s.provider.Put(ctx, s.token, kv)
	if e != nil {
		return
	}
	if s.keys == nil {
		return
	}
	for i := 0; i < count; i++ {
		val := Value{
			Exists: true,
			Bytes:  kv[i].Value,
		}
		s.keys[kv[i].Key] = value{
			Value: val,
		}
	}
	return
}

// Keys return all token's key
func (s *Session) Keys(ctx context.Context) (key []string, e error) {
	return s.provider.Keys(ctx, s.token)
}

// Prepare get key's value from provider to local cache
func (s *Session) Prepare(ctx context.Context, key ...string) (e error) {
	count := len(key)
	if count == 0 {
		return
	}
	vals, e := s.provider.Get(ctx, s.token, key)
	if e != nil {
		return
	}
	if len(vals) != count {
		e = ErrProviderReturnNotMatch
		return
	}
	if s.keys == nil {
		s.keys = make(map[string]value)
	}
	for i := 0; i < count; i++ {
		s.keys[key[i]] = value{
			Value: vals[i],
		}
	}
	return
}
func (s *Session) getKey(key string) (val value, exists bool) {
	if s.keys == nil {
		return
	}
	val, exists = s.keys[key]
	return
}

// Get key's value
func (s *Session) Get(key string, pointer interface{}) (e error) {
	vo := reflect.ValueOf(pointer)
	if vo.Kind() != reflect.Ptr {
		e = ErrNeedsPointer
		return
	} else if vo.Elem().Kind() == reflect.Ptr {
		e = ErrPointerToPointer
		return
	}

	if v, ok := s.getKey(key); ok {
		if v.Exists {
			if v.Ready {
				vo.Elem().Set(v.Elem.Elem())
			} else {
				e = s.coder.Decode(v.Bytes, pointer)
				if e != nil {
					return
				}
				v.Ready = true
				v.Bytes = nil
				p := reflect.New(vo.Elem().Type())
				p.Elem().Set(vo.Elem())
				v.Elem = p
				s.keys[key] = v
			}
			return
		}
		e = ErrKeyNotExists
		return
	}
	return
}

// Delete keys
func (s *Session) Delete(ctx context.Context, token string, key ...string) (e error) {
	return s.provider.Delete(ctx, token, key)
}
