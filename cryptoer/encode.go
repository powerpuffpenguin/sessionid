package cryptoer

import (
	"crypto/rand"
	"encoding/base64"
	"strings"

	"github.com/google/uuid"
)

func SessionID() (sessionid string, e error) {
	b := make([]byte, 16+2)
	u, e := uuid.NewUUID()
	if e != nil {
		return
	}
	copy(b, u[:])
	_, e = rand.Read(b[16:])
	if e != nil {
		return
	}
	sessionid = base64.URLEncoding.EncodeToString(b)
	return
}
func Encode(method SigningMethod, key []byte, id, sessionid string) (token string, e error) {
	value := id + `.` + sessionid
	sig, e := method.Sign(value, key)
	if e != nil {
		return
	}
	token = value + `.` + sig
	return
}
func Decode(method SigningMethod, key []byte, token string) (id, sessionid string, e error) {
	index := strings.LastIndex(token, `.`)
	if index < 1 {
		e = ErrInvalidToken
		return
	}
	value := token[:index]
	i := strings.LastIndex(value, `.`)
	if i < 0 {
		e = ErrInvalidToken
		return
	}

	e = method.Verify(value, token[index+1:], key)
	if e != nil {
		return
	}

	id = value[:i]
	sessionid = value[i+1:]
	return
}
