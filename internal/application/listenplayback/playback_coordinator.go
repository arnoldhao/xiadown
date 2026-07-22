package listenplayback

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"sync"
	"time"
)

const playbackCompensationTimeout = 3 * time.Second
const maxPlaybackRollbackPoints = 32

// PlaybackSnapshotListener observes committed coordinator state. Callbacks are
// invoked synchronously after the coordinator releases its internal lock.
type PlaybackSnapshotListener func(PlaybackSnapshot)

type coordinatorSession struct {
	snapshot       PlaybackSessionSnapshot
	backend        PlaybackBackend
	needsReload    bool
	resumeEligible bool
	resumePolicy   PreviewResumePolicy
}

type playbackNotification struct {
	snapshot  PlaybackSnapshot
	listeners []PlaybackSnapshotListener
}

type playbackRollbackPoint struct {
	active    *coordinatorSession
	suspended *coordinatorSession
}

// PlaybackCoordinator owns the application's single audible playback slot.
// Persistent station playback can be suspended by one transient preview and is
// restored according to the preview's resume policy.
type PlaybackCoordinator struct {
	opMu sync.Mutex
	mu   sync.RWMutex

	backends map[PlaybackProvider]PlaybackBackend

	active               *coordinatorSession
	suspendedPersistent  *coordinatorSession
	version              uint64
	nextSessionID        uint64
	nextSubscriberID     uint64
	subscribers          map[uint64]PlaybackSnapshotListener
	pendingNotifications []playbackNotification
	rollbackPoints       map[string]playbackRollbackPoint
	rollbackOrder        []string
}

// NewPlaybackCoordinator constructs a coordinator and rejects ambiguous
// duplicate providers.
func NewPlaybackCoordinator(backends ...PlaybackBackend) (*PlaybackCoordinator, error) {
	coordinator := &PlaybackCoordinator{
		backends:       make(map[PlaybackProvider]PlaybackBackend),
		subscribers:    make(map[uint64]PlaybackSnapshotListener),
		rollbackPoints: make(map[string]playbackRollbackPoint),
	}
	for _, backend := range backends {
		if err := coordinator.RegisterBackend(backend); err != nil {
			return nil, err
		}
	}
	return coordinator, nil
}

// RegisterBackend makes one provider implementation available to the
// coordinator. Existing registrations are never silently replaced.
func (coordinator *PlaybackCoordinator) RegisterBackend(backend PlaybackBackend) error {
	if backend == nil {
		return fmt.Errorf("register playback backend: backend is nil")
	}
	provider := backend.Provider()
	if provider == "" {
		return fmt.Errorf("register playback backend: provider is required")
	}
	coordinator.mu.Lock()
	defer coordinator.mu.Unlock()
	if _, exists := coordinator.backends[provider]; exists {
		return fmt.Errorf("register playback backend: provider %q already registered", provider)
	}
	coordinator.backends[provider] = backend
	return nil
}

// StartPersistent starts station playback that can outlive workspace changes.
func (coordinator *PlaybackCoordinator) StartPersistent(
	ctx context.Context,
	item MediaItem,
) (PlaybackSnapshot, error) {
	return coordinator.StartSession(ctx, PlaybackSessionRequest{
		Focus: PlaybackFocusPersistent,
		Item:  item,
	})
}

// StartTransientPreview starts a preview that temporarily owns audio focus.
func (coordinator *PlaybackCoordinator) StartTransientPreview(
	ctx context.Context,
	item MediaItem,
	policy PreviewResumePolicy,
) (PlaybackSnapshot, error) {
	return coordinator.StartSession(ctx, PlaybackSessionRequest{
		Focus:               PlaybackFocusTransientPreview,
		Item:                item,
		PreviewResumePolicy: policy,
	})
}

