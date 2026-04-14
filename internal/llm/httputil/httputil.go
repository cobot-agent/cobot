package httputil

import (
	"maps"
	"net"
	"net/http"
	"slices"
	"time"
)

// DefaultTransport is based on http.DefaultTransport (preserving proxy,
// HTTP/2, and connection pool settings) with overridden timeout values.
var DefaultTransport = func() *http.Transport {
	base, ok := http.DefaultTransport.(*http.Transport)
	if !ok {
		base = &http.Transport{}
	}
	t := base.Clone()
	t.DialContext = (&net.Dialer{
		Timeout:   30 * time.Second,
		KeepAlive: 30 * time.Second,
	}).DialContext
	t.TLSHandshakeTimeout = 10 * time.Second
	t.ResponseHeaderTimeout = 2 * time.Minute
	return t
}()

// SortedMapKeys returns the keys of m in sorted order.
func SortedMapKeys[V any](m map[int]V) []int {
	return slices.Sorted(maps.Keys(m))
}
