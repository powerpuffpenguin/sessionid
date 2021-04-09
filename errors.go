package sessionid

import "errors"

var (
	ErrInvalidToken           = errors.New(`invalid token`)
	ErrProviderReturnNotMatch = errors.New(`provider return not matched`)

	ErrNeedsPointer     = errors.New(`needs a pointer to a value`)
	ErrPointerToPointer = errors.New(`a pointer to a pointer is not allowed`)
	ErrKeyNotExists     = errors.New(`key not exists`)
)
