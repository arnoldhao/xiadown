package rss

import (
	"context"
	"errors"
	"io"
	"net/http"
	"net/http/cookiejar"
	"net/url"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	appcookies "xiadown/internal/application/cookies"
)

type videoPlayerCookieProviderStub struct {
	records []appcookies.Record
	err     error
}

type videoPlayerHTTPClientProviderStub struct {
	client *http.Client
}

func (provider videoPlayerHTTPClientProviderStub) HTTPClient() *http.Client {
	return provider.client
}

type videoPlayerRoundTripFunc func(*http.Request) (*http.Response, error)

func (roundTrip videoPlayerRoundTripFunc) RoundTrip(request *http.Request) (*http.Response, error) {
	return roundTrip(request)
}

func videoPlayerHTTPResponse(status int, body string) *http.Response {
	return &http.Response{
		StatusCode: status,
		Header:     make(http.Header),
		Body:       io.NopCloser(strings.NewReader(body)),
	}
}

type blockingVideoPlayerCookieProvider struct {
	started     chan struct{}
	release     chan struct{}
	releaseOnce sync.Once
	records     []appcookies.Record
}

func newBlockingVideoPlayerCookieProvider(records []appcookies.Record) *blockingVideoPlayerCookieProvider {
	return &blockingVideoPlayerCookieProvider{
		started: make(chan struct{}, 4),
		release: make(chan struct{}),
		records: append([]appcookies.Record(nil), records...),
	}
}

func (provider *blockingVideoPlayerCookieProvider) RecordsForSiteKey(
	_ context.Context,
	siteKey string,
) ([]appcookies.Record, error) {
	if siteKey != BilibiliVideoPlatform {
		return nil, errors.New("unexpected site key")
	}
	provider.started <- struct{}{}
	<-provider.release
	return append([]appcookies.Record(nil), provider.records...), nil
}

func (provider *blockingVideoPlayerCookieProvider) unblock() {
	provider.releaseOnce.Do(func() { close(provider.release) })
}

func (stub videoPlayerCookieProviderStub) RecordsForSiteKey(
	_ context.Context,
	siteKey string,
) ([]appcookies.Record, error) {
	if siteKey != BilibiliVideoPlatform {
		return nil, errors.New("unexpected site key")
	}
	return append([]appcookies.Record(nil), stub.records...), stub.err
}

func TestVideoPlayerServicePreparesCanonicalBilibiliVideoPage(t *testing.T) {
	now := time.Date(2026, 7, 14, 9, 30, 0, 0, time.UTC)
	service := NewVideoPlayerService(videoPlayerCookieProviderStub{records: []appcookies.Record{
		{Name: "SESSDATA", Value: "session", Domain: ".bilibili.com", Path: "/", Expires: now.Add(time.Hour).Unix(), HttpOnly: true, Secure: true},
		{Name: "bili_jct", Value: "csrf", Domain: ".bilibili.com", Path: "/", Expires: now.Add(time.Hour).Unix(), Secure: true},
		{Name: "expired", Value: "old", Domain: ".bilibili.com", Path: "/", Expires: now.Unix()},
		{Name: "foreign", Value: "secret", Domain: ".attacker.example", Path: "/", Expires: now.Add(time.Hour).Unix()},
	}})
	service.now = func() time.Time { return now }

	descriptor, err := service.PrepareBilibili(context.Background(), "BV1xx411c7mD")
	if err != nil {
		t.Fatalf("PrepareBilibili returned error: %v", err)
	}
	if descriptor.Platform != "bilibili" || descriptor.Adapter != BilibiliVideoAdapter || descriptor.PlatformVideoID != "BV1xx411c7mD" || !descriptor.Authenticated {
		t.Fatalf("unexpected descriptor: %#v", descriptor)
	}
	parsed, err := url.Parse(descriptor.PlayerURL)
	if err != nil {
		t.Fatalf("parse player URL: %v", err)
	}
	if parsed.Scheme != "https" || parsed.Host != "www.bilibili.com" || parsed.Path != "/video/BV1xx411c7mD/" {
		t.Fatalf("untrusted player URL: %q", descriptor.PlayerURL)
	}
	if parsed.RawQuery != "" || descriptor.PlayerURL != "https://www.bilibili.com/video/BV1xx411c7mD/" {
		t.Fatalf("player URL is not canonical: %q", descriptor.PlayerURL)
	}
	if len(descriptor.Cookies) != 2 {
		t.Fatalf("injected cookies = %#v, want only live Bilibili cookies", descriptor.Cookies)
	}
}

