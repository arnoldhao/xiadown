package service

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	appcookies "xiadown/internal/application/cookies"
	"xiadown/internal/domain/connectors"
)

func TestCookiesForConnectorType(t *testing.T) {
	cookiesJSON, err := encodeCookies([]appcookies.Record{
		{Name: "SID", Value: "test-sid", Domain: ".youtube.com", Path: "/"},
	})
	if err != nil {
		t.Fatalf("encode cookies: %v", err)
	}
	connector, err := connectors.NewConnector(connectors.ConnectorParams{
		ID:          "connector-youtube",
		Type:        string(connectors.ConnectorYouTube),
		Status:      string(connectors.StatusConnected),
		CookiesJSON: cookiesJSON,
	})
	if err != nil {
		t.Fatalf("create connector: %v", err)
	}
	service := NewConnectorsService(newMemoryConnectorRepo(connector))

	records, err := service.CookiesForConnectorType(context.Background(), connectors.ConnectorYouTube)
	if err != nil {
		t.Fatalf("cookies for connector: %v", err)
	}
	if len(records) != 1 || records[0].Name != "SID" || records[0].Value != "test-sid" {
		t.Fatalf("unexpected records: %#v", records)
	}
}

func TestCookiesForConnectorTypeSkipsProfileConnector(t *testing.T) {
	cookiesJSON, err := encodeCookies([]appcookies.Record{
		{Name: "sessionid", Value: "legacy", Domain: ".douyin.com", Path: "/"},
	})
	if err != nil {
		t.Fatalf("encode cookies: %v", err)
	}
	connector, err := connectors.NewConnector(connectors.ConnectorParams{
		ID:             "connector-china-private",
		Type:           string(connectors.ConnectorChinaPrivate),
		Status:         string(connectors.StatusConnected),
		CredentialMode: string(connectors.CredentialModeProfile),
		CookiesJSON:    cookiesJSON,
	})
	if err != nil {
		t.Fatalf("create connector: %v", err)
	}
	service := NewConnectorsService(newMemoryConnectorRepo(connector))

	_, err = service.CookiesForConnectorType(context.Background(), connectors.ConnectorChinaPrivate)
	if !errors.Is(err, connectors.ErrNoCookies) {
		t.Fatalf("expected ErrNoCookies, got %v", err)
	}
}

func TestExportConnectorCookiesSkipsProfileConnector(t *testing.T) {
	cookiesJSON, err := encodeCookies([]appcookies.Record{
		{Name: "sessionid", Value: "legacy", Domain: ".douyin.com", Path: "/"},
	})
	if err != nil {
		t.Fatalf("encode cookies: %v", err)
	}
	connector, err := connectors.NewConnector(connectors.ConnectorParams{
		ID:             "connector-china-private",
		Type:           string(connectors.ConnectorChinaPrivate),
		Status:         string(connectors.StatusConnected),
		CredentialMode: string(connectors.CredentialModeProfile),
		CookiesJSON:    cookiesJSON,
	})
	if err != nil {
		t.Fatalf("create connector: %v", err)
	}
	service := NewConnectorsService(newMemoryConnectorRepo(connector))

	_, err = service.ExportConnectorCookies(context.Background(), connector.ID, CookiesExportTXT)
	if !errors.Is(err, connectors.ErrNoCookies) {
		t.Fatalf("expected ErrNoCookies, got %v", err)
	}
}

func TestWriteNetscapeCookiesAddsSubdomainEntryForPlainDomain(t *testing.T) {
	path := filepath.Join(t.TempDir(), "cookies.txt")

	if err := writeNetscapeCookies(path, []appcookies.Record{
		{
			Name:    "SID",
			Value:   "test-sid",
			Domain:  "youtube.com",
			Path:    "/",
			Expires: 4102444800,
			Secure:  true,
		},
	}); err != nil {
		t.Fatalf("write netscape cookies: %v", err)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read cookies: %v", err)
	}
	body := string(data)
	if !strings.Contains(body, "youtube.com\tFALSE\t/\tTRUE\t4102444800\tSID\ttest-sid") {
		t.Fatalf("expected host-only cookie entry, got:\n%s", body)
	}
	if !strings.Contains(body, ".youtube.com\tTRUE\t/\tTRUE\t4102444800\tSID\ttest-sid") {
		t.Fatalf("expected subdomain cookie entry, got:\n%s", body)
	}
}

