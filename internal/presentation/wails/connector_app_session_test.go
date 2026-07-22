package wails

import (
	"context"
	"errors"
	"fmt"
	"runtime"
	"slices"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	appcookies "xiadown/internal/application/cookies"
	"xiadown/internal/domain/appsessions"
)

func nextYouTubeCookieSyncToken(t *testing.T, provider *NativeAppSessionProvider) (uint64, uint64) {
	t.Helper()
	epoch, sequence := provider.BeginAppSessionCookieSync("youtube")
	if epoch == 0 || sequence == 0 {
		t.Fatalf("invalid YouTube cookie sync token: (%d, %d)", epoch, sequence)
	}
	return epoch, sequence
}

func testYouTubeAuthSnapshot(value string) []appcookies.Record {
	return []appcookies.Record{{
		Name: "SAPISID", Value: value, Domain: ".youtube.com", Path: "/", Expires: 4_102_444_800,
	}}
}

func TestConnectorAppSessionCompleteCloseIsSingleTransition(t *testing.T) {
	session := &connectorAppSessionWindow{done: make(chan struct{})}
	start := make(chan struct{})
	var wait sync.WaitGroup
	for i := 0; i < 32; i++ {
		wait.Add(1)
		go func() {
			defer wait.Done()
			<-start
			session.completeClose(nil)
		}()
	}
	close(start)
	wait.Wait()

	select {
	case <-session.done:
	default:
		t.Fatal("expected done channel to be closed")
	}
}

func TestConnectorAppSessionCompletedWindowRetainsCloseSnapshot(t *testing.T) {
	want := []appcookies.Record{{
		Name: "SAPISID", Value: "captured-before-close", Domain: ".youtube.com", Path: "/",
	}}
	session := &connectorAppSessionWindow{
		last:      append([]appcookies.Record(nil), want...),
		completed: true,
	}

	got, err := session.Cookies(context.Background())
	if err != nil {
		t.Fatalf("read completed App Session snapshot: %v", err)
	}
	if !slices.Equal(got, want) {
		t.Fatalf("completed App Session snapshot = %#v, want %#v", got, want)
	}
	got[0].Value = "mutated-caller"
	if session.last[0].Value != "captured-before-close" {
		t.Fatal("completed App Session returned its owned close snapshot")
	}
}

func TestNativeAppSessionCookieCacheLoadsOnceAndReturnsCopies(t *testing.T) {
	var cache nativeAppSessionCookieCache
	var calls atomic.Int32
	source := []appcookies.Record{{
		Name:   "SID",
		Value:  "original-secret",
		Domain: ".youtube.com",
		Path:   "/",
	}}
	loader := func() ([]appcookies.Record, bool, error) {
		calls.Add(1)
		return source, true, nil
	}

	first, available, err := cache.load(" YouTube ", loader)
	if err != nil {
		t.Fatalf("first cache load: %v", err)
	}
	if !available || len(first) != 1 {
		t.Fatalf("first cache load unavailable: %#v", first)
	}
	source[0].Value = "mutated-source"
	first[0].Value = "mutated-caller"

	second, available, err := cache.load("youtube", loader)
	if err != nil {
		t.Fatalf("second cache load: %v", err)
	}
	if !available || len(second) != 1 || second[0].Value != "original-secret" {
		t.Fatalf("cache leaked a cookie slice mutation: %#v", second)
	}
	if calls.Load() != 1 {
		t.Fatalf("loader calls = %d, want 1", calls.Load())
	}

	second[0].Value = "mutated-second-caller"
	third, _, err := cache.load("YOUTUBE", loader)
	if err != nil {
		t.Fatalf("third cache load: %v", err)
	}
	if third[0].Value != "original-secret" {
		t.Fatalf("returned slice was not cloned: %#v", third)
	}
}

func TestConnectorAppSessionYouTubeInitialCookiesAreStableOnly(t *testing.T) {
	now := time.Unix(1_700_000_000, 0)
	filtered := connectorAppSessionInitialCookies("youtube", []appcookies.Record{
		{Name: "SAPISID", Value: "stable", Domain: ".youtube.com", Path: "/", Expires: now.Add(time.Hour).Unix()},
		{Name: "__Secure-3PSID", Value: "live-session", Domain: ".youtube.com", Path: "/", Expires: now.Add(time.Hour).Unix()},
		{Name: "__Secure-3PSIDTS", Value: "rotating", Domain: ".youtube.com", Path: "/", Expires: now.Add(time.Hour).Unix()},
	}, now)
	if len(filtered) != 1 || filtered[0].Name != "SAPISID" {
		t.Fatalf("YouTube connector initial cookies = %#v, want stable-only", filtered)
	}
}

func TestNativeAppSessionProviderLoadsPlatformYouTubePersistence(t *testing.T) {
	stored := []appcookies.Record{
		{Name: "SAPISID", Value: "stable", Domain: ".youtube.com", Path: "/", Expires: 4_102_444_800},
		{Name: "__Secure-1PSIDTS", Value: "stale-ts", Domain: ".youtube.com", Path: "/", Expires: 4_102_444_800},
		{Name: "SIDCC", Value: "stale-cc", Domain: ".youtube.com", Path: "/", Expires: 4_102_444_800},
	}
	provider := &NativeAppSessionProvider{
		loadStored: func(string) ([]appcookies.Record, error) {
			return stored, nil
		},
	}
	records, err := provider.LoadAppSessionCookies(context.Background(), "youtube")
	if err != nil {
		t.Fatalf("load YouTube persistence: %v", err)
	}
	expected := nativeYouTubePersistentCookies(stored, time.Now())
	if !slices.Equal(records, expected) {
		t.Fatalf("loaded persistence = %#v, want %#v", records, expected)
	}
}

func TestNativeAppSessionProviderSerializesYouTubeLoadWithSessionState(t *testing.T) {
	provider := &NativeAppSessionProvider{}
	provider.loadStored = func(string) ([]appcookies.Record, error) {
		if provider.runtimeSyncMu.TryLock() {
			provider.runtimeSyncMu.Unlock()
			t.Fatal("YouTube persistence load was not serialized with session state")
		}
		return []appcookies.Record{{
			Name: "SAPISID", Value: "stable", Domain: ".youtube.com", Path: "/", Expires: 4_102_444_800,
		}}, nil
	}
	provider.clearStored = func(string, []string) error { return nil }

	if _, err := provider.LoadAppSessionCookies(context.Background(), "youtube"); err != nil {
		t.Fatalf("load YouTube persistence: %v", err)
	}
	if err := provider.ClearAppSession(context.Background(), "youtube", nil); err != nil {
		t.Fatalf("clear YouTube persistence: %v", err)
	}
	if provider.youtubeRuntimeSyncAllowed {
		t.Fatal("clear left runtime synchronization enabled")
	}
	records, available, err := provider.cookieCache.load("youtube", nil)
	if err != nil || available || len(records) != 0 {
		t.Fatalf("clear did not leave an authoritative empty cache: records=%#v available=%t err=%v", records, available, err)
	}
}

