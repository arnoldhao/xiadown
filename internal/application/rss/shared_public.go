package rss

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/url"
	"sort"
	"strings"
	"time"
	"unicode"
	"unicode/utf8"

	"xiadown/internal/application/networkpolicy"
	domainrss "xiadown/internal/domain/rss"

	"github.com/google/uuid"
	"golang.org/x/text/unicode/norm"
)

const (
	maxSharedPublicObservationEntries = 200
	maxSharedPublicValidatorBytes     = 1024
	maxSharedPublicGUIDBytes          = 32 << 10
	maxSharedPublicMutationFields     = 8
	defaultSharedPublicLeaseTTL       = 2 * time.Minute
	sharedPublicDNSValidationTimeout  = 3 * time.Second
)

type SharedSubscriptionMutationRequest struct {
	MutationID       string                                  `json:"mutationId"`
	Operation        domainrss.SubscriptionMutationOperation `json:"operation"`
	ExpectedRevision *int64                                  `json:"expectedRevision,omitempty"`
	FieldMask        []string                                `json:"fieldMask,omitempty"`
	Title            string                                  `json:"title,omitempty"`
	ViewType         string                                  `json:"viewType,omitempty"`
	CategoryID       *string                                 `json:"categoryId,omitempty"`
	SortOrder        *int                                    `json:"sortOrder,omitempty"`
	Enabled          *bool                                   `json:"enabled,omitempty"`
	SourceAccess     domainrss.SubscriptionSourceAccess      `json:"sourceAccess,omitempty"`
	PublicFeedURL    string                                  `json:"publicFeedURL,omitempty"`
}

type FeedObservationRequest struct {
	MutationID   string                       `json:"mutationId"`
	UpstreamETag string                       `json:"upstreamETag,omitempty"`
	LastModified string                       `json:"lastModified,omitempty"`
	FetchedAt    time.Time                    `json:"fetchedAt"`
	ContentHash  string                       `json:"contentHash"`
	Entries      []domainrss.ObservationEntry `json:"entries"`
}

type FetchLeaseApplicationRequest struct {
	TTLSeconds int `json:"ttlSeconds,omitempty"`
}

