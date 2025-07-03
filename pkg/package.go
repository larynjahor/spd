package pkg

import "errors"

var (
	ErrPackageNotFound = errors.New("package not found")
	ErrDirNotResolved  = errors.New("cannot resolve directory")
	ErrIgnore          = errors.New("ignore")
	ErrUnimplemented   = errors.New("unimplemented")
)