// StartSession pauses any active backend before starting the requested one.
// State is committed only after Start succeeds, so a failed handoff restores
// the previously audible session whenever possible.
func (coordinator *PlaybackCoordinator) StartSession(
	ctx context.Context,
	request PlaybackSessionRequest,
) (PlaybackSnapshot, error) {
	coordinator.opMu.Lock()
	defer coordinator.finishOperation()

	item, err := normalizeMediaItem(request.Item)
	if err != nil {
		return coordinator.Snapshot(), err
	}
	if request.Focus != PlaybackFocusPersistent && request.Focus != PlaybackFocusTransientPreview {
		return coordinator.Snapshot(), fmt.Errorf("invalid playback focus %q", request.Focus)
	}
	if request.Focus == PlaybackFocusTransientPreview && request.PreviewResumePolicy == "" {
		request.PreviewResumePolicy = PreviewResumeIfPreviouslyPlaying
	}
	if request.Focus == PlaybackFocusTransientPreview &&
		request.PreviewResumePolicy != PreviewResumeIfPreviouslyPlaying &&
		request.PreviewResumePolicy != PreviewKeepPersistentPaused {
		return coordinator.Snapshot(), fmt.Errorf("invalid preview resume policy %q", request.PreviewResumePolicy)
	}

	backend, err := coordinator.backendFor(item.Source.Provider)
	if err != nil {
		return coordinator.Snapshot(), err
	}
	capabilities := backend.Capabilities()
	if err := validateStartCapabilities(item, capabilities); err != nil {
		return coordinator.Snapshot(), err
	}
	active, suspended := coordinator.currentSessions()
	var rollbackPoint *playbackRollbackPoint
	if request.RetainRollback {
		rollbackPoint = &playbackRollbackPoint{
			active:    cloneCoordinatorSession(active),
			suspended: cloneCoordinatorSession(suspended),
		}
	}
	wasAudible := active != nil && playbackStateMayBeAudible(active.snapshot.State)
	pausedActive := false
	if active != nil {
		if !active.snapshot.Capabilities.PlayPause {
			return coordinator.Snapshot(), unsupportedCapabilityError(
				active.backend.Provider(),
				active.snapshot.Capabilities,
				"pause",
			)
		}
		if err := active.backend.Pause(ctx); err != nil {
			return coordinator.Snapshot(), fmt.Errorf("pause active playback session %q: %w", active.snapshot.ID, err)
		}
		pausedActive = true
	}

	volume := 1.0
	muted := request.Muted
	if request.Volume != nil {
		volume = clampVolume(*request.Volume)
	} else if active != nil {
		// Volume is global transport state. A caller that does not override it
		// must not make a provider or track handoff jump back to full volume.
		volume = active.snapshot.Volume
		muted = muted || active.snapshot.Muted
	}
	muted = muted || volume <= 0
	sessionID := request.SessionID
	if sessionID == "" {
		sessionID = coordinator.newSessionID(request.Focus)
	}
	startRequest := PlaybackStartRequest{
		SessionID:    sessionID,
		Item:         item,
		StartSeconds: clampSeconds(request.StartSeconds),
		Volume:       volume,
		Muted:        muted,
		ForceReload:  request.ForceReload,
	}
	if err := backend.Start(ctx, startRequest); err != nil {
		rollbackErr := coordinator.rollbackStartFailure(ctx, active, suspended, backend, wasAudible, pausedActive)
		if rollbackErr != nil {
			return coordinator.Snapshot(), errors.Join(
				fmt.Errorf("start playback session %q: %w", sessionID, err),
				fmt.Errorf("restore previous playback session: %w", rollbackErr),
			)
		}
		return coordinator.Snapshot(), fmt.Errorf("start playback session %q: %w", sessionID, err)
	}
	next := &coordinatorSession{
		snapshot: PlaybackSessionSnapshot{
			ID:           sessionID,
			Focus:        request.Focus,
			State:        PlaybackStatePlaying,
			Item:         item,
			Capabilities: capabilities,
			Position:     clampSeconds(request.StartSeconds),
			Duration:     item.Duration,
			Volume:       volume,
			Muted:        muted,
			Queue:        []MediaItem{item},
			CurrentIndex: 0,
			RepeatMode:   RepeatModeOff,
		},
		backend:      backend,
		resumePolicy: request.PreviewResumePolicy,
	}

	var nextSuspended *coordinatorSession
	retainsActive := false
	if request.Focus == PlaybackFocusTransientPreview {
		switch {
		case active != nil && active.snapshot.Focus == PlaybackFocusPersistent:
			retainsActive = true
			nextSuspended = cloneCoordinatorSession(active)
			nextSuspended.snapshot.State = PlaybackStatePaused
			nextSuspended.resumeEligible = wasAudible
		case active != nil && active.snapshot.Focus == PlaybackFocusTransientPreview:
			nextSuspended = cloneCoordinatorSession(suspended)
		default:
			nextSuspended = nil
		}
		if nextSuspended != nil && nextSuspended.backend.Provider() == backend.Provider() {
			nextSuspended.needsReload = true
		}
	}
	if active != nil && !retainsActive && active.backend.Provider() != backend.Provider() {
		// The handoff has succeeded and this session is no longer represented by
		// either active or suspended state. Release cross-provider media resources
		// (notably local file handles) without risking the newly started backend.
		cleanupCtx, cleanupCancel := playbackCompensationContext(ctx)
		_ = stopBackend(cleanupCtx, active)
		cleanupCancel()
	}

	snapshot := coordinator.commit(next, nextSuspended)
	if rollbackPoint != nil {
		coordinator.rememberRollbackPoint(sessionID, *rollbackPoint)
	}
	return snapshot, nil
}

// AcceptSessionRollback commits a reversible handoff and drops its retained
// predecessor chain. It is idempotent so UI acknowledgements can be retried.
func (coordinator *PlaybackCoordinator) AcceptSessionRollback(sessionID string) PlaybackSnapshot {
	coordinator.opMu.Lock()
	defer coordinator.finishOperation()
	coordinator.forgetRollbackChain(strings.TrimSpace(sessionID))
	return coordinator.Snapshot()
}

