package libraryapi

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"

	"xiadown/internal/application/library/access"
	applicationrss "xiadown/internal/application/rss"
	"xiadown/internal/domain/library"
	domainrss "xiadown/internal/domain/rss"
)

const testRSSEpoch = "0123456789abcdef0123456789abcdef"

type rssServiceStub struct {
	overviewResult      domainrss.SyncOverview
	overviewErr         error
	snapshotResult      applicationrss.SyncSnapshotResult
	snapshotErr         error
	subscriptionsResult []domainrss.Subscription
	subscriptionsErr    error
	entriesResult       domainrss.EntryPage
	entriesErr          error
	entryResult         domainrss.Entry
	entryErr            error
	changesResult       domainrss.ChangePage
	changesErr          error
	stateResult         domainrss.EntryState
	stateErr            error

	entriesRequest    applicationrss.ListEntriesRequest
	entryRequest      applicationrss.SubscriptionRequest
	changesRequest    applicationrss.ListChangesRequest
	stateRequest      applicationrss.SetEntryReadRequest
	stateV2Request    applicationrss.SetEntryStateRequest
	stateDeviceID     string
	overviewCatalogID string
	snapshotRequest   applicationrss.SyncSnapshotRequest
	snapshotCalls     int

	subscriptionsCalls int
	entriesCalls       int
	entryCalls         int
	changesCalls       int
	stateCalls         int
}

func (stub *rssServiceStub) GetSyncOverview(_ context.Context, catalogID string) (domainrss.SyncOverview, error) {
	stub.overviewCatalogID = catalogID
	return stub.overviewResult, stub.overviewErr
}

func (stub *rssServiceStub) GetSyncSnapshot(_ context.Context, request applicationrss.SyncSnapshotRequest) (applicationrss.SyncSnapshotResult, error) {
	stub.snapshotCalls++
	stub.snapshotRequest = request
	return stub.snapshotResult, stub.snapshotErr
}

func (stub *rssServiceStub) ListSyncSubscriptions(context.Context) ([]domainrss.SyncSubscription, error) {
	stub.subscriptionsCalls++
	items := make([]domainrss.SyncSubscription, 0, len(stub.subscriptionsResult))
	for _, item := range stub.subscriptionsResult {
		items = append(items, domainrss.SyncSubscription{ID: item.ID, Title: item.Title, Revision: item.Revision})
	}
	return items, stub.subscriptionsErr
}

func (stub *rssServiceStub) ListSyncEntries(_ context.Context, request applicationrss.ListEntriesRequest) (applicationrss.SyncEntryPage, error) {
	stub.entriesCalls++
	stub.entriesRequest = request
	items := make([]domainrss.SyncEntry, 0, len(stub.entriesResult.Items))
	for _, item := range stub.entriesResult.Items {
		items = append(items, domainrss.SyncEntry{ID: item.ID, Title: item.Title, ContentRevision: item.Revision})
	}
	return applicationrss.SyncEntryPage{Items: items, Total: stub.entriesResult.Total, NextOffset: stub.entriesResult.NextOffset}, stub.entriesErr
}

func (stub *rssServiceStub) GetSyncEntry(_ context.Context, request applicationrss.SubscriptionRequest) (applicationrss.SyncEntryDetail, error) {
	stub.entryCalls++
	stub.entryRequest = request
	return applicationrss.SyncEntryDetail{SyncEntry: domainrss.SyncEntry{ID: stub.entryResult.ID, Title: stub.entryResult.Title}}, stub.entryErr
}

func (stub *rssServiceStub) ListSyncChanges(_ context.Context, request applicationrss.ListChangesRequest) (domainrss.ChangePage, error) {
	stub.changesCalls++
	stub.changesRequest = request
	return stub.changesResult, stub.changesErr
}

func (stub *rssServiceStub) SetEntryStateForDevice(
	_ context.Context,
	deviceID string,
	request applicationrss.SetEntryStateRequest,
) (domainrss.EntryState, error) {
	stub.stateCalls++
	stub.stateDeviceID = deviceID
	stub.stateV2Request = request
	stub.stateRequest = applicationrss.SetEntryReadRequest{
		ID: request.ID, ExpectedRevision: request.ExpectedRevision, MutationID: request.MutationID,
	}
	if request.Read != nil {
		stub.stateRequest.Read = *request.Read
	}
	return stub.stateResult, stub.stateErr
}

