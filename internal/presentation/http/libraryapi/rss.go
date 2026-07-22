package libraryapi

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"unicode"
	"unicode/utf8"

	applicationrss "xiadown/internal/application/rss"
	"xiadown/internal/domain/library"
	domainrss "xiadown/internal/domain/rss"
)

const (
	maxRSSStateBodyBytes             = 32 << 10
	maxRSSSubscriptionMutationBytes  = 64 << 10
	maxRSSObservationBodyBytes       = 4 << 20
	maxRSSFetchLeaseBodyBytes        = 8 << 10
	defaultPublicRSSEntryPageSize    = 100
	defaultPublicRSSChangePageSize   = 200
	maxPublicRSSPageSize             = 500
	maxPublicRSSIDLength             = 255
	maxPublicRSSSnapshotCursorLength = 4096
)

// RSSService is the device-facing subset of the RSS application service. Feed
// administration and refresh operations intentionally stay desktop-only.
type RSSService interface {
	GetSyncOverview(context.Context, string) (domainrss.SyncOverview, error)
	ListSyncSubscriptions(context.Context) ([]domainrss.SyncSubscription, error)
	ListSyncEntries(context.Context, applicationrss.ListEntriesRequest) (applicationrss.SyncEntryPage, error)
	GetSyncEntry(context.Context, applicationrss.SubscriptionRequest) (applicationrss.SyncEntryDetail, error)
	GetSyncSnapshot(context.Context, applicationrss.SyncSnapshotRequest) (applicationrss.SyncSnapshotResult, error)
	ListSyncChanges(context.Context, applicationrss.ListChangesRequest) (domainrss.ChangePage, error)
	SetEntryStateForDevice(context.Context, string, applicationrss.SetEntryStateRequest) (domainrss.EntryState, error)
}

type RSSSharedPublicService interface {
	MutateSubscriptionForDevice(context.Context, string, string, applicationrss.SharedSubscriptionMutationRequest) (domainrss.SubscriptionMutationResult, error)
	SubmitFeedObservationForDevice(context.Context, string, string, applicationrss.FeedObservationRequest) (domainrss.ObservationResult, error)
	AcquireFetchLeaseForDevice(context.Context, string, string, applicationrss.FetchLeaseApplicationRequest) (domainrss.FetchLeaseResult, error)
}

type RSSFetchProjectionService interface {
	GetSyncSubscriptionSource(context.Context, string) (domainrss.SubscriptionSourceAccess, string, error)
}

type RSSAPI struct {
	service             RSSService
	resourceService     RSSResourceService
	discoveryResources  RSSDiscoveryResourceService
	resourceClient      *http.Client
	resourceProvider    applicationrss.HTTPClientProvider
	imageCache          *rssImageCache
	resourceSlots       chan struct{}
	pairedResourceSlots chan struct{}
	resourceTimeouts    rssResourceTimeoutPolicy
}

func NewRSSAPI(service RSSService, providers ...applicationrss.HTTPClientProvider) (*RSSAPI, error) {
	if service == nil {
		return nil, errors.New("Library public RSS API requires an application service")
	}
	var provider applicationrss.HTTPClientProvider
	if len(providers) > 0 {
		provider = providers[0]
	}
	resourceService, _ := service.(RSSResourceService)
	discoveryResources, _ := service.(RSSDiscoveryResourceService)
	resourceClient := applicationrss.NewRemoteResourceHTTPClient(nil)
	if provider != nil {
		// ProxyManager is mutable. Rebuild the restricted client per request so
		// disabling or rotating a proxy cannot retain old proxy credentials.
		resourceClient = nil
	}
	return &RSSAPI{
		service: service, resourceService: resourceService, discoveryResources: discoveryResources,
		resourceClient:      resourceClient,
		resourceProvider:    provider,
		imageCache:          newRSSImageCache(),
		resourceSlots:       make(chan struct{}, defaultMaxConcurrentRSSResourceStreams),
		pairedResourceSlots: make(chan struct{}, defaultMaxConcurrentPairedRSSResourceStreams),
		resourceTimeouts:    defaultRSSResourceTimeoutPolicy,
	}, nil
}

