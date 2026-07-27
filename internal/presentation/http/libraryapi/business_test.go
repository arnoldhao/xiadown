package libraryapi

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"xiadown/internal/application/library/access"
	"xiadown/internal/application/library/dto"
	"xiadown/internal/domain/library"
)

func TestPublicContentURLResolvesBelowLANAndTailscaleTransportBases(t *testing.T) {
	href, err := url.Parse("api/v1/library/assets/asset-1/content")
	if err != nil {
		t.Fatal(err)
	}
	for base, want := range map[string]string{
		"https://192.168.1.20:43127/":            "https://192.168.1.20:43127/api/v1/library/assets/asset-1/content",
		"https://studio.example.ts.net/xiadown/": "https://studio.example.ts.net/xiadown/api/v1/library/assets/asset-1/content",
	} {
		transport, parseErr := url.Parse(base)
		if parseErr != nil {
			t.Fatal(parseErr)
		}
		if got := transport.ResolveReference(href).String(); got != want {
			t.Fatalf("resolve %q below %q = %q, want %q", href, base, got, want)
		}
	}
}

type businessCatalogStub struct {
	detail      dto.CatalogItemDetailDTO
	list        dto.ListCatalogItemsResponse
	collections []dto.CatalogCollectionDTO
	tags        []dto.CatalogTagDTO
	lastGet     *dto.GetCatalogItemRequest
}

type recordingBusinessCatalog struct {
	businessCatalogStub
	listRequest dto.ListCatalogItemsRequest
	listCalls   int
}

type businessSnapshotCatalogStub struct {
	businessCatalogStub
	items           []dto.CatalogItemDTO
	requestedAfter  []string
	requestedLimits []int
}

func (stub *businessSnapshotCatalogStub) ListCatalogSnapshotItems(
	_ context.Context,
	catalogID string,
	afterID string,
	limit int,
) ([]dto.CatalogItemDTO, error) {
	stub.requestedAfter = append(stub.requestedAfter, afterID)
	stub.requestedLimits = append(stub.requestedLimits, limit)
	result := make([]dto.CatalogItemDTO, 0, limit)
	for _, item := range stub.items {
		if item.CatalogID != catalogID || item.ID <= afterID {
			continue
		}
		result = append(result, item)
		if len(result) == limit {
			break
		}
	}
	return result, nil
}

func (stub *recordingBusinessCatalog) ListCatalogItems(_ context.Context, request dto.ListCatalogItemsRequest) (dto.ListCatalogItemsResponse, error) {
	stub.listCalls++
	stub.listRequest = request
	return stub.list, nil
}

func (stub businessCatalogStub) GetDefaultCatalogOverview(context.Context) (dto.CatalogOverviewDTO, error) {
	return dto.CatalogOverviewDTO{}, nil
}
func (stub businessCatalogStub) ListCatalogItems(context.Context, dto.ListCatalogItemsRequest) (dto.ListCatalogItemsResponse, error) {
	return stub.list, nil
}
func (stub businessCatalogStub) GetCatalogItem(_ context.Context, request dto.GetCatalogItemRequest) (dto.CatalogItemDetailDTO, error) {
	if stub.lastGet != nil {
		*stub.lastGet = request
	}
	return stub.detail, nil
}
func (stub businessCatalogStub) ListCatalogCollections(context.Context) ([]dto.CatalogCollectionDTO, error) {
	return append([]dto.CatalogCollectionDTO(nil), stub.collections...), nil
}
func (stub businessCatalogStub) ListCatalogTags(context.Context) ([]dto.CatalogTagDTO, error) {
	return append([]dto.CatalogTagDTO(nil), stub.tags...), nil
}
func (stub businessCatalogStub) ListCatalogCollectionsPage(_ context.Context, limit, offset, memberLimit int) ([]dto.CatalogCollectionDTO, error) {
	if offset >= len(stub.collections) {
		return []dto.CatalogCollectionDTO{}, nil
	}
	items := append([]dto.CatalogCollectionDTO(nil), stub.collections[offset:min(offset+limit, len(stub.collections))]...)
	for index := range items {
		items[index].ItemIDs = append([]string(nil), items[index].ItemIDs...)
		if len(items[index].ItemIDs) > memberLimit {
			items[index].ItemIDs = items[index].ItemIDs[:memberLimit]
			items[index].ItemIDsTruncated = true
		}
	}
	return items, nil
}
func (stub businessCatalogStub) ListCatalogTagsPage(_ context.Context, limit, offset int) ([]dto.CatalogTagDTO, error) {
	if offset >= len(stub.tags) {
		return []dto.CatalogTagDTO{}, nil
	}
	return append([]dto.CatalogTagDTO(nil), stub.tags[offset:min(offset+limit, len(stub.tags))]...), nil
}
func (stub businessCatalogStub) ListCatalogCollectionItemsPage(_ context.Context, collectionID string, limit, offset int) (dto.CatalogCollectionItemsPageDTO, error) {
	for _, collection := range stub.collections {
		if collection.ID != collectionID {
			continue
		}
		catalogID := strings.TrimSpace(collection.CatalogID)
		if catalogID == "" {
			catalogID = "catalog-1"
		}
		if catalogID != "catalog-1" {
			return dto.CatalogCollectionItemsPageDTO{}, sql.ErrNoRows
		}
		if offset >= len(collection.ItemIDs) {
			return dto.CatalogCollectionItemsPageDTO{
				CatalogID: catalogID, CollectionID: collectionID,
				Items: []dto.CatalogCollectionItemDTO{}, NextOffset: offset,
			}, nil
		}
		end := min(offset+limit, len(collection.ItemIDs))
		items := make([]dto.CatalogCollectionItemDTO, 0, end-offset)
		for position := offset; position < end; position++ {
			items = append(items, dto.CatalogCollectionItemDTO{
				ID: fmt.Sprintf("%s-member-%06d", collectionID, position), CollectionID: collectionID,
				ItemID: collection.ItemIDs[position], Position: position,
			})
		}
		return dto.CatalogCollectionItemsPageDTO{
			CatalogID: catalogID, CollectionID: collectionID, Items: items,
			NextOffset: end, HasMore: end < len(collection.ItemIDs),
		}, nil
	}
	return dto.CatalogCollectionItemsPageDTO{}, sql.ErrNoRows
}

type businessItemRepoStub struct{ item library.Item }

func (stub businessItemRepoStub) ListByCatalogID(context.Context, string) ([]library.Item, error) {
	return []library.Item{stub.item}, nil
}
func (stub businessItemRepoStub) Get(context.Context, string) (library.Item, error) {
	return stub.item, nil
}
func (businessItemRepoStub) Save(context.Context, library.Item) error { return nil }
func (businessItemRepoStub) Delete(context.Context, string) error     { return nil }

type businessAssetRepoStub struct{ asset library.ItemAsset }

func (stub businessAssetRepoStub) ListByItemID(context.Context, string) ([]library.ItemAsset, error) {
	return []library.ItemAsset{stub.asset}, nil
}
func (stub businessAssetRepoStub) Get(context.Context, string) (library.ItemAsset, error) {
	return stub.asset, nil
}
func (businessAssetRepoStub) Save(context.Context, library.ItemAsset) error { return nil }
func (businessAssetRepoStub) Delete(context.Context, string) error          { return nil }

type businessFileRepoStub struct{ file library.LibraryFile }

func (stub businessFileRepoStub) List(context.Context) ([]library.LibraryFile, error) {
	return []library.LibraryFile{stub.file}, nil
}
func (stub businessFileRepoStub) ListByLibraryID(context.Context, string) ([]library.LibraryFile, error) {
	return []library.LibraryFile{stub.file}, nil
}
func (stub businessFileRepoStub) Get(context.Context, string) (library.LibraryFile, error) {
	return stub.file, nil
}
func (businessFileRepoStub) Save(context.Context, library.LibraryFile) error { return nil }
func (businessFileRepoStub) Delete(context.Context, string) error            { return nil }

