package rss

import (
	"strings"
	"testing"
	"time"

	domainrss "xiadown/internal/domain/rss"
)

func TestResolveVideoPlatformRecognizesSupportedVideoURLs(t *testing.T) {
	tests := []struct {
		name        string
		rawURL      string
		media       []domainrss.Media
		platform    string
		videoID     string
		playbackURL string
	}{
		{
			name:        "YouTube watch URL",
			rawURL:      "https://www.youtube.com/watch?v=AbCdEfGhI12&feature=share",
			platform:    "youtube",
			videoID:     "AbCdEfGhI12",
			playbackURL: "https://www.youtube-nocookie.com/embed/AbCdEfGhI12",
		},
		{
			name:        "YouTube short URL",
			rawURL:      "https://youtu.be/AbCdEfGhI12?t=20",
			platform:    "youtube",
			videoID:     "AbCdEfGhI12",
			playbackURL: "https://www.youtube-nocookie.com/embed/AbCdEfGhI12",
		},
		{
			name:        "Bilibili BV URL",
			rawURL:      "https://www.bilibili.com/video/BV1xx411c7mD/?spm_id_from=333.1007",
			platform:    "bilibili",
			videoID:     "BV1xx411c7mD",
			playbackURL: "https://www.bilibili.com/video/BV1xx411c7mD/",
		},
		{
			name:        "Bilibili av URL",
			rawURL:      "https://m.bilibili.com/video/av170001",
			platform:    "bilibili",
			videoID:     "av170001",
			playbackURL: "https://www.bilibili.com/video/av170001/",
		},
		{
			name:        "Bilibili bangumi episode URL",
			rawURL:      "https://www.bilibili.com/bangumi/play/ep123?from_spmid=666.25.episode.0",
			platform:    "bilibili",
			videoID:     "ep123",
			playbackURL: "https://www.bilibili.com/bangumi/play/ep123",
		},
		{
			name:        "Bilibili bangumi season URL",
			rawURL:      "https://www.bilibili.com/bangumi/play/ss123/",
			platform:    "bilibili",
			videoID:     "ss123",
			playbackURL: "https://www.bilibili.com/bangumi/play/ss123",
		},
		{
			name:   "generic enclosure",
			rawURL: "https://example.com/posts/clip",
			media: []domainrss.Media{{
				URL: "https://cdn.example.com/clip.mp4", MIMEType: "video/mp4", Kind: "video",
			}},
			platform:    "generic",
			playbackURL: "https://cdn.example.com/clip.mp4",
		},
		{
			name:     "lookalike host is not YouTube",
			rawURL:   "https://youtube.com.attacker.example/watch?v=AbCdEfGhI12",
			platform: "",
		},
		{
			name:     "lookalike host is not Bilibili bangumi",
			rawURL:   "https://www.bilibili.com.attacker.example/bangumi/play/ep123",
			platform: "",
		},
		{
			name:     "Bilibili bangumi rejects invalid ID",
			rawURL:   "https://www.bilibili.com/bangumi/play/epnot-a-number",
			platform: "",
		},
		{
			name:     "Bilibili bangumi rejects zero ID",
			rawURL:   "https://www.bilibili.com/bangumi/play/ss0",
			platform: "",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			platform, videoID, playbackURL := resolveVideoPlatform(test.rawURL, test.media)
			if platform != test.platform || videoID != test.videoID || playbackURL != test.playbackURL {
				t.Fatalf("resolveVideoPlatform() = (%q, %q, %q), want (%q, %q, %q)",
					platform, videoID, playbackURL, test.platform, test.videoID, test.playbackURL)
			}
		})
	}
}

