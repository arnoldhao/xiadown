package ytdlp

import "testing"

func TestValidateNetworkURLAllowsOnlyAbsoluteHTTPWithoutUserinfo(t *testing.T) {
	t.Parallel()

	for _, rawURL := range []string{
		"http://example.com/media",
		"https://example.com/media?email=user@example.com",
		"HTTPS://example.com/media",
		"http://[::1]:8080/media",
	} {
		if err := ValidateNetworkURL(rawURL); err != nil {
			t.Errorf("ValidateNetworkURL(%q) = %v", rawURL, err)
		}
	}
	for _, rawURL := range []string{
		"",
		"/relative/media",
		"//example.com/media",
		"file:///tmp/media",
		"ftp://example.com/media",
		"rtmp://example.com/live",
		"rtsp://example.com/live",
		"srt://example.com:9000",
		"udp://example.com:9000",
		"tcp://example.com:9000",
		"https:///missing-host",
		"https://user@example.com/media",
		"https://user:secret@example.com/media",
	} {
		if err := ValidateNetworkURL(rawURL); err == nil {
			t.Errorf("ValidateNetworkURL(%q) unexpectedly succeeded", rawURL)
		}
	}
}

func TestResolveManifestReferenceRejectsUnsafeAbsoluteReferences(t *testing.T) {
	t.Parallel()

	baseURL := "https://media.example/path/master.m3u8"
	if got := ResolveManifestReference(baseURL, "../segment.ts"); got != "https://media.example/segment.ts" {
		t.Fatalf("relative reference resolved to %q", got)
	}
	for _, reference := range []string{
		"file:///etc/passwd",
		"ftp://media.example/segment.ts",
		"rtmp://media.example/live",
		"tcp://media.example:9000",
		"//user:secret@media.example/segment.ts",
	} {
		if got := ResolveManifestReference(baseURL, reference); got != "" {
			t.Errorf("ResolveManifestReference(%q) = %q, want empty", reference, got)
		}
	}
}
