package wails

import (
	"os"
	"strings"
	"testing"
)

func TestRSSBilibiliTopLevelNavigationPolicy(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		url  string
		want bool
	}{
		{name: "bootstrap blank document", url: "about:blank", want: true},
		{name: "canonical BV page", url: "https://www.bilibili.com/video/BV1xx411c7mD/", want: true},
		{name: "canonical BV query", url: "https://www.bilibili.com/video/BV1xx411c7mD?spm_id_from=333", want: true},
		{name: "canonical av page", url: "https://www.bilibili.com/video/av170001/", want: true},
		{name: "canonical episode page", url: "https://www.bilibili.com/bangumi/play/ep3854807", want: true},
		{name: "normalized episode prefix", url: "https://www.bilibili.com/bangumi/play/EP3854807/", want: true},
		{name: "canonical season query", url: "https://www.bilibili.com/bangumi/play/ss28747?from_spmid=666", want: true},
		{name: "explicit default port", url: "https://www.bilibili.com:443/video/BV1xx411c7mD/", want: true},
		{name: "external player", url: "https://player.bilibili.com/player.html?bvid=BV1xx411c7mD", want: false},
		{name: "homepage", url: "https://www.bilibili.com/", want: false},
		{name: "extra path", url: "https://www.bilibili.com/video/BV1xx411c7mD/comments", want: false},
		{name: "short BV id", url: "https://www.bilibili.com/video/BV123/", want: false},
		{name: "zero av id", url: "https://www.bilibili.com/video/av0/", want: false},
		{name: "zero episode id", url: "https://www.bilibili.com/bangumi/play/ep0", want: false},
		{name: "leading zero episode id", url: "https://www.bilibili.com/bangumi/play/ep03854807", want: false},
		{name: "bangumi child path", url: "https://www.bilibili.com/bangumi/play/ep3854807/comments", want: false},
		{name: "video identity on bangumi path", url: "https://www.bilibili.com/bangumi/play/BV1xx411c7mD", want: false},
		{name: "escaped episode id", url: "https://www.bilibili.com/bangumi/play/%65p3854807", want: false},
		{name: "escaped id", url: "https://www.bilibili.com/video/%42V1xx411c7mD/", want: false},
		{name: "insecure page", url: "http://www.bilibili.com/video/BV1xx411c7mD/", want: false},
		{name: "lookalike suffix", url: "https://www.bilibili.com.attacker.example/video/BV1xx411c7mD/", want: false},
		{name: "lookalike prefix", url: "https://notwww.bilibili.com/video/BV1xx411c7mD/", want: false},
		{name: "non default port", url: "https://www.bilibili.com:8443/video/BV1xx411c7mD/", want: false},
		{name: "userinfo", url: "https://user@www.bilibili.com/video/BV1xx411c7mD/", want: false},
		{name: "script", url: "javascript:location.href='https://www.bilibili.com/video/BV1xx411c7mD/'", want: false},
		{name: "empty", url: "", want: false},
	}

	for _, test := range tests {
		test := test
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			if got := rssBilibiliAllowsTopLevelNavigation(test.url); got != test.want {
				t.Fatalf("rssBilibiliAllowsTopLevelNavigation(%q) = %v, want %v", test.url, got, test.want)
			}
		})
	}
}

func TestRSSBilibiliTopLevelNavigationPolicyLocksPreparedBangumiIdentity(t *testing.T) {
	t.Parallel()

	const (
		expectedAdapter = rssBilibiliBangumiAdapter
		expectedID      = "ep3854807"
	)
	tests := []struct {
		name string
		url  string
		want bool
	}{
		{name: "exact episode", url: "https://www.bilibili.com/bangumi/play/ep3854807", want: true},
		{name: "normalized episode prefix", url: "https://www.bilibili.com/bangumi/play/EP3854807/", want: true},
		{name: "same episode query", url: "https://www.bilibili.com/bangumi/play/ep3854807?from_spmid=666", want: true},
		{name: "same episode fragment", url: "https://www.bilibili.com/bangumi/play/ep3854807#reply", want: true},
		{name: "related episode", url: "https://www.bilibili.com/bangumi/play/ep3854808", want: false},
		{name: "season identity", url: "https://www.bilibili.com/bangumi/play/ss28747", want: false},
		{name: "ordinary video adapter", url: "https://www.bilibili.com/video/BV1xx411c7mD/", want: false},
		{name: "post-install blank navigation", url: "about:blank", want: false},
	}
	for _, test := range tests {
		test := test
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			if got := rssBilibiliAllowsTopLevelNavigationForPlayback(test.url, expectedAdapter, expectedID); got != test.want {
				t.Fatalf("exact Bangumi navigation policy = %v, want %v for %q", got, test.want, test.url)
			}
		})
	}

	canonicalURL := "https://www.bilibili.com/bangumi/play/ep3854807"
	if rssBilibiliAllowsTopLevelNavigationForPlayback(canonicalURL, rssBilibiliVideoAdapter, expectedID) {
		t.Fatal("Bangumi URL was accepted under the ordinary video adapter")
	}
	if rssBilibiliAllowsTopLevelNavigationForPlayback(canonicalURL, expectedAdapter, "EP3854807") {
		t.Fatal("non-canonical expected identity was accepted")
	}
}

