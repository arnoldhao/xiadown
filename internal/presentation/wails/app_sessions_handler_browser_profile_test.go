package wails

import (
	"context"
	"errors"
	"runtime"
	"testing"

	appsessionsdto "xiadown/internal/application/appsessions/dto"
	appsessionsservice "xiadown/internal/application/appsessions/service"
	"xiadown/internal/domain/appsessions"
)

func TestAppSessionBrowserProfileBoundaryExcludesUnadaptedCandidates(t *testing.T) {
	t.Parallel()

	for _, browserID := range []string{"chrome", "edge", "brave", "arc", "vivaldi", "opera"} {
		if !appsessionsservice.SupportsBrowserProfileImport(browserID) {
			t.Fatalf("adapted browser %q is not supported", browserID)
		}
	}
	if appsessionsservice.SupportsBrowserProfileImport("safari") != (runtime.GOOS == "darwin") {
		t.Fatalf("unexpected Safari support on %s", runtime.GOOS)
	}
	for _, browserID := range []string{"chromium", "opera-gx", "yandex", "helium", "firefox"} {
		if appsessionsservice.SupportsBrowserProfileImport(browserID) {
			t.Fatalf("unadapted browser %q crossed the App Session boundary", browserID)
		}
	}
}

func TestDiscoverBrowserProfilesRejectsUnsupportedBrowserBeforeFilesystemAccess(t *testing.T) {
	t.Parallel()
	handler := &AppSessionsHandler{}
	_, err := handler.DiscoverBrowserProfiles(context.Background(), appsessionsdto.BrowserProfileDiscoveryRequest{
		BrowserID: "firefox",
	})
	if !errors.Is(err, appsessions.ErrUnsupported) {
		t.Fatalf("expected unsupported browser to fail closed, got %v", err)
	}
}

func TestListBrowserProfileSourcesIncludesSafariOnlyOnDarwin(t *testing.T) {
	t.Parallel()
	handler := &AppSessionsHandler{}
	sources := handler.ListBrowserProfileSources(context.Background())
	seen := make(map[string]bool, len(sources))
	for _, source := range sources {
		if seen[source.ID] {
			t.Fatalf("duplicate browser source %q", source.ID)
		}
		seen[source.ID] = true
	}
	if !seen["chrome"] {
		t.Fatal("Chrome source must be represented")
	}
	if seen["safari"] != (runtime.GOOS == "darwin") {
		t.Fatalf("unexpected Safari source presence on %s: %#v", runtime.GOOS, sources)
	}
}
