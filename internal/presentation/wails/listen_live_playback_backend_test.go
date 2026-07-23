package wails

import (
	"context"
	"fmt"
	"reflect"
	"strings"
	"sync"
	"testing"
	"time"

	"xiadown/internal/application/listenplayback"
)

type fakeListenLivePlaybackPlayer struct {
	mu        sync.Mutex
	calls     []string
	provider  listenplayback.PlaybackProvider
	sessionID string
	request   ListenPlayerPlayRequest
	listeners map[uint64]listenplayback.PlaybackBackendEventListener
	nextID    uint64
}

func newFakeListenLivePlaybackPlayer() *fakeListenLivePlaybackPlayer {
	return &fakeListenLivePlaybackPlayer{listeners: make(map[uint64]listenplayback.PlaybackBackendEventListener)}
}

func (player *fakeListenLivePlaybackPlayer) StartPlaybackSession(
	provider listenplayback.PlaybackProvider,
	sessionID string,
	request ListenPlayerPlayRequest,
) error {
	player.mu.Lock()
	defer player.mu.Unlock()
	player.provider = provider
	player.sessionID = sessionID
	player.request = request
	player.calls = append(player.calls, "start:"+string(provider)+":"+sessionID)
	return nil
}

func (player *fakeListenLivePlaybackPlayer) ResumePlaybackSession(provider listenplayback.PlaybackProvider, sessionID string) error {
	return player.recordForSession(provider, sessionID, "play")
}
func (player *fakeListenLivePlaybackPlayer) PausePlaybackSession(provider listenplayback.PlaybackProvider, sessionID string) error {
	return player.recordForSession(provider, sessionID, "pause")
}
func (player *fakeListenLivePlaybackPlayer) ResetPlaybackSession(provider listenplayback.PlaybackProvider, sessionID string) error {
	return player.recordForSession(provider, sessionID, "stop")
}
func (player *fakeListenLivePlaybackPlayer) SeekPlaybackSession(provider listenplayback.PlaybackProvider, sessionID string, seconds float64) error {
	return player.recordForSession(provider, sessionID, fmt.Sprintf("seek:%g", seconds))
}
func (player *fakeListenLivePlaybackPlayer) SetPlaybackSessionVolume(provider listenplayback.PlaybackProvider, sessionID string, volume float64, muted bool) error {
	return player.recordForSession(provider, sessionID, fmt.Sprintf("volume:%g:%t", volume, muted))
}

func (player *fakeListenLivePlaybackPlayer) SubscribePlaybackEvents(
	listener listenplayback.PlaybackBackendEventListener,
) func() {
	player.mu.Lock()
	player.nextID++
	id := player.nextID
	player.listeners[id] = listener
	player.mu.Unlock()
	return func() {
		player.mu.Lock()
		delete(player.listeners, id)
		player.mu.Unlock()
	}
}

func (player *fakeListenLivePlaybackPlayer) emit(event listenplayback.PlaybackBackendEvent) {
	player.mu.Lock()
	listeners := make([]listenplayback.PlaybackBackendEventListener, 0, len(player.listeners))
	for _, listener := range player.listeners {
		listeners = append(listeners, listener)
	}
	player.mu.Unlock()
	for _, listener := range listeners {
		listener(event)
	}
}

func (player *fakeListenLivePlaybackPlayer) record(call string) error {
	player.mu.Lock()
	player.calls = append(player.calls, call)
	player.mu.Unlock()
	return nil
}

func (player *fakeListenLivePlaybackPlayer) recordForSession(
	provider listenplayback.PlaybackProvider,
	sessionID string,
	call string,
) error {
	player.mu.Lock()
	defer player.mu.Unlock()
	if player.provider != provider || player.sessionID != sessionID {
		return nil
	}
	player.calls = append(player.calls, call)
	if call == "stop" {
		player.provider = ""
		player.sessionID = ""
	}
	return nil
}

func (player *fakeListenLivePlaybackPlayer) snapshot() ([]string, listenplayback.PlaybackProvider, string, ListenPlayerPlayRequest) {
	player.mu.Lock()
	defer player.mu.Unlock()
	return append([]string(nil), player.calls...), player.provider, player.sessionID, player.request
}

