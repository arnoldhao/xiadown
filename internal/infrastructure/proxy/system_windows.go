//go:build windows
// +build windows

package proxy

import (
	"runtime"

	"golang.org/x/sys/windows/registry"
)

func getSystemProxyFromRegistry() (string, error) {
	if runtime.GOOS != "windows" {
		return "", nil
	}

	key, err := registry.OpenKey(registry.CURRENT_USER, `Software\Microsoft\Windows\CurrentVersion\Internet Settings`, registry.QUERY_VALUE)
	if err != nil {
		return "", err
	}
	defer key.Close()

	proxyEnabled, _, err := key.GetIntegerValue("ProxyEnable")
	if err != nil || proxyEnabled == 0 {
		return "", nil
	}

	proxyServer, _, err := key.GetStringValue("ProxyServer")
	if err != nil {
		return "", err
	}

	return proxyServer, nil
}
