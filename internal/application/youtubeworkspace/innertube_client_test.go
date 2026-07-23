package youtubeworkspace

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	appcookies "xiadown/internal/application/cookies"
	"xiadown/internal/domain/appsessions"
)

type innerTubeCookieProviderStub struct {
	mu      sync.Mutex
	records []appcookies.Record
	err     error
	keys    []string
}

func (stub *innerTubeCookieProviderStub) RecordsForSiteKey(
	_ context.Context,
	siteKey string,
) ([]appcookies.Record, error) {
	stub.mu.Lock()
	defer stub.mu.Unlock()
	stub.keys = append(stub.keys, siteKey)
	return append([]appcookies.Record(nil), stub.records...), stub.err
}

func (stub *innerTubeCookieProviderStub) set(records []appcookies.Record, err error) {
	stub.mu.Lock()
	stub.records = append([]appcookies.Record(nil), records...)
	stub.err = err
	stub.mu.Unlock()
}

type innerTubeHTTPClientProviderStub struct {
	client *http.Client
}

func (stub *innerTubeHTTPClientProviderStub) HTTPClient() *http.Client {
	return stub.client
}

type innerTubeRoundTripFunc func(*http.Request) (*http.Response, error)

func (fn innerTubeRoundTripFunc) RoundTrip(request *http.Request) (*http.Response, error) {
	return fn(request)
}

func TestInnerTubeSAPISIDHashMatchesYouTubeOriginVector(t *testing.T) {
	got := innerTubeSAPISIDHash("test-sapisid", youtubeInnerTubeOrigin, 1_700_000_000)
	want := "1700000000_14963cac63f39c9532ddd26bf69ca8d5e4d8aab6"
	if got != want {
		t.Fatalf("expected %q, got %q", want, got)
	}
	if music := innerTubeSAPISIDHash("test-sapisid", "https://music.youtube.com", 1_700_000_000); music == got {
		t.Fatal("youtube and youtube music origins must produce different hashes")
	}
}

