package rss

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"html"
	"io"
	"net/http"
	"net/url"
	"regexp"
	"slices"
	"sort"
	"strconv"
	"strings"
	"time"
	"unicode"

	"xiadown/internal/application/networkpolicy"
	domainrss "xiadown/internal/domain/rss"
)

const (
	discoveryCacheTTL                 = 24 * time.Hour
	discoveryRequestTimeout           = 45 * time.Second
	maxDiscoveryResponseSize          = 8 << 20
	maxDiscoveryJSONDepth             = 32
	maxDiscoveryJSONTokens            = 250_000
	maxDiscoveryJSONStringBytes       = 128 << 10
	maxDiscoveryNamespaces            = 4096
	maxDiscoveryRoutes                = 10_000
	maxDiscoveryCategories            = 32
	maxDiscoveryTemplateSegments      = 64
	maxDiscoveryParameters            = 32
	maxDiscoveryParameterOptions      = 128
	maxDiscoveryPathAliases           = 64
	maxDiscoveryCatchAllParameters    = 8
	maxDiscoveryNamespaceIDBytes      = 512
	maxDiscoveryRouteKeyBytes         = 4096
	maxDiscoveryNameBytes             = 4096
	maxDiscoveryDescriptionInputBytes = 96 << 10
	maxDiscoveryLanguageBytes         = 128
	maxDiscoveryParameterValueBytes   = 2048
	defaultDiscoveryLimit             = 80
	maximumDiscoveryLimit             = 200
)

var (
	discoveryBuildURLs = []string{
		"https://raw.githubusercontent.com/DIYgod/RSSHub/refs/heads/gh-pages/build/routes.json",
		"https://cdn.jsdelivr.net/gh/DIYgod/RSSHub@gh-pages/build/routes.json",
	}
	discoveryCategoryOrder = []string{
		"social-media", "new-media", "traditional-media", "bbs", "blog", "programming",
		"design", "live", "multimedia", "picture", "anime", "program-update", "university",
		"forecast", "travel", "shopping", "game", "reading", "government", "study",
		"journal", "finance", "sport", "other",
	}
	discoveryRSSHubURLPattern = regexp.MustCompile(`(?i)(?:https?://[^/\s"'<>]*rsshub[^/\s"'<>]*/|rsshub://)([^\s"'<>，。；、]+)`)
	discoveryRoutePattern     = regexp.MustCompile(`/[A-Za-z0-9][A-Za-z0-9._~@%-]*(?:/[^\s"'<>，。；、)）]+)+`)
	discoveryTemplatePattern  = regexp.MustCompile(`(^|/):`)
	discoveryChinesePattern   = regexp.MustCompile(`[\x{4e00}-\x{9fff}]|\.cn\b|bilibili|weibo|zhihu|sspai|36kr|douban`)
	discoveryExplicitToken    = regexp.MustCompile(`(^|[^a-z0-9])(jav|r18)([^a-z0-9]|$)`)
	discoveryExplicitSources  = map[string]struct{}{
		"0xxx": {}, "141jav": {}, "141ppv": {}, "18comic": {}, "2048": {},
		"4kup": {}, "7mmtv": {}, "8kcos": {}, "91porn": {}, "95mm": {},
		"abskoop": {}, "asiantolick": {}, "chikubi": {}, "cool18": {}, "coomer": {},
		"e-hentai": {}, "ehentai": {}, "everia": {}, "f95zone": {}, "fansly": {},
		"gelbooru": {}, "hanime1": {}, "iwara": {}, "jable": {}, "javbus": {},
		"javdb": {}, "javlibrary": {}, "javtiful": {}, "javtrailers": {}, "jpxgmn": {},
		"literotica": {}, "manyvids": {}, "missav": {}, "myfans": {}, "netflav": {},
		"nhentai": {}, "ntrblog": {}, "oreno3d": {}, "pawchive": {}, "pornhub": {},
		"projectjav": {}, "rule34video": {}, "sehuatang": {}, "shuiguopai": {}, "sis001": {},
		"spankbang": {}, "t66y": {}, "uraaka-joshi": {}, "wnacg": {}, "xbookcn": {},
		"xhamster": {}, "xsijishe": {},
	}
	discoveryMarkdownLink = regexp.MustCompile(`\[([^\]]+)]\([^)]+\)`)
	discoveryMarkdownCode = regexp.MustCompile("`([^`]+)`")
)

func (service *Service) ListDiscovery(ctx context.Context, request DiscoveryRequest) (DiscoveryResult, error) {
	if service == nil || service.discoveryRepository == nil {
		return DiscoveryResult{}, errors.New("RSS discovery repository unavailable")
	}
	state, err := service.discoveryRepository.GetDiscoveryState(ctx)
	if err != nil {
		return DiscoveryResult{}, fmt.Errorf("load RSS discovery state: %w", err)
	}
	if request.ForceRefresh || discoveryStateExpired(state, service.now()) {
		if !request.ForceRefresh && state.RouteCount > 0 {
			service.startDiscoveryRefresh(ctx)
		} else {
			state, err = service.refreshDiscoveryCache(ctx, state, request.ForceRefresh)
			if err != nil && (request.ForceRefresh || state.RouteCount == 0) {
				return DiscoveryResult{}, err
			}
		}
	}
	page, err := service.discoveryRepository.QueryDiscovery(ctx, normalizeDiscoveryQuery(request))
	if err != nil {
		return DiscoveryResult{}, fmt.Errorf("query RSS discovery catalog: %w", err)
	}
	return discoveryResultFromPage(page), nil
}

func (service *Service) startDiscoveryRefresh(ctx context.Context) {
	if !service.discoveryRefreshing.CompareAndSwap(false, true) {
		return
	}
	// Discovery is a rebuildable public cache. Detach from the short-lived Wails
	// request so callers can use stale data immediately, while retaining request
	// values and enforcing the same hard refresh deadline below.
	background := context.WithoutCancel(ctx)
	go func() {
		defer service.discoveryRefreshing.Store(false)
		_, _ = service.refreshDiscoveryCache(background, domainrss.DiscoveryState{}, false)
	}()
}

func discoveryStateExpired(state domainrss.DiscoveryState, now time.Time) bool {
	return state.RouteCount <= 0 || state.FetchedAt.IsZero() || !state.FetchedAt.Add(discoveryCacheTTL).After(now)
}

func (service *Service) refreshDiscoveryCache(ctx context.Context, previous domainrss.DiscoveryState, force bool) (domainrss.DiscoveryState, error) {
	timeout := service.discoveryTimeout
	if timeout <= 0 {
		timeout = discoveryRequestTimeout
	}
	refreshCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	service.discoveryMu.Lock()
	defer service.discoveryMu.Unlock()

	// A caller may have waited behind another successful refresh. Re-read after
	// acquiring the lock and avoid a duplicate request unless this was an
	// explicit force refresh.
	current, err := service.discoveryRepository.GetDiscoveryState(refreshCtx)
	if err != nil {
		return previous, fmt.Errorf("reload RSS discovery cache: %w", err)
	}
	if !force && !discoveryStateExpired(current, service.now()) {
		return current, nil
	}
	fetcher := service.fetchDiscoveryIndex
	if service.discoveryFetcher != nil {
		fetcher = service.discoveryFetcher
	}
	refreshed, err := fetcher(refreshCtx)
	if err != nil {
		if current.RouteCount > 0 {
			return current, fmt.Errorf("refresh RSS discovery cache: %w", err)
		}
		return previous, fmt.Errorf("refresh RSS discovery cache: %w", err)
	}
	refreshed = normalizeDiscoveryCache(refreshed)
	if len(refreshed.Routes) == 0 {
		return current, errors.New("refresh RSS discovery cache: catalog contained no usable routes")
	}
	if err := service.discoveryRepository.ReplaceDiscoveryCache(refreshCtx, refreshed); err != nil {
		return current, fmt.Errorf("save RSS discovery cache: %w", err)
	}
	return domainrss.DiscoveryState{
		SourceURL: refreshed.SourceURL, FetchedAt: refreshed.FetchedAt, RouteCount: len(refreshed.Routes),
	}, nil
}

func (service *Service) fetchDiscoveryIndex(ctx context.Context) (domainrss.DiscoveryCache, error) {
	errorsByURL := make([]string, 0, len(discoveryBuildURLs)+len(service.mirrors))
	for _, sourceURL := range discoveryBuildURLs {
		body, err := service.fetchDiscoveryDocument(ctx, sourceURL, "application/json, */*;q=0.2")
		if err == nil {
			routes, parseErr := parseRSSHubBuildRoutes(body)
			if parseErr == nil && len(routes) > 0 {
				return domainrss.DiscoveryCache{Routes: routes, SourceURL: sourceURL, FetchedAt: service.now()}, nil
			}
			if parseErr == nil {
				parseErr = errors.New("catalog contained no usable routes")
			}
			err = parseErr
		}
		errorsByURL = append(errorsByURL, fmt.Sprintf("%s: %v", sourceURL, err))
	}
	for _, mirror := range service.mirrors {
		sourceURL := strings.TrimRight(mirror, "/") + "/rsshub/routes"
		body, err := service.fetchDiscoveryDocument(ctx, sourceURL, "application/rss+xml, application/xml, text/xml, */*;q=0.2")
		if err == nil {
			routes, parseErr := parseRSSHubRoutesFeed(body)
			if parseErr == nil && len(routes) > 0 {
				return domainrss.DiscoveryCache{Routes: routes, SourceURL: sourceURL, FetchedAt: service.now()}, nil
			}
			if parseErr == nil {
				parseErr = errors.New("feed contained no usable routes")
			}
			err = parseErr
		}
		errorsByURL = append(errorsByURL, fmt.Sprintf("%s: %v", sourceURL, err))
	}
	return domainrss.DiscoveryCache{}, fmt.Errorf("RSSHub discovery index unavailable: %s", strings.Join(errorsByURL, " | "))
}

