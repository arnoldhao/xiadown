package wails

import "testing"

func TestWebViewYouTubeMusicTopLevelNavigationPolicy(t *testing.T) {
	t.Parallel()

	policy, ok := webViewRemoteNavigationPolicyForPlayer(
		listenPlayerWindowName,
		"https://music.youtube.com/watch?v=videoid0001",
	)
	if !ok {
		t.Fatal("valid YouTube Music target was rejected")
	}
	for _, test := range []struct {
		name string
		url  string
		want bool
	}{
		{name: "blank", url: "about:blank", want: true},
		{name: "watch", url: "https://music.youtube.com/watch?v=videoid0001", want: true},
		{name: "supported next video", url: "https://music.youtube.com/watch?v=BaW_jenozKc&list=RDAMVM", want: true},
		{name: "home", url: "https://music.youtube.com/", want: false},
		{name: "duplicate identity", url: "https://music.youtube.com/watch?v=videoid0001&v=BaW_jenozKc", want: false},
		{name: "lookalike", url: "https://music.youtube.com.attacker.example/watch?v=videoid0001", want: false},
		{name: "wrong port", url: "https://music.youtube.com:8443/watch?v=videoid0001", want: false},
		{name: "credentials", url: "https://user@music.youtube.com/watch?v=videoid0001", want: false},
		{name: "script", url: "javascript:location='https://music.youtube.com/watch?v=videoid0001'", want: false},
	} {
		test := test
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			if got := policy.allows(test.url); got != test.want {
				t.Fatalf("policy.allows(%q) = %v, want %v", test.url, got, test.want)
			}
		})
	}
}

func TestWebViewYouTubeLiveTopLevelNavigationLocksRequestedIdentity(t *testing.T) {
	t.Parallel()

	policy, ok := webViewRemoteNavigationPolicyForPlayer(
		listenLivePlayerWindowName,
		"https://www.youtube.com/watch?v=videoid0001#xiadown-request=videoid0001",
	)
	if !ok {
		t.Fatal("valid YouTube live target was rejected")
	}
	for _, test := range []struct {
		name string
		url  string
		want bool
	}{
		{name: "same watch", url: "https://www.youtube.com/watch?v=videoid0001&autoplay=1", want: true},
		{name: "same embed", url: "https://www.youtube.com/embed/videoid0001?playsinline=1", want: true},
		{name: "different watch", url: "https://www.youtube.com/watch?v=BaW_jenozKc", want: false},
		{name: "encoded embed identity", url: "https://www.youtube.com/embed/%64Qw4w9WgXcQ", want: false},
		{name: "embed child path", url: "https://www.youtube.com/embed/videoid0001/more", want: false},
		{name: "shorts", url: "https://www.youtube.com/shorts/videoid0001", want: false},
		{name: "account", url: "https://accounts.google.com/", want: false},
	} {
		test := test
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			if got := policy.allows(test.url); got != test.want {
				t.Fatalf("policy.allows(%q) = %v, want %v", test.url, got, test.want)
			}
		})
	}
}

func TestWebViewRSSNavigationPolicyLocksPreparedIdentity(t *testing.T) {
	t.Parallel()

	policy, ok := webViewRemoteNavigationPolicyForPlayer(
		rssBilibiliPlayerWindowName,
		"https://www.bilibili.com/video/BV1xx411c7mD/",
	)
	if !ok {
		t.Fatal("valid RSS Bilibili target was rejected")
	}
	if policy.expectedAdapter != rssBilibiliVideoAdapter || policy.expectedVideoID != "BV1xx411c7mD" {
		t.Fatalf("ordinary-video policy identity = %q:%q", policy.expectedAdapter, policy.expectedVideoID)
	}
	if !policy.allows("https://www.bilibili.com/video/BV1xx411c7mD/?p=2") {
		t.Fatal("same Bilibili identity was rejected")
	}
	if policy.allows("https://www.bilibili.com/video/BV1Q541167Qg/") {
		t.Fatal("different Bilibili identity was allowed")
	}
	if policy.allows("https://www.bilibili.com/bangumi/play/ep3854807") {
		t.Fatal("Bangumi adapter escaped an ordinary-video policy")
	}
}

