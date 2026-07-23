package libraryapi

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"xiadown/internal/application/library/access"
	applicationrss "xiadown/internal/application/rss"
	"xiadown/internal/domain/library"
	domainrss "xiadown/internal/domain/rss"
)

type rssSharedPublicServiceStub struct {
	*rssServiceStub
	subscriptions     []domainrss.SyncSubscription
	mutation          applicationrss.SharedSubscriptionMutationRequest
	mutationID        string
	mutationDevice    string
	mutationResult    domainrss.SubscriptionMutationResult
	mutationErr       error
	observation       applicationrss.FeedObservationRequest
	observationID     string
	observationDevice string
	observationResult domainrss.ObservationResult
	observationErr    error
	lease             applicationrss.FetchLeaseApplicationRequest
	leaseID           string
	leaseDevice       string
	leaseResult       domainrss.FetchLeaseResult
	leaseErr          error
	sourceAccess      domainrss.SubscriptionSourceAccess
	sourceURL         string
	sourceErr         error
	mutationCalls     int
	observationCalls  int
	leaseCalls        int
	sourceCalls       int
}

func (stub *rssSharedPublicServiceStub) ListSyncSubscriptions(context.Context) ([]domainrss.SyncSubscription, error) {
	return append([]domainrss.SyncSubscription(nil), stub.subscriptions...), nil
}

func (stub *rssSharedPublicServiceStub) MutateSubscriptionForDevice(
	_ context.Context,
	deviceID, subscriptionID string,
	request applicationrss.SharedSubscriptionMutationRequest,
) (domainrss.SubscriptionMutationResult, error) {
	stub.mutationCalls++
	stub.mutationDevice, stub.mutationID, stub.mutation = deviceID, subscriptionID, request
	return stub.mutationResult, stub.mutationErr
}

func (stub *rssSharedPublicServiceStub) SubmitFeedObservationForDevice(
	_ context.Context,
	deviceID, subscriptionID string,
	request applicationrss.FeedObservationRequest,
) (domainrss.ObservationResult, error) {
	stub.observationCalls++
	stub.observationDevice, stub.observationID, stub.observation = deviceID, subscriptionID, request
	return stub.observationResult, stub.observationErr
}

func (stub *rssSharedPublicServiceStub) AcquireFetchLeaseForDevice(
	_ context.Context,
	deviceID, subscriptionID string,
	request applicationrss.FetchLeaseApplicationRequest,
) (domainrss.FetchLeaseResult, error) {
	stub.leaseCalls++
	stub.leaseDevice, stub.leaseID, stub.lease = deviceID, subscriptionID, request
	return stub.leaseResult, stub.leaseErr
}

func (stub *rssSharedPublicServiceStub) GetSyncSubscriptionSource(
	context.Context,
	string,
) (domainrss.SubscriptionSourceAccess, string, error) {
	stub.sourceCalls++
	return stub.sourceAccess, stub.sourceURL, stub.sourceErr
}

