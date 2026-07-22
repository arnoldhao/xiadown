package rss

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"regexp"
	"strconv"
	"strings"
	"time"

	appcookies "xiadown/internal/application/cookies"
)

const (
	BilibiliVideoPlatform               = "bilibili"
	BilibiliVideoAdapter                = "video"
	BilibiliBangumiAdapter              = "bangumi"
	bilibiliPlaybackCookieLookupTimeout = time.Second
	bilibiliViewLookupTimeout           = 2 * time.Second
	bilibiliViewResponseLimit           = 64 << 10
	bilibiliViewAPIURL                  = "https://api.bilibili.com/x/web-interface/view"
)

var bilibiliBVIDPattern = regexp.MustCompile(`^BV[0-9A-Za-z]{10}$`)

var errBilibiliPlaybackCookieLookupTimeout = errors.New("bilibili playback cookie lookup timed out")

// VideoPlayerCookieProvider is deliberately narrower than AppSessionsService.
// Playback may consume an authenticated cookie snapshot, but it must never
// open or otherwise control the App Session browser itself.
type VideoPlayerCookieProvider interface {
	RecordsForSiteKey(ctx context.Context, siteKey string) ([]appcookies.Record, error)
}

// VideoPlayerService prepares trusted, short-lived inputs for the native RSS
// video player. Cookies remain process-local and are never part of the Wails
// response returned to the renderer.
type VideoPlayerService struct {
	cookies             VideoPlayerCookieProvider
	httpClients         HTTPClientProvider
	now                 func() time.Time
	cookieLookupTimeout time.Duration
	viewLookupTimeout   time.Duration
}

type videoPlayerCookieLookupResult struct {
	records []appcookies.Record
	err     error
}

type BilibiliPlaybackDescriptor struct {
	Platform        string              `json:"platform"`
	Adapter         string              `json:"adapter"`
	PlatformVideoID string              `json:"platformVideoId"`
	PlayerURL       string              `json:"playerUrl"`
	Authenticated   bool                `json:"authenticated"`
	Cookies         []appcookies.Record `json:"-"`
}

func NewVideoPlayerService(
	cookies VideoPlayerCookieProvider,
	httpClientProviders ...HTTPClientProvider,
) *VideoPlayerService {
	service := &VideoPlayerService{
		cookies:             cookies,
		now:                 time.Now,
		cookieLookupTimeout: bilibiliPlaybackCookieLookupTimeout,
		viewLookupTimeout:   bilibiliViewLookupTimeout,
	}
	if len(httpClientProviders) > 0 {
		service.httpClients = httpClientProviders[0]
	}
	return service
}

// PrepareBilibili accepts an opaque platform video ID, validates it against
// the formats XiaDown's RSS parser emits, and constructs the only URL the
// native player is allowed to navigate to. Missing or invalid credentials are
// an intentional guest fallback; cancellation still propagates to the caller.
func (service *VideoPlayerService) PrepareBilibili(
	ctx context.Context,
	platformVideoID string,
) (BilibiliPlaybackDescriptor, error) {
	if ctx == nil {
		ctx = context.Background()
	}
	if err := ctx.Err(); err != nil {
		return BilibiliPlaybackDescriptor{}, err
	}
	adapter, canonicalID, playerURL, err := buildBilibiliPlaybackTarget(platformVideoID)
	if err != nil {
		return BilibiliPlaybackDescriptor{}, err
	}
	if adapter == BilibiliVideoAdapter {
		bangumiID, lookupErr := service.resolveBilibiliBangumiRedirect(ctx, canonicalID)
		if lookupErr != nil {
			return BilibiliPlaybackDescriptor{}, lookupErr
		}
		if bangumiID != "" {
			adapter = BilibiliBangumiAdapter
			canonicalID = bangumiID
			playerURL = canonicalBilibiliBangumiPage(bangumiID)
		}
	}
	descriptor := BilibiliPlaybackDescriptor{
		Platform:        BilibiliVideoPlatform,
		Adapter:         adapter,
		PlatformVideoID: canonicalID,
		PlayerURL:       playerURL,
	}
	if service == nil || service.cookies == nil {
		return descriptor, nil
	}
	records, err := service.bilibiliPlaybackCookies(ctx)
	if err != nil {
		if errors.Is(err, errBilibiliPlaybackCookieLookupTimeout) {
			return descriptor, nil
		}
		if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
			return BilibiliPlaybackDescriptor{}, err
		}
		return descriptor, nil
	}
	now := time.Now()
	if service.now != nil {
		now = service.now()
	}
	records, authenticated := filterBilibiliPlaybackCookies(records, playerURL, now)
	if !authenticated {
		return descriptor, nil
	}
	descriptor.Authenticated = true
	descriptor.Cookies = records
	return descriptor, nil
}

