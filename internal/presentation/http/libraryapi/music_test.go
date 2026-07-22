package libraryapi

import (
	"bytes"
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/google/uuid"

	"xiadown/internal/application/library/access"
	"xiadown/internal/domain/library"
)

const testMusicEpoch = "0123456789abcdef0123456789abcdef"

type musicReadRepositoryStub struct {
	mu            sync.Mutex
	position      library.ListenLocalMusicSyncPosition
	snapshot      library.ListenLocalMusicSnapshotPage
	changes       library.ListenLocalMusicChangePage
	track         library.ListenLocalMusicTrackProjection
	playlists     library.ListenLocalMusicPlaylistPage
	resource      library.ListenLocalMusicResource
	positionErr   error
	snapshotErr   error
	changesErr    error
	trackErr      error
	playlistsErr  error
	resourceErr   error
	snapshotQuery library.ListenLocalMusicSnapshotQuery
	changesQuery  library.ListenLocalMusicChangeQuery
	playlistQuery library.ListenLocalMusicPlaylistQuery
	trackID       string
	resourceTrack string
	resourceID    string
	resolveCalls  int
}

type musicWriteRepositoryStub struct {
	mutation       library.ListenLocalMusicMutation
	mutationResult library.ListenLocalMusicMutationResult
	mutationErr    error
	mutationCalls  int
	playEvent      library.ListenLocalMusicPlayEvent
	playResult     library.ListenLocalMusicPlayEventResult
	playErr        error
	playCalls      int
}

type musicCompatibleRepresentationStub struct {
	status         library.ListenLocalMusicCompatibleRepresentationStatus
	statusErr      error
	requestStatus  library.ListenLocalMusicCompatibleRepresentationStatus
	requestErr     error
	statusTrackIDs []string
	requestTrackID string
	requestID      string
	statusCalls    int
	requestCalls   int
}

func (stub *musicCompatibleRepresentationStub) GetIOSCompatibleRepresentationStatuses(
	_ context.Context,
	trackIDs []string,
) (map[string]library.ListenLocalMusicCompatibleRepresentationStatus, error) {
	stub.statusCalls++
	stub.statusTrackIDs = append([]string(nil), trackIDs...)
	if stub.statusErr != nil {
		return nil, stub.statusErr
	}
	result := make(map[string]library.ListenLocalMusicCompatibleRepresentationStatus, len(trackIDs))
	for _, trackID := range trackIDs {
		result[trackID] = stub.status
	}
	return result, nil
}

func (stub *musicCompatibleRepresentationStub) RequestIOSCompatibleRepresentation(
	_ context.Context,
	trackID string,
	requestID string,
) (library.ListenLocalMusicCompatibleRepresentationStatus, error) {
	stub.requestCalls++
	stub.requestTrackID = trackID
	stub.requestID = requestID
	return stub.requestStatus, stub.requestErr
}

func (stub *musicWriteRepositoryStub) ApplyMutation(
	_ context.Context,
	mutation library.ListenLocalMusicMutation,
) (library.ListenLocalMusicMutationResult, error) {
	stub.mutationCalls++
	stub.mutation = mutation
	return stub.mutationResult, stub.mutationErr
}

func (stub *musicWriteRepositoryStub) ApplyPlayEvent(
	_ context.Context,
	event library.ListenLocalMusicPlayEvent,
) (library.ListenLocalMusicPlayEventResult, error) {
	stub.playCalls++
	stub.playEvent = event
	return stub.playResult, stub.playErr
}

func (stub *musicReadRepositoryStub) GetSyncPosition(context.Context) (library.ListenLocalMusicSyncPosition, error) {
	return stub.position, stub.positionErr
}

func (stub *musicReadRepositoryStub) ListSnapshot(
	_ context.Context,
	query library.ListenLocalMusicSnapshotQuery,
) (library.ListenLocalMusicSnapshotPage, error) {
	stub.snapshotQuery = query
	return stub.snapshot, stub.snapshotErr
}

func (stub *musicReadRepositoryStub) ListChanges(
	_ context.Context,
	query library.ListenLocalMusicChangeQuery,
) (library.ListenLocalMusicChangePage, error) {
	stub.changesQuery = query
	return stub.changes, stub.changesErr
}

func (stub *musicReadRepositoryStub) GetTrackProjection(
	_ context.Context,
	trackID string,
) (library.ListenLocalMusicTrackProjection, error) {
	stub.trackID = trackID
	return stub.track, stub.trackErr
}

func (stub *musicReadRepositoryStub) ListPlaylistProjections(
	_ context.Context,
	query library.ListenLocalMusicPlaylistQuery,
) (library.ListenLocalMusicPlaylistPage, error) {
	stub.playlistQuery = query
	return stub.playlists, stub.playlistsErr
}

func (stub *musicReadRepositoryStub) ResolveTrackResource(
	_ context.Context,
	trackID string,
	resourceID string,
) (library.ListenLocalMusicResource, error) {
	stub.mu.Lock()
	defer stub.mu.Unlock()
	stub.resolveCalls++
	stub.resourceTrack, stub.resourceID = trackID, resourceID
	if stub.resourceErr != nil {
		return library.ListenLocalMusicResource{}, stub.resourceErr
	}
	if trackID != "track-1" || resourceID != stub.resource.ID {
		return library.ListenLocalMusicResource{}, sql.ErrNoRows
	}
	return stub.resource, nil
}

func TestMusicAPIRoutesUseOnlyMusicReadAndRouterEnforcesIsolation(t *testing.T) {
	stub := &musicReadRepositoryStub{position: library.ListenLocalMusicSyncPosition{
		Epoch: testMusicEpoch, HighWater: 7, MinimumCursor: 2,
	}}
	api, err := NewMusicAPI(MusicConfig{CatalogID: "catalog-1", Reader: stub})
	if err != nil {
		t.Fatal(err)
	}
	wantRoutes := map[string]struct{}{
		"GET /api/v1/music/overview":                                    {},
		"GET /api/v1/music/snapshot":                                    {},
		"GET /api/v1/music/changes":                                     {},
		"GET /api/v1/music/tracks/{id}":                                 {},
		"GET /api/v1/music/playlists":                                   {},
		"GET /api/v1/music/tracks/{id}/resources/{resourceId}/content":  {},
		"HEAD /api/v1/music/tracks/{id}/resources/{resourceId}/content": {},
	}
	for _, route := range api.Routes() {
		if route.Scope != library.DeviceScopeMusicRead {
			t.Fatalf("Music route %s %s scope=%q", route.Method, route.Path, route.Scope)
		}
		delete(wantRoutes, route.Method+" "+route.Path)
	}
	if len(wantRoutes) != 0 {
		t.Fatalf("missing Music routes: %#v", wantRoutes)
	}

	authenticator := &fakeAuthenticator{principal: access.Principal{
		GrantID: "grant-1", CatalogID: "catalog-1", Scopes: []library.DeviceScope{library.DeviceScopeLibraryRead},
	}}
	router, err := NewRouter(Config{
		Version: "test", CatalogID: "catalog-1", Authenticator: authenticator, Pairer: &fakePairer{},
		Routes: append(api.Routes(), ProtectedRoute{
			Method: http.MethodGet, Path: "/api/v1/library/probe", Scope: library.DeviceScopeLibraryRead,
			Handler: http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusNoContent) }),
		}),
	})
	if err != nil {
		t.Fatal(err)
	}
	forbidden := performRequest(router, http.MethodGet, "/api/v1/music/overview", "Bearer library-only", "")
	if forbidden.Code != http.StatusForbidden || !strings.Contains(forbidden.Body.String(), "insufficient_scope") {
		t.Fatalf("library.read reached Music: %d %s", forbidden.Code, forbidden.Body.String())
	}
	authenticator.principal.Scopes = []library.DeviceScope{library.DeviceScopeMusicRead}
	allowed := performRequest(router, http.MethodGet, "/api/v1/music/overview", "Bearer music", "")
	if allowed.Code != http.StatusOK {
		t.Fatalf("music.read overview=%d: %s", allowed.Code, allowed.Body.String())
	}
	libraryForbidden := performRequest(router, http.MethodGet, "/api/v1/library/probe", "Bearer music", "")
	if libraryForbidden.Code != http.StatusForbidden {
		t.Fatalf("music.read enumerated Library route: %d", libraryForbidden.Code)
	}
	loopback := performRequest(router, http.MethodGet, "/api/listen/local/tracks", "Bearer music", "")
	if loopback.Code != http.StatusNotFound {
		t.Fatalf("public router exposed loopback Listen route: %d", loopback.Code)
	}
}

