package listenplayback

import (
	"context"
	"errors"
	"fmt"
	"reflect"
	"strings"
	"sync"
	"testing"
)

type fakePlaybackBackend struct {
	mu sync.Mutex

	provider PlaybackProvider
	kinds    []MediaKind
	calls    []string
	starts   []PlaybackStartRequest
	audible  bool

	failNextStart error
	failPause     error
	failPlay      error
	failStop      error
	beforeStart   func()
	honorContext  bool
}

func newFakePlaybackBackend(provider PlaybackProvider, kinds ...MediaKind) *fakePlaybackBackend {
	return &fakePlaybackBackend{provider: provider, kinds: kinds}
}

func (backend *fakePlaybackBackend) Provider() PlaybackProvider {
	return backend.provider
}

func (backend *fakePlaybackBackend) Capabilities() PlaybackCapabilities {
	return PlaybackCapabilities{
		Available:  true,
		MediaKinds: append([]MediaKind(nil), backend.kinds...),
		PlayPause:  true,
		Stop:       true,
		Seek:       true,
		Previous:   true,
		Next:       true,
		Volume:     true,
		Queue:      true,
	}
}

func (backend *fakePlaybackBackend) Start(ctx context.Context, request PlaybackStartRequest) error {
	backend.mu.Lock()
	defer backend.mu.Unlock()
	backend.calls = append(backend.calls, "start:"+request.Item.ID)
	backend.starts = append(backend.starts, request)
	if backend.beforeStart != nil {
		beforeStart := backend.beforeStart
		backend.beforeStart = nil
		beforeStart()
	}
	if backend.honorContext && ctx != nil && ctx.Err() != nil {
		return ctx.Err()
	}
	if backend.failNextStart != nil {
		err := backend.failNextStart
		backend.failNextStart = nil
		return err
	}
	backend.audible = true
	return nil
}

func (backend *fakePlaybackBackend) Play(ctx context.Context) error {
	backend.mu.Lock()
	defer backend.mu.Unlock()
	backend.calls = append(backend.calls, "play")
	if backend.honorContext && ctx != nil && ctx.Err() != nil {
		return ctx.Err()
	}
	if backend.failPlay != nil {
		return backend.failPlay
	}
	backend.audible = true
	return nil
}

func (backend *fakePlaybackBackend) Pause(context.Context) error {
	backend.mu.Lock()
	defer backend.mu.Unlock()
	backend.calls = append(backend.calls, "pause")
	if backend.failPause != nil {
		return backend.failPause
	}
	backend.audible = false
	return nil
}

func (backend *fakePlaybackBackend) Stop(context.Context) error {
	backend.mu.Lock()
	defer backend.mu.Unlock()
	backend.calls = append(backend.calls, "stop")
	if backend.failStop != nil {
		return backend.failStop
	}
	backend.audible = false
	return nil
}

func (backend *fakePlaybackBackend) Seek(_ context.Context, seconds float64) error {
	backend.mu.Lock()
	defer backend.mu.Unlock()
	backend.calls = append(backend.calls, fmt.Sprintf("seek:%g", seconds))
	return nil
}

func (backend *fakePlaybackBackend) SetVolume(_ context.Context, volume float64, muted bool) error {
	backend.mu.Lock()
	defer backend.mu.Unlock()
	backend.calls = append(backend.calls, fmt.Sprintf("volume:%g:%t", volume, muted))
	return nil
}

func (backend *fakePlaybackBackend) Previous(context.Context) error {
	backend.mu.Lock()
	defer backend.mu.Unlock()
	backend.calls = append(backend.calls, "previous")
	return nil
}

func (backend *fakePlaybackBackend) Next(context.Context) error {
	backend.mu.Lock()
	defer backend.mu.Unlock()
	backend.calls = append(backend.calls, "next")
	return nil
}

func (backend *fakePlaybackBackend) snapshot() ([]string, []PlaybackStartRequest, bool) {
	backend.mu.Lock()
	defer backend.mu.Unlock()
	return append([]string(nil), backend.calls...), append([]PlaybackStartRequest(nil), backend.starts...), backend.audible
}

