package route

import (
	"context"
	"net/http"
)

// Chain returns a Middlewares type from a slice of middleware handlers.
func Chain(middlewares ...*Middleware) Middlewares {
	return Middlewares(middlewares)
}

// Handler builds and returns a http.Handler from the chain of middlewares,
// with `h http.Handler` as the final handler.
func (mws Middlewares) Handler(h interface{}) Handler {
	if len(mws) == 0 {
		return HttpHandler(h)
	}
	return &ChainHandler{Middlewares: mws, Endpoint: HttpHandler(h)}
}

// ChainHandler is a http.Handler with support for handler composition and
// execution.
type ChainHandler struct {
	Middlewares Middlewares
	Endpoint    ContextHandler

	Index   int
	request *http.Request
	Writer  ResponseWriterWithStatus
	Context *RouteContext
	next    bool
}

func (c *ChainHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	c.Next(w, r, c.Context)
}

func (c *ChainHandler) ServeHTTPContext(w http.ResponseWriter, r *http.Request, rctx *RouteContext) {
	if rctx == nil {
		rctx = NewRouteContext()
	}
	rctx.Handler = c.Endpoint
	copy := &ChainHandler{Middlewares: c.Middlewares, Endpoint: c.Endpoint, Context: rctx, request: r, Writer: ResponseWriter(w)}
	copy.Next()
}

func (c *ChainHandler) Request() *http.Request {
	return c.request
}

func (c *ChainHandler) SetRequest(r *http.Request) {
	c.request = r
	if c.Context != nil {
		for _, setter := range c.Context.RequestSetters {
			setter.SetRequest(r)
		}
		for _, setter := range c.Context.ChainRequestSetters {
			setter.SetRequest(c, r)
		}
	}
}

func (c *ChainHandler) Middleware() *Middleware {
	return c.Middlewares[c.Index]
}

func (c *ChainHandler) Pass() {
	c.next = true
}

func (c *ChainHandler) Next(values ...interface{}) {
	w, r, arg := c.Writer, c.request, c.Context
	defer func() {
		c.Writer, c.request, c.Context = w, r, arg
	}()
	for _, v := range values {
		switch vt := v.(type) {
		case *http.Request:
			c.SetRequest(vt)
		case ResponseWriterWithStatus:
			c.Writer = vt
		case http.ResponseWriter:
			c.Writer = ResponseWriter(vt)
		case *RouteContext:
			c.Context = vt
		case context.Context:
			c.SetRequest(c.request.WithContext(vt))
		}
	}

	oldNext := c.next

	for {
		if c.Index < len(c.Middlewares) {
			h := c.Middlewares[c.Index].Handler
			c.Index++
			h(c)
		} else if c.Index == len(c.Middlewares) {
			c.Index++
			c.Endpoint.ServeHTTPContext(c.Writer, c.request, c.Context)
		}
		if c.next {
			c.next = false
		} else {
			break
		}
	}

	c.next = oldNext
}
