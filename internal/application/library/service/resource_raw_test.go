package service

import (
	"context"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/chromedp/cdproto/network"

	"xiadown/internal/application/library/dto"
)

func TestResourceSniffRawKindAndDownloadable(t *testing.T) {
	tests := []struct {
		name         string
		resource     dto.ResourceSniffRawResource
		expectedKind string
		downloadable bool
	}{
		{
			name: "m3u8 live",
			resource: dto.ResourceSniffRawResource{
				URL:         "https://media.example/video/index.m3u8",
				ContentType: "application/vnd.apple.mpegurl",
			},
			expectedKind: "live",
		},
		{
			name: "dash live",
			resource: dto.ResourceSniffRawResource{
				URL:         "https://media.example/video/index.mpd",
				ContentType: "application/dash+xml",
			},
			expectedKind: "live",
		},
		{
			name: "flv live",
			resource: dto.ResourceSniffRawResource{
				URL:         "https://media.example/live.flv",
				ContentType: "video/x-flv",
			},
			expectedKind: "live",
		},
		{
			name: "finite flv video",
			resource: dto.ResourceSniffRawResource{
				URL:         "https://media.example/replay.flv",
				ContentType: "video/x-flv",
				SizeBytes:   8 * 1024 * 1024,
			},
			expectedKind: "video",
			downloadable: true,
		},
		{
			name: "iso media segment",
			resource: dto.ResourceSniffRawResource{
				URL:         "https://media.example/live/00001.m4s",
				ContentType: "video/iso.segment",
			},
			expectedKind: "segment",
		},
		{
			name: "hls mpeg ts media segment",
			resource: dto.ResourceSniffRawResource{
				URL:          "https://media.example/live/00001.ts",
				ContentType:  "video/mp2t",
				ResourceType: "Media",
			},
			expectedKind: "segment",
		},
		{
			name: "hls explicit mp2t extension",
			resource: dto.ResourceSniffRawResource{
				URL:         "https://media.example/live/chunk-42.mp2t",
				ContentType: "application/octet-stream",
			},
			expectedKind: "segment",
		},
		{
			name: "cmaf video segment",
			resource: dto.ResourceSniffRawResource{
				URL:         "https://media.example/live/chunk-42.cmfv",
				ContentType: "video/mp4",
			},
			expectedKind: "segment",
		},
		{
			name: "hds media segment",
			resource: dto.ResourceSniffRawResource{
				URL:         "https://media.example/live/Seg1-Frag2.f4f",
				ContentType: "video/mp4",
			},
			expectedKind: "segment",
		},
		{
			name: "smooth streaming media fragment",
			resource: dto.ResourceSniffRawResource{
				URL:         "https://media.example/live/video.ism/QualityLevels(1000000)/Fragments(video=123456)",
				ContentType: "video/mp4",
			},
			expectedKind: "segment",
		},
		{
			name: "hds manifest",
			resource: dto.ResourceSniffRawResource{
				URL:         "https://media.example/live/manifest.f4m",
				ContentType: "application/f4m+xml",
			},
			expectedKind: "live",
		},
		{
			name: "smooth streaming manifest",
			resource: dto.ResourceSniffRawResource{
				URL:         "https://media.example/live/video.ism/Manifest",
				ContentType: "application/vnd.ms-sstr+xml",
			},
			expectedKind: "live",
		},
		{
			name: "typescript script is not media segment",
			resource: dto.ResourceSniffRawResource{
				URL:          "https://app.example/src/main.ts",
				ContentType:  "text/javascript",
				ResourceType: "Script",
			},
			expectedKind: "other",
			downloadable: true,
		},
		{
			name: "log url with encoded m3u8",
			resource: dto.ResourceSniffRawResource{
				URL:         "https://data.bilibili.com/log/web?u=https%3A%2F%2Fcdn.example%2Flive%2Findex.m3u8%3Ftoken%3D1",
				ContentType: "text/plain",
			},
			expectedKind: "other",
			downloadable: true,
		},
		{
			name: "api response",
			resource: dto.ResourceSniffRawResource{
				Source:      "api_response",
				URL:         "https://api.example/item",
				ContentType: "application/json",
			},
			expectedKind: "api",
			downloadable: true,
		},
		{
			name: "subtitle",
			resource: dto.ResourceSniffRawResource{
				Source:      "subtitle",
				URL:         "https://media.example/captions.vtt",
				ContentType: "text/vtt",
			},
			expectedKind: "subtitle",
			downloadable: true,
		},
		{
			name: "video content wins over subtitle source",
			resource: dto.ResourceSniffRawResource{
				Source:       "subtitle",
				URL:          "https://cdn.example/video?type=vtt",
				ContentType:  "video/mp4",
				ResourceType: "Media",
				SizeBytes:    8 * 1024 * 1024,
			},
			expectedKind: "video",
			downloadable: true,
		},
		{
			name: "image url wins over subtitle source without content type",
			resource: dto.ResourceSniffRawResource{
				Source: "subtitle",
				URL:    "https://p5-pro.a.yximgs.com/uhead/AB/2020/06/16/08/BMjAyMDA2MTYwODMwMjlfMTk3NDQyMjU5Ml8yX2hkMzdfODI2_s.jpg",
			},
			expectedKind: "image",
			downloadable: true,
		},
		{
			name: "network subtitle",
			resource: dto.ResourceSniffRawResource{
				URL:         "https://media.example/captions.srt",
				ContentType: "application/x-subrip",
			},
			expectedKind: "subtitle",
			downloadable: true,
		},
		{
			name: "network sbv subtitle",
			resource: dto.ResourceSniffRawResource{
				URL:         "https://media.example/captions.sbv",
				ContentType: "text/sbv",
			},
			expectedKind: "subtitle",
			downloadable: true,
		},
		{
			name: "image",
			resource: dto.ResourceSniffRawResource{
				URL:         "https://cdn.example/cover.webp",
				ContentType: "image/webp",
			},
			expectedKind: "image",
			downloadable: true,
		},
		{
			name: "data image",
			resource: dto.ResourceSniffRawResource{
				URL:         "data:image/webp;base64,UklGRg==",
				ContentType: "image/webp",
			},
			expectedKind: "image",
		},
		{
			name: "blob video",
			resource: dto.ResourceSniffRawResource{
				URL:         "blob:https://page.example/123",
				ContentType: "video/mp4",
			},
			expectedKind: "video",
		},
		{
			name: "unknown cdp media resource type",
			resource: dto.ResourceSniffRawResource{
				URL:          "https://media.example/chunk",
				ResourceType: "Media",
			},
			expectedKind: "other",
			downloadable: true,
		},
		{
			name: "document",
			resource: dto.ResourceSniffRawResource{
				URL:         "https://cdn.example/report.pdf",
				ContentType: "application/pdf",
			},
			expectedKind: "document",
			downloadable: true,
		},
		{
			name: "font",
			resource: dto.ResourceSniffRawResource{
				URL:         "https://cdn.example/font.woff2",
				ContentType: "font/woff2",
			},
			expectedKind: "font",
			downloadable: true,
		},
		{
			name: "archive",
			resource: dto.ResourceSniffRawResource{
				URL:         "https://cdn.example/archive.zip",
				ContentType: "application/zip",
			},
			expectedKind: "archive",
			downloadable: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt.resource.Kind = resourceSniffRawKind(
				tt.resource.Source,
				tt.resource.URL,
				tt.resource.MimeType,
				tt.resource.ContentType,
				tt.resource.ResourceType,
				tt.resource.SizeBytes,
			)
			if tt.resource.Kind != tt.expectedKind {
				t.Fatalf("expected kind %q, got %q", tt.expectedKind, tt.resource.Kind)
			}
			if got := resourceSniffRawDownloadable(tt.resource); got != tt.downloadable {
				t.Fatalf("expected downloadable %v, got %v", tt.downloadable, got)
			}
		})
	}
}

