package wails

import (
	"context"
	"strings"
	"testing"
	"time"

	appcookies "xiadown/internal/application/cookies"
	"xiadown/internal/application/listenplayback"
)

func TestFilterListenPlaybackCookiesKeepsUsableYouTubeCookies(t *testing.T) {
	now := time.Unix(1_700_000_000, 0)
	records := []appcookies.Record{
		{Name: "SID", Value: "sid", Domain: ".youtube.com", Path: "/", Expires: now.Add(time.Hour).Unix(), Secure: true},
		{Name: "expired", Value: "gone", Domain: ".youtube.com", Path: "/", Expires: now.Add(-time.Hour).Unix()},
		{Name: "missing-domain", Value: "value", Path: "/"},
		{Name: "SID", Value: "duplicate", Domain: ".youtube.com", Path: "/", Expires: now.Add(time.Hour).Unix()},
	}

	cookies := filterListenPlaybackCookies(records, now)
	if len(cookies) != 1 {
		t.Fatalf("expected one usable cookie, got %d", len(cookies))
	}
	if cookies[0].Name != "SID" || cookies[0].Value != "sid" {
		t.Fatalf("unexpected cookie: %+v", cookies[0])
	}
}

func TestListenBridgePreservesPauseIntent(t *testing.T) {
	script := listenYouTubeMusicBridgeScript(ListenPlayerPlayRequest{
		VideoID: "TESTVID001A",
	})
	expected := `if (!video.ended && lastRequestedAction !== "pause") lastRequestedAction = "";`
	if !strings.Contains(script, expected) {
		t.Fatalf("bridge script should keep pause intent until a play request replaces it")
	}
	if !strings.Contains(script, `play-blocked-after-pause`) ||
		!strings.Contains(script, `playing-blocked-after-pause`) {
		t.Fatalf("bridge script should block late play events after pause")
	}
	if !strings.Contains(script, `observedVideoId`) ||
		!strings.Contains(script, `trackChanged`) ||
		!strings.Contains(script, `track-ended`) {
		t.Fatalf("bridge script should report observed tracks and explicit track-ended events")
	}
	playSettlingIndex := strings.Index(script, `if (lastRequestedAction === "play")`)
	pausedAPIIndex := strings.Index(script, `if (apiState === 2) return "paused";`)
	if playSettlingIndex < 0 || pausedAPIIndex < 0 || playSettlingIndex > pausedAPIIndex {
		t.Fatalf("bridge script should not report paused while a play request is still settling")
	}
	if !strings.Contains(script, `sendState("stale-video-ended", true)`) {
		t.Fatalf("bridge script should ignore ended events while another video is still playing")
	}
	if !strings.Contains(script, `otherVideoPlaying(video)`) {
		t.Fatalf("bridge script should not suppress the current video's ended event only because the API is lagging")
	}
	if !strings.Contains(script, `videoAvailabilitySnapshot`) ||
		!strings.Contains(script, `videoAvailable: videoAvailability.available`) ||
		!strings.Contains(script, `videoAvailabilityKnown: videoAvailability.known`) ||
		!strings.Contains(script, `ytmusic-av-switcher #video-button`) {
		t.Fatalf("bridge script should report YouTube Music video availability from the DOM")
	}
	if !strings.Contains(script, "pauseVideo") {
		t.Fatalf("bridge pause path should use the YouTube player API when available")
	}
	if !strings.Contains(script, `document.querySelectorAll("video")`) ||
		!strings.Contains(script, "pauseVideos()") {
		t.Fatalf("bridge pause path should consider all video elements")
	}
	for _, expected := range []string{
		"navigator.mediaSession.setActionHandler",
		"scheduleMediaSessionOverrideLoop",
		"window.requestAnimationFrame",
		`post({ type: "remote-next" })`,
		`post({ type: "remote-previous" })`,
		`type: "lyrics-time"`,
		"startLyricsPoll",
		"stopLyricsPoll",
	} {
		if !strings.Contains(script, expected) {
			t.Fatalf("bridge script should override media session action %q", expected)
		}
	}
}