func TestNativeAppSessionProviderHydratesRuntimeFromLiveStore(t *testing.T) {
	provider := NewNativeAppSessionProvider(nil)
	provider.loadStored = func(string) ([]appcookies.Record, error) {
		return []appcookies.Record{
			{Name: "SAPISID", Value: "stable", Domain: ".youtube.com", Path: "/", Expires: 4_102_444_800},
			{Name: "__Secure-3PSIDTS", Value: "legacy-ts", Domain: ".youtube.com", Path: "/", Expires: 4_102_444_800},
		}, nil
	}
	provider.loadRuntime = func() ([]appcookies.Record, error) {
		return []appcookies.Record{
			{Name: "SAPISID", Value: "stable", Domain: ".youtube.com", Path: "/", Expires: 4_102_444_800},
			{Name: "LOGIN_INFO", Value: "live-login", Domain: ".youtube.com", Path: "/", Expires: 4_102_444_800},
			{Name: "__Secure-3PSID", Value: "live-session", Domain: ".youtube.com", Path: "/", Expires: 4_102_444_800},
			{Name: "__Secure-3PSIDTS", Value: "rotating-ts", Domain: ".youtube.com", Path: "/", Expires: 4_102_444_800},
		}, nil
	}
	provider.saveStored = func(string, []appcookies.Record) error { return nil }
	provider.MarkWebKitReady()

	if err := provider.EnsureAppSessionRuntimeCookies(context.Background(), "youtube", true); err != nil {
		t.Fatalf("hydrate YouTube runtime cookies: %v", err)
	}
	records, err := provider.LoadAppSessionCookies(context.Background(), "youtube")
	if err != nil {
		t.Fatalf("load hydrated runtime cookies: %v", err)
	}
	if len(records) != 4 {
		t.Fatalf("hydrated request cache = %#v, want the complete live snapshot", records)
	}
	foundLiveTS := false
	for _, record := range records {
		if record.Name == "__Secure-3PSIDTS" && record.Value == "rotating-ts" {
			foundLiveTS = true
		}
	}
	if !foundLiveTS {
		t.Fatalf("hydration discarded the live WebKit generation: %#v", records)
	}
}

func TestNativeAppSessionProviderRefreshesStaleRuntimeHydration(t *testing.T) {
	provider := NewNativeAppSessionProvider(nil)
	provider.loadStored = func(string) ([]appcookies.Record, error) {
		return []appcookies.Record{{
			Name: "SAPISID", Value: "stable", Domain: ".youtube.com", Path: "/", Expires: 4_102_444_800,
		}}, nil
	}
	var runtimeLoads atomic.Int32
	provider.loadRuntime = func() ([]appcookies.Record, error) {
		generation := runtimeLoads.Add(1)
		return []appcookies.Record{
			{Name: "SAPISID", Value: "stable", Domain: ".youtube.com", Path: "/", Expires: 4_102_444_800},
			{Name: "__Secure-3PSIDTS", Value: fmt.Sprintf("generation-%d", generation), Domain: ".youtube.com", Path: "/", Expires: 4_102_444_800},
		}, nil
	}
	provider.saveStored = func(string, []appcookies.Record) error { return nil }
	provider.MarkWebKitReady()

	if err := provider.EnsureAppSessionRuntimeCookies(context.Background(), "youtube", true); err != nil {
		t.Fatalf("initial hydration: %v", err)
	}
	if err := provider.EnsureAppSessionRuntimeCookies(context.Background(), "youtube", true); err != nil {
		t.Fatalf("fresh hydration reuse: %v", err)
	}
	if runtimeLoads.Load() != 1 {
		t.Fatalf("fresh hydration read WebKit %d times, want 1", runtimeLoads.Load())
	}

	provider.hydratedAt = time.Now().Add(-youtubeRuntimeHydrationFreshness - time.Second)
	if err := provider.EnsureAppSessionRuntimeCookies(context.Background(), "youtube", true); err != nil {
		t.Fatalf("stale hydration refresh: %v", err)
	}
	if runtimeLoads.Load() != 2 {
		t.Fatalf("stale hydration read WebKit %d times, want 2", runtimeLoads.Load())
	}
	records, err := provider.LoadAppSessionCookies(context.Background(), "youtube")
	if err != nil {
		t.Fatalf("load refreshed runtime: %v", err)
	}
	foundLatest := false
	for _, record := range records {
		if record.Name == "__Secure-3PSIDTS" && record.Value == "generation-2" {
			foundLatest = true
		}
	}
	if !foundLatest {
		t.Fatalf("runtime cache did not receive refreshed generation: %#v", records)
	}
}

func TestNativeAppSessionProviderCoalescesFailedRuntimeRefresh(t *testing.T) {
	provider := NewNativeAppSessionProvider(nil)
	provider.loadStored = func(string) ([]appcookies.Record, error) {
		return []appcookies.Record{{
			Name: "SAPISID", Value: "stable", Domain: ".youtube.com", Path: "/", Expires: 4_102_444_800,
		}}, nil
	}
	var runtimeLoads atomic.Int32
	var failRefresh atomic.Bool
	failRefresh.Store(true)
	refreshErr := errors.New("WebKit store unavailable")
	provider.loadRuntime = func() ([]appcookies.Record, error) {
		load := runtimeLoads.Add(1)
		if load == 1 || !failRefresh.Load() {
			return []appcookies.Record{{
				Name: "SAPISID", Value: "stable", Domain: ".youtube.com", Path: "/", Expires: 4_102_444_800,
			}}, nil
		}
		return nil, refreshErr
	}
	provider.saveStored = func(string, []appcookies.Record) error { return nil }
	provider.MarkWebKitReady()

	if err := provider.EnsureAppSessionRuntimeCookies(context.Background(), "youtube", true); err != nil {
		t.Fatalf("initial hydration: %v", err)
	}
	provider.hydratedAt = time.Now().Add(-youtubeRuntimeHydrationFreshness - time.Second)
	if err := provider.EnsureAppSessionRuntimeCookies(context.Background(), "youtube", true); !errors.Is(err, refreshErr) {
		t.Fatalf("failed refresh error = %v, want %v", err, refreshErr)
	}
	if err := provider.EnsureAppSessionRuntimeCookies(context.Background(), "youtube", true); !errors.Is(err, refreshErr) {
		t.Fatalf("coalesced refresh error = %v, want %v", err, refreshErr)
	}
	if runtimeLoads.Load() != 2 {
		t.Fatalf("failed refresh was not cooled down; WebKit reads = %d", runtimeLoads.Load())
	}
	failRefresh.Store(false)
	provider.hydrationFailed.at = time.Now().Add(-youtubeRuntimeHydrationFailureCooldown - time.Second)
	if err := provider.EnsureAppSessionRuntimeCookies(context.Background(), "youtube", true); err != nil {
		t.Fatalf("refresh after cooldown: %v", err)
	}
	if runtimeLoads.Load() != 3 || provider.hydrationFailed.err != nil {
		t.Fatalf("refresh did not recover after cooldown: reads=%d failure=%v", runtimeLoads.Load(), provider.hydrationFailed.err)
	}
}

