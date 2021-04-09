package sessionid

import (
	"context"
	"reflect"
	"time"
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
	id        string
	sessionid string
	token     string
	provider  Provider
	coder     Coder
	keys      map[interface{}]value
}

func newSession(id, sessionid, token string, provider Provider, coder Coder) *Session {
	return &Session{
		id:        id,
		sessionid: sessionid,
		token:     token,
		provider:  provider,
		coder:     coder,
	}
}

// Destroy session
func (s *Session) Destroy(ctx context.Context) error {
	return s.provider.DestroyByToken(ctx, s.token)
}

// IsValid return true if session not expired
func (s *Session) IsValid(ctx context.Context) (bool, error) {
	return s.provider.IsValid(ctx, s.token)
}

// SetExpiry set the token expiration time.
func (s *Session) SetExpiry(ctx context.Context, token string, expiration time.Duration) (exists bool, e error) {
	return s.provider.SetExpiry(ctx, token, expiration)
}

// Set key value for token
func (s *Session) Set(ctx context.Context, pair ...interface{}) (e error) {
	count := len(pair)
	if count == 0 {
		return
	}
	kv := make([][]byte, count)
	var b []byte
	for i := 0; i < count; i++ {
		b, e = s.coder.Encode(pair[i])
		if e != nil {
			return
		}
		kv = append(kv, b)
	}
	e = s.provider.Set(ctx, s.token, kv)
	if e != nil {
		return
	}
	if s.keys == nil {
		return
	}
	for i := 0; i < count; i += 2 {
		k := kv[i]
		val := Value{
			Exists: true,
		}
		if i+1 < count {
			val.Bytes = kv[i+1]
		}
		s.keys[k] = value{
			Value: val,
		}
	}
	return
}

// Prepare get key's value from provider to local cache
func (s *Session) Prepare(ctx context.Context, key ...interface{}) (e error) {
	count := len(key)
	if count == 0 {
		return
	}
	k := make([][]byte, count)
	var b []byte
	for i := 0; i < count; i++ {
		b, e = s.coder.Encode(key[i])
		if e != nil {
			return
		}
		k = append(k, b)
	}
	vals, e := s.provider.Get(ctx, s.token, k)
	if e != nil {
		return
	}
	if len(vals) != count {
		e = ErrProviderReturnNotMatch
		return
	}
	if s.keys == nil {
		s.keys = make(map[interface{}]value)
	}
	for i := 0; i < count; i++ {
		s.keys[key[i]] = value{
			Value: vals[i],
		}
	}
	return
}
func (s *Session) getKey(key interface{}) (val value, exists bool) {
	if s.keys == nil {
		return
	}
	val, exists = s.keys[key]
	return
}
func (s *Session) Get(key interface{}, pointer interface{}) (e error) {
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
