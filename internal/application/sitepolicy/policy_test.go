package sitepolicy

import "testing"

func TestForURLMatchesYouTubePolicy(t *testing.T) {
	t.Parallel()

	policy, ok := ForURL("https://www.youtube.com/watch?v=TESTVID001A")
	if !ok {
		t.Fatalf("expected youtube policy match")
	}
	if policy.Key != "youtube" {
		t.Fatalf("expected youtube policy key, got %q", policy.Key)
	}
	if policy.SiteKey != "youtube" {
		t.Fatalf("expected youtube site key, got %q", policy.SiteKey)
	}
}

func TestForSiteKeyYouTubeDomainsIncludeShortURL(t *testing.T) {
	t.Parallel()

	policy, ok := ForSiteKey("youtube")
	if !ok {
		t.Fatalf("expected youtube site policy")
	}
	if !MatchDomains("https://youtu.be/test", policy.Domains) {
		t.Fatalf("expected youtube site domains to cover short URLs")
	}
}

func TestForSiteKeyChinaPrivateUsesProfileScope(t *testing.T) {
	t.Parallel()

	policy, ok := ForSiteKey("china_private")
	if !ok {
		t.Fatalf("expected china private site policy")
	}
	for _, rawURL := range []string{
		"https://www.douyin.com/video/123",
		"https://v.douyin.com/example/",
		"https://www.iesdouyin.com/share/video/123/",
		"https://www.xiaohongshu.com/explore/123",
		"https://www.rednote.com/explore/123",
		"https://xhslink.com/a/example",
		"https://xhslink.cn/a/example",
		"http://xhsurl.com/example",
		"http://xhs.cn/example",
		"https://rl.ink/example",
	} {
		if !MatchDomains(rawURL, policy.Domains) {
			t.Fatalf("expected china private profile policy to match %s", rawURL)
		}
	}
	sites := ProfileSitesForSiteKey("china_private")
	if len(sites) != 2 {
		t.Fatalf("expected two profile sites, got %#v", sites)
	}
	if sites[0].URL != "https://www.douyin.com/" ||
		sites[1].URL != "https://www.xiaohongshu.com/explore" {
		t.Fatalf("unexpected profile sites: %#v", sites)
	}
	if sites[0].Label != "douyin.com" ||
		sites[1].Label != "xiaohongshu.com" {
		t.Fatalf("unexpected profile site labels: %#v", sites)
	}
	if MatchDomains("https://verify.snssdk.com/captcha/reportFrontend", policy.Domains) {
		t.Fatal("expected china private profile policy not to include verification hosts")
	}
}

func TestForURLMatchesNewBuiltinSites(t *testing.T) {
	t.Parallel()

	cases := map[string]string{
		"https://www.youtube.com/watch?v=test":         "youtube",
		"https://www.bilibili.com/video/BV1xx411c7mD/": "bilibili",
		"https://www.tiktok.com/@creator/video/123":    "tiktok",
		"https://www.douyin.com/video/123":             "china_private",
		"https://www.iesdouyin.com/share/video/123/":   "china_private",
		"https://www.xiaohongshu.com/explore/123":      "china_private",
		"https://www.rednote.com/explore/123":          "china_private",
		"https://xhslink.com/a/example":                "china_private",
		"https://www.instagram.com/reel/abc/":          "instagram",
		"https://x.com/example/status/1":               "x",
		"https://www.facebook.com/watch/?v=123":        "facebook",
		"https://vimeo.com/123456":                     "vimeo",
		"https://www.twitch.tv/videos/123":             "twitch",
		"https://www.nicovideo.jp/watch/sm123456789":   "niconico",
	}

	for rawURL, expected := range cases {
		policy, ok := ForURL(rawURL)
		if !ok {
			t.Fatalf("expected policy match for %s", rawURL)
		}
		if policy.Key != expected {
			t.Fatalf("expected policy %q for %s, got %q", expected, rawURL, policy.Key)
		}
	}
}
