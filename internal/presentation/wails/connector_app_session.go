package wails

import (
	"context"
	"errors"
	"strings"
	"sync"
	"time"

	"github.com/wailsapp/wails/v3/pkg/application"
	"github.com/wailsapp/wails/v3/pkg/events"
	"go.uber.org/zap"

	appsessionidentity "xiadown/internal/application/appsessions"
	appsessionsservice "xiadown/internal/application/appsessions/service"
	appcookies "xiadown/internal/application/cookies"
	"xiadown/internal/application/youtubecookies"
	"xiadown/internal/domain/appsessions"
)

const (
	connectorAppSessionBlankURL            = "about:blank"
	youtubeRuntimeHydrationFreshness       = 5 * time.Second
	youtubeRuntimeHydrationFailureCooldown = 2 * time.Second
)

type NativeAppSessionProvider struct {
	app         *application.App
	secretVault appsessionidentity.SecretVault
	cookieCache nativeAppSessionCookieCache
	loadStored  func(string) ([]appcookies.Record, error)
	saveStored  func(string, []appcookies.Record) error
	clearStored func(string, []string) error
	loadRuntime func() ([]appcookies.Record, error)

	runtimeSyncMu                  sync.Mutex
	youtubeRuntimeSyncAllowed      bool
	youtubeRuntimeSyncEpoch        uint64
	youtubeRuntimeSampleSequence   uint64
	youtubeRuntimeAppliedSequence  uint64
	youtubeNativeStoreRestoreEpoch uint64

	webkitReady     chan struct{}
	webkitReadyOnce sync.Once
	hydrationGateMu sync.Mutex
	hydrationGate   chan struct{}
	hydratedEpoch   uint64
	hydratedAt      time.Time
	hydrationFailed struct {
		epoch uint64
		at    time.Time
		err   error
	}
}

// nativeAppSessionCookieCache keeps vault-backed App Session cookies in
// process memory. An entry exists after the first load even when no cookies
// were found, so read-heavy clients such as InnerTube do not repeatedly ask
// the platform store for the same empty result.
//
// The cache owns every slice it stores and every slice it returns. A per-key
// in-flight entry also coalesces concurrent first reads without serializing
// unrelated site keys behind a platform load.
type nativeAppSessionCookieCache struct {
	mu      sync.Mutex
	entries map[string]*nativeAppSessionCookieCacheEntry
}

type nativeAppSessionCookieCacheEntry struct {
	records   []appcookies.Record
	available bool
	err       error
	loading   bool
	ready     chan struct{}
}

type connectorAppSessionWindow struct {
	mu         sync.Mutex
	window     *application.WebviewWindow
	domains    []string
	done       chan struct{}
	last       []appcookies.Record
	closing    bool
	allowClose bool
	completed  bool
	closeOnce  sync.Once
}

func NewNativeAppSessionProvider(app *application.App, vaults ...appsessionidentity.SecretVault) *NativeAppSessionProvider {
	provider := &NativeAppSessionProvider{
		app:         app,
		webkitReady: make(chan struct{}),
	}
	if len(vaults) > 0 {
		provider.secretVault = vaults[0]
	}
	return provider
}

// MarkWebKitReady releases cookie consumers that may start during bootstrap,
// before application.Run has made the WebKit main queue available.
func (provider *NativeAppSessionProvider) MarkWebKitReady() {
	if provider == nil {
		return
	}
	provider.webkitReadyOnce.Do(func() {
		if provider.webkitReady != nil {
			close(provider.webkitReady)
		}
	})
}