func (service *Service) MutateSubscriptionForDevice(
	ctx context.Context,
	deviceID, subscriptionID string,
	request SharedSubscriptionMutationRequest,
) (domainrss.SubscriptionMutationResult, error) {
	if service == nil {
		return domainrss.SubscriptionMutationResult{}, errorsUnavailableSharedPublicRepository()
	}
	repository, ok := service.repository.(domainrss.SharedPublicRepository)
	if !ok {
		return domainrss.SubscriptionMutationResult{}, errorsUnavailableSharedPublicRepository()
	}
	deviceID = strings.TrimSpace(deviceID)
	mutationID, mutationOK := canonicalUUID(request.MutationID)
	subscriptionID, subscriptionOK := canonicalUUID(subscriptionID)
	if deviceID == "" || !mutationOK || !subscriptionOK {
		return domainrss.SubscriptionMutationResult{}, invalidRSSRequest("invalid RSS subscription mutation identity")
	}
	now := service.now().UTC()
	mutation := domainrss.SubscriptionMutation{
		DeviceID: deviceID, MutationID: mutationID, Operation: request.Operation,
		SubscriptionID: subscriptionID, ExpectedRevision: request.ExpectedRevision,
		Title: strings.TrimSpace(request.Title), CategoryID: request.CategoryID,
		SortOrder: request.SortOrder, Enabled: request.Enabled,
		SourceAccess: request.SourceAccess, ChangedAt: now,
	}
	if request.CategoryID != nil {
		value := strings.TrimSpace(*request.CategoryID)
		if !validSharedPublicText(value, 255, true) {
			return domainrss.SubscriptionMutationResult{}, invalidRSSRequest("invalid RSS category identity")
		}
		mutation.CategoryID = &value
		if value != "" {
			if service.organizationRepository == nil {
				return domainrss.SubscriptionMutationResult{}, errorsUnavailableSharedPublicRepository()
			}
			if _, err := service.organizationRepository.GetCategory(ctx, value); err != nil {
				return domainrss.SubscriptionMutationResult{}, err
			}
		}
	}
	if !validSharedPublicText(mutation.Title, 512, true) {
		return domainrss.SubscriptionMutationResult{}, invalidRSSRequest("invalid RSS subscription title")
	}
	if request.SortOrder != nil && (*request.SortOrder < -1_000_000 || *request.SortOrder > 1_000_000) {
		return domainrss.SubscriptionMutationResult{}, invalidRSSRequest("invalid RSS subscription sort order")
	}
	view, err := normalizeViewType(request.ViewType)
	if err != nil {
		return domainrss.SubscriptionMutationResult{}, invalidRSSRequest("invalid RSS subscription view type")
	}
	mutation.ViewType = view

	switch request.Operation {
	case domainrss.SubscriptionMutationAdd, domainrss.SubscriptionMutationPromote:
		if request.ExpectedRevision != nil || len(request.FieldMask) != 0 || request.SourceAccess != domainrss.SubscriptionSourceSharedPublic {
			return domainrss.SubscriptionMutationResult{}, invalidRSSRequest("invalid shared-public RSS creation")
		}
		mutation.PublicFeedURL, err = service.validateSharedPublicFeedURL(ctx, request.PublicFeedURL)
		if err != nil {
			return domainrss.SubscriptionMutationResult{}, err
		}
	case domainrss.SubscriptionMutationUpdate:
		if request.ExpectedRevision == nil || *request.ExpectedRevision < 1 || len(request.FieldMask) == 0 || len(request.FieldMask) > maxSharedPublicMutationFields {
			return domainrss.SubscriptionMutationResult{}, invalidRSSRequest("invalid RSS subscription update revision")
		}
		if request.SourceAccess != "" {
			return domainrss.SubscriptionMutationResult{}, invalidRSSRequest("sourceAccess requires explicit Desktop confirmation")
		}
		mutation.FieldMask, err = normalizeSubscriptionFieldMask(request.FieldMask)
		if err != nil {
			return domainrss.SubscriptionMutationResult{}, err
		}
		if containsString(mutation.FieldMask, "viewType") && strings.TrimSpace(request.ViewType) == "" {
			return domainrss.SubscriptionMutationResult{}, invalidRSSRequest("viewType is required by the RSS field mask")
		}
		if containsString(mutation.FieldMask, "title") && mutation.Title == "" {
			return domainrss.SubscriptionMutationResult{}, invalidRSSRequest("title is required by the RSS field mask")
		}
		if containsString(mutation.FieldMask, "publicFeedURL") {
			mutation.PublicFeedURL, err = service.validateSharedPublicFeedURL(ctx, request.PublicFeedURL)
			if err != nil {
				return domainrss.SubscriptionMutationResult{}, err
			}
		}
	case domainrss.SubscriptionMutationDelete:
		if request.ExpectedRevision == nil || *request.ExpectedRevision < 1 || len(request.FieldMask) != 0 || request.SourceAccess != "" {
			return domainrss.SubscriptionMutationResult{}, invalidRSSRequest("invalid RSS subscription delete revision")
		}
	default:
		return domainrss.SubscriptionMutationResult{}, invalidRSSRequest("invalid RSS subscription mutation operation")
	}
	mutation.RequestHash, err = canonicalSHA256(struct {
		Operation        domainrss.SubscriptionMutationOperation `json:"operation"`
		SubscriptionID   string                                  `json:"subscriptionId"`
		ExpectedRevision *int64                                  `json:"expectedRevision,omitempty"`
		FieldMask        []string                                `json:"fieldMask,omitempty"`
		Title            string                                  `json:"title,omitempty"`
		ViewType         domainrss.ViewType                      `json:"viewType,omitempty"`
		CategoryID       *string                                 `json:"categoryId,omitempty"`
		SortOrder        *int                                    `json:"sortOrder,omitempty"`
		Enabled          *bool                                   `json:"enabled,omitempty"`
		SourceAccess     domainrss.SubscriptionSourceAccess      `json:"sourceAccess,omitempty"`
		PublicFeedURL    string                                  `json:"publicFeedURL,omitempty"`
	}{
		mutation.Operation, mutation.SubscriptionID, mutation.ExpectedRevision, mutation.FieldMask,
		mutation.Title, mutation.ViewType, mutation.CategoryID, mutation.SortOrder, mutation.Enabled,
		mutation.SourceAccess, mutation.PublicFeedURL,
	})
	if err != nil {
		return domainrss.SubscriptionMutationResult{}, err
	}
	return repository.ApplySubscriptionMutation(ctx, mutation)
}

