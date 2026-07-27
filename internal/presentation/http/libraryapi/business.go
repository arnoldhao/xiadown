package libraryapi

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/base64"
	"encoding/json"
	"errors"
	"io"
	"math"
	"mime"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"
	"unicode"

	"xiadown/internal/application/library/dto"
	"xiadown/internal/domain/library"
)

type CatalogReader interface {
	GetDefaultCatalogOverview(context.Context) (dto.CatalogOverviewDTO, error)
	ListCatalogItems(context.Context, dto.ListCatalogItemsRequest) (dto.ListCatalogItemsResponse, error)
	GetCatalogItem(context.Context, dto.GetCatalogItemRequest) (dto.CatalogItemDetailDTO, error)
	ListCatalogCollections(context.Context) ([]dto.CatalogCollectionDTO, error)
	ListCatalogCollectionItemsPage(context.Context, string, int, int) (dto.CatalogCollectionItemsPageDTO, error)
	ListCatalogTags(context.Context) ([]dto.CatalogTagDTO, error)
}

type catalogPageReader interface {
	ListCatalogCollectionsPage(context.Context, int, int, int) ([]dto.CatalogCollectionDTO, error)
	ListCatalogTagsPage(context.Context, int, int) ([]dto.CatalogTagDTO, error)
}

type catalogSnapshotReader interface {
	ListCatalogSnapshotItems(context.Context, string, string, int) ([]dto.CatalogItemDTO, error)
}

type BusinessConfig struct {
	CatalogID string
	Catalog   CatalogReader
	Items     library.CatalogItemRepository
	Assets    library.ItemAssetRepository
	Files     library.FileRepository
	Changes   library.CatalogChangeRepository
	Sync      library.CatalogSyncStateRepository
	// Asset streams have independent resource guards because media responses
	// can remain open for hours while metadata requests are short lived.
	MaxConcurrentAssetStreams int
	AssetWriteIdleTimeout     time.Duration
}

type BusinessAPI struct {
	config      BusinessConfig
	streamSlots chan struct{}
}

const (
	maxPublicCatalogPageSize         = 500
	defaultPublicSnapshotPageSize    = 200
	maxPublicSnapshotCursorLength    = 1024
	defaultPublicTaxonomyPageSize    = 100
	maxPublicTaxonomyPageSize        = 500
	maxPublicCollectionPageSize      = 100
	maxPublicCollectionItemIDs       = 500
	defaultMaxConcurrentAssetStreams = 16
	defaultAssetWriteIdleTimeout     = 45 * time.Second
	LibrarySnapshotKeysetCapability  = "snapshot-keyset-v1"
)

func NewBusinessAPI(config BusinessConfig) (*BusinessAPI, error) {
	config.CatalogID = strings.TrimSpace(config.CatalogID)
	if config.CatalogID == "" || config.Catalog == nil || config.Items == nil ||
		config.Assets == nil || config.Files == nil || config.Changes == nil || config.Sync == nil {
		return nil, errors.New("Library public business API is incomplete")
	}
	if config.MaxConcurrentAssetStreams <= 0 {
		config.MaxConcurrentAssetStreams = defaultMaxConcurrentAssetStreams
	}
	if config.AssetWriteIdleTimeout <= 0 {
		config.AssetWriteIdleTimeout = defaultAssetWriteIdleTimeout
	}
	return &BusinessAPI{
		config: config, streamSlots: make(chan struct{}, config.MaxConcurrentAssetStreams),
	}, nil
}

func (api *BusinessAPI) Routes() []ProtectedRoute {
	read := library.DeviceScopeLibraryRead
	return []ProtectedRoute{
		{Method: http.MethodGet, Path: "/api/v1/library", Scope: read, Handler: http.HandlerFunc(api.overview)},
		{Method: http.MethodGet, Path: "/api/v1/library/snapshot", Scope: read, Handler: http.HandlerFunc(api.snapshot)},
		{Method: http.MethodGet, Path: "/api/v1/library/items", Scope: read, Handler: http.HandlerFunc(api.listItems)},
		{Method: http.MethodGet, Path: "/api/v1/library/items/{id}", Scope: read, Handler: http.HandlerFunc(api.getItem)},
		{Method: http.MethodGet, Path: "/api/v1/library/assets/{id}/content", Scope: read, Handler: api.withAssetStreamGuard(http.HandlerFunc(api.assetContent))},
		{Method: http.MethodGet, Path: "/api/v1/library/collections", Scope: read, Handler: http.HandlerFunc(api.collections)},
		{Method: http.MethodGet, Path: "/api/v1/library/collections/{id}/items", Scope: read, Handler: http.HandlerFunc(api.collectionItems)},
		{Method: http.MethodGet, Path: "/api/v1/library/tags", Scope: read, Handler: http.HandlerFunc(api.tags)},
		{Method: http.MethodGet, Path: "/api/v1/library/changes", Scope: read, Handler: http.HandlerFunc(api.changes)},
	}
}

func (api *BusinessAPI) withAssetStreamGuard(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, request *http.Request) {
		select {
		case api.streamSlots <- struct{}{}:
			defer func() { <-api.streamSlots }()
		case <-request.Context().Done():
			return
		default:
			w.Header().Set("Retry-After", "1")
			writeError(w, http.StatusTooManyRequests, "asset_stream_limit_reached")
			return
		}
		writer := &idleWriteResponseWriter{
			ResponseWriter: w,
			controller:     http.NewResponseController(w),
			timeout:        api.config.AssetWriteIdleTimeout,
		}
		writer.refreshDeadline()
		defer writer.clearDeadline()
		next.ServeHTTP(writer, request)
	})
}

// idleWriteResponseWriter applies a sliding write deadline. Unlike an
// http.Server WriteTimeout, it is refreshed after every successful chunk and
// therefore does not impose an absolute duration on a healthy multi-gigabyte
// stream.
type idleWriteResponseWriter struct {
	http.ResponseWriter
	controller *http.ResponseController
	timeout    time.Duration
}

func (writer *idleWriteResponseWriter) Unwrap() http.ResponseWriter { return writer.ResponseWriter }

