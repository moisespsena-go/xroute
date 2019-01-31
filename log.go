package xroute

import (
	"net/http"

	"github.com/op/go-logging"
)

var (
	RequestLoggerFactory = DefaultRequestLoggerFactory
)

func NewLogger(module string) *logging.Logger {
	return logging.MustGetLogger(module)
}

func DefaultRequestLoggerFactory(r *http.Request, ctx *RouteContext) *logging.Logger {
	return NewLogger(r.Host)
}