func (api *RSSAPI) Routes() []ProtectedRoute {
	return []ProtectedRoute{
		{Method: http.MethodGet, Path: "/api/v1/rss/overview", Scope: library.DeviceScopeRSSRead, Handler: http.HandlerFunc(api.getOverview)},
		{Method: http.MethodGet, Path: "/api/v1/rss/snapshot", Scope: library.DeviceScopeRSSRead, Handler: http.HandlerFunc(api.getSnapshot)},
		{Method: http.MethodGet, Path: "/api/v1/rss/subscriptions", Scope: library.DeviceScopeRSSRead, Handler: http.HandlerFunc(api.listSubscriptions)},
		{Method: http.MethodGet, Path: "/api/v1/rss/subscriptions/{id}/icon", Scope: library.DeviceScopeRSSRead, Handler: http.HandlerFunc(api.getSubscriptionResource)},
		{Method: http.MethodGet, Path: "/api/v1/rss/entries", Scope: library.DeviceScopeRSSRead, Handler: http.HandlerFunc(api.listEntries)},
		{Method: http.MethodGet, Path: "/api/v1/rss/entries/{id}", Scope: library.DeviceScopeRSSRead, Handler: http.HandlerFunc(api.getEntry)},
		{Method: http.MethodGet, Path: "/api/v1/rss/entries/{id}/resources/{slot}", Scope: library.DeviceScopeRSSRead, Handler: http.HandlerFunc(api.getEntryResource)},
		{Method: http.MethodGet, Path: "/api/v1/rss/changes", Scope: library.DeviceScopeRSSRead, Handler: http.HandlerFunc(api.listChanges)},
		{Method: http.MethodPatch, Path: "/api/v1/rss/entries/{id}/state", Scope: library.DeviceScopeRSSState, Handler: http.HandlerFunc(api.setEntryState)},
		{Method: http.MethodPost, Path: "/api/v1/rss/subscriptions/{id}/mutations", Scope: library.DeviceScopeRSSManage, Handler: http.HandlerFunc(api.mutateSubscription)},
		{Method: http.MethodPost, Path: "/api/v1/rss/subscriptions/{id}/fetch-lease", Scope: library.DeviceScopeRSSFetch, Handler: http.HandlerFunc(api.acquireFetchLease)},
		{Method: http.MethodPost, Path: "/api/v1/rss/subscriptions/{id}/observations", Scope: library.DeviceScopeRSSFetch, Handler: http.HandlerFunc(api.submitObservation)},
	}
}

func (api *RSSAPI) getOverview(w http.ResponseWriter, request *http.Request) {
	principal, ok := PrincipalFromContext(request.Context())
	if !ok || strings.TrimSpace(principal.CatalogID) == "" {
		unauthorized(w)
		return
	}
	result, err := api.service.GetSyncOverview(request.Context(), principal.CatalogID)
	if err != nil {
		writeRSSError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, result)
}

func (api *RSSAPI) getSnapshot(w http.ResponseWriter, request *http.Request) {
	query := request.URL.Query()
	epoch := strings.TrimSpace(query.Get("epoch"))
	highWaterRaw := strings.TrimSpace(query.Get("highWater"))
	highWater, highWaterErr := optionalInt64(highWaterRaw)
	limit, limitErr := optionalInteger(query.Get("limit"))
	cursor := strings.TrimSpace(query.Get("cursor"))
	if !validPublicRSSEpoch(epoch) || highWaterRaw == "" || highWaterErr != nil || highWater < 0 {
		writeError(w, http.StatusBadRequest, "invalid_cursor")
		return
	}
	if limitErr != nil || limit < 0 || limit > maxPublicRSSPageSize {
		writeError(w, http.StatusBadRequest, "invalid_pagination")
		return
	}
	if len(cursor) > maxPublicRSSSnapshotCursorLength || strings.IndexFunc(cursor, unicode.IsSpace) >= 0 {
		writeError(w, http.StatusBadRequest, "invalid_cursor")
		return
	}
	if limit == 0 {
		limit = defaultPublicRSSChangePageSize
	}
	result, err := api.service.GetSyncSnapshot(request.Context(), applicationrss.SyncSnapshotRequest{
		Epoch: epoch, HighWater: highWater, Cursor: cursor, Limit: limit,
	})
	if err != nil {
		writeRSSError(w, err)
		return
	}
	if result.Records == nil {
		result.Records = make([]domainrss.SyncSnapshotRecord, 0)
	}
	result.Records = api.projectSubscriptionRecords(request, result.Records)
	writeJSON(w, http.StatusOK, result)
}

