package libraryapi

import (
	"encoding/json"
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"strconv"
	"strings"
	"testing"

	"gopkg.in/yaml.v3"
)

type openAPIDocument struct {
	OpenAPI    string                                 `yaml:"openapi"`
	Security   []map[string][]string                  `yaml:"security"`
	Casing     map[string]string                      `yaml:"x-xiadown-casing"`
	Scopes     map[string]string                      `yaml:"x-xiadown-bearer-scopes"`
	Paths      map[string]map[string]openAPIOperation `yaml:"paths"`
	Components struct {
		SecuritySchemes map[string]openAPISecurityScheme `yaml:"securitySchemes"`
		Parameters      map[string]openAPIParameter      `yaml:"parameters"`
		Schemas         map[string]openAPISchema         `yaml:"schemas"`
	} `yaml:"components"`
}

type openAPISecurityScheme struct {
	Type   string `yaml:"type"`
	Scheme string `yaml:"scheme"`
}

type openAPIOperation struct {
	OperationID string                    `yaml:"operationId"`
	Scope       string                    `yaml:"x-xiadown-required-scope"`
	Security    []map[string][]string     `yaml:"security"`
	Parameters  []openAPIReference        `yaml:"parameters"`
	Responses   map[string]map[string]any `yaml:"responses"`
	RequestBody struct {
		MaxBodyBytes int `yaml:"x-max-body-bytes"`
	} `yaml:"requestBody"`
}

type openAPIReference struct {
	Ref string `yaml:"$ref"`
}

type openAPIParameter struct {
	Name        string `yaml:"name"`
	Location    string `yaml:"in"`
	Description string `yaml:"description"`
	Schema      struct {
		Minimum   *int   `yaml:"minimum"`
		Maximum   *int   `yaml:"maximum"`
		MaxLength *int   `yaml:"maxLength"`
		Default   any    `yaml:"default"`
		Pattern   string `yaml:"pattern"`
	} `yaml:"schema"`
}

type openAPISchema struct {
	Enum                 []string                 `yaml:"enum"`
	Required             []string                 `yaml:"required"`
	Properties           map[string]openAPISchema `yaml:"properties"`
	AdditionalProperties *bool                    `yaml:"additionalProperties"`
}

type publicRouteContract struct {
	Method        string
	Path          string
	Scope         string
	Authenticated bool
}

func TestLibraryOpenAPIParsesAsYAMLAndJSONAndResolvesLocalReferences(t *testing.T) {
	data, document, raw := loadLibraryOpenAPI(t)
	if document.OpenAPI != "3.1.0" {
		t.Fatalf("openapi version = %q, want 3.1.0", document.OpenAPI)
	}
	if len(data) == 0 {
		t.Fatal("OpenAPI document is empty")
	}
	encoded, err := json.Marshal(raw)
	if err != nil {
		t.Fatalf("marshal YAML representation as JSON: %v", err)
	}
	var jsonDocument map[string]any
	if err := json.Unmarshal(encoded, &jsonDocument); err != nil {
		t.Fatalf("parse JSON representation: %v", err)
	}
	if jsonDocument["openapi"] != "3.1.0" {
		t.Fatalf("JSON representation lost OpenAPI version: %#v", jsonDocument["openapi"])
	}
	assertOpenAPIReferencesResolve(t, raw)
}

func TestLibraryOpenAPIRoutesAndBearerScopesMatchCode(t *testing.T) {
	_, document, _ := loadLibraryOpenAPI(t)
	codeRoutes := libraryAPIRouteContracts(t)
	documentedRoutes := make(map[string]openAPIOperation)
	operationIDs := make(map[string]string)
	for path, pathItem := range document.Paths {
		for method, operation := range pathItem {
			if method != "get" && method != "head" && method != "post" && method != "patch" {
				t.Fatalf("unsupported or incorrectly cased OpenAPI method %q on %s", method, path)
			}
			key := strings.ToUpper(method) + " " + path
			if operation.OperationID == "" {
				t.Fatalf("%s has no operationId", key)
			}
			if previous, exists := operationIDs[operation.OperationID]; exists {
				t.Fatalf("duplicate operationId %q on %s and %s", operation.OperationID, previous, key)
			}
			operationIDs[operation.OperationID] = key
			documentedRoutes[key] = operation
		}
	}
	if len(documentedRoutes) != len(codeRoutes) {
		t.Fatalf("documented route count = %d, code route count = %d\ndocumented=%v\ncode=%v",
			len(documentedRoutes), len(codeRoutes), sortedMapKeys(documentedRoutes), sortedMapKeys(codeRoutes))
	}
	for key, route := range codeRoutes {
		operation, exists := documentedRoutes[key]
		if !exists {
			t.Fatalf("code route %s is absent from OpenAPI", key)
		}
		if operation.Scope != route.Scope {
			t.Fatalf("%s scope = %q, want %q", key, operation.Scope, route.Scope)
		}
		if route.Scope == "" && !route.Authenticated {
			if len(operation.Security) != 0 {
				t.Fatalf("public route %s unexpectedly requires operation security", key)
			}
			continue
		}
		if !usesBearerSecurity(operation.Security) {
			t.Fatalf("protected route %s does not declare bearerAuth", key)
		}
	}
	for key := range documentedRoutes {
		if _, exists := codeRoutes[key]; !exists {
			t.Fatalf("OpenAPI route %s is not implemented", key)
		}
	}
	if len(document.Security) != 0 {
		t.Fatalf("top-level security must remain empty so health, version, and pair stay public: %#v", document.Security)
	}
	bearer, exists := document.Components.SecuritySchemes["bearerAuth"]
	if !exists || bearer.Type != "http" || bearer.Scheme != "bearer" {
		t.Fatalf("bearerAuth = %#v, want HTTP bearer", bearer)
	}
	assertStringSet(t, "documented Bearer scopes", mapKeys(document.Scopes), []string{
		"library.read", "music.read", "music.state", "music.manage",
		"rss.read", "rss.state", "rss.manage", "rss.fetch",
		"tasks.read", "tasks.create", "tasks.control",
	})
}