func TestResolveVideoPlatformRecognizesStrictSocialVideoPages(t *testing.T) {
	tests := []struct {
		name        string
		rawURL      string
		platform    string
		videoID     string
		playbackURL string
	}{
		{
			name:        "TikTok creator video",
			rawURL:      "https://www.tiktok.com/@Creator.Name/video/7351234567890123456?is_from_webapp=1",
			platform:    "tiktok",
			videoID:     "7351234567890123456",
			playbackURL: "https://www.tiktok.com/@Creator.Name/video/7351234567890123456",
		},
		{
			name:        "Douyin video",
			rawURL:      "https://www.douyin.com/video/7351234567890123456?previous_page=web_code_link",
			platform:    "douyin",
			videoID:     "7351234567890123456",
			playbackURL: "https://www.douyin.com/video/7351234567890123456",
		},
		{
			name:        "Instagram reel",
			rawURL:      "https://www.instagram.com/reel/C9_ab-CdE1/?igsh=tracking",
			platform:    "instagram",
			videoID:     "C9_ab-CdE1",
			playbackURL: "https://www.instagram.com/reel/C9_ab-CdE1/",
		},
		{
			name:        "Facebook reel",
			rawURL:      "https://m.facebook.com/reel/123456789012345/?mibextid=tracking",
			platform:    "facebook",
			videoID:     "123456789012345",
			playbackURL: "https://www.facebook.com/reel/123456789012345/",
		},
		{
			name:        "Facebook watch query",
			rawURL:      "https://www.facebook.com/watch/?ref=sharing&v=123456789012345",
			platform:    "facebook",
			videoID:     "123456789012345",
			playbackURL: "https://www.facebook.com/watch/?v=123456789012345",
		},
		{
			name:        "Facebook watch short path",
			rawURL:      "https://fb.watch/Ab_cd-Ef1/?ref=share",
			platform:    "facebook",
			videoID:     "Ab_cd-Ef1",
			playbackURL: "https://fb.watch/Ab_cd-Ef1/",
		},
		{
			name:        "Twitch recorded video",
			rawURL:      "https://www.twitch.tv/videos/1234567890?t=1h2m",
			platform:    "twitch",
			videoID:     "1234567890",
			playbackURL: "https://www.twitch.tv/videos/1234567890",
		},
		{
			name:        "Twitch clip",
			rawURL:      "https://clips.twitch.tv/InventiveClip-Slug_1?tt_content=url",
			platform:    "twitch",
			videoID:     "InventiveClip-Slug_1",
			playbackURL: "https://clips.twitch.tv/InventiveClip-Slug_1",
		},
		{
			name:        "Niconico watch",
			rawURL:      "https://www.nicovideo.jp/watch/SM123456789?from=feed",
			platform:    "niconico",
			videoID:     "sm123456789",
			playbackURL: "https://www.nicovideo.jp/watch/sm123456789",
		},
		{
			name:        "Niconico short watch",
			rawURL:      "https://nico.ms/NM00042?from=feed",
			platform:    "niconico",
			videoID:     "nm42",
			playbackURL: "https://www.nicovideo.jp/watch/nm42",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			platform, videoID, playbackURL := resolveVideoPlatform(test.rawURL, nil)
			if platform != test.platform || videoID != test.videoID || playbackURL != test.playbackURL {
				t.Fatalf("resolveVideoPlatform() = (%q, %q, %q), want (%q, %q, %q)",
					platform, videoID, playbackURL, test.platform, test.videoID, test.playbackURL)
			}
		})
	}
}

