package libraryapi

import (
	"bytes"
	"context"
	"crypto/sha256"
	"crypto/subtle"
	"database/sql"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"mime"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"
	"unicode"

	"github.com/google/uuid"

	"xiadown/internal/domain/library"
)

const (
	defaultPublicMusicSyncPageSize        = 200
	maxPublicMusicSyncPageSize            = 500
	defaultPublicMusicPlaylistSize        = 100
	maxPublicMusicPlaylistSize            = 200
	maxPublicMusicCursorLength            = 4096
	maxPublicMusicIDLength                = 255
	defaultMaxConcurrentMusicStreams      = 16
	defaultMusicWriteIdleTimeout          = 45 * time.Second
	musicCursorVersion                    = 1
	maxPublicMusicMutationBodyBytes       = 128 << 10
	maxPublicMusicPlayEventBodyBytes      = 32 << 10
	maxPublicMusicRepresentationBodyBytes = 4 << 10

	MusicIOSAudioRepresentationCapability = "music-ios-audio-representation-v1"
)

var (
	publicMusicEpochPattern       = regexp.MustCompile(`^[0-9a-f]{32}$`)
	publicMusicRequestHashPattern = regexp.MustCompile(`^sha256:[0-9a-f]{64}$`)
	musicSingleRangePattern       = regexp.MustCompile(`^bytes=(?:[0-9]+-[0-9]*|-[0-9]+)$`)
	publicMusicIOSMediaTypes      = map[string]struct{}{
		"audio/aac": {}, "audio/flac": {}, "audio/mp4": {}, "audio/mpeg": {},
		"audio/wav": {}, "audio/x-aac": {}, "audio/x-flac": {}, "audio/x-m4a": {}, "audio/x-wav": {},
	}
	publicMusicIOSContainers = map[string]struct{}{
		"aac": {}, "caf": {}, "flac": {}, "m4a": {}, "mp3": {}, "mp4": {}, "wav": {}, "wave": {},
	}
	publicMusicIOSCodecs = map[string]struct{}{
		"aac": {}, "alac": {}, "flac": {}, "lpcm": {}, "mp3": {}, "pcm": {}, "pcm_s16le": {}, "pcm_s24le": {},
	}
	publicMusicCapabilities = []string{
		"music-sync-v1",
		"snapshot-keyset-v1",
		"changes-epoch-v1",
		"versioned-resource-id-v1",
	}
	publicMusicWriteCapabilities = []string{
		"music-mutations-v1",
		"music-play-events-v1",
		"music-track-state-v1",
		"music-playlists-v1",
		"music-provider-lyric-selection-v1",
	}
)

type MusicConfig struct {
	CatalogID                 string
	Reader                    library.ListenLocalMusicReadRepository
	Writer                    library.ListenLocalMusicWriteRepository
	CompatibleRepresentation  library.ListenLocalMusicCompatibleRepresentationCoordinator
	MaxConcurrentAssetStreams int
	AssetWriteIdleTimeout     time.Duration
	ResourceCacheDirectory    string
	MaxResourceCacheEntries   int
	MaxResourceCacheBytes     int64
}

type MusicAPI struct {
	config        MusicConfig
	streamSlots   chan struct{}
	resourceCache *publicMusicResourceMaterializer
}

func NewMusicAPI(config MusicConfig) (*MusicAPI, error) {
	config.CatalogID = strings.TrimSpace(config.CatalogID)
	if config.CatalogID == "" || config.Reader == nil {
		return nil, errors.New("Library public Music API is incomplete")
	}
	if config.MaxConcurrentAssetStreams <= 0 {
		config.MaxConcurrentAssetStreams = defaultMaxConcurrentMusicStreams
	}
	if config.AssetWriteIdleTimeout <= 0 {
		config.AssetWriteIdleTimeout = defaultMusicWriteIdleTimeout
	}
	var resourceCache *publicMusicResourceMaterializer
	if strings.TrimSpace(config.ResourceCacheDirectory) != "" {
		var err error
		resourceCache, err = newPublicMusicResourceMaterializer(
			config.ResourceCacheDirectory, config.MaxResourceCacheEntries, config.MaxResourceCacheBytes,
		)
		if err != nil {
			return nil, err
		}
	}
	return &MusicAPI{
		config: config, streamSlots: make(chan struct{}, config.MaxConcurrentAssetStreams), resourceCache: resourceCache,
	}, nil
}

func (api *MusicAPI) Routes() []ProtectedRoute {
	read := library.DeviceScopeMusicRead
	routes := []ProtectedRoute{
		{Method: http.MethodGet, Path: "/api/v1/music/overview", Scope: read, Handler: http.HandlerFunc(api.overview)},
		{Method: http.MethodGet, Path: "/api/v1/music/snapshot", Scope: read, Handler: http.HandlerFunc(api.snapshot)},
		{Method: http.MethodGet, Path: "/api/v1/music/changes", Scope: read, Handler: http.HandlerFunc(api.changes)},
		{Method: http.MethodGet, Path: "/api/v1/music/tracks/{id}", Scope: read, Handler: http.HandlerFunc(api.track)},
		{Method: http.MethodGet, Path: "/api/v1/music/playlists", Scope: read, Handler: http.HandlerFunc(api.playlists)},
		{
			Method: http.MethodGet, Path: "/api/v1/music/tracks/{id}/resources/{resourceId}/content", Scope: read,
			Handler: api.withStreamGuard(http.HandlerFunc(api.resourceContent)),
		},
		{
			Method: http.MethodHead, Path: "/api/v1/music/tracks/{id}/resources/{resourceId}/content", Scope: read,
			Handler: api.withStreamGuard(http.HandlerFunc(api.resourceContent)),
		},
	}
	if api.config.Writer != nil {
		routes = append(routes,
			ProtectedRoute{Method: http.MethodPost, Path: "/api/v1/music/state/mutations", Scope: library.DeviceScopeMusicState, Handler: http.HandlerFunc(api.stateMutation)},
			ProtectedRoute{Method: http.MethodPost, Path: "/api/v1/music/manage/mutations", Scope: library.DeviceScopeMusicManage, Handler: http.HandlerFunc(api.manageMutation)},
			ProtectedRoute{Method: http.MethodPost, Path: "/api/v1/music/play-events", Scope: library.DeviceScopeMusicState, Handler: http.HandlerFunc(api.playEvent)},
		)
	}
	if api.config.CompatibleRepresentation != nil {
		routes = append(routes, ProtectedRoute{
			Method: http.MethodPost, Path: "/api/v1/music/tracks/{id}/compatible-representation",
			Scope: library.DeviceScopeMusicManage, Handler: http.HandlerFunc(api.createCompatibleRepresentation),
		})
	}
	return routes
}

type publicMusicNamespace struct {
	CatalogID   string `json:"catalogId"`
	WorkspaceID string `json:"workspaceId"`
	SubjectID   string `json:"subjectId"`
}

type publicMusicSyncWindow struct {
	Epoch         string `json:"epoch"`
	HighWater     int64  `json:"highWater"`
	MinimumCursor int64  `json:"minimumCursor"`
}

type publicMusicOverview struct {
	Namespace    publicMusicNamespace  `json:"namespace"`
	Sync         publicMusicSyncWindow `json:"sync"`
	Capabilities []string              `json:"capabilities"`
}

type publicMusicResource struct {
	ResourceID       string `json:"resourceId"`
	ResourceRevision int64  `json:"resourceRevision"`
	Kind             string `json:"kind"`
	MediaType        string `json:"mediaType,omitempty"`
	Container        string `json:"container,omitempty"`
	Codec            string `json:"codec,omitempty"`
	ByteLength       *int64 `json:"byteLength,omitempty"`
	ETag             string `json:"etag,omitempty"`
	Checksum         string `json:"checksum,omitempty"`
	Availability     string `json:"availability"`
}

type publicMusicCompatibleRepresentation struct {
	Status    string `json:"status"`
	ErrorCode string `json:"errorCode,omitempty"`
}