func TestRSSSharedPublicRoutesEnforceIndependentManageAndFetchScopes(t *testing.T) {
	service := &rssSharedPublicServiceStub{rssServiceStub: &rssServiceStub{}}
	api, err := NewRSSAPI(service)
	if err != nil {
		t.Fatal(err)
	}
	authenticator := &fakeAuthenticator{principal: access.Principal{
		GrantID: "grant-rss", CatalogID: "catalog-1", DeviceID: "iphone-1",
		Scopes: []library.DeviceScope{library.DeviceScopeRSSManage},
	}}
	router, err := NewRouter(Config{
		Version: "test", CatalogID: "catalog-1", Authenticator: authenticator,
		Pairer: &fakePairer{}, Routes: api.Routes(),
	})
	if err != nil {
		t.Fatal(err)
	}
	const subscriptionID = "7f7a68d0-49ab-4e90-88bb-28fd77551d4e"
	updateBody := `{"mutationId":"6553f2ea-6359-4ced-b554-05905ca16632","operation":"update","expectedRevision":1,"fieldMask":["title"],"title":"Renamed"}`
	updated := performRequest(router, http.MethodPost, "/api/v1/rss/subscriptions/"+subscriptionID+"/mutations", "Bearer token", updateBody)
	if updated.Code != http.StatusOK || service.mutationCalls != 1 || service.mutationDevice != "iphone-1" || service.mutationID != subscriptionID {
		t.Fatalf("manage update status=%d calls=%d device=%q id=%q body=%s", updated.Code, service.mutationCalls, service.mutationDevice, service.mutationID, updated.Body.String())
	}

	addBody := `{"mutationId":"6553f2ea-6359-4ced-b554-05905ca16632","operation":"add","sourceAccess":"sharedPublic","publicFeedURL":"https://feeds.example.com/feed.xml"}`
	addWithoutFetch := performRequest(router, http.MethodPost, "/api/v1/rss/subscriptions/"+subscriptionID+"/mutations", "Bearer token", addBody)
	if addWithoutFetch.Code != http.StatusForbidden || !strings.Contains(addWithoutFetch.Body.String(), "insufficient_scope") || service.mutationCalls != 1 {
		t.Fatalf("manage-only public add status=%d calls=%d body=%s", addWithoutFetch.Code, service.mutationCalls, addWithoutFetch.Body.String())
	}

	publicURLUpdate := `{"mutationId":"6553f2ea-6359-4ced-b554-05905ca16632","operation":"update","expectedRevision":1,"fieldMask":["publicFeedURL"],"publicFeedURL":"https://feeds.example.com/new.xml"}`
	publicURLWithoutFetch := performRequest(router, http.MethodPost, "/api/v1/rss/subscriptions/"+subscriptionID+"/mutations", "Bearer token", publicURLUpdate)
	promoteWithoutFetch := performRequest(router, http.MethodPost, "/api/v1/rss/subscriptions/"+subscriptionID+"/mutations", "Bearer token", strings.Replace(addBody, `"operation":"add"`, `"operation":"promote"`, 1))
	if publicURLWithoutFetch.Code != http.StatusForbidden || promoteWithoutFetch.Code != http.StatusForbidden || service.mutationCalls != 1 {
		t.Fatalf("manage-only URL/promote statuses=%d/%d calls=%d", publicURLWithoutFetch.Code, promoteWithoutFetch.Code, service.mutationCalls)
	}

	fetchLeaseWithoutFetch := performRequest(router, http.MethodPost, "/api/v1/rss/subscriptions/"+subscriptionID+"/fetch-lease", "Bearer token", `{}`)
	observationWithoutFetch := performRequest(router, http.MethodPost, "/api/v1/rss/subscriptions/"+subscriptionID+"/observations", "Bearer token", `{}`)
	if fetchLeaseWithoutFetch.Code != http.StatusForbidden || observationWithoutFetch.Code != http.StatusForbidden ||
		service.leaseCalls != 0 || service.observationCalls != 0 {
		t.Fatalf("manage-only fetch statuses=%d/%d calls=%d/%d", fetchLeaseWithoutFetch.Code, observationWithoutFetch.Code, service.leaseCalls, service.observationCalls)
	}

	authenticator.principal.Scopes = []library.DeviceScope{library.DeviceScopeRSSManage, library.DeviceScopeRSSFetch}
	added := performRequest(router, http.MethodPost, "/api/v1/rss/subscriptions/"+subscriptionID+"/mutations", "Bearer token", addBody)
	leased := performRequest(router, http.MethodPost, "/api/v1/rss/subscriptions/"+subscriptionID+"/fetch-lease", "Bearer token", `{"ttlSeconds":60}`)
	observed := performRequest(router, http.MethodPost, "/api/v1/rss/subscriptions/"+subscriptionID+"/observations", "Bearer token",
		`{"mutationId":"72233b29-c756-4a81-af79-2fff7dcc289f","fetchedAt":"2026-07-21T12:00:00.000Z","contentHash":"4f53cda18c2baa0c0354bb5f9a3ecbe5ed12ab4d8e11ba873c2f11161202b945","entries":[]}`)
	if added.Code != http.StatusCreated || leased.Code != http.StatusOK || observed.Code != http.StatusOK ||
		service.mutationCalls != 2 || service.leaseCalls != 1 || service.observationCalls != 1 ||
		service.leaseDevice != "iphone-1" || service.observationDevice != "iphone-1" ||
		service.leaseID != subscriptionID || service.observationID != subscriptionID || service.lease.TTLSeconds != 60 {
		t.Fatalf("full-scope statuses=%d/%d/%d calls=%d/%d/%d", added.Code, leased.Code, observed.Code,
			service.mutationCalls, service.leaseCalls, service.observationCalls)
	}

	authenticator.principal.Scopes = []library.DeviceScope{library.DeviceScopeRSSFetch}
	fetchOnlyMutation := performRequest(router, http.MethodPost, "/api/v1/rss/subscriptions/"+subscriptionID+"/mutations", "Bearer token", updateBody)
	if fetchOnlyMutation.Code != http.StatusForbidden || service.mutationCalls != 2 {
		t.Fatalf("fetch-only mutation status=%d calls=%d body=%s", fetchOnlyMutation.Code, service.mutationCalls, fetchOnlyMutation.Body.String())
	}
}

