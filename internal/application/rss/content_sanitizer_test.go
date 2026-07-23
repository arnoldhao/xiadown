package rss

import (
	"fmt"
	"strings"
	"testing"
	"time"
	"unicode/utf8"

	domainrss "xiadown/internal/domain/rss"
)

func TestSanitizeEntryHTMLNormalizesRelativeURLsAndRemovesActiveContent(t *testing.T) {
	markup := `<section onclick="steal()" style="background:red">
<script>alert(1)</script><style>body{display:none}</style>
<h2 id="part-1">Heading</h2>
<a href="../next?from=rss">Next</a><a href="javascript:alert(2)">Unsafe</a>
<img src="./image.jpg" srcset="http://tracker.example/2x 2x" onerror="steal()" alt="Cover">
<img src="http://127.0.0.1/admin" alt="Local">
<img src="http://printer/admin" alt="Single label">
<img src="http://reader.local/admin" alt="mDNS">
<img src="http://service.internal/admin" alt="Internal">
<img src="http://gateway.lan/admin" alt="LAN">
<img src="http://nas.home/admin" alt="Home">
<iframe src="https://tracker.example/embed"></iframe>
<form action="https://tracker.example/"><input name="token"></form>
<video src="../media/clip.mp4" autoplay></video>
</section>`
	result := sanitizeEntryHTML(markup, "https://news.example/posts/2026/item")

	for _, forbidden := range []string{
		"<script", "<style", "<iframe", "<form", "<input", "onclick", "onerror",
		"javascript:", "127.0.0.1", "http://printer", "reader.local", "service.internal",
		"gateway.lan", "nas.home", "srcset", "autoplay", "tracker.example",
	} {
		if strings.Contains(strings.ToLower(result), strings.ToLower(forbidden)) {
			t.Fatalf("sanitized HTML retained %q: %s", forbidden, result)
		}
	}
	for _, expected := range []string{
		`href="https://news.example/posts/next?from=rss"`,
		`src="https://news.example/posts/2026/image.jpg"`,
		`src="https://news.example/posts/media/clip.mp4"`,
		`rel="noopener noreferrer"`, `referrerpolicy="no-referrer"`,
		`loading="lazy"`, `controls=""`, `id="part-1"`,
	} {
		if !strings.Contains(result, expected) {
			t.Fatalf("sanitized HTML missing %q: %s", expected, result)
		}
	}
}

func TestSanitizeEntryHTMLKeepsOnlyCanonicalVideoEmbedMarkers(t *testing.T) {
	result := sanitizeEntryHTML(`<p>Before</p>
<iframe title="YouTube" width="560" height="315" src="https://www.youtube.com/embed/AbCdEfGhI12?autoplay=1"></iframe>
<iframe src="https://player.vimeo.com/video/123456?tracking=1"></iframe>
<iframe src="https://player.bilibili.com/player.html?bvid=BV1xx411c7mD&page=2"></iframe>
<iframe src="https://widgets.example.com/embed/123"></iframe>
<figure data-xiadown-rss-video-provider="youtube" data-xiadown-rss-video-id="invalid"></figure>
<p>After</p>`, "https://example.com/posts/1")

	for _, expected := range []string{
		`data-xiadown-rss-video-provider="youtube" data-xiadown-rss-video-id="AbCdEfGhI12"`,
		`data-xiadown-rss-video-width="560" data-xiadown-rss-video-height="315"`,
		`data-xiadown-rss-video-provider="vimeo" data-xiadown-rss-video-id="123456"`,
		`data-xiadown-rss-video-provider="bilibili" data-xiadown-rss-video-id="BV1xx411c7mD"`,
		`<p>Before</p>`, `<p>After</p>`,
	} {
		if !strings.Contains(result, expected) {
			t.Fatalf("sanitized video embeds missing %q: %s", expected, result)
		}
	}
	for _, forbidden := range []string{"<iframe", "widgets.example.com", "autoplay", "tracking=", `video-id="invalid"`} {
		if strings.Contains(result, forbidden) {
			t.Fatalf("sanitized video embeds retained %q: %s", forbidden, result)
		}
	}
}

