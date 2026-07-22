//go:build darwin && (ios || !cgo || server)

package proxy

import "net/url"

func platformSystemProxyURL(target *url.URL) (*url.URL, error) {
	return environmentSystemProxyURL(target)
}
