package route

import (
	"context"
	"net/http"
	"net/url"
)

const ORIGINAL_URL = "origianal_url"

func SetOriginalUrl(r *http.Request) *http.Request {
	URL := *r.URL
	return r.WithContext(context.WithValue(r.Context(), ORIGINAL_URL, &URL))
}

func SetOriginalUrlIfNotSetted(r *http.Request) *http.Request {
	if r.Context().Value(ORIGINAL_URL) == nil {
		return SetOriginalUrl(r)
	}
	return r
}

func GetOriginalUrl(r *http.Request) *url.URL {
	URL := r.Context().Value(ORIGINAL_URL)
	if URL == nil {
		return nil
	}
	return URL.(*url.URL)
}
