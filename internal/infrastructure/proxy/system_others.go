//go:build !windows
// +build !windows

package proxy

func getSystemProxyFromRegistry() (string, error) {
	return "", nil
}