func (api *RSSAPI) listSubscriptions(w http.ResponseWriter, request *http.Request) {
	w.Header().Set("Deprecation", "true")
	w.Header().Set("Link", `</api/v1/rss/snapshot>; rel="successor-version"`)
	items, err := api.service.ListSyncSubscriptions(request.Context())
	if err != nil {
		writeRSSError(w, err)
		return
	}
	if items == nil {
		items = make([]domainrss.SyncSubscription, 0)
	}
	includePublicURL := requestHasScope(request, library.DeviceScopeRSSFetch)
	for index := range items {
		items[index] = sanitizeDeviceSubscription(items[index], includePublicURL)
	}
	writeJSON(w, http.StatusOK, items)
}

func (api *RSSAPI) listEntries(w http.ResponseWriter, request *http.Request) {
	query := request.URL.Query()
	searchQuery := query.Get("q")
	limit, limitErr := optionalInteger(query.Get("limit"))
	offset, offsetErr := optionalInteger(query.Get("offset"))
	unreadOnly, unreadErr := optionalRSSBool(query.Get("unreadOnly"))
	starredOnly, starredErr := optionalRSSStarred(query.Get("is_starred"))
	kind := strings.ToLower(strings.TrimSpace(query.Get("kind")))
	if limitErr != nil || offsetErr != nil || limit < 0 || limit > maxPublicRSSPageSize || offset < 0 {
		writeError(w, http.StatusBadRequest, "invalid_pagination")
		return
	}
	if unreadErr != nil || starredErr != nil || !validPublicRSSKind(kind) || !validPublicSearchQuery(searchQuery) {
		writeError(w, http.StatusBadRequest, "invalid_request")
		return
	}
	if limit == 0 {
		limit = defaultPublicRSSEntryPageSize
	}
	subscriptionID := strings.TrimSpace(query.Get("subscriptionId"))
	if subscriptionID != "" && !validPublicRSSID(subscriptionID) {
		writeError(w, http.StatusBadRequest, "invalid_request")
		return
	}
	result, err := api.service.ListSyncEntries(request.Context(), applicationrss.ListEntriesRequest{
		SubscriptionID: subscriptionID,
		Kind:           kind,
		Query:          strings.TrimSpace(searchQuery),
		UnreadOnly:     unreadOnly,
		StarredOnly:    starredOnly,
		Limit:          limit,
		Offset:         offset,
	})
	if err != nil {
		writeRSSError(w, err)
		return
	}
	if result.Items == nil {
		result.Items = make([]domainrss.SyncEntry, 0)
	}
	writeJSON(w, http.StatusOK, result)
}

func (api *RSSAPI) getEntry(w http.ResponseWriter, request *http.Request) {
	id := strings.TrimSpace(request.PathValue("id"))
	if !validPublicRSSID(id) {
		writeError(w, http.StatusBadRequest, "invalid_request")
		return
	}
	result, err := api.service.GetSyncEntry(request.Context(), applicationrss.SubscriptionRequest{ID: id})
	if err != nil {
		writeRSSError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, result)
}