func (service *Service) SubmitFeedObservationForDevice(
	ctx context.Context,
	deviceID, subscriptionID string,
	request FeedObservationRequest,
) (domainrss.ObservationResult, error) {
	if service == nil {
		return domainrss.ObservationResult{}, errorsUnavailableSharedPublicRepository()
	}
	repository, ok := service.repository.(domainrss.SharedPublicRepository)
	if !ok || service.syncRepository == nil {
		return domainrss.ObservationResult{}, errorsUnavailableSharedPublicRepository()
	}
	deviceID = strings.TrimSpace(deviceID)
	mutationID, mutationOK := canonicalUUID(request.MutationID)
	subscriptionID, subscriptionOK := canonicalUUID(subscriptionID)
	if deviceID == "" || !mutationOK || !subscriptionOK || len(request.Entries) > maxSharedPublicObservationEntries {
		return domainrss.ObservationResult{}, invalidRSSRequest("invalid RSS observation identity or entry count")
	}
	now := service.now().UTC()
	fetchedAt := request.FetchedAt.UTC()
	if fetchedAt.IsZero() || fetchedAt.After(now.Add(5*time.Minute)) || fetchedAt.Before(now.Add(-90*24*time.Hour)) {
		return domainrss.ObservationResult{}, invalidRSSRequest("invalid RSS observation fetch time")
	}
	if !validFeedValidator(request.UpstreamETag) || !validFeedValidator(request.LastModified) || !validSHA256(request.ContentHash) {
		return domainrss.ObservationResult{}, invalidRSSRequest("invalid RSS observation validator")
	}
	subscription, err := service.syncRepository.GetSyncSubscription(ctx, subscriptionID)
	if err != nil {
		return domainrss.ObservationResult{}, err
	}
	if normalizeSyncSourceAccess(subscription.SourceAccess) != domainrss.SubscriptionSourceSharedPublic || strings.TrimSpace(subscription.PublicFeedURL) == "" {
		return domainrss.ObservationResult{}, invalidRSSRequest("RSS subscription is not shared public")
	}

	normalizedEntries := make([]domainrss.ObservationEntry, 0, len(request.Entries))
	parsedEntries := make([]parsedEntry, 0, len(request.Entries))
	seenOrigins := make(map[string]struct{}, len(request.Entries))
	for _, candidate := range request.Entries {
		normalized, parsed, normalizeErr := normalizeObservationEntry(candidate, subscriptionID, subscription.PublicFeedURL)
		if normalizeErr != nil {
			return domainrss.ObservationResult{}, normalizeErr
		}
		if _, duplicate := seenOrigins[normalized.OriginKey]; duplicate {
			return domainrss.ObservationResult{}, invalidRSSRequest("duplicate RSS observation origin key")
		}
		seenOrigins[normalized.OriginKey] = struct{}{}
		normalizedEntries = append(normalizedEntries, normalized)
		parsedEntries = append(parsedEntries, parsed)
	}
	canonicalEntries := entriesFromFeed(subscriptionID, subscription.ViewType, parsedFeed{
		SiteURL: subscription.PublicFeedURL, Entries: parsedEntries,
	}, fetchedAt)
	if len(canonicalEntries) != len(normalizedEntries) {
		return domainrss.ObservationResult{}, invalidRSSRequest("invalid RSS observation entries")
	}
	for index := range canonicalEntries {
		canonicalEntries[index].OriginKey = normalizedEntries[index].OriginKey
		canonicalEntries[index].ObservedAt = fetchedAt
	}
	observation := domainrss.FeedObservation{
		DeviceID: deviceID, MutationID: mutationID, SubscriptionID: subscriptionID,
		UpstreamETag: strings.TrimSpace(request.UpstreamETag), LastModified: strings.TrimSpace(request.LastModified),
		FetchedAt: fetchedAt, AcceptedAt: now, ContentHash: strings.ToLower(strings.TrimSpace(request.ContentHash)),
		Entries: normalizedEntries, CanonicalEntries: canonicalEntries,
	}
	observation.RequestHash, err = canonicalSHA256(struct {
		SubscriptionID string                       `json:"subscriptionId"`
		UpstreamETag   string                       `json:"upstreamETag,omitempty"`
		LastModified   string                       `json:"lastModified,omitempty"`
		FetchedAt      time.Time                    `json:"fetchedAt"`
		ContentHash    string                       `json:"contentHash"`
		Entries        []domainrss.ObservationEntry `json:"entries"`
	}{
		observation.SubscriptionID, observation.UpstreamETag, observation.LastModified,
		observation.FetchedAt, observation.ContentHash, observation.Entries,
	})
	if err != nil {
		return domainrss.ObservationResult{}, err
	}
	return repository.ApplyFeedObservation(ctx, observation)
}

