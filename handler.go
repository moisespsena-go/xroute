package xroute

import (
	"fmt"
	"net/http"
)

type ContextHandlerFunc func(handler ContextHandler, w http.ResponseWriter, r *http.Request, rctx *RouteContext)
type HandlerFunc func(w http.ResponseWriter, r *http.Request, rctx *RouteContext)

type ContextHandler interface {
	ServeHTTPContext(w http.ResponseWriter, r *http.Request, rctx *RouteContext)
}

func NewContextHandler(f func(w http.ResponseWriter, r *http.Request, rctx *RouteContext)) ContextHandler {
	return &RouteContextFuncHandler{f}
}

// http.HandlerFunc

type HTTPHandlerFuncGetter interface {
	Handler() http.HandlerFunc
}

type HTTPHandlerFunc struct {
	Value func(http.ResponseWriter, *http.Request)
}

func (h *HTTPHandlerFunc) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	h.Value(w, r)
}

func (h *HTTPHandlerFunc) Handler() http.HandlerFunc {
	return h.Value
}

func (h *HTTPHandlerFunc) ServeHTTPContext(w http.ResponseWriter, r *http.Request, rctx *RouteContext) {
	h.Value(w, r)
}

// http.Handler

type HTTPHandlerGetter interface {
	Handler() http.Handler
}

type HTTPHandler struct {
	Value http.Handler
}

func (h *HTTPHandler) Handler() http.Handler {
	return h.Value
}

func (h *HTTPHandler) ServeHTTPContext(w http.ResponseWriter, r *http.Request, rctx *RouteContext) {
	h.Value.ServeHTTP(w, r)
}

// RouteContextHandler

type RouteContextHandlerGetter interface {
	Handler() HandlerFunc
}

type RouteContextFuncHandler struct {
	Value func(http.ResponseWriter, *http.Request, *RouteContext)
}

func (h *RouteContextFuncHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	h.ServeHTTPContext(w, r, nil)
}

func (h *RouteContextFuncHandler) Handler() HandlerFunc {
	return h.Value
}

func (h *RouteContextFuncHandler) ServeHTTPContext(w http.ResponseWriter, r *http.Request, rctx *RouteContext) {
	h.Value(w, r, rctx)
}

type RouteContextArgHandler struct {
	Value func(*RouteContext)
}

func (h *RouteContextArgHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	h.ServeHTTPContext(w, r, nil)
}

func (h *RouteContextArgHandler) Handler() interface{} {
	return h.Value
}

func (h *RouteContextArgHandler) ServeHTTPContext(w http.ResponseWriter, r *http.Request, rctx *RouteContext) {
	h.Value(rctx)
}

type RouteContextHandler interface {
	Handler() HandlerFunc
}

// InterfaceHandler

type HandlerInterfaceGetter interface {
	Handler() interface{}
}

type RouteInterfaceHandler struct {
	Value func(interface{})
}

func (h *RouteInterfaceHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	h.ServeHTTPContext(w, r, nil)
}

func (h *RouteInterfaceHandler) Handler() interface{} {
	return h.Value
}

func (h *RouteInterfaceHandler) ServeHTTPContext(w http.ResponseWriter, r *http.Request, rctx *RouteContext) {
	if rctx.DefaultValueKey != nil {
		h.Value(rctx.Data[rctx.DefaultValueKey])
	} else {
		h.Value(rctx)
	}
}

func HttpHandler(handler interface{}) (h Handler) {
	if handler != nil {
		if ch, ok := handler.(ContextHandler); ok {
			h = &HttpContextHandler{ch, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				ch.ServeHTTPContext(w, r, nil)
			})}
		} else if httpHandler, ok := handler.(http.Handler); ok {
			h = &HTTPHandlerFunc{httpHandler.ServeHTTP}
		} else if httpHandlerFunc, ok := handler.(func(http.ResponseWriter, *http.Request)); ok {
			h = &HTTPHandlerFunc{httpHandlerFunc}
		} else if httpHandlerFunc, ok := handler.(func(*RouteContext)); ok {
			h = &RouteContextArgHandler{httpHandlerFunc}
		} else if httpHandlerArg, ok := handler.(func(http.ResponseWriter, *http.Request, *RouteContext)); ok {
			h = &RouteContextFuncHandler{httpHandlerArg}
		} else if httpHandlerArg, ok := handler.(func(interface{})); ok {
			h = &RouteInterfaceHandler{httpHandlerArg}
		} else {
			panic(fmt.Errorf("Invalid handler type: %v", h))
		}
		return
	}
	return nil
}

func NotFoundHandler() ContextHandler {
	return HttpHandler(http.NotFoundHandler())
}

type EndpointHandler struct {
	*endpoint
}

func (eh EndpointHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	eh.ServeHTTPContext(w, r, nil)
}

func (eh EndpointHandler) ServeHTTPContext(w http.ResponseWriter, r *http.Request, rctx *RouteContext) {
	if h := eh.find(r.Header); h == nil {
		w.WriteHeader(http.StatusBadRequest)
	} else {
		h.handler.ServeHTTPContext(w, r, rctx)
	}
}
