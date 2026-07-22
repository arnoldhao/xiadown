package service

import (
	"bytes"
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/chromedp/cdproto/network"
	targetpkg "github.com/chromedp/cdproto/target"

	"xiadown/internal/application/apperrors"
	appsessionsdto "xiadown/internal/application/appsessions/dto"
	appsessionsservice "xiadown/internal/application/appsessions/service"
	"xiadown/internal/application/library/dto"
	"xiadown/internal/application/sniffprofile"
	"xiadown/internal/domain/library"
)

type resourceAppSessionReaderStub struct {
	items []appsessionsdto.AppSession
}

type resourceHTTPClientProviderStub struct {
	client *http.Client
}

func (stub resourceAppSessionReaderStub) ListAppSessions(context.Context) ([]appsessionsdto.AppSession, error) {
	return stub.items, nil
}

func (resourceAppSessionReaderStub) ExportAppSessionCookies(context.Context, string, appsessionsservice.CookiesExportFormat) (string, error) {
	return "", nil
}

func (stub resourceHTTPClientProviderStub) HTTPClient() *http.Client {
	return stub.client
}

func TestResourceDownloadURLSupportsChinaPrivateSites(t *testing.T) {
	t.Parallel()

	if !isResourceDownloadURL("https://www.douyin.com/video/123") {
		t.Fatal("expected douyin url to support resource sniff")
	}
	if !isResourceDownloadURL("https://v.douyin.com/abc/") {
		t.Fatal("expected douyin short subdomain to support resource sniff")
	}
	if !isResourceDownloadURL("https://www.xiaohongshu.com/explore/123") {
		t.Fatal("expected xiaohongshu url to support resource sniff")
	}
	for _, rawURL := range []string{
		"https://www.iesdouyin.com/share/video/123/",
		"https://www.rednote.com/explore/123",
		"https://xhs.cn/example",
		"https://xhslink.com/a/example",
		"https://xhslink.cn/a/example",
		"http://xhsurl.com/example",
		"https://rl.ink/example",
		"https://www.kuaishou.com/short-video/example",
		"https://example.com/video/123",
	} {
		if !isResourceDownloadURL(rawURL) {
			t.Fatalf("expected %q to support resource sniff", rawURL)
		}
	}
	for _, rawURL := range []string{
		"https://www.youtube.com/watch?v=abc",
	} {
		if isResourceDownloadURL(rawURL) {
			t.Fatalf("expected %q not to support resource sniff", rawURL)
		}
	}
}

func TestResourceFormatOptionPreservesFormatNote(t *testing.T) {
	option := resourceFormatOptionWithID("original", resourceMedia{
		Ext:        "mp4",
		Kind:       "video",
		FormatNote: "原画",
	})
	if option.FormatNote != "原画" {
		t.Fatalf("expected format note, got %#v", option)
	}
}

func TestResourceConnectorProfilePathUsesSniffProfileForPreferredBrowser(t *testing.T) {
	t.Parallel()

	service := &LibraryService{}
	want, err := sniffprofile.PathForPreferredBrowser("")
	if err != nil {
		t.Fatalf("expected sniff profile path: %v", err)
	}

	got, err := service.resourceConnectorProfilePath(context.Background(), "https://www.douyin.com/video/123")
	if err != nil {
		t.Fatalf("resolve sniff profile path: %v", err)
	}
	if got != want {
		t.Fatalf("expected sniff profile path %q, got %q", want, got)
	}
}

func TestScoreResourceCandidatePrefersDouyinVideo(t *testing.T) {
	t.Parallel()

	videoScore := scoreResourceCandidate(
		"https://v3-dy-o.zjcdn.com/abc/douyinvod/video/tos/cn/tos-cn-ve-15c001.mp4",
		"video/mp4",
		network.Headers{"content-type": "video/mp4", "content-length": "10485760"},
		network.ResourceTypeMedia,
	)
	xiaohongshuScore := scoreResourceCandidate(
		"https://sns-video-bd.xhscdn.com/stream/110/258/abc",
		"video/mp4",
		network.Headers{"content-type": "video/mp4", "content-length": "10485760"},
		network.ResourceTypeMedia,
	)
	imageScore := scoreResourceCandidate(
		"https://www.douyin.com/cover.jpg",
		"image/jpeg",
		network.Headers{"content-type": "image/jpeg", "content-length": "10485760"},
		network.ResourceTypeImage,
	)
	playlistScore := scoreResourceCandidate(
		"https://example.com/video.m3u8",
		"application/vnd.apple.mpegurl",
		network.Headers{"content-type": "application/vnd.apple.mpegurl", "content-length": "10485760"},
		network.ResourceTypeMedia,
	)
	segmentScore := scoreResourceCandidate(
		"https://example.com/live/00001.m4s",
		"video/iso.segment",
		network.Headers{"content-type": "video/iso.segment", "content-length": "10485760"},
		network.ResourceTypeMedia,
	)
	mpegTSSegmentScore := scoreResourceCandidate(
		"https://example.com/live/00001.ts",
		"video/mp2t",
		network.Headers{"content-type": "video/mp2t", "content-length": "10485760"},
		network.ResourceTypeMedia,
	)
	cmafSegmentScore := scoreResourceCandidate(
		"https://example.com/live/chunk-42.cmfv",
		"video/mp4",
		network.Headers{"content-type": "video/mp4", "content-length": "10485760"},
		network.ResourceTypeMedia,
	)
	liveFLVScore := scoreResourceCandidate(
		"https://example.com/live.flv",
		"video/x-flv",
		network.Headers{"content-type": "video/x-flv"},
		network.ResourceTypeMedia,
	)
	finiteFLVScore := scoreResourceCandidate(
		"https://example.com/replay.flv",
		"video/x-flv",
		network.Headers{"content-type": "video/x-flv", "content-length": "10485760"},
		network.ResourceTypeMedia,
	)
	tinyScore := scoreResourceCandidate(
		"https://v3-dy-o.zjcdn.com/abc/douyinvod/video/tos/cn/tos-cn-ve-15c001.mp4",
		"video/mp4",
		network.Headers{"content-type": "video/mp4", "content-length": "37"},
		network.ResourceTypeMedia,
	)
	jsonScore := scoreResourceCandidate(
		"https://vcs.snssdk.com/vc/setting",
		"application/json",
		network.Headers{"content-type": "application/json"},
		network.ResourceTypeXHR,
	)
	captchaScore := scoreResourceCandidate(
		"https://rmc.bytedance.com/verifycenter/captcha/v2",
		"text/html",
		network.Headers{"content-type": "text/html; charset=utf-8"},
		network.ResourceTypeDocument,
	)

	if videoScore <= 0 {
		t.Fatalf("expected video candidate to score positive, got %d", videoScore)
	}
	if xiaohongshuScore <= 0 {
		t.Fatalf("expected xiaohongshu video candidate to score positive, got %d", xiaohongshuScore)
	}
	if imageScore != 0 {
		t.Fatalf("expected image candidate to be rejected, got %d", imageScore)
	}
	if playlistScore >= videoScore {
		t.Fatalf("expected mp4 score %d to beat m3u8 score %d", videoScore, playlistScore)
	}
	if segmentScore != 0 {
		t.Fatalf("expected media segment candidate to be rejected, got %d", segmentScore)
	}
	if mpegTSSegmentScore != 0 {
		t.Fatalf("expected mpeg-ts media segment candidate to be rejected, got %d", mpegTSSegmentScore)
	}
	if cmafSegmentScore != 0 {
		t.Fatalf("expected cmaf media segment candidate to be rejected, got %d", cmafSegmentScore)
	}
	if liveFLVScore != 0 {
		t.Fatalf("expected unbounded flv live stream candidate to be rejected, got %d", liveFLVScore)
	}
	if finiteFLVScore <= 0 {
		t.Fatalf("expected finite flv candidate to score positive, got %d", finiteFLVScore)
	}
	if tinyScore != 0 {
		t.Fatalf("expected tiny video candidate to be rejected, got %d", tinyScore)
	}
	if jsonScore != 0 {
		t.Fatalf("expected snssdk json candidate to be rejected, got %d", jsonScore)
	}
	if captchaScore != 0 {
		t.Fatalf("expected captcha html candidate to be rejected, got %d", captchaScore)
	}
}