func (service *Service) AcquireFetchLeaseForDevice(
	ctx context.Context,
	deviceID, subscriptionID string,
	request FetchLeaseApplicationRequest,
) (domainrss.FetchLeaseResult, error) {
	if service == nil {
		return domainrss.FetchLeaseResult{}, errorsUnavailableSharedPublicRepository()
	}
	repository, ok := service.repository.(domainrss.SharedPublicRepository)
	if !ok {
		return domainrss.FetchLeaseResult{}, errorsUnavailableSharedPublicRepository()
	}
	deviceID = strings.TrimSpace(deviceID)
	subscriptionID, subscriptionOK := canonicalUUID(subscriptionID)
	if deviceID == "" || !subscriptionOK || request.TTLSeconds < 0 || request.TTLSeconds > 600 {
		return domainrss.FetchLeaseResult{}, invalidRSSRequest("invalid RSS fetch lease request")
	}
	ttl := defaultSharedPublicLeaseTTL
	if request.TTLSeconds != 0 {
		ttl = time.Duration(request.TTLSeconds) * time.Second
	}
	return repository.AcquireFetchLease(ctx, domainrss.FetchLeaseRequest{
		DeviceID: deviceID, SubscriptionID: subscriptionID, LeaseID: uuid.NewString(),
		RequestedTTL: ttl, RequestedAt: service.now().UTC(),
	})
}

func (service *Service) validateSharedPublicFeedURL(ctx context.Context, raw string) (string, error) {
	value, parsed, err := normalizeSharedPublicURL(raw)
	if err != nil {
		return "", invalidRSSRequest("invalid shared-public RSS feed URL")
	}
	resolveCtx, cancel := context.WithTimeout(ctx, sharedPublicDNSValidationTimeout)
	defer cancel()
	if _, err := networkpolicy.ResolvePublicIPs(resolveCtx, service.resolver, parsed.Hostname()); err != nil {
		return "", invalidRSSRequest("shared-public RSS feed host is not public")
	}
	return value, nil
}

func normalizeObservationEntry(
	item domainrss.ObservationEntry,
	subscriptionID, baseURL string,
) (domainrss.ObservationEntry, parsedEntry, error) {
	item.OriginKey = strings.TrimSpace(item.OriginKey)
	item.GUID = strings.TrimSpace(item.GUID)
	item.Title = strings.TrimSpace(item.Title)
	item.Author = strings.TrimSpace(item.Author)
	item.Summary = strings.TrimSpace(item.Summary)
	item.ContentHTML = strings.TrimSpace(item.ContentHTML)
	if !validOriginKey(item.OriginKey) || !validSharedPublicText(item.GUID, maxSharedPublicGUIDBytes, true) ||
		!validSharedPublicText(item.Title, maxRSSEntryTitleBytes, true) || !validSharedPublicText(item.Author, 8<<10, true) ||
		!validSharedPublicText(item.Summary, maxRSSEntrySummaryBytes, true) ||
		!utf8.ValidString(item.ContentHTML) || len(item.ContentHTML) > maxRSSEntryContentHTMLBytes ||
		len(item.Enclosures) > maxRSSParsedEntryMediaItems {
		return domainrss.ObservationEntry{}, parsedEntry{}, invalidRSSRequest("invalid RSS observation entry")
	}
	if item.PublishedAt != nil {
		value := item.PublishedAt.UTC()
		item.PublishedAt = &value
	}
	if item.UpdatedAt != nil {
		value := item.UpdatedAt.UTC()
		item.UpdatedAt = &value
	}
	if strings.TrimSpace(item.CanonicalLink) != "" {
		resolved, err := normalizeSharedPublicResourceURL(baseURL, item.CanonicalLink)
		if err != nil {
			return domainrss.ObservationEntry{}, parsedEntry{}, invalidRSSRequest("invalid RSS observation canonical link")
		}
		item.CanonicalLink = resolved
	}
	media := make([]parsedMedia, 0, len(item.Enclosures))
	for index := range item.Enclosures {
		enclosure := &item.Enclosures[index]
		resolved, err := normalizeSharedPublicResourceURL(baseURL, enclosure.URL)
		if err != nil || enclosure.ByteLength < 0 || enclosure.Width < 0 || enclosure.Height < 0 || enclosure.DurationMS < 0 {
			return domainrss.ObservationEntry{}, parsedEntry{}, invalidRSSRequest("invalid RSS observation enclosure")
		}
		enclosure.URL = resolved
		enclosure.MIMEType = strings.ToLower(strings.TrimSpace(enclosure.MIMEType))
		if !validSharedPublicText(enclosure.MIMEType, maxRSSMediaMIMETypeBytes, true) {
			return domainrss.ObservationEntry{}, parsedEntry{}, invalidRSSRequest("invalid RSS observation enclosure type")
		}
		if strings.TrimSpace(enclosure.ThumbnailURL) != "" {
			enclosure.ThumbnailURL, err = normalizeSharedPublicResourceURL(baseURL, enclosure.ThumbnailURL)
			if err != nil {
				return domainrss.ObservationEntry{}, parsedEntry{}, invalidRSSRequest("invalid RSS observation thumbnail")
			}
		}
		media = append(media, parsedMedia{
			URL: enclosure.URL, MIMEType: enclosure.MIMEType, Thumbnail: enclosure.ThumbnailURL,
			Width: enclosure.Width, Height: enclosure.Height, Duration: enclosure.DurationMS,
		})
	}
	wantOrigin := RSSOriginKeyV1(subscriptionID, item.GUID, item.CanonicalLink, item.Title, item.PublishedAt, item.Enclosures)
	if item.OriginKey != wantOrigin {
		return domainrss.ObservationEntry{}, parsedEntry{}, invalidRSSRequest("RSS observation origin key mismatch")
	}
	return item, parsedEntry{
		ExternalID: item.GUID, URL: item.CanonicalLink, Title: item.Title, Author: item.Author,
		Summary: item.Summary, Content: item.ContentHTML, Published: item.PublishedAt,
		Updated: item.UpdatedAt, Media: media,
	}, nil
}