func TestVideoPlayerServiceCanonicalizesAVID(t *testing.T) {
	service := NewVideoPlayerService(nil)
	descriptor, err := service.PrepareBilibili(context.Background(), "AV000170001")
	if err != nil {
		t.Fatalf("PrepareBilibili returned error: %v", err)
	}
	if descriptor.PlatformVideoID != "av170001" {
		t.Fatalf("canonical ID = %q, want av170001", descriptor.PlatformVideoID)
	}
	if descriptor.Adapter != BilibiliVideoAdapter {
		t.Fatalf("adapter = %q, want %q", descriptor.Adapter, BilibiliVideoAdapter)
	}
	parsed, _ := url.Parse(descriptor.PlayerURL)
	if parsed.Host != "www.bilibili.com" || parsed.Path != "/video/av170001/" || parsed.RawQuery != "" {
		t.Fatalf("unexpected av player URL: %q", descriptor.PlayerURL)
	}
}

func TestVideoPlayerServicePreparesCanonicalBilibiliBangumiPages(t *testing.T) {
	t.Parallel()

	service := NewVideoPlayerService(nil)
	for _, test := range []struct {
		name    string
		input   string
		wantID  string
		wantURL string
	}{
		{
			name:    "episode",
			input:   "EP003854807",
			wantID:  "ep3854807",
			wantURL: "https://www.bilibili.com/bangumi/play/ep3854807",
		},
		{
			name:    "season",
			input:   "ss00028747",
			wantID:  "ss28747",
			wantURL: "https://www.bilibili.com/bangumi/play/ss28747",
		},
	} {
		test := test
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			descriptor, err := service.PrepareBilibili(context.Background(), test.input)
			if err != nil {
				t.Fatalf("PrepareBilibili returned error: %v", err)
			}
			if descriptor.Platform != BilibiliVideoPlatform ||
				descriptor.Adapter != BilibiliBangumiAdapter ||
				descriptor.PlatformVideoID != test.wantID ||
				descriptor.PlayerURL != test.wantURL {
				t.Fatalf("unexpected Bangumi descriptor: %#v", descriptor)
			}
		})
	}
}

func TestVideoPlayerServiceResolvesBVIDRedirectToBangumiAdapter(t *testing.T) {
	jar, err := cookiejar.New(nil)
	if err != nil {
		t.Fatalf("create cookie jar: %v", err)
	}
	apiURL, err := url.Parse(bilibiliViewAPIURL)
	if err != nil {
		t.Fatalf("parse API URL: %v", err)
	}
	jar.SetCookies(apiURL, []*http.Cookie{{Name: "SESSDATA", Value: "must-not-leak"}})

	var seen bool
	client := &http.Client{
		Jar: jar,
		Transport: videoPlayerRoundTripFunc(func(request *http.Request) (*http.Response, error) {
			seen = true
			if request.Method != http.MethodGet || request.URL.Scheme != "https" ||
				request.URL.Host != "api.bilibili.com" || request.URL.Path != "/x/web-interface/view" {
				t.Fatalf("unexpected view request: %s %s", request.Method, request.URL.String())
			}
			if request.URL.Query().Get("bvid") != "BV1HVNZ67Emb" || len(request.URL.Query()) != 1 {
				t.Fatalf("unexpected view query: %q", request.URL.RawQuery)
			}
			if cookie := request.Header.Get("Cookie"); cookie != "" {
				t.Fatalf("view lookup leaked cookie header: %q", cookie)
			}
			return videoPlayerHTTPResponse(http.StatusOK, `{"code":0,"data":{"aid":116935870121583,"bvid":"BV1HVNZ67Emb","redirect_url":"https://www.bilibili.com/bangumi/play/ep3854807"}}`), nil
		}),
	}
	service := NewVideoPlayerService(nil, videoPlayerHTTPClientProviderStub{client: client})
	descriptor, err := service.PrepareBilibili(context.Background(), "BV1HVNZ67Emb")
	if err != nil {
		t.Fatalf("PrepareBilibili returned error: %v", err)
	}
	if !seen {
		t.Fatal("PrepareBilibili did not inspect the fixed Bilibili view endpoint")
	}
	if descriptor.Platform != BilibiliVideoPlatform || descriptor.Adapter != BilibiliBangumiAdapter ||
		descriptor.PlatformVideoID != "ep3854807" ||
		descriptor.PlayerURL != "https://www.bilibili.com/bangumi/play/ep3854807" {
		t.Fatalf("unexpected resolved Bangumi descriptor: %#v", descriptor)
	}
}