func TestRSSSharedPublicHandlersRejectUnknownCredentialAndSessionFields(t *testing.T) {
	service := &rssSharedPublicServiceStub{rssServiceStub: &rssServiceStub{}}
	api, _ := NewRSSAPI(service)
	principal := access.Principal{
		DeviceID: "iphone-1",
		Scopes:   []library.DeviceScope{library.DeviceScopeRSSManage, library.DeviceScopeRSSFetch},
	}
	const subscriptionID = "7f7a68d0-49ab-4e90-88bb-28fd77551d4e"
	tests := []struct {
		name    string
		handler func(http.ResponseWriter, *http.Request)
		body    string
	}{
		{
			name: "mutation private descriptor", handler: api.mutateSubscription,
			body: `{"mutationId":"6553f2ea-6359-4ced-b554-05905ca16632","operation":"add","sourceAccess":"sharedPublic","publicFeedURL":"https://feeds.example.com/feed.xml","privateFeedURL":"https://private.example/token"}`,
		},
		{
			name: "observation cookies", handler: api.submitObservation,
			body: `{"mutationId":"72233b29-c756-4a81-af79-2fff7dcc289f","fetchedAt":"2026-07-21T12:00:00Z","contentHash":"aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa","entries":[],"cookies":{"session":"secret"}}`,
		},
		{
			name: "lease headers", handler: api.acquireFetchLease,
			body: `{"ttlSeconds":60,"headers":{"Authorization":"Bearer secret"}}`,
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			request := httptest.NewRequest(http.MethodPost, "/api/v1/rss/subscriptions/"+subscriptionID, strings.NewReader(test.body))
			request.SetPathValue("id", subscriptionID)
			request = request.WithContext(context.WithValue(request.Context(), principalContextKey{}, principal))
			recorder := httptest.NewRecorder()
			test.handler(recorder, request)
			if recorder.Code != http.StatusBadRequest || !strings.Contains(recorder.Body.String(), "invalid_request") {
				t.Fatalf("status=%d body=%s", recorder.Code, recorder.Body.String())
			}
		})
	}
	if service.mutationCalls != 0 || service.observationCalls != 0 || service.leaseCalls != 0 {
		t.Fatalf("unknown sensitive fields reached service: mutation=%d observation=%d lease=%d",
			service.mutationCalls, service.observationCalls, service.leaseCalls)
	}
}

