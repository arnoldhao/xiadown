//go:build !windows

package proxy

func newPlatformSystemProxyResolver() systemProxyResolver {
	return statelessSystemProxyResolver{}
}
