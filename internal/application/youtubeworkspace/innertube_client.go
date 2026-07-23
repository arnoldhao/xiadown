package youtubeworkspace

import (
	"bytes"
	"context"
	"crypto/sha1"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"sort"
	"strings"
	"sync"
	"time"

	appcookies "xiadown/internal/application/cookies"
	"xiadown/internal/application/youtubecookies"
	"xiadown/internal/domain/appsessions"
)

const (
	youtubeInnerTubeBaseURL       = "https://www.youtube.com/youtubei/v1"
	youtubeInnerTubeOrigin        = "https://www.youtube.com"
	youtubeInnerTubeClientName    = "WEB"
	youtubeInnerTubeClientVersion = "2.20260611.01.00"
	youtubeInnerTubeDefaultUA     = "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/605.1.15 (KHTML, like Gecko) Version/17.0 Safari/605.1.15"

	youtubeInnerTubeReadCacheTTL = 2 * time.Minute
	youtubeInnerTubeCacheLimit   = 128
	youtubeInnerTubeResponseMax  = 16 << 20
	youtubeInnerTubeHTTPTimeout  = 20 * time.Second
)

var (
	errYouTubeInnerTubeNotAuthenticated   = errors.New("youtube is not authenticated")
	errYouTubeInnerTubeAuthExpired        = errors.New("youtube authentication expired")
	errYouTubeInnerTubeRequestTimedOut    = errors.New("youtube request timed out")
	errYouTubeInnerTubeNetworkUnavailable = errors.New("youtube network unavailable")
)

type innerTubeAuthPolicy uint8

const (
	innerTubeAuthOptional innerTubeAuthPolicy = iota
	innerTubeAuthRequired
)

type innerTubeCookieProvider interface {
	RecordsForSiteKey(context.Context, string) ([]appcookies.Record, error)
}

type innerTubeHTTPClientProvider interface {
	HTTPClient() *http.Client
}

type innerTubeRequestOptions struct {
	authPolicy     innerTubeAuthPolicy
	cacheTTL       time.Duration
	bypassCache    bool
	retryTransient bool
}

type innerTubeHTTPStatusError struct {
	StatusCode int
	Detail     string
}

func (err *innerTubeHTTPStatusError) Error() string {
	if err == nil {
		return "youtube api request failed"
	}
	if strings.TrimSpace(err.Detail) == "" {
		return fmt.Sprintf("youtube api status %d", err.StatusCode)
	}
	return fmt.Sprintf("youtube api status %d: %s", err.StatusCode, strings.TrimSpace(err.Detail))
}

type innerTubeAuthMaterial struct {
	headers       map[string]string
	authenticated bool
	cacheScope    string
}

type innerTubeCacheEntry struct {
	data         []byte
	expiresAt    time.Time
	lastAccessed time.Time
}

// innerTubeClient is the regular YouTube WEB InnerTube request core. It owns
// request identity, authentication, retry, and account-isolated response
// caching; route-specific endpoint selection and response parsing stay in
// separate workspace modules.
type innerTubeClient struct {
	cookies            innerTubeCookieProvider
	httpClient         *http.Client
	httpClientProvider innerTubeHTTPClientProvider
	baseURL            string
	userAgent          string
	now                func() time.Time
	retryDelays        []time.Duration
	cacheTTL           time.Duration
	cacheLimit         int

	cacheMu sync.Mutex
	cache   map[string]innerTubeCacheEntry
}

func newInnerTubeClient(
	cookies innerTubeCookieProvider,
	httpClientProvider innerTubeHTTPClientProvider,
) *innerTubeClient {
	return &innerTubeClient{
		cookies:            cookies,
		httpClient:         &http.Client{Timeout: youtubeInnerTubeHTTPTimeout},
		httpClientProvider: httpClientProvider,
		baseURL:            youtubeInnerTubeBaseURL,
		userAgent:          youtubeInnerTubeDefaultUA,
		now:                time.Now,
		retryDelays:        []time.Duration{150 * time.Millisecond, 450 * time.Millisecond},
		cacheTTL:           youtubeInnerTubeReadCacheTTL,
		cacheLimit:         youtubeInnerTubeCacheLimit,
		cache:              make(map[string]innerTubeCacheEntry),
	}
}

