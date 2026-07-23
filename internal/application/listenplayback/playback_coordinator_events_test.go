package listenplayback

import (
	"context"
	"errors"
	"reflect"
	"strings"
	"sync"
	"testing"
)

type snapshotFakePlaybackBackend struct {
	*fakePlaybackBackend
	snapshotMu sync.Mutex
	current    PlaybackSnapshot
}

func newSnapshotFakePlaybackBackend(provider PlaybackProvider, kinds ...MediaKind) *snapshotFakePlaybackBackend {
	return &snapshotFakePlaybackBackend{fakePlaybackBackend: newFakePlaybackBackend(provider, kinds...)}
}

func (backend *snapshotFakePlaybackBackend) Snapshot(context.Context) PlaybackSnapshot {
	backend.snapshotMu.Lock()
	defer backend.snapshotMu.Unlock()
	return clonePlaybackSnapshot(backend.current)
}

func (backend *snapshotFakePlaybackBackend) setSnapshot(snapshot PlaybackSnapshot) {
	backend.snapshotMu.Lock()
	backend.current = clonePlaybackSnapshot(snapshot)
	backend.snapshotMu.Unlock()
}

func TestPlaybackCoordinatorObservesOnlyCurrentBackendSession(t *testing.T) {
	backend := newFakePlaybackBackend(PlaybackProviderLocal, MediaKindAudio)
	coordinator := mustPlaybackCoordinator(t, backend)
	started, err := coordinator.StartSession(context.Background(), PlaybackSessionRequest{
		SessionID: "local-current",
		Focus:     PlaybackFocusPersistent,
		Item:      testMediaItem(PlaybackProviderLocal, MediaKindAudio, "song"),
	})
	if err != nil {
		t.Fatalf("StartSession: %v", err)
	}

	observed := coordinator.ObserveBackendEvent(PlaybackBackendEvent{
		Provider:  PlaybackProviderLocal,
		SessionID: "local-current",
		State:     PlaybackStateBuffering,
		Position:  14,
		Duration:  90,
		Volume:    .35,
		Muted:     false,
		HasTiming: true,
		HasVolume: true,
	})
	if observed.Active == nil || observed.Active.State != PlaybackStateBuffering ||
		observed.Active.Position != 14 || observed.Active.Duration != 90 || observed.Active.Volume != .35 {
		t.Fatalf("observed snapshot = %+v", observed)
	}
	failed := coordinator.ObserveBackendEvent(PlaybackBackendEvent{
		Provider:  PlaybackProviderLocal,
		SessionID: "local-current",
		State:     PlaybackStateError,
		Error:     "decoder failed",
	})
	if failed.Active == nil || failed.Active.ErrorMessage != "decoder failed" {
		t.Fatalf("failed snapshot = %+v", failed)
	}

	ignored := coordinator.ObserveBackendEvent(PlaybackBackendEvent{
		Provider:  PlaybackProviderLocal,
		SessionID: "stale-session",
		State:     PlaybackStateError,
		Position:  80,
		HasTiming: true,
	})
	if ignored.Version != failed.Version || ignored.Active.State != PlaybackStateError || ignored.Active.Position != 14 {
		t.Fatalf("stale event was not ignored: started=%+v observed=%+v failed=%+v ignored=%+v", started, observed, failed, ignored)
	}
}

func TestPlaybackCoordinatorStopReloadsTheSameItemOnPlay(t *testing.T) {
	backend := newFakePlaybackBackend(PlaybackProviderLocal, MediaKindAudio)
	coordinator := mustPlaybackCoordinator(t, backend)
	_, err := coordinator.StartSession(context.Background(), PlaybackSessionRequest{
		SessionID: "local",
		Focus:     PlaybackFocusPersistent,
		Item:      testMediaItem(PlaybackProviderLocal, MediaKindAudio, "song"),
	})
	if err != nil {
		t.Fatalf("StartSession: %v", err)
	}
	stopped, err := coordinator.Stop(context.Background())
	if err != nil {
		t.Fatalf("Stop: %v", err)
	}
	if stopped.Active == nil || stopped.Active.State != PlaybackStateEnded || stopped.Active.Position != 0 || stopped.AudibleSessionID != "" {
		t.Fatalf("stopped snapshot = %+v", stopped)
	}
	resumed, err := coordinator.Play(context.Background())
	if err != nil {
		t.Fatalf("Play: %v", err)
	}
	if resumed.Active == nil || resumed.Active.State != PlaybackStatePlaying {
		t.Fatalf("resumed snapshot = %+v", resumed)
	}
	calls, starts, _ := backend.snapshot()
	if !reflect.DeepEqual(calls, []string{"start:song", "stop", "start:song"}) || len(starts) != 2 || !starts[1].ForceReload {
		t.Fatalf("calls/starts = %#v / %#v", calls, starts)
	}
}

