//go:build darwin && cgo && !server && !ios

package proxy

import (
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
)

const deterministicPAC = `
function FindProxyForURL(url, host) {
	if (host == "direct.example") return "DIRECT";
	return "PROXY first.proxy.test:8080; SOCKS second.proxy.test:1080; DIRECT";
}`

func TestDarwinPACJavaScriptIsPerURLAndPreservesOrder(t *testing.T) {
	t.Parallel()

	proxyURL, err := resolveDarwinPACScript(deterministicPAC, mustParseURL(t, "https://music.example/song?id=one"))
	if err != nil {
		t.Fatal(err)
	}
	if got := proxyURL.String(); got != "http://first.proxy.test:8080" {
		t.Fatalf("PAC first route = %q", got)
	}

	proxyURL, err = resolveDarwinPACScript(deterministicPAC, mustParseURL(t, "https://direct.example/song?id=one"))
	if err != nil || proxyURL != nil {
		t.Fatalf("PAC DIRECT = %v, %v", proxyURL, err)
	}
}

func TestDarwinPACURLIsDownloadedAndEvaluated(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(response http.ResponseWriter, _ *http.Request) {
		response.Header().Set("Content-Type", "application/x-ns-proxy-autoconfig")
		_, _ = response.Write([]byte(deterministicPAC))
	}))
	defer server.Close()
	proxyURL, err := resolveDarwinPACURL(server.URL, mustParseURL(t, "https://music.example/song?id=one"))
	if err != nil {
		t.Fatal(err)
	}
	if got := proxyURL.String(); got != "http://first.proxy.test:8080" {
		t.Fatalf("PAC URL first route = %q", got)
	}
}

func TestDarwinNativeSystemProxyResolver(t *testing.T) {
	if os.Getenv("XIADOWN_TEST_NATIVE_SYSTEM_PROXY") != "1" {
		t.Skip("set XIADOWN_TEST_NATIVE_SYSTEM_PROXY=1 to inspect this Mac's live CFNetwork decision")
	}
	proxyURL, err := platformSystemProxyURL(mustParseURL(t, "https://music.youtube.com/watch?v=probe"))
	if err != nil {
		t.Fatal(err)
	}
	if proxyURL != nil {
		t.Logf("CFNetwork decision: %s://%s", proxyURL.Scheme, proxyURL.Host)
		switch proxyURL.Scheme {
		case "http", "https", "socks5", "socks5h":
		default:
			t.Fatalf("unsupported native proxy result %q", proxyURL.Scheme)
		}
		if proxyURL.Hostname() == "" {
			t.Fatal("native proxy result has no host")
		}
	} else {
		t.Log("CFNetwork decision: DIRECT")
	}
}