func TestListenLivePlayerBackendMapsYouTubeSession(t *testing.T) {
	player := newFakeListenLivePlaybackPlayer()
	backend := NewListenLivePlayerBackend(listenplayback.PlaybackProviderYouTube, player)
	capabilities := backend.Capabilities()
	if !capabilities.Available || !capabilities.PlayPause || !capabilities.Seek || !capabilities.Volume ||
		!capabilities.Video || !capabilities.Fullscreen || capabilities.Next || capabilities.Previous {
		t.Fatalf("capabilities = %+v", capabilities)
	}
	err := backend.Start(context.Background(), listenplayback.PlaybackStartRequest{
		SessionID: "youtube-session",
		Item: listenplayback.MediaItem{
			ID:       "AbCdEfGh123",
			Kind:     listenplayback.MediaKindVideo,
			Source:   listenplayback.PlaybackSource{Provider: listenplayback.PlaybackProviderYouTube, ID: "AbCdEfGh123"},
			Title:    "Video",
			Artist:   "Creator",
			Metadata: map[string]string{"language": "zh-CN"},
		},
		StartSeconds: 12,
		Volume:       .6,
		Muted:        true,
	})
	if err != nil {
		t.Fatalf("Start: %v", err)
	}
	_, provider, sessionID, request := player.snapshot()
	if provider != listenplayback.PlaybackProviderYouTube || sessionID != "youtube-session" ||
		request.VideoID != "AbCdEfGh123" || request.Language != "zh-CN" ||
		request.StartSeconds != 12 || request.Volume != .6 || !request.Muted {
		t.Fatalf("mapped request = %q %q %+v", provider, sessionID, request)
	}
}

func TestListenLivePlayerBackendsFilterSharedPlayerEvents(t *testing.T) {
	player := newFakeListenLivePlaybackPlayer()
	stream := NewListenLivePlayerBackend(listenplayback.PlaybackProviderStream, player)
	youtube := NewListenLivePlayerBackend(listenplayback.PlaybackProviderYouTube, player)
	streamEvents := make(chan listenplayback.PlaybackBackendEvent, 1)
	youtubeEvents := make(chan listenplayback.PlaybackBackendEvent, 1)
	stream.setSessionID("radio")
	youtube.setSessionID("video")
	unsubscribeStream := stream.Subscribe(func(event listenplayback.PlaybackBackendEvent) { streamEvents <- event })
	unsubscribeYouTube := youtube.Subscribe(func(event listenplayback.PlaybackBackendEvent) { youtubeEvents <- event })
	defer unsubscribeStream()
	defer unsubscribeYouTube()

	player.emit(listenplayback.PlaybackBackendEvent{Provider: listenplayback.PlaybackProviderStream, SessionID: "radio", State: listenplayback.PlaybackStatePlaying})
	select {
	case event := <-streamEvents:
		if event.SessionID != "radio" {
			t.Fatalf("stream event = %+v", event)
		}
	case <-time.After(time.Second):
		t.Fatal("stream event was not relayed")
	}
	select {
	case event := <-youtubeEvents:
		t.Fatalf("stream event leaked to youtube backend: %+v", event)
	case <-time.After(20 * time.Millisecond):
	}
	player.emit(listenplayback.PlaybackBackendEvent{Provider: listenplayback.PlaybackProviderStream, SessionID: "stale-radio", State: listenplayback.PlaybackStatePaused})
	select {
	case event := <-streamEvents:
		t.Fatalf("stale stream session leaked through backend: %+v", event)
	case <-time.After(20 * time.Millisecond):
	}
}