func TestVideoPlayerServiceResolvesMatchingAVIDRedirectToBangumiAdapter(t *testing.T) {
	client := &http.Client{Transport: videoPlayerRoundTripFunc(func(request *http.Request) (*http.Response, error) {
		if request.URL.Query().Get("aid") != "170001" || len(request.URL.Query()) != 1 {
			t.Fatalf("unexpected av view query: %q", request.URL.RawQuery)
		}
		return videoPlayerHTTPResponse(http.StatusOK, `{"code":0,"data":{"aid":170001,"bvid":"BV1xx411c7mD","redirect_url":"https://www.bilibili.com/bangumi/play/ss28747"}}`), nil
	})}
	service := NewVideoPlayerService(nil, videoPlayerHTTPClientProviderStub{client: client})
	descriptor, err := service.PrepareBilibili(context.Background(), "av000170001")
	if err != nil {
		t.Fatalf("PrepareBilibili returned error: %v", err)
	}
	if descriptor.Adapter != BilibiliBangumiAdapter || descriptor.PlatformVideoID != "ss28747" ||
		descriptor.PlayerURL != "https://www.bilibili.com/bangumi/play/ss28747" {
		t.Fatalf("unexpected av-resolved Bangumi descriptor: %#v", descriptor)
	}
}

func TestVideoPlayerServiceKeepsOrdinaryBVVideoAdapter(t *testing.T) {
	client := &http.Client{Transport: videoPlayerRoundTripFunc(func(*http.Request) (*http.Response, error) {
		return videoPlayerHTTPResponse(http.StatusOK, `{"code":0,"data":{"aid":170001,"bvid":"BV1xx411c7mD","redirect_url":""}}`), nil
	})}
	service := NewVideoPlayerService(nil, videoPlayerHTTPClientProviderStub{client: client})
	descriptor, err := service.PrepareBilibili(context.Background(), "BV1xx411c7mD")
	if err != nil {
		t.Fatalf("PrepareBilibili returned error: %v", err)
	}
	assertBilibiliVideoDescriptor(t, descriptor, "BV1xx411c7mD")
}

func TestVideoPlayerServiceViewFailuresSafelyKeepVideoAdapter(t *testing.T) {
	tests := []struct {
		name      string
		roundTrip videoPlayerRoundTripFunc
		timeout   time.Duration
	}{
		{
			name:    "timeout",
			timeout: 15 * time.Millisecond,
			roundTrip: func(request *http.Request) (*http.Response, error) {
				<-request.Context().Done()
				return nil, request.Context().Err()
			},
		},
		{
			name: "non-200",
			roundTrip: func(*http.Request) (*http.Response, error) {
				return videoPlayerHTTPResponse(http.StatusBadGateway, `{}`), nil
			},
		},
		{
			name: "bad-json",
			roundTrip: func(*http.Request) (*http.Response, error) {
				return videoPlayerHTTPResponse(http.StatusOK, `{"code":`), nil
			},
		},
		{
			name: "missing-redirect",
			roundTrip: func(*http.Request) (*http.Response, error) {
				return videoPlayerHTTPResponse(http.StatusOK, `{"code":0,"data":{"bvid":"BV1xx411c7mD"}}`), nil
			},
		},
		{
			name: "mismatched-identity",
			roundTrip: func(*http.Request) (*http.Response, error) {
				return videoPlayerHTTPResponse(http.StatusOK, `{"code":0,"data":{"bvid":"BV1HVNZ67Emb","redirect_url":"https://www.bilibili.com/bangumi/play/ep3854807"}}`), nil
			},
		},
		{
			name: "oversized-response",
			roundTrip: func(*http.Request) (*http.Response, error) {
				return videoPlayerHTTPResponse(http.StatusOK, strings.Repeat(" ", bilibiliViewResponseLimit+1)), nil
			},
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			service := NewVideoPlayerService(nil, videoPlayerHTTPClientProviderStub{
				client: &http.Client{Transport: test.roundTrip},
			})
			if test.timeout > 0 {
				service.viewLookupTimeout = test.timeout
			}
			descriptor, err := service.PrepareBilibili(context.Background(), "BV1xx411c7mD")
			if err != nil {
				t.Fatalf("PrepareBilibili returned error: %v", err)
			}
			assertBilibiliVideoDescriptor(t, descriptor, "BV1xx411c7mD")
		})
	}
}

