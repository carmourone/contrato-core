package storage

import "errors"

var (
	ErrUnsupported = errors.New("storage: unsupported")
	ErrNotFound    = errors.New("storage: not found")
	ErrConflict    = errors.New("storage: conflict")
)