func TestResourceResolveErrorCodeUsesGlobalAppCode(t *testing.T) {
	t.Parallel()

	err := apperrors.New(apperrors.CodeResourceVerificationRequired, "blocked")
	if got := resourceResolveErrorCode(err); got != string(apperrors.CodeResourceVerificationRequired) {
		t.Fatalf("resourceResolveErrorCode() = %q, want %q", got, apperrors.CodeResourceVerificationRequired)
	}
}

func TestResourceRejectedCandidateCapturesRequestCookieHeader(t *testing.T) {
	t.Parallel()

	state := newResourceCaptureState()
	requestID := network.RequestID("request-1")
	state.recordRequest(requestID, "https://www.douyin.com/video/123", "https://www.douyin.com/video/123", network.Headers{
		"Cookie":     "sid=1",
		"User-Agent": "TestAgent",
	})
	state.recordResponse(
		requestID,
		"https://www.douyin.com/video/123",
		http.StatusOK,
		"text/html",
		network.Headers{"content-type": "text/html"},
		network.ResourceTypeDocument,
	)
	_, rejected := state.snapshot()

	if len(rejected) != 1 {
		t.Fatalf("expected one rejected candidate, got %d", len(rejected))
	}
	if !resourceHasHeader(rejected[0].headers, "Cookie") {
		t.Fatalf("expected rejected candidate to retain request cookie header, got %#v", rejected[0].headers)
	}
}

func TestNormalizeResourceDownloadHeadersKeepsCookieAndDropsUnsafeHeaders(t *testing.T) {
	t.Parallel()

	got := normalizeResourceDownloadHeaders(map[string]string{
		"Cookie":         "sid=1",
		"Range":          "bytes=0-1024",
		"Content-Length": "1024",
		"Sec-Fetch-Mode": "no-cors",
		"User-Agent":     "TestAgent",
	}, "https://www.douyin.com/video/123")

	if value, ok := findHeader(got, "Cookie"); !ok || value != "sid=1" {
		t.Fatalf("expected cookie header to be preserved, got %#v", got)
	}
	if value, ok := findHeader(got, "User-Agent"); !ok || value != "TestAgent" {
		t.Fatalf("expected user-agent header to be preserved, got %#v", got)
	}
	for _, key := range []string{"Range", "Content-Length", "Sec-Fetch-Mode"} {
		if _, ok := findHeader(got, key); ok {
			t.Fatalf("expected %s to be removed, got %#v", key, got)
		}
	}
}

func TestResourceCaptureRecordsSubtitleResponsesForCurrentPage(t *testing.T) {
	t.Parallel()

	state := newResourceCaptureState()
	requestID := network.RequestID("subtitle-1")
	state.recordRequest(
		requestID,
		"https://cdn.example.com/captions.en.vtt",
		"https://www.douyin.com/video/123",
		network.Headers{"Cookie": "sid=1", "User-Agent": "TestAgent"},
	)
	state.recordResponse(
		requestID,
		"https://cdn.example.com/captions.en.vtt",
		http.StatusOK,
		"text/vtt",
		network.Headers{"content-type": "text/vtt", "content-length": "128"},
		network.ResourceTypeXHR,
	)

	subtitles := state.subtitlesSnapshot()
	options := resourceSubtitleOptions(subtitles)
	if len(options) != 1 {
		t.Fatalf("expected one subtitle option, got %d: %#v", len(options), options)
	}
	if options[0].Language != "en" || options[0].Ext != "vtt" || options[0].IsAuto {
		t.Fatalf("unexpected subtitle option: %#v", options[0])
	}
	if value, ok := findHeader(subtitles[0].RequestHeaders, "Cookie"); !ok || value != "sid=1" {
		t.Fatalf("expected subtitle request headers to retain cookie, got %#v", subtitles[0].RequestHeaders)
	}
}