func TestNativeAppSessionProviderHydrationWaitsForWebKitReady(t *testing.T) {
	provider := NewNativeAppSessionProvider(nil)
	provider.loadStored = func(string) ([]appcookies.Record, error) {
		return []appcookies.Record{{
			Name: "SAPISID", Value: "stable", Domain: ".youtube.com", Path: "/", Expires: 4_102_444_800,
		}}, nil
	}
	provider.loadRuntime = func() ([]appcookies.Record, error) {
		t.Fatal("runtime loader ran before WebKit was ready")
		return nil, nil
	}

	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Millisecond)
	defer cancel()
	if err := provider.EnsureAppSessionRuntimeCookies(ctx, "youtube", true); !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("hydration before WebKit ready error = %v, want deadline exceeded", err)
	}
}

func TestNativeAppSessionProviderRecoversMissingVaultFromConnectedLiveStore(t *testing.T) {
	var saved []appcookies.Record
	var persistenceLoads atomic.Int32
	live := []appcookies.Record{
		{Name: "SAPISID", Value: "live-auth", Domain: ".youtube.com", Path: "/", Expires: 4_102_444_800},
		{Name: "LOGIN_INFO", Value: "live-login", Domain: ".youtube.com", Path: "/", Expires: 4_102_444_800},
		{Name: "__Secure-3PSIDTS", Value: "live-ts", Domain: ".youtube.com", Path: "/", Expires: 4_102_444_800},
	}
	provider := NewNativeAppSessionProvider(nil)
	provider.loadStored = func(string) ([]appcookies.Record, error) {
		persistenceLoads.Add(1)
		return nil, appsessions.ErrNoCookies
	}
	provider.loadRuntime = func() ([]appcookies.Record, error) {
		return live, nil
	}
	provider.saveStored = func(_ string, records []appcookies.Record) error {
		saved = append([]appcookies.Record(nil), records...)
		return nil
	}
	provider.MarkWebKitReady()

	if err := provider.EnsureAppSessionRuntimeCookies(context.Background(), "youtube", true); err != nil {
		t.Fatalf("recover connected live store: %v", err)
	}
	records, err := provider.LoadAppSessionCookies(context.Background(), "youtube")
	if err != nil || len(records) != 3 {
		t.Fatalf("recovered runtime = %#v, err=%v", records, err)
	}
	if persistenceLoads.Load() != 1 {
		t.Fatalf("vault bootstrap loads = %d, want 1", persistenceLoads.Load())
	}
	expectedSaved := nativeYouTubePersistentCookies(live, time.Now())
	if !slices.Equal(saved, expectedSaved) {
		t.Fatalf("bootstrap persistence = %#v, want %#v", saved, expectedSaved)
	}
	if epoch := provider.AppSessionCookieSyncEpoch("youtube"); epoch != 1 {
		t.Fatalf("bootstrap epoch = %d, want 1", epoch)
	}
}

func TestNativeAppSessionProviderDoesNotBootstrapGoogleOnlyCookies(t *testing.T) {
	provider := NewNativeAppSessionProvider(nil)
	provider.loadStored = func(string) ([]appcookies.Record, error) {
		return nil, appsessions.ErrNoCookies
	}
	provider.loadRuntime = func() ([]appcookies.Record, error) {
		return []appcookies.Record{{
			Name: "SAPISID", Value: "google-only", Domain: ".google.com", Path: "/", Expires: 4_102_444_800,
		}}, nil
	}
	var saves atomic.Int32
	provider.saveStored = func(string, []appcookies.Record) error {
		saves.Add(1)
		return nil
	}
	provider.MarkWebKitReady()

	err := provider.EnsureAppSessionRuntimeCookies(context.Background(), "youtube", true)
	if !errors.Is(err, appsessions.ErrNoCookies) {
		t.Fatalf("Google-only bootstrap error = %v, want no cookies", err)
	}
	if saves.Load() != 0 || provider.AppSessionCookieSyncEpoch("youtube") != 0 {
		t.Fatalf("Google-only cookies created a YouTube session: saves=%d epoch=%d", saves.Load(), provider.AppSessionCookieSyncEpoch("youtube"))
	}
}

func TestNativeAppSessionProviderDoesNotReviveDisconnectedLiveStore(t *testing.T) {
	provider := NewNativeAppSessionProvider(nil)
	provider.loadStored = func(string) ([]appcookies.Record, error) {
		return nil, appsessions.ErrNoCookies
	}
	var runtimeLoads atomic.Int32
	provider.loadRuntime = func() ([]appcookies.Record, error) {
		runtimeLoads.Add(1)
		return []appcookies.Record{{
			Name: "SAPISID", Value: "stale-live", Domain: ".youtube.com", Path: "/", Expires: 4_102_444_800,
		}}, nil
	}
	provider.MarkWebKitReady()

	err := provider.EnsureAppSessionRuntimeCookies(context.Background(), "youtube", false)
	if !errors.Is(err, appsessions.ErrNoCookies) {
		t.Fatalf("disconnected hydration error = %v, want no cookies", err)
	}
	if runtimeLoads.Load() != 0 {
		t.Fatalf("disconnected session read stale WebKit cookies %d times", runtimeLoads.Load())
	}
}