type publicMusicTrack struct {
	ID                       string                               `json:"id"`
	CatalogItemID            *string                              `json:"catalogItemId,omitempty"`
	Revision                 int64                                `json:"revision"`
	ContentIdentityRevision  int64                                `json:"contentIdentityRevision"`
	MetadataRevision         int64                                `json:"metadataRevision"`
	ResourceRevision         int64                                `json:"resourceRevision"`
	Title                    string                               `json:"title"`
	ArtistName               *string                              `json:"artistName,omitempty"`
	AlbumTitle               *string                              `json:"albumTitle,omitempty"`
	AlbumArtistName          *string                              `json:"albumArtistName,omitempty"`
	Genre                    *string                              `json:"genre,omitempty"`
	TrackNumber              *int                                 `json:"trackNumber,omitempty"`
	DiscNumber               *int                                 `json:"discNumber,omitempty"`
	Year                     *int                                 `json:"year,omitempty"`
	DurationMs               *int64                               `json:"durationMs,omitempty"`
	Format                   *string                              `json:"format,omitempty"`
	Codec                    *string                              `json:"codec,omitempty"`
	Availability             string                               `json:"availability"`
	PlaybackResources        []publicMusicResource                `json:"playbackResources"`
	ArtworkResource          *publicMusicResource                 `json:"artworkResource,omitempty"`
	CompatibleRepresentation *publicMusicCompatibleRepresentation `json:"compatibleRepresentation,omitempty"`
	CreatedAt                string                               `json:"createdAt"`
	UpdatedAt                string                               `json:"updatedAt"`
	DeletedAt                *string                              `json:"deletedAt"`
}

type publicMusicTrackDisplaySnapshot struct {
	Title      string  `json:"title"`
	ArtistName *string `json:"artistName,omitempty"`
	AlbumTitle *string `json:"albumTitle,omitempty"`
	DurationMs *int64  `json:"durationMs,omitempty"`
}

type publicMusicPlaylistItem struct {
	ID                   string                          `json:"id"`
	PlaylistID           string                          `json:"playlistId"`
	TrackID              string                          `json:"trackId"`
	OrderKey             string                          `json:"orderKey"`
	AddedAt              string                          `json:"addedAt"`
	Revision             int64                           `json:"revision"`
	DeletedAt            *string                         `json:"deletedAt"`
	TrackDisplaySnapshot publicMusicTrackDisplaySnapshot `json:"trackDisplaySnapshot"`
}

type publicMusicPlaylist struct {
	ID        string                     `json:"id"`
	Name      string                     `json:"name"`
	Revision  int64                      `json:"revision"`
	Items     *[]publicMusicPlaylistItem `json:"items,omitempty"`
	CreatedAt string                     `json:"createdAt"`
	UpdatedAt string                     `json:"updatedAt"`
	DeletedAt *string                    `json:"deletedAt"`
}

// publicMusicMembership deliberately contains only opaque protocol identity
// and index policy. It never exposes a Desktop path or arbitrary Library
// metadata even though the internal membership is keyed by a Library File.
type publicMusicMembership struct {
	FileID    string `json:"fileId"`
	State     string `json:"state"`
	Reason    string `json:"reason,omitempty"`
	Revision  int64  `json:"revision"`
	CreatedAt string `json:"createdAt"`
	UpdatedAt string `json:"updatedAt"`
}

type publicMusicSnapshotRecord struct {
	EntityType string          `json:"entityType"`
	EntityID   string          `json:"entityId"`
	Revision   int64           `json:"revision"`
	Payload    json.RawMessage `json:"payload"`
}

type publicMusicSnapshotPage struct {
	Records       []publicMusicSnapshotRecord `json:"records"`
	Epoch         string                      `json:"epoch"`
	HighWater     int64                       `json:"highWater"`
	MinimumCursor int64                       `json:"minimumCursor"`
	NextCursor    string                      `json:"nextCursor,omitempty"`
	HasMore       bool                        `json:"hasMore"`
}

type publicMusicChange struct {
	Sequence   int64           `json:"sequence"`
	EntityType string          `json:"entityType"`
	EntityID   string          `json:"entityId"`
	Operation  string          `json:"operation"`
	Revision   int64           `json:"revision"`
	Payload    json.RawMessage `json:"payload,omitempty"`
	OccurredAt string          `json:"occurredAt"`
}

type publicMusicChangePage struct {
	Changes       []publicMusicChange `json:"changes"`
	Epoch         string              `json:"epoch"`
	Cursor        int64               `json:"cursor"`
	HighWater     int64               `json:"highWater"`
	MinimumCursor int64               `json:"minimumCursor"`
	HasMore       bool                `json:"hasMore"`
}

type publicMusicPlaylistsPage struct {
	Items      []publicMusicPlaylist `json:"items"`
	NextCursor string                `json:"nextCursor,omitempty"`
	HasMore    bool                  `json:"hasMore"`
}

type publicMusicResetEnvelope struct {
	Error string `json:"error"`
	Sync  struct {
		Epoch         string `json:"epoch"`
		Cursor        int64  `json:"cursor"`
		MinimumCursor int64  `json:"minimumCursor"`
	} `json:"sync"`
}

type musicSnapshotCursor struct {
	Version     int    `json:"v"`
	Epoch       string `json:"e"`
	HighWater   int64  `json:"h"`
	EntityType  string `json:"t"`
	AfterEntity string `json:"a"`
}

type musicPlaylistCursor struct {
	Version int    `json:"v"`
	Epoch   string `json:"e"`
	AfterID string `json:"a"`
}

type publicMusicMutationRequest struct {
	MutationID          string          `json:"mutationId"`
	RequestHash         string          `json:"requestHash"`
	Type                string          `json:"type"`
	EntityID            string          `json:"entityId"`
	ExpectedRevision    *int64          `json:"expectedRevision"`
	DependsOnMutationID *string         `json:"dependsOnMutationId,omitempty"`
	Payload             json.RawMessage `json:"payload"`
}

type publicMusicPlayEventRequest struct {
	EventID                      string  `json:"eventId"`
	RequestHash                  string  `json:"requestHash"`
	PlaySessionID                string  `json:"playSessionId"`
	Sequence                     *int64  `json:"sequence"`
	TrackID                      string  `json:"trackId"`
	ContentIdentityRevision      *int64  `json:"contentIdentityRevision"`
	CumulativeListenedDurationMs *int64  `json:"cumulativeListenedDurationMs"`
	PositionMs                   *int64  `json:"positionMs"`
	Terminal                     *bool   `json:"terminal"`
	Completed                    *bool   `json:"completed"`
	EndReason                    *string `json:"endReason,omitempty"`
	DeviceOccurredAt             *string `json:"deviceOccurredAt,omitempty"`
}

type publicMusicCompatibleRepresentationRequest struct {
	RequestID string `json:"requestId"`
}

type publicMusicCompatibleRepresentationResponse struct {
	RequestID string `json:"requestId"`
	Status    string `json:"status"`
	ErrorCode string `json:"errorCode,omitempty"`
}

var publicMusicMutationTypeAllowlist = map[string]map[string]struct{}{
	library.ListenLocalMusicMutationFamilyState: {
		"setFavorite": {}, "setProgress": {}, "selectProviderLyric": {},
	},
	library.ListenLocalMusicMutationFamilyManage: {
		"createPlaylist": {}, "renamePlaylist": {}, "deletePlaylist": {},
		"addPlaylistItem": {}, "removePlaylistItem": {}, "reorderPlaylist": {}, "setMembership": {},
	},
}

func (api *MusicAPI) stateMutation(w http.ResponseWriter, request *http.Request) {
	api.mutation(w, request, library.ListenLocalMusicMutationFamilyState)
}

func (api *MusicAPI) manageMutation(w http.ResponseWriter, request *http.Request) {
	api.mutation(w, request, library.ListenLocalMusicMutationFamilyManage)
}

