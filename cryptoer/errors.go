package cryptoer

import "errors"

// Error constants
var (
	ErrInvalidKey      = errors.New("key is invalid")
	ErrInvalidToken    = errors.New("token is invalid")
	ErrInvalidKeyType  = errors.New("key is of invalid type")
	ErrHashUnavailable = errors.New("the requested hash function is unavailable")
)