func TestRSSImageExtractionRecoversLazyAndSrcsetCandidates(t *testing.T) {
	markup := `<figure>
		<img src="data:image/gif;base64,R0lGODlhAQABAIAAAAAAAP" data-original="../media/original.jpg" alt="Original">
		<img data-lazy-src="/media/lazy.webp" alt="Lazy">
		<img data-srcset="/media/small.jpg 1x, /media/large.jpg 2x" alt="Srcset">
	</figure>`
	baseURL := "https://news.example/posts/2026/item"
	sanitized := sanitizeEntryHTML(markup, baseURL)
	for _, expected := range []string{
		`src="https://news.example/posts/media/original.jpg"`,
		`src="https://news.example/media/lazy.webp"`,
		`src="https://news.example/media/small.jpg"`,
	} {
		if !strings.Contains(sanitized, expected) {
			t.Fatalf("sanitized lazy image missing %q: %s", expected, sanitized)
		}
	}
	for _, forbidden := range []string{"data-original", "data-lazy-src", "data-srcset", "base64"} {
		if strings.Contains(sanitized, forbidden) {
			t.Fatalf("sanitized lazy image retained %q: %s", forbidden, sanitized)
		}
	}

	media := extractHTMLMedia(markup, baseURL)
	if len(media) != 3 ||
		media[0].URL != "https://news.example/posts/media/original.jpg" ||
		media[1].URL != "https://news.example/media/lazy.webp" ||
		media[2].URL != "https://news.example/media/small.jpg" {
		t.Fatalf("lazy image media = %#v", media)
	}
}

func TestRSSImageCandidatesSkipUnsafeSourcesBeforeValidSrcsetFallbacks(t *testing.T) {
	markup := `<picture>
		<source src="data:image/avif;base64,AAAA" srcset="data:image/avif;base64,BBBB 1x, //cdn.example.com/cover.avif 2x" type="image/avif">
		<source src="http://127.0.0.1/private.jpg" data-srcset="http://printer/private.jpg 1x, ../media/cover.webp 2x" type="image/webp">
		<img src="data:image/gif;base64,R0lGODlhAQABAIAAAAAAAP" srcset="http://localhost/private.jpg 1x, ../media/cover.jpg 2x" alt="Cover">
	</picture>`
	baseURL := "https://news.example/posts/2026/item"
	sanitized := sanitizeEntryHTML(markup, baseURL)

	for _, expected := range []string{
		`src="https://cdn.example.com/cover.avif"`,
		`src="https://news.example/posts/media/cover.webp"`,
		`src="https://news.example/posts/media/cover.jpg"`,
	} {
		if !strings.Contains(sanitized, expected) {
			t.Fatalf("sanitized picture missing %q: %s", expected, sanitized)
		}
	}
	for _, forbidden := range []string{
		"data:image", "base64", "127.0.0.1", "localhost", "http://printer", "srcset", "data-srcset",
		"https://news.example/posts/BBBB",
	} {
		if strings.Contains(sanitized, forbidden) {
			t.Fatalf("sanitized picture retained or synthesized %q: %s", forbidden, sanitized)
		}
	}

	media := extractHTMLMedia(markup, baseURL)
	want := []string{
		"https://cdn.example.com/cover.avif",
		"https://news.example/posts/media/cover.webp",
		"https://news.example/posts/media/cover.jpg",
	}
	if len(media) != len(want) {
		t.Fatalf("picture media = %#v, want %d items", media, len(want))
	}
	for index, item := range media {
		if item.URL != want[index] || item.Kind != "image" {
			t.Fatalf("picture media[%d] = %#v, want image %q", index, item, want[index])
		}
	}
}

func TestParseRSSSrcsetCandidatesDoesNotSplitDataURLCommasIntoRelativeURLs(t *testing.T) {
	values := map[string]string{
		"src":         "data:image/svg+xml,%3Csvg%20xmlns='http://www.w3.org/2000/svg'%3E,%3C/svg%3E",
		"srcset":      "data:image/png;base64,iVBORw0KGgoAAAANSUhEUgAAAAEAAAAB 1x, /images/valid.png 2x",
		"data-srcset": "data:image/gif;base64,R0lGODlhAQABAIAAAAAAAP, /images/fallback.gif 3x",
	}
	candidates := rssImageCandidates(values)
	if len(candidates) != 5 {
		t.Fatalf("candidates = %#v", candidates)
	}
	if candidates[1] != "data:image/png;base64,iVBORw0KGgoAAAANSUhEUgAAAAEAAAAB" ||
		candidates[2] != "/images/valid.png" ||
		candidates[3] != "data:image/gif;base64,R0lGODlhAQABAIAAAAAAAP" ||
		candidates[4] != "/images/fallback.gif" {
		t.Fatalf("srcset candidates = %#v", candidates)
	}
	if got := firstSafeRSSImageCandidate("https://example.com/post", values); got != "https://example.com/images/valid.png" {
		t.Fatalf("safe candidate = %q, want valid srcset URL", got)
	}
}

