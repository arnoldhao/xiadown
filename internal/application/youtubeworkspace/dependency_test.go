package youtubeworkspace

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestYouTubeWorkspaceProductionCodeDoesNotDependOnYTDLP(t *testing.T) {
	t.Parallel()
	entries, err := os.ReadDir(".")
	if err != nil {
		t.Fatalf("read package directory: %v", err)
	}
	forbidden := []string{
		"application/ytdlp",
		"appytdlp",
		"FetchInfo",
		"ToolResolver",
		"ExportAppSessionCookies",
		"CookiesExportTXT",
	}
	for _, entry := range entries {
		name := entry.Name()
		if entry.IsDir() || !strings.HasSuffix(name, ".go") || strings.HasSuffix(name, "_test.go") {
			continue
		}
		data, readErr := os.ReadFile(filepath.Clean(name))
		if readErr != nil {
			t.Fatalf("read %s: %v", name, readErr)
		}
		source := string(data)
		for _, marker := range forbidden {
			if strings.Contains(source, marker) {
				t.Fatalf("production YouTube workspace file %s still contains forbidden dependency %q", name, marker)
			}
		}
	}
}