func TestPlaybackCoordinatorReconcilesRichProviderSnapshot(t *testing.T) {
	backend := newFakePlaybackBackend(PlaybackProviderYouTubeMusic, MediaKindAudio)
	coordinator := mustPlaybackCoordinator(t, backend)
	_, err := coordinator.StartSession(context.Background(), PlaybackSessionRequest{
		SessionID: "music",
		Focus:     PlaybackFocusPersistent,
		Item:      testMediaItem(PlaybackProviderYouTubeMusic, MediaKindAudio, "first"),
	})
	if err != nil {
		t.Fatalf("StartSession: %v", err)
	}
	next := testMediaItem(PlaybackProviderYouTubeMusic, MediaKindAudio, "next")
	observed := coordinator.ObserveBackendSnapshot(PlaybackProviderYouTubeMusic, PlaybackSessionSnapshot{
		State:          PlaybackStatePlaying,
		Item:           next,
		Position:       27,
		Duration:       180,
		Volume:         .8,
		Queue:          []MediaItem{next},
		CurrentIndex:   4,
		ShuffleEnabled: true,
		RepeatMode:     RepeatModeAll,
	})
	if observed.Active == nil || observed.Active.ID != "music" || observed.Active.Item.ID != "next" ||
		observed.Active.Position != 27 || observed.Active.Duration != 180 || observed.Active.CurrentIndex != 0 ||
		!observed.Active.ShuffleEnabled || observed.Active.RepeatMode != RepeatModeAll {
		t.Fatalf("observed rich snapshot = %+v", observed)
	}
}

func TestPlaybackCoordinatorAutomaticallyRestoresPersistentAfterPreviewEnds(t *testing.T) {
	music := newFakePlaybackBackend(PlaybackProviderYouTubeMusic, MediaKindAudio)
	preview := newFakePlaybackBackend(PlaybackProviderLocal, MediaKindAudio)
	coordinator := mustPlaybackCoordinator(t, music, preview)
	_, err := coordinator.StartSession(context.Background(), PlaybackSessionRequest{
		SessionID: "music",
		Focus:     PlaybackFocusPersistent,
		Item:      testMediaItem(PlaybackProviderYouTubeMusic, MediaKindAudio, "song"),
	})
	if err != nil {
		t.Fatalf("start persistent: %v", err)
	}
	_, err = coordinator.StartSession(context.Background(), PlaybackSessionRequest{
		SessionID:           "preview",
		Focus:               PlaybackFocusTransientPreview,
		Item:                testMediaItem(PlaybackProviderLocal, MediaKindAudio, "clip"),
		PreviewResumePolicy: PreviewResumeIfPreviouslyPlaying,
	})
	if err != nil {
		t.Fatalf("start preview: %v", err)
	}
	restored := coordinator.ObserveBackendEvent(PlaybackBackendEvent{
		Provider:  PlaybackProviderLocal,
		SessionID: "preview",
		State:     PlaybackStateEnded,
	})
	if restored.Active == nil || restored.Active.ID != "music" || restored.Active.State != PlaybackStatePlaying ||
		restored.SuspendedPersistent != nil || restored.AudibleSessionID != "music" {
		t.Fatalf("restored snapshot = %+v", restored)
	}
	calls, _, _ := music.snapshot()
	if !reflect.DeepEqual(calls, []string{"start:song", "pause", "play"}) {
		t.Fatalf("music calls = %#v", calls)
	}
}