type bilibiliViewResponse struct {
	Code int `json:"code"`
	Data *struct {
		BVID        string `json:"bvid"`
		AID         uint64 `json:"aid"`
		RedirectURL string `json:"redirect_url"`
	} `json:"data"`
}

// resolveBilibiliBangumiRedirect recognizes Bilibili PGC entries whose feed
// identity is an ordinary BV/av ID. The API is advisory: transport failures,
// malformed responses, and untrusted redirects all preserve the validated
// video descriptor. Only cancellation of the caller's transaction propagates.
func (service *VideoPlayerService) resolveBilibiliBangumiRedirect(
	ctx context.Context,
	canonicalVideoID string,
) (string, error) {
	if service == nil || service.httpClients == nil {
		return "", nil
	}
	baseClient := service.httpClients.HTTPClient()
	if baseClient == nil {
		return "", nil
	}
	if err := ctx.Err(); err != nil {
		return "", err
	}

	endpoint, err := url.Parse(bilibiliViewAPIURL)
	if err != nil {
		return "", nil
	}
	query := endpoint.Query()
	switch {
	case strings.HasPrefix(canonicalVideoID, "BV"):
		query.Set("bvid", canonicalVideoID)
	case strings.HasPrefix(canonicalVideoID, "av"):
		query.Set("aid", canonicalVideoID[2:])
	default:
		return "", nil
	}
	endpoint.RawQuery = query.Encode()

	timeout := service.viewLookupTimeout
	if timeout <= 0 {
		timeout = bilibiliViewLookupTimeout
	}
	lookupCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	request, err := http.NewRequestWithContext(lookupCtx, http.MethodGet, endpoint.String(), nil)
	if err != nil {
		return "", nil
	}
	request.Header.Set("Accept", "application/json")

	// Copy the provider's route-aware client so the shared proxy generation is
	// retained without inheriting a cookie jar or mutating redirect policy used
	// by other services.
	client := *baseClient
	client.Jar = nil
	client.CheckRedirect = func(_ *http.Request, _ []*http.Request) error {
		return http.ErrUseLastResponse
	}
	if client.Timeout <= 0 || client.Timeout > timeout {
		client.Timeout = timeout
	}
	response, err := client.Do(request)
	if err != nil {
		if callerErr := ctx.Err(); callerErr != nil {
			return "", callerErr
		}
		return "", nil
	}
	defer response.Body.Close()
	if callerErr := ctx.Err(); callerErr != nil {
		return "", callerErr
	}
	if response.StatusCode != http.StatusOK {
		return "", nil
	}

	body, err := io.ReadAll(io.LimitReader(response.Body, bilibiliViewResponseLimit+1))
	if err != nil {
		if callerErr := ctx.Err(); callerErr != nil {
			return "", callerErr
		}
		return "", nil
	}
	if callerErr := ctx.Err(); callerErr != nil {
		return "", callerErr
	}
	if len(body) > bilibiliViewResponseLimit {
		return "", nil
	}
	var payload bilibiliViewResponse
	if err := json.Unmarshal(body, &payload); err != nil || payload.Code != 0 || payload.Data == nil {
		return "", nil
	}
	if err := ctx.Err(); err != nil {
		return "", err
	}
	if !bilibiliViewIdentityMatches(canonicalVideoID, payload.Data.BVID, payload.Data.AID) {
		return "", nil
	}
	return strictBilibiliBangumiRedirectID(payload.Data.RedirectURL), nil
}

func bilibiliViewIdentityMatches(canonicalVideoID, returnedBVID string, returnedAID uint64) bool {
	if strings.HasPrefix(canonicalVideoID, "BV") {
		return returnedBVID == canonicalVideoID
	}
	if !strings.HasPrefix(canonicalVideoID, "av") {
		return false
	}
	expectedAID, err := strconv.ParseUint(canonicalVideoID[2:], 10, 64)
	return err == nil && expectedAID > 0 && returnedAID == expectedAID
}