func TestNewRSSAPIRequiresServiceAndRoutesUseLeastPrivilegeScopes(t *testing.T) {
	if _, err := NewRSSAPI(nil); err == nil {
		t.Fatal("nil RSS application service was accepted")
	}
	api, err := NewRSSAPI(&rssServiceStub{})
	if err != nil {
		t.Fatal(err)
	}
	want := map[string]library.DeviceScope{
		"GET /api/v1/rss/overview":                         library.DeviceScopeRSSRead,
		"GET /api/v1/rss/snapshot":                         library.DeviceScopeRSSRead,
		"GET /api/v1/rss/subscriptions":                    library.DeviceScopeRSSRead,
		"GET /api/v1/rss/subscriptions/{id}/icon":          library.DeviceScopeRSSRead,
		"GET /api/v1/rss/entries":                          library.DeviceScopeRSSRead,
		"GET /api/v1/rss/entries/{id}":                     library.DeviceScopeRSSRead,
		"GET /api/v1/rss/entries/{id}/resources/{slot}":    library.DeviceScopeRSSRead,
		"GET /api/v1/rss/changes":                          library.DeviceScopeRSSRead,
		"PATCH /api/v1/rss/entries/{id}/state":             library.DeviceScopeRSSState,
		"POST /api/v1/rss/subscriptions/{id}/mutations":    library.DeviceScopeRSSManage,
		"POST /api/v1/rss/subscriptions/{id}/fetch-lease":  library.DeviceScopeRSSFetch,
		"POST /api/v1/rss/subscriptions/{id}/observations": library.DeviceScopeRSSFetch,
	}
	for _, route := range api.Routes() {
		key := route.Method + " " + route.Path
		if route.Scope != want[key] {
			t.Fatalf("route %s scope = %q, want %q", key, route.Scope, want[key])
		}
		delete(want, key)
	}
	if len(want) != 0 {
		t.Fatalf("missing RSS routes: %#v", want)
	}
}

func TestRSSOverviewAndSnapshotExposeExplicitBootstrapContract(t *testing.T) {
	stub := &rssServiceStub{
		overviewResult: domainrss.SyncOverview{
			CatalogID: "catalog-1", WorkspaceID: domainrss.DefaultWorkspaceID,
			SubjectID: domainrss.DefaultSubjectID, Epoch: testRSSEpoch, HighWater: 42,
			RetainedFrom: 7, Capabilities: []string{"snapshot-keyset-v1", "entry-state-v2"},
		},
		snapshotResult: applicationrss.SyncSnapshotResult{
			Records: []domainrss.SyncSnapshotRecord{{
				EntityType: "entry", EntityID: "entry-1", Revision: 3,
				Payload: json.RawMessage(`{"id":"entry-1","title":"Post"}`),
			}},
			Epoch: testRSSEpoch, HighWater: 42, RetainedFrom: 7, NextCursor: "next_cursor", HasMore: true,
		},
	}
	api, _ := NewRSSAPI(stub)
	overviewRequest := httptest.NewRequest(http.MethodGet, "/api/v1/rss/overview", nil)
	overviewRequest = overviewRequest.WithContext(context.WithValue(
		overviewRequest.Context(), principalContextKey{}, access.Principal{CatalogID: "catalog-1", DeviceID: "iphone-1"},
	))
	overview := httptest.NewRecorder()
	api.getOverview(overview, overviewRequest)
	if overview.Code != http.StatusOK || stub.overviewCatalogID != "catalog-1" ||
		!strings.Contains(overview.Body.String(), `"workspaceId":"rss-default"`) ||
		!strings.Contains(overview.Body.String(), `"highWater":42`) ||
		!strings.Contains(overview.Body.String(), `"retainedFrom":7`) {
		t.Fatalf("overview status=%d catalog=%q body=%s", overview.Code, stub.overviewCatalogID, overview.Body.String())
	}

	snapshot := httptest.NewRecorder()
	api.getSnapshot(snapshot, httptest.NewRequest(http.MethodGet,
		"/api/v1/rss/snapshot?epoch="+testRSSEpoch+"&highWater=42&cursor=opaque_cursor&limit=25", nil))
	if snapshot.Code != http.StatusOK || stub.snapshotCalls != 1 || stub.snapshotRequest.Epoch != testRSSEpoch ||
		stub.snapshotRequest.HighWater != 42 || stub.snapshotRequest.Cursor != "opaque_cursor" || stub.snapshotRequest.Limit != 25 ||
		!strings.Contains(snapshot.Body.String(), `"nextCursor":"next_cursor"`) {
		t.Fatalf("snapshot status=%d calls=%d request=%#v body=%s", snapshot.Code, stub.snapshotCalls, stub.snapshotRequest, snapshot.Body.String())
	}

	invalid := httptest.NewRecorder()
	api.getSnapshot(invalid, httptest.NewRequest(http.MethodGet, "/api/v1/rss/snapshot?highWater=42", nil))
	if invalid.Code != http.StatusBadRequest || stub.snapshotCalls != 1 {
		t.Fatalf("invalid snapshot status=%d calls=%d body=%s", invalid.Code, stub.snapshotCalls, invalid.Body.String())
	}
}