func (writer *idleWriteResponseWriter) WriteHeader(status int) {
	writer.refreshDeadline()
	writer.ResponseWriter.WriteHeader(status)
}

func (writer *idleWriteResponseWriter) Write(value []byte) (int, error) {
	writer.refreshDeadline()
	written, err := writer.ResponseWriter.Write(value)
	if err == nil {
		writer.refreshDeadline()
	}
	return written, err
}

func (writer *idleWriteResponseWriter) refreshDeadline() {
	if writer.controller != nil && writer.timeout > 0 {
		_ = writer.controller.SetWriteDeadline(time.Now().Add(writer.timeout))
	}
}

func (writer *idleWriteResponseWriter) clearDeadline() {
	if writer.controller != nil {
		_ = writer.controller.SetWriteDeadline(time.Time{})
	}
}

func (api *BusinessAPI) overview(w http.ResponseWriter, request *http.Request) {
	// Read the high-water mark first. If a mutation commits while the overview is
	// being assembled, the client may observe it twice, but can never skip it.
	syncState, err := api.config.Sync.GetCatalogSyncState(request.Context(), api.config.CatalogID)
	if err != nil {
		writeBusinessError(w, err)
		return
	}
	result, err := api.config.Catalog.GetDefaultCatalogOverview(request.Context())
	if err != nil {
		writeBusinessError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, publicLibraryOverview{
		Catalog:      result.Catalog,
		Categories:   result.Categories,
		Statuses:     result.Statuses,
		Health:       result.Health,
		Sync:         newPublicSyncState(syncState),
		Capabilities: []string{LibrarySnapshotKeysetCapability},
	})
}

type librarySnapshotCursor struct {
	Version   int    `json:"v"`
	Epoch     string `json:"e"`
	HighWater int64  `json:"h"`
	AfterID   string `json:"a"`
}

type publicLibrarySnapshotPage struct {
	Items     []dto.CatalogItemDTO `json:"items"`
	Epoch     string               `json:"epoch"`
	HighWater int64                `json:"highWater"`
	Next      string               `json:"next,omitempty"`
	HasMore   bool                 `json:"hasMore"`
}

func (api *BusinessAPI) snapshot(w http.ResponseWriter, request *http.Request) {
	query := request.URL.Query()
	epoch := strings.TrimSpace(query.Get("epoch"))
	highWaterRaw := strings.TrimSpace(query.Get("highWater"))
	highWater, highWaterErr := optionalInt64(highWaterRaw)
	limit, limitErr := optionalInteger(query.Get("limit"))
	if !validPublicSyncEpoch(epoch) || highWaterRaw == "" || highWaterErr != nil || highWater < 0 {
		writeError(w, http.StatusBadRequest, "invalid_cursor")
		return
	}
	if limitErr != nil || limit < 0 || limit > maxPublicCatalogPageSize {
		writeError(w, http.StatusBadRequest, "invalid_pagination")
		return
	}
	if limit == 0 {
		limit = defaultPublicSnapshotPageSize
	}

	afterID := ""
	if encoded := strings.TrimSpace(query.Get("after")); encoded != "" {
		cursor, err := decodeLibrarySnapshotCursor(encoded)
		if err != nil || cursor.Epoch != epoch || cursor.HighWater != highWater {
			writeError(w, http.StatusBadRequest, "invalid_cursor")
			return
		}
		afterID = cursor.AfterID
	}

	before, err := api.config.Sync.GetCatalogSyncState(request.Context(), api.config.CatalogID)
	if err != nil {
		writeBusinessError(w, err)
		return
	}
	if before.Epoch != epoch || before.Cursor != highWater {
		writeLibraryResetRequired(w, before)
		return
	}

	items, pageErr := api.listSnapshotItems(request.Context(), afterID, limit+1)
	after, syncErr := api.config.Sync.GetCatalogSyncState(request.Context(), api.config.CatalogID)
	if syncErr != nil {
		writeBusinessError(w, syncErr)
		return
	}
	if after.Epoch != before.Epoch || after.Cursor != before.Cursor {
		writeLibraryResetRequired(w, after)
		return
	}
	if pageErr != nil {
		writeBusinessError(w, pageErr)
		return
	}
	if err := validateLibrarySnapshotItems(items, api.config.CatalogID, afterID, limit+1); err != nil {
		writeBusinessError(w, err)
		return
	}

	hasMore := len(items) > limit
	if hasMore {
		items = items[:limit]
	}
	result := publicLibrarySnapshotPage{
		Items: items, Epoch: before.Epoch, HighWater: before.Cursor, HasMore: hasMore,
	}
	if hasMore {
		result.Next, err = encodeLibrarySnapshotCursor(librarySnapshotCursor{
			Version: 1, Epoch: before.Epoch, HighWater: before.Cursor,
			AfterID: items[len(items)-1].ID,
		})
		if err != nil {
			writeBusinessError(w, err)
			return
		}
	}
	writeJSON(w, http.StatusOK, result)
}

func (api *BusinessAPI) listSnapshotItems(ctx context.Context, afterID string, limit int) ([]dto.CatalogItemDTO, error) {
	if reader, ok := api.config.Catalog.(catalogSnapshotReader); ok {
		return reader.ListCatalogSnapshotItems(ctx, api.config.CatalogID, afterID, limit)
	}

	var items []library.Item
	var err error
	if reader, ok := api.config.Items.(library.CatalogItemSnapshotRepository); ok {
		items, err = reader.ListSnapshotPageByCatalogID(ctx, api.config.CatalogID, afterID, limit)
	} else {
		items, err = api.config.Items.ListByCatalogID(ctx, api.config.CatalogID)
		if err == nil {
			items = publicSnapshotFallbackPage(items, afterID, limit)
		}
	}
	if err != nil {
		return nil, err
	}
	result := make([]dto.CatalogItemDTO, 0, len(items))
	for _, item := range items {
		result = append(result, publicCatalogItemSummary(item))
	}
	return result, nil
}