type businessChangeRepoStub struct{ changes []library.CatalogChange }

func (stub businessChangeRepoStub) ListAfter(_ context.Context, _ string, after int64, limit int) ([]library.CatalogChange, error) {
	result := make([]library.CatalogChange, 0, limit)
	for _, item := range stub.changes {
		if item.Sequence <= after {
			continue
		}
		result = append(result, item)
		if len(result) == limit {
			break
		}
	}
	return result, nil
}
func (businessChangeRepoStub) Save(context.Context, library.CatalogChange) error      { return nil }
func (businessChangeRepoStub) SaveTombstone(context.Context, library.Tombstone) error { return nil }
func (businessChangeRepoStub) DeleteExpiredTombstones(context.Context, time.Time) (int, error) {
	return 0, nil
}

const businessSyncEpoch = "0123456789abcdef0123456789abcdef"

type businessSyncRepoStub struct {
	state library.CatalogSyncState
	err   error
}

func (stub businessSyncRepoStub) GetCatalogSyncState(context.Context, string) (library.CatalogSyncState, error) {
	return stub.state, stub.err
}

type businessSequencedSyncRepoStub struct {
	states []library.CatalogSyncState
	calls  int
}

func (stub *businessSequencedSyncRepoStub) GetCatalogSyncState(context.Context, string) (library.CatalogSyncState, error) {
	if len(stub.states) == 0 {
		return library.CatalogSyncState{}, errors.New("missing sync fixture")
	}
	state := stub.states[min(stub.calls, len(stub.states)-1)]
	stub.calls++
	return state, nil
}

func TestPublicOverviewIncludesSyncGeneration(t *testing.T) {
	api := newBusinessTestAPI(t, "/tmp/movie.mp4")
	request := httptest.NewRequest(http.MethodGet, "/api/v1/library", nil)
	recorder := httptest.NewRecorder()
	api.overview(recorder, request)
	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d, body=%s", recorder.Code, recorder.Body.String())
	}
	var payload map[string]any
	if err := json.Unmarshal(recorder.Body.Bytes(), &payload); err != nil {
		t.Fatalf("decode public overview: %v", err)
	}
	assertExactJSONKeys(t, payload, "catalog", "categories", "statuses", "health", "sync", "capabilities")
	syncState, ok := payload["sync"].(map[string]any)
	if !ok {
		t.Fatalf("public overview sync = %#v", payload["sync"])
	}
	assertExactJSONKeys(t, syncState, "epoch", "cursor")
	if syncState["epoch"] != businessSyncEpoch || syncState["cursor"] != float64(100) {
		t.Fatalf("public overview sync = %#v", syncState)
	}
	capabilities, ok := payload["capabilities"].([]any)
	if !ok || len(capabilities) != 1 || capabilities[0] != LibrarySnapshotKeysetCapability {
		t.Fatalf("public overview capabilities = %#v", payload["capabilities"])
	}
}

func TestPublicLibrarySnapshotUsesOpaqueStableKeysetAndPathFreeSummaries(t *testing.T) {
	api := newBusinessTestAPI(t, "/Users/arnold/Private/never-public.mp4")
	base := api.config.Catalog.(businessCatalogStub)
	stub := &businessSnapshotCatalogStub{
		businessCatalogStub: base,
		items: []dto.CatalogItemDTO{
			{ID: "item-001", CatalogID: "catalog-1", Category: "audio", Status: "active", Availability: "available", Title: "One", SortTitle: "One", Revision: 1, PrimaryAssetID: "asset-001", ArtworkAssetID: "cover-001"},
			{ID: "item-002", CatalogID: "catalog-1", Category: "audio", Status: "missing", Availability: "missing", Title: "Two", SortTitle: "Two", Revision: 2, PrimaryAssetID: "asset-002", ArtworkAssetID: "cover-002"},
			{ID: "item-004", CatalogID: "catalog-1", Category: "video", Status: "active", Availability: "available", Title: "Four", SortTitle: "Four", Revision: 1, PrimaryAssetID: "asset-004", ArtworkAssetID: "cover-004"},
		},
	}
	api.config.Catalog = stub

	firstRecorder := httptest.NewRecorder()
	api.snapshot(firstRecorder, httptest.NewRequest(
		http.MethodGet,
		"/api/v1/library/snapshot?epoch="+businessSyncEpoch+"&highWater=100&limit=2",
		nil,
	))
	if firstRecorder.Code != http.StatusOK {
		t.Fatalf("first snapshot status=%d body=%s", firstRecorder.Code, firstRecorder.Body.String())
	}
	assertBodyOmits(t, firstRecorder.Body.String(), "/Users/arnold/Private", `"localPath"`, `"documentId"`)
	var first publicLibrarySnapshotPage
	if err := json.Unmarshal(firstRecorder.Body.Bytes(), &first); err != nil {
		t.Fatalf("decode first snapshot: %v", err)
	}
	if len(first.Items) != 2 || first.Items[0].ID != "item-001" || first.Items[1].ID != "item-002" ||
		!first.HasMore || first.Next == "" || first.Epoch != businessSyncEpoch || first.HighWater != 100 {
		t.Fatalf("first snapshot=%#v", first)
	}
	cursor, err := decodeLibrarySnapshotCursor(first.Next)
	if err != nil || cursor.AfterID != "item-002" || cursor.Epoch != businessSyncEpoch || cursor.HighWater != 100 {
		t.Fatalf("opaque cursor=%#v err=%v", cursor, err)
	}

	secondRecorder := httptest.NewRecorder()
	api.snapshot(secondRecorder, httptest.NewRequest(
		http.MethodGet,
		"/api/v1/library/snapshot?epoch="+businessSyncEpoch+"&highWater=100&limit=2&after="+url.QueryEscape(first.Next),
		nil,
	))
	var second publicLibrarySnapshotPage
	if secondRecorder.Code != http.StatusOK || json.Unmarshal(secondRecorder.Body.Bytes(), &second) != nil ||
		len(second.Items) != 1 || second.Items[0].ID != "item-004" || second.HasMore || second.Next != "" {
		t.Fatalf("second snapshot status=%d page=%#v body=%s", secondRecorder.Code, second, secondRecorder.Body.String())
	}
	if fmt.Sprint(stub.requestedAfter) != "[ item-002]" || fmt.Sprint(stub.requestedLimits) != "[3 3]" {
		t.Fatalf("keyset requests after=%#v limits=%#v", stub.requestedAfter, stub.requestedLimits)
	}
}

func TestPublicLibrarySnapshotResetsWhenSyncStateChangesDuringPageRead(t *testing.T) {
	api := newBusinessTestAPI(t, "/tmp/movie.mp4")
	now := time.Date(2026, 7, 22, 12, 0, 0, 0, time.UTC)
	api.config.Sync = &businessSequencedSyncRepoStub{states: []library.CatalogSyncState{
		{CatalogID: "catalog-1", Epoch: businessSyncEpoch, Cursor: 100, RotatedAt: now},
		{CatalogID: "catalog-1", Epoch: businessSyncEpoch, Cursor: 101, RotatedAt: now},
	}}
	recorder := httptest.NewRecorder()
	api.snapshot(recorder, httptest.NewRequest(
		http.MethodGet,
		"/api/v1/library/snapshot?epoch="+businessSyncEpoch+"&highWater=100&limit=2",
		nil,
	))
	if recorder.Code != http.StatusConflict || !strings.Contains(recorder.Body.String(), `"error":"reset_required"`) ||
		!strings.Contains(recorder.Body.String(), `"cursor":101`) || strings.Contains(recorder.Body.String(), `"items"`) {
		t.Fatalf("concurrent snapshot status=%d body=%s", recorder.Code, recorder.Body.String())
	}
}

