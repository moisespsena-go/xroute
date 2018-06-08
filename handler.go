package route

import (
	"net/http"
	"fmt"
	"reflect"
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

func (h *RouteContextFuncHandler) Handler() HandlerFunc {
	return h.Value
}

func (h *RouteContextFuncHandler) ServeHTTPContext(w http.ResponseWriter, r *http.Request, rctx *RouteContext) {
	h.Value(w, r, rctx)
}

// InterfaceHandler

type HandlerInterfaceGetter interface {
	Handler() interface{}
}

type RouteInterfaceHandler struct {
	Value func(interface{})
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

func HttpHandler(handler interface{}) (h ContextHandler) {
	if handler != nil {
		if httpHandler, ok := handler.(ContextHandler); ok {
			h = httpHandler
		} else if httpHandler, ok := handler.(http.Handler); ok {
			h = &HTTPHandlerFunc{httpHandler.ServeHTTP}
		} else if httpHandlerFunc, ok := handler.(func(http.ResponseWriter, *http.Request)); ok {
			h = &HTTPHandlerFunc{httpHandlerFunc}
		} else if httpHandlerArg, ok := handler.(func(http.ResponseWriter, *http.Request, *RouteContext)); ok {
			h = &RouteContextFuncHandler{httpHandlerArg}
		} else if httpHandlerArg, ok := handler.(func(interface{})); ok {
			h = &RouteInterfaceHandler{httpHandlerArg}
		} else if typ := reflect.TypeOf(handler); typ.Kind() == reflect.Func {
			f := reflect.ValueOf(handler)
			h = &RouteInterfaceHandler{func(v interface{}) {
				f.Call([]reflect.Value{reflect.ValueOf(v)})
			}}
		} else {
			panic(fmt.Errorf("Invalid handler type: %v", h))
		}
	}
	return
}