func TestResolveVideoPlatformRejectsAmbiguousSocialAndLookalikePages(t *testing.T) {
	tests := []struct {
		name   string
		rawURL string
	}{
		{name: "TikTok profile", rawURL: "https://www.tiktok.com/@creator"},
		{name: "TikTok unresolved short link", rawURL: "https://vm.tiktok.com/ZMexample/"},
		{name: "TikTok unresolved short link shaped as a video", rawURL: "https://vm.tiktok.com/@creator/video/123"},
		{name: "TikTok short-link subdomain is unresolved", rawURL: "https://foo.vm.tiktok.com/@creator/video/123"},
		{name: "TikTok path without creator identity", rawURL: "https://www.tiktok.com/video/123"},
		{name: "TikTok zero ID", rawURL: "https://www.tiktok.com/@creator/video/0"},
		{name: "TikTok lookalike host", rawURL: "https://www.tiktok.com.attacker.example/@creator/video/123"},
		{name: "Douyin profile", rawURL: "https://www.douyin.com/user/MS4wLjABAAAA"},
		{name: "Douyin non-video note path", rawURL: "https://www.douyin.com/note/123"},
		{name: "Douyin lookalike host", rawURL: "https://douyin.com.attacker.example/video/123"},
		{name: "Instagram ordinary post", rawURL: "https://www.instagram.com/p/C9_ab-CdE1/"},
		{name: "Instagram reel with an extra path segment", rawURL: "https://www.instagram.com/reel/C9_ab-CdE1/comments"},
		{name: "Instagram lookalike host", rawURL: "https://instagram.com.attacker.example/reel/C9_ab-CdE1/"},
		{name: "Facebook profile", rawURL: "https://www.facebook.com/example"},
		{name: "Facebook watch without video query", rawURL: "https://www.facebook.com/watch/"},
		{name: "Facebook watch with ambiguous video query", rawURL: "https://www.facebook.com/watch/?v=123&v=456"},
		{name: "Facebook watch with nonnumeric identity", rawURL: "https://www.facebook.com/watch/?v=not-a-video"},
		{name: "Facebook reel with nonnumeric identity", rawURL: "https://www.facebook.com/reel/not-a-video/"},
		{name: "Facebook lookalike host", rawURL: "https://facebook.com.attacker.example/reel/123/"},
		{name: "Facebook short host without identity", rawURL: "https://fb.watch/"},
		{name: "Facebook short host lookalike", rawURL: "https://fb.watch.attacker.example/Ab_cd-Ef1/"},
		{name: "Twitch channel", rawURL: "https://www.twitch.tv/example"},
		{name: "Twitch zero video ID", rawURL: "https://www.twitch.tv/videos/0"},
		{name: "Twitch clip host without slug", rawURL: "https://clips.twitch.tv/"},
		{name: "Twitch clip subdomain is not a video page", rawURL: "https://foo.clips.twitch.tv/videos/123"},
		{name: "Twitch lookalike host", rawURL: "https://twitch.tv.attacker.example/videos/123"},
		{name: "Niconico user page", rawURL: "https://www.nicovideo.jp/user/123"},
		{name: "Niconico watch without recognized identity", rawURL: "https://www.nicovideo.jp/watch/video123"},
		{name: "Niconico zero video ID", rawURL: "https://www.nicovideo.jp/watch/sm0"},
		{name: "Niconico lookalike host", rawURL: "https://nicovideo.jp.attacker.example/watch/sm123"},
		{name: "Niconico short host without identity", rawURL: "https://nico.ms/"},
		{name: "Niconico short host with extra path", rawURL: "https://nico.ms/sm123/comments"},
		{name: "Niconico short host lookalike", rawURL: "https://nico.ms.attacker.example/sm123"},
		{name: "X status is not hard-classified", rawURL: "https://x.com/example/status/123"},
		{name: "Xiaohongshu explore is not hard-classified", rawURL: "https://www.xiaohongshu.com/explore/abc123"},
		{name: "unsafe userinfo is rejected", rawURL: "https://feed-user@www.instagram.com/reel/C9_ab-CdE1/"},
		{name: "unsafe scheme is rejected", rawURL: "ftp://www.instagram.com/reel/C9_ab-CdE1/"},
		{name: "nondefault port is rejected", rawURL: "https://www.instagram.com:444/reel/C9_ab-CdE1/"},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			platform, videoID, playbackURL := resolveVideoPlatform(test.rawURL, nil)
			if platform != "" || videoID != "" || playbackURL != "" {
				t.Fatalf("resolveVideoPlatform() = (%q, %q, %q), want no platform", platform, videoID, playbackURL)
			}
		})
	}
}