func (api *MusicAPI) mutation(w http.ResponseWriter, request *http.Request, family string) {
	principal, ok := PrincipalFromContext(request.Context())
	if !ok || strings.TrimSpace(principal.DeviceID) == "" {
		writeError(w, http.StatusUnauthorized, "unauthorized")
		return
	}
	request.Body = http.MaxBytesReader(w, request.Body, maxPublicMusicMutationBodyBytes)
	decoder := json.NewDecoder(request.Body)
	decoder.DisallowUnknownFields()
	var payload publicMusicMutationRequest
	if err := decoder.Decode(&payload); err != nil || ensureJSONEOF(decoder) != nil ||
		!validPublicMusicMutationRequest(payload, family) {
		writeError(w, http.StatusBadRequest, "invalid_music_mutation")
		return
	}
	payloadObject, err := decodePublicMusicJSONObject(payload.Payload)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid_music_mutation")
		return
	}
	canonical := map[string]any{
		"family": family, "mutationId": payload.MutationID, "type": payload.Type,
		"entityId": payload.EntityID, "expectedRevision": *payload.ExpectedRevision, "payload": payloadObject,
	}
	dependsOn := ""
	if payload.DependsOnMutationID != nil {
		dependsOn = *payload.DependsOnMutationID
		canonical["dependsOnMutationId"] = dependsOn
	}
	if !publicMusicRequestHashMatches(payload.RequestHash, canonical) {
		writeError(w, http.StatusBadRequest, "invalid_music_mutation")
		return
	}
	result, err := api.config.Writer.ApplyMutation(request.Context(), library.ListenLocalMusicMutation{
		SubjectID: library.ListenLocalMusicSubjectID, ActorDeviceID: principal.DeviceID, Family: family,
		MutationID: payload.MutationID, RequestHash: payload.RequestHash, Type: payload.Type,
		EntityID: payload.EntityID, ExpectedRevision: *payload.ExpectedRevision,
		DependsOnMutationID: dependsOn, Payload: payload.Payload, OccurredAt: time.Now().UTC(),
	})
	if err != nil {
		writeMusicMutationError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, result)
}

func (api *MusicAPI) playEvent(w http.ResponseWriter, request *http.Request) {
	principal, ok := PrincipalFromContext(request.Context())
	if !ok || strings.TrimSpace(principal.DeviceID) == "" {
		writeError(w, http.StatusUnauthorized, "unauthorized")
		return
	}
	request.Body = http.MaxBytesReader(w, request.Body, maxPublicMusicPlayEventBodyBytes)
	decoder := json.NewDecoder(request.Body)
	decoder.DisallowUnknownFields()
	var payload publicMusicPlayEventRequest
	if err := decoder.Decode(&payload); err != nil || ensureJSONEOF(decoder) != nil ||
		!validPublicMusicPlayEventRequest(payload) {
		writeError(w, http.StatusBadRequest, "invalid_music_mutation")
		return
	}
	canonical := map[string]any{
		"playSessionId": payload.PlaySessionID, "sequence": *payload.Sequence, "trackId": payload.TrackID,
		"contentIdentityRevision":      *payload.ContentIdentityRevision,
		"cumulativeListenedDurationMs": *payload.CumulativeListenedDurationMs,
		"positionMs":                   *payload.PositionMs, "terminal": *payload.Terminal, "completed": *payload.Completed,
	}
	endReason := ""
	if payload.EndReason != nil {
		endReason = *payload.EndReason
		canonical["endReason"] = endReason
	}
	var occurredAt *time.Time
	if payload.DeviceOccurredAt != nil {
		value, err := time.Parse(time.RFC3339Nano, *payload.DeviceOccurredAt)
		if err != nil {
			writeError(w, http.StatusBadRequest, "invalid_music_mutation")
			return
		}
		occurredAt = &value
		canonical["deviceOccurredAt"] = *payload.DeviceOccurredAt
	}
	if !publicMusicRequestHashMatches(payload.RequestHash, canonical) {
		writeError(w, http.StatusBadRequest, "invalid_music_mutation")
		return
	}
	result, err := api.config.Writer.ApplyPlayEvent(request.Context(), library.ListenLocalMusicPlayEvent{
		SubjectID: library.ListenLocalMusicSubjectID, ActorDeviceID: principal.DeviceID,
		EventID: payload.EventID, RequestHash: payload.RequestHash, PlaySessionID: payload.PlaySessionID,
		Sequence: *payload.Sequence, TrackID: payload.TrackID,
		ContentIdentityRevision:      *payload.ContentIdentityRevision,
		CumulativeListenedDurationMs: *payload.CumulativeListenedDurationMs,
		PositionMs:                   *payload.PositionMs, Terminal: *payload.Terminal, Completed: *payload.Completed,
		EndReason: endReason, DeviceOccurredAt: occurredAt, ReceivedAt: time.Now().UTC(),
	})
	if err != nil {
		writeMusicMutationError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, result)
}

func (api *MusicAPI) createCompatibleRepresentation(w http.ResponseWriter, request *http.Request) {
	principal, ok := PrincipalFromContext(request.Context())
	if !ok || strings.TrimSpace(principal.DeviceID) == "" || api.config.CompatibleRepresentation == nil {
		writeError(w, http.StatusUnauthorized, "unauthorized")
		return
	}
	trackID := strings.TrimSpace(request.PathValue("id"))
	if !validPublicMusicID(trackID) {
		writeError(w, http.StatusNotFound, "not_found")
		return
	}
	request.Body = http.MaxBytesReader(w, request.Body, maxPublicMusicRepresentationBodyBytes)
	decoder := json.NewDecoder(request.Body)
	decoder.DisallowUnknownFields()
	var payload publicMusicCompatibleRepresentationRequest
	if err := decoder.Decode(&payload); err != nil || ensureJSONEOF(decoder) != nil {
		writeError(w, http.StatusBadRequest, "invalid_compatible_representation_request")
		return
	}
	parsedRequestID, err := uuid.Parse(strings.TrimSpace(payload.RequestID))
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid_compatible_representation_request")
		return
	}
	requestID := parsedRequestID.String()
	projection, err := api.config.Reader.GetTrackProjection(request.Context(), trackID)
	if err != nil {
		writeMusicError(w, err)
		return
	}
	projected, err := publicMusicTrackFromProjection(projection)
	if err != nil {
		writeMusicError(w, err)
		return
	}
	if publicMusicTrackHasIOSCompatibleResource(projected) {
		writeJSON(w, http.StatusOK, publicMusicCompatibleRepresentationResponse{
			RequestID: requestID, Status: library.ListenLocalMusicCompatibleRepresentationReady,
		})
		return
	}
	status, err := api.config.CompatibleRepresentation.RequestIOSCompatibleRepresentation(
		request.Context(), trackID, requestID,
	)
	if err != nil {
		switch {
		case errors.Is(err, library.ErrListenLocalMusicCompatibleRepresentationConflict):
			writeError(w, http.StatusConflict, "music_compatible_representation_conflict")
		case errors.Is(err, library.ErrListenLocalMusicCompatibleRepresentationUnavailable):
			writeError(w, http.StatusConflict, "music_compatible_representation_unavailable")
		default:
			writeMusicError(w, err)
		}
		return
	}
	value, ok := publicMusicCompatibleRepresentationFromDomain(status, false)
	if !ok {
		writeError(w, http.StatusInternalServerError, "internal_error")
		return
	}
	responseStatus := http.StatusOK
	if value.Status == library.ListenLocalMusicCompatibleRepresentationGenerating {
		responseStatus = http.StatusAccepted
	}
	writeJSON(w, responseStatus, publicMusicCompatibleRepresentationResponse{
		RequestID: requestID, Status: value.Status, ErrorCode: value.ErrorCode,
	})
}