func publicSnapshotFallbackPage(items []library.Item, afterID string, limit int) []library.Item {
	filtered := make([]library.Item, 0, min(limit, len(items)))
	for _, item := range items {
		if item.Status == library.ItemStatusTrashed || item.TrashedAt != nil || strings.Compare(item.ID, afterID) <= 0 {
			continue
		}
		filtered = append(filtered, item)
	}
	sort.Slice(filtered, func(left, right int) bool { return filtered[left].ID < filtered[right].ID })
	if len(filtered) > limit {
		filtered = filtered[:limit]
	}
	return filtered
}

func publicCatalogItemSummary(item library.Item) dto.CatalogItemDTO {
	availability := library.ItemAvailabilityAvailable
	if item.Status == library.ItemStatusMissing || item.Status == library.ItemStatusTrashed {
		availability = library.ItemAvailabilityMissing
	}
	result := dto.CatalogItemDTO{
		ID: item.ID, CatalogID: item.CatalogID, Category: string(item.Category), Status: string(item.Status),
		Availability: string(availability),
		Title:        item.Title, SortTitle: item.SortTitle, Description: item.Description, Revision: item.Revision,
		CreatedAt: item.CreatedAt.UTC().Format(time.RFC3339Nano), UpdatedAt: item.UpdatedAt.UTC().Format(time.RFC3339Nano),
	}
	if item.TrashedAt != nil {
		result.TrashedAt = item.TrashedAt.UTC().Format(time.RFC3339Nano)
	}
	return result
}

func validateLibrarySnapshotItems(items []dto.CatalogItemDTO, catalogID, afterID string, maximum int) error {
	if len(items) > maximum {
		return errors.New("invalid Library snapshot page size")
	}
	previousID := afterID
	for _, item := range items {
		if item.CatalogID != catalogID || !safePublicOpaqueID(item.ID) || strings.Compare(item.ID, previousID) <= 0 ||
			strings.EqualFold(strings.TrimSpace(item.Status), string(library.ItemStatusTrashed)) ||
			!validPublicItemAvailability(item.Availability) ||
			strings.TrimSpace(item.TrashedAt) != "" {
			return errors.New("invalid Library snapshot item")
		}
		previousID = item.ID
	}
	return nil
}

func validPublicItemAvailability(value string) bool {
	switch library.ItemAvailability(strings.ToLower(strings.TrimSpace(value))) {
	case library.ItemAvailabilityAvailable, library.ItemAvailabilityChecking,
		library.ItemAvailabilityOffline, library.ItemAvailabilityMissing,
		library.ItemAvailabilityError:
		return true
	default:
		return false
	}
}

func encodeLibrarySnapshotCursor(cursor librarySnapshotCursor) (string, error) {
	encoded, err := json.Marshal(cursor)
	if err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(encoded), nil
}

func decodeLibrarySnapshotCursor(value string) (librarySnapshotCursor, error) {
	if len(value) > maxPublicSnapshotCursorLength {
		return librarySnapshotCursor{}, errors.New("snapshot cursor is too long")
	}
	raw, err := base64.RawURLEncoding.DecodeString(value)
	if err != nil {
		return librarySnapshotCursor{}, err
	}
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.DisallowUnknownFields()
	var cursor librarySnapshotCursor
	if err := decoder.Decode(&cursor); err != nil {
		return librarySnapshotCursor{}, err
	}
	if err := decoder.Decode(&struct{}{}); !errors.Is(err, io.EOF) {
		return librarySnapshotCursor{}, errors.New("snapshot cursor has trailing data")
	}
	if cursor.Version != 1 || !validPublicSyncEpoch(cursor.Epoch) || cursor.HighWater < 0 ||
		!safePublicOpaqueID(cursor.AfterID) || cursor.AfterID != strings.TrimSpace(cursor.AfterID) {
		return librarySnapshotCursor{}, errors.New("invalid snapshot cursor")
	}
	return cursor, nil
}

func validPublicSyncEpoch(value string) bool {
	if len(value) != library.CatalogSyncEpochLength {
		return false
	}
	for _, character := range value {
		if (character < '0' || character > '9') && (character < 'a' || character > 'f') {
			return false
		}
	}
	return true
}

func writeLibraryResetRequired(w http.ResponseWriter, state library.CatalogSyncState) {
	writeJSON(w, http.StatusConflict, publicResetRequired{
		Error: "reset_required", Sync: newPublicSyncState(state),
	})
}

func (api *BusinessAPI) listItems(w http.ResponseWriter, request *http.Request) {
	query := request.URL.Query()
	searchQuery := query.Get("q")
	limit, limitErr := optionalInteger(query.Get("limit"))
	offset, offsetErr := optionalInteger(query.Get("offset"))
	if limitErr != nil || offsetErr != nil || limit < 0 || limit > maxPublicCatalogPageSize || offset < 0 {
		writeError(w, http.StatusBadRequest, "invalid_pagination")
		return
	}
	if !validPublicSearchQuery(searchQuery) {
		writeError(w, http.StatusBadRequest, "invalid_request")
		return
	}
	excludeTrashed := false
	if raw := strings.TrimSpace(query.Get("excludeTrashed")); raw != "" {
		var parseErr error
		excludeTrashed, parseErr = strconv.ParseBool(raw)
		if parseErr != nil {
			writeError(w, http.StatusBadRequest, "invalid_request")
			return
		}
	}
	result, err := api.config.Catalog.ListCatalogItems(request.Context(), dto.ListCatalogItemsRequest{
		Category: query.Get("category"), Status: query.Get("status"), Query: searchQuery,
		ExcludeTrashed: excludeTrashed, Sort: query.Get("sort"), Limit: limit, Offset: offset,
	})
	if err != nil {
		writeBusinessError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, result)
}

type publicAsset struct {
	ID            string `json:"id"`
	FileID        string `json:"fileId"`
	Role          string `json:"role"`
	Label         string `json:"label,omitempty"`
	Position      int    `json:"position"`
	FileAvailable bool   `json:"fileAvailable"`
	ContentURL    string `json:"contentUrl,omitempty"`
}

