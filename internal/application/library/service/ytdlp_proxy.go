package service

import (
	"fmt"
	"net"
	"net/url"
	"strconv"
	"strings"

	"xiadown/internal/application/browsercdp"
	"xiadown/internal/application/library/dto"
)

// managedConsumerProxyURL returns the process-lifetime loopback gateway used
// by browser and helper-process consumers. These consumers must never treat a
// missing gateway as permission to inherit their own system proxy or connect
// directly.
func (service *LibraryService) managedConsumerProxyURL() (string, error) {
	if service == nil || service.proxyClient == nil {
		return "", fmt.Errorf("managed network gateway is unavailable")
	}
	provider, ok := service.proxyClient.(interface {
		ConsumerProxyURL() string
	})
	if !ok {
		return "", fmt.Errorf("managed network gateway provider is unavailable")
	}
	raw := strings.TrimSpace(provider.ConsumerProxyURL())
	parsed, err := url.Parse(raw)
	if err != nil || parsed == nil {
		return "", fmt.Errorf("managed network gateway URL is invalid")
	}
	if !strings.EqualFold(parsed.Scheme, "http") || parsed.User != nil || parsed.Opaque != "" ||
		parsed.RawQuery != "" || parsed.Fragment != "" || (parsed.Path != "" && parsed.Path != "/") {
		return "", fmt.Errorf("managed network gateway URL is invalid")
	}
	host := strings.TrimSpace(parsed.Hostname())
	ip := net.ParseIP(strings.Trim(host, "[]"))
	if !strings.EqualFold(host, "localhost") && (ip == nil || !ip.IsLoopback()) {
		return "", fmt.Errorf("managed network gateway is not loopback")
	}
	port, err := strconv.ParseUint(parsed.Port(), 10, 16)
	if err != nil || port == 0 {
		return "", fmt.Errorf("managed network gateway port is invalid")
	}
	return strings.TrimSuffix(raw, "/"), nil
}

func (service *LibraryService) managedBrowserNetworkRoute() (*browsercdp.ManagedNetworkRoute, error) {
	proxyURL, err := service.managedConsumerProxyURL()
	if err != nil {
		return nil, err
	}
	provider, ok := service.proxyClient.(interface {
		ConsumerProxyAttestation() (string, string)
	})
	if !ok {
		return nil, fmt.Errorf("managed network gateway attestation is unavailable")
	}
	attestationURL, attestationToken := provider.ConsumerProxyAttestation()
	if strings.TrimSpace(attestationURL) == "" || strings.TrimSpace(attestationToken) == "" {
		return nil, fmt.Errorf("managed network gateway attestation is unavailable")
	}
	return &browsercdp.ManagedNetworkRoute{
		ProxyURL:         proxyURL,
		AttestationURL:   strings.TrimSpace(attestationURL),
		AttestationToken: strings.TrimSpace(attestationToken),
	}, nil
}

func isRestrictedPublicRequest(request dto.CreateYTDLPJobRequest) bool {
	return strings.EqualFold(strings.TrimSpace(request.Source), "public_api")
}

func (service *LibraryService) resolveYTDLPProxyForRequest(request dto.CreateYTDLPJobRequest) string {
	if restricted := strings.TrimSpace(request.RestrictedProxyURL); restricted != "" {
		return restricted
	}
	return service.resolveYTDLPProxy(request.URL)
}

func (service *LibraryService) resolveYTDLPProxy(targetURL string) string {
	if service == nil || service.proxyClient == nil {
		return ""
	}
	// Browser and helper consumers always connect to the stable XiaDown
	// gateway. NoProxy and system PAC decisions are made for each actual target
	// inside that gateway; returning an empty value for a DIRECT decision would
	// let Chromium/yt-dlp fall back to their own system proxy instead.
	if gateway, ok := service.proxyClient.(interface {
		ConsumerProxyURL() string
	}); ok {
		if proxyURL := strings.TrimSpace(gateway.ConsumerProxyURL()); proxyURL != "" {
			return proxyURL
		}
	}
	resolver, ok := service.proxyClient.(interface {
		ResolveProxy(string) (string, error)
	})
	if !ok {
		return ""
	}
	proxyURL, err := resolver.ResolveProxy(strings.TrimSpace(targetURL))
	if err != nil {
		return ""
	}
	return strings.TrimSpace(proxyURL)
}