func (client *innerTubeClient) setUserAgent(userAgent string) {
	if client == nil {
		return
	}
	value := strings.TrimSpace(userAgent)
	if value == "" {
		value = youtubeInnerTubeDefaultUA
	}
	if value == client.userAgent {
		return
	}
	client.userAgent = value
	client.clearCache()
}

func (client *innerTubeClient) requestRead(
	ctx context.Context,
	endpoint string,
	body map[string]any,
	authPolicy innerTubeAuthPolicy,
) (map[string]any, error) {
	cacheTTL := youtubeInnerTubeReadCacheTTL
	if client != nil && client.cacheTTL > 0 {
		cacheTTL = client.cacheTTL
	}
	return client.request(ctx, endpoint, body, innerTubeRequestOptions{
		authPolicy:     authPolicy,
		cacheTTL:       cacheTTL,
		retryTransient: true,
	})
}

func (client *innerTubeClient) requestReadFresh(
	ctx context.Context,
	endpoint string,
	body map[string]any,
	authPolicy innerTubeAuthPolicy,
) (map[string]any, error) {
	return client.request(ctx, endpoint, body, innerTubeRequestOptions{
		authPolicy:     authPolicy,
		cacheTTL:       youtubeInnerTubeReadCacheTTL,
		bypassCache:    true,
		retryTransient: true,
	})
}

func (client *innerTubeClient) requestMutation(
	ctx context.Context,
	endpoint string,
	body map[string]any,
) (map[string]any, error) {
	result, err := client.request(ctx, endpoint, body, innerTubeRequestOptions{
		authPolicy: innerTubeAuthRequired,
	})
	if err == nil {
		client.clearCache()
	}
	return result, err
}

func (client *innerTubeClient) request(
	ctx context.Context,
	endpoint string,
	body map[string]any,
	options innerTubeRequestOptions,
) (map[string]any, error) {
	if client == nil {
		return nil, errors.New("youtube innertube client is nil")
	}
	if ctx == nil {
		ctx = context.Background()
	}
	cleanEndpoint := strings.Trim(strings.TrimSpace(endpoint), "/")
	if cleanEndpoint == "" {
		return nil, errors.New("youtube innertube endpoint is required")
	}

	auth, err := client.authMaterial(ctx, options.authPolicy)
	if err != nil {
		return nil, err
	}
	locale := innerTubeLocaleFromContext(ctx)
	requestBody := make(map[string]any, len(body)+1)
	for key, value := range body {
		requestBody[key] = value
	}
	requestBody["context"] = client.requestContext(locale)
	payload, err := json.Marshal(requestBody)
	if err != nil {
		return nil, err
	}

	cacheKey := ""
	if options.cacheTTL > 0 {
		cacheKey = innerTubeCacheKey(cleanEndpoint, locale, auth.cacheScope, payload)
		if !options.bypassCache {
			if cached, ok := client.cachedResponse(cacheKey); ok {
				return decodeInnerTubeResponse(cached)
			}
		}
	}

	requestURL, err := url.Parse(strings.TrimRight(client.baseURL, "/") + "/" + cleanEndpoint)
	if err != nil {
		return nil, err
	}
	query := requestURL.Query()
	query.Set("prettyPrint", "false")
	requestURL.RawQuery = query.Encode()

	var responseData []byte
	for attempt := 0; ; attempt++ {
		responseData, err = client.doRequest(
			ctx,
			requestURL.String(),
			payload,
			auth.headers,
			auth.authenticated,
			locale,
		)
		if err == nil || !client.shouldRetry(err, options, attempt) {
			break
		}
		if waitErr := client.waitBeforeRetry(ctx, attempt); waitErr != nil {
			return nil, waitErr
		}
	}
	if err != nil {
		return nil, err
	}

	decoded, err := decodeInnerTubeResponse(responseData)
	if err != nil {
		return nil, err
	}
	if cacheKey != "" {
		client.cacheResponse(cacheKey, responseData, options.cacheTTL)
	}
	return decoded, nil
}

