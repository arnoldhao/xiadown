package app

import (
	"bytes"
	"context"
	"errors"
	"log"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
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
		if got := r.Header.Get("Cookie"); got != "SESSDATA=session-value" {
			t.Fatalf("unexpected Bilibili Cookie header %q", got)
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
			{Name: "quoted", Value: `unsafe"value`, Domain: host, Path: "/"},
			{Name: "controlled", Value: "unsafe\nvalue", Domain: host, Path: "/"},
			{Name: "separated", Value: "unsafe;value", Domain: host, Path: "/"},
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
	if len(account.Badges) != 1 {
		t.Fatalf("badges count = %d", len(account.Badges))
	}
	if account.Badges[0].Key != "level_6" || account.Badges[0].Label != "LV6" {
		t.Fatalf("badges = %#v", account.Badges)
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

func TestAppSessionAddCookiesSkipsUnsafeRecordsWithoutLogging(t *testing.T) {
	var standardLog bytes.Buffer
	originalLogOutput := log.Writer()
	log.SetOutput(&standardLog)
	t.Cleanup(func() {
		log.SetOutput(originalLogOutput)
	})

	req := httptest.NewRequest(http.MethodGet, "https://example.com/account", nil)
	appSessionAddCookies(
		req,
		[]appcookies.Record{
			{Name: "sessionid", Value: "valid-value", Domain: ".example.com", Path: "/"},
			{Name: "quoted", Value: `unsafe"value`, Domain: ".example.com", Path: "/"},
			{Name: "controlled", Value: "unsafe\nvalue", Domain: ".example.com", Path: "/"},
			{Name: "separated", Value: "unsafe;value", Domain: ".example.com", Path: "/"},
			{Name: `quoted"name`, Value: "unsafe", Domain: ".example.com", Path: "/"},
			{Name: "controlled\rname", Value: "unsafe", Domain: ".example.com", Path: "/"},
			{Name: "separated;name", Value: "unsafe", Domain: ".example.com", Path: "/"},
		},
		[]string{"example.com"},
		"https://example.com/account",
	)

	if standardLog.Len() != 0 {
		t.Fatalf("unsafe cookies wrote %d bytes to the standard log", standardLog.Len())
	}
	if got := req.Header.Get("Cookie"); got != "sessionid=valid-value" {
		t.Fatalf("Cookie header = %q", got)
	}
	if cookie, err := req.Cookie("sessionid"); err != nil || cookie.Value != "valid-value" {
		t.Fatalf("valid cookie was not preserved: cookie=%#v err=%v", cookie, err)
	}
}

func TestMapBilibiliNavAccountUsesStableVipKeyWhenAPIOmitsLabel(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name        string
		vipType     int
		wantTierKey string
	}{
		{
			name:        "monthly vip",
			vipType:     1,
			wantTierKey: "vip",
		},
		{
			name:        "annual vip",
			vipType:     2,
			wantTierKey: "vip_annual",
		},
	}

	for _, test := range tests {
		test := test
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			var decoded bilibiliNavResponse
			decoded.Data.IsLogin = true
			decoded.Data.Mid = 123456
			decoded.Data.Uname = "Bili User"
			decoded.Data.Vip.Type = test.vipType
			decoded.Data.Vip.Status = 1

			account := mapBilibiliNavAccount(decoded)
			if account.TierKey != test.wantTierKey || account.TierLabel != "" {
				t.Fatalf("tier = %q %q", account.TierKey, account.TierLabel)
			}
			if len(account.Badges) != 0 {
				t.Fatalf("badges = %#v", account.Badges)
			}
		})
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

func TestFetchTikTokAppSessionAccountFromURL(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/passport/web/account/info/" {
			t.Fatalf("unexpected path %s", r.URL.Path)
		}
		if r.Header.Get("Referer") != "https://www.tiktok.com/" {
			t.Fatalf("missing tiktok referer")
		}
		if _, err := r.Cookie("sessionid"); err != nil {
			t.Fatalf("missing sessionid cookie: %v", err)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{
			"message": "success",
			"data": {
				"user_id": "70001",
				"username": "tiktok_user",
				"screen_name": "TikTok User",
				"avatar_url": "//p16-sign.tiktokcdn-us.com/avatar.jpeg"
			}
		}`))
	}))
	defer server.Close()

	account, err := fetchTikTokAppSessionAccountFromURL(
		context.Background(),
		server.Client(),
		[]appcookies.Record{{Name: "sessionid", Value: "session-value", Domain: ".tiktok.com", Path: "/"}},
		server.URL+"/passport/web/account/info/?aid=1988",
	)
	if err != nil {
		t.Fatalf("fetch account: %v", err)
	}
	if account.DisplayName != "TikTok User" || account.Handle != "tiktok_user" {
		t.Fatalf("unexpected account = %#v", account)
	}
	if account.AvatarURL != "https://p16-sign.tiktokcdn-us.com/avatar.jpeg" {
		t.Fatalf("avatar url = %q", account.AvatarURL)
	}
	if account.Metadata["userID"] != "70001" {
		t.Fatalf("user metadata = %#v", account.Metadata)
	}
}

func TestFetchTikTokAppSessionAccountFromURLReturnsNoCookiesForSessionExpired(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"message":"session_expired","data":{"name":"session_expired"}}`))
	}))
	defer server.Close()

	_, err := fetchTikTokAppSessionAccountFromURL(
		context.Background(),
		server.Client(),
		[]appcookies.Record{{Name: "sessionid", Value: "session-value", Domain: ".tiktok.com", Path: "/"}},
		server.URL+"/passport/web/account/info/?aid=1988",
	)
	if !errors.Is(err, appsessions.ErrNoCookies) {
		t.Fatalf("expected ErrNoCookies, got %v", err)
	}
}

