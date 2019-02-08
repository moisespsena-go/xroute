package xroute

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"regexp"
	"runtime/debug"
	"strings"
	"sync"
	"time"
)

var (
	_ Router = &Mux{}
)

type ErrorHandler func(url *url.URL, debug bool, w ResponseWriterWithStatus, r *http.Request, context *RouteContext, begin time.Time, err interface{})

// Mux is a simple HTTP route multiplexer that parses a request path,
// records any URL params, and executes an end handler. It implements
// the http.Handler interface and is friendly with the standard library.
//
// Mux is designed to be fast, minimal and offer a powerful API for building
// modular and composable HTTP services with a large set of handlers. It's
// particularly useful for writing large REST API services that break a handler
// into many smaller parts composed of middlewares and end handlers.
type Mux struct {
	Name         string
	prefix       string
	routeHandler ContextHandlerFunc
	errorHandler ErrorHandler

	arg    interface{}
	argSet bool

	// The radix trie router
	tree *node

	// The interseptors stack
	interseptors *MiddlewaresStack

	registerInterseptorOption int

	// The interseptors stack
	handlerInterseptors *MiddlewaresStack

	registerHandlerInterseptorOption int

	// The middleware stack
	middlewares *MiddlewaresStack

	// Controls the behaviour of middleware chain generation when a mux
	// is registered as an inline group inside another mux.
	inline bool
	parent *Mux

	// The computed mux handler made of the chained middleware stack and
	// the tree router
	handler ContextHandler

	// Routing context pool
	pool sync.Pool

	// Custom route not found handler
	notFoundHandler ContextHandler

	// Custom method not allowed handler
	methodNotAllowedHandler ContextHandler
	buildRouterMutex        sync.Mutex
	logRequests             bool
	logRequestHandler       func(url *url.URL, w ResponseWriterWithStatus, r *http.Request, arg *RouteContext, begin time.Time)
	interseptErrors         bool
	debug                   bool
	api                     bool
	headers                 http.Header
	ApiExtensions           []string

	overrides bool
}

var LogRequestIgnore, _ = regexp.Compile("\\.(css|js|jpg|png|ico|ttf|woff2?)$")

func DefaultLogRequestsHandler(url *url.URL, w ResponseWriterWithStatus, r *http.Request, context *RouteContext, begin time.Time) {
	if !LogRequestIgnore.MatchString(r.URL.Path) || w.Status() >= 400 {
		method := r.Method
		if context.RouteMethod != method {
			method += " -> " + context.RouteMethod
		}
		context.Log.Debugf("Finish [%s] %d %v Took %.2fms", method, w.Status(), url, time.Now().Sub(begin).Seconds()*1000)
	}
}

func prepareStack(stack []byte) []byte {
	/*i := strings.LastIndex(string(stack), "runtime/panic.go")
	i += len("runtime/panic.go") + 1
	stack = stack[i:]
	i = strings.IndexByte(string(stack), '\n')
	stack = stack[i+1:]*/
	return stack
}

func DefaultErrorHandler(url *url.URL, isdebug bool, w ResponseWriterWithStatus, r *http.Request, context *RouteContext, begin time.Time, err interface{}) {
	w.WriteHeader(500)
	errMessage := []byte(fmt.Sprintf("\nRequest failure: %v\n", err))
	os.Stderr.Write(errMessage)

	var stack []byte
	if t, ok := err.(TracedError); ok {
		stack = append(prepareStack(t.Trace()), []byte("\n")...)
	} else {
		stack = prepareStack(debug.Stack())
	}

	os.Stderr.Write(stack)
	if isdebug {
		w.Write(errMessage)
		w.Write(stack)
	} else {
		w.Write([]byte("Request failure. See system administrator to solve it."))
	}
	method := r.Method
	if context.RouteMethod != method {
		method += " -> " + context.RouteMethod
	}
	fmt.Printf("Finish [%s] %d %v Took %.2fms\n", method, w.Status(), url, time.Now().Sub(begin).Seconds()*1000)
}

