package libraryapi

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	appevents "xiadown/internal/application/events"
	"xiadown/internal/application/library/access"
	"xiadown/internal/domain/library"
)

// SyncEventsCapability is advertised by device-access when the shared,
// foreground-only realtime signal plane is mounted. The stream is deliberately
// only a wake-up hint: Station overview/changes remain the correctness boundary.
const SyncEventsCapability = "sync-events-v1"

const (
	defaultSyncEventPollInterval      = 2 * time.Second
	defaultSyncEventKeepaliveInterval = 15 * time.Second
	defaultSyncEventAuthCheckInterval = 15 * time.Second
	defaultSyncEventProbeTimeout      = 3 * time.Second
	defaultSyncEventHistoryLimit      = 256
	maxSyncEventHistoryLimit          = 4096
	maxSyncEventEpochLength           = 128
	maxLastEventIDLength              = 20
	libraryOperationTopic             = "library.operation"
)

// SyncStationPosition is the complete data vocabulary allowed in a
// station-dirty event. It intentionally has no entity, title, URL, path,
// credential, or error fields.
type SyncStationPosition struct {
	Station   string `json:"station"`
	Epoch     string `json:"epoch"`
	HighWater int64  `json:"highWater"`
}

// SyncPositionProbe reads one Station's durable synchronization position.
// Probe failures suppress a hint and are retried on the next interval; they do
// not alter the Station's durable synchronization contract.
type SyncPositionProbe func(context.Context) (SyncStationPosition, error)

type SyncEventConfig struct {
	LibraryProbe SyncPositionProbe
	MusicProbe   SyncPositionProbe
	RSSProbe     SyncPositionProbe
	Events       appevents.Bus
	// Revalidator checks the live grant behind an already-open stream. The raw
	// Bearer value is retained only inside the handler and is never published,
	// logged, or stored in the hub.
	Revalidator       Authenticator
	PollInterval      time.Duration
	KeepaliveInterval time.Duration
	AuthCheckInterval time.Duration
	ProbeTimeout      time.Duration
	HistoryLimit      int
}

// SyncEventAPI owns a process-local, bounded hint hub. It never persists
// cursor state and cannot be used to acknowledge or apply synchronization.
type SyncEventAPI struct {
	config SyncEventConfig
	hub    *syncEventHub
	wake   chan struct{}
}

func NewSyncEventAPI(config SyncEventConfig) (*SyncEventAPI, error) {
	if config.Revalidator == nil {
		return nil, errors.New("sync event API requires live grant revalidation")
	}
	if config.LibraryProbe == nil && config.MusicProbe == nil && config.RSSProbe == nil && config.Events == nil {
		return nil, errors.New("sync event API requires at least one mounted signal source")
	}
	if config.PollInterval <= 0 {
		config.PollInterval = defaultSyncEventPollInterval
	}
	if config.KeepaliveInterval <= 0 {
		config.KeepaliveInterval = defaultSyncEventKeepaliveInterval
	}
	if config.AuthCheckInterval <= 0 {
		config.AuthCheckInterval = defaultSyncEventAuthCheckInterval
	}
	if config.ProbeTimeout <= 0 {
		config.ProbeTimeout = defaultSyncEventProbeTimeout
	}
	if config.HistoryLimit <= 0 {
		config.HistoryLimit = defaultSyncEventHistoryLimit
	} else if config.HistoryLimit > maxSyncEventHistoryLimit {
		config.HistoryLimit = maxSyncEventHistoryLimit
	}
	return &SyncEventAPI{
		config: config, hub: newSyncEventHub(config.HistoryLimit), wake: make(chan struct{}, 1),
	}, nil
}

func (api *SyncEventAPI) Routes() []AuthenticatedRoute {
	if api == nil {
		return nil
	}
	return []AuthenticatedRoute{{
		Method:  http.MethodGet,
		Path:    "/api/v1/sync/events",
		Handler: http.HandlerFunc(api.events),
	}}
}

// Run samples all mounted durable high-water sources from one central loop and
// converts the existing task event topic into an opaque dirty hint. It returns
// only after ctx is cancelled and always unregisters the task subscription.
func (api *SyncEventAPI) Run(ctx context.Context) {
	if api == nil || api.hub == nil {
		return
	}
	var unsubscribe func()
	if api.config.Events != nil {
		unsubscribe = api.config.Events.Subscribe(libraryOperationTopic, func(appevents.Event) {
			api.hub.publishTasksDirty()
		})
		defer unsubscribe()
	}

	ticker := time.NewTicker(api.config.PollInterval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-api.wake:
			if api.hub.hasSubscribers() {
				api.sample(ctx)
			}
		case <-ticker.C:
			if api.hub.hasSubscribers() {
				api.sample(ctx)
			}
		}
	}
}