func TestPlaybackCoordinatorKeepsOnlyOneAudibleSession(t *testing.T) {
	ctx := context.Background()
	music := newFakePlaybackBackend(PlaybackProviderYouTubeMusic, MediaKindAudio)
	youtube := newFakePlaybackBackend(PlaybackProviderYouTube, MediaKindVideo)
	coordinator := mustPlaybackCoordinator(t, music, youtube)

	first, err := coordinator.StartSession(ctx, PlaybackSessionRequest{
		SessionID: "music",
		Focus:     PlaybackFocusPersistent,
		Item:      testMediaItem(PlaybackProviderYouTubeMusic, MediaKindAudio, "song"),
	})
	if err != nil {
		t.Fatalf("start music: %v", err)
	}
	if first.AudibleSessionID != "music" {
		t.Fatalf("AudibleSessionID = %q, want music", first.AudibleSessionID)
	}

	second, err := coordinator.StartSession(ctx, PlaybackSessionRequest{
		SessionID: "youtube",
		Focus:     PlaybackFocusPersistent,
		Item:      testMediaItem(PlaybackProviderYouTube, MediaKindVideo, "video"),
	})
	if err != nil {
		t.Fatalf("start YouTube: %v", err)
	}
	_, _, musicAudible := music.snapshot()
	_, _, youtubeAudible := youtube.snapshot()
	if musicAudible || !youtubeAudible {
		t.Fatalf("audible backends: music=%t youtube=%t, want only YouTube", musicAudible, youtubeAudible)
	}
	if second.AudibleSessionID != "youtube" || second.Active == nil || second.Active.ID != "youtube" {
		t.Fatalf("unexpected active snapshot: %+v", second)
	}
	if second.SuspendedPersistent != nil {
		t.Fatalf("persistent-to-persistent switch should not suspend the old session: %+v", second)
	}
	musicCalls, _, _ := music.snapshot()
	if !reflect.DeepEqual(musicCalls, []string{"start:song", "pause", "stop"}) {
		t.Fatalf("music calls = %#v, want start, pause, then resource-releasing stop", musicCalls)
	}
}

func TestPlaybackCoordinatorPreviewPausesAndResumesPersistent(t *testing.T) {
	ctx := context.Background()
	music := newFakePlaybackBackend(PlaybackProviderYouTubeMusic, MediaKindAudio)
	preview := newFakePlaybackBackend(PlaybackProviderLocal, MediaKindAudio)
	coordinator := mustPlaybackCoordinator(t, music, preview)

	persistent, err := coordinator.StartSession(ctx, PlaybackSessionRequest{
		SessionID: "persistent",
		Focus:     PlaybackFocusPersistent,
		Item:      testMediaItem(PlaybackProviderYouTubeMusic, MediaKindAudio, "song"),
	})
	if err != nil {
		t.Fatalf("start persistent: %v", err)
	}
	previewSnapshot, err := coordinator.StartSession(ctx, PlaybackSessionRequest{
		SessionID:           "preview",
		Focus:               PlaybackFocusTransientPreview,
		Item:                testMediaItem(PlaybackProviderLocal, MediaKindAudio, "sample"),
		PreviewResumePolicy: PreviewResumeIfPreviouslyPlaying,
	})
	if err != nil {
		t.Fatalf("start preview: %v", err)
	}
	if previewSnapshot.Active == nil || previewSnapshot.Active.ID != "preview" {
		t.Fatalf("preview is not active: %+v", previewSnapshot)
	}
	if previewSnapshot.SuspendedPersistent == nil ||
		previewSnapshot.SuspendedPersistent.ID != persistent.Active.ID ||
		previewSnapshot.SuspendedPersistent.State != PlaybackStatePaused {
		t.Fatalf("persistent session is not suspended: %+v", previewSnapshot)
	}
	_, _, musicAudible := music.snapshot()
	_, _, previewAudible := preview.snapshot()
	if musicAudible || !previewAudible {
		t.Fatalf("preview focus did not silence persistent backend: music=%t preview=%t", musicAudible, previewAudible)
	}

	restored, err := coordinator.CloseSession(ctx, "preview")
	if err != nil {
		t.Fatalf("close preview: %v", err)
	}
	if restored.Active == nil || restored.Active.ID != "persistent" || restored.Active.State != PlaybackStatePlaying {
		t.Fatalf("persistent session was not restored: %+v", restored)
	}
	if restored.SuspendedPersistent != nil || restored.AudibleSessionID != "persistent" {
		t.Fatalf("unexpected restored focus snapshot: %+v", restored)
	}
	_, _, musicAudible = music.snapshot()
	_, _, previewAudible = preview.snapshot()
	if !musicAudible || previewAudible {
		t.Fatalf("restore did not preserve a single audible backend: music=%t preview=%t", musicAudible, previewAudible)
	}
	previewCalls, _, _ := preview.snapshot()
	if !reflect.DeepEqual(previewCalls, []string{"start:sample", "stop"}) {
		t.Fatalf("preview calls = %#v, want start then stop", previewCalls)
	}
	musicCalls, _, _ := music.snapshot()
	if !reflect.DeepEqual(musicCalls, []string{"start:song", "pause", "play"}) {
		t.Fatalf("music calls = %#v, want start, pause, play", musicCalls)
	}
}