func TestFetchDouyinAppSessionAccountFromURL(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/aweme/v1/web/user/profile/self/" {
			t.Fatalf("unexpected path %s", r.URL.Path)
		}
		if r.Header.Get("Referer") != "https://www.douyin.com/" {
			t.Fatalf("missing douyin referer")
		}
		if _, err := r.Cookie("sessionid_ss"); err != nil {
			t.Fatalf("missing sessionid_ss cookie: %v", err)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{
			"status_code": 0,
			"user": {
				"uid": "71001",
				"sec_uid": "MS4wLjABAAAAfixture",
				"unique_id": "douyin_user",
				"nickname": "Douyin User",
				"avatar_thumb": {"url_list": ["http://douyin.test/avatar.jpeg"]}
			}
		}`))
	}))
	defer server.Close()

	account, err := fetchDouyinAppSessionAccountFromURL(
		context.Background(),
		server.Client(),
		[]appcookies.Record{{Name: "sessionid_ss", Value: "session-value", Domain: ".douyin.com", Path: "/"}},
		server.URL+"/aweme/v1/web/user/profile/self/?aid=6383",
	)
	if err != nil {
		t.Fatalf("fetch account: %v", err)
	}
	if account.DisplayName != "Douyin User" || account.Handle != "douyin_user" {
		t.Fatalf("unexpected account = %#v", account)
	}
	if account.AvatarURL != "https://douyin.test/avatar.jpeg" {
		t.Fatalf("avatar url = %q", account.AvatarURL)
	}
	if account.Metadata["userID"] != "71001" || account.Metadata["secureUserID"] != "MS4wLjABAAAAfixture" {
		t.Fatalf("user metadata = %#v", account.Metadata)
	}
}

func TestFetchDouyinAppSessionAccountFromURLReturnsNoCookiesWhenLoggedOut(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"status_code":8,"status_msg":"用户未登录","user":null}`))
	}))
	defer server.Close()

	_, err := fetchDouyinAppSessionAccountFromURL(
		context.Background(),
		server.Client(),
		[]appcookies.Record{{Name: "sessionid", Value: "session-value", Domain: ".douyin.com", Path: "/"}},
		server.URL+"/aweme/v1/web/user/profile/self/",
	)
	if !errors.Is(err, appsessions.ErrNoCookies) {
		t.Fatalf("expected ErrNoCookies, got %v", err)
	}
}

func TestFetchDouyinAppSessionAccountFromURLDoesNotTreatEmptyPayloadAsLoggedOut(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{}`))
	}))
	defer server.Close()

	_, err := fetchDouyinAppSessionAccountFromURL(
		context.Background(),
		server.Client(),
		[]appcookies.Record{{Name: "sessionid", Value: "session-value", Domain: ".douyin.com", Path: "/"}},
		server.URL+"/aweme/v1/web/user/profile/self/",
	)
	if err == nil || errors.Is(err, appsessions.ErrNoCookies) {
		t.Fatalf("expected malformed account response error, got %v", err)
	}
}

func TestFetchXiaohongshuAppSessionAccountFromURL(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/sns/web/v2/user/me" {
			t.Fatalf("unexpected path %s", r.URL.Path)
		}
		if r.Header.Get("Origin") != "https://www.xiaohongshu.com" || r.Header.Get("Referer") != "https://www.xiaohongshu.com/" {
			t.Fatalf("missing Xiaohongshu origin headers")
		}
		if _, err := r.Cookie("web_session"); err != nil {
			t.Fatalf("missing web_session cookie: %v", err)
		}
		if _, err := r.Cookie("short_link_cookie"); err == nil {
			t.Fatal("short-link cookie crossed into the Xiaohongshu account API")
		}
		if _, err := r.Cookie("creator_cookie"); err == nil {
			t.Fatal("creator-only cookie crossed into the Xiaohongshu account API")
		}
		if _, err := r.Cookie("path_cookie"); err == nil {
			t.Fatal("path-scoped cookie crossed into the Xiaohongshu account API")
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{
			"code": 0,
			"success": true,
			"data": {
				"guest": false,
				"basic_info": {
					"user_id": "64dccf7d000000000100577e",
					"red_id": "red-user",
					"nickname": "XHS User",
					"imageb": "http://sns-avatar.test/avatar.jpeg"
				}
			}
		}`))
	}))
	defer server.Close()

	account, err := fetchXiaohongshuAppSessionAccountFromURL(
		context.Background(),
		server.Client(),
		[]appcookies.Record{
			{Name: "a1", Value: "device-value", Domain: ".xiaohongshu.com", Path: "/"},
			{Name: "web_session", Value: "session-value", Domain: ".xiaohongshu.com", Path: "/"},
			{Name: "short_link_cookie", Value: "must-not-send", Domain: ".xhslink.com", Path: "/"},
			{Name: "creator_cookie", Value: "must-not-send", Domain: ".creator.xiaohongshu.com", Path: "/"},
			{Name: "path_cookie", Value: "must-not-send", Domain: ".xiaohongshu.com", Path: "/creator"},
		},
		server.URL+"/api/sns/web/v2/user/me",
	)
	if err != nil {
		t.Fatalf("fetch account: %v", err)
	}
	if account.DisplayName != "XHS User" || account.Handle != "red-user" {
		t.Fatalf("unexpected account = %#v", account)
	}
	if account.AvatarURL != "https://sns-avatar.test/avatar.jpeg" {
		t.Fatalf("avatar url = %q", account.AvatarURL)
	}
	if account.Metadata["userID"] != "64dccf7d000000000100577e" {
		t.Fatalf("user metadata = %#v", account.Metadata)
	}
}

