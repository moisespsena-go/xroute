package route

import (
	"context"
	"net"
	"net/http"
	"strings"

	"github.com/op/go-logging"
)

var (
	// RouteCtxKey is the context.Context key to store the request context.
	RouteCtxKey = &contextKey{"RouteContext"}
)

// Context is the default routing context set on the root node of a
// request context to track route patterns, URL parameters and
// an optional routing path.
type RouteContext struct {
	Routes Routes

	// Routing path/method override used during the route search.
	// See Mux#routeHTTP method.
	RoutePath   string
	RouteMethod string

	// Routing pattern stack throughout the lifecycle of the request,
	// across all connected routers. It is a record of all matching
	// patterns across a stack of Sub-routers.
	RoutePatterns []string

	// URLParams are the stack of routeParams captured during the
	// routing lifecycle across a stack of Sub-routers.
	URLParams *OrderedMap

	// The endpoint routing pattern that matched the request URI path
	// or `RoutePath` of the current Sub-router. This value will update
	// during the lifecycle of a request passing through a stack of
	// Sub-routers.
	routePattern string

	// Route parameters matched for the current Sub-router. It is
	// intentionally unexported so it cant be tampered.
	routeParams RouteParams

	// methodNotAllowed hint
	methodNotAllowed bool

	DefaultValueKey     interface{}
	Data                map[interface{}]interface{}
	RequestSetters      map[interface{}]RequestSetter
	ChainRequestSetters map[interface{}]ChainRequestSetter
	Handler             interface{}
	RouterStack         []Router
	Log                 *logging.Logger

	ApiExt string
}

// NewRouteContext returns a new routing Context object.
func NewRouteContext() *RouteContext {
	c := &RouteContext{}
	c.Reset()
	return c
}

func (x *RouteContext) with(r Router) func() {
	x.RouterStack = append(x.RouterStack, r)
	return func() {
		x.RouterStack = x.RouterStack[0 : len(x.RouterStack)-1]
	}
}

func (x *RouteContext) Router() Router {
	if x.RouterStack != nil {
		return x.RouterStack[len(x.RouterStack)-1]
	}
	return nil
}

func (x *RouteContext) Routers() []Router {
	return x.RouterStack
}

func (x *RouteContext) SetValue(v interface{}) *RouteContext {
	x.Data[x.DefaultValueKey] = v
	return x
}

func (x *RouteContext) Value() interface{} {
	return x.Data[x.DefaultValueKey]
}

// Reset a routing context to its initial state.
func (x *RouteContext) Reset() {
	x.Routes = nil
	x.RoutePath = ""
	x.RouteMethod = ""
	x.RoutePatterns = x.RoutePatterns[:0]
	x.URLParams = NewOrderedMap()

	x.routePattern = ""
	x.routeParams.Keys = x.routeParams.Keys[:0]
	x.routeParams.Values = x.routeParams.Values[:0]
	x.methodNotAllowed = false
	x.Data = make(map[interface{}]interface{})
	x.RequestSetters = make(map[interface{}]RequestSetter)
	x.ChainRequestSetters = make(map[interface{}]ChainRequestSetter)
}

// URLParam returns the corresponding URL parameter value from the request
// routing context.
func (x *RouteContext) URLParam(key string) string {
	return x.URLParams.Get(key)
}

// RoutePattern builds the routing pattern string for the particular
// request, at the particular point during routing. This means, the value
// will change throughout the execution of a request in a router. That is
// why its advised to only use this value after calling the next handler.
//
// For example,
//
// func Instrument(next http.Handler) http.Handler {
//   return http.HandlerFunc(func(w http.ResponseWriter, r *http.request) {
//     next.ServeHTTP(w, r)
//     routePattern := chi.RouteContext(r.Context()).RoutePattern()
//     measure(w, r, routePattern)
// 	 })
// }
func (x *RouteContext) RoutePattern() string {
	routePattern := strings.Join(x.RoutePatterns, "")
	return strings.Replace(routePattern, "/*/", "/", -1)
}

// RouteContext returns chi's routing Context object from a
// http.request Context.
func RouteContextFromContext(ctx context.Context) *RouteContext {
	v := ctx.Value(RouteCtxKey)
	if v == nil {
		return nil
	}
	return v.(*RouteContext)
}

// RouteContext returns chi's routing Context object from a
// http.request Context.
func RouteContextFromRequest(r *http.Request) *RouteContext {
	return RouteContextFromContext(r.Context())
}

func SetRouteContextToRequest(r *http.Request, rctx *RouteContext) *http.Request {
	return r.WithContext(context.WithValue(r.Context(), RouteCtxKey, rctx))
}

func NewRouteContextForRequest(r *http.Request) (*http.Request, *RouteContext) {
	rctx := NewRouteContext()
	r = SetRouteContextToRequest(r, rctx)
	return r, rctx
}

func GetOrNewRouteContextForRequest(r *http.Request) (*http.Request, *RouteContext) {
	rctx := RouteContextFromRequest(r)
	if rctx == nil {
		rctx = NewRouteContext()
		r = SetRouteContextToRequest(r, rctx)
	}
	return r, rctx
}

// URLParam returns the url parameter from a http.request object.
func URLParam(r *http.Request, key string) string {
	if rctx := RouteContextFromContext(r.Context()); rctx != nil {
		return rctx.URLParam(key)
	}
	return ""
}

// URLParamFromCtx returns the url parameter from a http.request Context.
func URLParamFromCtx(ctx context.Context, key string) string {
	if rctx := RouteContextFromContext(ctx); rctx != nil {
		return rctx.URLParam(key)
	}
	return ""
}

// RouteParams is a structure to track URL routing parameters efficiently.
type RouteParams struct {
	Keys, Values []string
}

// Add will append a URL parameter to the end of the route param
func (s *RouteParams) Add(key, value string) {
	(*s).Keys = append((*s).Keys, key)
	(*s).Values = append((*s).Values, value)
}

// ServerBaseContext wraps an http.Handler to set the request context to the
// `baseCtx`.
func ServerBaseContext(baseCtx context.Context, h http.Handler) http.Handler {
	fn := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()
		baseCtx := baseCtx

		// Copy over default net/http server context keys
		if v, ok := ctx.Value(http.ServerContextKey).(*http.Server); ok {
			baseCtx = context.WithValue(baseCtx, http.ServerContextKey, v)
		}
		if v, ok := ctx.Value(http.LocalAddrContextKey).(net.Addr); ok {
			baseCtx = context.WithValue(baseCtx, http.LocalAddrContextKey, v)
		}

		h.ServeHTTP(w, r.WithContext(baseCtx))
	})
	return fn
}

// contextKey is a value for use with context.WithValue. It's used as
// a pointer so it fits in an interface{} without allocation. This technique
// for defining context keys was copied from Go 1.7's new use of context in net/http.
type contextKey struct {
	name string
}

func (k *contextKey) String() string {
	return "chi context value " + k.name
}
