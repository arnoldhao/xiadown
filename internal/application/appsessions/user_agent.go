package appsessions

import (
	"runtime"
)

const (
	macDesktopSafariUserAgent   = "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/605.1.15 (KHTML, like Gecko) Version/17.0 Safari/605.1.15"
	windowsDesktopEdgeUserAgent = "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/124.0.0.0 Safari/537.36 Edg/124.0.0.0"
	linuxDesktopChromeUserAgent = "Mozilla/5.0 (X11; Linux x86_64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/124.0.0.0 Safari/537.36"
)

func WebViewUserAgent(_ string) string {
	return desktopUserAgent()
}

func HTTPUserAgent(_ string) string {
	return desktopUserAgent()
}

func desktopUserAgent() string {
	switch runtime.GOOS {
	case "windows":
		return windowsDesktopEdgeUserAgent
	case "darwin":
		return macDesktopSafariUserAgent
	default:
		return linuxDesktopChromeUserAgent
	}
}