func (client *innerTubeClient) authMaterial(
	ctx context.Context,
	policy innerTubeAuthPolicy,
) (innerTubeAuthMaterial, error) {
	guest := client.guestAuthMaterial()
	if client.cookies == nil {
		if policy == innerTubeAuthRequired {
			return innerTubeAuthMaterial{}, errYouTubeInnerTubeNotAuthenticated
		}
		return guest, nil
	}

	records, err := client.cookies.RecordsForSiteKey(ctx, "youtube")
	if err != nil {
		if isMissingInnerTubeSessionError(err) {
			if policy == innerTubeAuthRequired {
				return innerTubeAuthMaterial{}, fmt.Errorf("%w: %w", errYouTubeInnerTubeNotAuthenticated, err)
			}
			return guest, nil
		}
		return innerTubeAuthMaterial{}, err
	}

	now := client.currentTime()
	matched := youtubecookies.Runtime(
		activeInnerTubeCookies(appcookies.MatchURL(records, youtubeInnerTubeOrigin+"/"), now),
		now,
	)
	if len(matched) == 0 {
		if policy == innerTubeAuthRequired {
			return innerTubeAuthMaterial{}, errYouTubeInnerTubeNotAuthenticated
		}
		return guest, nil
	}

	sapisid := findInnerTubeSAPISID(matched)
	if sapisid == "" {
		if policy == innerTubeAuthRequired {
			return innerTubeAuthMaterial{}, errYouTubeInnerTubeAuthExpired
		}
		return guest, nil
	}
	cookieHeader := buildInnerTubeCookieHeader(matched)
	if cookieHeader == "" {
		if policy == innerTubeAuthRequired {
			return innerTubeAuthMaterial{}, errYouTubeInnerTubeNotAuthenticated
		}
		return guest, nil
	}

	timestamp := now.Unix()
	headers := client.baseHeaders()
	headers["Authorization"] = "SAPISIDHASH " + innerTubeSAPISIDHash(
		sapisid,
		youtubeInnerTubeOrigin,
		timestamp,
	)
	headers["Cookie"] = cookieHeader
	headers["X-Goog-AuthUser"] = "0"
	headers["X-Origin"] = youtubeInnerTubeOrigin

	return innerTubeAuthMaterial{
		headers:       headers,
		authenticated: true,
		cacheScope:    "auth:" + innerTubeCookieMaterialHash(cookieHeader),
	}, nil
}

func (client *innerTubeClient) guestAuthMaterial() innerTubeAuthMaterial {
	return innerTubeAuthMaterial{
		headers:       client.baseHeaders(),
		authenticated: false,
		cacheScope:    "guest",
	}
}

func (client *innerTubeClient) baseHeaders() map[string]string {
	return map[string]string{
		"Content-Type": "application/json",
		"Origin":       youtubeInnerTubeOrigin,
		"Referer":      youtubeInnerTubeOrigin,
		"User-Agent":   client.browserUserAgent(),
	}
}

func (client *innerTubeClient) requestContext(locale string) map[string]any {
	_, utcOffsetSeconds := client.currentTime().Zone()
	userAgent := client.browserUserAgent()
	identity := innerTubeBrowserIdentityFromUserAgent(userAgent)
	return map[string]any{
		"client": map[string]any{
			"clientName":       youtubeInnerTubeClientName,
			"clientVersion":    youtubeInnerTubeClientVersion,
			"hl":               normalizeInnerTubeLocale(locale),
			"gl":               "US",
			"browserName":      identity.browserName,
			"browserVersion":   identity.browserVersion,
			"osName":           identity.osName,
			"osVersion":        identity.osVersion,
			"platform":         "DESKTOP",
			"userAgent":        userAgent,
			"utcOffsetMinutes": utcOffsetSeconds / 60,
		},
		"user": map[string]any{
			"lockedSafetyMode": false,
		},
	}
}