func TestVideoPlayerServiceRejectsUntrustedBangumiRedirects(t *testing.T) {
	for _, redirectURL := range []string{
		"http://www.bilibili.com/bangumi/play/ep3854807",
		"https://evil.example/bangumi/play/ep3854807",
		"https://www.bilibili.com.evil.example/bangumi/play/ep3854807",
		"https://www.bilibili.com:443/bangumi/play/ep3854807",
		"https://www.bilibili.com@evil.example/bangumi/play/ep3854807",
		"https://www.bilibili.com/bangumi/play/ep3854807?from=feed",
		"https://www.bilibili.com/bangumi/play/ep3854807#fragment",
		"https://www.bilibili.com/bangumi/play/ep3854807/extra",
		"https://www.bilibili.com/bangumi/play/EP3854807",
		"https://www.bilibili.com/bangumi/play/ep003854807",
		"https://www.bilibili.com/bangumi/play/ep3854807%2Fextra",
	} {
		redirectURL := redirectURL
		t.Run(redirectURL, func(t *testing.T) {
			body := `{"code":0,"data":{"bvid":"BV1HVNZ67Emb","redirect_url":"` + redirectURL + `"}}`
			service := NewVideoPlayerService(nil, videoPlayerHTTPClientProviderStub{client: &http.Client{
				Transport: videoPlayerRoundTripFunc(func(*http.Request) (*http.Response, error) {
					return videoPlayerHTTPResponse(http.StatusOK, body), nil
				}),
			}})
			descriptor, err := service.PrepareBilibili(context.Background(), "BV1HVNZ67Emb")
			if err != nil {
				t.Fatalf("PrepareBilibili returned error: %v", err)
			}
			assertBilibiliVideoDescriptor(t, descriptor, "BV1HVNZ67Emb")
		})
	}
}

func TestVideoPlayerServiceViewLookupRejectsHTTPRedirects(t *testing.T) {
	var calls atomic.Int32
	client := &http.Client{Transport: videoPlayerRoundTripFunc(func(request *http.Request) (*http.Response, error) {
		calls.Add(1)
		response := videoPlayerHTTPResponse(http.StatusFound, "redirected")
		response.Header.Set("Location", "https://attacker.example/collect")
		return response, nil
	})}
	service := NewVideoPlayerService(nil, videoPlayerHTTPClientProviderStub{client: client})
	descriptor, err := service.PrepareBilibili(context.Background(), "BV1HVNZ67Emb")
	if err != nil {
		t.Fatalf("PrepareBilibili returned error: %v", err)
	}
	if calls.Load() != 1 {
		t.Fatalf("view lookup transport calls = %d, want one request with redirects rejected", calls.Load())
	}
	assertBilibiliVideoDescriptor(t, descriptor, "BV1HVNZ67Emb")
}

func TestVideoPlayerServiceViewLookupPropagatesCallerCancellation(t *testing.T) {
	started := make(chan struct{})
	client := &http.Client{Transport: videoPlayerRoundTripFunc(func(request *http.Request) (*http.Response, error) {
		close(started)
		<-request.Context().Done()
		return nil, request.Context().Err()
	})}
	service := NewVideoPlayerService(nil, videoPlayerHTTPClientProviderStub{client: client})
	service.viewLookupTimeout = time.Second
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() {
		_, err := service.PrepareBilibili(ctx, "BV1HVNZ67Emb")
		done <- err
	}()
	select {
	case <-started:
	case <-time.After(500 * time.Millisecond):
		t.Fatal("PrepareBilibili did not start view lookup")
	}
	cancel()
	select {
	case err := <-done:
		if !errors.Is(err, context.Canceled) {
			t.Fatalf("canceled PrepareBilibili error = %v, want context.Canceled", err)
		}
	case <-time.After(500 * time.Millisecond):
		t.Fatal("caller cancellation did not release Bilibili view lookup")
	}
}