func TestListenLivePlayerBackendsSwitchCoordinatorFocusWithoutTwoStarts(t *testing.T) {
	player := newFakeListenLivePlaybackPlayer()
	stream := NewListenLivePlayerBackend(listenplayback.PlaybackProviderStream, player)
	youtube := NewListenLivePlayerBackend(listenplayback.PlaybackProviderYouTube, player)
	coordinator, err := listenplayback.NewPlaybackCoordinator(stream, youtube)
	if err != nil {
		t.Fatalf("NewPlaybackCoordinator: %v", err)
	}
	_, err = coordinator.StartSession(context.Background(), listenplayback.PlaybackSessionRequest{
		SessionID: "radio",
		Focus:     listenplayback.PlaybackFocusPersistent,
		Item: listenplayback.MediaItem{
			ID: "AbCdEfGh123", Kind: listenplayback.MediaKindAudio,
			Source: listenplayback.PlaybackSource{Provider: listenplayback.PlaybackProviderStream, ID: "AbCdEfGh123"}, Title: "Radio",
		},
	})
	if err != nil {
		t.Fatalf("start stream: %v", err)
	}
	_, err = coordinator.StartSession(context.Background(), listenplayback.PlaybackSessionRequest{
		SessionID: "video",
		Focus:     listenplayback.PlaybackFocusPersistent,
		Item: listenplayback.MediaItem{
			ID: "ZyXwVuTs987", Kind: listenplayback.MediaKindVideo,
			Source: listenplayback.PlaybackSource{Provider: listenplayback.PlaybackProviderYouTube, ID: "ZyXwVuTs987"}, Title: "Video",
		},
	})
	if err != nil {
		t.Fatalf("start youtube: %v", err)
	}
	calls, _, _, _ := player.snapshot()
	want := []string{"start:stream:radio", "pause", "start:youtube:video"}
	if !reflect.DeepEqual(calls, want) {
		t.Fatalf("focus handoff calls = %#v, want %#v", calls, want)
	}
	if err := stream.Pause(context.Background()); err != nil {
		t.Fatalf("stale stream Pause: %v", err)
	}
	calls, _, _, _ = player.snapshot()
	if !reflect.DeepEqual(calls, want) {
		t.Fatalf("stale stream command changed youtube player: %#v", calls)
	}
}

func TestListenLivePlayerHandlerLegacyControlsOnlyCurrentStreamSession(t *testing.T) {
	player := &ListenYouTubeLivePlayer{
		currentVideo:      "AbCdEfGh123",
		currentState:      "playing",
		targetVolume:      .8,
		playbackProvider:  listenplayback.PlaybackProviderStream,
		playbackSessionID: "radio",
		playbackListeners: make(map[uint64]listenplayback.PlaybackBackendEventListener),
	}
	handler := NewListenLivePlayerHandler(player)
	ctx := context.Background()
	if err := handler.Pause(ctx); err != nil {
		t.Fatalf("Pause: %v", err)
	}
	if player.Status().State != "paused" {
		t.Fatalf("pause status = %+v", player.Status())
	}
	if err := handler.Resume(ctx); err != nil {
		t.Fatalf("Resume: %v", err)
	}
	if err := handler.Seek(ctx, ListenPlayerSeekRequest{Seconds: 7}); err != nil {
		t.Fatalf("Seek: %v", err)
	}
	if err := handler.SetVolume(ctx, ListenPlayerVolumeRequest{Volume: .4}); err != nil {
		t.Fatalf("SetVolume: %v", err)
	}
	status := player.Status()
	if status.State != "buffering" || status.Volume != .4 ||
		status.Provider != listenplayback.PlaybackProviderStream || status.SessionID != "radio" {
		t.Fatalf("stream status = %+v", status)
	}

	player.mu.Lock()
	player.playbackProvider = listenplayback.PlaybackProviderYouTube
	player.playbackSessionID = "video"
	player.currentState = "playing"
	player.mu.Unlock()
	if err := handler.Pause(ctx); err != nil {
		t.Fatalf("stale legacy Pause: %v", err)
	}
	if err := handler.Reset(ctx); err != nil {
		t.Fatalf("stale legacy Reset: %v", err)
	}
	status = player.Status()
	if status.State != "playing" || status.Provider != listenplayback.PlaybackProviderYouTube || status.SessionID != "video" {
		t.Fatalf("legacy stream command changed youtube session: %+v", status)
	}
}

