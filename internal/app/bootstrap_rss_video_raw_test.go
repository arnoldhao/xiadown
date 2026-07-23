package app

import (
	"os"
	"strings"
	"testing"
)

func TestBootstrapRoutesRawMessagesToRSSVideoPlayer(t *testing.T) {
	source, err := os.ReadFile("bootstrap.go")
	if err != nil {
		t.Fatal(err)
	}
	text := string(source)
	required := `rssVideoPlayerRawHandler != nil && rssVideoPlayerRawHandler.HandleRawMessage(window, message, originInfo)`
	if !strings.Contains(text, required) {
		t.Fatalf("bootstrap raw-message router is missing %q", required)
	}
}