func (service *Service) fetchDiscoveryDocument(ctx context.Context, rawURL, accept string) ([]byte, error) {
	if _, err := networkpolicy.ValidatePublicHTTPURL(rawURL); err != nil {
		return nil, err
	}
	requestCtx, cancel := context.WithTimeout(ctx, discoveryRequestTimeout)
	defer cancel()
	request, err := http.NewRequestWithContext(requestCtx, http.MethodGet, rawURL, nil)
	if err != nil {
		return nil, err
	}
	request.Header.Set("Accept", accept)
	request.Header.Set("User-Agent", "XiaDown RSS/1.0")
	response, err := service.httpClient().Do(request)
	if err != nil {
		return nil, err
	}
	defer response.Body.Close()
	if response.StatusCode < http.StatusOK || response.StatusCode >= http.StatusMultipleChoices {
		_, _ = io.Copy(io.Discard, io.LimitReader(response.Body, 32<<10))
		return nil, fmt.Errorf("HTTP %d", response.StatusCode)
	}
	body, err := io.ReadAll(io.LimitReader(response.Body, maxDiscoveryResponseSize+1))
	if err != nil {
		return nil, err
	}
	if len(body) > maxDiscoveryResponseSize {
		return nil, fmt.Errorf("response exceeds %d bytes", maxDiscoveryResponseSize)
	}
	return body, nil
}

type rssHubBuildDocument struct {
	Namespaces []rssHubBuildNamespace
}

type rssHubBuildNamespace struct {
	ID, Name, URL, Language string
	Categories              []string
	Routes                  []rssHubBuildRoute
}

type rssHubBuildRoute struct {
	Key, Name, URL, Language, Description, Example string
	Paths                                          []string
	Categories                                     []string
	Parameters                                     []rssHubBuildParameter
	View                                           int
	RequiresConfig, RequiresPuppeteer              bool
}

type rssHubBuildParameter struct {
	Name, Description, Type string
	DefaultValue            *string
	Optional                *bool
	Options                 []domainrss.DiscoveryParameterOption
	Simple                  bool
}

type discoveryJSONReader struct {
	decoder *json.Decoder
	tokens  int
}

func decodeRSSHubBuildDocument(body []byte) (rssHubBuildDocument, error) {
	if len(body) > maxDiscoveryResponseSize {
		return rssHubBuildDocument{}, fmt.Errorf("RSSHub routes JSON exceeds %d bytes", maxDiscoveryResponseSize)
	}
	if err := validateDiscoveryJSONLexicalBudget(body); err != nil {
		return rssHubBuildDocument{}, err
	}
	decoder := json.NewDecoder(bytes.NewReader(body))
	decoder.UseNumber()
	reader := &discoveryJSONReader{decoder: decoder}
	start, err := reader.next()
	if err != nil {
		return rssHubBuildDocument{}, err
	}
	if !jsonTokenIsDelimiter(start, '{') {
		return rssHubBuildDocument{}, errors.New("RSSHub routes JSON root must be an object")
	}

	document := rssHubBuildDocument{Namespaces: make([]rssHubBuildNamespace, 0, 2048)}
	namespaceIDs := make(map[string]struct{}, 2048)
	namespaceCount := 0
	totalRoutes := 0
	for decoder.More() {
		keyToken, tokenErr := reader.next()
		if tokenErr != nil {
			return rssHubBuildDocument{}, tokenErr
		}
		namespaceID, ok := keyToken.(string)
		if !ok {
			return rssHubBuildDocument{}, errors.New("RSSHub namespace key must be a string")
		}
		namespaceCount++
		if namespaceCount > maxDiscoveryNamespaces {
			return rssHubBuildDocument{}, fmt.Errorf("RSSHub routes JSON exceeds %d namespaces", maxDiscoveryNamespaces)
		}
		if len(namespaceID) == 0 || len(namespaceID) > maxDiscoveryNamespaceIDBytes {
			return rssHubBuildDocument{}, errors.New("RSSHub namespace identifier exceeds the safety limit")
		}
		if _, duplicate := namespaceIDs[namespaceID]; duplicate {
			return rssHubBuildDocument{}, fmt.Errorf("duplicate RSSHub namespace %q", limitString(namespaceID, 128))
		}
		namespaceIDs[namespaceID] = struct{}{}
		valueToken, tokenErr := reader.next()
		if tokenErr != nil {
			return rssHubBuildDocument{}, tokenErr
		}
		namespace, usable, parseErr := decodeRSSHubBuildNamespace(reader, namespaceID, valueToken, &totalRoutes)
		if parseErr != nil {
			return rssHubBuildDocument{}, parseErr
		}
		if usable {
			document.Namespaces = append(document.Namespaces, namespace)
		}
	}
	if err := reader.consumeEnd('}'); err != nil {
		return rssHubBuildDocument{}, err
	}
	if token, trailingErr := reader.next(); trailingErr == nil {
		return rssHubBuildDocument{}, fmt.Errorf("unexpected trailing RSSHub JSON token %v", token)
	} else if !errors.Is(trailingErr, io.EOF) {
		return rssHubBuildDocument{}, trailingErr
	}
	sort.Slice(document.Namespaces, func(left, right int) bool {
		return document.Namespaces[left].ID < document.Namespaces[right].ID
	})
	return document, nil
}

func decodeRSSHubBuildNamespace(
	reader *discoveryJSONReader,
	namespaceID string,
	first any,
	totalRoutes *int,
) (rssHubBuildNamespace, bool, error) {
	if !jsonTokenIsDelimiter(first, '{') {
		return rssHubBuildNamespace{}, false, reader.skipValue(first)
	}
	namespace := rssHubBuildNamespace{ID: namespaceID}
	for reader.decoder.More() {
		key, value, err := reader.nextObjectField()
		if err != nil {
			return rssHubBuildNamespace{}, false, err
		}
		switch key {
		case "name":
			namespace.Name, err = reader.stringValue(value, maxDiscoveryNameBytes)
		case "url":
			namespace.URL, err = reader.stringValue(value, maxDiscoveryRouteKeyBytes)
		case "lang":
			namespace.Language, err = reader.stringValue(value, maxDiscoveryLanguageBytes)
		case "categories":
			namespace.Categories, err = reader.stringArray(value, maxDiscoveryCategories, maxDiscoveryNameBytes)
		case "routes":
			namespace.Routes, err = decodeRSSHubBuildRoutesObject(reader, value, totalRoutes)
		default:
			err = reader.skipValue(value)
		}
		if err != nil {
			return rssHubBuildNamespace{}, false, err
		}
	}
	if err := reader.consumeEnd('}'); err != nil {
		return rssHubBuildNamespace{}, false, err
	}
	sort.Slice(namespace.Routes, func(left, right int) bool {
		return namespace.Routes[left].Key < namespace.Routes[right].Key
	})
	return namespace, true, nil
}

func decodeRSSHubBuildRoutesObject(reader *discoveryJSONReader, first any, totalRoutes *int) ([]rssHubBuildRoute, error) {
	if !jsonTokenIsDelimiter(first, '{') {
		return nil, reader.skipValue(first)
	}
	routes := make([]rssHubBuildRoute, 0, 8)
	keys := make(map[string]struct{})
	for reader.decoder.More() {
		keyToken, err := reader.next()
		if err != nil {
			return nil, err
		}
		key, ok := keyToken.(string)
		if !ok {
			return nil, errors.New("RSSHub route key must be a string")
		}
		(*totalRoutes)++
		if *totalRoutes > maxDiscoveryRoutes {
			return nil, fmt.Errorf("RSSHub routes JSON exceeds %d routes", maxDiscoveryRoutes)
		}
		value, err := reader.next()
		if err != nil {
			return nil, err
		}
		if len(key) == 0 || len(key) > maxDiscoveryRouteKeyBytes {
			if err := reader.skipValue(value); err != nil {
				return nil, err
			}
			continue
		}
		if _, duplicate := keys[key]; duplicate {
			return nil, fmt.Errorf("duplicate RSSHub route %q", limitString(key, 128))
		}
		keys[key] = struct{}{}
		route, usable, err := decodeRSSHubBuildRoute(reader, key, value)
		if err != nil {
			return nil, err
		}
		if usable {
			routes = append(routes, route)
		}
	}
	if err := reader.consumeEnd('}'); err != nil {
		return nil, err
	}
	return routes, nil
}

