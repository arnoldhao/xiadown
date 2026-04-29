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
	if policy.ConnectorType != "youtube" {
		t.Fatalf("expected youtube connector cookies, got %q", policy.ConnectorType)
	}
}

func TestForConnectorTypeYouTubeDomainsIncludeShortURL(t *testing.T) {
	t.Parallel()

	policy, ok := ForConnectorType("youtube")
	if !ok {
		t.Fatalf("expected youtube connector policy")
	}
	if !MatchDomains("https://youtu.be/test", policy.Domains) {
		t.Fatalf("expected youtube connector domains to cover short URLs")
	}
}

func TestForURLMatchesNewBuiltinSites(t *testing.T) {
	t.Parallel()

	cases := map[string]string{
		"https://www.youtube.com/watch?v=test":         "youtube",
		"https://www.bilibili.com/video/BV1xx411c7mD/": "bilibili",
		"https://www.tiktok.com/@creator/video/123":    "tiktok",
		"https://www.douyin.com/video/123":             "douyin",
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
