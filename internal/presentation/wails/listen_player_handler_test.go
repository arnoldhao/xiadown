package wails

import (
	"context"
	"os"
	"strings"
	"testing"
	"time"

	appcookies "xiadown/internal/application/cookies"
	"xiadown/internal/application/listenplayback"
	"xiadown/internal/application/youtubemusic"

	"github.com/wailsapp/wails/v3/pkg/application"
)

func TestListenMusicNativeWindowFullscreenEventConfirmsEnter(t *testing.T) {
	if !listenEmbeddedVideoUsesNativeWindowFullscreen() {
		t.Skip("native player-window fullscreen is not used on this platform")
	}
	window := &application.WebviewWindow{}
	waiter := make(chan bool, 1)
	player := &ListenYouTubeMusicPlayer{
		window:                         window,
		embeddedFullscreenTransition:   true,
		embeddedFullscreenGeneration:   3,
		embeddedNativeWindowFullscreen: true,
		embeddedNativeFullscreenWaiter: waiter,
	}
	player.handleNativeWindowFullscreenEvent(window, true)
	if !player.embeddedFullscreenActive || player.embeddedFullscreenTransition {
		t.Fatal("native enter must transfer presentation ownership to the music player window")
	}
	select {
	case active := <-waiter:
		if !active {
			t.Fatal("native enter waiter received an exit state")
		}
	default:
		t.Fatal("native enter did not complete the music fullscreen waiter")
	}

	player.mu.Lock()
	cleared := player.clearEmbeddedFullscreenStateLocked()
	player.mu.Unlock()
	if cleared != waiter || player.embeddedFullscreenActive ||
		player.embeddedFullscreenTransition || player.embeddedNativeWindowFullscreen {
		t.Fatal("fullscreen cleanup must make a second native enter possible")
	}
}