func (api *RSSAPI) listChanges(w http.ResponseWriter, request *http.Request) {
	epoch := strings.TrimSpace(request.URL.Query().Get("epoch"))
	after, afterErr := optionalInt64(request.URL.Query().Get("after"))
	limit, limitErr := optionalInteger(request.URL.Query().Get("limit"))
	if !validPublicRSSEpoch(epoch) || afterErr != nil || after < 0 {
		writeError(w, http.StatusBadRequest, "invalid_cursor")
		return
	}
	if limitErr != nil || limit < 0 || limit > maxPublicRSSPageSize {
		writeError(w, http.StatusBadRequest, "invalid_pagination")
		return
	}
	if limit == 0 {
		limit = defaultPublicRSSChangePageSize
	}
	result, err := api.service.ListSyncChanges(request.Context(), applicationrss.ListChangesRequest{Epoch: epoch, After: after, Limit: limit})
	if err != nil {
		writeRSSError(w, err)
		return
	}
	if result.Changes == nil {
		result.Changes = make([]domainrss.Change, 0)
	}
	for index := range result.Changes {
		if result.Changes[index].Operation == "delete" {
			continue
		}
		records := api.projectSubscriptionRecords(request, []domainrss.SyncSnapshotRecord{{
			EntityType: result.Changes[index].EntityType, EntityID: result.Changes[index].EntityID,
			Revision: result.Changes[index].Revision, Payload: result.Changes[index].Payload,
		}})
		result.Changes[index].Payload = records[0].Payload
	}
	writeJSON(w, http.StatusOK, result)
}

func (api *RSSAPI) mutateSubscription(w http.ResponseWriter, request *http.Request) {
	principal, ok := PrincipalFromContext(request.Context())
	service, serviceOK := api.service.(RSSSharedPublicService)
	id := strings.TrimSpace(request.PathValue("id"))
	if !ok || !serviceOK || !validPublicRSSID(principal.DeviceID) {
		unauthorized(w)
		return
	}
	if !validPublicRSSID(id) {
		writeError(w, http.StatusBadRequest, "invalid_request")
		return
	}
	request.Body = http.MaxBytesReader(w, request.Body, maxRSSSubscriptionMutationBytes)
	decoder := json.NewDecoder(request.Body)
	decoder.DisallowUnknownFields()
	var payload applicationrss.SharedSubscriptionMutationRequest
	if err := decoder.Decode(&payload); err != nil || ensureJSONEOF(decoder) != nil {
		writeError(w, http.StatusBadRequest, "invalid_request")
		return
	}
	needsFetch := payload.Operation == domainrss.SubscriptionMutationAdd ||
		payload.Operation == domainrss.SubscriptionMutationPromote ||
		containsRSSField(payload.FieldMask, "publicFeedURL")
	if needsFetch && !principal.HasScope(library.DeviceScopeRSSFetch) {
		writeError(w, http.StatusForbidden, "insufficient_scope")
		return
	}
	result, err := service.MutateSubscriptionForDevice(request.Context(), principal.DeviceID, id, payload)
	if err != nil {
		if errors.Is(err, domainrss.ErrRevisionConflict) {
			writeError(w, http.StatusConflict, "revision_conflict")
			return
		}
		writeRSSError(w, err)
		return
	}
	if result.Subscription != nil {
		if principal.HasScope(library.DeviceScopeRSSFetch) && result.Subscription.SourceAccess == domainrss.SubscriptionSourceSharedPublic {
			if projection, projectionOK := api.service.(RSSFetchProjectionService); projectionOK {
				access, publicURL, projectionErr := projection.GetSyncSubscriptionSource(request.Context(), result.Subscription.ID)
				if projectionErr == nil && access == domainrss.SubscriptionSourceSharedPublic {
					result.Subscription.PublicFeedURL = strings.TrimSpace(publicURL)
				}
			}
		}
		item := sanitizeDeviceSubscription(*result.Subscription, principal.HasScope(library.DeviceScopeRSSFetch))
		result.Subscription = &item
	}
	status := http.StatusOK
	if payload.Operation == domainrss.SubscriptionMutationAdd || payload.Operation == domainrss.SubscriptionMutationPromote {
		status = http.StatusCreated
	}
	writeJSON(w, status, result)
}

