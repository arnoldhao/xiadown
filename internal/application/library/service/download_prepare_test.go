package service

import (
	"context"
	"testing"

	"xiadown/internal/application/apperrors"
	appsessionsdto "xiadown/internal/application/appsessions/dto"
	"xiadown/internal/application/library/dto"
)

func TestValidateDownloadURLAcceptsValidDomainAndAddsScheme(t *testing.T) {
	t.Parallel()

	cases := []struct {
		input   string
		wantURL string
	}{
		{
			input:   "www.youtube.com/watch?v=AbCdEfGh123",
			wantURL: "https://www.youtube.com/watch?v=AbCdEfGh123",
		},
		{
			input:   "www.youtube.com:443/watch?v=AbCdEfGh123",
			wantURL: "https://www.youtube.com:443/watch?v=AbCdEfGh123",
		},
	}
	for _, tc := range cases {
		gotURL, gotDomain, err := validateDownloadURL(tc.input)
		if err != nil {
			t.Fatalf("validateDownloadURL(%q) error = %v", tc.input, err)
		}
		if gotURL != tc.wantURL {
			t.Fatalf("url = %q, want %q", gotURL, tc.wantURL)
		}
		if gotDomain != "youtube.com" {
			t.Fatalf("domain = %q", gotDomain)
		}
	}
}

func TestValidateDownloadURLRejectsInvalidDomain(t *testing.T) {
	t.Parallel()

	for _, rawURL := range []string{
		"localhost/watch?v=AbCdEfGh123",
		"https://watch?v=AbCdEfGh123",
		"watch?v=too-short",
		"video/not-a-bilibili-id",
	} {
		if _, _, err := validateDownloadURL(rawURL); err == nil {
			t.Fatalf("expected %q to be rejected", rawURL)
		}
	}
}

func TestValidateDownloadURLCompletesKnownVideoSuffixes(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name       string
		input      string
		wantURL    string
		wantDomain string
	}{
		{
			name:       "youtube watch path",
			input:      "watch?v=AbCdEfGh123&t=1s",
			wantURL:    "https://www.youtube.com/watch?v=AbCdEfGh123&t=1s",
			wantDomain: "youtube.com",
		},
		{
			name:       "youtube query suffix",
			input:      "?v=AbCdEfGh123",
			wantURL:    "https://www.youtube.com/watch?v=AbCdEfGh123",
			wantDomain: "youtube.com",
		},
		{
			name:       "youtube naked id",
			input:      "AbCdEfGh123",
			wantURL:    "https://www.youtube.com/watch?v=AbCdEfGh123",
			wantDomain: "youtube.com",
		},
		{
			name:       "youtube shorts path",
			input:      "shorts/QwErTyUi123",
			wantURL:    "https://www.youtube.com/shorts/QwErTyUi123",
			wantDomain: "youtube.com",
		},
		{
			name:       "bilibili video path",
			input:      "video/BVfixture003",
			wantURL:    "https://www.bilibili.com/video/BVfixture003",
			wantDomain: "bilibili.com",
		},
		{
			name:       "bilibili av path",
			input:      "/video/av0000000000/",
			wantURL:    "https://www.bilibili.com/video/av0000000000/",
			wantDomain: "bilibili.com",
		},
		{
			name:       "bilibili naked bvid",
			input:      "BVfixture003",
			wantURL:    "https://www.bilibili.com/video/BVfixture003",
			wantDomain: "bilibili.com",
		},
		{
			name:       "bilibili festival bvid",
			input:      "festival/sample-event?bvid=BVfixtureFestival&",
			wantURL:    "https://www.bilibili.com/festival/sample-event?bvid=BVfixtureFestival&",
			wantDomain: "bilibili.com",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			gotURL, gotDomain, err := validateDownloadURL(tc.input)
			if err != nil {
				t.Fatalf("validateDownloadURL() error = %v", err)
			}
			if gotURL != tc.wantURL {
				t.Fatalf("url = %q, want %q", gotURL, tc.wantURL)
			}
			if gotDomain != tc.wantDomain {
				t.Fatalf("domain = %q, want %q", gotDomain, tc.wantDomain)
			}
		})
	}
}

