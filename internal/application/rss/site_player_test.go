package rss

import (
	"context"
	"errors"
	"net"
	"testing"
	"time"

	appcookies "xiadown/internal/application/cookies"
	"xiadown/internal/application/networkpolicy"
)

type sitePlayerStaticResolver map[string][]net.IPAddr

func (resolver sitePlayerStaticResolver) LookupIPAddr(_ context.Context, host string) ([]net.IPAddr, error) {
	addresses, ok := resolver[host]
	if !ok {
		return nil, errors.New("unexpected DNS host " + host)
	}
	return addresses, nil
}

type sitePlayerCookieProvider struct {
	records  []appcookies.Record
	siteKeys []string
}

func (provider *sitePlayerCookieProvider) RecordsForSiteKey(_ context.Context, siteKey string) ([]appcookies.Record, error) {
	provider.siteKeys = append(provider.siteKeys, siteKey)
	return provider.records, nil
}

func TestSitePlayerPrepareDerivesKnownPolicyAndCookieScopeFromURL(t *testing.T) {
	provider := &sitePlayerCookieProvider{records: []appcookies.Record{
		{Name: "SESSDATA", Value: "session", Domain: ".bilibili.com", Path: "/", Expires: time.Now().Add(time.Hour).Unix()},
		{Name: "foreign", Value: "secret", Domain: ".attacker.example", Path: "/"},
		{Name: "expired", Value: "old", Domain: ".b23.tv", Path: "/", Expires: time.Now().Add(-time.Hour).Unix()},
	}}
	service := NewSitePlayerService(provider)
	service.resolver = sitePlayerStaticResolver{"b23.tv": {{IP: net.ParseIP("8.8.8.8")}}}

	descriptor, err := service.Prepare(context.Background(), "https://b23.tv/abc123")
	if err != nil {
		t.Fatal(err)
	}
	if descriptor.SiteKey != "bilibili" || descriptor.RegistrableSite != "" {
		t.Fatalf("policy identity = siteKey %q registrable %q", descriptor.SiteKey, descriptor.RegistrableSite)
	}
	if len(provider.siteKeys) != 1 || provider.siteKeys[0] != "bilibili" {
		t.Fatalf("credential lookup keys = %#v", provider.siteKeys)
	}
	if !descriptor.CredentialsLoaded || len(descriptor.Cookies) != 1 || descriptor.Cookies[0].Name != "SESSDATA" {
		t.Fatalf("filtered credentials = loaded %v records %#v", descriptor.CredentialsLoaded, descriptor.Cookies)
	}
	if len(descriptor.AllowedDomains) != 2 {
		t.Fatalf("allowed domains = %#v", descriptor.AllowedDomains)
	}
}

func TestSitePlayerPrepareUnknownSiteUsesRegistrableScopeWithoutCredentials(t *testing.T) {
	provider := &sitePlayerCookieProvider{records: []appcookies.Record{{Name: "secret", Value: "value", Domain: ".example.com"}}}
	service := NewSitePlayerService(provider)
	service.resolver = sitePlayerStaticResolver{"video.example.com": {{IP: net.ParseIP("1.1.1.1")}}}

	descriptor, err := service.Prepare(context.Background(), "https://video.example.com/watch/42")
	if err != nil {
		t.Fatal(err)
	}
	if descriptor.SiteKey != "example.com" || descriptor.RegistrableSite != "example.com" || len(descriptor.AllowedDomains) != 0 {
		t.Fatalf("unknown-site scope = %#v", descriptor)
	}
	if descriptor.CredentialsLoaded || len(descriptor.Cookies) != 0 || len(provider.siteKeys) != 0 {
		t.Fatalf("unknown site received credentials: descriptor=%#v lookups=%#v", descriptor, provider.siteKeys)
	}
}

func TestSitePlayerPrepareDouyinPolicyLoadsScopedCookies(t *testing.T) {
	provider := &sitePlayerCookieProvider{records: []appcookies.Record{{Name: "secret", Value: "value", Domain: ".douyin.com"}}}
	service := NewSitePlayerService(provider)
	service.resolver = sitePlayerStaticResolver{"www.douyin.com": {{IP: net.ParseIP("8.8.4.4")}}}

	descriptor, err := service.Prepare(context.Background(), "https://www.douyin.com/video/123")
	if err != nil {
		t.Fatal(err)
	}
	if descriptor.SiteKey != "douyin" || !descriptor.CredentialsLoaded || len(descriptor.Cookies) != 1 {
		t.Fatalf("douyin cookie policy = descriptor %#v lookups %#v", descriptor, provider.siteKeys)
	}
	if len(provider.siteKeys) != 1 || provider.siteKeys[0] != "douyin" {
		t.Fatalf("douyin credential lookup keys = %#v", provider.siteKeys)
	}
}

func TestSitePlayerPrepareXiaohongshuPolicyLoadsOnlyMainlandCookies(t *testing.T) {
	provider := &sitePlayerCookieProvider{records: []appcookies.Record{
		{Name: "web_session", Value: "session", Domain: ".xiaohongshu.com", Path: "/"},
		{Name: "rednote_session", Value: "separate-platform", Domain: ".rednote.com", Path: "/"},
	}}
	service := NewSitePlayerService(provider)
	service.resolver = sitePlayerStaticResolver{"www.xiaohongshu.com": {{IP: net.ParseIP("8.8.8.8")}}}

	descriptor, err := service.Prepare(context.Background(), "https://www.xiaohongshu.com/explore/123")
	if err != nil {
		t.Fatal(err)
	}
	if descriptor.SiteKey != "xiaohongshu" || !descriptor.CredentialsLoaded || len(descriptor.Cookies) != 1 || descriptor.Cookies[0].Name != "web_session" {
		t.Fatalf("xiaohongshu cookie policy = descriptor %#v lookups %#v", descriptor, provider.siteKeys)
	}
	if len(provider.siteKeys) != 1 || provider.siteKeys[0] != "xiaohongshu" {
		t.Fatalf("xiaohongshu credential lookup keys = %#v", provider.siteKeys)
	}
}

func TestSitePlayerPrepareRejectsPrivateDNSAnswer(t *testing.T) {
	service := NewSitePlayerService(nil)
	service.resolver = sitePlayerStaticResolver{"video.example.com": {{IP: net.ParseIP("127.0.0.1")}}}
	_, err := service.Prepare(context.Background(), "https://video.example.com/watch")
	if !errors.Is(err, networkpolicy.ErrDestinationBlocked) {
		t.Fatalf("private DNS error = %v, want ErrDestinationBlocked", err)
	}
}

func TestSitePlayerPrepareRejectsUnsafeURLsBeforeDNS(t *testing.T) {
	service := NewSitePlayerService(nil)
	for _, rawURL := range []string{
		"http://example.com/video",
		"https://user@example.com/video",
		"https://localhost/video",
		"https://127.0.0.1/video",
		"https://example.com:8443/video",
		"file:///tmp/video",
	} {
		if _, err := service.Prepare(context.Background(), rawURL); err == nil {
			t.Errorf("Prepare(%q) unexpectedly succeeded", rawURL)
		}
	}
}