func decodeRSSHubBuildRoute(reader *discoveryJSONReader, key string, first any) (rssHubBuildRoute, bool, error) {
	if !jsonTokenIsDelimiter(first, '{') {
		return rssHubBuildRoute{}, false, reader.skipValue(first)
	}
	route := rssHubBuildRoute{Key: key}
	for reader.decoder.More() {
		field, value, err := reader.nextObjectField()
		if err != nil {
			return rssHubBuildRoute{}, false, err
		}
		switch field {
		case "path":
			route.Paths, err = reader.pathValue(value)
		case "example":
			route.Example, err = reader.stringValue(value, maxDiscoveryRouteKeyBytes)
		case "name":
			route.Name, err = reader.stringValue(value, maxDiscoveryNameBytes)
		case "url":
			route.URL, err = reader.stringValue(value, maxDiscoveryRouteKeyBytes)
		case "lang":
			route.Language, err = reader.stringValue(value, maxDiscoveryLanguageBytes)
		case "description":
			route.Description, err = reader.stringValue(value, maxDiscoveryDescriptionInputBytes)
		case "categories":
			route.Categories, err = reader.stringArray(value, maxDiscoveryCategories, maxDiscoveryNameBytes)
		case "view":
			route.View, err = reader.intValue(value)
		case "features":
			route.RequiresConfig, route.RequiresPuppeteer, err = decodeRSSHubBuildFeatures(reader, value)
		case "parameters":
			route.Parameters, err = decodeRSSHubBuildParameters(reader, value)
		default:
			err = reader.skipValue(value)
		}
		if err != nil {
			return rssHubBuildRoute{}, false, err
		}
	}
	if err := reader.consumeEnd('}'); err != nil {
		return rssHubBuildRoute{}, false, err
	}
	return route, true, nil
}

func decodeRSSHubBuildFeatures(reader *discoveryJSONReader, first any) (bool, bool, error) {
	if !jsonTokenIsDelimiter(first, '{') {
		return false, false, reader.skipValue(first)
	}
	requiresConfig, requiresPuppeteer := false, false
	for reader.decoder.More() {
		key, value, err := reader.nextObjectField()
		if err != nil {
			return false, false, err
		}
		flag, ok := value.(bool)
		if key == "requireConfig" && ok {
			requiresConfig = flag
		} else if key == "requirePuppeteer" && ok {
			requiresPuppeteer = flag
		} else if err := reader.skipValue(value); err != nil {
			return false, false, err
		}
	}
	return requiresConfig, requiresPuppeteer, reader.consumeEnd('}')
}

func decodeRSSHubBuildParameters(reader *discoveryJSONReader, first any) ([]rssHubBuildParameter, error) {
	if !jsonTokenIsDelimiter(first, '{') {
		return nil, reader.skipValue(first)
	}
	parameters := make([]rssHubBuildParameter, 0, 8)
	for reader.decoder.More() {
		keyToken, err := reader.next()
		if err != nil {
			return nil, err
		}
		name, ok := keyToken.(string)
		if !ok {
			return nil, errors.New("RSSHub parameter key must be a string")
		}
		value, err := reader.next()
		if err != nil {
			return nil, err
		}
		if len(parameters) >= maxDiscoveryParameters || len(name) == 0 || len(name) > maxDiscoveryNameBytes {
			if err := reader.skipValue(value); err != nil {
				return nil, err
			}
			continue
		}
		parameter, usable, err := decodeRSSHubBuildParameter(reader, name, value)
		if err != nil {
			return nil, err
		}
		if usable {
			parameters = append(parameters, parameter)
		}
	}
	return parameters, reader.consumeEnd('}')
}

func decodeRSSHubBuildParameter(reader *discoveryJSONReader, name string, first any) (rssHubBuildParameter, bool, error) {
	if description, ok := first.(string); ok {
		return rssHubBuildParameter{
			Name: name, Description: limitString(description, 4096), Type: "string", Simple: true,
		}, true, nil
	}
	if !jsonTokenIsDelimiter(first, '{') {
		return rssHubBuildParameter{}, false, reader.skipValue(first)
	}
	parameter := rssHubBuildParameter{Name: name, Type: "string"}
	for reader.decoder.More() {
		key, value, err := reader.nextObjectField()
		if err != nil {
			return rssHubBuildParameter{}, false, err
		}
		switch key {
		case "description":
			parameter.Description, err = reader.stringValue(value, 4096)
		case "default":
			if text, ok := value.(string); ok {
				text = limitString(text, maxDiscoveryParameterValueBytes)
				parameter.DefaultValue = &text
			} else {
				err = reader.skipValue(value)
			}
		case "optional":
			if optional, ok := value.(bool); ok {
				parameter.Optional = &optional
			} else {
				err = reader.skipValue(value)
			}
		case "type":
			parameter.Type, err = reader.stringValue(value, 64)
			if strings.TrimSpace(parameter.Type) == "" {
				parameter.Type = "string"
			}
		case "options":
			parameter.Options, err = decodeRSSHubBuildParameterOptions(reader, value)
		default:
			err = reader.skipValue(value)
		}
		if err != nil {
			return rssHubBuildParameter{}, false, err
		}
	}
	return parameter, true, reader.consumeEnd('}')
}

func decodeRSSHubBuildParameterOptions(reader *discoveryJSONReader, first any) ([]domainrss.DiscoveryParameterOption, error) {
	if !jsonTokenIsDelimiter(first, '[') {
		return nil, reader.skipValue(first)
	}
	options := make([]domainrss.DiscoveryParameterOption, 0, 8)
	for reader.decoder.More() {
		value, err := reader.next()
		if err != nil {
			return nil, err
		}
		if len(options) >= maxDiscoveryParameterOptions {
			if err := reader.skipValue(value); err != nil {
				return nil, err
			}
			continue
		}
		option, usable, err := decodeRSSHubBuildParameterOption(reader, value)
		if err != nil {
			return nil, err
		}
		if usable {
			options = append(options, option)
		}
	}
	return options, reader.consumeEnd(']')
}

func decodeRSSHubBuildParameterOption(reader *discoveryJSONReader, first any) (domainrss.DiscoveryParameterOption, bool, error) {
	if !jsonTokenIsDelimiter(first, '{') {
		return domainrss.DiscoveryParameterOption{}, false, reader.skipValue(first)
	}
	option := domainrss.DiscoveryParameterOption{}
	for reader.decoder.More() {
		key, value, err := reader.nextObjectField()
		if err != nil {
			return domainrss.DiscoveryParameterOption{}, false, err
		}
		switch key {
		case "value":
			option.Value, err = reader.stringValue(value, maxDiscoveryParameterValueBytes)
		case "label":
			option.Label, err = reader.stringValue(value, 512)
		default:
			err = reader.skipValue(value)
		}
		if err != nil {
			return domainrss.DiscoveryParameterOption{}, false, err
		}
	}
	if err := reader.consumeEnd('}'); err != nil {
		return domainrss.DiscoveryParameterOption{}, false, err
	}
	return option, option.Value != "" && option.Label != "", nil
}

func (reader *discoveryJSONReader) next() (any, error) {
	reader.tokens++
	if reader.tokens > maxDiscoveryJSONTokens {
		return nil, fmt.Errorf("RSSHub routes JSON exceeds %d tokens", maxDiscoveryJSONTokens)
	}
	return reader.decoder.Token()
}

func (reader *discoveryJSONReader) nextObjectField() (string, any, error) {
	keyToken, err := reader.next()
	if err != nil {
		return "", nil, err
	}
	key, ok := keyToken.(string)
	if !ok {
		return "", nil, errors.New("RSSHub object key must be a string")
	}
	value, err := reader.next()
	return key, value, err
}

func (reader *discoveryJSONReader) consumeEnd(want json.Delim) error {
	token, err := reader.next()
	if err != nil {
		return err
	}
	if !jsonTokenIsDelimiter(token, want) {
		return fmt.Errorf("invalid RSSHub JSON container ending: %v", token)
	}
	return nil
}

func (reader *discoveryJSONReader) skipValue(first any) error {
	delimiter, ok := first.(json.Delim)
	if !ok || (delimiter != '{' && delimiter != '[') {
		return nil
	}
	depth := 1
	for depth > 0 {
		token, err := reader.next()
		if err != nil {
			return err
		}
		if nested, ok := token.(json.Delim); ok {
			switch nested {
			case '{', '[':
				depth++
			case '}', ']':
				depth--
			}
		}
	}
	return nil
}

func (reader *discoveryJSONReader) stringValue(first any, limit int) (string, error) {
	value, ok := first.(string)
	if !ok {
		return "", reader.skipValue(first)
	}
	return limitString(value, limit), nil
}

func (reader *discoveryJSONReader) intValue(first any) (int, error) {
	value, ok := first.(json.Number)
	if !ok {
		return 0, reader.skipValue(first)
	}
	parsed, err := value.Int64()
	if err != nil || parsed <= 0 {
		return 0, nil
	}
	maxInt := int64(^uint(0) >> 1)
	if parsed > maxInt {
		return int(maxInt), nil
	}
	return int(parsed), nil
}

func (reader *discoveryJSONReader) stringArray(first any, limit, stringLimit int) ([]string, error) {
	if !jsonTokenIsDelimiter(first, '[') {
		return nil, reader.skipValue(first)
	}
	values := make([]string, 0, min(limit, 8))
	for reader.decoder.More() {
		token, err := reader.next()
		if err != nil {
			return nil, err
		}
		value, ok := token.(string)
		if !ok {
			if err := reader.skipValue(token); err != nil {
				return nil, err
			}
			continue
		}
		if len(values) < limit {
			values = append(values, limitString(value, stringLimit))
		}
	}
	return values, reader.consumeEnd(']')
}

func (reader *discoveryJSONReader) pathValue(first any) ([]string, error) {
	if value, ok := first.(string); ok {
		return []string{limitString(value, maxDiscoveryRouteKeyBytes)}, nil
	}
	return reader.stringArray(first, maxDiscoveryPathAliases, maxDiscoveryRouteKeyBytes)
}