func TestParseDownloadURLsExtractsSingleURLFromShareText(t *testing.T) {
	t.Parallel()

	raw := "【示例分享标题】 https://www.bilibili.com/video/BVfixture001/?share_source=copy_web&vd_source=test_source"
	items, err := parseDownloadURLs(raw)
	if err != nil {
		t.Fatalf("parseDownloadURLs() error = %v", err)
	}
	if len(items) != 1 {
		t.Fatalf("len = %d, want 1: %#v", len(items), items)
	}
	if items[0].URL != "https://www.bilibili.com/video/BVfixture001/?share_source=copy_web&vd_source=test_source" {
		t.Fatalf("url = %q", items[0].URL)
	}
	if items[0].Domain != "bilibili.com" {
		t.Fatalf("domain = %q", items[0].Domain)
	}
}

func TestValidateDownloadURLKeepsBilibiliShareURLSingle(t *testing.T) {
	t.Parallel()

	raw := "https://www.bilibili.com/video/BVfixture002/?share_source=copy_web&vd_source=test_source"
	gotURL, gotDomain, err := validateDownloadURL(raw)
	if err != nil {
		t.Fatalf("validateDownloadURL() error = %v", err)
	}
	if gotURL != raw {
		t.Fatalf("url = %q", gotURL)
	}
	if gotDomain != "bilibili.com" {
		t.Fatalf("domain = %q", gotDomain)
	}
}

func TestParseDownloadURLsExtractsSchemeLessURLFromText(t *testing.T) {
	t.Parallel()

	items, err := parseDownloadURLs("下载 www.youtube.com/watch?v=AbCdEfGh123&t=1s")
	if err != nil {
		t.Fatalf("parseDownloadURLs() error = %v", err)
	}
	if len(items) != 1 {
		t.Fatalf("len = %d, want 1: %#v", len(items), items)
	}
	if items[0].URL != "https://www.youtube.com/watch?v=AbCdEfGh123&t=1s" {
		t.Fatalf("url = %q", items[0].URL)
	}
}

func TestParseDownloadURLsExtractsMultipleURLsInOrderAndDedupes(t *testing.T) {
	t.Parallel()

	raw := "https://www.bilibili.com/video/BVfixture003\nhttps://youtu.be/AbCdEfGh123\nhttps://www.bilibili.com/video/BVfixture003"
	items, err := parseDownloadURLs(raw)
	if err != nil {
		t.Fatalf("parseDownloadURLs() error = %v", err)
	}
	if len(items) != 2 {
		t.Fatalf("len = %d, want 2: %#v", len(items), items)
	}
	if items[0].URL != "https://www.bilibili.com/video/BVfixture003" {
		t.Fatalf("first url = %q", items[0].URL)
	}
	if items[1].URL != "https://youtu.be/AbCdEfGh123" {
		t.Fatalf("second url = %q", items[1].URL)
	}
}

func TestParseDownloadURLsDoesNotMergeYouTubeQueryWithNextLine(t *testing.T) {
	t.Parallel()

	raw := `https://youtu.be/XyZabc12345?si=test_share_token
8.20 02/12 Uyg:/ :7pm r@r.eB 示例描述文本 60帧修复 完整MV 示例标签 https://v.douyin.com/testClipA/ 复制此链接，打开应用搜索，直接观看视频！
【【4K60FPS】示例标题 A】 https://www.bilibili.com/video/BVfixture004/?share_source=copy_web&vd_source=test_source
【【AI示例】示例标题 B】 https://www.bilibili.com/video/BVfixture005/?share_source=copy_web&vd_source=test_source
【【示例作者】示例标题 C】 https://www.bilibili.com/video/BVfixture006/?share_source=copy_web&vd_source=test_source`
	items, err := parseDownloadURLs(raw)
	if err != nil {
		t.Fatalf("parseDownloadURLs() error = %v", err)
	}
	want := []string{
		"https://youtu.be/XyZabc12345?si=test_share_token",
		"https://v.douyin.com/testClipA/",
		"https://www.bilibili.com/video/BVfixture004/?share_source=copy_web&vd_source=test_source",
		"https://www.bilibili.com/video/BVfixture005/?share_source=copy_web&vd_source=test_source",
		"https://www.bilibili.com/video/BVfixture006/?share_source=copy_web&vd_source=test_source",
	}
	if len(items) != len(want) {
		t.Fatalf("len = %d, want %d: %#v", len(items), len(want), items)
	}
	for index, wantURL := range want {
		if items[index].URL != wantURL {
			t.Fatalf("url[%d] = %q, want %q", index, items[index].URL, wantURL)
		}
	}
}

