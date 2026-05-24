package http

import (
	"net/http/httptest"
	"strings"
	"testing"

	"xiadown/internal/application/youtubemusic"
)

func TestListenTrackHandlerReturnsTrackMetadata(t *testing.T) {
	handler := NewListenTrackHandler(fakeListenMusicClient{
		trackMetadata: youtubemusic.TrackMetadata{
			VideoID:        "TESTVID007G",
			Title:          "Never Gonna Give You Up",
			Channel:        "Rick Astley",
			Artists:        []youtubemusic.TrackArtist{{Name: "Rick Astley", BrowseID: "UCuAXFkgsw1L7xaCfnd5JJOw"}},
			ArtistBrowseID: "UCuAXFkgsw1L7xaCfnd5JJOw",
			DurationLabel:  "3:33",
			ThumbnailURL:   "https://i.ytimg.com/vi/TESTVID007G/hqdefault.jpg",
			MusicVideoType: "MUSIC_VIDEO_TYPE_OMV",
		},
	})
	request := httptest.NewRequest("GET", "/api/listen/track?id=TESTVID007G", nil)
	recorder := httptest.NewRecorder()

	handler.ServeHTTP(recorder, request)

	if recorder.Result().StatusCode != 200 {
		t.Fatalf("expected 200, got %d", recorder.Result().StatusCode)
	}
	body := recorder.Body.String()
	for _, expected := range []string{
		`"videoId":"TESTVID007G"`,
		`"title":"Never Gonna Give You Up"`,
		`"channel":"Rick Astley"`,
		`"artists":[`,
		`"artistBrowseId":"UCuAXFkgsw1L7xaCfnd5JJOw"`,
		`"durationLabel":"3:33"`,
		`"musicVideoType":"MUSIC_VIDEO_TYPE_OMV"`,
		`"hasVideo":true`,
		`"videoAvailabilityKnown":true`,
	} {
		if !strings.Contains(body, expected) {
			t.Fatalf("expected response body to contain %s, got %s", expected, body)
		}
	}
}

func TestListenTrackHandlerConfirmsAudioTrackUnavailable(t *testing.T) {
	handler := NewListenTrackHandler(fakeListenMusicClient{
		trackMetadata: youtubemusic.TrackMetadata{
			VideoID:        "TESTVID007G",
			Title:          "Audio Track",
			Channel:        "Artist",
			MusicVideoType: "MUSIC_VIDEO_TYPE_ATV",
		},
	})
	request := httptest.NewRequest("GET", "/api/listen/track?id=TESTVID007G", nil)
	recorder := httptest.NewRecorder()

	handler.ServeHTTP(recorder, request)

	if recorder.Result().StatusCode != 200 {
		t.Fatalf("expected 200, got %d", recorder.Result().StatusCode)
	}
	body := recorder.Body.String()
	if !strings.Contains(body, `"musicVideoType":"MUSIC_VIDEO_TYPE_ATV"`) {
		t.Fatalf("expected response body to contain audio musicVideoType, got %s", body)
	}
	if strings.Contains(body, `"hasVideo"`) {
		t.Fatalf("expected false hasVideo to be omitted, got %s", body)
	}
	if !strings.Contains(body, `"videoAvailabilityKnown":true`) {
		t.Fatalf("audio endpoint type should be serialized as confirmed no-video availability: %s", body)
	}
}

func TestListenTrackHandlerDoesNotConfirmUGCAvailability(t *testing.T) {
	handler := NewListenTrackHandler(fakeListenMusicClient{
		trackMetadata: youtubemusic.TrackMetadata{
			VideoID:        "TESTVID007G",
			Title:          "Fan Upload",
			Channel:        "Uploader",
			MusicVideoType: "MUSIC_VIDEO_TYPE_UGC",
		},
	})
	request := httptest.NewRequest("GET", "/api/listen/track?id=TESTVID007G", nil)
	recorder := httptest.NewRecorder()

	handler.ServeHTTP(recorder, request)

	if recorder.Result().StatusCode != 200 {
		t.Fatalf("expected 200, got %d", recorder.Result().StatusCode)
	}
	body := recorder.Body.String()
	if !strings.Contains(body, `"musicVideoType":"MUSIC_VIDEO_TYPE_UGC"`) {
		t.Fatalf("expected response body to contain UGC musicVideoType, got %s", body)
	}
	if strings.Contains(body, `"hasVideo"`) || strings.Contains(body, `"videoAvailabilityKnown"`) {
		t.Fatalf("UGC endpoint type should not be serialized as confirmed video availability: %s", body)
	}
}

func TestListenTrackHandlerConfirmsNoVideoFromNonVideoThumbnail(t *testing.T) {
	handler := NewListenTrackHandler(fakeListenMusicClient{
		trackMetadata: youtubemusic.TrackMetadata{
			VideoID:        "TESTVID007G",
			Title:          "Audio Artwork",
			Channel:        "Uploader",
			MusicVideoType: "MUSIC_VIDEO_TYPE_UGC",
			ThumbnailURL:   "https://lh3.googleusercontent.com/art=w544-h544",
		},
	})
	request := httptest.NewRequest("GET", "/api/listen/track?id=TESTVID007G", nil)
	recorder := httptest.NewRecorder()

	handler.ServeHTTP(recorder, request)

	if recorder.Result().StatusCode != 200 {
		t.Fatalf("expected 200, got %d", recorder.Result().StatusCode)
	}
	body := recorder.Body.String()
	if strings.Contains(body, `"hasVideo"`) {
		t.Fatalf("expected false hasVideo to be omitted, got %s", body)
	}
	if !strings.Contains(body, `"videoAvailabilityKnown":true`) {
		t.Fatalf("non-video thumbnail should serialize confirmed no-video availability: %s", body)
	}
}

func TestListenTrackHandlerInfersVideoFromYouTubeThumbnail(t *testing.T) {
	handler := NewListenTrackHandler(fakeListenMusicClient{
		trackMetadata: youtubemusic.TrackMetadata{
			VideoID:        "TESTVID007G",
			Title:          "Video Upload",
			Channel:        "Uploader",
			MusicVideoType: "MUSIC_VIDEO_TYPE_PODCAST_EPISODE",
			ThumbnailURL:   "https://i.ytimg.com/vi/TESTVID007G/hq720.jpg",
		},
	})
	request := httptest.NewRequest("GET", "/api/listen/track?id=TESTVID007G", nil)
	recorder := httptest.NewRecorder()

	handler.ServeHTTP(recorder, request)

	if recorder.Result().StatusCode != 200 {
		t.Fatalf("expected 200, got %d", recorder.Result().StatusCode)
	}
	body := recorder.Body.String()
	for _, expected := range []string{
		`"hasVideo":true`,
		`"videoAvailabilityKnown":true`,
	} {
		if !strings.Contains(body, expected) {
			t.Fatalf("expected response body to contain %s, got %s", expected, body)
		}
	}
}