func TestPublicLibrarySnapshotRejectsMalformedOrReboundCursors(t *testing.T) {
	api := newBusinessTestAPI(t, "/tmp/movie.mp4")
	bound, err := encodeLibrarySnapshotCursor(librarySnapshotCursor{
		Version: 1, Epoch: businessSyncEpoch, HighWater: 99, AfterID: "item-001",
	})
	if err != nil {
		t.Fatal(err)
	}
	for name, target := range map[string]string{
		"missing high water": "/api/v1/library/snapshot?epoch=" + businessSyncEpoch,
		"invalid epoch":      "/api/v1/library/snapshot?epoch=not-an-epoch&highWater=100",
		"malformed after":    "/api/v1/library/snapshot?epoch=" + businessSyncEpoch + "&highWater=100&after=not!base64",
		"rebound after":      "/api/v1/library/snapshot?epoch=" + businessSyncEpoch + "&highWater=100&after=" + url.QueryEscape(bound),
	} {
		t.Run(name, func(t *testing.T) {
			recorder := httptest.NewRecorder()
			api.snapshot(recorder, httptest.NewRequest(http.MethodGet, target, nil))
			if recorder.Code != http.StatusBadRequest || !strings.Contains(recorder.Body.String(), `"error":"invalid_cursor"`) {
				t.Fatalf("status=%d body=%s", recorder.Code, recorder.Body.String())
			}
		})
	}
}

func TestPublicLibrarySnapshotRouteRequiresLibraryReadScope(t *testing.T) {
	api := newBusinessTestAPI(t, "/tmp/movie.mp4")
	authenticator := &fakeAuthenticator{principal: access.Principal{
		GrantID: "grant-1", CatalogID: "catalog-1", Scopes: []library.DeviceScope{library.DeviceScopeTasksRead},
	}}
	router, err := NewRouter(Config{
		Version: "test", CatalogID: "catalog-1", Authenticator: authenticator,
		Pairer: &fakePairer{}, Routes: api.Routes(),
	})
	if err != nil {
		t.Fatalf("NewRouter: %v", err)
	}
	target := "/api/v1/library/snapshot?epoch=" + businessSyncEpoch + "&highWater=100"
	if recorder := performRequest(router, http.MethodGet, target, "", ""); recorder.Code != http.StatusUnauthorized {
		t.Fatalf("anonymous snapshot status=%d body=%s", recorder.Code, recorder.Body.String())
	}
	if recorder := performRequest(router, http.MethodGet, target, "Bearer tasks-only", ""); recorder.Code != http.StatusForbidden || !strings.Contains(recorder.Body.String(), "insufficient_scope") {
		t.Fatalf("tasks-only snapshot status=%d body=%s", recorder.Code, recorder.Body.String())
	}
	authenticator.principal.Scopes = []library.DeviceScope{library.DeviceScopeLibraryRead}
	if recorder := performRequest(router, http.MethodGet, target, "Bearer library-reader", ""); recorder.Code != http.StatusOK || !strings.Contains(recorder.Body.String(), `"items"`) {
		t.Fatalf("library-reader snapshot status=%d body=%s", recorder.Code, recorder.Body.String())
	}
}

func TestPublicItemDetailNeverSerializesLegacyPaths(t *testing.T) {
	secretPath := "/Users/arnold/Private/movie.mp4"
	api := newBusinessTestAPI(t, secretPath)
	request := httptest.NewRequest(http.MethodGet, "/api/v1/library/items/item-1", nil)
	request.SetPathValue("id", "item-1")
	recorder := httptest.NewRecorder()
	api.getItem(recorder, request)
	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d, body=%s", recorder.Code, recorder.Body.String())
	}
	body := recorder.Body.String()
	if strings.Contains(body, secretPath) || strings.Contains(body, "localPath") || strings.Contains(body, "documentId") {
		t.Fatalf("public item leaked a physical reference: %s", body)
	}
	if !strings.Contains(body, `"contentUrl":"api/v1/library/assets/asset-1/content"`) {
		t.Fatalf("public item missing opaque content URL: %s", body)
	}
	var payload map[string]any
	if err := json.Unmarshal(recorder.Body.Bytes(), &payload); err != nil {
		t.Fatalf("decode public detail: %v", err)
	}
	if _, ok := payload["item"].(map[string]any); !ok {
		t.Fatalf("legacy item member changed shape: %#v", payload["item"])
	}
	if assets, ok := payload["assets"].([]any); !ok || len(assets) != 1 {
		t.Fatalf("legacy assets member changed shape: %#v", payload["assets"])
	}
	if tags, ok := payload["tags"].([]any); !ok || len(tags) != 0 {
		t.Fatalf("legacy tags member changed shape: %#v", payload["tags"])
	}
	if representations, ok := payload["representations"].([]any); !ok || len(representations) != 0 {
		t.Fatalf("representations must be an additive empty array: %#v", payload["representations"])
	}
	if metadata, ok := payload["metadata"].([]any); !ok || len(metadata) != 0 {
		t.Fatalf("metadata must be an additive empty array: %#v", payload["metadata"])
	}
}

func TestPublicItemListRejectsPaginationOutsideContract(t *testing.T) {
	api := newBusinessTestAPI(t, "/tmp/movie.mp4")
	for _, target := range []string{
		"/api/v1/library/items?limit=-1",
		"/api/v1/library/items?limit=501",
		"/api/v1/library/items?offset=-1",
		"/api/v1/library/items?limit=not-a-number",
	} {
		t.Run(target, func(t *testing.T) {
			request := httptest.NewRequest(http.MethodGet, target, nil)
			recorder := httptest.NewRecorder()
			api.listItems(recorder, request)
			if recorder.Code != http.StatusBadRequest || !strings.Contains(recorder.Body.String(), `"error":"invalid_pagination"`) {
				t.Fatalf("status=%d body=%s", recorder.Code, recorder.Body.String())
			}
		})
	}
}

func TestPublicItemSearchQueryMatchesOpenAPIUnicodeLengthContract(t *testing.T) {
	api := newBusinessTestAPI(t, "/tmp/movie.mp4")
	stub := &recordingBusinessCatalog{businessCatalogStub: api.config.Catalog.(businessCatalogStub)}
	api.config.Catalog = stub
	validQuery := strings.Repeat("界", maxPublicSearchQueryLength)

	validRecorder := httptest.NewRecorder()
	api.listItems(validRecorder, httptest.NewRequest(http.MethodGet, "/api/v1/library/items?q="+url.QueryEscape(validQuery), nil))
	if validRecorder.Code != http.StatusOK || stub.listCalls != 1 || stub.listRequest.Query != validQuery {
		t.Fatalf("valid Unicode query status=%d calls=%d query=%q body=%s", validRecorder.Code, stub.listCalls, stub.listRequest.Query, validRecorder.Body.String())
	}

	for name, target := range map[string]string{
		"too many code points": "/api/v1/library/items?q=" + url.QueryEscape(validQuery+"界"),
		"invalid UTF-8":        "/api/v1/library/items?q=%FF",
	} {
		t.Run(name, func(t *testing.T) {
			recorder := httptest.NewRecorder()
			api.listItems(recorder, httptest.NewRequest(http.MethodGet, target, nil))
			if recorder.Code != http.StatusBadRequest || stub.listCalls != 1 || !strings.Contains(recorder.Body.String(), `"error":"invalid_request"`) {
				t.Fatalf("status=%d calls=%d body=%s", recorder.Code, stub.listCalls, recorder.Body.String())
			}
		})
	}
}

func TestPublicItemListForwardsExcludeTrashed(t *testing.T) {
	api := newBusinessTestAPI(t, "/tmp/movie.mp4")
	stub := &recordingBusinessCatalog{businessCatalogStub: api.config.Catalog.(businessCatalogStub)}
	api.config.Catalog = stub

	recorder := httptest.NewRecorder()
	api.listItems(recorder, httptest.NewRequest(
		http.MethodGet,
		"/api/v1/library/items?status=all&excludeTrashed=true",
		nil,
	))
	if recorder.Code != http.StatusOK || stub.listCalls != 1 {
		t.Fatalf("status=%d calls=%d body=%s", recorder.Code, stub.listCalls, recorder.Body.String())
	}
	if !stub.listRequest.ExcludeTrashed || stub.listRequest.Status != "all" {
		t.Fatalf("unexpected list request: %#v", stub.listRequest)
	}

	invalid := httptest.NewRecorder()
	api.listItems(invalid, httptest.NewRequest(
		http.MethodGet,
		"/api/v1/library/items?excludeTrashed=definitely",
		nil,
	))
	if invalid.Code != http.StatusBadRequest || stub.listCalls != 1 ||
		!strings.Contains(invalid.Body.String(), `"error":"invalid_request"`) {
		t.Fatalf("status=%d calls=%d body=%s", invalid.Code, stub.listCalls, invalid.Body.String())
	}
}

