//go:build linux && (android || !cgo || server)

package proxy

import "net/url"

func platformSystemProxyURL(target *url.URL) (*url.URL, error) {
	return environmentSystemProxyURL(target)
}