func TestMusicCompatibleRepresentationCommandIsCapabilityAndManageScoped(t *testing.T) {
	now := time.Now().UTC()
	reader := &musicReadRepositoryStub{
		position: library.ListenLocalMusicSyncPosition{Epoch: testMusicEpoch, HighWater: 7, MinimumCursor: 2},
		track:    testUnsupportedMusicTrackProjection(now),
	}
	coordinator := &musicCompatibleRepresentationStub{
		status: library.ListenLocalMusicCompatibleRepresentationStatus{
			Status: library.ListenLocalMusicCompatibleRepresentationUnsupported,
		},
		requestStatus: library.ListenLocalMusicCompatibleRepresentationStatus{
			Status: library.ListenLocalMusicCompatibleRepresentationGenerating,
		},
	}
	api, err := NewMusicAPI(MusicConfig{
		CatalogID: "catalog-1", Reader: reader, CompatibleRepresentation: coordinator,
	})
	if err != nil {
		t.Fatal(err)
	}
	if !musicTestContains(api.capabilities(), MusicIOSAudioRepresentationCapability) {
		t.Fatalf("overview capabilities=%#v", api.capabilities())
	}
	commandScope := library.DeviceScope("")
	for _, route := range api.Routes() {
		if route.Method == http.MethodPost && route.Path == "/api/v1/music/tracks/{id}/compatible-representation" {
			commandScope = route.Scope
		}
	}
	if commandScope != library.DeviceScopeMusicManage {
		t.Fatalf("compatible representation scope=%q", commandScope)
	}

	authenticator := &fakeAuthenticator{principal: access.Principal{
		GrantID: "grant-1", CatalogID: "catalog-1", DeviceID: "device-1",
		Scopes: []library.DeviceScope{library.DeviceScopeMusicRead},
	}}
	router, err := NewRouter(Config{
		Version: "test", CatalogID: "catalog-1", Authenticator: authenticator, Pairer: &fakePairer{},
		Routes: api.Routes(),
	})
	if err != nil {
		t.Fatal(err)
	}
	requestID := "f2d2a831-7a90-4bee-9c19-bcd8e9f36d64"
	body := `{"requestId":"` + requestID + `"}`
	path := "/api/v1/music/tracks/track-1/compatible-representation"
	forbidden := performRequest(router, http.MethodPost, path, "Bearer music-read", body)
	if forbidden.Code != http.StatusForbidden || coordinator.requestCalls != 0 {
		t.Fatalf("music.read command=%d calls=%d", forbidden.Code, coordinator.requestCalls)
	}
	authenticator.principal.Scopes = []library.DeviceScope{library.DeviceScopeMusicManage}
	accepted := performRequest(router, http.MethodPost, path, "Bearer music-manage", body)
	if accepted.Code != http.StatusAccepted {
		t.Fatalf("music.manage command=%d %s", accepted.Code, accepted.Body.String())
	}
	if coordinator.requestCalls != 1 || coordinator.requestTrackID != "track-1" || coordinator.requestID != requestID {
		t.Fatalf("command projection calls=%d track=%q request=%q", coordinator.requestCalls, coordinator.requestTrackID, coordinator.requestID)
	}
	if !strings.Contains(accepted.Body.String(), `"status":"generating"`) {
		t.Fatalf("command response=%s", accepted.Body.String())
	}
	assertMusicJSONHasNoPrivateLocation(t, accepted.Body.Bytes())
}

func TestMusicTrackProjectsOriginalRepresentationGeneratingFailedAndUnsupported(t *testing.T) {
	now := time.Now().UTC()
	readyOriginal, err := publicMusicTrackFromProjection(testMusicTrackProjection(now))
	if err != nil || readyOriginal.CompatibleRepresentation == nil ||
		readyOriginal.CompatibleRepresentation.Status != library.ListenLocalMusicCompatibleRepresentationReady {
		t.Fatalf("original projection=%#v err=%v", readyOriginal.CompatibleRepresentation, err)
	}
	representationProjection := testMusicTrackProjection(now)
	representationProjection.PlaybackResources[0].Kind = library.ListenLocalMusicResourcePlaybackRepresentation
	representationProjection.PlaybackResources[0].MediaType = "audio/mp4"
	representationProjection.PlaybackResources[0].Container = "m4a"
	representationProjection.PlaybackResources[0].Codec = "aac"
	readyRepresentation, err := publicMusicTrackFromProjection(representationProjection)
	if err != nil || readyRepresentation.CompatibleRepresentation == nil ||
		readyRepresentation.CompatibleRepresentation.Status != library.ListenLocalMusicCompatibleRepresentationReady {
		t.Fatalf("representation projection=%#v err=%v", readyRepresentation.CompatibleRepresentation, err)
	}

	for _, test := range []struct {
		name      string
		status    string
		errorCode string
	}{
		{name: "generating", status: library.ListenLocalMusicCompatibleRepresentationGenerating},
		{name: "failed", status: library.ListenLocalMusicCompatibleRepresentationFailed, errorCode: "generation_failed"},
		{name: "unsupported", status: library.ListenLocalMusicCompatibleRepresentationUnsupported},
	} {
		t.Run(test.name, func(t *testing.T) {
			reader := &musicReadRepositoryStub{track: testUnsupportedMusicTrackProjection(now)}
			coordinator := &musicCompatibleRepresentationStub{status: library.ListenLocalMusicCompatibleRepresentationStatus{
				Status: test.status, ErrorCode: test.errorCode,
			}}
			api, apiErr := NewMusicAPI(MusicConfig{
				CatalogID: "catalog-1", Reader: reader, CompatibleRepresentation: coordinator,
			})
			if apiErr != nil {
				t.Fatal(apiErr)
			}
			request := httptest.NewRequest(http.MethodGet, "/api/v1/music/tracks/track-1", nil)
			request.SetPathValue("id", "track-1")
			response := httptest.NewRecorder()
			api.track(response, request)
			if response.Code != http.StatusOK {
				t.Fatalf("track=%d %s", response.Code, response.Body.String())
			}
			var payload publicMusicTrack
			if decodeErr := json.Unmarshal(response.Body.Bytes(), &payload); decodeErr != nil {
				t.Fatal(decodeErr)
			}
			if payload.CompatibleRepresentation == nil || payload.CompatibleRepresentation.Status != test.status ||
				payload.CompatibleRepresentation.ErrorCode != test.errorCode {
				t.Fatalf("lifecycle=%#v", payload.CompatibleRepresentation)
			}
			assertMusicJSONHasNoPrivateLocation(t, response.Body.Bytes())
		})
	}
}

func TestMusicSnapshotBatchesCompatibleRepresentationStatusForWholePage(t *testing.T) {
	now := time.Date(2026, 7, 22, 8, 0, 0, 0, time.UTC)
	first := testUnsupportedMusicTrackProjection(now)
	second := testUnsupportedMusicTrackProjection(now.Add(time.Second))
	second.Track.FileID = "track-2"
	entities := make([]library.ListenLocalMusicCanonicalEntity, 0, 2)
	for _, projection := range []library.ListenLocalMusicTrackProjection{first, second} {
		copy := projection
		entities = append(entities, library.ListenLocalMusicCanonicalEntity{
			EntityType: library.ListenLocalMusicEntityTrack, EntityID: copy.Track.FileID,
			Revision: copy.Track.Revision, Track: &copy,
		})
	}
	reader := &musicReadRepositoryStub{snapshot: library.ListenLocalMusicSnapshotPage{
		Entities: entities,
		Position: library.ListenLocalMusicSyncPosition{Epoch: testMusicEpoch, HighWater: 9},
	}}
	coordinator := &musicCompatibleRepresentationStub{status: library.ListenLocalMusicCompatibleRepresentationStatus{
		Status: library.ListenLocalMusicCompatibleRepresentationGenerating,
	}}
	api, err := NewMusicAPI(MusicConfig{
		CatalogID: "catalog-1", Reader: reader, CompatibleRepresentation: coordinator,
	})
	if err != nil {
		t.Fatal(err)
	}
	response := httptest.NewRecorder()
	api.snapshot(response, httptest.NewRequest(http.MethodGet,
		"/api/v1/music/snapshot?epoch="+testMusicEpoch+"&highWater=9", nil))
	if response.Code != http.StatusOK {
		t.Fatalf("snapshot=%d %s", response.Code, response.Body.String())
	}
	if coordinator.statusCalls != 1 || !reflect.DeepEqual(
		coordinator.statusTrackIDs, []string{"track-1", "track-2"},
	) {
		t.Fatalf("status batches=%d Track IDs=%#v", coordinator.statusCalls, coordinator.statusTrackIDs)
	}
	if strings.Count(response.Body.String(), `"status":"generating"`) != 2 {
		t.Fatalf("batched lifecycle projection=%s", response.Body.String())
	}
}

