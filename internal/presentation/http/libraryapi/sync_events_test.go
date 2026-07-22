package libraryapi

import (
	"bufio"
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	appevents "xiadown/internal/application/events"
	"xiadown/internal/application/library/access"
	"xiadown/internal/domain/library"
)

type mutableSyncAuthenticator struct {
	mu        sync.Mutex
	principal access.Principal
	err       error
	calls     int
}

func (authenticator *mutableSyncAuthenticator) Authenticate(
	_ context.Context,
	_ string,
) (access.Principal, error) {
	authenticator.mu.Lock()
	defer authenticator.mu.Unlock()
	authenticator.calls++
	return authenticator.principal, authenticator.err
}

func (authenticator *mutableSyncAuthenticator) setPrincipal(principal access.Principal) {
	authenticator.mu.Lock()
	authenticator.principal = principal
	authenticator.err = nil
	authenticator.mu.Unlock()
}

func (authenticator *mutableSyncAuthenticator) revoke() {
	authenticator.mu.Lock()
	authenticator.err = access.ErrUnauthorized
	authenticator.mu.Unlock()
}

func (authenticator *mutableSyncAuthenticator) callCount() int {
	authenticator.mu.Lock()
	defer authenticator.mu.Unlock()
	return authenticator.calls
}

func TestSyncEventAPIRequiresLiveGrantRevalidation(t *testing.T) {
	if _, err := NewSyncEventAPI(SyncEventConfig{Events: appevents.NewInMemoryBus()}); err == nil {
		t.Fatal("SyncEventAPI accepted a stream configuration without live grant revalidation")
	}
}

func TestSyncEventHubFiltersScopesAndCoalescesSlowConsumers(t *testing.T) {
	hub := newSyncEventHub(8)
	subscription := hub.subscribe(access.Principal{Scopes: []library.DeviceScope{
		library.DeviceScopeMusicRead, library.DeviceScopeTasksRead,
	}}, 0)
	defer subscription.close()

	hub.publishStation(SyncStationPosition{Station: "library", Epoch: strings.Repeat("a", 32), HighWater: 1})
	hub.publishStation(SyncStationPosition{Station: "music", Epoch: strings.Repeat("b", 32), HighWater: 2})
	for highWater := int64(3); highWater <= 100; highWater++ {
		hub.publishStation(SyncStationPosition{Station: "music", Epoch: strings.Repeat("b", 32), HighWater: highWater})
	}
	hub.publishStation(SyncStationPosition{Station: "rss", Epoch: strings.Repeat("c", 32), HighWater: 4})
	hub.publishTasksDirty()

	subscription.mu.Lock()
	if len(subscription.pending) != 2 {
		t.Fatalf("pending hints = %d, want one Music plus one Tasks hint", len(subscription.pending))
	}
	if music := subscription.pending["music"]; music.position.HighWater != 100 {
		t.Fatalf("coalesced Music highWater = %d, want 100", music.position.HighWater)
	}
	if _, exists := subscription.pending["library"]; exists {
		t.Fatal("library.read-free grant received a Library hint")
	}
	if _, exists := subscription.pending["rss"]; exists {
		t.Fatal("rss.read-free grant received an RSS hint")
	}
	subscription.mu.Unlock()
}

func TestSyncEventPayloadVocabularyCannotCarryInternalTaskOrEntityData(t *testing.T) {
	for _, event := range []syncServerEvent{
		{id: 1, kind: "station-dirty", position: SyncStationPosition{
			Station: "library", Epoch: strings.Repeat("a", 32), HighWater: 44,
		}},
		{id: 2, kind: "tasks-dirty"},
	} {
		response := httptest.NewRecorder()
		if err := writeSyncServerEvent(response, event); err != nil {
			t.Fatal(err)
		}
		body := response.Body.String()
		for _, forbidden := range []string{"entity", "title", "path", "url", "token", "payload", "operation"} {
			if strings.Contains(strings.ToLower(body), forbidden) {
				t.Fatalf("%s event contains forbidden vocabulary %q: %q", event.kind, forbidden, body)
			}
		}
		if event.kind == "tasks-dirty" && body != "id: 2\nevent: tasks-dirty\ndata: {\"dirty\":true}\n\n" {
			t.Fatalf("tasks-dirty wire body = %q", body)
		}
	}
}