func TestResourceSubtitleFromResponseRejectsDeclaredMedia(t *testing.T) {
	t.Parallel()

	if _, ok := resourceSubtitleFromResponse(
		"https://cdn.example/video?type=vtt",
		"https://page.example/watch",
		"video/mp4",
		"video/mp4",
		string(network.ResourceTypeMedia),
		nil,
		time.Now(),
	); ok {
		t.Fatal("expected video response with subtitle-like query hint not to be captured as a subtitle")
	}
}

func TestResourceSniffListPolicyHidesSegments(t *testing.T) {
	t.Parallel()

	items := []resourceSniffRawResource{
		{
			ResourceSniffRawResource: dto.ResourceSniffRawResource{
				Source:      "network",
				URL:         "https://media.example/live/index.m3u8",
				ContentType: "application/vnd.apple.mpegurl",
				Kind:        "live",
			},
		},
		{
			ResourceSniffRawResource: dto.ResourceSniffRawResource{
				Source:      "network",
				URL:         "https://media.example/live/00001.m4s",
				ContentType: "video/iso.segment",
				Kind:        "segment",
			},
		},
		{
			ResourceSniffRawResource: dto.ResourceSniffRawResource{
				Source:       "network",
				URL:          "https://media.example/live/00002.ts",
				ContentType:  "video/mp2t",
				ResourceType: "Media",
				Kind:         "video",
			},
		},
		{
			ResourceSniffRawResource: dto.ResourceSniffRawResource{
				Source:      "network",
				URL:         "https://media.example/live/Seg1-Frag2.f4f",
				ContentType: "video/mp4",
				Kind:        "video",
			},
		},
	}

	got := applyResourceSniffListPolicy(items, resourceSniffListPolicy{
		scope:    "all",
		minBytes: 1,
		retain:   100,
	})
	if len(got) != 1 {
		t.Fatalf("expected only the live entry to remain, got %d: %#v", len(got), got)
	}
	if got[0].Kind != "live" {
		t.Fatalf("expected live entry to remain, got %#v", got[0].ResourceSniffRawResource)
	}
}