type innerTubeBrowserIdentity struct {
	browserName    string
	browserVersion string
	osName         string
	osVersion      string
}

func innerTubeBrowserIdentityFromUserAgent(userAgent string) innerTubeBrowserIdentity {
	identity := innerTubeBrowserIdentity{
		browserName:    "Safari",
		browserVersion: "17.0",
		osName:         "Macintosh",
		osVersion:      "10_15_7",
	}
	switch {
	case strings.Contains(userAgent, "Windows NT"):
		identity.osName = "Windows"
		identity.osVersion = innerTubeUserAgentVersion(userAgent, "Windows NT ")
	case strings.Contains(userAgent, "Mac OS X"):
		identity.osVersion = innerTubeUserAgentVersion(userAgent, "Mac OS X ")
	case strings.Contains(userAgent, "Linux"):
		identity.osName = "Linux"
		identity.osVersion = "x86_64"
	}
	switch {
	case strings.Contains(userAgent, "Edg/"):
		identity.browserName = "Edge"
		identity.browserVersion = innerTubeUserAgentVersion(userAgent, "Edg/")
	case strings.Contains(userAgent, "Chrome/"):
		identity.browserName = "Chrome"
		identity.browserVersion = innerTubeUserAgentVersion(userAgent, "Chrome/")
	case strings.Contains(userAgent, "Version/") && strings.Contains(userAgent, "Safari/"):
		identity.browserVersion = innerTubeUserAgentVersion(userAgent, "Version/")
	}
	if identity.browserVersion == "" {
		identity.browserVersion = "17.0"
	}
	if identity.osVersion == "" {
		identity.osVersion = "10_15_7"
	}
	return identity
}

func innerTubeUserAgentVersion(userAgent string, marker string) string {
	start := strings.Index(userAgent, marker)
	if start < 0 {
		return ""
	}
	value := userAgent[start+len(marker):]
	if end := strings.IndexAny(value, " );"); end >= 0 {
		value = value[:end]
	}
	return strings.TrimSpace(value)
}

func (client *innerTubeClient) doRequest(
	ctx context.Context,
	requestURL string,
	payload []byte,
	headers map[string]string,
	authenticated bool,
	locale string,
) ([]byte, error) {
	request, err := http.NewRequestWithContext(ctx, http.MethodPost, requestURL, bytes.NewReader(payload))
	if err != nil {
		return nil, err
	}
	for key, value := range headers {
		request.Header.Set(key, value)
	}
	request.Header.Set("Accept-Language", innerTubeAcceptLanguage(locale))

	response, err := client.httpClientForRequest().Do(request)
	if err != nil {
		return nil, wrapInnerTubeRequestError(err)
	}
	defer response.Body.Close()

	data, readErr := io.ReadAll(io.LimitReader(response.Body, youtubeInnerTubeResponseMax+1))
	if readErr != nil {
		return nil, wrapInnerTubeRequestError(readErr)
	}
	if len(data) > youtubeInnerTubeResponseMax {
		return nil, errors.New("youtube api response exceeded size limit")
	}

	if response.StatusCode == http.StatusUnauthorized || response.StatusCode == http.StatusForbidden {
		if authenticated {
			return nil, fmt.Errorf("%w: status %d", errYouTubeInnerTubeAuthExpired, response.StatusCode)
		}
		return nil, fmt.Errorf("%w: status %d", errYouTubeInnerTubeNotAuthenticated, response.StatusCode)
	}
	if response.StatusCode < http.StatusOK || response.StatusCode >= http.StatusMultipleChoices {
		return nil, &innerTubeHTTPStatusError{
			StatusCode: response.StatusCode,
			Detail:     string(data),
		}
	}
	return data, nil
}

func (client *innerTubeClient) httpClientForRequest() *http.Client {
	if client != nil && client.httpClientProvider != nil {
		if provided := client.httpClientProvider.HTTPClient(); provided != nil {
			return provided
		}
	}
	if client != nil && client.httpClient != nil {
		return client.httpClient
	}
	return &http.Client{Timeout: youtubeInnerTubeHTTPTimeout}
}

