package route

import "net/http"

type Handler interface {
	http.Handler
	ContextHandler
}