func TestPublicItemListExposesOnlyOpaquePreviewReferences(t *testing.T) {
	secretPath := "/Users/arnold/Private/movie.mp4"
	api := newBusinessTestAPI(t, secretPath)
	stub := api.config.Catalog.(businessCatalogStub)
	stub.list = dto.ListCatalogItemsResponse{
		Items: []dto.CatalogItemDTO{{
			ID: "item-1", CatalogID: "catalog-1", Category: "video", Status: "active", Availability: "available",
			Title: "Movie", SortTitle: "Movie", Revision: 1,
			PrimaryAssetID: "asset-original", PrimaryFileID: "file-original",
			ArtworkAssetID: "asset-artwork", ArtworkFileID: "file-artwork",
		}},
		Total: 1, Limit: 100, Offset: 0,
	}
	api.config.Catalog = stub

	request := httptest.NewRequest(http.MethodGet, "/api/v1/library/items", nil)
	recorder := httptest.NewRecorder()
	api.listItems(recorder, request)
	if recorder.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", recorder.Code, recorder.Body.String())
	}
	body := recorder.Body.String()
	if strings.Contains(body, secretPath) || strings.Contains(body, "localPath") || strings.Contains(body, "documentId") {
		t.Fatalf("public list leaked a physical reference: %s", body)
	}
	for _, reference := range []string{"asset-original", "file-original", "asset-artwork", "file-artwork"} {
		if !strings.Contains(body, reference) {
			t.Fatalf("public list missing opaque reference %q: %s", reference, body)
		}
	}
}

func TestPublicItemDetailExposesOnlySafeRepresentationAndMetadataFields(t *testing.T) {
	secretPath := "/Users/arnold/Private/library.db"
	secretWindowsPath := `C:\Users\Arnold\Private\WINDOWS_PRIVATE_PATH_SENTINEL.mp4`
	secretProvenance := "ffprobe --input " + secretPath + " authorization=Bearer internal-token"
	secretHash := strings.Repeat("a", 64)
	width, height := 1920, 1080
	duration, bitrate, size := int64(7_200_000), int64(8_000_000), int64(4_200_000_000)
	confidence := 0.98

	api := newBusinessTestAPI(t, "/tmp/movie.mp4")
	stub := api.config.Catalog.(businessCatalogStub)
	stub.detail.Representations = []dto.CatalogRepresentationDTO{{
		ID: "representation-1", CatalogID: "catalog-1", ItemID: "item-1", AssetID: "asset-1",
		Kind: "optimized", Purpose: "playback", MediaType: "video/mp4", Container: "mp4", Codec: "h264",
		Width: &width, Height: &height, DurationMs: &duration, BitrateBps: &bitrate, Language: "en",
		ChecksumAlgorithm: "sha256", Checksum: secretHash, SizeBytes: &size,
		Availability: "available", Revision: 9, CreatedAt: "2026-07-13T00:00:00Z", UpdatedAt: "2026-07-13T00:01:00Z",
	}}
	stub.detail.Metadata = []dto.CatalogMetadataEntryDTO{
		{
			ID: "metadata-title", CatalogID: "catalog-1", ItemID: "item-1", RepresentationID: "representation-1",
			Namespace: "dc", Key: "title", ValueType: "string", ValueJSON: `"A Safe Movie"`,
			Language: "en", Source: "embedded", Provenance: secretProvenance, Confidence: &confidence, Locked: true,
		},
		{
			ID: "metadata-duration", CatalogID: "catalog-1", ItemID: "item-1",
			Namespace: "xiadown.media", Key: "duration_ms", ValueType: "duration_ms", ValueJSON: "7200000",
			Source: "derived", Provenance: "internal-media-probe:v4",
		},
		{
			ID: "metadata-legacy", Namespace: "xiadown.legacy.file", Key: "metadata", ValueType: "json",
			ValueJSON: `{"localPath":"/Users/arnold/Private/movie.mp4","token":"private-token"}`,
			Source:    "migration", Provenance: secretProvenance,
		},
		{
			ID: "metadata-path", Namespace: "dc", Key: "description", ValueType: "string",
			ValueJSON: `"indexed at /Users/arnold/Private/movie.mp4"`, Source: "user", Provenance: "desktop-user",
		},
		{
			ID: "metadata-windows-path", Namespace: "dc", Key: "description", ValueType: "string",
			ValueJSON: string(mustMarshalJSON(t, secretWindowsPath)), Source: "user", Provenance: "desktop-user",
		},
		{
			ID: "metadata-token", Namespace: "dc", Key: "creator", ValueType: "string",
			ValueJSON: `"Bearer abcdefghijklmnopqrstuvwxyz"`, Source: "remote", Provenance: "remote-provider-internal",
		},
		{
			ID: "metadata-hash", Namespace: "dc", Key: "identifier", ValueType: "string",
			ValueJSON: `"` + secretHash + `"`, Source: "system", Provenance: "checksum-scanner",
		},
		{
			ID: "metadata-object", Namespace: "dc", Key: "title", ValueType: "object",
			ValueJSON: `{"title":"not a scalar","secret":"private-token"}`, Source: "user", Provenance: "desktop-user",
		},
		{
			ID: "metadata-type-confusion", Namespace: "dc", Key: "title", ValueType: "integer",
			ValueJSON: `{"token":"private-token"}`, Source: "user", Provenance: "desktop-user",
		},
		{
			ID: "metadata-internal-key", Namespace: "dc", Key: "access_token", ValueType: "string",
			ValueJSON: `"private-token"`, Source: "remote", Provenance: "remote-provider-internal",
		},
	}
	api.config.Catalog = stub

	request := httptest.NewRequest(http.MethodGet, "/api/v1/library/items/item-1", nil)
	request.SetPathValue("id", "item-1")
	recorder := httptest.NewRecorder()
	api.getItem(recorder, request)
	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d, body=%s", recorder.Code, recorder.Body.String())
	}
	body := recorder.Body.String()
	for _, forbidden := range []string{
		secretPath, "WINDOWS_PRIVATE_PATH_SENTINEL", secretProvenance, secretHash, "private-token", "internal-media-probe", "remote-provider-internal",
		`"checksum"`, `"checksumAlgorithm"`, `"provenance"`, `"valueJson"`, `"localPath"`, `"access_token"`,
	} {
		if strings.Contains(body, forbidden) {
			t.Fatalf("public item leaked %q: %s", forbidden, body)
		}
	}

	var payload struct {
		Representations []map[string]any `json:"representations"`
		Metadata        []map[string]any `json:"metadata"`
	}
	if err := json.Unmarshal(recorder.Body.Bytes(), &payload); err != nil {
		t.Fatalf("decode public detail: %v", err)
	}
	if len(payload.Representations) != 1 {
		t.Fatalf("representations = %#v", payload.Representations)
	}
	representation := payload.Representations[0]
	assertExactJSONKeys(t, representation,
		"id", "assetId", "kind", "purpose", "mediaType", "container", "codec", "width", "height",
		"durationMs", "bitrateBps", "language", "sizeBytes", "availability",
	)
	if representation["assetId"] != "asset-1" || representation["durationMs"] != float64(duration) {
		t.Fatalf("representation technical fields = %#v", representation)
	}
	if len(payload.Metadata) != 2 {
		t.Fatalf("expected only two safe scalar metadata entries, got %#v", payload.Metadata)
	}
	assertExactJSONKeys(t, payload.Metadata[0],
		"id", "representationId", "namespace", "key", "valueType", "value", "language",
		"position", "source", "confidence", "locked",
	)
	if payload.Metadata[0]["value"] != "A Safe Movie" || payload.Metadata[0]["source"] != "embedded" {
		t.Fatalf("typed public title metadata = %#v", payload.Metadata[0])
	}
	assertExactJSONKeys(t, payload.Metadata[1],
		"id", "namespace", "key", "valueType", "value", "position", "source", "locked",
	)
	if payload.Metadata[1]["value"] != float64(duration) {
		t.Fatalf("typed public duration metadata = %#v", payload.Metadata[1])
	}
}

