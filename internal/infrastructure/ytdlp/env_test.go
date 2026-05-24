package ytdlp

import (
	"reflect"
	"testing"
)

func TestSanitizeArgsMasksSensitiveHeaderArgs(t *testing.T) {
	t.Parallel()

	got := SanitizeArgs([]string{
		"--add-header", "Cookie: sid=secret",
		"--add-headers", "Authorization: Bearer token",
		"--add-header", "Referer: https://page.example/watch",
		"--add-header", "X-CSRF-Token: csrf-secret",
		"--proxy", "http://user:pass@127.0.0.1:8080",
	})
	want := []string{
		"--add-header", "Cookie: ****",
		"--add-headers", "Authorization: ****",
		"--add-header", "Referer: https://page.example/watch",
		"--add-header", "X-CSRF-Token: ****",
		"--proxy", "http://%2A%2A%2A%2A@127.0.0.1:8080",
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("unexpected sanitized args:\n got: %#v\nwant: %#v", got, want)
	}
}