// RollbackSession restores the exact active and suspended coordinator state
// replaced by a reversible StartSession. The active-session comparison and
// restore happen under the coordinator operation lock, so a later provider
// handoff is never overwritten by a stale cancellation.
func (coordinator *PlaybackCoordinator) RollbackSession(
	ctx context.Context,
	sessionID string,
) (PlaybackSnapshot, error) {
	if ctx == nil {
		ctx = context.Background()
	}
	coordinator.opMu.Lock()
	defer coordinator.finishOperation()
	sessionID = strings.TrimSpace(sessionID)
	active, _ := coordinator.currentSessions()
	if active == nil || active.snapshot.ID != sessionID {
		return coordinator.Snapshot(), fmt.Errorf(
			"%w: expected %q",
			ErrPlaybackSessionNotActive,
			sessionID,
		)
	}
	point, ok := coordinator.rollbackPoints[sessionID]
	if !ok {
		return coordinator.Snapshot(), ErrPlaybackRollbackUnavailable
	}
	if err := stopBackend(ctx, active); err != nil {
		return coordinator.Snapshot(), fmt.Errorf("stop canceled playback session %q: %w", sessionID, err)
	}
	delete(coordinator.rollbackPoints, sessionID)
	restoredActive := cloneCoordinatorSession(point.active)
	restoredSuspended := cloneCoordinatorSession(point.suspended)
	if restoredActive == nil {
		return coordinator.commit(nil, nil), nil
	}

	wasAudible := playbackStateMayBeAudible(restoredActive.snapshot.State)
	restoredActive.needsReload = true
	if wasAudible {
		if err := resumeCoordinatorSession(ctx, restoredActive); err != nil {
			restoredActive.snapshot.State = PlaybackStatePaused
			restoredActive.snapshot.ErrorMessage = err.Error()
			restoredActive.needsReload = true
			snapshot := coordinator.commit(restoredActive, restoredSuspended)
			return snapshot, fmt.Errorf("restore canceled playback session %q: %w", sessionID, err)
		}
		restoredActive.needsReload = false
	}
	if restoredSuspended != nil {
		suspendedProvider := restoredSuspended.backend.Provider()
		if suspendedProvider == restoredActive.backend.Provider() ||
			suspendedProvider == active.backend.Provider() {
			restoredSuspended.needsReload = true
		}
	}
	return coordinator.commit(restoredActive, restoredSuspended), nil
}

// CloseSession closes the active session. Closing a transient preview returns
// focus to its suspended persistent session and optionally resumes it.
func (coordinator *PlaybackCoordinator) CloseSession(
	ctx context.Context,
	sessionID string,
) (PlaybackSnapshot, error) {
	coordinator.opMu.Lock()
	defer coordinator.finishOperation()

	active, suspended := coordinator.currentSessions()
	if active == nil || active.snapshot.ID != sessionID {
		return coordinator.Snapshot(), fmt.Errorf("%w: %q", ErrPlaybackSessionNotActive, sessionID)
	}
	if err := stopBackend(ctx, active); err != nil {
		return coordinator.Snapshot(), fmt.Errorf("close playback session %q: %w", sessionID, err)
	}
	if active.snapshot.Focus == PlaybackFocusPersistent || suspended == nil {
		return coordinator.commit(nil, nil), nil
	}

	restored := cloneCoordinatorSession(suspended)
	restored.snapshot.State = PlaybackStatePaused
	if active.backend.Provider() == restored.backend.Provider() {
		restored.needsReload = true
	}
	shouldResume := active.resumePolicy == PreviewResumeIfPreviouslyPlaying && restored.resumeEligible
	if !shouldResume {
		return coordinator.commit(restored, nil), nil
	}

	if err := resumeCoordinatorSession(ctx, restored); err != nil {
		snapshot := coordinator.commit(restored, nil)
		return snapshot, fmt.Errorf("resume persistent playback session %q: %w", restored.snapshot.ID, err)
	}
	restored.snapshot.State = PlaybackStatePlaying
	restored.needsReload = false
	return coordinator.commit(restored, nil), nil
}

// Play resumes the active session, reloading it first when a same-provider
// preview replaced the backend's loaded item.
func (coordinator *PlaybackCoordinator) Play(ctx context.Context) (PlaybackSnapshot, error) {
	coordinator.opMu.Lock()
	defer coordinator.finishOperation()
	active, suspended := coordinator.currentSessions()
	if active == nil {
		return coordinator.Snapshot(), ErrPlaybackSessionNotActive
	}
	if active.snapshot.State == PlaybackStatePlaying && !active.needsReload {
		return coordinator.Snapshot(), nil
	}
	if !active.snapshot.Capabilities.PlayPause {
		return coordinator.Snapshot(), unsupportedCapabilityError(
			active.backend.Provider(), active.snapshot.Capabilities, "play",
		)
	}
	if err := resumeCoordinatorSession(ctx, active); err != nil {
		return coordinator.Snapshot(), fmt.Errorf("resume playback session %q: %w", active.snapshot.ID, err)
	}
	active.snapshot.State = PlaybackStatePlaying
	active.snapshot.ErrorMessage = ""
	active.needsReload = false
	return coordinator.commit(active, suspended), nil
}

// Pause pauses the active session without closing its focus scope.
func (coordinator *PlaybackCoordinator) Pause(ctx context.Context) (PlaybackSnapshot, error) {
	coordinator.opMu.Lock()
	defer coordinator.finishOperation()
	active, suspended := coordinator.currentSessions()
	if active == nil {
		return coordinator.Snapshot(), ErrPlaybackSessionNotActive
	}
	if active.snapshot.State == PlaybackStatePaused {
		return coordinator.Snapshot(), nil
	}
	if !active.snapshot.Capabilities.PlayPause {
		return coordinator.Snapshot(), unsupportedCapabilityError(
			active.backend.Provider(), active.snapshot.Capabilities, "pause",
		)
	}
	if err := active.backend.Pause(ctx); err != nil {
		return coordinator.Snapshot(), fmt.Errorf("pause playback session %q: %w", active.snapshot.ID, err)
	}
	active.snapshot.State = PlaybackStatePaused
	return coordinator.commit(active, suspended), nil
}