func TestLibraryOpenAPICriticalSchemasPreservePublicJSONCasing(t *testing.T) {
	_, document, _ := loadLibraryOpenAPI(t)
	schemas := document.Components.Schemas
	assertObjectSchema(t, schemas, "ErrorEnvelope", []string{"error"}, []string{"error"})
	assertObjectSchema(t, schemas, "PairRequest",
		[]string{"nonce", "code", "deviceID", "name", "publicKeyHash"},
		[]string{"nonce", "code", "deviceID", "name", "publicKeyHash"})
	assertObjectSchema(t, schemas, "PairResponse",
		[]string{"grantID", "token", "scopes"}, []string{"grantID", "token", "scopes"})
	assertObjectSchema(t, schemas, "DeviceAccessResponse",
		[]string{"grantId", "catalogId", "deviceId", "scopes", "capabilities", "stations"}, []string{"grantId", "catalogId", "deviceId", "scopes"})
	assertObjectSchema(t, schemas, "StationAccessSummary",
		[]string{"supported", "authorized", "capabilities"}, []string{"supported", "authorized", "capabilities"})
	assertObjectSchema(t, schemas, "SyncStationDirtyEvent",
		[]string{"station", "epoch", "highWater"}, []string{"station", "epoch", "highWater"})
	assertObjectSchema(t, schemas, "SyncTasksDirtyEvent", []string{"dirty"}, []string{"dirty"})
	assertObjectSchema(t, schemas, "LibraryOverview",
		[]string{"catalog", "categories", "statuses", "health", "sync", "capabilities"},
		[]string{"catalog", "categories", "statuses", "health", "sync", "capabilities"})
	assertObjectSchema(t, schemas, "LibraryItem", []string{
		"id", "catalogId", "category", "kind", "format", "durationMs", "sizeBytes",
		"primaryAssetId", "primaryFileId", "artworkAssetId", "artworkFileId", "status", "availability",
		"title", "sortTitle", "description", "revision", "trashedAt", "createdAt", "updatedAt",
	}, []string{"id", "catalogId", "category", "status", "availability", "title", "sortTitle", "revision", "createdAt", "updatedAt"})
	assertObjectSchema(t, schemas, "LibrarySnapshotPage",
		[]string{"items", "epoch", "highWater", "next", "hasMore"},
		[]string{"items", "epoch", "highWater", "hasMore"})
	assertObjectSchema(t, schemas, "LibraryAsset",
		[]string{"id", "fileId", "role", "label", "position", "fileAvailable", "contentUrl"},
		[]string{"id", "fileId", "role", "position", "fileAvailable"})
	assertObjectSchema(t, schemas, "LibraryRepresentation", []string{
		"id", "assetId", "kind", "purpose", "mediaType", "container", "codec", "width", "height",
		"durationMs", "bitrateBps", "language", "sizeBytes", "availability",
	}, []string{"id", "assetId", "kind", "purpose", "availability"})
	assertObjectSchema(t, schemas, "LibraryMetadata", []string{
		"id", "representationId", "namespace", "key", "valueType", "value", "language",
		"position", "source", "confidence", "locked",
	}, []string{"id", "namespace", "key", "valueType", "value", "position", "source", "locked"})
	assertObjectSchema(t, schemas, "LibraryItemDetail",
		[]string{"item", "assets", "representations", "metadata", "tags"},
		[]string{"item", "assets", "representations", "metadata", "tags"})
	assertObjectSchema(t, schemas, "LibraryCollection", []string{
		"id", "name", "description", "kind", "revision", "itemIds", "itemIdsTruncated", "createdAt", "updatedAt",
	}, []string{"id", "name", "kind", "revision", "itemIds"})
	assertObjectSchema(t, schemas, "CollectionItemsPage",
		[]string{"collectionId", "items", "nextOffset", "hasMore"},
		[]string{"collectionId", "items", "nextOffset", "hasMore"})
	assertObjectSchema(t, schemas, "LibraryChange",
		[]string{"sequence", "entityType", "entityId", "kind", "revision", "occurredAt"},
		[]string{"sequence", "entityType", "entityId", "kind", "revision", "occurredAt"})
	assertObjectSchema(t, schemas, "MusicNamespace",
		[]string{"catalogId", "workspaceId", "subjectId"}, []string{"catalogId", "workspaceId", "subjectId"})
	assertObjectSchema(t, schemas, "MusicSyncWindow",
		[]string{"epoch", "highWater", "minimumCursor"}, []string{"epoch", "highWater", "minimumCursor"})
	assertObjectSchema(t, schemas, "MusicOverview",
		[]string{"namespace", "sync", "capabilities"}, []string{"namespace", "sync", "capabilities"})
	assertObjectSchema(t, schemas, "MusicResourceDescriptor", []string{
		"resourceId", "resourceRevision", "kind", "mediaType", "container", "codec", "byteLength", "etag", "checksum", "availability",
	}, []string{"resourceId", "resourceRevision", "kind", "availability"})
	assertObjectSchema(t, schemas, "MusicCompatibleRepresentationStatus",
		[]string{"status", "errorCode"}, []string{"status"})
	assertObjectSchema(t, schemas, "MusicCompatibleRepresentationRequest",
		[]string{"requestId"}, []string{"requestId"})
	assertObjectSchema(t, schemas, "MusicCompatibleRepresentationResponse",
		[]string{"requestId", "status", "errorCode"}, []string{"requestId", "status"})
	assertObjectSchema(t, schemas, "MusicTrack", []string{
		"id", "catalogItemId", "revision", "contentIdentityRevision", "metadataRevision", "resourceRevision",
		"title", "artistName", "albumTitle", "albumArtistName", "genre", "trackNumber", "discNumber", "year",
		"durationMs", "format", "codec", "availability", "playbackResources", "artworkResource", "compatibleRepresentation", "createdAt", "updatedAt", "deletedAt",
	}, []string{
		"id", "revision", "contentIdentityRevision", "metadataRevision", "resourceRevision", "title", "availability",
		"playbackResources", "compatibleRepresentation", "createdAt", "updatedAt", "deletedAt",
	})
	assertObjectSchema(t, schemas, "MusicTrackDisplaySnapshot",
		[]string{"title", "artistName", "albumTitle", "durationMs"}, []string{"title"})
	assertObjectSchema(t, schemas, "MusicPlaylistItem", []string{
		"id", "playlistId", "trackId", "orderKey", "addedAt", "revision", "deletedAt", "trackDisplaySnapshot",
	}, []string{"id", "playlistId", "trackId", "orderKey", "addedAt", "revision", "deletedAt", "trackDisplaySnapshot"})
	assertObjectSchema(t, schemas, "MusicPlaylist",
		[]string{"id", "name", "revision", "items", "createdAt", "updatedAt", "deletedAt"},
		[]string{"id", "name", "revision", "createdAt", "updatedAt", "deletedAt"})
	assertObjectSchema(t, schemas, "MusicTrackState", []string{
		"subjectId", "trackId", "revision", "favorite", "favoriteRevision", "positionMs", "playSessionId",
		"contentIdentityRevision", "progressRevision", "cumulativeListenedDurationMs", "playCount", "skipCount", "updatedAt",
	}, []string{
		"subjectId", "trackId", "revision", "favorite", "favoriteRevision", "positionMs",
		"contentIdentityRevision", "progressRevision", "cumulativeListenedDurationMs", "playCount", "skipCount", "updatedAt",
	})
	assertObjectSchema(t, schemas, "MusicLyricDocument", []string{
		"id", "trackId", "revision", "sourceKind", "providerId", "providerTrackId", "timingKind", "language",
		"contentHash", "availability", "licensePolicy", "createdAt", "updatedAt",
	}, []string{"id", "trackId", "revision", "sourceKind", "timingKind", "availability", "licensePolicy", "createdAt", "updatedAt"})
	assertObjectSchema(t, schemas, "MusicLyricSelection",
		[]string{"subjectId", "trackId", "documentId", "offsetMs", "revision", "updatedAt"},
		[]string{"subjectId", "trackId", "documentId", "offsetMs", "revision", "updatedAt"})
	assertObjectSchema(t, schemas, "MusicMutationRequest", []string{
		"mutationId", "requestHash", "type", "entityId", "expectedRevision", "dependsOnMutationId", "payload",
	}, []string{"mutationId", "requestHash", "type", "entityId", "expectedRevision", "payload"})
	assertObjectSchema(t, schemas, "MusicMutationResult",
		[]string{"mutationId", "type", "entityId", "revision", "replayed", "result"},
		[]string{"mutationId", "type", "entityId", "revision", "replayed", "result"})
	assertObjectSchema(t, schemas, "MusicPlayEventRequest", []string{
		"eventId", "requestHash", "playSessionId", "sequence", "trackId", "contentIdentityRevision",
		"cumulativeListenedDurationMs", "positionMs", "terminal", "completed", "endReason", "deviceOccurredAt",
	}, []string{
		"eventId", "requestHash", "playSessionId", "sequence", "trackId", "contentIdentityRevision",
		"cumulativeListenedDurationMs", "positionMs", "terminal", "completed",
	})
	assertObjectSchema(t, schemas, "MusicPlayEventResult", []string{
		"eventId", "playSessionId", "acknowledgedSequence", "cumulativeListenedDurationMs", "positionMs",
		"terminal", "accepted", "replayed", "trackStateRevision", "trackState",
	}, []string{
		"eventId", "playSessionId", "acknowledgedSequence", "cumulativeListenedDurationMs", "positionMs",
		"terminal", "accepted", "replayed", "trackStateRevision",
	})
	assertObjectSchema(t, schemas, "MusicSnapshotRecord",
		[]string{"entityType", "entityId", "revision", "payload"}, []string{"entityType", "entityId", "revision", "payload"})
	assertObjectSchema(t, schemas, "MusicIndexMembership",
		[]string{"fileId", "state", "reason", "revision", "createdAt", "updatedAt"},
		[]string{"fileId", "state", "revision", "createdAt", "updatedAt"})
	assertObjectSchema(t, schemas, "MusicSnapshotPage",
		[]string{"records", "epoch", "highWater", "minimumCursor", "nextCursor", "hasMore"},
		[]string{"records", "epoch", "highWater", "minimumCursor", "hasMore"})
	assertObjectSchema(t, schemas, "MusicChange",
		[]string{"sequence", "entityType", "entityId", "operation", "revision", "payload", "occurredAt"},
		[]string{"sequence", "entityType", "entityId", "operation", "revision", "occurredAt"})
	assertObjectSchema(t, schemas, "MusicChangePage",
		[]string{"changes", "epoch", "cursor", "highWater", "minimumCursor", "hasMore"},
		[]string{"changes", "epoch", "cursor", "highWater", "minimumCursor", "hasMore"})
	assertObjectSchema(t, schemas, "MusicPlaylistsPage",
		[]string{"items", "nextCursor", "hasMore"}, []string{"items", "hasMore"})
	assertObjectSchema(t, schemas, "MusicResetRequiredEnvelope",
		[]string{"error", "sync"}, []string{"error", "sync"})
	assertObjectSchema(t, schemas, "RSSSubscription", []string{
		"id", "workspaceId", "title", "description", "iconAvailable", "viewType", "categoryId", "sortOrder", "sourceAccess", "publicFeedURL", "enabled",
		"unreadCount", "createdAt", "updatedAt", "revision",
	}, []string{"id", "workspaceId", "title", "iconAvailable", "viewType", "sortOrder", "sourceAccess", "enabled", "unreadCount", "createdAt", "updatedAt", "revision"})
	assertObjectSchema(t, schemas, "RSSSubscriptionMutationRequest", []string{
		"mutationId", "operation", "expectedRevision", "fieldMask", "title", "viewType", "categoryId", "sortOrder", "enabled", "sourceAccess", "publicFeedURL",
	}, []string{"mutationId", "operation"})
	assertObjectSchema(t, schemas, "RSSSubscriptionMutationResult",
		[]string{"mutationId", "operation", "subscription", "deletedId", "revision", "changeCursor"},
		[]string{"mutationId", "operation", "revision", "changeCursor"})
	assertObjectSchema(t, schemas, "RSSFetchLeaseRequest", []string{"ttlSeconds"}, nil)
	assertObjectSchema(t, schemas, "RSSFetchLeaseResult",
		[]string{"granted", "leaseId", "expiresAt", "retryAfterSeconds"}, []string{"granted"})
	assertObjectSchema(t, schemas, "RSSFeedObservationRequest",
		[]string{"mutationId", "upstreamETag", "lastModified", "fetchedAt", "contentHash", "entries"},
		[]string{"mutationId", "fetchedAt", "contentHash", "entries"})
	assertObjectSchema(t, schemas, "RSSFeedObservationResult",
		[]string{"mutationId", "acceptedAt", "created", "updated", "mappings", "changeCursor"},
		[]string{"mutationId", "acceptedAt", "created", "updated", "mappings", "changeCursor"})
	assertObjectSchema(t, schemas, "RSSOverview", []string{
		"catalogId", "workspaceId", "subjectId", "epoch", "highWater", "retainedFrom", "capabilities",
	}, []string{"catalogId", "workspaceId", "subjectId", "epoch", "highWater", "retainedFrom", "capabilities"})
	assertObjectSchema(t, schemas, "RSSEntry", []string{
		"id", "subscriptionId", "title", "author", "summary", "kind", "thumbnailAvailable", "platform", "platformVideoId",
		"publishedAt", "sourceUpdatedAt", "read", "readAt", "starred", "starredAt", "articleProgress",
		"videoProgressSeconds", "videoDurationSeconds", "videoCompleted", "fieldRevisions", "stateRevision", "contentRevision", "createdAt", "modifiedAt",
	}, []string{"id", "subscriptionId", "title", "kind", "thumbnailAvailable", "read", "starred", "videoCompleted", "fieldRevisions", "stateRevision", "contentRevision", "createdAt", "modifiedAt"})
	assertObjectSchema(t, schemas, "RSSEntryPage", []string{"items", "total", "nextOffset"}, []string{"items", "total"})
	assertObjectSchema(t, schemas, "UpdateRSSEntryStateRequest",
		[]string{"field", "value", "videoDurationSeconds", "expectedRevision", "mutationId"}, []string{"field", "value", "expectedRevision", "mutationId"})
	assertObjectSchema(t, schemas, "RSSEntryState", []string{
		"entryId", "subjectId", "read", "readAt", "starred", "starredAt", "articleProgress", "videoProgressSeconds",
		"videoDurationSeconds", "videoCompleted", "fieldRevisions", "revision", "updatedAt", "updatedBy", "mutationId",
	}, []string{"entryId", "subjectId", "read", "starred", "videoCompleted", "fieldRevisions", "revision", "updatedAt"})
	assertObjectSchema(t, schemas, "RSSStateFieldRevisions",
		[]string{"read", "starred", "articleProgress", "videoProgressSeconds"},
		[]string{"read", "starred", "articleProgress", "videoProgressSeconds"})
	assertObjectSchema(t, schemas, "RSSSnapshotPage",
		[]string{"records", "epoch", "highWater", "retainedFrom", "nextCursor", "hasMore"},
		[]string{"records", "epoch", "highWater", "retainedFrom", "hasMore"})
	assertObjectSchema(t, schemas, "RSSChangePage",
		[]string{"changes", "epoch", "cursor", "highWater", "retainedFrom", "hasMore"},
		[]string{"changes", "epoch", "cursor", "highWater", "retainedFrom", "hasMore"})
	assertObjectSchema(t, schemas, "Task", []string{
		"id", "name", "kind", "status", "domain", "platform", "uploader", "publishTime",
		"progress", "metrics", "errorCode", "createdAt", "startedAt", "finishedAt",
	}, []string{"id", "name", "kind", "status", "metrics", "createdAt"})
	assertObjectSchema(t, schemas, "CreateTaskRequest", []string{
		"url", "title", "mode", "quality", "formatId", "audioFormatId", "writeThumbnail",
		"subtitleLangs", "subtitleAuto", "subtitleAll", "subtitleFormat", "transcodePresetId",
		"deleteSourceFileAfterTranscode",
	}, []string{"url"})
}