func TestPlaybackCoordinatorAdoptsAudibleLegacyProviderAndSilencesCurrent(t *testing.T) {
	local := newFakePlaybackBackend(PlaybackProviderLocal, MediaKindAudio)
	music := newFakePlaybackBackend(PlaybackProviderYouTubeMusic, MediaKindAudio)
	coordinator := mustPlaybackCoordinator(t, local, music)
	_, err := coordinator.StartSession(context.Background(), PlaybackSessionRequest{
		SessionID: "local",
		Focus:     PlaybackFocusPersistent,
		Item:      testMediaItem(PlaybackProviderLocal, MediaKindAudio, "local-song"),
	})
	if err != nil {
		t.Fatalf("start local: %v", err)
	}
	adopted, err := coordinator.AdoptBackendSnapshot(context.Background(), PlaybackProviderYouTubeMusic, PlaybackSessionSnapshot{
		ID:       "legacy:youtube_music",
		State:    PlaybackStateLoading,
		Item:     testMediaItem(PlaybackProviderYouTubeMusic, MediaKindAudio, "web-song"),
		Position: 4,
		Duration: 180,
		Volume:   .7,
	})
	if err != nil {
		t.Fatalf("AdoptBackendSnapshot: %v", err)
	}
	if adopted.Active == nil || adopted.Active.ID != "legacy:youtube_music" ||
		adopted.Active.Item.Source.Provider != PlaybackProviderYouTubeMusic ||
		adopted.Active.Focus != PlaybackFocusPersistent || adopted.AudibleSessionID == "" {
		t.Fatalf("adopted snapshot = %+v", adopted)
	}
	localCalls, _, _ := local.snapshot()
	if !reflect.DeepEqual(localCalls, []string{"start:local-song", "pause"}) {
		t.Fatalf("local calls = %#v", localCalls)
	}
	musicCalls, _, _ := music.snapshot()
	if len(musicCalls) != 0 {
		t.Fatalf("adoption should not restart the legacy backend: %#v", musicCalls)
	}
}

func TestPlaybackCoordinatorSynchronizesLegacySnapshotAfterPendingHandoff(t *testing.T) {
	local := newFakePlaybackBackend(PlaybackProviderLocal, MediaKindAudio)
	music := newSnapshotFakePlaybackBackend(PlaybackProviderYouTubeMusic, MediaKindAudio)
	coordinator := mustPlaybackCoordinator(t, local, music)
	music.setSnapshot(PlaybackSnapshot{Active: &PlaybackSessionSnapshot{
		ID:    "legacy:youtube_music",
		State: PlaybackStatePlaying,
		Item:  testMediaItem(PlaybackProviderYouTubeMusic, MediaKindAudio, "old-song"),
	}})

	startEntered := make(chan struct{})
	releaseStart := make(chan struct{})
	local.beforeStart = func() {
		close(startEntered)
		<-releaseStart
	}
	startResult := make(chan error, 1)
	go func() {
		_, err := coordinator.StartSession(context.Background(), PlaybackSessionRequest{
			SessionID: "local-current",
			Focus:     PlaybackFocusPersistent,
			Item:      testMediaItem(PlaybackProviderLocal, MediaKindAudio, "local-song"),
		})
		startResult <- err
	}()
	<-startEntered

	syncStarted := make(chan struct{})
	syncResult := make(chan error, 1)
	go func() {
		close(syncStarted)
		_, err := coordinator.SynchronizeBackendSnapshot(context.Background(), PlaybackProviderYouTubeMusic)
		syncResult <- err
	}()
	<-syncStarted
	// The handoff paused the legacy backend while synchronization waited for
	// the coordinator lock. Synchronization must read this current state, not
	// replay the older playing notification that caused the wake-up.
	music.setSnapshot(PlaybackSnapshot{Active: &PlaybackSessionSnapshot{
		ID:    "legacy:youtube_music",
		State: PlaybackStatePaused,
		Item:  testMediaItem(PlaybackProviderYouTubeMusic, MediaKindAudio, "old-song"),
	}})
	close(releaseStart)
	if err := <-startResult; err != nil {
		t.Fatalf("StartSession: %v", err)
	}
	if err := <-syncResult; err != nil {
		t.Fatalf("SynchronizeBackendSnapshot: %v", err)
	}

	snapshot := coordinator.Snapshot()
	if snapshot.Active == nil || snapshot.Active.ID != "local-current" || snapshot.Active.Item.ID != "local-song" {
		t.Fatalf("stale legacy snapshot reclaimed focus: %+v", snapshot)
	}
	localCalls, _, localAudible := local.snapshot()
	if !localAudible || !reflect.DeepEqual(localCalls, []string{"start:local-song"}) {
		t.Fatalf("local backend was interrupted: calls=%#v audible=%t", localCalls, localAudible)
	}
}