func TestInnerTubeAuthenticatedRequestUsesWEBIdentityWithoutAPIKey(t *testing.T) {
	fixedNow := time.Unix(1_700_000_000, 0)
	authCookies := innerTubeTestCookies("test-sapisid", "sid-a")
	authCookies = append(authCookies,
		appcookies.Record{Name: "SIDCC", Value: "live-cc", Domain: ".youtube.com", Path: "/", Expires: 4_102_444_800},
		appcookies.Record{Name: "__Secure-3PSIDTS", Value: "live-ts", Domain: ".youtube.com", Path: "/", Expires: 4_102_444_800},
	)
	cookies := &innerTubeCookieProviderStub{records: authCookies}
	var capturedBody map[string]any
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		if request.Method != http.MethodPost || request.URL.Path != "/youtubei/v1/browse" {
			t.Fatalf("unexpected request: %s %s", request.Method, request.URL.Path)
		}
		if request.URL.Query().Get("prettyPrint") != "false" {
			t.Fatalf("prettyPrint must be false: %s", request.URL.RawQuery)
		}
		if _, exists := request.URL.Query()["key"]; exists {
			t.Fatalf("regular YouTube WEB requests must not send an API key: %s", request.URL.RawQuery)
		}
		if got := request.Header.Get("Origin"); got != youtubeInnerTubeOrigin {
			t.Fatalf("unexpected Origin: %q", got)
		}
		if got := request.Header.Get("Referer"); got != youtubeInnerTubeOrigin {
			t.Fatalf("unexpected Referer: %q", got)
		}
		if got := request.Header.Get("X-Origin"); got != youtubeInnerTubeOrigin {
			t.Fatalf("unexpected X-Origin: %q", got)
		}
		if got := request.Header.Get("X-Goog-AuthUser"); got != "0" {
			t.Fatalf("unexpected X-Goog-AuthUser: %q", got)
		}
		wantAuthorization := "SAPISIDHASH " + innerTubeSAPISIDHash(
			"test-sapisid",
			youtubeInnerTubeOrigin,
			fixedNow.Unix(),
		)
		if got := request.Header.Get("Authorization"); got != wantAuthorization {
			t.Fatalf("expected authorization %q, got %q", wantAuthorization, got)
		}
		if got := request.Header.Get("Cookie"); !strings.Contains(got, "__Secure-3PAPISID=test-sapisid") || !strings.Contains(got, "SID=sid-a") {
			t.Fatalf("unexpected Cookie header: %q", got)
		}
		if got := request.Header.Get("Cookie"); !strings.Contains(got, "SIDCC=live-cc") || !strings.Contains(got, "__Secure-3PSIDTS=live-ts") {
			t.Fatalf("live WebKit security cookies missing from request header: %q", got)
		}
		if got := request.Header.Get("User-Agent"); got != "XiaDown-Test-UA" {
			t.Fatalf("unexpected User-Agent: %q", got)
		}
		if got := request.Header.Get("Accept-Language"); got != "zh-CN,zh;q=0.9,en;q=0.7" {
			t.Fatalf("unexpected Accept-Language: %q", got)
		}
		if err := json.NewDecoder(request.Body).Decode(&capturedBody); err != nil {
			t.Fatalf("decode body: %v", err)
		}
		writer.Header().Set("Content-Type", "application/json")
		_, _ = io.WriteString(writer, `{"ok":true}`)
	}))
	defer server.Close()

	client := newInnerTubeClient(cookies, nil)
	client.baseURL = server.URL + "/youtubei/v1"
	client.now = func() time.Time { return fixedNow }
	client.setUserAgent("XiaDown-Test-UA")
	client.retryDelays = nil

	_, err := client.requestRead(
		withInnerTubeLocale(context.Background(), "zh-Hans-CN"),
		"browse",
		map[string]any{"browseId": "FEwhat_to_watch"},
		innerTubeAuthOptional,
	)
	if err != nil {
		t.Fatalf("request: %v", err)
	}
	if capturedBody["browseId"] != "FEwhat_to_watch" {
		t.Fatalf("request body lost browseId: %#v", capturedBody)
	}
	contextValue, _ := capturedBody["context"].(map[string]any)
	clientContext, _ := contextValue["client"].(map[string]any)
	if clientContext["clientName"] != youtubeInnerTubeClientName ||
		clientContext["clientVersion"] != youtubeInnerTubeClientVersion ||
		clientContext["hl"] != "zh-CN" || clientContext["userAgent"] != "XiaDown-Test-UA" {
		t.Fatalf("unexpected WEB client context: %#v", clientContext)
	}
	if len(cookies.keys) != 1 || cookies.keys[0] != "youtube" {
		t.Fatalf("expected direct youtube App Session access, got %#v", cookies.keys)
	}
}

func TestInnerTubeBrowserIdentityMatchesPlatformUserAgent(t *testing.T) {
	tests := []struct {
		name        string
		userAgent   string
		browserName string
		browser     string
		osName      string
		osVersion   string
	}{
		{
			name:        "macOS Safari",
			userAgent:   youtubeInnerTubeDefaultUA,
			browserName: "Safari",
			browser:     "17.0",
			osName:      "Macintosh",
			osVersion:   "10_15_7",
		},
		{
			name:        "Windows Edge",
			userAgent:   "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/124.0.0.0 Safari/537.36 Edg/124.0.0.0",
			browserName: "Edge",
			browser:     "124.0.0.0",
			osName:      "Windows",
			osVersion:   "10.0",
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			identity := innerTubeBrowserIdentityFromUserAgent(test.userAgent)
			if identity.browserName != test.browserName || identity.browserVersion != test.browser ||
				identity.osName != test.osName || identity.osVersion != test.osVersion {
				t.Fatalf("unexpected browser identity: %#v", identity)
			}
		})
	}
}