func (api *SyncEventAPI) sample(ctx context.Context) {
	probes := []SyncPositionProbe{api.config.LibraryProbe, api.config.MusicProbe, api.config.RSSProbe}
	var wait sync.WaitGroup
	for _, probe := range probes {
		if probe == nil {
			continue
		}
		wait.Add(1)
		go func(probe SyncPositionProbe) {
			defer wait.Done()
			probeCtx, cancel := context.WithTimeout(ctx, api.config.ProbeTimeout)
			defer cancel()
			if position, err := probe(probeCtx); err == nil {
				api.hub.publishStation(position)
			}
		}(probe)
	}
	wait.Wait()
}

func (api *SyncEventAPI) events(w http.ResponseWriter, request *http.Request) {
	principal, ok := PrincipalFromContext(request.Context())
	if !ok || strings.TrimSpace(principal.GrantID) == "" {
		unauthorized(w)
		return
	}
	token, ok := bearerToken(request.Header.Get("Authorization"))
	if !ok {
		unauthorized(w)
		return
	}
	lastID, err := parseLastEventID(request.Header.Get("Last-Event-ID"))
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid_last_event_id")
		return
	}
	flusher, ok := w.(http.Flusher)
	if !ok {
		writeError(w, http.StatusInternalServerError, "streaming_unavailable")
		return
	}

	subscription := api.hub.subscribe(principal, lastID)
	defer subscription.close()
	authorization := syncStreamAuthorization{
		authenticator: api.config.Revalidator,
		token:         token,
		principal:     principal,
		checkedAt:     time.Now(),
		interval:      api.config.AuthCheckInterval,
	}
	select {
	case api.wake <- struct{}{}:
	default:
	}
	w.Header().Set("Content-Type", "text/event-stream; charset=utf-8")
	w.Header().Set("Cache-Control", "no-store")
	w.Header().Set("Connection", "keep-alive")
	w.Header().Set("X-Accel-Buffering", "no")
	w.Header().Add("Vary", "Authorization")
	w.WriteHeader(http.StatusOK)
	flusher.Flush()

	keepalive := time.NewTicker(api.config.KeepaliveInterval)
	defer keepalive.Stop()
	for {
		select {
		case <-request.Context().Done():
			return
		case <-keepalive.C:
			refreshed, valid := authorization.refresh(request.Context())
			if !valid {
				return
			}
			api.hub.updateSubscriptionScopes(subscription, refreshed.Scopes)
			if _, err := fmt.Fprint(w, ": keepalive\n\n"); err != nil {
				return
			}
			flusher.Flush()
		case <-subscription.ready:
			refreshed, valid := authorization.refresh(request.Context())
			if !valid {
				return
			}
			api.hub.updateSubscriptionScopes(subscription, refreshed.Scopes)
			event, exists := subscription.pop()
			if !exists {
				continue
			}
			if err := writeSyncServerEvent(w, event); err != nil {
				return
			}
			flusher.Flush()
		}
	}
}

// syncStreamAuthorization deliberately has no exported or JSON-visible
// fields. It retains the credential only for periodic server-side validation
// and drops the stream silently when the grant is revoked, expired, moved to a
// different Catalog/device identity, or otherwise stops authenticating.
type syncStreamAuthorization struct {
	authenticator Authenticator
	token         string
	principal     access.Principal
	checkedAt     time.Time
	interval      time.Duration
}

func (authorization *syncStreamAuthorization) refresh(ctx context.Context) (access.Principal, bool) {
	if authorization == nil || authorization.authenticator == nil || authorization.token == "" {
		return access.Principal{}, false
	}
	if time.Since(authorization.checkedAt) < authorization.interval {
		return authorization.principal, true
	}
	principal, err := authorization.authenticator.Authenticate(ctx, authorization.token)
	if err != nil || principal.GrantID != authorization.principal.GrantID ||
		principal.CatalogID != authorization.principal.CatalogID ||
		principal.DeviceID != authorization.principal.DeviceID {
		return access.Principal{}, false
	}
	authorization.principal = principal
	authorization.checkedAt = time.Now()
	return principal, true
}