func jsonTokenIsDelimiter(token any, want json.Delim) bool {
	delimiter, ok := token.(json.Delim)
	return ok && delimiter == want
}

func validateDiscoveryJSONLexicalBudget(body []byte) error {
	depth := 0
	for index := 0; index < len(body); index++ {
		switch body[index] {
		case '"':
			start := index + 1
			index++
			for ; index < len(body); index++ {
				if index-start > maxDiscoveryJSONStringBytes {
					return fmt.Errorf("RSSHub routes JSON string exceeds %d bytes", maxDiscoveryJSONStringBytes)
				}
				if body[index] == '\\' {
					index++
					continue
				}
				if body[index] == '"' {
					break
				}
			}
			if index >= len(body) {
				return errors.New("unterminated RSSHub JSON string")
			}
		case '{', '[':
			depth++
			if depth > maxDiscoveryJSONDepth {
				return fmt.Errorf("RSSHub routes JSON exceeds depth %d", maxDiscoveryJSONDepth)
			}
		case '}', ']':
			depth--
			if depth < 0 {
				return errors.New("invalid RSSHub JSON container nesting")
			}
		}
	}
	return nil
}

func (route rssHubBuildRoute) declarationPath() string {
	if strings.HasPrefix(route.Key, "/") || route.Key == "*" || strings.Contains(route.Key, ":") {
		return route.Key
	}
	for _, candidate := range route.Paths {
		if strings.TrimSpace(candidate) != "" {
			return candidate
		}
	}
	return route.Key
}

func (route rssHubBuildRoute) parameterMetadata() map[string]any {
	metadata := make(map[string]any, min(len(route.Parameters), maxDiscoveryParameters))
	for index, parameter := range route.Parameters {
		if index >= maxDiscoveryParameters {
			break
		}
		if parameter.Simple {
			metadata[parameter.Name] = parameter.Description
			continue
		}
		value := map[string]any{
			"description": parameter.Description,
			"type":        parameter.Type,
		}
		if parameter.DefaultValue != nil {
			value["default"] = *parameter.DefaultValue
		}
		if parameter.Optional != nil {
			value["optional"] = *parameter.Optional
		}
		options := make([]any, 0, len(parameter.Options))
		for _, option := range parameter.Options {
			options = append(options, map[string]any{"value": option.Value, "label": option.Label})
		}
		value["options"] = options
		metadata[parameter.Name] = value
	}
	return metadata
}

func parseRSSHubBuildRoutes(body []byte) ([]domainrss.DiscoveryRoute, error) {
	document, err := decodeRSSHubBuildDocument(body)
	if err != nil {
		return nil, fmt.Errorf("parse RSSHub routes JSON: %w", err)
	}
	routes := make([]domainrss.DiscoveryRoute, 0)
	seen := make(map[string]struct{})
	for _, namespace := range document.Namespaces {
		namespaceID := namespace.ID
		namespaceName := displayText(namespace.Name, namespaceID)
		namespaceURL := homepageURL(namespace.URL)
		namespaceCategories := normalizeStringCategories(namespace.Categories)
		namespaceLanguage := normalizeDiscoveryLanguage(namespace.Language)
		for routeIndex, value := range namespace.Routes {
			// Current RSSHub catalogs use the map key as the exact alias path, while
			// older/mirror documents can use a symbolic key and put the declaration
			// in route.path. Keep both forms interoperable.
			declarationPath := value.declarationPath()
			routePath := normalizeDiscoveryRouteTemplate(namespaceID, declarationPath)
			if routePath == "" {
				continue
			}
			examplePath := normalizeDiscoveryRoutePath(value.Example)
			if !concreteDiscoveryRoutePath(examplePath) {
				examplePath = ""
			}
			parameterMetadata := value.parameterMetadata()
			parameters := createDiscoveryParameters(routePath, parameterMetadata, examplePath)
			needsParameters := len(parameters) > 0
			canonical := canonicalDiscoveryTemplateURL(routePath)
			if canonical == "" {
				continue
			}
			categories := normalizeStringCategories(value.Categories)
			if len(categories) == 0 {
				categories = append([]string(nil), namespaceCategories...)
			}
			routeName := displayText(value.Name, discoveryRouteTitle(routePath))
			title := routeName
			if !strings.EqualFold(namespaceName, routeName) {
				title = namespaceName + " - " + routeName
			}
			siteURL := homepageURL(value.URL)
			if siteURL == "" {
				siteURL = namespaceURL
			}
			language := normalizeDiscoveryLanguage(value.Language)
			if language == "" {
				language = namespaceLanguage
			}
			heat := value.View
			if heat < 0 {
				heat = 0
			}
			if heat == 0 {
				heat = len(namespace.Routes) - routeIndex
			}
			description := cleanDiscoveryDescription(value.Description)
			if description == "" {
				description = cleanDiscoveryDescription(formatDiscoveryParameters(parameterMetadata))
			}
			if description == "" {
				description = routeName + " updates."
			}
			route := newDiscoveryRoute(discoveryRouteInput{
				SourceID: namespaceID, SourceName: namespaceName, SourceURL: siteURL,
				SiteURL: siteURL, RoutePath: routePath, ExamplePath: examplePath,
				Title: title, URL: canonical, Description: description, Categories: categories,
				Heat: heat, Language: language,
				RequiresConfig:    value.RequiresConfig,
				RequiresPuppeteer: value.RequiresPuppeteer,
				NeedsParameters:   needsParameters,
				Parameters:        parameters,
			})
			appendUniqueDiscoveryRoute(&routes, seen, route)
		}
	}
	sortDiscoveryRoutes(routes, "popular")
	return routes, nil
}

func selectDiscoveryDeclarationPath(routeKey string, rawPath any) string {
	routeKey = strings.TrimSpace(routeKey)
	if strings.HasPrefix(routeKey, "/") || routeKey == "*" || strings.Contains(routeKey, ":") {
		return routeKey
	}
	switch typed := rawPath.(type) {
	case string:
		if strings.TrimSpace(typed) != "" {
			return typed
		}
	case []any:
		for _, candidate := range typed {
			if path, ok := candidate.(string); ok && strings.TrimSpace(path) != "" {
				return path
			}
		}
	}
	return routeKey
}

func parseRSSHubRoutesFeed(body []byte) ([]domainrss.DiscoveryRoute, error) {
	feed, err := parseFeed(body, "application/rss+xml")
	if err != nil {
		return nil, fmt.Errorf("parse RSSHub routes feed: %w", err)
	}
	routes := make([]domainrss.DiscoveryRoute, 0, len(feed.Entries))
	seen := make(map[string]struct{})
	for index, item := range feed.Entries {
		routePath := normalizeDiscoveryRouteTemplate("", item.ExternalID)
		if routePath == "" {
			continue
		}
		examplePath := firstConcreteDiscoveryRoute(item.Summary + " " + item.Content + " " + item.URL)
		if !concreteDiscoveryRoutePath(examplePath) {
			examplePath = ""
		}
		parameters := createDiscoveryParameters(routePath, nil, examplePath)
		needsParameters := len(parameters) > 0
		canonical := canonicalDiscoveryTemplateURL(routePath)
		if canonical == "" {
			continue
		}
		title := strings.TrimSpace(item.Title)
		if title == "" {
			title = discoveryRouteTitle(routePath)
		}
		sourceID := discoverySourceID(routePath)
		sourceName := displayText(strings.Split(title, " - ")[0], sourceID)
		categories := []string{guessDiscoveryCategory(routePath, title)}
		description := cleanDiscoveryDescription(firstNonEmpty(item.Summary, item.Content))
		if description == "" {
			description = title + " updates."
		}
		route := newDiscoveryRoute(discoveryRouteInput{
			SourceID: sourceID, SourceName: sourceName, SourceURL: item.URL,
			RoutePath: routePath, ExamplePath: examplePath, Title: title, URL: canonical,
			Description: description, Categories: categories, Heat: len(feed.Entries) - index,
			NeedsParameters: needsParameters, Parameters: parameters,
		})
		appendUniqueDiscoveryRoute(&routes, seen, route)
	}
	sortDiscoveryRoutes(routes, "popular")
	return routes, nil
}

type discoveryRouteInput struct {
	SourceID, SourceName, SourceURL, SiteURL string
	RoutePath, ExamplePath, Title, URL       string
	Description, Language                    string
	Categories                               []string
	Parameters                               []domainrss.DiscoveryParameter
	Heat                                     int
	RequiresConfig, RequiresPuppeteer        bool
	NeedsParameters                          bool
}

func newDiscoveryRoute(input discoveryRouteInput) domainrss.DiscoveryRoute {
	categories := append([]string(nil), input.Categories...)
	if len(categories) == 0 {
		categories = []string{"other"}
	}
	language := normalizeDiscoveryLanguage(input.Language)
	if language == "" {
		language = guessDiscoveryLanguage(input.SourceName, input.SourceURL, input.Title, input.URL)
	}
	sourceURL := homepageURL(input.SourceURL)
	siteURL := homepageURL(input.SiteURL)
	return domainrss.DiscoveryRoute{
		ID: discoveryRouteID(RSSHubScheme + input.RoutePath), Title: input.Title, URL: input.URL, Provider: "rsshub",
		Description: input.Description, SourceID: input.SourceID, SourceName: input.SourceName,
		SourceURL: sourceURL, SiteURL: siteURL, IconURL: discoveryFaviconURL(siteURL, sourceURL), RoutePath: input.RoutePath,
		ExamplePath: input.ExamplePath, Categories: categories, Heat: max(input.Heat, 0),
		Language: language, Region: discoveryRegion(language, input.SourceName, input.SourceURL, input.Title, input.URL),
		ViewType: guessDiscoveryViewType(categories), RequiresConfig: input.RequiresConfig,
		RequiresPuppeteer: input.RequiresPuppeteer, NeedsParameters: input.NeedsParameters,
		Parameters: append([]domainrss.DiscoveryParameter(nil), input.Parameters...),
	}
}

