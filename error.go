package xroute

import (
	"fmt"

	"github.com/go-errors/errors"
)

var (
	ErrDuplicateHandler = errors.New("duplicate handler")
	ErrNoHandlers       = errors.New("attempting to route to a mux with no handlers.")
)

type BadPathern struct {
	pattern string
	message string
}

func (this BadPathern) Error() string {
	return fmt.Sprintf("bad route pattern %q: %s", this.pattern, this.message)
}
