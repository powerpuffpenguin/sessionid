package sessionid

import (
	"errors"
)

var (
	ErrTokenExpired           = errors.New(`token expired `) // http code 401
	ErrTokenInvalid           = errors.New(`token invalid`)
	ErrTokenNotExists         = errors.New(`token not exists`)
	ErrTokenIDNotMatched      = errors.New(`token id not matched`)
	ErrRefreshTokenNotMatched = errors.New(`refresh token not matched`)
	ErrProviderReturnNotMatch = errors.New(`provider return not matched`)
	ErrProviderClosed         = errors.New(`provider already closed`)
	ErrNeedsPointer           = errors.New(`needs a pointer to a value`)
	ErrPointerToPointer       = errors.New(`a pointer to a pointer is not allowed`)
	ErrKeyNotExists           = errors.New(`key not exists`)
)

func IsTokenExpired(e error) bool {
	return errors.Is(e, ErrTokenInvalid)
}