func TestResourceSniffHLSVODManifestIsDownloadableVideo(t *testing.T) {
	t.Parallel()

	seenAt := time.Date(2026, 5, 24, 10, 0, 0, 0, time.UTC)
	capture := &resourceCaptureState{
		observed: []resourceObservedResource{
			{
				url:          "https://media.example/replay/index.m3u8",
				pageURL:      "https://page.example/watch",
				contentType:  "application/vnd.apple.mpegurl",
				resourceType: string(network.ResourceTypeMedia),
				status:       200,
				sizeBytes:    128,
				seenAt:       seenAt,
			},
		},
		apiResponses: []resourceAPIResponse{
			{
				URL:          "https://media.example/replay/index.m3u8",
				PageURL:      "https://page.example/watch",
				ContentType:  "application/vnd.apple.mpegurl",
				Status:       200,
				ResourceType: network.ResourceTypeMedia,
				Body: []byte(`#EXTM3U
#EXT-X-TARGETDURATION:6
#EXTINF:6,
00001.ts
#EXT-X-ENDLIST
`),
				SeenAt: seenAt.Add(time.Second),
			},
		},
	}
	service := &LibraryService{}
	resources := service.listResourceSniffRawResources(&resourceSniffSession{
		Tabs: map[string]*resourceSniffTab{
			"target-1": {TargetID: "target-1", Capture: capture},
		},
	})
	if len(resources) != 1 {
		t.Fatalf("expected deduped hls vod resource, got %d: %#v", len(resources), resources)
	}
	got := resources[0].ResourceSniffRawResource
	if got.Kind != "video" || !got.Downloadable || got.Source != "network" {
		t.Fatalf("expected downloadable network video for hls vod, got %#v", got)
	}
}

func TestResourceSniffHLSLiveManifestRemainsLiveAndNotDownloadable(t *testing.T) {
	t.Parallel()

	items := rawResourcesFromAPIResponses("target-1", []resourceAPIResponse{
		{
			URL:          "https://media.example/live/index.m3u8",
			PageURL:      "https://page.example/live",
			ContentType:  "application/vnd.apple.mpegurl",
			Status:       200,
			ResourceType: network.ResourceTypeMedia,
			Body: []byte(`#EXTM3U
#EXT-X-TARGETDURATION:6
#EXTINF:6,
00042.ts
`),
			SeenAt: time.Date(2026, 5, 24, 10, 0, 0, 0, time.UTC),
		},
	})
	if len(items) != 1 {
		t.Fatalf("expected one raw hls resource, got %d", len(items))
	}
	got := items[0].ResourceSniffRawResource
	if got.Kind != "live" || got.Downloadable {
		t.Fatalf("expected live hls manifest to remain preview-only, got %#v", got)
	}
}

func TestResourceSniffHLSManifestCaptureUpgradesEventToReplay(t *testing.T) {
	t.Parallel()

	state := newResourceCaptureState()
	state.recordAPIResponse(resourceAPIResponse{
		URL:         "https://media.example/event/index.m3u8",
		PageURL:     "https://page.example/live",
		ContentType: "application/vnd.apple.mpegurl",
		Body: []byte(`#EXTM3U
#EXT-X-PLAYLIST-TYPE:EVENT
#EXTINF:6,
00001.ts
`),
		SeenAt: time.Date(2026, 5, 24, 10, 0, 0, 0, time.UTC),
	})
	state.recordAPIResponse(resourceAPIResponse{
		URL:         "https://media.example/event/index.m3u8",
		PageURL:     "https://page.example/live",
		ContentType: "application/vnd.apple.mpegurl",
		Body: []byte(`#EXTM3U
#EXT-X-PLAYLIST-TYPE:EVENT
#EXTINF:6,
00001.ts
#EXT-X-ENDLIST
`),
		SeenAt: time.Date(2026, 5, 24, 10, 1, 0, 0, time.UTC),
	})
	items := rawResourcesFromAPIResponses("target-1", state.apiResponsesSnapshot())
	if len(items) != 1 {
		t.Fatalf("expected one captured hls manifest, got %d", len(items))
	}
	got := items[0].ResourceSniffRawResource
	if got.Kind != "video" || !got.Downloadable {
		t.Fatalf("expected ended event manifest to upgrade to downloadable video, got %#v", got)
	}
}

