package rss

import (
	"encoding/json"
	"encoding/xml"
	"fmt"
	"strings"
	"testing"
	"time"
)

func TestParseFeedSupportsRSSAtomAndJSONFeed(t *testing.T) {
	tests := []struct {
		name        string
		contentType string
		body        string
		assert      func(*testing.T, parsedFeed)
	}{
		{
			name:        "RSS 2.0 with namespaced content and media",
			contentType: "application/rss+xml",
			body: `<?xml version="1.0" encoding="UTF-8"?>
<rss version="2.0"
  xmlns:content="http://purl.org/rss/1.0/modules/content/"
  xmlns:dc="http://purl.org/dc/elements/1.1/"
  xmlns:media="http://search.yahoo.com/mrss/">
  <channel>
    <title>Example &amp; News</title>
    <link>https://example.com/</link>
    <description><![CDATA[<p>Daily <b>updates</b>.</p>]]></description>
    <image><url>https://example.com/icon.png</url></image>
    <item>
      <guid>post-1</guid>
      <title><![CDATA[<b>Launch</b> &amp; notes]]></title>
      <link>https://example.com/posts/1</link>
      <description><![CDATA[<p>Short <em>summary</em>.</p>]]></description>
      <content:encoded><![CDATA[<article><p>Full body.</p></article>]]></content:encoded>
      <dc:creator>Arnold</dc:creator>
      <pubDate>Mon, 13 Jul 2026 10:00:00 +0800</pubDate>
      <media:content url="https://cdn.example.com/video.mp4" type="video/mp4" width="1920" height="1080" duration="42">
        <media:thumbnail url="https://cdn.example.com/poster.jpg" />
      </media:content>
    </item>
  </channel>
</rss>`,
			assert: func(t *testing.T, feed parsedFeed) {
				t.Helper()
				if feed.Title != "Example & News" || feed.SiteURL != "https://example.com/" ||
					feed.Description != "Daily updates." || feed.IconURL != "https://example.com/icon.png" {
					t.Fatalf("unexpected RSS metadata: %#v", feed)
				}
				if len(feed.Entries) != 1 {
					t.Fatalf("RSS entries = %d, want 1", len(feed.Entries))
				}
				entry := feed.Entries[0]
				if entry.ExternalID != "post-1" || entry.URL != "https://example.com/posts/1" ||
					entry.Title != "Launch & notes" || entry.Author != "Arnold" || entry.Summary != "Short summary." ||
					entry.Content != "<article><p>Full body.</p></article>" {
					t.Fatalf("unexpected RSS entry: %#v", entry)
				}
				assertFeedTime(t, entry.Published, time.Date(2026, 7, 13, 2, 0, 0, 0, time.UTC))
				if len(entry.Media) != 1 {
					t.Fatalf("RSS media = %#v, want one item", entry.Media)
				}
				media := entry.Media[0]
				if media.URL != "https://cdn.example.com/video.mp4" || media.MIMEType != "video/mp4" ||
					media.Thumbnail != "https://cdn.example.com/poster.jpg" || media.Width != 1920 ||
					media.Height != 1080 || media.Duration != 42_000 {
					t.Fatalf("unexpected RSS media: %#v", media)
				}
			},
		},
		{
			name:        "Atom with alternate and enclosure links",
			contentType: "application/atom+xml",
			body: `<?xml version="1.0" encoding="utf-8"?>
<feed xmlns="http://www.w3.org/2005/Atom">
  <title>Atom Digest</title>
  <subtitle>Independent updates</subtitle>
  <icon>https://atom.example/icon.svg</icon>
  <link rel="self" href="https://atom.example/feed.xml" />
  <link rel="alternate" href="https://atom.example/" />
  <entry>
    <id>tag:atom.example,2026:entry-1</id>
    <title>First entry</title>
    <summary>Atom summary</summary>
    <content><![CDATA[<p>Atom body.</p>]]></content>
    <author><name>Lin</name></author>
    <published>2026-07-12T23:59:00+08:00</published>
    <updated>2026-07-13T00:05:00+08:00</updated>
    <link href="https://atom.example/posts/1" />
    <link rel="enclosure" type="image/webp" href="https://atom.example/posts/1.webp" />
  </entry>
</feed>`,
			assert: func(t *testing.T, feed parsedFeed) {
				t.Helper()
				if feed.Title != "Atom Digest" || feed.Description != "Independent updates" ||
					feed.SiteURL != "https://atom.example/" || feed.IconURL != "https://atom.example/icon.svg" {
					t.Fatalf("unexpected Atom metadata: %#v", feed)
				}
				if len(feed.Entries) != 1 {
					t.Fatalf("Atom entries = %d, want 1", len(feed.Entries))
				}
				entry := feed.Entries[0]
				if entry.ExternalID != "tag:atom.example,2026:entry-1" || entry.URL != "https://atom.example/posts/1" ||
					entry.Title != "First entry" || entry.Author != "Lin" || entry.Summary != "Atom summary" ||
					entry.Content != "<p>Atom body.</p>" {
					t.Fatalf("unexpected Atom entry: %#v", entry)
				}
				assertFeedTime(t, entry.Published, time.Date(2026, 7, 12, 15, 59, 0, 0, time.UTC))
				assertFeedTime(t, entry.Updated, time.Date(2026, 7, 12, 16, 5, 0, 0, time.UTC))
				if len(entry.Media) != 1 || entry.Media[0].URL != "https://atom.example/posts/1.webp" ||
					entry.Media[0].MIMEType != "image/webp" {
					t.Fatalf("unexpected Atom enclosure: %#v", entry.Media)
				}
			},
		},
		{
			name:        "JSON Feed 1.1 with text content and attachment",
			contentType: "application/feed+json; charset=utf-8",
			body: `{
  "version": "https://jsonfeed.org/version/1.1",
  "title": "JSON Digest",
  "home_page_url": "https://json.example/",
  "description": "A JSON feed",
  "favicon": "https://json.example/favicon.ico",
  "items": [{
    "id": "json-1",
    "external_url": "https://json.example/posts/1",
    "title": "JSON entry",
    "content_text": "One < two & three",
    "date_published": "2026-07-13T04:00:00Z",
    "date_modified": "2026-07-13T04:05:00Z",
    "authors": [{"name": "Mei"}],
    "attachments": [{
      "url": "https://cdn.json.example/clip.webm",
      "mime_type": "video/webm",
      "duration_in_seconds": 75
    }]
  }]
}`,
			assert: func(t *testing.T, feed parsedFeed) {
				t.Helper()
				if feed.Title != "JSON Digest" || feed.SiteURL != "https://json.example/" ||
					feed.Description != "A JSON feed" || feed.IconURL != "https://json.example/favicon.ico" {
					t.Fatalf("unexpected JSON Feed metadata: %#v", feed)
				}
				if len(feed.Entries) != 1 {
					t.Fatalf("JSON Feed entries = %d, want 1", len(feed.Entries))
				}
				entry := feed.Entries[0]
				if entry.ExternalID != "json-1" || entry.URL != "https://json.example/posts/1" ||
					entry.Title != "JSON entry" || entry.Author != "Mei" || entry.Summary != "One < two & three" ||
					entry.Content != "One &lt; two &amp; three" {
					t.Fatalf("unexpected JSON Feed entry: %#v", entry)
				}
				assertFeedTime(t, entry.Published, time.Date(2026, 7, 13, 4, 0, 0, 0, time.UTC))
				assertFeedTime(t, entry.Updated, time.Date(2026, 7, 13, 4, 5, 0, 0, time.UTC))
				if len(entry.Media) != 1 || entry.Media[0].URL != "https://cdn.json.example/clip.webm" ||
					entry.Media[0].MIMEType != "video/webm" || entry.Media[0].Duration != 75_000 {
					t.Fatalf("unexpected JSON Feed attachment: %#v", entry.Media)
				}
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			feed, err := parseFeed([]byte(test.body), test.contentType)
			if err != nil {
				t.Fatalf("parseFeed: %v", err)
			}
			test.assert(t, feed)
		})
	}
}