func (api *MusicAPI) overview(w http.ResponseWriter, request *http.Request) {
	position, err := api.config.Reader.GetSyncPosition(request.Context())
	if err != nil {
		writeMusicError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, publicMusicOverview{
		Namespace: publicMusicNamespace{
			CatalogID: api.config.CatalogID, WorkspaceID: library.ListenLocalMusicWorkspaceID,
			SubjectID: library.ListenLocalMusicSubjectID,
		},
		Sync: publicMusicSyncWindow{
			Epoch: position.Epoch, HighWater: position.HighWater, MinimumCursor: position.MinimumCursor,
		},
		Capabilities: api.capabilities(),
	})
}

func (api *MusicAPI) capabilities() []string {
	result := append([]string(nil), publicMusicCapabilities...)
	if api.config.Writer != nil {
		result = append(result, publicMusicWriteCapabilities...)
	}
	if api.config.CompatibleRepresentation != nil {
		result = append(result, MusicIOSAudioRepresentationCapability)
	}
	return result
}

func (api *MusicAPI) snapshot(w http.ResponseWriter, request *http.Request) {
	query := request.URL.Query()
	epoch := strings.TrimSpace(query.Get("epoch"))
	highWaterRaw := strings.TrimSpace(query.Get("highWater"))
	highWater, highWaterErr := optionalInt64(highWaterRaw)
	limit, limitErr := optionalInteger(query.Get("limit"))
	if !validPublicMusicEpoch(epoch) || highWaterRaw == "" || highWaterErr != nil || highWater < 0 {
		writeError(w, http.StatusBadRequest, "invalid_cursor")
		return
	}
	if limitErr != nil || limit < 0 || limit > maxPublicMusicSyncPageSize {
		writeError(w, http.StatusBadRequest, "invalid_pagination")
		return
	}
	if limit == 0 {
		limit = defaultPublicMusicSyncPageSize
	}
	afterType, afterEntity := "", ""
	if cursorValue := strings.TrimSpace(query.Get("cursor")); cursorValue != "" {
		cursor, err := decodeMusicSnapshotCursor(cursorValue)
		if err != nil || cursor.Epoch != epoch || cursor.HighWater != highWater {
			writeError(w, http.StatusBadRequest, "invalid_cursor")
			return
		}
		afterType, afterEntity = cursor.EntityType, cursor.AfterEntity
	}
	page, err := api.config.Reader.ListSnapshot(request.Context(), library.ListenLocalMusicSnapshotQuery{
		Epoch: epoch, HighWater: highWater, AfterType: afterType, AfterEntity: afterEntity, Limit: limit,
	})
	if err != nil {
		writeMusicError(w, err)
		return
	}
	compatibleStatuses, err := api.compatibleRepresentationStatusesForEntities(request.Context(), page.Entities)
	if err != nil {
		writeMusicError(w, err)
		return
	}
	result := publicMusicSnapshotPage{
		Records: make([]publicMusicSnapshotRecord, 0, len(page.Entities)),
		Epoch:   page.Position.Epoch, HighWater: page.Position.HighWater,
		MinimumCursor: page.Position.MinimumCursor, HasMore: page.HasMore,
	}
	for _, entity := range page.Entities {
		record, err := api.publicMusicSnapshotRecordFromEntity(entity, compatibleStatuses)
		if err != nil {
			writeMusicError(w, err)
			return
		}
		result.Records = append(result.Records, record)
	}
	if page.HasMore {
		result.NextCursor, err = encodeMusicCursor(musicSnapshotCursor{
			Version: musicCursorVersion, Epoch: page.Position.Epoch, HighWater: page.Position.HighWater,
			EntityType: page.NextType, AfterEntity: page.NextEntity,
		})
		if err != nil {
			writeMusicError(w, err)
			return
		}
	}
	writeJSON(w, http.StatusOK, result)
}

func (api *MusicAPI) changes(w http.ResponseWriter, request *http.Request) {
	epoch := strings.TrimSpace(request.URL.Query().Get("epoch"))
	after, afterErr := optionalInt64(request.URL.Query().Get("after"))
	limit, limitErr := optionalInteger(request.URL.Query().Get("limit"))
	if !validPublicMusicEpoch(epoch) || afterErr != nil || after < 0 {
		writeError(w, http.StatusBadRequest, "invalid_cursor")
		return
	}
	if limitErr != nil || limit < 0 || limit > maxPublicMusicSyncPageSize {
		writeError(w, http.StatusBadRequest, "invalid_pagination")
		return
	}
	if limit == 0 {
		limit = defaultPublicMusicSyncPageSize
	}
	page, err := api.config.Reader.ListChanges(request.Context(), library.ListenLocalMusicChangeQuery{
		Epoch: epoch, After: after, Limit: limit,
	})
	if err != nil {
		writeMusicError(w, err)
		return
	}
	result := publicMusicChangePage{
		Changes: make([]publicMusicChange, 0, len(page.Changes)), Epoch: page.Position.Epoch,
		Cursor: page.Cursor, HighWater: page.Position.HighWater,
		MinimumCursor: page.Position.MinimumCursor, HasMore: page.HasMore,
	}
	upsertEntities := make([]library.ListenLocalMusicCanonicalEntity, 0, len(page.Changes))
	for _, change := range page.Changes {
		if change.Operation == "upsert" && change.Entity != nil {
			upsertEntities = append(upsertEntities, *change.Entity)
		}
	}
	compatibleStatuses, err := api.compatibleRepresentationStatusesForEntities(request.Context(), upsertEntities)
	if err != nil {
		writeMusicError(w, err)
		return
	}
	for _, change := range page.Changes {
		item := publicMusicChange{
			Sequence: change.Sequence, EntityType: change.EntityType, EntityID: change.EntityID,
			Operation: change.Operation, Revision: change.Revision,
			OccurredAt: change.OccurredAt.UTC().Format(time.RFC3339Nano),
		}
		if change.Operation == "upsert" {
			if change.Entity == nil {
				writeMusicError(w, errors.New("Music upsert change is missing its canonical entity"))
				return
			}
			payload, err := api.marshalPublicMusicEntity(*change.Entity, compatibleStatuses)
			if err != nil {
				writeMusicError(w, err)
				return
			}
			item.Payload = payload
		}
		result.Changes = append(result.Changes, item)
	}
	writeJSON(w, http.StatusOK, result)
}

func (api *MusicAPI) track(w http.ResponseWriter, request *http.Request) {
	id := strings.TrimSpace(request.PathValue("id"))
	if !validPublicMusicID(id) {
		writeError(w, http.StatusNotFound, "not_found")
		return
	}
	projection, err := api.config.Reader.GetTrackProjection(request.Context(), id)
	if err != nil {
		writeMusicError(w, err)
		return
	}
	compatibleStatuses, err := api.compatibleRepresentationStatuses(request.Context(), []library.ListenLocalMusicTrackProjection{projection})
	if err != nil {
		writeMusicError(w, err)
		return
	}
	result, err := api.publicMusicTrackFromProjection(projection, compatibleStatuses)
	if err != nil {
		writeMusicError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, result)
}

func (api *MusicAPI) playlists(w http.ResponseWriter, request *http.Request) {
	limit, limitErr := optionalInteger(request.URL.Query().Get("limit"))
	if limitErr != nil || limit < 0 || limit > maxPublicMusicPlaylistSize {
		writeError(w, http.StatusBadRequest, "invalid_pagination")
		return
	}
	if limit == 0 {
		limit = defaultPublicMusicPlaylistSize
	}
	position, err := api.config.Reader.GetSyncPosition(request.Context())
	if err != nil {
		writeMusicError(w, err)
		return
	}
	afterID := ""
	if cursorValue := strings.TrimSpace(request.URL.Query().Get("cursor")); cursorValue != "" {
		cursor, err := decodeMusicPlaylistCursor(cursorValue)
		if err != nil || cursor.Epoch != position.Epoch {
			writeError(w, http.StatusBadRequest, "invalid_cursor")
			return
		}
		afterID = cursor.AfterID
	}
	page, err := api.config.Reader.ListPlaylistProjections(request.Context(), library.ListenLocalMusicPlaylistQuery{
		Epoch: position.Epoch, AfterID: afterID, Limit: limit,
	})
	if err != nil {
		writeMusicError(w, err)
		return
	}
	result := publicMusicPlaylistsPage{Items: make([]publicMusicPlaylist, 0, len(page.Items)), HasMore: page.HasMore}
	for _, item := range page.Items {
		playlist, err := publicMusicPlaylistFromProjection(item, true)
		if err != nil {
			writeMusicError(w, err)
			return
		}
		result.Items = append(result.Items, playlist)
	}
	if page.HasMore {
		result.NextCursor, err = encodeMusicCursor(musicPlaylistCursor{
			Version: musicCursorVersion, Epoch: position.Epoch, AfterID: page.NextID,
		})
		if err != nil {
			writeMusicError(w, err)
			return
		}
	}
	writeJSON(w, http.StatusOK, result)
}