func TestRSSSnapshotAndChangesReturnActionableResetPosition(t *testing.T) {
	reset := &domainrss.SyncResetError{Position: domainrss.SyncPosition{Epoch: testRSSEpoch, Cursor: 20, RetainedFrom: 5}}
	stub := &rssServiceStub{snapshotErr: reset, changesErr: reset}
	api, _ := NewRSSAPI(stub)

	snapshot := httptest.NewRecorder()
	api.getSnapshot(snapshot, httptest.NewRequest(http.MethodGet,
		"/api/v1/rss/snapshot?epoch="+testRSSEpoch+"&highWater=2", nil))
	if snapshot.Code != http.StatusConflict || !strings.Contains(snapshot.Body.String(), `"error":"reset_required"`) ||
		!strings.Contains(snapshot.Body.String(), `"cursor":20`) || !strings.Contains(snapshot.Body.String(), `"retainedFrom":5`) {
		t.Fatalf("snapshot reset status=%d body=%s", snapshot.Code, snapshot.Body.String())
	}

	changes := httptest.NewRecorder()
	api.listChanges(changes, httptest.NewRequest(http.MethodGet,
		"/api/v1/rss/changes?epoch="+testRSSEpoch+"&after=21", nil))
	if changes.Code != http.StatusConflict || !strings.Contains(changes.Body.String(), `"epoch":"`+testRSSEpoch+`"`) {
		t.Fatalf("changes reset status=%d body=%s", changes.Code, changes.Body.String())
	}
}

