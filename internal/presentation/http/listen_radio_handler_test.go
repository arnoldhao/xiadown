package http

import (
	"net/http/httptest"
	"strings"
	"testing"

	"xiadown/internal/application/youtubemusic"
)

func TestListenRadioHandlerServesYouTubeMusicRadio(t *testing.T) {
	handler := NewListenRadioHandler(fakeListenMusicClient{radioTracks: []youtubemusic.Track{{
		VideoID:       "TESTVID008H",
		Title:         "Lofi Radio",
		Channel:       "Lofi Girl",
		DurationLabel: "LIVE",
	}}})
	request := httptest.NewRequest("GET", "/api/listen/radio?id=TESTVID007G", nil)
	recorder := httptest.NewRecorder()

	handler.ServeHTTP(recorder, request)

	if recorder.Result().StatusCode != 200 {
		t.Fatalf("expected 200, got %d", recorder.Result().StatusCode)
	}
	if body := recorder.Body.String(); !strings.Contains(body, `"id":"ytmusic-radio-TESTVID008H"`) {
		t.Fatalf("unexpected body: %s", body)
	}
}