func TestRSSPublicFeedURLOmissionTracksCurrentFetchScopeAcrossDTOs(t *testing.T) {
	now := time.Date(2026, 7, 21, 15, 0, 0, 0, time.UTC)
	shared := domainrss.SyncSubscription{
		ID: "shared-subscription", WorkspaceID: domainrss.DefaultWorkspaceID, Title: "Shared",
		ViewType: domainrss.ViewTypeArticle, Enabled: true, CreatedAt: now, UpdatedAt: now, Revision: 1,
		SourceAccess:  domainrss.SubscriptionSourceSharedPublic,
		PublicFeedURL: "https://feeds.example.com/service-value.xml",
	}
	desktop := shared
	desktop.ID = "desktop-subscription"
	desktop.Title = "Desktop private"
	desktop.SourceAccess = domainrss.SubscriptionSourceDesktopManaged
	desktop.PublicFeedURL = "https://private.example/session-token"
	payload, err := json.Marshal(domainrss.SyncSubscription{
		ID: shared.ID, WorkspaceID: shared.WorkspaceID, Title: shared.Title,
		ViewType: shared.ViewType, Enabled: true, CreatedAt: now, UpdatedAt: now, Revision: 1,
		SourceAccess:  domainrss.SubscriptionSourceSharedPublic,
		PublicFeedURL: "https://untrusted-journal.example/leak",
	})
	if err != nil {
		t.Fatal(err)
	}
	service := &rssSharedPublicServiceStub{
		rssServiceStub: &rssServiceStub{
			snapshotResult: applicationrss.SyncSnapshotResult{
				Epoch: testRSSEpoch, HighWater: 1,
				Records: []domainrss.SyncSnapshotRecord{{EntityType: "subscription", EntityID: shared.ID, Revision: 1, Payload: payload}},
			},
			changesResult: domainrss.ChangePage{
				Epoch: testRSSEpoch, Cursor: 1, HighWater: 1,
				Changes: []domainrss.Change{{Sequence: 1, EntityType: "subscription", EntityID: shared.ID, Revision: 1, Payload: payload}},
			},
		},
		subscriptions: []domainrss.SyncSubscription{shared, desktop},
		sourceAccess:  domainrss.SubscriptionSourceSharedPublic,
		sourceURL:     "https://feeds.example.com/authorized-source.xml",
	}
	api, _ := NewRSSAPI(service)

	readOnly := rssRequestWithPrincipal(http.MethodGet, "/api/v1/rss/subscriptions", "iphone-1", library.DeviceScopeRSSRead)
	readOnlyList := httptest.NewRecorder()
	api.listSubscriptions(readOnlyList, readOnly)
	if readOnlyList.Code != http.StatusOK || strings.Contains(readOnlyList.Body.String(), "publicFeedURL") ||
		strings.Contains(readOnlyList.Body.String(), "session-token") {
		t.Fatalf("read-only subscriptions leaked descriptor: %s", readOnlyList.Body.String())
	}
	if !strings.Contains(readOnlyList.Body.String(), `"sourceAccess":"sharedPublic"`) {
		t.Fatalf("read-only subscription lost sourceAccess signal: %s", readOnlyList.Body.String())
	}

	withFetch := rssRequestWithPrincipal(http.MethodGet, "/api/v1/rss/subscriptions", "iphone-1",
		library.DeviceScopeRSSRead, library.DeviceScopeRSSFetch)
	fetchList := httptest.NewRecorder()
	api.listSubscriptions(fetchList, withFetch)
	if fetchList.Code != http.StatusOK || !strings.Contains(fetchList.Body.String(), shared.PublicFeedURL) ||
		strings.Contains(fetchList.Body.String(), desktop.PublicFeedURL) {
		t.Fatalf("fetch-scope subscriptions projection: %s", fetchList.Body.String())
	}

	readOnlySnapshot := rssRequestWithPrincipal(http.MethodGet,
		"/api/v1/rss/snapshot?epoch="+testRSSEpoch+"&highWater=1", "iphone-1", library.DeviceScopeRSSRead)
	readSnapshotRecorder := httptest.NewRecorder()
	api.getSnapshot(readSnapshotRecorder, readOnlySnapshot)
	if readSnapshotRecorder.Code != http.StatusOK || strings.Contains(readSnapshotRecorder.Body.String(), "publicFeedURL") || service.sourceCalls != 0 {
		t.Fatalf("read-only snapshot leaked/enriched URL: calls=%d body=%s", service.sourceCalls, readSnapshotRecorder.Body.String())
	}

	fetchSnapshot := rssRequestWithPrincipal(http.MethodGet,
		"/api/v1/rss/snapshot?epoch="+testRSSEpoch+"&highWater=1", "iphone-1",
		library.DeviceScopeRSSRead, library.DeviceScopeRSSFetch)
	fetchSnapshotRecorder := httptest.NewRecorder()
	api.getSnapshot(fetchSnapshotRecorder, fetchSnapshot)
	if fetchSnapshotRecorder.Code != http.StatusOK || !strings.Contains(fetchSnapshotRecorder.Body.String(), service.sourceURL) ||
		strings.Contains(fetchSnapshotRecorder.Body.String(), "untrusted-journal") || service.sourceCalls != 1 {
		t.Fatalf("fetch snapshot projection calls=%d body=%s", service.sourceCalls, fetchSnapshotRecorder.Body.String())
	}

	readChanges := rssRequestWithPrincipal(http.MethodGet,
		"/api/v1/rss/changes?epoch="+testRSSEpoch+"&after=0", "iphone-1", library.DeviceScopeRSSRead)
	readChangesRecorder := httptest.NewRecorder()
	api.listChanges(readChangesRecorder, readChanges)
	if readChangesRecorder.Code != http.StatusOK || strings.Contains(readChangesRecorder.Body.String(), "publicFeedURL") || service.sourceCalls != 1 {
		t.Fatalf("scope downgrade did not purge change URL: calls=%d body=%s", service.sourceCalls, readChangesRecorder.Body.String())
	}
}