func TestFetchXiaohongshuAppSessionAccountFromURLRejectsLoggedOutAndGuestSessions(t *testing.T) {
	t.Parallel()

	for _, testCase := range []struct {
		name string
		body string
	}{
		{name: "missing login", body: `{"code":-101,"success":false,"msg":"无登录信息，或登录信息为空","data":{}}`},
		{name: "guest", body: `{"code":0,"success":true,"data":{"guest":true,"user_id":"guest","nickname":"Guest"}}`},
	} {
		t.Run(testCase.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				w.Header().Set("Content-Type", "application/json")
				_, _ = w.Write([]byte(testCase.body))
			}))
			defer server.Close()

			_, err := fetchXiaohongshuAppSessionAccountFromURL(
				context.Background(),
				server.Client(),
				[]appcookies.Record{{Name: "web_session", Value: "session-value", Domain: ".xiaohongshu.com", Path: "/"}},
				server.URL+"/api/sns/web/v2/user/me",
			)
			if !errors.Is(err, appsessions.ErrNoCookies) {
				t.Fatalf("expected ErrNoCookies, got %v", err)
			}
		})
	}
}

func TestFetchXiaohongshuAppSessionAccountFromURLKeepsRiskControlAsVerificationError(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"code":300015,"success":false,"msg":"signature verification failed","data":{}}`))
	}))
	defer server.Close()

	_, err := fetchXiaohongshuAppSessionAccountFromURL(
		context.Background(),
		server.Client(),
		[]appcookies.Record{{Name: "web_session", Value: "session-value", Domain: ".xiaohongshu.com", Path: "/"}},
		server.URL+"/api/sns/web/v2/user/me",
	)
	if err == nil || errors.Is(err, appsessions.ErrNoCookies) {
		t.Fatalf("expected actionable verification error, got %v", err)
	}
}

func TestFetchXiaohongshuAppSessionAccountFromURLKeepsTransportChallengesAsVerificationErrors(t *testing.T) {
	t.Parallel()

	for _, testCase := range []struct {
		name        string
		status      int
		contentType string
		body        string
	}{
		{name: "forbidden", status: http.StatusForbidden, contentType: "application/json", body: `{"code":300015,"success":false}`},
		{name: "html challenge", status: http.StatusOK, contentType: "text/html", body: `<html><title>verification</title></html>`},
	} {
		t.Run(testCase.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				w.Header().Set("Content-Type", testCase.contentType)
				w.WriteHeader(testCase.status)
				_, _ = w.Write([]byte(testCase.body))
			}))
			defer server.Close()

			_, err := fetchXiaohongshuAppSessionAccountFromURL(
				context.Background(),
				server.Client(),
				[]appcookies.Record{{Name: "web_session", Value: "session-value", Domain: ".xiaohongshu.com", Path: "/"}},
				server.URL+"/api/sns/web/v2/user/me",
			)
			if err == nil || errors.Is(err, appsessions.ErrNoCookies) {
				t.Fatalf("expected verification error, got %v", err)
			}
		})
	}
}

func TestFetchInstagramAppSessionAccountFromURL(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/v1/accounts/current_user/" {
			t.Fatalf("unexpected path %s", r.URL.Path)
		}
		if r.Header.Get("X-IG-App-ID") == "" {
			t.Fatalf("missing instagram app id")
		}
		if _, err := r.Cookie("sessionid"); err != nil {
			t.Fatalf("missing sessionid cookie: %v", err)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{
			"user": {
				"pk": "90001",
				"username": "insta_user",
				"full_name": "Instagram User",
				"hd_profile_pic_url_info": {
					"url": "http://instagram.test/avatar-hd.jpg"
				},
				"profile_pic_url_hd": "http://instagram.test/avatar.jpg"
			}
		}`))
	}))
	defer server.Close()

	account, err := fetchInstagramAppSessionAccountFromURL(
		context.Background(),
		server.Client(),
		[]appcookies.Record{{Name: "sessionid", Value: "session-value", Domain: ".instagram.com", Path: "/"}},
		server.URL+"/api/v1/accounts/current_user/?edit=true",
	)
	if err != nil {
		t.Fatalf("fetch account: %v", err)
	}
	if account.DisplayName != "Instagram User" || account.Handle != "insta_user" {
		t.Fatalf("unexpected account = %#v", account)
	}
	if account.AvatarURL != "https://instagram.test/avatar-hd.jpg" {
		t.Fatalf("avatar url = %q", account.AvatarURL)
	}
	if account.Metadata["userID"] != "90001" {
		t.Fatalf("user metadata = %#v", account.Metadata)
	}
}