// publicRepresentation is deliberately independent from the desktop DTO.
// The public API needs playback and presentation characteristics, but it does
// not need Catalog internals, revision bookkeeping, or content checksums.
type publicRepresentation struct {
	ID           string `json:"id"`
	AssetID      string `json:"assetId"`
	Kind         string `json:"kind"`
	Purpose      string `json:"purpose"`
	MediaType    string `json:"mediaType,omitempty"`
	Container    string `json:"container,omitempty"`
	Codec        string `json:"codec,omitempty"`
	Width        *int   `json:"width,omitempty"`
	Height       *int   `json:"height,omitempty"`
	DurationMs   *int64 `json:"durationMs,omitempty"`
	BitrateBps   *int64 `json:"bitrateBps,omitempty"`
	Language     string `json:"language,omitempty"`
	SizeBytes    *int64 `json:"sizeBytes,omitempty"`
	Availability string `json:"availability"`
}

// publicMetadata contains descriptive, scalar metadata only. Provenance is an
// internal audit record and is never part of the public contract. Value is a
// typed JSON scalar rather than the desktop-only valueJson transport string.
type publicMetadata struct {
	ID               string          `json:"id"`
	RepresentationID string          `json:"representationId,omitempty"`
	Namespace        string          `json:"namespace"`
	Key              string          `json:"key"`
	ValueType        string          `json:"valueType"`
	Value            json.RawMessage `json:"value"`
	Language         string          `json:"language,omitempty"`
	Position         int             `json:"position"`
	Source           string          `json:"source"`
	Confidence       *float64        `json:"confidence,omitempty"`
	Locked           bool            `json:"locked"`
}

// publicTag omits Catalog ownership and normalized lookup fields. Those fields
// are desktop implementation details and are not needed to render or filter a
// remote Library client.
type publicTag struct {
	ID        string `json:"id"`
	Name      string `json:"name"`
	CreatedAt string `json:"createdAt,omitempty"`
	UpdatedAt string `json:"updatedAt,omitempty"`
}

// publicCollection is a presentation contract, not a desktop Catalog DTO.
// SmartQuery is deliberately absent: it is an opaque executable expression
// and may contain paths, provider internals or credentials.
type publicCollection struct {
	ID               string   `json:"id"`
	Name             string   `json:"name"`
	Description      string   `json:"description,omitempty"`
	Kind             string   `json:"kind"`
	Revision         int64    `json:"revision"`
	ItemIDs          []string `json:"itemIds"`
	ItemIDsTruncated bool     `json:"itemIdsTruncated,omitempty"`
	CreatedAt        string   `json:"createdAt,omitempty"`
	UpdatedAt        string   `json:"updatedAt,omitempty"`
}

type publicCollectionItem struct {
	ID       string `json:"id"`
	ItemID   string `json:"itemId"`
	Position int    `json:"position"`
}

type publicCollectionItemsPage struct {
	CollectionID string                 `json:"collectionId"`
	Items        []publicCollectionItem `json:"items"`
	NextOffset   int                    `json:"nextOffset"`
	HasMore      bool                   `json:"hasMore"`
}

// publicChange omits CatalogID and ActorID and uses explicit JSON field names.
// In particular, the actor is an opaque internal identifier and must never be
// allowed to carry a token or a path into the public change feed.
type publicChange struct {
	Sequence   int64  `json:"sequence"`
	EntityType string `json:"entityType"`
	EntityID   string `json:"entityId"`
	Kind       string `json:"kind"`
	Revision   int64  `json:"revision"`
	OccurredAt string `json:"occurredAt"`
}

type publicChangeList struct {
	Changes []publicChange `json:"changes"`
	Epoch   string         `json:"epoch"`
	Next    int64          `json:"next"`
	HasMore bool           `json:"hasMore"`
}

type publicSyncState struct {
	Epoch  string `json:"epoch"`
	Cursor int64  `json:"cursor"`
}

type publicLibraryOverview struct {
	// Keep the mobile/public v1 payload explicit. Desktop-only overview fields
	// must not silently expand this versioned contract through struct embedding.
	Catalog      dto.CatalogDTO            `json:"catalog"`
	Categories   dto.CatalogCountDTO       `json:"categories"`
	Statuses     dto.CatalogStatusCountDTO `json:"statuses"`
	Health       dto.CatalogHealthCountDTO `json:"health"`
	Sync         publicSyncState           `json:"sync"`
	Capabilities []string                  `json:"capabilities"`
}

type publicResetRequired struct {
	Error string          `json:"error"`
	Sync  publicSyncState `json:"sync"`
}

func newPublicSyncState(state library.CatalogSyncState) publicSyncState {
	return publicSyncState{Epoch: state.Epoch, Cursor: state.Cursor}
}

type publicItemDetail struct {
	Item            dto.CatalogItemDTO     `json:"item"`
	Assets          []publicAsset          `json:"assets"`
	Representations []publicRepresentation `json:"representations"`
	Metadata        []publicMetadata       `json:"metadata"`
	Tags            []publicTag            `json:"tags"`
}

func (api *BusinessAPI) getItem(w http.ResponseWriter, request *http.Request) {
	// A device grant is not yet bound to a XiaDown user identity. Never accept
	// a caller-selected user ID: doing so would let an otherwise valid device
	// probe another user's private playback or reading state.
	detail, err := api.config.Catalog.GetCatalogItem(request.Context(), dto.GetCatalogItemRequest{
		ID: request.PathValue("id"),
	})
	if err != nil {
		writeBusinessError(w, err)
		return
	}
	result := publicItemDetail{
		Item:            detail.Item,
		Assets:          make([]publicAsset, 0, len(detail.Assets)),
		Representations: make([]publicRepresentation, 0, len(detail.Representations)),
		Metadata:        make([]publicMetadata, 0, len(detail.Metadata)),
		Tags:            make([]publicTag, 0, len(detail.Tags)),
	}
	itemTrashed := strings.EqualFold(strings.TrimSpace(detail.Item.Status), string(library.ItemStatusTrashed))
	for _, asset := range detail.Assets {
		fileAvailable := asset.FileAvailable && !itemTrashed
		item := publicAsset{
			ID: asset.ID, FileID: asset.FileID, Role: asset.Role, Label: asset.Label,
			Position: asset.Position, FileAvailable: fileAvailable,
		}
		if fileAvailable && asset.File != nil && strings.TrimSpace(asset.File.Storage.LocalPath) != "" {
			// This href is relative to the selected transport base. A LAN client
			// resolves it below https://host:port/, while a Tailscale client keeps
			// its managed /xiadown prefix instead of jumping to the origin root.
			item.ContentURL = "api/v1/library/assets/" + asset.ID + "/content"
		}
		result.Assets = append(result.Assets, item)
	}
	for _, representation := range detail.Representations {
		if item, ok := sanitizePublicRepresentation(representation); ok {
			result.Representations = append(result.Representations, item)
		}
	}
	for _, metadata := range detail.Metadata {
		if item, ok := sanitizePublicMetadata(metadata); ok {
			result.Metadata = append(result.Metadata, item)
		}
	}
	for _, tag := range detail.Tags {
		if item, ok := sanitizePublicTag(tag); ok {
			result.Tags = append(result.Tags, item)
		}
	}
	writeJSON(w, http.StatusOK, result)
}