func TestRSSMutationResultOmitsPublicURLWithoutFetchScopeAndMapsConflicts(t *testing.T) {
	now := time.Date(2026, 7, 21, 16, 0, 0, 0, time.UTC)
	item := domainrss.SyncSubscription{
		ID: "7f7a68d0-49ab-4e90-88bb-28fd77551d4e", WorkspaceID: domainrss.DefaultWorkspaceID,
		Title: "Renamed", ViewType: domainrss.ViewTypeArticle, Enabled: true,
		CreatedAt: now, UpdatedAt: now, Revision: 2,
		SourceAccess:  domainrss.SubscriptionSourceSharedPublic,
		PublicFeedURL: "https://feeds.example.com/public.xml",
	}
	service := &rssSharedPublicServiceStub{
		rssServiceStub: &rssServiceStub{},
		mutationResult: domainrss.SubscriptionMutationResult{
			MutationID: "6553f2ea-6359-4ced-b554-05905ca16632", Operation: "update", Subscription: &item, Revision: 2,
		},
	}
	api, _ := NewRSSAPI(service)
	newRequest := func() *http.Request {
		request := httptest.NewRequest(http.MethodPost, "/api/v1/rss/subscriptions/"+item.ID+"/mutations", strings.NewReader(
			`{"mutationId":"6553f2ea-6359-4ced-b554-05905ca16632","operation":"update","expectedRevision":1,"fieldMask":["title"],"title":"Renamed"}`,
		))
		request.SetPathValue("id", item.ID)
		return request.WithContext(context.WithValue(request.Context(), principalContextKey{}, access.Principal{
			DeviceID: "iphone-1", Scopes: []library.DeviceScope{library.DeviceScopeRSSManage},
		}))
	}

	recorder := httptest.NewRecorder()
	api.mutateSubscription(recorder, newRequest())
	if recorder.Code != http.StatusOK || strings.Contains(recorder.Body.String(), "publicFeedURL") ||
		!strings.Contains(recorder.Body.String(), `"sourceAccess":"sharedPublic"`) {
		t.Fatalf("manage-only mutation result status=%d body=%s", recorder.Code, recorder.Body.String())
	}

	for _, test := range []struct {
		err  error
		code string
	}{
		{err: domainrss.ErrRevisionConflict, code: "revision_conflict"},
		{err: domainrss.ErrIdempotencyConflict, code: "idempotency_conflict"},
	} {
		service.mutationErr = test.err
		conflict := httptest.NewRecorder()
		api.mutateSubscription(conflict, newRequest())
		if conflict.Code != http.StatusConflict || !strings.Contains(conflict.Body.String(), `"error":"`+test.code+`"`) {
			t.Fatalf("error %v mapped to status=%d body=%s", test.err, conflict.Code, conflict.Body.String())
		}
	}
}

func rssRequestWithPrincipal(method, target, deviceID string, scopes ...library.DeviceScope) *http.Request {
	request := httptest.NewRequest(method, target, nil)
	return request.WithContext(context.WithValue(request.Context(), principalContextKey{}, access.Principal{
		DeviceID: deviceID, Scopes: append([]library.DeviceScope(nil), scopes...),
	}))
}