func TestFetchInstagramAppSessionAccountFromURLParsesPrefixedJSONAndAvatarVersions(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`for (;;);{
			"user": {
				"pk_id": "90003",
				"username": "insta_user",
				"hd_profile_pic_versions": [
					{"url": "http://instagram.test/avatar-small.jpg"},
					{"url": "http://instagram.test/avatar-large.jpg"}
				]
			}
		}`))
	}))
	defer server.Close()

	account, err := fetchInstagramAppSessionAccountFromURL(
		context.Background(),
		server.Client(),
		[]appcookies.Record{{Name: "sessionid", Value: "session-value", Domain: ".instagram.com", Path: "/"}},
		server.URL+"/api/v1/accounts/current_user/?edit=true",
	)
	if err != nil {
		t.Fatalf("fetch account: %v", err)
	}
	if account.DisplayName != "insta_user" || account.Handle != "insta_user" {
		t.Fatalf("unexpected account = %#v", account)
	}
	if account.AvatarURL != "https://instagram.test/avatar-large.jpg" {
		t.Fatalf("avatar url = %q", account.AvatarURL)
	}
	if account.Metadata["userID"] != "90003" {
		t.Fatalf("user metadata = %#v", account.Metadata)
	}
}

func TestFetchInstagramAppSessionAccountFromURLFallsBackToEditFormAndProfile(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if _, err := r.Cookie("sessionid"); err != nil {
			t.Fatalf("missing sessionid cookie: %v", err)
		}
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/api/v1/accounts/current_user/":
			w.WriteHeader(http.StatusBadRequest)
			_, _ = w.Write([]byte(`{"status":"fail","message":"bad request"}`))
		case "/api/v1/accounts/edit/web_form_data/":
			_, _ = w.Write([]byte(`{
				"form_data": {
					"username": "insta_user",
					"first_name": "Instagram User"
				}
			}`))
		case "/api/v1/users/web_profile_info/":
			if r.URL.Query().Get("username") != "insta_user" {
				t.Fatalf("unexpected username query %q", r.URL.RawQuery)
			}
			_, _ = w.Write([]byte(`{
				"data": {
					"user": {
						"id": "90004",
						"username": "insta_user",
						"full_name": "Instagram User",
						"profile_pic_url_hd": "http://instagram.test/profile.jpg"
					}
				}
			}`))
		default:
			t.Fatalf("unexpected path %s", r.URL.Path)
		}
	}))
	defer server.Close()

	account, err := fetchInstagramAppSessionAccountFromURL(
		context.Background(),
		server.Client(),
		[]appcookies.Record{{Name: "sessionid", Value: "session-value", Domain: ".instagram.com", Path: "/"}},
		server.URL+"/api/v1/accounts/current_user/?edit=true",
	)
	if err != nil {
		t.Fatalf("fetch account: %v", err)
	}
	if account.DisplayName != "Instagram User" || account.Handle != "insta_user" {
		t.Fatalf("unexpected account = %#v", account)
	}
	if account.AvatarURL != "https://instagram.test/profile.jpg" {
		t.Fatalf("avatar url = %q", account.AvatarURL)
	}
	if account.Metadata["accountEndpoint"] != "api/v1/accounts/edit/web_form_data" ||
		account.Metadata["profileEndpoint"] != "api/v1/users/web_profile_info" ||
		account.Metadata["userID"] != "90004" {
		t.Fatalf("metadata = %#v", account.Metadata)
	}
}