func normalizeDiscoveryCache(cache domainrss.DiscoveryCache) domainrss.DiscoveryCache {
	routes := make([]domainrss.DiscoveryRoute, 0, len(cache.Routes))
	for _, cached := range cache.Routes {
		if route, ok := normalizeCachedDiscoveryRoute(cached); ok {
			routes = append(routes, route)
		}
	}
	cache.Routes = routes
	cache.SourceURL = strings.TrimSpace(cache.SourceURL)
	cache.FetchedAt = cache.FetchedAt.UTC()
	return cache
}

func normalizeDiscoveryQuery(request DiscoveryRequest) domainrss.DiscoveryQuery {
	limit := request.Limit
	if limit <= 0 {
		limit = defaultDiscoveryLimit
	}
	limit = min(limit, maximumDiscoveryLimit)
	sortMode := "popular"
	if strings.EqualFold(strings.TrimSpace(request.Sort), "title") {
		sortMode = "title"
	}
	return domainrss.DiscoveryQuery{
		Query: normalizeDiscoverySearch(request.Query), CategoryID: strings.TrimSpace(request.CategoryID),
		Language: normalizeDiscoveryLanguage(request.Language), Sort: sortMode,
		Offset: max(request.Offset, 0), Limit: limit,
	}
}

func discoveryResultFromPage(page domainrss.DiscoveryPage) DiscoveryResult {
	routes := make([]domainrss.DiscoveryRoute, 0, len(page.Routes))
	for _, cached := range page.Routes {
		if route, ok := normalizeCachedDiscoveryRoute(cached); ok {
			routes = append(routes, route)
		}
	}
	categories := append([]domainrss.DiscoveryCategory(nil), page.Categories...)
	for index := range categories {
		categories[index].Examples = append([]string(nil), categories[index].Examples...)
		if len(categories[index].Examples) > 2 {
			categories[index].Examples = categories[index].Examples[:2]
		}
	}
	sort.SliceStable(categories, func(left, right int) bool {
		if categories[left].ID == "all" || categories[right].ID == "all" {
			return categories[left].ID == "all"
		}
		leftOrder, rightOrder := discoveryCategoryRank(categories[left].ID), discoveryCategoryRank(categories[right].ID)
		if leftOrder != rightOrder {
			return leftOrder < rightOrder
		}
		if categories[left].Count != categories[right].Count {
			return categories[left].Count > categories[right].Count
		}
		return categories[left].ID < categories[right].ID
	})
	fetchedAt := ""
	if !page.State.FetchedAt.IsZero() {
		fetchedAt = page.State.FetchedAt.UTC().Format(time.RFC3339Nano)
	}
	return DiscoveryResult{
		Categories: categories, Routes: routes, TotalRouteCount: page.State.RouteCount,
		FilteredRouteCount: page.FilteredRouteCount, Offset: page.Offset, Limit: page.Limit,
		HasMore: page.HasMore, SourceURL: page.State.SourceURL, FetchedAt: fetchedAt,
	}
}

func createDiscoveryResult(cache domainrss.DiscoveryCache, request DiscoveryRequest) DiscoveryResult {
	catalogRoutes := make([]domainrss.DiscoveryRoute, 0, len(cache.Routes))
	for _, cached := range cache.Routes {
		if route, ok := normalizeCachedDiscoveryRoute(cached); ok {
			catalogRoutes = append(catalogRoutes, route)
		}
	}
	language := normalizeDiscoveryLanguage(request.Language)
	languageRoutes := make([]domainrss.DiscoveryRoute, 0, len(catalogRoutes))
	for _, route := range catalogRoutes {
		if language == "" || normalizeDiscoveryLanguage(route.Language) == language {
			languageRoutes = append(languageRoutes, route)
		}
	}
	query := normalizeDiscoverySearch(request.Query)
	categoryID := strings.TrimSpace(request.CategoryID)
	matching := make([]domainrss.DiscoveryRoute, 0, len(languageRoutes))
	for _, route := range languageRoutes {
		if categoryID != "" && categoryID != "all" && !slices.Contains(route.Categories, categoryID) {
			continue
		}
		if !discoveryRouteMatchesQuery(route, query) {
			continue
		}
		matching = append(matching, route)
	}
	sortDiscoveryRoutesForQuery(matching, query, request.Sort)
	offset := max(request.Offset, 0)
	limit := request.Limit
	if limit <= 0 {
		limit = defaultDiscoveryLimit
	}
	limit = min(limit, maximumDiscoveryLimit)
	start := min(offset, len(matching))
	end := min(start+limit, len(matching))
	fetchedAt := ""
	if !cache.FetchedAt.IsZero() {
		fetchedAt = cache.FetchedAt.UTC().Format(time.RFC3339Nano)
	}
	return DiscoveryResult{
		Categories: createDiscoveryCategories(languageRoutes), Routes: matching[start:end],
		TotalRouteCount: len(catalogRoutes), FilteredRouteCount: len(matching),
		Offset: offset, Limit: limit, HasMore: end < len(matching),
		SourceURL: cache.SourceURL, FetchedAt: fetchedAt,
	}
}

func normalizeCachedDiscoveryRoute(route domainrss.DiscoveryRoute) (domainrss.DiscoveryRoute, bool) {
	routePath := normalizeDiscoveryRouteTemplate("", route.RoutePath)
	if routePath == "" {
		return domainrss.DiscoveryRoute{}, false
	}
	examplePath := normalizeDiscoveryRoutePath(route.ExamplePath)
	if !concreteDiscoveryRoutePath(examplePath) {
		examplePath = ""
	}
	baseParameters := createDiscoveryParameters(routePath, nil, examplePath)
	existingParameters := make(map[string]domainrss.DiscoveryParameter, min(len(route.Parameters), maxDiscoveryParameters))
	for index, parameter := range route.Parameters {
		if index >= maxDiscoveryParameters {
			break
		}
		existingParameters[parameter.Name] = parameter
	}
	for index := range baseParameters {
		existing, ok := existingParameters[baseParameters[index].Name]
		if !ok {
			continue
		}
		baseParameters[index].Description = limitString(existing.Description, 4096)
		baseParameters[index].DefaultValue = existing.DefaultValue
		baseParameters[index].Type = strings.TrimSpace(existing.Type)
		if baseParameters[index].Type == "" {
			baseParameters[index].Type = "string"
		}
		options := existing.Options
		if len(options) > maxDiscoveryParameterOptions {
			options = options[:maxDiscoveryParameterOptions]
		}
		baseParameters[index].Options = make([]domainrss.DiscoveryParameterOption, 0, len(options))
		for _, option := range options {
			baseParameters[index].Options = append(baseParameters[index].Options, domainrss.DiscoveryParameterOption{
				Value: limitString(option.Value, maxDiscoveryParameterValueBytes), Label: limitString(option.Label, 512),
			})
		}
		if baseParameters[index].Options == nil {
			baseParameters[index].Options = []domainrss.DiscoveryParameterOption{}
		}
	}
	route.RoutePath = routePath
	route.ExamplePath = examplePath
	route.Parameters = baseParameters
	route.NeedsParameters = len(baseParameters) > 0
	route.URL = canonicalDiscoveryTemplateURL(routePath)
	if route.URL == "" {
		return domainrss.DiscoveryRoute{}, false
	}
	route.ID = discoveryRouteID(route.URL)
	route.Provider = "rsshub"
	route.SourceURL = homepageURL(route.SourceURL)
	route.SiteURL = homepageURL(route.SiteURL)
	// Never trust a cached icon URL independently of the catalog homepage. The
	// desktop projection exposes only an opaque local route; the resource
	// resolver derives the upstream favicon again from these sanitized fields.
	route.IconURL = discoveryFaviconURL(route.SiteURL, route.SourceURL)
	route.Title = displayText(route.Title, discoveryRouteTitle(routePath))
	route.SourceID = displayText(route.SourceID, discoverySourceID(routePath))
	route.SourceName = displayText(route.SourceName, route.SourceID)
	route.Description = cleanDiscoveryDescription(route.Description)
	route.Categories = normalizeStringCategories(route.Categories)
	if len(route.Categories) == 0 {
		route.Categories = []string{"other"}
	}
	route.Heat = max(route.Heat, 0)
	route.Language = normalizeDiscoveryLanguage(route.Language)
	if route.Language == "" {
		route.Language = guessDiscoveryLanguage(route.SourceName, route.SourceURL, route.Title, route.URL)
	}
	if route.Region == "" {
		route.Region = discoveryRegion(route.Language, route.SourceName, route.SourceURL, route.Title, route.URL)
	}
	switch route.ViewType {
	case domainrss.ViewTypeAuto, domainrss.ViewTypeArticle, domainrss.ViewTypeSocial,
		domainrss.ViewTypeImage, domainrss.ViewTypeVideo:
	default:
		route.ViewType = guessDiscoveryViewType(route.Categories)
	}
	if explicitDiscoveryRoute(route) {
		return domainrss.DiscoveryRoute{}, false
	}
	return route, true
}

