package libraryicons

import (
	"bytes"
	"context"
	"io"
	"net/http"
	"os"
	"strings"
	"testing"
)

type faviconRoundTripFunc func(*http.Request) (*http.Response, error)

func (fn faviconRoundTripFunc) RoundTrip(request *http.Request) (*http.Response, error) {
	return fn(request)
}

func TestFaviconCacheBoundsMemoryAndMissingEntries(t *testing.T) {
	cache := NewFaviconCache()
	cache.maxMemoryEntries = 2
	cache.maxMissingEntries = 2

	cache.storeIcon("a.example", "icon-a")
	cache.storeIcon("b.example", "icon-b")
	cache.storeIcon("c.example", "icon-c")

	if len(cache.memory) != 2 {
		t.Fatalf("expected bounded memory cache size 2, got %d", len(cache.memory))
	}
	if _, ok := cache.memory["a.example"]; ok {
		t.Fatalf("expected oldest in-memory icon entry to be evicted")
	}
	if got := cache.memory["c.example"]; got != "icon-c" {
		t.Fatalf("expected newest icon to be retained, got %q", got)
	}

	cache.markMissing("miss-a.example")
	cache.markMissing("miss-b.example")
	cache.markMissing("miss-c.example")

	if len(cache.missing) != 2 {
		t.Fatalf("expected bounded missing cache size 2, got %d", len(cache.missing))
	}
	if _, ok := cache.missing["miss-a.example"]; ok {
		t.Fatalf("expected oldest missing entry to be evicted")
	}
	if _, ok := cache.missing["miss-c.example"]; !ok {
		t.Fatalf("expected newest missing entry to be retained")
	}
}

func TestFaviconCacheRejectsOversizedRemoteBodies(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name          string
		contentLength int64
		body          []byte
	}{
		{name: "declared", contentLength: maxFaviconBytes + 1, body: []byte("ignored")},
		{name: "streamed", contentLength: -1, body: bytes.Repeat([]byte{'x'}, int(maxFaviconBytes)+1)},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			cache := NewFaviconCache()
			cache.baseDir = ""
			cache.httpClient = &http.Client{Transport: faviconRoundTripFunc(func(*http.Request) (*http.Response, error) {
				return &http.Response{
					StatusCode:    http.StatusOK,
					Body:          io.NopCloser(bytes.NewReader(test.body)),
					ContentLength: test.contentLength,
					Header:        make(http.Header),
				}, nil
			})}

			_, err := cache.ResolveDomainIcon(context.Background(), "example.com")
			if err == nil || !strings.Contains(err.Error(), "exceeds") {
				t.Fatalf("expected oversized favicon rejection, got %v", err)
			}
		})
	}
}

func TestFaviconCacheRejectsOversizedDiskEntry(t *testing.T) {
	t.Parallel()

	cache := NewFaviconCache()
	cache.baseDir = t.TempDir()
	if err := os.WriteFile(
		cache.iconPath("example.com"),
		bytes.Repeat([]byte{'x'}, int(maxFaviconBytes)+1),
		0o600,
	); err != nil {
		t.Fatalf("write oversized cache entry: %v", err)
	}

	if icon, ok := cache.ResolveDomainIconCached(context.Background(), "example.com"); ok || icon != "" {
		t.Fatalf("expected oversized disk cache entry to be ignored, got ok=%v icon length=%d", ok, len(icon))
	}
}