func TestMusicWriteRoutesEnforceFamilyHashIdentityAndPlayEventContract(t *testing.T) {
	reader := &musicReadRepositoryStub{position: library.ListenLocalMusicSyncPosition{
		Epoch: testMusicEpoch, HighWater: 7, MinimumCursor: 2,
	}}
	writer := &musicWriteRepositoryStub{mutationResult: library.ListenLocalMusicMutationResult{
		MutationID: "placeholder", Type: "setFavorite", EntityID: "track-1", Revision: 1,
		Result: json.RawMessage(`{"trackState":{"revision":1}}`),
	}}
	api, err := NewMusicAPI(MusicConfig{CatalogID: "catalog-1", Reader: reader, Writer: writer})
	if err != nil {
		t.Fatal(err)
	}
	routeScopes := map[string]library.DeviceScope{}
	for _, route := range api.Routes() {
		routeScopes[route.Method+" "+route.Path] = route.Scope
	}
	if routeScopes["POST /api/v1/music/state/mutations"] != library.DeviceScopeMusicState ||
		routeScopes["POST /api/v1/music/manage/mutations"] != library.DeviceScopeMusicManage ||
		routeScopes["POST /api/v1/music/play-events"] != library.DeviceScopeMusicState {
		t.Fatalf("write route scopes=%#v", routeScopes)
	}
	for _, capability := range publicMusicWriteCapabilities {
		if !musicTestContains(api.capabilities(), capability) {
			t.Fatalf("writer overview omitted capability %q: %#v", capability, api.capabilities())
		}
	}

	mutationID := uuid.NewString()
	mutationPayload := map[string]any{"favorite": true}
	mutationCanonical := map[string]any{
		"family": "state", "mutationId": mutationID, "type": "setFavorite",
		"entityId": "track-1", "expectedRevision": int64(0), "payload": mutationPayload,
	}
	requestHash := musicTestRequestHash(t, mutationCanonical)
	body := mustMusicTestJSON(t, map[string]any{
		"mutationId": mutationID, "requestHash": requestHash, "type": "setFavorite",
		"entityId": "track-1", "expectedRevision": int64(0), "payload": mutationPayload,
	})
	request := httptest.NewRequest(http.MethodPost, "/api/v1/music/state/mutations", strings.NewReader(body))
	request = request.WithContext(context.WithValue(request.Context(), principalContextKey{}, access.Principal{DeviceID: "iphone-1"}))
	response := httptest.NewRecorder()
	api.stateMutation(response, request)
	if response.Code != http.StatusOK || writer.mutationCalls != 1 ||
		writer.mutation.ActorDeviceID != "iphone-1" || writer.mutation.Family != "state" ||
		writer.mutation.RequestHash != requestHash {
		t.Fatalf("state mutation status=%d calls=%d mutation=%#v body=%s",
			response.Code, writer.mutationCalls, writer.mutation, response.Body.String())
	}

	wrongFamilyCanonical := map[string]any{
		"family": "state", "mutationId": uuid.NewString(), "type": "createPlaylist",
		"entityId": uuid.NewString(), "expectedRevision": int64(0), "payload": map[string]any{"name": "No"},
	}
	wrongFamilyBody := mustMusicTestJSON(t, map[string]any{
		"mutationId": wrongFamilyCanonical["mutationId"], "requestHash": musicTestRequestHash(t, wrongFamilyCanonical),
		"type": "createPlaylist", "entityId": wrongFamilyCanonical["entityId"],
		"expectedRevision": int64(0), "payload": map[string]any{"name": "No"},
	})
	request = httptest.NewRequest(http.MethodPost, "/api/v1/music/state/mutations", strings.NewReader(wrongFamilyBody))
	request = request.WithContext(context.WithValue(request.Context(), principalContextKey{}, access.Principal{DeviceID: "iphone-1"}))
	response = httptest.NewRecorder()
	api.stateMutation(response, request)
	if response.Code != http.StatusBadRequest || writer.mutationCalls != 1 {
		t.Fatalf("cross-family mutation status=%d calls=%d body=%s", response.Code, writer.mutationCalls, response.Body.String())
	}

	wrongManageCanonical := map[string]any{
		"family": "manage", "mutationId": uuid.NewString(), "type": "setFavorite",
		"entityId": "track-1", "expectedRevision": int64(0), "payload": map[string]any{"favorite": true},
	}
	wrongManageBody := mustMusicTestJSON(t, map[string]any{
		"mutationId": wrongManageCanonical["mutationId"], "requestHash": musicTestRequestHash(t, wrongManageCanonical),
		"type": "setFavorite", "entityId": "track-1",
		"expectedRevision": int64(0), "payload": map[string]any{"favorite": true},
	})
	request = httptest.NewRequest(http.MethodPost, "/api/v1/music/manage/mutations", strings.NewReader(wrongManageBody))
	request = request.WithContext(context.WithValue(request.Context(), principalContextKey{}, access.Principal{DeviceID: "iphone-1"}))
	response = httptest.NewRecorder()
	api.manageMutation(response, request)
	if response.Code != http.StatusBadRequest || writer.mutationCalls != 1 {
		t.Fatalf("manage endpoint accepted state mutation: status=%d calls=%d body=%s",
			response.Code, writer.mutationCalls, response.Body.String())
	}

	authenticator := &fakeAuthenticator{principal: access.Principal{
		GrantID: "grant-1", CatalogID: "catalog-1", DeviceID: "iphone-1",
		Scopes: []library.DeviceScope{library.DeviceScopeMusicManage},
	}}
	router, routerErr := NewRouter(Config{
		Version: "test", CatalogID: "catalog-1", Authenticator: authenticator, Pairer: &fakePairer{},
		Routes: api.Routes(),
	})
	if routerErr != nil {
		t.Fatal(routerErr)
	}
	manageCannotCallState := performRequest(
		router, http.MethodPost, "/api/v1/music/state/mutations", "Bearer manage-only", body,
	)
	if manageCannotCallState.Code != http.StatusForbidden ||
		!strings.Contains(manageCannotCallState.Body.String(), "insufficient_scope") {
		t.Fatalf("music.manage reached state route: %d %s", manageCannotCallState.Code, manageCannotCallState.Body.String())
	}
	authenticator.principal.Scopes = []library.DeviceScope{library.DeviceScopeMusicState}
	stateCannotCallManage := performRequest(
		router, http.MethodPost, "/api/v1/music/manage/mutations", "Bearer state-only", wrongFamilyBody,
	)
	if stateCannotCallManage.Code != http.StatusForbidden ||
		!strings.Contains(stateCannotCallManage.Body.String(), "insufficient_scope") {
		t.Fatalf("music.state reached manage route: %d %s", stateCannotCallManage.Code, stateCannotCallManage.Body.String())
	}

	playSessionID, eventID := uuid.NewString(), uuid.NewString()
	playCanonical := map[string]any{
		"playSessionId": playSessionID, "sequence": int64(1), "trackId": "track-1",
		"contentIdentityRevision": int64(2), "cumulativeListenedDurationMs": int64(12_000),
		"positionMs": int64(10_000), "terminal": false, "completed": false,
	}
	playHash := musicTestRequestHash(t, playCanonical)
	writer.playResult = library.ListenLocalMusicPlayEventResult{
		EventID: eventID, PlaySessionID: playSessionID, Sequence: 1, Accepted: true,
	}
	playBody := mustMusicTestJSON(t, map[string]any{
		"eventId": eventID, "requestHash": playHash, "playSessionId": playSessionID,
		"sequence": int64(1), "trackId": "track-1", "contentIdentityRevision": int64(2),
		"cumulativeListenedDurationMs": int64(12_000), "positionMs": int64(10_000),
		"terminal": false, "completed": false,
	})
	request = httptest.NewRequest(http.MethodPost, "/api/v1/music/play-events", strings.NewReader(playBody))
	request = request.WithContext(context.WithValue(request.Context(), principalContextKey{}, access.Principal{DeviceID: "iphone-1"}))
	response = httptest.NewRecorder()
	api.playEvent(response, request)
	if response.Code != http.StatusOK || writer.playCalls != 1 || writer.playEvent.EventID != eventID ||
		writer.playEvent.RequestHash != playHash || !strings.Contains(response.Body.String(), `"acknowledgedSequence":1`) {
		t.Fatalf("play event status=%d calls=%d event=%#v body=%s", response.Code, writer.playCalls, writer.playEvent, response.Body.String())
	}
}

func TestMusicCanonicalRequestHashCrossLanguageFixtures(t *testing.T) {
	mutation := map[string]any{
		"family":              "state",
		"mutationId":          "11111111-1111-4111-8111-111111111111",
		"type":                "setFavorite",
		"entityId":            "track-1",
		"expectedRevision":    int64(8),
		"dependsOnMutationId": "22222222-2222-4222-8222-222222222222",
		"payload":             map[string]any{"favorite": true},
	}
	if got, want := musicTestRequestHash(t, mutation), "sha256:2967676d375e8f9e3a7047fc7687cfbc7aa4ac559ce86619ce46450298598368"; got != want {
		t.Fatalf("mutation canonical hash=%q want=%q", got, want)
	}
	playEvent := map[string]any{
		"playSessionId": "33333333-3333-4333-8333-333333333333",
		"sequence":      int64(7), "trackId": "track-1", "contentIdentityRevision": int64(3),
		"cumulativeListenedDurationMs": int64(90_000), "positionMs": int64(88_000),
		"terminal": true, "completed": true, "endReason": "completed",
		"deviceOccurredAt": "2026-07-21T12:34:56.789Z",
	}
	if got, want := musicTestRequestHash(t, playEvent), "sha256:d183cd4ab9df95a836e32442f56c035988eba998b33e511cc0fbee888f0af660"; got != want {
		t.Fatalf("play-event canonical hash=%q want=%q", got, want)
	}
}

