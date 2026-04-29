package http

import (
	"net/url"
	"testing"
)

func TestResolveDreamFMLiveStatusFromHTML(t *testing.T) {
	tests := []struct {
		name   string
		html   string
		status string
	}{
		{
			name:   "live now",
			html:   `{"videoDetails":{"isLive":true},"microformat":{"playerMicroformatRenderer":{"liveBroadcastDetails":{"isLiveNow":true}}}}`,
			status: "live",
		},
		{
			name:   "upcoming",
			html:   `{"microformat":{"playerMicroformatRenderer":{"liveBroadcastContent":"upcoming","upcomingEventData":{}}}}`,
			status: "upcoming",
		},
		{
			name:   "offline livestream",
			html:   `{"playabilityStatus":{"status":"LIVE_STREAM_OFFLINE"},"videoDetails":{"isLiveContent":true}}`,
			status: "offline",
		},
		{
			name:   "unavailable",
			html:   `{"playabilityStatus":{"status":"UNPLAYABLE","reason":"Video unavailable"}}`,
			status: "unavailable",
		},
		{
			name:   "unknown",
			html:   `{"videoDetails":{"title":"Normal upload"}}`,
			status: "unknown",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := resolveDreamFMLiveStatusFromHTML("TESTVID008H", tt.html)
			if got.Status != tt.status {
				t.Fatalf("expected %q, got %#v", tt.status, got)
			}
		})
	}
}

func TestDreamFMLiveStatusVideoIDs(t *testing.T) {
	query := make(url.Values)
	query["id"] = []string{"TESTVID008H", "TESTVID008H"}
	query["ids"] = []string{"TESTVID002B,TESTVID010J"}
	videoIDs, err := dreamFMLiveStatusVideoIDs(query)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	want := []string{"TESTVID008H", "TESTVID002B", "TESTVID010J"}
	if len(videoIDs) != len(want) {
		t.Fatalf("expected %d ids, got %#v", len(want), videoIDs)
	}
	for index := range want {
		if videoIDs[index] != want[index] {
			t.Fatalf("unexpected ids: %#v", videoIDs)
		}
	}
}
