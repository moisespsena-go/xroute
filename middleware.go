package xroute

import (
	"fmt"
	"net/http"
	"strings"
	"sync"

	"github.com/moisespsena-go/topsort"
)

const (
	DUPLICATION_OVERRIDE = 0
	DUPLICATION_ABORT    = 1
	DUPLICATION_SKIP     = 2
)

// Middlewares type is a slice of standard middleware handlers with methods
// to compose middleware chains and interface{}'s.
type Middlewares []*Middleware

func (this *Middlewares) Add(md ...*Middleware) {
	*this = append(*this, md...)
}

type Middleware struct {
	Name    string
	Handler func(chain *ChainHandler)
	Before  []string
	After   []string
}

func NewMiddleware(f interface{}) *Middleware {
	switch ft := f.(type) {
	case func(chain *ChainHandler):
		return &Middleware{Handler: ft}
	case *Middleware:
		return ft
	case func(http.Handler) http.Handler:
		return &Middleware{Handler: func(chain *ChainHandler) {
			ft(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				chain.Next(w, r)
			})).ServeHTTP(chain.Writer, chain.request)
		}}
	default:
		panic(fmt.Errorf("Invalid Middleware: %s", f))
	}
}

type MiddlewaresStack struct {
	Name            string
	ByName          map[string]*Middleware
	Items           Middlewares
	Anonymous       Middlewares
	acceptAnonymous bool
	Len             int
	mu              sync.Mutex
}

func NewMiddlewaresStack(name string, acceptAnonymous bool) *MiddlewaresStack {
	return &MiddlewaresStack{
		Name:            name,
		ByName:          make(map[string]*Middleware),
		acceptAnonymous: acceptAnonymous,
	}
}

func (stack *MiddlewaresStack) Copy() *MiddlewaresStack {
	byName := make(map[string]*Middleware)

	for key, md := range stack.ByName {
		byName[key] = md
	}

	anonymous := make(Middlewares, len(stack.Anonymous))
	copy(anonymous, stack.Anonymous)

	items := make(Middlewares, len(stack.Items))
	copy(items, stack.Items)

	return &MiddlewaresStack{
		Name:            stack.Name,
		ByName:          byName,
		Items:           items,
		Anonymous:       anonymous,
		acceptAnonymous: stack.acceptAnonymous,
		Len:             stack.Len,
	}
}

func (stack *MiddlewaresStack) All() (items []*Middleware) {
	if stack.Items != nil {
		return stack.Items
	}
	items = append(items, stack.Anonymous...)
	for _, md := range stack.ByName {
		items = append(items, md)
	}
	return
}

func (stack *MiddlewaresStack) Override(items Middlewares, option int) *MiddlewaresStack {
	return NewMiddlewaresStack(stack.Name, stack.acceptAnonymous).Add(items, option)
}

func (stack *MiddlewaresStack) Has(name ...string) bool {
	for _, n := range name {
		if _, ok := stack.ByName[n]; !ok {
			return false
		}
	}
	return true
}

func (stack *MiddlewaresStack) AddInterface(items []interface{}, option int) *MiddlewaresStack {
	var mds Middlewares
	for _, item := range items {
		switch it := item.(type) {
		case Middlewares:
			mds = append(mds, it...)
		case []*Middleware:
			mds = append(mds, it...)
		default:
			mds = append(mds, NewMiddleware(item))
		}
	}
	return stack.Add(mds, option)
}

func (stack *MiddlewaresStack) Add(items Middlewares, option int) *MiddlewaresStack {
	if stack.ByName == nil {
		stack.ByName = make(map[string]*Middleware)
	}

	for i, md := range items {
		if md.Name == "" {
			if stack.acceptAnonymous {
				stack.Anonymous = append(stack.Anonymous, md)
				stack.Len++
			} else {
				panic(fmt.Errorf("%v[%v]: Name is empty.", stack.Name, i))
			}
		} else {
			if stack.Has(md.Name) {
				switch option {
				case DUPLICATION_ABORT:
					panic(fmt.Errorf("%v: %q has be registered.", stack.Name, md.Name))
				case DUPLICATION_SKIP:
					continue
				case DUPLICATION_OVERRIDE:
				default:
					panic(fmt.Errorf("%v: Invalid interseptor option %v.", option))
				}
			}
			stack.ByName[md.Name] = md
			stack.Len++
		}
	}
	return stack
}

func (stack *MiddlewaresStack) Build() *MiddlewaresStack {
	if len(stack.Items) == stack.Len {
		return stack
	}

	stack.mu.Lock()
	defer stack.mu.Unlock()

	notFound := make(map[string][]string)

	graph := topsort.NewGraph()

	for _, md := range stack.ByName {
		graph.AddNode(md.Name)
	}

	for _, md := range stack.ByName {
		for _, to := range md.Before {
			if stack.Has(to) {
				graph.AddEdge(md.Name, to)
			} else {
				if _, ok := notFound[md.Name]; !ok {
					notFound[md.Name] = make([]string, 1)
				}
				notFound[md.Name] = append(notFound[md.Name], to)
			}
		}
		for _, from := range md.After {
			if stack.Has(from) {
				graph.AddEdge(from, md.Name)
			} else {
				if _, ok := notFound[md.Name]; !ok {
					notFound[md.Name] = make([]string, 1)
				}
				notFound[md.Name] = append(notFound[md.Name], from)
			}
		}
	}

	if len(notFound) > 0 {
		var msgs []string
		for n, items := range notFound {
			msgs = append(msgs, fmt.Sprintf("%v: Required by %q: %v.", stack.Name, n, strings.Join(items, ", ")))
		}
		panic(fmt.Errorf("qor/route %v: middlewares dependency error:\n - %v\n", stack.Name, strings.Join(msgs, "\n - ")))
	}

	names, err := graph.DepthFirst()

	if err != nil {
		panic(fmt.Errorf("qor/route %v: topological middlewares sorter error: %v", stack.Name, err))
	}

	stack.Items = make(Middlewares, 0)

	// named middlewares at begin
	for _, name := range names {
		stack.Items = append(stack.Items, stack.ByName[name])
	}

	// named middlewares at end
	for _, md := range stack.Anonymous {
		stack.Items = append(stack.Items, md)
	}

	return stack
}