func TestResourceNormalizeSubtitleLanguageUsesCanonicalHLSTagCasing(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name  string
		input string
		want  string
	}{
		{name: "script", input: "zh-hans", want: "zh-Hans"},
		{name: "script_region", input: "ZH_hant_tw", want: "zh-Hant-TW"},
		{name: "region", input: "pt-br", want: "pt-BR"},
		{name: "numeric_region", input: "es-419", want: "es-419"},
		{name: "reserved_word", input: "captions", want: ""},
		{name: "invalid_tag", input: "track/file", want: ""},
	}
	for _, test := range tests {
		test := test
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			if got := resourceNormalizeSubtitleLanguage(test.input); got != test.want {
				t.Fatalf("resourceNormalizeSubtitleLanguage(%q) = %q, want %q", test.input, got, test.want)
			}
		})
	}
}

func TestSelectResourceSubtitlesMatchesLanguageFormatAndAutoFlag(t *testing.T) {
	t.Parallel()

	subtitles := []resourceSubtitle{
		{URL: "https://cdn.example.com/en.vtt", Language: "en", Ext: "vtt"},
		{URL: "https://cdn.example.com/en.srt", Language: "en", Ext: "srt"},
		{URL: "https://cdn.example.com/en-auto.vtt", Language: "en", Ext: "vtt", IsAuto: true},
	}
	selected := selectResourceSubtitlesForRequest(dto.CreateYTDLPJobRequest{
		SubtitleLangs:  []string{"en"},
		SubtitleFormat: "srt/vtt",
	}, subtitles)
	if len(selected) != 1 || selected[0].URL != "https://cdn.example.com/en.srt" {
		t.Fatalf("expected manual srt subtitle, got %#v", selected)
	}

	selectedAuto := selectResourceSubtitlesForRequest(dto.CreateYTDLPJobRequest{
		SubtitleLangs: []string{"en"},
		SubtitleAuto:  true,
	}, subtitles)
	if len(selectedAuto) != 1 || selectedAuto[0].URL != "https://cdn.example.com/en-auto.vtt" {
		t.Fatalf("expected auto subtitle, got %#v", selectedAuto)
	}

	scriptSubtitles := []resourceSubtitle{
		{URL: "https://cdn.example.com/zh-Hans.vtt", Language: "zh-Hans", Ext: "vtt"},
	}
	selectedScript := selectResourceSubtitlesForRequest(dto.CreateYTDLPJobRequest{
		SubtitleLangs: []string{"zh-hans"},
	}, scriptSubtitles)
	if len(selectedScript) != 1 || selectedScript[0].URL != "https://cdn.example.com/zh-Hans.vtt" {
		t.Fatalf("expected canonical script language subtitle match, got %#v", selectedScript)
	}
}

func TestWriteResourceSubtitleDownloadsWithCapturedHeaders(t *testing.T) {
	t.Parallel()

	var captured http.Header
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		captured = r.Header.Clone()
		w.Header().Set("Content-Type", "text/vtt")
		_, _ = w.Write([]byte("WEBVTT\n\n00:00.000 --> 00:01.000\nHello\n"))
	}))
	defer server.Close()

	path, err := (&LibraryService{}).writeResourceSubtitle(
		context.Background(),
		resourceSubtitle{
			URL:      server.URL + "/captions.en.vtt",
			Language: "en",
			Ext:      "vtt",
			RequestHeaders: map[string]string{
				"Cookie":     "sid=1",
				"User-Agent": "TestAgent",
			},
		},
		filepath.Join(t.TempDir(), "video.en.vtt"),
		"",
	)
	if err != nil {
		t.Fatalf("writeResourceSubtitle() error = %v", err)
	}
	content, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read subtitle: %v", err)
	}
	if !strings.Contains(string(content), "WEBVTT") {
		t.Fatalf("expected VTT content, got %q", string(content))
	}
	if captured.Get("Cookie") != "sid=1" || captured.Get("User-Agent") != "TestAgent" {
		t.Fatalf("expected captured headers to be reused, got %#v", captured)
	}
}

func TestResourceMediaSnapshotConsumesWithoutSession(t *testing.T) {
	t.Parallel()

	service := &LibraryService{}
	mediaID := service.putResourceMediaSnapshot(resourceMedia{
		URL:   "https://v3-dy-o.zjcdn.com/abc/video.mp4",
		Title: "Queued Video",
	})

	if mediaID == "" {
		t.Fatal("expected media snapshot id")
	}
	media, ok := service.consumeResourceMediaSnapshot(mediaID)
	if !ok {
		t.Fatal("expected media snapshot")
	}
	if media.Title != "Queued Video" {
		t.Fatalf("expected snapshot title, got %q", media.Title)
	}
	if _, ok := service.consumeResourceMediaSnapshot(mediaID); ok {
		t.Fatal("expected media snapshot to be one-shot")
	}
}