// Stop stops the active transport while retaining the session metadata. A
// later Play reloads the same item from the beginning, which is important for
// HTMLMediaElement transports whose stop operation releases the media source.
func (coordinator *PlaybackCoordinator) Stop(ctx context.Context) (PlaybackSnapshot, error) {
	coordinator.opMu.Lock()
	defer coordinator.finishOperation()
	active, suspended := coordinator.currentSessions()
	if active == nil {
		return coordinator.Snapshot(), ErrPlaybackSessionNotActive
	}
	if !active.snapshot.Capabilities.Stop {
		return coordinator.Snapshot(), unsupportedCapabilityError(
			active.backend.Provider(), active.snapshot.Capabilities, "stop",
		)
	}
	if err := active.backend.Stop(ctx); err != nil {
		return coordinator.Snapshot(), fmt.Errorf("stop playback session %q: %w", active.snapshot.ID, err)
	}
	active.snapshot.State = PlaybackStateEnded
	active.snapshot.Position = 0
	active.needsReload = true
	return coordinator.commit(active, suspended), nil
}

// Seek updates the active session position. A session awaiting reload stores the
// desired position so its next Play starts at that location.
func (coordinator *PlaybackCoordinator) Seek(
	ctx context.Context,
	seconds float64,
) (PlaybackSnapshot, error) {
	coordinator.opMu.Lock()
	defer coordinator.finishOperation()
	active, suspended := coordinator.currentSessions()
	if active == nil {
		return coordinator.Snapshot(), ErrPlaybackSessionNotActive
	}
	if !active.snapshot.Capabilities.Seek {
		return coordinator.Snapshot(), unsupportedCapabilityError(
			active.backend.Provider(), active.snapshot.Capabilities, "seek",
		)
	}
	target := clampSeconds(seconds)
	if active.snapshot.Duration > 0 && target > active.snapshot.Duration {
		target = active.snapshot.Duration
	}
	if !active.needsReload {
		if err := active.backend.Seek(ctx, target); err != nil {
			return coordinator.Snapshot(), fmt.Errorf("seek playback session %q: %w", active.snapshot.ID, err)
		}
	}
	active.snapshot.Position = target
	return coordinator.commit(active, suspended), nil
}

// SetVolume updates both the active backend and global snapshot.
func (coordinator *PlaybackCoordinator) SetVolume(
	ctx context.Context,
	volume float64,
	muted bool,
) (PlaybackSnapshot, error) {
	coordinator.opMu.Lock()
	defer coordinator.finishOperation()
	active, suspended := coordinator.currentSessions()
	if active == nil {
		return coordinator.Snapshot(), ErrPlaybackSessionNotActive
	}
	if !active.snapshot.Capabilities.Volume {
		return coordinator.Snapshot(), unsupportedCapabilityError(
			active.backend.Provider(), active.snapshot.Capabilities, "volume",
		)
	}
	volume = clampVolume(volume)
	muted = muted || volume <= 0
	if err := active.backend.SetVolume(ctx, volume, muted); err != nil {
		return coordinator.Snapshot(), fmt.Errorf("set playback session %q volume: %w", active.snapshot.ID, err)
	}
	active.snapshot.Volume = volume
	active.snapshot.Muted = muted
	return coordinator.commit(active, suspended), nil
}

// Previous delegates provider queue navigation when supported.
func (coordinator *PlaybackCoordinator) Previous(ctx context.Context) error {
	return coordinator.navigate(ctx, false)
}

// Next delegates provider queue navigation when supported.
func (coordinator *PlaybackCoordinator) Next(ctx context.Context) error {
	return coordinator.navigate(ctx, true)
}

func (coordinator *PlaybackCoordinator) navigate(ctx context.Context, next bool) error {
	coordinator.opMu.Lock()
	defer coordinator.finishOperation()
	active, _ := coordinator.currentSessions()
	if active == nil {
		return ErrPlaybackSessionNotActive
	}
	if (next && !active.snapshot.Capabilities.Next) || (!next && !active.snapshot.Capabilities.Previous) {
		capability := "previous"
		if next {
			capability = "next"
		}
		return unsupportedCapabilityError(active.backend.Provider(), active.snapshot.Capabilities, capability)
	}
	if next {
		return active.backend.Next(ctx)
	}
	return active.backend.Previous(ctx)
}

// Snapshot returns an isolated copy safe for UI and subscriber use.
func (coordinator *PlaybackCoordinator) Snapshot() PlaybackSnapshot {
	coordinator.mu.RLock()
	defer coordinator.mu.RUnlock()
	return coordinator.snapshotLocked()
}

// Subscribe adds a global playback observer and returns an idempotent removal
// function. The current snapshot can be read separately with Snapshot.
func (coordinator *PlaybackCoordinator) Subscribe(listener PlaybackSnapshotListener) func() {
	if listener == nil {
		return func() {}
	}
	coordinator.mu.Lock()
	coordinator.nextSubscriberID++
	id := coordinator.nextSubscriberID
	coordinator.subscribers[id] = listener
	coordinator.mu.Unlock()
	var once sync.Once
	return func() {
		once.Do(func() {
			coordinator.mu.Lock()
			delete(coordinator.subscribers, id)
			coordinator.mu.Unlock()
		})
	}
}