func TestInnerTubeOptionalAuthFallsBackToGuest(t *testing.T) {
	cookies := &innerTubeCookieProviderStub{err: appsessions.ErrNoCookies}
	requests := 0
	client := newInnerTubeClient(cookies, nil)
	client.retryDelays = nil
	client.httpClient = &http.Client{Transport: innerTubeRoundTripFunc(func(request *http.Request) (*http.Response, error) {
		requests++
		if request.Header.Get("Authorization") != "" || request.Header.Get("Cookie") != "" ||
			request.Header.Get("X-Origin") != "" || request.Header.Get("X-Goog-AuthUser") != "" {
			t.Fatalf("guest request leaked authenticated headers: %#v", request.Header)
		}
		return innerTubeTestHTTPResponse(request, http.StatusOK, `{"guest":true}`), nil
	})}

	result, err := client.requestRead(
		context.Background(),
		"search",
		map[string]any{"query": "lofi"},
		innerTubeAuthOptional,
	)
	if err != nil {
		t.Fatalf("guest request: %v", err)
	}
	if result["guest"] != true || requests != 1 {
		t.Fatalf("unexpected guest response: requests=%d result=%#v", requests, result)
	}
}

func TestInnerTubeRequiredAuthRejectsGuestWithoutNetworkRequest(t *testing.T) {
	cookies := &innerTubeCookieProviderStub{err: appsessions.ErrSessionNotFound}
	requests := 0
	client := newInnerTubeClient(cookies, nil)
	client.httpClient = &http.Client{Transport: innerTubeRoundTripFunc(func(request *http.Request) (*http.Response, error) {
		requests++
		return innerTubeTestHTTPResponse(request, http.StatusOK, `{}`), nil
	})}

	_, err := client.requestRead(
		context.Background(),
		"browse",
		map[string]any{"browseId": "FEsubscriptions"},
		innerTubeAuthRequired,
	)
	if !errors.Is(err, errYouTubeInnerTubeNotAuthenticated) ||
		!errors.Is(err, appsessions.ErrSessionNotFound) {
		t.Fatalf("expected wrapped required-auth error, got %v", err)
	}
	if requests != 0 {
		t.Fatalf("required-auth failure must not make a request, got %d", requests)
	}
}

func TestInnerTubeCacheIsIsolatedByGuestAndHashedCookieMaterial(t *testing.T) {
	fixedNow := time.Unix(1_700_000_000, 0)
	cookies := &innerTubeCookieProviderStub{err: appsessions.ErrNoCookies}
	requests := 0
	client := newInnerTubeClient(cookies, nil)
	client.now = func() time.Time { return fixedNow }
	client.retryDelays = nil
	client.httpClient = &http.Client{Transport: innerTubeRoundTripFunc(func(request *http.Request) (*http.Response, error) {
		requests++
		scope := "guest"
		if cookie := request.Header.Get("Cookie"); cookie != "" {
			scope = cookie
		}
		return innerTubeTestHTTPResponse(
			request,
			http.StatusOK,
			fmt.Sprintf(`{"scope":%q}`, scope),
		), nil
	})}

	request := func() string {
		result, err := client.requestRead(
			context.Background(),
			"browse",
			map[string]any{"browseId": "FEwhat_to_watch"},
			innerTubeAuthOptional,
		)
		if err != nil {
			t.Fatalf("request: %v", err)
		}
		value, _ := result["scope"].(string)
		return value
	}

	if got := request(); got != "guest" {
		t.Fatalf("unexpected guest scope: %q", got)
	}
	cookies.set(innerTubeTestCookies("account-a-secret", "sid-a-secret"), nil)
	accountA := request()
	cookies.set(innerTubeTestCookies("account-b-secret", "sid-b-secret"), nil)
	accountB := request()
	cookies.set(innerTubeTestCookies("account-a-secret", "sid-a-secret"), nil)
	accountAFromCache := request()

	if requests != 3 {
		t.Fatalf("guest, account A, and account B must use isolated cache entries; got %d requests", requests)
	}
	if accountA == accountB || accountAFromCache != accountA {
		t.Fatalf("unexpected account cache results: A=%q B=%q cachedA=%q", accountA, accountB, accountAFromCache)
	}
	client.cacheMu.Lock()
	defer client.cacheMu.Unlock()
	for key := range client.cache {
		if strings.Contains(key, "account-a-secret") || strings.Contains(key, "account-b-secret") ||
			strings.Contains(key, "sid-a-secret") || strings.Contains(key, "sid-b-secret") {
			t.Fatalf("cache key contains raw credential material: %q", key)
		}
	}
}