func sanitizePublicRepresentation(item dto.CatalogRepresentationDTO) (publicRepresentation, bool) {
	if !safePublicOpaqueID(item.ID) || !safePublicOpaqueID(item.AssetID) ||
		!publicRepresentationKind(item.Kind) || !publicRepresentationPurpose(item.Purpose) ||
		!publicRepresentationAvailability(item.Availability) ||
		!validPublicPositiveInt(item.Width) || !validPublicPositiveInt(item.Height) ||
		!validPublicNonNegativeInt64(item.DurationMs) || !validPublicPositiveInt64(item.BitrateBps) ||
		!validPublicNonNegativeInt64(item.SizeBytes) {
		return publicRepresentation{}, false
	}
	mediaType := strings.ToLower(strings.TrimSpace(item.MediaType))
	if mediaType != "" {
		parsed, parameters, err := mime.ParseMediaType(mediaType)
		if err != nil || len(parameters) != 0 || parsed != mediaType {
			mediaType = ""
		}
	}
	container := safePublicTechnicalToken(item.Container)
	codec := safePublicTechnicalToken(item.Codec)
	language := safePublicLanguage(item.Language)
	return publicRepresentation{
		ID: item.ID, AssetID: item.AssetID, Kind: item.Kind, Purpose: item.Purpose,
		MediaType: mediaType, Container: container, Codec: codec,
		Width: item.Width, Height: item.Height, DurationMs: item.DurationMs,
		BitrateBps: item.BitrateBps, Language: language,
		SizeBytes: item.SizeBytes, Availability: item.Availability,
	}, true
}

func publicRepresentationKind(value string) bool {
	switch library.RepresentationKind(value) {
	case library.RepresentationKindOriginal, library.RepresentationKindOptimized,
		library.RepresentationKindThumbnail, library.RepresentationKindTranscript,
		library.RepresentationKindSubtitle, library.RepresentationKindArtwork,
		library.RepresentationKindPreview, library.RepresentationKindAttachment:
		return true
	default:
		return false
	}
}

func publicRepresentationPurpose(value string) bool {
	switch library.RepresentationPurpose(value) {
	case library.RepresentationPurposePrimary, library.RepresentationPurposePlayback,
		library.RepresentationPurposePreview, library.RepresentationPurposeAccessibility,
		library.RepresentationPurposeArtwork, library.RepresentationPurposeAttachment,
		library.RepresentationPurposeIndexing:
		return true
	default:
		return false
	}
}

func publicRepresentationAvailability(value string) bool {
	switch library.RepresentationAvailability(value) {
	case library.RepresentationAvailabilityAvailable, library.RepresentationAvailabilityProcessing,
		library.RepresentationAvailabilityOffline, library.RepresentationAvailabilityMissing,
		library.RepresentationAvailabilityCorrupt:
		return true
	default:
		return false
	}
}

var publicMetadataNamespaces = map[string]struct{}{
	"audio": {}, "book": {}, "dc": {}, "dc.terms": {}, "dcterms": {},
	"image": {}, "media": {}, "music": {}, "video": {},
	"xiadown.descriptive": {}, "xiadown.media": {},
}

var publicMetadataKeys = map[string]struct{}{
	"album": {}, "album_artist": {}, "alternative": {}, "artist": {}, "author": {},
	"available": {}, "chapter_count": {}, "composer": {}, "contributor": {}, "created": {},
	"creator": {}, "date": {}, "description": {}, "disc_number": {}, "duration": {},
	"duration_ms": {}, "episode": {}, "extent": {}, "format": {}, "genre": {},
	"identifier": {}, "isbn": {}, "issued": {}, "language": {}, "license": {},
	"medium": {}, "modified": {}, "narrator": {}, "orientation": {}, "page_count": {},
	"publisher": {}, "rating": {}, "release_date": {}, "rights": {}, "season": {},
	"series": {}, "series_index": {}, "sort_title": {}, "subject": {}, "subtitle": {},
	"title": {}, "track": {}, "track_number": {}, "type": {}, "valid": {}, "year": {},
}

var (
	publicMetadataLocalPathPattern = regexp.MustCompile(`(?i)(?:file://|(?:^|[\s"'(=])/(?:[^/\s]+/)+[^/\s]+|(?:^|[\s"'(=])[a-z]:[\\/]|\\\\[^\\/\s]+[\\/][^\\/\s]+)`)
	publicMetadataHashPattern      = regexp.MustCompile(`(?i)(?:^|[^0-9a-f])(?:[0-9a-f]{40}|[0-9a-f]{64})(?:$|[^0-9a-f])`)
	publicMetadataJWTLikePattern   = regexp.MustCompile(`[A-Za-z0-9_-]{16,}\.[A-Za-z0-9_-]{16,}\.[A-Za-z0-9_-]{16,}`)
)

