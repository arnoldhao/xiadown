package http

import (
	"net/http/httptest"
	"strings"
	"testing"

	"xiadown/internal/application/youtubemusic"
)

func TestDreamFMRadioHandlerServesYouTubeMusicRadio(t *testing.T) {
	handler := NewDreamFMRadioHandler(fakeDreamFMMusicClient{radioTracks: []youtubemusic.Track{{
		VideoID:       "TESTVID008H",
		Title:         "Lofi Radio",
		Channel:       "Lofi Girl",
		DurationLabel: "LIVE",
	}}})
	request := httptest.NewRequest("GET", "/api/dreamfm/radio?id=TESTVID007G", nil)
	recorder := httptest.NewRecorder()

	handler.ServeHTTP(recorder, request)

	if recorder.Result().StatusCode != 200 {
		t.Fatalf("expected 200, got %d", recorder.Result().StatusCode)
	}
	if body := recorder.Body.String(); !strings.Contains(body, `"id":"ytmusic-radio-TESTVID008H"`) {
		t.Fatalf("unexpected body: %s", body)
	}
}
