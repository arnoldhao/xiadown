package libraryapi

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"reflect"
	"strings"
	"testing"

	"xiadown/internal/application/library/access"
	"xiadown/internal/domain/library"
)

type fakeAuthenticator struct {
	principal access.Principal
	err       error
	calls     int
}

func (fake *fakeAuthenticator) Authenticate(_ context.Context, _ string) (access.Principal, error) {
	fake.calls++
	return fake.principal, fake.err
}

type fakePairer struct {
	result  access.PairResult
	err     error
	request access.PairRequest
}

func (fake *fakePairer) Pair(_ context.Context, request access.PairRequest) (access.PairResult, error) {
	fake.request = request
	return fake.result, fake.err
}

func TestPublicEndpointsAndProtectedScopeEnforcement(t *testing.T) {
	authenticator := &fakeAuthenticator{principal: access.Principal{
		GrantID: "grant-1", CatalogID: "catalog-1", Scopes: []library.DeviceScope{library.DeviceScopeLibraryRead},
	}}
	pairer := &fakePairer{}
	router, err := NewRouter(Config{
		Version: "2.0.0", CatalogID: "catalog-1", Authenticator: authenticator, Pairer: pairer,
		Routes: []ProtectedRoute{{
			Method: http.MethodGet, Path: "/api/v1/items", Scope: library.DeviceScopeLibraryRead,
			Handler: http.HandlerFunc(func(w http.ResponseWriter, request *http.Request) {
				principal, ok := PrincipalFromContext(request.Context())
				if !ok || principal.GrantID != "grant-1" {
					t.Fatal("authenticated principal missing from context")
				}
				w.WriteHeader(http.StatusNoContent)
			}),
		}},
	})
	if err != nil {
		t.Fatal(err)
	}

	for _, path := range []string{"/api/v1/health", "/api/v1/version"} {
		response := performRequest(router, http.MethodGet, path, "", "")
		if response.Code != http.StatusOK {
			t.Fatalf("%s: expected public endpoint, got %d", path, response.Code)
		}
	}
	if authenticator.calls != 0 {
		t.Fatal("health/version unexpectedly invoked bearer authentication")
	}

	unauthorized := performRequest(router, http.MethodGet, "/api/v1/items", "", "")
	if unauthorized.Code != http.StatusUnauthorized || unauthorized.Header().Get("WWW-Authenticate") == "" {
		t.Fatalf("expected bearer challenge, got %d %#v", unauthorized.Code, unauthorized.Header())
	}
	authorized := performRequest(router, http.MethodGet, "/api/v1/items", "Bearer token", "")
	if authorized.Code != http.StatusNoContent {
		t.Fatalf("expected authorized response, got %d: %s", authorized.Code, authorized.Body.String())
	}
	authenticator.principal.CatalogID = "catalog-2"
	wrongCatalog := performRequest(router, http.MethodGet, "/api/v1/items", "Bearer token", "")
	if wrongCatalog.Code != http.StatusForbidden || !strings.Contains(wrongCatalog.Body.String(), "catalog_access_denied") {
		t.Fatalf("expected catalog rejection, got %d: %s", wrongCatalog.Code, wrongCatalog.Body.String())
	}
	authenticator.principal.CatalogID = "catalog-1"

	authenticator.principal.Scopes = []library.DeviceScope{library.DeviceScopeTasksRead}
	forbidden := performRequest(router, http.MethodGet, "/api/v1/items", "Bearer token", "")
	if forbidden.Code != http.StatusForbidden {
		t.Fatalf("expected scope rejection, got %d", forbidden.Code)
	}
	authenticator.err = access.ErrUnauthorized
	invalid := performRequest(router, http.MethodGet, "/api/v1/items", "Bearer invalid", "")
	if invalid.Code != http.StatusUnauthorized {
		t.Fatalf("expected invalid token rejection, got %d", invalid.Code)
	}
}