func TestRSSReadHandlersDelegateBoundedSyncQueries(t *testing.T) {
	now := time.Date(2026, 7, 13, 10, 0, 0, 0, time.UTC)
	stub := &rssServiceStub{
		subscriptionsResult: []domainrss.Subscription{{ID: "sub-1", Title: "Feed", ETag: "private-etag", LastModified: "private-date"}},
		entriesResult:       domainrss.EntryPage{Items: []domainrss.Entry{{ID: "entry-1", Title: "Post", ContentHash: "private-hash"}}, Total: 1},
		entryResult:         domainrss.Entry{ID: "entry-1", Title: "Post", ContentHash: "private-hash"},
		changesResult:       domainrss.ChangePage{Epoch: testRSSEpoch, Cursor: 9, HighWater: 12, Changes: []domainrss.Change{{Sequence: 9, EntityType: "entry", EntityID: "entry-1", ChangedAt: now}}},
	}
	api, err := NewRSSAPI(stub)
	if err != nil {
		t.Fatal(err)
	}

	subscriptions := httptest.NewRecorder()
	api.listSubscriptions(subscriptions, httptest.NewRequest(http.MethodGet, "/api/v1/rss/subscriptions", nil))
	if subscriptions.Code != http.StatusOK || !strings.Contains(subscriptions.Body.String(), `"id":"sub-1"`) {
		t.Fatalf("subscriptions status=%d body=%s", subscriptions.Code, subscriptions.Body.String())
	}
	if subscriptions.Header().Get("Deprecation") != "true" ||
		!strings.Contains(subscriptions.Header().Get("Link"), "/api/v1/rss/snapshot") {
		t.Fatalf("legacy subscription headers = %#v", subscriptions.Header())
	}
	if strings.Contains(subscriptions.Body.String(), "private-etag") || strings.Contains(subscriptions.Body.String(), "private-date") {
		t.Fatalf("conditional feed metadata escaped public JSON: %s", subscriptions.Body.String())
	}

	entries := httptest.NewRecorder()
	api.listEntries(entries, httptest.NewRequest(
		http.MethodGet,
		"/api/v1/rss/entries?subscriptionId=sub-1&kind=video&q=launch&unreadOnly=true&is_starred=1&limit=40&offset=2",
		nil,
	))
	if entries.Code != http.StatusOK {
		t.Fatalf("entries status=%d body=%s", entries.Code, entries.Body.String())
	}
	if stub.entriesRequest.SubscriptionID != "sub-1" || stub.entriesRequest.Kind != "video" ||
		stub.entriesRequest.Query != "launch" || !stub.entriesRequest.UnreadOnly || !stub.entriesRequest.StarredOnly ||
		stub.entriesRequest.Limit != 40 || stub.entriesRequest.Offset != 2 {
		t.Fatalf("entries request = %#v", stub.entriesRequest)
	}
	if strings.Contains(entries.Body.String(), "private-hash") {
		t.Fatalf("entry content hash escaped public JSON: %s", entries.Body.String())
	}

	entryRequest := httptest.NewRequest(http.MethodGet, "/api/v1/rss/entries/entry-1", nil)
	entryRequest.SetPathValue("id", "entry-1")
	entry := httptest.NewRecorder()
	api.getEntry(entry, entryRequest)
	if entry.Code != http.StatusOK || stub.entryRequest.ID != "entry-1" {
		t.Fatalf("entry status=%d request=%#v body=%s", entry.Code, stub.entryRequest, entry.Body.String())
	}

	changes := httptest.NewRecorder()
	api.listChanges(changes, httptest.NewRequest(http.MethodGet, "/api/v1/rss/changes?epoch="+testRSSEpoch+"&after=8&limit=25", nil))
	if changes.Code != http.StatusOK || stub.changesRequest.After != 8 || stub.changesRequest.Limit != 25 ||
		stub.changesRequest.Epoch != testRSSEpoch || !strings.Contains(changes.Body.String(), `"epoch":"`+testRSSEpoch+`"`) {
		t.Fatalf("changes status=%d request=%#v body=%s", changes.Code, stub.changesRequest, changes.Body.String())
	}
}

func TestRSSReadQueriesRejectInvalidValuesBeforeApplicationService(t *testing.T) {
	tests := []string{
		"/api/v1/rss/entries?limit=501",
		"/api/v1/rss/entries?offset=-1",
		"/api/v1/rss/entries?unreadOnly=1",
		"/api/v1/rss/entries?is_starred=true",
		"/api/v1/rss/entries?is_starred=0",
		"/api/v1/rss/entries?is_starred=2",
		"/api/v1/rss/entries?kind=audio",
		"/api/v1/rss/entries?subscriptionId=bad%20id",
		"/api/v1/rss/entries?q=" + strings.Repeat("a", maxPublicSearchQueryLength+1),
		"/api/v1/rss/entries?q=%FF",
	}
	for _, target := range tests {
		t.Run(target, func(t *testing.T) {
			stub := &rssServiceStub{}
			api, _ := NewRSSAPI(stub)
			recorder := httptest.NewRecorder()
			api.listEntries(recorder, httptest.NewRequest(http.MethodGet, target, nil))
			if recorder.Code != http.StatusBadRequest || stub.entriesCalls != 0 {
				t.Fatalf("status=%d calls=%d body=%s", recorder.Code, stub.entriesCalls, recorder.Body.String())
			}
		})
	}

	for _, target := range []string{
		"/api/v1/rss/changes?after=-1",
		"/api/v1/rss/changes?after=nope",
		"/api/v1/rss/changes?limit=501",
	} {
		stub := &rssServiceStub{}
		api, _ := NewRSSAPI(stub)
		recorder := httptest.NewRecorder()
		api.listChanges(recorder, httptest.NewRequest(http.MethodGet, target, nil))
		if recorder.Code != http.StatusBadRequest || stub.changesCalls != 0 {
			t.Fatalf("target=%s status=%d calls=%d body=%s", target, recorder.Code, stub.changesCalls, recorder.Body.String())
		}
	}
}