func TestPlaybackCoordinatorRollbackRestoresFullPreviewAndSuspendedState(t *testing.T) {
	ctx := context.Background()
	music := newFakePlaybackBackend(PlaybackProviderYouTubeMusic, MediaKindAudio)
	preview := newFakePlaybackBackend(PlaybackProviderLocal, MediaKindAudio)
	youtube := newFakePlaybackBackend(PlaybackProviderYouTube, MediaKindVideo)
	coordinator := mustPlaybackCoordinator(t, music, preview, youtube)

	if _, err := coordinator.StartSession(ctx, PlaybackSessionRequest{
		SessionID: "persistent",
		Focus:     PlaybackFocusPersistent,
		Item:      testMediaItem(PlaybackProviderYouTubeMusic, MediaKindAudio, "song"),
	}); err != nil {
		t.Fatalf("start persistent: %v", err)
	}
	if _, err := coordinator.StartSession(ctx, PlaybackSessionRequest{
		SessionID:           "preview",
		Focus:               PlaybackFocusTransientPreview,
		Item:                testMediaItem(PlaybackProviderLocal, MediaKindAudio, "sample"),
		PreviewResumePolicy: PreviewResumeIfPreviouslyPlaying,
	}); err != nil {
		t.Fatalf("start preview: %v", err)
	}
	previewSnapshot := coordinator.Snapshot().Active
	if previewSnapshot == nil {
		t.Fatal("preview has no active snapshot")
	}
	previewSnapshot.Queue = []MediaItem{
		testMediaItem(PlaybackProviderLocal, MediaKindAudio, "sample-one"),
		testMediaItem(PlaybackProviderLocal, MediaKindAudio, "sample-two"),
	}
	previewSnapshot.CurrentIndex = 1
	previewSnapshot.ShuffleEnabled = true
	previewSnapshot.RepeatMode = RepeatModeAll
	coordinator.ObserveBackendSnapshot(PlaybackProviderLocal, *previewSnapshot)
	before := coordinator.Snapshot()
	replacement, err := coordinator.StartSession(ctx, PlaybackSessionRequest{
		SessionID:      "youtube",
		Focus:          PlaybackFocusPersistent,
		Item:           testMediaItem(PlaybackProviderYouTube, MediaKindVideo, "video"),
		RetainRollback: true,
	})
	if err != nil {
		t.Fatalf("start reversible YouTube: %v", err)
	}
	if replacement.Active == nil {
		t.Fatalf("replacement has no active session: %+v", replacement)
	}
	restored, err := coordinator.RollbackSession(ctx, replacement.Active.ID)
	if err != nil {
		t.Fatalf("RollbackSession: %v", err)
	}
	if !reflect.DeepEqual(restored.Active, before.Active) ||
		!reflect.DeepEqual(restored.SuspendedPersistent, before.SuspendedPersistent) ||
		restored.AudibleSessionID != before.AudibleSessionID {
		t.Fatalf("rollback did not restore full snapshot:\nbefore=%+v\nafter=%+v", before, restored)
	}
	closed, err := coordinator.CloseSession(ctx, "preview")
	if err != nil {
		t.Fatalf("close restored preview: %v", err)
	}
	if closed.Active == nil || closed.Active.ID != "persistent" ||
		closed.Active.Item.ID != "song" || closed.Active.State != PlaybackStatePlaying {
		t.Fatalf("restored preview resumed wrong persistent session: %+v", closed)
	}
}