func TestDeviceAccessRequiresCatalogBoundBearerButNoBusinessScope(t *testing.T) {
	authenticator := &fakeAuthenticator{principal: access.Principal{
		GrantID: "grant-1", CatalogID: "catalog-1", DeviceID: "iphone-1",
		Scopes: []library.DeviceScope{library.DeviceScopeTasksRead},
	}}
	router, err := NewRouter(Config{
		Version: "test", CatalogID: "catalog-1", Authenticator: authenticator, Pairer: &fakePairer{},
	})
	if err != nil {
		t.Fatal(err)
	}
	unauthorized := performRequest(router, http.MethodGet, "/api/v1/device-access", "", "")
	if unauthorized.Code != http.StatusUnauthorized {
		t.Fatalf("anonymous device access status = %d, want 401", unauthorized.Code)
	}
	authorized := performRequest(router, http.MethodGet, "/api/v1/device-access", "Bearer token", "")
	if authorized.Code != http.StatusOK {
		t.Fatalf("scope-free device access status = %d: %s", authorized.Code, authorized.Body.String())
	}
	var payload struct {
		GrantID   string                `json:"grantId"`
		CatalogID string                `json:"catalogId"`
		DeviceID  string                `json:"deviceId"`
		Scopes    []library.DeviceScope `json:"scopes"`
	}
	if err := json.Unmarshal(authorized.Body.Bytes(), &payload); err != nil {
		t.Fatal(err)
	}
	if payload.GrantID != "grant-1" || payload.CatalogID != "catalog-1" || payload.DeviceID != "iphone-1" ||
		len(payload.Scopes) != 1 || payload.Scopes[0] != library.DeviceScopeTasksRead {
		t.Fatalf("device access payload = %#v", payload)
	}
	authenticator.principal.CatalogID = "catalog-2"
	wrongCatalog := performRequest(router, http.MethodGet, "/api/v1/device-access", "Bearer token", "")
	if wrongCatalog.Code != http.StatusForbidden || !strings.Contains(wrongCatalog.Body.String(), "catalog_access_denied") {
		t.Fatalf("wrong-catalog device access = %d: %s", wrongCatalog.Code, wrongCatalog.Body.String())
	}
}