func (api *RSSAPI) acquireFetchLease(w http.ResponseWriter, request *http.Request) {
	principal, ok := PrincipalFromContext(request.Context())
	service, serviceOK := api.service.(RSSSharedPublicService)
	id := strings.TrimSpace(request.PathValue("id"))
	if !ok || !serviceOK || !validPublicRSSID(principal.DeviceID) {
		unauthorized(w)
		return
	}
	if !validPublicRSSID(id) {
		writeError(w, http.StatusBadRequest, "invalid_request")
		return
	}
	request.Body = http.MaxBytesReader(w, request.Body, maxRSSFetchLeaseBodyBytes)
	decoder := json.NewDecoder(request.Body)
	decoder.DisallowUnknownFields()
	var payload applicationrss.FetchLeaseApplicationRequest
	if err := decoder.Decode(&payload); err != nil || ensureJSONEOF(decoder) != nil {
		writeError(w, http.StatusBadRequest, "invalid_request")
		return
	}
	result, err := service.AcquireFetchLeaseForDevice(request.Context(), principal.DeviceID, id, payload)
	if err != nil {
		writeRSSError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, result)
}

func (api *RSSAPI) submitObservation(w http.ResponseWriter, request *http.Request) {
	principal, ok := PrincipalFromContext(request.Context())
	service, serviceOK := api.service.(RSSSharedPublicService)
	id := strings.TrimSpace(request.PathValue("id"))
	if !ok || !serviceOK || !validPublicRSSID(principal.DeviceID) {
		unauthorized(w)
		return
	}
	if !validPublicRSSID(id) {
		writeError(w, http.StatusBadRequest, "invalid_request")
		return
	}
	request.Body = http.MaxBytesReader(w, request.Body, maxRSSObservationBodyBytes)
	body, readErr := io.ReadAll(request.Body)
	if readErr != nil {
		writeError(w, http.StatusBadRequest, "invalid_request")
		return
	}
	decoder := json.NewDecoder(bytes.NewReader(body))
	decoder.DisallowUnknownFields()
	var payload applicationrss.FeedObservationRequest
	if err := decoder.Decode(&payload); err != nil || ensureJSONEOF(decoder) != nil ||
		!validPublicRSSID(payload.MutationID) || payload.FetchedAt.IsZero() || payload.Entries == nil ||
		!verifyRSSObservationContentHash(body, payload.ContentHash) {
		writeError(w, http.StatusBadRequest, "invalid_request")
		return
	}
	result, err := service.SubmitFeedObservationForDevice(request.Context(), principal.DeviceID, id, payload)
	if err != nil {
		writeRSSError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, result)
}

func verifyRSSObservationContentHash(body []byte, claimed string) bool {
	var envelope struct {
		Entries json.RawMessage `json:"entries"`
	}
	if err := json.Unmarshal(body, &envelope); err != nil || len(envelope.Entries) == 0 {
		return false
	}
	decoder := json.NewDecoder(bytes.NewReader(envelope.Entries))
	decoder.UseNumber()
	var value any
	if err := decoder.Decode(&value); err != nil || ensureJSONEOF(decoder) != nil {
		return false
	}
	var canonical bytes.Buffer
	encoder := json.NewEncoder(&canonical)
	encoder.SetEscapeHTML(false)
	if err := encoder.Encode(value); err != nil {
		return false
	}
	digest := sha256.Sum256(bytes.TrimSpace(canonical.Bytes()))
	return strings.EqualFold(strings.TrimSpace(claimed), fmt.Sprintf("%x", digest[:]))
}