func sanitizePublicMetadata(item dto.CatalogMetadataEntryDTO) (publicMetadata, bool) {
	namespace := strings.ToLower(strings.TrimSpace(item.Namespace))
	key := strings.ToLower(strings.TrimSpace(item.Key))
	if _, ok := publicMetadataNamespaces[namespace]; !ok {
		return publicMetadata{}, false
	}
	if _, ok := publicMetadataKeys[key]; !ok {
		return publicMetadata{}, false
	}
	valueType := strings.ToLower(strings.TrimSpace(item.ValueType))
	value, ok := sanitizePublicMetadataValue(valueType, item.ValueJSON)
	if !ok || !safePublicOpaqueID(item.ID) ||
		item.RepresentationID != "" && !safePublicOpaqueID(item.RepresentationID) ||
		item.Position < 0 || !publicMetadataSource(item.Source) ||
		item.Confidence != nil && (math.IsNaN(*item.Confidence) || math.IsInf(*item.Confidence, 0) || *item.Confidence < 0 || *item.Confidence > 1) {
		return publicMetadata{}, false
	}
	return publicMetadata{
		ID: item.ID, RepresentationID: item.RepresentationID,
		Namespace: namespace, Key: key, ValueType: valueType, Value: value,
		Language: safePublicLanguage(item.Language), Position: item.Position, Source: item.Source,
		Confidence: item.Confidence, Locked: item.Locked,
	}, true
}

func sanitizePublicMetadataValue(valueType, raw string) (json.RawMessage, bool) {
	if !json.Valid([]byte(raw)) {
		return nil, false
	}
	decoder := json.NewDecoder(strings.NewReader(raw))
	decoder.UseNumber()
	var decoded any
	if err := decoder.Decode(&decoded); err != nil {
		return nil, false
	}
	switch library.MetadataValueType(valueType) {
	case library.MetadataValueString:
		text, ok := decoded.(string)
		if !ok || !safePublicMetadataText(text) {
			return nil, false
		}
	case library.MetadataValueInteger, library.MetadataValueDurationMs:
		number, ok := decoded.(json.Number)
		if !ok {
			return nil, false
		}
		integer, err := strconv.ParseInt(number.String(), 10, 64)
		if err != nil || library.MetadataValueType(valueType) == library.MetadataValueDurationMs && integer < 0 {
			return nil, false
		}
	case library.MetadataValueNumber:
		number, ok := decoded.(json.Number)
		if !ok {
			return nil, false
		}
		value, err := strconv.ParseFloat(number.String(), 64)
		if err != nil || math.IsNaN(value) || math.IsInf(value, 0) {
			return nil, false
		}
	case library.MetadataValueBoolean:
		if _, ok := decoded.(bool); !ok {
			return nil, false
		}
	case library.MetadataValueDate, library.MetadataValueDateTime:
		text, ok := decoded.(string)
		if !ok {
			return nil, false
		}
		layout := "2006-01-02"
		if library.MetadataValueType(valueType) == library.MetadataValueDateTime {
			layout = time.RFC3339Nano
		}
		if _, err := time.Parse(layout, text); err != nil {
			return nil, false
		}
	default:
		return nil, false
	}
	return json.RawMessage(append([]byte(nil), raw...)), true
}

func publicMetadataSource(value string) bool {
	switch library.MetadataSource(value) {
	case library.MetadataSourceUser, library.MetadataSourceEmbedded, library.MetadataSourceSidecar,
		library.MetadataSourceRemote, library.MetadataSourceDerived, library.MetadataSourceMigration,
		library.MetadataSourceSystem:
		return true
	default:
		return false
	}
}

func safePublicMetadataText(value string) bool {
	if len(value) > 16<<10 || publicMetadataLocalPathPattern.MatchString(value) ||
		publicMetadataHashPattern.MatchString(value) || publicMetadataJWTLikePattern.MatchString(value) {
		return false
	}
	for _, char := range value {
		if unicode.IsControl(char) && char != '\n' && char != '\r' && char != '\t' {
			return false
		}
	}
	lower := strings.ToLower(value)
	for _, marker := range []string{
		"authorization:", "authorization=", "bearer ", "access_token", "access-token",
		"refresh_token", "refresh-token", "token=", "token:", "api_key", "api-key", "password=", "password:",
		"passwd=", "passwd:", "private_key", "private-key", "client_secret", "client-secret",
		"secret=", "secret:", "cookie:", "cookie=", "hash=", "hash:",
		"checksum=", "checksum:", "sha256:", "-----begin private key-----",
	} {
		if strings.Contains(lower, marker) {
			return false
		}
	}
	return true
}

func safePublicOpaqueID(value string) bool {
	value = strings.TrimSpace(value)
	if value == "" || len(value) > 255 || !safePublicMetadataText(value) {
		return false
	}
	for _, char := range value {
		if unicode.IsSpace(char) || unicode.IsControl(char) {
			return false
		}
	}
	return true
}

func safePublicTechnicalToken(value string) string {
	value = strings.ToLower(strings.TrimSpace(value))
	if value == "" || len(value) > 127 {
		return ""
	}
	for _, char := range value {
		if char != '.' && char != '_' && char != '+' && char != '-' &&
			(char < 'a' || char > 'z') && (char < '0' || char > '9') {
			return ""
		}
	}
	return value
}

func safePublicLanguage(value string) string {
	value = strings.TrimSpace(value)
	if value == "" || len(value) > 63 || strings.HasPrefix(value, "-") || strings.HasSuffix(value, "-") {
		return ""
	}
	for _, char := range value {
		if char != '-' && !unicode.IsLetter(char) && !unicode.IsDigit(char) {
			return ""
		}
	}
	return value
}

func safePublicDisplayText(value string) string {
	value = strings.TrimSpace(value)
	if value == "" || !safePublicMetadataText(value) {
		return ""
	}
	return value
}

func safePublicTimestamp(value string) string {
	parsed, err := time.Parse(time.RFC3339Nano, strings.TrimSpace(value))
	if err != nil {
		return ""
	}
	return parsed.UTC().Format(time.RFC3339Nano)
}

func sanitizePublicTag(item dto.CatalogTagDTO) (publicTag, bool) {
	name := safePublicDisplayText(item.Name)
	if !safePublicOpaqueID(item.ID) || name == "" {
		return publicTag{}, false
	}
	return publicTag{
		ID: item.ID, Name: name,
		CreatedAt: safePublicTimestamp(item.CreatedAt), UpdatedAt: safePublicTimestamp(item.UpdatedAt),
	}, true
}

