package rss

import (
	"context"
	"errors"
	"fmt"
	"net"
	"strings"
	"time"

	appcookies "xiadown/internal/application/cookies"
	"xiadown/internal/application/networkpolicy"
	"xiadown/internal/application/sitepolicy"

	"golang.org/x/net/idna"
	"golang.org/x/net/publicsuffix"
)

const (
	sitePlayerCookieLookupTimeout = time.Second
	sitePlayerDNSLookupTimeout    = 3 * time.Second
)

var errSitePlayerCookieLookupTimeout = errors.New("RSS site player cookie lookup timed out")

// SitePlaybackDescriptor is a process-local playback capability. Only its
// public identity fields cross the Wails boundary; navigation scope and
// credentials remain in the native process.
type SitePlaybackDescriptor struct {
	URL               string              `json:"url"`
	SiteKey           string              `json:"siteKey"`
	CredentialsLoaded bool                `json:"credentialsLoaded"`
	AllowedDomains    []string            `json:"-"`
	RegistrableSite   string              `json:"-"`
	Cookies           []appcookies.Record `json:"-"`
}

// SitePlayerService prepares an interactive, site-owned playback page. It is
// deliberately separate from VideoPlayerService so the optimized Bilibili
// bridge keeps its narrow identity and scripting contract.
type SitePlayerService struct {
	cookies             VideoPlayerCookieProvider
	resolver            networkpolicy.Resolver
	now                 func() time.Time
	cookieLookupTimeout time.Duration
	dnsLookupTimeout    time.Duration
}

func NewSitePlayerService(cookies VideoPlayerCookieProvider) *SitePlayerService {
	return &SitePlayerService{
		cookies:             cookies,
		now:                 time.Now,
		cookieLookupTimeout: sitePlayerCookieLookupTimeout,
		dnsLookupTimeout:    sitePlayerDNSLookupTimeout,
	}
}

func (service *SitePlayerService) Prepare(
	ctx context.Context,
	rawURL string,
) (SitePlaybackDescriptor, error) {
	canonicalURL, host, registrableSite, err := normalizeSitePlayerURL(rawURL)
	if err != nil {
		return SitePlaybackDescriptor{}, err
	}
	if ctx == nil {
		ctx = context.Background()
	}
	dnsTimeout := sitePlayerDNSLookupTimeout
	if service != nil && service.dnsLookupTimeout > 0 {
		dnsTimeout = service.dnsLookupTimeout
	}
	resolveCtx, cancel := context.WithTimeout(ctx, dnsTimeout)
	_, resolveErr := networkpolicy.ResolvePublicIPs(resolveCtx, serviceResolver(service), host)
	cancel()
	if resolveErr != nil {
		return SitePlaybackDescriptor{}, resolveErr
	}

	descriptor := SitePlaybackDescriptor{
		URL:             canonicalURL,
		SiteKey:         registrableSite,
		RegistrableSite: registrableSite,
	}
	policy, known := sitepolicy.ForURL(canonicalURL)
	if !known {
		return descriptor, nil
	}
	if !sitepolicy.MatchDomains(canonicalURL, policy.Domains) {
		return SitePlaybackDescriptor{}, fmt.Errorf("RSS site player URL is outside its site policy")
	}
	descriptor.SiteKey = strings.TrimSpace(policy.SiteKey)
	descriptor.AllowedDomains = cloneNonEmptyStrings(policy.Domains)
	descriptor.RegistrableSite = ""
	if descriptor.SiteKey == "" || len(descriptor.AllowedDomains) == 0 {
		return SitePlaybackDescriptor{}, fmt.Errorf("RSS site player policy for %s is incomplete", host)
	}
	if service == nil || service.cookies == nil || !sitePolicyHasCapability(policy, "cookies") {
		return descriptor, nil
	}

	records, err := service.sitePlayerCookies(ctx, descriptor.SiteKey)
	if err != nil {
		if errors.Is(err, errSitePlayerCookieLookupTimeout) {
			return descriptor, nil
		}
		if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
			return SitePlaybackDescriptor{}, err
		}
		return descriptor, nil
	}
	now := time.Now()
	if service.now != nil {
		now = service.now()
	}
	descriptor.Cookies = filterSitePlayerCookies(records, descriptor.AllowedDomains, now)
	descriptor.CredentialsLoaded = len(descriptor.Cookies) > 0
	return descriptor, nil
}