func TestPublicItemDetailDoesNotAcceptCallerSelectedUserState(t *testing.T) {
	api := newBusinessTestAPI(t, "/tmp/movie.mp4")
	stub := api.config.Catalog.(businessCatalogStub)
	var captured dto.GetCatalogItemRequest
	stub.lastGet = &captured
	secret := strings.Join(sensitivePublicSentinels(), " | ")
	stub.detail.UserState = &dto.CatalogUserStateDTO{
		ID: "state-1", CatalogID: "catalog-1", ItemID: "item-1", UserID: secret,
		Favorite: true, Rating: 4, Progress: 0.5, PositionMs: 42_000,
		Locator: secret, Completed: false, Revision: 7,
		LastOpenedAt: "2026-07-13T12:00:00Z", CreatedAt: "2026-07-12T12:00:00Z", UpdatedAt: "2026-07-13T12:00:00Z",
	}
	api.config.Catalog = stub

	request := httptest.NewRequest(http.MethodGet, "/api/v1/library/items/item-1?userId=another-user", nil)
	request.SetPathValue("id", "item-1")
	recorder := httptest.NewRecorder()
	api.getItem(recorder, request)
	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d, body=%s", recorder.Code, recorder.Body.String())
	}
	assertBodyOmits(t, recorder.Body.String(), append(sensitivePublicSentinels(),
		`"userState"`, `"locator"`, `"userId"`, `"createdAt":"2026-07-12T12:00:00Z"`,
	)...)
	if captured.ID != "item-1" || captured.UserID != "" {
		t.Fatalf("public item request must not trust caller userId: %#v", captured)
	}
}

func TestPublicTrashedItemNeverAdvertisesAssetContentAsAvailable(t *testing.T) {
	api := newBusinessTestAPI(t, "/tmp/movie.mp4")
	stub := api.config.Catalog.(businessCatalogStub)
	stub.detail.Item.Status = string(library.ItemStatusTrashed)
	if len(stub.detail.Assets) == 0 || !stub.detail.Assets[0].FileAvailable {
		t.Fatal("fixture must reproduce the desktop DTO availability mismatch")
	}
	api.config.Catalog = stub

	request := httptest.NewRequest(http.MethodGet, "/api/v1/library/items/item-1", nil)
	request.SetPathValue("id", "item-1")
	recorder := httptest.NewRecorder()
	api.getItem(recorder, request)
	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d, body=%s", recorder.Code, recorder.Body.String())
	}
	var payload struct {
		Assets []struct {
			FileAvailable bool   `json:"fileAvailable"`
			ContentURL    string `json:"contentUrl"`
		} `json:"assets"`
	}
	if err := json.Unmarshal(recorder.Body.Bytes(), &payload); err != nil {
		t.Fatal(err)
	}
	if len(payload.Assets) != 1 || payload.Assets[0].FileAvailable || payload.Assets[0].ContentURL != "" {
		t.Fatalf("trashed public asset remained fetchable: %#v", payload.Assets)
	}
}

func TestPublicCollectionsOmitSmartQueryAndSanitizeOpaqueValues(t *testing.T) {
	api := newBusinessTestAPI(t, "/tmp/movie.mp4")
	stub := api.config.Catalog.(businessCatalogStub)
	secret := strings.Join(sensitivePublicSentinels(), " | ")
	stub.collections = []dto.CatalogCollectionDTO{
		{
			ID: "collection-1", CatalogID: "catalog-1", Name: "Recent videos", Description: "A safe description",
			Kind: "smart", SmartQuery: secret, Revision: 3,
			ItemIDs:   []string{"item-1", sensitivePublicSentinels()[0], sensitivePublicSentinels()[1], sensitivePublicSentinels()[3]},
			CreatedAt: "2026-07-12T12:00:00Z", UpdatedAt: "2026-07-13T12:00:00Z",
		},
		{
			ID: "collection-2", Name: "Manual list", Description: secret, Kind: "manual", Revision: 1,
			ItemIDs: []string{}, UpdatedAt: "2026-07-13T12:00:00Z",
		},
		{
			ID: "collection-3", Name: "Bearer private-collection-token", Kind: "manual", Revision: 1,
		},
	}
	api.config.Catalog = stub

	request := httptest.NewRequest(http.MethodGet, "/api/v1/library/collections", nil)
	recorder := httptest.NewRecorder()
	api.collections(recorder, request)
	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d, body=%s", recorder.Code, recorder.Body.String())
	}
	assertBodyOmits(t, recorder.Body.String(), append(sensitivePublicSentinels(),
		`"smartQuery"`, `"catalogId"`, "private-collection-token",
	)...)

	var payload []map[string]any
	if err := json.Unmarshal(recorder.Body.Bytes(), &payload); err != nil {
		t.Fatalf("decode public collections: %v", err)
	}
	if len(payload) != 2 {
		t.Fatalf("unsafe collection name was not rejected: %#v", payload)
	}
	assertExactJSONKeys(t, payload[0],
		"id", "name", "description", "kind", "revision", "itemIds", "createdAt", "updatedAt",
	)
	if itemIDs, ok := payload[0]["itemIds"].([]any); !ok || len(itemIDs) != 1 || itemIDs[0] != "item-1" {
		t.Fatalf("public collection did not filter unsafe item IDs: %#v", payload[0]["itemIds"])
	}
	assertExactJSONKeys(t, payload[1], "id", "name", "kind", "revision", "itemIds", "updatedAt")
}