func TestParseFeedRejectsEmptyAndUnsupportedDocuments(t *testing.T) {
	tests := []struct {
		name string
		body string
	}{
		{name: "empty", body: "  \n\t"},
		{name: "unsupported XML root", body: `<html><body>not a feed</body></html>`},
		{name: "unsupported JSON", body: `{"items":[]}`},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if _, err := parseFeed([]byte(test.body), ""); err == nil {
				t.Fatal("parseFeed unexpectedly accepted unsupported input")
			}
		})
	}
}

func TestParseFeedResolvesJSONAndAtomHistoryLinks(t *testing.T) {
	jsonResult, err := parseFeed([]byte(`{
  "version":"https://jsonfeed.org/version/1.1",
  "title":"Paged JSON",
  "next_url":"../archive/page-2.json",
  "items":[]
}`), "application/feed+json")
	if err != nil {
		t.Fatal(err)
	}
	jsonResult = resolveFeedURLs(jsonResult, "https://feeds.example/current/feed.json")
	if jsonResult.HistoryURL != "https://feeds.example/archive/page-2.json" {
		t.Fatalf("JSON next_url = %q", jsonResult.HistoryURL)
	}

	atomResult, err := parseFeed([]byte(`
<feed xmlns="http://www.w3.org/2005/Atom">
  <title>Archived Atom</title>
  <link rel="next" href="../paged/page-2.atom" />
  <link rel="next-archive" href="../archive/newer.atom" />
  <link rel="prev-archive" href="../archive/older.atom" />
</feed>`), "application/atom+xml")
	if err != nil {
		t.Fatal(err)
	}
	atomResult = resolveFeedURLs(atomResult, "https://feeds.example/current/index.atom")
	if atomResult.HistoryURL != "https://feeds.example/archive/older.atom" {
		t.Fatalf("Atom archive cursor = %q, want prev-archive", atomResult.HistoryURL)
	}

	pagedAtom, err := parseFeed([]byte(`
<feed xmlns="http://www.w3.org/2005/Atom">
  <title>Paged Atom</title>
  <link rel="next" href="?page=2" />
</feed>`), "application/atom+xml")
	if err != nil {
		t.Fatal(err)
	}
	pagedAtom = resolveFeedURLs(pagedAtom, "https://feeds.example/current/index.atom?page=1")
	if pagedAtom.HistoryURL != "https://feeds.example/current/index.atom?page=2" {
		t.Fatalf("Atom next cursor = %q", pagedAtom.HistoryURL)
	}
}