func TestResourceSniffRawDownloadableIsKindCaseInsensitive(t *testing.T) {
	t.Parallel()

	if !resourceSniffRawDownloadable(dto.ResourceSniffRawResource{
		Kind: "Image",
		URL:  "https://cdn.example/cover.png",
	}) {
		t.Fatal("expected uppercase image kind to remain downloadable")
	}
	if resourceSniffRawDownloadable(dto.ResourceSniffRawResource{
		Kind:         "Video",
		URL:          "https://cdn.example/live/00001.ts",
		ContentType:  "video/mp2t",
		ResourceType: "Media",
	}) {
		t.Fatal("expected mpeg-ts media segment to be non-downloadable")
	}
}

func TestResourceSniffRawExtForImagesAndSubtitles(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		resource dto.ResourceSniffRawResource
		want     string
	}{
		{
			name: "jpeg content type",
			resource: dto.ResourceSniffRawResource{
				Kind:        "image",
				URL:         "https://cdn.example/cover",
				ContentType: "image/jpeg; charset=binary",
			},
			want: "jpg",
		},
		{
			name: "webp url extension",
			resource: dto.ResourceSniffRawResource{
				Kind: "image",
				URL:  "https://cdn.example/cover.webp?token=1",
			},
			want: "webp",
		},
		{
			name: "vtt content type",
			resource: dto.ResourceSniffRawResource{
				Kind:        "subtitle",
				URL:         "https://cdn.example/subtitle",
				ContentType: "text/vtt",
			},
			want: "vtt",
		},
		{
			name: "srt content type",
			resource: dto.ResourceSniffRawResource{
				Kind:        "subtitle",
				URL:         "https://cdn.example/subtitle",
				ContentType: "application/x-subrip",
			},
			want: "srt",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := resourceSniffRawExt(tt.resource); got != tt.want {
				t.Fatalf("expected extension %q, got %q", tt.want, got)
			}
		})
	}
}

func TestResourceSniffRawPreviewLeaseKind(t *testing.T) {
	t.Parallel()

	if got := resourceSniffPreviewLeaseKind(dto.ResourceSniffRawResource{
		Kind:        "image",
		URL:         "https://cdn.example/cover.avif",
		ContentType: "image/avif",
	}); got != "image" {
		t.Fatalf("expected avif image preview lease, got %q", got)
	}
	if got := resourceSniffPreviewLeaseKind(dto.ResourceSniffRawResource{
		Kind:        "video",
		URL:         "https://cdn.example/video.mp4?token=1",
		ContentType: "video/mp4",
	}); got != "video" {
		t.Fatalf("expected progressive video preview lease, got %q", got)
	}
	if got := resourceSniffPreviewLeaseKind(dto.ResourceSniffRawResource{
		Kind:        "live",
		URL:         "https://cdn.example/live.flv",
		ContentType: "video/x-flv",
	}); got != "flv" {
		t.Fatalf("expected flv preview lease, got %q", got)
	}
	if got := resourceSniffPreviewLeaseKind(dto.ResourceSniffRawResource{
		Kind:        "video",
		URL:         "https://cdn.example/index.m3u8",
		ContentType: "application/vnd.apple.mpegurl",
	}); got != "live" {
		t.Fatalf("expected manifest video to use live preview lease, got %q", got)
	}
	if got := resourceSniffPreviewLeaseKind(dto.ResourceSniffRawResource{
		Kind:        "live",
		URL:         "https://cdn.example/index.m3u8",
		ContentType: "application/vnd.apple.mpegurl",
	}); got != "live" {
		t.Fatalf("expected hls live preview lease, got %q", got)
	}
	if got := resourceSniffPreviewLeaseKind(dto.ResourceSniffRawResource{
		Kind:        "segment",
		URL:         "https://cdn.example/00001.m4s",
		ContentType: "video/iso.segment",
	}); got != "" {
		t.Fatalf("expected media segment to be rejected for preview, got %q", got)
	}
	if got := resourceSniffPreviewLeaseKind(dto.ResourceSniffRawResource{
		Kind:         "video",
		URL:          "https://cdn.example/00001.ts",
		ContentType:  "video/mp2t",
		ResourceType: "Media",
	}); got != "" {
		t.Fatalf("expected mpeg-ts media segment to be rejected for preview, got %q", got)
	}
}