func TestSyncEventHubLastEventIDReplayAndGapFallback(t *testing.T) {
	hub := newSyncEventHub(2)
	hub.publishStation(SyncStationPosition{Station: "library", Epoch: strings.Repeat("a", 32), HighWater: 1}) // id 1, evicted
	hub.publishStation(SyncStationPosition{Station: "music", Epoch: strings.Repeat("b", 32), HighWater: 2})   // id 2
	hub.publishStation(SyncStationPosition{Station: "library", Epoch: strings.Repeat("a", 32), HighWater: 3}) // id 3

	current := hub.subscribe(access.Principal{Scopes: []library.DeviceScope{
		library.DeviceScopeLibraryRead, library.DeviceScopeMusicRead,
	}}, 0)
	defer current.close()
	if len(current.pending) != 2 || current.pending["library"].position.HighWater != 3 ||
		current.pending["music"].position.HighWater != 2 {
		t.Fatalf("initial latest hints = %#v", current.pending)
	}

	replay := hub.subscribe(access.Principal{Scopes: []library.DeviceScope{library.DeviceScopeLibraryRead}}, 2)
	defer replay.close()
	if len(replay.pending) != 1 || replay.pending["library"].id != 3 {
		t.Fatalf("Last-Event-ID replay = %#v, want only event 3", replay.pending)
	}

	gap := hub.subscribe(access.Principal{Scopes: []library.DeviceScope{
		library.DeviceScopeLibraryRead, library.DeviceScopeMusicRead,
	}}, 1)
	defer gap.close()
	if len(gap.pending) != 2 || gap.pending["library"].id != 3 || gap.pending["music"].id != 2 {
		t.Fatalf("history-gap fallback = %#v, want current Station positions", gap.pending)
	}
}

func TestSyncEventsHTTPStreamIsAuthenticatedFilteredAndCleansUp(t *testing.T) {
	authenticator := &fakeAuthenticator{principal: access.Principal{
		GrantID: "grant-music", CatalogID: "catalog-1", DeviceID: "iphone-1",
		Scopes: []library.DeviceScope{library.DeviceScopeMusicRead},
	}}
	api, err := NewSyncEventAPI(SyncEventConfig{
		Events: appevents.NewInMemoryBus(), Revalidator: authenticator, KeepaliveInterval: time.Hour,
	})
	if err != nil {
		t.Fatal(err)
	}
	router, err := NewRouter(Config{
		Version: "test", CatalogID: "catalog-1", Authenticator: authenticator, Pairer: &fakePairer{},
		AuthenticatedRoutes: api.Routes(),
	})
	if err != nil {
		t.Fatal(err)
	}
	server := httptest.NewServer(router)
	defer server.Close()

	unauthorized, err := server.Client().Get(server.URL + "/api/v1/sync/events")
	if err != nil {
		t.Fatal(err)
	}
	_ = unauthorized.Body.Close()
	if unauthorized.StatusCode != http.StatusUnauthorized {
		t.Fatalf("anonymous stream status = %d, want 401", unauthorized.StatusCode)
	}

	requestCtx, cancel := context.WithCancel(context.Background())
	request, _ := http.NewRequestWithContext(requestCtx, http.MethodGet, server.URL+"/api/v1/sync/events", nil)
	request.Header.Set("Authorization", "Bearer token")
	response, err := server.Client().Do(request)
	if err != nil {
		t.Fatal(err)
	}
	if response.StatusCode != http.StatusOK || !strings.HasPrefix(response.Header.Get("Content-Type"), "text/event-stream") ||
		response.Header.Get("X-Accel-Buffering") != "no" {
		cancel()
		_ = response.Body.Close()
		t.Fatalf("stream response = %d %#v", response.StatusCode, response.Header)
	}

	api.hub.publishStation(SyncStationPosition{Station: "library", Epoch: strings.Repeat("a", 32), HighWater: 7})
	api.hub.publishStation(SyncStationPosition{Station: "music", Epoch: strings.Repeat("b", 32), HighWater: 9})
	api.hub.publishStation(SyncStationPosition{Station: "rss", Epoch: strings.Repeat("c", 32), HighWater: 11})

	block := readSSEBlock(t, response.Body)
	if !strings.Contains(block, "event: station-dirty") ||
		!strings.Contains(block, `data: {"station":"music","epoch":"`+strings.Repeat("b", 32)+`","highWater":9}`) {
		t.Fatalf("Music stream block = %q", block)
	}
	if strings.Contains(block, "library") || strings.Contains(block, "rss") || strings.Contains(block, "path") ||
		strings.Contains(block, "url") || strings.Contains(block, "token") {
		t.Fatalf("stream leaked unauthorized or sensitive vocabulary: %q", block)
	}

	cancel()
	_ = response.Body.Close()
	deadline := time.Now().Add(time.Second)
	for time.Now().Before(deadline) {
		api.hub.mu.Lock()
		count := len(api.hub.subscribers)
		api.hub.mu.Unlock()
		if count == 0 {
			return
		}
		time.Sleep(time.Millisecond)
	}
	t.Fatal("disconnected SSE subscriber was not removed")
}

