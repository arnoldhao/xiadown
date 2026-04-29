package wails

import (
	"strings"
	"testing"
	"time"

	appcookies "xiadown/internal/application/cookies"
)

func TestFilterDreamFMPlaybackCookiesKeepsUsableYouTubeCookies(t *testing.T) {
	now := time.Unix(1_700_000_000, 0)
	records := []appcookies.Record{
		{Name: "SID", Value: "sid", Domain: ".youtube.com", Path: "/", Expires: now.Add(time.Hour).Unix(), Secure: true},
		{Name: "expired", Value: "gone", Domain: ".youtube.com", Path: "/", Expires: now.Add(-time.Hour).Unix()},
		{Name: "missing-domain", Value: "value", Path: "/"},
		{Name: "SID", Value: "duplicate", Domain: ".youtube.com", Path: "/", Expires: now.Add(time.Hour).Unix()},
	}

	cookies := filterDreamFMPlaybackCookies(records, now)
	if len(cookies) != 1 {
		t.Fatalf("expected one usable cookie, got %d", len(cookies))
	}
	if cookies[0].Name != "SID" || cookies[0].Value != "sid" {
		t.Fatalf("unexpected cookie: %+v", cookies[0])
	}
}

func TestDreamFMBridgePreservesPauseIntent(t *testing.T) {
	script := dreamFMYouTubeMusicBridgeScript(DreamFMPlayerPlayRequest{
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
}

func TestDreamFMPlayerStatusPrefersObservedMetadata(t *testing.T) {
	player := &DreamFMYouTubeMusicPlayer{
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

func TestDreamFMPauseScriptOnlyPausesVideoElement(t *testing.T) {
	script := dreamFMYouTubeMusicPauseScript()
	if !strings.Contains(script, `document.querySelectorAll("video")`) {
		t.Fatalf("pause script should pause HTML video elements")
	}
	if strings.Contains(script, ".pauseVideo") || strings.Contains(script, "pauseVideo()") {
		t.Fatalf("pause script should not call YouTube internal pause APIs")
	}
}

func TestDreamFMAirPlayScriptUsesWebKitPlaybackTargetPicker(t *testing.T) {
	script := dreamFMYouTubeMusicAirPlayScript()
	if !strings.Contains(script, "webkitShowPlaybackTargetPicker") {
		t.Fatalf("airplay script should use WebKit playback target picker")
	}
	if !strings.Contains(script, "__dreamfmNativePlayer") {
		t.Fatalf("airplay script should prefer the bridge API")
	}
}

func TestDreamFMVideoModeScriptUsesYouTubeMusicVideoMode(t *testing.T) {
	script := dreamFMYouTubeMusicVideoModeScript()
	for _, expected := range []string{
		"ytmusic-av-switcher",
		"#video-button",
		"ytmusic-player-page",
		"dreamfm-video-visible",
		"video.dreamfm-video-visible",
		"let current = video",
		"current.classList.add(\"dreamfm-video-visible\")",
		"requestAnimationFrame(enforce)",
	} {
		if !strings.Contains(script, expected) {
			t.Fatalf("video mode script should contain %q", expected)
		}
	}
}

func TestDreamFMLivePlayerUsesYouTubeEmbed(t *testing.T) {
	targetURL := dreamFMYouTubeLiveEmbedURL("TESTVID002B")
	for _, expected := range []string{
		"https://www.youtube.com/embed/TESTVID002B",
		"autoplay=1",
		"enablejsapi=1",
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

func TestDreamFMLiveBridgeReportsRequestedVideoIdentity(t *testing.T) {
	script := dreamFMYouTubeLiveBridgeScript(DreamFMPlayerPlayRequest{
		VideoID: "TESTVID002B",
		Title:   "Synthetic live radio",
		Artist:  "Dream.FM",
	})
	for _, expected := range []string{
		"dreamfm-youtube-live-player",
		"__dreamfmLivePlayer",
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
	} {
		if !strings.Contains(script, expected) {
			t.Fatalf("live bridge script should contain %q", expected)
		}
	}
	if strings.Contains(script, "ytmusic-player") {
		t.Fatalf("live bridge should not depend on YouTube Music DOM")
	}
}

func TestDreamFMLiveVideoModeScriptUsesYouTubePlayer(t *testing.T) {
	script := dreamFMYouTubeLiveVideoModeScript()
	for _, expected := range []string{
		"movie_player",
		"dreamfm-live-video-root",
		"dreamfm-live-video-visible",
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

func TestDreamFMLiveVolumeScriptSeparatesVolumeAndMuted(t *testing.T) {
	script := dreamFMYouTubeLiveVolumeScript(0.42, false)
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