func TestListenPlayerStatusPrefersObservedMetadata(t *testing.T) {
	player := &ListenYouTubeMusicPlayer{
		currentVideo:   "requested-id",
		currentState:   "playing",
		requestTitle:   "Requested title",
		requestArtist:  "Requested artist",
		observedVideo:  "observed-id",
		observedTitle:  "Observed title",
		observedArtist: "Observed artist",
		observedThumb:  "https://example.test/thumb.jpg",
		videoAvailable: true,
		videoKnown:     true,
		currentTime:    12.5,
		duration:       180,
		bufferedTime:   48,
	}

	status := player.Status()
	if status.Title != "Observed title" || status.Artist != "Observed artist" {
		t.Fatalf("status should prefer observed metadata, got title=%q artist=%q", status.Title, status.Artist)
	}
	if status.VideoID != "requested-id" || status.ObservedVideoID != "observed-id" {
		t.Fatalf("status should include requested and observed ids, got %+v", status)
	}
	if status.ThumbnailURL != "https://example.test/thumb.jpg" ||
		!status.VideoAvailable ||
		!status.VideoKnown ||
		status.CurrentTime != 12.5 ||
		status.Duration != 180 ||
		status.BufferedTime != 48 {
		t.Fatalf("status should include observed playback details, got %+v", status)
	}
}

func TestListenPauseScriptOnlyPausesVideoElement(t *testing.T) {
	script := listenYouTubeMusicPauseScript()
	if !strings.Contains(script, `document.querySelectorAll("video")`) {
		t.Fatalf("pause script should pause HTML video elements")
	}
	if strings.Contains(script, ".pauseVideo") || strings.Contains(script, "pauseVideo()") {
		t.Fatalf("pause script should not call YouTube internal pause APIs")
	}
}

func TestListenAirPlayScriptUsesWebKitPlaybackTargetPicker(t *testing.T) {
	script := listenYouTubeMusicAirPlayScript()
	if !strings.Contains(script, "webkitShowPlaybackTargetPicker") {
		t.Fatalf("airplay script should use WebKit playback target picker")
	}
	if !strings.Contains(script, "__listenNativePlayer") {
		t.Fatalf("airplay script should prefer the bridge API")
	}
}

func TestListenVideoModeScriptUsesYouTubeMusicVideoMode(t *testing.T) {
	script := listenYouTubeMusicVideoModeScript()
	for _, expected := range []string{
		"ytmusic-av-switcher",
		"#video-button",
		"ytmusic-player-page",
		"listen-video-visible",
		"video.listen-video-visible",
		"let current = video",
		"current.classList.add(\"listen-video-visible\")",
		"requestAnimationFrame(enforce)",
	} {
		if !strings.Contains(script, expected) {
			t.Fatalf("video mode script should contain %q", expected)
		}
	}
}

func TestListenMusicWatchURLCarriesLocale(t *testing.T) {
	targetURL := listenYouTubeMusicWatchURL("TESTVID001A", "zh-CN")
	for _, expected := range []string{
		"https://music.youtube.com/watch?",
		"v=TESTVID001A",
		"hl=zh-CN",
		"persist_hl=1",
	} {
		if !strings.Contains(targetURL, expected) {
			t.Fatalf("watch url should contain %q, got %s", expected, targetURL)
		}
	}
}

func TestListenMusicBridgeAppliesVolumeBeforeAutoplay(t *testing.T) {
	script := listenYouTubeMusicBridgeScript(ListenPlayerPlayRequest{
		VideoID: "TESTVID001A",
		Volume:  0.42,
	})
	for _, expected := range []string{
		"installVolumeGuards();",
		"patchMediaElementVolumeProperty(\"volume\")",
		"patchMediaElementPlay()",
		"currentVolumeState()",
		"scheduleVolumeBurst();",
		"const bootMetadata = metadataSnapshot();",
		`video.addEventListener("volumechange"`,
		"const volumeBurst = window.setInterval",
		"request: (next) =>",
		"sendState(\"api-request\", true)",
		"next.forceReload = next.forceReload === true",
	} {
		if !strings.Contains(script, expected) {
			t.Fatalf("music bridge should contain %q", expected)
		}
	}
}