func TestFetchInstagramAppSessionAccountFromURLEmbedsAvatarDataURL(t *testing.T) {
	t.Parallel()

	var server *httptest.Server
	server = httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/api/v1/accounts/current_user/":
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{
				"user": {
					"pk": "90005",
					"username": "insta_user",
					"full_name": "Instagram User",
					"profile_pic_url_hd": "` + server.URL + `/avatar.jpg"
				}
			}`))
		case "/avatar.jpg":
			if r.Header.Get("Referer") != "https://www.instagram.com/" {
				t.Fatalf("avatar referer = %q", r.Header.Get("Referer"))
			}
			w.Header().Set("Content-Type", "image/jpeg")
			_, _ = w.Write([]byte{0xff, 0xd8, 0xff, 0xd9})
		default:
			t.Fatalf("unexpected path %s", r.URL.Path)
		}
	}))
	defer server.Close()

	account, err := fetchInstagramAppSessionAccountFromURL(
		context.Background(),
		server.Client(),
		[]appcookies.Record{{Name: "sessionid", Value: "session-value", Domain: ".instagram.com", Path: "/"}},
		server.URL+"/api/v1/accounts/current_user/?edit=true",
	)
	if err != nil {
		t.Fatalf("fetch account: %v", err)
	}
	if !strings.HasPrefix(account.AvatarURL, "data:image/jpeg;base64,") {
		t.Fatalf("avatar url = %q", account.AvatarURL)
	}
}

func TestFetchInstagramAppSessionAccountFromURLReturnsNoCookiesForLoginRequired(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"status":"fail","message":"login_required"}`))
	}))
	defer server.Close()

	_, err := fetchInstagramAppSessionAccountFromURL(
		context.Background(),
		server.Client(),
		[]appcookies.Record{{Name: "sessionid", Value: "session-value", Domain: ".instagram.com", Path: "/"}},
		server.URL+"/api/v1/accounts/current_user/?edit=true",
	)
	if !errors.Is(err, appsessions.ErrNoCookies) {
		t.Fatalf("expected ErrNoCookies, got %v", err)
	}
}