func TestClaimResourceMediaForQueuedOperationSurvivesSessionSnapshotCleanup(t *testing.T) {
	t.Parallel()

	service := &LibraryService{}
	originalID := service.putResourceMediaSnapshot(resourceMedia{
		URL:   "https://v3-dy-o.zjcdn.com/abc/video.mp4",
		Title: "Queued Video",
		RequestHeaders: map[string]string{
			"Referer": "https://www.douyin.com/",
		},
		Subtitles: []resourceSubtitle{
			{
				URL:      "https://example.com/caption.vtt",
				Language: "zh-CN",
				Ext:      "vtt",
				RequestHeaders: map[string]string{
					"Referer": "https://www.douyin.com/",
				},
			},
		},
	})
	service.resourceSniffs = map[string]*resourceSniffSession{
		"session-1": {
			ID:           "session-1",
			LastMediaIDs: []string{originalID},
		},
	}

	claimedRequest, claimedID, err := service.claimResourceMediaForQueuedOperation(dto.CreateYTDLPJobRequest{
		URL:               "https://www.douyin.com/video/123",
		ResourceSessionID: "session-1",
		FormatID:          originalID,
	})
	if err != nil {
		t.Fatalf("claim resource media: %v", err)
	}
	if claimedID == "" {
		t.Fatal("expected claimed media id")
	}
	if claimedID == originalID {
		t.Fatal("expected claim to allocate an operation-owned media id")
	}
	if claimedRequest.ResourceSessionID != "session-1" {
		t.Fatalf("expected claimed request to keep session cleanup id, got %q", claimedRequest.ResourceSessionID)
	}
	if claimedRequest.ResourceMediaID != claimedID || claimedRequest.FormatID != claimedID {
		t.Fatalf("expected request to point at claimed media id, got media=%q format=%q claim=%q", claimedRequest.ResourceMediaID, claimedRequest.FormatID, claimedID)
	}

	service.discardResourceMediaSnapshots(originalID)
	if _, ok := service.consumeResourceMediaSnapshot(originalID); ok {
		t.Fatal("expected original session media snapshot to be gone")
	}
	media, ok := service.consumeResourceSniffMedia("session-1", claimedID)
	if !ok {
		t.Fatal("expected claimed operation media snapshot to survive session snapshot cleanup")
	}
	if media.Title != "Queued Video" {
		t.Fatalf("expected claimed media title, got %q", media.Title)
	}
	if media.RequestHeaders["Referer"] != "https://www.douyin.com/" {
		t.Fatalf("expected claimed request headers to be preserved, got %#v", media.RequestHeaders)
	}
	if len(media.Subtitles) != 1 || media.Subtitles[0].RequestHeaders["Referer"] != "https://www.douyin.com/" {
		t.Fatalf("expected claimed subtitle headers to be preserved, got %#v", media.Subtitles)
	}
}

func TestConsumeResourceSniffMediaFallsBackToClaimedSnapshotWhenSessionGone(t *testing.T) {
	t.Parallel()

	service := &LibraryService{}
	claimedID := service.putResourceMediaSnapshot(resourceMedia{
		URL:   "https://v3-dy-o.zjcdn.com/abc/video.mp4",
		Title: "Claimed Video",
	})

	media, ok := service.consumeResourceSniffMedia("already-closed-session", claimedID)
	if !ok {
		t.Fatal("expected claimed media to be available after session cleanup")
	}
	if media.Title != "Claimed Video" {
		t.Fatalf("expected claimed media title, got %q", media.Title)
	}
	if _, ok := service.consumeResourceMediaSnapshot(claimedID); ok {
		t.Fatal("expected claimed media to remain one-shot")
	}
}

func TestResourceMediaSnapshotDisablesAutoRetry(t *testing.T) {
	t.Parallel()

	if shouldAutoRetryYTDLP(dto.CreateYTDLPJobRequest{Mode: "quick", ResourceMediaID: "media-1"}, "timeout") {
		t.Fatal("expected resource media snapshot downloads to skip auto retry")
	}
}

func TestPickResourceSniffActiveTabPrefersVisiblePage(t *testing.T) {
	t.Parallel()

	now := time.Now()
	service := &LibraryService{
		resourceSniffs: map[string]*resourceSniffSession{
			"session-1": {
				ID:       "session-1",
				ActiveID: "old-tab",
				Tabs: map[string]*resourceSniffTab{
					"old-tab": {
						TargetID:   "old-tab",
						CurrentURL: "https://www.douyin.com/video/old",
						Visibility: "hidden",
						LastSeen:   now,
					},
					"visible-tab": {
						TargetID:   "visible-tab",
						CurrentURL: "https://www.douyin.com/video/current",
						Visibility: "visible",
						LastSeen:   now.Add(-1 * time.Hour),
					},
				},
			},
		},
	}

	tab := service.pickResourceSniffActiveTab("session-1")
	if tab == nil || tab.TargetID != "visible-tab" {
		t.Fatalf("expected visible tab to be selected, got %#v", tab)
	}
}

func TestResourceSniffIgnoredTargetURLSkipsTransientBrowserPages(t *testing.T) {
	t.Parallel()

	for _, rawURL := range []string{"", "about:blank", "chrome://newtab/", "devtools://devtools/bundled/inspector.html"} {
		if !resourceSniffIgnoredTargetURL(rawURL) {
			t.Fatalf("expected %q to be ignored", rawURL)
		}
	}
	if resourceSniffIgnoredTargetURL("https://www.douyin.com/video/123") {
		t.Fatal("expected douyin page to be tracked")
	}
}

func TestResourceSniffTrackPendingTargetURLOnlyTracksBlankNavigationPages(t *testing.T) {
	t.Parallel()

	for _, rawURL := range []string{"", "about:blank", "chrome://newtab/", "edge://newtab/"} {
		if !resourceSniffTrackPendingTargetURL(rawURL) {
			t.Fatalf("expected %q to be tracked as pending navigation target", rawURL)
		}
	}
	for _, rawURL := range []string{"chrome://settings/", "devtools://devtools/bundled/inspector.html", "https://www.douyin.com/video/123"} {
		if resourceSniffTrackPendingTargetURL(rawURL) {
			t.Fatalf("expected %q not to be tracked as pending navigation target", rawURL)
		}
	}
}

func TestPickResourceSniffActiveTabSkipsPendingBlankTarget(t *testing.T) {
	t.Parallel()

	service := &LibraryService{}
	session := &resourceSniffSession{
		ActiveID: "blank",
		Tabs: map[string]*resourceSniffTab{
			"blank": {
				TargetID:          "blank",
				CurrentURL:        "about:blank",
				PendingNavigation: true,
				LastSeen:          time.Now(),
			},
			"page": {
				TargetID:   "page",
				CurrentURL: "https://www.douyin.com/video/123",
				LastSeen:   time.Now().Add(-time.Second),
			},
		},
	}

	got := service.pickResourceSniffActiveTabLocked(session)
	if got == nil || got.TargetID != "page" {
		t.Fatalf("expected real page tab to be active, got %#v", got)
	}
}

