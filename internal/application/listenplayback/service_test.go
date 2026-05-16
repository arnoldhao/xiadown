package listenplayback

import (
	"context"
	"testing"
	"time"
)

type fakeTransport struct {
	currentVideoID string
	actions        []string
	loads          []fakeLoad
	seeks          []float64
}

type fakeLoad struct {
	videoID      string
	startSeconds float64
	forceReload  bool
	loadStrategy VideoLoadStrategy
	restart      bool
	volume       float64
	muted        bool
}

type fakeLibraryClient struct {
	radioTracks       []Track
	mixQueueResult    MixQueueResult
	continuationQueue MixQueueResult
	metadata          map[string]Track
}

type fakeSessionStore struct {
	session RestoredPlaybackSession
	ok      bool
	saved   RestoredPlaybackSession
	cleared bool
}

func (client fakeLibraryClient) Radio(context.Context, string, int) ([]Track, error) {
	return cloneTracks(client.radioTracks), nil
}

func (client fakeLibraryClient) MixQueue(context.Context, string, string) (MixQueueResult, error) {
	result := client.mixQueueResult
	result.Tracks = cloneTracks(result.Tracks)
	return result, nil
}

func (client fakeLibraryClient) MixQueueContinuation(context.Context, string) (MixQueueResult, error) {
	result := client.continuationQueue
	result.Tracks = cloneTracks(result.Tracks)
	return result, nil
}

func (client fakeLibraryClient) TrackMetadata(_ context.Context, videoID string) (Track, error) {
	if client.metadata != nil {
		if track, ok := client.metadata[videoID]; ok {
			return track, nil
		}
	}
	return Track{ID: videoID, VideoID: videoID, Title: videoID}, nil
}

func (store *fakeSessionStore) SavePlaybackSession(_ context.Context, session RestoredPlaybackSession) error {
	store.saved = session
	store.ok = true
	return nil
}

func (store *fakeSessionStore) LoadPlaybackSession(context.Context) (RestoredPlaybackSession, bool, error) {
	return store.session, store.ok, nil
}

func (store *fakeSessionStore) ClearPlaybackSession(context.Context) error {
	store.cleared = true
	store.ok = false
	store.session = RestoredPlaybackSession{}
	return nil
}

func (transport *fakeTransport) LoadVideo(_ context.Context, request PlayRequest, strategy VideoLoadStrategy) error {
	transport.currentVideoID = request.Track.VideoID
	transport.loads = append(transport.loads, fakeLoad{
		videoID:      request.Track.VideoID,
		startSeconds: request.StartSeconds,
		forceReload:  request.ForceReload,
		loadStrategy: strategy,
		restart:      request.RestartFromStart,
		volume:       request.Volume,
		muted:        request.Muted,
	})
	transport.actions = append(transport.actions, "load:"+request.Track.VideoID)
	return nil
}

func (transport *fakeTransport) Play(context.Context) error {
	transport.actions = append(transport.actions, "play")
	return nil
}

func (transport *fakeTransport) Pause(context.Context) error {
	transport.actions = append(transport.actions, "pause")
	return nil
}

func (transport *fakeTransport) Seek(_ context.Context, seconds float64) error {
	transport.seeks = append(transport.seeks, seconds)
	transport.actions = append(transport.actions, "seek")
	return nil
}

func (transport *fakeTransport) SetVolume(_ context.Context, _ float64, _ bool) error {
	transport.actions = append(transport.actions, "volume")
	return nil
}

func (transport *fakeTransport) Next(context.Context) error {
	transport.actions = append(transport.actions, "next")
	return nil
}

func (transport *fakeTransport) Previous(context.Context) error {
	transport.actions = append(transport.actions, "previous")
	return nil
}

func (transport *fakeTransport) CurrentVideoID(context.Context) string {
	return transport.currentVideoID
}

func makeTracks() []Track {
	return []Track{
		{ID: "one", VideoID: "video-one", Title: "One", Artist: "Artist"},
		{ID: "two", VideoID: "video-two", Title: "Two", Artist: "Artist"},
		{ID: "three", VideoID: "video-three", Title: "Three", Artist: "Artist"},
	}
}

func newTestService(transport *fakeTransport) *PlayerService {
	service := NewPlayerService(transport, WithRandomIndex(func(limit int) int {
		if limit <= 1 {
			return 0
		}
		return limit - 1
	}))
	service.ConfirmPlaybackStarted()
	return service
}