func TestFetchXAppSessionAccountFromURL(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !strings.HasPrefix(r.Header.Get("Authorization"), "Bearer ") {
			t.Fatalf("missing x bearer authorization")
		}
		if r.Header.Get("X-CSRF-Token") != "csrf-value" {
			t.Fatalf("missing x csrf token")
		}
		if _, err := r.Cookie("auth_token"); err != nil {
			t.Fatalf("missing auth_token cookie: %v", err)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{
			"id_str": "12345",
			"screen_name": "x_user",
			"name": "X User",
			"profile_image_url_https": "https://pbs.twimg.com/profile_images/avatar_normal.jpg",
			"verified": true
		}`))
	}))
	defer server.Close()

	account, err := fetchXAppSessionAccountFromURL(
		context.Background(),
		server.Client(),
		[]appcookies.Record{
			{Name: "auth_token", Value: "auth-value", Domain: ".x.com", Path: "/"},
			{Name: "ct0", Value: "csrf-value", Domain: ".x.com", Path: "/"},
		},
		server.URL+"/1.1/account/verify_credentials.json",
	)
	if err != nil {
		t.Fatalf("fetch account: %v", err)
	}
	if account.DisplayName != "X User" || account.Handle != "x_user" {
		t.Fatalf("unexpected account = %#v", account)
	}
	if account.AvatarURL != "https://pbs.twimg.com/profile_images/avatar_normal.jpg" {
		t.Fatalf("avatar url = %q", account.AvatarURL)
	}
	if account.Metadata["userID"] != "12345" || account.Metadata["verified"] != true {
		t.Fatalf("metadata = %#v", account.Metadata)
	}
}

func TestFetchXAppSessionAccountFromURLFallsBackToViewerGraphQL(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") == "" {
			t.Fatalf("missing x bearer authorization")
		}
		if r.Header.Get("X-CSRF-Token") != "csrf-value" {
			t.Fatalf("missing x csrf token")
		}
		if _, err := r.Cookie("auth_token"); err != nil {
			t.Fatalf("missing auth_token cookie: %v", err)
		}
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/1.1/account/verify_credentials.json":
			w.WriteHeader(http.StatusNotFound)
			_, _ = w.Write([]byte(`{"errors":[{"message":"Not Found"}]}`))
		case "/i/api/graphql/u4ni7JqpqdAQxWQfkLsdUQ/Viewer":
			if r.URL.Query().Get("variables") != "{}" || r.URL.Query().Get("features") == "" || r.URL.Query().Get("fieldToggles") == "" {
				t.Fatalf("missing graphql params %q", r.URL.RawQuery)
			}
			_, _ = w.Write([]byte(`{
				"data": {
					"viewer": {
						"user_results": {
							"result": {
								"__typename": "User",
								"rest_id": "12345",
								"is_blue_verified": true,
								"legacy": {
									"screen_name": "x_user",
									"name": "X User",
									"profile_image_url_https": "https://pbs.twimg.com/profile_images/avatar_normal.jpg"
								}
							}
						}
					}
				}
			}`))
		default:
			t.Fatalf("unexpected path %s", r.URL.Path)
		}
	}))
	defer server.Close()

	account, err := fetchXAppSessionAccountFromURL(
		context.Background(),
		server.Client(),
		[]appcookies.Record{
			{Name: "auth_token", Value: "auth-value", Domain: ".x.com", Path: "/"},
			{Name: "ct0", Value: "csrf-value", Domain: ".x.com", Path: "/"},
		},
		server.URL+"/1.1/account/verify_credentials.json",
	)
	if err != nil {
		t.Fatalf("fetch account: %v", err)
	}
	if account.DisplayName != "X User" || account.Handle != "x_user" {
		t.Fatalf("unexpected account = %#v", account)
	}
	if account.AvatarURL != "https://pbs.twimg.com/profile_images/avatar_normal.jpg" {
		t.Fatalf("avatar url = %q", account.AvatarURL)
	}
	if account.Metadata["accountEndpoint"] != "graphql/Viewer" ||
		account.Metadata["userID"] != "12345" ||
		account.Metadata["blueVerified"] != true {
		t.Fatalf("metadata = %#v", account.Metadata)
	}
}

func TestFetchXAppSessionAccountFromURLFallsBackToTWIDAndUserByRestIDGraphQL(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/1.1/account/verify_credentials.json":
			w.WriteHeader(http.StatusNotFound)
			_, _ = w.Write([]byte(`{"errors":[{"message":"Not Found"}]}`))
		case "/i/api/graphql/u4ni7JqpqdAQxWQfkLsdUQ/Viewer":
			w.WriteHeader(http.StatusForbidden)
			_, _ = w.Write([]byte(`{"errors":[{"message":"Forbidden"}]}`))
		case "/i/api/graphql/DaeC_2LfMgwCujE03HSZtw/UserByRestId":
			if !strings.Contains(r.URL.Query().Get("variables"), `"userId":"12345"`) {
				t.Fatalf("unexpected graphql variables %q", r.URL.Query().Get("variables"))
			}
			if strings.Contains(r.Header.Get("Cookie"), "personalization_id=") {
				t.Fatalf("cookie header included an unsafe quoted value: %q", r.Header.Get("Cookie"))
			}
			_, _ = w.Write([]byte(`{
				"data": {
					"user": {
						"result": {
							"__typename": "User",
							"rest_id": "12345",
							"avatar": {
								"image_url": "https://pbs.twimg.com/profile_images/avatar_normal.jpg"
							},
							"core": {
								"screen_name": "x_user",
								"name": "X User"
							},
							"legacy": {
								"default_profile": true
							}
						}
					}
				}
			}`))
		default:
			t.Fatalf("unexpected path %s", r.URL.Path)
		}
	}))
	defer server.Close()

	account, err := fetchXAppSessionAccountFromURL(
		context.Background(),
		server.Client(),
		[]appcookies.Record{
			{Name: "auth_token", Value: "auth-value", Domain: ".x.com", Path: "/"},
			{Name: "ct0", Value: "csrf-value", Domain: ".x.com", Path: "/"},
			{Name: "twid", Value: "u%3D12345", Domain: ".x.com", Path: "/"},
			{Name: "personalization_id", Value: `"v1_test"`, Domain: ".x.com", Path: "/"},
		},
		server.URL+"/1.1/account/verify_credentials.json",
	)
	if err != nil {
		t.Fatalf("fetch account: %v", err)
	}
	if account.DisplayName != "X User" || account.Handle != "x_user" {
		t.Fatalf("unexpected account = %#v", account)
	}
	if account.AvatarURL != "https://pbs.twimg.com/profile_images/avatar_normal.jpg" {
		t.Fatalf("avatar url = %q", account.AvatarURL)
	}
	if account.Metadata["accountEndpoint"] != "graphql/UserByRestId" || account.Metadata["userID"] != "12345" {
		t.Fatalf("metadata = %#v", account.Metadata)
	}
}

func TestFetchXAppSessionAccountFromURLFallsBackToSettingsHandleOnly(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/1.1/account/verify_credentials.json":
			w.WriteHeader(http.StatusNotFound)
			_, _ = w.Write([]byte(`{"errors":[{"message":"Not Found"}]}`))
		case "/i/api/graphql/u4ni7JqpqdAQxWQfkLsdUQ/Viewer":
			w.WriteHeader(http.StatusForbidden)
			_, _ = w.Write([]byte(`{"errors":[{"message":"Forbidden"}]}`))
		case "/i/api/1.1/account/settings.json":
			_, _ = w.Write([]byte(`{"screen_name":"x_user"}`))
		case "/i/api/1.1/users/show.json":
			w.WriteHeader(http.StatusForbidden)
			_, _ = w.Write([]byte(`{"errors":[{"message":"Forbidden"}]}`))
		default:
			t.Fatalf("unexpected path %s", r.URL.Path)
		}
	}))
	defer server.Close()

	account, err := fetchXAppSessionAccountFromURL(
		context.Background(),
		server.Client(),
		[]appcookies.Record{
			{Name: "auth_token", Value: "auth-value", Domain: ".x.com", Path: "/"},
			{Name: "ct0", Value: "csrf-value", Domain: ".x.com", Path: "/"},
		},
		server.URL+"/1.1/account/verify_credentials.json",
	)
	if err != nil {
		t.Fatalf("fetch account: %v", err)
	}
	if account.DisplayName != "x_user" || account.Handle != "x_user" {
		t.Fatalf("unexpected account = %#v", account)
	}
	if account.AvatarURL != "" {
		t.Fatalf("avatar url = %q", account.AvatarURL)
	}
	if account.Metadata["accountEndpoint"] != "1.1/account/settings" {
		t.Fatalf("metadata = %#v", account.Metadata)
	}
}

func TestFetchFacebookAppSessionAccountFromURL(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if _, err := r.Cookie("c_user"); err != nil {
			t.Fatalf("missing c_user cookie: %v", err)
		}
		w.Header().Set("Content-Type", "text/html")
		switch r.URL.Path {
		case "/me":
			_, _ = w.Write([]byte(`<html><head><title>Redirecting...</title><meta http-equiv="refresh" content="0; url=/profile.php?id=10101"></head></html>`))
		case "/profile.php":
			if r.URL.Query().Get("id") != "10101" {
				t.Fatalf("unexpected profile id %q", r.URL.Query().Get("id"))
			}
			_, _ = w.Write([]byte(`<html><head>
				<meta property="og:title" content="Facebook User | Facebook">
				<meta property="og:image" content="https://scontent.xx.fbcdn.net/v/t39.30808/avatar.jpg?oh=1&amp;oe=2">
				<link rel="canonical" href="https://www.facebook.com/facebook.user">
			</head><body>{"NAME":"Facebook User"}</body></html>`))
		default:
			t.Fatalf("unexpected path %s", r.URL.Path)
		}
	}))
	defer server.Close()

	account, err := fetchFacebookAppSessionAccountFromURL(
		context.Background(),
		server.Client(),
		[]appcookies.Record{
			{Name: "c_user", Value: "10101", Domain: ".facebook.com", Path: "/"},
			{Name: "xs", Value: "xs-value", Domain: ".facebook.com", Path: "/"},
		},
		server.URL+"/me",
	)
	if err != nil {
		t.Fatalf("fetch account: %v", err)
	}
	if account.DisplayName != "Facebook User" || account.Handle != "facebook.user" {
		t.Fatalf("unexpected account = %#v", account)
	}
	if account.AvatarURL != "https://scontent.xx.fbcdn.net/v/t39.30808/avatar.jpg?oh=1&oe=2" {
		t.Fatalf("avatar url = %q", account.AvatarURL)
	}
	if account.Metadata["userID"] != "10101" {
		t.Fatalf("metadata = %#v", account.Metadata)
	}
}

func TestExtractFacebookDisplayNameIgnoresRedirectingShell(t *testing.T) {
	t.Parallel()

	name := extractFacebookDisplayName(`<html><head><title>Redirecting...</title></head><body>{"name":"Facebook"}</body></html>`)
	if name != "" {
		t.Fatalf("display name = %q", name)
	}
}

func TestFetchVimeoAppSessionAccountFromURL(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if _, err := r.Cookie("vimeo"); err != nil {
			t.Fatalf("missing vimeo cookie: %v", err)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{
			"user": {
				"id": 111,
				"name": "Vimeo User",
				"link": "https://vimeo.com/vimeo-user",
				"pictures": {
					"sizes": [
						{"link": "https://i.vimeocdn.com/small.jpg"},
						{"link": "//i.vimeocdn.com/large.jpg"}
					]
				}
			}
		}`))
	}))
	defer server.Close()

	account, err := fetchVimeoAppSessionAccountFromURL(
		context.Background(),
		server.Client(),
		[]appcookies.Record{{Name: "vimeo", Value: "session-value", Domain: ".vimeo.com", Path: "/"}},
		server.URL+"/_next/viewer",
	)
	if err != nil {
		t.Fatalf("fetch account: %v", err)
	}
	if account.DisplayName != "Vimeo User" || account.Handle != "vimeo-user" {
		t.Fatalf("unexpected account = %#v", account)
	}
	if account.AvatarURL != "https://i.vimeocdn.com/large.jpg" {
		t.Fatalf("avatar url = %q", account.AvatarURL)
	}
	if account.Metadata["userID"] != "111" {
		t.Fatalf("metadata = %#v", account.Metadata)
	}
}

