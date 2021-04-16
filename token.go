package sessionid

import (
	"strings"

	"github.com/powerpuffpenguin/sessionid/cryptoer"
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
func CreateToken(method cryptoer.SigningMethod, key []byte, id string) (access, refresh string, e error) {
	access, e = createToken(method, key, id)
	if e != nil {
		return
	}
	refresh, e = createToken(method, key, id)
	if e != nil {
		return
	}
	return
}
func createToken(method cryptoer.SigningMethod, key []byte, id string) (token string, e error) {
	sessionid, e := newSessionID()
	if e != nil {
		return
	}
	signingString := Join(id, sessionid)
	signature, e := method.Sign(signingString, key)
	if e != nil {
		return
	}
	token = Join(signingString, signature)
	return
}