func (api *MusicAPI) withStreamGuard(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, request *http.Request) {
		select {
		case api.streamSlots <- struct{}{}:
			defer func() { <-api.streamSlots }()
		case <-request.Context().Done():
			return
		default:
			w.Header().Set("Retry-After", "1")
			writeError(w, http.StatusTooManyRequests, "music_stream_limit_reached")
			return
		}
		writer := &idleWriteResponseWriter{
			ResponseWriter: w, controller: http.NewResponseController(w), timeout: api.config.AssetWriteIdleTimeout,
		}
		writer.refreshDeadline()
		defer writer.clearDeadline()
		next.ServeHTTP(writer, request)
	})
}

func (api *MusicAPI) resourceContent(w http.ResponseWriter, request *http.Request) {
	trackID := strings.TrimSpace(request.PathValue("id"))
	resourceID := strings.TrimSpace(request.PathValue("resourceId"))
	if !validPublicMusicID(trackID) || !validPublicMusicID(resourceID) {
		writeError(w, http.StatusNotFound, "not_found")
		return
	}
	if rawRange := strings.TrimSpace(request.Header.Get("Range")); rawRange != "" &&
		(len(rawRange) > 128 || !musicSingleRangePattern.MatchString(rawRange)) {
		w.WriteHeader(http.StatusRequestedRangeNotSatisfiable)
		return
	}
	resource, err := api.config.Reader.ResolveTrackResource(request.Context(), trackID, resourceID)
	if err != nil {
		writeMusicError(w, err)
		return
	}
	opened, info, err := api.resourceCache.open(request.Context(), resource)
	if err != nil {
		// Paths and cache state are internal implementation details. Treat every
		// preparation failure as opaque absence so a paired device cannot infer
		// filesystem layout or distinguish checksum failures from missing content.
		writeError(w, http.StatusNotFound, "not_found")
		return
	}
	defer opened.Close()
	contentType := safePublicMusicMediaType(resource.MediaType)
	if contentType == "" {
		contentType = mime.TypeByExtension(strings.ToLower(filepath.Ext(resource.LocalPath)))
	}
	if contentType != "" {
		w.Header().Set("Content-Type", contentType)
	}
	w.Header().Set("Accept-Ranges", "bytes")
	w.Header().Set("Content-Disposition", "inline")
	w.Header().Set("ETag", resource.ETag)
	http.ServeContent(w, request, "music-resource", info.ModTime(), opened)
}

func (api *MusicAPI) publicMusicSnapshotRecordFromEntity(
	entity library.ListenLocalMusicCanonicalEntity,
	compatibleStatuses map[string]library.ListenLocalMusicCompatibleRepresentationStatus,
) (publicMusicSnapshotRecord, error) {
	payload, err := api.marshalPublicMusicEntity(entity, compatibleStatuses)
	if err != nil {
		return publicMusicSnapshotRecord{}, err
	}
	return publicMusicSnapshotRecord{
		EntityType: entity.EntityType, EntityID: entity.EntityID, Revision: entity.Revision, Payload: payload,
	}, nil
}

func (api *MusicAPI) marshalPublicMusicEntity(
	entity library.ListenLocalMusicCanonicalEntity,
	compatibleStatuses map[string]library.ListenLocalMusicCompatibleRepresentationStatus,
) (json.RawMessage, error) {
	if entity.EntityType != library.ListenLocalMusicEntityTrack {
		return marshalPublicMusicEntity(entity)
	}
	if entity.Track == nil {
		return nil, errors.New("Music Track payload is missing")
	}
	track, err := api.publicMusicTrackFromProjection(*entity.Track, compatibleStatuses)
	if err != nil {
		return nil, err
	}
	payload, err := json.Marshal(track)
	return json.RawMessage(payload), err
}

func (api *MusicAPI) publicMusicTrackFromProjection(
	projection library.ListenLocalMusicTrackProjection,
	compatibleStatuses map[string]library.ListenLocalMusicCompatibleRepresentationStatus,
) (publicMusicTrack, error) {
	result, err := publicMusicTrackFromProjection(projection)
	if err != nil || api.config.CompatibleRepresentation == nil || publicMusicTrackHasIOSCompatibleResource(result) {
		return result, err
	}
	status, ok := compatibleStatuses[projection.Track.FileID]
	if !ok {
		return result, nil
	}
	// A completed operation becomes playable only when the Catalog projection
	// is visible in this exact Track read. Keep the state generating across the
	// tiny completion/projection window instead of claiming ready with no bytes.
	if status.Status == library.ListenLocalMusicCompatibleRepresentationReady {
		status.Status = library.ListenLocalMusicCompatibleRepresentationGenerating
		status.ErrorCode = ""
	}
	if value, ok := publicMusicCompatibleRepresentationFromDomain(status, false); ok {
		result.CompatibleRepresentation = &value
	}
	return result, nil
}

func (api *MusicAPI) compatibleRepresentationStatusesForEntities(
	ctx context.Context,
	entities []library.ListenLocalMusicCanonicalEntity,
) (map[string]library.ListenLocalMusicCompatibleRepresentationStatus, error) {
	projections := make([]library.ListenLocalMusicTrackProjection, 0, len(entities))
	for _, entity := range entities {
		if entity.EntityType == library.ListenLocalMusicEntityTrack && entity.Track != nil {
			projections = append(projections, *entity.Track)
		}
	}
	return api.compatibleRepresentationStatuses(ctx, projections)
}

func (api *MusicAPI) compatibleRepresentationStatuses(
	ctx context.Context,
	projections []library.ListenLocalMusicTrackProjection,
) (map[string]library.ListenLocalMusicCompatibleRepresentationStatus, error) {
	if api == nil || api.config.CompatibleRepresentation == nil {
		return nil, nil
	}
	trackIDs := make([]string, 0, len(projections))
	seen := make(map[string]struct{}, len(projections))
	for _, projection := range projections {
		track, err := publicMusicTrackFromProjection(projection)
		if err != nil {
			return nil, err
		}
		trackID := strings.TrimSpace(projection.Track.FileID)
		if publicMusicTrackHasIOSCompatibleResource(track) {
			continue
		}
		if _, ok := seen[trackID]; ok {
			continue
		}
		seen[trackID] = struct{}{}
		trackIDs = append(trackIDs, trackID)
	}
	if len(trackIDs) == 0 {
		return map[string]library.ListenLocalMusicCompatibleRepresentationStatus{}, nil
	}
	return api.config.CompatibleRepresentation.GetIOSCompatibleRepresentationStatuses(ctx, trackIDs)
}

func publicMusicSnapshotRecordFromEntity(entity library.ListenLocalMusicCanonicalEntity) (publicMusicSnapshotRecord, error) {
	payload, err := marshalPublicMusicEntity(entity)
	if err != nil {
		return publicMusicSnapshotRecord{}, err
	}
	return publicMusicSnapshotRecord{
		EntityType: entity.EntityType, EntityID: entity.EntityID, Revision: entity.Revision, Payload: payload,
	}, nil
}