// NewMux returns a newly initialized Mux object that implements the Router
// interface.
func NewMux(name ...string) *Mux {
	mux := &Mux{
		tree:                &node{},
		handlerInterseptors: NewMiddlewaresStack("HandlerInterseptors", false),
		interseptors:        NewMiddlewaresStack("Interseptors", false),
		middlewares:         NewMiddlewaresStack("Middlewares", true),
		ApiExtensions:       []string{"json"},
	}

	if len(name) > 0 {
		mux.Name = name[0]
	}

	mux.pool.New = func() interface{} {
		return NewRouteContext()
	}
	return mux
}

func (mx *Mux) IsLogRequests() bool {
	return mx.logRequests
}

func (mx *Mux) LogRequests() *Mux {
	mx.logRequests = true
	return mx
}

func (mx *Mux) SetLogRequests(v bool) {
	mx.logRequests = v
}

func (mx *Mux) SetInterseptErrors(v bool) {
	mx.interseptErrors = v
}

func (mx *Mux) IsInterseptErrors() bool {
	return mx.interseptErrors
}

func (mx *Mux) InterseptErrors() *Mux {
	mx.interseptErrors = true
	return mx
}

func (mx *Mux) SetDebug(v bool) {
	mx.debug = v
}

func (mx *Mux) IsDebug() bool {
	return mx.debug
}

func (mx *Mux) Debug() *Mux {
	mx.debug = true
	return mx
}

func (mx *Mux) SetName(name string) *Mux {
	mx.Name = name
	return mx
}

func (mx *Mux) GetInterseptor(name string) *Middleware {
	if m, ok := mx.interseptors.ByName[name]; ok {
		return m
	}
	return nil
}

func (mx *Mux) GetHandlerInterseptor(name string) *Middleware {
	if m, ok := mx.handlerInterseptors.ByName[name]; ok {
		return m
	}
	return nil
}

func (mx *Mux) GetMiddleware(name string) *Middleware {
	if m, ok := mx.middlewares.ByName[name]; ok {
		return m
	}
	return nil
}

func (mx *Mux) Prefix() string {
	return mx.prefix
}

func (mx *Mux) SetPrefix(p string) {
	mx.prefix = p
}

func (mx *Mux) SetRouteHandler(handler ContextHandlerFunc) {
	mx.routeHandler = handler
}

func (mx *Mux) GetRouteHandler() ContextHandlerFunc {
	return mx.routeHandler
}

func (mx *Mux) IsArgSet() bool {
	return mx.argSet
}

func (mx *Mux) SetArg(arg interface{}) {
	mx.arg = arg
	mx.argSet = true
}

func (mx *Mux) Arg() interface{} {
	return mx.arg
}

func (mx *Mux) ClearArg() {
	mx.arg = nil
	mx.argSet = false
}

func (mx *Mux) AcceptMultipartForm(method string) bool {
	return method == "POST" || method == "PUT"
}

// ServeHTTP is the single method of the http.Handler interface that makes
// Mux interoperable with the standard library. It uses a sync.Pool to get and
// reuse routing contexts for each request.
func (mx *Mux) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	mx.ServeHTTPContext(w, r, nil)
}