func strictBilibiliBangumiRedirectID(rawRedirectURL string) string {
	parsed, err := url.Parse(rawRedirectURL)
	if err != nil || parsed.Scheme != "https" || parsed.Host != "www.bilibili.com" || parsed.User != nil ||
		parsed.Opaque != "" || parsed.RawQuery != "" || parsed.Fragment != "" || parsed.ForceQuery || parsed.RawPath != "" {
		return ""
	}
	const prefix = "/bangumi/play/"
	if !strings.HasPrefix(parsed.Path, prefix) {
		return ""
	}
	videoID := strings.TrimPrefix(parsed.Path, prefix)
	canonical := canonicalBilibiliBangumiID(videoID)
	if canonical == "" || canonical != videoID {
		return ""
	}
	return canonical
}

// bilibiliPlaybackCookies keeps optional App Session credentials off the
// playback critical path. A local Keychain lookup should normally complete in
// milliseconds; one second leaves ample cold-start headroom while ensuring a
// blocked Security.framework interaction cannot hold the RSS player forever.
//
// The result channel belongs to this invocation. If the provider ignores
// cancellation and returns after the deadline, its result is discarded and
// cannot authenticate or otherwise mutate a later Prepare transaction.
func (service *VideoPlayerService) bilibiliPlaybackCookies(
	ctx context.Context,
) ([]appcookies.Record, error) {
	if ctx == nil {
		ctx = context.Background()
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	timeout := service.cookieLookupTimeout
	if timeout <= 0 {
		timeout = bilibiliPlaybackCookieLookupTimeout
	}
	lookupCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	result := make(chan videoPlayerCookieLookupResult, 1)
	go func() {
		records, err := service.cookies.RecordsForSiteKey(lookupCtx, BilibiliVideoPlatform)
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
		return nil, errBilibiliPlaybackCookieLookupTimeout
	case loaded := <-result:
		// Explicit caller cancellation wins over a concurrently completed
		// credential read and therefore preserves stale Prepare cancellation.
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		// A successful Keychain result racing the private deadline is still late.
		// Never let select scheduling turn that stale credential jar into an
		// authenticated descriptor after guest fallback became authoritative.
		if lookupCtx.Err() != nil {
			return nil, errBilibiliPlaybackCookieLookupTimeout
		}
		return loaded.records, loaded.err
	}
}

func buildBilibiliPlaybackTarget(rawVideoID string) (string, string, string, error) {
	videoID := strings.TrimSpace(rawVideoID)
	if len(videoID) >= 2 && strings.EqualFold(videoID[:2], "BV") {
		videoID = "BV" + videoID[2:]
		if !bilibiliBVIDPattern.MatchString(videoID) {
			return "", "", "", fmt.Errorf("invalid Bilibili BV video ID")
		}
		return BilibiliVideoAdapter, videoID, canonicalBilibiliVideoPage(videoID), nil
	}
	if len(videoID) >= 2 && strings.EqualFold(videoID[:2], "av") {
		numeric := videoID[2:]
		aid, err := strconv.ParseUint(numeric, 10, 64)
		if err != nil || aid == 0 {
			return "", "", "", fmt.Errorf("invalid Bilibili av video ID")
		}
		canonical := "av" + strconv.FormatUint(aid, 10)
		return BilibiliVideoAdapter, canonical, canonicalBilibiliVideoPage(canonical), nil
	}
	if canonical := canonicalBilibiliBangumiID(videoID); canonical != "" {
		return BilibiliBangumiAdapter, canonical, canonicalBilibiliBangumiPage(canonical), nil
	}
	return "", "", "", fmt.Errorf("unsupported Bilibili video ID")
}

// buildBilibiliPlayerURL remains as the URL-only compatibility helper for
// callers that do not need to distinguish the native transport adapter.
func buildBilibiliPlayerURL(rawVideoID string) (string, string, error) {
	_, canonicalID, playerURL, err := buildBilibiliPlaybackTarget(rawVideoID)
	return canonicalID, playerURL, err
}

func filterBilibiliPlaybackCookies(
	records []appcookies.Record,
	playerURL string,
	now time.Time,
) ([]appcookies.Record, bool) {
	records = appcookies.FilterByDomains(records, []string{"bilibili.com"})
	matched := appcookies.MatchURL(records, playerURL)
	if len(matched) == 0 {
		return nil, false
	}
	filtered := make([]appcookies.Record, 0, len(matched))
	hasSession := false
	nowUnix := now.Unix()
	for _, record := range matched {
		name := strings.TrimSpace(record.Name)
		if name == "" || strings.TrimSpace(record.Value) == "" {
			continue
		}
		if record.Expires > 0 && record.Expires <= nowUnix {
			continue
		}
		if strings.EqualFold(name, "SESSDATA") {
			hasSession = true
		}
		filtered = append(filtered, record)
	}
	if !hasSession {
		return nil, false
	}
	return filtered, true
}