func TestListenMusicNextPreviousUseBridgeVolumeGuard(t *testing.T) {
	nextScript := listenYouTubeMusicNextScript()
	for _, expected := range []string{
		`const api = window.__listenNativePlayer`,
		`typeof api.next === "function"`,
		`api.next();`,
	} {
		if !strings.Contains(nextScript, expected) {
			t.Fatalf("next script should contain %q", expected)
		}
	}

	previousScript := listenYouTubeMusicPreviousScript()
	for _, expected := range []string{
		`const api = window.__listenNativePlayer`,
		`typeof api.previous === "function"`,
		`api.previous();`,
	} {
		if !strings.Contains(previousScript, expected) {
			t.Fatalf("previous script should contain %q", expected)
		}
	}
}

func TestListenMusicBridgeRetriesStartSeekUntilObserved(t *testing.T) {
	script := listenYouTubeMusicBridgeScript(ListenPlayerPlayRequest{
		VideoID:      "TESTVID001A",
		StartSeconds: 12,
	})
	for _, expected := range []string{
		"START_POSITION_MAX_ATTEMPTS",
		"startApplyAttemptCount += 1",
		"apply-start-seconds-timeout",
		"Math.abs(current - start) <= START_POSITION_TOLERANCE_SECONDS || current > start",
		"applyStartPosition(video);\n        sendState(\"seeked\", true)",
	} {
		if !strings.Contains(script, expected) {
			t.Fatalf("music bridge should retry start seek until observed; missing %q", expected)
		}
	}
}

type listenPlaybackTestTransport struct {
	currentVideoID string
	loads          []string
}

func (transport *listenPlaybackTestTransport) LoadVideo(_ context.Context, request listenplayback.PlayRequest, _ listenplayback.VideoLoadStrategy) error {
	transport.currentVideoID = request.Track.VideoID
	transport.loads = append(transport.loads, request.Track.VideoID)
	return nil
}

func (transport *listenPlaybackTestTransport) Play(context.Context) error {
	return nil
}

func (transport *listenPlaybackTestTransport) Pause(context.Context) error {
	return nil
}

func (transport *listenPlaybackTestTransport) Seek(context.Context, float64) error {
	return nil
}

func (transport *listenPlaybackTestTransport) SetVolume(context.Context, float64, bool) error {
	return nil
}

func (transport *listenPlaybackTestTransport) Next(context.Context) error {
	return nil
}

func (transport *listenPlaybackTestTransport) Previous(context.Context) error {
	return nil
}

func (transport *listenPlaybackTestTransport) CurrentVideoID(context.Context) string {
	return transport.currentVideoID
}

func TestListenRawMusicTrackEndedSyncsPlaybackServiceDirectly(t *testing.T) {
	ctx := context.Background()
	transport := &listenPlaybackTestTransport{}
	service := listenplayback.NewPlayerService(
		transport,
		listenplayback.WithUserInteractionUnlocked(),
	)
	tracks := []listenplayback.Track{
		{ID: "one", VideoID: "video-one", Title: "One", Artist: "Artist"},
		{ID: "two", VideoID: "video-two", Title: "Two", Artist: "Artist"},
	}
	if err := service.PlayQueue(ctx, tracks, 0, "Queue"); err != nil {
		t.Fatal(err)
	}
	player := NewListenYouTubeMusicPlayer(nil, nil, nil)
	player.syncPlaybackServiceFromNativeEvent(
		service,
		"track-ended",
		"ended",
		"video-one",
		"One",
		"Artist",
		"",
		"",
		false,
		false,
		false,
		map[string]any{"currentTime": 180, "duration": 180},
	)

	_, index := service.Queue()
	if index != 1 {
		t.Fatalf("expected queue to advance to index 1, got %d", index)
	}
	if got := transport.loads[len(transport.loads)-1]; got != "video-two" {
		t.Fatalf("expected direct sync to load video-two, got %q", got)
	}
}