func TestInnerTubeReadCacheIsBounded(t *testing.T) {
	client := newInnerTubeClient(nil, nil)
	client.cacheLimit = 2
	client.retryDelays = nil
	client.httpClient = &http.Client{Transport: innerTubeRoundTripFunc(func(request *http.Request) (*http.Response, error) {
		return innerTubeTestHTTPResponse(request, http.StatusOK, `{"ok":true}`), nil
	})}
	for _, query := range []string{"one", "two", "three"} {
		if _, err := client.requestRead(
			context.Background(),
			"search",
			map[string]any{"query": query},
			innerTubeAuthOptional,
		); err != nil {
			t.Fatalf("search %q: %v", query, err)
		}
	}
	client.cacheMu.Lock()
	defer client.cacheMu.Unlock()
	if len(client.cache) != 2 {
		t.Fatalf("expected bounded two-entry cache, got %d", len(client.cache))
	}
}

func TestInnerTubeReadRetriesTransientStatusOnlyWithinBound(t *testing.T) {
	calls := 0
	client := newInnerTubeClient(nil, nil)
	client.retryDelays = []time.Duration{0, 0}
	client.httpClient = &http.Client{Transport: innerTubeRoundTripFunc(func(request *http.Request) (*http.Response, error) {
		calls++
		if calls < 3 {
			return innerTubeTestHTTPResponse(request, http.StatusServiceUnavailable, `temporary`), nil
		}
		return innerTubeTestHTTPResponse(request, http.StatusOK, `{"ok":true}`), nil
	})}

	result, err := client.requestRead(
		context.Background(),
		"browse",
		map[string]any{"browseId": "FEwhat_to_watch"},
		innerTubeAuthOptional,
	)
	if err != nil {
		t.Fatalf("request after retries: %v", err)
	}
	if calls != 3 || result["ok"] != true {
		t.Fatalf("unexpected retry result: calls=%d result=%#v", calls, result)
	}
}

func TestInnerTubeRequestWrapsTimeoutAndUsesLatestHTTPClient(t *testing.T) {
	provider := &innerTubeHTTPClientProviderStub{}
	client := newInnerTubeClient(nil, provider)
	client.retryDelays = nil
	provider.client = &http.Client{Transport: innerTubeRoundTripFunc(func(*http.Request) (*http.Response, error) {
		return nil, context.DeadlineExceeded
	})}
	_, err := client.requestRead(
		context.Background(),
		"search",
		map[string]any{"query": "first"},
		innerTubeAuthOptional,
	)
	if !errors.Is(err, errYouTubeInnerTubeRequestTimedOut) || !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("expected wrapped timeout, got %v", err)
	}

	provider.client = &http.Client{Transport: innerTubeRoundTripFunc(func(request *http.Request) (*http.Response, error) {
		return innerTubeTestHTTPResponse(request, http.StatusOK, `{"ok":true}`), nil
	})}
	result, err := client.requestRead(
		context.Background(),
		"search",
		map[string]any{"query": "second"},
		innerTubeAuthOptional,
	)
	if err != nil || result["ok"] != true {
		t.Fatalf("latest provider client was not used: result=%#v err=%v", result, err)
	}
}

func innerTubeTestCookies(sapisid string, sid string) []appcookies.Record {
	return []appcookies.Record{
		{
			Name:     "__Secure-3PAPISID",
			Value:    sapisid,
			Domain:   ".youtube.com",
			Path:     "/",
			Expires:  4_102_444_800,
			Secure:   true,
			HttpOnly: true,
		},
		{
			Name:     "SID",
			Value:    sid,
			Domain:   ".youtube.com",
			Path:     "/",
			Expires:  4_102_444_800,
			Secure:   true,
			HttpOnly: true,
		},
	}
}

func innerTubeTestHTTPResponse(
	request *http.Request,
	statusCode int,
	body string,
) *http.Response {
	return &http.Response{
		StatusCode: statusCode,
		Header:     make(http.Header),
		Body:       io.NopCloser(strings.NewReader(body)),
		Request:    request,
	}
}