func sanitizePublicCollection(item dto.CatalogCollectionDTO) (publicCollection, bool) {
	name := safePublicDisplayText(item.Name)
	kind := strings.ToLower(strings.TrimSpace(item.Kind))
	if !safePublicOpaqueID(item.ID) || name == "" || item.Revision < 1 || !publicCollectionKind(kind) {
		return publicCollection{}, false
	}
	description := safePublicDisplayText(item.Description)
	capacity := len(item.ItemIDs)
	if capacity > maxPublicCollectionItemIDs {
		capacity = maxPublicCollectionItemIDs
	}
	itemIDs := make([]string, 0, capacity)
	truncated := item.ItemIDsTruncated
	for _, itemID := range item.ItemIDs {
		if safePublicOpaqueID(itemID) {
			if len(itemIDs) == maxPublicCollectionItemIDs {
				truncated = true
				break
			}
			itemIDs = append(itemIDs, strings.TrimSpace(itemID))
		}
	}
	return publicCollection{
		ID: item.ID, Name: name, Description: description, Kind: kind,
		Revision: item.Revision, ItemIDs: itemIDs, ItemIDsTruncated: truncated,
		CreatedAt: safePublicTimestamp(item.CreatedAt), UpdatedAt: safePublicTimestamp(item.UpdatedAt),
	}, true
}

func publicCollectionKind(value string) bool {
	switch library.CollectionKind(value) {
	case library.CollectionKindManual, library.CollectionKindSmart, library.CollectionKindPlaylist,
		library.CollectionKindAlbum, library.CollectionKindShelf, library.CollectionKindSeries:
		return true
	default:
		return false
	}
}

func sanitizePublicChange(item library.CatalogChange) (publicChange, bool) {
	entityType := string(item.EntityType)
	kind := string(item.Kind)
	if item.Sequence < 1 || item.Revision < 1 || item.OccurredAt.IsZero() ||
		!safePublicOpaqueID(item.EntityID) || !publicChangeEntityType(item.EntityType) ||
		(kind != string(library.CatalogChangeUpsert) && kind != string(library.CatalogChangeDelete)) {
		return publicChange{}, false
	}
	return publicChange{
		Sequence: item.Sequence, EntityType: entityType, EntityID: strings.TrimSpace(item.EntityID),
		Kind: kind, Revision: item.Revision, OccurredAt: item.OccurredAt.UTC().Format(time.RFC3339Nano),
	}, true
}

func publicChangeEntityType(value library.CatalogEntityType) bool {
	switch value {
	case library.CatalogEntityCatalog, library.CatalogEntityItem,
		library.CatalogEntityCollection, library.CatalogEntityTag:
		return true
	default:
		// Nested rows are private persistence details. Their transactions append
		// a durable owning Item/Collection invalidation that public clients can
		// consume through an existing GET. User state and administration entities
		// are deliberately private as device grants are not user-bound.
		return false
	}
}

func validPublicPositiveInt(value *int) bool {
	return value == nil || *value > 0
}

func validPublicNonNegativeInt64(value *int64) bool {
	return value == nil || *value >= 0
}

func validPublicPositiveInt64(value *int64) bool {
	return value == nil || *value > 0
}

func (api *BusinessAPI) assetContent(w http.ResponseWriter, request *http.Request) {
	asset, err := api.config.Assets.Get(request.Context(), strings.TrimSpace(request.PathValue("id")))
	if err != nil {
		writeBusinessError(w, err)
		return
	}
	item, err := api.config.Items.Get(request.Context(), asset.ItemID)
	if err != nil || item.CatalogID != api.config.CatalogID || item.Status == library.ItemStatusTrashed {
		if err == nil {
			err = sql.ErrNoRows
		}
		writeBusinessError(w, err)
		return
	}
	file, err := api.config.Files.Get(request.Context(), asset.FileID)
	if err != nil {
		writeBusinessError(w, err)
		return
	}
	path := strings.TrimSpace(file.Storage.LocalPath)
	if path == "" || publicLibraryFileUnavailable(file) {
		writeError(w, http.StatusNotFound, "asset_unavailable")
		return
	}
	opened, err := openPublicAssetNoFollow(path)
	if err != nil {
		writeBusinessError(w, err)
		return
	}
	defer opened.Close()
	info, err := opened.Stat()
	if err != nil || !info.Mode().IsRegular() {
		writeError(w, http.StatusNotFound, "asset_unavailable")
		return
	}
	if contentType := mime.TypeByExtension(strings.ToLower(filepath.Ext(info.Name()))); contentType != "" {
		w.Header().Set("Content-Type", contentType)
	}
	w.Header().Set("Accept-Ranges", "bytes")
	w.Header().Set("Content-Disposition", `inline; filename="`+safeFilename(info.Name())+`"`)
	http.ServeContent(w, request, info.Name(), info.ModTime(), opened)
}

func publicLibraryFileUnavailable(file library.LibraryFile) bool {
	if file.State.Deleted || strings.TrimSpace(file.State.LastError) != "" {
		return true
	}
	switch strings.ToLower(strings.TrimSpace(file.State.Status)) {
	case "deleted", "missing", "offline", "error", "unavailable":
		return true
	default:
		return false
	}
}

func (api *BusinessAPI) collections(w http.ResponseWriter, request *http.Request) {
	limit, offset, paginationErr := publicTaxonomyPagination(request)
	if paginationErr != nil || limit > maxPublicCollectionPageSize {
		writeError(w, http.StatusBadRequest, "invalid_pagination")
		return
	}
	var items []dto.CatalogCollectionDTO
	var err error
	alreadyPaged := false
	if pager, ok := api.config.Catalog.(catalogPageReader); ok {
		items, err = pager.ListCatalogCollectionsPage(request.Context(), limit, offset, maxPublicCollectionItemIDs)
		alreadyPaged = true
	} else {
		items, err = api.config.Catalog.ListCatalogCollections(request.Context())
	}
	if err != nil {
		writeBusinessError(w, err)
		return
	}
	result := make([]publicCollection, 0, min(limit, len(items)))
	validOffset := 0
	if alreadyPaged {
		validOffset = offset
	}
	for _, item := range items {
		if value, ok := sanitizePublicCollection(item); ok {
			if validOffset < offset {
				validOffset++
				continue
			}
			result = append(result, value)
			if len(result) == limit {
				break
			}
		}
	}
	writeJSON(w, http.StatusOK, result)
}