func TestHandleResourceSniffTargetInfoDropsPendingInternalTarget(t *testing.T) {
	t.Parallel()

	canceled := false
	service := &LibraryService{
		resourceSniffs: map[string]*resourceSniffSession{
			"session-1": {
				ID: "session-1",
				Tabs: map[string]*resourceSniffTab{
					"blank": {
						TargetID:          "blank",
						CurrentURL:        "about:blank",
						PendingNavigation: true,
						Cancel: func() {
							canceled = true
						},
					},
				},
			},
		},
	}

	service.handleResourceSniffTargetInfo("session-1", &targetpkg.Info{
		TargetID: "blank",
		Type:     "page",
		URL:      "chrome://settings/",
	})

	if _, ok := service.resourceSniffs["session-1"].Tabs["blank"]; ok {
		t.Fatal("expected pending internal target to be removed")
	}
	if !canceled {
		t.Fatal("expected removed tab context to be canceled")
	}
}

func TestRemoveMissingResourceSniffTabsKeepsSeenTargets(t *testing.T) {
	t.Parallel()

	canceled := false
	service := &LibraryService{
		resourceSniffs: map[string]*resourceSniffSession{
			"session-1": {
				ID:       "session-1",
				ActiveID: "tab-1",
				Tabs: map[string]*resourceSniffTab{
					"tab-1": {
						TargetID:   "tab-1",
						CurrentURL: "chrome://newtab/",
						Cancel: func() {
							canceled = true
						},
					},
				},
			},
		},
	}

	service.removeMissingResourceSniffTabs("session-1", map[string]struct{}{"tab-1": {}})

	if canceled {
		t.Fatal("expected seen target to remain open")
	}
	if _, ok := service.resourceSniffs["session-1"].Tabs["tab-1"]; !ok {
		t.Fatal("expected seen target to remain tracked")
	}
}

func TestRemoveResourceSniffTabBySessionIDRemovesMatchingTab(t *testing.T) {
	t.Parallel()

	canceled := false
	service := &LibraryService{
		resourceSniffs: map[string]*resourceSniffSession{
			"session-1": {
				ID:       "session-1",
				ActiveID: "tab-1",
				Tabs: map[string]*resourceSniffTab{
					"tab-1": {
						TargetID:        "tab-1",
						TargetSessionID: "cdp-session-1",
						CurrentURL:      "https://www.douyin.com/video/123",
						Cancel: func() {
							canceled = true
						},
					},
				},
			},
		},
	}

	service.removeResourceSniffTabBySessionID("session-1", "cdp-session-1", "target_detached")

	if !canceled {
		t.Fatal("expected matching detached target context to be canceled")
	}
	if _, ok := service.resourceSniffs["session-1"].Tabs["tab-1"]; ok {
		t.Fatal("expected matching detached target to be removed")
	}
	if service.resourceSniffs["session-1"].ActiveID != "" {
		t.Fatalf("expected active target to be cleared, got %q", service.resourceSniffs["session-1"].ActiveID)
	}
}

func TestResourceSniffSessionStatusReportsTabClosedWhenNoUsableTabs(t *testing.T) {
	t.Parallel()

	state, browserStatus := resourceSniffSessionStateAndBrowserStatus(resourceSniffStateRunning, true, 0)

	if state != resourceSniffStateRunning {
		t.Fatalf("expected running state, got %q", state)
	}
	if browserStatus != resourceSniffBrowserStatusTabClosed {
		t.Fatalf("expected tab closed status, got %q", browserStatus)
	}
}

func TestResourceSniffSessionStatusReportsClosingDuringGracefulExit(t *testing.T) {
	t.Parallel()

	state, browserStatus := resourceSniffSessionStateAndBrowserStatus(resourceSniffStateClosing, false, 0)

	if state != resourceSniffStateClosing {
		t.Fatalf("expected closing state, got %q", state)
	}
	if browserStatus != resourceSniffBrowserStatusClosing {
		t.Fatalf("expected closing status, got %q", browserStatus)
	}
}

func TestApplyResourceMediaToOperationCarriesMetadata(t *testing.T) {
	t.Parallel()

	operation := library.LibraryOperation{}
	history := library.HistoryRecord{}
	request := dto.CreateYTDLPJobRequest{URL: "https://media.example/video/123"}
	updated := (&LibraryService{}).applyResourceMediaToOperation(
		context.Background(),
		&operation,
		&history,
		request,
		resourceMedia{
			PageURL:      "https://media.example/video/456",
			Title:        "Resolved Title",
			Author:       "Creator",
			ThumbnailURL: "https://example.com/cover.jpg",
			Extractor:    "sniff",
			Domain:       "media.example",
		},
	)

	if updated.URL != "https://media.example/video/456" {
		t.Fatalf("expected request url to switch to active page url, got %q", updated.URL)
	}
	if updated.Title != "Resolved Title" || operation.DisplayName != "Resolved Title" || history.DisplayName != "Resolved Title" {
		t.Fatalf("expected title to populate request and operation, got request=%q operation=%q history=%q", updated.Title, operation.DisplayName, history.DisplayName)
	}
	if updated.Author != "Creator" || operation.Meta.Uploader != "Creator" {
		t.Fatalf("expected author metadata, got request=%q operation=%q", updated.Author, operation.Meta.Uploader)
	}
	if updated.ThumbnailURL != "https://example.com/cover.jpg" {
		t.Fatalf("expected thumbnail url, got %q", updated.ThumbnailURL)
	}
	if operation.Meta.Platform != "sniff" || operation.SourceDomain != "media.example" {
		t.Fatalf("expected resource platform/domain, got platform=%q domain=%q", operation.Meta.Platform, operation.SourceDomain)
	}
}