func TestLibraryOpenAPIEnumsPaginationRangeAndBodyLimitsMatchHandlers(t *testing.T) {
	_, document, _ := loadLibraryOpenAPI(t)
	schemas := document.Components.Schemas
	enums := map[string][]string{
		"DeviceScope":                      {"library.read", "music.read", "music.state", "music.manage", "rss.read", "rss.state", "rss.manage", "rss.fetch", "tasks.read", "tasks.create", "tasks.control"},
		"ItemCategory":                     {"video", "audio", "book", "image", "other"},
		"ItemCategoryFilter":               {"all", "video", "audio", "book", "books", "image", "images", "other", "others"},
		"ItemStatus":                       {"active", "needs_review", "missing", "trashed"},
		"ItemSort":                         {"updated_desc", "created_desc", "created_asc", "title_asc", "title_desc", "category_asc"},
		"RepresentationKind":               {"original", "optimized", "thumbnail", "transcript", "subtitle", "artwork", "preview", "attachment"},
		"RepresentationPurpose":            {"primary", "playback", "preview", "accessibility", "artwork", "attachment", "indexing"},
		"RepresentationAvailability":       {"available", "processing", "offline", "missing", "corrupt"},
		"MetadataValueType":                {"string", "integer", "number", "boolean", "date", "datetime", "duration_ms"},
		"MetadataSource":                   {"user", "embedded", "sidecar", "remote", "derived", "migration", "system"},
		"CollectionKind":                   {"manual", "smart", "playlist", "album", "shelf", "series"},
		"ChangeEntityType":                 {"catalog", "item", "collection", "tag"},
		"ChangeKind":                       {"upsert", "delete"},
		"TaskStatus":                       {"queued", "running", "succeeded", "failed", "canceled"},
		"TaskMode":                         {"quick", "custom"},
		"TaskQuality":                      {"best", "bitrate", "audio"},
		"RSSEntryKind":                     {"article", "social", "image", "video"},
		"RSSViewType":                      {"auto", "article", "social", "image", "video"},
		"RSSSourceAccess":                  {"desktopManaged", "sharedPublic"},
		"RSSSubscriptionMutationOperation": {"add", "promote", "update", "delete"},
		"RSSChangeEntityType":              {"subscription", "entry", "entry_state", "download"},
		"RSSChangeOperation":               {"upsert", "delete"},
		"RSSStateField":                    {"read", "starred", "articleProgress", "videoProgressSeconds"},
		"RSSSnapshotEntityType":            {"subscription", "entry"},
		"MusicEntityType":                  {"track", "playlist", "playlist_item", "track_state", "lyric_document", "lyric_selection", "membership"},
		"MusicChangeOperation":             {"upsert", "delete"},
		"MusicResourceKind":                {"original", "playbackRepresentation", "artwork"},
		"MusicTrackAvailability":           {"available", "missing", "unsupported"},
		"MusicResourceAvailability":        {"available", "processing", "offline", "missing", "corrupt"},
	}
	for name, expected := range enums {
		assertStringSet(t, name+" enum", schemas[name].Enum, expected)
	}
	assertStringSet(t, "ErrorCode enum", schemas["ErrorCode"].Enum, []string{
		"asset_stream_limit_reached", "asset_unavailable", "catalog_access_denied", "idempotency_conflict", "insufficient_scope",
		"internal_error", "invalid_cursor", "invalid_music_mutation", "invalid_pagination", "invalid_request",
		"music_content_changed", "music_dependency_pending", "music_entity_not_found", "music_idempotency_conflict",
		"music_revision_conflict", "music_stream_limit_reached", "not_found",
		"pairing_failed", "pairing_invalid", "reset_required", "rss_state_conflict", "task_state_conflict", "unauthorized",
	})

	parameters := document.Components.Parameters
	assertPaginationParameter(t, parameters, "ItemLimit", 0, 500, 100)
	assertPaginationParameter(t, parameters, "CollectionLimit", 0, 100, 100)
	assertPaginationParameter(t, parameters, "TaxonomyLimit", 0, 500, 100)
	assertPaginationParameter(t, parameters, "TaskLimit", 0, 200, 100)
	assertPaginationParameter(t, parameters, "ChangeLimit", 0, 500, 200)
	assertPaginationParameter(t, parameters, "LibrarySnapshotLimit", 0, 500, 200)
	assertPaginationParameter(t, parameters, "RSSLimit", 0, 500, 100)
	assertPaginationParameter(t, parameters, "RSSChangeLimit", 0, 500, 200)
	assertPaginationParameter(t, parameters, "RSSSnapshotLimit", 0, 500, 200)
	assertPaginationParameter(t, parameters, "MusicSyncLimit", 0, 500, 200)
	assertPaginationParameter(t, parameters, "MusicPlaylistLimit", 0, 200, 100)
	assertPaginationParameter(t, parameters, "Offset", 0, -1, 0)
	if cursor := parameters["LibrarySnapshotAfter"]; cursor.Name != "after" || cursor.Location != "query" ||
		cursor.Schema.MaxLength == nil || *cursor.Schema.MaxLength != maxPublicSnapshotCursorLength {
		t.Fatalf("LibrarySnapshotAfter = %#v", cursor)
	}
	if search := parameters["SearchQuery"]; search.Schema.MaxLength == nil || *search.Schema.MaxLength != maxPublicSearchQueryLength {
		t.Fatalf("SearchQuery maxLength = %#v, want %d", search.Schema.MaxLength, maxPublicSearchQueryLength)
	} else if !strings.Contains(search.Description, "Unicode code points") || !strings.Contains(search.Description, "never truncated") {
		t.Fatalf("SearchQuery description does not define runtime length semantics: %q", search.Description)
	}
	for _, path := range []string{"/api/v1/library/items", "/api/v1/rss/entries", "/api/v1/tasks"} {
		if operation := document.Paths[path]["get"]; !operationReferences(operation, "#/components/parameters/SearchQuery") {
			t.Fatalf("GET %s must reference the shared SearchQuery parameter", path)
		}
	}
	if value := parameters["Range"]; value.Name != "Range" || value.Location != "header" || value.Schema.Pattern != "^bytes=" {
		t.Fatalf("Range parameter = %#v", value)
	}
	assetOperation := document.Paths["/api/v1/library/assets/{id}/content"]["get"]
	assertStringSet(t, "asset response statuses", mapKeys(assetOperation.Responses),
		[]string{"200", "206", "401", "403", "404", "416", "429", "500"})
	if !operationReferences(assetOperation, "#/components/parameters/Range") {
		t.Fatal("asset operation does not reference the Range parameter")
	}
	musicResourcePath := document.Paths["/api/v1/music/tracks/{id}/resources/{resourceId}/content"]
	for _, method := range []string{"get", "head"} {
		operation := musicResourcePath[method]
		assertStringSet(t, "Music "+method+" resource response statuses", mapKeys(operation.Responses),
			[]string{"200", "206", "401", "403", "404", "416", "429", "500"})
		if !operationReferences(operation, "#/components/parameters/MusicSingleRange") ||
			!operationReferences(operation, "#/components/parameters/MusicResourceID") {
			t.Fatalf("Music %s resource operation lacks fixed resource/range parameters", method)
		}
	}
	if document.Paths["/api/v1/pair"]["post"].RequestBody.MaxBodyBytes != maxPairingBodyBytes {
		t.Fatalf("pair body limit does not match %d bytes", maxPairingBodyBytes)
	}
	if document.Paths["/api/v1/tasks"]["post"].RequestBody.MaxBodyBytes != maxTaskBodyBytes {
		t.Fatalf("task body limit does not match %d bytes", maxTaskBodyBytes)
	}
	if document.Paths["/api/v1/rss/entries/{id}/state"]["patch"].RequestBody.MaxBodyBytes != maxRSSStateBodyBytes {
		t.Fatalf("RSS state body limit does not match %d bytes", maxRSSStateBodyBytes)
	}
	if document.Paths["/api/v1/rss/subscriptions/{id}/mutations"]["post"].RequestBody.MaxBodyBytes != maxRSSSubscriptionMutationBytes {
		t.Fatalf("RSS subscription mutation body limit does not match %d bytes", maxRSSSubscriptionMutationBytes)
	}
	if document.Paths["/api/v1/rss/subscriptions/{id}/fetch-lease"]["post"].RequestBody.MaxBodyBytes != maxRSSFetchLeaseBodyBytes {
		t.Fatalf("RSS fetch lease body limit does not match %d bytes", maxRSSFetchLeaseBodyBytes)
	}
	if document.Paths["/api/v1/rss/subscriptions/{id}/observations"]["post"].RequestBody.MaxBodyBytes != maxRSSObservationBodyBytes {
		t.Fatalf("RSS observation body limit does not match %d bytes", maxRSSObservationBodyBytes)
	}
	for _, path := range []string{"/api/v1/music/state/mutations", "/api/v1/music/manage/mutations"} {
		if document.Paths[path]["post"].RequestBody.MaxBodyBytes != maxPublicMusicMutationBodyBytes {
			t.Fatalf("Music mutation body limit for %s does not match %d bytes", path, maxPublicMusicMutationBodyBytes)
		}
	}
	if document.Paths["/api/v1/music/play-events"]["post"].RequestBody.MaxBodyBytes != maxPublicMusicPlayEventBodyBytes {
		t.Fatalf("Music play-event body limit does not match %d bytes", maxPublicMusicPlayEventBodyBytes)
	}
	for _, key := range []string{"paths-and-query-names", "request-property-names", "response-property-names", "scope-names", "bearer-scheme"} {
		if strings.TrimSpace(document.Casing[key]) == "" {
			t.Fatalf("missing casing rule %q", key)
		}
	}
}