func TestRSSSearchQueryMatchesOpenAPIUnicodeLengthContract(t *testing.T) {
	stub := &rssServiceStub{}
	api, _ := NewRSSAPI(stub)
	validQuery := strings.Repeat("界", maxPublicSearchQueryLength)

	validRecorder := httptest.NewRecorder()
	api.listEntries(validRecorder, httptest.NewRequest(
		http.MethodGet,
		"/api/v1/rss/entries?q="+url.QueryEscape(validQuery),
		nil,
	))
	if validRecorder.Code != http.StatusOK || stub.entriesCalls != 1 || stub.entriesRequest.Query != validQuery {
		t.Fatalf("valid Unicode query status=%d calls=%d query=%q body=%s", validRecorder.Code, stub.entriesCalls, stub.entriesRequest.Query, validRecorder.Body.String())
	}

	invalidRecorder := httptest.NewRecorder()
	api.listEntries(invalidRecorder, httptest.NewRequest(
		http.MethodGet,
		"/api/v1/rss/entries?q="+url.QueryEscape(validQuery+"界"),
		nil,
	))
	if invalidRecorder.Code != http.StatusBadRequest || stub.entriesCalls != 1 || !strings.Contains(invalidRecorder.Body.String(), `"error":"invalid_request"`) {
		t.Fatalf("oversized Unicode query status=%d calls=%d body=%s", invalidRecorder.Code, stub.entriesCalls, invalidRecorder.Body.String())
	}
}

func TestRSSStateUsesAuthenticatedDeviceAndStrictOptimisticMutation(t *testing.T) {
	readAt := time.Date(2026, 7, 13, 11, 0, 0, 0, time.UTC)
	stub := &rssServiceStub{stateResult: domainrss.EntryState{
		EntryID: "entry-1", Read: true, ReadAt: &readAt, Revision: 4,
		UpdatedAt: readAt, UpdatedBy: "iphone-1", MutationID: "mutation-1",
	}}
	api, _ := NewRSSAPI(stub)
	request := httptest.NewRequest(http.MethodPatch, "/api/v1/rss/entries/entry-1/state", strings.NewReader(
		`{"field":"read","value":true,"expectedRevision":3,"mutationId":"mutation-1"}`,
	))
	request.SetPathValue("id", "entry-1")
	request = request.WithContext(context.WithValue(request.Context(), principalContextKey{}, access.Principal{DeviceID: "iphone-1"}))
	recorder := httptest.NewRecorder()
	api.setEntryState(recorder, request)
	if recorder.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", recorder.Code, recorder.Body.String())
	}
	if stub.stateDeviceID != "iphone-1" || stub.stateRequest.ID != "entry-1" || !stub.stateRequest.Read ||
		stub.stateRequest.ExpectedRevision == nil || *stub.stateRequest.ExpectedRevision != 3 ||
		stub.stateRequest.MutationID != "mutation-1" {
		t.Fatalf("state identity/mutation was not forced: device=%q request=%#v", stub.stateDeviceID, stub.stateRequest)
	}
}