func TestClassifyEntryOnlyPromotesExplicitSocialVideoPaths(t *testing.T) {
	videoURLs := []string{
		"https://www.tiktok.com/@creator/video/123",
		"https://www.douyin.com/video/123",
		"https://www.instagram.com/reel/Reel_123/",
		"https://www.facebook.com/watch/?v=123",
		"https://fb.watch/Clip_123/",
		"https://www.twitch.tv/videos/123",
		"https://clips.twitch.tv/Clip_123",
		"https://www.nicovideo.jp/watch/sm123",
		"https://nico.ms/so123",
	}
	for _, rawURL := range videoURLs {
		if got := classifyEntry(domainrss.ViewTypeArticle, rawURL, nil); got != domainrss.EntryKindVideo {
			t.Errorf("classifyEntry(article, %q) = %q, want %q", rawURL, got, domainrss.EntryKindVideo)
		}
	}

	nonVideoURLs := []string{
		"https://www.tiktok.com/@creator",
		"https://vm.tiktok.com/ZMexample/",
		"https://www.douyin.com/user/example",
		"https://www.instagram.com/p/Post_123/",
		"https://www.facebook.com/example",
		"https://www.twitch.tv/example",
		"https://www.nicovideo.jp/user/123",
		"https://x.com/example/status/123",
		"https://www.xiaohongshu.com/explore/abc123",
	}
	for _, rawURL := range nonVideoURLs {
		if got := classifyEntry(domainrss.ViewTypeAuto, rawURL, nil); got == domainrss.EntryKindVideo {
			t.Errorf("classifyEntry(auto, %q) unexpectedly promoted to video", rawURL)
		}
	}
}

func TestEntriesFromFeedProducesStableIDsAndClassifiesEntryKinds(t *testing.T) {
	now := time.Date(2026, 7, 13, 12, 0, 0, 0, time.UTC)
	feed := parsedFeed{
		SiteURL: "https://example.com/",
		Entries: []parsedEntry{
			{
				ExternalID: "video-1",
				URL:        "https://www.youtube.com/watch?v=AbCdEfGhI12",
				Title:      "Video entry",
			},
			{
				ExternalID: "social-1",
				URL:        "https://x.com/example/status/123",
				Title:      "Social entry",
			},
			{
				URL:     "https://example.com/gallery/one",
				Title:   "Image entry",
				Content: `<p>Gallery</p><img src="/media/one.webp"><img src="/media/one.webp">`,
			},
			{
				ExternalID: "article-1",
				URL:        "https://example.com/posts/article",
				Title:      "Article entry",
				Summary:    "A plain article",
			},
		},
	}

	entries := entriesFromFeed("subscription-a", domainrss.ViewTypeAuto, feed, now)
	if len(entries) != 4 {
		t.Fatalf("entriesFromFeed returned %d entries, want 4", len(entries))
	}
	byExternalID := make(map[string]domainrss.Entry, len(entries))
	for _, entry := range entries {
		byExternalID[entry.ExternalID] = entry
		if !strings.HasPrefix(entry.ID, "rss-entry-") || len(entry.ID) != len("rss-entry-")+32 {
			t.Errorf("entry %q has unstable-looking ID %q", entry.ExternalID, entry.ID)
		}
		if entry.Revision != 1 || !entry.CreatedAt.Equal(now) || !entry.ModifiedAt.Equal(now) || entry.ContentHash == "" {
			t.Errorf("entry %q missing initial version metadata: %#v", entry.ExternalID, entry)
		}
	}

	video := byExternalID["video-1"]
	if video.Kind != domainrss.EntryKindVideo || video.Platform != "youtube" ||
		video.PlatformVideoID != "AbCdEfGhI12" ||
		video.PlaybackURL != "https://www.youtube-nocookie.com/embed/AbCdEfGhI12" ||
		video.DownloadTarget != video.URL {
		t.Fatalf("unexpected video projection: %#v", video)
	}
	if social := byExternalID["social-1"]; social.Kind != domainrss.EntryKindSocial {
		t.Fatalf("social kind = %q, want %q", social.Kind, domainrss.EntryKindSocial)
	}
	image := byExternalID["https://example.com/gallery/one"]
	if image.Kind != domainrss.EntryKindImage || len(image.ImageURLs) != 1 ||
		image.ImageURLs[0] != "https://example.com/media/one.webp" || image.ThumbnailURL != image.ImageURLs[0] {
		t.Fatalf("unexpected image projection: %#v", image)
	}
	if article := byExternalID["article-1"]; article.Kind != domainrss.EntryKindArticle {
		t.Fatalf("article kind = %q, want %q", article.Kind, domainrss.EntryKindArticle)
	}

	changedFeed := feed
	changedFeed.Entries = append([]parsedEntry(nil), feed.Entries...)
	changedFeed.Entries[0].Title = "Renamed video entry"
	changedFeed.Entries[0].Summary = "New summary"
	changedFeed.Entries[2].Title = "Renamed image entry"
	changed := entriesFromFeed("subscription-a", domainrss.ViewTypeAuto, changedFeed, now.Add(time.Hour))
	if changed[0].ID != entries[0].ID {
		t.Fatalf("external-ID-backed entry identity changed: %q -> %q", entries[0].ID, changed[0].ID)
	}
	if changed[0].ContentHash == entries[0].ContentHash {
		t.Fatal("content hash did not change with user-visible entry content")
	}
	if changed[2].ID != entries[2].ID {
		t.Fatalf("URL-backed entry identity changed: %q -> %q", entries[2].ID, changed[2].ID)
	}

	otherSubscription := entriesFromFeed("subscription-b", domainrss.ViewTypeAuto, feed, now)
	if otherSubscription[0].ID == entries[0].ID {
		t.Fatalf("entry IDs collide across subscriptions: %q", entries[0].ID)
	}
}

