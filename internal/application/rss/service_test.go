package rss

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	domainrss "xiadown/internal/domain/rss"
)

func TestNormalizeFeedURLPreservesCanonicalRSSHubRoutes(t *testing.T) {
	tests := map[string]string{
		"rsshub scheme":   "rsshub://youtube/user/@OpenAI",
		"route shorthand": "/youtube/user/@OpenAI",
	}
	for name, input := range tests {
		t.Run(name, func(t *testing.T) {
			got, err := normalizeFeedURL(input)
			if err != nil {
				t.Fatal(err)
			}
			if got != "rsshub://youtube/user/@OpenAI" {
				t.Fatalf("normalizeFeedURL(%q) = %q", input, got)
			}
		})
	}
}

func TestNormalizeFeedURLCanonicalizesSafeHTTPEquivalents(t *testing.T) {
	tests := map[string]string{
		"scheme host default HTTPS port and empty path": "HTTPS://Example.COM:443#reader",
		"already canonical root":                        "https://example.com/",
		"default HTTP port":                             "http://EXAMPLE.com:80/feed.xml#reader",
		"non-default port":                              "https://EXAMPLE.com:8443/feed.xml",
		"query preserved":                               "https://EXAMPLE.com?source=One",
	}
	want := map[string]string{
		"scheme host default HTTPS port and empty path": "https://example.com/",
		"already canonical root":                        "https://example.com/",
		"default HTTP port":                             "http://example.com/feed.xml",
		"non-default port":                              "https://example.com:8443/feed.xml",
		"query preserved":                               "https://example.com/?source=One",
	}
	for name, input := range tests {
		t.Run(name, func(t *testing.T) {
			got, err := normalizeFeedURL(input)
			if err != nil {
				t.Fatal(err)
			}
			if got != want[name] {
				t.Fatalf("normalizeFeedURL(%q) = %q, want %q", input, got, want[name])
			}
		})
	}
}

func TestNormalizeFeedURLCollapsesCanonicalDuplicateAliases(t *testing.T) {
	aliases := []string{
		"HTTPS://Feeds.Example.COM:443",
		"https://feeds.example.com/",
		"https://FEEDS.EXAMPLE.COM:443/#fragment",
	}
	var canonical string
	for _, alias := range aliases {
		got, err := normalizeFeedURL(alias)
		if err != nil {
			t.Fatalf("normalizeFeedURL(%q): %v", alias, err)
		}
		if canonical == "" {
			canonical = got
			continue
		}
		if got != canonical {
			t.Fatalf("duplicate aliases normalized to %q and %q", canonical, got)
		}
	}
}

func TestNormalizeFeedValidatorURLBindsCanonicalFullURL(t *testing.T) {
	got, err := normalizeFeedValidatorURL("HTTPS://Example.COM:443/feed.xml?source=one#client")
	if err != nil {
		t.Fatal(err)
	}
	if got != "https://example.com/feed.xml?source=one" {
		t.Fatalf("validator URL = %q", got)
	}
	other, err := normalizeFeedValidatorURL("https://example.com/other.xml?source=one")
	if err != nil {
		t.Fatal(err)
	}
	if got == other {
		t.Fatal("different full feed paths shared validator provenance")
	}
}

func TestResolveFetchCandidatesUsesCeziMirrorsInOrder(t *testing.T) {
	service := NewService(nil, nil)
	candidates, err := service.resolveFetchCandidates("rsshub://bilibili/user/video/123")
	if err != nil {
		t.Fatal(err)
	}
	want := []string{
		"https://rsshub.rssforever.com/bilibili/user/video/123",
		"https://rsshub.umzzz.com/bilibili/user/video/123",
	}
	if len(candidates) != len(want) {
		t.Fatalf("candidates = %#v", candidates)
	}
	for index := range want {
		if candidates[index] != want[index] {
			t.Fatalf("candidate[%d] = %q, want %q", index, candidates[index], want[index])
		}
	}
}

func TestNormalizeFeedURLRejectsUnsafeRSSHubRoutes(t *testing.T) {
	for _, value := range []string{
		"rsshub://", "rsshub://../private", "rsshub://https://example.com/feed",
		"rsshub://youtube/user/:id", "rsshub://mail/imap/:email/:folder{.+}?", "rsshub://example/*",
	} {
		if _, err := normalizeFeedURL(value); err == nil {
			t.Fatalf("normalizeFeedURL(%q) unexpectedly succeeded", value)
		}
	}
}

func TestDiscoverFeedURLUsesBoundedStreamingTokens(t *testing.T) {
	valid := []byte(`<html><head><link rel="alternate" type="application/rss+xml" href="/feed.xml"></head></html>`)
	if got := discoverFeedURL(valid, "https://example.com/articles/page"); got != "https://example.com/feed.xml" {
		t.Fatalf("discovered URL = %q", got)
	}

	deep := []byte(strings.Repeat("<div>", maxFeedDiscoveryHTMLTokens+1) +
		`<link rel="alternate" type="application/rss+xml" href="/too-late.xml">`)
	if len(deep) > maxFeedDiscoveryHTMLBytes {
		t.Fatalf("deep regression fixture unexpectedly exceeded byte budget: %d", len(deep))
	}
	if got := discoverFeedURL(deep, "https://example.com/page"); got != "" {
		t.Fatalf("deep HTML escaped token budget: %q", got)
	}

	oversized := make([]byte, maxFeedDiscoveryHTMLBytes+1)
	if got := discoverFeedURL(oversized, "https://example.com/page"); got != "" {
		t.Fatalf("oversized discovery HTML returned %q", got)
	}
	longHref := `<link rel="alternate" type="application/rss+xml" href="https://example.com/` +
		strings.Repeat("x", maxFeedValidatorURLBytes) + `">`
	if got := discoverFeedURL([]byte(longHref), "https://example.com/page"); got != "" {
		t.Fatalf("oversized discovery href returned %q", got)
	}
}