func TestPublicTaxonomyPaginationAndCollectionMembershipAreBounded(t *testing.T) {
	api := newBusinessTestAPI(t, "/tmp/movie.mp4")
	stub := api.config.Catalog.(businessCatalogStub)
	memberIDs := make([]string, maxPublicCollectionItemIDs+1)
	for index := range memberIDs {
		memberIDs[index] = fmt.Sprintf("item-%d", index)
	}
	stub.collections = []dto.CatalogCollectionDTO{
		{ID: "collection-1", Name: "First", Kind: "manual", Revision: 1, ItemIDs: []string{}},
		{ID: "collection-2", Name: "Second", Kind: "manual", Revision: 1, ItemIDs: memberIDs},
		{ID: "collection-3", Name: "Third", Kind: "manual", Revision: 1, ItemIDs: []string{}},
	}
	stub.tags = []dto.CatalogTagDTO{{ID: "tag-1", Name: "First"}, {ID: "tag-2", Name: "Second"}, {ID: "tag-3", Name: "Third"}}
	api.config.Catalog = stub

	collectionsRequest := httptest.NewRequest(http.MethodGet, "/api/v1/library/collections?limit=1&offset=1", nil)
	collectionsRecorder := httptest.NewRecorder()
	api.collections(collectionsRecorder, collectionsRequest)
	if collectionsRecorder.Code != http.StatusOK {
		t.Fatalf("collections status=%d body=%s", collectionsRecorder.Code, collectionsRecorder.Body.String())
	}
	var collections []publicCollection
	if err := json.Unmarshal(collectionsRecorder.Body.Bytes(), &collections); err != nil {
		t.Fatal(err)
	}
	if len(collections) != 1 || collections[0].ID != "collection-2" ||
		len(collections[0].ItemIDs) != maxPublicCollectionItemIDs || !collections[0].ItemIDsTruncated {
		t.Fatalf("bounded collection page = %#v", collections)
	}

	firstMembersRequest := httptest.NewRequest(http.MethodGet, "/api/v1/library/collections/collection-2/items?limit=500", nil)
	firstMembersRequest.SetPathValue("id", "collection-2")
	firstMembersRecorder := httptest.NewRecorder()
	api.collectionItems(firstMembersRecorder, firstMembersRequest)
	if firstMembersRecorder.Code != http.StatusOK {
		t.Fatalf("first member page status=%d body=%s", firstMembersRecorder.Code, firstMembersRecorder.Body.String())
	}
	var firstMembers publicCollectionItemsPage
	if err := json.Unmarshal(firstMembersRecorder.Body.Bytes(), &firstMembers); err != nil {
		t.Fatal(err)
	}
	if len(firstMembers.Items) != 500 || !firstMembers.HasMore || firstMembers.NextOffset != 500 ||
		firstMembers.Items[499].Position != 499 || firstMembers.Items[499].ItemID != "item-499" {
		t.Fatalf("first member page = %#v", firstMembers)
	}
	secondMembersRequest := httptest.NewRequest(http.MethodGet, "/api/v1/library/collections/collection-2/items?limit=500&offset=500", nil)
	secondMembersRequest.SetPathValue("id", "collection-2")
	secondMembersRecorder := httptest.NewRecorder()
	api.collectionItems(secondMembersRecorder, secondMembersRequest)
	var secondMembers publicCollectionItemsPage
	if secondMembersRecorder.Code != http.StatusOK || json.Unmarshal(secondMembersRecorder.Body.Bytes(), &secondMembers) != nil ||
		len(secondMembers.Items) != 1 || secondMembers.HasMore || secondMembers.NextOffset != 501 ||
		secondMembers.Items[0].ID != "collection-2-member-000500" || secondMembers.Items[0].Position != 500 {
		t.Fatalf("second member page status=%d page=%#v body=%s", secondMembersRecorder.Code, secondMembers, secondMembersRecorder.Body.String())
	}
	encodedMembers := firstMembersRecorder.Body.String() + secondMembersRecorder.Body.String()
	if strings.Contains(encodedMembers, `"catalogId"`) || strings.Contains(encodedMembers, `"collectionId":"catalog-1"`) {
		t.Fatalf("member page leaked Catalog ownership internals: %s", encodedMembers)
	}

	tagsRequest := httptest.NewRequest(http.MethodGet, "/api/v1/library/tags?limit=1&offset=2", nil)
	tagsRecorder := httptest.NewRecorder()
	api.tags(tagsRecorder, tagsRequest)
	if tagsRecorder.Code != http.StatusOK || !strings.Contains(tagsRecorder.Body.String(), `"id":"tag-3"`) || strings.Contains(tagsRecorder.Body.String(), `"id":"tag-2"`) {
		t.Fatalf("bounded tag page status=%d body=%s", tagsRecorder.Code, tagsRecorder.Body.String())
	}

	for _, target := range []string{
		"/api/v1/library/collections?limit=101",
		"/api/v1/library/collections?limit=501",
	} {
		invalid := httptest.NewRecorder()
		api.collections(invalid, httptest.NewRequest(http.MethodGet, target, nil))
		if invalid.Code != http.StatusBadRequest {
			t.Fatalf("oversized collection page status=%d body=%s", invalid.Code, invalid.Body.String())
		}
	}
}

func TestPublicCollectionMembersEnforceCatalogOwnership(t *testing.T) {
	api := newBusinessTestAPI(t, "/tmp/movie.mp4")
	stub := api.config.Catalog.(businessCatalogStub)
	stub.collections = []dto.CatalogCollectionDTO{{
		ID: "foreign-collection", CatalogID: "catalog-2", Name: "Foreign", Kind: "manual", Revision: 1,
		ItemIDs: []string{"foreign-item"},
	}}
	api.config.Catalog = stub
	request := httptest.NewRequest(http.MethodGet, "/api/v1/library/collections/foreign-collection/items", nil)
	request.SetPathValue("id", "foreign-collection")
	recorder := httptest.NewRecorder()
	api.collectionItems(recorder, request)
	if recorder.Code != http.StatusNotFound || strings.Contains(recorder.Body.String(), "foreign-item") {
		t.Fatalf("foreign collection response status=%d body=%s", recorder.Code, recorder.Body.String())
	}
}

func TestPublicChangesUseAllowlistAndOmitActorAndInternalEntities(t *testing.T) {
	api := newBusinessTestAPI(t, "/tmp/movie.mp4")
	now := time.Date(2026, 7, 13, 12, 0, 0, 0, time.UTC)
	secret := strings.Join(sensitivePublicSentinels(), " | ")
	api.config.Changes = businessChangeRepoStub{changes: []library.CatalogChange{
		{
			Sequence: 1, CatalogID: "catalog-1", EntityType: library.CatalogEntityItem,
			EntityID: "item-1", Kind: library.CatalogChangeUpsert, Revision: 2, ActorID: secret, OccurredAt: now,
		},
		{
			Sequence: 2, CatalogID: "catalog-1", EntityType: library.CatalogEntityDeviceGrant,
			EntityID: "grant-1", Kind: library.CatalogChangeUpsert, Revision: 1, ActorID: secret, OccurredAt: now,
		},
		{
			Sequence: 3, CatalogID: "catalog-1", EntityType: library.CatalogEntityItem,
			EntityID: sensitivePublicSentinels()[0], Kind: library.CatalogChangeUpsert, Revision: 1, OccurredAt: now,
		},
		{
			Sequence: 4, CatalogID: "catalog-1", EntityType: library.CatalogEntityTag,
			EntityID: "tag-1", Kind: library.CatalogChangeDelete, Revision: 3, ActorID: secret, OccurredAt: now,
		},
	}}

	request := httptest.NewRequest(http.MethodGet, "/api/v1/library/changes?epoch="+businessSyncEpoch+"&after=0&limit=10", nil)
	recorder := httptest.NewRecorder()
	api.changes(recorder, request)
	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d, body=%s", recorder.Code, recorder.Body.String())
	}
	assertBodyOmits(t, recorder.Body.String(), append(sensitivePublicSentinels(),
		`"ActorID"`, `"actorId"`, `"CatalogID"`, `"catalogId"`, "device_grant",
	)...)

	var payload struct {
		Changes []map[string]any `json:"changes"`
		Epoch   string           `json:"epoch"`
		Next    int64            `json:"next"`
		HasMore bool             `json:"hasMore"`
	}
	if err := json.Unmarshal(recorder.Body.Bytes(), &payload); err != nil {
		t.Fatalf("decode public changes: %v", err)
	}
	if len(payload.Changes) != 2 || payload.Epoch != businessSyncEpoch || payload.Next != 4 || payload.HasMore {
		t.Fatalf("public change envelope = %#v", payload)
	}
	for _, item := range payload.Changes {
		assertExactJSONKeys(t, item, "sequence", "entityType", "entityId", "kind", "revision", "occurredAt")
	}
	if payload.Changes[0]["entityType"] != "item" || payload.Changes[1]["entityType"] != "tag" {
		t.Fatalf("public changes lost useful content entities: %#v", payload.Changes)
	}
}