func TestParseDownloadURLsStopsExplicitURLAtWhitespace(t *testing.T) {
	t.Parallel()

	raw := "https://youtu.be/XyZabc12345?si=test_share_token 8.20 02/12 Uyg:/ :7pm r@r.eB https://v.douyin.com/testClipA/"
	items, err := parseDownloadURLs(raw)
	if err != nil {
		t.Fatalf("parseDownloadURLs() error = %v", err)
	}
	want := []string{
		"https://youtu.be/XyZabc12345?si=test_share_token",
		"https://v.douyin.com/testClipA/",
	}
	if len(items) != len(want) {
		t.Fatalf("len = %d, want %d: %#v", len(items), len(want), items)
	}
	for index, wantURL := range want {
		if items[index].URL != wantURL {
			t.Fatalf("url[%d] = %q, want %q", index, items[index].URL, wantURL)
		}
	}
}

func TestNormalizeYTDLPBatchItemsExpandsMultiURLItem(t *testing.T) {
	t.Parallel()

	items, err := normalizeYTDLPBatchItems([]dto.CreateYTDLPJobRequest{
		{
			URL:     "https://www.bilibili.com/video/BVfixture003\nhttps://youtu.be/AbCdEfGh123",
			Quality: "audio",
		},
	}, "batch-run")
	if err != nil {
		t.Fatalf("normalizeYTDLPBatchItems() error = %v", err)
	}
	if len(items) != 2 {
		t.Fatalf("len = %d, want 2: %#v", len(items), items)
	}
	if items[0].URL != "https://www.bilibili.com/video/BVfixture003" {
		t.Fatalf("first url = %q", items[0].URL)
	}
	if items[1].URL != "https://youtu.be/AbCdEfGh123" {
		t.Fatalf("second url = %q", items[1].URL)
	}
	for _, item := range items {
		if item.Quality != "audio" {
			t.Fatalf("quality = %q", item.Quality)
		}
		if item.RunID != "batch-run" {
			t.Fatalf("run id = %q", item.RunID)
		}
	}
}

func TestParseDownloadURLsSupportsStructuredShortBatch(t *testing.T) {
	t.Parallel()

	items, err := parseDownloadURLs("BVfixture003\nwatch?v=AbCdEfGh123")
	if err != nil {
		t.Fatalf("parseDownloadURLs() error = %v", err)
	}
	if len(items) != 2 {
		t.Fatalf("len = %d, want 2: %#v", len(items), items)
	}
	if items[0].URL != "https://www.bilibili.com/video/BVfixture003" {
		t.Fatalf("first url = %q", items[0].URL)
	}
	if items[1].URL != "https://www.youtube.com/watch?v=AbCdEfGh123" {
		t.Fatalf("second url = %q", items[1].URL)
	}
}

func TestParseDownloadURLsIgnoresEmailsAndLowConfidenceDomains(t *testing.T) {
	t.Parallel()

	for _, raw := range []string{
		"contact support@example.com",
		"read docs at example.com before download",
	} {
		if _, err := parseDownloadURLs(raw); apperrors.CodeOf(err) != apperrors.CodeDownloadURLInvalid {
			t.Fatalf("parseDownloadURLs(%q) code = %q, err = %v", raw, apperrors.CodeOf(err), err)
		}
	}
}