func loadLibraryOpenAPI(t *testing.T) ([]byte, openAPIDocument, map[string]any) {
	t.Helper()
	_, testFile, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("resolve OpenAPI test path")
	}
	path := filepath.Clean(filepath.Join(filepath.Dir(testFile), "..", "..", "..", "..", "docs", "api", "library-v1.openapi.yaml"))
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}
	var document openAPIDocument
	if err := yaml.Unmarshal(data, &document); err != nil {
		t.Fatalf("parse OpenAPI YAML: %v", err)
	}
	var raw map[string]any
	if err := yaml.Unmarshal(data, &raw); err != nil {
		t.Fatalf("parse OpenAPI YAML generically: %v", err)
	}
	return data, document, raw
}

func libraryAPIRouteContracts(t *testing.T) map[string]publicRouteContract {
	t.Helper()
	result := fixedRouterRouteContracts(t)
	for _, route := range deviceAccessRoutes(nil, nil) {
		addRouteContract(t, result, publicRouteContract{
			Method: route.Method, Path: route.Path, Authenticated: true,
		})
	}
	for _, route := range (&SyncEventAPI{}).Routes() {
		addRouteContract(t, result, publicRouteContract{
			Method: route.Method, Path: route.Path, Authenticated: true,
		})
	}
	for _, route := range (&BusinessAPI{}).Routes() {
		addRouteContract(t, result, publicRouteContract{
			Method: route.Method, Path: route.Path, Scope: string(route.Scope),
		})
	}
	for _, route := range (&TaskAPI{}).Routes() {
		addRouteContract(t, result, publicRouteContract{
			Method: route.Method, Path: route.Path, Scope: string(route.Scope),
		})
	}
	for _, route := range (&RSSAPI{}).Routes() {
		addRouteContract(t, result, publicRouteContract{
			Method: route.Method, Path: route.Path, Scope: string(route.Scope),
		})
	}
	for _, route := range (&MusicAPI{config: MusicConfig{
		Writer:                   &musicWriteRepositoryStub{},
		CompatibleRepresentation: &musicCompatibleRepresentationStub{},
	}}).Routes() {
		addRouteContract(t, result, publicRouteContract{
			Method: route.Method, Path: route.Path, Scope: string(route.Scope),
		})
	}
	return result
}

