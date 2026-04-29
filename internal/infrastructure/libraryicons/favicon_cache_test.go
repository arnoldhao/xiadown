package libraryicons

import "testing"

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