func TestWriteNetscapeCookiesKeepsDottedDomainSingle(t *testing.T) {
	path := filepath.Join(t.TempDir(), "cookies.txt")

	if err := writeNetscapeCookies(path, []appcookies.Record{
		{Name: "SID", Value: "test-sid", Domain: ".youtube.com", Path: "/"},
	}); err != nil {
		t.Fatalf("write netscape cookies: %v", err)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read cookies: %v", err)
	}
	body := string(data)
	if strings.Count(body, "\tSID\ttest-sid") != 1 {
		t.Fatalf("expected one cookie entry, got:\n%s", body)
	}
	if !strings.Contains(body, ".youtube.com\tTRUE\t/\tFALSE\t0\tSID\ttest-sid") {
		t.Fatalf("expected dotted subdomain cookie entry, got:\n%s", body)
	}
}

func TestMapConnectorDTOReportsProfileCredentialState(t *testing.T) {
	profilePath := t.TempDir()
	if err := os.MkdirAll(filepath.Join(profilePath, "Default", "Cache"), 0o755); err != nil {
		t.Fatalf("create profile dirs: %v", err)
	}
	if err := os.WriteFile(filepath.Join(profilePath, "Default", "Cookies"), []byte("cookie-data"), 0o644); err != nil {
		t.Fatalf("write profile cookie file: %v", err)
	}
	if err := os.WriteFile(filepath.Join(profilePath, "Local State"), []byte("state"), 0o644); err != nil {
		t.Fatalf("write profile state file: %v", err)
	}
	connector, err := connectors.NewConnector(connectors.ConnectorParams{
		ID:             "connector-china-private",
		Type:           string(connectors.ConnectorChinaPrivate),
		Status:         string(connectors.StatusDisconnected),
		CredentialMode: string(connectors.CredentialModeProfile),
		ProfileKey:     "connector-china-private",
		ProfilePath:    profilePath,
	})
	if err != nil {
		t.Fatalf("create connector: %v", err)
	}

	got := mapConnectorDTO(connector)

	if got.CredentialMode != "profile" {
		t.Fatalf("expected profile credential mode, got %q", got.CredentialMode)
	}
	if got.CredentialState != "profile" {
		t.Fatalf("expected profile credential state, got %q", got.CredentialState)
	}
	if got.CookiesCount != 0 || len(got.Cookies) != 0 {
		t.Fatalf("expected profile connector to hide cookies, got count=%d cookies=%#v", got.CookiesCount, got.Cookies)
	}
	if got.ProfileInfo == nil || !got.ProfileInfo.Exists {
		t.Fatalf("expected profile info to exist, got %#v", got.ProfileInfo)
	}
	if got.ProfileInfo.Path != profilePath {
		t.Fatalf("expected profile path %q, got %q", profilePath, got.ProfileInfo.Path)
	}
	if got.ProfileInfo.SizeBytes <= 0 || got.ProfileInfo.FileCount != 2 || got.ProfileInfo.DirectoryCount != 2 {
		t.Fatalf("unexpected profile totals: %#v", got.ProfileInfo)
	}
	if len(got.ProfileInfo.Components) == 0 {
		t.Fatalf("expected profile components, got %#v", got.ProfileInfo.Components)
	}
}

func TestMapConnectorDTOReportsMissingProfileAsDisconnected(t *testing.T) {
	profilePath := filepath.Join(t.TempDir(), "missing-profile")
	connector, err := connectors.NewConnector(connectors.ConnectorParams{
		ID:             "connector-china-private",
		Type:           string(connectors.ConnectorChinaPrivate),
		Status:         string(connectors.StatusConnected),
		CredentialMode: string(connectors.CredentialModeProfile),
		ProfileKey:     "connector-china-private",
		ProfilePath:    profilePath,
	})
	if err != nil {
		t.Fatalf("create connector: %v", err)
	}

	got := mapConnectorDTO(connector)

	if got.CredentialState != "disconnected" {
		t.Fatalf("expected disconnected credential state, got %q", got.CredentialState)
	}
	if got.ProfileInfo == nil || got.ProfileInfo.Exists {
		t.Fatalf("expected missing profile info, got %#v", got.ProfileInfo)
	}
}