func TestNativeAppSessionProviderClearBlocksLiveHydration(t *testing.T) {
	provider := NewNativeAppSessionProvider(nil)
	provider.loadStored = func(string) ([]appcookies.Record, error) {
		return []appcookies.Record{{
			Name: "SAPISID", Value: "stored", Domain: ".youtube.com", Path: "/", Expires: 4_102_444_800,
		}}, nil
	}
	provider.clearStored = func(string, []string) error { return nil }
	var runtimeLoads atomic.Int32
	provider.loadRuntime = func() ([]appcookies.Record, error) {
		runtimeLoads.Add(1)
		return []appcookies.Record{{
			Name: "SAPISID", Value: "stale-live", Domain: ".youtube.com", Path: "/", Expires: 4_102_444_800,
		}}, nil
	}
	provider.MarkWebKitReady()
	if _, err := provider.LoadAppSessionCookies(context.Background(), "youtube"); err != nil {
		t.Fatalf("prime provider: %v", err)
	}
	if err := provider.ClearAppSession(context.Background(), "youtube", []string{"youtube.com"}); err != nil {
		t.Fatalf("clear provider: %v", err)
	}

	err := provider.EnsureAppSessionRuntimeCookies(context.Background(), "youtube", true)
	if !errors.Is(err, appsessions.ErrNoCookies) {
		t.Fatalf("post-clear hydration error = %v, want no cookies", err)
	}
	if runtimeLoads.Load() != 0 {
		t.Fatalf("post-clear hydration read stale WebKit cookies %d times", runtimeLoads.Load())
	}
}

func TestNativeAppSessionProviderSlowAuthoritativeHydrationCannotOverwriteSave(t *testing.T) {
	if runtime.GOOS != "darwin" {
		t.Skip("authoritative WebKit hydration is the macOS request path")
	}
	provider := NewNativeAppSessionProvider(nil)
	provider.saveStored = func(string, []appcookies.Record) error { return nil }
	started := make(chan struct{})
	release := make(chan struct{})
	provider.loadRuntime = func() ([]appcookies.Record, error) {
		close(started)
		<-release
		return testYouTubeAuthSnapshot("stale-webkit"), nil
	}
	provider.MarkWebKitReady()

	done := make(chan error, 1)
	go func() {
		done <- provider.EnsureAppSessionRuntimeCookies(context.Background(), "youtube", true)
	}()
	<-started
	if err := provider.SaveAppSessionCookies(context.Background(), "youtube", testYouTubeAuthSnapshot("fresh-save")); err != nil {
		t.Fatalf("concurrent save: %v", err)
	}
	close(release)
	if err := <-done; !errors.Is(err, appsessions.ErrNoCookies) {
		t.Fatalf("superseded hydration error = %v, want no cookies", err)
	}
	records, err := provider.LoadAppSessionCookies(context.Background(), "youtube")
	if err != nil || len(records) != 1 || records[0].Value != "fresh-save" {
		t.Fatalf("slow hydration overwrote save: records=%#v err=%v", records, err)
	}
	currentEpoch := provider.AppSessionCookieSyncEpoch("youtube")
	if provider.hydratedEpoch == currentEpoch {
		t.Fatalf("superseded hydration claimed fresh epoch %d", currentEpoch)
	}
}

func TestNativeAppSessionProviderSlowAuthoritativeHydrationCannotOverwriteNewerSync(t *testing.T) {
	provider := &NativeAppSessionProvider{
		saveStored: func(string, []appcookies.Record) error { return nil },
	}
	if err := provider.SaveAppSessionCookies(context.Background(), "youtube", testYouTubeAuthSnapshot("initial")); err != nil {
		t.Fatalf("prime provider: %v", err)
	}
	started := make(chan struct{})
	release := make(chan struct{})
	provider.loadRuntime = func() ([]appcookies.Record, error) {
		close(started)
		<-release
		return testYouTubeAuthSnapshot("stale-hydration"), nil
	}
	done := make(chan error, 1)
	go func() {
		_, err := provider.hydrateAuthoritativeYouTubeRuntimeCookies(context.Background(), true)
		done <- err
	}()
	<-started
	epoch, newerSequence := nextYouTubeCookieSyncToken(t, provider)
	if err := provider.SyncAppSessionCookies(
		context.Background(), "youtube", testYouTubeAuthSnapshot("fresh-sync"), epoch, newerSequence,
	); err != nil {
		t.Fatalf("apply newer sync: %v", err)
	}
	close(release)
	if err := <-done; !errors.Is(err, appsessions.ErrNoCookies) {
		t.Fatalf("superseded hydration error = %v, want no cookies", err)
	}
	records, err := provider.LoadAppSessionCookies(context.Background(), "youtube")
	if err != nil || len(records) != 1 || records[0].Value != "fresh-sync" {
		t.Fatalf("slow hydration overwrote newer sync: records=%#v err=%v", records, err)
	}
	if provider.youtubeRuntimeAppliedSequence != newerSequence {
		t.Fatalf("applied sequence = %d, want newer %d", provider.youtubeRuntimeAppliedSequence, newerSequence)
	}
}

func TestNativeAppSessionProviderAuthoritativeHydrationRejectsOlderSyncToken(t *testing.T) {
	provider := &NativeAppSessionProvider{
		saveStored: func(string, []appcookies.Record) error { return nil },
		loadRuntime: func() ([]appcookies.Record, error) {
			return testYouTubeAuthSnapshot("fresh-hydration"), nil
		},
	}
	if err := provider.SaveAppSessionCookies(context.Background(), "youtube", testYouTubeAuthSnapshot("initial")); err != nil {
		t.Fatalf("prime provider: %v", err)
	}
	oldEpoch, oldSequence := nextYouTubeCookieSyncToken(t, provider)
	if _, err := provider.hydrateAuthoritativeYouTubeRuntimeCookies(context.Background(), true); err != nil {
		t.Fatalf("apply authoritative hydration: %v", err)
	}
	if err := provider.SyncAppSessionCookies(
		context.Background(), "youtube", testYouTubeAuthSnapshot("stale-sync"), oldEpoch, oldSequence,
	); !errors.Is(err, appsessions.ErrNoCookies) {
		t.Fatalf("older sync error = %v, want no cookies", err)
	}
	records, err := provider.LoadAppSessionCookies(context.Background(), "youtube")
	if err != nil || len(records) != 1 || records[0].Value != "fresh-hydration" {
		t.Fatalf("older sync overwrote hydration: records=%#v err=%v", records, err)
	}
}

func TestNativeAppSessionProviderClearInvalidatesInFlightAuthoritativeHydration(t *testing.T) {
	provider := &NativeAppSessionProvider{
		saveStored:  func(string, []appcookies.Record) error { return nil },
		clearStored: func(string, []string) error { return nil },
	}
	if err := provider.SaveAppSessionCookies(context.Background(), "youtube", testYouTubeAuthSnapshot("initial")); err != nil {
		t.Fatalf("prime provider: %v", err)
	}
	started := make(chan struct{})
	release := make(chan struct{})
	provider.loadRuntime = func() ([]appcookies.Record, error) {
		close(started)
		<-release
		return testYouTubeAuthSnapshot("stale-webkit"), nil
	}
	done := make(chan error, 1)
	go func() {
		_, err := provider.hydrateAuthoritativeYouTubeRuntimeCookies(context.Background(), true)
		done <- err
	}()
	<-started
	if err := provider.ClearAppSession(context.Background(), "youtube", []string{"youtube.com"}); err != nil {
		t.Fatalf("clear provider: %v", err)
	}
	close(release)
	if err := <-done; !errors.Is(err, appsessions.ErrNoCookies) {
		t.Fatalf("post-clear hydration error = %v, want no cookies", err)
	}
	if provider.youtubeRuntimeSyncAllowed {
		t.Fatal("in-flight hydration re-enabled a cleared session")
	}
	if records, err := provider.LoadAppSessionCookies(context.Background(), "youtube"); !errors.Is(err, appsessions.ErrNoCookies) || len(records) != 0 {
		t.Fatalf("in-flight hydration repopulated clear: records=%#v err=%v", records, err)
	}
}