func TestActiveSyncStreamAppliesScopeRevocationBeforeNextHint(t *testing.T) {
	principal := access.Principal{
		GrantID: "grant-live", CatalogID: "catalog-1", DeviceID: "iphone-1",
		Scopes: []library.DeviceScope{library.DeviceScopeLibraryRead, library.DeviceScopeMusicRead},
	}
	authenticator := &mutableSyncAuthenticator{principal: principal}
	api, err := NewSyncEventAPI(SyncEventConfig{
		Events: appevents.NewInMemoryBus(), Revalidator: authenticator,
		KeepaliveInterval: time.Hour, AuthCheckInterval: 5 * time.Millisecond,
	})
	if err != nil {
		t.Fatal(err)
	}
	router, err := NewRouter(Config{
		Version: "test", CatalogID: "catalog-1", Authenticator: authenticator, Pairer: &fakePairer{},
		AuthenticatedRoutes: api.Routes(),
	})
	if err != nil {
		t.Fatal(err)
	}
	server := httptest.NewServer(router)
	defer server.Close()
	request, _ := http.NewRequest(http.MethodGet, server.URL+"/api/v1/sync/events", nil)
	request.Header.Set("Authorization", "Bearer credential-that-must-not-cross-the-stream")
	response, err := server.Client().Do(request)
	if err != nil {
		t.Fatal(err)
	}
	defer response.Body.Close()

	api.hub.publishStation(SyncStationPosition{Station: "music", Epoch: strings.Repeat("b", 32), HighWater: 1})
	if block := readSSEBlock(t, response.Body); !strings.Contains(block, `"station":"music"`) {
		t.Fatalf("initial authorized Music hint = %q", block)
	}

	principal.Scopes = []library.DeviceScope{library.DeviceScopeLibraryRead}
	authenticator.setPrincipal(principal)
	time.Sleep(10 * time.Millisecond)
	// Queue the now-revoked hint first. Revalidation must remove it atomically
	// before pop, then permit the still-authorized Library hint.
	api.hub.publishStation(SyncStationPosition{Station: "music", Epoch: strings.Repeat("b", 32), HighWater: 2})
	api.hub.publishStation(SyncStationPosition{Station: "library", Epoch: strings.Repeat("a", 32), HighWater: 3})
	block := readSSEBlock(t, response.Body)
	if !strings.Contains(block, `"station":"library"`) || strings.Contains(block, `"station":"music"`) ||
		strings.Contains(block, "credential-that-must-not-cross-the-stream") {
		t.Fatalf("post-revocation hint = %q", block)
	}
	if authenticator.callCount() < 2 {
		t.Fatalf("Authenticate calls = %d, want handshake plus active-stream revalidation", authenticator.callCount())
	}
}