func TestRSSStateV2DecodesStarArticleAndVideoFields(t *testing.T) {
	tests := []struct {
		name  string
		body  string
		check func(t *testing.T, request applicationrss.SetEntryStateRequest)
	}{
		{
			name: "starred", body: `{"field":"starred","value":true,"expectedRevision":2,"mutationId":"star-1"}`,
			check: func(t *testing.T, request applicationrss.SetEntryStateRequest) {
				if request.Field != domainrss.EntryStateFieldStarred || request.Starred == nil || !*request.Starred {
					t.Fatalf("star request = %#v", request)
				}
			},
		},
		{
			name: "article", body: `{"field":"articleProgress","value":{"fraction":0.6,"anchor":"section-3","contentRevision":8},"expectedRevision":1,"mutationId":"article-1"}`,
			check: func(t *testing.T, request applicationrss.SetEntryStateRequest) {
				if request.Field != domainrss.EntryStateFieldArticleProgress || request.ArticleProgress == nil ||
					request.ArticleProgress.Fraction != 0.6 || request.ArticleProgress.Anchor != "section-3" ||
					request.ArticleProgress.ContentRevision != 8 {
					t.Fatalf("article request = %#v", request)
				}
			},
		},
		{
			name: "video", body: `{"field":"videoProgressSeconds","value":12.5,"videoDurationSeconds":30,"expectedRevision":4,"mutationId":"video-1"}`,
			check: func(t *testing.T, request applicationrss.SetEntryStateRequest) {
				if request.Field != domainrss.EntryStateFieldVideoProgressSeconds || request.VideoProgressSeconds == nil ||
					*request.VideoProgressSeconds != 12.5 || request.VideoDurationSeconds == nil || *request.VideoDurationSeconds != 30 {
					t.Fatalf("video request = %#v", request)
				}
			},
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			revision := int64(9)
			stub := &rssServiceStub{stateResult: domainrss.EntryState{
				EntryID: "entry-1", SubjectID: domainrss.DefaultSubjectID, Revision: revision,
				UpdatedAt: time.Date(2026, 7, 13, 12, 0, 0, 0, time.UTC),
			}}
			api, _ := NewRSSAPI(stub)
			request := httptest.NewRequest(http.MethodPatch, "/api/v1/rss/entries/entry-1/state", strings.NewReader(test.body))
			request.SetPathValue("id", "entry-1")
			request = request.WithContext(context.WithValue(request.Context(), principalContextKey{}, access.Principal{DeviceID: "iphone-1"}))
			recorder := httptest.NewRecorder()
			api.setEntryState(recorder, request)
			if recorder.Code != http.StatusOK || stub.stateCalls != 1 || stub.stateDeviceID != "iphone-1" {
				t.Fatalf("status=%d calls=%d body=%s", recorder.Code, stub.stateCalls, recorder.Body.String())
			}
			test.check(t, stub.stateV2Request)
		})
	}
}

func TestRSSStateV2RejectsAmbiguousValuesAndReturnsConflictContracts(t *testing.T) {
	invalidBodies := []string{
		`{"field":"starred","value":true,"read":true,"expectedRevision":0,"mutationId":"m-1"}`,
		`{"field":"articleProgress","value":{"fraction":0.5,"contentRevision":1,"script":"x"},"expectedRevision":0,"mutationId":"m-1"}`,
		`{"field":"videoProgressSeconds","value":null,"expectedRevision":0,"mutationId":"m-1"}`,
		`{"field":"read","value":true,"videoDurationSeconds":3,"expectedRevision":0,"mutationId":"m-1"}`,
	}
	for _, body := range invalidBodies {
		stub := &rssServiceStub{}
		api, _ := NewRSSAPI(stub)
		request := httptest.NewRequest(http.MethodPatch, "/api/v1/rss/entries/entry-1/state", strings.NewReader(body))
		request.SetPathValue("id", "entry-1")
		request = request.WithContext(context.WithValue(request.Context(), principalContextKey{}, access.Principal{DeviceID: "iphone-1"}))
		recorder := httptest.NewRecorder()
		api.setEntryState(recorder, request)
		if recorder.Code != http.StatusBadRequest || stub.stateCalls != 0 {
			t.Fatalf("body=%s status=%d calls=%d response=%s", body, recorder.Code, stub.stateCalls, recorder.Body.String())
		}
	}

	conflictState := domainrss.EntryState{
		EntryID: "entry-1", SubjectID: domainrss.DefaultSubjectID, Starred: true,
		FieldRevisions: domainrss.StateFieldRevisions{Starred: 3}, Revision: 4,
		UpdatedAt: time.Date(2026, 7, 13, 12, 0, 0, 0, time.UTC),
	}
	for name, stateErr := range map[string]error{
		"field conflict": &domainrss.StateConflictError{State: conflictState},
		"idempotency":    domainrss.ErrIdempotencyConflict,
	} {
		t.Run(name, func(t *testing.T) {
			stub := &rssServiceStub{stateErr: stateErr}
			api, _ := NewRSSAPI(stub)
			request := httptest.NewRequest(http.MethodPatch, "/api/v1/rss/entries/entry-1/state", strings.NewReader(
				`{"field":"starred","value":true,"expectedRevision":2,"mutationId":"star-1"}`,
			))
			request.SetPathValue("id", "entry-1")
			request = request.WithContext(context.WithValue(request.Context(), principalContextKey{}, access.Principal{DeviceID: "iphone-1"}))
			recorder := httptest.NewRecorder()
			api.setEntryState(recorder, request)
			if recorder.Code != http.StatusConflict {
				t.Fatalf("status=%d body=%s", recorder.Code, recorder.Body.String())
			}
			if name == "field conflict" && (!strings.Contains(recorder.Body.String(), `"state"`) ||
				!strings.Contains(recorder.Body.String(), `"starred":3`)) {
				t.Fatalf("conflict body=%s", recorder.Body.String())
			}
			if name == "idempotency" && !strings.Contains(recorder.Body.String(), `"error":"idempotency_conflict"`) {
				t.Fatalf("idempotency body=%s", recorder.Body.String())
			}
		})
	}
}