func TestNativeAppSessionProviderAuthoritativeEmptyBlocksKeychainFallback(t *testing.T) {
	tests := []struct {
		name    string
		runtime []appcookies.Record
	}{
		{name: "empty"},
		{name: "google-only", runtime: []appcookies.Record{{
			Name: "SAPISID", Value: "not-youtube", Domain: ".google.com", Path: "/", Expires: 4_102_444_800,
		}}},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			var keychainLoads atomic.Int32
			provider := &NativeAppSessionProvider{
				loadStored: func(string) ([]appcookies.Record, error) {
					keychainLoads.Add(1)
					return testYouTubeAuthSnapshot("stale-keychain"), nil
				},
				loadRuntime: func() ([]appcookies.Record, error) {
					return test.runtime, nil
				},
			}
			if _, err := provider.hydrateAuthoritativeYouTubeRuntimeCookies(context.Background(), true); !errors.Is(err, appsessions.ErrNoCookies) {
				t.Fatalf("authoritative empty error = %v, want no cookies", err)
			}
			if records, err := provider.LoadAppSessionCookies(context.Background(), "youtube"); !errors.Is(err, appsessions.ErrNoCookies) || len(records) != 0 {
				t.Fatalf("authoritative empty fell back: records=%#v err=%v", records, err)
			}
			if keychainLoads.Load() != 0 {
				t.Fatalf("authoritative empty read Keychain %d times", keychainLoads.Load())
			}
		})
	}
}

func TestNativeAppSessionProviderAuthoritativeReadFailureBlocksKeychainFallback(t *testing.T) {
	var keychainLoads atomic.Int32
	webkitErr := errors.New("WebKit cookie store unavailable")
	provider := &NativeAppSessionProvider{
		loadStored: func(string) ([]appcookies.Record, error) {
			keychainLoads.Add(1)
			return testYouTubeAuthSnapshot("stale-keychain"), nil
		},
		loadRuntime: func() ([]appcookies.Record, error) {
			return nil, webkitErr
		},
	}
	if _, err := provider.hydrateAuthoritativeYouTubeRuntimeCookies(context.Background(), true); !errors.Is(err, webkitErr) {
		t.Fatalf("authoritative read failure = %v, want %v", err, webkitErr)
	}
	if records, err := provider.LoadAppSessionCookies(context.Background(), "youtube"); !errors.Is(err, appsessions.ErrNoCookies) || len(records) != 0 {
		t.Fatalf("failed WebKit read fell back: records=%#v err=%v", records, err)
	}
	if keychainLoads.Load() != 0 {
		t.Fatalf("failed WebKit read touched Keychain %d times", keychainLoads.Load())
	}
}

func TestNativeAppSessionProviderAuthoritativeReadFailurePreservesValidatedCache(t *testing.T) {
	webkitErr := errors.New("WebKit cookie store unavailable")
	provider := &NativeAppSessionProvider{
		saveStored: func(string, []appcookies.Record) error { return nil },
		loadRuntime: func() ([]appcookies.Record, error) {
			return nil, webkitErr
		},
	}
	if err := provider.SaveAppSessionCookies(context.Background(), "youtube", testYouTubeAuthSnapshot("validated-cache")); err != nil {
		t.Fatalf("prime provider: %v", err)
	}
	if _, err := provider.hydrateAuthoritativeYouTubeRuntimeCookies(context.Background(), true); !errors.Is(err, webkitErr) {
		t.Fatalf("authoritative read failure = %v, want %v", err, webkitErr)
	}
	records, err := provider.LoadAppSessionCookies(context.Background(), "youtube")
	if err != nil || len(records) != 1 || records[0].Value != "validated-cache" {
		t.Fatalf("WebKit read failure discarded validated cache: records=%#v err=%v", records, err)
	}
}

func TestNativeAppSessionProviderDisconnectedAuthoritativeStateBlocksKeychainFallback(t *testing.T) {
	var keychainLoads atomic.Int32
	var runtimeLoads atomic.Int32
	provider := &NativeAppSessionProvider{
		loadStored: func(string) ([]appcookies.Record, error) {
			keychainLoads.Add(1)
			return testYouTubeAuthSnapshot("stale-keychain"), nil
		},
		loadRuntime: func() ([]appcookies.Record, error) {
			runtimeLoads.Add(1)
			return testYouTubeAuthSnapshot("stale-webkit"), nil
		},
	}
	if _, err := provider.hydrateAuthoritativeYouTubeRuntimeCookies(context.Background(), false); !errors.Is(err, appsessions.ErrNoCookies) {
		t.Fatalf("disconnected authoritative state error = %v, want no cookies", err)
	}
	if records, err := provider.LoadAppSessionCookies(context.Background(), "youtube"); !errors.Is(err, appsessions.ErrNoCookies) || len(records) != 0 {
		t.Fatalf("disconnected state fell back: records=%#v err=%v", records, err)
	}
	if runtimeLoads.Load() != 0 || keychainLoads.Load() != 0 {
		t.Fatalf("disconnected state touched stores: runtime=%d keychain=%d", runtimeLoads.Load(), keychainLoads.Load())
	}
}

func TestNativeAppSessionProviderHydrationGateHonoursContext(t *testing.T) {
	provider := NewNativeAppSessionProvider(nil)
	provider.loadStored = func(string) ([]appcookies.Record, error) {
		return []appcookies.Record{{
			Name: "SAPISID", Value: "stored", Domain: ".youtube.com", Path: "/", Expires: 4_102_444_800,
		}}, nil
	}
	started := make(chan struct{})
	release := make(chan struct{})
	provider.loadRuntime = func() ([]appcookies.Record, error) {
		close(started)
		<-release
		return []appcookies.Record{{
			Name: "SAPISID", Value: "stored", Domain: ".youtube.com", Path: "/", Expires: 4_102_444_800,
		}}, nil
	}
	provider.saveStored = func(string, []appcookies.Record) error { return nil }
	provider.MarkWebKitReady()
	firstDone := make(chan error, 1)
	go func() {
		firstDone <- provider.EnsureAppSessionRuntimeCookies(context.Background(), "youtube", true)
	}()
	<-started

	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Millisecond)
	defer cancel()
	if err := provider.EnsureAppSessionRuntimeCookies(ctx, "youtube", true); !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("contended hydration error = %v, want deadline exceeded", err)
	}
	close(release)
	if err := <-firstDone; err != nil {
		t.Fatalf("first hydration failed: %v", err)
	}
}

