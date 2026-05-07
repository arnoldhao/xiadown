package service

import (
	"errors"
	"os"
	"testing"
)

func TestNormalizeOnlinePetImportSiteIDSupportsCodexpetXYZ(t *testing.T) {
	tests := []string{
		"codexpet-xyz",
		"codexpet.xyz",
		"https://codexpet.xyz/",
	}

	for _, input := range tests {
		if got := normalizeOnlinePetImportSiteID(input); got != onlinePetImportSiteCodexpetXYZ {
			t.Fatalf("normalizeOnlinePetImportSiteID(%q) = %q, want %q", input, got, onlinePetImportSiteCodexpetXYZ)
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

func assertPathRemoved(t *testing.T, path string) {
	t.Helper()
	_, err := os.Stat(path)
	if !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("path %q still exists or stat failed with unexpected error: %v", path, err)
	}
}