func TestPublicNestedChangesExposeOnlyConsumableOwningItemInvalidations(t *testing.T) {
	api := newBusinessTestAPI(t, "/tmp/movie.mp4")
	now := time.Date(2026, 7, 13, 12, 0, 0, 0, time.UTC)
	stub := api.config.Catalog.(businessCatalogStub)
	stub.detail.Assets = append(stub.detail.Assets, dto.CatalogItemAssetDTO{
		ID: "asset-auxiliary", ItemID: "item-1", FileID: "file-auxiliary",
		Role: "auxiliary", Label: "Subtitles", Position: 1, FileAvailable: true,
	})
	stub.detail.Metadata = []dto.CatalogMetadataEntryDTO{{
		ID: "metadata-title", CatalogID: "catalog-1", ItemID: "item-1",
		Namespace: "dc", Key: "title", ValueType: "string", ValueJSON: `"Updated Movie"`,
		Source: "user", Revision: 1,
	}}
	api.config.Catalog = stub
	api.config.Changes = businessChangeRepoStub{changes: []library.CatalogChange{
		{Sequence: 1, CatalogID: "catalog-1", EntityType: library.CatalogEntityItemAsset, EntityID: "asset-auxiliary", Kind: library.CatalogChangeUpsert, Revision: 1, OccurredAt: now},
		{Sequence: 2, CatalogID: "catalog-1", EntityType: library.CatalogEntityItem, EntityID: "item-1", Kind: library.CatalogChangeUpsert, Revision: 1, OccurredAt: now},
		{Sequence: 3, CatalogID: "catalog-1", EntityType: library.CatalogEntityMetadataEntry, EntityID: "metadata-title", Kind: library.CatalogChangeUpsert, Revision: 1, OccurredAt: now},
		{Sequence: 4, CatalogID: "catalog-1", EntityType: library.CatalogEntityItem, EntityID: "item-1", Kind: library.CatalogChangeUpsert, Revision: 1, OccurredAt: now},
		{Sequence: 5, CatalogID: "catalog-1", EntityType: library.CatalogEntityUserState, EntityID: "private-user-state", Kind: library.CatalogChangeUpsert, Revision: 1, OccurredAt: now},
	}}
	api.config.Sync = businessSyncRepoStub{state: library.CatalogSyncState{
		CatalogID: "catalog-1", Epoch: businessSyncEpoch, Cursor: 5, RotatedAt: now,
	}}

	changesRecorder := httptest.NewRecorder()
	api.changes(changesRecorder, httptest.NewRequest(
		http.MethodGet, "/api/v1/library/changes?epoch="+businessSyncEpoch+"&after=0&limit=10", nil,
	))
	if changesRecorder.Code != http.StatusOK {
		t.Fatalf("changes status=%d body=%s", changesRecorder.Code, changesRecorder.Body.String())
	}
	var feed publicChangeList
	if err := json.Unmarshal(changesRecorder.Body.Bytes(), &feed); err != nil {
		t.Fatalf("decode changes: %v", err)
	}
	if feed.Next != 5 || len(feed.Changes) != 2 {
		t.Fatalf("public aggregate feed = %#v", feed)
	}
	for _, change := range feed.Changes {
		if change.EntityType != string(library.CatalogEntityItem) || change.EntityID != "item-1" {
			t.Fatalf("nested persistence entity escaped public feed: %#v", feed.Changes)
		}
		request := httptest.NewRequest(http.MethodGet, "/api/v1/library/items/"+change.EntityID, nil)
		request.SetPathValue("id", change.EntityID)
		itemRecorder := httptest.NewRecorder()
		api.getItem(itemRecorder, request)
		if itemRecorder.Code != http.StatusOK || !strings.Contains(itemRecorder.Body.String(), "asset-auxiliary") ||
			!strings.Contains(itemRecorder.Body.String(), "metadata-title") {
			t.Fatalf("owning item is not consumable: status=%d body=%s", itemRecorder.Code, itemRecorder.Body.String())
		}
	}
	if strings.Contains(changesRecorder.Body.String(), "user_state") ||
		strings.Contains(changesRecorder.Body.String(), "private-user-state") {
		t.Fatalf("private user state escaped public feed: %s", changesRecorder.Body.String())
	}
}

func TestPublicChangesRequireCurrentEpochAndValidCursor(t *testing.T) {
	api := newBusinessTestAPI(t, "/tmp/movie.mp4")
	api.config.Sync = businessSyncRepoStub{state: library.CatalogSyncState{
		CatalogID: "catalog-1", Epoch: businessSyncEpoch, Cursor: 17,
		RotatedAt: time.Date(2026, 7, 13, 12, 0, 0, 0, time.UTC),
	}}
	for name, target := range map[string]string{
		"missing epoch": "/api/v1/library/changes?after=0",
		"old epoch":     "/api/v1/library/changes?epoch=aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa&after=0",
		"future cursor": "/api/v1/library/changes?epoch=" + businessSyncEpoch + "&after=18",
	} {
		t.Run(name, func(t *testing.T) {
			request := httptest.NewRequest(http.MethodGet, target, nil)
			recorder := httptest.NewRecorder()
			api.changes(recorder, request)
			if recorder.Code != http.StatusConflict {
				t.Fatalf("status = %d, body=%s", recorder.Code, recorder.Body.String())
			}
			var payload map[string]any
			if err := json.Unmarshal(recorder.Body.Bytes(), &payload); err != nil {
				t.Fatalf("decode reset response: %v", err)
			}
			assertExactJSONKeys(t, payload, "error", "sync")
			if payload["error"] != "reset_required" {
				t.Fatalf("reset error = %#v", payload)
			}
			syncState, ok := payload["sync"].(map[string]any)
			if !ok || syncState["epoch"] != businessSyncEpoch || syncState["cursor"] != float64(17) {
				t.Fatalf("reset sync state = %#v", payload["sync"])
			}
		})
	}
}

func TestPublicChangesKeepCursorMonotonicWithinEpoch(t *testing.T) {
	api := newBusinessTestAPI(t, "/tmp/movie.mp4")
	api.config.Sync = businessSyncRepoStub{state: library.CatalogSyncState{
		CatalogID: "catalog-1", Epoch: businessSyncEpoch, Cursor: 4,
		RotatedAt: time.Date(2026, 7, 13, 12, 0, 0, 0, time.UTC),
	}}
	request := httptest.NewRequest(http.MethodGet, "/api/v1/library/changes?epoch="+businessSyncEpoch+"&after=4", nil)
	recorder := httptest.NewRecorder()
	api.changes(recorder, request)
	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d, body=%s", recorder.Code, recorder.Body.String())
	}
	var payload publicChangeList
	if err := json.Unmarshal(recorder.Body.Bytes(), &payload); err != nil {
		t.Fatalf("decode public changes: %v", err)
	}
	if payload.Epoch != businessSyncEpoch || payload.Next != 4 || len(payload.Changes) != 0 || payload.HasMore {
		t.Fatalf("public change envelope = %#v", payload)
	}
}

func TestPublicItemDetailRouteRequiresLibraryReadScope(t *testing.T) {
	api := newBusinessTestAPI(t, "/tmp/movie.mp4")
	authenticator := &fakeAuthenticator{principal: access.Principal{
		GrantID: "grant-1", CatalogID: "catalog-1", Scopes: []library.DeviceScope{library.DeviceScopeTasksRead},
	}}
	router, err := NewRouter(Config{
		Version: "test", CatalogID: "catalog-1", Authenticator: authenticator, Pairer: &fakePairer{}, Routes: api.Routes(),
	})
	if err != nil {
		t.Fatalf("NewRouter: %v", err)
	}

	unauthorized := performRequest(router, http.MethodGet, "/api/v1/library/items/item-1", "", "")
	if unauthorized.Code != http.StatusUnauthorized {
		t.Fatalf("anonymous detail status = %d", unauthorized.Code)
	}
	forbidden := performRequest(router, http.MethodGet, "/api/v1/library/items/item-1", "Bearer tasks-only", "")
	if forbidden.Code != http.StatusForbidden || !strings.Contains(forbidden.Body.String(), "insufficient_scope") {
		t.Fatalf("tasks-only detail status=%d body=%s", forbidden.Code, forbidden.Body.String())
	}
	authenticator.principal.Scopes = []library.DeviceScope{library.DeviceScopeLibraryRead}
	authorized := performRequest(router, http.MethodGet, "/api/v1/library/items/item-1", "Bearer library-reader", "")
	if authorized.Code != http.StatusOK {
		t.Fatalf("library reader detail status=%d body=%s", authorized.Code, authorized.Body.String())
	}
	if !strings.Contains(authorized.Body.String(), `"representations":[]`) ||
		!strings.Contains(authorized.Body.String(), `"metadata":[]`) {
		t.Fatalf("v1 additive detail members must remain stable arrays: %s", authorized.Body.String())
	}
}