func (mx *Mux) ServeHTTPContext(w http.ResponseWriter, r *http.Request, rctx *RouteContext) {
	url := GetOriginalURL(r)
	if url == nil {
		urlCopy := *r.URL
		url = &urlCopy
	}
	if rctx == nil {
		r, rctx = GetOrNewRouteContextForRequest(r)
	}

	if rctx.Log == nil {
		rctx.Log = RequestLoggerFactory(r, rctx)
	}

	defer rctx.with(mx)()

	ws := ResponseWriter(w)
	w = ws

	if mx.logRequests || mx.interseptErrors {
		begin := time.Now()
		var rec interface{}
		var defers []func()
		if mx.interseptErrors {
			defers = append(defers, func() {
				if rec != nil {
					handler := mx.errorHandler
					if handler == nil {
						handler = DefaultErrorHandler
					}
					handler(url, mx.debug, ws, r, rctx, begin, rec)
				}
			})
		}
		if mx.logRequests {
			defers = append(defers, func() {
				handler := mx.logRequestHandler
				if handler == nil {
					handler = DefaultLogRequestsHandler
				}
				handler(url, ws, r, rctx, begin)
			})
		}

		defer func() {
			rec = recover()
			for _, def := range defers {
				def()
			}
		}()
	}
	// Ensure the mux has some routes defined on the mux
	if mx.handler == nil {
		// Build the final routing handler for this Mux.
		if !mx.inline && mx.handler == nil {
			mx.buildRouteHandler()
		} else {
			panic(ErrNoHandlers)
		}
	}

	if rctx != nil {
		mx.handler.ServeHTTPContext(w, r, rctx)
		return
	}

	// Fetch a RouteContext object from the sync pool, and call the computed
	// mx.handler that is comprised of mx.middlewares + mx.routeHTTP.
	// Once the request is finished, reset the routing context and put it back
	// into the pool for reuse from another request.
	rctx = mx.pool.Get().(*RouteContext)
	rctx.Reset()
	rctx.Routes = mx
	r = r.WithContext(context.WithValue(r.Context(), RouteCtxKey, rctx))
	mx.handler.ServeHTTPContext(w, r, rctx)
	mx.pool.Put(rctx)
}

// Use appends a middleware handler to the Mux middleware stack.
//
// The middleware stack for any Mux will execute before searching for a matching
// route to a specific handler, which provides opportunity to respond early,
// change the course of the request execution, or set request-scoped values for
// the next Handler.
func (mx *Mux) Intersept(interseptors ...interface{}) {
	mx.interseptors.AddInterface(interseptors, mx.registerInterseptorOption)
}

func (mx *Mux) HandlerInterseptOption(option int, interseptors ...interface{}) {
	old := mx.registerHandlerInterseptorOption
	defer func() {
		mx.registerHandlerInterseptorOption = old
	}()
	mx.HandlerIntersept(interseptors...)
}

// Use appends a middleware handler to the Mux middleware stack.
//
// The middleware stack for any Mux will execute before searching for a matching
// route to a specific handler, which provides opportunity to respond early,
// change the course of the request execution, or set request-scoped values for
// the next Handler.
func (mx *Mux) HandlerIntersept(interseptors ...interface{}) {
	mx.handlerInterseptors.AddInterface(interseptors, mx.registerHandlerInterseptorOption)
}

// Use appends a middleware handler to the Mux middleware stack.
//
// The middleware stack for any Mux will execute before searching for a matching
// route to a specific handler, which provides opportunity to respond early,
// change the course of the request execution, or set request-scoped values for
// the next Handler.
func (mx *Mux) Use(middlewares ...interface{}) {
	mx.middlewares.AddInterface(middlewares, DUPLICATION_ABORT)
}

// Handle adds the route `pattern` that matches any http method to
// execute the `handler` Handler.
func (mx *Mux) Handle(pattern string, handler interface{}) {
	mx.handle(ALL, pattern, handler)
}

// HandleFunc adds the route `pattern` that matches any http method to
// execute the `handler` HandlerFunc.
func (mx *Mux) HandleFunc(pattern string, handler interface{}) {
	mx.handle(ALL, pattern, handler)
}

// Method adds the route `pattern` that matches `method` http method to
// execute the `handler` Handler.
func (mx *Mux) Method(method, pattern string, handler interface{}) {
	m, ok := methodMap[strings.ToUpper(method)]
	if !ok {
		panic(fmt.Sprintf("chi: '%s' http method is not supported.", method))
	}
	mx.handle(m, pattern, handler)
}

// HandleMethod adds the route `pattern` that matches `method` http method to
// execute the `handler` Handler.
func (mx *Mux) MethodT(method MethodType, pattern string, handler interface{}) {
	for _, m := range methodMap {
		if (method & m) != 0 {
			mx.handle(m, pattern, handler)
		}
	}
}

// Connect adds the route `pattern` that matches a CONNECT http method to
// execute the `handler` Handler.
func (mx *Mux) Connect(pattern string, handler interface{}) {
	mx.handle(CONNECT, pattern, handler)
}