func (api *BusinessAPI) tags(w http.ResponseWriter, request *http.Request) {
	limit, offset, paginationErr := publicTaxonomyPagination(request)
	if paginationErr != nil {
		writeError(w, http.StatusBadRequest, "invalid_pagination")
		return
	}
	var items []dto.CatalogTagDTO
	var err error
	alreadyPaged := false
	if pager, ok := api.config.Catalog.(catalogPageReader); ok {
		items, err = pager.ListCatalogTagsPage(request.Context(), limit, offset)
		alreadyPaged = true
	} else {
		items, err = api.config.Catalog.ListCatalogTags(request.Context())
	}
	if err != nil {
		writeBusinessError(w, err)
		return
	}
	result := make([]publicTag, 0, min(limit, len(items)))
	validOffset := 0
	if alreadyPaged {
		validOffset = offset
	}
	for _, item := range items {
		if value, ok := sanitizePublicTag(item); ok {
			if validOffset < offset {
				validOffset++
				continue
			}
			result = append(result, value)
			if len(result) == limit {
				break
			}
		}
	}
	writeJSON(w, http.StatusOK, result)
}

func (api *BusinessAPI) collectionItems(w http.ResponseWriter, request *http.Request) {
	limit, offset, paginationErr := publicTaxonomyPagination(request)
	if paginationErr != nil {
		writeError(w, http.StatusBadRequest, "invalid_pagination")
		return
	}
	collectionID := strings.TrimSpace(request.PathValue("id"))
	if !safePublicOpaqueID(collectionID) {
		writeError(w, http.StatusNotFound, "not_found")
		return
	}
	page, err := api.config.Catalog.ListCatalogCollectionItemsPage(request.Context(), collectionID, limit, offset)
	if err != nil {
		writeBusinessError(w, err)
		return
	}
	if strings.TrimSpace(page.CatalogID) != api.config.CatalogID || strings.TrimSpace(page.CollectionID) != collectionID ||
		page.NextOffset < offset || page.NextOffset > offset+limit || len(page.Items) > limit {
		writeBusinessError(w, errors.New("invalid Catalog collection member page"))
		return
	}
	result := publicCollectionItemsPage{
		CollectionID: collectionID, Items: make([]publicCollectionItem, 0, len(page.Items)),
		NextOffset: page.NextOffset, HasMore: page.HasMore,
	}
	for _, member := range page.Items {
		if strings.TrimSpace(member.CollectionID) != collectionID || !safePublicOpaqueID(member.ID) ||
			!safePublicOpaqueID(member.ItemID) || member.Position < 0 {
			continue
		}
		result.Items = append(result.Items, publicCollectionItem{
			ID: strings.TrimSpace(member.ID), ItemID: strings.TrimSpace(member.ItemID), Position: member.Position,
		})
	}
	writeJSON(w, http.StatusOK, result)
}

func publicTaxonomyPagination(request *http.Request) (int, int, error) {
	limit, limitErr := optionalInteger(request.URL.Query().Get("limit"))
	offset, offsetErr := optionalInteger(request.URL.Query().Get("offset"))
	if limitErr != nil || offsetErr != nil || limit < 0 || limit > maxPublicTaxonomyPageSize || offset < 0 {
		return 0, 0, errors.New("invalid taxonomy pagination")
	}
	if limit == 0 {
		limit = defaultPublicTaxonomyPageSize
	}
	return limit, offset, nil
}

func (api *BusinessAPI) changes(w http.ResponseWriter, request *http.Request) {
	after, err := optionalInt64(request.URL.Query().Get("after"))
	if err != nil || after < 0 {
		writeError(w, http.StatusBadRequest, "invalid_cursor")
		return
	}
	limit, err := optionalInteger(request.URL.Query().Get("limit"))
	if err != nil || limit < 0 || limit > 500 {
		writeError(w, http.StatusBadRequest, "invalid_pagination")
		return
	}
	if limit == 0 {
		limit = 200
	}
	syncState, err := api.config.Sync.GetCatalogSyncState(request.Context(), api.config.CatalogID)
	if err != nil {
		writeBusinessError(w, err)
		return
	}
	if strings.TrimSpace(request.URL.Query().Get("epoch")) != syncState.Epoch || after > syncState.Cursor {
		writeLibraryResetRequired(w, syncState)
		return
	}
	items, err := api.config.Changes.ListAfter(request.Context(), api.config.CatalogID, after, limit)
	if err != nil {
		writeBusinessError(w, err)
		return
	}
	next := after
	if len(items) > 0 {
		next = items[len(items)-1].Sequence
	}
	result := publicChangeList{
		Changes: make([]publicChange, 0, len(items)), Epoch: syncState.Epoch,
		Next: next, HasMore: len(items) == limit,
	}
	for _, item := range items {
		if value, ok := sanitizePublicChange(item); ok {
			result.Changes = append(result.Changes, value)
		}
	}
	writeJSON(w, http.StatusOK, result)
}

func writeBusinessError(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, sql.ErrNoRows), errors.Is(err, os.ErrNotExist):
		writeError(w, http.StatusNotFound, "not_found")
	case errors.Is(err, library.ErrInvalidCatalogItem):
		writeError(w, http.StatusBadRequest, "invalid_request")
	default:
		writeError(w, http.StatusInternalServerError, "internal_error")
	}
}

func optionalInteger(value string) (int, error) {
	if strings.TrimSpace(value) == "" {
		return 0, nil
	}
	return strconv.Atoi(value)
}

func optionalInt64(value string) (int64, error) {
	if strings.TrimSpace(value) == "" {
		return 0, nil
	}
	return strconv.ParseInt(value, 10, 64)
}

func safeFilename(value string) string {
	value = strings.ReplaceAll(filepath.Base(value), `"`, "")
	value = strings.ReplaceAll(value, "\r", "")
	value = strings.ReplaceAll(value, "\n", "")
	if value == "" || value == "." {
		return "asset"
	}
	return value
}