func parseLastEventID(raw string) (uint64, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return 0, nil
	}
	if len(raw) > maxLastEventIDLength {
		return 0, errors.New("last event id is too long")
	}
	value, err := strconv.ParseUint(raw, 10, 64)
	if err != nil || value == 0 {
		return 0, errors.New("invalid last event id")
	}
	return value, nil
}

type syncServerEvent struct {
	id       uint64
	kind     string
	position SyncStationPosition
}

func (event syncServerEvent) key() string {
	if event.kind == "tasks-dirty" {
		return "tasks"
	}
	return event.position.Station
}

func writeSyncServerEvent(w http.ResponseWriter, event syncServerEvent) error {
	var payload []byte
	var err error
	switch event.kind {
	case "station-dirty":
		payload, err = json.Marshal(event.position)
	case "tasks-dirty":
		payload = []byte(`{"dirty":true}`)
	default:
		return errors.New("unknown sync server event")
	}
	if err != nil {
		return err
	}
	_, err = fmt.Fprintf(w, "id: %d\nevent: %s\ndata: %s\n\n", event.id, event.kind, payload)
	return err
}

type syncEventHub struct {
	mu           sync.Mutex
	nextID       uint64
	historyLimit int
	history      []syncServerEvent
	latest       map[string]syncServerEvent
	subscribers  map[*syncEventSubscription]struct{}
}

func newSyncEventHub(historyLimit int) *syncEventHub {
	return &syncEventHub{
		historyLimit: historyLimit,
		latest:       make(map[string]syncServerEvent, 4),
		subscribers:  make(map[*syncEventSubscription]struct{}),
	}
}

func (hub *syncEventHub) hasSubscribers() bool {
	hub.mu.Lock()
	defer hub.mu.Unlock()
	return len(hub.subscribers) > 0
}

func (hub *syncEventHub) publishStation(position SyncStationPosition) {
	position.Station = strings.TrimSpace(position.Station)
	position.Epoch = strings.TrimSpace(position.Epoch)
	if !validSyncEventStation(position.Station) || position.Epoch == "" ||
		len(position.Epoch) > maxSyncEventEpochLength || position.HighWater < 0 {
		return
	}
	hub.mu.Lock()
	defer hub.mu.Unlock()
	previous, exists := hub.latest[position.Station]
	if exists && previous.kind == "station-dirty" && previous.position == position {
		return
	}
	hub.publishLocked(syncServerEvent{kind: "station-dirty", position: position})
}

func (hub *syncEventHub) publishTasksDirty() {
	hub.mu.Lock()
	defer hub.mu.Unlock()
	hub.publishLocked(syncServerEvent{kind: "tasks-dirty"})
}

func (hub *syncEventHub) publishLocked(event syncServerEvent) {
	hub.nextID++
	event.id = hub.nextID
	hub.latest[event.key()] = event
	hub.history = append(hub.history, event)
	if overflow := len(hub.history) - hub.historyLimit; overflow > 0 {
		copy(hub.history, hub.history[overflow:])
		hub.history = hub.history[:hub.historyLimit]
	}
	for subscription := range hub.subscribers {
		if subscription.allows(event) {
			subscription.enqueue(event)
		}
	}
}

func (hub *syncEventHub) subscribe(principal access.Principal, lastID uint64) *syncEventSubscription {
	subscription := &syncEventSubscription{
		hub: hub, scopes: scopeSet(principal.Scopes), ready: make(chan struct{}, 1),
		pending: make(map[string]syncServerEvent, 4),
	}
	hub.mu.Lock()
	hub.subscribers[subscription] = struct{}{}
	gap := lastID == 0 || lastID > hub.nextID
	if lastID > 0 && len(hub.history) > 0 && lastID < hub.history[0].id-1 {
		gap = true
	}
	if gap {
		latest := make([]syncServerEvent, 0, len(hub.latest))
		for _, event := range hub.latest {
			latest = append(latest, event)
		}
		sort.Slice(latest, func(left, right int) bool { return latest[left].id < latest[right].id })
		for _, event := range latest {
			if subscription.allows(event) {
				subscription.enqueue(event)
			}
		}
	} else {
		for _, event := range hub.history {
			if event.id > lastID && subscription.allows(event) {
				subscription.enqueue(event)
			}
		}
	}
	hub.mu.Unlock()
	return subscription
}

