package route

import (
	"context"
	"net/http"
	"net/url"
)

const ORIGINAL_URL = "origianal_url"

func SetOriginalURL(r *http.Request) *http.Request {
	var u url.URL
	u = *r.URL
	return r.WithContext(context.WithValue(r.Context(), ORIGINAL_URL, &u))
}

func SetOriginalURLIfNotSetted(r *http.Request) *http.Request {
	if r.Context().Value(ORIGINAL_URL) == nil {
		return SetOriginalURL(r)
	}
	return r
}

func GetOriginalURL(r *http.Request) *url.URL {
	URL := r.Context().Value(ORIGINAL_URL)
	if URL == nil {
		return r.URL
	}
	return URL.(*url.URL)
}