func TestEntriesFromFeedPersistsOnlySanitizedPublicMedia(t *testing.T) {
	now := time.Date(2026, 7, 13, 12, 0, 0, 0, time.UTC)
	items := entriesFromFeed("subscription-1", domainrss.ViewTypeAuto, parsedFeed{
		SiteURL: "https://example.com/root/",
		Entries: []parsedEntry{{
			ExternalID: "entry-1", URL: "posts/1", Title: "Post",
			Content: `<p>Hello<img src="../media/cover.jpg"><script>bad()</script></p>`,
			Media: []parsedMedia{
				{URL: "https://cdn.example.com/video.mp4", MIMEType: "video/mp4"},
				{URL: "http://192.168.1.5/private.mp4", MIMEType: "video/mp4"},
			},
		}},
	}, now)
	if len(items) != 1 {
		t.Fatalf("entries = %#v", items)
	}
	item := items[0]
	if item.URL != "https://example.com/root/posts/1" || strings.Contains(item.ContentHTML, "script") ||
		!strings.Contains(item.ContentHTML, `src="https://example.com/root/media/cover.jpg"`) {
		t.Fatalf("sanitized entry = %#v", item)
	}
	for _, media := range item.Media {
		if strings.Contains(media.URL, "192.168.1.5") {
			t.Fatalf("private media URL survived: %#v", item.Media)
		}
	}
	if item.MediaURL != "https://cdn.example.com/video.mp4" || len(item.ImageURLs) != 1 ||
		item.ImageURLs[0] != "https://example.com/root/media/cover.jpg" {
		t.Fatalf("public media projection = %#v", item)
	}
}

func TestEntryHTMLProcessingFailsClosedWithinDepthNodeAndByteBudgets(t *testing.T) {
	started := time.Now()
	deep := strings.Repeat("<div>", maxRSSEntryHTMLDepth+8) +
		`Deep text<iframe src="https://www.youtube.com/embed/AbCdEfGhI12"></iframe>` +
		strings.Repeat("</div>", maxRSSEntryHTMLDepth+8)
	deepResult := sanitizeEntryHTML(deep, "https://example.com/posts/1")
	if len(deepResult) > maxRSSEntryContentHTMLBytes || !utf8.ValidString(deepResult) ||
		strings.Contains(deepResult, "<iframe") || !strings.Contains(deepResult, "Deep text") {
		t.Fatalf("deep fallback is not bounded inert text: %d bytes %q", len(deepResult), deepResult)
	}
	if media := extractHTMLMedia(deep, "https://example.com/posts/1"); len(media) != 0 {
		t.Fatalf("over-depth media extraction did not fail closed: %#v", media)
	}

	var wide strings.Builder
	for index := 0; index < maxRSSEntryHTMLNodes+8; index++ {
		_, _ = fmt.Fprintf(&wide, "<span>%d</span>", index)
	}
	wideResult := sanitizeEntryHTML(wide.String(), "https://example.com/posts/1")
	if len(wideResult) > maxRSSEntryContentHTMLBytes || !utf8.ValidString(wideResult) {
		t.Fatalf("wide fallback = %d bytes, utf8=%v", len(wideResult), utf8.ValidString(wideResult))
	}

	expandedResult := sanitizeEntryHTML(strings.Repeat("&", maxRSSEntryContentHTMLBytes), "https://example.com/posts/1")
	if len(expandedResult) > maxRSSEntryContentHTMLBytes || !utf8.ValidString(expandedResult) {
		t.Fatalf("expanded HTML = %d bytes, utf8=%v", len(expandedResult), utf8.ValidString(expandedResult))
	}
	if elapsed := time.Since(started); elapsed > 10*time.Second {
		t.Fatalf("bounded HTML processing took %s", elapsed)
	}
}