func TestListenYouTubeLivePlayerPublishesProviderSessionStatus(t *testing.T) {
	player := &ListenYouTubeLivePlayer{
		currentVideo:      "AbCdEfGh123",
		currentState:      "loading",
		targetVolume:      .65,
		playbackProvider:  listenplayback.PlaybackProviderYouTube,
		playbackSessionID: "video-session",
		playbackListeners: make(map[uint64]listenplayback.PlaybackBackendEventListener),
	}
	events := make(chan listenplayback.PlaybackBackendEvent, 1)
	unsubscribe := player.SubscribePlaybackEvents(func(event listenplayback.PlaybackBackendEvent) { events <- event })
	defer unsubscribe()
	player.handlePlaybackPayload(map[string]any{
		"type":        "state",
		"state":       "playing",
		"videoId":     "AbCdEfGh123",
		"currentTime": float64(18),
		"duration":    float64(120),
	})
	event := <-events
	if event.Provider != listenplayback.PlaybackProviderYouTube || event.SessionID != "video-session" ||
		event.State != listenplayback.PlaybackStatePlaying || event.Position != 18 || event.Duration != 120 ||
		event.Volume != .65 || !event.HasTiming || !event.HasVolume {
		t.Fatalf("playback event = %+v", event)
	}
	status := player.Status()
	if status.Provider != listenplayback.PlaybackProviderYouTube || status.SessionID != "video-session" {
		t.Fatalf("public status identity = %+v", status)
	}
}

func TestListenYouTubeLivePlayerResetPublishesOriginalIdentity(t *testing.T) {
	player := &ListenYouTubeLivePlayer{
		currentVideo:      "AbCdEfGh123",
		currentState:      "playing",
		targetVolume:      .5,
		playbackProvider:  listenplayback.PlaybackProviderYouTube,
		playbackSessionID: "video-session",
		playbackListeners: make(map[uint64]listenplayback.PlaybackBackendEventListener),
	}
	events := make(chan listenplayback.PlaybackBackendEvent, 1)
	unsubscribe := player.SubscribePlaybackEvents(func(event listenplayback.PlaybackBackendEvent) { events <- event })
	defer unsubscribe()
	if err := player.ResetPlaybackSession(listenplayback.PlaybackProviderYouTube, "video-session"); err != nil {
		t.Fatalf("ResetPlaybackSession: %v", err)
	}
	event := <-events
	if event.Provider != listenplayback.PlaybackProviderYouTube || event.SessionID != "video-session" || event.State != listenplayback.PlaybackStateIdle {
		t.Fatalf("reset event identity = %+v", event)
	}
	status := player.Status()
	if status.Provider != "" || status.SessionID != "" || status.State != "idle" {
		t.Fatalf("reset status = %+v", status)
	}
}

func TestListenYouTubeLivePlayerRejectsStaleVideoEventsAfterProviderHandoff(t *testing.T) {
	player := &ListenYouTubeLivePlayer{
		currentVideo:      "ZyXwVuTs987",
		currentState:      "loading",
		targetVolume:      1,
		playbackProvider:  listenplayback.PlaybackProviderYouTube,
		playbackSessionID: "new-video",
		playbackListeners: make(map[uint64]listenplayback.PlaybackBackendEventListener),
	}
	events := make(chan listenplayback.PlaybackBackendEvent, 1)
	unsubscribe := player.SubscribePlaybackEvents(func(event listenplayback.PlaybackBackendEvent) { events <- event })
	defer unsubscribe()
	player.handlePlaybackPayload(map[string]any{
		"type":             "state",
		"state":            "paused",
		"videoId":          "ZyXwVuTs987",
		"requestedVideoId": "ZyXwVuTs987",
		"observedVideoId":  "AbCdEfGh123",
		"title":            "Unexpected autoplay video",
		"trackChanged":     true,
		"autonavBlocked":   true,
	})
	select {
	case event := <-events:
		t.Fatalf("stale playback event was reattributed: %+v", event)
	case <-time.After(20 * time.Millisecond):
	}
	status := player.Status()
	if status.State != "loading" || status.Title == "Unexpected autoplay video" || status.VideoID != "ZyXwVuTs987" {
		t.Fatalf("blocked autonav event changed requested playback state: %+v", status)
	}
}