func TestMusicMutationConflictEnvelopesAreStable(t *testing.T) {
	tests := []struct {
		name      string
		err       error
		status    int
		fragments []string
	}{
		{
			name: "revision",
			err: &library.ListenLocalMusicRevisionConflictError{
				CurrentRevision: 7, Current: json.RawMessage(`{"favorite":true}`),
			},
			status:    http.StatusConflict,
			fragments: []string{`"error":"music_revision_conflict"`, `"currentRevision":7`, `"currentPayload":{"favorite":true}`},
		},
		{name: "idempotency", err: library.ErrListenLocalMusicIdempotencyConflict, status: http.StatusConflict,
			fragments: []string{`"error":"music_idempotency_conflict"`}},
		{name: "dependency", err: library.ErrListenLocalMusicDependencyPending, status: http.StatusConflict,
			fragments: []string{`"error":"music_dependency_pending"`, `"retryable":true`}},
		{name: "content", err: library.ErrListenLocalMusicContentChanged, status: http.StatusConflict,
			fragments: []string{`"error":"music_content_changed"`, `"retryable":false`}},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			response := httptest.NewRecorder()
			writeMusicMutationError(response, test.err)
			if response.Code != test.status {
				t.Fatalf("status=%d body=%s", response.Code, response.Body.String())
			}
			for _, fragment := range test.fragments {
				if !strings.Contains(response.Body.String(), fragment) {
					t.Fatalf("body omitted %q: %s", fragment, response.Body.String())
				}
			}
		})
	}
}

func TestMusicOverviewSnapshotChangesAndTrackMatchSwiftWireShapeWithoutPrivatePaths(t *testing.T) {
	now := time.Date(2026, 7, 21, 12, 0, 0, 123, time.UTC)
	projection := testMusicTrackProjection(now)
	entity := library.ListenLocalMusicCanonicalEntity{
		EntityType: library.ListenLocalMusicEntityTrack, EntityID: projection.Track.FileID,
		Revision: projection.Track.Revision, Track: &projection,
	}
	stub := &musicReadRepositoryStub{
		position: library.ListenLocalMusicSyncPosition{Epoch: testMusicEpoch, HighWater: 42, MinimumCursor: 7},
		track:    projection,
		snapshot: library.ListenLocalMusicSnapshotPage{
			Entities: []library.ListenLocalMusicCanonicalEntity{entity},
			Position: library.ListenLocalMusicSyncPosition{Epoch: testMusicEpoch, HighWater: 42, MinimumCursor: 7},
			NextType: library.ListenLocalMusicEntityTrack, NextEntity: projection.Track.FileID, HasMore: true,
		},
		changes: library.ListenLocalMusicChangePage{
			Changes: []library.ListenLocalMusicChange{{
				Sequence: 42, EntityType: library.ListenLocalMusicEntityTrack, EntityID: projection.Track.FileID,
				Operation: "upsert", Revision: projection.Track.Revision, OccurredAt: now, Entity: &entity,
			}},
			Position: library.ListenLocalMusicSyncPosition{Epoch: testMusicEpoch, HighWater: 42, MinimumCursor: 7},
			Cursor:   42,
		},
	}
	api, err := NewMusicAPI(MusicConfig{CatalogID: "catalog-1", Reader: stub})
	if err != nil {
		t.Fatal(err)
	}

	overview := httptest.NewRecorder()
	api.overview(overview, httptest.NewRequest(http.MethodGet, "/api/v1/music/overview", nil))
	if overview.Code != http.StatusOK || !strings.Contains(overview.Body.String(), `"workspaceId":"music-default"`) ||
		!strings.Contains(overview.Body.String(), `"subjectId":"music-owner"`) ||
		!strings.Contains(overview.Body.String(), `"minimumCursor":7`) {
		t.Fatalf("overview=%d %s", overview.Code, overview.Body.String())
	}

	snapshot := httptest.NewRecorder()
	api.snapshot(snapshot, httptest.NewRequest(http.MethodGet,
		"/api/v1/music/snapshot?epoch="+testMusicEpoch+"&highWater=42&limit=25", nil))
	if snapshot.Code != http.StatusOK || stub.snapshotQuery.Epoch != testMusicEpoch ||
		stub.snapshotQuery.HighWater != 42 || stub.snapshotQuery.Limit != 25 ||
		!strings.Contains(snapshot.Body.String(), `"entityType":"track"`) ||
		!strings.Contains(snapshot.Body.String(), `"nextCursor":`) {
		t.Fatalf("snapshot=%d query=%#v body=%s", snapshot.Code, stub.snapshotQuery, snapshot.Body.String())
	}

	changes := httptest.NewRecorder()
	api.changes(changes, httptest.NewRequest(http.MethodGet,
		"/api/v1/music/changes?epoch="+testMusicEpoch+"&after=7&limit=30", nil))
	if changes.Code != http.StatusOK || stub.changesQuery.After != 7 || stub.changesQuery.Limit != 30 ||
		!strings.Contains(changes.Body.String(), `"operation":"upsert"`) {
		t.Fatalf("changes=%d query=%#v body=%s", changes.Code, stub.changesQuery, changes.Body.String())
	}

	trackRequest := httptest.NewRequest(http.MethodGet, "/api/v1/music/tracks/track-1", nil)
	trackRequest.SetPathValue("id", "track-1")
	track := httptest.NewRecorder()
	api.track(track, trackRequest)
	if track.Code != http.StatusOK || stub.trackID != "track-1" {
		t.Fatalf("track=%d id=%q body=%s", track.Code, stub.trackID, track.Body.String())
	}
	var payload map[string]any
	if err := json.Unmarshal(track.Body.Bytes(), &payload); err != nil {
		t.Fatal(err)
	}
	for _, required := range []string{
		"id", "revision", "contentIdentityRevision", "metadataRevision", "resourceRevision", "title",
		"availability", "playbackResources", "artworkResource", "createdAt", "updatedAt", "deletedAt",
	} {
		if _, exists := payload[required]; !exists {
			t.Fatalf("MusicTrack missing Swift field %q: %#v", required, payload)
		}
	}
	assertMusicJSONHasNoPrivateLocation(t, track.Body.Bytes())
	assertMusicJSONHasNoPrivateLocation(t, snapshot.Body.Bytes())
	assertMusicJSONHasNoPrivateLocation(t, changes.Body.Bytes())
	if !strings.Contains(track.Body.String(), `"checksum":"sha256:`) {
		t.Fatalf("Track descriptor lost checksum algorithm: %s", track.Body.String())
	}
}

func TestMusicCanonicalPathFallbackMapperRemainsPathAndFileIDFree(t *testing.T) {
	projection := testMusicTrackProjection(time.Date(2026, 7, 22, 19, 0, 0, 0, time.UTC))
	projection.CatalogItemID = ""
	projection.Track.LocalPath = "/Users/arnold/Private/Music/legacy-song.mp3"
	projection.Track.CoverLocalPath = "/Users/arnold/Private/Music/legacy-cover.jpg"
	projection.PlaybackResources[0].FileID = "private-legacy-playback-file"
	projection.PlaybackResources[0].LocalPath = projection.Track.LocalPath
	projection.ArtworkResource.FileID = "private-legacy-artwork-file"
	projection.ArtworkResource.LocalPath = projection.Track.CoverLocalPath

	mapped, err := publicMusicTrackFromProjection(projection)
	if err != nil {
		t.Fatal(err)
	}
	encoded, err := json.Marshal(mapped)
	if err != nil {
		t.Fatal(err)
	}
	assertMusicJSONHasNoPrivateLocation(t, encoded)
	for _, forbidden := range []string{
		"private-legacy-playback-file", "private-legacy-artwork-file", "legacy-song.mp3", "legacy-cover.jpg",
	} {
		if strings.Contains(strings.ToLower(string(encoded)), forbidden) {
			t.Fatalf("fallback mapper leaked %q: %s", forbidden, encoded)
		}
	}
	if !strings.Contains(string(encoded), `"resourceId":"mr1_playback"`) ||
		!strings.Contains(string(encoded), `"resourceId":"mr1_artwork"`) {
		t.Fatalf("fallback mapper omitted opaque descriptors: %s", encoded)
	}
}