func TestFeedDecodersApplyCardinalityLimitsDuringDecode(t *testing.T) {
	started := time.Now()

	t.Run("RSS channel items and media", func(t *testing.T) {
		var body strings.Builder
		body.WriteString(`<rss><channel><title>Bounded RSS</title><item><guid>first</guid>`)
		for index := 0; index < maxRSSParsedEntryMediaItems+8; index++ {
			_, _ = fmt.Fprintf(&body, `<enclosure url="https://cdn.example/%d.mp4" type="video/mp4"/>`, index)
		}
		body.WriteString(`</item>`)
		for index := 1; index < maxRSSFeedEntries+8; index++ {
			_, _ = fmt.Fprintf(&body, `<item><guid>item-%d</guid></item>`, index)
		}
		body.WriteString(`</channel></rss>`)

		var document rssDocument
		if err := xml.Unmarshal([]byte(body.String()), &document); err != nil {
			t.Fatal(err)
		}
		if len(document.Channel.Items) != maxRSSFeedEntries || len(document.Channel.Items[0].Media) != maxRSSParsedEntryMediaItems {
			t.Fatalf("decoded RSS cardinality = entries %d, media %d", len(document.Channel.Items), len(document.Channel.Items[0].Media))
		}
		feed := mapRSS(document)
		if len(feed.Entries) != maxRSSFeedEntries || len(feed.Entries[0].Media) != maxRSSParsedEntryMediaItems ||
			feed.Entries[0].Media[63].URL != "https://cdn.example/63.mp4" {
			t.Fatalf("mapped RSS cardinality/order = entries %d, media %#v", len(feed.Entries), feed.Entries[0].Media)
		}
	})

	t.Run("RDF root items", func(t *testing.T) {
		var body strings.Builder
		body.WriteString(`<rdf:RDF xmlns:rdf="http://www.w3.org/1999/02/22-rdf-syntax-ns#">`)
		for index := 0; index < maxRSSFeedEntries+8; index++ {
			_, _ = fmt.Fprintf(&body, `<item><guid>rdf-%d</guid></item>`, index)
		}
		body.WriteString(`</rdf:RDF>`)
		var document rssDocument
		if err := xml.Unmarshal([]byte(body.String()), &document); err != nil {
			t.Fatal(err)
		}
		if len(document.Items) != maxRSSFeedEntries {
			t.Fatalf("decoded RDF entries = %d", len(document.Items))
		}
	})

	t.Run("Atom entries and links", func(t *testing.T) {
		var body strings.Builder
		body.WriteString(`<feed xmlns="http://www.w3.org/2005/Atom"><title>Bounded Atom</title><entry><id>first</id>`)
		for index := 0; index < maxRSSParsedEntryMediaItems+8; index++ {
			_, _ = fmt.Fprintf(&body, `<link rel="enclosure" href="https://cdn.example/%d.mp4" type="video/mp4"/>`, index)
		}
		body.WriteString(`</entry>`)
		for index := 1; index < maxRSSFeedEntries+8; index++ {
			_, _ = fmt.Fprintf(&body, `<entry><id>atom-%d</id></entry>`, index)
		}
		body.WriteString(`</feed>`)
		var document atomDocument
		if err := xml.Unmarshal([]byte(body.String()), &document); err != nil {
			t.Fatal(err)
		}
		if len(document.Entries) != maxRSSFeedEntries || len(document.Entries[0].Links) != maxRSSParsedEntryMediaItems {
			t.Fatalf("decoded Atom cardinality = entries %d, links %d", len(document.Entries), len(document.Entries[0].Links))
		}
	})

	t.Run("JSON items attachments and authors", func(t *testing.T) {
		var body strings.Builder
		body.WriteString(`{"version":"https://jsonfeed.org/version/1.1","title":"Bounded JSON","items":[{"id":"first","authors":[`)
		for index := 0; index < 8; index++ {
			if index > 0 {
				body.WriteByte(',')
			}
			_, _ = fmt.Fprintf(&body, `{"name":"author-%d"}`, index)
		}
		body.WriteString(`],"attachments":[`)
		for index := 0; index < maxRSSParsedEntryMediaItems+8; index++ {
			if index > 0 {
				body.WriteByte(',')
			}
			_, _ = fmt.Fprintf(&body, `{"url":"https://cdn.example/%d.mp4","mime_type":"video/mp4"}`, index)
		}
		body.WriteString(`]}`)
		for index := 1; index < maxRSSFeedEntries+8; index++ {
			_, _ = fmt.Fprintf(&body, `,{"id":"json-%d"}`, index)
		}
		body.WriteString(`]}`)
		var document jsonFeed
		if err := json.Unmarshal([]byte(body.String()), &document); err != nil {
			t.Fatal(err)
		}
		if len(document.Items) != maxRSSFeedEntries || len(document.Items[0].Attachments) != maxRSSParsedEntryMediaItems ||
			len(document.Items[0].Authors) != 1 {
			t.Fatalf("decoded JSON cardinality = items %d, attachments %d, authors %d", len(document.Items), len(document.Items[0].Attachments), len(document.Items[0].Authors))
		}
	})

	if elapsed := time.Since(started); elapsed > 10*time.Second {
		t.Fatalf("bounded high-cardinality feed decoding took %s", elapsed)
	}
}