func TestRewriteResourceSniffHLSManifestProxiesReferences(t *testing.T) {
	t.Parallel()

	request := httptest.NewRequest("GET", "http://127.0.0.1/api/sniff/resource-preview/lease-id/index.m3u8", nil)
	lease := resourceSniffPreviewLease{
		ID:       "lease-id",
		Kind:     "live",
		URL:      "https://cdn.example/live/index.m3u8?auth=1",
		FileName: "index.m3u8",
	}
	got := rewriteResourceSniffHLSManifest(
		"#EXTM3U\n#EXT-X-MAP:URI=\"init.mp4\"\nsegment-1.m4s?token=2\n",
		"https://cdn.example/live/index.m3u8?auth=1",
		request,
		lease,
	)
	if !strings.Contains(got, "proxy?") || !strings.Contains(got, "url=") {
		t.Fatalf("expected rewritten manifest to proxy references, got:\n%s", got)
	}
	if strings.Contains(got, "/api/sniff/resource-preview/lease-id/proxy") {
		t.Fatalf("expected rewritten manifest to keep proxy references relative, got:\n%s", got)
	}
	if strings.Contains(got, "\nsegment-1.m4s") || strings.Contains(got, `URI="init.mp4"`) {
		t.Fatalf("expected relative references to be rewritten, got:\n%s", got)
	}
	if !strings.Contains(got, url.QueryEscape("https://cdn.example/live/init.mp4")) {
		t.Fatalf("expected init map target to keep original URL, got:\n%s", got)
	}
	if !strings.Contains(got, url.QueryEscape("https://cdn.example/live/segment-1.m4s?token=2")) {
		t.Fatalf("expected segment to preserve its own query, got:\n%s", got)
	}
	if !strings.Contains(got, resourceSniffPreviewManifestQueryParam+"=auth%3D1") {
		t.Fatalf("expected manifest query to be carried as a fallback, got:\n%s", got)
	}
}

func TestResourceSniffPreviewServesNormalizedHLSKeyOverride(t *testing.T) {
	t.Parallel()

	keyText := "ba9bf05693b9fa202d922dd43a08f281"
	segmentBody := encryptHLSProbeFixture(t, []byte(keyText[:16]), make([]byte, 16))
	origin := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/stream.m3u8":
			w.Header().Set("Content-Type", "application/vnd.apple.mpegurl")
			_, _ = w.Write([]byte(`#EXTM3U
#EXT-X-KEY:METHOD=AES-128,URI="/key",IV=0x00000000000000000000000000000000
#EXTINF:4.0,
/segment/0
#EXT-X-ENDLIST
`))
		case "/key":
			if r.URL.Query().Get("sig") != "1" {
				http.Error(w, "missing query", http.StatusForbidden)
				return
			}
			w.Header().Set("Content-Type", "application/octet-stream")
			_, _ = w.Write([]byte(keyText))
		case "/segment/0":
			if r.URL.Query().Get("sig") != "1" {
				http.Error(w, "missing query", http.StatusForbidden)
				return
			}
			w.Header().Set("Content-Type", "application/octet-stream")
			_, _ = w.Write(segmentBody)
		default:
			http.NotFound(w, r)
		}
	}))
	defer origin.Close()

	service := &LibraryService{
		resourcePreviewLeases: map[string]resourceSniffPreviewLease{
			"lease-id": {
				ID:        "lease-id",
				Kind:      "live",
				URL:       origin.URL + "/stream.m3u8?sig=1",
				FileName:  "stream.m3u8",
				ExpiresAt: time.Now().Add(time.Hour),
			},
		},
		nowFunc: func() time.Time { return time.Now() },
	}

	manifestRequest := httptest.NewRequest("GET", "/api/sniff/resource-preview/lease-id/stream.m3u8", nil)
	manifestResponse := httptest.NewRecorder()
	service.ServeResourceSniffPreview(manifestResponse, manifestRequest)

	if manifestResponse.Code != http.StatusOK {
		t.Fatalf("expected manifest preview status 200, got %d: %s", manifestResponse.Code, manifestResponse.Body.String())
	}
	if got := manifestResponse.Body.String(); !strings.Contains(got, "proxy?") || !strings.Contains(got, "url=") {
		t.Fatalf("expected HLS references to be proxied, got:\n%s", got)
	}

	keyRequest := httptest.NewRequest(
		"GET",
		"/api/sniff/resource-preview/lease-id/proxy?url="+url.QueryEscape(origin.URL+"/key")+"&manifest_query="+url.QueryEscape("sig=1"),
		nil,
	)
	keyResponse := httptest.NewRecorder()
	service.ServeResourceSniffPreview(keyResponse, keyRequest)

	if keyResponse.Code != http.StatusOK {
		t.Fatalf("expected key preview status 200, got %d: %s", keyResponse.Code, keyResponse.Body.String())
	}
	if got := keyResponse.Body.String(); got != keyText[:16] {
		t.Fatalf("expected preview key to use normalized first 16 bytes, got %q", got)
	}
}