func TestListenRawMusicSyncReconcilesVideoDriftWithoutTrackChanged(t *testing.T) {
	ctx := context.Background()
	transport := &listenPlaybackTestTransport{}
	service := listenplayback.NewPlayerService(
		transport,
		listenplayback.WithUserInteractionUnlocked(),
	)
	tracks := []listenplayback.Track{
		{ID: "one", VideoID: "video-one", Title: "One", Artist: "Artist"},
		{ID: "two", VideoID: "video-two", Title: "Two", Artist: "Artist"},
	}
	if err := service.PlayQueue(ctx, tracks, 0, "Queue"); err != nil {
		t.Fatal(err)
	}
	service.SetRepeatMode(listenplayback.RepeatModeOne)

	player := NewListenYouTubeMusicPlayer(nil, nil, nil)
	player.syncPlaybackServiceFromNativeEvent(
		service,
		"state",
		"playing",
		"video-two",
		"Two",
		"Artist",
		"",
		"",
		false,
		false,
		false,
		map[string]any{"currentTime": 4, "duration": 180},
	)

	_, index := service.Queue()
	if index != 0 {
		t.Fatalf("expected repeat-one drift recovery to keep index 0, got %d", index)
	}
	if got := transport.loads[len(transport.loads)-1]; got != "video-one" {
		t.Fatalf("expected direct sync to reload intended video-one, got %q", got)
	}
}

func TestListenRawMusicRemoteNextSyncsPlaybackServiceDirectly(t *testing.T) {
	ctx := context.Background()
	transport := &listenPlaybackTestTransport{}
	service := listenplayback.NewPlayerService(
		transport,
		listenplayback.WithUserInteractionUnlocked(),
	)
	tracks := []listenplayback.Track{
		{ID: "one", VideoID: "video-one", Title: "One", Artist: "Artist"},
		{ID: "two", VideoID: "video-two", Title: "Two", Artist: "Artist"},
	}
	if err := service.PlayQueue(ctx, tracks, 0, "Queue"); err != nil {
		t.Fatal(err)
	}
	player := NewListenYouTubeMusicPlayer(nil, nil, nil)
	player.syncPlaybackServiceFromNativeEvent(
		service,
		"remote-next",
		"",
		"",
		"",
		"",
		"",
		"",
		false,
		false,
		false,
		map[string]any{},
	)

	_, index := service.Queue()
	if index != 1 {
		t.Fatalf("expected remote-next to advance to index 1, got %d", index)
	}
	if got := transport.loads[len(transport.loads)-1]; got != "video-two" {
		t.Fatalf("expected remote-next to load video-two, got %q", got)
	}
}

func TestListenSameVideoResumeDoesNotSubmitStoredRequest(t *testing.T) {
	script := listenYouTubeMusicSameVideoResumeScript(ListenPlayerPlayRequest{
		VideoID:      "TESTVID001A",
		StartSeconds: 12,
		Volume:       0.42,
	})
	if strings.Contains(script, "api.request") {
		t.Fatalf("same-video resume should not submit a new playback request")
	}
	if strings.Contains(script, "__listenPlaybackRequest") {
		t.Fatalf("same-video resume should not rewrite persisted playback request")
	}
	for _, expected := range []string{
		`api.volume(request.volume, request.muted)`,
		`video.currentTime = start`,
		`api.play()`,
	} {
		if !strings.Contains(script, expected) {
			t.Fatalf("same-video resume should contain %q", expected)
		}
	}
}

func TestListenSkipAdScriptsUseCurrentYouTubeControls(t *testing.T) {
	musicBridge := listenYouTubeMusicBridgeScript(ListenPlayerPlayRequest{VideoID: "TESTVID001A"})
	liveBridge := listenYouTubeLiveBridgeScript(ListenPlayerPlayRequest{VideoID: "TESTVID002B"})
	fallbacks := []string{
		listenYouTubeMusicSkipAdScript(),
		listenYouTubeLiveSkipAdScript(),
	}
	for _, script := range []string{musicBridge, liveBridge} {
		for _, expected := range []string{
			"typeof api.skipAd",
			".ytp-ad-skip-button-modern",
			".ytp-skip-ad-button",
			"button[aria-label*='跳过']",
			"PointerEvent",
			"pointerdown",
			"const retryButton = skipButton();",
			"skip-ad-confirm",
		} {
			if !strings.Contains(script, expected) {
				t.Fatalf("bridge skip script should contain %q", expected)
			}
		}
		if strings.Contains(script, "api.skipAd();\n        sendState(reason || \"skip-ad\", true)") {
			t.Fatalf("bridge skip script should not return before trying the visible skip button")
		}
	}
	for _, script := range fallbacks {
		for _, expected := range []string{
			".ytp-ad-skip-button-modern",
			".ytp-skip-ad-button",
			"button[aria-label*='跳过']",
			"PointerEvent",
			"pointerdown",
			"clickVisibleSkipButton",
			"window.setTimeout(clickVisibleSkipButton, 180)",
		} {
			if !strings.Contains(script, expected) {
				t.Fatalf("fallback skip script should contain %q", expected)
			}
		}
		if strings.Contains(script, "api.skipAd();\n    return;") {
			t.Fatalf("fallback skip script should not return before trying the visible skip button")
		}
	}
}