func TestPlaybackCoordinatorRollbackReloadsSuspendedSessionOverwrittenByReplacement(t *testing.T) {
	ctx := context.Background()
	youtube := newFakePlaybackBackend(PlaybackProviderYouTube, MediaKindVideo)
	preview := newFakePlaybackBackend(PlaybackProviderLocal, MediaKindAudio)
	coordinator := mustPlaybackCoordinator(t, youtube, preview)
	if _, err := coordinator.StartSession(ctx, PlaybackSessionRequest{
		SessionID: "old-youtube",
		Focus:     PlaybackFocusPersistent,
		Item:      testMediaItem(PlaybackProviderYouTube, MediaKindVideo, "old-video"),
	}); err != nil {
		t.Fatalf("start old YouTube: %v", err)
	}
	if _, err := coordinator.StartSession(ctx, PlaybackSessionRequest{
		SessionID:           "preview",
		Focus:               PlaybackFocusTransientPreview,
		Item:                testMediaItem(PlaybackProviderLocal, MediaKindAudio, "sample"),
		PreviewResumePolicy: PreviewResumeIfPreviouslyPlaying,
	}); err != nil {
		t.Fatalf("start preview: %v", err)
	}
	replacement, err := coordinator.StartSession(ctx, PlaybackSessionRequest{
		SessionID:      "new-youtube",
		Focus:          PlaybackFocusPersistent,
		Item:           testMediaItem(PlaybackProviderYouTube, MediaKindVideo, "new-video"),
		RetainRollback: true,
	})
	if err != nil || replacement.Active == nil {
		t.Fatalf("start replacement: snapshot=%+v error=%v", replacement, err)
	}
	if _, err := coordinator.RollbackSession(ctx, replacement.Active.ID); err != nil {
		t.Fatalf("RollbackSession: %v", err)
	}
	closed, err := coordinator.CloseSession(ctx, "preview")
	if err != nil {
		t.Fatalf("close restored preview: %v", err)
	}
	if closed.Active == nil || closed.Active.ID != "old-youtube" ||
		closed.Active.Item.ID != "old-video" || closed.Active.State != PlaybackStatePlaying {
		t.Fatalf("wrong suspended session restored: %+v", closed)
	}
	calls, _, _ := youtube.snapshot()
	if len(calls) == 0 || calls[len(calls)-1] != "start:old-video" {
		t.Fatalf("overwritten suspended YouTube was not reloaded: %#v", calls)
	}
}