func TestEntriesFromFeedBoundsDurableFieldsResourcesAndExternalIdentity(t *testing.T) {
	longURL := "https://cdn.example/" + strings.Repeat("x", maxRSSDurableURLBytes)
	var content strings.Builder
	_, _ = fmt.Fprintf(&content, `<a href="%s">Long link</a><img src="%s">`, longURL, longURL)
	media := make([]parsedMedia, 0, maxRSSParsedEntryMediaItems+8)
	media = append(media, parsedMedia{URL: longURL, MIMEType: "video/mp4"})
	for index := 0; index < maxRSSParsedEntryMediaItems+8; index++ {
		media = append(media, parsedMedia{
			URL: "https://cdn.example/" + fmt.Sprint(index) + ".jpg", MIMEType: "image/jpeg",
			Width: maxRSSMediaDimension + 1, Height: -1, Duration: maxRSSMediaDurationMillis + 1,
		})
	}
	sharedPrefix := strings.Repeat("i", maxRSSEntryExternalIDBytes)
	feed := parsedFeed{SiteURL: "https://example.com/", Entries: []parsedEntry{
		{
			ExternalID: sharedPrefix + "-one", URL: longURL,
			Title: strings.Repeat("界", maxRSSEntryTitleBytes), Author: strings.Repeat("作", maxRSSEntryAuthorBytes),
			Summary: strings.Repeat("摘", maxRSSEntrySummaryBytes), Content: content.String(), Media: media,
		},
		{ExternalID: sharedPrefix + "-two", Title: "second"},
	}}
	entries := entriesFromFeed("subscription-1", domainrss.ViewTypeAuto, feed, time.Now())
	if len(entries) != 2 || entries[0].ID == entries[1].ID || entries[0].ExternalID == entries[1].ExternalID {
		t.Fatalf("bounded external IDs collided: %#v", entries)
	}
	first := entries[0]
	for name, value := range map[string]string{
		"externalId":  first.ExternalID,
		"title":       first.Title,
		"author":      first.Author,
		"summary":     first.Summary,
		"contentHtml": first.ContentHTML,
	} {
		limit := map[string]int{
			"externalId":  maxRSSEntryExternalIDBytes,
			"title":       maxRSSEntryTitleBytes,
			"author":      maxRSSEntryAuthorBytes,
			"summary":     maxRSSEntrySummaryBytes,
			"contentHtml": maxRSSEntryContentHTMLBytes,
		}[name]
		if len(value) > limit || !utf8.ValidString(value) {
			t.Fatalf("%s = %d bytes, utf8=%v", name, len(value), utf8.ValidString(value))
		}
	}
	if first.URL != "" || strings.Contains(first.ContentHTML, longURL) {
		t.Fatalf("overlong durable URL survived: %#v", first)
	}
	if len(first.Media) != maxRSSParsedEntryMediaItems-1 || first.Media[len(first.Media)-1].URL != "https://cdn.example/62.jpg" {
		t.Fatalf("persisted media did not retain bounded original order: %d %#v", len(first.Media), first.Media)
	}
	for _, item := range first.Media {
		if item.Width != maxRSSMediaDimension || item.Height != 0 || item.Duration != maxRSSMediaDurationMillis {
			t.Fatalf("persisted numeric media was not bounded: %#v", item)
		}
	}
	if len(first.ImageURLs) != maxRSSParsedEntryMediaItems-1 || first.ImageURLs[len(first.ImageURLs)-1] != "https://cdn.example/62.jpg" {
		t.Fatalf("persisted images = %#v", first.ImageURLs)
	}
}

func TestEntriesFromFeedExtractsKnownVideoIframeBeforeSanitizingContent(t *testing.T) {
	entries := entriesFromFeed("subscription-1", domainrss.ViewTypeAuto, parsedFeed{
		SiteURL: "https://example.com/",
		Entries: []parsedEntry{{
			ExternalID: "video-1", Title: "Video",
			Content: `<iframe src="https://www.youtube-nocookie.com/embed/AbCdEfGhI12"></iframe>`,
		}}}, time.Now())
	if len(entries) != 1 || len(entries[0].Media) != 1 || entries[0].Platform != "youtube" ||
		strings.Contains(entries[0].ContentHTML, "iframe") ||
		!strings.Contains(entries[0].ContentHTML, `data-xiadown-rss-video-provider="youtube"`) ||
		!strings.Contains(entries[0].ContentHTML, `data-xiadown-rss-video-id="AbCdEfGhI12"`) {
		t.Fatalf("known iframe extraction/sanitization = %#v", entries)
	}
}