func TestParseFeedBoundsNumericMediaAndDocumentBytes(t *testing.T) {
	rssFeed, err := parseFeed([]byte(`<rss xmlns:media="http://search.yahoo.com/mrss/"><channel><title>x</title><item>`+
		`<media:content url="https://cdn.example/video.mp4" type="video/mp4" width="-1" height="999999" duration="9223372036854775807"/>`+
		`</item></channel></rss>`), "application/rss+xml")
	if err != nil {
		t.Fatal(err)
	}
	media := rssFeed.Entries[0].Media[0]
	if media.Width != 0 || media.Height != maxRSSMediaDimension || media.Duration != maxRSSMediaDurationMillis {
		t.Fatalf("bounded RSS numeric media = %#v", media)
	}

	jsonFeed, err := parseFeed([]byte(`{"version":"https://jsonfeed.org/version/1.1","title":"x","items":[{"id":"1","attachments":[`+
		`{"url":"https://cdn.example/negative.mp4","duration_in_seconds":-1},`+
		`{"url":"https://cdn.example/huge.mp4","duration_in_seconds":9223372036854775807}]}]}`), "application/feed+json")
	if err != nil {
		t.Fatal(err)
	}
	if jsonFeed.Entries[0].Media[0].Duration != 0 || jsonFeed.Entries[0].Media[1].Duration != maxRSSMediaDurationMillis {
		t.Fatalf("bounded JSON durations = %#v", jsonFeed.Entries[0].Media)
	}

	if _, err := parseFeed(make([]byte, maxRSSFeedDocumentBytes+1), "application/rss+xml"); err == nil {
		t.Fatal("oversized feed document was accepted")
	}
}

func assertFeedTime(t *testing.T, got *time.Time, want time.Time) {
	t.Helper()
	if got == nil || !got.Equal(want) {
		t.Fatalf("feed time = %v, want %v", got, want)
	}
}
