package proxy

import "testing"

func TestShouldBypassHostPortUsesStrictBoundaries(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name  string
		host  string
		port  string
		rules []string
		want  bool
	}{
		{name: "exact domain", host: "example.com", port: "443", rules: []string{"example.com"}, want: true},
		{name: "boundary suffix", host: "api.example.com", port: "443", rules: []string{"example.com"}, want: true},
		{name: "not substring", host: "badexample.com", port: "443", rules: []string{"example.com"}, want: false},
		{name: "leading dot suffix", host: "api.example.com", port: "443", rules: []string{".example.com"}, want: true},
		{name: "wildcard excludes apex", host: "example.com", port: "443", rules: []string{"*.example.com"}, want: false},
		{name: "wildcard includes child", host: "a.example.com", port: "443", rules: []string{"*.example.com"}, want: true},
		{name: "port match", host: "example.com", port: "8443", rules: []string{"example.com:8443"}, want: true},
		{name: "port mismatch", host: "example.com", port: "443", rules: []string{"example.com:8443"}, want: false},
		{name: "IPv4 CIDR", host: "10.23.4.5", port: "80", rules: []string{"10.0.0.0/8"}, want: true},
		{name: "IPv4 outside CIDR", host: "11.23.4.5", port: "80", rules: []string{"10.0.0.0/8"}, want: false},
		{name: "IPv6 CIDR", host: "2001:db8::42", port: "443", rules: []string{"2001:db8::/32"}, want: true},
		{name: "IPv6 exact with port", host: "2001:db8::42", port: "8443", rules: []string{"[2001:db8::42]:8443"}, want: true},
		{name: "IPv6 port mismatch", host: "2001:db8::42", port: "443", rules: []string{"[2001:db8::42]:8443"}, want: false},
		{name: "case and trailing dot", host: "API.EXAMPLE.COM.", port: "443", rules: []string{"Example.Com"}, want: true},
		{name: "URL rule", host: "api.example.com", port: "8443", rules: []string{"https://api.example.com:8443"}, want: true},
		{name: "loopback IPv4 automatic", host: "127.99.1.2", port: "80", want: true},
		{name: "loopback IPv6 automatic", host: "::1", port: "80", want: true},
		{name: "localhost automatic", host: "localhost", port: "80", want: true},
		{name: "all", host: "anything.test", port: "80", rules: []string{"*"}, want: true},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			if got := shouldBypassHostPort(test.host, test.port, test.rules); got != test.want {
				t.Fatalf("shouldBypassHostPort(%q, %q, %v) = %v, want %v", test.host, test.port, test.rules, got, test.want)
			}
		})
	}
}
