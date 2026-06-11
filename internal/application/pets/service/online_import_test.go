package service

import (
	"context"
	"errors"
	"os"
	"testing"

	settingsdto "xiadown/internal/application/settings/dto"
)

func TestNormalizeOnlinePetImportSiteIDSupportsKnownSites(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{input: "codexpet-xyz", want: onlinePetImportSiteCodexpetXYZ},
		{input: "codexpet.xyz", want: onlinePetImportSiteCodexpetXYZ},
		{input: "https://codexpet.xyz/", want: onlinePetImportSiteCodexpetXYZ},
		{input: "petdex-dev", want: onlinePetImportSitePetdexDev},
		{input: "petdex.dev", want: onlinePetImportSitePetdexDev},
		{input: "https://petdex.dev/", want: onlinePetImportSitePetdexDev},
	}

	for _, test := range tests {
		if got := normalizeOnlinePetImportSiteID(test.input); got != test.want {
			t.Fatalf("normalizeOnlinePetImportSiteID(%q) = %q, want %q", test.input, got, test.want)
		}
	}
}

func TestOnlinePetImportSiteSupportsCodexpetXYZ(t *testing.T) {
	label, siteURL, err := onlinePetImportSite(onlinePetImportSiteCodexpetXYZ)
	if err != nil {
		t.Fatalf("onlinePetImportSite(%q) returned error: %v", onlinePetImportSiteCodexpetXYZ, err)
	}
	if label != "codexpet.xyz" {
		t.Fatalf("label = %q, want codexpet.xyz", label)
	}
	if siteURL != "https://codexpet.xyz/" {
		t.Fatalf("siteURL = %q, want https://codexpet.xyz/", siteURL)
	}
}

func TestOnlinePetImportSiteSupportsPetdexDev(t *testing.T) {
	label, siteURL, err := onlinePetImportSite(onlinePetImportSitePetdexDev)
	if err != nil {
		t.Fatalf("onlinePetImportSite(%q) returned error: %v", onlinePetImportSitePetdexDev, err)
	}
	if label != "petdex.dev" {
		t.Fatalf("label = %q, want petdex.dev", label)
	}
	if siteURL != "https://petdex.dev" {
		t.Fatalf("siteURL = %q, want https://petdex.dev", siteURL)
	}
}

func TestMarkOnlinePetImportBrowserClosedCleansResourcesButKeepsSession(t *testing.T) {
	service := NewService(t.TempDir(), nil, "", "")
	downloadDir := t.TempDir()
	userDataDir := t.TempDir()
	session := &onlinePetImportSession{
		ID:            "session-1",
		State:         onlinePetImportStateRunning,
		BrowserStatus: onlinePetBrowserStatusOpen,
		DownloadDir:   downloadDir,
		UserDataDir:   userDataDir,
	}
	service.importSessions[session.ID] = session

	service.markOnlinePetImportBrowserClosed(session.ID)

	snapshot := service.snapshotOnlinePetImportSession(session.ID)
	if snapshot.SessionID != session.ID {
		t.Fatalf("snapshot session id = %q, want %q", snapshot.SessionID, session.ID)
	}
	if snapshot.BrowserStatus != onlinePetBrowserStatusBrowserClosed {
		t.Fatalf("browser status = %q, want %q", snapshot.BrowserStatus, onlinePetBrowserStatusBrowserClosed)
	}
	assertPathRemoved(t, downloadDir)
	assertPathRemoved(t, userDataDir)
}

func TestShutdownOnlinePetImportSessionsCleansResourcesAndRemovesSessions(t *testing.T) {
	service := NewService(t.TempDir(), nil, "", "")
	downloadDir := t.TempDir()
	userDataDir := t.TempDir()
	session := &onlinePetImportSession{
		ID:            "session-1",
		State:         onlinePetImportStateRunning,
		BrowserStatus: onlinePetBrowserStatusOpen,
		DownloadDir:   downloadDir,
		UserDataDir:   userDataDir,
	}
	service.importSessions[session.ID] = session

	service.ShutdownOnlinePetImportSessions()

	snapshot := service.snapshotOnlinePetImportSession(session.ID)
	if snapshot.SessionID != "" {
		t.Fatalf("session still available after shutdown: %#v", snapshot)
	}
	assertPathRemoved(t, downloadDir)
	assertPathRemoved(t, userDataDir)
}

func TestOnlinePetImportPreferredBrowserReadsSettings(t *testing.T) {
	service := NewService(
		t.TempDir(),
		nil,
		"",
		"",
		WithSettingsReader(petSettingsReaderStub{settings: settingsdto.Settings{SniffBrowser: "vivaldi"}}),
	)

	if got := service.preferredBrowser(context.Background()); got != "vivaldi" {
		t.Fatalf("expected preferred browser from settings, got %q", got)
	}
}

type petSettingsReaderStub struct {
	settings settingsdto.Settings
	err      error
}

func (stub petSettingsReaderStub) GetSettings(context.Context) (settingsdto.Settings, error) {
	if stub.err != nil {
		return settingsdto.Settings{}, stub.err
	}
	return stub.settings, nil
}

func assertPathRemoved(t *testing.T, path string) {
	t.Helper()
	_, err := os.Stat(path)
	if !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("path %q still exists or stat failed with unexpected error: %v", path, err)
	}
}