func waitForTrackArtist(t *testing.T, service *PlayerService, videoID string, artist string) {
	t.Helper()
	deadline := time.Now().Add(time.Second)
	for time.Now().Before(deadline) {
		snapshot := service.Snapshot(context.Background())
		if snapshot.CurrentTrack != nil &&
			snapshot.CurrentTrack.VideoID == videoID &&
			snapshot.CurrentTrack.Artist == artist {
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	snapshot := service.Snapshot(context.Background())
	t.Fatalf("timed out waiting for %q artist %q, snapshot: %#v", videoID, artist, snapshot.CurrentTrack)
}

func TestPlayTrackEnrichesMissingArtistFromLibraryMetadata(t *testing.T) {
	ctx := context.Background()
	transport := &fakeTransport{}
	service := NewPlayerService(
		transport,
		WithLibraryClient(fakeLibraryClient{
			metadata: map[string]Track{
				"video-one": {
					ID:             "video-one",
					VideoID:        "video-one",
					Title:          "One",
					Artist:         "Metadata Artist",
					ArtistBrowseID: "UCmetadata",
					DurationLabel:  "5:20",
					ThumbnailURL:   "https://example.test/art.jpg",
					MusicVideoType: "MUSIC_VIDEO_TYPE_ATV",
				},
			},
		}),
	)
	service.ConfirmPlaybackStarted()

	if err := service.PlayTrack(ctx, Track{ID: "video-one", VideoID: "video-one", Title: "One"}, PlayOptions{}); err != nil {
		t.Fatal(err)
	}

	waitForTrackArtist(t, service, "video-one", "Metadata Artist")
}

func TestPlayWithMixShufflesFetchedQueueBeforePlayback(t *testing.T) {
	ctx := context.Background()
	transport := &fakeTransport{}
	randomCalls := 0
	service := NewPlayerService(
		transport,
		WithLibraryClient(fakeLibraryClient{
			mixQueueResult: MixQueueResult{
				Tracks:            makeTracks(),
				ContinuationToken: "next",
			},
		}),
		WithRandomIndex(func(limit int) int {
			randomCalls++
			return 0
		}),
	)
	service.ConfirmPlaybackStarted()

	if err := service.PlayWithMix(ctx, "mix-playlist", "", "Mix"); err != nil {
		t.Fatal(err)
	}

	queue, index := service.Queue()
	if index != 0 {
		t.Fatalf("expected mix to start at index 0, got %d", index)
	}
	if randomCalls == 0 {
		t.Fatal("expected mix queue to be shuffled")
	}
	if got := queue[0].VideoID; got != "video-two" {
		t.Fatalf("expected deterministic shuffled first track video-two, got %q", got)
	}
	if got := transport.loads[len(transport.loads)-1].videoID; got != queue[0].VideoID {
		t.Fatalf("expected loaded track to match shuffled first queue item, loaded=%q queue=%q", got, queue[0].VideoID)
	}
}

func TestPlayRadioQueueKeepsRadioKindAndStartsAtRequestedIndex(t *testing.T) {
	ctx := context.Background()
	transport := &fakeTransport{}
	service := newTestService(transport)

	if err := service.PlayRadioQueue(ctx, makeTracks(), 1, "Radio"); err != nil {
		t.Fatal(err)
	}

	snapshot := service.Snapshot(ctx)
	if snapshot.QueueKind != QueueKindRadio {
		t.Fatalf("expected radio queue kind, got %q", snapshot.QueueKind)
	}
	if snapshot.CurrentIndex != 1 {
		t.Fatalf("expected current index 1, got %d", snapshot.CurrentIndex)
	}
	if got := transport.loads[len(transport.loads)-1].videoID; got != "video-two" {
		t.Fatalf("expected video-two to load, got %q", got)
	}
}

func TestNextPublishesCurrentTrackSnapshot(t *testing.T) {
	ctx := context.Background()
	transport := &fakeTransport{}
	service := newTestService(transport)
	var snapshots []Snapshot
	unsubscribe := service.Subscribe(func(snapshot Snapshot) {
		snapshots = append(snapshots, snapshot)
	})
	defer unsubscribe()

	if err := service.PlayQueue(ctx, makeTracks(), 0, "Queue"); err != nil {
		t.Fatal(err)
	}
	snapshots = nil

	if err := service.Next(ctx); err != nil {
		t.Fatal(err)
	}

	if len(snapshots) == 0 {
		t.Fatal("expected next to publish a playback snapshot")
	}
	last := snapshots[len(snapshots)-1]
	if last.Version == 0 {
		t.Fatal("expected published snapshot to include a version")
	}
	if last.CurrentIndex != 1 {
		t.Fatalf("expected current index 1, got %d", last.CurrentIndex)
	}
	if last.CurrentTrack == nil || last.CurrentTrack.VideoID != "video-two" {
		t.Fatalf("expected current track video-two, got %#v", last.CurrentTrack)
	}
}

func TestPlayQueuePreservesDuplicateVideoIDs(t *testing.T) {
	ctx := context.Background()
	transport := &fakeTransport{}
	service := newTestService(transport)
	tracks := []Track{
		{ID: "first", VideoID: "video-one", Title: "One"},
		{ID: "repeat", VideoID: "video-one", Title: "One Again"},
		{ID: "second", VideoID: "video-two", Title: "Two"},
	}

	if err := service.PlayQueue(ctx, tracks, 1, "Queue"); err != nil {
		t.Fatal(err)
	}

	queue, index := service.Queue()
	if len(queue) != len(tracks) {
		t.Fatalf("expected duplicate queue entries to be preserved, got %d want %d", len(queue), len(tracks))
	}
	if index != 1 {
		t.Fatalf("expected current duplicate index 1, got %d", index)
	}
	if got := queue[1].Title; got != "One Again" {
		t.Fatalf("expected duplicate row metadata to be preserved, got %q", got)
	}
}

func TestSetVolumePersistsCurrentSessionVolume(t *testing.T) {
	ctx := context.Background()
	transport := &fakeTransport{}
	store := &fakeSessionStore{}
	service := NewPlayerService(
		transport,
		WithSessionStore(store),
		WithUserInteractionUnlocked(),
	)
	if err := service.PlayQueue(ctx, makeTracks(), 0, "Queue"); err != nil {
		t.Fatal(err)
	}

	if err := service.SetVolume(ctx, 0.35, false); err != nil {
		t.Fatal(err)
	}

	if store.saved.Volume != 0.35 {
		t.Fatalf("expected persisted volume 0.35, got %f", store.saved.Volume)
	}
	if store.saved.Muted {
		t.Fatal("expected persisted session to be unmuted")
	}
}

func TestQueuedTrackLoadsCarryCurrentVolume(t *testing.T) {
	ctx := context.Background()
	transport := &fakeTransport{}
	service := NewPlayerService(transport, WithUserInteractionUnlocked())
	if err := service.PlayQueue(ctx, makeTracks(), 0, "Queue"); err != nil {
		t.Fatal(err)
	}
	if err := service.SetVolume(ctx, 0.35, false); err != nil {
		t.Fatal(err)
	}
	if err := service.Next(ctx); err != nil {
		t.Fatal(err)
	}

	if len(transport.loads) == 0 {
		t.Fatal("expected track load")
	}
	last := transport.loads[len(transport.loads)-1]
	if last.videoID != "video-two" {
		t.Fatalf("expected next load for video-two, got %q", last.videoID)
	}
	if last.volume != 0.35 {
		t.Fatalf("expected load volume 0.35, got %f", last.volume)
	}
	if last.muted {
		t.Fatal("expected load to stay unmuted")
	}
}

func TestSetVolumePersistsVolumeBeforeMute(t *testing.T) {
	ctx := context.Background()
	store := &fakeSessionStore{}
	service := NewPlayerService(
		&fakeTransport{},
		WithSessionStore(store),
		WithUserInteractionUnlocked(),
	)
	if err := service.PlayQueue(ctx, makeTracks(), 0, "Queue"); err != nil {
		t.Fatal(err)
	}
	if err := service.SetVolume(ctx, 0.35, false); err != nil {
		t.Fatal(err)
	}
	if err := service.SetVolume(ctx, 0, true); err != nil {
		t.Fatal(err)
	}
	if store.saved.VolumeBeforeMute != 0.35 {
		t.Fatalf("expected persisted volumeBeforeMute 0.35, got %f", store.saved.VolumeBeforeMute)
	}

	restoreStore := &fakeSessionStore{session: store.saved, ok: true}
	restored := NewPlayerService(&fakeTransport{}, WithSessionStore(restoreStore))
	if _, err := restored.RestorePlaybackSession(ctx); err != nil {
		t.Fatal(err)
	}
	snapshot := restored.Snapshot(ctx)
	if snapshot.VolumeBeforeMute != 0.35 {
		t.Fatalf("expected restored volumeBeforeMute 0.35, got %f", snapshot.VolumeBeforeMute)
	}
}

func TestNextWithRepeatOneAdvancesQueue(t *testing.T) {
	ctx := context.Background()
	transport := &fakeTransport{}
	service := newTestService(transport)

	if err := service.PlayQueue(ctx, makeTracks(), 1, "Queue"); err != nil {
		t.Fatal(err)
	}
	service.SetRepeatMode(RepeatModeOne)
	if err := service.Next(ctx); err != nil {
		t.Fatal(err)
	}

	_, index := service.Queue()
	if index != 2 {
		t.Fatalf("expected current index 2, got %d", index)
	}
	if got := transport.loads[len(transport.loads)-1].videoID; got != "video-three" {
		t.Fatalf("expected video-three to load, got %q", got)
	}
}

func TestNextWithShuffleAndRepeatOneUsesShuffle(t *testing.T) {
	ctx := context.Background()
	transport := &fakeTransport{}
	service := newTestService(transport)

	if err := service.PlayQueue(ctx, makeTracks(), 1, "Queue"); err != nil {
		t.Fatal(err)
	}
	service.ToggleShuffle()
	service.SetRepeatMode(RepeatModeOne)
	if err := service.Next(ctx); err != nil {
		t.Fatal(err)
	}

	_, index := service.Queue()
	if index != 2 {
		t.Fatalf("expected shuffle to pick deterministic index 2, got %d", index)
	}
	if got := transport.loads[len(transport.loads)-1].videoID; got != "video-three" {
		t.Fatalf("expected video-three to load, got %q", got)
	}
}

func TestPreviousSeeksToStartBeforeWalkingForwardSkipStack(t *testing.T) {
	ctx := context.Background()
	transport := &fakeTransport{}
	service := newTestService(transport)

	if err := service.PlayQueue(ctx, makeTracks(), 0, "Queue"); err != nil {
		t.Fatal(err)
	}
	if err := service.UpdatePlaybackState(ctx, true, 100, 180); err != nil {
		t.Fatal(err)
	}
	if err := service.Next(ctx); err != nil {
		t.Fatal(err)
	}
	if err := service.UpdatePlaybackState(ctx, true, 8, 180); err != nil {
		t.Fatal(err)
	}
	if err := service.Previous(ctx); err != nil {
		t.Fatal(err)
	}
	if len(transport.seeks) == 0 || transport.seeks[len(transport.seeks)-1] != 0 {
		t.Fatalf("expected previous to seek to start first, seeks=%v", transport.seeks)
	}
	_, index := service.Queue()
	if index != 1 {
		t.Fatalf("expected index to remain 1 after seek-to-start, got %d", index)
	}

	if err := service.Previous(ctx); err != nil {
		t.Fatal(err)
	}
	_, index = service.Queue()
	if index != 0 {
		t.Fatalf("expected second previous to restore index 0, got %d", index)
	}
}

func TestPlaybackStatePublishesOnlyMaterialProgressChanges(t *testing.T) {
	ctx := context.Background()
	service := newTestService(&fakeTransport{})
	if err := service.PlayQueue(ctx, makeTracks(), 0, "Queue"); err != nil {
		t.Fatal(err)
	}
	var snapshots []Snapshot
	unsubscribe := service.Subscribe(func(snapshot Snapshot) {
		snapshots = append(snapshots, snapshot)
	})
	defer unsubscribe()

	if err := service.UpdatePlaybackState(ctx, true, 1, 180); err != nil {
		t.Fatal(err)
	}
	if len(snapshots) != 1 {
		t.Fatalf("expected initial playing update to publish once, got %d", len(snapshots))
	}
	if err := service.UpdatePlaybackState(ctx, true, 1.2, 180); err != nil {
		t.Fatal(err)
	}
	if len(snapshots) != 1 {
		t.Fatalf("expected sub-threshold progress update to stay local, got %d snapshots", len(snapshots))
	}
	if err := service.UpdatePlaybackState(ctx, true, 1.5, 180); err != nil {
		t.Fatal(err)
	}
	if len(snapshots) != 2 {
		t.Fatalf("expected material progress update to publish, got %d snapshots", len(snapshots))
	}
	if snapshots[len(snapshots)-1].Progress != 1.5 {
		t.Fatalf("expected latest published progress 1.5, got %f", snapshots[len(snapshots)-1].Progress)
	}
}

func TestPlayQueueResetsProgressForLoadingTrack(t *testing.T) {
	ctx := context.Background()
	service := newTestService(&fakeTransport{})
	if err := service.PlayQueue(ctx, makeTracks(), 0, "Queue"); err != nil {
		t.Fatal(err)
	}
	if err := service.UpdatePlaybackState(ctx, true, 90, 180); err != nil {
		t.Fatal(err)
	}
	if err := service.Next(ctx); err != nil {
		t.Fatal(err)
	}

	snapshot := service.Snapshot(ctx)
	if snapshot.CurrentTrack == nil || snapshot.CurrentTrack.VideoID != "video-two" {
		t.Fatalf("expected second track snapshot, got %#v", snapshot.CurrentTrack)
	}
	if snapshot.Progress != 0 {
		t.Fatalf("expected loading track progress to reset, got %f", snapshot.Progress)
	}
}

func TestTrackEndedRepeatOneRestartsCurrentQueueTrack(t *testing.T) {
	ctx := context.Background()
	transport := &fakeTransport{}
	service := newTestService(transport)

	if err := service.PlayQueue(ctx, makeTracks()[:2], 0, "Queue"); err != nil {
		t.Fatal(err)
	}
	service.SetRepeatMode(RepeatModeOne)
	if err := service.HandleTrackEnded(ctx, "video-one"); err != nil {
		t.Fatal(err)
	}

	_, index := service.Queue()
	if index != 0 {
		t.Fatalf("expected repeat one to keep index 0, got %d", index)
	}
	if len(transport.seeks) == 0 || transport.seeks[len(transport.seeks)-1] != 0 {
		t.Fatalf("expected repeat one to seek to start, seeks=%v", transport.seeks)
	}
	if transport.actions[len(transport.actions)-1] != "play" {
		t.Fatalf("expected repeat one to resume after seek, actions=%v", transport.actions)
	}
}

func TestStaleTrackEndedEventDoesNotAdvanceQueue(t *testing.T) {
	ctx := context.Background()
	transport := &fakeTransport{}
	service := newTestService(transport)

	if err := service.PlayQueue(ctx, makeTracks()[:2], 0, "Queue"); err != nil {
		t.Fatal(err)
	}
	if err := service.UpdatePlaybackState(ctx, true, 20, 180); err != nil {
		t.Fatal(err)
	}
	if err := service.HandleTrackEnded(ctx, "video-two"); err != nil {
		t.Fatal(err)
	}

	_, index := service.Queue()
	if index != 0 {
		t.Fatalf("expected stale ended event to keep index 0, got %d", index)
	}
	if service.State() != PlaybackStatePlaying {
		t.Fatalf("expected state to remain playing, got %s", service.State())
	}
}

func TestRepeatOneDoesNotRealignQueueWhenObservedInQueueVideo(t *testing.T) {
	ctx := context.Background()
	transport := &fakeTransport{}
	service := newTestService(transport)

	if err := service.PlayQueue(ctx, makeTracks(), 1, "Queue"); err != nil {
		t.Fatal(err)
	}
	service.SetRepeatMode(RepeatModeOne)
	if err := service.UpdateTrackMetadata(ctx, ObservedTrack{
		ObservedVideoID: "video-three",
		Title:           "Three",
		Artist:          "Artist",
		TrackChanged:    true,
	}); err != nil {
		t.Fatal(err)
	}

	_, index := service.Queue()
	if index != 1 {
		t.Fatalf("expected repeat one to keep queue index 1, got %d", index)
	}
	track, ok := service.CurrentTrack()
	if !ok || track.VideoID != "video-two" {
		t.Fatalf("expected current track to remain video-two, got %#v", track)
	}
	if got := transport.loads[len(transport.loads)-1]; got.videoID != "video-two" || !got.forceReload {
		t.Fatalf("expected repeat one to force reload current queue track, got %#v", got)
	}
}

func TestRepeatOneRecoversWhenTitleDriftsBeforeVideoID(t *testing.T) {
	ctx := context.Background()
	transport := &fakeTransport{}
	service := newTestService(transport)

	if err := service.PlayQueue(ctx, makeTracks()[:2], 0, "Queue"); err != nil {
		t.Fatal(err)
	}
	service.SetRepeatMode(RepeatModeOne)
	if err := service.UpdateTrackMetadata(ctx, ObservedTrack{
		Title:        "Autoplay Suggestion",
		Artist:       "Someone Else",
		TrackChanged: true,
	}); err != nil {
		t.Fatal(err)
	}

	_, index := service.Queue()
	if index != 0 {
		t.Fatalf("expected repeat one title drift recovery to keep index 0, got %d", index)
	}
	track, ok := service.CurrentTrack()
	if !ok || track.VideoID != "video-one" || track.Title != "One" {
		t.Fatalf("expected visible track to remain queue metadata, got %#v", track)
	}
}

func TestQueueEndSuppressesUnexpectedAutoplay(t *testing.T) {
	ctx := context.Background()
	transport := &fakeTransport{}
	service := newTestService(transport)

	if err := service.PlayQueue(ctx, makeTracks()[:1], 0, "Queue"); err != nil {
		t.Fatal(err)
	}
	if err := service.UpdatePlaybackState(ctx, true, 178, 180); err != nil {
		t.Fatal(err)
	}
	if err := service.HandleTrackEnded(ctx, "video-one"); err != nil {
		t.Fatal(err)
	}
	if service.State() != PlaybackStateEnded {
		t.Fatalf("expected queue end state ended, got %s", service.State())
	}
	if transport.actions[len(transport.actions)-1] != "pause" {
		t.Fatalf("expected queue end to pause transport, actions=%v", transport.actions)
	}

	if err := service.UpdateTrackMetadata(ctx, ObservedTrack{
		ObservedVideoID: "video-two",
		Title:           "Autoplay",
		Artist:          "Other",
		TrackChanged:    true,
	}); err != nil {
		t.Fatal(err)
	}
	track, ok := service.CurrentTrack()
	if !ok || track.VideoID != "video-one" {
		t.Fatalf("expected visible track to remain video-one, got %#v", track)
	}
}

func TestNearEndVideoIDOnlyTransitionKeepsExpectedQueueMetadata(t *testing.T) {
	ctx := context.Background()
	transport := &fakeTransport{}
	service := newTestService(transport)

	if err := service.PlayQueue(ctx, makeTracks(), 0, "Queue"); err != nil {
		t.Fatal(err)
	}
	if err := service.UpdateTrackMetadata(ctx, ObservedTrack{
		ObservedVideoID: "video-one",
		Title:           "One",
		Artist:          "Artist",
		TrackChanged:    false,
	}); err != nil {
		t.Fatal(err)
	}
	if err := service.UpdatePlaybackState(ctx, true, 179, 180); err != nil {
		t.Fatal(err)
	}
	if err := service.UpdateTrackMetadata(ctx, ObservedTrack{
		ObservedVideoID: "video-two",
		TrackChanged:    true,
	}); err != nil {
		t.Fatal(err)
	}

	_, index := service.Queue()
	if index != 1 {
		t.Fatalf("expected near-end transition to move to expected queue index 1, got %d", index)
	}
	track, ok := service.CurrentTrack()
	if !ok || track.VideoID != "video-two" || track.Title != "Two" || track.Artist != "Artist" {
		t.Fatalf("expected queue metadata for video-two to stay visible, got %#v", track)
	}
}

func TestSameQueueTrackObservedMetadataDoesNotReplaceAuthoritativeQueueMetadata(t *testing.T) {
	ctx := context.Background()
	transport := &fakeTransport{}
	service := newTestService(transport)
	tracks := makeTracks()
	tracks[0].MusicVideoType = "MUSIC_VIDEO_TYPE_OMV"
	tracks[0].HasVideo = true
	tracks[0].VideoAvailabilityKnown = true
	tracks[0].ThumbnailURL = "https://example.com/queue.jpg"

	if err := service.PlayQueue(ctx, tracks, 0, "Queue"); err != nil {
		t.Fatal(err)
	}
	if err := service.UpdateTrackMetadata(ctx, ObservedTrack{
		ObservedVideoID:        "video-one",
		Title:                  "One",
		Artist:                 "Artist",
		VideoAvailable:         true,
		VideoAvailabilityKnown: true,
		TrackChanged:           true,
	}); err != nil {
		t.Fatal(err)
	}
	if err := service.UpdateTrackMetadata(ctx, ObservedTrack{
		ObservedVideoID: "video-one",
		Title:           "Stale DOM Title",
		Artist:          "Stale Artist",
		ThumbnailURL:    "https://example.com/player-bar-low-res.jpg",
		TrackChanged:    true,
	}); err != nil {
		t.Fatal(err)
	}

	track, ok := service.CurrentTrack()
	if !ok {
		t.Fatal("expected current track")
	}
	if track.Title != "One" || track.Artist != "Artist" {
		t.Fatalf("expected queue metadata to remain visible, got %#v", track)
	}
	if !track.VideoAvailabilityKnown || !track.HasVideo {
		t.Fatalf("expected known video availability to be preserved, got %#v", track)
	}
	if track.MusicVideoType != "MUSIC_VIDEO_TYPE_OMV" {
		t.Fatalf("expected rich queue fields to be preserved, got %#v", track)
	}
	if track.ThumbnailURL != "https://example.com/queue.jpg" {
		t.Fatalf("expected queue thumbnail to stay canonical, got %q", track.ThumbnailURL)
	}
}

func TestObservedMetadataFillsMissingQueueThumbnail(t *testing.T) {
	ctx := context.Background()
	service := newTestService(&fakeTransport{})
	tracks := makeTracks()

	if err := service.PlayQueue(ctx, tracks, 0, "Queue"); err != nil {
		t.Fatal(err)
	}
	if err := service.UpdateTrackMetadata(ctx, ObservedTrack{
		ObservedVideoID: "video-one",
		Title:           "One",
		Artist:          "Artist",
		ThumbnailURL:    "https://example.com/player-bar-thumbnail.jpg",
		TrackChanged:    true,
	}); err != nil {
		t.Fatal(err)
	}

	track, ok := service.CurrentTrack()
	if !ok {
		t.Fatal("expected current track")
	}
	if track.ThumbnailURL != "https://example.com/player-bar-thumbnail.jpg" {
		t.Fatalf("expected missing thumbnail to be filled, got %q", track.ThumbnailURL)
	}
}

func TestVideoAvailabilityUpdatesCurrentQueueTrackWithoutUnknownResets(t *testing.T) {
	ctx := context.Background()
	service := newTestService(&fakeTransport{})
	if err := service.PlayQueue(ctx, makeTracks(), 0, "Queue"); err != nil {
		t.Fatal(err)
	}

	service.UpdateVideoAvailability(ctx, true, true)
	track, ok := service.CurrentTrack()
	if !ok || !track.VideoAvailabilityKnown || !track.HasVideo {
		t.Fatalf("expected available video state, got %#v", track)
	}
	service.UpdateVideoAvailability(ctx, false, false)
	track, ok = service.CurrentTrack()
	if !ok || !track.VideoAvailabilityKnown || !track.HasVideo {
		t.Fatalf("expected unknown video update to preserve known state, got %#v", track)
	}
}

func TestAppInitiatedPlaybackReassertsIntendedQueueTrack(t *testing.T) {
	ctx := context.Background()
	transport := &fakeTransport{}
	service := newTestService(transport)

	if err := service.PlayQueue(ctx, makeTracks(), 0, "Queue"); err != nil {
		t.Fatal(err)
	}
	if err := service.Next(ctx); err != nil {
		t.Fatal(err)
	}
	if err := service.UpdateTrackMetadata(ctx, ObservedTrack{
		ObservedVideoID: "outside-video",
		Title:           "Outside",
		Artist:          "Other",
		TrackChanged:    true,
	}); err != nil {
		t.Fatal(err)
	}

	last := transport.loads[len(transport.loads)-1]
	if last.videoID != "video-two" {
		t.Fatalf("expected intended queue track to reload, got %q", last.videoID)
	}
	if !last.forceReload || last.loadStrategy != VideoLoadForceFullPageWhenSameVideoID {
		t.Fatalf("expected force full-page reload, got %#v", last)
	}
}

func TestTrackEndRepeatAllWrapsWhenObservedAlreadyWrapped(t *testing.T) {
	ctx := context.Background()
	transport := &fakeTransport{}
	service := newTestService(transport)

	if err := service.PlayQueue(ctx, makeTracks()[:2], 1, "Queue"); err != nil {
		t.Fatal(err)
	}
	service.SetRepeatMode(RepeatModeAll)
	if err := service.HandleTrackEnded(ctx, "video-one"); err != nil {
		t.Fatal(err)
	}

	_, index := service.Queue()
	if index != 0 {
		t.Fatalf("expected repeat-all ended event to wrap to index 0, got %d", index)
	}
	track, ok := service.CurrentTrack()
	if !ok || track.VideoID != "video-one" {
		t.Fatalf("expected current track video-one after wrap, got %#v", track)
	}
}

func TestMoveQueueItemsMaintainsCurrentTrackAndBlocksMovingCurrent(t *testing.T) {
	ctx := context.Background()
	transport := &fakeTransport{}
	service := newTestService(transport)

	if err := service.PlayQueue(ctx, makeTracks(), 1, "Queue"); err != nil {
		t.Fatal(err)
	}
	service.MoveQueueItems(ctx, []int{0}, 3)
	queue, index := service.Queue()
	if got := []string{queue[0].VideoID, queue[1].VideoID, queue[2].VideoID}; got[0] != "video-two" || got[1] != "video-three" || got[2] != "video-one" {
		t.Fatalf("expected moved queue [two three one], got %v", got)
	}
	if index != 0 {
		t.Fatalf("expected current track to remain video-two at new index 0, got %d", index)
	}

	service.MoveQueueItems(ctx, []int{0}, 2)
	queue, index = service.Queue()
	if got := []string{queue[0].VideoID, queue[1].VideoID, queue[2].VideoID}; got[0] != "video-two" || got[1] != "video-three" || got[2] != "video-one" {
		t.Fatalf("expected moving current track to be ignored, got %v", got)
	}
	if index != 0 {
		t.Fatalf("expected index to stay 0 after ignored move, got %d", index)
	}
}

func TestRestoredPlaybackSessionSeeksThenAutoResumes(t *testing.T) {
	ctx := context.Background()
	transport := &fakeTransport{}
	service := newTestService(transport)

	service.ApplyRestoredPlaybackSession(makeTracks(), 1, 42, 240)
	if service.State() != PlaybackStatePaused {
		t.Fatalf("expected restored state paused, got %s", service.State())
	}
	if err := service.Resume(ctx); err != nil {
		t.Fatal(err)
	}
	if last := transport.loads[len(transport.loads)-1]; last.videoID != "video-two" || last.startSeconds != 42 {
		t.Fatalf("expected restored load at 42s, got %#v", last)
	}
	if err := service.UpdatePlaybackState(ctx, false, 0, 240); err != nil {
		t.Fatal(err)
	}
	if len(transport.seeks) == 0 || transport.seeks[len(transport.seeks)-1] != 42 {
		t.Fatalf("expected restored reconcile to seek to 42, seeks=%v", transport.seeks)
	}
	if err := service.UpdatePlaybackState(ctx, false, 42, 240); err != nil {
		t.Fatal(err)
	}
	if transport.actions[len(transport.actions)-1] != "play" {
		t.Fatalf("expected restored reconcile to resume playback, actions=%v", transport.actions)
	}
}

func TestRestorePlaybackSessionPrefersPersistedCurrentVideoID(t *testing.T) {
	ctx := context.Background()
	store := &fakeSessionStore{
		ok: true,
		session: RestoredPlaybackSession{
			Queue:          makeTracks(),
			CurrentIndex:   0,
			CurrentVideoID: "video-three",
			Progress:       12,
			Duration:       200,
			Volume:         1,
		},
	}
	service := NewPlayerService(&fakeTransport{}, WithSessionStore(store))

	restored, err := service.RestorePlaybackSession(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if !restored {
		t.Fatal("expected session to restore")
	}

	queue, index := service.Queue()
	if len(queue) != 3 {
		t.Fatalf("expected restored queue, got %d items", len(queue))
	}
	if index != 2 {
		t.Fatalf("expected currentVideoId to resolve index 2, got %d", index)
	}
	track, ok := service.CurrentTrack()
	if !ok || track.VideoID != "video-three" {
		t.Fatalf("expected restored current track video-three, got %#v", track)
	}
}

func TestNearEndAutoplayWithRepeatOneDoesNotAdvanceQueue(t *testing.T) {
	ctx := context.Background()
	transport := &fakeTransport{}
	service := newTestService(transport)

	if err := service.PlayQueue(ctx, makeTracks(), 1, "Queue"); err != nil {
		t.Fatal(err)
	}
	service.SetRepeatMode(RepeatModeOne)
	if err := service.UpdatePlaybackState(ctx, true, 199, 200); err != nil {
		t.Fatal(err)
	}
	if err := service.UpdateTrackMetadata(ctx, ObservedTrack{
		ObservedVideoID: "video-three",
		Title:           "Three",
		Artist:          "Artist",
		TrackChanged:    true,
	}); err != nil {
		t.Fatal(err)
	}

	_, index := service.Queue()
	if index != 1 {
		t.Fatalf("expected repeat one recovery to keep index 1, got %d", index)
	}
	track, ok := service.CurrentTrack()
	if !ok || track.VideoID != "video-two" {
		t.Fatalf("expected current track to remain video-two, got %#v", track)
	}
}

func TestMergeTrackMetadataUpdatesQueueSnapshotFromService(t *testing.T) {
	ctx := context.Background()
	transport := &fakeTransport{}
	service := newTestService(transport)
	tracks := []Track{
		{ID: "video-one", VideoID: "video-one", Title: "video-one", Artist: "YouTube Music"},
		{ID: "video-two", VideoID: "video-two", Title: "video-two", Artist: "YouTube Music"},
	}

	if err := service.PlayQueue(ctx, tracks, 1, "Queue"); err != nil {
		t.Fatal(err)
	}
	service.MergeTrackMetadata(ctx, Track{
		ID:           "two",
		VideoID:      "video-two",
		Title:        "Resolved Two",
		Artist:       "Resolved Artist",
		ThumbnailURL: "https://example.test/two.jpg",
	})

	snapshot := service.Snapshot(ctx)
	if snapshot.CurrentTrack == nil || snapshot.CurrentTrack.Title != "Resolved Two" {
		t.Fatalf("expected current track metadata from service, got %#v", snapshot.CurrentTrack)
	}
	if got := snapshot.Queue[1]; got.Title != "Resolved Two" || got.ThumbnailURL == "" {
		t.Fatalf("expected queue snapshot metadata from service, got %#v", got)
	}
}