func fixedRouterRouteContracts(t *testing.T) map[string]publicRouteContract {
	t.Helper()
	_, testFile, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("resolve router source path")
	}
	filename := filepath.Join(filepath.Dir(testFile), "router.go")
	parsed, err := parser.ParseFile(token.NewFileSet(), filename, nil, 0)
	if err != nil {
		t.Fatalf("parse router source: %v", err)
	}
	result := make(map[string]publicRouteContract)
	ast.Inspect(parsed, func(node ast.Node) bool {
		call, ok := node.(*ast.CallExpr)
		if !ok || len(call.Args) == 0 {
			return true
		}
		selector, ok := call.Fun.(*ast.SelectorExpr)
		if !ok || selector.Sel.Name != "Handle" && selector.Sel.Name != "HandleFunc" {
			return true
		}
		literal, ok := call.Args[0].(*ast.BasicLit)
		if !ok || literal.Kind != token.STRING {
			return true
		}
		pattern, err := strconv.Unquote(literal.Value)
		if err != nil {
			t.Fatalf("unquote router pattern %s: %v", literal.Value, err)
		}
		parts := strings.Fields(pattern)
		if len(parts) != 2 || !strings.HasPrefix(parts[1], "/api/v1/") {
			return true
		}
		addRouteContract(t, result, publicRouteContract{Method: parts[0], Path: parts[1]})
		return true
	})
	if len(result) != 3 {
		t.Fatalf("fixed router route count = %d, want health/version/pair: %v", len(result), sortedMapKeys(result))
	}
	return result
}