func TestNativeAppSessionProviderSyncSeparatesRuntimeAndPersistentCookies(t *testing.T) {
	var saved [][]appcookies.Record
	provider := &NativeAppSessionProvider{
		loadStored: func(string) ([]appcookies.Record, error) {
			return []appcookies.Record{
				{Name: "SAPISID", Value: "stable", Domain: ".youtube.com", Path: "/", Expires: 4_102_444_800},
				{Name: "SIDCC", Value: "legacy-rotating", Domain: ".youtube.com", Path: "/", Expires: 4_102_444_800},
			}, nil
		},
		saveStored: func(_ string, records []appcookies.Record) error {
			saved = append(saved, append([]appcookies.Record(nil), records...))
			return nil
		},
	}
	if _, err := provider.LoadAppSessionCookies(context.Background(), "youtube"); err != nil {
		t.Fatalf("prime YouTube persistence: %v", err)
	}
	epoch, sequence := nextYouTubeCookieSyncToken(t, provider)
	live := []appcookies.Record{
		{Name: "SAPISID", Value: "stable", Domain: ".youtube.com", Path: "/", Expires: 4_102_444_800},
		{Name: "LOGIN_INFO", Value: "live-only", Domain: ".youtube.com", Path: "/", Expires: 4_102_444_800},
		{Name: "__Secure-3PSIDCC", Value: "rotating-cc", Domain: ".youtube.com", Path: "/", Expires: 4_102_444_800},
		{Name: "__Secure-3PSIDTS", Value: "rotating-ts", Domain: ".youtube.com", Path: "/", Expires: 4_102_444_800},
	}
	if err := provider.SyncAppSessionCookies(context.Background(), "youtube", live, epoch, sequence); err != nil {
		t.Fatalf("sync live YouTube cookies: %v", err)
	}
	expectedSaved := nativeYouTubePersistentCookies(live, time.Now())
	expectedSaveCount := 1
	if len(saved) != expectedSaveCount ||
		(expectedSaveCount == 1 && !slices.Equal(saved[0], expectedSaved)) {
		t.Fatalf("persistent snapshots = %#v, want count %d and snapshot %#v", saved, expectedSaveCount, expectedSaved)
	}
	runtimeRecords, err := provider.LoadAppSessionCookies(context.Background(), "youtube")
	if err != nil {
		t.Fatalf("load runtime snapshot: %v", err)
	}
	if len(runtimeRecords) != 4 {
		t.Fatalf("runtime request cache did not retain the complete live snapshot: %#v", runtimeRecords)
	}
	rotatingCount := 0
	for _, record := range runtimeRecords {
		switch record.Name {
		case "SIDCC", "__Secure-1PSIDCC", "__Secure-1PSIDTS", "__Secure-3PSIDCC", "__Secure-3PSIDTS":
			rotatingCount++
		}
	}
	if rotatingCount != 2 {
		t.Fatalf("runtime request cache rotating count = %d, want 2: %#v", rotatingCount, runtimeRecords)
	}
	epoch, sequence = nextYouTubeCookieSyncToken(t, provider)
	if err := provider.SyncAppSessionCookies(context.Background(), "youtube", live, epoch, sequence); err != nil {
		t.Fatalf("repeat sync: %v", err)
	}
	if len(saved) != expectedSaveCount {
		t.Fatalf("unchanged stable snapshot rewrote persistence: got %d writes, want %d", len(saved), expectedSaveCount)
	}
}

func TestNativeAppSessionProviderRejectsOutOfOrderRuntimeSnapshot(t *testing.T) {
	provider := &NativeAppSessionProvider{
		loadStored: func(string) ([]appcookies.Record, error) {
			return []appcookies.Record{{
				Name: "SAPISID", Value: "initial", Domain: ".youtube.com", Path: "/", Expires: 4_102_444_800,
			}}, nil
		},
		saveStored: func(string, []appcookies.Record) error { return nil },
	}
	if _, err := provider.LoadAppSessionCookies(context.Background(), "youtube"); err != nil {
		t.Fatalf("prime provider: %v", err)
	}
	olderEpoch, olderSequence := nextYouTubeCookieSyncToken(t, provider)
	newerEpoch, newerSequence := nextYouTubeCookieSyncToken(t, provider)
	if newerEpoch != olderEpoch || newerSequence <= olderSequence {
		t.Fatalf("non-monotonic sync tokens: older=(%d,%d) newer=(%d,%d)", olderEpoch, olderSequence, newerEpoch, newerSequence)
	}
	newer := []appcookies.Record{{
		Name: "SAPISID", Value: "generation-b", Domain: ".youtube.com", Path: "/", Expires: 4_102_444_800,
	}}
	if err := provider.SyncAppSessionCookies(
		context.Background(), "youtube", newer, newerEpoch, newerSequence,
	); err != nil {
		t.Fatalf("apply newer snapshot: %v", err)
	}
	older := []appcookies.Record{{
		Name: "SAPISID", Value: "generation-a", Domain: ".youtube.com", Path: "/", Expires: 4_102_444_800,
	}}
	if err := provider.SyncAppSessionCookies(
		context.Background(), "youtube", older, olderEpoch, olderSequence,
	); !errors.Is(err, appsessions.ErrNoCookies) {
		t.Fatalf("out-of-order snapshot error = %v, want no cookies", err)
	}
	records, err := provider.LoadAppSessionCookies(context.Background(), "youtube")
	if err != nil || len(records) != 1 || records[0].Value != "generation-b" {
		t.Fatalf("out-of-order snapshot rolled runtime cache back: records=%#v err=%v", records, err)
	}
}

