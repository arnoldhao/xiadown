//go:build !darwin && !linux && !windows

package proxy

import "net/url"

func platformSystemProxyURL(target *url.URL) (*url.URL, error) {
	return environmentSystemProxyURL(target)
}