func TestActiveSyncStreamClosesWhenGrantIsRevoked(t *testing.T) {
	principal := access.Principal{
		GrantID: "grant-revoked", CatalogID: "catalog-1", DeviceID: "iphone-1",
		Scopes: []library.DeviceScope{library.DeviceScopeMusicRead},
	}
	authenticator := &mutableSyncAuthenticator{principal: principal}
	api, err := NewSyncEventAPI(SyncEventConfig{
		Events: appevents.NewInMemoryBus(), Revalidator: authenticator,
		KeepaliveInterval: time.Hour, AuthCheckInterval: 5 * time.Millisecond,
	})
	if err != nil {
		t.Fatal(err)
	}
	router, err := NewRouter(Config{
		Version: "test", CatalogID: "catalog-1", Authenticator: authenticator, Pairer: &fakePairer{},
		AuthenticatedRoutes: api.Routes(),
	})
	if err != nil {
		t.Fatal(err)
	}
	server := httptest.NewServer(router)
	defer server.Close()
	request, _ := http.NewRequest(http.MethodGet, server.URL+"/api/v1/sync/events", nil)
	request.Header.Set("Authorization", "Bearer revoked-token")
	response, err := server.Client().Do(request)
	if err != nil {
		t.Fatal(err)
	}
	defer response.Body.Close()

	authenticator.revoke()
	time.Sleep(10 * time.Millisecond)
	api.hub.publishStation(SyncStationPosition{Station: "music", Epoch: strings.Repeat("b", 32), HighWater: 8})
	type readResult struct {
		body string
		err  error
	}
	result := make(chan readResult, 1)
	go func() {
		buffer := make([]byte, 1024)
		count, readErr := response.Body.Read(buffer)
		result <- readResult{body: string(buffer[:count]), err: readErr}
	}()
	select {
	case read := <-result:
		if read.err == nil || strings.Contains(read.body, "station-dirty") || strings.Contains(read.body, "revoked-token") {
			t.Fatalf("revoked stream read = body %q error %v", read.body, read.err)
		}
	case <-time.After(time.Second):
		t.Fatal("revoked active grant did not close its SSE stream")
	}
}

func TestSyncEventsKeepaliveAndInvalidLastEventID(t *testing.T) {
	principal := access.Principal{
		GrantID: "grant-1", CatalogID: "catalog-1", Scopes: []library.DeviceScope{library.DeviceScopeLibraryRead},
	}
	authenticator := &fakeAuthenticator{principal: principal}
	api, err := NewSyncEventAPI(SyncEventConfig{
		Events: appevents.NewInMemoryBus(), Revalidator: authenticator, KeepaliveInterval: 5 * time.Millisecond,
	})
	if err != nil {
		t.Fatal(err)
	}

	invalidRequest := httptest.NewRequest(http.MethodGet, "/api/v1/sync/events", nil)
	invalidRequest.Header.Set("Authorization", "Bearer token")
	invalidRequest.Header.Set("Last-Event-ID", "future-secret")
	invalidRequest = invalidRequest.WithContext(context.WithValue(invalidRequest.Context(), principalContextKey{}, principal))
	invalidResponse := httptest.NewRecorder()
	api.events(invalidResponse, invalidRequest)
	if invalidResponse.Code != http.StatusBadRequest || !strings.Contains(invalidResponse.Body.String(), "invalid_last_event_id") {
		t.Fatalf("invalid Last-Event-ID response = %d %s", invalidResponse.Code, invalidResponse.Body.String())
	}

	router, err := NewRouter(Config{
		Version: "test", CatalogID: "catalog-1",
		Authenticator: authenticator, Pairer: &fakePairer{},
		AuthenticatedRoutes: api.Routes(),
	})
	if err != nil {
		t.Fatal(err)
	}
	server := httptest.NewServer(router)
	defer server.Close()
	request, _ := http.NewRequest(http.MethodGet, server.URL+"/api/v1/sync/events", nil)
	request.Header.Set("Authorization", "Bearer token")
	response, err := server.Client().Do(request)
	if err != nil {
		t.Fatal(err)
	}
	defer response.Body.Close()
	if block := readSSEBlock(t, response.Body); block != ": keepalive\n\n" {
		t.Fatalf("keepalive block = %q", block)
	}
}