func TestResourceOutputBaseNameUsesGenericFallback(t *testing.T) {
	t.Parallel()

	if got := resourceOutputBaseName(resourceMedia{}, "1234567890"); got != "resource-12345678" {
		t.Fatalf("expected generic resource fallback, got %q", got)
	}
}

func TestResourceFormatOptionUsesResolutionLabel(t *testing.T) {
	t.Parallel()

	option := resourceFormatOption(resourceMedia{
		Ext:       ".mp4",
		Height:    1080,
		SizeBytes: 10485760,
	})

	if option.Height != 1080 {
		t.Fatalf("expected height 1080, got %d", option.Height)
	}
	if option.Label != "1080p · mp4 · 10MB" {
		t.Fatalf("expected resolution label, got %q", option.Label)
	}
}

func TestResourceFormatOptionUsesPortraitResolutionLabelWithoutQuality(t *testing.T) {
	t.Parallel()

	option := resourceFormatOption(resourceMedia{
		Ext:    ".mp4",
		Width:  720,
		Height: 2160,
	})

	if option.Height != 0 {
		t.Fatalf("expected no quality height for portrait without quality metadata, got %d", option.Height)
	}
	if option.Label != "720x2160 · mp4" {
		t.Fatalf("expected portrait resolution label, got %q", option.Label)
	}
}

func TestResourceFormatOptionUsesUnknownLabelWithoutResolution(t *testing.T) {
	t.Parallel()

	option := resourceFormatOption(resourceMedia{
		Ext: ".mp4",
	})

	if option.Height != 0 {
		t.Fatalf("expected no quality height without resolution metadata, got %d", option.Height)
	}
	if option.Label != "Unknown · mp4" {
		t.Fatalf("expected unknown resolution label, got %q", option.Label)
	}
}

func TestResourceFormatOptionUsesResourceKindTrackFlags(t *testing.T) {
	t.Parallel()

	image := resourceFormatOption(resourceMedia{
		Kind:        "image",
		Ext:         ".webp",
		ContentType: "image/webp",
	})
	if image.HasVideo || image.HasAudio {
		t.Fatalf("expected image format to have no media tracks, got video=%v audio=%v", image.HasVideo, image.HasAudio)
	}

	subtitle := resourceFormatOption(resourceMedia{
		Kind:        "subtitle",
		Ext:         ".vtt",
		ContentType: "text/vtt",
	})
	if subtitle.HasVideo || subtitle.HasAudio {
		t.Fatalf("expected subtitle format to have no media tracks, got video=%v audio=%v", subtitle.HasVideo, subtitle.HasAudio)
	}

	misclassifiedVideo := resourceFormatOption(resourceMedia{
		Kind:        "subtitle",
		Ext:         ".mp4",
		ContentType: "video/mp4",
	})
	if !misclassifiedVideo.HasVideo || !misclassifiedVideo.HasAudio {
		t.Fatalf("expected declared video content to expose media tracks, got video=%v audio=%v", misclassifiedVideo.HasVideo, misclassifiedVideo.HasAudio)
	}

	audio := resourceFormatOption(resourceMedia{
		Kind:        "audio",
		Ext:         ".mp3",
		ContentType: "audio/mpeg",
	})
	if audio.HasVideo || !audio.HasAudio {
		t.Fatalf("expected audio format to expose audio-only tracks, got video=%v audio=%v", audio.HasVideo, audio.HasAudio)
	}

	unknown := resourceFormatOption(resourceMedia{
		Kind: "other",
	})
	if unknown.HasVideo || unknown.HasAudio || unknown.Ext != "bin" {
		t.Fatalf("expected unknown format to expose no tracks and bin extension, got video=%v audio=%v ext=%q", unknown.HasVideo, unknown.HasAudio, unknown.Ext)
	}

	unsupported := resourceFormatOption(resourceMedia{
		Kind: "unsupported",
	})
	if unsupported.HasVideo || unsupported.HasAudio || unsupported.Ext != "bin" {
		t.Fatalf("expected unsupported kind to behave as other, got video=%v audio=%v ext=%q", unsupported.HasVideo, unsupported.HasAudio, unsupported.Ext)
	}
}