func TestResourceSniffPreviewRetriesProxyTargetWithManifestQuery(t *testing.T) {
	t.Parallel()

	origin := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/segment/0":
			if r.URL.Query().Get("sig") != "1" {
				http.Error(w, "missing query", http.StatusForbidden)
				return
			}
			w.Header().Set("Content-Type", "video/mp2t")
			_, _ = w.Write([]byte("segment"))
		default:
			http.NotFound(w, r)
		}
	}))
	defer origin.Close()

	service := &LibraryService{
		resourcePreviewLeases: map[string]resourceSniffPreviewLease{
			"lease-id": {
				ID:        "lease-id",
				Kind:      "live",
				URL:       origin.URL + "/stream.m3u8?sig=1",
				FileName:  "stream.m3u8",
				ExpiresAt: time.Now().Add(time.Hour),
			},
		},
		nowFunc: func() time.Time { return time.Now() },
	}
	request := httptest.NewRequest(
		"GET",
		"/api/sniff/resource-preview/lease-id/proxy?url="+url.QueryEscape(origin.URL+"/segment/0")+"&manifest_query="+url.QueryEscape("sig=1"),
		nil,
	)
	response := httptest.NewRecorder()

	service.ServeResourceSniffPreview(response, request)

	if response.Code != http.StatusOK {
		t.Fatalf("expected retry with manifest query to succeed, got %d: %s", response.Code, response.Body.String())
	}
	if got := response.Body.String(); got != "segment" {
		t.Fatalf("expected segment body, got %q", got)
	}
}

func TestResourceMediaFromRawResourceUsesSniffExtractor(t *testing.T) {
	media := resourceMediaFromRawResource(resourceSniffRawResource{
		ResourceSniffRawResource: dto.ResourceSniffRawResource{
			URL:         "https://media.example/video.mp4?token=1",
			PageURL:     "https://page.example/watch/1",
			Domain:      "media.example",
			Kind:        "video",
			ContentType: "video/mp4",
			MimeType:    "video/mp4",
			SizeBytes:   1234,
		},
		headers: map[string]string{
			"Referer": "https://page.example/watch/1",
		},
	})

	if media.Extractor != "sniff" {
		t.Fatalf("expected sniff extractor, got %q", media.Extractor)
	}
	if media.PageURL != "https://page.example/watch/1" {
		t.Fatalf("expected page url to be preserved, got %q", media.PageURL)
	}
	if media.Ext != "mp4" {
		t.Fatalf("expected mp4 extension, got %q", media.Ext)
	}
	if media.Kind != "video" {
		t.Fatalf("expected raw kind to be preserved, got %q", media.Kind)
	}
	if media.RequestHeaders["Referer"] != "https://page.example/watch/1" {
		t.Fatalf("expected request headers to be preserved, got %#v", media.RequestHeaders)
	}
}

func TestResourceSniffRawResourcesExposeCaptureWithoutExtractor(t *testing.T) {
	t.Parallel()

	seenAt := time.Date(2026, 5, 21, 10, 0, 0, 0, time.UTC)
	capture := &resourceCaptureState{
		candidates: []resourceCandidate{
			{
				url:          "https://cdn.example/video-720p.mp4",
				pageURL:      "https://www.kuaishou.com/short-video/example",
				mimeType:     "video/mp4",
				contentType:  "video/mp4",
				resourceType: string(network.ResourceTypeMedia),
				status:       200,
				sizeBytes:    123456,
				headers:      map[string]string{"Referer": "https://www.kuaishou.com/short-video/example"},
				score:        80,
				seenAt:       seenAt,
			},
		},
		apiResponses: []resourceAPIResponse{
			{
				URL:          "https://www.kuaishou.com/graphql",
				PageURL:      "https://www.kuaishou.com/short-video/example",
				ContentType:  "application/json",
				Status:       200,
				ResourceType: network.ResourceTypeXHR,
				SizeBytes:    42,
				SeenAt:       seenAt.Add(time.Second),
			},
		},
		subtitles: []resourceSubtitle{
			{
				URL:         "https://cdn.example/captions.en.vtt",
				PageURL:     "https://www.kuaishou.com/short-video/example",
				Language:    "en",
				Ext:         "vtt",
				ContentType: "text/vtt",
				SeenAt:      seenAt.Add(2 * time.Second),
			},
		},
	}
	service := &LibraryService{}
	resources := service.listResourceSniffRawResources(&resourceSniffSession{
		Tabs: map[string]*resourceSniffTab{
			"target-1": {
				TargetID: "target-1",
				Capture:  capture,
			},
		},
	})

	if len(resources) != 3 {
		t.Fatalf("expected candidate, api response, and subtitle raw resources, got %d: %#v", len(resources), resources)
	}
	bySource := map[string]dto.ResourceSniffRawResource{}
	for _, resource := range resources {
		bySource[resource.Source] = resource.ResourceSniffRawResource
	}
	candidate := bySource["candidate"]
	if candidate.Kind != "video" || !candidate.Downloadable {
		t.Fatalf("expected downloadable raw video candidate, got %#v", candidate)
	}
	if candidate.Domain != "example" && candidate.Domain != "cdn.example" {
		t.Fatalf("expected candidate domain to be classified, got %#v", candidate)
	}
	if api := bySource["api_response"]; api.Kind != "api" || !api.Downloadable {
		t.Fatalf("expected downloadable raw api response, got %#v", api)
	}
	if subtitle := bySource["subtitle"]; subtitle.Kind != "subtitle" || !subtitle.Downloadable {
		t.Fatalf("expected downloadable subtitle raw resource, got %#v", subtitle)
	}
}