func TestEntryContentHashTracksThumbnailAndPublishedTimeChanges(t *testing.T) {
	now := time.Date(2026, 7, 13, 12, 0, 0, 0, time.UTC)
	published := now.Add(-time.Hour)
	feed := parsedFeed{Entries: []parsedEntry{{
		ExternalID: "video-1",
		URL:        "https://example.com/watch/1",
		Title:      "Video",
		Published:  &published,
		Media: []parsedMedia{{
			URL: "https://cdn.example.com/video.mp4", MIMEType: "video/mp4",
			Thumbnail: "https://cdn.example.com/poster-v1.jpg",
		}},
	}}}
	baseline := entriesFromFeed("subscription-a", domainrss.ViewTypeAuto, feed, now)
	if len(baseline) != 1 {
		t.Fatalf("baseline entries = %d", len(baseline))
	}

	thumbnailChanged := feed
	thumbnailChanged.Entries = append([]parsedEntry(nil), feed.Entries...)
	thumbnailChanged.Entries[0].Media = append([]parsedMedia(nil), feed.Entries[0].Media...)
	thumbnailChanged.Entries[0].Media[0].Thumbnail = "https://cdn.example.com/poster-v2.jpg"
	withNewThumbnail := entriesFromFeed("subscription-a", domainrss.ViewTypeAuto, thumbnailChanged, now.Add(time.Minute))
	if withNewThumbnail[0].ContentHash == baseline[0].ContentHash {
		t.Fatal("thumbnail-only update did not change content hash")
	}

	publishedChanged := feed
	publishedChanged.Entries = append([]parsedEntry(nil), feed.Entries...)
	correctedPublished := published.Add(-24 * time.Hour)
	publishedChanged.Entries[0].Published = &correctedPublished
	withNewPublished := entriesFromFeed("subscription-a", domainrss.ViewTypeAuto, publishedChanged, now.Add(2*time.Minute))
	if withNewPublished[0].ContentHash == baseline[0].ContentHash {
		t.Fatal("published-time-only update did not change content hash")
	}
}

