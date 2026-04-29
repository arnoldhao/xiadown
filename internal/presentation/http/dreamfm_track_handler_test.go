package http

import (
	"net/http/httptest"
	"strings"
	"testing"

	"xiadown/internal/application/youtubemusic"
)

func TestDreamFMTrackHandlerReturnsTrackMetadata(t *testing.T) {
	handler := NewDreamFMTrackHandler(fakeDreamFMMusicClient{
		trackMetadata: youtubemusic.TrackMetadata{
			VideoID:        "TESTVID007G",
			Title:          "Never Gonna Give You Up",
			Channel:        "Rick Astley",
			ArtistBrowseID: "UCuAXFkgsw1L7xaCfnd5JJOw",
			DurationLabel:  "3:33",
			ThumbnailURL:   "https://i.ytimg.com/vi/TESTVID007G/hqdefault.jpg",
			MusicVideoType: "MUSIC_VIDEO_TYPE_OMV",
		},
	})
	request := httptest.NewRequest("GET", "/api/dreamfm/track?id=TESTVID007G", nil)
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