func TestRSSStateRejectsIdentitySpoofingAndIncompleteMutations(t *testing.T) {
	tests := map[string]string{
		"caller user ID":    `{"field":"read","value":true,"expectedRevision":0,"mutationId":"m-1","userId":"rss-owner"}`,
		"caller device ID":  `{"field":"read","value":true,"expectedRevision":0,"mutationId":"m-1","deviceId":"other"}`,
		"caller field":      `{"field":"read","value":true,"expectedRevision":0,"mutationId":"m-1","caller":"other"}`,
		"missing read":      `{"expectedRevision":0,"mutationId":"m-1"}`,
		"missing revision":  `{"field":"read","value":true,"mutationId":"m-1"}`,
		"negative revision": `{"field":"read","value":true,"expectedRevision":-1,"mutationId":"m-1"}`,
		"missing mutation":  `{"field":"read","value":true,"expectedRevision":0}`,
		"multiple values":   `{"field":"read","value":true,"expectedRevision":0,"mutationId":"m-1"}{}`,
	}
	for name, body := range tests {
		t.Run(name, func(t *testing.T) {
			stub := &rssServiceStub{}
			api, _ := NewRSSAPI(stub)
			request := httptest.NewRequest(http.MethodPatch, "/api/v1/rss/entries/entry-1/state", strings.NewReader(body))
			request.SetPathValue("id", "entry-1")
			request = request.WithContext(context.WithValue(request.Context(), principalContextKey{}, access.Principal{DeviceID: "iphone-1"}))
			recorder := httptest.NewRecorder()
			api.setEntryState(recorder, request)
			if recorder.Code != http.StatusBadRequest || stub.stateCalls != 0 {
				t.Fatalf("status=%d calls=%d body=%s", recorder.Code, stub.stateCalls, recorder.Body.String())
			}
		})
	}

	stub := &rssServiceStub{}
	api, _ := NewRSSAPI(stub)
	request := httptest.NewRequest(http.MethodPatch, "/api/v1/rss/entries/entry-1/state", strings.NewReader(
		`{"field":"read","value":true,"expectedRevision":0,"mutationId":"m-1"}`,
	))
	request.SetPathValue("id", "entry-1")
	recorder := httptest.NewRecorder()
	api.setEntryState(recorder, request)
	if recorder.Code != http.StatusUnauthorized || recorder.Header().Get("WWW-Authenticate") == "" || stub.stateCalls != 0 {
		t.Fatalf("missing principal status=%d calls=%d headers=%#v", recorder.Code, stub.stateCalls, recorder.Header())
	}
}