func normalizeStringCategories(values []string) []string {
	result := make([]string, 0, min(len(values), maxDiscoveryCategories))
	seen := make(map[string]struct{}, cap(result))
	for _, value := range values {
		if len(result) >= maxDiscoveryCategories {
			break
		}
		value = strings.TrimSpace(limitString(value, maxDiscoveryNameBytes))
		if value == "" || value == "popular" {
			continue
		}
		if _, duplicate := seen[value]; duplicate {
			continue
		}
		seen[value] = struct{}{}
		result = append(result, value)
	}
	return result
}

func createDiscoveryCategories(routes []domainrss.DiscoveryRoute) []domainrss.DiscoveryCategory {
	type categoryAccumulator struct {
		count              int
		examples           []string
		iconURL, iconLabel string
	}
	values := make(map[string]*categoryAccumulator)
	for _, route := range routes {
		for _, category := range route.Categories {
			entry := values[category]
			if entry == nil {
				entry = &categoryAccumulator{examples: make([]string, 0, 2)}
				values[category] = entry
			}
			entry.count++
			if len(entry.examples) < 2 && !slices.Contains(entry.examples, route.Title) {
				entry.examples = append(entry.examples, route.Title)
			}
			if entry.iconURL == "" {
				entry.iconURL = firstNonEmpty(route.IconURL, discoveryFaviconURL(route.SiteURL, route.SourceURL))
				if entry.iconURL != "" {
					entry.iconLabel = firstNonEmpty(route.SourceName, route.Title)
				}
			}
		}
	}
	all := domainrss.DiscoveryCategory{ID: "all", Count: len(routes), Examples: make([]string, 0, 2)}
	for _, route := range routes {
		if len(all.Examples) < 2 {
			all.Examples = append(all.Examples, route.Title)
		}
		if all.IconURL == "" {
			all.IconURL = firstNonEmpty(route.IconURL, discoveryFaviconURL(route.SiteURL, route.SourceURL))
			if all.IconURL != "" {
				all.IconLabel = firstNonEmpty(route.SourceName, route.Title)
			}
		}
	}
	ids := make([]string, 0, len(values))
	for id := range values {
		ids = append(ids, id)
	}
	sort.Slice(ids, func(left, right int) bool {
		leftOrder, rightOrder := discoveryCategoryRank(ids[left]), discoveryCategoryRank(ids[right])
		if leftOrder != rightOrder {
			return leftOrder < rightOrder
		}
		if values[ids[left]].count != values[ids[right]].count {
			return values[ids[left]].count > values[ids[right]].count
		}
		return ids[left] < ids[right]
	})
	result := make([]domainrss.DiscoveryCategory, 0, len(ids)+1)
	result = append(result, all)
	for _, id := range ids {
		entry := values[id]
		result = append(result, domainrss.DiscoveryCategory{
			ID: id, Count: entry.count, Examples: entry.examples,
			IconURL: entry.iconURL, IconLabel: entry.iconLabel,
		})
	}
	return result
}

func discoveryRouteMatchesQuery(route domainrss.DiscoveryRoute, query string) bool {
	if query == "" {
		return true
	}
	return discoveryRouteSearchScore(route, query) > 0
}

func discoverySearchTokens(value string) []string {
	parts := strings.FieldsFunc(normalizeDiscoverySearch(value), func(r rune) bool {
		return !(unicode.IsLetter(r) || unicode.IsDigit(r))
	})
	result := make([]string, 0, min(len(parts), 12))
	seen := make(map[string]struct{}, cap(result))
	for _, part := range parts {
		if part == "" {
			continue
		}
		if _, duplicate := seen[part]; duplicate {
			continue
		}
		seen[part] = struct{}{}
		result = append(result, part)
		if len(result) == 12 {
			break
		}
	}
	return result
}

func discoveryRouteSearchScore(route domainrss.DiscoveryRoute, query string) int {
	query = normalizeDiscoverySearch(query)
	if query == "" {
		return 1
	}
	fields := []struct {
		value  string
		weight int
	}{
		{route.Title, 12},
		{route.SourceName, 10},
		{route.RoutePath, 7},
		{route.URL, 6},
		{route.Description, 4},
		{route.SourceURL, 3},
		{route.SiteURL, 3},
	}
	for _, category := range route.Categories {
		fields = append(fields, struct {
			value  string
			weight int
		}{category, 4})
	}
	tokens := discoverySearchTokens(query)
	if len(tokens) == 0 {
		return 0
	}
	score := 0
	matchedTokens := 0
	for _, token := range tokens {
		best := 0
		for _, field := range fields {
			normalized := normalizeDiscoverySearch(field.value)
			if !strings.Contains(normalized, token) {
				continue
			}
			candidate := field.weight
			if slices.Contains(discoverySearchTokens(normalized), token) {
				candidate += field.weight
			}
			best = max(best, candidate)
		}
		if best > 0 {
			matchedTokens++
			score += best
		}
	}
	for _, field := range fields {
		if strings.Contains(normalizeDiscoverySearch(field.value), query) {
			score += field.weight * 8
		}
	}
	if matchedTokens == len(tokens) {
		score += 48
	}
	return score
}

func sortDiscoveryRoutesForQuery(routes []domainrss.DiscoveryRoute, query, mode string) {
	if query == "" || strings.EqualFold(strings.TrimSpace(mode), "title") {
		sortDiscoveryRoutes(routes, mode)
		return
	}
	sort.SliceStable(routes, func(left, right int) bool {
		leftScore := discoveryRouteSearchScore(routes[left], query)
		rightScore := discoveryRouteSearchScore(routes[right], query)
		if leftScore != rightScore {
			return leftScore > rightScore
		}
		if routes[left].Heat != routes[right].Heat {
			return routes[left].Heat > routes[right].Heat
		}
		leftTitle, rightTitle := strings.ToLower(routes[left].Title), strings.ToLower(routes[right].Title)
		if leftTitle != rightTitle {
			return leftTitle < rightTitle
		}
		return routes[left].ID < routes[right].ID
	})
}

func sortDiscoveryRoutes(routes []domainrss.DiscoveryRoute, mode string) {
	titleSort := strings.EqualFold(strings.TrimSpace(mode), "title")
	sort.SliceStable(routes, func(left, right int) bool {
		if !titleSort && routes[left].Heat != routes[right].Heat {
			return routes[left].Heat > routes[right].Heat
		}
		leftTitle, rightTitle := strings.ToLower(routes[left].Title), strings.ToLower(routes[right].Title)
		if leftTitle != rightTitle {
			return leftTitle < rightTitle
		}
		return routes[left].ID < routes[right].ID
	})
}

func appendUniqueDiscoveryRoute(routes *[]domainrss.DiscoveryRoute, seen map[string]struct{}, route domainrss.DiscoveryRoute) {
	if explicitDiscoveryRoute(route) {
		return
	}
	key := strings.ToLower(route.ID)
	if _, exists := seen[key]; exists {
		return
	}
	seen[key] = struct{}{}
	*routes = append(*routes, route)
}

type discoveryTemplateParameter struct {
	Name     string
	Optional bool
	CatchAll bool
}

type discoveryTemplatePart struct {
	Literal   string
	Parameter *discoveryTemplateParameter
}

func normalizeDiscoveryRouteTemplate(namespace, route string) string {
	route = strings.TrimSpace(route)
	if namespace = strings.Trim(strings.TrimSpace(namespace), "/"); namespace != "" {
		route = namespace + "/" + strings.TrimLeft(route, "/")
	}
	route = normalizeDiscoveryRoutePath(route)
	if route == "" {
		return ""
	}
	parts := splitDiscoveryRoutePath(route)
	if len(parts) == 0 || len(parts) > maxDiscoveryTemplateSegments {
		return ""
	}
	wildcardIndex := 0
	for index, part := range parts {
		if part != "*" {
			continue
		}
		wildcardIndex++
		name := "catchAll"
		if wildcardIndex > 1 {
			name += strconv.Itoa(wildcardIndex)
		}
		parts[index] = ":" + name + "{.+}"
	}
	normalized := strings.Join(parts, "/")
	parameterCount, catchAllCount := 0, 0
	for _, part := range parseDiscoveryTemplate(normalized) {
		if part.Parameter == nil {
			continue
		}
		parameterCount++
		if part.Parameter.CatchAll {
			catchAllCount++
		}
	}
	if parameterCount > maxDiscoveryParameters || catchAllCount > maxDiscoveryCatchAllParameters {
		return ""
	}
	return normalized
}

func splitDiscoveryRoutePath(path string) []string {
	path = strings.Trim(strings.TrimSpace(path), "/")
	if path == "" {
		return nil
	}
	parts := make([]string, 0, strings.Count(path, "/")+1)
	start, braceDepth := 0, 0
	for index, character := range path {
		switch character {
		case '{':
			braceDepth++
		case '}':
			if braceDepth > 0 {
				braceDepth--
			}
		case '/':
			if braceDepth == 0 {
				if part := path[start:index]; part != "" {
					parts = append(parts, part)
				}
				start = index + 1
			}
		}
	}
	if start < len(path) {
		parts = append(parts, path[start:])
	}
	return parts
}