// EnsureAppSessionRuntimeCookies hydrates the request cache from the shared
// native cookie store at most once per freshness window. Routine Music,
// YouTube and Radio use may update this in-memory cache, but it never mutates
// the persisted App Session credential snapshot.
func (provider *NativeAppSessionProvider) EnsureAppSessionRuntimeCookies(
	ctx context.Context,
	siteKey string,
	allowBootstrap bool,
) (resultErr error) {
	if provider == nil || !strings.EqualFold(strings.TrimSpace(siteKey), "youtube") {
		return appsessions.ErrUnsupported
	}
	if ctx == nil {
		ctx = context.Background()
	}
	if ready := provider.webkitReady; ready != nil {
		select {
		case <-ready:
		case <-ctx.Done():
			return ctx.Err()
		}
	}

	releaseHydration, err := provider.acquireHydration(ctx)
	if err != nil {
		return err
	}
	defer releaseHydration()
	if err := ctx.Err(); err != nil {
		return err
	}

	currentEpoch := provider.AppSessionCookieSyncEpoch("youtube")
	if provider.hydrationFailed.err != nil &&
		provider.hydrationFailed.epoch == currentEpoch &&
		time.Since(provider.hydrationFailed.at) < youtubeRuntimeHydrationFailureCooldown {
		return provider.hydrationFailed.err
	}
	defer func() {
		if resultErr == nil {
			provider.hydrationFailed = struct {
				epoch uint64
				at    time.Time
				err   error
			}{}
			return
		}
		if errors.Is(resultErr, context.Canceled) || errors.Is(resultErr, context.DeadlineExceeded) {
			return
		}
		provider.hydrationFailed.epoch = provider.AppSessionCookieSyncEpoch("youtube")
		provider.hydrationFailed.at = time.Now()
		provider.hydrationFailed.err = resultErr
	}()
	if currentEpoch != 0 &&
		provider.hydratedEpoch == currentEpoch &&
		!provider.hydratedAt.IsZero() &&
		time.Since(provider.hydratedAt) < youtubeRuntimeHydrationFreshness {
		return nil
	}
	storedRecords, err := provider.LoadAppSessionCookies(ctx, "youtube")
	if err != nil {
		if !errors.Is(err, appsessions.ErrNoCookies) || currentEpoch != 0 || !allowBootstrap {
			return err
		}
		records, readErr := provider.loadRuntimeCookies()
		if readErr != nil {
			return readErr
		}
		if err := ctx.Err(); err != nil {
			return err
		}
		if bootstrapErr := provider.bootstrapYouTubeRuntimeCookies(records); bootstrapErr != nil {
			return bootstrapErr
		}
		provider.hydratedEpoch = provider.AppSessionCookieSyncEpoch("youtube")
		provider.hydratedAt = time.Now()
		return nil
	}
	records, sampledNativeStore, err := provider.ensureNativeYouTubeStableRestore(ctx, storedRecords)
	if err != nil {
		return err
	}
	epoch, sequence := provider.BeginAppSessionCookieSync("youtube")
	if epoch == 0 {
		return appsessions.ErrNoCookies
	}
	if !sampledNativeStore {
		records, err = provider.loadRuntimeCookies()
		if err != nil {
			return err
		}
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	if err := provider.SyncAppSessionCookies(ctx, "youtube", records, epoch, sequence); err != nil {
		return err
	}
	provider.hydratedEpoch = epoch
	provider.hydratedAt = time.Now()
	return nil
}

// ensureNativeYouTubeStableRestore mirrors Kaset's startup restore boundary:
// the encrypted stable backup may fill identities missing from the shared
// browser store, but it never overwrites values already owned by that store.
// Full runtime cookies enter the store only through explicit login/import or
// normal browser response processing.
func (provider *NativeAppSessionProvider) ensureNativeYouTubeStableRestore(
	ctx context.Context,
	records []appcookies.Record,
) ([]appcookies.Record, bool, error) {
	stable := nativeYouTubePersistentCookies(records, time.Now())
	if len(stable) == 0 {
		return nil, false, appsessions.ErrNoCookies
	}

	provider.runtimeSyncMu.Lock()
	epoch := provider.youtubeRuntimeSyncEpoch
	if !provider.youtubeRuntimeSyncAllowed || epoch == 0 {
		provider.runtimeSyncMu.Unlock()
		return nil, false, appsessions.ErrNoCookies
	}
	if provider.youtubeNativeStoreRestoreEpoch == epoch {
		provider.runtimeSyncMu.Unlock()
		return nil, false, nil
	}
	provider.runtimeSyncMu.Unlock()

	live, err := provider.loadRuntimeCookies()
	if err != nil && !errors.Is(err, appsessions.ErrNoCookies) {
		return nil, true, err
	}
	missing := youtubecookies.MissingStableAuth(stable, live, time.Now())
	if len(missing) > 0 {
		if err := ctx.Err(); err != nil {
			return nil, true, err
		}
		if err := publishNativeYouTubeRuntimeCookies(provider.app, missing); err != nil {
			return nil, true, err
		}
		live = youtubecookies.Runtime(append(live, missing...), time.Now())
	}

	provider.runtimeSyncMu.Lock()
	defer provider.runtimeSyncMu.Unlock()
	if !provider.youtubeRuntimeSyncAllowed || provider.youtubeRuntimeSyncEpoch != epoch {
		return nil, true, appsessions.ErrNoCookies
	}
	provider.youtubeNativeStoreRestoreEpoch = epoch
	return live, true, nil
}

func (provider *NativeAppSessionProvider) hydrateAuthoritativeYouTubeRuntimeCookies(
	ctx context.Context,
	allowBootstrap bool,
) (uint64, error) {
	provider.runtimeSyncMu.Lock()
	expectedEpoch := provider.youtubeRuntimeSyncEpoch
	allowed := provider.youtubeRuntimeSyncAllowed
	if (expectedEpoch == 0 && !allowBootstrap) || (expectedEpoch != 0 && !allowed) {
		// The shared native cookie store is authoritative. Cache its deliberately unavailable
		// state as an empty, already-loaded entry so the caller cannot fall back
		// to a stale compatibility Keychain snapshot.
		provider.cookieCache.store("youtube", nil)
		provider.runtimeSyncMu.Unlock()
		return 0, appsessions.ErrNoCookies
	}
	expectedSequence := provider.nextYouTubeRuntimeSampleSequenceLocked()
	provider.runtimeSyncMu.Unlock()

	records, err := provider.loadRuntimeCookies()
	if err != nil {
		provider.runtimeSyncMu.Lock()
		if provider.authoritativeYouTubeRuntimeSampleCurrentLocked(
			expectedEpoch, expectedSequence, allowBootstrap,
		) {
			// A WebKit infrastructure failure is not evidence that the user is
			// signed out. Preserve an existing validated cache, but make a missing
			// or in-flight persistence entry fail closed so request code cannot
			// revive an unrelated compatibility Keychain snapshot.
			provider.cookieCache.blockPersistenceFallback("youtube")
		}
		provider.runtimeSyncMu.Unlock()
		return 0, err
	}
	if err := ctx.Err(); err != nil {
		return 0, err
	}
	now := time.Now()
	runtimeRecords := youtubecookies.Runtime(records, now)
	hasYouTubeAuth := len(runtimeRecords) > 0 &&
		youtubecookies.HasAuthForURL(runtimeRecords, "https://www.youtube.com/", now)

	provider.runtimeSyncMu.Lock()
	defer provider.runtimeSyncMu.Unlock()
	if !provider.authoritativeYouTubeRuntimeSampleCurrentLocked(
		expectedEpoch, expectedSequence, allowBootstrap,
	) {
		return 0, appsessions.ErrNoCookies
	}
	if !hasYouTubeAuth {
		// A successful WebKit read with no usable YouTube authentication is an
		// authoritative empty observation. Keep the epoch/allowed state so a
		// later, higher-sequence login snapshot may recover the session.
		provider.cookieCache.store("youtube", nil)
		provider.youtubeRuntimeAppliedSequence = expectedSequence
		return 0, appsessions.ErrNoCookies
	}
	if expectedEpoch == 0 {
		provider.youtubeRuntimeSyncEpoch = 1
		expectedEpoch = provider.youtubeRuntimeSyncEpoch
	}
	provider.cookieCache.store("youtube", runtimeRecords)
	provider.youtubeRuntimeSyncAllowed = true
	provider.youtubeRuntimeAppliedSequence = expectedSequence
	return expectedEpoch, nil
}

func (provider *NativeAppSessionProvider) authoritativeYouTubeRuntimeSampleCurrentLocked(
	expectedEpoch uint64,
	expectedSequence uint64,
	allowBootstrap bool,
) bool {
	return provider != nil &&
		expectedSequence != 0 &&
		provider.youtubeRuntimeSyncEpoch == expectedEpoch &&
		expectedSequence > provider.youtubeRuntimeAppliedSequence &&
		((expectedEpoch == 0 && allowBootstrap && !provider.youtubeRuntimeSyncAllowed) ||
			(expectedEpoch != 0 && provider.youtubeRuntimeSyncAllowed))
}

func (provider *NativeAppSessionProvider) acquireHydration(ctx context.Context) (func(), error) {
	provider.hydrationGateMu.Lock()
	if provider.hydrationGate == nil {
		provider.hydrationGate = make(chan struct{}, 1)
		provider.hydrationGate <- struct{}{}
	}
	gate := provider.hydrationGate
	provider.hydrationGateMu.Unlock()

	select {
	case <-gate:
		return func() { gate <- struct{}{} }, nil
	case <-ctx.Done():
		return nil, ctx.Err()
	}
}

// bootstrapYouTubeRuntimeCookies makes an already authenticated shared store
// available to request consumers without reconstructing or replacing the
// persisted App Session credential snapshot.
func (provider *NativeAppSessionProvider) bootstrapYouTubeRuntimeCookies(records []appcookies.Record) error {
	now := time.Now()
	runtimeRecords := youtubecookies.Runtime(records, now)
	persistedRecords := nativeYouTubePersistentCookies(runtimeRecords, now)
	if len(runtimeRecords) == 0 ||
		!youtubecookies.HasAuthForURL(runtimeRecords, "https://www.youtube.com/", now) ||
		len(persistedRecords) == 0 {
		return appsessions.ErrNoCookies
	}

	provider.runtimeSyncMu.Lock()
	defer provider.runtimeSyncMu.Unlock()
	if provider.youtubeRuntimeSyncEpoch != 0 || provider.youtubeRuntimeSyncAllowed {
		return appsessions.ErrNoCookies
	}

	provider.cookieCache.store("youtube", runtimeRecords)
	provider.youtubeRuntimeSyncAllowed = true
	provider.youtubeRuntimeSyncEpoch = 1
	provider.youtubeRuntimeAppliedSequence = 0
	provider.youtubeNativeStoreRestoreEpoch = provider.youtubeRuntimeSyncEpoch
	return nil
}

func (provider *NativeAppSessionProvider) AppSessionsSupported() bool {
	return connectorAppSessionNativeSupported()
}

func (provider *NativeAppSessionProvider) StartAppSession(ctx context.Context, request appsessionsservice.AppSessionStartRequest) (appsessionsservice.AppSessionBrowser, error) {
	if provider == nil || provider.app == nil {
		return nil, appsessions.ErrUnsupported
	}
	if !connectorAppSessionNativeSupported() {
		return nil, appsessions.ErrUnsupported
	}
	targetURL := strings.TrimSpace(request.TargetURL)
	if targetURL == "" {
		targetURL = "https://www.youtube.com/"
	}
	title := "Connect " + siteAppSessionWindowTitle(request.SiteKey)
	handle := &connectorAppSessionWindow{
		domains: append([]string(nil), request.Domains...),
		done:    make(chan struct{}),
	}
	window := provider.app.Window.NewWithOptions(withRemoteWebViewPermissionPolicy(application.WebviewWindowOptions{
		Name:      "site-app-session-" + strings.TrimSpace(request.SessionID),
		Title:     title,
		Width:     560,
		Height:    720,
		MinWidth:  420,
		MinHeight: 520,
		URL:       connectorAppSessionBlankURL,
		Mac: application.MacWindow{
			WebviewPreferences: application.MacWebviewPreferences{
				FullscreenEnabled: application.Disabled,
			},
		},
	}))
	handle.window = window
	initialCookies := connectorAppSessionInitialCookies(request.SiteKey, request.InitialCookies, time.Now())
	prepareConnectorAppSessionNativeWindow(window, targetURL, request.SiteKey, initialCookies, request.Domains)
	registerWebViewRemoteCapabilityPolicy(window)
	window.RegisterHook(events.Common.WindowClosing, func(event *application.WindowEvent) {
		handle.handleWindowClosing(event)
	})
	loadConnectorAppSessionNativeURL(window, targetURL)
	window.Show()
	window.Focus()
	return handle, nil
}

func connectorAppSessionInitialCookies(siteKey string, records []appcookies.Record, now time.Time) []appcookies.Record {
	if strings.EqualFold(strings.TrimSpace(siteKey), "youtube") {
		return youtubecookies.StableAuth(records, now)
	}
	return cloneAppSessionCookieRecords(records)
}

func planConnectorAppSessionCookieRestore(
	siteKey string,
	persisted []appcookies.Record,
	current []appcookies.Record,
	now time.Time,
	storeAvailable bool,
) []appcookies.Record {
	if !strings.EqualFold(strings.TrimSpace(siteKey), "youtube") {
		return cloneAppSessionCookieRecords(persisted)
	}
	return planListenPlaybackCookieRestore(
		persisted,
		current,
		now,
		storeAvailable,
	)
}

func (provider *NativeAppSessionProvider) LoadAppSessionCookies(ctx context.Context, siteKey string) ([]appcookies.Record, error) {
	if provider == nil {
		return nil, appsessions.ErrNoCookies
	}
	youtube := strings.EqualFold(strings.TrimSpace(siteKey), "youtube")
	if youtube {
		// Serialize persistence loads with Save, Clear and live-store sync. In
		// particular, a load that began before Clear must not re-enable runtime
		// synchronization after the cleared state has become authoritative.
		provider.runtimeSyncMu.Lock()
		defer provider.runtimeSyncMu.Unlock()
	}
	loadedFromPersistence := false
	records, available, err := provider.cookieCache.loadContext(ctx, siteKey, func() ([]appcookies.Record, bool, error) {
		loadedFromPersistence = true
		loaded, err := provider.loadStoredCookies(ctx, siteKey)
		if err != nil {
			if errors.Is(err, appsessions.ErrNoCookies) || errors.Is(err, appsessions.ErrUnsupported) {
				return nil, false, nil
			}
			return nil, false, err
		}
		if youtube {
			now := time.Now()
			loaded = nativeYouTubePersistentCookies(youtubecookies.Runtime(loaded, now), now)
		}
		return loaded, len(loaded) > 0, nil
	})
	if err != nil {
		return nil, err
	}
	if !available {
		return nil, appsessions.ErrNoCookies
	}
	if loadedFromPersistence && youtube {
		provider.youtubeRuntimeSyncAllowed = true
		if provider.youtubeRuntimeSyncEpoch == 0 {
			provider.youtubeRuntimeSyncEpoch = 1
		}
	}
	return records, nil
}

func (provider *NativeAppSessionProvider) SaveAppSessionCookies(ctx context.Context, siteKey string, records []appcookies.Record) error {
	youtube := strings.EqualFold(strings.TrimSpace(siteKey), "youtube")
	cacheRecords := records
	persistedRecords := records
	if youtube {
		now := time.Now()
		cacheRecords = youtubecookies.Runtime(records, now)
		persistedRecords = nativeYouTubePersistentCookies(cacheRecords, now)
		if len(persistedRecords) == 0 {
			return appsessions.ErrNoCookies
		}
		provider.runtimeSyncMu.Lock()
		defer provider.runtimeSyncMu.Unlock()
	}
	if err := provider.saveStoredCookies(ctx, siteKey, persistedRecords); err != nil {
		if errors.Is(err, appsessions.ErrUnsupported) {
			return appsessions.ErrUnsupported
		}
		if errors.Is(err, appsessions.ErrNoCookies) {
			return appsessions.ErrNoCookies
		}
		return err
	}
	provider.cookieCache.store(siteKey, cacheRecords)
	if youtube {
		provider.youtubeRuntimeSyncAllowed = true
		provider.youtubeRuntimeSyncEpoch++
		if provider.youtubeRuntimeSyncEpoch == 0 {
			provider.youtubeRuntimeSyncEpoch = 1
		}
		provider.youtubeRuntimeAppliedSequence = 0
		provider.youtubeNativeStoreRestoreEpoch = provider.youtubeRuntimeSyncEpoch
	}
	return nil
}

// SyncAppSessionCookies applies a shared native-store read to the in-memory
// request cache. It never writes the App Session vault/SQLite snapshot.
// Explicit login/import uses SaveAppSessionCookies or
// CacheImportedAppSessionCookies; explicit logout uses ClearAppSession.
func (provider *NativeAppSessionProvider) SyncAppSessionCookies(
	ctx context.Context,
	siteKey string,
	records []appcookies.Record,
	expectedEpoch uint64,
	expectedSequence uint64,
) error {
	if provider == nil || !strings.EqualFold(strings.TrimSpace(siteKey), "youtube") {
		return appsessions.ErrUnsupported
	}
	if ctx != nil {
		if err := ctx.Err(); err != nil {
			return err
		}
	}
	now := time.Now()
	runtimeRecords := youtubecookies.Runtime(records, now)
	stableRecords := nativeYouTubePersistentCookies(runtimeRecords, now)
	if len(runtimeRecords) == 0 ||
		!youtubecookies.HasAuthForURL(runtimeRecords, "https://www.youtube.com/", now) ||
		len(stableRecords) == 0 {
		return appsessions.ErrNoCookies
	}

	provider.runtimeSyncMu.Lock()
	defer provider.runtimeSyncMu.Unlock()
	if !provider.youtubeRuntimeSyncAllowed ||
		expectedEpoch == 0 ||
		expectedEpoch != provider.youtubeRuntimeSyncEpoch ||
		expectedSequence == 0 ||
		expectedSequence <= provider.youtubeRuntimeAppliedSequence {
		return appsessions.ErrNoCookies
	}

	current, _, _ := provider.cookieCache.load(siteKey, nil)
	currentStable := nativeYouTubePersistentCookies(current, now)
	missingStable := youtubecookies.MissingStableAuth(
		currentStable,
		stableRecords,
		now,
	)
	if len(missingStable) > 0 {
		return appsessions.ErrNoCookies
	}
	provider.cookieCache.store(siteKey, runtimeRecords)
	provider.youtubeRuntimeAppliedSequence = expectedSequence
	return nil
}

func (provider *NativeAppSessionProvider) ClearAppSession(ctx context.Context, siteKey string, domains []string) error {
	youtube := strings.EqualFold(strings.TrimSpace(siteKey), "youtube")
	if youtube {
		provider.runtimeSyncMu.Lock()
	}
	if err := provider.clearStoredCookies(ctx, siteKey, domains); err != nil {
		if youtube {
			provider.runtimeSyncMu.Unlock()
		}
		if errors.Is(err, appsessions.ErrUnsupported) {
			return appsessions.ErrUnsupported
		}
		if errors.Is(err, appsessions.ErrNoCookies) {
			return appsessions.ErrNoCookies
		}
		return err
	}
	provider.cookieCache.store(siteKey, nil)
	if youtube {
		provider.youtubeRuntimeSyncAllowed = false
		provider.youtubeRuntimeSyncEpoch++
		if provider.youtubeRuntimeSyncEpoch == 0 {
			provider.youtubeRuntimeSyncEpoch = 1
		}
		provider.youtubeRuntimeAppliedSequence = 0
		provider.youtubeNativeStoreRestoreEpoch = 0
		provider.runtimeSyncMu.Unlock()
	}
	if ctx == nil {
		ctx = context.Background()
	}
	if err := clearConnectorAppSessionNativeRuntimeData(ctx, provider.app, siteKey, domains); err != nil &&
		!errors.Is(err, appsessions.ErrUnsupported) &&
		!errors.Is(err, appsessions.ErrNoCookies) {
		zap.L().Warn(
			"clear App Session WebView runtime data",
			zap.String("siteKey", strings.TrimSpace(siteKey)),
			zap.Error(err),
		)
	}
	return nil
}

// CacheImportedAppSessionCookies publishes a transactionally committed
// browser-profile import to request consumers without performing a second
// persistence write.
func (provider *NativeAppSessionProvider) CacheImportedAppSessionCookies(siteKey string, records []appcookies.Record) {
	if provider == nil {
		return
	}
	youtube := strings.EqualFold(strings.TrimSpace(siteKey), "youtube")
	cacheRecords := cloneAppSessionCookieRecords(records)
	if youtube {
		cacheRecords = youtubecookies.Runtime(cacheRecords, time.Now())
		provider.runtimeSyncMu.Lock()
		defer provider.runtimeSyncMu.Unlock()
	}
	provider.cookieCache.store(siteKey, cacheRecords)
	if youtube {
		nativePublishErr := publishNativeYouTubeRuntimeCookies(provider.app, cacheRecords)
		if nativePublishErr != nil {
			zap.L().Error(
				"youtube imported cookies native publish failed",
				zap.Error(nativePublishErr),
			)
		}
		provider.youtubeRuntimeSyncAllowed = len(cacheRecords) > 0
		provider.youtubeRuntimeSyncEpoch++
		if provider.youtubeRuntimeSyncEpoch == 0 {
			provider.youtubeRuntimeSyncEpoch = 1
		}
		provider.youtubeRuntimeAppliedSequence = 0
		if nativePublishErr == nil {
			provider.youtubeNativeStoreRestoreEpoch = provider.youtubeRuntimeSyncEpoch
		} else {
			provider.youtubeNativeStoreRestoreEpoch = 0
		}
	}
}

func (provider *NativeAppSessionProvider) AppSessionCookieSyncEpoch(siteKey string) uint64 {
	if provider == nil || !strings.EqualFold(strings.TrimSpace(siteKey), "youtube") {
		return 0
	}
	provider.runtimeSyncMu.Lock()
	epoch := provider.youtubeRuntimeSyncEpoch
	provider.runtimeSyncMu.Unlock()
	return epoch
}

func (provider *NativeAppSessionProvider) BeginAppSessionCookieSync(siteKey string) (uint64, uint64) {
	if provider == nil || !strings.EqualFold(strings.TrimSpace(siteKey), "youtube") {
		return 0, 0
	}
	provider.runtimeSyncMu.Lock()
	defer provider.runtimeSyncMu.Unlock()
	if !provider.youtubeRuntimeSyncAllowed || provider.youtubeRuntimeSyncEpoch == 0 {
		return 0, 0
	}
	return provider.youtubeRuntimeSyncEpoch, provider.nextYouTubeRuntimeSampleSequenceLocked()
}

func (provider *NativeAppSessionProvider) nextYouTubeRuntimeSampleSequenceLocked() uint64 {
	provider.youtubeRuntimeSampleSequence++
	if provider.youtubeRuntimeSampleSequence == 0 {
		provider.youtubeRuntimeSampleSequence = 1
		provider.youtubeRuntimeAppliedSequence = 0
	}
	return provider.youtubeRuntimeSampleSequence
}

func (provider *NativeAppSessionProvider) loadStoredCookies(ctx context.Context, siteKey string) ([]appcookies.Record, error) {
	if provider != nil && provider.loadStored != nil {
		return provider.loadStored(siteKey)
	}
	if provider == nil || provider.secretVault == nil {
		return nil, appsessions.ErrUnsupported
	}
	plaintext, err := provider.secretVault.LoadAppSessionSecret(ctx, siteKey)
	if err != nil {
		return nil, err
	}
	records := appcookies.DecodeJSON(string(plaintext))
	if len(records) == 0 {
		return nil, appsessions.ErrNoCookies
	}
	return records, nil
}

func (provider *NativeAppSessionProvider) loadRuntimeCookies() ([]appcookies.Record, error) {
	if provider != nil && provider.loadRuntime != nil {
		return provider.loadRuntime()
	}
	if provider == nil || provider.app == nil {
		return nil, appsessions.ErrUnsupported
	}
	return loadNativeYouTubeRuntimeCookies(provider.app)
}

// Every platform persists only the Kaset-compatible stable bootstrap
// credentials. Rotating request-time cookies belong exclusively to the live
// native cookie store and the in-memory request cache.
func nativeYouTubePersistentCookies(records []appcookies.Record, now time.Time) []appcookies.Record {
	return youtubecookies.StableAuth(records, now)
}

func (provider *NativeAppSessionProvider) saveStoredCookies(ctx context.Context, siteKey string, records []appcookies.Record) error {
	if provider != nil && provider.saveStored != nil {
		return provider.saveStored(siteKey, records)
	}
	if provider == nil || provider.secretVault == nil {
		return appsessions.ErrUnsupported
	}
	data, err := appcookies.EncodeJSON(records)
	if err != nil {
		return err
	}
	if data == "" {
		return appsessions.ErrNoCookies
	}
	return provider.secretVault.SaveAppSessionSecret(ctx, siteKey, []byte(data))
}

func (provider *NativeAppSessionProvider) clearStoredCookies(ctx context.Context, siteKey string, domains []string) error {
	if provider != nil && provider.clearStored != nil {
		return provider.clearStored(siteKey, domains)
	}
	if provider == nil || provider.secretVault == nil {
		return appsessions.ErrUnsupported
	}
	return provider.secretVault.DeleteAppSessionSecret(ctx, siteKey)
}

func (cache *nativeAppSessionCookieCache) load(
	siteKey string,
	loader func() ([]appcookies.Record, bool, error),
) ([]appcookies.Record, bool, error) {
	return cache.loadContext(context.Background(), siteKey, loader)
}

func (cache *nativeAppSessionCookieCache) loadContext(
	ctx context.Context,
	siteKey string,
	loader func() ([]appcookies.Record, bool, error),
) ([]appcookies.Record, bool, error) {
	if ctx == nil {
		ctx = context.Background()
	}
	if err := ctx.Err(); err != nil {
		return nil, false, err
	}
	if cache == nil {
		if loader == nil {
			return nil, false, nil
		}
		records, available, err := loader()
		if err != nil {
			return nil, false, err
		}
		cloned := cloneAppSessionCookieRecords(records)
		return cloned, available && len(cloned) > 0, nil
	}
	key := normalizeAppSessionCookieCacheKey(siteKey)
	cache.mu.Lock()
	if cache.entries == nil {
		cache.entries = make(map[string]*nativeAppSessionCookieCacheEntry)
	}
	if entry := cache.entries[key]; entry != nil {
		if entry.loading {
			ready := entry.ready
			cache.mu.Unlock()
			select {
			case <-ready:
			case <-ctx.Done():
				return nil, false, ctx.Err()
			}
			cache.mu.Lock()
		}
		records := cloneAppSessionCookieRecords(entry.records)
		available := entry.available
		err := entry.err
		cache.mu.Unlock()
		return records, available, err
	}

	entry := &nativeAppSessionCookieCacheEntry{
		loading: true,
		ready:   make(chan struct{}),
	}
	cache.entries[key] = entry
	cache.mu.Unlock()

	var loaded []appcookies.Record
	available := false
	var loadErr error
	if loader != nil {
		loaded, available, loadErr = loader()
	}
	loaded = cloneAppSessionCookieRecords(loaded)
	available = available && len(loaded) > 0

	cache.mu.Lock()
	current := cache.entries[key]
	completedLoad := current == entry && current.loading
	if completedLoad {
		current.loading = false
		current.err = loadErr
		if loadErr == nil {
			current.records = loaded
			current.available = available
		} else {
			// Platform/keychain failures are not evidence that the account has
			// no cookies. Wake coalesced readers, but remove the entry so a
			// later user action can retry after the keychain is unlocked.
			delete(cache.entries, key)
		}
		close(current.ready)
		current.ready = nil
	}
	if loadErr != nil && completedLoad {
		cache.mu.Unlock()
		return nil, false, loadErr
	}
	// A concurrent Save/Clear replaced the load with an authoritative value.
	// Return that value instead of either the loaded data or a stale error.
	current = cache.entries[key]
	if current == nil {
		cache.mu.Unlock()
		return nil, false, loadErr
	}
	records := cloneAppSessionCookieRecords(current.records)
	available = current.available
	err := current.err
	cache.mu.Unlock()
	return records, available, err
}

// store marks a site as loaded. Passing nil records is the cached
// representation of a successful clear or a known no-cookie result.
func (cache *nativeAppSessionCookieCache) store(siteKey string, records []appcookies.Record) {
	if cache == nil {
		return
	}
	key := normalizeAppSessionCookieCacheKey(siteKey)
	cloned := cloneAppSessionCookieRecords(records)
	cache.mu.Lock()
	if cache.entries == nil {
		cache.entries = make(map[string]*nativeAppSessionCookieCacheEntry)
	}
	entry := cache.entries[key]
	if entry == nil {
		entry = &nativeAppSessionCookieCacheEntry{}
		cache.entries[key] = entry
	}
	entry.records = cloned
	entry.available = len(cloned) > 0
	entry.err = nil
	if entry.loading {
		entry.loading = false
		close(entry.ready)
		entry.ready = nil
	}
	cache.mu.Unlock()
}

// blockPersistenceFallback preserves a completed cache entry, but converts a
// missing or in-flight persistence load into an authoritative unavailable
// result. macOS uses this after a WebKit infrastructure failure so a request may
// continue with already validated runtime cookies but can never revive a stale
// compatibility Keychain snapshot.
func (cache *nativeAppSessionCookieCache) blockPersistenceFallback(siteKey string) {
	if cache == nil {
		return
	}
	key := normalizeAppSessionCookieCacheKey(siteKey)
	cache.mu.Lock()
	if cache.entries == nil {
		cache.entries = make(map[string]*nativeAppSessionCookieCacheEntry)
	}
	entry := cache.entries[key]
	if entry == nil {
		cache.entries[key] = &nativeAppSessionCookieCacheEntry{}
		cache.mu.Unlock()
		return
	}
	if entry.loading {
		entry.records = nil
		entry.available = false
		entry.err = nil
		entry.loading = false
		close(entry.ready)
		entry.ready = nil
	}
	cache.mu.Unlock()
}

func normalizeAppSessionCookieCacheKey(siteKey string) string {
	return strings.ToLower(strings.TrimSpace(siteKey))
}

func cloneAppSessionCookieRecords(records []appcookies.Record) []appcookies.Record {
	if len(records) == 0 {
		return nil
	}
	return append([]appcookies.Record(nil), records...)
}

func (session *connectorAppSessionWindow) Cookies(ctx context.Context) ([]appcookies.Record, error) {
	if session == nil {
		return nil, appsessions.ErrSessionGone
	}
	session.mu.Lock()
	window := session.window
	cached := append([]appcookies.Record(nil), session.last...)
	completed := session.completed
	session.mu.Unlock()
	if completed && len(cached) > 0 {
		return cached, nil
	}
	if window == nil {
		if len(cached) > 0 {
			return cached, nil
		}
		return nil, appsessions.ErrSessionDead
	}
	records, err := readConnectorAppSessionNativeWindowCookies(ctx, window, session.domains)
	if err != nil {
		if len(cached) > 0 {
			return cached, nil
		}
		return nil, err
	}
	session.mu.Lock()
	session.last = append([]appcookies.Record(nil), records...)
	session.mu.Unlock()
	return records, nil
}

func (session *connectorAppSessionWindow) Done() <-chan struct{} {
	if session == nil {
		done := make(chan struct{})
		close(done)
		return done
	}
	return session.done
}

func (session *connectorAppSessionWindow) Close() {
	if session == nil {
		return
	}
	session.closeOnce.Do(func() {
		captureBeforeClose := connectorAppSessionCaptureBeforeClose()
		var completeNow bool
		session.mu.Lock()
		window := session.window
		if session.completed {
			window = nil
		} else {
			session.closing = true
			session.allowClose = true
			if !captureBeforeClose || window == nil {
				completeNow = session.completeCloseLocked(window)
			}
		}
		session.mu.Unlock()
		if window != nil {
			window.Close()
		}
		if completeNow {
			session.closeDone()
		} else if captureBeforeClose && window != nil {
			session.completeClose(window)
		}
	})
}

func (session *connectorAppSessionWindow) handleWindowClosing(event *application.WindowEvent) {
	if session == nil {
		return
	}
	if !connectorAppSessionCaptureBeforeClose() {
		session.mu.Lock()
		completeNow := session.completeCloseLocked(session.window)
		session.mu.Unlock()
		if completeNow {
			session.closeDone()
		}
		return
	}
	session.mu.Lock()
	if session.allowClose {
		completeNow := session.completeCloseLocked(session.window)
		session.mu.Unlock()
		if completeNow {
			session.closeDone()
		}
		return
	}
	if session.closing {
		session.mu.Unlock()
		event.Cancel()
		return
	}
	session.closing = true
	session.mu.Unlock()

	event.Cancel()
	go session.captureCookiesAndClose()
}

func (session *connectorAppSessionWindow) captureCookiesAndClose() {
	if session == nil {
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), 6*time.Second)
	records, err := session.Cookies(ctx)
	cancel()
	if err == nil && len(records) > 0 {
		session.mu.Lock()
		session.last = append([]appcookies.Record(nil), records...)
		session.mu.Unlock()
	} else if err != nil {
		zap.L().Debug("capture App Session cookies before native window close", zap.Error(err))
	}

	session.mu.Lock()
	window := session.window
	if session.completed {
		session.mu.Unlock()
		return
	}
	session.allowClose = true
	session.mu.Unlock()

	if window != nil {
		window.Close()
		return
	}

	session.completeClose(nil)
}

