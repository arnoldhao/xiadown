package browsercdp

import (
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"sync/atomic"
	"testing"
	"time"

	"github.com/chromedp/chromedp"
)

func TestManagedBrowserNetworkRouteLive(t *testing.T) {
	if os.Getenv("XIADOWN_BROWSER_ROUTE_LIVE") != "1" {
		t.Skip("set XIADOWN_BROWSER_ROUTE_LIVE=1 to run the managed browser route probe")
	}
	const token = "live-route-token"
	const proofID = "0123456789abcdef0123456789abcdef0123456789abcdef01234567"
	const beginHost = "0123456789abcdef0123456789abcdef.attest.xiadown.invalid"
	const challengeAuthority = proofID + ".connect." + beginHost
	const challengeHost = challengeAuthority + ":443"
	var requests atomic.Int32
	var connectObserved atomic.Bool
	proxyServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, request *http.Request) {
		requests.Add(1)
		if request.Method == http.MethodConnect {
			if request.Host != challengeHost {
				http.Error(w, "unexpected CONNECT", http.StatusForbidden)
				return
			}
			connectObserved.Store(true)
			http.Error(w, "CONNECT observed", http.StatusBadGateway)
			return
		}
		if request.URL.Hostname() != beginHost {
			http.Error(w, "unexpected destination", http.StatusBadGateway)
			return
		}
		if request.URL.RawQuery == "" {
			w.Header().Set("X-XiaDown-Gateway-Connect-Challenge", "https://"+challengeAuthority+"/.well-known/xiadown-network-route")
			w.WriteHeader(http.StatusNoContent)
			return
		}
		if request.URL.Query().Get("proof") != proofID || !connectObserved.Swap(false) {
			http.Error(w, "proof incomplete", http.StatusForbidden)
			return
		}
		w.Header().Set("X-XiaDown-Gateway-Attestation", token)
		w.WriteHeader(http.StatusNoContent)
	}))
	defer proxyServer.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	runtime, err := Start(ctx, LaunchOptions{
		Headless:    os.Getenv("XIADOWN_BROWSER_ROUTE_HEADFUL") != "1",
		UserDataDir: t.TempDir(),
		NetworkRoute: &ManagedNetworkRoute{
			ProxyURL:         proxyServer.URL,
			AttestationURL:   "http://" + beginHost + "/.well-known/xiadown-network-route",
			AttestationToken: token,
		},
	})
	if err != nil {
		t.Fatalf("%v (proxy requests=%d)", err, requests.Load())
	}
	if err := runtime.VerifyNetworkRoute(ctx); err != nil {
		runtime.Stop()
		t.Fatalf("repeat network route probe: %v (proxy requests=%d)", err, requests.Load())
	}
	targets, err := chromedp.Targets(runtime.BrowserContext())
	if err != nil {
		runtime.Stop()
		t.Fatalf("list targets after route probe: %v", err)
	}
	for _, info := range targets {
		if isManagedNetworkProbeTargetInfo(info) {
			runtime.Stop()
			t.Fatalf("managed network probe target leaked after verification: %s", info.TargetID)
		}
	}
	runtime.Stop()
}