func TestListenYouTubeLivePlayerPublishesDetectedControlsAndSelections(t *testing.T) {
	player := &ListenYouTubeLivePlayer{
		currentVideo:      "AbCdEfGh123",
		currentState:      "loading",
		targetVolume:      1,
		playbackProvider:  listenplayback.PlaybackProviderYouTube,
		playbackSessionID: "video-session",
	}
	player.handlePlaybackPayload(map[string]any{
		"type":              "state",
		"state":             "playing",
		"videoId":           "AbCdEfGh123",
		"volume":            float64(.45),
		"muted":             true,
		"controls":          map[string]any{"like": true, "captions": true, "audioTrack": true, "quality": true, "volume": true, "playbackRate": true},
		"captionOptions":    []any{map[string]any{"id": "en", "label": "English"}},
		"audioTrackOptions": []any{map[string]any{"id": "en-0", "label": "English"}},
		"qualityOptions":    []any{map[string]any{"id": "hd1080", "label": "hd1080"}},
		"playbackRateOptions": []any{
			map[string]any{"id": "1", "label": "1x"},
			map[string]any{"id": "1.5", "label": "1.5x"},
		},
		"selections": map[string]any{
			"rating":         "like",
			"captionId":      "en",
			"audioTrackId":   "en-0",
			"qualityId":      "hd1080",
			"playbackRateId": "1.5",
		},
	})
	status := player.Status()
	if !status.Controls.Like || status.Controls.Dislike || !status.Controls.Captions ||
		!status.Controls.AudioTrack || !status.Controls.Quality || !status.Controls.Volume ||
		!status.Controls.PlaybackRate {
		t.Fatalf("controls = %+v", status.Controls)
	}
	if status.Selections.Rating != "like" || status.Selections.CaptionID != "en" ||
		status.Selections.AudioTrackID != "en-0" || status.Selections.QualityID != "hd1080" ||
		status.Selections.PlaybackRateID != "1.5" {
		t.Fatalf("selections = %+v", status.Selections)
	}
	if len(status.CaptionOptions) != 1 || len(status.AudioTrackOptions) != 1 || len(status.QualityOptions) != 1 ||
		len(status.PlaybackRateOptions) != 2 ||
		status.Volume != 1 || status.Muted {
		t.Fatalf("status = %+v", status)
	}
}

func TestListenLivePlayerControlSessionRejectsStaleIdentityBeforeMutation(t *testing.T) {
	player := &ListenYouTubeLivePlayer{
		targetVolume:      .8,
		controls:          ListenLivePlayerControls{Volume: true},
		playbackProvider:  listenplayback.PlaybackProviderYouTube,
		playbackSessionID: "active-session",
	}
	handler := NewListenLivePlayerHandler(player)
	err := handler.ControlSession(context.Background(), ListenLivePlaybackControlRequest{
		Provider:  listenplayback.PlaybackProviderYouTube,
		SessionID: "stale-session",
		Command:   listenLiveControlSetVolume,
		Volume:    .2,
		Muted:     true,
	})
	if err == nil || !strings.Contains(err.Error(), "stale") {
		t.Fatalf("stale command error = %v", err)
	}
	status := player.Status()
	if status.Volume != .8 || status.Muted {
		t.Fatalf("stale command mutated player = %+v", status)
	}
}

func TestListenLiveControlBridgeUsesVerifiedFeatureDetection(t *testing.T) {
	script := listenYouTubeLiveBridgeScript(ListenPlayerPlayRequest{VideoID: "AbCdEfGh123"})
	for _, expected := range []string{
		"getLikeStatus",
		".ytp-like-button",
		`getOption("captions", "tracklist")`,
		"getAvailableAudioTracks",
		"setAudioTrack",
		"setPlaybackQualityRange",
		"getAvailablePlaybackRates",
		"setPlaybackRate",
		"verifyControlLater",
		"control-result",
	} {
		if !strings.Contains(script, expected) {
			t.Fatalf("live control bridge should contain %q", expected)
		}
	}
	if strings.Contains(script, "api.setPlaybackQuality(") {
		t.Fatal("live control bridge must not rely on deprecated setPlaybackQuality")
	}
}

