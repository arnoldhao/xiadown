package service

import "testing"

type onlinePetGatewayStub struct {
	proxyURL         string
	attestationURL   string
	attestationToken string
}

func (gateway onlinePetGatewayStub) ConsumerProxyURL() string { return gateway.proxyURL }
func (gateway onlinePetGatewayStub) ConsumerProxyAttestation() (string, string) {
	return gateway.attestationURL, gateway.attestationToken
}

func TestOnlinePetBrowserArgsAlwaysUseManagedGateway(t *testing.T) {
	t.Parallel()

	service := &Service{networkGateway: onlinePetGatewayStub{
		proxyURL:         "http://127.0.0.1:43199",
		attestationURL:   "http://192.0.2.1/.well-known/xiadown-network-route",
		attestationToken: "test",
	}}
	arguments, route, err := service.onlinePetBrowserOptions()
	if err != nil {
		t.Fatal(err)
	}
	if len(arguments) != 1 || route.ProxyURL != "http://127.0.0.1:43199" || route.AttestationToken != "test" {
		t.Fatalf("unexpected browser options: %#v %#v", arguments, route)
	}
}

func TestOnlinePetBrowserArgsRejectNonLoopbackGateway(t *testing.T) {
	t.Parallel()

	service := &Service{networkGateway: onlinePetGatewayStub{proxyURL: "http://proxy.example:8080"}}
	_, _, err := service.onlinePetBrowserOptions()
	if err == nil {
		t.Fatal("non-loopback gateway was accepted")
	}
}