func TestPlaybackCoordinatorPreviewCanKeepPersistentPaused(t *testing.T) {
	ctx := context.Background()
	music := newFakePlaybackBackend(PlaybackProviderYouTubeMusic, MediaKindAudio)
	preview := newFakePlaybackBackend(PlaybackProviderLocal, MediaKindAudio)
	coordinator := mustPlaybackCoordinator(t, music, preview)

	_, err := coordinator.StartSession(ctx, PlaybackSessionRequest{
		SessionID: "persistent",
		Focus:     PlaybackFocusPersistent,
		Item:      testMediaItem(PlaybackProviderYouTubeMusic, MediaKindAudio, "song"),
	})
	if err != nil {
		t.Fatalf("start persistent: %v", err)
	}
	_, err = coordinator.StartSession(ctx, PlaybackSessionRequest{
		SessionID:           "preview",
		Focus:               PlaybackFocusTransientPreview,
		Item:                testMediaItem(PlaybackProviderLocal, MediaKindAudio, "sample"),
		PreviewResumePolicy: PreviewKeepPersistentPaused,
	})
	if err != nil {
		t.Fatalf("start preview: %v", err)
	}

	paused, err := coordinator.CloseSession(ctx, "preview")
	if err != nil {
		t.Fatalf("close preview: %v", err)
	}
	if paused.Active == nil || paused.Active.ID != "persistent" || paused.Active.State != PlaybackStatePaused {
		t.Fatalf("persistent session should retain focus while paused: %+v", paused)
	}
	if paused.AudibleSessionID != "" {
		t.Fatalf("paused snapshot should not identify an audible session: %+v", paused)
	}
	_, _, musicAudible := music.snapshot()
	_, _, previewAudible := preview.snapshot()
	if musicAudible || previewAudible {
		t.Fatalf("keep-paused policy left an audible backend: music=%t preview=%t", musicAudible, previewAudible)
	}

	resumed, err := coordinator.Play(ctx)
	if err != nil {
		t.Fatalf("resume persistent manually: %v", err)
	}
	if resumed.AudibleSessionID != "persistent" || resumed.Active.State != PlaybackStatePlaying {
		t.Fatalf("manual resume failed: %+v", resumed)
	}
}

func TestPlaybackCoordinatorDoesNotResumePersistentThatWasAlreadyPaused(t *testing.T) {
	ctx := context.Background()
	music := newFakePlaybackBackend(PlaybackProviderYouTubeMusic, MediaKindAudio)
	preview := newFakePlaybackBackend(PlaybackProviderLocal, MediaKindAudio)
	coordinator := mustPlaybackCoordinator(t, music, preview)

	_, err := coordinator.StartSession(ctx, PlaybackSessionRequest{
		SessionID: "persistent",
		Focus:     PlaybackFocusPersistent,
		Item:      testMediaItem(PlaybackProviderYouTubeMusic, MediaKindAudio, "song"),
	})
	if err != nil {
		t.Fatalf("start persistent: %v", err)
	}
	if _, err := coordinator.Pause(ctx); err != nil {
		t.Fatalf("pause persistent: %v", err)
	}
	_, err = coordinator.StartSession(ctx, PlaybackSessionRequest{
		SessionID:           "preview",
		Focus:               PlaybackFocusTransientPreview,
		Item:                testMediaItem(PlaybackProviderLocal, MediaKindAudio, "sample"),
		PreviewResumePolicy: PreviewResumeIfPreviouslyPlaying,
	})
	if err != nil {
		t.Fatalf("start preview: %v", err)
	}
	restored, err := coordinator.CloseSession(ctx, "preview")
	if err != nil {
		t.Fatalf("close preview: %v", err)
	}
	if restored.Active == nil || restored.Active.State != PlaybackStatePaused || restored.AudibleSessionID != "" {
		t.Fatalf("originally paused persistent session was incorrectly resumed: %+v", restored)
	}
	musicCalls, _, musicAudible := music.snapshot()
	if musicAudible {
		t.Fatal("originally paused persistent backend is audible")
	}
	for _, call := range musicCalls {
		if call == "play" {
			t.Fatalf("unexpected automatic play call after preview: %#v", musicCalls)
		}
	}
}