func marshalPublicMusicEntity(entity library.ListenLocalMusicCanonicalEntity) (json.RawMessage, error) {
	var value any
	switch entity.EntityType {
	case library.ListenLocalMusicEntityTrack:
		if entity.Track == nil {
			return nil, errors.New("Music Track payload is missing")
		}
		track, err := publicMusicTrackFromProjection(*entity.Track)
		if err != nil {
			return nil, err
		}
		value = track
	case library.ListenLocalMusicEntityPlaylist:
		if entity.Playlist == nil {
			return nil, errors.New("Music Playlist payload is missing")
		}
		playlist, err := publicMusicPlaylistFromDomain(*entity.Playlist, nil)
		if err != nil {
			return nil, err
		}
		value = playlist
	case library.ListenLocalMusicEntityPlaylistItem:
		if entity.PlaylistItem == nil {
			return nil, errors.New("Music Playlist Item payload is missing")
		}
		item, err := publicMusicPlaylistItemFromDomain(*entity.PlaylistItem)
		if err != nil {
			return nil, err
		}
		value = item
	case library.ListenLocalMusicEntityMembership:
		if entity.Membership == nil || entity.Membership.FileID != entity.EntityID ||
			!validPublicMusicID(entity.Membership.FileID) || entity.Membership.Revision < 1 ||
			(entity.Membership.State != library.ListenLocalMusicMembershipIncluded &&
				entity.Membership.State != library.ListenLocalMusicMembershipExcluded) ||
			!validPublicMusicMembershipReason(entity.Membership.Reason) ||
			entity.Membership.CreatedAt.IsZero() || entity.Membership.UpdatedAt.IsZero() {
			return nil, errors.New("invalid public Music membership")
		}
		value = publicMusicMembership{
			FileID: entity.Membership.FileID, State: string(entity.Membership.State),
			Reason: entity.Membership.Reason, Revision: entity.Membership.Revision,
			CreatedAt: entity.Membership.CreatedAt.UTC().Format(time.RFC3339Nano),
			UpdatedAt: entity.Membership.UpdatedAt.UTC().Format(time.RFC3339Nano),
		}
	case library.ListenLocalMusicEntityTrackState:
		if entity.TrackState == nil || entity.TrackState.SubjectID != library.ListenLocalMusicSubjectID ||
			!validPublicMusicID(entity.TrackState.TrackID) || entity.TrackState.Revision < 1 ||
			entity.TrackState.FavoriteRevision < 0 || entity.TrackState.ProgressRevision < 0 ||
			entity.TrackState.PositionMs < 0 || entity.TrackState.CumulativeListenedMs < 0 ||
			entity.TrackState.PlayCount < 0 || entity.TrackState.SkipCount < 0 || entity.TrackState.UpdatedAt.IsZero() {
			return nil, errors.New("invalid public Music Track State")
		}
		value = entity.TrackState
	case library.ListenLocalMusicEntityLyricDocument:
		if entity.LyricDocument == nil || !validPublicMusicID(entity.LyricDocument.ID) ||
			!validPublicMusicID(entity.LyricDocument.TrackID) || entity.LyricDocument.Revision < 1 ||
			entity.LyricDocument.SourceKind != "provider" || entity.LyricDocument.CreatedAt.IsZero() ||
			entity.LyricDocument.UpdatedAt.IsZero() {
			return nil, errors.New("invalid public Music Lyric Document")
		}
		value = entity.LyricDocument
	case library.ListenLocalMusicEntityLyricSelection:
		if entity.LyricSelection == nil || entity.LyricSelection.SubjectID != library.ListenLocalMusicSubjectID ||
			!validPublicMusicID(entity.LyricSelection.TrackID) ||
			!validPublicMusicID(entity.LyricSelection.DocumentID) || entity.LyricSelection.Revision < 1 ||
			entity.LyricSelection.UpdatedAt.IsZero() {
			return nil, errors.New("invalid public Music Lyric Selection")
		}
		value = entity.LyricSelection
	default:
		return nil, errors.New("unsupported public Music entity")
	}
	payload, err := json.Marshal(value)
	if err != nil {
		return nil, err
	}
	return json.RawMessage(payload), nil
}

func publicMusicTrackFromProjection(projection library.ListenLocalMusicTrackProjection) (publicMusicTrack, error) {
	track := projection.Track
	if !validPublicMusicID(track.FileID) || track.Revision < 1 || track.ContentIdentityRevision < 1 ||
		track.MetadataRevision < 1 || track.ResourceRevision < 1 || track.CreatedAt.IsZero() || track.UpdatedAt.IsZero() {
		return publicMusicTrack{}, errors.New("invalid public Music Track")
	}
	title := safePublicDisplayText(track.Title)
	if title == "" {
		title = "Untitled Track"
	}
	result := publicMusicTrack{
		ID: track.FileID, Revision: track.Revision, ContentIdentityRevision: track.ContentIdentityRevision,
		MetadataRevision: track.MetadataRevision, ResourceRevision: track.ResourceRevision,
		Title: title, ArtistName: optionalPublicMusicText(track.Author), AlbumTitle: optionalPublicMusicText(track.Album),
		AlbumArtistName: optionalPublicMusicText(track.AlbumArtist), Genre: optionalPublicMusicText(track.Genre),
		TrackNumber: positivePublicMusicInt(track.TrackNumber), DiscNumber: positivePublicMusicInt(track.DiscNumber),
		Year: positivePublicMusicInt(track.Year), DurationMs: nonNegativePublicMusicInt64(track.DurationMs),
		Format: optionalPublicMusicToken(track.Format), Codec: optionalPublicMusicToken(track.AudioCodec),
		Availability:      safePublicTechnicalToken(track.Availability),
		PlaybackResources: make([]publicMusicResource, 0, len(projection.PlaybackResources)),
		CreatedAt:         track.CreatedAt.UTC().Format(time.RFC3339Nano), UpdatedAt: track.UpdatedAt.UTC().Format(time.RFC3339Nano),
	}
	if result.Availability == "" {
		result.Availability = "missing"
	}
	if safePublicOpaqueID(projection.CatalogItemID) {
		value := strings.TrimSpace(projection.CatalogItemID)
		result.CatalogItemID = &value
	}
	for _, resource := range projection.PlaybackResources {
		if value, ok := publicMusicResourceFromDomain(resource); ok {
			result.PlaybackResources = append(result.PlaybackResources, value)
		}
	}
	if projection.ArtworkResource != nil {
		if value, ok := publicMusicResourceFromDomain(*projection.ArtworkResource); ok {
			result.ArtworkResource = &value
		}
	}
	if len(result.PlaybackResources) == 0 && result.Availability == "available" {
		result.Availability = "unsupported"
	}
	compatibleStatus := library.ListenLocalMusicCompatibleRepresentationUnsupported
	if publicMusicTrackHasIOSCompatibleResource(result) {
		compatibleStatus = library.ListenLocalMusicCompatibleRepresentationReady
	}
	result.CompatibleRepresentation = &publicMusicCompatibleRepresentation{Status: compatibleStatus}
	return result, nil
}

func publicMusicResourceFromDomain(resource library.ListenLocalMusicResource) (publicMusicResource, bool) {
	availability := safePublicTechnicalToken(resource.Availability)
	if !validPublicMusicID(resource.ID) || resource.Revision < 1 || availability == "" {
		return publicMusicResource{}, false
	}
	kind := strings.TrimSpace(resource.Kind)
	switch kind {
	case library.ListenLocalMusicResourceOriginal,
		library.ListenLocalMusicResourcePlaybackRepresentation,
		library.ListenLocalMusicResourceArtwork:
	default:
		return publicMusicResource{}, false
	}
	return publicMusicResource{
		ResourceID: resource.ID, ResourceRevision: resource.Revision, Kind: kind,
		MediaType: safePublicMusicMediaType(resource.MediaType), Container: safePublicTechnicalToken(resource.Container),
		Codec: safePublicTechnicalToken(resource.Codec), ByteLength: nonNegativePublicMusicInt64(resource.ByteLength),
		ETag: safePublicMusicETag(resource.ETag), Checksum: safePublicMusicChecksum(resource.Checksum),
		Availability: availability,
	}, true
}