// RSSOriginKeyV1 is the protocol identity shared with the iOS public-feed
// parser. The prefix is part of the wire contract and permits a future
// migration without treating a new hash algorithm as the same namespace.
func RSSOriginKeyV1(
	subscriptionID, guid, canonicalLink, title string,
	publishedAt *time.Time,
	enclosures []domainrss.ObservationEnclosure,
) string {
	identity := ""
	if normalizedGUID := normalizeOriginText(guid, false); normalizedGUID != "" {
		identity = "guid:" + normalizedGUID
	} else if normalizedLink := normalizedOriginURL(canonicalLink); normalizedLink != "" {
		identity = "link:" + normalizedLink
	} else {
		published := ""
		if publishedAt != nil {
			published = publishedAt.UTC().Format("2006-01-02T15:04:05.000Z")
		}
		enclosure := ""
		if len(enclosures) > 0 {
			enclosure = normalizedOriginURL(enclosures[0].URL)
		}
		identity = "fallback:" + published + "\x1f" + normalizeOriginText(title, true) + "\x1f" + enclosure
	}
	digest := sha256.Sum256([]byte(strings.TrimSpace(subscriptionID) + "\x1e" + identity))
	return "rss-origin-v1:" + hex.EncodeToString(digest[:])
}

func rssOriginKeyFromParsed(subscriptionID string, item parsedEntry) string {
	enclosures := make([]domainrss.ObservationEnclosure, 0, len(item.Media))
	for _, media := range item.Media {
		enclosures = append(enclosures, domainrss.ObservationEnclosure{URL: media.URL})
	}
	return RSSOriginKeyV1(subscriptionID, item.ExternalID, item.URL, item.Title, item.Published, enclosures)
}

func normalizeOriginText(value string, lowercase bool) string {
	value = norm.NFC.String(strings.TrimSpace(value))
	if lowercase {
		value = strings.ToLower(value)
	}
	return strings.Join(strings.Fields(value), " ")
}

func normalizedOriginURL(raw string) string {
	parsed, err := url.Parse(strings.TrimSpace(raw))
	if err != nil || parsed.Scheme == "" || parsed.Hostname() == "" {
		return ""
	}
	scheme, host, port, ok := normalizedHTTPOrigin(parsed)
	if !ok {
		return ""
	}
	parsed.Scheme = scheme
	parsed.Host = canonicalHTTPValidatorHost(host, port, scheme)
	parsed.Fragment = ""
	return parsed.String()
}

func normalizeSharedPublicURL(raw string) (string, *url.URL, error) {
	return normalizeSharedPublicURLWithPathPolicy(raw, true)
}