func assertBilibiliVideoDescriptor(t *testing.T, descriptor BilibiliPlaybackDescriptor, videoID string) {
	t.Helper()
	if descriptor.Platform != BilibiliVideoPlatform || descriptor.Adapter != BilibiliVideoAdapter ||
		descriptor.PlatformVideoID != videoID || descriptor.PlayerURL != canonicalBilibiliVideoPage(videoID) {
		t.Fatalf("unexpected video descriptor: %#v", descriptor)
	}
}

func TestVideoPlayerServiceRejectsNonIDInput(t *testing.T) {
	service := NewVideoPlayerService(nil)
	for _, value := range []string{
		"", "https://www.bilibili.com/video/BV1xx411c7mD", "BV1xx411c7mD?autoplay=0", "av0", "ep0", "ss0", "ep3854807?autoplay=0", "javascript:alert(1)",
	} {
		if _, err := service.PrepareBilibili(context.Background(), value); err == nil {
			t.Errorf("PrepareBilibili(%q) unexpectedly succeeded", value)
		}
	}
}

func TestVideoPlayerServiceFallsBackToGuestWithoutLiveSession(t *testing.T) {
	now := time.Date(2026, 7, 14, 9, 30, 0, 0, time.UTC)
	service := NewVideoPlayerService(videoPlayerCookieProviderStub{records: []appcookies.Record{
		{Name: "SESSDATA", Value: "expired", Domain: ".bilibili.com", Path: "/", Expires: now.Unix()},
		{Name: "buvid3", Value: "device", Domain: ".bilibili.com", Path: "/", Expires: now.Add(time.Hour).Unix()},
	}})
	service.now = func() time.Time { return now }
	descriptor, err := service.PrepareBilibili(context.Background(), "BV1xx411c7mD")
	if err != nil {
		t.Fatalf("PrepareBilibili returned error: %v", err)
	}
	if descriptor.Authenticated || len(descriptor.Cookies) != 0 {
		t.Fatalf("expired App Session must fall back to guest: %#v", descriptor)
	}
}

func TestVideoPlayerServicePropagatesCancellationButNotCredentialAbsence(t *testing.T) {
	credentialErr := errors.New("cookie store unavailable")
	guestService := NewVideoPlayerService(videoPlayerCookieProviderStub{err: credentialErr})
	guest, err := guestService.PrepareBilibili(context.Background(), "BV1xx411c7mD")
	if err != nil || guest.Authenticated {
		t.Fatalf("credential absence should be guest fallback, descriptor=%#v err=%v", guest, err)
	}

	cancelledService := NewVideoPlayerService(videoPlayerCookieProviderStub{err: context.Canceled})
	if _, err := cancelledService.PrepareBilibili(context.Background(), "BV1xx411c7mD"); !errors.Is(err, context.Canceled) {
		t.Fatalf("cancellation error = %v, want context.Canceled", err)
	}
}

func TestVideoPlayerServiceCookieTimeoutFallsBackToGuest(t *testing.T) {
	now := time.Date(2026, 7, 14, 9, 30, 0, 0, time.UTC)
	provider := newBlockingVideoPlayerCookieProvider([]appcookies.Record{{
		Name: "SESSDATA", Value: "late-session", Domain: ".bilibili.com", Path: "/", Expires: now.Add(time.Hour).Unix(),
	}})
	t.Cleanup(provider.unblock)
	service := NewVideoPlayerService(provider)
	service.now = func() time.Time { return now }
	service.cookieLookupTimeout = 20 * time.Millisecond

	type prepareResult struct {
		descriptor BilibiliPlaybackDescriptor
		err        error
	}
	done := make(chan prepareResult, 1)
	go func() {
		descriptor, err := service.PrepareBilibili(context.Background(), "BV1xx411c7mD")
		done <- prepareResult{descriptor: descriptor, err: err}
	}()
	select {
	case <-provider.started:
	case <-time.After(500 * time.Millisecond):
		t.Fatal("PrepareBilibili did not start credential lookup")
	}

	select {
	case result := <-done:
		if result.err != nil {
			t.Fatalf("timed-out credential lookup returned error: %v", result.err)
		}
		if result.descriptor.Authenticated || len(result.descriptor.Cookies) != 0 {
			t.Fatalf("late credentials escaped into guest playback: %#v", result.descriptor)
		}
	case <-time.After(500 * time.Millisecond):
		t.Fatal("PrepareBilibili remained blocked after its credential timeout")
	}

	// Release the provider only after Prepare has committed to guest. The
	// invocation-private result channel must discard this late authenticated jar.
	provider.unblock()
}

