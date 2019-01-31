package xroute

import "github.com/go-errors/errors"

type TracedError interface {
	error
	Trace() []byte
}

var (
	ErrDuplicateHandler = errors.New("duplicate handler")
	ErrNoHandlers       = errors.New("attempting to route to a mux with no handlers.")
)