func TestNativeAppSessionProviderRejectsIncompleteRuntimeSnapshot(t *testing.T) {
	var saves atomic.Int32
	provider := &NativeAppSessionProvider{
		loadStored: func(string) ([]appcookies.Record, error) {
			return []appcookies.Record{{
				Name: "SAPISID", Value: "youtube-auth", Domain: ".youtube.com", Path: "/", Expires: 4_102_444_800,
			}}, nil
		},
		saveStored: func(string, []appcookies.Record) error {
			saves.Add(1)
			return nil
		},
	}
	if _, err := provider.LoadAppSessionCookies(context.Background(), "youtube"); err != nil {
		t.Fatalf("prime provider: %v", err)
	}
	epoch, sequence := nextYouTubeCookieSyncToken(t, provider)
	incomplete := []appcookies.Record{
		{Name: "SAPISID", Value: "google-only", Domain: ".google.com", Path: "/", Expires: 4_102_444_800},
		{Name: "LOGIN_INFO", Value: "not-auth-for-youtube", Domain: ".youtube.com", Path: "/", Expires: 4_102_444_800},
	}
	if err := provider.SyncAppSessionCookies(
		context.Background(), "youtube", incomplete, epoch, sequence,
	); !errors.Is(err, appsessions.ErrNoCookies) {
		t.Fatalf("incomplete snapshot error = %v, want no cookies", err)
	}
	if saves.Load() != 0 {
		t.Fatalf("incomplete snapshot rewrote persistence %d times", saves.Load())
	}
	records, err := provider.LoadAppSessionCookies(context.Background(), "youtube")
	if err != nil || len(records) != 1 || records[0].Value != "youtube-auth" {
		t.Fatalf("incomplete snapshot replaced authenticated cache: records=%#v err=%v", records, err)
	}
}

func TestNativeAppSessionProviderDoesNotRewriteCanonicalPersistence(t *testing.T) {
	var saveCount int
	live := []appcookies.Record{
		{Name: "SAPISID", Value: "stable", Domain: ".youtube.com", Path: "/", Expires: 4_102_444_800},
		{Name: "LOGIN_INFO", Value: "live", Domain: ".youtube.com", Path: "/", Expires: 4_102_444_800},
	}
	persisted := nativeYouTubePersistentCookies(live, time.Now())
	provider := &NativeAppSessionProvider{
		loadStored: func(string) ([]appcookies.Record, error) {
			return persisted, nil
		},
		saveStored: func(string, []appcookies.Record) error {
			saveCount++
			return nil
		},
	}
	if _, err := provider.LoadAppSessionCookies(context.Background(), "youtube"); err != nil {
		t.Fatalf("prime canonical persistence: %v", err)
	}
	epoch, sequence := nextYouTubeCookieSyncToken(t, provider)
	if err := provider.SyncAppSessionCookies(context.Background(), "youtube", live, epoch, sequence); err != nil {
		t.Fatalf("sync canonical persistence: %v", err)
	}
	if saveCount != 0 {
		t.Fatalf("canonical snapshot rewrote persistence %d times", saveCount)
	}
}

func TestNativeAppSessionProviderClearBlocksDelayedRuntimeSync(t *testing.T) {
	var saveCount atomic.Int32
	provider := &NativeAppSessionProvider{
		loadStored: func(string) ([]appcookies.Record, error) {
			return []appcookies.Record{{Name: "SAPISID", Value: "stable", Domain: ".youtube.com", Path: "/", Expires: 4_102_444_800}}, nil
		},
		saveStored: func(string, []appcookies.Record) error {
			saveCount.Add(1)
			return nil
		},
		clearStored: func(string, []string) error { return nil },
	}
	if _, err := provider.LoadAppSessionCookies(context.Background(), "youtube"); err != nil {
		t.Fatalf("prime provider: %v", err)
	}
	epoch, sequence := nextYouTubeCookieSyncToken(t, provider)
	if err := provider.ClearAppSession(context.Background(), "youtube", []string{"youtube.com"}); err != nil {
		t.Fatalf("clear provider: %v", err)
	}
	err := provider.SyncAppSessionCookies(context.Background(), "youtube", []appcookies.Record{
		{Name: "SAPISID", Value: "stale-delayed", Domain: ".youtube.com", Path: "/", Expires: 4_102_444_800},
	}, epoch, sequence)
	if !errors.Is(err, appsessions.ErrNoCookies) {
		t.Fatal("delayed runtime sync recreated a cleared session")
	}
	if saveCount.Load() != 0 {
		t.Fatalf("delayed sync wrote Keychain after clear: %d", saveCount.Load())
	}
}

func TestNativeAppSessionProviderRetriesFailedStablePersistence(t *testing.T) {
	var attempts atomic.Int32
	provider := &NativeAppSessionProvider{
		loadStored: func(string) ([]appcookies.Record, error) {
			return []appcookies.Record{{Name: "SAPISID", Value: "stable-a", Domain: ".youtube.com", Path: "/", Expires: 4_102_444_800}}, nil
		},
		saveStored: func(string, []appcookies.Record) error {
			if attempts.Add(1) == 1 {
				return errors.New("transient vault failure")
			}
			return nil
		},
	}
	if _, err := provider.LoadAppSessionCookies(context.Background(), "youtube"); err != nil {
		t.Fatalf("prime provider: %v", err)
	}
	epoch, sequence := nextYouTubeCookieSyncToken(t, provider)
	live := []appcookies.Record{{Name: "SAPISID", Value: "stable-b", Domain: ".youtube.com", Path: "/", Expires: 4_102_444_800}}
	if err := provider.SyncAppSessionCookies(context.Background(), "youtube", live, epoch, sequence); err == nil {
		t.Fatal("expected first stable persistence attempt to fail")
	}
	epoch, sequence = nextYouTubeCookieSyncToken(t, provider)
	if err := provider.SyncAppSessionCookies(context.Background(), "youtube", live, epoch, sequence); err != nil {
		t.Fatalf("stable persistence was not retried: %v", err)
	}
	if attempts.Load() != 2 {
		t.Fatalf("stable persistence attempts = %d, want 2", attempts.Load())
	}
}

func TestNativeAppSessionCookieCacheRemembersNoCookies(t *testing.T) {
	var cache nativeAppSessionCookieCache
	var calls atomic.Int32
	loader := func() ([]appcookies.Record, bool, error) {
		calls.Add(1)
		return nil, false, nil
	}

	for range 3 {
		if records, available, err := cache.load("youtube", loader); err != nil || available || len(records) != 0 {
			t.Fatalf("empty cache result = (%#v, %t), want unavailable", records, available)
		}
	}
	if calls.Load() != 1 {
		t.Fatalf("empty loader calls = %d, want 1", calls.Load())
	}
}