func TestVideoPlayerServiceBlockedCookieLookupPreservesCallerCancellation(t *testing.T) {
	provider := newBlockingVideoPlayerCookieProvider(nil)
	t.Cleanup(provider.unblock)
	service := NewVideoPlayerService(provider)
	service.cookieLookupTimeout = time.Second
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() {
		_, err := service.PrepareBilibili(ctx, "BV1xx411c7mD")
		done <- err
	}()
	select {
	case <-provider.started:
	case <-time.After(500 * time.Millisecond):
		t.Fatal("PrepareBilibili did not start credential lookup")
	}
	cancel()

	select {
	case err := <-done:
		if !errors.Is(err, context.Canceled) {
			t.Fatalf("canceled PrepareBilibili error = %v, want context.Canceled", err)
		}
	case <-time.After(500 * time.Millisecond):
		t.Fatal("caller cancellation did not release blocked PrepareBilibili")
	}
	provider.unblock()
}

func TestVideoPlayerServiceLateCookieResultIsIsolatedFromNewerPrepare(t *testing.T) {
	now := time.Date(2026, 7, 14, 9, 30, 0, 0, time.UTC)
	provider := newBlockingVideoPlayerCookieProvider([]appcookies.Record{{
		Name: "SESSDATA", Value: "session", Domain: ".bilibili.com", Path: "/", Expires: now.Add(time.Hour).Unix(),
	}})
	t.Cleanup(provider.unblock)
	service := NewVideoPlayerService(provider)
	service.now = func() time.Time { return now }
	service.cookieLookupTimeout = 20 * time.Millisecond

	type prepareResult struct {
		descriptor BilibiliPlaybackDescriptor
		err        error
	}
	firstDone := make(chan prepareResult, 1)
	go func() {
		descriptor, err := service.PrepareBilibili(context.Background(), "BV1xx411c7mD")
		firstDone <- prepareResult{descriptor: descriptor, err: err}
	}()
	select {
	case <-provider.started:
	case <-time.After(500 * time.Millisecond):
		t.Fatal("first PrepareBilibili did not start credential lookup")
	}
	select {
	case first := <-firstDone:
		if first.err != nil || first.descriptor.Authenticated {
			t.Fatalf("first Prepare must commit to guest on timeout: descriptor=%#v err=%v", first.descriptor, first.err)
		}
	case <-time.After(500 * time.Millisecond):
		t.Fatal("first PrepareBilibili did not return after credential timeout")
	}

	service.cookieLookupTimeout = time.Second
	secondDone := make(chan prepareResult, 1)
	go func() {
		descriptor, err := service.PrepareBilibili(context.Background(), "BV1xx411c7mD")
		secondDone <- prepareResult{descriptor: descriptor, err: err}
	}()
	select {
	case <-provider.started:
	case <-time.After(500 * time.Millisecond):
		t.Fatal("newer PrepareBilibili did not start its own credential lookup")
	}

	// Both provider calls now finish. The first authenticated result is late and
	// has only its expired invocation channel; the second owns a distinct result.
	provider.unblock()
	select {
	case second := <-secondDone:
		if second.err != nil || !second.descriptor.Authenticated || len(second.descriptor.Cookies) != 1 {
			t.Fatalf("newer Prepare did not receive its own credentials: descriptor=%#v err=%v", second.descriptor, second.err)
		}
	case <-time.After(500 * time.Millisecond):
		t.Fatal("newer PrepareBilibili did not receive credential result")
	}
}