func TestFetchTwitchAppSessionAccountFromURLs(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/oauth2/validate":
			if r.Header.Get("Authorization") != "OAuth auth-token-value" {
				t.Fatalf("missing twitch oauth header")
			}
			_, _ = w.Write([]byte(`{"client_id":"client-123","login":"twitch_user","user_id":"70002"}`))
		case "/helix/users":
			if r.Header.Get("Authorization") != "Bearer auth-token-value" {
				t.Fatalf("missing twitch bearer header")
			}
			if r.Header.Get("Client-Id") != "client-123" {
				t.Fatalf("missing twitch client id")
			}
			if r.URL.Query().Get("id") != "70002" {
				t.Fatalf("unexpected twitch user id query")
			}
			_, _ = w.Write([]byte(`{"data":[{"id":"70002","login":"twitch_user","display_name":"Twitch User","profile_image_url":"https://static-cdn.jtvnw.net/avatar.png"}]}`))
		default:
			t.Fatalf("unexpected path %s", r.URL.Path)
		}
	}))
	defer server.Close()

	account, err := fetchTwitchAppSessionAccountFromURLs(
		context.Background(),
		server.Client(),
		[]appcookies.Record{{Name: "auth-token", Value: "auth-token-value", Domain: ".twitch.tv", Path: "/"}},
		server.URL+"/oauth2/validate",
		server.URL+"/helix/users",
	)
	if err != nil {
		t.Fatalf("fetch account: %v", err)
	}
	if account.DisplayName != "Twitch User" || account.Handle != "twitch_user" {
		t.Fatalf("unexpected account = %#v", account)
	}
	if account.AvatarURL != "https://static-cdn.jtvnw.net/avatar.png" {
		t.Fatalf("avatar url = %q", account.AvatarURL)
	}
}