// Delete adds the route `pattern` that matches a DELETE http method to
// execute the `handler` Handler.
func (mx *Mux) Delete(pattern string, handler interface{}) {
	mx.handle(DELETE, pattern, handler)
}

// Get adds the route `pattern` that matches a GET http method to
// execute the `handler` Handler.
func (mx *Mux) Get(pattern string, handler interface{}) {
	mx.handle(GET, pattern, handler)
}

// Head adds the route `pattern` that matches a HEAD http method to
// execute the `handler` Handler.
func (mx *Mux) Head(pattern string, handler interface{}) {
	mx.handle(HEAD, pattern, handler)
}

// Options adds the route `pattern` that matches a OPTIONS http method to
// execute the `handler` Handler.
func (mx *Mux) Options(pattern string, handler interface{}) {
	mx.handle(OPTIONS, pattern, handler)
}

// Patch adds the route `pattern` that matches a PATCH http method to
// execute the `handler` Handler.
func (mx *Mux) Patch(pattern string, handler interface{}) {
	mx.handle(PATCH, pattern, handler)
}

// Post adds the route `pattern` that matches a POST http method to
// execute the `handler` Handler.
func (mx *Mux) Post(pattern string, handler interface{}) {
	mx.handle(POST, pattern, handler)
}

// Put adds the route `pattern` that matches a PUT http method to
// execute the `handler` Handler.
func (mx *Mux) Put(pattern string, handler interface{}) {
	mx.handle(PUT, pattern, handler)
}

// Trace adds the route `pattern` that matches a TRACE http method to
// execute the `handler` Handler.
func (mx *Mux) Trace(pattern string, handler interface{}) {
	mx.handle(TRACE, pattern, handler)
}

// NotFound sets a custom Handler for routing paths that could
// not be found. The default 404 handler is `http.NotFound`.
func (mx *Mux) NotFound(handler interface{}) {
	// Build NotFound handler chain
	m := mx
	hh := HttpHandler(handler)
	if mx.inline && mx.parent != nil {
		m = mx.parent
		hh = mx.chainHandler(hh)
	}

	// Update the notFoundHandler from this point forward
	m.notFoundHandler = hh
	m.updateSubRoutes(func(subMux *Mux) {
		if subMux.notFoundHandler == nil {
			subMux.NotFound(hh)
		}
	})
}

// MethodNotAllowed sets a custom Handler for routing paths where the
// method is unresolved. The default handler returns a 405 with an empty body.
func (mx *Mux) MethodNotAllowed(handler interface{}) {
	// Build MethodNotAllowed handler chain
	m := mx
	h := HttpHandler(handler)

	if mx.inline && mx.parent != nil {
		m = mx.parent
		h = mx.chainHandler(h)
	}

	// Update the methodNotAllowedHandler from this point forward
	m.methodNotAllowedHandler = h
	m.updateSubRoutes(func(subMux *Mux) {
		if subMux.methodNotAllowedHandler == nil {
			subMux.MethodNotAllowed(h)
		}
	})
}

// With adds inline middlewares for an endpoint handler.
func (mx *Mux) With(middlewares ...interface{}) Router {
	// Copy middlewares from parent inline muxs
	var md, its, hits *MiddlewaresStack
	if mx.inline {
		md, its, hits = mx.middlewares.Copy(), mx.interseptors.Copy(), mx.handlerInterseptors.Copy()
	} else {
		md = NewMiddlewaresStack("Middleware", true)
		its = NewMiddlewaresStack("Interseptors", false)
		hits = NewMiddlewaresStack("HandlerInterseptors", false)
	}

	im := &Mux{
		inline:              true,
		parent:              mx,
		tree:                mx.tree,
		middlewares:         md,
		interseptors:        its,
		handlerInterseptors: hits,
	}
	im.Use(middlewares...)
	return im
}

// Group creates a new inline-Mux with a fresh middleware stack. It's useful
// for a group of handlers along the same routing path that use an additional
// set of middlewares. See _examples/.
func (mx *Mux) Group(fn func(r Router)) Router {
	im := mx.With().(*Mux)
	if fn != nil {
		fn(im)
	}
	return im
}