func (client *innerTubeClient) shouldRetry(
	err error,
	options innerTubeRequestOptions,
	attempt int,
) bool {
	if err == nil || !options.retryTransient || attempt >= len(client.retryDelays) {
		return false
	}
	if errors.Is(err, context.Canceled) || errors.Is(err, errYouTubeInnerTubeNotAuthenticated) ||
		errors.Is(err, errYouTubeInnerTubeAuthExpired) {
		return false
	}
	var statusErr *innerTubeHTTPStatusError
	if errors.As(err, &statusErr) {
		switch statusErr.StatusCode {
		case http.StatusTooManyRequests,
			http.StatusInternalServerError,
			http.StatusBadGateway,
			http.StatusServiceUnavailable,
			http.StatusGatewayTimeout:
			return true
		default:
			return false
		}
	}
	return errors.Is(err, errYouTubeInnerTubeRequestTimedOut) ||
		errors.Is(err, errYouTubeInnerTubeNetworkUnavailable)
}

func (client *innerTubeClient) waitBeforeRetry(ctx context.Context, attempt int) error {
	if attempt < 0 || attempt >= len(client.retryDelays) {
		return nil
	}
	delay := client.retryDelays[attempt]
	if delay <= 0 {
		return nil
	}
	timer := time.NewTimer(delay)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-timer.C:
		return nil
	}
}

func (client *innerTubeClient) cachedResponse(key string) ([]byte, bool) {
	if client == nil || key == "" {
		return nil, false
	}
	now := client.currentTime()
	client.cacheMu.Lock()
	defer client.cacheMu.Unlock()
	entry, ok := client.cache[key]
	if !ok {
		return nil, false
	}
	if !entry.expiresAt.After(now) {
		delete(client.cache, key)
		return nil, false
	}
	entry.lastAccessed = now
	client.cache[key] = entry
	return append([]byte(nil), entry.data...), true
}

func (client *innerTubeClient) cacheResponse(key string, data []byte, ttl time.Duration) {
	if client == nil || key == "" || len(data) == 0 || ttl <= 0 || client.cacheLimit <= 0 {
		return
	}
	now := client.currentTime()
	client.cacheMu.Lock()
	defer client.cacheMu.Unlock()
	if client.cache == nil {
		client.cache = make(map[string]innerTubeCacheEntry)
	}
	for existingKey, entry := range client.cache {
		if !entry.expiresAt.After(now) {
			delete(client.cache, existingKey)
		}
	}
	if _, replacing := client.cache[key]; !replacing && len(client.cache) >= client.cacheLimit {
		oldestKey := ""
		var oldestTime time.Time
		for existingKey, entry := range client.cache {
			if oldestKey == "" || entry.lastAccessed.Before(oldestTime) {
				oldestKey = existingKey
				oldestTime = entry.lastAccessed
			}
		}
		if oldestKey != "" {
			delete(client.cache, oldestKey)
		}
	}
	client.cache[key] = innerTubeCacheEntry{
		data:         append([]byte(nil), data...),
		expiresAt:    now.Add(ttl),
		lastAccessed: now,
	}
}

func (client *innerTubeClient) clearCache() {
	if client == nil {
		return
	}
	client.cacheMu.Lock()
	client.cache = make(map[string]innerTubeCacheEntry)
	client.cacheMu.Unlock()
}

func (client *innerTubeClient) currentTime() time.Time {
	if client != nil && client.now != nil {
		return client.now()
	}
	return time.Now()
}

func (client *innerTubeClient) browserUserAgent() string {
	if client != nil {
		if userAgent := strings.TrimSpace(client.userAgent); userAgent != "" {
			return userAgent
		}
	}
	return youtubeInnerTubeDefaultUA
}

func innerTubeSAPISIDHash(sapisid string, origin string, timestamp int64) string {
	input := fmt.Sprintf("%d %s %s", timestamp, sapisid, origin)
	hash := sha1.Sum([]byte(input))
	return fmt.Sprintf("%d_%x", timestamp, hash)
}