func TestResourceSniffImagePreviewUsesCapturedCDPBody(t *testing.T) {
	seenAt := time.Date(2026, 5, 21, 10, 0, 0, 0, time.UTC)
	body := []byte{0x89, 'P', 'N', 'G', '\r', '\n', 0x1a, '\n'}
	capture := newResourceCaptureState()
	requestID := network.RequestID("image-request-1")
	capture.recordRequest(
		requestID,
		"https://cdn.example/cover.png",
		"https://page.example/watch",
		network.Headers{"Referer": "https://page.example/watch"},
	)
	capture.recordResponse(
		requestID,
		"https://cdn.example/cover.png",
		200,
		"image/png",
		network.Headers{
			"Content-Type":   "image/png",
			"Content-Length": "8",
		},
		network.ResourceTypeImage,
	)
	capture.mu.Lock()
	request := capture.requests[requestID]
	request.seenAt = seenAt
	capture.requests[requestID] = request
	capture.mu.Unlock()
	capture.recordResponseBody(requestID, body)

	service := &LibraryService{
		resourceSniffs: map[string]*resourceSniffSession{
			"session-1": {
				ID: "session-1",
				Tabs: map[string]*resourceSniffTab{
					"target-1": {
						TargetID: "target-1",
						Capture:  capture,
					},
				},
			},
		},
	}
	resources := service.listResourceSniffRawResources(service.resourceSniffs["session-1"])
	if len(resources) != 1 {
		t.Fatalf("expected one raw image resource, got %d: %#v", len(resources), resources)
	}
	image := resources[0].ResourceSniffRawResource
	if image.Kind != "image" || !image.Downloadable {
		t.Fatalf("expected downloadable raw image resource, got %#v", image)
	}
	if !image.PreviewAvailable || image.PreviewKind != "image" || image.PreviewMimeType != "image/png" || image.PreviewSizeBytes != int64(len(body)) {
		t.Fatalf("expected image preview metadata, got %#v", image)
	}
	if image.URL != "https://cdn.example/cover.png" {
		t.Fatalf("expected image resource url to remain original, got %q", image.URL)
	}
	if image.PreviewDataBase64 != "iVBORw0KGgo=" {
		t.Fatalf("expected list image preview base64, got %q", image.PreviewDataBase64)
	}
	preview, err := service.GetResourceSniffPreview(context.Background(), dto.GetResourceSniffPreviewRequest{
		SessionID:  "session-1",
		ResourceID: image.ID,
	})
	if err != nil {
		t.Fatalf("get resource sniff preview: %v", err)
	}
	if preview.Kind != "image" || preview.MimeType != "image/png" || preview.SizeBytes != int64(len(body)) {
		t.Fatalf("expected image preview response, got %#v", preview)
	}
	if preview.DataBase64 != "iVBORw0KGgo=" {
		t.Fatalf("expected base64 encoded png header, got %q", preview.DataBase64)
	}
}

func TestResourceSniffImagePreviewCapturesImageFormatsAsBase64Candidates(t *testing.T) {
	tests := []struct {
		name     string
		url      string
		body     []byte
		mimeType string
	}{
		{
			name:     "webp",
			url:      "https://cdn.example/image.webp",
			body:     []byte{'R', 'I', 'F', 'F', 0, 0, 0, 0, 'W', 'E', 'B', 'P', 'V', 'P', '8', ' '},
			mimeType: "image/webp",
		},
		{
			name:     "avif",
			url:      "https://cdn.example/image.avif",
			body:     []byte{0, 0, 0, 24, 'f', 't', 'y', 'p', 'a', 'v', 'i', 'f', 0, 0, 0, 0},
			mimeType: "image/avif",
		},
		{
			name:     "gif",
			url:      "https://cdn.example/image.gif",
			body:     []byte{'G', 'I', 'F', '8', '9', 'a', 1, 0, 1, 0},
			mimeType: "image/gif",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			request := resourceRequest{
				url:          tt.url,
				status:       200,
				contentType:  "application/octet-stream",
				resourceType: network.ResourceTypeImage,
			}
			if !shouldCaptureResourcePreviewSnapshot(request) {
				t.Fatal("expected image format to be captured for list preview")
			}
			preview, ok := resourcePreviewSnapshotFromResponse(request, tt.body)
			if !ok {
				t.Fatal("expected preview snapshot")
			}
			if preview.MimeType != tt.mimeType {
				t.Fatalf("expected preview mime %q, got %q", tt.mimeType, preview.MimeType)
			}
			if len(preview.Body) != len(tt.body) {
				t.Fatalf("expected preview body length %d, got %d", len(tt.body), len(preview.Body))
			}
		})
	}
}