// Route creates a new Mux with a fresh middleware stack and mounts it
// along the `pattern` as a subrouter. Effectively, this is a short-hand
// call to Mount. See _examples/.
func (mx *Mux) Route(pattern string, fn func(r Router)) Router {
	subRouter := NewRouter()
	if fn != nil {
		fn(subRouter)
	}
	mx.Mount(pattern, subRouter)
	return subRouter
}

type MountHandler struct {
	handler func(w http.ResponseWriter, r *http.Request, context *RouteContext)
	Handler interface{}
}

func (h *MountHandler) ServeHTTPContext(w http.ResponseWriter, r *http.Request, context *RouteContext) {
	h.handler(w, r, context)
}

// Mount attaches another Handler or chi Router as a subrouter along a routing
// path. It's very useful to split up a large API as many independent routers and
// compose them as a single service using Mount. See _examples/.
//
// Note that Mount() simply sets a wildcard along the `pattern` that will continue
// routing at the `handler`, which in most cases is another chi.Router. As a result,
// if you define two Mount() routes on the exact same pattern the mount will panic.
func (mx *Mux) Mount(pattern string, handler interface{}) {
	// Provide runtime safety for ensuring a pattern isn't mounted on an existing
	// routing pattern.
	if mx.tree.findPattern(pattern+"*") || mx.tree.findPattern(pattern+"/*") {
		panic(fmt.Sprintf("chi: attempting to Mount() a handler on an existing path, '%s'", pattern))
	}

	// Assign Sub-Router's with the parent not found & method not allowed handler if not specified.
	subr, ok := handler.(*Mux)
	if ok && subr.notFoundHandler == nil && mx.notFoundHandler != nil {
		subr.NotFound(mx.notFoundHandler)
	}
	if ok && subr.methodNotAllowedHandler == nil && mx.methodNotAllowedHandler != nil {
		subr.MethodNotAllowed(mx.methodNotAllowedHandler)
	}

	httpHandler := HttpHandler(handler)
	var mh ContextHandler

	if mux, ok := handler.(Router); ok {
		mux.SetPrefix(pattern)
		// Wrap the Sub-router in a handlerFunc to scope the request path for routing.
		mmx, ok := mux.(*Mux)

		if ok {
			mmx.parent = mx
		}

		mh = &MountHandler{func(w http.ResponseWriter, r *http.Request, ctx *RouteContext) {
			ctx.RoutePath = mx.nextRoutePath(ctx)
			httpHandler.ServeHTTPContext(w, r, ctx)
		}, mux}
	} else {
		// Wrap the Sub-router in a handlerFunc to scope the request path for routing.
		mh = &MountHandler{func(w http.ResponseWriter, r *http.Request, ctx *RouteContext) {
			ctx.RoutePath = mx.nextRoutePath(ctx)
			httpHandler.ServeHTTPContext(w, r, ctx)
		}, handler}
	}

	if pattern == "" || pattern[len(pattern)-1] != '/' {
		notFoundHandler := HttpHandler(func(w http.ResponseWriter, r *http.Request, arg *RouteContext) {
			mx.NotFoundHandler().ServeHTTPContext(w, r, arg)
		})

		mx.handle(ALL|STUB, pattern, mh)
		mx.handle(ALL|STUB, pattern+"/", notFoundHandler)
		pattern += "/"
	}

	method := ALL
	subroutes, _ := handler.(Routes)
	if subroutes != nil {
		method |= STUB
	}

	for _, n := range mx.handle(method, pattern+"*", mh) {
		if subroutes != nil {
			n.subroutes = subroutes
		}
	}
}

// Routes returns a slice of routing information from the tree,
// useful for traversing available routes of a router.
func (mx *Mux) Routes() []Route {
	return mx.tree.routes()
}

// Middlewares returns a slice of middleware handler functions.
func (mx *Mux) Middlewares() Middlewares {
	return mx.middlewares.Items
}