func TestClassifyEntryRequiresEvidenceBeforeUsingVideoKind(t *testing.T) {
	image := []domainrss.Media{{URL: "https://example.com/image.jpg", Kind: "image"}}
	video := []domainrss.Media{{URL: "https://example.com/video.mp4", Kind: "video"}}
	tests := []struct {
		name  string
		view  domainrss.ViewType
		url   string
		media []domainrss.Media
		want  domainrss.EntryKind
	}{
		{name: "auto article", view: domainrss.ViewTypeAuto, url: "https://example.com/post", want: domainrss.EntryKindArticle},
		{name: "auto social", view: domainrss.ViewTypeAuto, url: "https://social.example.bsky.app/profile/example/post/1", want: domainrss.EntryKindSocial},
		{name: "auto image", view: domainrss.ViewTypeAuto, url: "https://example.com/gallery", media: image, want: domainrss.EntryKindImage},
		{name: "explicit social", view: domainrss.ViewTypeSocial, url: "https://example.com/post", want: domainrss.EntryKindSocial},
		{name: "explicit image", view: domainrss.ViewTypeImage, url: "https://example.com/post", want: domainrss.EntryKindImage},
		{name: "video view keeps plain page readable", view: domainrss.ViewTypeVideo, url: "https://example.com/post", want: domainrss.EntryKindArticle},
		{name: "video view accepts an enclosure", view: domainrss.ViewTypeVideo, url: "https://example.com/post", media: video, want: domainrss.EntryKindVideo},
		{name: "video view accepts a recognized page", view: domainrss.ViewTypeVideo, url: "https://www.youtube.com/watch?v=AbCdEfGhI12", want: domainrss.EntryKindVideo},
		{name: "article keeps image as article", view: domainrss.ViewTypeArticle, url: "https://example.com/post", media: image, want: domainrss.EntryKindArticle},
		{name: "video enclosure overrides article", view: domainrss.ViewTypeArticle, url: "https://example.com/post", media: video, want: domainrss.EntryKindVideo},
		{name: "platform URL overrides article", view: domainrss.ViewTypeArticle, url: "https://www.bilibili.com/video/BV1xx411c7mD", want: domainrss.EntryKindVideo},
		{name: "bangumi URL overrides article", view: domainrss.ViewTypeArticle, url: "https://www.bilibili.com/bangumi/play/ep123", want: domainrss.EntryKindVideo},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if got := classifyEntry(test.view, test.url, test.media); got != test.want {
				t.Fatalf("classifyEntry() = %q, want %q", got, test.want)
			}
		})
	}
}

func TestEntriesFromVideoViewKeepOrdinaryPagesReadable(t *testing.T) {
	now := time.Date(2026, 7, 17, 21, 30, 0, 0, time.UTC)
	entries := entriesFromFeed("subscription-video-view", domainrss.ViewTypeVideo, parsedFeed{
		SiteURL: "https://example.com/",
		Entries: []parsedEntry{
			{ExternalID: "article", URL: "https://example.com/posts/article", Title: "Article"},
			{ExternalID: "video", URL: "https://www.youtube.com/watch?v=AbCdEfGhI12", Title: "Video"},
			{ExternalID: "nico", URL: "https://nico.ms/sm123", Title: "Niconico video"},
		},
	}, now)
	if len(entries) != 3 {
		t.Fatalf("entriesFromFeed returned %d entries, want 3", len(entries))
	}
	if entries[0].Kind != domainrss.EntryKindArticle || entries[0].Platform != "" {
		t.Fatalf("ordinary page projection = %#v, want article without video platform", entries[0])
	}
	if entries[1].Kind != domainrss.EntryKindVideo || entries[1].Platform != "youtube" {
		t.Fatalf("recognized page projection = %#v, want YouTube video", entries[1])
	}
	if entries[2].Kind != domainrss.EntryKindVideo ||
		entries[2].Platform != "niconico" ||
		entries[2].PlaybackURL != "https://www.nicovideo.jp/watch/sm123" {
		t.Fatalf("short Niconico projection = %#v, want canonical Niconico video", entries[2])
	}
}