// ObserveBackendEvent reconciles asynchronous state reported by a playback
// engine with the currently active coordinator session. Stale provider or
// session events are ignored, preventing a replaced local file from mutating
// the new playback snapshot.
func (coordinator *PlaybackCoordinator) ObserveBackendEvent(event PlaybackBackendEvent) PlaybackSnapshot {
	coordinator.opMu.Lock()
	defer coordinator.finishOperation()
	active, suspended := coordinator.currentSessions()
	if active == nil || event.Provider == "" || active.backend.Provider() != event.Provider {
		return coordinator.Snapshot()
	}
	if event.SessionID != "" && event.SessionID != active.snapshot.ID {
		return coordinator.Snapshot()
	}
	if isObservedPlaybackState(event.State) {
		active.snapshot.State = event.State
	}
	active.snapshot.ErrorMessage = ""
	if event.State == PlaybackStateError {
		active.snapshot.ErrorMessage = event.Error
	}
	if event.HasTiming {
		active.snapshot.Position = clampSeconds(event.Position)
		active.snapshot.Duration = clampSeconds(event.Duration)
		if active.snapshot.Duration > 0 && active.snapshot.Position > active.snapshot.Duration {
			active.snapshot.Position = active.snapshot.Duration
		}
	}
	if event.HasVolume {
		active.snapshot.Volume = clampVolume(event.Volume)
		active.snapshot.Muted = event.Muted || active.snapshot.Volume <= 0
	}
	if event.State == PlaybackStateEnded {
		if restored, ok := coordinator.restoreAfterEndedPreview(context.Background(), active, suspended); ok {
			return coordinator.commit(restored, nil)
		}
		active.needsReload = true
	}
	return coordinator.commit(active, suspended)
}

// ObserveBackendSnapshot reconciles a provider's richer legacy snapshot with
// a session that was started through this coordinator. It intentionally never
// adopts an out-of-band session: callers must acquire audio focus through
// StartSession first.
func (coordinator *PlaybackCoordinator) ObserveBackendSnapshot(
	provider PlaybackProvider,
	observed PlaybackSessionSnapshot,
) PlaybackSnapshot {
	coordinator.opMu.Lock()
	defer coordinator.finishOperation()
	active, suspended := coordinator.currentSessions()
	if active == nil || provider == "" || active.backend.Provider() != provider {
		return coordinator.Snapshot()
	}
	coordinator.reconcileObservedSession(active, observed)
	if observed.State == PlaybackStateEnded {
		if restored, ok := coordinator.restoreAfterEndedPreview(context.Background(), active, suspended); ok {
			return coordinator.commit(restored, nil)
		}
		active.needsReload = true
	}
	return coordinator.commit(active, suspended)
}

func (coordinator *PlaybackCoordinator) reconcileObservedSession(active *coordinatorSession, observed PlaybackSessionSnapshot) {
	if isObservedPlaybackState(observed.State) {
		active.snapshot.State = observed.State
	}
	if observed.State != PlaybackStateError {
		active.snapshot.ErrorMessage = ""
	} else {
		active.snapshot.ErrorMessage = observed.ErrorMessage
	}
	if observed.Item.Source.Provider == active.backend.Provider() {
		if item, err := normalizeMediaItem(observed.Item); err == nil {
			active.snapshot.Item = item
		}
	}
	active.snapshot.Position = clampSeconds(observed.Position)
	active.snapshot.Duration = clampSeconds(observed.Duration)
	if active.snapshot.Duration > 0 && active.snapshot.Position > active.snapshot.Duration {
		active.snapshot.Position = active.snapshot.Duration
	}
	active.snapshot.Volume = clampVolume(observed.Volume)
	active.snapshot.Muted = observed.Muted || active.snapshot.Volume <= 0
	active.snapshot.Queue = cloneMediaItems(observed.Queue)
	active.snapshot.CurrentIndex = observed.CurrentIndex
	if active.snapshot.CurrentIndex < 0 {
		active.snapshot.CurrentIndex = 0
	}
	if len(active.snapshot.Queue) > 0 && active.snapshot.CurrentIndex >= len(active.snapshot.Queue) {
		active.snapshot.CurrentIndex = len(active.snapshot.Queue) - 1
	}
	active.snapshot.ShuffleEnabled = observed.ShuffleEnabled
	active.snapshot.RepeatMode = observed.RepeatMode
}

// AdoptBackendSnapshot brings playback started by a legacy entry point under
// coordinator ownership. Only an audible observation may take focus from a
// different provider. If the old provider cannot be silenced, the newcomer is
// paused as rollback so the coordinator does not knowingly leave two audible
// backends running.
func (coordinator *PlaybackCoordinator) AdoptBackendSnapshot(
	ctx context.Context,
	provider PlaybackProvider,
	observed PlaybackSessionSnapshot,
) (PlaybackSnapshot, error) {
	coordinator.opMu.Lock()
	defer coordinator.finishOperation()
	return coordinator.adoptBackendSnapshotLocked(ctx, provider, observed)
}

