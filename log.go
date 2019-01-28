package route

import (
	"net/http"
	"os"

	"github.com/op/go-logging"
)

var (
	Format               = logging.MustStringFormatter(`%{color}%{time:15:04:05.000} %{level:.4s} [%{module}]%{color:reset} %{message}`)
	Backend              = logging.NewLogBackend(os.Stderr, "", 0)
	backendFormatter     logging.Backend
	RequestLoggerFactory = DefaultRequestLoggerFactory
)

func InitLogger() {
	backendFormatter := logging.NewBackendFormatter(Backend, Format)
	logging.SetBackend(backendFormatter)
}

func NewLogger(module string) *logging.Logger {
	if backendFormatter == nil {
		InitLogger()
	}

	return logging.MustGetLogger(module)
}

func DefaultRequestLoggerFactory(r *http.Request, ctx *RouteContext) *logging.Logger {
	return NewLogger(r.Host)
}
