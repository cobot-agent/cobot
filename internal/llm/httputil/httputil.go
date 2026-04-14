package httputil

import (
	"maps"
	"net"
	"net/http"
	"slices"
	"time"
)

var DefaultTransport = &http.Transport{
	DialContext: (&net.Dialer{
		Timeout:   30 * time.Second,
		KeepAlive: 30 * time.Second,
	}).DialContext,
	TLSHandshakeTimeout:   10 * time.Second,
	ResponseHeaderTimeout: 2 * time.Minute,
}

// SortedMapKeys returns the keys of m in sorted order.
func SortedMapKeys[V any](m map[int]V) []int {
	return slices.Sorted(maps.Keys(m))
}
