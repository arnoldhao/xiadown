package app

import (
	"path/filepath"
	"testing"
)

func TestRSSImageDiskCacheDirectoryUsesUserCacheHierarchy(t *testing.T) {
	base := t.TempDir()
	want := filepath.Join(base, "xiadown", "rss", "resources", "v1")
	if got := rssImageDiskCacheDirectory(base); got != want {
		t.Fatalf("RSS image cache directory = %q, want %q", got, want)
	}
	if got := rssImageDiskCacheDirectory("  "); got != "" {
		t.Fatalf("empty cache base produced %q", got)
	}
}

func TestMusicResourceCacheDirectoryUsesUserCacheHierarchy(t *testing.T) {
	base := t.TempDir()
	production := filepath.Join(base, "xiadown", "music", "resources", "v1", "production")
	development := filepath.Join(base, "xiadown", "music", "resources", "v1", "development")
	if got := musicResourceCacheDirectory(base, "1.2.3"); got != production {
		t.Fatalf("production Music resource cache directory = %q, want %q", got, production)
	}
	if got := musicResourceCacheDirectory(base, "dev"); got != development {
		t.Fatalf("development Music resource cache directory = %q, want %q", got, development)
	}
	if production == development {
		t.Fatal("production and development Music caches must be isolated")
	}
	if got := musicResourceCacheDirectory("  ", "dev"); got != "" {
		t.Fatalf("empty cache base produced %q", got)
	}
}

func TestLibraryVideoThumbnailCacheDirectoryUsesUserCacheHierarchy(t *testing.T) {
	base := t.TempDir()
	production := filepath.Join(base, "xiadown", "library", "video-thumbnails", "v1", "production")
	development := filepath.Join(base, "xiadown", "library", "video-thumbnails", "v1", "development")
	if got := libraryVideoThumbnailCacheDirectory(base, "1.2.3"); got != production {
		t.Fatalf("production Library thumbnail cache directory = %q, want %q", got, production)
	}
	if got := libraryVideoThumbnailCacheDirectory(base, "dev"); got != development {
		t.Fatalf("development Library thumbnail cache directory = %q, want %q", got, development)
	}
	if production == development {
		t.Fatal("production and development Library thumbnail caches must be isolated")
	}
	if got := libraryVideoThumbnailCacheDirectory("  ", "dev"); got != "" {
		t.Fatalf("empty cache base produced %q", got)
	}
}