func TestRSSRouterEnforcesReadAndStateScopesIndependently(t *testing.T) {
	stub := &rssServiceStub{stateResult: domainrss.EntryState{EntryID: "entry-1", Revision: 1}}
	api, _ := NewRSSAPI(stub)
	authenticator := &fakeAuthenticator{principal: access.Principal{
		CatalogID: "catalog-1", DeviceID: "iphone-1", Scopes: []library.DeviceScope{library.DeviceScopeRSSRead},
	}}
	router, err := NewRouter(Config{
		Version: "test", CatalogID: "catalog-1", Authenticator: authenticator, Pairer: &fakePairer{}, Routes: api.Routes(),
	})
	if err != nil {
		t.Fatal(err)
	}
	readResponse := performRequest(router, http.MethodGet, "/api/v1/rss/subscriptions", "Bearer token", "")
	if readResponse.Code != http.StatusOK {
		t.Fatalf("rss.read status=%d body=%s", readResponse.Code, readResponse.Body.String())
	}
	stateBody := `{"field":"read","value":true,"expectedRevision":0,"mutationId":"m-1"}`
	forbidden := performRequest(router, http.MethodPatch, "/api/v1/rss/entries/entry-1/state", "Bearer token", stateBody)
	if forbidden.Code != http.StatusForbidden || stub.stateCalls != 0 {
		t.Fatalf("rss.read changed state: status=%d calls=%d body=%s", forbidden.Code, stub.stateCalls, forbidden.Body.String())
	}

	authenticator.principal.Scopes = []library.DeviceScope{library.DeviceScopeRSSState}
	allowed := performRequest(router, http.MethodPatch, "/api/v1/rss/entries/entry-1/state", "Bearer token", stateBody)
	if allowed.Code != http.StatusOK || stub.stateCalls != 1 || stub.stateDeviceID != "iphone-1" {
		t.Fatalf("rss.state status=%d calls=%d device=%q body=%s", allowed.Code, stub.stateCalls, stub.stateDeviceID, allowed.Body.String())
	}
	readForbidden := performRequest(router, http.MethodGet, "/api/v1/rss/subscriptions", "Bearer token", "")
	if readForbidden.Code != http.StatusForbidden {
		t.Fatalf("rss.state unexpectedly read subscriptions: status=%d body=%s", readForbidden.Code, readForbidden.Body.String())
	}
}

func TestRSSErrorsAreStableAndOpaque(t *testing.T) {
	for name, test := range map[string]struct {
		err  error
		code int
		body string
	}{
		"not found": {err: domainrss.ErrNotFound, code: http.StatusNotFound, body: "not_found"},
		"conflict":  {err: domainrss.ErrRevisionConflict, code: http.StatusConflict, body: "rss_state_conflict"},
		"internal":  {err: errors.New("database /private/path unavailable"), code: http.StatusInternalServerError, body: "internal_error"},
	} {
		t.Run(name, func(t *testing.T) {
			stub := &rssServiceStub{stateErr: test.err}
			api, _ := NewRSSAPI(stub)
			request := httptest.NewRequest(http.MethodPatch, "/api/v1/rss/entries/entry-1/state", strings.NewReader(
				`{"field":"read","value":false,"expectedRevision":1,"mutationId":"m-1"}`,
			))
			request.SetPathValue("id", "entry-1")
			request = request.WithContext(context.WithValue(request.Context(), principalContextKey{}, access.Principal{DeviceID: "iphone-1"}))
			recorder := httptest.NewRecorder()
			api.setEntryState(recorder, request)
			if recorder.Code != test.code || !strings.Contains(recorder.Body.String(), test.body) || strings.Contains(recorder.Body.String(), "/private/path") {
				t.Fatalf("status=%d body=%s", recorder.Code, recorder.Body.String())
			}
		})
	}
}

func TestRSSStateBodyLimitIsEnforced(t *testing.T) {
	stub := &rssServiceStub{}
	api, _ := NewRSSAPI(stub)
	oversized := `{"field":"read","value":true,"expectedRevision":0,"mutationId":"` + strings.Repeat("a", maxRSSStateBodyBytes) + `"}`
	request := httptest.NewRequest(http.MethodPatch, "/api/v1/rss/entries/entry-1/state", strings.NewReader(oversized))
	request.SetPathValue("id", "entry-1")
	request = request.WithContext(context.WithValue(request.Context(), principalContextKey{}, access.Principal{DeviceID: "iphone-1"}))
	recorder := httptest.NewRecorder()
	api.setEntryState(recorder, request)
	if recorder.Code != http.StatusBadRequest || stub.stateCalls != 0 {
		t.Fatalf("status=%d calls=%d body=%s", recorder.Code, stub.stateCalls, recorder.Body.String())
	}
}

func TestRSSSubscriptionListUsesArrayForEmptyResult(t *testing.T) {
	api, _ := NewRSSAPI(&rssServiceStub{})
	recorder := httptest.NewRecorder()
	api.listSubscriptions(recorder, httptest.NewRequest(http.MethodGet, "/api/v1/rss/subscriptions", nil))
	var payload []json.RawMessage
	if err := json.Unmarshal(recorder.Body.Bytes(), &payload); err != nil {
		t.Fatal(err)
	}
	if payload == nil || len(payload) != 0 {
		t.Fatalf("empty subscriptions = %#v", payload)
	}
}