func publicMusicCompatibleRepresentationFromDomain(
	status library.ListenLocalMusicCompatibleRepresentationStatus,
	hasCompatibleResource bool,
) (publicMusicCompatibleRepresentation, bool) {
	value := publicMusicCompatibleRepresentation{Status: strings.TrimSpace(status.Status)}
	if hasCompatibleResource {
		value.Status = library.ListenLocalMusicCompatibleRepresentationReady
	}
	switch value.Status {
	case library.ListenLocalMusicCompatibleRepresentationUnsupported,
		library.ListenLocalMusicCompatibleRepresentationGenerating,
		library.ListenLocalMusicCompatibleRepresentationReady:
		value.ErrorCode = ""
	case library.ListenLocalMusicCompatibleRepresentationFailed:
		value.ErrorCode = safePublicTechnicalToken(status.ErrorCode)
		if value.ErrorCode == "" {
			value.ErrorCode = "generation_failed"
		}
	default:
		return publicMusicCompatibleRepresentation{}, false
	}
	return value, true
}

func publicMusicTrackHasIOSCompatibleResource(track publicMusicTrack) bool {
	for _, resource := range track.PlaybackResources {
		if !strings.EqualFold(resource.Availability, "available") {
			continue
		}
		if publicMusicIOSCompatibleToken(resource.MediaType, publicMusicIOSMediaTypes) ||
			publicMusicIOSCompatibleToken(resource.Container, publicMusicIOSContainers) ||
			publicMusicIOSCompatibleToken(resource.Codec, publicMusicIOSCodecs) {
			return true
		}
		if resource.MediaType == "" && resource.Container == "" && resource.Codec == "" && track.Format != nil &&
			publicMusicIOSCompatibleToken(*track.Format, publicMusicIOSContainers) {
			return true
		}
	}
	return false
}

func publicMusicIOSCompatibleToken(value string, allowed map[string]struct{}) bool {
	value = strings.ToLower(strings.TrimSpace(value))
	if separator := strings.IndexByte(value, ';'); separator >= 0 {
		value = strings.TrimSpace(value[:separator])
	}
	_, ok := allowed[value]
	return ok
}

func publicMusicPlaylistFromProjection(
	projection library.ListenLocalMusicPlaylistProjection,
	includeItems bool,
) (publicMusicPlaylist, error) {
	var items *[]publicMusicPlaylistItem
	if includeItems {
		values := make([]publicMusicPlaylistItem, 0, len(projection.Items))
		for _, item := range projection.Items {
			value, err := publicMusicPlaylistItemFromDomain(item)
			if err != nil {
				return publicMusicPlaylist{}, err
			}
			values = append(values, value)
		}
		items = &values
	}
	return publicMusicPlaylistFromDomain(projection.Playlist, items)
}

func publicMusicPlaylistFromDomain(
	playlist library.ListenLocalPlaylist,
	items *[]publicMusicPlaylistItem,
) (publicMusicPlaylist, error) {
	name := safePublicDisplayText(playlist.Name)
	if !validPublicMusicID(playlist.ID) || name == "" || playlist.Revision < 1 ||
		playlist.CreatedAt.IsZero() || playlist.UpdatedAt.IsZero() {
		return publicMusicPlaylist{}, errors.New("invalid public Music Playlist")
	}
	return publicMusicPlaylist{
		ID: playlist.ID, Name: name, Revision: playlist.Revision, Items: items,
		CreatedAt: playlist.CreatedAt.UTC().Format(time.RFC3339Nano),
		UpdatedAt: playlist.UpdatedAt.UTC().Format(time.RFC3339Nano),
	}, nil
}

func publicMusicPlaylistItemFromDomain(item library.ListenLocalPlaylistItem) (publicMusicPlaylistItem, error) {
	if !validPublicMusicID(item.ID) || !validPublicMusicID(item.PlaylistID) || !validPublicMusicID(item.FileID) ||
		item.Position < 0 || item.Revision < 1 || item.AddedAt.IsZero() {
		return publicMusicPlaylistItem{}, errors.New("invalid public Music Playlist Item")
	}
	title := safePublicDisplayText(item.TrackDisplaySnapshot.Title)
	if title == "" {
		title = "Unavailable Track"
	}
	return publicMusicPlaylistItem{
		ID: item.ID, PlaylistID: item.PlaylistID, TrackID: item.FileID,
		OrderKey: fmt.Sprintf("%020d", item.Position), AddedAt: item.AddedAt.UTC().Format(time.RFC3339Nano),
		Revision: item.Revision,
		TrackDisplaySnapshot: publicMusicTrackDisplaySnapshot{
			Title: title, ArtistName: optionalPublicMusicText(item.TrackDisplaySnapshot.Author),
			AlbumTitle: optionalPublicMusicText(item.TrackDisplaySnapshot.Album),
			DurationMs: nonNegativePublicMusicInt64(item.TrackDisplaySnapshot.DurationMs),
		},
	}, nil
}

func encodeMusicCursor(value any) (string, error) {
	payload, err := json.Marshal(value)
	if err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(payload), nil
}

func decodeMusicSnapshotCursor(value string) (musicSnapshotCursor, error) {
	var cursor musicSnapshotCursor
	if err := decodeMusicCursor(value, &cursor); err != nil || cursor.Version != musicCursorVersion ||
		!validPublicMusicEpoch(cursor.Epoch) || cursor.HighWater < 0 ||
		!publicMusicSnapshotEntityType(cursor.EntityType) || !validPublicMusicID(cursor.AfterEntity) {
		return musicSnapshotCursor{}, errors.New("invalid Music snapshot cursor")
	}
	return cursor, nil
}

func decodeMusicPlaylistCursor(value string) (musicPlaylistCursor, error) {
	var cursor musicPlaylistCursor
	if err := decodeMusicCursor(value, &cursor); err != nil || cursor.Version != musicCursorVersion ||
		!validPublicMusicEpoch(cursor.Epoch) || !validPublicMusicID(cursor.AfterID) {
		return musicPlaylistCursor{}, errors.New("invalid Music playlist cursor")
	}
	return cursor, nil
}

func decodeMusicCursor(value string, destination any) error {
	value = strings.TrimSpace(value)
	if value == "" || len(value) > maxPublicMusicCursorLength || strings.IndexFunc(value, unicode.IsSpace) >= 0 {
		return errors.New("invalid Music cursor")
	}
	payload, err := base64.RawURLEncoding.DecodeString(value)
	if err != nil || len(payload) > maxPublicMusicCursorLength {
		return errors.New("invalid Music cursor")
	}
	decoder := json.NewDecoder(bytes.NewReader(payload))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(destination); err != nil {
		return err
	}
	return ensureJSONEOF(decoder)
}

func validPublicMusicEpoch(value string) bool {
	return publicMusicEpochPattern.MatchString(strings.TrimSpace(value))
}

func validPublicMusicID(value string) bool {
	value = strings.TrimSpace(value)
	return len(value) <= maxPublicMusicIDLength && safePublicOpaqueID(value)
}

func publicMusicSnapshotEntityType(value string) bool {
	switch strings.TrimSpace(value) {
	case library.ListenLocalMusicEntityTrack,
		library.ListenLocalMusicEntityPlaylist,
		library.ListenLocalMusicEntityPlaylistItem,
		library.ListenLocalMusicEntityMembership,
		library.ListenLocalMusicEntityTrackState,
		library.ListenLocalMusicEntityLyricDocument,
		library.ListenLocalMusicEntityLyricSelection:
		return true
	default:
		return false
	}
}

func validPublicMusicMembershipReason(value string) bool {
	switch strings.TrimSpace(value) {
	case "", "user", "unsupported", "policy":
		return true
	default:
		return false
	}
}