func (service *SitePlayerService) sitePlayerCookies(
	ctx context.Context,
	siteKey string,
) ([]appcookies.Record, error) {
	if ctx == nil {
		ctx = context.Background()
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	timeout := service.cookieLookupTimeout
	if timeout <= 0 {
		timeout = sitePlayerCookieLookupTimeout
	}
	lookupCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	result := make(chan videoPlayerCookieLookupResult, 1)
	go func() {
		records, err := service.cookies.RecordsForSiteKey(lookupCtx, siteKey)
		select {
		case result <- videoPlayerCookieLookupResult{records: records, err: err}:
		case <-lookupCtx.Done():
		}
	}()

	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	case <-lookupCtx.Done():
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		return nil, errSitePlayerCookieLookupTimeout
	case loaded := <-result:
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		if lookupCtx.Err() != nil {
			return nil, errSitePlayerCookieLookupTimeout
		}
		return loaded.records, loaded.err
	}
}

func normalizeSitePlayerURL(rawURL string) (string, string, string, error) {
	trimmed := strings.TrimSpace(rawURL)
	parsed, err := networkpolicy.ValidatePublicHTTPURL(trimmed)
	if err != nil || parsed == nil || parsed.Opaque != "" || parsed.User != nil || parsed.Hostname() == "" {
		return "", "", "", fmt.Errorf("invalid RSS site player URL")
	}
	if !strings.EqualFold(parsed.Scheme, "https") || (parsed.Port() != "" && parsed.Port() != "443") {
		return "", "", "", fmt.Errorf("RSS site player URL must use HTTPS on the default port")
	}
	host, err := idna.Lookup.ToASCII(strings.TrimSuffix(strings.ToLower(parsed.Hostname()), "."))
	if err != nil || host == "" || net.ParseIP(host) != nil {
		return "", "", "", fmt.Errorf("RSS site player URL requires a public DNS host")
	}
	registrableSite, err := publicsuffix.EffectiveTLDPlusOne(host)
	if err != nil || registrableSite == "" {
		return "", "", "", fmt.Errorf("RSS site player URL requires a registrable host")
	}
	return trimmed, host, strings.ToLower(registrableSite), nil
}

func serviceResolver(service *SitePlayerService) networkpolicy.Resolver {
	if service != nil && service.resolver != nil {
		return service.resolver
	}
	return net.DefaultResolver
}

func filterSitePlayerCookies(records []appcookies.Record, domains []string, now time.Time) []appcookies.Record {
	filtered := appcookies.FilterByDomains(records, domains)
	result := make([]appcookies.Record, 0, len(filtered))
	nowUnix := now.Unix()
	for _, record := range filtered {
		if strings.TrimSpace(record.Name) == "" || strings.TrimSpace(record.Value) == "" {
			continue
		}
		if record.Expires > 0 && record.Expires <= nowUnix {
			continue
		}
		result = append(result, record)
	}
	return result
}

func sitePolicyHasCapability(policy sitepolicy.Policy, capability string) bool {
	for _, candidate := range policy.Capabilities {
		if strings.EqualFold(strings.TrimSpace(candidate), strings.TrimSpace(capability)) {
			return true
		}
	}
	return false
}

func cloneNonEmptyStrings(values []string) []string {
	result := make([]string, 0, len(values))
	for _, value := range values {
		if value = strings.TrimSpace(value); value != "" {
			result = append(result, value)
		}
	}
	return result
}