func TestListenLivePlayerUsesYouTubeEmbed(t *testing.T) {
	targetURL := listenYouTubeLiveEmbedURL("TESTVID002B", "zh-CN")
	for _, expected := range []string{
		"https://www.youtube.com/embed/TESTVID002B",
		"autoplay=1",
		"enablejsapi=1",
		"hl=zh-CN",
		"origin=https%3A%2F%2Fcom.dreamapp.xiadown",
	} {
		if !strings.Contains(targetURL, expected) {
			t.Fatalf("live embed url should contain %q, got %s", expected, targetURL)
		}
	}
	if strings.Contains(targetURL, "music.youtube.com") {
		t.Fatalf("live embed url should not use YouTube Music: %s", targetURL)
	}
	if strings.Contains(targetURL, "/watch") {
		t.Fatalf("live embed url should not use watch playback: %s", targetURL)
	}
}

func TestListenLiveBridgeReportsRequestedVideoIdentity(t *testing.T) {
	script := listenYouTubeLiveBridgeScript(ListenPlayerPlayRequest{
		VideoID: "TESTVID002B",
		Title:   "Synthetic live radio",
		Artist:  "Listen",
	})
	for _, expected := range []string{
		"listen-youtube-live-player",
		"__listenLivePlayer",
		"movie_player",
		"observedVideoId: metadata.videoId",
		"requestedVideoId: metadata.videoId",
		"advertising",
		"adSnapshot",
		"visibleAdElements",
		"adLabel",
		"adSkippable",
		"skipAd",
		"Object.assign({}, INITIAL_REQUEST, stored, { videoId: initialVideoId })",
		"navigator.mediaSession.setActionHandler",
		"scheduleMediaSessionOverrideLoop",
		"window.requestAnimationFrame",
		`post({ type: "remote-next" })`,
		`post({ type: "remote-previous" })`,
	} {
		if !strings.Contains(script, expected) {
			t.Fatalf("live bridge script should contain %q", expected)
		}
	}
	if strings.Contains(script, "ytmusic-player") {
		t.Fatalf("live bridge should not depend on YouTube Music DOM")
	}
}

func TestListenLiveVideoModeScriptUsesYouTubePlayer(t *testing.T) {
	script := listenYouTubeLiveVideoModeScript()
	for _, expected := range []string{
		"movie_player",
		"listen-live-video-root",
		"listen-live-video-visible",
		"requestAnimationFrame(enforce)",
	} {
		if !strings.Contains(script, expected) {
			t.Fatalf("live video mode script should contain %q", expected)
		}
	}
	if strings.Contains(script, "ytmusic-player") {
		t.Fatalf("live video mode should not depend on YouTube Music DOM")
	}
}

func TestListenLiveVolumeScriptSeparatesVolumeAndMuted(t *testing.T) {
	script := listenYouTubeLiveVolumeScript(0.42, false)
	for _, expected := range []string{
		"api.volume(0.420000, false)",
		"video.volume = volume",
		"video.muted = false",
		"moviePlayer.setVolume(Math.round(volume * 100))",
		"else moviePlayer.unMute()",
	} {
		if !strings.Contains(script, expected) {
			t.Fatalf("live volume script should contain %q", expected)
		}
	}
	if strings.Contains(script, "effectiveVolume") {
		t.Fatalf("live volume script should not collapse muted state into volume")
	}
}
