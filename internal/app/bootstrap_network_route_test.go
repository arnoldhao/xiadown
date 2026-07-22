package app

import (
	"os"
	"strings"
	"testing"
)

func TestBootstrapInstallsManagedGatewayBeforeAnyWebView(t *testing.T) {
	t.Parallel()

	sourceBytes, err := os.ReadFile("bootstrap.go")
	if err != nil {
		t.Fatal(err)
	}
	source := string(sourceBytes)
	manager := strings.Index(source, "proxy.NewManager(")
	applicationNew := strings.Index(source, "application.New(application.Options{")
	windowManager := strings.Index(source, "wails.NewWindowManager(")
	policyApply := strings.Index(source, "proxyManager.Apply(proxy.Config{")
	if manager < 0 || applicationNew < 0 || windowManager < 0 || policyApply < 0 {
		t.Fatalf("bootstrap is missing network route construction: manager=%d app=%d apply=%d windows=%d", manager, applicationNew, policyApply, windowManager)
	}
	if !(manager < applicationNew && applicationNew < policyApply && policyApply < windowManager) {
		t.Fatalf("network route order is unsafe: manager=%d app=%d apply=%d windows=%d", manager, applicationNew, policyApply, windowManager)
	}

	for _, required := range []string{
		"wails.NewWebViewNetworkRoute(proxyManager)",
		"webViewNetworkRoute.WebView2BrowserArguments()",
		"AdditionalBrowserArgs: webView2BrowserArguments",
		"application.NewService(webViewNetworkRoute)",
		"petsservice.WithNetworkGateway(proxyManager)",
		"proxyManager.Close()",
	} {
		if !strings.Contains(source, required) {
			t.Fatalf("bootstrap is missing %q", required)
		}
	}
}
