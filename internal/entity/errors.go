package entity

import "errors"

var (
	ErrNotFound          = errors.New("resource not found")
	ErrDuplicateEmail    = errors.New("a user with that email already exists")
	ErrDuplicateUsername = errors.New("a user with that username already exists")
	ErrInvalidPassword   = errors.New("invalid password")
	ErrPasswordTooLong   = errors.New("password exceeds 72 bytes")
	ErrAccountInactive   = errors.New("account not activated")
	ErrEmailExists       = errors.New("email already registered")
)
