package service

import (
	"context"
	"errors"
	"os"
	"testing"
	"time"

	appcookies "xiadown/internal/application/cookies"
	"xiadown/internal/domain/appsessions"
)

func TestAppSessionCredentialValidationRejectsExpiredBilibiliSession(t *testing.T) {
	now := time.Date(2026, 7, 14, 10, 0, 0, 0, time.UTC)
	records := []appcookies.Record{
		{Name: "SESSDATA", Value: "expired", Domain: ".bilibili.com", Path: "/", Expires: now.Unix()},
		{Name: "buvid3", Value: "device", Domain: ".bilibili.com", Path: "/", Expires: now.Add(time.Hour).Unix()},
		{Name: "foreign", Value: "secret", Domain: ".attacker.example", Path: "/", Expires: now.Add(time.Hour).Unix()},
	}
	filtered := filterAppSessionCookiesAt("bilibili", records, now)
	if len(filtered) != 1 || filtered[0].Name != "buvid3" {
		t.Fatalf("filtered cookies = %#v, want only live in-policy device cookie", filtered)
	}
	if appSessionHasAuthenticationCookiesAt("bilibili", filtered, now) {
		t.Fatal("device cookie must not prove Bilibili authentication")
	}
}

func TestAppSessionCredentialValidationAcceptsLiveBilibiliSession(t *testing.T) {
	now := time.Date(2026, 7, 14, 10, 0, 0, 0, time.UTC)
	records := filterAppSessionCookiesAt("bilibili", []appcookies.Record{
		{Name: "SESSDATA", Value: "live", Domain: ".bilibili.com", Path: "/", Expires: now.Add(time.Hour).Unix()},
		{Name: "bili_jct", Value: "csrf", Domain: ".bilibili.com", Path: "/", Expires: now.Add(time.Hour).Unix()},
	}, now)
	if !appSessionHasAuthenticationCookiesAt("bilibili", records, now) {
		t.Fatal("live SESSDATA should prove Bilibili authentication")
	}
}

func TestExportAppSessionCookiesRejectsDeviceOnlyBilibiliJar(t *testing.T) {
	now := time.Now().UTC()
	session, err := appsessions.NewSession(appsessions.SessionParams{
		ID:        "site-app-session-bilibili",
		SiteKey:   "bilibili",
		Status:    string(appsessions.StatusConnected),
		CreatedAt: &now,
		UpdatedAt: &now,
	})
	if err != nil {
		t.Fatalf("create session: %v", err)
	}
	service := NewAppSessionsService(
		newAppSessionRepoStub(session),
		WithProvider(&appSessionProviderStub{loadRecords: []appcookies.Record{
			{Name: "SESSDATA", Value: "expired", Domain: ".bilibili.com", Path: "/", Expires: now.Add(-time.Minute).Unix()},
			{Name: "buvid3", Value: "device", Domain: ".bilibili.com", Path: "/", Expires: now.Add(time.Hour).Unix()},
		}}),
	)
	if _, err := service.ExportAppSessionCookies(context.Background(), session.ID, CookiesExportTXT); !errors.Is(err, appsessions.ErrNoCookies) {
		t.Fatalf("ExportAppSessionCookies error = %v, want ErrNoCookies", err)
	}
}

func TestExportAppSessionCookiesWritesLiveBilibiliJar(t *testing.T) {
	now := time.Now().UTC()
	session, err := appsessions.NewSession(appsessions.SessionParams{
		ID:        "site-app-session-bilibili",
		SiteKey:   "bilibili",
		Status:    string(appsessions.StatusConnected),
		CreatedAt: &now,
		UpdatedAt: &now,
	})
	if err != nil {
		t.Fatalf("create session: %v", err)
	}
	service := NewAppSessionsService(
		newAppSessionRepoStub(session),
		WithProvider(&appSessionProviderStub{loadRecords: []appcookies.Record{
			{Name: "SESSDATA", Value: "live", Domain: ".bilibili.com", Path: "/", Expires: now.Add(time.Hour).Unix()},
		}}),
	)
	path, err := service.ExportAppSessionCookies(context.Background(), session.ID, CookiesExportTXT)
	if err != nil {
		t.Fatalf("ExportAppSessionCookies returned error: %v", err)
	}
	defer os.Remove(path)
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read exported cookie file: %v", err)
	}
	if len(data) == 0 {
		t.Fatal("exported cookie file is empty")
	}
}
