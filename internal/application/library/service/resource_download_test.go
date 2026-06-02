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
	appcookies "xiadown/internal/application/cookies"
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

func TestResourceLooksBlockedDetectsDouyinCaptcha(t *testing.T) {
	t.Parallel()

	if !resourceDouyinLooksBlocked(map[string]string{"title": "验证码中间页"}, nil) {
		t.Fatal("expected captcha title to be treated as blocked")
	}
	if resourceDouyinLooksBlocked(nil, []resourceRejectedCandidate{{
		url:          "https://rmc.bytedance.com/verifycenter/captcha/v2",
		mimeType:     "text/html",
		resourceType: "Document",
	}}) {
		t.Fatal("expected rejected verifycenter document not to be treated as blocked without page-level signal")
	}
	if resourceDouyinLooksBlocked(nil, []resourceRejectedCandidate{{
		url:          "https://lf-rc1.yhgfb-cn-static.com/obj/rc-verifycenter/sec_sdk_build/4.0.22/captcha/index.js",
		mimeType:     "application/javascript",
		resourceType: "Script",
		reason:       "too_small",
	}}) {
		t.Fatal("expected static captcha assets not to be treated as page verification")
	}
	if resourceDouyinLooksBlocked(map[string]string{"loginHint": "login_or_verify_text_detected"}, nil) {
		t.Fatal("expected generic page login text not to be treated as blocked")
	}
}

func TestResourceResolveErrorCodeUsesGlobalAppCode(t *testing.T) {
	t.Parallel()

	err := apperrors.New(apperrors.CodeResourceVerificationRequired, "blocked")
	if got := resourceResolveErrorCode(err); got != string(apperrors.CodeResourceVerificationRequired) {
		t.Fatalf("resourceResolveErrorCode() = %q, want %q", got, apperrors.CodeResourceVerificationRequired)
	}
}

func TestResourceExtractorRegistryRoutesKnownSites(t *testing.T) {
	t.Parallel()

	douyinRules := resourceExtractorForURL("https://www.douyin.com/video/123")
	if douyinRules.Name() != "douyin" || douyinRules.Extractor() != "resource:douyin" {
		t.Fatalf("expected douyin rules, got %s %s", douyinRules.Name(), douyinRules.Extractor())
	}
	iesDouyinRules := resourceExtractorForURL("https://www.iesdouyin.com/share/video/123/")
	if iesDouyinRules.Name() != "douyin" || iesDouyinRules.Extractor() != "resource:douyin" {
		t.Fatalf("expected iesdouyin rules, got %s %s", iesDouyinRules.Name(), iesDouyinRules.Extractor())
	}

	xiaohongshuRules := resourceExtractorForURL("https://www.xiaohongshu.com/explore/test-note-id")
	if xiaohongshuRules.Name() != "xiaohongshu" || xiaohongshuRules.Extractor() != "resource:xiaohongshu" {
		t.Fatalf("expected xiaohongshu rules, got %s %s", xiaohongshuRules.Name(), xiaohongshuRules.Extractor())
	}
	rednoteRules := resourceExtractorForURL("https://www.rednote.com/explore/test-note-id")
	if rednoteRules.Name() != "xiaohongshu" || rednoteRules.Extractor() != "resource:xiaohongshu" {
		t.Fatalf("expected rednote rules, got %s %s", rednoteRules.Name(), rednoteRules.Extractor())
	}
	xhsLinkRules := resourceExtractorForURL("https://xhslink.com/a/example")
	if xhsLinkRules.Name() != "xiaohongshu" || xhsLinkRules.Extractor() != "resource:xiaohongshu" {
		t.Fatalf("expected xhslink rules, got %s %s", xhsLinkRules.Name(), xhsLinkRules.Extractor())
	}

	defaultRules := resourceExtractorForURL("https://example.com/video/123")
	if defaultRules.Name() != "default" || defaultRules.Extractor() != "resource" {
		t.Fatalf("expected default rules, got %s %s", defaultRules.Name(), defaultRules.Extractor())
	}
}

func TestResourceCleanMetadataTextOnlyTrimsWrappedQuotes(t *testing.T) {
	t.Parallel()

	if got := resourceCleanMetadataText(`"quoted title"`); got != "quoted title" {
		t.Fatalf("expected wrapped quotes to be trimmed, got %q", got)
	}
	if got := resourceCleanMetadataText(`@甜”`); got != `@甜”` {
		t.Fatalf("expected unmatched trailing quote to be preserved, got %q", got)
	}
	if got := resourceCleanMetadataText(`“@甜”`); got != `@甜` {
		t.Fatalf("expected paired smart quotes to be trimmed, got %q", got)
	}
}

func TestResourceExtractorRejectsCandidateWithoutVideoDimensions(t *testing.T) {
	t.Parallel()

	now := time.Now()
	candidates := []resourceCandidate{
		{
			url:       "https://v3-dy-o.zjcdn.com/current/douyinvod/video/tos/current.mp4",
			pageURL:   "https://www.douyin.com/search/topic?modal_id=1",
			mimeType:  "video/mp4",
			score:     120,
			sizeBytes: 2 * 1024 * 1024,
			seenAt:    now,
		},
	}

	_, ok := resourceDouyinSiteRules{}.SelectCandidate(candidates, map[string]string{
		"location": "https://www.douyin.com/search/topic?modal_id=1",
		"title":    "Scan QR code to download APP",
	}, now)

	if ok {
		t.Fatal("expected candidate without video dimensions to be rejected")
	}
}