func TestRunBoundedRSSRefreshesKeepsLargeBatchesAtFourWorkers(t *testing.T) {
	const subscriptionCount = 10_000
	subscriptions := make([]domainrss.Subscription, subscriptionCount)
	enabled := 0
	for index := range subscriptions {
		subscriptions[index] = domainrss.Subscription{
			ID:      fmt.Sprintf("subscription-%d", index),
			Enabled: index%5 != 0,
		}
		if subscriptions[index].Enabled {
			enabled++
		}
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	release := make(chan struct{})
	started := make(chan struct{}, maxConcurrentRSSRefreshes)
	type outcome struct {
		result RefreshResult
		err    error
	}
	done := make(chan outcome, 1)
	var active, peak, calls atomic.Int64
	refresh := func(ctx context.Context, id string) (domainrss.UpsertResult, error) {
		current := active.Add(1)
		defer active.Add(-1)
		for {
			previous := peak.Load()
			if current <= previous || peak.CompareAndSwap(previous, current) {
				break
			}
		}
		call := calls.Add(1)
		if call <= maxConcurrentRSSRefreshes {
			started <- struct{}{}
		}
		select {
		case <-release:
		case <-ctx.Done():
			return domainrss.UpsertResult{}, ctx.Err()
		}
		if id == "subscription-9999" {
			return domainrss.UpsertResult{}, errors.New("fixture refresh failed")
		}
		return domainrss.UpsertResult{Created: 1, Updated: 2}, nil
	}
	go func() {
		result, err := runBoundedRSSRefreshes(ctx, subscriptions, maxConcurrentRSSRefreshes, refresh)
		done <- outcome{result: result, err: err}
	}()
	for range maxConcurrentRSSRefreshes {
		select {
		case <-started:
		case <-time.After(5 * time.Second):
			t.Fatal("timed out waiting for the fixed refresh workers")
		}
	}
	if got := calls.Load(); got != maxConcurrentRSSRefreshes {
		t.Fatalf("blocked refresh calls = %d, want %d workers", got, maxConcurrentRSSRefreshes)
	}
	close(release)
	var completed outcome
	select {
	case completed = <-done:
	case <-time.After(10 * time.Second):
		t.Fatal("large bounded refresh did not finish")
	}
	if completed.err != nil {
		t.Fatal(completed.err)
	}
	if got := peak.Load(); got != maxConcurrentRSSRefreshes {
		t.Fatalf("peak refresh workers = %d, want %d", got, maxConcurrentRSSRefreshes)
	}
	if got := int(calls.Load()); got != enabled {
		t.Fatalf("refresh calls = %d, want %d enabled subscriptions", got, enabled)
	}
	if completed.result.Subscriptions != enabled || completed.result.Failed != 1 ||
		completed.result.Created != enabled-1 || completed.result.Updated != (enabled-1)*2 {
		t.Fatalf("large refresh result = %#v", completed.result)
	}
}

func TestRunBoundedRSSRefreshesCancelsOnlyFixedWorkers(t *testing.T) {
	const subscriptionCount = 10_000
	subscriptions := make([]domainrss.Subscription, subscriptionCount)
	for index := range subscriptions {
		subscriptions[index] = domainrss.Subscription{ID: fmt.Sprintf("subscription-%d", index), Enabled: true}
	}

	ctx, cancel := context.WithCancel(context.Background())
	started := make(chan struct{}, maxConcurrentRSSRefreshes)
	type outcome struct {
		result RefreshResult
		err    error
	}
	done := make(chan outcome, 1)
	var calls atomic.Int64
	go func() {
		result, err := runBoundedRSSRefreshes(
			ctx,
			subscriptions,
			maxConcurrentRSSRefreshes,
			func(ctx context.Context, _ string) (domainrss.UpsertResult, error) {
				calls.Add(1)
				started <- struct{}{}
				<-ctx.Done()
				return domainrss.UpsertResult{}, ctx.Err()
			},
		)
		done <- outcome{result: result, err: err}
	}()
	for range maxConcurrentRSSRefreshes {
		select {
		case <-started:
		case <-time.After(5 * time.Second):
			cancel()
			t.Fatal("timed out waiting for cancellable refresh workers")
		}
	}
	if got := calls.Load(); got != maxConcurrentRSSRefreshes {
		cancel()
		t.Fatalf("refresh calls before cancellation = %d, want %d", got, maxConcurrentRSSRefreshes)
	}
	cancel()
	select {
	case completed := <-done:
		if !errors.Is(completed.err, context.Canceled) {
			t.Fatalf("cancel error = %v", completed.err)
		}
		if completed.result.Subscriptions != subscriptionCount || completed.result.Failed != maxConcurrentRSSRefreshes {
			t.Fatalf("cancel result = %#v", completed.result)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("cancelled bounded refresh did not finish")
	}
	if got := calls.Load(); got != maxConcurrentRSSRefreshes {
		t.Fatalf("refresh calls after cancellation = %d, want %d", got, maxConcurrentRSSRefreshes)
	}
}