// Match searches the routing tree for a handler that matches the method/path.
// It's similar to routing a http request, but without executing the handler
// thereafter.
//
// Note: the *Context state is updated during execution, so manage
// the state carefully or make a NewRouteContext().
func (mx *Mux) Match(rctx *RouteContext, method, path string) bool {
	m, ok := methodMap[method]
	if !ok {
		return false
	}

	node, _, h := mx.tree.FindRoute(rctx, m, path)

	if node != nil && node.subroutes != nil {
		rctx.RoutePath = mx.nextRoutePath(rctx)
		return node.subroutes.Match(rctx, method, rctx.RoutePath)
	}

	return h != nil
}

// NotFoundHandler returns the default Mux 404 responder whenever a route
// cannot be found.
func (mx *Mux) NotFoundHandler() ContextHandler {
	if mx.notFoundHandler != nil {
		return mx.notFoundHandler
	}
	return HttpHandler(http.NotFoundHandler())
}

// MethodNotAllowedHandler returns the default Mux 405 responder whenever
// a method cannot be resolved for a route.
func (mx *Mux) MethodNotAllowedHandler() ContextHandler {
	if mx.methodNotAllowedHandler != nil {
		return mx.methodNotAllowedHandler
	}
	return methodNotAllowedHandler
}

// buildRouteHandler builds the single mux handler that is a chain of the middleware
// stack, as defined by calls to Use(), and the tree router (Mux) itself. After this
// point, no other middlewares can be registered on this Mux's stack. But you can still
// compose additional middlewares via Group()'s or using a chained middleware handler.
func (mx *Mux) buildRouteHandler() {
	mx.buildRouterMutex.Lock()
	defer mx.buildRouterMutex.Unlock()
	if mx.handler == nil {
		h := HttpHandler(mx.routeHTTP)
		if mx.routeHandler != nil {
			mainHandler := h
			h = HttpHandler(func(w http.ResponseWriter, r *http.Request, rctx *RouteContext) {
				mx.routeHandler(mainHandler, w, r, rctx)
			})
		}

		var minterseptors []Middlewares
		p := mx
		for p != nil {
			if p.handlerInterseptors.Len > 0 {
				minterseptors = append(minterseptors, p.handlerInterseptors.All())
			}
			p = p.parent
		}

		var hinterseptors Middlewares
		for i := len(minterseptors) - 1; i >= 0; i-- {
			hinterseptors = append(hinterseptors, minterseptors[i]...)
		}

		mx.handlerInterseptors.Add(hinterseptors, DUPLICATION_SKIP).Build()
		mx.interseptors.Build()
		mx.middlewares.Build()
		mx.handler = mx.chainHandler(h)
	}
}

func (mx *Mux) chainHandler(h interface{}) Handler {
	return Chain(append(append(Middlewares{}, mx.interseptors.Build().Items...), mx.middlewares.Build().Items...)...).Handler(h)
}

func (mx *Mux) Api(f func(r Router)) {
	old := mx.api
	defer func() {
		mx.api = old
	}()
	mx.api = true
	f(mx)
}
func (mx *Mux) Headers(headers http.Header, f func(r Router)) {
	old := mx.headers
	defer func() {
		mx.headers = old
	}()
	mx.headers = headers
	f(mx)
}

// handle registers a http.Handler in the routing tree for a particular http method
// and routing pattern.
func (mx *Mux) handle(method MethodType, pattern string, handler interface{}) (nodes []*node) {
	if len(pattern) == 0 || pattern[0] != '/' {
		panic(fmt.Sprintf("chi: routing pattern must begin with '/' in '%s'", pattern))
	}

	// Build endpoint handler with inline middlewares for the route
	h := HttpHandler(handler)

	if mx.inline {
		mx.handler = HttpHandler(mx.routeHTTP)
		h = mx.chainHandler(h)
	}

	// Add the endpoint to the tree and return the node
	if mx.api {
		if pattern == "/" {
			for _, ext := range mx.ApiExtensions {
				nodes = append(nodes, mx.tree.InsertRoute(mx.overrides, method, "/."+ext, h, mx.headers))
			}
		} else {
			for _, ext := range mx.ApiExtensions {
				nodes = append(nodes, mx.tree.InsertRoute(mx.overrides, method, pattern+"."+ext, h, mx.headers))
			}
		}
	}
	nodes = append(nodes, mx.tree.InsertRoute(mx.overrides, method, pattern, h, mx.headers))
	return
}