func TestClearResourceSniffResourcesClearsCaptureState(t *testing.T) {
	seenAt := time.Date(2026, 5, 21, 10, 0, 0, 0, time.UTC)
	capture := &resourceCaptureState{
		requests: map[network.RequestID]resourceRequest{
			"request-1": {
				url:    "https://cdn.example/video.mp4",
				seenAt: seenAt,
			},
		},
		candidates: []resourceCandidate{{
			url:     "https://cdn.example/video.mp4",
			pageURL: "https://page.example/watch",
			score:   80,
			seenAt:  seenAt,
		}},
		rejected: []resourceRejectedCandidate{{
			url:    "https://cdn.example/rejected.ts",
			reason: "weak_candidate",
		}},
		apiResponses: []resourceAPIResponse{{
			URL:    "https://api.example/detail",
			Body:   []byte(`{"ok":true}`),
			SeenAt: seenAt,
		}},
		subtitles: []resourceSubtitle{{
			URL:    "https://cdn.example/captions.vtt",
			Ext:    "vtt",
			SeenAt: seenAt,
		}},
	}
	service := &LibraryService{
		resourceSniffs: map[string]*resourceSniffSession{
			"session-1": {
				ID: "session-1",
				Tabs: map[string]*resourceSniffTab{
					"target-1": {
						TargetID: "target-1",
						Capture:  capture,
					},
				},
			},
		},
	}

	if got := service.listResourceSniffRawResources(service.resourceSniffs["session-1"]); len(got) == 0 {
		t.Fatal("expected resources before clearing")
	}
	if err := service.ClearResourceSniffResources(context.Background(), dto.ClearResourceSniffResourcesRequest{SessionID: "session-1"}); err != nil {
		t.Fatalf("clear resource sniff resources: %v", err)
	}
	if got := service.listResourceSniffRawResources(service.resourceSniffs["session-1"]); len(got) != 0 {
		t.Fatalf("expected resources to be cleared, got %#v", got)
	}
	capture.mu.Lock()
	defer capture.mu.Unlock()
	if len(capture.requests) != 0 {
		t.Fatalf("expected request cache to be cleared, got %#v", capture.requests)
	}
}

func TestResourceSniffSnapshotDeepClonesCaptureBoundary(t *testing.T) {
	t.Parallel()

	pageMeta := map[string]string{"title": "Before"}
	candidates := []resourceCandidate{{
		url:     "https://cdn.example/video.mp4",
		headers: map[string]string{"Referer": "https://page.example/watch"},
	}}
	rejected := []resourceRejectedCandidate{{
		url:     "https://cdn.example/rejected.ts",
		headers: map[string]string{"Range": "bytes=0-1"},
	}}
	apiResponses := []resourceAPIResponse{{
		URL:             "https://api.example/detail",
		RequestHeaders:  map[string]string{"Accept": "application/json"},
		ResponseHeaders: map[string]string{"Content-Type": "application/json"},
		Body:            []byte(`{"ok":true}`),
	}}
	subtitles := []resourceSubtitle{{
		URL:            "https://cdn.example/captions.vtt",
		Ext:            "vtt",
		RequestHeaders: map[string]string{"Referer": "https://page.example/watch"},
	}}

	snapshot := newResourceSniffSnapshot("https://page.example/watch", pageMeta, candidates, rejected, apiResponses, subtitles, time.Now())
	pageMeta["title"] = "After"
	candidates[0].headers["Referer"] = "mutated"
	rejected[0].headers["Range"] = "mutated"
	apiResponses[0].RequestHeaders["Accept"] = "mutated"
	apiResponses[0].ResponseHeaders["Content-Type"] = "mutated"
	apiResponses[0].Body[0] = '['
	subtitles[0].RequestHeaders["Referer"] = "mutated"

	if snapshot.PageMeta["title"] != "Before" {
		t.Fatalf("expected page meta to be cloned, got %#v", snapshot.PageMeta)
	}
	if snapshot.Candidates[0].headers["Referer"] != "https://page.example/watch" {
		t.Fatalf("expected candidate headers to be cloned, got %#v", snapshot.Candidates[0].headers)
	}
	if snapshot.Rejected[0].headers["Range"] != "bytes=0-1" {
		t.Fatalf("expected rejected headers to be cloned, got %#v", snapshot.Rejected[0].headers)
	}
	if snapshot.APIResponses[0].RequestHeaders["Accept"] != "application/json" ||
		snapshot.APIResponses[0].ResponseHeaders["Content-Type"] != "application/json" ||
		string(snapshot.APIResponses[0].Body) != `{"ok":true}` {
		t.Fatalf("expected api response to be cloned, got %#v body=%s", snapshot.APIResponses[0], string(snapshot.APIResponses[0].Body))
	}
	if snapshot.CapturedSubtitles[0].RequestHeaders["Referer"] != "https://page.example/watch" {
		t.Fatalf("expected subtitle headers to be cloned, got %#v", snapshot.CapturedSubtitles[0].RequestHeaders)
	}
}