func TestMusicMembershipSnapshotAndChangesUseOpaqueSwiftWireShape(t *testing.T) {
	now := time.Date(2026, 7, 21, 12, 30, 0, 123, time.UTC)
	membership, err := library.NewListenLocalMusicMembership(library.ListenLocalMusicMembershipParams{
		FileID: "track-1", State: string(library.ListenLocalMusicMembershipExcluded),
		Reason: "user", Revision: 4, CreatedAt: &now, UpdatedAt: &now,
	})
	if err != nil {
		t.Fatal(err)
	}
	entity := library.ListenLocalMusicCanonicalEntity{
		EntityType: library.ListenLocalMusicEntityMembership, EntityID: membership.FileID,
		Revision: membership.Revision, Membership: &membership,
	}
	position := library.ListenLocalMusicSyncPosition{Epoch: testMusicEpoch, HighWater: 43, MinimumCursor: 7}
	stub := &musicReadRepositoryStub{
		position: position,
		snapshot: library.ListenLocalMusicSnapshotPage{
			Entities: []library.ListenLocalMusicCanonicalEntity{entity}, Position: position,
		},
		changes: library.ListenLocalMusicChangePage{
			Changes: []library.ListenLocalMusicChange{{
				Sequence: 43, EntityType: entity.EntityType, EntityID: entity.EntityID,
				Operation: "upsert", Revision: entity.Revision, OccurredAt: now, Entity: &entity,
			}},
			Position: position, Cursor: 43,
		},
	}
	api, err := NewMusicAPI(MusicConfig{CatalogID: "catalog-1", Reader: stub})
	if err != nil {
		t.Fatal(err)
	}

	snapshot := httptest.NewRecorder()
	api.snapshot(snapshot, httptest.NewRequest(http.MethodGet,
		"/api/v1/music/snapshot?epoch="+testMusicEpoch+"&highWater=43", nil))
	changes := httptest.NewRecorder()
	api.changes(changes, httptest.NewRequest(http.MethodGet,
		"/api/v1/music/changes?epoch="+testMusicEpoch+"&after=42", nil))
	for name, response := range map[string]*httptest.ResponseRecorder{"snapshot": snapshot, "changes": changes} {
		if response.Code != http.StatusOK ||
			!strings.Contains(response.Body.String(), `"entityType":"membership"`) ||
			!strings.Contains(response.Body.String(), `"fileId":"track-1"`) ||
			!strings.Contains(response.Body.String(), `"state":"excluded"`) ||
			!strings.Contains(response.Body.String(), `"reason":"user"`) ||
			strings.Contains(response.Body.String(), "FileID") || strings.Contains(response.Body.String(), "/Users/") {
			t.Fatalf("%s membership=%d %s", name, response.Code, response.Body.String())
		}
	}
}

func TestMusicResetRequiredEnvelopeAndCursorValidation(t *testing.T) {
	position := library.ListenLocalMusicSyncPosition{Epoch: testMusicEpoch, HighWater: 99, MinimumCursor: 50}
	stub := &musicReadRepositoryStub{
		position:   position,
		changesErr: &library.ListenLocalMusicSyncResetError{Position: position},
	}
	api, _ := NewMusicAPI(MusicConfig{
		CatalogID: "catalog-1", Reader: stub, ResourceCacheDirectory: filepath.Join(t.TempDir(), "music-cas"),
	})

	response := httptest.NewRecorder()
	api.changes(response, httptest.NewRequest(http.MethodGet,
		"/api/v1/music/changes?epoch="+testMusicEpoch+"&after=1", nil))
	if response.Code != http.StatusConflict || response.Body.String() !=
		`{"error":"reset_required","sync":{"epoch":"`+testMusicEpoch+`","cursor":99,"minimumCursor":50}}`+"\n" {
		t.Fatalf("reset response=%d %s", response.Code, response.Body.String())
	}

	for _, target := range []string{
		"/api/v1/music/snapshot?epoch=BAD&highWater=99",
		"/api/v1/music/snapshot?epoch=" + testMusicEpoch,
		"/api/v1/music/snapshot?epoch=" + testMusicEpoch + "&highWater=99&cursor=not_base64!",
	} {
		invalid := httptest.NewRecorder()
		api.snapshot(invalid, httptest.NewRequest(http.MethodGet, target, nil))
		if invalid.Code != http.StatusBadRequest {
			t.Fatalf("invalid snapshot %s status=%d body=%s", target, invalid.Code, invalid.Body.String())
		}
	}
}