func parseDiscoveryTemplate(path string) []discoveryTemplatePart {
	segments := splitDiscoveryRoutePath(path)
	if len(segments) > maxDiscoveryTemplateSegments {
		return nil
	}
	parts := make([]discoveryTemplatePart, 0, len(segments))
	for _, segment := range segments {
		if !strings.HasPrefix(segment, ":") {
			parts = append(parts, discoveryTemplatePart{Literal: segment})
			continue
		}
		nameEnd := 1
		for nameEnd < len(segment) {
			character := segment[nameEnd]
			if (character >= 'a' && character <= 'z') || (character >= 'A' && character <= 'Z') ||
				(character >= '0' && character <= '9') || character == '_' || character == '-' {
				nameEnd++
				continue
			}
			break
		}
		if nameEnd == 1 {
			parts = append(parts, discoveryTemplatePart{Literal: segment})
			continue
		}
		parts = append(parts, discoveryTemplatePart{Parameter: &discoveryTemplateParameter{
			Name: segment[1:nameEnd], Optional: strings.HasSuffix(segment, "?"),
			CatchAll: strings.Contains(segment[nameEnd:], "{"),
		}})
	}
	return parts
}

func createDiscoveryParameters(routePath string, raw any, examplePath string) []domainrss.DiscoveryParameter {
	metadata, _ := raw.(map[string]any)
	exampleValues := matchDiscoveryExample(routePath, examplePath)
	template := parseDiscoveryTemplate(routePath)
	parameters := make([]domainrss.DiscoveryParameter, 0, min(len(template), maxDiscoveryParameters))
	usedMetadata := make(map[string]struct{})
	for _, part := range template {
		if len(parameters) >= maxDiscoveryParameters {
			break
		}
		if part.Parameter == nil {
			continue
		}
		specification := *part.Parameter
		value, exists := metadata[specification.Name]
		if exists {
			usedMetadata[specification.Name] = struct{}{}
		} else if specification.CatchAll && len(metadata) <= maxDiscoveryParameters {
			// RSSHub uses bare `*` for a handful of routes. When it provides one
			// unambiguous declaration (for example `product`), reuse its help text
			// while keeping the stable synthetic catchAll path name.
			var candidate any
			candidateCount := 0
			for name, item := range metadata {
				if _, used := usedMetadata[name]; used {
					continue
				}
				candidate, candidateCount = item, candidateCount+1
			}
			if candidateCount == 1 {
				value = candidate
			}
		}
		parameter := discoveryParameterFromMetadata(specification, value)
		parameter.ExampleValue = exampleValues[specification.Name]
		parameters = append(parameters, parameter)
	}
	return parameters
}

func discoveryParameterFromMetadata(specification discoveryTemplateParameter, value any) domainrss.DiscoveryParameter {
	parameter := domainrss.DiscoveryParameter{
		Name: specification.Name, Optional: specification.Optional, CatchAll: specification.CatchAll,
		Type: "string", Options: make([]domainrss.DiscoveryParameterOption, 0),
	}
	switch typed := value.(type) {
	case string:
		parameter.Description = strings.TrimSpace(limitString(typed, 4096))
	case map[string]any:
		parameter.Description = strings.TrimSpace(limitString(stringValue(typed["description"]), 4096))
		if defaultValue, exists := typed["default"]; exists {
			if text, ok := defaultValue.(string); ok {
				text = limitString(text, 2048)
				parameter.DefaultValue = &text
			}
		}
		if optional, exists := typed["optional"].(bool); exists {
			parameter.Optional = optional
		}
		if parameterType := strings.TrimSpace(stringValue(typed["type"])); parameterType != "" {
			parameter.Type = limitString(parameterType, 64)
		}
		if options, ok := typed["options"].([]any); ok {
			for index, item := range options {
				if index >= maxDiscoveryParameterOptions {
					break
				}
				option, ok := item.(map[string]any)
				if !ok {
					continue
				}
				optionValue, valueOK := option["value"].(string)
				optionLabel, labelOK := option["label"].(string)
				if !valueOK || !labelOK {
					continue
				}
				parameter.Options = append(parameter.Options, domainrss.DiscoveryParameterOption{
					Value: limitString(optionValue, 2048), Label: limitString(optionLabel, 512),
				})
			}
		}
	}
	return parameter
}

func matchDiscoveryExample(routePath, examplePath string) map[string]string {
	values := make(map[string]string)
	if routePath == "" || !concreteDiscoveryRoutePath(examplePath) {
		return values
	}
	template := parseDiscoveryTemplate(routePath)
	exampleRoutePath, _, _ := strings.Cut(examplePath, "?")
	example := splitDiscoveryRoutePath(exampleRoutePath)
	if len(template) == 0 || len(template) > maxDiscoveryTemplateSegments || len(example) > maxDiscoveryTemplateSegments {
		return values
	}
	type matchState struct{ templateIndex, exampleIndex int }
	memo := make(map[matchState]bool, (len(template)+1)*(len(example)+1))
	visited := make(map[matchState]bool, len(memo))
	choices := make(map[matchState]int, len(template))
	var match func(int, int) bool
	match = func(templateIndex, exampleIndex int) bool {
		state := matchState{templateIndex: templateIndex, exampleIndex: exampleIndex}
		if visited[state] {
			return memo[state]
		}
		visited[state] = true
		if templateIndex == len(template) {
			memo[state] = exampleIndex == len(example)
			return memo[state]
		}
		part := template[templateIndex]
		if part.Parameter == nil {
			if exampleIndex >= len(example) || part.Literal != example[exampleIndex] {
				memo[state] = false
				return false
			}
			memo[state] = match(templateIndex+1, exampleIndex+1)
			return memo[state]
		}
		parameter := part.Parameter
		minimum := 1
		if parameter.Optional {
			minimum = 0
		}
		maximum := minimum
		if parameter.CatchAll {
			maximum = len(example) - exampleIndex
		} else if exampleIndex < len(example) {
			maximum = 1
		}
		for consumed := maximum; consumed >= minimum; consumed-- {
			if exampleIndex+consumed > len(example) {
				continue
			}
			if match(templateIndex+1, exampleIndex+consumed) {
				choices[state] = consumed
				memo[state] = true
				return true
			}
		}
		memo[state] = false
		return false
	}
	if !match(0, 0) {
		return map[string]string{}
	}
	for templateIndex, exampleIndex := 0, 0; templateIndex < len(template); templateIndex++ {
		part := template[templateIndex]
		if part.Parameter == nil {
			exampleIndex++
			continue
		}
		state := matchState{templateIndex: templateIndex, exampleIndex: exampleIndex}
		consumed := choices[state]
		if consumed > 0 {
			segments := make([]string, 0, consumed)
			for _, segment := range example[exampleIndex : exampleIndex+consumed] {
				if decoded, err := url.PathUnescape(segment); err == nil {
					segment = decoded
				}
				segments = append(segments, segment)
			}
			values[part.Parameter.Name] = limitString(strings.Join(segments, "/"), maxDiscoveryParameterValueBytes)
		}
		exampleIndex += consumed
	}
	return values
}

func explicitDiscoveryRoute(route domainrss.DiscoveryRoute) bool {
	if _, blocked := discoveryExplicitSources[strings.ToLower(strings.TrimSpace(route.SourceID))]; blocked {
		return true
	}
	text := strings.ToLower(strings.Join([]string{
		route.Title, route.Description, route.SourceName, route.SourceURL,
		route.SiteURL, route.URL, route.RoutePath,
	}, " "))
	for _, marker := range []string{"porn", "nsfw", "adult", "hentai", "onlyfans", "fansly", "18禁"} {
		if strings.Contains(text, marker) {
			return true
		}
	}
	return discoveryExplicitToken.MatchString(text)
}

func normalizeDiscoveryRoutePath(value string) string {
	value = strings.TrimSpace(value)
	if len(value) > 4096 {
		return ""
	}
	if strings.HasPrefix(strings.ToLower(value), RSSHubScheme) {
		value = value[len(RSSHubScheme):]
	}
	value = strings.TrimLeft(value, "/")
	if value == "" || strings.Contains(value, "://") || strings.Contains(value, "..") {
		return ""
	}
	value = strings.TrimRight(value, "/")
	if len(splitDiscoveryRoutePath(value)) > maxDiscoveryTemplateSegments {
		return ""
	}
	return value
}

func concreteDiscoveryRoutePath(value string) bool {
	value = normalizeDiscoveryRoutePath(value)
	return value != "" && !strings.Contains(value, "{") && !strings.Contains(value, "}") &&
		!discoveryTemplatePattern.MatchString(value)
}

func canonicalDiscoveryURL(path string) string {
	path = normalizeDiscoveryRoutePath(path)
	if !concreteDiscoveryRoutePath(path) {
		return ""
	}
	return RSSHubScheme + path
}

func canonicalDiscoveryTemplateURL(path string) string {
	path = normalizeDiscoveryRoutePath(path)
	if path == "" {
		return ""
	}
	return RSSHubScheme + path
}

func firstConcreteDiscoveryRoute(value string) string {
	for _, match := range discoveryRSSHubURLPattern.FindAllStringSubmatch(value, -1) {
		if len(match) > 1 {
			candidate := cleanDiscoveryCandidate(match[1])
			if concreteDiscoveryRoutePath(candidate) {
				return normalizeDiscoveryRoutePath(candidate)
			}
		}
	}
	for _, match := range discoveryRoutePattern.FindAllString(value, -1) {
		candidate := cleanDiscoveryCandidate(match)
		if concreteDiscoveryRoutePath(candidate) {
			return normalizeDiscoveryRoutePath(candidate)
		}
	}
	return ""
}