func TestPlaybackCoordinatorReloadsPersistentAfterSameBackendPreview(t *testing.T) {
	ctx := context.Background()
	backend := newFakePlaybackBackend(PlaybackProviderStream, MediaKindAudio)
	coordinator := mustPlaybackCoordinator(t, backend)

	_, err := coordinator.StartSession(ctx, PlaybackSessionRequest{
		SessionID:    "station",
		Focus:        PlaybackFocusPersistent,
		Item:         testMediaItem(PlaybackProviderStream, MediaKindAudio, "station-item"),
		StartSeconds: 37,
	})
	if err != nil {
		t.Fatalf("start persistent: %v", err)
	}
	_, err = coordinator.StartSession(ctx, PlaybackSessionRequest{
		SessionID:           "preview",
		Focus:               PlaybackFocusTransientPreview,
		Item:                testMediaItem(PlaybackProviderStream, MediaKindAudio, "preview-item"),
		PreviewResumePolicy: PreviewResumeIfPreviouslyPlaying,
	})
	if err != nil {
		t.Fatalf("start preview: %v", err)
	}
	restored, err := coordinator.CloseSession(ctx, "preview")
	if err != nil {
		t.Fatalf("close preview: %v", err)
	}
	if restored.Active == nil || restored.Active.ID != "station" || restored.Active.State != PlaybackStatePlaying {
		t.Fatalf("persistent session was not restored: %+v", restored)
	}
	calls, starts, audible := backend.snapshot()
	if !reflect.DeepEqual(calls, []string{
		"start:station-item",
		"pause",
		"start:preview-item",
		"stop",
		"start:station-item",
	}) {
		t.Fatalf("same-backend calls = %#v", calls)
	}
	if len(starts) != 3 || starts[2].StartSeconds != 37 || !starts[2].ForceReload {
		t.Fatalf("persistent reload request = %+v, want position 37 and ForceReload", starts)
	}
	if !audible {
		t.Fatal("restored persistent backend should be audible")
	}
}

func TestPlaybackCoordinatorRestoresPersistentAfterFailedHandoff(t *testing.T) {
	ctx := context.Background()
	music := newFakePlaybackBackend(PlaybackProviderYouTubeMusic, MediaKindAudio)
	youtube := newFakePlaybackBackend(PlaybackProviderYouTube, MediaKindVideo)
	coordinator := mustPlaybackCoordinator(t, music, youtube)

	initial, err := coordinator.StartSession(ctx, PlaybackSessionRequest{
		SessionID: "persistent",
		Focus:     PlaybackFocusPersistent,
		Item:      testMediaItem(PlaybackProviderYouTubeMusic, MediaKindAudio, "song"),
	})
	if err != nil {
		t.Fatalf("start persistent: %v", err)
	}
	youtube.failNextStart = errors.New("load failed")
	failed, err := coordinator.StartSession(ctx, PlaybackSessionRequest{
		SessionID: "replacement",
		Focus:     PlaybackFocusPersistent,
		Item:      testMediaItem(PlaybackProviderYouTube, MediaKindVideo, "video"),
	})
	if err == nil {
		t.Fatal("failed backend handoff unexpectedly succeeded")
	}
	if failed.Active == nil || failed.Active.ID != initial.Active.ID || failed.AudibleSessionID != "persistent" {
		t.Fatalf("previous session snapshot was not retained: %+v", failed)
	}
	_, _, musicAudible := music.snapshot()
	_, _, youtubeAudible := youtube.snapshot()
	if !musicAudible || youtubeAudible {
		t.Fatalf("failed handoff did not restore prior audio: music=%t youtube=%t", musicAudible, youtubeAudible)
	}
	musicCalls, _, _ := music.snapshot()
	if !reflect.DeepEqual(musicCalls, []string{"start:song", "pause", "play"}) {
		t.Fatalf("rollback calls = %#v", musicCalls)
	}
}