func TestSyncEventRunSamplesDurablePositionsAndTaskBus(t *testing.T) {
	bus := appevents.NewInMemoryBus()
	api, err := NewSyncEventAPI(SyncEventConfig{
		Events: bus, Revalidator: &fakeAuthenticator{}, PollInterval: 5 * time.Millisecond, ProbeTimeout: time.Second,
		LibraryProbe: func(context.Context) (SyncStationPosition, error) {
			return SyncStationPosition{Station: "library", Epoch: strings.Repeat("a", 32), HighWater: 17}, nil
		},
		MusicProbe: func(context.Context) (SyncStationPosition, error) {
			return SyncStationPosition{Station: "music", Epoch: strings.Repeat("b", 32), HighWater: 23}, nil
		},
		RSSProbe: func(context.Context) (SyncStationPosition, error) {
			return SyncStationPosition{Station: "rss", Epoch: strings.Repeat("c", 32), HighWater: 31}, nil
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	subscription := api.hub.subscribe(access.Principal{Scopes: []library.DeviceScope{
		library.DeviceScopeLibraryRead, library.DeviceScopeMusicRead,
		library.DeviceScopeRSSRead, library.DeviceScopeTasksRead,
	}}, 0)
	defer subscription.close()
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	go func() {
		api.Run(ctx)
		close(done)
	}()

	deadline := time.Now().Add(time.Second)
	for time.Now().Before(deadline) {
		api.hub.mu.Lock()
		ready := len(api.hub.latest) == 3
		api.hub.mu.Unlock()
		if ready {
			break
		}
		time.Sleep(time.Millisecond)
	}
	if err := bus.Publish(context.Background(), appevents.Event{Topic: libraryOperationTopic, Payload: "must-not-cross-boundary"}); err != nil {
		t.Fatal(err)
	}
	api.hub.mu.Lock()
	if len(api.hub.latest) != 4 || api.hub.latest["tasks"].kind != "tasks-dirty" ||
		api.hub.latest["library"].position.HighWater != 17 || api.hub.latest["music"].position.HighWater != 23 ||
		api.hub.latest["rss"].position.HighWater != 31 {
		t.Fatalf("sampled hints = %#v", api.hub.latest)
	}
	api.hub.mu.Unlock()
	cancel()
	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("SyncEventAPI.Run did not stop after cancellation")
	}
}

func readSSEBlock(t *testing.T, body io.Reader) string {
	t.Helper()
	reader := bufio.NewReader(body)
	var result strings.Builder
	done := make(chan error, 1)
	go func() {
		for {
			line, err := reader.ReadString('\n')
			result.WriteString(line)
			if err != nil || line == "\n" {
				done <- err
				return
			}
		}
	}()
	select {
	case err := <-done:
		if err != nil {
			t.Fatalf("read SSE block: %v", err)
		}
	case <-time.After(time.Second):
		t.Fatal("timed out reading SSE block")
	}
	return result.String()
}