// updateSubscriptionScopes applies a live grant change under the same lock
// order used by publish/close. Revoked pending hints are removed before the
// handler can pop them. Newly granted Stations receive the hub's current hint,
// while unchanged scopes do not cause duplicate wakeups.
func (hub *syncEventHub) updateSubscriptionScopes(
	subscription *syncEventSubscription,
	scopes []library.DeviceScope,
) {
	if hub == nil || subscription == nil {
		return
	}
	next := scopeSet(scopes)
	hub.mu.Lock()
	if _, exists := hub.subscribers[subscription]; !exists {
		hub.mu.Unlock()
		return
	}
	subscription.mu.Lock()
	if subscription.closed || scopeSetsEqual(subscription.scopes, next) {
		subscription.mu.Unlock()
		hub.mu.Unlock()
		return
	}
	previous := subscription.scopes
	subscription.scopes = next
	for key, event := range subscription.pending {
		if !scopeSetAllows(next, event) {
			delete(subscription.pending, key)
		}
	}
	added := false
	for key, event := range hub.latest {
		if scopeSetAllows(next, event) && !scopeSetAllows(previous, event) {
			subscription.pending[key] = event
			added = true
		}
	}
	subscription.mu.Unlock()
	hub.mu.Unlock()
	if added {
		select {
		case subscription.ready <- struct{}{}:
		default:
		}
	}
}

func validSyncEventStation(station string) bool {
	return station == "library" || station == "music" || station == "rss"
}

func scopeSet(scopes []library.DeviceScope) map[library.DeviceScope]struct{} {
	result := make(map[library.DeviceScope]struct{}, len(scopes))
	for _, scope := range scopes {
		result[scope] = struct{}{}
	}
	return result
}

func scopeSetsEqual(left, right map[library.DeviceScope]struct{}) bool {
	if len(left) != len(right) {
		return false
	}
	for scope := range left {
		if _, exists := right[scope]; !exists {
			return false
		}
	}
	return true
}

func scopeSetAllows(scopes map[library.DeviceScope]struct{}, event syncServerEvent) bool {
	required := library.DeviceScopeTasksRead
	switch event.position.Station {
	case "library":
		required = library.DeviceScopeLibraryRead
	case "music":
		required = library.DeviceScopeMusicRead
	case "rss":
		required = library.DeviceScopeRSSRead
	}
	_, allowed := scopes[required]
	return allowed
}

type syncEventSubscription struct {
	hub     *syncEventHub
	scopes  map[library.DeviceScope]struct{}
	ready   chan struct{}
	mu      sync.Mutex
	pending map[string]syncServerEvent
	closed  bool
}

func (subscription *syncEventSubscription) allows(event syncServerEvent) bool {
	subscription.mu.Lock()
	defer subscription.mu.Unlock()
	return !subscription.closed && scopeSetAllows(subscription.scopes, event)
}

// enqueue replaces an older pending event for the same Station. The pending
// map is therefore bounded by library/music/rss/tasks even if a client never
// reads, while the publisher remains non-blocking with respect to network IO.
func (subscription *syncEventSubscription) enqueue(event syncServerEvent) {
	subscription.mu.Lock()
	if subscription.closed {
		subscription.mu.Unlock()
		return
	}
	subscription.pending[event.key()] = event
	subscription.mu.Unlock()
	select {
	case subscription.ready <- struct{}{}:
	default:
	}
}

func (subscription *syncEventSubscription) pop() (syncServerEvent, bool) {
	subscription.mu.Lock()
	defer subscription.mu.Unlock()
	var selected syncServerEvent
	selectedKey := ""
	for key, event := range subscription.pending {
		if selectedKey == "" || event.id < selected.id {
			selected, selectedKey = event, key
		}
	}
	if selectedKey == "" {
		return syncServerEvent{}, false
	}
	delete(subscription.pending, selectedKey)
	if len(subscription.pending) > 0 {
		select {
		case subscription.ready <- struct{}{}:
		default:
		}
	}
	return selected, true
}

func (subscription *syncEventSubscription) close() {
	if subscription == nil || subscription.hub == nil {
		return
	}
	subscription.hub.mu.Lock()
	delete(subscription.hub.subscribers, subscription)
	subscription.hub.mu.Unlock()
	subscription.mu.Lock()
	subscription.closed = true
	clear(subscription.pending)
	subscription.mu.Unlock()
}