func addRouteContract(t *testing.T, routes map[string]publicRouteContract, route publicRouteContract) {
	t.Helper()
	route.Method = strings.ToUpper(strings.TrimSpace(route.Method))
	route.Path = strings.TrimSpace(route.Path)
	key := route.Method + " " + route.Path
	if _, exists := routes[key]; exists {
		t.Fatalf("duplicate code route %s", key)
	}
	routes[key] = route
}

func usesBearerSecurity(security []map[string][]string) bool {
	if len(security) != 1 {
		return false
	}
	scopes, exists := security[0]["bearerAuth"]
	return exists && len(scopes) == 0
}

func operationReferences(operation openAPIOperation, reference string) bool {
	for _, parameter := range operation.Parameters {
		if parameter.Ref == reference {
			return true
		}
	}
	return false
}

func assertObjectSchema(t *testing.T, schemas map[string]openAPISchema, name string, properties, required []string) {
	t.Helper()
	schema, exists := schemas[name]
	if !exists {
		t.Fatalf("missing schema %s", name)
	}
	if schema.AdditionalProperties == nil || *schema.AdditionalProperties {
		t.Fatalf("schema %s must set additionalProperties: false", name)
	}
	assertStringSet(t, name+" properties", mapKeys(schema.Properties), properties)
	assertStringSet(t, name+" required", schema.Required, required)
}