func TestRSSBilibiliTopLevelNavigationPolicyLocksPreparedIdentity(t *testing.T) {
	t.Parallel()

	const expected = "BV1xx411c7mD"
	tests := []struct {
		name string
		url  string
		want bool
	}{
		{name: "post-install blank navigation", url: "about:blank", want: false},
		{name: "exact video", url: "https://www.bilibili.com/video/BV1xx411c7mD/", want: true},
		{name: "normalized BV prefix", url: "https://www.bilibili.com/video/bv1xx411c7mD/", want: true},
		{name: "same video query", url: "https://www.bilibili.com/video/BV1xx411c7mD/?p=2&t=31", want: true},
		{name: "same video fragment", url: "https://www.bilibili.com/video/BV1xx411c7mD/#reply", want: true},
		{name: "related BV", url: "https://www.bilibili.com/video/BV1Q541167Qg/", want: false},
		{name: "case changed identity", url: "https://www.bilibili.com/video/BV1XX411C7MD/", want: false},
		{name: "av identity", url: "https://www.bilibili.com/video/av170001/", want: false},
	}
	for _, test := range tests {
		test := test
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			if got := rssBilibiliAllowsTopLevelNavigationForVideo(test.url, expected); got != test.want {
				t.Fatalf("exact navigation policy = %v, want %v for %q", got, test.want, test.url)
			}
		})
	}
}

func TestDarwinRSSBilibiliPlayerBlocksInAppNewWindows(t *testing.T) {
	t.Parallel()

	source, err := os.ReadFile("listen_player_webview_darwin.go")
	if err != nil {
		t.Fatal(err)
	}
	text := string(source)
	for _, required := range []string{
		"createWebViewWithConfiguration",
		"navigationAction.targetFrame == nil",
		"WKNavigationActionPolicyCancel",
		"return nil;",
		"listenRSSBilibiliIsCanonicalVideoPath",
		"listenRSSBilibiliIsTrustedVideoPageURL",
		`[components[1] isEqualToString:@"bangumi"]`,
		`[components[2] isEqualToString:@"play"]`,
		`listenRSSBilibiliIsPositiveDecimalIdentifier(bangumiID, @"ep")`,
		`listenRSSBilibiliIsPositiveDecimalIdentifier(bangumiID, @"ss")`,
		`@"www.bilibili.com"`,
		"components.percentEncodedPath",
		"listenInstallRSSBilibiliNavigationPolicy",
		"listenExpectedVideoID",
		"[videoID isEqualToString:expectedVideoID]",
		"return showListenNativeEmbeddedWebView(playerNativeWindow, hostNativeWindow, rect)",
	} {
		if !strings.Contains(text, required) {
			t.Fatalf("Darwin RSS player navigation policy is missing %q", required)
		}
	}
}

func TestDarwinRSSBilibiliCanonicalPageKeepsAppSessionBeforeNavigation(t *testing.T) {
	t.Parallel()

	source, err := os.ReadFile("listen_player_webview_darwin.go")
	if err != nil {
		t.Fatal(err)
	}
	text := string(source)
	start := strings.Index(text, "static void listenLoadRSSBilibiliURL(")
	if start < 0 {
		t.Fatal("Darwin RSS canonical-page loader source is missing")
	}
	end := strings.Index(text[start:], "static void listenEvaluateYouTubeMusicJavaScript(")
	if end < 0 {
		t.Fatal("Darwin RSS canonical-page loader source is missing")
	}
	loader := text[start : start+end]
	for _, required := range []string{
		"listenRSSBilibiliIsTrustedVideoPageURL(url)",
		`@"https://www.bilibili.com/"`,
		"webView.configuration.websiteDataStore.httpCookieStore",
		"[cookieStore setCookie:cookie completionHandler:",
		"listenLoadRequestOnMain(webView, request)",
		"latestGeneration == navigationGeneration",
	} {
		if !strings.Contains(loader, required) {
			t.Fatalf("Darwin RSS canonical-page loader is missing %q", required)
		}
	}

	goLoader := rssVideoFunctionSource(t, text, "func loadRSSVideoPlayerURL(")
	for _, required := range []string{
		"rssBilibiliPlaybackIdentityFromURL(targetURL)",
		"!rssBilibiliAllowsTopLevelNavigationForPlayback(targetURL, expectedAdapter, expectedVideoID)",
		"C.listenInstallRSSBilibiliNavigationPolicy(nativeWindow, cExpectedVideoID) != 0",
	} {
		if !strings.Contains(goLoader, required) {
			t.Fatalf("Darwin RSS canonical-page loader is missing %q", required)
		}
	}
	policy := strings.Index(goLoader, "C.listenInstallRSSBilibiliNavigationPolicy(nativeWindow, cExpectedVideoID) != 0")
	navigate := strings.Index(goLoader, "C.listenLoadRSSBilibiliURL(nativeWindow, cTargetURL, cCookies)")
	if policy < 0 || navigate < 0 || policy >= navigate {
		t.Fatal("Darwin RSS player must install its exact navigation policy before authenticated navigation")
	}
}
