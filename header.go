package xroute

import "net/http"

func Header(pairs ...string) (header http.Header) {
	header = make(http.Header, len(pairs)/2)
	for i := 0; i < len(pairs); i += 2 {
		header.Set(pairs[i], pairs[i+1])
	}
	return
}
