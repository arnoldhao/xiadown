package http

import "testing"

func TestListenLivePreviewAuthorThumbnailURL(t *testing.T) {
	html := `{"contents":{"twoColumnWatchNextResults":{"results":{"results":{"contents":[{"videoSecondaryInfoRenderer":{"owner":{"videoOwnerRenderer":{"thumbnail":{"thumbnails":[{"url":"https://yt3.ggpht.com/avatar=s48-c-k-c0x00ffffff-no-rj","width":48},{"url":"https://yt3.ggpht.com/avatar=s88-c-k-c0x00ffffff-no-rj","width":88}]}}}}}]}}}}}`

	got := listenLivePreviewAuthorThumbnailURL(html)
	if got != "https://yt3.ggpht.com/avatar" {
		t.Fatalf("expected high quality author avatar URL, got %q", got)
	}
}

func TestListenLivePreviewHighQualityAuthorImageURL(t *testing.T) {
	tests := []struct {
		name string
		raw  string
		want string
	}{
		{
			name: "strip s88 suffix",
			raw:  "https://yt3.googleusercontent.com/avatar=s88-c-k-c0x00ffffff-no-rj",
			want: "https://yt3.googleusercontent.com/avatar",
		},
		{
			name: "strip suffix before query",
			raw:  "https://yt3.ggpht.com/avatar=s176-c-k-c0x00ffffff-no-rj?foo=bar",
			want: "https://yt3.ggpht.com/avatar?foo=bar",
		},
		{
			name: "decode escaped ampersand",
			raw:  "https://yt3.ggpht.com/avatar=s88-c-k-c0x00ffffff-no-rj\\u0026x=1",
			want: "https://yt3.ggpht.com/avatar&x=1",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := listenLivePreviewHighQualityAuthorImageURL(tt.raw); got != tt.want {
				t.Fatalf("expected %q, got %q", tt.want, got)
			}
		})
	}
}
