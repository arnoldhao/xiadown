package proxy

import (
	"errors"
	"net/url"
	"strings"
	"testing"
)

func TestParseWindowsResolvedProxyForScheme(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name       string
		proxy      string
		target     string
		want       string
		wantDirect bool
		wantErr    error
	}{
		{name: "static protocol map", proxy: "http=one.test:80;https=two.test:443", target: "https", want: "http://two.test:443"},
		{name: "PAC HTTP directive", proxy: "PROXY one.test:8080; DIRECT", target: "https", want: "http://one.test:8080"},
		{name: "PAC HTTPS directive", proxy: "HTTPS secure.test:8443", target: "https", want: "https://secure.test:8443"},
		{name: "PAC SOCKS directive", proxy: "SOCKS5 socks.test:1080", target: "https", want: "socks5://socks.test:1080"},
		{name: "PAC generic SOCKS is v4", proxy: "SOCKS socks.test:1080", target: "https", wantErr: errWindowsSystemSOCKS4Unsupported},
		{name: "PAC explicit SOCKS4", proxy: "SOCKS4 socks.test:1080", target: "https", wantErr: errWindowsSystemSOCKS4Unsupported},
		{name: "static generic SOCKS is v4", proxy: "socks=socks.test:1080", target: "https", wantErr: errWindowsSystemSOCKS4Unsupported},
		{name: "static explicit SOCKS5", proxy: "socks5=socks.test:1080", target: "https", want: "socks5://socks.test:1080"},
		{name: "PAC direct", proxy: "DIRECT", target: "https", wantDirect: true},
		{name: "ordered named list", proxy: "first.test:80;second.test:80", target: "http", want: "http://first.test:80"},
		{name: "whitespace protocol map", proxy: "http=one.test:80 https=two.test:443", target: "https", want: "http://two.test:443"},
		{name: "whitespace named list", proxy: "first.test:80 second.test:80", target: "http", want: "http://first.test:80"},
		{name: "unmapped protocol is direct", proxy: "http=one.test:80", target: "https", wantDirect: true},
		{name: "unkeyed default fills protocol", proxy: "http=one.test:80;default.test:8080", target: "https", want: "http://default.test:8080"},
	}
	for _, test := range tests {
		test := test
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			got, err := parseWindowsResolvedProxyForScheme(test.proxy, test.target)
			if test.wantErr != nil {
				if !errors.Is(err, test.wantErr) {
					t.Fatalf("error = %v, want %v", err, test.wantErr)
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			if test.wantDirect {
				if got != nil {
					t.Fatalf("proxy = %s, want DIRECT", got)
				}
				return
			}
			if got == nil || got.String() != test.want {
				t.Fatalf("proxy = %v, want %q", got, test.want)
			}
		})
	}
}

func TestWindowsProxyBypass(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name   string
		target string
		list   string
		want   bool
	}{
		{name: "local hostname", target: "http://intranet/", list: "<local>", want: true},
		{name: "local does not include IP", target: "http://10.0.0.8/", list: "<local>", want: false},
		{name: "wildcard domain", target: "https://API.Example.COM/", list: "*.example.com", want: true},
		{name: "port matches", target: "https://example.com:8443/", list: "example.com:8443", want: true},
		{name: "port differs", target: "https://example.com:443/", list: "example.com:8443", want: false},
		{name: "whitespace separator", target: "https://foo.corp.test/", list: "localhost *.corp.test", want: true},
		{name: "IPv6 literal", target: "http://[::1]/", list: "[::1]", want: true},
		{name: "URL form is not a server pattern", target: "https://example.net/", list: "https://example.net", want: false},
		{name: "unrelated", target: "https://example.net/", list: "*.example.com;<local>", want: false},
	}
	for _, test := range tests {
		test := test
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			target, err := url.Parse(test.target)
			if err != nil {
				t.Fatal(err)
			}
			if got := windowsProxyBypasses(target, test.list); got != test.want {
				t.Fatalf("windowsProxyBypasses(%q, %q) = %v, want %v", test.target, test.list, got, test.want)
			}
		})
	}
}

func TestResolveWindowsNamedProxyAppliesProxyOverride(t *testing.T) {
	t.Parallel()
	target, _ := url.Parse("https://music.example.com/watch")
	proxyURL, err := resolveWindowsNamedProxy(target, "http=proxy.test:8080;https=secure-proxy.test:8443", "*.example.com")
	if err != nil {
		t.Fatal(err)
	}
	if proxyURL != nil {
		t.Fatalf("proxy = %s, want DIRECT from ProxyOverride", proxyURL)
	}
}

func TestWindowsProxyEndpointRejectsPathAndInvalidPortWithoutEchoingInput(t *testing.T) {
	t.Parallel()
	for _, raw := range []string{"http://sensitive-proxy.test/private", "sensitive-proxy.test:99999"} {
		if _, err := parseWindowsProxyEndpoint(raw, "https"); err == nil {
			t.Fatalf("proxy %q was accepted", raw)
		} else if strings.Contains(err.Error(), "sensitive-proxy") || strings.Contains(err.Error(), "private") {
			t.Fatalf("proxy parse error leaked configuration: %q", err)
		}
	}
}