func TestResourceMediaLibraryFileKindUsesRawKind(t *testing.T) {
	t.Parallel()

	if got := resourceMediaLibraryFileKind(resourceMedia{Kind: "image", Ext: ".png"}); got != string(library.FileKindThumbnail) {
		t.Fatalf("expected image resource to use image-compatible file kind, got %q", got)
	}
	if got := resourceMediaLibraryFileKind(resourceMedia{Kind: "subtitle", Ext: ".vtt"}); got != string(library.FileKindSubtitle) {
		t.Fatalf("expected subtitle resource to use subtitle file kind, got %q", got)
	}
	if got := resourceMediaLibraryFileKind(resourceMedia{Kind: "subtitle", Ext: ".mp4", ContentType: "video/mp4"}); got != string(library.FileKindVideo) {
		t.Fatalf("expected declared video content to override subtitle kind, got %q", got)
	}
	if got := resourceMediaLibraryFileKind(resourceMedia{Kind: "video", Ext: ".itt"}); got != string(library.FileKindSubtitle) {
		t.Fatalf("expected subtitle extension to use subtitle file kind, got %q", got)
	}
	if got := resourceMediaLibraryFileKind(resourceMedia{Kind: "video", Ext: ".sbv"}); got != string(library.FileKindSubtitle) {
		t.Fatalf("expected sbv subtitle extension to use subtitle file kind, got %q", got)
	}
	if got := resourceMediaLibraryFileKind(resourceMedia{Kind: "audio", Ext: ".mp3"}); got != string(library.FileKindAudio) {
		t.Fatalf("expected audio resource to use audio file kind, got %q", got)
	}
	if got := resourceMediaLibraryFileKind(resourceMedia{Kind: "live", Ext: ".m3u8"}); got != string(library.FileKindManifest) {
		t.Fatalf("expected live manifest resource to use manifest file kind, got %q", got)
	}
	if got := resourceMediaLibraryFileKind(resourceMedia{Kind: "document", Ext: ".pdf"}); got != string(library.FileKindDocument) {
		t.Fatalf("expected document resource to use document file kind, got %q", got)
	}
	if got := resourceMediaLibraryFileKind(resourceMedia{Kind: "font", Ext: ".woff2"}); got != string(library.FileKindFont) {
		t.Fatalf("expected font resource to use font file kind, got %q", got)
	}
	if got := resourceMediaLibraryFileKind(resourceMedia{Kind: "api", Ext: ".json"}); got != string(library.FileKindAPI) {
		t.Fatalf("expected api resource to use api file kind, got %q", got)
	}
	if got := resourceMediaLibraryFileKind(resourceMedia{Kind: "archive", Ext: ".zip"}); got != string(library.FileKindArchive) {
		t.Fatalf("expected archive resource to use archive file kind, got %q", got)
	}
	if got := resourceMediaLibraryFileKind(resourceMedia{Kind: "manifest", Ext: ".mpd"}); got != string(library.FileKindManifest) {
		t.Fatalf("expected manifest resource to use manifest file kind, got %q", got)
	}
	if got := resourceMediaLibraryFileKind(resourceMedia{Kind: "other"}); got != string(library.FileKindOther) {
		t.Fatalf("expected unknown resource to use other file kind, got %q", got)
	}
	if got := resourceMediaLibraryFileKind(resourceMedia{Kind: "unsupported"}); got != string(library.FileKindOther) {
		t.Fatalf("expected unsupported resource kind to use other file kind, got %q", got)
	}
	if got := resourceMediaLibraryFileKind(resourceMedia{}); got != string(library.FileKindOther) {
		t.Fatalf("expected empty resource metadata to use other file kind, got %q", got)
	}
}

func TestNormalizeResourceMediaWithDownloadedFileCorrectsMP4(t *testing.T) {
	t.Parallel()

	outputPath := filepath.Join(t.TempDir(), "download.vtt")
	mp4Header := []byte{
		0x00, 0x00, 0x00, 0x18,
		'f', 't', 'y', 'p',
		'i', 's', 'o', 'm',
		0x00, 0x00, 0x00, 0x00,
		'i', 's', 'o', 'm',
		'm', 'p', '4', '1',
	}
	if err := os.WriteFile(outputPath, mp4Header, 0o644); err != nil {
		t.Fatalf("write mp4 fixture: %v", err)
	}

	media := normalizeResourceMediaWithDownloadedFile(resourceMedia{
		Kind:        "subtitle",
		Ext:         ".vtt",
		ContentType: "text/vtt",
	}, outputPath)

	if media.Kind != "video" || media.Ext != ".mp4" || media.ContentType != "video/mp4" {
		t.Fatalf("expected downloaded mp4 to correct media type, got %#v", media)
	}
}

func TestNormalizeResourceMediaWithDownloadedFileCorrectsMisclassifiedJPG(t *testing.T) {
	t.Parallel()

	outputPath := filepath.Join(t.TempDir(), "download.jpg")
	jpegHeader := []byte{
		0xff, 0xd8, 0xff, 0xe0,
		0x00, 0x10,
		'J', 'F', 'I', 'F', 0x00,
		0x01, 0x01, 0x00,
		0x00, 0x01, 0x00, 0x01,
		0x00, 0x00,
	}
	if err := os.WriteFile(outputPath, jpegHeader, 0o644); err != nil {
		t.Fatalf("write jpeg fixture: %v", err)
	}

	media := normalizeResourceMediaWithDownloadedFile(resourceMedia{
		Kind: "subtitle",
		Ext:  ".jpg",
	}, outputPath)

	if media.Kind != "image" || media.ContentType != "image/jpeg" {
		t.Fatalf("expected downloaded jpg to correct media type, got %#v", media)
	}
}

func TestNormalizeResourceMediaWithDownloadedFileKeepsUnknownAsOther(t *testing.T) {
	t.Parallel()

	outputPath := filepath.Join(t.TempDir(), "download.bin")
	if err := os.WriteFile(outputPath, []byte{0x00, 0x01, 0x02, 0x03}, 0o644); err != nil {
		t.Fatalf("write binary fixture: %v", err)
	}

	media := normalizeResourceMediaWithDownloadedFile(resourceMedia{}, outputPath)

	if media.Kind != "other" {
		t.Fatalf("expected unknown download to remain other, got %#v", media)
	}
}

func TestResourceFormatOptionIncludesVideoCodecInLabel(t *testing.T) {
	t.Parallel()

	option := resourceFormatOption(resourceMedia{
		Ext:    ".mp4",
		VCodec: "h264",
	})

	if option.Label != "Unknown · mp4 · H.264" {
		t.Fatalf("expected video codec in label, got %q", option.Label)
	}
}