func (session *connectorAppSessionWindow) completeClose(window *application.WebviewWindow) {
	if session == nil {
		return
	}
	session.mu.Lock()
	completeNow := session.completeCloseLocked(window)
	session.mu.Unlock()
	if completeNow {
		session.closeDone()
	}
}

func (session *connectorAppSessionWindow) completeCloseLocked(window *application.WebviewWindow) bool {
	if session == nil {
		return false
	}
	if window == nil || session.window == window {
		session.window = nil
	}
	session.closing = true
	session.allowClose = true
	if session.completed {
		return false
	}
	session.completed = true
	return true
}

func (session *connectorAppSessionWindow) closeDone() {
	close(session.done)
}

func appSessionWebViewUserAgent(siteKey string) string {
	return appsessionidentity.WebViewUserAgent(siteKey)
}

func siteAppSessionWindowTitle(siteKey string) string {
	switch strings.TrimSpace(siteKey) {
	case "youtube":
		return "YouTube"
	case "bilibili":
		return "Bilibili"
	case "tiktok":
		return "TikTok"
	case "douyin":
		return "Douyin"
	case "xiaohongshu":
		return "Xiaohongshu"
	case "instagram":
		return "Instagram"
	case "x":
		return "X"
	case "facebook":
		return "Facebook"
	case "vimeo":
		return "Vimeo"
	case "twitch":
		return "Twitch"
	case "niconico":
		return "Niconico"
	default:
		return "App Session"
	}
}