func TestDeviceAccessAdvertisesOnlyMountedStationsAndSeparatesAuthorization(t *testing.T) {
	authenticator := &fakeAuthenticator{principal: access.Principal{
		GrantID: "grant-1", CatalogID: "catalog-1", DeviceID: "iphone-1",
		Scopes: []library.DeviceScope{
			library.DeviceScopeLibraryRead, library.DeviceScopeMusicRead,
			library.DeviceScopeRSSManage, library.DeviceScopeRSSFetch,
		},
	}}
	router, err := NewRouter(Config{
		Version: "test", CatalogID: "catalog-1", Authenticator: authenticator, Pairer: &fakePairer{},
		Capabilities: []string{"sync-events-v1", "sync-events-v1", ""},
		StationCapabilities: map[string][]string{
			"library": {"library-v1", "library-v1"},
			"rss": {
				"rss-shared-public-fetch-v1", "opaque-resource-slots-v1", "rss-sync-v1",
				"rss-subscription-mutations-v1", "rss-observations-v1", "rss-fetch-lease-v1",
			},
			"unknown": {"must-not-leak"},
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	response := performRequest(router, http.MethodGet, "/api/v1/device-access", "Bearer token", "")
	if response.Code != http.StatusOK {
		t.Fatalf("device access status = %d: %s", response.Code, response.Body.String())
	}
	var payload struct {
		Scopes       []library.DeviceScope                 `json:"scopes"`
		Capabilities []string                              `json:"capabilities"`
		Stations     map[string]deviceAccessStationSummary `json:"stations"`
	}
	if err := json.Unmarshal(response.Body.Bytes(), &payload); err != nil {
		t.Fatal(err)
	}
	if len(payload.Stations) != 2 || payload.Stations["music"].Supported ||
		!payload.Stations["library"].Authorized || payload.Stations["rss"].Authorized {
		t.Fatalf("station summaries = %#v", payload.Stations)
	}
	if !reflect.DeepEqual(payload.Scopes, authenticator.principal.Scopes) {
		t.Fatalf("device access scopes = %#v, want %#v", payload.Scopes, authenticator.principal.Scopes)
	}
	if !reflect.DeepEqual(payload.Capabilities, []string{SyncEventsCapability}) {
		t.Fatalf("device access capabilities = %#v, want %q", payload.Capabilities, SyncEventsCapability)
	}
	wantRSS := []string{
		"opaque-resource-slots-v1", "rss-fetch-lease-v1", "rss-observations-v1",
		"rss-shared-public-fetch-v1", "rss-subscription-mutations-v1", "rss-sync-v1",
	}
	if !reflect.DeepEqual(payload.Stations["rss"].Capabilities, wantRSS) {
		t.Fatalf("RSS capabilities = %#v, want %#v", payload.Stations["rss"].Capabilities, wantRSS)
	}
}

func TestRSSManageAndFetchScopesDoNotImplyReadOrState(t *testing.T) {
	authenticator := &fakeAuthenticator{principal: access.Principal{
		GrantID: "grant-rss", CatalogID: "catalog-1", DeviceID: "iphone-1",
		Scopes: []library.DeviceScope{library.DeviceScopeRSSManage, library.DeviceScopeRSSFetch},
	}}
	router, err := NewRouter(Config{
		Version: "test", CatalogID: "catalog-1", Authenticator: authenticator, Pairer: &fakePairer{},
		Routes: []ProtectedRoute{
			{
				Method: http.MethodGet, Path: "/api/v1/scope-contract/rss-read",
				Scope: library.DeviceScopeRSSRead, Handler: http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
					w.WriteHeader(http.StatusNoContent)
				}),
			},
			{
				Method: http.MethodPatch, Path: "/api/v1/scope-contract/rss-state",
				Scope: library.DeviceScopeRSSState, Handler: http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
					w.WriteHeader(http.StatusNoContent)
				}),
			},
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	for _, request := range []struct {
		method string
		path   string
	}{
		{method: http.MethodGet, path: "/api/v1/scope-contract/rss-read"},
		{method: http.MethodPatch, path: "/api/v1/scope-contract/rss-state"},
	} {
		response := performRequest(router, request.method, request.path, "Bearer token", "")
		if response.Code != http.StatusForbidden || !strings.Contains(response.Body.String(), "insufficient_scope") {
			t.Fatalf("%s %s = %d: %s", request.method, request.path, response.Code, response.Body.String())
		}
	}
}

func TestRouterIsolationNeverExposesDesktopAssetOrWebsocketRoutes(t *testing.T) {
	authenticator := &fakeAuthenticator{principal: access.Principal{
		GrantID: "grant-1", CatalogID: "catalog-1", Scopes: []library.DeviceScope{library.DeviceScopeTasksRead},
	}}
	router, err := NewRouter(Config{Version: "test", CatalogID: "catalog-1", Authenticator: authenticator, Pairer: &fakePairer{}})
	if err != nil {
		t.Fatal(err)
	}
	for _, path := range []string{
		"/api/library/asset?path=/etc/passwd", "/ws", "/api/library/file/maintenance",
	} {
		response := performRequest(router, http.MethodGet, path, "Bearer valid", "")
		if response.Code != http.StatusNotFound {
			t.Fatalf("isolated router exposed %s with status %d", path, response.Code)
		}
	}
	if authenticator.calls != 0 {
		t.Fatal("desktop paths should not even enter public API authentication")
	}

	unknown := performRequest(router, http.MethodGet, "/api/v1/not-registered", "", "")
	if unknown.Code != http.StatusUnauthorized {
		t.Fatalf("unregistered v1 path must reject anonymous access, got %d", unknown.Code)
	}
	unknown = performRequest(router, http.MethodGet, "/api/v1/not-registered", "Bearer valid", "")
	if unknown.Code != http.StatusNotFound {
		t.Fatalf("authenticated unregistered v1 path should be absent, got %d", unknown.Code)
	}
}

func TestPairEndpointReturnsCredentialOnceAndMapsErrors(t *testing.T) {
	pairer := &fakePairer{result: access.PairResult{
		GrantID: "grant-1", Token: "one-time-token",
		Scopes: []library.DeviceScope{library.DeviceScopeLibraryRead, library.DeviceScopeTasksRead},
	}}
	router, err := NewRouter(Config{Version: "test", CatalogID: "catalog-1", Authenticator: &fakeAuthenticator{}, Pairer: pairer})
	if err != nil {
		t.Fatal(err)
	}
	body := `{"nonce":"nonce-1","code":"123456","deviceID":"iphone","name":"iPhone","publicKeyHash":"hash"}`
	response := performRequest(router, http.MethodPost, "/api/v1/pair", "", body)
	if response.Code != http.StatusCreated {
		t.Fatalf("expected pairing success, got %d: %s", response.Code, response.Body.String())
	}
	var payload map[string]any
	if err := json.Unmarshal(response.Body.Bytes(), &payload); err != nil {
		t.Fatal(err)
	}
	if payload["token"] != "one-time-token" || pairer.request.DeviceName != "iPhone" {
		t.Fatalf("unexpected pairing exchange: payload=%#v request=%#v", payload, pairer.request)
	}

	pairer.err = access.ErrPairingInvalid
	response = performRequest(router, http.MethodPost, "/api/v1/pair", "", body)
	if response.Code != http.StatusUnauthorized || strings.Contains(response.Body.String(), "nonce-1") {
		t.Fatalf("expected opaque pairing rejection, got %d: %s", response.Code, response.Body.String())
	}
	pairer.err = errors.New("database failed")
	response = performRequest(router, http.MethodPost, "/api/v1/pair", "", body)
	if response.Code != http.StatusInternalServerError {
		t.Fatalf("expected internal pairing error, got %d", response.Code)
	}
}

func performRequest(handler http.Handler, method, path, authorization, body string) *httptest.ResponseRecorder {
	request := httptest.NewRequest(method, path, strings.NewReader(body))
	if authorization != "" {
		request.Header.Set("Authorization", authorization)
	}
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)
	return response
}