func TestResourceExtractorRejectsHiddenPrimaryVideo(t *testing.T) {
	t.Parallel()

	now := time.Now()
	candidates := []resourceCandidate{
		{
			url:       "https://v3-dy-o.zjcdn.com/current/douyinvod/video/tos/current.mp4",
			pageURL:   "https://www.douyin.com/search/topic?modal_id=1",
			mimeType:  "video/mp4",
			score:     120,
			sizeBytes: 2 * 1024 * 1024,
			seenAt:    now,
		},
	}

	_, ok := resourceDouyinSiteRules{}.SelectCandidate(candidates, map[string]string{
		"location":    "https://www.douyin.com/search/topic?modal_id=1",
		"videoWidth":  "1080",
		"videoHeight": "1920",
		"videoItems":  `[{"currentSrc":"https://v3-dy-o.zjcdn.com/current/douyinvod/video/tos/current.mp4","visibleArea":0}]`,
	}, now)

	if ok {
		t.Fatal("expected hidden primary video to be rejected")
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

func TestResourceSniffLogSummariesRedactURLSecrets(t *testing.T) {
	t.Parallel()

	secretURL := "https://v3-dy-o.zjcdn.com/current/douyinvod/video/tos/current.mp4?token=secret#frag"
	values := []string{
		resourceSniffLogURL(secretURL, 240),
		resourceSniffLogVideoItems(`[{"currentSrc":"`+secretURL+`","visibleArea":120000}]`, 360),
		strings.Join(resourceSniffCandidateSummaries([]resourceCandidate{{url: secretURL, mimeType: "video/mp4", status: http.StatusOK, score: 120}}, 1), "\n"),
		strings.Join(resourceSniffStructuredMediaSummaries([]resourceStructuredMedia{{ID: "a", VideoURL: secretURL, QualityHeight: 1080}}, 1), "\n"),
		strings.Join(resourceSniffNoMediaHintSummaries([]resourceNoMediaHint{{Kind: "video", ID: "a", SourceURL: secretURL}}, 1), "\n"),
	}
	combined := strings.Join(values, "\n")

	for _, leaked := range []string{"token=secret", "#frag"} {
		if strings.Contains(combined, leaked) {
			t.Fatalf("expected log summaries to redact %q, got:\n%s", leaked, combined)
		}
	}
	if !strings.Contains(combined, "https://v3-dy-o.zjcdn.com/current/douyinvod/video/tos/current.mp4") {
		t.Fatalf("expected sanitized media path to remain useful, got:\n%s", combined)
	}
}

func TestResourceCaptureBestForPagePrefersCurrentVideoSource(t *testing.T) {
	t.Parallel()

	now := time.Now()
	state := newResourceCaptureState()
	state.candidates = []resourceCandidate{
		{
			url:       "https://v3-dy-o.zjcdn.com/old/douyinvod/video/tos/old.mp4",
			mimeType:  "video/mp4",
			score:     200,
			sizeBytes: 200 * 1024 * 1024,
			seenAt:    now.Add(-30 * time.Second),
		},
		{
			url:       "https://v3-dy-o.zjcdn.com/current/douyinvod/video/tos/current.mp4?token=1",
			mimeType:  "video/mp4",
			score:     120,
			sizeBytes: 2 * 1024 * 1024,
			seenAt:    now.Add(-20 * time.Second),
		},
	}

	candidate, ok := state.bestForPage(map[string]string{
		"videoItems":  `[{"currentSrc":"https://v3-dy-o.zjcdn.com/current/douyinvod/video/tos/current.mp4?token=1"}]`,
		"videoWidth":  "1080",
		"videoHeight": "1920",
	}, time.Time{})

	if !ok {
		t.Fatal("expected matched candidate")
	}
	if candidate.url != "https://v3-dy-o.zjcdn.com/current/douyinvod/video/tos/current.mp4?token=1" {
		t.Fatalf("expected current video source to win, got %q", candidate.url)
	}
}

func TestResourceCaptureBestForPageDoesNotUseStalePerformanceResource(t *testing.T) {
	t.Parallel()

	now := time.Now()
	state := newResourceCaptureState()
	state.candidates = []resourceCandidate{
		{
			url:       "https://v3-dy-o.zjcdn.com/old/douyinvod/video/tos/old.mp4",
			pageURL:   "https://www.douyin.com/search/topic?modal_id=old",
			mimeType:  "video/mp4",
			score:     200,
			sizeBytes: 200 * 1024 * 1024,
			seenAt:    now.Add(-30 * time.Second),
		},
	}

	_, ok := state.bestForPage(map[string]string{
		"location":           "https://www.douyin.com/search/topic?modal_id=current",
		"resourceVideoItems": `[{"name":"https://v3-dy-o.zjcdn.com/old/douyinvod/video/tos/old.mp4"}]`,
		"videoWidth":         "1080",
		"videoHeight":        "1920",
	}, now)

	if ok {
		t.Fatal("expected stale performance resource to be ignored")
	}
}

func TestResourceCaptureBestForPageDoesNotUseSecondaryVideoItem(t *testing.T) {
	t.Parallel()

	now := time.Now()
	state := newResourceCaptureState()
	state.candidates = []resourceCandidate{
		{
			url:       "https://v3-dy-o.zjcdn.com/old/douyinvod/video/tos/old.mp4",
			pageURL:   "https://www.douyin.com/search/topic?modal_id=old",
			mimeType:  "video/mp4",
			score:     200,
			sizeBytes: 200 * 1024 * 1024,
			seenAt:    now.Add(-30 * time.Second),
		},
	}

	_, ok := state.bestForPage(map[string]string{
		"location": "https://www.douyin.com/search/topic?modal_id=current",
		"videoItems": `[
			{"currentSrc":""},
			{"currentSrc":"https://v3-dy-o.zjcdn.com/old/douyinvod/video/tos/old.mp4"}
		]`,
		"videoWidth":  "1080",
		"videoHeight": "1920",
	}, now)

	if ok {
		t.Fatal("expected secondary video item to be ignored")
	}
}

func TestResourceCaptureBestForPageDoesNotUseRecentFallback(t *testing.T) {
	t.Parallel()

	now := time.Now()
	state := newResourceCaptureState()
	state.candidates = []resourceCandidate{
		{
			url:       "https://v3-dy-o.zjcdn.com/old/douyinvod/video/tos/old.mp4",
			mimeType:  "video/mp4",
			score:     200,
			sizeBytes: 200 * 1024 * 1024,
			seenAt:    now.Add(-30 * time.Second),
		},
		{
			url:       "https://v3-dy-o.zjcdn.com/recent/douyinvod/video/tos/recent.mp4",
			mimeType:  "video/mp4",
			score:     120,
			sizeBytes: 2 * 1024 * 1024,
			seenAt:    now.Add(500 * time.Millisecond),
		},
	}

	_, ok := state.bestForPage(map[string]string{
		"videoWidth":  "1080",
		"videoHeight": "1920",
	}, now)

	if ok {
		t.Fatal("expected unmatched recent candidate to be rejected")
	}
}

func TestDouyinResourceSelectionDoesNotUseRecentFallback(t *testing.T) {
	t.Parallel()

	now := time.Now()
	state := newResourceCaptureState()
	state.candidates = []resourceCandidate{
		{
			url:       "https://v3-dy-o.zjcdn.com/old/douyinvod/video/tos/old.mp4",
			mimeType:  "video/mp4",
			score:     200,
			sizeBytes: 200 * 1024 * 1024,
			seenAt:    now.Add(500 * time.Millisecond),
		},
	}

	_, ok := state.bestForPageUsing(resourceDouyinSiteRules{}, map[string]string{
		"location":    "https://www.douyin.com/video/current",
		"videoWidth":  "1080",
		"videoHeight": "1920",
	}, now)

	if ok {
		t.Fatal("expected douyin rules to reject unmatched recent candidate")
	}
}

func TestDouyinResourceSelectionRejectsUnanchoredSearchPageURLFallback(t *testing.T) {
	t.Parallel()

	now := time.Now()
	state := newResourceCaptureState()
	state.candidates = []resourceCandidate{
		{
			url:       "https://v3-dy-o.zjcdn.com/old/douyinvod/video/tos/old.mp4",
			pageURL:   "https://www.douyin.com/search/topic",
			mimeType:  "video/mp4",
			score:     200,
			sizeBytes: 200 * 1024 * 1024,
			seenAt:    now,
		},
	}

	_, ok := state.bestForPageUsing(resourceDouyinSiteRules{}, map[string]string{
		"location":    "https://www.douyin.com/search/topic",
		"videoWidth":  "1080",
		"videoHeight": "1920",
	}, now)

	if ok {
		t.Fatal("expected douyin rules to reject search page url fallback without modal id")
	}
}

func TestResourceSniffSelectionRejectsWeakGlobalFallbackBeforeSpecialRules(t *testing.T) {
	t.Parallel()

	now := time.Now()
	service := &LibraryService{}
	_, ok := service.selectResourceSniffMedia(
		"https://www.douyin.com/video/current",
		map[string]string{
			"location":    "https://www.douyin.com/video/current",
			"videoWidth":  "1080",
			"videoHeight": "1920",
		},
		[]resourceCandidate{{
			url:       "https://v3-dy-o.zjcdn.com/recent/douyinvod/video/tos/recent.mp4",
			mimeType:  "video/mp4",
			score:     120,
			sizeBytes: 2 * 1024 * 1024,
			seenAt:    now.Add(500 * time.Millisecond),
		}},
		nil,
		now,
	)

	if ok {
		t.Fatal("expected weak global fallback to be rejected")
	}
}

func TestResourceCaptureBestForPageRejectsStaleHistory(t *testing.T) {
	t.Parallel()

	now := time.Now()
	state := newResourceCaptureState()
	state.candidates = []resourceCandidate{
		{
			url:       "https://v3-dy-o.zjcdn.com/old/douyinvod/video/tos/old.mp4",
			pageURL:   "https://www.douyin.com/video/old",
			mimeType:  "video/mp4",
			score:     200,
			sizeBytes: 200 * 1024 * 1024,
			seenAt:    now.Add(-30 * time.Second),
		},
	}

	_, ok := state.bestForPage(map[string]string{
		"location":    "https://www.douyin.com/video/current",
		"videoWidth":  "1080",
		"videoHeight": "1920",
	}, now)

	if ok {
		t.Fatal("expected stale historical candidate to be rejected")
	}
}

func TestResourceCaptureBestForPageRejectsCurrentPageHistoryWithoutCurrentSource(t *testing.T) {
	t.Parallel()

	now := time.Now()
	state := newResourceCaptureState()
	state.candidates = []resourceCandidate{
		{
			url:       "https://v3-dy-o.zjcdn.com/current/douyinvod/video/tos/current.mp4",
			pageURL:   "https://www.douyin.com/video/current?modal_id=1",
			mimeType:  "video/mp4",
			score:     120,
			sizeBytes: 2 * 1024 * 1024,
			seenAt:    now.Add(-30 * time.Second),
		},
	}

	_, ok := state.bestForPage(map[string]string{
		"location":    "https://www.douyin.com/video/current?modal_id=1",
		"videoWidth":  "1080",
		"videoHeight": "1920",
	}, now)

	if ok {
		t.Fatal("expected current page history to be rejected without current video source")
	}
}

func TestResourceCaptureBestForPageRejectsDifferentModalHistory(t *testing.T) {
	t.Parallel()

	now := time.Now()
	state := newResourceCaptureState()
	state.candidates = []resourceCandidate{
		{
			url:       "https://v3-dy-o.zjcdn.com/previous/douyinvod/video/tos/previous.mp4",
			pageURL:   "https://www.douyin.com/search/topic?modal_id=1",
			mimeType:  "video/mp4",
			score:     120,
			sizeBytes: 2 * 1024 * 1024,
			seenAt:    now.Add(-30 * time.Second),
		},
	}

	_, ok := state.bestForPage(map[string]string{
		"location":    "https://www.douyin.com/search/topic?modal_id=2",
		"videoWidth":  "1080",
		"videoHeight": "1920",
	}, now)

	if ok {
		t.Fatal("expected different modal history to be rejected")
	}
}

func TestResourceCaptureBestForPageRejectsNonModalTrackingQueryHistoryWithoutCurrentSource(t *testing.T) {
	t.Parallel()

	now := time.Now()
	state := newResourceCaptureState()
	state.candidates = []resourceCandidate{
		{
			url:       "https://v3-dy-o.zjcdn.com/current/douyinvod/video/tos/current.mp4",
			pageURL:   "https://www.douyin.com/video/current?previous_page=search",
			mimeType:  "video/mp4",
			score:     120,
			sizeBytes: 2 * 1024 * 1024,
			seenAt:    now.Add(-30 * time.Second),
		},
	}

	_, ok := state.bestForPage(map[string]string{
		"location":    "https://www.douyin.com/video/current?from=copy",
		"videoWidth":  "1080",
		"videoHeight": "1920",
	}, now)

	if ok {
		t.Fatal("expected non-modal tracking query history to be rejected without current video source")
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

	now := time.Now()
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

	subtitles := resourceSubtitlesForPage(
		"https://www.douyin.com/video/123",
		map[string]string{"location": "https://www.douyin.com/video/123"},
		state.subtitlesSnapshot(),
		nil,
		now,
	)
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

func TestResourceSubtitlesFromPageMetaTracks(t *testing.T) {
	t.Parallel()

	subtitles := resourceSubtitlesForPage(
		"https://www.douyin.com/video/123",
		map[string]string{
			"location":      "https://www.douyin.com/video/123",
			"subtitleItems": `[{"src":"/tracks/zh-Hans.vtt","srclang":"zh-Hans","label":"Chinese","kind":"subtitles"}]`,
		},
		nil,
		nil,
		time.Now(),
	)
	if len(subtitles) != 1 {
		t.Fatalf("expected one page subtitle, got %d: %#v", len(subtitles), subtitles)
	}
	if subtitles[0].URL != "https://www.douyin.com/tracks/zh-Hans.vtt" {
		t.Fatalf("expected relative subtitle URL to resolve, got %q", subtitles[0].URL)
	}
	if subtitles[0].Language != "zh-Hans" || subtitles[0].Name != "Chinese" || subtitles[0].Ext != "vtt" {
		t.Fatalf("unexpected page subtitle: %#v", subtitles[0])
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

func TestResourceSubtitlesFromAPIResponseUsesYTDLPShape(t *testing.T) {
	t.Parallel()

	response := resourceAPIResponse{
		URL:     "https://www.douyin.com/aweme/v1/web/aweme/detail/?aweme_id=123",
		PageURL: "https://www.douyin.com/video/123",
		RequestHeaders: map[string]string{
			"Cookie": "sid=1",
		},
		Body: []byte(`{
			"aweme_detail": {
				"captionTracks": [{
					"url": "https://cdn.example.com/timedtext?lang=ja&fmt=srt",
					"languageCode": "ja",
					"name": "Japanese",
					"format": "srt"
				}]
			}
		}`),
	}

	subtitles := resourceSubtitlesFromAPIResponse(response)
	if len(subtitles) != 1 {
		t.Fatalf("expected one API subtitle, got %d: %#v", len(subtitles), subtitles)
	}
	if subtitles[0].Language != "ja" || subtitles[0].Name != "Japanese" || subtitles[0].Ext != "srt" {
		t.Fatalf("unexpected API subtitle: %#v", subtitles[0])
	}
	if value, ok := findHeader(subtitles[0].RequestHeaders, "Cookie"); !ok || value != "sid=1" {
		t.Fatalf("expected API subtitle headers to retain cookie, got %#v", subtitles[0].RequestHeaders)
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

func TestResourceMediaFromCandidateKeepsPageDomain(t *testing.T) {
	t.Parallel()

	media := (&LibraryService{}).resourceMediaFromCandidate(
		"https://www.douyin.com/video/123",
		"douyin.com",
		resourceCandidate{
			url:      "https://v3-dy-o.zjcdn.com/abc/video.mp4",
			mimeType: "video/mp4",
			headers:  map[string]string{"Cookie": "sid=1"},
		},
		map[string]string{"title": "Example", "videoWidth": "1080", "videoHeight": "1920"},
	)

	if media.Domain != "douyin.com" {
		t.Fatalf("expected page domain douyin.com, got %q", media.Domain)
	}
	if media.Width != 1080 || media.Height != 1920 {
		t.Fatalf("expected video dimensions 1080x1920, got %dx%d", media.Width, media.Height)
	}
	if value, ok := findHeader(media.RequestHeaders, "Cookie"); !ok || value != "sid=1" {
		t.Fatalf("expected cookie header to be preserved, got %#v", media.RequestHeaders)
	}
}

func TestResourceMediaFromCandidateUsesExplicitVideoMetadata(t *testing.T) {
	t.Parallel()

	media := (&LibraryService{}).resourceMediaFromCandidate(
		"https://www.douyin.com/video/123",
		"douyin.com",
		resourceCandidate{
			url:      "https://v3-dy-o.zjcdn.com/abc/video.mp4",
			mimeType: "video/mp4",
		},
		map[string]string{
			"title":      "抖音 - 记录美好生活",
			"ogTitle":    "Old Title - 抖音",
			"videoTitle": "Real Title - 抖音",
			"apiAuthor":  " Creator ",
			"apiImage":   "https://example.com/cover.jpg",
		},
	)

	if media.Title != "Real Title" {
		t.Fatalf("expected cleaned video title, got %q", media.Title)
	}
	if media.Author != "Creator" {
		t.Fatalf("expected author, got %q", media.Author)
	}
	if media.ThumbnailURL != "https://example.com/cover.jpg" {
		t.Fatalf("expected thumbnail, got %q", media.ThumbnailURL)
	}
}

func TestResourceMediaFromCandidateDoesNotUseTitleAuthorOrDOMImageFallback(t *testing.T) {
	t.Parallel()

	media := (&LibraryService{}).resourceMediaFromCandidate(
		"https://www.douyin.com/video/123",
		"douyin.com",
		resourceCandidate{
			url:      "https://v3-dy-o.zjcdn.com/abc/video.mp4",
			mimeType: "video/mp4",
		},
		map[string]string{
			"title":    "示例账号A的抖音 - 抖音",
			"author":   "我的",
			"domImage": "http://example.com/dom-cover.jpg",
		},
	)

	if media.Author != "" {
		t.Fatalf("expected title/author fallback to be ignored, got %q", media.Author)
	}
	if media.ThumbnailURL != "" {
		t.Fatalf("expected DOM image fallback to be ignored, got %q", media.ThumbnailURL)
	}
}

func TestResourceMediaFromCandidateUsesAPIAuthorNotCapturedAccountFallback(t *testing.T) {
	t.Parallel()

	media := (&LibraryService{}).resourceMediaFromCandidate(
		"https://www.douyin.com/video/123",
		"douyin.com",
		resourceCandidate{
			url:      "https://v3-dy-o.zjcdn.com/abc/video.mp4",
			mimeType: "video/mp4",
		},
		map[string]string{
			"title":      "示例账号A的抖音 - 抖音",
			"author":     "示例账号B",
			"apiAuthor":  "示例账号C",
			"jsonAuthor": "示例账号D",
		},
	)

	if media.Author != "示例账号C" {
		t.Fatalf("expected API author, got %q", media.Author)
	}
}

func TestResourceSniffSelectionUsesDouyinAPIAuthorMatchedByCandidateURL(t *testing.T) {
	t.Parallel()

	now := time.Now()
	videoURL := "https://v3-dy-o.zjcdn.com/current/douyinvod/video/tos/current.mp4"
	service := &LibraryService{}
	selection, ok := service.selectResourceSniffMedia(
		"https://www.douyin.com/?recommend",
		map[string]string{
			"location":        "https://www.douyin.com/?recommend",
			"title":           "抖音 - 记录美好生活",
			"videoCurrentSrc": videoURL,
			"videoItems":      `[{"currentSrc":"https://v3-dy-o.zjcdn.com/current/douyinvod/video/tos/current.mp4","visibleArea":120000}]`,
			"videoWidth":      "1080",
			"videoHeight":     "1920",
		},
		[]resourceCandidate{{
			url:       videoURL,
			pageURL:   "https://www.douyin.com/video/current",
			mimeType:  "video/mp4",
			score:     120,
			sizeBytes: 2 * 1024 * 1024,
			seenAt:    now,
		}},
		[]resourceStructuredMedia{
			{
				ID:       "other",
				VideoURL: "https://v3-dy-o.zjcdn.com/other/douyinvod/video/tos/other.mp4",
				Title:    "Other API Title",
				Author:   "Other API Author",
				PageURL:  "https://www.douyin.com/video/other",
			},
			{
				ID:           "current",
				VideoURL:     videoURL,
				Title:        "Current API Title",
				Author:       "Synthetic API Author",
				ThumbnailURL: "http://p3-sign.douyinpic.com/current-cover.jpeg",
				PageURL:      "https://www.douyin.com/video/current",
			},
		},
		now,
	)
	if !ok {
		t.Fatal("expected douyin recommend feed media selection")
	}
	if selection.Media.Author != "Synthetic API Author" {
		t.Fatalf("expected API author matched by video URL, got %q", selection.Media.Author)
	}
	if selection.Media.Title != "Current API Title" {
		t.Fatalf("expected API title matched by video URL, got %q", selection.Media.Title)
	}
	if selection.Media.ThumbnailURL != "https://p3-sign.douyinpic.com/current-cover.jpeg" {
		t.Fatalf("expected API thumbnail matched by video URL, got %q", selection.Media.ThumbnailURL)
	}
}

func TestResourceSniffSelectionRejectsDouyinRecommendResourceVideoItemFallback(t *testing.T) {
	t.Parallel()

	now := time.Now()
	videoURL := "https://v95-hzyy-thr-daily-web.douyinvod.com/video/tos/current.mp4?token=1"
	service := &LibraryService{}
	_, ok := service.selectResourceSniffMedia(
		"https://www.douyin.com/?recommend=1",
		map[string]string{
			"location":           "https://www.douyin.com/?recommend=1",
			"title":              "抖音 - 记录美好生活",
			"videoCurrentSrc":    "blob:https://www.douyin.com/abc",
			"videoItems":         `[{"currentSrc":"blob:https://www.douyin.com/abc","visibleArea":120000}]`,
			"resourceVideoItems": `[{"name":"https://v95-hzyy-thr-daily-web.douyinvod.com/video/tos/current.mp4?token=1"}]`,
			"videoWidth":         "2560",
			"videoHeight":        "1440",
		},
		[]resourceCandidate{{
			url:        videoURL,
			pageURL:    "https://www.douyin.com/video/current",
			mimeType:   "video/mp4",
			score:      120,
			sizeBytes:  2 * 1024 * 1024,
			seenAt:     now,
			structured: true,
		}},
		[]resourceStructuredMedia{{
			ID:           "current",
			VideoURL:     videoURL,
			Title:        "Current API Title",
			Author:       "Synthetic Recommend Author",
			ThumbnailURL: "http://p3-sign.douyinpic.com/current-cover.jpeg",
			PageURL:      "https://www.douyin.com/video/current",
		}},
		now,
	)
	if ok {
		t.Fatal("expected douyin recommend page to reject resource performance fallback without a visible media source")
	}
}

func TestResourceSniffSelectionRejectsDouyinRecommendWeakGlobalFallback(t *testing.T) {
	t.Parallel()

	now := time.Now()
	service := &LibraryService{}
	_, ok := service.selectResourceSniffMedia(
		"https://www.douyin.com/?recommend",
		map[string]string{
			"location":    "https://www.douyin.com/?recommend",
			"title":       "抖音 - 记录美好生活",
			"videoWidth":  "1080",
			"videoHeight": "1920",
		},
		[]resourceCandidate{{
			url:       "https://v3-dy-o.zjcdn.com/previous/douyinvod/video/tos/previous.mp4",
			pageURL:   "https://www.douyin.com/?recommend",
			mimeType:  "video/mp4",
			score:     120,
			sizeBytes: 2 * 1024 * 1024,
			seenAt:    now.Add(500 * time.Millisecond),
		}},
		nil,
		now,
	)
	if ok {
		t.Fatal("expected douyin recommend page to reject weak global fallback")
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

func TestMapResourceSniffSessionReportsUnoptimizedRegistrableDomain(t *testing.T) {
	t.Parallel()

	service := &LibraryService{
		resourceSniffs: map[string]*resourceSniffSession{
			"session-1": {
				ID:       "session-1",
				URL:      "https://www.douyin.com/video/123",
				ActiveID: "tab-1",
				Tabs: map[string]*resourceSniffTab{
					"tab-1": {
						TargetID:   "tab-1",
						CurrentURL: "https://search.bilibili.com/video?keyword=test",
						LastSeen:   time.Now(),
					},
				},
			},
		},
	}

	session := service.mapResourceSniffSession(service.resourceSniffs["session-1"])

	if session.UnoptimizedDomain != "bilibili.com" {
		t.Fatalf("expected unoptimized registrable domain, got %q", session.UnoptimizedDomain)
	}
}

func TestMapResourceSniffSessionReportsUnoptimizedYouTubeDomain(t *testing.T) {
	t.Parallel()

	service := &LibraryService{
		resourceSniffs: map[string]*resourceSniffSession{
			"session-1": {
				ID:       "session-1",
				URL:      "https://www.youtube.com/watch?v=abc",
				ActiveID: "tab-1",
				Tabs: map[string]*resourceSniffTab{
					"tab-1": {
						TargetID:   "tab-1",
						CurrentURL: "https://www.youtube.com/watch?v=abc",
						LastSeen:   time.Now(),
					},
				},
			},
		},
	}

	session := service.mapResourceSniffSession(service.resourceSniffs["session-1"])

	if session.UnoptimizedDomain != "youtube.com" {
		t.Fatalf("expected youtube to be reported as unoptimized, got %q", session.UnoptimizedDomain)
	}
}

func TestMapResourceSniffSessionOmitsOptimizedDomainWarning(t *testing.T) {
	t.Parallel()

	service := &LibraryService{
		resourceSniffs: map[string]*resourceSniffSession{
			"session-1": {
				ID:       "session-1",
				URL:      "https://www.douyin.com/video/123",
				ActiveID: "tab-1",
				Tabs: map[string]*resourceSniffTab{
					"tab-1": {
						TargetID:   "tab-1",
						CurrentURL: "https://www.douyin.com/video/123",
						LastSeen:   time.Now(),
					},
				},
			},
		},
	}

	session := service.mapResourceSniffSession(service.resourceSniffs["session-1"])

	if session.UnoptimizedDomain != "" {
		t.Fatalf("expected optimized domain warning to be omitted, got %q", session.UnoptimizedDomain)
	}
}

func TestApplyResourceMediaToOperationCarriesMetadata(t *testing.T) {
	t.Parallel()

	operation := library.LibraryOperation{}
	history := library.HistoryRecord{}
	request := dto.CreateYTDLPJobRequest{URL: "https://www.douyin.com/video/123"}
	updated := (&LibraryService{}).applyResourceMediaToOperation(
		context.Background(),
		&operation,
		&history,
		request,
		resourceMedia{
			PageURL:      "https://www.douyin.com/video/456",
			Title:        "Resolved Title",
			Author:       "Creator",
			ThumbnailURL: "https://example.com/cover.jpg",
			Extractor:    "resource:douyin",
			Domain:       "douyin.com",
		},
	)

	if updated.URL != "https://www.douyin.com/video/456" {
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
	if operation.Meta.Platform != "resource:douyin" || operation.SourceDomain != "douyin.com" {
		t.Fatalf("expected resource platform/domain, got platform=%q domain=%q", operation.Meta.Platform, operation.SourceDomain)
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

func TestResourceCandidateOptionsForPageRequiresCurrentVideoSource(t *testing.T) {
	t.Parallel()

	now := time.Now()
	candidates := []resourceCandidate{
		{
			url:      "https://cdn.example.com/low.mp4",
			pageURL:  "https://example.com/watch/1",
			mimeType: "video/mp4",
			score:    100,
			seenAt:   now,
		},
		{
			url:      "https://cdn.example.com/high.mp4",
			pageURL:  "https://example.com/watch/1",
			mimeType: "video/mp4",
			score:    120,
			seenAt:   now,
		},
	}
	options := resourceCandidateOptionsForPage(candidates, map[string]string{
		"location":        "https://example.com/watch/1",
		"videoCurrentSrc": "https://cdn.example.com/high.mp4",
		"videoItems":      `[{"currentSrc":"https://cdn.example.com/high.mp4","visibleArea":120000}]`,
		"videoWidth":      "1920",
		"videoHeight":     "1080",
	})

	if len(options) != 1 {
		t.Fatalf("expected one current-source candidate, got %d", len(options))
	}
	if options[0].url != "https://cdn.example.com/high.mp4" {
		t.Fatalf("expected current-source candidate, got %q", options[0].url)
	}

	options = resourceCandidateOptionsForPage(candidates, map[string]string{
		"location":    "https://example.com/watch/1",
		"videoWidth":  "1920",
		"videoHeight": "1080",
	})
	if len(options) != 0 {
		t.Fatalf("expected no candidates without current video source, got %d", len(options))
	}
}

func TestResourceStructuredDataExtractsDownloadableHLSVODManifest(t *testing.T) {
	t.Parallel()

	seenAt := time.Date(2026, 5, 24, 10, 0, 0, 0, time.UTC)
	mediaItems, _ := resourceStructuredDataFromAPIResponses(resourceDefaultSiteRules{}, []resourceAPIResponse{
		{
			URL:            "https://media.example/replay/index.m3u8",
			PageURL:        "https://page.example/watch",
			ContentType:    "application/vnd.apple.mpegurl",
			RequestHeaders: map[string]string{"Referer": "https://page.example/watch"},
			Body: []byte(`#EXTM3U
#EXT-X-PLAYLIST-TYPE:EVENT
#EXT-X-TARGETDURATION:6
#EXTINF:6,
00001.ts
#EXT-X-ENDLIST
`),
			SeenAt: seenAt,
		},
	})
	if len(mediaItems) != 1 {
		t.Fatalf("expected one structured hls vod media item, got %d: %#v", len(mediaItems), mediaItems)
	}
	selection, ok := buildResourceSniffMediaSelection(
		&LibraryService{},
		resourceDefaultSiteRules{},
		"https://page.example/watch",
		"example",
		nil,
		map[string]string{"title": "Replay"},
		mediaItems,
		seenAt,
	)
	if !ok {
		t.Fatal("expected hls vod structured media to be selectable without segment candidates")
	}
	if selection.Media.URL != "https://media.example/replay/index.m3u8" ||
		selection.Media.Kind != "video" ||
		selection.Media.ContentType != "application/vnd.apple.mpegurl" ||
		selection.Media.Ext != ".m3u8" {
		t.Fatalf("unexpected selected hls media: %#v", selection.Media)
	}
	if selection.Media.RequestHeaders["Referer"] != "https://page.example/watch" {
		t.Fatalf("expected hls request headers to be preserved, got %#v", selection.Media.RequestHeaders)
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

func TestDouyinExtractsStructuredMediaFromAPIResponse(t *testing.T) {
	t.Parallel()

	response := resourceAPIResponse{
		URL:          "https://www.douyin.com/aweme/v1/web/aweme/detail/?aweme_id=test-aweme-api",
		PageURL:      "https://www.douyin.com/video/test-aweme-api",
		MimeType:     "application/json",
		ContentType:  "application/json; charset=utf-8",
		Status:       http.StatusOK,
		ResourceType: network.ResourceTypeXHR,
		SizeBytes:    int64(len(testDouyinAPIJSON())),
		RequestHeaders: map[string]string{
			"Cookie":     "sid=1",
			"User-Agent": "TestAgent",
		},
		Body: []byte(testDouyinAPIJSON()),
	}

	items := resourceDouyinSiteRules{}.ExtractMediaFromResponse(response)
	if len(items) != 2 {
		t.Fatalf("expected structured media items for play_addr and h264, got %d: %#v", len(items), items)
	}
	item := items[0]
	if item.ID != "test-aweme-api" {
		t.Fatalf("expected aweme id, got %q", item.ID)
	}
	if item.VideoURL != "https://v3-dy-o.zjcdn.com/current.mp4?token=1" {
		t.Fatalf("expected video url, got %q", item.VideoURL)
	}
	if item.Title != "API Title #tag" {
		t.Fatalf("expected title from API, got %q", item.Title)
	}
	if item.Author != "Fixture Author" {
		t.Fatalf("expected author from API, got %q", item.Author)
	}
	if item.ThumbnailURL != "http://p3-sign.douyinpic.com/cover.jpeg" {
		t.Fatalf("expected cover from API, got %q", item.ThumbnailURL)
	}
	if item.Width != 1080 || item.Height != 1920 {
		t.Fatalf("expected dimensions 1080x1920, got %dx%d", item.Width, item.Height)
	}
	if item.SizeBytes != 1234567 {
		t.Fatalf("expected size from API, got %d", item.SizeBytes)
	}
	if items[1].VideoURL != "https://v3-dy-o.zjcdn.com/current-h264.mp4?token=1" {
		t.Fatalf("expected h264 video url, got %q", items[1].VideoURL)
	}
	if value, ok := findHeader(item.Headers, "Cookie"); !ok || value != "sid=1" {
		t.Fatalf("expected request headers to be retained, got %#v", item.Headers)
	}
}

func TestDouyinExtractsWebBitrateInfoPlayAddr(t *testing.T) {
	t.Parallel()

	response := resourceAPIResponse{
		URL:     "https://www.douyin.com/aweme/v1/web/aweme/detail/?aweme_id=123",
		PageURL: "https://www.douyin.com/video/123",
		Body: []byte(`{
			"aweme_detail": {
				"aweme_id": "123",
				"desc": "Web API Title",
				"author": {"nickname": "Web Author"},
				"video": {
					"width": 720,
					"height": 1280,
					"cover": {"UrlList": ["http://p3-sign.douyinpic.com/web-cover.jpeg"]},
					"bitrateInfo": [{
						"bitrate": 1000,
							"PlayAddr": {
								"UrlKey": "v0200fg10000abc_h264_720p_1000000",
								"UrlList": ["https://v3-dy-o.zjcdn.com/web-current.mp4?token=1"],
								"DataSize": 3456789
							}
						}]
					}
				}
		}`),
	}

	items := resourceDouyinSiteRules{}.ExtractMediaFromResponse(response)
	if len(items) != 1 {
		t.Fatalf("expected one media item, got %d", len(items))
	}
	item := items[0]
	if item.VideoURL != "https://v3-dy-o.zjcdn.com/web-current.mp4?token=1" {
		t.Fatalf("expected web bitrate video URL, got %q", item.VideoURL)
	}
	if item.Title != "Web API Title" || item.Author != "Web Author" {
		t.Fatalf("expected web detail metadata, got title=%q author=%q", item.Title, item.Author)
	}
	if item.ThumbnailURL != "http://p3-sign.douyinpic.com/web-cover.jpeg" {
		t.Fatalf("expected web cover URL, got %q", item.ThumbnailURL)
	}
	if item.FormatID != "h264_720p_1000000" || item.QualityHeight != 720 || item.Width != 720 || item.Height != 1280 {
		t.Fatalf("expected web url_key dimensions, got format=%q quality=%d size=%dx%d", item.FormatID, item.QualityHeight, item.Width, item.Height)
	}
}

func TestDouyinExtractsProtocolRelativeWebBitrateInfoPlayAddr(t *testing.T) {
	t.Parallel()

	response := resourceAPIResponse{
		URL:     "https://www.douyin.com/aweme/v1/web/aweme/detail/?aweme_id=123",
		PageURL: "https://www.douyin.com/video/123",
		Body: []byte(`{
			"aweme_detail": {
				"aweme_id": "123",
				"desc": "Protocol Relative Title",
				"author": {"nickname": "Protocol Relative Author"},
				"video": {
					"width": 720,
					"height": 1280,
					"cover": {"UrlList": ["//p3-sign.douyinpic.com/web-cover.jpeg"]},
					"bitrateInfo": [{
						"bitrate": 1000,
							"PlayAddr": {
								"UrlKey": "v0200fg10000abc_h264_720p_1000000",
								"UrlList": ["//v3-dy-o.zjcdn.com/web-current.mp4?token=1"],
								"DataSize": 3456789
							}
						}]
					}
				}
		}`),
	}

	items := resourceDouyinSiteRules{}.ExtractMediaFromResponse(response)
	if len(items) != 1 {
		t.Fatalf("expected one media item, got %d", len(items))
	}
	item := items[0]
	if item.VideoURL != "https://v3-dy-o.zjcdn.com/web-current.mp4?token=1" {
		t.Fatalf("expected protocol-relative video URL to resolve to https, got %q", item.VideoURL)
	}
	if item.ThumbnailURL != "https://p3-sign.douyinpic.com/web-cover.jpeg" {
		t.Fatalf("expected protocol-relative cover URL to resolve to https, got %q", item.ThumbnailURL)
	}
}

func TestDouyinExtractsMultipleBitrateFormatsAndUsesURLKeyQuality(t *testing.T) {
	t.Parallel()

	response := resourceAPIResponse{
		URL:     "https://www.douyin.com/aweme/v1/web/aweme/detail/?aweme_id=123",
		PageURL: "https://www.douyin.com/video/123",
		Body: []byte(`{
			"aweme_detail": {
				"aweme_id": "123",
				"desc": "Quality Title",
				"author": {"nickname": "Quality Author"},
				"video": {
					"width": 720,
					"height": 2160,
					"bit_rate": [{
						"gear_name": "normal_720",
						"bit_rate": 1800000,
						"play_addr": {
							"url_key": "v0200fg10000abc_h264_720p_1800000",
							"url_list": ["https://v3-dy-o.zjcdn.com/720-a.mp4", "https://v3-dy-o.zjcdn.com/720-b.mp4"],
							"data_size": 222
						}
					}, {
						"gear_name": "normal_540",
						"bit_rate": 900000,
						"play_addr": {
							"url_key": "v0200fg10000abc_h264_540p_900000",
							"url_list": ["https://v3-dy-o.zjcdn.com/540.mp4"],
							"data_size": 111
						}
					}]
				}
			}
		}`),
	}

	items := resourceDouyinSiteRules{}.ExtractMediaFromResponse(response)
	if len(items) != 3 {
		t.Fatalf("expected every bitrate url_list entry as a format, got %d: %#v", len(items), items)
	}
	if items[0].QualityHeight != 720 || items[0].FormatID != "h264_720p_1800000" {
		t.Fatalf("expected 720p url_key metadata, got quality=%d format=%q", items[0].QualityHeight, items[0].FormatID)
	}
	if items[1].VideoURL != "https://v3-dy-o.zjcdn.com/720-b.mp4" || items[1].QualityHeight != 720 {
		t.Fatalf("expected second url_list entry to be retained as 720p, got url=%q quality=%d", items[1].VideoURL, items[1].QualityHeight)
	}
	if items[0].Width != 0 || items[0].Height != 0 {
		t.Fatalf("expected app bit_rate url_key not to inherit parent dimensions, got %dx%d", items[0].Width, items[0].Height)
	}
	if items[2].QualityHeight != 540 || items[2].FormatID != "h264_540p_900000" {
		t.Fatalf("expected 540p url_key metadata, got quality=%d format=%q", items[2].QualityHeight, items[2].FormatID)
	}
	options := resourceDouyinSiteRules{}.MediaOptionsFromStructured(
		"https://www.douyin.com/video/123",
		"douyin.com",
		map[string]string{
			"location":    "https://www.douyin.com/video/123",
			"awemeID":     "123",
			"videoWidth":  "720",
			"videoHeight": "2160",
		},
		items,
	)
	if len(options) != 2 {
		t.Fatalf("expected mirrored bitrate urls to collapse into two media options, got %d", len(options))
	}
	format := resourceFormatOption(options[0])
	if format.Height != 720 || !strings.Contains(format.Label, "720p") {
		t.Fatalf("expected 720p format label, got height=%d label=%q", format.Height, format.Label)
	}
	if options[0].URL != "https://v3-dy-o.zjcdn.com/720-a.mp4" {
		t.Fatalf("expected first mirrored url to be retained, got %q", options[0].URL)
	}
	if options[1].QualityHeight != 540 {
		t.Fatalf("expected 540p option after collapsed mirrors, got quality=%d", options[1].QualityHeight)
	}
}

func TestDouyinBitrateWithoutURLKeyDoesNotInheritPortraitHeight(t *testing.T) {
	t.Parallel()

	response := resourceAPIResponse{
		URL:     "https://www.douyin.com/aweme/v1/web/aweme/detail/?aweme_id=123",
		PageURL: "https://www.douyin.com/video/123",
		Body: []byte(`{
			"aweme_detail": {
				"aweme_id": "123",
				"video": {
					"width": 720,
					"height": 2160,
					"bit_rate": [{
						"gear_name": "normal_720",
						"bit_rate": 1800000,
						"play_addr": {
							"url_list": ["https://v3-dy-o.zjcdn.com/normal.mp4"],
							"data_size": 222
						}
					}]
				}
			}
		}`),
	}

	items := resourceDouyinSiteRules{}.ExtractMediaFromResponse(response)
	if len(items) != 1 {
		t.Fatalf("expected one bitrate format, got %d: %#v", len(items), items)
	}
	if items[0].QualityHeight != 0 || items[0].Width != 0 || items[0].Height != 0 {
		t.Fatalf("expected no inferred dimensions without url_key, got quality=%d size=%dx%d", items[0].QualityHeight, items[0].Width, items[0].Height)
	}
	options := resourceDouyinSiteRules{}.MediaOptionsFromStructured(
		"https://www.douyin.com/video/123",
		"douyin.com",
		map[string]string{
			"location":    "https://www.douyin.com/video/123",
			"awemeID":     "123",
			"videoWidth":  "720",
			"videoHeight": "2160",
		},
		items,
	)
	if len(options) != 1 {
		t.Fatalf("expected one media option, got %d", len(options))
	}
	format := resourceFormatOption(options[0])
	if format.Height != 0 || strings.Contains(format.Label, "2160p") {
		t.Fatalf("expected no 2160p fallback without url_key, got height=%d label=%q", format.Height, format.Label)
	}
}

func TestDouyinDownloadAddrUsesOwnWidthAndVideoRatio(t *testing.T) {
	t.Parallel()

	response := resourceAPIResponse{
		URL:     "https://www.douyin.com/aweme/v1/web/aweme/detail/?aweme_id=123",
		PageURL: "https://www.douyin.com/video/123",
		Body: []byte(`{
			"aweme_detail": {
				"aweme_id": "123",
				"video": {
					"width": 720,
					"height": 2160,
					"download_addr": {
						"width": 360,
						"height": 9999,
						"url_list": ["https://v3-dy-o.zjcdn.com/download.mp4"]
					}
				}
			}
		}`),
	}

	items := resourceDouyinSiteRules{}.ExtractMediaFromResponse(response)
	if len(items) != 1 {
		t.Fatalf("expected one download_addr format, got %d: %#v", len(items), items)
	}
	if items[0].Width != 360 || items[0].Height != 1080 {
		t.Fatalf("expected download_addr height from width/video ratio, got %dx%d", items[0].Width, items[0].Height)
	}
}

func TestDouyinExtractsWebDownloadAddrNestedURL(t *testing.T) {
	t.Parallel()

	response := resourceAPIResponse{
		URL:     "https://www.douyin.com/aweme/v1/web/aweme/detail/?aweme_id=123",
		PageURL: "https://www.douyin.com/video/123",
		Body: []byte(`{
			"aweme_detail": {
				"aweme_id": "123",
				"video": {
					"downloadAddr": {
						"download": {
							"url": "https://v3-dy-o.zjcdn.com/download-web.mp4"
						}
					}
				}
			}
		}`),
	}

	items := resourceDouyinSiteRules{}.ExtractMediaFromResponse(response)
	if len(items) != 1 {
		t.Fatalf("expected one nested web download format, got %d: %#v", len(items), items)
	}
	if items[0].VideoURL != "https://v3-dy-o.zjcdn.com/download-web.mp4" {
		t.Fatalf("expected nested web download URL, got %q", items[0].VideoURL)
	}
	if items[0].FormatID != "download" {
		t.Fatalf("expected nested web download format id, got %q", items[0].FormatID)
	}
}

func TestDouyinStructuredMediaOptionsPreferPlayableCodecOverBytevc2(t *testing.T) {
	t.Parallel()

	options := resourceDouyinSiteRules{}.MediaOptionsFromStructured(
		"https://www.douyin.com/video/123",
		"douyin.com",
		map[string]string{
			"location": "https://www.douyin.com/video/123",
			"awemeID":  "123",
		},
		[]resourceStructuredMedia{
			{
				ID:            "123",
				VideoURL:      "https://v3-dy-o.zjcdn.com/unplayable-bytevc2.mp4",
				FormatID:      "bytevc2_1080p_2000000",
				VCodec:        "bytevc2",
				QualityHeight: 1080,
			},
			{
				ID:            "123",
				VideoURL:      "https://v3-dy-o.zjcdn.com/playable-h264.mp4",
				FormatID:      "h264_720p_1000000",
				VCodec:        "h264",
				QualityHeight: 720,
			},
		},
	)

	if len(options) != 2 {
		t.Fatalf("expected two codec options, got %d", len(options))
	}
	if options[0].URL != "https://v3-dy-o.zjcdn.com/playable-h264.mp4" {
		t.Fatalf("expected playable codec to outrank bytevc2, got %q", options[0].URL)
	}
	if options[0].VCodec != "h264" {
		t.Fatalf("expected primary codec h264, got %q", options[0].VCodec)
	}
}

func TestDouyinStructuredMediaEnrichesPageMetaAndSelectsAPICandidate(t *testing.T) {
	t.Parallel()

	now := time.Now()
	mediaItems := []resourceStructuredMedia{{
		ID:           "123",
		VideoURL:     "https://v3-dy-o.zjcdn.com/current.mp4?token=1",
		Title:        "API Title",
		Author:       "Fixture Author",
		ThumbnailURL: "http://p3-sign.douyinpic.com/cover.jpeg",
		Width:        1080,
		Height:       1920,
		SizeBytes:    1234567,
		SeenAt:       now,
	}}
	pageMeta := resourceDouyinSiteRules{}.EnrichPageMeta(map[string]string{
		"location":    "https://www.douyin.com/search/topic?modal_id=123",
		"title":       "Captured Account的抖音 - 抖音",
		"videoWidth":  "1080",
		"videoHeight": "1920",
		"videoItems":  `[{"currentSrc":"https://v3-dy-o.zjcdn.com/old.mp4","visibleArea":1}]`,
	}, mediaItems)
	candidate, ok := resourceDouyinSiteRules{}.SelectCandidate([]resourceCandidate{
		{
			url:       "https://v3-dy-o.zjcdn.com/old.mp4",
			mimeType:  "video/mp4",
			score:     200,
			sizeBytes: 9 * 1024 * 1024,
			seenAt:    now,
		},
		{
			url:       "https://v3-dy-o.zjcdn.com/current.mp4?token=1",
			mimeType:  "video/mp4",
			score:     120,
			sizeBytes: 1234567,
			seenAt:    now,
		},
	}, pageMeta, now)
	if !ok {
		t.Fatal("expected API-backed candidate")
	}
	if candidate.url != "https://v3-dy-o.zjcdn.com/current.mp4?token=1" {
		t.Fatalf("expected API video candidate to win, got %q", candidate.url)
	}

	media := resourceDouyinSiteRules{}.MediaFromCandidate(&LibraryService{}, pageMeta["location"], "douyin.com", candidate, pageMeta)
	if media.Title != "API Title" || media.Author != "Fixture Author" {
		t.Fatalf("expected API metadata, got title=%q author=%q", media.Title, media.Author)
	}
	if media.ThumbnailURL != "https://p3-sign.douyinpic.com/cover.jpeg" {
		t.Fatalf("expected secure API cover, got %q", media.ThumbnailURL)
	}
}

func TestDouyinSelectsDetailAPIVideoWithoutCapturedVideoRequest(t *testing.T) {
	t.Parallel()

	now := time.Now()
	pageMeta := map[string]string{
		"location":     "https://www.douyin.com/video/123",
		"awemeID":      "123",
		"apiVideoURL":  "https://v3-dy-o.zjcdn.com/current.mp4?token=1",
		"apiTitle":     "API Title",
		"apiAuthor":    "Fixture Author",
		"apiImage":     "http://p3-sign.douyinpic.com/cover.jpeg",
		"apiSizeBytes": "1234567",
	}
	candidate, ok := resourceDouyinSiteRules{}.SelectCandidate([]resourceCandidate{{
		url:       "https://v3-dy-o.zjcdn.com/previous.mp4",
		pageURL:   "https://www.douyin.com/video/123",
		mimeType:  "video/mp4",
		score:     200,
		sizeBytes: 9 * 1024 * 1024,
		seenAt:    now,
	}}, pageMeta, now)
	if !ok {
		t.Fatal("expected detail API video candidate")
	}
	if candidate.url != pageMeta["apiVideoURL"] {
		t.Fatalf("expected detail API video URL, got %q", candidate.url)
	}
	if candidate.sizeBytes != 1234567 {
		t.Fatalf("expected detail API size, got %d", candidate.sizeBytes)
	}
}

func TestDouyinStructuredMediaDoesNotFallbackAcrossPageIDs(t *testing.T) {
	t.Parallel()

	pageMeta := resourceDouyinSiteRules{}.EnrichPageMeta(map[string]string{
		"location":    "https://www.douyin.com/search/topic?modal_id=current",
		"videoWidth":  "1080",
		"videoHeight": "1920",
	}, []resourceStructuredMedia{{
		ID:       "previous",
		VideoURL: "https://v3-dy-o.zjcdn.com/previous.mp4",
		Width:    1080,
		Height:   1920,
	}})

	if pageMeta["apiVideoURL"] != "" {
		t.Fatalf("expected mismatched API media not to enrich page meta, got %q", pageMeta["apiVideoURL"])
	}
	_, ok := resourceDouyinSiteRules{}.SelectCandidate([]resourceCandidate{{
		url:      "https://v3-dy-o.zjcdn.com/previous.mp4",
		mimeType: "video/mp4",
		score:    120,
	}}, pageMeta, time.Now())
	if ok {
		t.Fatal("expected mismatched API candidate to be rejected")
	}
}

func TestXiaohongshuExtractsStructuredMediaFromInitialState(t *testing.T) {
	t.Parallel()

	response := resourceAPIResponse{
		URL:     "https://www.xiaohongshu.com/explore/test-note-id",
		PageURL: "https://www.xiaohongshu.com/explore/test-note-id",
		RequestHeaders: map[string]string{
			"Cookie":     "xsec=1",
			"User-Agent": "TestAgent",
		},
		Body: []byte(`{
			"note": {
				"noteDetailMap": {
					"test-note-id": {
						"note": {
							"noteId": "test-note-id",
							"title": "XHS API Title",
							"desc": "#tag",
							"user": {"nickname": "XHS Author"},
							"imageList": [{
								"urlDefault": "https://sns-webpic-qc.xhscdn.com/cover.jpg",
								"width": 1080,
								"height": 1920
							}],
							"video": {
								"media": {
									"stream": {
										"h264": [{
											"masterUrl": "https://sns-video-bd.xhscdn.com/stream/master.mp4",
											"backupUrls": ["https://sns-video-bd.xhscdn.com/stream/backup.mp4"],
											"width": 1080,
											"height": 1920,
											"videoCodec": "h264",
											"audioCodec": "aac",
											"qualityType": "HD",
											"size": 1234567
										}]
									}
								},
								"consumer": {
									"originVideoKey": "origin/current.mp4"
								}
							}
						}
					}
				}
			}
		}`),
	}

	items := resourceXiaohongshuSiteRules{}.ExtractMediaFromResponse(response)
	if len(items) != 3 {
		t.Fatalf("expected xiaohongshu structured media formats, got %d: %#v", len(items), items)
	}
	item := items[0]
	if item.ID != "test-note-id" || item.VideoURL != "https://sns-video-bd.xhscdn.com/stream/master.mp4" {
		t.Fatalf("unexpected primary xiaohongshu media: %#v", item)
	}
	if item.Title != "XHS API Title" || item.Author != "XHS Author" {
		t.Fatalf("expected xiaohongshu metadata, got title=%q author=%q", item.Title, item.Author)
	}
	if item.ThumbnailURL != "https://sns-webpic-qc.xhscdn.com/cover.jpg" {
		t.Fatalf("expected xiaohongshu thumbnail, got %q", item.ThumbnailURL)
	}
	if item.Width != 1080 || item.Height != 1920 || item.QualityHeight != 0 {
		t.Fatalf("expected portrait dimensions without fake quality height, got quality=%d size=%dx%d", item.QualityHeight, item.Width, item.Height)
	}
	if items[2].VideoURL != "https://sns-video-bd.xhscdn.com/origin/current.mp4" {
		t.Fatalf("expected origin video URL, got %q", items[2].VideoURL)
	}

	options := resourceXiaohongshuSiteRules{}.MediaOptionsFromStructured(
		"https://www.xiaohongshu.com/explore/test-note-id",
		"xiaohongshu.com",
		map[string]string{
			"location": "https://www.xiaohongshu.com/explore/test-note-id",
			"noteID":   "test-note-id",
		},
		items,
	)
	if len(options) != 2 {
		t.Fatalf("expected mirrored xiaohongshu URLs to collapse into stream plus origin options, got %d", len(options))
	}
	format := resourceFormatOption(options[0])
	if format.Height != 0 || format.Label != "1080x1920 · mp4 · H.264 · 1.2MB" {
		t.Fatalf("expected portrait format label, got height=%d label=%q", format.Height, format.Label)
	}
}

func TestResourceCaptureResponseBodyStoresAPIResponseWithoutExtractorPollution(t *testing.T) {
	t.Parallel()

	state := newResourceCaptureState()
	requestID := network.RequestID("api-1")
	state.recordRequest(
		requestID,
		"https://www.douyin.com/aweme/v1/web/aweme/detail/?aweme_id=test-aweme-api",
		"https://www.douyin.com/video/test-aweme-api",
		network.Headers{"Cookie": "sid=1", "User-Agent": "TestAgent"},
	)
	state.recordResponse(
		requestID,
		"https://www.douyin.com/aweme/v1/web/aweme/detail/?aweme_id=test-aweme-api",
		http.StatusOK,
		"application/json",
		network.Headers{"content-type": "application/json", "content-length": strconv.Itoa(len(testDouyinAPIJSON()))},
		network.ResourceTypeXHR,
	)
	state.recordResponseBody(requestID, []byte(testDouyinAPIJSON()))

	responses := state.apiResponsesSnapshot()
	if len(responses) != 1 {
		t.Fatalf("expected captured api response, got %d", len(responses))
	}
	candidates := state.candidatesSnapshot()
	if len(candidates) != 0 {
		t.Fatalf("expected capture to avoid extractor-generated candidates, got %d", len(candidates))
	}
	if responses[0].URL != "https://www.douyin.com/aweme/v1/web/aweme/detail/?aweme_id=test-aweme-api" {
		t.Fatalf("expected api response url, got %q", responses[0].URL)
	}
	if value, ok := findHeader(responses[0].RequestHeaders, "Cookie"); !ok || value != "sid=1" {
		t.Fatalf("expected api response headers to retain cookie, got %#v", responses[0].RequestHeaders)
	}

	mediaItems, _ := resourceStructuredDataFromAPIResponses(resourceDouyinSiteRules{}, responses)
	if len(mediaItems) != 2 {
		t.Fatalf("expected extractor to derive structured media items, got %d", len(mediaItems))
	}
	if mediaItems[0].VideoURL != "https://v3-dy-o.zjcdn.com/current.mp4?token=1" {
		t.Fatalf("expected extractor media url, got %q", mediaItems[0].VideoURL)
	}
	if value, ok := findHeader(mediaItems[0].Headers, "Cookie"); !ok || value != "sid=1" {
		t.Fatalf("expected extractor media headers to retain cookie, got %#v", mediaItems[0].Headers)
	}
}

func TestResourceCaptureAllowsLargeDouyinRecommendFeedResponse(t *testing.T) {
	t.Parallel()

	request := resourceRequest{
		url:          "https://www.douyin.com/aweme/v1/web/tab/feed/?count=10&recommend=1",
		responseURL:  "https://www.douyin.com/aweme/v1/web/tab/feed/?count=10&recommend=1",
		status:       http.StatusOK,
		mimeType:     "application/json",
		contentType:  "application/json",
		sizeBytes:    int64(resourceMaxAPIResponseBodyBytes + 1),
		resourceType: network.ResourceTypeXHR,
	}
	if !shouldCaptureResourceAPIResponse(request) {
		t.Fatal("expected large douyin recommend feed response to be captured")
	}
	if resourceMaxAPIResponseBodyBytesForRequest(request) <= resourceMaxAPIResponseBodyBytes {
		t.Fatal("expected douyin recommend feed to use expanded API response body limit")
	}
}

func TestResourceDouyinTabFeedExtractsRecommendMediaAndLiveHint(t *testing.T) {
	t.Parallel()

	response := resourceAPIResponse{
		URL:     "https://www.douyin.com/aweme/v1/web/tab/feed/?count=10&recommend=1",
		PageURL: "https://www.douyin.com/?recommend=1",
		Body: []byte(`{
			"aweme_list": [
				{
					"aweme_id": "test-aweme-live",
					"group_id": "test-aweme-live",
					"aweme_type": 101,
					"cell_room": {
						"room_id": "test-live-room",
						"title": "直播间标题",
						"owner": {"nickname": "直播作者"}
					}
				},
				{
					"aweme_id": "test-aweme-video",
					"group_id": "test-aweme-video",
					"aweme_type": 0,
					"desc": "Synthetic recommend feed title",
					"author": {"nickname": "Synthetic Recommend Author"},
					"video": {
						"width": 2560,
						"height": 1440,
						"play_addr": {
							"url_list": ["https://v95-hzyy-thr-daily-web.douyinvod.com/video/tos/current.mp4?token=1"],
							"data_size": 2234567
						},
						"cover": {"url_list": ["http://p3-sign.douyinpic.com/current.jpeg"]}
					}
				}
			],
			"has_more": 1,
			"status_code": 0
		}`),
		SeenAt: time.Now(),
	}
	rules := resourceDouyinSiteRules{}

	mediaItems := rules.ExtractMediaFromResponse(response)
	if len(mediaItems) != 1 {
		t.Fatalf("expected only normal recommend media item, got %d: %#v", len(mediaItems), mediaItems)
	}
	if mediaItems[0].ID != "test-aweme-video" || mediaItems[0].Author != "Synthetic Recommend Author" {
		t.Fatalf("unexpected recommend media metadata: %#v", mediaItems[0])
	}

	hints := rules.ExtractNoMediaHintsFromResponse(response)
	if len(hints) != 1 {
		t.Fatalf("expected one live no-media hint, got %d: %#v", len(hints), hints)
	}
	if hints[0].Kind != resourceDouyinLiveHintKind || hints[0].ID != "test-aweme-live" {
		t.Fatalf("unexpected live hint: %#v", hints[0])
	}
	if len(hints[0].AltIDs) != 1 || hints[0].AltIDs[0] != "test-live-room" {
		t.Fatalf("expected live room alt id, got %#v", hints[0].AltIDs)
	}
	if failure, ok := rules.NoMediaFailure(map[string]string{
		"location":       "https://www.douyin.com/?recommend=1",
		"visibleAwemeID": "test-aweme-live",
	}, hints, time.Now()); ok {
		t.Fatalf("expected live hint to fall back to generic no-media handling, got %#v", failure)
	}
	if failure, ok := rules.NoMediaFailure(map[string]string{
		"location":       "https://www.douyin.com/?recommend=1",
		"visibleAwemeID": "test-aweme-video",
	}, hints, time.Now()); ok {
		t.Fatalf("expected no live failure for normal visible media, got %#v", failure)
	}
	if failure, ok := rules.NoMediaFailure(map[string]string{
		"location":      "https://www.douyin.com/?recommend=1",
		"visibleLiveID": "test-live-room",
	}, hints, time.Now()); ok {
		t.Fatalf("expected visible live room id to fall back to generic no-media handling, got %#v", failure)
	}
}

func TestResourceDouyinCurrentAwemeIDUsesRecommendVisibleID(t *testing.T) {
	t.Parallel()

	got := resourceDouyinCurrentAwemeID(map[string]string{
		"location":       "https://www.douyin.com/?recommend=1",
		"visibleAwemeID": "test-aweme-live",
	})
	if got != "test-aweme-live" {
		t.Fatalf("expected recommend visible aweme id, got %q", got)
	}
}

func TestResourceDouyinRecommendLoginCookieDetection(t *testing.T) {
	t.Parallel()

	now := time.Now()
	pageURL := "https://www.douyin.com/?recommend=1"
	if resourceDouyinCookieRecordsLoggedIn(nil, pageURL, now) {
		t.Fatal("expected empty cookies to be treated as logged out")
	}
	if resourceDouyinCookieRecordsLoggedIn([]appcookies.Record{{
		Name:   "ttwid",
		Value:  "anonymous",
		Domain: ".douyin.com",
		Path:   "/",
	}}, pageURL, now) {
		t.Fatal("expected anonymous douyin cookies not to satisfy login")
	}
	if resourceDouyinCookieRecordsLoggedIn([]appcookies.Record{{
		Name:    "sessionid",
		Value:   "expired",
		Domain:  ".douyin.com",
		Path:    "/",
		Expires: now.Add(-time.Minute).Unix(),
	}}, pageURL, now) {
		t.Fatal("expected expired login cookie to be ignored")
	}
	if !resourceDouyinCookieRecordsLoggedIn([]appcookies.Record{{
		Name:    "sessionid",
		Value:   "active-session",
		Domain:  ".douyin.com",
		Path:    "/",
		Expires: now.Add(time.Hour).Unix(),
	}}, pageURL, now) {
		t.Fatal("expected active douyin session cookie to satisfy login")
	}
	if !resourceDouyinCookieRecordsLoggedIn([]appcookies.Record{{
		Name:   "sid_ucp_sso_v1",
		Value:  "active-sso-session",
		Domain: ".douyin.com",
		Path:   "/",
	}}, pageURL, now) {
		t.Fatal("expected douyin sso session cookie to satisfy login")
	}
}

func TestResourceXiaohongshuLoginCookieDetection(t *testing.T) {
	t.Parallel()

	now := time.Now()
	pageURL := "https://www.xiaohongshu.com/explore/test-note-id"
	if resourceXiaohongshuCookieRecordsLoggedIn([]appcookies.Record{{
		Name:   "a1",
		Value:  "anonymous",
		Domain: ".xiaohongshu.com",
		Path:   "/",
	}}, pageURL, now) {
		t.Fatal("expected anonymous xiaohongshu cookies not to satisfy login")
	}
	if resourceXiaohongshuCookieRecordsLoggedIn([]appcookies.Record{{
		Name:    "web_session",
		Value:   "expired",
		Domain:  ".xiaohongshu.com",
		Path:    "/",
		Expires: now.Add(-time.Minute).Unix(),
	}}, pageURL, now) {
		t.Fatal("expected expired xiaohongshu login cookie to be ignored")
	}
	if !resourceXiaohongshuCookieRecordsLoggedIn([]appcookies.Record{{
		Name:    "web_session",
		Value:   "active-session",
		Domain:  ".xiaohongshu.com",
		Path:    "/",
		Expires: now.Add(time.Hour).Unix(),
	}}, pageURL, now) {
		t.Fatal("expected active xiaohongshu session cookie to satisfy login")
	}
}

func TestResourceDouyinRecommendLoginFailureOnlyAppliesToRecommendPage(t *testing.T) {
	t.Parallel()

	if failure, ok := resourceDouyinRecommendLoginFailure(context.Background(), "https://www.douyin.com/?recommend=1", nil); !ok || failure.Code != resourceSniffFailureDouyinRecommendLogin {
		t.Fatalf("expected recommend page login failure, got ok=%v failure=%#v", ok, failure)
	}
	if _, ok := resourceDouyinRecommendLoginFailure(context.Background(), "https://www.douyin.com/video/123", nil); ok {
		t.Fatal("expected normal douyin video page not to require recommend login gate")
	}
	if _, ok := resourceDouyinRecommendLoginFailure(context.Background(), "https://www.xiaohongshu.com/explore/123", nil); ok {
		t.Fatal("expected non-douyin page not to require recommend login gate")
	}
}

func TestResourceDouyinPageMetaScriptDoesNotParseAPIFromDOM(t *testing.T) {
	t.Parallel()

	script := resourceDouyinPageMetaScript()
	for _, forbidden := range []string{
		"fetch(",
		"RENDER_DATA",
		"SIGI_STATE",
		"__UNIVERSAL_DATA_FOR_REHYDRATION__",
		"document.body",
		"innerText",
	} {
		if strings.Contains(script, forbidden) {
			t.Fatalf("expected douyin page meta script not to contain %q", forbidden)
		}
	}
}

func TestResourceDouyinRecommendDoesNotTreatUnanchoredLiveHintAsCurrent(t *testing.T) {
	t.Parallel()

	failure, ok := resourceNoMediaHintFailure(resourceDouyinSiteRules{}, map[string]string{
		"location": "https://www.douyin.com/?recommend=1",
	}, []resourceNoMediaHint{{
		Kind:   resourceDouyinLiveHintKind,
		ID:     "test-aweme-live",
		AltIDs: []string{"test-live-room"},
	}}, time.Now())

	if ok {
		t.Fatalf("expected recommend page not to use unanchored live hint, got %#v", failure)
	}
}

func TestResourceDouyinLivePageFallsBackToGenericNoMedia(t *testing.T) {
	t.Parallel()

	failure, ok := resourceNoMediaHintFailure(resourceDouyinSiteRules{}, map[string]string{
		"location": "https://live.douyin.com/test-live-room",
	}, nil, time.Now())

	if ok {
		t.Fatalf("expected douyin live page to fall back to generic no-media handling, got %#v", failure)
	}
}

func TestResourceDouyinRecommendLiveHintFallsBackToGenericNoMedia(t *testing.T) {
	t.Parallel()

	rules := resourceDouyinSiteRules{}
	hints := rules.ExtractNoMediaHintsFromResponse(resourceAPIResponse{
		URL:     "https://www.douyin.com/aweme/v1/web/tab/feed/?count=10&recommend=1",
		PageURL: "https://www.douyin.com/?recommend=1",
		Body: []byte(`{
			"aweme_list": [{
				"aweme_id": "test-aweme-live",
				"aweme_type": 101,
				"cell_room": {
					"room_id": "test-live-room",
					"title": "直播间标题"
				}
			}]
		}`),
		SeenAt: time.Now(),
	})
	if len(hints) != 1 || len(hints[0].AltIDs) != 1 || hints[0].AltIDs[0] != "test-live-room" {
		t.Fatalf("expected live hint with room id, got %#v", hints)
	}

	failure, ok := rules.NoMediaFailure(map[string]string{
		"location":       "https://www.douyin.com/?recommend=1",
		"visibleLiveIDs": `["test-live-room"]`,
	}, hints, time.Now())
	if ok {
		t.Fatalf("expected visible live room id to fall back to generic no-media handling, got %#v", failure)
	}
}

func TestResourceDouyinLVDetailReturnsSpecificNoMediaFailure(t *testing.T) {
	t.Parallel()

	failure, ok := resourceNoMediaHintFailure(resourceDouyinSiteRules{}, map[string]string{
		"location": "https://www.douyin.com/lvdetail/test-lvdetail?previous_page_enter_method=live_cover",
	}, nil, time.Now())

	if !ok || failure.Code != resourceSniffFailureUnsupportedDouyinLVDetail {
		t.Fatalf("expected lvdetail no-media failure, got ok=%v failure=%#v", ok, failure)
	}
}

func TestDouyinRecommendSelectsStructuredMediaByVisibleAwemeID(t *testing.T) {
	t.Parallel()

	now := time.Now()
	rules := resourceDouyinSiteRules{}
	mediaItems := []resourceStructuredMedia{
		{
			ID:            "previous",
			VideoURL:      "https://v95-hzyy-thr-daily-web.douyinvod.com/previous.mp4",
			Title:         "Previous API Title",
			Author:        "Previous Author",
			VCodec:        "h264",
			QualityHeight: 720,
			Width:         1280,
			Height:        720,
			SeenAt:        now.Add(-time.Minute),
		},
		{
			ID:            "test-aweme-video",
			VideoURL:      "https://v95-hzyy-thr-daily-web.douyinvod.com/current.mp4?token=1",
			Title:         "Current API Title",
			Author:        "Current Author",
			ThumbnailURL:  "http://p3-sign.douyinpic.com/current.jpeg",
			VCodec:        "h264",
			QualityHeight: 1080,
			Width:         1920,
			Height:        1080,
			SizeBytes:     2234567,
			SeenAt:        now,
		},
	}
	pageMeta := rules.EnrichPageMeta(map[string]string{
		"location":        "https://www.douyin.com/?recommend=1",
		"visibleAwemeID":  "test-aweme-video",
		"visibleAwemeIDs": `["test-aweme-video"]`,
		"videoWidth":      "1920",
		"videoHeight":     "1080",
		"videoItems":      `[{"currentSrc":"blob:https://www.douyin.com/current","visibleArea":1}]`,
	}, mediaItems)

	if pageMeta["awemeID"] != "test-aweme-video" {
		t.Fatalf("expected visible API item to enrich awemeID, got %q", pageMeta["awemeID"])
	}
	candidate, ok := rules.SelectCandidate(nil, pageMeta, now)
	if !ok {
		t.Fatal("expected API candidate for visible recommend item")
	}
	if candidate.url != "https://v95-hzyy-thr-daily-web.douyinvod.com/current.mp4?token=1" {
		t.Fatalf("expected visible API candidate, got %q", candidate.url)
	}
	mediaOptions := rules.MediaOptionsFromStructured("https://www.douyin.com/?recommend=1", "douyin.com", pageMeta, mediaItems)
	if len(mediaOptions) == 0 || mediaOptions[0].Title != "Current API Title" || mediaOptions[0].Author != "Current Author" {
		t.Fatalf("expected visible API metadata, got %#v", mediaOptions)
	}
}

func TestDouyinRecommendDoesNotFallbackFromVisibleLiveToAdjacentMedia(t *testing.T) {
	t.Parallel()

	rules := resourceDouyinSiteRules{}
	pageMeta := rules.EnrichPageMeta(map[string]string{
		"location":        "https://www.douyin.com/?recommend=1",
		"visibleAwemeID":  "test-aweme-live",
		"visibleAwemeIDs": `["test-aweme-live","test-aweme-video"]`,
		"videoWidth":      "480",
		"videoHeight":     "852",
		"videoItems":      `[{"currentSrc":"blob:https://www.douyin.com/live","visibleArea":1}]`,
	}, []resourceStructuredMedia{{
		ID:       "test-aweme-video",
		VideoURL: "https://v95-hzyy-thr-daily-web.douyinvod.com/adjacent.mp4",
		Title:    "Adjacent API Title",
		Author:   "Adjacent Author",
		Width:    1920,
		Height:   1080,
	}})

	if pageMeta["apiVideoURL"] != "" {
		t.Fatalf("expected visible live item not to enrich adjacent API media, got %q", pageMeta["apiVideoURL"])
	}
	if options := rules.MediaOptionsFromStructured("https://www.douyin.com/?recommend=1", "douyin.com", pageMeta, []resourceStructuredMedia{{
		ID:       "test-aweme-video",
		VideoURL: "https://v95-hzyy-thr-daily-web.douyinvod.com/adjacent.mp4",
	}}); len(options) != 0 {
		t.Fatalf("expected visible live item not to select adjacent media, got %#v", options)
	}
	if candidate, ok := rules.SelectCandidate([]resourceCandidate{{
		url:      "https://v95-hzyy-thr-daily-web.douyinvod.com/adjacent.mp4",
		mimeType: "video/mp4",
		score:    120,
	}}, mergePageMeta(pageMeta, map[string]string{
		"resourceVideoItems": `[{"name":"https://v95-hzyy-thr-daily-web.douyinvod.com/adjacent.mp4"}]`,
	}), time.Now()); ok {
		t.Fatalf("expected visible live item not to select adjacent candidate, got %#v", candidate)
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

func testDouyinAPIJSON() string {
	return `{
		"aweme_detail": {
			"aweme_id": "test-aweme-api",
			"desc": "API Title #tag",
			"share_url": "https://www.douyin.com/video/test-aweme-api",
			"author": {
				"nickname": "Fixture Author"
			},
			"video": {
				"width": 1080,
				"height": 1920,
				"play_addr_h264": {
					"url_list": ["https://v3-dy-o.zjcdn.com/current-h264.mp4?token=1"],
					"data_size": 2234567
				},
				"play_addr": {
					"url_list": ["https://v3-dy-o.zjcdn.com/current.mp4?token=1"],
					"data_size": 1234567
				},
				"cover": {
					"url_list": ["http://p3-sign.douyinpic.com/cover.jpeg"]
				}
			}
		}
	}`
}