func TestFilterListenPlaybackCookiesKeepsOnlyStableYouTubeCookies(t *testing.T) {
	now := time.Unix(1_700_000_000, 0)
	records := []appcookies.Record{
		{Name: "SID", Value: "sid", Domain: ".youtube.com", Path: "/", Expires: now.Add(time.Hour).Unix(), Secure: true},
		{Name: "__Secure-1PSIDTS", Value: "rotating", Domain: ".youtube.com", Path: "/", Expires: now.Add(time.Hour).Unix(), Secure: true},
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
	expected := `if (!video.ended && lastRequestedAction !== "pause") {`
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
		`playbackRate: finiteNumber(video.playbackRate, 1)`,
		"startLyricsPoll",
		"stopLyricsPoll",
		`sendLyricsTime("seeked")`,
		`sendLyricsTime("api-seek")`,
		`sendLyricsTime("ratechange")`,
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

func TestListenMusicPrepareLoadLeavesNewPlaybackRequestAsFinalIntent(t *testing.T) {
	script := listenYouTubeMusicPrepareLoadScript(ListenPlayerPlayRequest{VideoID: "TESTVID001A"})
	pause := strings.Index(script, `api.pause()`)
	request := strings.Index(script, `api.request(request)`)
	if pause < 0 || request < 0 || pause > request {
		t.Fatal("switching music videos must pause the old session before recording the new play intent")
	}
}

func TestListenMusicBridgeKeepsLoadPlayIntentAcrossLateHistoryPause(t *testing.T) {
	script := listenYouTubeMusicBridgeScript(ListenPlayerPlayRequest{VideoID: "TESTVID001A"})
	for _, expected := range []string{
		`let playWhenReady = true`,
		`if (!playWhenReady || !autoplayRecoveryPending || lastRequestedAction === "pause")`,
		`playWhenReady = false;`,
		`if (playWhenReady) {`,
		`autoplayRecoveryPending = true;`,
		`scheduleAutoplay();`,
		`activeVideoId !== requestedVideoId`,
		`if (recovery === "not-ready")`,
	} {
		if !strings.Contains(script, expected) {
			t.Fatalf("music bridge must preserve play-when-ready intent; missing %q", expected)
		}
	}
}

func TestListenMusicBridgeExpiresLoadRecoveryAndHonorsNativePause(t *testing.T) {
	script := listenYouTubeMusicBridgeScript(ListenPlayerPlayRequest{VideoID: "TESTVID001A"})
	for _, expected := range []string{
		`const PLAY_WHEN_READY_SETTLE_MS = 2000`,
		`function schedulePlayWhenReadySettlement(video)`,
		`expirePlayWhenReady();`,
		`if (!playWhenReady) {`,
		`sendState("autoplay-expired", true);`,
		`navigator.mediaSession.setActionHandler("pause", () => invokePause("media-session-pause"))`,
		`function installPlaybackIntentPointerGuard()`,
		`if (video && !video.paused && !video.ended && !adSnapshot().advertising) expirePlayWhenReady()`,
	} {
		if !strings.Contains(script, expected) {
			t.Fatalf("music bridge must bound recovery and honor native pause; missing %q", expected)
		}
	}
}

func TestListenMusicBridgeDoesNotSettleTargetPlayIntentDuringAds(t *testing.T) {
	script := listenYouTubeMusicBridgeScript(ListenPlayerPlayRequest{VideoID: "TESTVID001A"})
	for _, expected := range []string{
		`if (adSnapshot().advertising || wasAdvertisingRecently())`,
		`playWhenReadyDeadline = Date.now() + PLAY_WHEN_READY_MAX_MS`,
		`if (!requestedVideoId || activeVideoId !== requestedVideoId)`,
		`schedulePlayWhenReadySettlement(video)`,
	} {
		if !strings.Contains(script, expected) {
			t.Fatalf("advertising must not consume the target track play intent; missing %q", expected)
		}
	}
}

func TestListenMusicBridgeManualSeekReleasesRestartGuard(t *testing.T) {
	script := listenYouTubeMusicBridgeScript(ListenPlayerPlayRequest{
		VideoID:          "TESTVID001A",
		RestartFromStart: true,
	})
	for _, expected := range []string{
		`if (applyKey) startAppliedForVideo = applyKey`,
		`restartFromStartGuardKey = ""`,
		`restartFromStartPlayingAt = 0`,
		`video.currentTime = finiteNumber(Number(seconds || 0), 0)`,
	} {
		if !strings.Contains(script, expected) {
			t.Fatalf("manual seek must release the load-time restart guard; missing %q", expected)
		}
	}
}

func TestListenMusicBridgeKeepsRestartGuardUntilPlaybackSettles(t *testing.T) {
	script := listenYouTubeMusicBridgeScript(ListenPlayerPlayRequest{
		VideoID:          "TESTVID001A",
		RestartFromStart: true,
	})
	for _, expected := range []string{
		`RESTART_FROM_START_SETTLE_MS`,
		`let restartFromStartPlayingAt = 0`,
		`const allowedProgress = restartFromStartPlayingAt > 0`,
		`if (current > allowedProgress)`,
		`video.currentTime = 0`,
		`now - restartFromStartPlayingAt >= RESTART_FROM_START_SETTLE_MS`,
		`video.addEventListener("timeupdate", () => applyStartPosition(video))`,
	} {
		if !strings.Contains(script, expected) {
			t.Fatalf("restart-from-start must guard against late YouTube history restoration; missing %q", expected)
		}
	}
	tooEarly := `if (applyKey) startAppliedForVideo = applyKey;\n      } catch (error) {}\n      return;`
	if strings.Contains(script, tooEarly) {
		t.Fatal("restart-from-start must not be marked complete on the initial zero-time observation")
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

func TestListenMusicSameVideoLanguageChangeRequiresReload(t *testing.T) {
	request := normalizeListenPlayerPlayRequest(ListenPlayerPlayRequest{
		VideoID:  "TESTVID001A",
		Language: "zh-CN",
	})
	if !listenYouTubeMusicCanResumeSameVideo(request, false, request.VideoID, "", "zh-CN") {
		t.Fatal("same video and language should keep the existing document")
	}
	if listenYouTubeMusicCanResumeSameVideo(request, false, request.VideoID, "", "en") {
		t.Fatal("same video with a different language must reload")
	}
}

func TestListenPlaybackAdapterForwardsSessionLanguage(t *testing.T) {
	request := listenPlayerPlayRequestFromPlaybackRequest(listenplayback.PlayRequest{
		Track:    listenplayback.Track{VideoID: "TESTVID001A"},
		Language: "zh-TW",
	}, listenplayback.VideoLoadStandard)
	if request.Language != "zh-TW" {
		t.Fatalf("expected adapter language zh-TW, got %q", request.Language)
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
		"if (count >= 4)",
		"scheduleVolumeApply();",
		"installFullscreenEscape();",
		"DOCUMENT_VOLUME_BOOT_KEY",
		"request: (next) =>",
		"sendState(\"api-request\", true)",
		"next.forceReload = next.forceReload === true",
	} {
		if !strings.Contains(script, expected) {
			t.Fatalf("music bridge should contain %q", expected)
		}
	}
	if strings.Contains(script, "const volumeBurst = window.setInterval") {
		t.Fatal("music bridge should not start a second per-element volume burst")
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

func TestListenRawMusicBufferingSyncPreservesBufferingState(t *testing.T) {
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

	if service.State() != listenplayback.PlaybackStateBuffering {
		t.Fatalf("expected raw buffering sync to preserve buffering state, got %s", service.State())
	}

	player.syncPlaybackServiceFromNativeEvent(
		service,
		"state",
		"playing",
		"video-one",
		"One",
		"Artist",
		"",
		"",
		false,
		map[string]any{"currentTime": 15, "duration": 180},
	)
	if service.State() != listenplayback.PlaybackStatePlaying {
		t.Fatalf("expected following raw playing sync to restore playing state, got %s", service.State())
	}
}

func TestObservePlaybackPreservesBufferingState(t *testing.T) {
	ctx := context.Background()
	transport := &listenPlaybackTestTransport{}
	service := listenplayback.NewPlayerService(
		transport,
		listenplayback.WithUserInteractionUnlocked(),
	)
	if err := service.PlayQueue(ctx, []listenplayback.Track{
		{ID: "one", VideoID: "video-one", Title: "One", Artist: "Artist"},
	}, 0, "Queue"); err != nil {
		t.Fatal(err)
	}

	handler := NewListenPlayerHandler(nil, service)
	snapshot, err := handler.ObservePlayback(ctx, ListenPlaybackObservationRequest{
		State:    listenplayback.PlaybackStateBuffering,
		Progress: 14,
		Duration: 180,
	})
	if err != nil {
		t.Fatal(err)
	}
	if snapshot.State != listenplayback.PlaybackStateBuffering {
		t.Fatalf("expected ObservePlayback to preserve buffering state, got %s", snapshot.State)
	}
}

func TestInsertAfterQueueItemHandlerForwardsAnchorAndQueueIdentity(t *testing.T) {
	ctx := context.Background()
	transport := &listenPlaybackTestTransport{}
	service := listenplayback.NewPlayerService(
		transport,
		listenplayback.WithUserInteractionUnlocked(),
	)
	if err := service.PlayQueueWithIdentity(ctx, []listenplayback.Track{
		{ID: "current", VideoID: "video-current", Title: "Current"},
		{ID: "anchor", VideoID: "video-anchor", Title: "Anchor"},
		{ID: "existing", VideoID: "video-existing", Title: "Existing"},
	}, 0, "Queue", "client:active"); err != nil {
		t.Fatal(err)
	}
	queue, _ := service.Queue()
	handler := NewListenPlayerHandler(nil, service)
	snapshot, err := handler.InsertAfterQueueItem(
		ctx,
		ListenPlaybackQueueItemsRequest{
			Tracks: []listenplayback.Track{
				{ID: "continuation", VideoID: "video-continuation", Title: "Continuation"},
			},
			AnchorTrackID:         queue[1].ID,
			ExpectedQueueIdentity: "client:active",
		},
	)
	if err != nil {
		t.Fatal(err)
	}
	if len(snapshot.Queue) != 4 || snapshot.Queue[2].VideoID != "video-continuation" {
		t.Fatalf("expected handler to insert continuation after anchor, got %#v", snapshot.Queue)
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

func TestListenSameVideoRestartPersistsGuardRequestBeforePlaying(t *testing.T) {
	script := listenYouTubeMusicSameVideoResumeScript(ListenPlayerPlayRequest{
		VideoID:          "TESTVID001A",
		RestartFromStart: true,
		Volume:           0.42,
	})
	requestIndex := strings.Index(script, `api.request(request)`)
	seekIndex := strings.Index(script, `request.restartFromStart === true`)
	playIndex := strings.Index(script, `api.play()`)
	if requestIndex < 0 || seekIndex < 0 || playIndex < 0 || requestIndex > seekIndex || seekIndex > playIndex {
		t.Fatalf("same-video restart must persist the guarded request before seeking and playing: %s", script)
	}
	for _, expected := range []string{`"restartFromStart":true`, `video.currentTime = 0`} {
		if !strings.Contains(script, expected) {
			t.Fatalf("same-video restart must contain %q", expected)
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

func TestListenStreamProviderKeepsYouTubeEmbed(t *testing.T) {
	targetURL := listenYouTubePlaybackURL(
		listenplayback.PlaybackProviderStream,
		"TESTVID002B",
		"zh-CN",
	)
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
		t.Fatalf("stream embed url should not use watch playback: %s", targetURL)
	}
}

func TestListenYouTubeProviderUsesWatchPage(t *testing.T) {
	targetURL := listenYouTubePlaybackURL(
		listenplayback.PlaybackProviderYouTube,
		"TESTVID002B",
		"zh-CN",
	)
	for _, expected := range []string{
		"https://www.youtube.com/watch?",
		"v=TESTVID002B",
		"autoplay=1",
		"hl=zh-CN",
		"persist_hl=1",
		"#xiadown-request=TESTVID002B",
	} {
		if !strings.Contains(targetURL, expected) {
			t.Fatalf("youtube watch url should contain %q, got %s", expected, targetURL)
		}
	}
	if strings.Contains(targetURL, "/embed/") || strings.Contains(targetURL, "music.youtube.com") {
		t.Fatalf("youtube provider should use the regular watch page: %s", targetURL)
	}
}

func TestListenYouTubeLiveDocumentReuseRequiresSameLanguage(t *testing.T) {
	request := ListenPlayerPlayRequest{
		VideoID:  "TESTVID002B",
		Language: "zh-Hans-CN",
	}
	if !listenYouTubeLiveCanReuseDocument(
		"TESTVID002B",
		listenplayback.PlaybackProviderYouTube,
		"zh-CN",
		listenplayback.PlaybackProviderYouTube,
		request,
	) {
		t.Fatal("equivalent normalized language should reuse the current document")
	}
	if listenYouTubeLiveCanReuseDocument(
		"TESTVID002B",
		listenplayback.PlaybackProviderYouTube,
		"en",
		listenplayback.PlaybackProviderYouTube,
		request,
	) {
		t.Fatal("language change must reload the current video document")
	}
	request.ForceReload = true
	if listenYouTubeLiveCanReuseDocument(
		"TESTVID002B",
		listenplayback.PlaybackProviderYouTube,
		"zh-CN",
		listenplayback.PlaybackProviderYouTube,
		request,
	) {
		t.Fatal("force reload must bypass same-video document reuse")
	}
}

func TestListenLiveBridgeSeparatesObservedIdentityAndBlocksWatchAutonav(t *testing.T) {
	script := listenYouTubeLiveBridgeScript(ListenPlayerPlayRequest{
		VideoID: "TESTVID002B",
		Title:   "Synthetic live radio",
		Artist:  "Listen",
	})
	for _, expected := range []string{
		"listen-youtube-live-player",
		"__listenLivePlayer",
		"movie_player",
		"function playerVideoData()",
		"api.getVideoData()",
		"data.video_id || data.videoId",
		"observedVideoId: metadata.observedVideoId",
		"requestedVideoId: metadata.requestedVideoId",
		"const useRequestedMetadata = Boolean(advertising) || !trackChanged",
		"String(observedTitle || observedVideoId)",
		"function disableWatchAutonav()",
		`api.setOption("autonav", "autoplay", false)`,
		".ytp-autonav-toggle-button",
		"function blockUnexpectedWatchPlayback(metadata, advertising)",
		`let pendingRequestedVideoId = ""`,
		"pendingRequestedVideoId === metadata.requestedVideoId",
		"pendingRequestedVideoId = validYouTubeVideoId(request.videoId)",
		`urlVideoId === pendingRequestedVideoId`,
		"const urlVideoId = validYouTubeVideoId(videoIdFromURL())",
		"urlVideoId === metadata.requestedVideoId",
		"storedVideoId === pendingRequestedVideoId",
		"navigationRequestedVideoId === urlVideoId",
		`new URLSearchParams(hash).get("xiadown-request")`,
		"function requestForNavigationVideo(videoId)",
		`request.title = ""`,
		`request.artist = ""`,
		"request.startSeconds = 0",
		"!validYouTubeVideoId(storedVideoId)",
		"requestForNavigationVideo(urlVideoId)",
		`lastRequestedAction = "pause"`,
		"api.pauseVideo()",
		"videoElements().forEach",
		"if (!video.paused) video.pause()",
		"autonavBlocked",
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
	if strings.Contains(script, "observedVideoId: metadata.videoId") ||
		strings.Contains(script, "requestedVideoId: metadata.videoId") {
		t.Fatal("watch bridge must not collapse observed and requested identities")
	}
	targetLoadGuard := strings.Index(script, "urlVideoId === metadata.requestedVideoId")
	if targetLoadGuard < 0 {
		t.Fatal("the requested watch URL guard is missing")
	}
	autonavPause := strings.Index(script[targetLoadGuard:], `lastRequestedAction = "pause"`)
	if autonavPause < 0 {
		t.Fatal("the requested watch URL must bypass autonav blocking before it can cancel autoplay")
	}
	if strings.Contains(script, "ytmusic-player") {
		t.Fatalf("live bridge should not depend on YouTube Music DOM")
	}
}

func TestListenLivePlayDoesNotReparentAnOwnedFullscreenSurface(t *testing.T) {
	source, err := os.ReadFile("listen_live_player_handler.go")
	if err != nil {
		t.Fatalf("read live player source: %v", err)
	}
	playStart := strings.Index(string(source), "func (player *ListenYouTubeLivePlayer) Play(")
	playEnd := strings.Index(string(source)[playStart:], "func (player *ListenYouTubeLivePlayer) Pause(")
	if playStart < 0 || playEnd < 0 {
		t.Fatal("live player Play method not found")
	}
	play := string(source)[playStart : playStart+playEnd]
	guard := strings.Index(play, "applyInlineGeometry := listenEmbeddedVideoCanApplyInlineGeometry")
	reparent := strings.Index(play, "player.showEmbeddedVideoWindow(window, embeddedRect)")
	if guard < 0 || reparent < 0 || guard > reparent {
		t.Fatal("Play must guard inline reparenting with fullscreen presentation ownership")
	}
	if !strings.Contains(play, "listenYouTubeLiveVideoModeScript()") {
		t.Fatal("Play must still apply video-only CSS without inline geometry while fullscreen owns the WebView")
	}
}

func TestListenLivePrepareLoadLeavesNewPlaybackRequestAsFinalIntent(t *testing.T) {
	script := listenYouTubeLivePrepareLoadScript(ListenPlayerPlayRequest{VideoID: "TESTVID002B"})
	pause := strings.Index(script, `api.pause()`)
	request := strings.Index(script, `api.request(request)`)
	if pause < 0 || request < 0 || pause > request {
		t.Fatal("switching videos must pause the old session before starting the new request")
	}
}

func TestListenLiveBridgeRestoresPureVideoAtDocumentStart(t *testing.T) {
	script := listenYouTubeLiveBridgeScript(ListenPlayerPlayRequest{VideoID: "TESTVID002B"})
	for _, expected := range []string{
		`const VIDEO_MODE_STYLE_ID = "listen-live-video-mode-style"`,
		`window.localStorage.getItem("__listenLiveVideoModeActive") === "true"`,
		"installLiveVideoModeAtDocumentStart",
		"videoModeDocumentRootObserver.observe(document, { childList: true, subtree: true })",
		`video?.closest("#movie_player, .html5-video-player")`,
		"let current = video",
		`current.classList.add("listen-live-video-surface")`,
		"if (current === root) reachedRoot = true",
		`root.classList.add("listen-live-video-root")`,
		"restoreLiveVideoMode();",
		"transform: none !important",
		"clip-path: none !important",
		".listen-live-video-root .ytp-caption-window-container *",
	} {
		if !strings.Contains(script, expected) {
			t.Fatalf("document-start live bridge should contain %q", expected)
		}
	}
	if strings.Contains(script, ".listen-live-video-root *") {
		t.Fatal("video-only bridge must not reveal every YouTube player descendant")
	}
	waitForDOM := strings.LastIndex(script, `if (document.readyState === "loading")`)
	if waitForDOM < 0 ||
		strings.LastIndex(script[:waitForDOM], "installLiveVideoModeAtDocumentStart();") < 0 {
		t.Fatal("persisted video isolation must be installed before DOMContentLoaded")
	}
}

func TestListenLiveVideoModeScriptUsesYouTubePlayer(t *testing.T) {
	script := listenYouTubeLiveVideoModeScript()
	for _, expected := range []string{
		"movie_player",
		"listen-live-video-root",
		"listen-live-video-visible",
		"html body * { visibility: hidden !important; }",
		".listen-live-video-root .ytp-caption-window-container *",
		".listen-live-video-root .caption-window *",
		"requestAnimationFrame(enforce)",
		"__listenNativeWindowFullscreenActive",
		"listen-live-native-window-fullscreen",
		".ytp-chrome-bottom * { visibility: visible !important; opacity: 1 !important; pointer-events: auto !important; }",
		"html:fullscreen .listen-live-video-root .ytp-chrome-bottom",
		"html:-webkit-full-screen .listen-live-video-root .ytp-chrome-bottom",
		"html:fullscreen .listen-live-video-root .ytp-fullscreen-button { display: none !important; }",
		"html:-webkit-full-screen .listen-live-video-root .ytp-fullscreen-button { display: none !important; }",
		".listen-live-video-root:fullscreen .ytp-chrome-bottom",
		".listen-live-video-root:-webkit-full-screen .ytp-chrome-bottom",
	} {
		if !strings.Contains(script, expected) {
			t.Fatalf("live video mode script should contain %q", expected)
		}
	}
	for _, expected := range []string{
		`video?.closest("#movie_player, .html5-video-player")`,
		"let current = video",
		"if (current === root) reachedRoot = true",
		"function videoTreeIsIsolated(video, root, expectedWidth, expectedHeight)",
		`current.classList.contains("listen-live-video-surface")`,
		`rootStyle.position !== "fixed"`,
		"ready: viewportMatches && videoMatches && rootMatches && hasVideoFrame && treeIsolated",
	} {
		if !strings.Contains(script, expected) {
			t.Fatalf("live video reveal readiness should contain %q", expected)
		}
	}
	if strings.Contains(script, ".listen-live-video-root *") {
		t.Fatal("video-only mode must reveal only the active video's ancestor chain")
	}
	if strings.Contains(script, "ytmusic-player") {
		t.Fatalf("live video mode should not depend on YouTube Music DOM")
	}
}

func TestListenYouTubeEmbeddedVideoRevealRequiresIsolatedDOMAck(t *testing.T) {
	tests := []struct {
		name        string
		nativeShown bool
		resizeReady bool
		ownerActive bool
		want        bool
	}{
		{name: "isolated player ready", nativeShown: true, resizeReady: true, ownerActive: true, want: true},
		{name: "watch page not isolated", nativeShown: true, resizeReady: false, ownerActive: true, want: false},
		{name: "native mount failed", nativeShown: false, resizeReady: true, ownerActive: true, want: false},
		{name: "owner changed", nativeShown: true, resizeReady: true, ownerActive: false, want: false},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if got := listenYouTubeEmbeddedVideoRevealReady(test.nativeShown, test.resizeReady, test.ownerActive); got != test.want {
				t.Fatalf("listenYouTubeEmbeddedVideoRevealReady() = %v, want %v", got, test.want)
			}
		})
	}
	if listenLiveEmbeddedVideoRevealReady(
		listenplayback.PlaybackProviderYouTube,
		true,
		false,
		true,
	) {
		t.Fatal("YouTube watch page must not reveal before its video-only DOM acknowledgement")
	}
	if !listenLiveEmbeddedVideoRevealReady(
		listenplayback.PlaybackProviderStream,
		true,
		false,
		true,
	) {
		t.Fatal("the lightweight stream embed should retain the native-mount reveal policy")
	}
}

func TestListenLiveNativeWindowFullscreenModeKeepsPlayerControlsReachable(t *testing.T) {
	enter := listenYouTubeLiveNativeWindowFullscreenModeScript(true)
	exit := listenYouTubeLiveNativeWindowFullscreenModeScript(false)
	for _, expected := range []string{
		`const active = true`,
		`sessionStorage.setItem("__listenNativeWindowFullscreenActive", "true")`,
		`classList.toggle("listen-live-native-window-fullscreen", active)`,
	} {
		if !strings.Contains(enter, expected) {
			t.Fatalf("native fullscreen enter script missing %q", expected)
		}
	}
	for _, expected := range []string{
		`const active = false`,
		`sessionStorage.removeItem("__listenNativeWindowFullscreenActive")`,
	} {
		if !strings.Contains(exit, expected) {
			t.Fatalf("native fullscreen exit script missing %q", expected)
		}
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
