package app

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"

	appcookies "xiadown/internal/application/cookies"
	"xiadown/internal/domain/appsessions"
)

func TestFetchBilibiliAppSessionAccountFromURL(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/x/web-interface/nav" {
			t.Fatalf("unexpected path %s", r.URL.Path)
		}
		if r.Header.Get("Referer") != "https://www.bilibili.com/" {
			t.Fatalf("missing bilibili referer")
		}
		if r.Header.Get("User-Agent") == "" {
			t.Fatalf("missing user agent")
		}
		if _, err := r.Cookie("SESSDATA"); err != nil {
			t.Fatalf("missing SESSDATA cookie: %v", err)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{
			"code": 0,
			"message": "0",
			"data": {
				"isLogin": true,
				"mid": 123456,
				"uname": "Bili User",
				"face": "//i0.hdslb.com/bfs/face/avatar.jpg",
				"vip": {
					"type": 2,
					"status": 1,
					"label": {
						"text": "Annual VIP",
						"label_theme": "annual_vip"
					}
				},
				"level_info": {
					"current_level": 6
				}
			}
		}`))
	}))
	defer server.Close()

	host := serverHost(t, server.URL)
	account, err := fetchBilibiliAppSessionAccountFromURL(
		context.Background(),
		server.Client(),
		[]appcookies.Record{
			{Name: "SESSDATA", Value: "session-value", Domain: host, Path: "/"},
			{Name: "unrelated", Value: "ignored", Domain: "example.com", Path: "/"},
		},
		server.URL+"/x/web-interface/nav",
	)
	if err != nil {
		t.Fatalf("fetch account: %v", err)
	}
	if account.DisplayName != "Bili User" {
		t.Fatalf("display name = %q", account.DisplayName)
	}
	if account.AvatarURL != "https://i0.hdslb.com/bfs/face/avatar.jpg" {
		t.Fatalf("avatar url = %q", account.AvatarURL)
	}
	if account.TierKey != "vip_annual" || account.TierLabel != "Annual VIP" {
		t.Fatalf("tier = %q %q", account.TierKey, account.TierLabel)
	}
	if len(account.Badges) != 2 {
		t.Fatalf("badges count = %d", len(account.Badges))
	}
	if account.Metadata["mid"] != int64(123456) {
		t.Fatalf("mid metadata = %#v", account.Metadata["mid"])
	}
	if account.Metadata["level"] != 6 {
		t.Fatalf("level metadata = %#v", account.Metadata["level"])
	}
}

func TestBilibiliAccountCookiesUsesBilibiliDomainCookies(t *testing.T) {
	t.Parallel()

	matched := bilibiliAccountCookies(
		[]appcookies.Record{
			{Name: "SESSDATA", Value: "session-value", Domain: "www.bilibili.com", Path: "/"},
			{Name: "bili_jct", Value: "csrf-value", Domain: ".bilibili.com", Path: "/"},
			{Name: "unrelated", Value: "ignored", Domain: "example.com", Path: "/"},
		},
		"https://api.bilibili.com/x/web-interface/nav",
	)
	if len(matched) != 2 {
		t.Fatalf("matched cookies count = %d", len(matched))
	}
	if matched[0].Name != "SESSDATA" || matched[1].Name != "bili_jct" {
		t.Fatalf("matched cookies = %#v", matched)
	}
}

func TestFetchBilibiliAppSessionAccountFromURLReturnsNoCookiesWhenLoggedOut(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"code": -101, "message": "not login", "data": {"isLogin": false}}`))
	}))
	defer server.Close()

	_, err := fetchBilibiliAppSessionAccountFromURL(
		context.Background(),
		server.Client(),
		[]appcookies.Record{{Name: "SESSDATA", Value: "session-value", Domain: serverHost(t, server.URL), Path: "/"}},
		server.URL+"/x/web-interface/nav",
	)
	if !errors.Is(err, appsessions.ErrNoCookies) {
		t.Fatalf("expected ErrNoCookies, got %v", err)
	}
}

func TestAppSessionAccountFetcherUnsupportedSites(t *testing.T) {
	t.Parallel()

	fetcher := newAppSessionAccountFetcher(nil)
	_, err := fetcher(context.Background(), "x", []appcookies.Record{{Name: "auth_token", Value: "value", Domain: "x.com", Path: "/"}})
	if !errors.Is(err, appsessions.ErrUnsupported) {
		t.Fatalf("expected ErrUnsupported, got %v", err)
	}
}

func serverHost(t *testing.T, rawURL string) string {
	t.Helper()
	parsed, err := url.Parse(rawURL)
	if err != nil {
		t.Fatalf("parse server url: %v", err)
	}
	return parsed.Hostname()
}