func TestMusicResourceContentUsesVersionBoundTrackOwnershipAndSingleRange(t *testing.T) {
	directory := t.TempDir()
	path := filepath.Join(directory, "track.bin")
	content := []byte("0123456789abcdefghijklmnopqrstuvwxyz")
	if err := os.WriteFile(path, content, 0o600); err != nil {
		t.Fatal(err)
	}
	path, err := filepath.EvalSymlinks(path)
	if err != nil {
		t.Fatal(err)
	}
	info, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	length := int64(len(content))
	contentDigest := sha256.Sum256(content)
	stub := &musicReadRepositoryStub{resource: library.ListenLocalMusicResource{
		ID: "mr1_current", FileID: "private-file-id", Revision: 4,
		Kind: library.ListenLocalMusicResourceOriginal, MediaType: "audio/mpeg", ByteLength: &length,
		ETag: `"music-etag"`, Checksum: "sha256:" + hex.EncodeToString(contentDigest[:]),
		Availability: "available", LocalPath: path, ModTimeUnixNano: info.ModTime().UnixNano(),
	}}
	api, _ := NewMusicAPI(MusicConfig{
		CatalogID: "catalog-1", Reader: stub, ResourceCacheDirectory: filepath.Join(directory, "music-cas"),
	})
	var materializations atomic.Int32
	api.resourceCache.beforeCopy = func() { materializations.Add(1) }

	request := musicResourceRequest(http.MethodGet, "track-1", "mr1_current", "bytes=4-9")
	partial := httptest.NewRecorder()
	api.resourceContent(partial, request)
	if partial.Code != http.StatusPartialContent || partial.Body.String() != "456789" ||
		partial.Header().Get("Content-Range") != "bytes 4-9/36" ||
		partial.Header().Get("Accept-Ranges") != "bytes" || partial.Header().Get("ETag") != `"music-etag"` {
		t.Fatalf("partial=%d headers=%#v body=%q", partial.Code, partial.Header(), partial.Body.String())
	}
	if stub.resourceTrack != "track-1" || stub.resourceID != "mr1_current" {
		t.Fatalf("resolver ownership args=(%q,%q)", stub.resourceTrack, stub.resourceID)
	}

	head := httptest.NewRecorder()
	api.resourceContent(head, musicResourceRequest(http.MethodHead, "track-1", "mr1_current", ""))
	if head.Code != http.StatusOK || head.Body.Len() != 0 || head.Header().Get("Content-Length") != "36" {
		t.Fatalf("HEAD=%d headers=%#v body=%q", head.Code, head.Header(), head.Body.String())
	}
	matchingIfRange := musicResourceRequest(http.MethodGet, "track-1", "mr1_current", "bytes=10-12")
	matchingIfRange.Header.Set("If-Range", `"music-etag"`)
	matching := httptest.NewRecorder()
	api.resourceContent(matching, matchingIfRange)
	if matching.Code != http.StatusPartialContent || matching.Body.String() != "abc" {
		t.Fatalf("matching If-Range=%d body=%q", matching.Code, matching.Body.String())
	}
	mismatchingIfRange := musicResourceRequest(http.MethodGet, "track-1", "mr1_current", "bytes=10-12")
	mismatchingIfRange.Header.Set("If-Range", `"older-etag"`)
	mismatching := httptest.NewRecorder()
	api.resourceContent(mismatching, mismatchingIfRange)
	if mismatching.Code != http.StatusOK || mismatching.Body.String() != string(content) {
		t.Fatalf("mismatching If-Range=%d body=%q", mismatching.Code, mismatching.Body.String())
	}
	if materializations.Load() != 1 {
		t.Fatalf("HEAD/Range/If-Range materializations=%d, want one", materializations.Load())
	}

	invalidCalls := stub.resolveCalls
	multiple := httptest.NewRecorder()
	api.resourceContent(multiple, musicResourceRequest(http.MethodGet, "track-1", "mr1_current", "bytes=0-1,4-5"))
	if multiple.Code != http.StatusRequestedRangeNotSatisfiable || stub.resolveCalls != invalidCalls {
		t.Fatalf("multiple range=%d resolveCalls=%d want=%d", multiple.Code, stub.resolveCalls, invalidCalls)
	}
	unsatisfiable := httptest.NewRecorder()
	api.resourceContent(unsatisfiable, musicResourceRequest(http.MethodGet, "track-1", "mr1_current", "bytes=999-1000"))
	if unsatisfiable.Code != http.StatusRequestedRangeNotSatisfiable {
		t.Fatalf("unsatisfiable range=%d body=%s", unsatisfiable.Code, unsatisfiable.Body.String())
	}

	for _, pair := range [][2]string{{"track-2", "mr1_current"}, {"track-1", "mr1_stale"}} {
		missing := httptest.NewRecorder()
		api.resourceContent(missing, musicResourceRequest(http.MethodGet, pair[0], pair[1], ""))
		if missing.Code != http.StatusNotFound || !strings.Contains(missing.Body.String(), "not_found") {
			t.Fatalf("cross/stale resource %#v=%d %s", pair, missing.Code, missing.Body.String())
		}
	}

	replacement := []byte(strings.Repeat("z", len(content)))
	if err := os.WriteFile(path, replacement, 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.Chtimes(path, info.ModTime(), info.ModTime()); err != nil {
		t.Fatal(err)
	}
	changedBytes := httptest.NewRecorder()
	api.resourceContent(changedBytes, musicResourceRequest(http.MethodGet, "track-1", "mr1_current", ""))
	if changedBytes.Code != http.StatusOK || changedBytes.Body.String() != string(content) {
		t.Fatalf("same-size source rewrite escaped immutable blob=%d %s", changedBytes.Code, changedBytes.Body.String())
	}
	if err := os.WriteFile(path, content, 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.Chtimes(path, info.ModTime(), info.ModTime()); err != nil {
		t.Fatal(err)
	}

	stub.resource.ByteLength = func() *int64 { value := length + 1; return &value }()
	changed := httptest.NewRecorder()
	api.resourceContent(changed, musicResourceRequest(http.MethodGet, "track-1", "mr1_current", ""))
	if changed.Code != http.StatusNotFound {
		t.Fatalf("changed bytes fingerprint=%d %s", changed.Code, changed.Body.String())
	}
}

func TestMusicResourceCASMaterializesOnceAcrossConcurrentRanges(t *testing.T) {
	directory := t.TempDir()
	path := filepath.Join(directory, "concurrent-track.bin")
	content := []byte("0123456789abcdefghijklmnopqrstuvwxyz")
	if err := os.WriteFile(path, content, 0o600); err != nil {
		t.Fatal(err)
	}
	path, err := filepath.EvalSymlinks(path)
	if err != nil {
		t.Fatal(err)
	}
	info, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	length := int64(len(content))
	digest := sha256.Sum256(content)
	stub := &musicReadRepositoryStub{resource: library.ListenLocalMusicResource{
		ID: "mr1_concurrent", FileID: "track-1", Revision: 1,
		Kind: library.ListenLocalMusicResourceOriginal, MediaType: "audio/mpeg", ByteLength: &length,
		ETag: `"concurrent-etag"`, Checksum: "sha256:" + hex.EncodeToString(digest[:]),
		Availability: "available", LocalPath: path, ModTimeUnixNano: info.ModTime().UnixNano(),
	}}
	cacheDirectory := filepath.Join(directory, "music-cas")
	api, err := NewMusicAPI(MusicConfig{
		CatalogID: "catalog-1", Reader: stub, ResourceCacheDirectory: cacheDirectory,
	})
	if err != nil {
		t.Fatal(err)
	}
	entered := make(chan struct{})
	release := make(chan struct{})
	var materializations atomic.Int32
	api.resourceCache.beforeCopy = func() {
		materializations.Add(1)
		close(entered)
		<-release
	}

	const requests = 10
	start := make(chan struct{})
	results := make(chan *httptest.ResponseRecorder, requests)
	var wait sync.WaitGroup
	for range requests {
		wait.Add(1)
		go func() {
			defer wait.Done()
			<-start
			response := httptest.NewRecorder()
			api.resourceContent(response, musicResourceRequest(http.MethodGet, "track-1", "mr1_concurrent", "bytes=0-0"))
			results <- response
		}()
	}
	close(start)
	select {
	case <-entered:
	case <-time.After(2 * time.Second):
		t.Fatal("materialization leader did not start")
	}
	close(release)
	wait.Wait()
	close(results)
	for response := range results {
		if response.Code != http.StatusPartialContent || response.Body.String() != "0" {
			t.Fatalf("concurrent Range=%d body=%q", response.Code, response.Body.String())
		}
	}
	if materializations.Load() != 1 {
		t.Fatalf("concurrent materializations=%d, want one", materializations.Load())
	}
	entries, err := os.ReadDir(cacheDirectory)
	if err != nil {
		t.Fatal(err)
	}
	blobs := 0
	for _, entry := range entries {
		if publicMusicResourceBlobPattern.MatchString(entry.Name()) {
			blobs++
			blobInfo, infoErr := entry.Info()
			if infoErr != nil {
				t.Fatalf("stat CAS blob: %v", infoErr)
			}
			assertProtectedPublicMusicResourceBlob(
				t,
				filepath.Join(cacheDirectory, entry.Name()),
				blobInfo,
			)
		}
	}
	if blobs != 1 {
		t.Fatalf("CAS blobs=%d, want one", blobs)
	}
}

func TestMusicResourceCASChecksumMismatchDoesNotPoisonCacheAndRevisionRotates(t *testing.T) {
	directory := t.TempDir()
	path := filepath.Join(directory, "track.bin")
	content := []byte("verified-content")
	if err := os.WriteFile(path, content, 0o600); err != nil {
		t.Fatal(err)
	}
	path, err := filepath.EvalSymlinks(path)
	if err != nil {
		t.Fatal(err)
	}
	info, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	length := int64(len(content))
	correctDigest := sha256.Sum256(content)
	stub := &musicReadRepositoryStub{resource: library.ListenLocalMusicResource{
		ID: "mr1_v1", FileID: "track-1", Revision: 1,
		Kind: library.ListenLocalMusicResourceOriginal, MediaType: "audio/mpeg", ByteLength: &length,
		ETag: `"v1"`, Checksum: "sha256:" + strings.Repeat("0", 64),
		Availability: "available", LocalPath: path, ModTimeUnixNano: info.ModTime().UnixNano(),
	}}
	cacheDirectory := filepath.Join(directory, "music-cas")
	api, err := NewMusicAPI(MusicConfig{
		CatalogID: "catalog-1", Reader: stub, ResourceCacheDirectory: cacheDirectory,
	})
	if err != nil {
		t.Fatal(err)
	}
	mismatch := httptest.NewRecorder()
	api.resourceContent(mismatch, musicResourceRequest(http.MethodGet, "track-1", "mr1_v1", ""))
	if mismatch.Code != http.StatusNotFound {
		t.Fatalf("checksum mismatch status=%d body=%s", mismatch.Code, mismatch.Body.String())
	}
	assertPublicMusicCacheHasNoArtifacts(t, cacheDirectory)

	stub.mu.Lock()
	stub.resource.Checksum = "sha256:" + hex.EncodeToString(correctDigest[:])
	stub.resource.ETag = `"v1-correct"`
	stub.mu.Unlock()
	valid := httptest.NewRecorder()
	api.resourceContent(valid, musicResourceRequest(http.MethodGet, "track-1", "mr1_v1", ""))
	if valid.Code != http.StatusOK || valid.Body.String() != string(content) {
		t.Fatalf("valid resource after mismatch=%d body=%q", valid.Code, valid.Body.String())
	}

	newContent := []byte("new-revision-data")
	if err := os.WriteFile(path, newContent, 0o600); err != nil {
		t.Fatal(err)
	}
	newInfo, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	newLength := int64(len(newContent))
	newDigest := sha256.Sum256(newContent)
	stub.mu.Lock()
	stub.resource.ID = "mr1_v2"
	stub.resource.Revision = 2
	stub.resource.ByteLength = &newLength
	stub.resource.Checksum = "sha256:" + hex.EncodeToString(newDigest[:])
	stub.resource.ETag = `"v2"`
	stub.resource.ModTimeUnixNano = newInfo.ModTime().UnixNano()
	stub.mu.Unlock()
	old := httptest.NewRecorder()
	api.resourceContent(old, musicResourceRequest(http.MethodGet, "track-1", "mr1_v1", ""))
	if old.Code != http.StatusNotFound {
		t.Fatalf("old resource ID after revision=%d body=%s", old.Code, old.Body.String())
	}
	current := httptest.NewRecorder()
	api.resourceContent(current, musicResourceRequest(http.MethodGet, "track-1", "mr1_v2", ""))
	if current.Code != http.StatusOK || current.Body.String() != string(newContent) || current.Header().Get("ETag") != `"v2"` {
		t.Fatalf("new resource revision=%d etag=%q body=%q", current.Code, current.Header().Get("ETag"), current.Body.String())
	}
}

func TestMusicResourceCASUsesPrivateBoundedDirectory(t *testing.T) {
	base := t.TempDir()
	cacheDirectory := filepath.Join(base, "music-cas")
	cache, err := newPublicMusicResourceMaterializer(cacheDirectory, 1, 1<<20)
	if err != nil {
		t.Fatal(err)
	}
	directoryInfo, err := os.Stat(cache.directory)
	if err != nil {
		t.Fatalf("stat cache directory: %v", err)
	}
	assertPrivatePublicMusicResourceDirectory(t, cache.directory, directoryInfo)
	for index, body := range [][]byte{[]byte("first"), []byte("second")} {
		path := filepath.Join(base, fmt.Sprintf("source-%d.bin", index))
		if err := os.WriteFile(path, body, 0o600); err != nil {
			t.Fatal(err)
		}
		path, err = filepath.EvalSymlinks(path)
		if err != nil {
			t.Fatal(err)
		}
		info, err := os.Stat(path)
		if err != nil {
			t.Fatal(err)
		}
		length := int64(len(body))
		digest := sha256.Sum256(body)
		file, _, err := cache.open(context.Background(), library.ListenLocalMusicResource{
			ID: fmt.Sprintf("mr1_%d", index), Revision: int64(index + 1), ByteLength: &length,
			Checksum: "sha256:" + hex.EncodeToString(digest[:]), LocalPath: path,
			ModTimeUnixNano: info.ModTime().UnixNano(),
		})
		if err != nil {
			t.Fatal(err)
		}
		_ = file.Close()
	}
	entries, err := os.ReadDir(cache.directory)
	if err != nil {
		t.Fatal(err)
	}
	blobs := 0
	for _, entry := range entries {
		if publicMusicResourceBlobPattern.MatchString(entry.Name()) {
			blobs++
		}
	}
	if blobs != 1 {
		t.Fatalf("bounded cache blobs=%d, want one", blobs)
	}
	oversized := cache.maxBytes + 1
	if _, _, err := cache.resourceKey(library.ListenLocalMusicResource{
		ID: "mr1_oversized", Revision: 1, ByteLength: &oversized,
		Checksum: "sha256:" + strings.Repeat("0", 64),
	}); !errors.Is(err, errPublicMusicResourceChanged) {
		t.Fatalf("oversized cache resource error=%v, want rejection", err)
	}

	target := filepath.Join(base, "real-cache")
	if err := os.Mkdir(target, 0o700); err != nil {
		t.Fatal(err)
	}
	symlink := filepath.Join(base, "symlink-cache")
	if err := os.Symlink(target, symlink); err == nil {
		if _, err := newPublicMusicResourceMaterializer(symlink, 1, 1024); !errors.Is(err, errPublicMusicResourceCache) {
			t.Fatalf("symlink cache error=%v, want cache rejection", err)
		}
	}
}

func TestMusicResourceCASBoundsDifferentKeyMaterializationsBeforeCreatingTemps(t *testing.T) {
	base := t.TempDir()
	bodyLength := int64(256 * 1024)
	cache, err := newPublicMusicResourceMaterializer(
		filepath.Join(base, "music-cas"),
		2,
		bodyLength,
	)
	if err != nil {
		t.Fatal(err)
	}
	resources := make([]library.ListenLocalMusicResource, 0, 2)
	for index := range 2 {
		body := bytes.Repeat([]byte{byte(index + 1)}, int(bodyLength))
		path := filepath.Join(base, fmt.Sprintf("parallel-%d.bin", index))
		if err := os.WriteFile(path, body, 0o600); err != nil {
			t.Fatal(err)
		}
		path, err = filepath.EvalSymlinks(path)
		if err != nil {
			t.Fatal(err)
		}
		info, err := os.Stat(path)
		if err != nil {
			t.Fatal(err)
		}
		digest := sha256.Sum256(body)
		length := bodyLength
		resources = append(resources, library.ListenLocalMusicResource{
			ID: fmt.Sprintf("mr1_parallel_%d", index), Revision: 1,
			ByteLength: &length, Checksum: "sha256:" + hex.EncodeToString(digest[:]),
			LocalPath: path, ModTimeUnixNano: info.ModTime().UnixNano(),
		})
	}

	firstEntered := make(chan struct{})
	secondEntered := make(chan struct{})
	releaseFirst := make(chan struct{})
	var materializations atomic.Int32
	cache.beforeCopy = func() {
		switch materializations.Add(1) {
		case 1:
			close(firstEntered)
			<-releaseFirst
		case 2:
			close(secondEntered)
		}
	}

	start := make(chan struct{})
	errorsByResource := make(chan error, len(resources))
	var wait sync.WaitGroup
	for _, resource := range resources {
		resource := resource
		wait.Add(1)
		go func() {
			defer wait.Done()
			<-start
			file, _, openErr := cache.open(context.Background(), resource)
			if file != nil {
				_ = file.Close()
			}
			errorsByResource <- openErr
		}()
	}
	close(start)
	select {
	case <-firstEntered:
	case <-time.After(2 * time.Second):
		t.Fatal("first different-key materialization did not start")
	}
	select {
	case <-secondEntered:
		t.Fatal("second different-key materialization exceeded the shared byte reservation")
	case <-time.After(150 * time.Millisecond):
	}
	close(releaseFirst)
	wait.Wait()
	close(errorsByResource)
	for openErr := range errorsByResource {
		if openErr != nil {
			t.Fatalf("materialize different key: %v", openErr)
		}
	}
	if materializations.Load() != 2 {
		t.Fatalf("materializations=%d, want two serialized leaders", materializations.Load())
	}
	entries, err := os.ReadDir(cache.directory)
	if err != nil {
		t.Fatal(err)
	}
	var blobs int
	var blobBytes int64
	for _, entry := range entries {
		if strings.HasPrefix(entry.Name(), ".materialize-") {
			t.Fatalf("completed materialization left temp %q", entry.Name())
		}
		if !publicMusicResourceBlobPattern.MatchString(entry.Name()) {
			continue
		}
		info, infoErr := entry.Info()
		if infoErr != nil {
			t.Fatal(infoErr)
		}
		blobs++
		blobBytes += info.Size()
	}
	if blobs != 1 || blobBytes > cache.maxBytes {
		t.Fatalf("bounded cache blobs=%d bytes=%d limit=%d", blobs, blobBytes, cache.maxBytes)
	}
}

func TestMusicResourceCASKeepsServingBlobInsideHardBudget(t *testing.T) {
	base := t.TempDir()
	bodyLength := int64(64 * 1024)
	cache, err := newPublicMusicResourceMaterializer(
		filepath.Join(base, "music-cas"),
		2,
		bodyLength,
	)
	if err != nil {
		t.Fatal(err)
	}
	makeResource := func(index byte) library.ListenLocalMusicResource {
		t.Helper()
		body := bytes.Repeat([]byte{index}, int(bodyLength))
		path := filepath.Join(base, fmt.Sprintf("serving-%d.bin", index))
		if err := os.WriteFile(path, body, 0o600); err != nil {
			t.Fatal(err)
		}
		path, err = filepath.EvalSymlinks(path)
		if err != nil {
			t.Fatal(err)
		}
		info, err := os.Stat(path)
		if err != nil {
			t.Fatal(err)
		}
		digest := sha256.Sum256(body)
		length := bodyLength
		return library.ListenLocalMusicResource{
			ID: fmt.Sprintf("mr1_serving_%d", index), Revision: 1,
			ByteLength: &length, Checksum: "sha256:" + hex.EncodeToString(digest[:]),
			LocalPath: path, ModTimeUnixNano: info.ModTime().UnixNano(),
		}
	}
	first := makeResource(1)
	second := makeResource(2)
	serving, _, err := cache.open(context.Background(), first)
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()
	if file, _, openErr := cache.open(ctx, second); !errors.Is(openErr, context.DeadlineExceeded) {
		if file != nil {
			_ = file.Close()
		}
		t.Fatalf("second resource while first is served error=%v, want deadline", openErr)
	}
	entries, err := os.ReadDir(cache.directory)
	if err != nil {
		t.Fatal(err)
	}
	for _, entry := range entries {
		if strings.HasPrefix(entry.Name(), ".materialize-") {
			t.Fatalf("blocked capacity left temp %q", entry.Name())
		}
	}
	if err := serving.Close(); err != nil {
		t.Fatal(err)
	}
	replacement, _, err := cache.open(context.Background(), second)
	if err != nil {
		t.Fatalf("resource after serving lease closed: %v", err)
	}
	_ = replacement.Close()
}

func TestMusicResourceCASProtectsInstalledBlobUntilLeaderOpensIt(t *testing.T) {
	base := t.TempDir()
	bodyLength := int64(64 * 1024)
	cache, err := newPublicMusicResourceMaterializer(
		filepath.Join(base, "music-cas"),
		2,
		2*bodyLength,
	)
	if err != nil {
		t.Fatal(err)
	}
	resources := make([]library.ListenLocalMusicResource, 0, 2)
	for index := range 2 {
		body := bytes.Repeat([]byte{byte(index + 7)}, int(bodyLength))
		path := filepath.Join(base, fmt.Sprintf("installed-%d.bin", index))
		if err := os.WriteFile(path, body, 0o600); err != nil {
			t.Fatal(err)
		}
		path, err = filepath.EvalSymlinks(path)
		if err != nil {
			t.Fatal(err)
		}
		info, err := os.Stat(path)
		if err != nil {
			t.Fatal(err)
		}
		digest := sha256.Sum256(body)
		length := bodyLength
		resources = append(resources, library.ListenLocalMusicResource{
			ID: fmt.Sprintf("mr1_installed_%d", index), Revision: 1,
			ByteLength: &length, Checksum: "sha256:" + hex.EncodeToString(digest[:]),
			LocalPath: path, ModTimeUnixNano: info.ModTime().UnixNano(),
		})
	}
	firstInstalled := make(chan struct{})
	secondInstalled := make(chan struct{})
	releaseFirst := make(chan struct{})
	var installs atomic.Int32
	cache.afterInstall = func() {
		switch installs.Add(1) {
		case 1:
			close(firstInstalled)
			<-releaseFirst
		case 2:
			close(secondInstalled)
		}
	}
	firstResult := make(chan error, 1)
	go func() {
		file, _, openErr := cache.open(context.Background(), resources[0])
		if file != nil {
			_ = file.Close()
		}
		firstResult <- openErr
	}()
	select {
	case <-firstInstalled:
	case <-time.After(2 * time.Second):
		t.Fatal("first resource did not reach installed window")
	}
	secondResult := make(chan error, 1)
	go func() {
		file, _, openErr := cache.open(context.Background(), resources[1])
		if file != nil {
			_ = file.Close()
		}
		secondResult <- openErr
	}()
	select {
	case <-secondInstalled:
	case <-time.After(2 * time.Second):
		t.Fatal("second resource could not materialize while first was protected")
	}
	if err := <-secondResult; err != nil {
		t.Fatalf("second resource: %v", err)
	}
	close(releaseFirst)
	if err := <-firstResult; err != nil {
		t.Fatalf("first installed resource was pruned before open: %v", err)
	}
}

func TestMusicResourceCASRejectsGrowingSourceBeforeWritingPastReservation(t *testing.T) {
	base := t.TempDir()
	body := bytes.Repeat([]byte("a"), 32*1024)
	path := filepath.Join(base, "growing.bin")
	if err := os.WriteFile(path, body, 0o600); err != nil {
		t.Fatal(err)
	}
	path, err := filepath.EvalSymlinks(path)
	if err != nil {
		t.Fatal(err)
	}
	info, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	digest := sha256.Sum256(body)
	length := int64(len(body))
	cache, err := newPublicMusicResourceMaterializer(filepath.Join(base, "music-cas"), 2, length)
	if err != nil {
		t.Fatal(err)
	}
	cache.beforeCopy = func() {
		file, openErr := os.OpenFile(path, os.O_APPEND|os.O_WRONLY, 0)
		if openErr != nil {
			t.Errorf("open growing source: %v", openErr)
			return
		}
		_, writeErr := file.Write(bytes.Repeat([]byte("b"), 64*1024))
		_ = file.Close()
		if writeErr != nil {
			t.Errorf("grow source: %v", writeErr)
		}
	}
	file, _, openErr := cache.open(context.Background(), library.ListenLocalMusicResource{
		ID: "mr1_growing", Revision: 1, ByteLength: &length,
		Checksum: "sha256:" + hex.EncodeToString(digest[:]), LocalPath: path,
		ModTimeUnixNano: info.ModTime().UnixNano(),
	})
	if file != nil {
		_ = file.Close()
	}
	if !errors.Is(openErr, errPublicMusicResourceChanged) {
		t.Fatalf("growing source error=%v, want changed", openErr)
	}
	assertPublicMusicCacheHasNoArtifacts(t, cache.directory)
}

func TestMusicResourceCASReconcilesOversizedCommittedCacheOnStartup(t *testing.T) {
	cacheDirectory := filepath.Join(t.TempDir(), "music-cas")
	if err := os.MkdirAll(cacheDirectory, 0o700); err != nil {
		t.Fatal(err)
	}
	for index := 1; index <= 3; index++ {
		name := fmt.Sprintf("%064x.blob", index)
		if err := os.WriteFile(filepath.Join(cacheDirectory, name), []byte("data"), 0o400); err != nil {
			t.Fatal(err)
		}
	}
	cache, err := newPublicMusicResourceMaterializer(cacheDirectory, 1, 4)
	if err != nil {
		t.Fatal(err)
	}
	entries, err := os.ReadDir(cache.directory)
	if err != nil {
		t.Fatal(err)
	}
	blobs := 0
	var bytesOnDisk int64
	for _, entry := range entries {
		if !publicMusicResourceBlobPattern.MatchString(entry.Name()) {
			continue
		}
		info, infoErr := entry.Info()
		if infoErr != nil {
			t.Fatal(infoErr)
		}
		blobs++
		bytesOnDisk += info.Size()
	}
	if blobs != 1 || bytesOnDisk > cache.maxBytes {
		t.Fatalf("startup reconciliation blobs=%d bytes=%d", blobs, bytesOnDisk)
	}
}

func TestMusicResourceCASRemovesFreshCrashTempsOnStartup(t *testing.T) {
	cacheDirectory := filepath.Join(t.TempDir(), "music-cas")
	if err := os.MkdirAll(cacheDirectory, 0o700); err != nil {
		t.Fatal(err)
	}
	orphan := filepath.Join(cacheDirectory, ".materialize-crash-orphan")
	if err := os.WriteFile(orphan, []byte("partial private audio"), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := newPublicMusicResourceMaterializer(cacheDirectory, 2, 1<<20); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Lstat(orphan); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("fresh crash temp still exists: %v", err)
	}
}

func assertPublicMusicCacheHasNoArtifacts(t *testing.T, directory string) {
	t.Helper()
	entries, err := os.ReadDir(directory)
	if err != nil {
		t.Fatal(err)
	}
	for _, entry := range entries {
		if publicMusicResourceBlobPattern.MatchString(entry.Name()) || strings.HasPrefix(entry.Name(), ".materialize-") {
			t.Fatalf("failed materialization left cache artifact %q", entry.Name())
		}
	}
}

func testMusicTrackProjection(now time.Time) library.ListenLocalMusicTrackProjection {
	duration, byteLength := int64(180_000), int64(1234)
	playback := library.ListenLocalMusicResource{
		ID: "mr1_playback", FileID: "private-file", Revision: 3,
		Kind: library.ListenLocalMusicResourceOriginal, MediaType: "audio/mpeg", Container: "mp3", Codec: "mp3",
		ByteLength: &byteLength, ETag: `"etag"`, Checksum: "sha256:" + strings.Repeat("a", 64),
		Availability: "available", LocalPath: "/Users/arnold/Private/Music/song.mp3",
	}
	artwork := library.ListenLocalMusicResource{
		ID: "mr1_artwork", FileID: "private-cover", Revision: 2,
		Kind: library.ListenLocalMusicResourceArtwork, MediaType: "image/jpeg", Availability: "available",
		LocalPath: "/Users/arnold/Private/Music/cover.jpg",
	}
	return library.ListenLocalMusicTrackProjection{
		Track: library.ListenLocalTrack{
			FileID: "track-1", LibraryID: "private-library", Revision: 8, ContentIdentityRevision: 2,
			MetadataRevision: 4, ResourceRevision: 3, LocalPath: playback.LocalPath, CoverLocalPath: artwork.LocalPath,
			Title: "Song", Author: "Artist", Album: "Album", AlbumArtist: "Album Artist", Genre: "Rock",
			TrackNumber: 1, DiscNumber: 1, Year: 2026, Format: "mp3", AudioCodec: "mp3", DurationMs: &duration,
			Availability: library.ListenLocalTrackAvailable, CreatedAt: now, UpdatedAt: now,
		},
		CatalogItemID: "catalog-item-1", PlaybackResources: []library.ListenLocalMusicResource{playback},
		ArtworkResource: &artwork,
	}
}

func testUnsupportedMusicTrackProjection(now time.Time) library.ListenLocalMusicTrackProjection {
	projection := testMusicTrackProjection(now)
	projection.Track.Format = "ogg"
	projection.Track.AudioCodec = "opus"
	projection.PlaybackResources[0].MediaType = "audio/ogg"
	projection.PlaybackResources[0].Container = "ogg"
	projection.PlaybackResources[0].Codec = "opus"
	return projection
}

func musicResourceRequest(method, trackID, resourceID, byteRange string) *http.Request {
	request := httptest.NewRequest(method,
		"/api/v1/music/tracks/"+trackID+"/resources/"+resourceID+"/content", nil)
	request.SetPathValue("id", trackID)
	request.SetPathValue("resourceId", resourceID)
	if byteRange != "" {
		request.Header.Set("Range", byteRange)
	}
	return request
}

func assertMusicJSONHasNoPrivateLocation(t *testing.T, payload []byte) {
	t.Helper()
	lower := strings.ToLower(string(payload))
	for _, forbidden := range []string{
		"localpath", "coverlocalpath", "fileid", "contenturl", "file://", "/users/arnold/", "private/music",
	} {
		if strings.Contains(lower, forbidden) {
			t.Fatalf("Music JSON leaked %q: %s", forbidden, payload)
		}
	}
}

func musicTestRequestHash(t *testing.T, canonical any) string {
	t.Helper()
	encoded, err := encodePublicMusicCanonicalJSON(canonical)
	if err != nil {
		t.Fatal(err)
	}
	digest := sha256.Sum256(encoded)
	return "sha256:" + hex.EncodeToString(digest[:])
}

func mustMusicTestJSON(t *testing.T, value any) string {
	t.Helper()
	encoded, err := json.Marshal(value)
	if err != nil {
		t.Fatal(err)
	}
	return string(encoded)
}

func musicTestContains(values []string, wanted string) bool {
	for _, value := range values {
		if value == wanted {
			return true
		}
	}
	return false
}

var _ library.ListenLocalMusicReadRepository = (*musicReadRepositoryStub)(nil)
var _ library.ListenLocalMusicWriteRepository = (*musicWriteRepositoryStub)(nil)
var _ library.ListenLocalMusicCompatibleRepresentationCoordinator = (*musicCompatibleRepresentationStub)(nil)