func TestWebViewRSSNavigationPolicyLocksBangumiAdapterAndIdentity(t *testing.T) {
	t.Parallel()

	for _, test := range []struct {
		name        string
		target      string
		wantID      string
		allowed     string
		wrongID     string
		wrongPrefix string
	}{
		{
			name:        "episode",
			target:      "https://www.bilibili.com/bangumi/play/ep3854807",
			wantID:      "ep3854807",
			allowed:     "https://www.bilibili.com/bangumi/play/EP3854807/?from=feed",
			wrongID:     "https://www.bilibili.com/bangumi/play/ep3854808",
			wrongPrefix: "https://www.bilibili.com/bangumi/play/ss3854807",
		},
		{
			name:        "season",
			target:      "https://www.bilibili.com/bangumi/play/ss28747",
			wantID:      "ss28747",
			allowed:     "https://www.bilibili.com/bangumi/play/SS28747?from=feed",
			wrongID:     "https://www.bilibili.com/bangumi/play/ss28748",
			wrongPrefix: "https://www.bilibili.com/bangumi/play/ep28747",
		},
	} {
		test := test
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			policy, ok := webViewRemoteNavigationPolicyForPlayer(
				rssBilibiliPlayerWindowName,
				test.target,
			)
			if !ok {
				t.Fatalf("valid Bangumi %s target was rejected", test.name)
			}
			if policy.expectedAdapter != rssBilibiliBangumiAdapter || policy.expectedVideoID != test.wantID {
				t.Fatalf("Bangumi policy identity = %q:%q", policy.expectedAdapter, policy.expectedVideoID)
			}
			if !policy.allows(test.allowed) {
				t.Fatalf("same Bangumi %s identity was rejected", test.name)
			}
			for _, blocked := range []string{
				test.wrongID,
				test.wrongPrefix,
				"https://www.bilibili.com/video/BV1xx411c7mD/",
			} {
				if policy.allows(blocked) {
					t.Fatalf("Bangumi policy allowed identity escape %q", blocked)
				}
			}
			wrongAdapter := policy
			wrongAdapter.expectedAdapter = rssBilibiliVideoAdapter
			if wrongAdapter.allows(test.target) {
				t.Fatal("Bangumi target was accepted under ordinary-video adapter state")
			}
		})
	}
}

func TestWebViewAppSessionTopLevelNavigationRequiresHTTPS(t *testing.T) {
	t.Parallel()

	policy, ok := webViewRemoteNavigationPolicyForAppSession("https://www.youtube.com/")
	if !ok {
		t.Fatal("valid App Session target was rejected")
	}
	for _, test := range []struct {
		url  string
		want bool
	}{
		{url: "about:blank", want: true},
		{url: "https://accounts.google.com/o/oauth2/auth", want: true},
		{url: "https://example.com:443/login", want: true},
		{url: "http://example.com/login", want: false},
		{url: "https://example.com:8443/login", want: false},
		{url: "https://user@example.com/login", want: false},
		{url: "file:///etc/passwd", want: false},
	} {
		if got := policy.allows(test.url); got != test.want {
			t.Fatalf("policy.allows(%q) = %v, want %v", test.url, got, test.want)
		}
	}
}

func TestWebViewRSSSiteKnownPolicyAllowsOnlyItsDomainGroup(t *testing.T) {
	t.Parallel()
	policy, ok := webViewRemoteNavigationPolicyForRSSSite(
		"https://b23.tv/abc",
		[]string{"bilibili.com", "b23.tv"},
		"",
	)
	if !ok {
		t.Fatal("valid known-site target was rejected")
	}
	for _, test := range []struct {
		url  string
		want bool
	}{
		{url: "about:blank", want: true},
		{url: "https://www.bilibili.com/bangumi/play/ep123", want: true},
		{url: "https://sub.b23.tv/redirect", want: true},
		{url: "https://bilibili.com.attacker.example/video", want: false},
		{url: "https://youtube.com/watch?v=videoid0001", want: false},
		{url: "http://www.bilibili.com/video", want: false},
		{url: "https://user@www.bilibili.com/video", want: false},
		{url: "https://www.bilibili.com:8443/video", want: false},
	} {
		if got := policy.allows(test.url); got != test.want {
			t.Errorf("policy.allows(%q) = %v, want %v", test.url, got, test.want)
		}
	}
}

func TestWebViewRSSSiteUnknownPolicyStaysWithinInitialRegistrableSite(t *testing.T) {
	t.Parallel()
	policy, ok := webViewRemoteNavigationPolicyForRSSSite(
		"https://video.example.com/watch/42",
		nil,
		"example.com",
	)
	if !ok {
		t.Fatal("valid unknown-site target was rejected")
	}
	for _, test := range []struct {
		url  string
		want bool
	}{
		{url: "https://cdn.example.com/player", want: true},
		{url: "https://example.com/next", want: true},
		{url: "https://example.com.attacker.test/next", want: false},
		{url: "https://example.net/next", want: false},
		{url: "https://127.0.0.1/next", want: false},
		{url: "https://localhost/next", want: false},
	} {
		if got := policy.allows(test.url); got != test.want {
			t.Errorf("policy.allows(%q) = %v, want %v", test.url, got, test.want)
		}
	}
}
