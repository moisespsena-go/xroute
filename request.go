package xroute

import "net/http"

type RequestSetter interface {
	SetRequest(*http.Request)
}

type ChainRequestSetter interface {
	SetRequest(chain *ChainHandler, r *http.Request)
}

type chainRequestSetter struct {
	f func(chain *ChainHandler, r *http.Request)
}

func (crs *chainRequestSetter) SetRequest(chain *ChainHandler, r *http.Request) {
	crs.f(chain, r)
}

func NewChainRequestSetter(setter func(chain *ChainHandler, r *http.Request)) ChainRequestSetter {
	return &chainRequestSetter{setter}
}