func innerTubeCookieMaterialHash(cookieHeader string) string {
	hash := sha256.Sum256([]byte(cookieHeader))
	return hex.EncodeToString(hash[:])
}

func innerTubeCacheKey(endpoint string, locale string, scope string, payload []byte) string {
	hash := sha256.New()
	_, _ = io.WriteString(hash, strings.Trim(endpoint, "/"))
	_, _ = io.WriteString(hash, "\x00")
	_, _ = io.WriteString(hash, normalizeInnerTubeLocale(locale))
	_, _ = io.WriteString(hash, "\x00")
	_, _ = io.WriteString(hash, scope)
	_, _ = io.WriteString(hash, "\x00")
	_, _ = hash.Write(payload)
	return hex.EncodeToString(hash.Sum(nil))
}

func activeInnerTubeCookies(records []appcookies.Record, now time.Time) []appcookies.Record {
	active := make([]appcookies.Record, 0, len(records))
	for _, record := range records {
		if strings.TrimSpace(record.Name) == "" || strings.TrimSpace(record.Value) == "" {
			continue
		}
		if record.Expires > 0 && record.Expires <= now.Unix() {
			continue
		}
		active = append(active, record)
	}
	return active
}

func buildInnerTubeCookieHeader(records []appcookies.Record) string {
	sorted := append([]appcookies.Record(nil), records...)
	sort.SliceStable(sorted, func(left int, right int) bool {
		if sorted[left].Name != sorted[right].Name {
			return sorted[left].Name < sorted[right].Name
		}
		if len(sorted[left].Path) != len(sorted[right].Path) {
			return len(sorted[left].Path) > len(sorted[right].Path)
		}
		return sorted[left].Domain < sorted[right].Domain
	})
	parts := make([]string, 0, len(sorted))
	seen := make(map[string]struct{}, len(sorted))
	for _, record := range sorted {
		name := strings.TrimSpace(record.Name)
		value := strings.TrimSpace(record.Value)
		if name == "" || value == "" || strings.ContainsAny(name, ";\r\n\t ") ||
			strings.ContainsAny(value, "\r\n") {
			continue
		}
		if _, exists := seen[name]; exists {
			continue
		}
		seen[name] = struct{}{}
		parts = append(parts, name+"="+value)
	}
	return strings.Join(parts, "; ")
}

func findInnerTubeSAPISID(records []appcookies.Record) string {
	for _, name := range []string{"__Secure-3PAPISID", "SAPISID", "__Secure-1PAPISID"} {
		for _, record := range records {
			if record.Name == name {
				if value := strings.TrimSpace(record.Value); value != "" {
					return value
				}
			}
		}
	}
	return ""
}

func decodeInnerTubeResponse(data []byte) (map[string]any, error) {
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.UseNumber()
	var decoded map[string]any
	if err := decoder.Decode(&decoded); err != nil {
		return nil, fmt.Errorf("decode youtube api response: %w", err)
	}
	if decoded == nil {
		return nil, errors.New("youtube api response is not a JSON object")
	}
	return decoded, nil
}

func isMissingInnerTubeSessionError(err error) bool {
	return errors.Is(err, appsessions.ErrNoCookies) ||
		errors.Is(err, appsessions.ErrSessionNotFound) ||
		errors.Is(err, appsessions.ErrInvalidSession)
}

func wrapInnerTubeRequestError(err error) error {
	if err == nil || errors.Is(err, context.Canceled) {
		return err
	}
	if isInnerTubeTimeoutError(err) {
		return fmt.Errorf("%w: %w", errYouTubeInnerTubeRequestTimedOut, err)
	}
	if isInnerTubeNetworkError(err) {
		return fmt.Errorf("%w: %w", errYouTubeInnerTubeNetworkUnavailable, err)
	}
	return err
}