// SynchronizeBackendSnapshot treats a legacy backend notification as a
// wake-up signal and reads the provider's current snapshot only after command
// serialization is acquired. A snapshot that waited behind a newer provider
// handoff therefore cannot replay stale state over that handoff.
func (coordinator *PlaybackCoordinator) SynchronizeBackendSnapshot(
	ctx context.Context,
	provider PlaybackProvider,
) (PlaybackSnapshot, error) {
	coordinator.opMu.Lock()
	defer coordinator.finishOperation()
	if provider == "" {
		return coordinator.Snapshot(), fmt.Errorf("synchronize playback snapshot: provider is required")
	}
	if ctx == nil {
		ctx = context.Background()
	} else if err := ctx.Err(); err != nil {
		return coordinator.Snapshot(), err
	}
	backend, err := coordinator.backendFor(provider)
	if err != nil {
		return coordinator.Snapshot(), err
	}
	snapshotBackend, ok := backend.(PlaybackSnapshotBackend)
	if !ok {
		return coordinator.Snapshot(), fmt.Errorf("synchronize playback snapshot: backend %q does not expose snapshots", provider)
	}
	observed := snapshotBackend.Snapshot(ctx)
	if observed.Active != nil {
		return coordinator.adoptBackendSnapshotLocked(ctx, provider, *observed.Active)
	}

	active, suspended := coordinator.currentSessions()
	if active == nil || active.backend.Provider() != provider {
		return coordinator.Snapshot(), nil
	}
	active.snapshot.State = PlaybackStateIdle
	active.snapshot.ErrorMessage = ""
	return coordinator.commit(active, suspended), nil
}

func (coordinator *PlaybackCoordinator) adoptBackendSnapshotLocked(
	ctx context.Context,
	provider PlaybackProvider,
	observed PlaybackSessionSnapshot,
) (PlaybackSnapshot, error) {
	if provider == "" {
		return coordinator.Snapshot(), fmt.Errorf("adopt playback snapshot: provider is required")
	}
	if ctx != nil {
		if err := ctx.Err(); err != nil {
			return coordinator.Snapshot(), err
		}
	}
	backend, err := coordinator.backendFor(provider)
	if err != nil {
		return coordinator.Snapshot(), err
	}
	item, err := normalizeMediaItem(observed.Item)
	if err != nil {
		return coordinator.Snapshot(), fmt.Errorf("adopt playback snapshot: %w", err)
	}
	if item.Source.Provider != provider {
		return coordinator.Snapshot(), fmt.Errorf(
			"adopt playback snapshot: item provider %q does not match %q",
			item.Source.Provider,
			provider,
		)
	}
	active, suspended := coordinator.currentSessions()
	observed.Item = item
	if active != nil && active.backend.Provider() == provider {
		coordinator.reconcileObservedSession(active, observed)
		if observed.State == PlaybackStateEnded {
			if restored, ok := coordinator.restoreAfterEndedPreview(ctx, active, suspended); ok {
				return coordinator.commit(restored, nil), nil
			}
			active.needsReload = true
		}
		return coordinator.commit(active, suspended), nil
	}
	if !playbackStateMayBeAudible(observed.State) {
		return coordinator.Snapshot(), nil
	}
	capabilities := backend.Capabilities()
	if err := validateStartCapabilities(item, capabilities); err != nil {
		return coordinator.Snapshot(), err
	}
	if active != nil && playbackStateMayBeAudible(active.snapshot.State) {
		if !active.snapshot.Capabilities.PlayPause {
			adoptErr := unsupportedCapabilityError(active.backend.Provider(), active.snapshot.Capabilities, "pause")
			return coordinator.Snapshot(), coordinator.rollbackAdoption(ctx, backend, provider, adoptErr)
		}
		if err := active.backend.Pause(ctx); err != nil {
			adoptErr := fmt.Errorf("pause active playback session %q: %w", active.snapshot.ID, err)
			return coordinator.Snapshot(), coordinator.rollbackAdoption(ctx, backend, provider, adoptErr)
		}
	}
	observed.ID = strings.TrimSpace(observed.ID)
	if observed.ID == "" {
		observed.ID = coordinator.newSessionID(PlaybackFocusPersistent)
	}
	observed.Focus = PlaybackFocusPersistent
	observed.Capabilities = capabilities
	adopted := &coordinatorSession{snapshot: observed, backend: backend}
	coordinator.reconcileObservedSession(adopted, observed)
	return coordinator.commit(adopted, nil), nil
}

func (coordinator *PlaybackCoordinator) rollbackAdoption(
	ctx context.Context,
	backend PlaybackBackend,
	provider PlaybackProvider,
	cause error,
) error {
	rollbackCtx, cancel := playbackCompensationContext(ctx)
	defer cancel()
	if err := backend.Pause(rollbackCtx); err != nil {
		return errors.Join(cause, fmt.Errorf("pause adopted provider %q: %w", provider, err))
	}
	return cause
}

func (coordinator *PlaybackCoordinator) restoreAfterEndedPreview(
	ctx context.Context,
	active *coordinatorSession,
	suspended *coordinatorSession,
) (*coordinatorSession, bool) {
	if active == nil || active.snapshot.Focus != PlaybackFocusTransientPreview || suspended == nil {
		return nil, false
	}
	restored := cloneCoordinatorSession(suspended)
	restored.snapshot.State = PlaybackStatePaused
	restored.snapshot.ErrorMessage = ""
	if active.backend.Provider() == restored.backend.Provider() {
		restored.needsReload = true
	}
	shouldResume := active.resumePolicy == PreviewResumeIfPreviouslyPlaying && restored.resumeEligible
	if !shouldResume {
		return restored, true
	}
	if err := resumeCoordinatorSession(ctx, restored); err != nil {
		restored.needsReload = true
		restored.snapshot.ErrorMessage = err.Error()
		return restored, true
	}
	restored.snapshot.State = PlaybackStatePlaying
	restored.needsReload = false
	return restored, true
}

