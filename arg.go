package xroute

import "net/http"

type Handler interface {
	http.Handler
	ContextHandler
}

type HttpContextHandler struct {
	ContextHandler
	http.Handler
}