func optionalPublicMusicText(value string) *string {
	value = safePublicDisplayText(value)
	if value == "" {
		return nil
	}
	return &value
}

func optionalPublicMusicToken(value string) *string {
	value = safePublicTechnicalToken(value)
	if value == "" {
		return nil
	}
	return &value
}

func positivePublicMusicInt(value int) *int {
	if value < 1 {
		return nil
	}
	copy := value
	return &copy
}

func nonNegativePublicMusicInt64(value *int64) *int64 {
	if value == nil || *value < 0 {
		return nil
	}
	copy := *value
	return &copy
}

func safePublicMusicMediaType(value string) string {
	value = strings.TrimSpace(value)
	if value == "" || len(value) > 255 {
		return ""
	}
	mediaType, _, err := mime.ParseMediaType(value)
	if err != nil || !strings.Contains(mediaType, "/") {
		return ""
	}
	return strings.ToLower(mediaType)
}

func safePublicMusicETag(value string) string {
	value = strings.TrimSpace(value)
	if len(value) < 3 || len(value) > 128 || value[0] != '"' || value[len(value)-1] != '"' ||
		strings.ContainsAny(value, "\r\n") {
		return ""
	}
	return value
}

func safePublicMusicChecksum(value string) string {
	value = strings.ToLower(strings.TrimSpace(value))
	algorithm, digest, found := strings.Cut(value, ":")
	if !found || (algorithm != "sha256" && algorithm != "blake3") || len(digest) != 64 {
		return ""
	}
	for _, char := range digest {
		if (char < '0' || char > '9') && (char < 'a' || char > 'f') {
			return ""
		}
	}
	return algorithm + ":" + digest
}

func validPublicMusicMutationRequest(payload publicMusicMutationRequest, family string) bool {
	allowlist, knownFamily := publicMusicMutationTypeAllowlist[family]
	_, knownType := allowlist[payload.Type]
	if !knownFamily || !knownType || !canonicalPublicMusicUUIDv4(payload.MutationID) ||
		!publicMusicRequestHashPattern.MatchString(payload.RequestHash) ||
		payload.EntityID == "" || payload.EntityID != strings.TrimSpace(payload.EntityID) ||
		len(payload.EntityID) > maxPublicMusicIDLength || payload.ExpectedRevision == nil ||
		*payload.ExpectedRevision < 0 || len(payload.Payload) == 0 {
		return false
	}
	if payload.DependsOnMutationID != nil {
		if !canonicalPublicMusicUUIDv4(*payload.DependsOnMutationID) || *payload.DependsOnMutationID == payload.MutationID {
			return false
		}
	}
	return true
}

func validPublicMusicPlayEventRequest(payload publicMusicPlayEventRequest) bool {
	return canonicalPublicMusicUUIDv4(payload.EventID) && canonicalPublicMusicUUIDv4(payload.PlaySessionID) &&
		publicMusicRequestHashPattern.MatchString(payload.RequestHash) &&
		payload.Sequence != nil && *payload.Sequence > 0 &&
		payload.TrackID != "" && payload.TrackID == strings.TrimSpace(payload.TrackID) &&
		len(payload.TrackID) <= maxPublicMusicIDLength &&
		payload.ContentIdentityRevision != nil && *payload.ContentIdentityRevision > 0 &&
		payload.CumulativeListenedDurationMs != nil && *payload.CumulativeListenedDurationMs >= 0 &&
		payload.PositionMs != nil && *payload.PositionMs >= 0 &&
		payload.Terminal != nil && payload.Completed != nil && (!*payload.Completed || *payload.Terminal) &&
		(payload.EndReason == nil || (*payload.EndReason == strings.TrimSpace(*payload.EndReason) && len(*payload.EndReason) <= 120))
}

func canonicalPublicMusicUUIDv4(value string) bool {
	parsed, err := uuid.Parse(value)
	return err == nil && parsed.Version() == 4 && parsed.String() == value
}

func decodePublicMusicJSONObject(payload json.RawMessage) (map[string]any, error) {
	decoder := json.NewDecoder(bytes.NewReader(payload))
	decoder.UseNumber()
	var result map[string]any
	if err := decoder.Decode(&result); err != nil || result == nil || ensureJSONEOF(decoder) != nil {
		return nil, errors.New("invalid Music mutation payload")
	}
	return result, nil
}

func publicMusicRequestHashMatches(supplied string, canonical any) bool {
	encoded, err := encodePublicMusicCanonicalJSON(canonical)
	if err != nil {
		return false
	}
	digest := sha256.Sum256(encoded)
	expected := "sha256:" + hex.EncodeToString(digest[:])
	return len(expected) == len(supplied) && subtle.ConstantTimeCompare([]byte(expected), []byte(supplied)) == 1
}

func encodePublicMusicCanonicalJSON(value any) ([]byte, error) {
	var buffer bytes.Buffer
	encoder := json.NewEncoder(&buffer)
	encoder.SetEscapeHTML(false)
	if err := encoder.Encode(value); err != nil {
		return nil, err
	}
	return bytes.TrimSuffix(buffer.Bytes(), []byte("\n")), nil
}

type publicMusicRevisionConflictEnvelope struct {
	Error           string          `json:"error"`
	CurrentRevision int64           `json:"currentRevision"`
	CurrentPayload  json.RawMessage `json:"currentPayload"`
}

func writeMusicMutationError(w http.ResponseWriter, err error) {
	var conflict *library.ListenLocalMusicRevisionConflictError
	switch {
	case errors.As(err, &conflict):
		current := conflict.Current
		if len(current) == 0 || string(current) == "null" {
			current = json.RawMessage(`{}`)
		}
		writeJSON(w, http.StatusConflict, publicMusicRevisionConflictEnvelope{
			Error: "music_revision_conflict", CurrentRevision: conflict.CurrentRevision, CurrentPayload: current,
		})
	case errors.Is(err, library.ErrListenLocalMusicIdempotencyConflict):
		writeError(w, http.StatusConflict, "music_idempotency_conflict")
	case errors.Is(err, library.ErrListenLocalMusicDependencyPending):
		writeJSON(w, http.StatusConflict, map[string]any{"error": "music_dependency_pending", "retryable": true})
	case errors.Is(err, library.ErrListenLocalMusicContentChanged):
		writeJSON(w, http.StatusConflict, map[string]any{"error": "music_content_changed", "retryable": false})
	case errors.Is(err, library.ErrInvalidListenLocalMusicMutation),
		errors.Is(err, library.ErrInvalidListenLocalPlaylist),
		errors.Is(err, library.ErrInvalidListenLocalMusicMembership):
		writeError(w, http.StatusBadRequest, "invalid_music_mutation")
	case errors.Is(err, sql.ErrNoRows), errors.Is(err, library.ErrFileNotFound),
		errors.Is(err, library.ErrListenLocalPlaylistNotFound),
		errors.Is(err, library.ErrListenLocalMusicMembershipNotFound):
		writeError(w, http.StatusNotFound, "music_entity_not_found")
	default:
		writeError(w, http.StatusInternalServerError, "internal_error")
	}
}

func writeMusicError(w http.ResponseWriter, err error) {
	var reset *library.ListenLocalMusicSyncResetError
	switch {
	case errors.As(err, &reset):
		result := publicMusicResetEnvelope{Error: "reset_required"}
		result.Sync.Epoch = reset.Position.Epoch
		result.Sync.Cursor = reset.Position.HighWater
		result.Sync.MinimumCursor = reset.Position.MinimumCursor
		writeJSON(w, http.StatusConflict, result)
	case errors.Is(err, sql.ErrNoRows), errors.Is(err, os.ErrNotExist), errors.Is(err, library.ErrFileNotFound):
		writeError(w, http.StatusNotFound, "not_found")
	default:
		writeError(w, http.StatusInternalServerError, "internal_error")
	}
}

var _ interface {
	Routes() []ProtectedRoute
} = (*MusicAPI)(nil)