func (coordinator *PlaybackCoordinator) backendFor(provider PlaybackProvider) (PlaybackBackend, error) {
	coordinator.mu.RLock()
	backend := coordinator.backends[provider]
	coordinator.mu.RUnlock()
	if backend == nil {
		return nil, fmt.Errorf("%w: %s", ErrPlaybackBackendNotFound, provider)
	}
	return backend, nil
}

func (coordinator *PlaybackCoordinator) newSessionID(focus PlaybackFocus) string {
	coordinator.mu.Lock()
	coordinator.nextSessionID++
	id := fmt.Sprintf("%s-%d", focus, coordinator.nextSessionID)
	coordinator.mu.Unlock()
	return id
}

func (coordinator *PlaybackCoordinator) currentSessions() (*coordinatorSession, *coordinatorSession) {
	coordinator.mu.RLock()
	defer coordinator.mu.RUnlock()
	return cloneCoordinatorSession(coordinator.active), cloneCoordinatorSession(coordinator.suspendedPersistent)
}

func (coordinator *PlaybackCoordinator) rememberRollbackPoint(
	sessionID string,
	point playbackRollbackPoint,
) {
	if coordinator.rollbackPoints == nil {
		coordinator.rollbackPoints = make(map[string]playbackRollbackPoint)
	}
	coordinator.rollbackPoints[sessionID] = playbackRollbackPoint{
		active:    cloneCoordinatorSession(point.active),
		suspended: cloneCoordinatorSession(point.suspended),
	}
	compacted := coordinator.rollbackOrder[:0]
	for _, retainedID := range coordinator.rollbackOrder {
		if _, retained := coordinator.rollbackPoints[retainedID]; retained && retainedID != sessionID {
			compacted = append(compacted, retainedID)
		}
	}
	coordinator.rollbackOrder = compacted
	coordinator.rollbackOrder = append(coordinator.rollbackOrder, sessionID)
	for len(coordinator.rollbackPoints) > maxPlaybackRollbackPoints && len(coordinator.rollbackOrder) > 0 {
		oldest := coordinator.rollbackOrder[0]
		coordinator.rollbackOrder = coordinator.rollbackOrder[1:]
		delete(coordinator.rollbackPoints, oldest)
	}
}

func (coordinator *PlaybackCoordinator) forgetRollbackChain(sessionID string) {
	for sessionID != "" {
		point, ok := coordinator.rollbackPoints[sessionID]
		if !ok {
			return
		}
		delete(coordinator.rollbackPoints, sessionID)
		if point.active == nil {
			return
		}
		sessionID = point.active.snapshot.ID
	}
}

func (coordinator *PlaybackCoordinator) commit(
	active *coordinatorSession,
	suspended *coordinatorSession,
) PlaybackSnapshot {
	coordinator.mu.Lock()
	coordinator.active = cloneCoordinatorSession(active)
	coordinator.suspendedPersistent = cloneCoordinatorSession(suspended)
	coordinator.version++
	snapshot := coordinator.snapshotLocked()
	listeners := make([]PlaybackSnapshotListener, 0, len(coordinator.subscribers))
	for _, listener := range coordinator.subscribers {
		listeners = append(listeners, listener)
	}
	coordinator.mu.Unlock()
	coordinator.pendingNotifications = append(coordinator.pendingNotifications, playbackNotification{
		snapshot:  snapshot,
		listeners: listeners,
	})
	return snapshot
}

// finishOperation releases the command serialization lock before invoking
// observers, allowing listeners to issue a follow-up coordinator command.
func (coordinator *PlaybackCoordinator) finishOperation() {
	notifications := coordinator.pendingNotifications
	coordinator.pendingNotifications = nil
	coordinator.opMu.Unlock()
	for _, notification := range notifications {
		for _, listener := range notification.listeners {
			listener(clonePlaybackSnapshot(notification.snapshot))
		}
	}
}

func (coordinator *PlaybackCoordinator) snapshotLocked() PlaybackSnapshot {
	snapshot := PlaybackSnapshot{Version: coordinator.version}
	if coordinator.active != nil {
		active := clonePlaybackSessionSnapshot(coordinator.active.snapshot)
		snapshot.Active = &active
		if playbackStateMayBeAudible(active.State) {
			snapshot.AudibleSessionID = active.ID
		}
	}
	if coordinator.suspendedPersistent != nil {
		suspended := clonePlaybackSessionSnapshot(coordinator.suspendedPersistent.snapshot)
		snapshot.SuspendedPersistent = &suspended
	}
	return snapshot
}