func TestDownloadTargetForEntryPrefersGenericMediaAndPlatformPages(t *testing.T) {
	tests := []struct {
		name  string
		entry domainrss.Entry
		want  string
	}{
		{
			name: "generic enclosure",
			entry: domainrss.Entry{
				URL: "https://example.com/posts/clip", Platform: "generic",
				MediaURL: "https://cdn.example.com/clip.mp4",
			},
			want: "https://cdn.example.com/clip.mp4",
		},
		{
			name: "generic enclosure with platform rebuilt",
			entry: domainrss.Entry{
				URL: "https://example.com/posts/clip", MediaURL: "https://cdn.example.com/clip.mp4",
				Media: []domainrss.Media{{URL: "https://cdn.example.com/clip.mp4", Kind: "video"}},
			},
			want: "https://cdn.example.com/clip.mp4",
		},
		{
			name: "YouTube page",
			entry: domainrss.Entry{
				URL: "https://www.youtube.com/watch?v=AbCdEfGhI12", Platform: "youtube",
				MediaURL: "https://cdn.example.com/fallback.mp4",
			},
			want: "https://www.youtube.com/watch?v=AbCdEfGhI12",
		},
		{
			name: "Bilibili page with platform rebuilt",
			entry: domainrss.Entry{
				URL: "https://www.bilibili.com/video/BV1xx411c7mD", MediaURL: "https://cdn.example.com/fallback.mp4",
			},
			want: "https://www.bilibili.com/video/BV1xx411c7mD",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if got := downloadTargetForEntry(test.entry); got != test.want {
				t.Fatalf("downloadTargetForEntry() = %q, want %q", got, test.want)
			}
		})
	}
}

func TestEntriesFromFeedUsesEnclosureAsGenericVideoDownloadTarget(t *testing.T) {
	now := time.Date(2026, 7, 13, 12, 0, 0, 0, time.UTC)
	entries := entriesFromFeed("subscription-a", domainrss.ViewTypeAuto, parsedFeed{
		SiteURL: "https://example.com/",
		Entries: []parsedEntry{{
			ExternalID: "clip-1", URL: "https://example.com/posts/clip", Title: "Clip",
			Media: []parsedMedia{{URL: "https://cdn.example.com/clip.mp4", MIMEType: "video/mp4"}},
		}},
	}, now)
	if len(entries) != 1 {
		t.Fatalf("entriesFromFeed returned %d entries, want 1", len(entries))
	}
	entry := entries[0]
	if entry.Kind != domainrss.EntryKindVideo || entry.Platform != "generic" ||
		entry.MediaURL != "https://cdn.example.com/clip.mp4" || entry.DownloadTarget != entry.MediaURL {
		t.Fatalf("unexpected generic video projection: %#v", entry)
	}
}

func TestExtractHTMLMediaOnlyTreatsKnownIframesAsVideo(t *testing.T) {
	items := extractHTMLMedia(`
<iframe src="https://widgets.example.com/card/1"></iframe>
<iframe src="https://www.youtube-nocookie.com/embed/AbCdEfGhI12"></iframe>
<iframe src="https://player.bilibili.com/player.html?bvid=BV1xx411c7mD"></iframe>
<video src="/media/direct.mp4" type="video/mp4"></video>
`, "https://example.com/posts/1")
	want := []string{
		"https://www.youtube-nocookie.com/embed/AbCdEfGhI12",
		"https://player.bilibili.com/player.html?bvid=BV1xx411c7mD",
		"https://example.com/media/direct.mp4",
	}
	if len(items) != len(want) {
		t.Fatalf("extractHTMLMedia() = %#v, want URLs %#v", items, want)
	}
	for index, item := range items {
		if item.URL != want[index] || item.Kind != "video" {
			t.Fatalf("item[%d] = %#v, want video %q", index, item, want[index])
		}
	}
	if got := mediaKind("text/html", "https://widgets.example.com/card/1"); got != "" {
		t.Fatalf("arbitrary text/html media kind = %q, want empty", got)
	}
}
