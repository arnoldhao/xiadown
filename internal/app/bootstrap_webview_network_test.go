package app

import (
	"os"
	"strings"
	"testing"
)

func TestBootstrapLeavesNativeWebViewNetworkUnmanaged(t *testing.T) {
	t.Parallel()

	sourceBytes, err := os.ReadFile("bootstrap.go")
	if err != nil {
		t.Fatal(err)
	}
	source := string(sourceBytes)

	for _, required := range []string{
		"proxy.NewManager(",
		"application.New(application.Options{",
		"petsservice.WithNetworkGateway(proxyManager)",
		"proxyManager.Close()",
	} {
		if !strings.Contains(source, required) {
			t.Fatalf("bootstrap is missing %q", required)
		}
	}

	for _, forbidden := range []string{
		"NewWebViewNetworkRoute",
		"WebView2BrowserArguments",
		"AdditionalBrowserArgs",
		"WEBVIEW2_ADDITIONAL_BROWSER_ARGUMENTS",
		"application.NewService(webViewNetworkRoute)",
		"RegisterInternalLoopbackURL",
	} {
		if strings.Contains(source, forbidden) {
			t.Fatalf("bootstrap must leave native WebView networking to the platform; found %q", forbidden)
		}
	}
}