func TestFetchTwitchAppSessionAccountFromURLsFallsBackToGraphQL(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/oauth2/validate":
			w.WriteHeader(http.StatusNotFound)
			_, _ = w.Write([]byte(`{"message":"Not Found"}`))
		case "/gql":
			if r.Method != http.MethodPost {
				t.Fatalf("expected POST, got %s", r.Method)
			}
			if r.Header.Get("Authorization") != "OAuth auth-token-value" {
				t.Fatalf("missing twitch gql oauth header")
			}
			if r.Header.Get("Client-ID") == "" {
				t.Fatalf("missing twitch gql client id")
			}
			if _, err := r.Cookie("auth-token"); err != nil {
				t.Fatalf("missing auth-token cookie: %v", err)
			}
			_, _ = w.Write([]byte(`{
				"data": {
					"currentUser": {
						"id": "70002",
						"login": "twitch_user",
						"displayName": "Twitch User",
						"profileImageURL": "https://static-cdn.jtvnw.net/avatar.png"
					}
				}
			}`))
		default:
			t.Fatalf("unexpected path %s", r.URL.Path)
		}
	}))
	defer server.Close()

	account, err := fetchTwitchAppSessionAccountFromEndpoints(
		context.Background(),
		server.Client(),
		[]appcookies.Record{{Name: "auth-token", Value: "auth-token-value", Domain: ".twitch.tv", Path: "/"}},
		server.URL+"/oauth2/validate",
		server.URL+"/helix/users",
		server.URL+"/gql",
	)
	if err != nil {
		t.Fatalf("fetch account: %v", err)
	}
	if account.DisplayName != "Twitch User" || account.Handle != "twitch_user" {
		t.Fatalf("unexpected account = %#v", account)
	}
	if account.AvatarURL != "https://static-cdn.jtvnw.net/avatar.png" {
		t.Fatalf("avatar url = %q", account.AvatarURL)
	}
	if account.Metadata["accountEndpoint"] != "gql/currentUser" || account.Metadata["userID"] != "70002" {
		t.Fatalf("metadata = %#v", account.Metadata)
	}
}

func TestFetchNiconicoAppSessionAccountFromURL(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("X-Frontend-Id") != "6" {
			t.Fatalf("missing niconico frontend id")
		}
		if _, err := r.Cookie("user_session"); err != nil {
			t.Fatalf("missing user_session cookie: %v", err)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{
			"meta": {"status": 200},
			"data": {
				"user": {
					"id": 80003,
					"nickname": "Nico User",
					"icons": {"large": "//secure-dcdn.cdn.nimg.jp/avatar.jpg"},
					"isPremium": true
				}
			}
		}`))
	}))
	defer server.Close()

	account, err := fetchNiconicoAppSessionAccountFromURL(
		context.Background(),
		server.Client(),
		[]appcookies.Record{{Name: "user_session", Value: "session-value", Domain: ".nicovideo.jp", Path: "/"}},
		server.URL+"/v1/users/me",
	)
	if err != nil {
		t.Fatalf("fetch account: %v", err)
	}
	if account.DisplayName != "Nico User" || account.Handle != "" {
		t.Fatalf("unexpected account = %#v", account)
	}
	if account.AvatarURL != "https://secure-dcdn.cdn.nimg.jp/avatar.jpg" {
		t.Fatalf("avatar url = %q", account.AvatarURL)
	}
	if account.TierKey != "premium" || account.TierLabel != "Premium" {
		t.Fatalf("tier = %q %q", account.TierKey, account.TierLabel)
	}
}

func TestAppSessionAccountFetcherUnsupportedSites(t *testing.T) {
	t.Parallel()

	fetcher := newAppSessionAccountFetcher(nil)
	_, err := fetcher(context.Background(), "china_private", []appcookies.Record{{Name: "sessionid", Value: "value", Domain: "example.com", Path: "/"}})
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