func TestNativeAppSessionCookieCacheStoreAndClearOwnTheirSlices(t *testing.T) {
	var cache nativeAppSessionCookieCache
	stored := []appcookies.Record{{Name: "SAPISID", Value: "saved-secret"}}
	cache.store("youtube", stored)
	stored[0].Value = "mutated-input"

	loaderCalled := false
	loaded, available, err := cache.load("youtube", func() ([]appcookies.Record, bool, error) {
		loaderCalled = true
		return nil, false, nil
	})
	if err != nil {
		t.Fatalf("load stored cache: %v", err)
	}
	if loaderCalled || !available || len(loaded) != 1 || loaded[0].Value != "saved-secret" {
		t.Fatalf("stored cache entry was not isolated: %#v available=%t", loaded, available)
	}

	cache.store("youtube", nil)
	loaded, available, err = cache.load("youtube", func() ([]appcookies.Record, bool, error) {
		loaderCalled = true
		return stored, true, nil
	})
	if err != nil {
		t.Fatalf("load cleared cache: %v", err)
	}
	if loaderCalled || available || len(loaded) != 0 {
		t.Fatalf("cleared cache entry = (%#v, %t), want loaded empty", loaded, available)
	}
}

func TestNativeAppSessionCookieCacheCoalescesConcurrentLoads(t *testing.T) {
	var cache nativeAppSessionCookieCache
	var calls atomic.Int32
	started := make(chan struct{})
	release := make(chan struct{})
	loader := func() ([]appcookies.Record, bool, error) {
		if calls.Add(1) == 1 {
			close(started)
		}
		<-release
		return []appcookies.Record{{Name: "SID", Value: "shared-secret"}}, true, nil
	}

	const readers = 64
	start := make(chan struct{})
	results := make(chan []appcookies.Record, readers)
	var wait sync.WaitGroup
	for range readers {
		wait.Add(1)
		go func() {
			defer wait.Done()
			<-start
			records, available, err := cache.load("youtube", loader)
			if err != nil || !available {
				results <- nil
				return
			}
			results <- records
		}()
	}
	close(start)
	<-started
	close(release)
	wait.Wait()
	close(results)

	if calls.Load() != 1 {
		t.Fatalf("concurrent loader calls = %d, want 1", calls.Load())
	}
	for records := range results {
		if len(records) != 1 || records[0].Value != "shared-secret" {
			t.Fatalf("unexpected concurrent result: %#v", records)
		}
	}
}

func TestNativeAppSessionCookieCacheCoalescedWaitHonorsContext(t *testing.T) {
	var cache nativeAppSessionCookieCache
	started := make(chan struct{})
	release := make(chan struct{})
	firstDone := make(chan struct{})
	go func() {
		defer close(firstDone)
		_, _, _ = cache.load("youtube", func() ([]appcookies.Record, bool, error) {
			close(started)
			<-release
			return []appcookies.Record{{Name: "SID", Value: "secret"}}, true, nil
		})
	}()
	<-started

	ctx, cancel := context.WithCancel(context.Background())
	waiter := make(chan error, 1)
	go func() {
		_, _, err := cache.loadContext(ctx, "youtube", nil)
		waiter <- err
	}()
	// Give the coalesced reader time to enter the in-flight wait before
	// cancellation; it must return without waiting for the keychain loader.
	time.Sleep(20 * time.Millisecond)
	cancel()
	select {
	case err := <-waiter:
		if !errors.Is(err, context.Canceled) {
			t.Fatalf("coalesced wait error = %v, want context.Canceled", err)
		}
	case <-time.After(time.Second):
		t.Fatal("coalesced cache wait ignored context cancellation")
	}
	close(release)
	<-firstDone
}

func TestNativeAppSessionCookieCacheStoreWinsInFlightLoad(t *testing.T) {
	var cache nativeAppSessionCookieCache
	started := make(chan struct{})
	release := make(chan struct{})
	result := make(chan []appcookies.Record, 1)
	go func() {
		records, _, _ := cache.load("youtube", func() ([]appcookies.Record, bool, error) {
			close(started)
			<-release
			return []appcookies.Record{{Name: "SID", Value: "stale-secret"}}, true, nil
		})
		result <- records
	}()

	<-started
	cache.store("youtube", []appcookies.Record{{Name: "SID", Value: "fresh-secret"}})
	close(release)
	loaded := <-result
	if len(loaded) != 1 || loaded[0].Value != "fresh-secret" {
		t.Fatalf("in-flight load overwrote a successful store: %#v", loaded)
	}

	loaded, available, err := cache.load("youtube", nil)
	if err != nil {
		t.Fatalf("load fresh cache: %v", err)
	}
	if !available || len(loaded) != 1 || loaded[0].Value != "fresh-secret" {
		t.Fatalf("fresh cache value was not retained: %#v", loaded)
	}
}

func TestNativeAppSessionCookieCacheClearWinsInFlightLoad(t *testing.T) {
	var cache nativeAppSessionCookieCache
	started := make(chan struct{})
	release := make(chan struct{})
	type loadResult struct {
		records   []appcookies.Record
		available bool
	}
	result := make(chan loadResult, 1)
	go func() {
		records, available, _ := cache.load("youtube", func() ([]appcookies.Record, bool, error) {
			close(started)
			<-release
			return []appcookies.Record{{Name: "SID", Value: "stale-secret"}}, true, nil
		})
		result <- loadResult{records: records, available: available}
	}()

	<-started
	cache.store("youtube", nil)
	close(release)
	loaded := <-result
	if loaded.available || len(loaded.records) != 0 {
		t.Fatalf("in-flight load repopulated a cleared cache: %#v", loaded)
	}

	records, available, err := cache.load("youtube", nil)
	if err != nil {
		t.Fatalf("load cleared cache: %v", err)
	}
	if available || len(records) != 0 {
		t.Fatalf("cleared cache value was not retained: %#v", records)
	}
}

func TestNativeAppSessionCookieCacheRetriesTransientLoadFailure(t *testing.T) {
	var cache nativeAppSessionCookieCache
	var calls atomic.Int32
	transientErr := errors.New("keychain is locked")
	loader := func() ([]appcookies.Record, bool, error) {
		if calls.Add(1) == 1 {
			return nil, false, transientErr
		}
		return []appcookies.Record{{Name: "SID", Value: "unlocked-secret"}}, true, nil
	}

	if records, available, err := cache.load("youtube", loader); !errors.Is(err, transientErr) || available || len(records) != 0 {
		t.Fatalf("transient result = (%#v, %t, %v)", records, available, err)
	}
	records, available, err := cache.load("youtube", loader)
	if err != nil || !available || len(records) != 1 || records[0].Value != "unlocked-secret" {
		t.Fatalf("retry result = (%#v, %t, %v)", records, available, err)
	}
	if calls.Load() != 2 {
		t.Fatalf("loader calls = %d, want retry after transient failure", calls.Load())
	}

	if _, _, err := cache.load("youtube", loader); err != nil {
		t.Fatalf("cached successful retry: %v", err)
	}
	if calls.Load() != 2 {
		t.Fatalf("successful retry was not cached; loader calls = %d", calls.Load())
	}
}