func (mx *Mux) FindHandler(method, path string, header ...http.Header) ContextHandler {
	if h := mx.tree.GetRoute(methodMap[method], path); h != nil {
		if h := h.Handler(header...); h != nil {
			if mh, ok := h.(*MountHandler); ok {
				if finder, ok := mh.Handler.(HandlerFinder); ok {
					return finder.FindHandler(path, method)
				}
				return mh
			}
		}
	}
	return nil
}

// routeHTTP routes a http.request through the Mux routing tree to serve
// the matching handler for a particular http method.
func (mx *Mux) routeHTTP(w http.ResponseWriter, r *http.Request, rctx *RouteContext) {
	// The request routing path
	routePath := rctx.RoutePath
	if routePath == "" {
		if r.URL.RawPath != "" {
			routePath = r.URL.RawPath
		} else {
			routePath = r.URL.Path
		}
	}

	// Check if method is supported by chi
	if rctx.RouteMethod == "" {
		rctx.RouteMethod = r.Method
	}
	method, ok := methodMap[rctx.RouteMethod]
	if !ok {
		mx.MethodNotAllowedHandler().ServeHTTPContext(w, r, rctx)
		return
	}

	// Find the route
	if _, _, h := mx.tree.FindRoute(rctx, method, routePath); h != nil {
		if mh, ok := h.(*MountHandler); ok {
			mh.handler(w, r, rctx)
		} else if len(mx.handlerInterseptors.Items) > 0 {
			handlerChain := Chain(mx.handlerInterseptors.Items...).Handler(h)
			handlerChain.ServeHTTPContext(w, r, rctx)
		} else if eh, ok := h.(*EndpointHandler); ok {
			if ehh := eh.find(r.Header); ehh == nil {
				w.WriteHeader(http.StatusBadRequest)
			} else {
				rctx.Handler = ehh.handler
				ehh.handler.ServeHTTPContext(w, r, rctx)
			}
		} else {
			h.ServeHTTPContext(w, r, rctx)
		}
		return
	}

	for _, ext := range mx.ApiExtensions {
		if pos := strings.LastIndex(routePath, "."+ext); pos != -1 {
			routePath = routePath[0:pos] + "/." + ext
			// Find the route for api
			if _, _, h := mx.tree.FindRoute(rctx, method, routePath); h != nil {
				rctx.ApiExt = ext
				if mh, ok := h.(*MountHandler); ok {
					mh.handler(w, r, rctx)
				} else {
					handlerChain := Chain(mx.handlerInterseptors.Items...).Handler(h)
					handlerChain.ServeHTTPContext(w, r, rctx)
				}
				return
			}
		}
	}

	if rctx.methodNotAllowed {
		mx.MethodNotAllowedHandler().ServeHTTPContext(w, r, rctx)
	} else {
		mx.NotFoundHandler().ServeHTTPContext(w, r, rctx)
	}
}

func (mx *Mux) nextRoutePath(rctx *RouteContext) string {
	routePath := "/"
	nx := len(rctx.routeParams.Keys) - 1 // index of last param in list
	if nx >= 0 && rctx.routeParams.Keys[nx] == "*" && len(rctx.routeParams.Values) > nx {
		routePath += rctx.routeParams.Values[nx]
	}
	return routePath
}

// Recursively update data on child routers.
func (mx *Mux) updateSubRoutes(fn func(subMux *Mux)) {
	for _, r := range mx.tree.routes() {
		subMux, ok := r.SubRoutes.(*Mux)
		if !ok {
			continue
		}
		fn(subMux)
	}
}

func (mx *Mux) Overrides(f func(r Router)) {
	if mx.overrides {
		f(mx)
		return
	}
	mx.overrides = true
	defer func() { mx.overrides = false }()
	f(mx)
}

// methodNotAllowedHandler is a helper function to respond with a 405,
// method not allowed.
var methodNotAllowedHandler = HttpHandler(func(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(405)
	w.Write(nil)
})