func TestPlaybackCoordinatorIgnoresPausedOutOfBandSnapshot(t *testing.T) {
	local := newFakePlaybackBackend(PlaybackProviderLocal, MediaKindAudio)
	music := newFakePlaybackBackend(PlaybackProviderYouTubeMusic, MediaKindAudio)
	coordinator := mustPlaybackCoordinator(t, local, music)
	started, err := coordinator.StartSession(context.Background(), PlaybackSessionRequest{
		SessionID: "local",
		Focus:     PlaybackFocusPersistent,
		Item:      testMediaItem(PlaybackProviderLocal, MediaKindAudio, "local-song"),
	})
	if err != nil {
		t.Fatalf("start local: %v", err)
	}
	ignored, err := coordinator.AdoptBackendSnapshot(context.Background(), PlaybackProviderYouTubeMusic, PlaybackSessionSnapshot{
		State: PlaybackStatePaused,
		Item:  testMediaItem(PlaybackProviderYouTubeMusic, MediaKindAudio, "web-song"),
	})
	if err != nil {
		t.Fatalf("AdoptBackendSnapshot: %v", err)
	}
	if ignored.Version != started.Version || ignored.Active == nil || ignored.Active.ID != "local" {
		t.Fatalf("paused legacy snapshot stole focus: %+v", ignored)
	}
}

func TestPlaybackCoordinatorRollsBackAdoptionWhenCurrentCannotPause(t *testing.T) {
	local := newFakePlaybackBackend(PlaybackProviderLocal, MediaKindAudio)
	music := newFakePlaybackBackend(PlaybackProviderYouTubeMusic, MediaKindAudio)
	coordinator := mustPlaybackCoordinator(t, local, music)
	started, err := coordinator.StartSession(context.Background(), PlaybackSessionRequest{
		SessionID: "local",
		Focus:     PlaybackFocusPersistent,
		Item:      testMediaItem(PlaybackProviderLocal, MediaKindAudio, "local-song"),
	})
	if err != nil {
		t.Fatalf("start local: %v", err)
	}
	local.failPause = errors.New("local pause failed")
	rolledBack, err := coordinator.AdoptBackendSnapshot(context.Background(), PlaybackProviderYouTubeMusic, PlaybackSessionSnapshot{
		State: PlaybackStatePlaying,
		Item:  testMediaItem(PlaybackProviderYouTubeMusic, MediaKindAudio, "web-song"),
	})
	if err == nil || !strings.Contains(err.Error(), "local pause failed") {
		t.Fatalf("expected adoption failure, got %v", err)
	}
	if rolledBack.Version != started.Version || rolledBack.Active == nil || rolledBack.Active.ID != "local" {
		t.Fatalf("failed adoption changed ownership: %+v", rolledBack)
	}
	musicCalls, _, _ := music.snapshot()
	if !reflect.DeepEqual(musicCalls, []string{"pause"}) {
		t.Fatalf("new provider was not paused during rollback: %#v", musicCalls)
	}
}

func TestPlaybackCoordinatorSerializesConcurrentStartAndLegacyAdoption(t *testing.T) {
	local := newFakePlaybackBackend(PlaybackProviderLocal, MediaKindAudio)
	music := newFakePlaybackBackend(PlaybackProviderYouTubeMusic, MediaKindAudio)
	music.mu.Lock()
	music.audible = true // The legacy snapshot represents playback already started out of band.
	music.mu.Unlock()
	coordinator := mustPlaybackCoordinator(t, local, music)
	start := make(chan struct{})
	errorsCh := make(chan error, 2)
	var wait sync.WaitGroup
	wait.Add(2)
	go func() {
		defer wait.Done()
		<-start
		_, err := coordinator.StartSession(context.Background(), PlaybackSessionRequest{
			SessionID: "local",
			Focus:     PlaybackFocusPersistent,
			Item:      testMediaItem(PlaybackProviderLocal, MediaKindAudio, "local-song"),
		})
		errorsCh <- err
	}()
	go func() {
		defer wait.Done()
		<-start
		_, err := coordinator.AdoptBackendSnapshot(context.Background(), PlaybackProviderYouTubeMusic, PlaybackSessionSnapshot{
			ID:    "legacy:youtube_music",
			State: PlaybackStatePlaying,
			Item:  testMediaItem(PlaybackProviderYouTubeMusic, MediaKindAudio, "web-song"),
		})
		errorsCh <- err
	}()
	close(start)
	wait.Wait()
	close(errorsCh)
	for err := range errorsCh {
		if err != nil {
			t.Fatalf("concurrent operation: %v", err)
		}
	}
	_, _, localAudible := local.snapshot()
	_, _, musicAudible := music.snapshot()
	if localAudible && musicAudible {
		t.Fatalf("concurrent handoff left two audible backends: local=%t music=%t snapshot=%+v", localAudible, musicAudible, coordinator.Snapshot())
	}
	if !localAudible && !musicAudible {
		t.Fatalf("concurrent handoff left no audible owner: snapshot=%+v", coordinator.Snapshot())
	}
}