func assertPaginationParameter(t *testing.T, parameters map[string]openAPIParameter, name string, minimum, maximum, defaultValue int) {
	t.Helper()
	parameter, exists := parameters[name]
	actualDefault, defaultOK := parameter.Schema.Default.(int)
	if !exists || parameter.Schema.Minimum == nil || *parameter.Schema.Minimum != minimum ||
		!defaultOK || actualDefault != defaultValue {
		t.Fatalf("pagination parameter %s = %#v", name, parameter)
	}
	if maximum < 0 {
		if parameter.Schema.Maximum != nil {
			t.Fatalf("pagination parameter %s unexpectedly has maximum %d", name, *parameter.Schema.Maximum)
		}
	} else if parameter.Schema.Maximum == nil || *parameter.Schema.Maximum != maximum {
		t.Fatalf("pagination parameter %s maximum = %#v, want %d", name, parameter.Schema.Maximum, maximum)
	}
}

func assertStringSet(t *testing.T, label string, actual, expected []string) {
	t.Helper()
	actual = append([]string(nil), actual...)
	expected = append([]string(nil), expected...)
	sort.Strings(actual)
	sort.Strings(expected)
	if strings.Join(actual, "\x00") != strings.Join(expected, "\x00") {
		t.Fatalf("%s = %v, want %v", label, actual, expected)
	}
}

