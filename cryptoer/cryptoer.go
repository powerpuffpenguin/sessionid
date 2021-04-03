package cryptoer

import (
	"crypto/rand"
	"encoding/base64"

	"github.com/google/uuid"
)

var defautlCyptoer = &cyptoer{}

func DefautlCyptoer() Cryptoer {
	return defautlCyptoer
}

type Cryptoer interface {
	// generate unique session id
	SessionID() (sessionid string, e error)
	// Encode id sessionid
	Encode(id, sessionid string) (token string, e error)
	// Decode and return id sessionid
	Decode(token string) (id, sessionid string, e error)
}
type cyptoer struct {
}

// generate unique session id
func (c *cyptoer) SessionID() (sessionid string, e error) {
	b := make([]byte, 16+4)
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

// Encode id sessionid
func (c *cyptoer) Encode(id, sessionid string) (token string, e error) {
	return
}

// Decode and return id sessionid
func (c *cyptoer) Decode(token string) (id, sessionid string, e error) {
	return
}