func TestDownloadResourceFileReusesSafeCapturedHeaders(t *testing.T) {
	t.Parallel()

	var captured http.Header
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		captured = r.Header.Clone()
		w.Header().Set("Content-Type", "video/mp4")
		w.Header().Set("Content-Length", "5")
		_, _ = w.Write([]byte("video"))
	}))
	defer server.Close()

	outputPath := filepath.Join(t.TempDir(), "download.mp4")
	result, err := (&LibraryService{}).downloadResourceFile(context.Background(), resourceDownloadOptions{
		URL:        server.URL,
		OutputPath: outputPath,
		Headers: map[string]string{
			"Cookie":         "sid=1",
			"Range":          "bytes=0-1024",
			"Content-Length": "1024",
			"User-Agent":     "TestAgent",
		},
		TotalSize: 5,
	})
	if err != nil {
		t.Fatalf("downloadResourceFile() error = %v", err)
	}
	if result.Path != outputPath {
		t.Fatalf("downloadResourceFile() path = %q, want %q", result.Path, outputPath)
	}
	if result.SizeBytes != 5 {
		t.Fatalf("downloadResourceFile() size = %d, want 5", result.SizeBytes)
	}
	content, err := os.ReadFile(outputPath)
	if err != nil {
		t.Fatalf("read output: %v", err)
	}
	if string(content) != "video" {
		t.Fatalf("output content = %q, want video", string(content))
	}
	if captured.Get("Cookie") != "sid=1" {
		t.Fatalf("expected cookie header, got %#v", captured)
	}
	if captured.Get("User-Agent") != "TestAgent" {
		t.Fatalf("expected user-agent header, got %#v", captured)
	}
	if captured.Get("Range") != "" || captured.Get("Content-Length") != "" {
		t.Fatalf("expected unsafe headers to be dropped, got %#v", captured)
	}
}

func TestDownloadResourceFileUsesMultipartWhenRangeSupported(t *testing.T) {
	t.Parallel()

	data := make([]byte, int(resourceDownloadMinPartSize*3+12345))
	for index := range data {
		data[index] = byte(index % 251)
	}
	var mu sync.Mutex
	rangeRequests := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "video/mp4")
		w.Header().Set("Accept-Ranges", "bytes")
		w.Header().Set("Content-Length", strconv.Itoa(len(data)))
		switch r.Method {
		case http.MethodHead:
			return
		case http.MethodGet:
			rangeHeader := r.Header.Get("Range")
			start, end, ok := parseTestResourceRange(rangeHeader, len(data))
			if !ok {
				http.Error(w, "range required", http.StatusRequestedRangeNotSatisfiable)
				return
			}
			mu.Lock()
			rangeRequests++
			mu.Unlock()
			w.Header().Set("Content-Range", fmt.Sprintf("bytes %d-%d/%d", start, end, len(data)))
			w.Header().Set("Content-Length", strconv.Itoa(end-start+1))
			w.WriteHeader(http.StatusPartialContent)
			_, _ = w.Write(data[start : end+1])
		default:
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		}
	}))
	defer server.Close()

	outputPath := filepath.Join(t.TempDir(), "multipart.mp4")
	result, err := (&LibraryService{}).downloadResourceFile(context.Background(), resourceDownloadOptions{
		URL:        server.URL,
		OutputPath: outputPath,
		TotalSize:  int64(len(data)),
	})
	if err != nil {
		t.Fatalf("downloadResourceFile() error = %v", err)
	}
	if result.SizeBytes != int64(len(data)) {
		t.Fatalf("downloadResourceFile() size = %d, want %d", result.SizeBytes, len(data))
	}
	content, err := os.ReadFile(outputPath)
	if err != nil {
		t.Fatalf("read output: %v", err)
	}
	if !bytes.Equal(content, data) {
		t.Fatal("expected multipart output to match source data")
	}
	mu.Lock()
	gotRangeRequests := rangeRequests
	mu.Unlock()
	if gotRangeRequests == 0 {
		t.Fatal("expected range requests")
	}
}

func TestDownloadResourceFileFallsBackToSingleWhenRangeFails(t *testing.T) {
	t.Parallel()

	data := make([]byte, int(resourceDownloadMinPartSize*2+789))
	for index := range data {
		data[index] = byte((index * 7) % 251)
	}
	var mu sync.Mutex
	sawRange := false
	sawSingle := false
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "video/mp4")
		w.Header().Set("Accept-Ranges", "bytes")
		w.Header().Set("Content-Length", strconv.Itoa(len(data)))
		switch r.Method {
		case http.MethodHead:
			return
		case http.MethodGet:
			if r.Header.Get("Range") != "" {
				mu.Lock()
				sawRange = true
				mu.Unlock()
				http.Error(w, "range disabled", http.StatusRequestedRangeNotSatisfiable)
				return
			}
			mu.Lock()
			sawSingle = true
			mu.Unlock()
			_, _ = w.Write(data)
		default:
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		}
	}))
	defer server.Close()

	outputPath := filepath.Join(t.TempDir(), "fallback.mp4")
	result, err := (&LibraryService{}).downloadResourceFile(context.Background(), resourceDownloadOptions{
		URL:        server.URL,
		OutputPath: outputPath,
		TotalSize:  int64(len(data)),
	})
	if err != nil {
		t.Fatalf("downloadResourceFile() error = %v", err)
	}
	if result.SizeBytes != int64(len(data)) {
		t.Fatalf("downloadResourceFile() size = %d, want %d", result.SizeBytes, len(data))
	}
	content, err := os.ReadFile(outputPath)
	if err != nil {
		t.Fatalf("read output: %v", err)
	}
	if !bytes.Equal(content, data) {
		t.Fatal("expected fallback output to match source data")
	}
	mu.Lock()
	defer mu.Unlock()
	if !sawRange || !sawSingle {
		t.Fatalf("expected range attempt and single fallback, sawRange=%v sawSingle=%v", sawRange, sawSingle)
	}
}

func parseTestResourceRange(value string, size int) (int, int, bool) {
	trimmed := strings.TrimSpace(value)
	if !strings.HasPrefix(trimmed, "bytes=") || size <= 0 {
		return 0, 0, false
	}
	parts := strings.SplitN(strings.TrimPrefix(trimmed, "bytes="), "-", 2)
	if len(parts) != 2 {
		return 0, 0, false
	}
	start, err := strconv.Atoi(parts[0])
	if err != nil || start < 0 || start >= size {
		return 0, 0, false
	}
	end := size - 1
	if strings.TrimSpace(parts[1]) != "" {
		parsedEnd, err := strconv.Atoi(parts[1])
		if err != nil || parsedEnd < start {
			return 0, 0, false
		}
		end = parsedEnd
	}
	if end >= size {
		end = size - 1
	}
	return start, end, true
}