func mapKeys[V any](values map[string]V) []string {
	result := make([]string, 0, len(values))
	for key := range values {
		result = append(result, key)
	}
	return result
}

func sortedMapKeys[V any](values map[string]V) []string {
	result := mapKeys(values)
	sort.Strings(result)
	return result
}

func assertOpenAPIReferencesResolve(t *testing.T, root map[string]any) {
	t.Helper()
	var walk func(any)
	walk = func(value any) {
		switch typed := value.(type) {
		case map[string]any:
			if rawReference, exists := typed["$ref"]; exists {
				reference, ok := rawReference.(string)
				if !ok || !strings.HasPrefix(reference, "#/") {
					t.Fatalf("OpenAPI contains a non-local reference %#v", rawReference)
				}
				if !jsonPointerExists(root, reference) {
					t.Fatalf("OpenAPI reference does not resolve: %s", reference)
				}
			}
			for _, child := range typed {
				walk(child)
			}
		case []any:
			for _, child := range typed {
				walk(child)
			}
		}
	}
	walk(root)
}

func jsonPointerExists(root map[string]any, pointer string) bool {
	var current any = root
	for _, encoded := range strings.Split(strings.TrimPrefix(pointer, "#/"), "/") {
		part := strings.ReplaceAll(strings.ReplaceAll(encoded, "~1", "/"), "~0", "~")
		mapping, ok := current.(map[string]any)
		if !ok {
			return false
		}
		current, ok = mapping[part]
		if !ok {
			return false
		}
	}
	return true
}