func isInnerTubeTimeoutError(err error) bool {
	if errors.Is(err, context.DeadlineExceeded) {
		return true
	}
	var netErr net.Error
	if errors.As(err, &netErr) && netErr.Timeout() {
		return true
	}
	lower := strings.ToLower(strings.TrimSpace(err.Error()))
	return strings.Contains(lower, "context deadline exceeded") ||
		strings.Contains(lower, "client.timeout") ||
		strings.Contains(lower, "timed out") ||
		strings.Contains(lower, "i/o timeout") ||
		strings.Contains(lower, "tls handshake timeout")
}

func isInnerTubeNetworkError(err error) bool {
	if errors.Is(err, io.EOF) || errors.Is(err, io.ErrUnexpectedEOF) {
		return true
	}
	lower := strings.ToLower(strings.TrimSpace(err.Error()))
	for _, marker := range []string{
		"no such host",
		"network is unreachable",
		"connection refused",
		"connection reset",
		"broken pipe",
		"server misbehaving",
		"temporary failure in name resolution",
		"unexpected eof",
	} {
		if strings.Contains(lower, marker) {
			return true
		}
	}
	return false
}

type innerTubeLocaleContextKey struct{}

func withInnerTubeLocale(ctx context.Context, locale string) context.Context {
	if ctx == nil {
		ctx = context.Background()
	}
	normalized := normalizeInnerTubeLocale(locale)
	if normalized == "" {
		return ctx
	}
	return context.WithValue(ctx, innerTubeLocaleContextKey{}, normalized)
}

func innerTubeLocaleFromContext(ctx context.Context) string {
	if ctx != nil {
		if value, ok := ctx.Value(innerTubeLocaleContextKey{}).(string); ok {
			if normalized := normalizeInnerTubeLocale(value); normalized != "" {
				return normalized
			}
		}
	}
	return "en"
}

func normalizeInnerTubeLocale(locale string) string {
	value := strings.ReplaceAll(strings.TrimSpace(locale), "_", "-")
	if value == "" {
		return "en"
	}
	lower := strings.ToLower(value)
	switch {
	case strings.HasPrefix(lower, "zh-tw"), strings.HasPrefix(lower, "zh-hant"),
		strings.HasPrefix(lower, "zh-hk"), strings.HasPrefix(lower, "zh-mo"):
		return "zh-TW"
	case lower == "zh", strings.HasPrefix(lower, "zh-cn"), strings.HasPrefix(lower, "zh-hans"),
		strings.HasPrefix(lower, "zh-sg"), strings.HasPrefix(lower, "zh-"):
		return "zh-CN"
	case lower == "ja" || strings.HasPrefix(lower, "ja-"):
		return "ja-JP"
	case lower == "ko" || strings.HasPrefix(lower, "ko-"):
		return "ko-KR"
	case lower == "es" || strings.HasPrefix(lower, "es-"):
		return "es-419"
	case lower == "pt" || strings.HasPrefix(lower, "pt-"):
		return "pt-BR"
	case lower == "id" || strings.HasPrefix(lower, "id-"):
		return "id-ID"
	case lower == "vi" || strings.HasPrefix(lower, "vi-"):
		return "vi-VN"
	default:
		return "en"
	}
}

func innerTubeAcceptLanguage(locale string) string {
	switch normalizeInnerTubeLocale(locale) {
	case "zh-CN":
		return "zh-CN,zh;q=0.9,en;q=0.7"
	case "zh-TW":
		return "zh-TW,zh-Hant;q=0.9,zh;q=0.8,en;q=0.7"
	case "ja-JP":
		return "ja-JP,ja;q=0.9,en;q=0.7"
	case "ko-KR":
		return "ko-KR,ko;q=0.9,en;q=0.7"
	case "es-419":
		return "es-419,es;q=0.9,en;q=0.7"
	case "pt-BR":
		return "pt-BR,pt;q=0.9,en;q=0.7"
	case "id-ID":
		return "id-ID,id;q=0.9,en;q=0.7"
	case "vi-VN":
		return "vi-VN,vi;q=0.9,en;q=0.7"
	default:
		return "en-US,en;q=0.9"
	}
}