func TestPrepareYTDLPDownloadUsesAppSessionForNormalizedURL(t *testing.T) {
	t.Parallel()

	service := &LibraryService{
		appSessions: resourceAppSessionReaderStub{
			items: []appsessionsdto.AppSession{
				{
					ID:              "site-app-session-youtube",
					SiteKey:         "youtube",
					Status:          "connected",
					CredentialState: "app_session",
					CookiesCount:    2,
				},
			},
		},
	}

	youtube, err := service.PrepareYTDLPDownload(context.Background(), dtoPrepareRequest("watch?v=AbCdEfGh123"))
	if err != nil {
		t.Fatalf("prepare youtube: %v", err)
	}
	if youtube.URL != "https://www.youtube.com/watch?v=AbCdEfGh123" || youtube.Domain != "youtube.com" {
		t.Fatalf("unexpected youtube normalization: %#v", youtube)
	}
	if youtube.AppSessionID != "site-app-session-youtube" || !youtube.AppSessionAvailable || youtube.AppSessionCredentialMode != "app_session" {
		t.Fatalf("unexpected youtube app session: %#v", youtube)
	}

	chinaPrivate, err := service.PrepareYTDLPDownload(context.Background(), dtoPrepareRequest("douyin.com/video/123"))
	if err != nil {
		t.Fatalf("prepare china private: %v", err)
	}
	if chinaPrivate.URL != "https://douyin.com/video/123" || chinaPrivate.Domain != "douyin.com" {
		t.Fatalf("unexpected china private normalization: %#v", chinaPrivate)
	}
	if chinaPrivate.AppSessionID != "" || chinaPrivate.AppSessionAvailable || chinaPrivate.AppSessionCredentialMode != "" {
		t.Fatalf("unexpected china private app session availability: %#v", chinaPrivate)
	}

	for _, rawURL := range []string{
		"https://www.iesdouyin.com/share/video/123/",
		"https://www.rednote.com/explore/123",
		"https://xhslink.com/a/example",
	} {
		result, err := service.PrepareYTDLPDownload(context.Background(), dtoPrepareRequest(rawURL))
		if err != nil {
			t.Fatalf("prepare china private alias %s: %v", rawURL, err)
		}
		if result.AppSessionID != "" || result.AppSessionAvailable || result.AppSessionCredentialMode != "" {
			t.Fatalf("unexpected china private alias app session for %s: %#v", rawURL, result)
		}
	}
}

func TestPrepareYTDLPDownloadUsesConnectedAppSessionWithoutListedCookies(t *testing.T) {
	t.Parallel()

	service := &LibraryService{
		appSessions: resourceAppSessionReaderStub{
			items: []appsessionsdto.AppSession{
				{
					ID:              "site-app-session-bilibili",
					SiteKey:         "bilibili",
					Status:          "connected",
					CredentialState: "app_session",
					CookiesCount:    0,
				},
			},
		},
	}

	bilibili, err := service.PrepareYTDLPDownload(context.Background(), dtoPrepareRequest("https://www.bilibili.com/video/BV1xx411c7mD/"))
	if err != nil {
		t.Fatalf("prepare bilibili: %v", err)
	}
	if bilibili.AppSessionID != "site-app-session-bilibili" || !bilibili.AppSessionAvailable || bilibili.AppSessionCredentialMode != "app_session" {
		t.Fatalf("unexpected bilibili app session: %#v", bilibili)
	}
}

func TestPrepareYTDLPDownloadReturnsBatchForMultipleURLs(t *testing.T) {
	t.Parallel()

	service := &LibraryService{}
	result, err := service.PrepareYTDLPDownload(context.Background(), dtoPrepareRequest("BVfixture003\nwatch?v=AbCdEfGh123"))
	if err != nil {
		t.Fatalf("prepare batch: %v", err)
	}
	if result.Mode != "batch" {
		t.Fatalf("mode = %q, want batch", result.Mode)
	}
	if len(result.URLs) != 2 {
		t.Fatalf("len(urls) = %d, want 2: %#v", len(result.URLs), result.URLs)
	}
	if result.URL != result.URLs[0].URL {
		t.Fatalf("top-level url = %q, first url = %q", result.URL, result.URLs[0].URL)
	}
}

func TestPrepareYTDLPDownloadMarksDisconnectedAppSessionUnavailable(t *testing.T) {
	t.Parallel()

	service := &LibraryService{
		appSessions: resourceAppSessionReaderStub{
			items: []appsessionsdto.AppSession{
				{
					ID:              "site-app-session-youtube",
					SiteKey:         "youtube",
					Status:          "disconnected",
					CredentialState: "disconnected",
				},
			},
		},
	}

	youtube, err := service.PrepareYTDLPDownload(context.Background(), dtoPrepareRequest("youtube.com/watch?v=AbCdEfGh123"))
	if err != nil {
		t.Fatalf("prepare youtube: %v", err)
	}
	if youtube.AppSessionID != "site-app-session-youtube" || youtube.AppSessionAvailable || youtube.AppSessionCredentialMode != "app_session" {
		t.Fatalf("unexpected youtube app session availability: %#v", youtube)
	}
	if youtube.AppSessionCredentialState != "disconnected" {
		t.Fatalf("expected disconnected credential state, got %#v", youtube)
	}
}

func dtoPrepareRequest(rawURL string) dto.PrepareYTDLPDownloadRequest {
	return dto.PrepareYTDLPDownloadRequest{URL: rawURL}
}