func TestPublicAssetContentResolvesByIDAndSupportsRange(t *testing.T) {
	directory, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatalf("resolve fixture directory: %v", err)
	}
	path := filepath.Join(directory, "movie.bin")
	if err := os.WriteFile(path, []byte("abcdef"), 0o600); err != nil {
		t.Fatalf("write fixture: %v", err)
	}
	api := newBusinessTestAPI(t, path)
	request := httptest.NewRequest(http.MethodGet, "/api/v1/library/assets/asset-1/content", nil)
	request.SetPathValue("id", "asset-1")
	request.Header.Set("Range", "bytes=1-3")
	recorder := httptest.NewRecorder()
	api.assetContent(recorder, request)
	response := recorder.Result()
	defer response.Body.Close()
	body, _ := io.ReadAll(response.Body)
	if response.StatusCode != http.StatusPartialContent || string(body) != "bcd" {
		t.Fatalf("range response status=%d body=%q", response.StatusCode, body)
	}
}

func TestPublicAssetContentRejectsCataloguedFileMarkedUnavailable(t *testing.T) {
	directory, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatalf("resolve fixture directory: %v", err)
	}
	path := filepath.Join(directory, "still-present.bin")
	if err := os.WriteFile(path, []byte("must stay unavailable"), 0o600); err != nil {
		t.Fatalf("write fixture: %v", err)
	}
	api := newBusinessTestAPI(t, path)
	api.config.Files = businessFileRepoStub{file: library.LibraryFile{
		ID: "file-1", Storage: library.FileStorage{Mode: "local_path", LocalPath: path},
		State: library.FileState{Status: "active", LastError: "missing_local_file"},
	}}
	request := httptest.NewRequest(http.MethodGet, "/api/v1/library/assets/asset-1/content", nil)
	request.SetPathValue("id", "asset-1")
	recorder := httptest.NewRecorder()
	api.assetContent(recorder, request)
	if recorder.Code != http.StatusNotFound || !strings.Contains(recorder.Body.String(), "asset_unavailable") {
		t.Fatalf("unavailable response status=%d body=%q", recorder.Code, recorder.Body.String())
	}
}

func TestPublicAssetContentRejectsLeafSymlinkSubstitution(t *testing.T) {
	directory := t.TempDir()
	target := filepath.Join(directory, "outside-secret.bin")
	if err := os.WriteFile(target, []byte("must-not-be-served"), 0o600); err != nil {
		t.Fatalf("write target: %v", err)
	}
	link := filepath.Join(directory, "catalogued.bin")
	if err := os.Symlink(target, link); err != nil {
		t.Skipf("symlink unavailable: %v", err)
	}
	api := newBusinessTestAPI(t, link)
	request := httptest.NewRequest(http.MethodGet, "/api/v1/library/assets/asset-1/content", nil)
	request.SetPathValue("id", "asset-1")
	recorder := httptest.NewRecorder()
	api.assetContent(recorder, request)
	if recorder.Code != http.StatusNotFound || strings.Contains(recorder.Body.String(), "must-not-be-served") {
		t.Fatalf("symlink response status=%d body=%q", recorder.Code, recorder.Body.String())
	}
}

func TestPublicAssetContentRejectsAncestorSymlinkSubstitution(t *testing.T) {
	directory := t.TempDir()
	outside := t.TempDir()
	if err := os.WriteFile(filepath.Join(outside, "catalogued.bin"), []byte("ancestor-secret"), 0o600); err != nil {
		t.Fatalf("write target: %v", err)
	}
	link := filepath.Join(directory, "mutable-parent")
	if err := os.Symlink(outside, link); err != nil {
		t.Skipf("symlink unavailable: %v", err)
	}
	api := newBusinessTestAPI(t, filepath.Join(link, "catalogued.bin"))
	request := httptest.NewRequest(http.MethodGet, "/api/v1/library/assets/asset-1/content", nil)
	request.SetPathValue("id", "asset-1")
	recorder := httptest.NewRecorder()
	api.assetContent(recorder, request)
	if recorder.Code != http.StatusNotFound || strings.Contains(recorder.Body.String(), "ancestor-secret") {
		t.Fatalf("ancestor symlink response status=%d body=%q", recorder.Code, recorder.Body.String())
	}
}

func newBusinessTestAPI(t *testing.T, path string) *BusinessAPI {
	t.Helper()
	now := time.Date(2026, 7, 13, 12, 0, 0, 0, time.UTC)
	item := library.Item{ID: "item-1", CatalogID: "catalog-1", Category: library.ItemCategoryVideo, Status: library.ItemStatusActive, Title: "Movie", Revision: 1, CreatedAt: now, UpdatedAt: now}
	asset := library.ItemAsset{ID: "asset-1", ItemID: item.ID, FileID: "file-1", Role: library.ItemAssetRoleOriginal, CreatedAt: now, UpdatedAt: now}
	file := library.LibraryFile{ID: "file-1", Storage: library.FileStorage{Mode: "local_path", LocalPath: path}}
	api, err := NewBusinessAPI(BusinessConfig{
		CatalogID: "catalog-1",
		Catalog: businessCatalogStub{detail: dto.CatalogItemDetailDTO{
			Item: dto.CatalogItemDTO{ID: item.ID, CatalogID: item.CatalogID, Category: "video", Status: "active", Availability: "available", Title: "Movie", Revision: 1},
			Assets: []dto.CatalogItemAssetDTO{{
				ID: asset.ID, ItemID: item.ID, FileID: file.ID, Role: "original", FileAvailable: true,
				File: &dto.LibraryFileDTO{ID: file.ID, Storage: dto.LibraryFileStorageDTO{Mode: "local_path", LocalPath: path, DocumentID: "secret-document"}},
			}}, Tags: []dto.CatalogTagDTO{},
		}},
		Items: businessItemRepoStub{item: item}, Assets: businessAssetRepoStub{asset: asset},
		Files: businessFileRepoStub{file: file}, Changes: businessChangeRepoStub{},
		Sync: businessSyncRepoStub{state: library.CatalogSyncState{
			CatalogID: "catalog-1", Epoch: businessSyncEpoch, Cursor: 100, RotatedAt: now,
		}},
	})
	if err != nil {
		t.Fatalf("NewBusinessAPI: %v", err)
	}
	return api
}

func assertExactJSONKeys(t *testing.T, value map[string]any, expected ...string) {
	t.Helper()
	want := make(map[string]struct{}, len(expected))
	for _, key := range expected {
		want[key] = struct{}{}
	}
	for key := range value {
		if _, ok := want[key]; !ok {
			t.Fatalf("unexpected public JSON key %q in %#v", key, value)
		}
		delete(want, key)
	}
	if len(want) != 0 {
		t.Fatalf("missing public JSON keys %#v in %#v", want, value)
	}
}

func sensitivePublicSentinels() []string {
	return []string{
		"/Users/arnold/Private/PUBLIC_API_UNIX_PATH_SENTINEL.db",
		`C:\Users\Arnold\Private\PUBLIC_API_WINDOWS_PATH_SENTINEL.db`,
		"PUBLIC_API_WINDOWS_PATH_SENTINEL",
		"Bearer PUBLIC_API_BEARER_TOKEN_SENTINEL",
		"token=PUBLIC_API_TOKEN_SENTINEL",
		strings.Repeat("ab", 32),
		"abcdefghijklmnopqrst.uvwxyzABCDEFGHIJKLMN.0123456789abcdefghij",
	}
}

func assertBodyOmits(t *testing.T, body string, forbidden ...string) {
	t.Helper()
	for _, value := range forbidden {
		if value != "" && strings.Contains(body, value) {
			t.Fatalf("public API leaked %q: %s", value, body)
		}
	}
}

func mustMarshalJSON(t *testing.T, value any) []byte {
	t.Helper()
	encoded, err := json.Marshal(value)
	if err != nil {
		t.Fatalf("marshal JSON fixture: %v", err)
	}
	return encoded
}