func cleanDiscoveryCandidate(value string) string {
	return strings.TrimRight(strings.TrimLeft(strings.TrimSpace(value), "/"), ".,;:!?")
}

func normalizeDiscoveryCategories(value any, fallback []string) []string {
	items, ok := value.([]any)
	if !ok {
		return normalizeStringCategories(fallback)
	}
	result := make([]string, 0, min(len(items), maxDiscoveryCategories))
	seen := make(map[string]struct{}, cap(result))
	for _, item := range items {
		if len(result) >= maxDiscoveryCategories {
			break
		}
		category := strings.TrimSpace(limitString(stringValue(item), maxDiscoveryNameBytes))
		if category == "" || category == "popular" {
			continue
		}
		if _, duplicate := seen[category]; duplicate {
			continue
		}
		seen[category] = struct{}{}
		result = append(result, category)
	}
	if len(result) == 0 {
		return normalizeStringCategories(fallback)
	}
	return result
}

func normalizeDiscoveryLanguage(value string) string {
	value = strings.ToLower(strings.ReplaceAll(strings.TrimSpace(value), "_", "-"))
	if value == "" || value == "all" {
		return ""
	}
	switch {
	case value == "eng" || value == "en" || strings.HasPrefix(value, "en-"):
		return "en"
	case value == "cmn" || value == "zh" || value == "zh-cn":
		return "zh-CN"
	case value == "zh-tw":
		return "zh-TW"
	case value == "zh-hk":
		return "zh-HK"
	case value == "fra" || value == "fr" || value == "fr-fr":
		return "fr-FR"
	case value == "jpn" || value == "ja" || value == "ja-jp":
		return "ja"
	case value == "kor" || value == "ko" || value == "ko-kr":
		return "ko"
	case value == "deu" || value == "ger" || value == "de" || value == "de-de":
		return "de"
	case value == "rus" || value == "ru" || value == "ru-ru":
		return "ru"
	case value == "spa" || value == "es" || value == "es-es":
		return "es"
	default:
		return value
	}
}

func guessDiscoveryViewType(categories []string) domainrss.ViewType {
	if slices.Contains(categories, "picture") {
		return domainrss.ViewTypeImage
	}
	if slices.Contains(categories, "live") || slices.Contains(categories, "multimedia") {
		return domainrss.ViewTypeVideo
	}
	if slices.Contains(categories, "social-media") || slices.Contains(categories, "bbs") {
		return domainrss.ViewTypeSocial
	}
	return domainrss.ViewTypeArticle
}

func guessDiscoveryLanguage(values ...string) string {
	text := strings.ToLower(strings.Join(values, " "))
	if discoveryChinesePattern.MatchString(text) {
		return "zh-CN"
	}
	return "en"
}

func discoveryRegion(language string, values ...string) string {
	regions := map[string]string{
		"zh-CN": "CN", "zh-TW": "TW", "zh-HK": "HK", "fr-FR": "FR", "ja": "JP",
		"ko": "KR", "de": "DE", "ru": "RU", "es": "ES",
	}
	if region := regions[language]; region != "" {
		return region
	}
	if guessDiscoveryLanguage(values...) == "zh-CN" {
		return "CN"
	}
	return "global"
}

func guessDiscoveryCategory(routePath, title string) string {
	text := strings.ToLower(routePath + " " + title)
	switch {
	case containsAny(text, "twitter", "x/", "weibo", "telegram", "reddit", "mastodon", "discord", "threads", "facebook", "instagram"):
		return "social-media"
	case containsAny(text, "youtube", "bilibili", "vimeo", "tiktok", "podcast", "spotify", "music", "video", "tv", "radio"):
		return "multimedia"
	case containsAny(text, "github", "npm", "pypi", "v2ex", "solidot", "program", "developer", "dev", "code"):
		return "programming"
	case containsAny(text, "bbc", "reuters", "apnews", "thepaper", "zaobao", "news", "daily", "press", "media", "36kr"):
		return "traditional-media"
	case containsAny(text, "douban", "goodreads", "book", "reading", "sspai"):
		return "reading"
	case containsAny(text, "finance", "stock", "fund", "crypto", "money", "wallstreet"):
		return "finance"
	case containsAny(text, "weather", "forecast", "typhoon"):
		return "forecast"
	case containsAny(text, "gov", "government"):
		return "government"
	default:
		return "other"
	}
}

func cleanDiscoveryDescription(value string) string {
	value = strings.ReplaceAll(value, "Powered by RSSHub", "")
	value = strings.ReplaceAll(value, "powered by rsshub", "")
	value = discoveryMarkdownLink.ReplaceAllString(value, "$1")
	value = discoveryMarkdownCode.ReplaceAllString(value, "$1")
	value = strings.NewReplacer("#", " ", "*", " ", "_", " ", ">", " ", "|", " ").Replace(value)
	value = html.UnescapeString(stripMarkup(value))
	value = strings.Join(strings.Fields(value), " ")
	if len([]rune(value)) <= 180 {
		return value
	}
	runes := []rune(value)
	return strings.TrimSpace(string(runes[:177])) + "..."
}

func formatDiscoveryParameters(value any) string {
	values, ok := value.(map[string]any)
	if !ok {
		return ""
	}
	keys := sortedMapKeys(values)
	parts := make([]string, 0, len(keys))
	for index, key := range keys {
		if index >= maxDiscoveryParameters {
			break
		}
		parts = append(parts, limitString(key, maxDiscoveryNameBytes)+": "+limitString(stringValue(values[key]), 4096))
	}
	return strings.Join(parts, " ")
}

func discoveryRouteID(canonical string) string {
	value := strings.TrimPrefix(strings.ToLower(canonical), RSSHubScheme)
	var builder strings.Builder
	lastDash := false
	for _, character := range value {
		if unicode.IsLetter(character) || unicode.IsDigit(character) {
			builder.WriteRune(character)
			lastDash = false
		} else if !lastDash {
			builder.WriteByte('-')
			lastDash = true
		}
	}
	slug := strings.Trim(builder.String(), "-")
	if slug == "" {
		slug = "route"
	}
	slugRunes := []rune(slug)
	if len(slugRunes) > 120 {
		slug = string(slugRunes[:120])
	}
	digest := sha256.Sum256([]byte(canonical))
	return "rsshub:" + slug + "-" + hex.EncodeToString(digest[:6])
}

func discoveryRouteTitle(path string) string {
	parts := strings.Split(normalizeDiscoveryRoutePath(path), "/")
	if len(parts) > 2 {
		parts = parts[:2]
	}
	return strings.Join(parts, " / ")
}

func discoverySourceID(path string) string {
	part := strings.Split(normalizeDiscoveryRoutePath(path), "/")[0]
	var result strings.Builder
	for _, character := range strings.ToLower(part) {
		if (character >= 'a' && character <= 'z') || unicode.IsDigit(character) || character == '_' || character == '-' {
			result.WriteRune(character)
		}
	}
	if result.Len() == 0 {
		return "rsshub"
	}
	return result.String()
}

func homepageURL(value string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return ""
	}
	if !strings.HasPrefix(strings.ToLower(value), "http://") && !strings.HasPrefix(strings.ToLower(value), "https://") {
		value = "https://" + strings.TrimLeft(value, "/")
	}
	parsed, err := networkpolicy.ValidatePublicHTTPURL(value)
	if err != nil {
		return ""
	}
	parsed.RawQuery, parsed.Fragment = "", ""
	if parsed.Path == "/" {
		parsed.Path = ""
	}
	return parsed.String()
}

func discoveryFaviconURL(values ...string) string {
	for _, value := range values {
		parsed, err := networkpolicy.ValidatePublicHTTPURL(strings.TrimSpace(value))
		if err == nil {
			parsed.Path = "/favicon.ico"
			parsed.RawPath, parsed.RawQuery, parsed.Fragment = "", "", ""
			return parsed.String()
		}
	}
	return ""
}

func displayText(value any, fallback string) string {
	text := strings.Join(strings.Fields(stringValue(value)), " ")
	if text == "" {
		text = fallback
	}
	runes := []rune(text)
	if len(runes) > 512 {
		return string(runes[:512])
	}
	return text
}

func stringValue(value any) string {
	switch typed := value.(type) {
	case string:
		return typed
	case json.Number:
		return typed.String()
	case float64:
		return strconv.FormatFloat(typed, 'f', -1, 64)
	case nil:
		return ""
	default:
		return ""
	}
}

func boolValue(value any) bool {
	result, _ := value.(bool)
	return result
}

func numberAsInt(value any) int {
	switch typed := value.(type) {
	case json.Number:
		parsed, _ := typed.Int64()
		return int(parsed)
	case float64:
		return int(typed)
	case int:
		return typed
	default:
		return 0
	}
}

func sortedMapKeys[T any](values map[string]T) []string {
	keys := make([]string, 0, len(values))
	for key := range values {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	return keys
}

func normalizeDiscoverySearch(value string) string { return strings.ToLower(strings.TrimSpace(value)) }

func discoveryCategoryRank(value string) int {
	if index := slices.Index(discoveryCategoryOrder, value); index >= 0 {
		return index
	}
	return len(discoveryCategoryOrder) + 1
}

func containsAny(value string, candidates ...string) bool {
	for _, candidate := range candidates {
		if strings.Contains(value, candidate) {
			return true
		}
	}
	return false
}
