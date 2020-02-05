package xroute

import "github.com/go-errors/errors"

var (
	ErrDuplicateHandler = errors.New("duplicate handler")
	ErrNoHandlers       = errors.New("attempting to route to a mux with no handlers.")
)
