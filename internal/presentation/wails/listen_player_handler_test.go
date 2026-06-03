package wails

import (
	"context"
	"strings"
	"testing"
	"time"

	appcookies "xiadown/internal/application/cookies"
	"xiadown/internal/application/listenplayback"
	"xiadown/internal/application/youtubemusic"
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

func TestListenPlaybackMetadataMapsPlainAPITextToMetadataArtistSource(t *testing.T) {
	track := listenPlaybackTrackFromMetadata(youtubemusic.TrackMetadata{
		VideoID:      "CPONUbyJ3YM",
		Title:        "You are my magic",
		Channel:      "Accusefive",
		ArtistSource: "api-text",
	}, "CPONUbyJ3YM")

	if track.Artist != "Accusefive" || track.ArtistSource != listenplayback.TrackArtistSourceAPIMetadata {
		t.Fatalf("expected API metadata artist source, got %#v", track)
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
	if strings.Contains(script, `videoAvailabilitySnapshot`) ||
		strings.Contains(script, `videoAvailable: videoAvailability.available`) ||
		strings.Contains(script, `videoAvailabilityKnown: videoAvailability.known`) ||
		strings.Contains(script, `return { known: true, available: false };`) {
		t.Fatalf("bridge script should not report YouTube Music video availability from the DOM")
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
		"installMediaSessionActionHandlerGuard",
		`setActionHandler("seekforward", null)`,
		`setActionHandler("seekbackward", null)`,
		`type === "seekforward" || type === "seekbackward"`,
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

func TestListenBridgeAppliesAndReportsPlaybackAudioQuality(t *testing.T) {
	script := listenYouTubeMusicBridgeScript(ListenPlayerPlayRequest{
		VideoID:              "TESTVID001A",
		PlaybackAudioQuality: "AUDIO_QUALITY_HIGH",
	})
	for _, expected := range []string{
		`setAudioQuality`,
		`AUDIO_QUALITY_HIGH`,
		`audioQualityFromItag`,
		`xiadownPlaybackAudioQuality`,
		`__xiadownApplyPlaybackAudioQuality`,
		`getPreferredAudioQuality`,
		`PLAYBACK_AUDIO_QUALITY_OBSERVED`,
		`lastObservedAudioQualityKey`,
		`postObservedPlaybackAudioQuality(observedPlaybackAudioQuality(players))`,
		`getStatsForNerds`,
		`document.readyState !== "loading"`,
	} {
		if !strings.Contains(script, expected) {
			t.Fatalf("bridge script should contain %q", expected)
		}
	}
	if strings.Contains(script, `document.documentElement || document.body || document.readyState !== "loading"`) {
		t.Fatalf("bridge script should not boot before document-end just because documentElement exists")
	}
	if strings.Contains(script, "AUDIO_QUALITY_PROBE") {
		t.Fatalf("bridge script should not contain audio quality probe polling")
	}
	for _, redundant := range []string{
		strings.Join([]string{"PLAYBACK", "AUDIO", "QUALITY", "STATS"}, "_"),
		strings.Join([]string{"AUDIO", "QUALITY", "STATS", "MIN", "INTERVAL", "MS"}, "_"),
		"last" + "AudioQuality" + "StatsKey",
		"getAvailable" + "AudioQualityLevels",
	} {
		if strings.Contains(script, redundant) {
			t.Fatalf("bridge script should not retain redundant stats detail %q", redundant)
		}
	}
	if strings.Contains(script, `[XiaDown][AudioQuality]`) {
		t.Fatalf("bridge script should not log audio quality stats through console")
	}
	for _, suffix := range []string{"audioFormat", "audioQuality", "playbackQuality"} {
		ambiguous := strings.Join([]string{"debug", suffix}, "_")
		if strings.Contains(script, ambiguous) {
			t.Fatalf("bridge script should normalize ambiguous stats key %q", ambiguous)
		}
	}
}

func TestListenObservedPlaybackAudioQuality(t *testing.T) {
	observed, videoID, ok := listenObservedPlaybackAudioQuality(map[string]any{
		"type":     "PLAYBACK_AUDIO_QUALITY_OBSERVED",
		"observed": "AUDIO_QUALITY_MEDIUM",
		"videoId":  "fcnDmrtj6Sk",
	})
	if !ok || observed != "AUDIO_QUALITY_MEDIUM" || videoID != "fcnDmrtj6Sk" {
		t.Fatalf("expected medium audio quality stats, got observed=%q videoID=%q ok=%t", observed, videoID, ok)
	}

	_, _, ok = listenObservedPlaybackAudioQuality(map[string]any{
		"type":     "PLAYBACK_AUDIO_QUALITY_OBSERVED",
		"observed": "AUDIO_QUALITY_AUTO",
		"videoId":  "fcnDmrtj6Sk",
	})
	if ok {
		t.Fatalf("auto should not be treated as observed audio quality")
	}
}

func TestListenBridgeAttemptsAutoplayRecoveryOnReadyMedia(t *testing.T) {
	script := listenYouTubeMusicBridgeScript(ListenPlayerPlayRequest{
		VideoID: "TESTVID001A",
	})
	for _, expected := range []string{
		"autoplayRecoveryPending",
		"function attemptAutoplayRecovery(video, reason)",
		"function shouldDelayPlaybackForStartPosition(video)",
		`sendState(reason || "autoplay-waiting-for-start-position", true)`,
		`attemptAutoplayRecovery(video, "autoplay-recovery-" + name)`,
		`video.readyState >= 3`,
		`".play-pause-button.ytmusic-player-bar, ytmusic-player-bar .play-pause-button"`,
	} {
		if !strings.Contains(script, expected) {
			t.Fatalf("bridge script should include autoplay recovery %q", expected)
		}
	}
}

func TestListenBridgeUsesVideoPausedForPlayingState(t *testing.T) {
	script := listenYouTubeMusicBridgeScript(ListenPlayerPlayRequest{
		VideoID: "TESTVID001A",
	})
	activeMediaIndex := strings.Index(script, `if (media && media.playing)`)
	playSettlingIndex := strings.Index(script, `if (lastRequestedAction === "play")`)
	if activeMediaIndex < 0 || playSettlingIndex < 0 || activeMediaIndex > playSettlingIndex {
		t.Fatalf("bridge script should require active media before play-request loading fallback")
	}
	for _, expected := range []string{
		`function playerApiMediaSnapshot()`,
		`function effectiveMediaSnapshot(video, videoId)`,
		`function advertisingMediaSnapshot(video, fallback)`,
		`const payloadMedia = ad.advertising ? advertisingMediaSnapshot(video, media) : media`,
		`paused: payloadMedia.paused`,
		`ended: payloadMedia.ended`,
		`if (lastRequestedAction === "play") {
      return media && (media.currentTime > 0.15 || media.bufferedTime > 0.15) ? "buffering" : "loading";
    }`,
	} {
		if !strings.Contains(script, expected) {
			t.Fatalf("bridge script should expose media element playback truth %q", expected)
		}
	}
	if strings.Contains(script, `if (apiState === 1) return "playing";`) {
		t.Fatalf("bridge script should not trust the YouTube API playing state while the media element is paused")
	}
}

func TestListenBridgeAutoplayRecoveryDoesNotTrustStalePlayerAPI(t *testing.T) {
	script := listenYouTubeMusicBridgeScript(ListenPlayerPlayRequest{
		VideoID: "TESTVID001A",
	})
	for _, expected := range []string{
		`if (!video || !video.paused)`,
		`autoplayRecoveryPending = false;
    lastRequestedAction = "play";`,
		`const result = video.play();`,
		`const recovery = attemptAutoplayRecovery(video, "autoplay-recovery-timer");`,
		`if (recovery === "clicked" || recovery === "played")`,
	} {
		if !strings.Contains(script, expected) {
			t.Fatalf("bridge script should retry paused autoplay recovery from media state; missing %q", expected)
		}
	}
	if strings.Contains(script, `!video.paused || playerStateCode() === 1`) {
		t.Fatalf("bridge autoplay recovery should not skip a paused video because the player API reports playing")
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
		status.CurrentTime != 12.5 ||
		status.Duration != 180 ||
		status.BufferedTime != 48 {
		t.Fatalf("status should include observed playback details, got %+v", status)
	}
}

func TestListenPlayerStatusFallsBackToYouTubePoster(t *testing.T) {
	player := &ListenYouTubeMusicPlayer{
		currentVideo: "TESTVID001A",
		currentState: "playing",
	}

	status := player.Status()
	if status.ThumbnailURL != "https://i.ytimg.com/vi/TESTVID001A/hqdefault.jpg" {
		t.Fatalf("expected public YouTube thumbnail fallback, got %+v", status)
	}
}

func TestListenPlaybackPayloadUsesPausedMediaElement(t *testing.T) {
	if listenPlaybackPayloadIsPlaying("playing", map[string]any{"paused": true}) {
		t.Fatal("expected paused media element to override playing state")
	}
	if listenPlaybackPayloadIsPlaying("buffering", map[string]any{"ended": true}) {
		t.Fatal("expected ended media element to override buffering state")
	}
	if !listenPlaybackPayloadIsPlaying("buffering", map[string]any{"paused": false}) {
		t.Fatal("expected active buffering media element to count as playing")
	}
	if !listenPlaybackPayloadIsPlaying("playing", map[string]any{}) {
		t.Fatal("expected missing media element fields to preserve legacy playing state")
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
		"ENFORCE_BURST_MS",
		"ENFORCE_HEARTBEAT_MS",
		"MutationObserver(scheduleDeferredEnforce)",
		"__listenVideoModeCleanup",
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

func TestListenRawMusicBufferingSyncKeepsPlaybackPlaying(t *testing.T) {
	ctx := context.Background()
	transport := &listenPlaybackTestTransport{}
	service := listenplayback.NewPlayerService(
		transport,
		listenplayback.WithUserInteractionUnlocked(),
	)
	tracks := []listenplayback.Track{
		{ID: "one", VideoID: "video-one", Title: "One", Artist: "Artist"},
	}
	if err := service.PlayQueue(ctx, tracks, 0, "Queue"); err != nil {
		t.Fatal(err)
	}
	if err := service.UpdatePlaybackState(ctx, true, 12, 180); err != nil {
		t.Fatal(err)
	}

	player := NewListenYouTubeMusicPlayer(nil, nil, nil)
	player.syncPlaybackServiceFromNativeEvent(
		service,
		"state",
		"buffering",
		"video-one",
		"One",
		"Artist",
		"",
		"",
		false,
		map[string]any{"currentTime": 14, "duration": 180},
	)

	if service.State() != listenplayback.PlaybackStatePlaying {
		t.Fatalf("expected buffering sync to keep playback playing, got %s", service.State())
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

func TestListenSameVideoResumeUpdatesStoredPlaybackRequest(t *testing.T) {
	script := listenYouTubeMusicSameVideoResumeScript(ListenPlayerPlayRequest{
		VideoID:              "TESTVID001A",
		StartSeconds:         12,
		Volume:               0.42,
		PlaybackAudioQuality: "AUDIO_QUALITY_HIGH",
	})
	for _, expected := range []string{
		`api.request(request)`,
		`"playbackAudioQuality":"AUDIO_QUALITY_HIGH"`,
		`xiadownPlaybackAudioQuality`,
		`__listenPlaybackRequest`,
		`api.volume(request.volume, request.muted)`,
		`video.currentTime = start`,
		`api.play()`,
	} {
		if !strings.Contains(script, expected) {
			t.Fatalf("same-video resume should contain %q", expected)
		}
	}
}

func TestListenBridgeDoesNotExposeSkipAdControls(t *testing.T) {
	for _, script := range []string{
		listenYouTubeMusicBridgeScript(ListenPlayerPlayRequest{VideoID: "TESTVID001A"}),
		listenYouTubeLiveBridgeScript(ListenPlayerPlayRequest{VideoID: "TESTVID002B"}),
	} {
		for _, removed := range []string{
			"skipAd",
			"invokeSkipAd",
			"adSkippable",
			"adSkipLabel",
			"PointerEvent",
		} {
			if strings.Contains(script, removed) {
				t.Fatalf("bridge script should not expose removed skip-ad logic %q", removed)
			}
		}
		for _, expected := range []string{
			"advertising",
			"adSnapshot",
			"visibleAdElements",
			"adLabel",
		} {
			if !strings.Contains(script, expected) {
				t.Fatalf("bridge script should keep ad status signal %q", expected)
			}
		}
	}
}

func TestListenYouTubeAdBlockScriptPrunesPlayerAdFields(t *testing.T) {
	script := listenYouTubeAdBlockScript()
	for _, expected := range []string{
		"music.youtube.com",
		"www.youtube.com",
		"__xiadownYouTubeAdBlockerInstalled",
		"ytInitialPlayerResponse",
		"JSON.parse",
		"Response.prototype.json",
		"XMLHttpRequest.prototype.send",
		"__xiadownDisableYouTubeAdBlock",
		"__xiadownYouTubeAdBlockDisabledUntil",
		"Date.now() + duration",
		"disabledUntil() > Date.now()",
		"adPlacements",
		"adSlots",
		"playerAds",
		"reelWatchSequenceResponse",
		"isAdReelEntry",
	} {
		if !strings.Contains(script, expected) {
			t.Fatalf("adblock script should contain %q", expected)
		}
	}
	for _, removed := range []string{
		"clickVisibleSkipButton",
		"skipAd",
	} {
		if strings.Contains(script, removed) {
			t.Fatalf("adblock script should not contain skip-click logic %q", removed)
		}
	}
}

func TestListenYouTubeMusicBridgeFallsBackToUnfilteredAds(t *testing.T) {
	script := listenYouTubeMusicBridgeScript(ListenPlayerPlayRequest{VideoID: "TESTVID001A"})
	for _, expected := range []string{
		"AD_FILTER_FALLBACK_STUCK_MS",
		"advertisingMediaSnapshot",
		"maybeReloadWithUnfilteredAds",
		"__xiadownYouTubeAdBlockDisabledUntil",
		"ad-filter-fallback-reload",
		"window.location.reload",
	} {
		if !strings.Contains(script, expected) {
			t.Fatalf("music bridge should contain ad fallback logic %q", expected)
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