func TestListenLiveControlBridgeKeepsCaptionsSelectableAfterOff(t *testing.T) {
	script := listenYouTubeLiveBridgeScript(ListenPlayerPlayRequest{VideoID: "AbCdEfGh123"})
	for _, expected := range []string{
		"let captionTrackCache = [];",
		"captionTrackCache = rawOptions.slice();",
		"rawOptions = captionTrackCache.slice();",
		"const data = playerVideoData();",
		`failedControls.delete("captions");`,
		`else if (key !== "captions") failedControls.add(key);`,
		`Number.isInteger(index) && index >= 0 ? "caption-" + index : ""`,
		`enabled: button ? buttonPressed : Boolean(currentID)`,
		`const shouldEnable = command === "toggle-captions" ? !before.enabled : Boolean(value);`,
		`selected.button.getAttribute("aria-pressed") === "true"`,
		`const stateMatches = shouldEnable ? after.enabled : !after.enabled;`,
		`const trackMatches = !shouldEnable || !targetID || after.currentID === targetID;`,
	} {
		if !strings.Contains(script, expected) {
			t.Fatalf("caption bridge should contain %q", expected)
		}
	}
	if strings.Contains(script, `track.lang || ("caption-" + index)`) {
		t.Fatal("an empty YouTube caption track must represent Off, not a synthetic selected track")
	}
	start := strings.Index(script, "function captionSnapshot()")
	if start < 0 {
		t.Fatal("caption snapshot source missing")
	}
	end := strings.Index(script[start:], "function audioSnapshot()")
	if end < 0 {
		t.Fatal("caption snapshot source missing")
	}
	captionSnapshot := script[start : start+end]
	if !strings.Contains(captionSnapshot, "available: selectable,") {
		t.Fatal("caption capability must require selectable caption tracks")
	}
	if !strings.Contains(captionSnapshot, `typeof api.setOption === "function" && options.length > 0`) {
		t.Fatal("caption capability must require a non-empty selectable track list")
	}
	if strings.Contains(captionSnapshot, "available: domAvailable || selectable") {
		t.Fatal("a visible native CC button must not advertise captions without tracks")
	}
	if strings.Contains(captionSnapshot, "enabled: Boolean(buttonPressed || currentID)") {
		t.Fatal("a selected caption track must not be reported as visible while YouTube CC is off")
	}
	observed := strings.Index(captionSnapshot, "data && (data.video_id || data.videoId)")
	urlFallback := strings.Index(captionSnapshot, "videoIdFromURL()")
	if observed < 0 || urlFallback < 0 || observed > urlFallback {
		t.Fatal("caption cache must prefer the observed player video over the page URL")
	}
}

func TestListenLiveVolumeControlDoesNotDependOnTransientPageCapability(t *testing.T) {
	player := &ListenYouTubeLivePlayer{
		currentVideo: "AbCdEfGh123",
		controls:     ListenLivePlayerControls{Volume: false},
	}
	request := ListenLivePlaybackControlRequest{
		Command: listenLiveControlSetVolume,
		Volume:  .35,
	}
	if err := player.validateYouTubeControlLocked(request); err != nil {
		t.Fatalf("bridge-backed volume rejected after a transient page snapshot: %v", err)
	}

	player.currentVideo = ""
	if err := player.validateYouTubeControlLocked(request); err == nil {
		t.Fatal("volume should remain unavailable without a loaded YouTube video")
	}
}

func TestListenLiveVolumeBridgeStaysAvailableDuringMediaReplacement(t *testing.T) {
	script := listenYouTubeLiveBridgeScript(ListenPlayerPlayRequest{VideoID: "AbCdEfGh123"})
	for _, expected := range []string{
		`const bridgeAvailable = Boolean(validYouTubeVideoId(request.videoId || currentRequestVideoId()));`,
		`available: bridgeAvailable`,
		`volume: volume.available`,
		`key === "volume"`,
	} {
		if !strings.Contains(script, expected) {
			t.Fatalf("volume bridge should contain %q", expected)
		}
	}
	if strings.Contains(script, `volume: volume.available && !failedControls.has("volume")`) {
		t.Fatal("a transient volume verification failure must not permanently disable the bridge")
	}
}

func TestListenLivePlayerPlaybackRateControlRequiresAdvertisedRate(t *testing.T) {
	player := &ListenYouTubeLivePlayer{
		controls:            ListenLivePlayerControls{PlaybackRate: true},
		playbackRateOptions: []ListenLivePlayerOption{{ID: "1", Label: "1x"}, {ID: "1.5", Label: "1.5x"}},
	}
	if err := player.validateYouTubeControlLocked(ListenLivePlaybackControlRequest{
		Command: listenLiveControlPlaybackRate,
		Value:   "1.5",
	}); err != nil {
		t.Fatalf("advertised playback rate rejected: %v", err)
	}
	if err := player.validateYouTubeControlLocked(ListenLivePlaybackControlRequest{
		Command: listenLiveControlPlaybackRate,
		Value:   "3",
	}); err == nil {
		t.Fatal("unadvertised playback rate should be rejected")
	}
}
