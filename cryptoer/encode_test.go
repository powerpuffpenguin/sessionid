package cryptoer_test

import (
	"strconv"
	"testing"

	"github.com/powerpuffpenguin/sessionid/cryptoer"
	"github.com/stretchr/testify/assert"
)

func TestSessionID(t *testing.T) {
	count := 10000
	keys := make(map[string]bool, count)
	for i := 0; i < count; i++ {
		id, e := cryptoer.SessionID()
		assert.Nil(t, e)
		assert.False(t, keys[id])
		keys[id] = true
	}
}
func TestEncode(t *testing.T) {
	methods := []cryptoer.SigningMethod{
		cryptoer.SigningMethodHMD5,
		cryptoer.SigningMethodHS1,
		cryptoer.SigningMethodHS256,
		cryptoer.SigningMethodHS384,
		cryptoer.SigningMethodHS512,
	}
	key := []byte(`cerberus is an idea`)
	count := 10000
	var method cryptoer.SigningMethod
	for i := 0; i < count; i++ {
		method = methods[i%len(methods)]

		id := strconv.Itoa(i)
		sesionid, e := cryptoer.SessionID()
		assert.Nil(t, e)
		token, e := cryptoer.Encode(method, key, id, sesionid)
		assert.Nil(t, e)
		decid, decsesionid, e := cryptoer.Decode(method, key, token)
		assert.Nil(t, e)
		assert.Equal(t, id, decid)
		assert.Equal(t, sesionid, decsesionid)
	}
}
