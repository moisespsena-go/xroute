package xroute

import (
	"github.com/moisespsena-go/path-helpers"
	"net/http"

	"github.com/moisespsena-go/logging"
)

var (
	RequestLoggerFactory = DefaultRequestLoggerFactory
	log = logging.GetOrCreateLogger(path_helpers.GetCalledDir())
)

func NewLogger(host string) logging.Logger {
	return logging.WithPrefix(log, host)
}

func DefaultRequestLoggerFactory(r *http.Request, ctx *RouteContext) logging.Logger {
	return logging.WithPrefix(NewLogger(r.Host), r.RemoteAddr)
}