func TestPlaybackCoordinatorRollbackOutlivesCancelledHandoffContext(t *testing.T) {
	music := newFakePlaybackBackend(PlaybackProviderYouTubeMusic, MediaKindAudio)
	music.honorContext = true
	youtube := newFakePlaybackBackend(PlaybackProviderYouTube, MediaKindVideo)
	youtube.honorContext = true
	coordinator := mustPlaybackCoordinator(t, music, youtube)

	if _, err := coordinator.StartPersistent(
		context.Background(),
		testMediaItem(PlaybackProviderYouTubeMusic, MediaKindAudio, "song"),
	); err != nil {
		t.Fatalf("start persistent: %v", err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	youtube.beforeStart = cancel
	failed, err := coordinator.StartPersistent(
		ctx,
		testMediaItem(PlaybackProviderYouTube, MediaKindVideo, "video"),
	)
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("handoff error = %v, want context.Canceled", err)
	}
	if failed.Active == nil || failed.Active.Item.ID != "song" ||
		failed.Active.State != PlaybackStatePlaying || failed.AudibleSessionID == "" {
		t.Fatalf("cancelled handoff did not restore the previous session: %+v", failed)
	}
	_, _, audible := music.snapshot()
	if !audible {
		t.Fatal("previous backend remained paused after cancelled handoff")
	}
}

func TestPlaybackCoordinatorCommitsPausedStateWhenRollbackFails(t *testing.T) {
	music := newFakePlaybackBackend(PlaybackProviderYouTubeMusic, MediaKindAudio)
	youtube := newFakePlaybackBackend(PlaybackProviderYouTube, MediaKindVideo)
	coordinator := mustPlaybackCoordinator(t, music, youtube)

	if _, err := coordinator.StartPersistent(
		context.Background(),
		testMediaItem(PlaybackProviderYouTubeMusic, MediaKindAudio, "song"),
	); err != nil {
		t.Fatalf("start persistent: %v", err)
	}
	music.failPlay = errors.New("restore failed")
	youtube.failNextStart = errors.New("replacement failed")
	failed, err := coordinator.StartPersistent(
		context.Background(),
		testMediaItem(PlaybackProviderYouTube, MediaKindVideo, "video"),
	)
	if err == nil || !strings.Contains(err.Error(), "restore failed") {
		t.Fatalf("handoff error = %v, want rollback failure", err)
	}
	if failed.Active == nil || failed.Active.Item.ID != "song" ||
		failed.Active.State != PlaybackStatePaused || failed.AudibleSessionID != "" ||
		failed.Active.ErrorMessage != "restore failed" {
		t.Fatalf("failed rollback published an inaccurate snapshot: %+v", failed)
	}
	music.failPlay = nil
	resumed, err := coordinator.Play(context.Background())
	if err != nil {
		t.Fatalf("retry rollback session: %v", err)
	}
	if resumed.Active == nil || resumed.Active.State != PlaybackStatePlaying ||
		resumed.Active.ErrorMessage != "" || resumed.AudibleSessionID == "" {
		t.Fatalf("successful retry retained the rollback error: %+v", resumed)
	}
}

func TestPlaybackCoordinatorSubscriberCanIssueFollowUpCommand(t *testing.T) {
	ctx := context.Background()
	backend := newFakePlaybackBackend(PlaybackProviderStream, MediaKindAudio)
	coordinator := mustPlaybackCoordinator(t, backend)
	listenerCalls := 0
	var listenerErr error
	unsubscribe := coordinator.Subscribe(func(snapshot PlaybackSnapshot) {
		listenerCalls++
		if snapshot.Active != nil && snapshot.Active.State == PlaybackStatePlaying {
			_, listenerErr = coordinator.Pause(ctx)
		}
	})
	defer unsubscribe()

	if _, err := coordinator.StartPersistent(
		ctx,
		testMediaItem(PlaybackProviderStream, MediaKindAudio, "station-item"),
	); err != nil {
		t.Fatalf("start persistent: %v", err)
	}
	if listenerErr != nil {
		t.Fatalf("subscriber follow-up pause: %v", listenerErr)
	}
	if listenerCalls != 2 {
		t.Fatalf("listener calls = %d, want playing and paused notifications", listenerCalls)
	}
	snapshot := coordinator.Snapshot()
	if snapshot.Active == nil || snapshot.Active.State != PlaybackStatePaused || snapshot.AudibleSessionID != "" {
		t.Fatalf("subscriber follow-up did not pause coordinator: %+v", snapshot)
	}
}

func TestUnsupportedLocalMediaBackendIsExplicit(t *testing.T) {
	ctx := context.Background()
	local := NewUnsupportedLocalMediaBackend("native engine pending")
	if local.LocalMediaSupported() {
		t.Fatal("unsupported local backend reported support")
	}
	capabilities := local.Capabilities()
	if capabilities.Available || capabilities.UnsupportedReason != "native engine pending" {
		t.Fatalf("unexpected local capabilities: %+v", capabilities)
	}
	if err := local.Play(ctx); !errors.Is(err, ErrPlaybackUnsupported) {
		t.Fatalf("local.Play error = %v, want ErrPlaybackUnsupported", err)
	}

	coordinator := mustPlaybackCoordinator(t, local)
	snapshot, err := coordinator.StartPersistent(ctx, testMediaItem(PlaybackProviderLocal, MediaKindAudio, "local-file"))
	if !errors.Is(err, ErrPlaybackUnsupported) {
		t.Fatalf("StartPersistent error = %v, want ErrPlaybackUnsupported", err)
	}
	if snapshot.Active != nil || snapshot.AudibleSessionID != "" {
		t.Fatalf("unsupported local backend changed coordinator state: %+v", snapshot)
	}
}

func TestLegacyTrackConversionKeepsCompatibilityFields(t *testing.T) {
	track := Track{
		ID:                     "track-id",
		VideoID:                "video-id",
		Title:                  "Title",
		Artist:                 "Artist",
		Artists:                []TrackArtist{{Name: "Artist", BrowseID: "artist-browse"}},
		ArtistBrowseID:         "artist-browse",
		ArtistSource:           TrackArtistSourceAPILinked,
		DurationLabel:          "3:21",
		DurationSeconds:        201,
		ThumbnailURL:           "https://example.test/art.jpg",
		MusicVideoType:         "MUSIC_VIDEO_TYPE_ATV",
		HasVideo:               true,
		VideoAvailabilityKnown: true,
		LikeStatus:             "LIKE",
		InLibrary:              true,
		FeedbackTokens:         FeedbackTokens{Add: "add", Remove: "remove"},
	}
	item := MediaItemFromTrack(track, PlaybackProviderYouTubeMusic)
	converted := TrackFromMediaItem(item)
	if converted.ID != track.ID || converted.VideoID != track.VideoID || converted.Title != track.Title ||
		converted.Artist != track.Artist || converted.DurationSeconds != track.DurationSeconds ||
		converted.ThumbnailURL != track.ThumbnailURL || converted.ArtistBrowseID != track.ArtistBrowseID ||
		converted.MusicVideoType != track.MusicVideoType || converted.HasVideo != track.HasVideo ||
		converted.VideoAvailabilityKnown != track.VideoAvailabilityKnown || converted.LikeStatus != track.LikeStatus ||
		converted.InLibrary != track.InLibrary || converted.FeedbackTokens != track.FeedbackTokens {
		t.Fatalf("legacy conversion lost fields:\noriginal=%+v\nconverted=%+v", track, converted)
	}

	legacy := Snapshot{
		Version:        9,
		State:          PlaybackStatePlaying,
		CurrentTrack:   &track,
		Progress:       42,
		Duration:       201,
		Volume:         0.8,
		Queue:          []Track{track},
		CurrentIndex:   0,
		RepeatMode:     RepeatModeAll,
		ShuffleEnabled: true,
	}
	generic := PlaybackSnapshotFromLegacy(legacy, PlaybackProviderYouTubeMusic)
	if generic.Version != legacy.Version || generic.Active == nil || generic.Active.Item.Source.ID != track.VideoID ||
		generic.Active.Position != legacy.Progress || generic.AudibleSessionID == "" {
		t.Fatalf("legacy snapshot conversion failed: %+v", generic)
	}
}

func mustPlaybackCoordinator(t *testing.T, backends ...PlaybackBackend) *PlaybackCoordinator {
	t.Helper()
	coordinator, err := NewPlaybackCoordinator(backends...)
	if err != nil {
		t.Fatalf("NewPlaybackCoordinator: %v", err)
	}
	return coordinator
}

func testMediaItem(provider PlaybackProvider, kind MediaKind, id string) MediaItem {
	return MediaItem{
		ID:     id,
		Kind:   kind,
		Source: PlaybackSource{Provider: provider, ID: id},
		Title:  id,
	}
}