func (coordinator *PlaybackCoordinator) rollbackStartFailure(
	ctx context.Context,
	active *coordinatorSession,
	suspended *coordinatorSession,
	attemptedBackend PlaybackBackend,
	wasAudible bool,
	pausedActive bool,
) error {
	if active == nil || !pausedActive {
		return nil
	}
	rollbackCtx, cancel := playbackCompensationContext(ctx)
	defer cancel()
	if active.backend.Provider() == attemptedBackend.Provider() {
		if !wasAudible {
			active.snapshot.State = PlaybackStatePaused
			active.needsReload = true
			coordinator.commit(active, suspended)
			return nil
		}
		if err := active.backend.Start(rollbackCtx, startRequestForSession(active, true)); err != nil {
			active.snapshot.State = PlaybackStatePaused
			active.snapshot.ErrorMessage = err.Error()
			active.needsReload = true
			coordinator.commit(active, suspended)
			return err
		}
		return nil
	}
	if !wasAudible {
		return nil
	}
	if err := active.backend.Play(rollbackCtx); err != nil {
		active.snapshot.State = PlaybackStatePaused
		active.snapshot.ErrorMessage = err.Error()
		coordinator.commit(active, suspended)
		return err
	}
	return nil
}

// Compensation must still be able to restore audio focus after a caller's
// request is cancelled or reaches its deadline. Keep request-scoped values,
// detach cancellation, and bound the best-effort recovery independently.
func playbackCompensationContext(ctx context.Context) (context.Context, context.CancelFunc) {
	if ctx == nil {
		ctx = context.Background()
	}
	return context.WithTimeout(context.WithoutCancel(ctx), playbackCompensationTimeout)
}

func validateStartCapabilities(item MediaItem, capabilities PlaybackCapabilities) error {
	if !capabilities.Available {
		return unsupportedCapabilityError(item.Source.Provider, capabilities, "start")
	}
	if !capabilities.PlayPause {
		return unsupportedCapabilityError(item.Source.Provider, capabilities, "play/pause")
	}
	if !capabilities.SupportsKind(item.Kind) {
		return &PlaybackUnsupportedError{
			Provider: item.Source.Provider,
			Reason:   fmt.Sprintf("media kind %q is not supported", item.Kind),
		}
	}
	return nil
}

func isObservedPlaybackState(state PlaybackState) bool {
	switch state {
	case PlaybackStateIdle,
		PlaybackStateLoading,
		PlaybackStatePlaying,
		PlaybackStatePaused,
		PlaybackStateBuffering,
		PlaybackStateEnded,
		PlaybackStateError:
		return true
	default:
		return false
	}
}

func unsupportedCapabilityError(
	provider PlaybackProvider,
	capabilities PlaybackCapabilities,
	capability string,
) error {
	reason := capabilities.UnsupportedReason
	if reason == "" {
		reason = fmt.Sprintf("%s capability is unavailable", capability)
	}
	return &PlaybackUnsupportedError{Provider: provider, Reason: reason}
}

func stopBackend(ctx context.Context, session *coordinatorSession) error {
	if session.snapshot.Capabilities.Stop {
		return session.backend.Stop(ctx)
	}
	if session.snapshot.Capabilities.PlayPause {
		return session.backend.Pause(ctx)
	}
	return unsupportedCapabilityError(session.backend.Provider(), session.snapshot.Capabilities, "stop")
}

func resumeCoordinatorSession(ctx context.Context, session *coordinatorSession) error {
	if session.needsReload {
		return session.backend.Start(ctx, startRequestForSession(session, true))
	}
	return session.backend.Play(ctx)
}

func startRequestForSession(session *coordinatorSession, forceReload bool) PlaybackStartRequest {
	return PlaybackStartRequest{
		SessionID:    session.snapshot.ID,
		Item:         cloneMediaItem(session.snapshot.Item),
		StartSeconds: session.snapshot.Position,
		Volume:       session.snapshot.Volume,
		Muted:        session.snapshot.Muted,
		ForceReload:  forceReload,
	}
}

func cloneCoordinatorSession(session *coordinatorSession) *coordinatorSession {
	if session == nil {
		return nil
	}
	clone := *session
	clone.snapshot = clonePlaybackSessionSnapshot(session.snapshot)
	return &clone
}

func clonePlaybackSnapshot(snapshot PlaybackSnapshot) PlaybackSnapshot {
	clone := snapshot
	if snapshot.Active != nil {
		active := clonePlaybackSessionSnapshot(*snapshot.Active)
		clone.Active = &active
	}
	if snapshot.SuspendedPersistent != nil {
		suspended := clonePlaybackSessionSnapshot(*snapshot.SuspendedPersistent)
		clone.SuspendedPersistent = &suspended
	}
	return clone
}

func clonePlaybackSessionSnapshot(snapshot PlaybackSessionSnapshot) PlaybackSessionSnapshot {
	clone := snapshot
	clone.Item = cloneMediaItem(snapshot.Item)
	clone.Capabilities.MediaKinds = append([]MediaKind(nil), snapshot.Capabilities.MediaKinds...)
	clone.Queue = make([]MediaItem, len(snapshot.Queue))
	for index, item := range snapshot.Queue {
		clone.Queue[index] = cloneMediaItem(item)
	}
	return clone
}

func cloneMediaItem(item MediaItem) MediaItem {
	clone := item
	clone.Artists = append([]string(nil), item.Artists...)
	if item.Metadata != nil {
		clone.Metadata = make(map[string]string, len(item.Metadata))
		for key, value := range item.Metadata {
			clone.Metadata[key] = value
		}
	}
	return clone
}

func cloneMediaItems(items []MediaItem) []MediaItem {
	if len(items) == 0 {
		return nil
	}
	clone := make([]MediaItem, len(items))
	for index, item := range items {
		clone[index] = cloneMediaItem(item)
	}
	return clone
}
