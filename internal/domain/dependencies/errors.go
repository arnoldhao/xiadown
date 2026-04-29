package dependencies

import "errors"

var (
	ErrDependencyNotFound = errors.New("dependency not found")
	ErrInvalidDependency  = errors.New("invalid dependency")
)