func (api *RSSAPI) projectSubscriptionRecords(request *http.Request, records []domainrss.SyncSnapshotRecord) []domainrss.SyncSnapshotRecord {
	includePublicURL := requestHasScope(request, library.DeviceScopeRSSFetch)
	projection, projectionOK := api.service.(RSSFetchProjectionService)
	for index := range records {
		if records[index].EntityType != "subscription" || len(records[index].Payload) == 0 {
			continue
		}
		var item domainrss.SyncSubscription
		if err := json.Unmarshal(records[index].Payload, &item); err != nil {
			continue
		}
		item.PublicFeedURL = ""
		if includePublicURL && projectionOK && item.SourceAccess == domainrss.SubscriptionSourceSharedPublic {
			access, publicURL, err := projection.GetSyncSubscriptionSource(request.Context(), records[index].EntityID)
			if err == nil && access == domainrss.SubscriptionSourceSharedPublic {
				item.PublicFeedURL = strings.TrimSpace(publicURL)
			}
		}
		item = sanitizeDeviceSubscription(item, includePublicURL)
		if payload, err := json.Marshal(item); err == nil {
			records[index].Payload = payload
		}
	}
	return records
}

func sanitizeDeviceSubscription(item domainrss.SyncSubscription, includePublicURL bool) domainrss.SyncSubscription {
	if item.SourceAccess != domainrss.SubscriptionSourceSharedPublic {
		item.SourceAccess = domainrss.SubscriptionSourceDesktopManaged
		item.PublicFeedURL = ""
		return item
	}
	if !includePublicURL {
		item.PublicFeedURL = ""
	}
	return item
}

func requestHasScope(request *http.Request, scope library.DeviceScope) bool {
	principal, ok := PrincipalFromContext(request.Context())
	return ok && principal.HasScope(scope)
}

func containsRSSField(values []string, wanted string) bool {
	for _, value := range values {
		if strings.TrimSpace(value) == wanted {
			return true
		}
	}
	return false
}

type setRSSEntryStatePayload struct {
	Field                string          `json:"field"`
	ExpectedRevision     *int64          `json:"expectedRevision"`
	MutationID           string          `json:"mutationId"`
	Value                json.RawMessage `json:"value"`
	VideoDurationSeconds *float64        `json:"videoDurationSeconds"`
}

func (api *RSSAPI) setEntryState(w http.ResponseWriter, request *http.Request) {
	principal, ok := PrincipalFromContext(request.Context())
	if !ok || !validPublicRSSID(principal.DeviceID) {
		unauthorized(w)
		return
	}
	id := strings.TrimSpace(request.PathValue("id"))
	if !validPublicRSSID(id) {
		writeError(w, http.StatusBadRequest, "invalid_request")
		return
	}

	request.Body = http.MaxBytesReader(w, request.Body, maxRSSStateBodyBytes)
	decoder := json.NewDecoder(request.Body)
	decoder.DisallowUnknownFields()
	var payload setRSSEntryStatePayload
	if err := decoder.Decode(&payload); err != nil || ensureJSONEOF(decoder) != nil ||
		payload.ExpectedRevision == nil || *payload.ExpectedRevision < 0 ||
		!validPublicRSSID(payload.MutationID) {
		writeError(w, http.StatusBadRequest, "invalid_request")
		return
	}

	stateRequest, valid := decodeRSSEntryStateRequest(id, payload)
	if !valid {
		writeError(w, http.StatusBadRequest, "invalid_request")
		return
	}
	result, err := api.service.SetEntryStateForDevice(
		request.Context(),
		strings.TrimSpace(principal.DeviceID),
		stateRequest,
	)
	if err != nil {
		writeRSSError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, result)
}