func normalizeSharedPublicURLWithPathPolicy(raw string, forceRootPath bool) (string, *url.URL, error) {
	trimmed := strings.TrimSpace(raw)
	if len(trimmed) == 0 || len(trimmed) > maxFeedValidatorURLBytes {
		return "", nil, fmt.Errorf("invalid shared-public URL")
	}
	parsed, err := networkpolicy.ValidatePublicHTTPURL(trimmed)
	if err != nil || !strings.EqualFold(parsed.Scheme, "https") {
		return "", nil, fmt.Errorf("invalid shared-public URL")
	}
	// A denylist cannot prove that an unknown query parameter is harmless.
	// Signed provider/CDN URLs use many names (X-Amz-Credential,
	// X-Goog-Signature, and future variants), so the v1 public contract is
	// intentionally query-free.
	if parsed.RawQuery != "" || parsed.ForceQuery {
		return "", nil, fmt.Errorf("credentialed shared-public URL")
	}
	scheme, host, port, ok := normalizedHTTPOrigin(parsed)
	if !ok || scheme != "https" {
		return "", nil, fmt.Errorf("invalid shared-public URL")
	}
	parsed.Scheme = "https"
	parsed.Host = canonicalHTTPValidatorHost(host, port, "https")
	parsed.Fragment = ""
	if forceRootPath && parsed.Path == "" {
		parsed.Path = "/"
	}
	value := parsed.String()
	if len(value) > maxFeedValidatorURLBytes {
		return "", nil, fmt.Errorf("invalid shared-public URL")
	}
	return value, parsed, nil
}

func normalizeSharedPublicResourceURL(baseURL, raw string) (string, error) {
	base, err := url.Parse(strings.TrimSpace(baseURL))
	if err != nil {
		return "", err
	}
	candidate, err := url.Parse(strings.TrimSpace(raw))
	if err != nil {
		return "", err
	}
	resolved := base.ResolveReference(candidate)
	value, _, err := normalizeSharedPublicURLWithPathPolicy(resolved.String(), false)
	return value, err
}

func normalizeSubscriptionFieldMask(values []string) ([]string, error) {
	result := make([]string, 0, len(values))
	seen := make(map[string]struct{}, len(values))
	for _, value := range values {
		value = strings.TrimSpace(value)
		switch value {
		case "title", "viewType", "categoryId", "sortOrder", "enabled", "publicFeedURL":
		default:
			return nil, invalidRSSRequest("invalid RSS subscription field mask")
		}
		if _, duplicate := seen[value]; duplicate {
			return nil, invalidRSSRequest("duplicate RSS subscription field mask")
		}
		seen[value] = struct{}{}
		result = append(result, value)
	}
	sort.Strings(result)
	return result, nil
}

func validSharedPublicText(value string, maxBytes int, allowEmpty bool) bool {
	if !utf8.ValidString(value) || len(value) > maxBytes || (!allowEmpty && strings.TrimSpace(value) == "") {
		return false
	}
	for _, character := range value {
		if unicode.IsControl(character) && character != '\n' && character != '\t' && character != '\r' {
			return false
		}
	}
	return true
}

func validFeedValidator(value string) bool {
	value = strings.TrimSpace(value)
	return len(value) <= maxSharedPublicValidatorBytes && !strings.ContainsAny(value, "\r\n") && utf8.ValidString(value)
}

func validOriginKey(value string) bool {
	const prefix = "rss-origin-v1:"
	return strings.HasPrefix(value, prefix) && len(value) == len(prefix)+64 && validLowerHex(value[len(prefix):])
}

func validSHA256(value string) bool {
	value = strings.TrimSpace(value)
	return len(value) == 64 && validLowerHex(strings.ToLower(value))
}

func validLowerHex(value string) bool {
	for _, character := range value {
		if (character < '0' || character > '9') && (character < 'a' || character > 'f') {
			return false
		}
	}
	return true
}

func canonicalUUID(value string) (string, bool) {
	parsed, err := uuid.Parse(strings.TrimSpace(value))
	if err != nil {
		return "", false
	}
	return strings.ToLower(parsed.String()), true
}

func canonicalSHA256(value any) (string, error) {
	encoded, err := json.Marshal(value)
	if err != nil {
		return "", err
	}
	digest := sha256.Sum256(encoded)
	return hex.EncodeToString(digest[:]), nil
}

func containsString(values []string, wanted string) bool {
	index := sort.SearchStrings(values, wanted)
	return index < len(values) && values[index] == wanted
}

func errorsUnavailableSharedPublicRepository() error {
	return fmt.Errorf("RSS shared-public repository unavailable")
}