func decodeRSSEntryStateRequest(id string, payload setRSSEntryStatePayload) (applicationrss.SetEntryStateRequest, bool) {
	request := applicationrss.SetEntryStateRequest{
		ID: strings.TrimSpace(id), ExpectedRevision: payload.ExpectedRevision,
		MutationID: strings.TrimSpace(payload.MutationID),
	}
	field := domainrss.EntryStateField(strings.TrimSpace(payload.Field))
	if field == "" {
		return applicationrss.SetEntryStateRequest{}, false
	}
	if len(bytes.TrimSpace(payload.Value)) == 0 || bytes.Equal(bytes.TrimSpace(payload.Value), []byte("null")) {
		return applicationrss.SetEntryStateRequest{}, false
	}
	request.Field = field
	switch field {
	case domainrss.EntryStateFieldRead:
		var value bool
		if !decodeStrictRSSValue(payload.Value, &value) || payload.VideoDurationSeconds != nil {
			return applicationrss.SetEntryStateRequest{}, false
		}
		request.Read = &value
	case domainrss.EntryStateFieldStarred:
		var value bool
		if !decodeStrictRSSValue(payload.Value, &value) || payload.VideoDurationSeconds != nil {
			return applicationrss.SetEntryStateRequest{}, false
		}
		request.Starred = &value
	case domainrss.EntryStateFieldArticleProgress:
		var value domainrss.ArticleProgress
		if !decodeStrictRSSValue(payload.Value, &value) || payload.VideoDurationSeconds != nil {
			return applicationrss.SetEntryStateRequest{}, false
		}
		request.ArticleProgress = &value
	case domainrss.EntryStateFieldVideoProgressSeconds:
		var value float64
		if !decodeStrictRSSValue(payload.Value, &value) {
			return applicationrss.SetEntryStateRequest{}, false
		}
		request.VideoProgressSeconds = &value
		request.VideoDurationSeconds = payload.VideoDurationSeconds
	default:
		return applicationrss.SetEntryStateRequest{}, false
	}
	return request, true
}

func decodeStrictRSSValue(raw json.RawMessage, target any) bool {
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.DisallowUnknownFields()
	return decoder.Decode(target) == nil && ensureJSONEOF(decoder) == nil
}

func optionalRSSBool(value string) (bool, error) {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "", "false":
		return false, nil
	case "true":
		return true, nil
	default:
		return false, errors.New("invalid RSS boolean")
	}
}

// is_starred intentionally uses the stable integer sentinel used by mobile
// collection filters. It is separate from kind because starred is owner state,
// not an entry presentation type.
func optionalRSSStarred(value string) (bool, error) {
	value = strings.TrimSpace(value)
	if value == "" {
		return false, nil
	}
	if value == "1" {
		return true, nil
	}
	return false, errors.New("invalid RSS starred filter")
}

func validPublicRSSKind(value string) bool {
	switch domainrss.EntryKind(value) {
	case "", domainrss.EntryKindArticle, domainrss.EntryKindSocial, domainrss.EntryKindImage, domainrss.EntryKindVideo:
		return true
	default:
		return false
	}
}

func validPublicRSSID(value string) bool {
	value = strings.TrimSpace(value)
	if value == "" || !utf8.ValidString(value) || utf8.RuneCountInString(value) > maxPublicRSSIDLength {
		return false
	}
	for _, item := range value {
		if unicode.IsSpace(item) || unicode.IsControl(item) {
			return false
		}
	}
	return true
}

func validPublicRSSEpoch(value string) bool {
	if len(value) != 32 {
		return false
	}
	for _, item := range value {
		if (item < '0' || item > '9') && (item < 'a' || item > 'f') {
			return false
		}
	}
	return true
}

func writeRSSError(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, domainrss.ErrInvalidRequest):
		writeError(w, http.StatusBadRequest, "invalid_request")
	case errors.Is(err, domainrss.ErrSyncResetRequired):
		var reset *domainrss.SyncResetError
		if errors.As(err, &reset) {
			writeJSON(w, http.StatusConflict, map[string]any{"error": "reset_required", "sync": reset.Position})
			return
		}
		writeError(w, http.StatusConflict, "reset_required")
	case errors.Is(err, domainrss.ErrNotFound):
		writeError(w, http.StatusNotFound, "not_found")
	case errors.Is(err, domainrss.ErrRevisionConflict):
		var conflict *domainrss.StateConflictError
		if errors.As(err, &conflict) {
			writeJSON(w, http.StatusConflict, map[string]any{"error": "rss_state_conflict", "state": conflict.State})
			return
		}
		writeError(w, http.StatusConflict, "rss_state_conflict")
	case errors.Is(err, domainrss.ErrIdempotencyConflict):
		writeError(w, http.StatusConflict, "idempotency_conflict")
	default:
		writeError(w, http.StatusInternalServerError, "internal_error")
	}
}
