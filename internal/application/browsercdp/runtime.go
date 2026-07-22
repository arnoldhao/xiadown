package browsercdp

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/chromedp/cdproto/cdp"
	"github.com/chromedp/cdproto/network"
	"github.com/chromedp/cdproto/page"
	targetpkg "github.com/chromedp/cdproto/target"
	"github.com/chromedp/chromedp"
	"go.uber.org/zap"
)

type LaunchOptions struct {
	PreferredBrowser   string
	ExecutableIdentity ExecutableIdentity
	Headless           bool
	NoSandbox          bool
	ExtraArgs          []string
	UserDataDir        string
	PersistentProfile  bool
	NetworkRoute       *ManagedNetworkRoute
}

// ManagedNetworkRoute is the single reviewed browser egress configuration.
// The attestation is fetched through Chromium before Start returns so a
// machine policy which overrides --proxy-server cannot silently bypass the
// App route.
type ManagedNetworkRoute struct {
	ProxyURL         string
	AttestationURL   string
	AttestationToken string
}

type Status struct {
	Ready                  bool        `json:"ready"`
	Candidates             []Candidate `json:"candidates,omitempty"`
	SelectedBrowser        string      `json:"selectedBrowser,omitempty"`
	ChosenBrowser          string      `json:"chosenBrowser,omitempty"`
	DetectedExecutablePath string      `json:"detectedExecutablePath,omitempty"`
	DetectError            string      `json:"detectError,omitempty"`
	CDPURL                 string      `json:"cdpUrl,omitempty"`
	CDPPort                int         `json:"cdpPort,omitempty"`
	Headless               bool        `json:"headless"`
	Ownership              string      `json:"ownership,omitempty"`
}

type RuntimeOwnership string

const (
	RuntimeOwnershipOwned    RuntimeOwnership = "owned"
	RuntimeOwnershipBorrowed RuntimeOwnership = "borrowed"
)

type ProcessInfo struct {
	PID            int
	ProcessGroupID int
	ExecutablePath string
	UserDataDir    string
}

type Runtime struct {
	mu sync.Mutex

	ownership RuntimeOwnership
	options   LaunchOptions
	candidate Candidate
	status    Status

	cmd                  *exec.Cmd
	userDataDir          string
	rootPID              int
	processGroupID       int
	registryID           string
	allocCtx             context.Context
	allocCancel          context.CancelFunc
	browserCtx           context.Context
	browserCancel        context.CancelFunc
	networkMonitorCancel context.CancelFunc
	networkRoute         *ManagedNetworkRoute
	networkRouteVerifyMu sync.Mutex
	targetManager        *PageTargetManager
	borrowedContextID    cdp.BrowserContextID
	borrowedContextSet   bool
	borrowedTargetNextID uint64
	borrowedTargetStops  map[uint64]context.CancelFunc
	stopping             bool
	stopped              chan struct{}
	stoppedOnce          sync.Once
}

const runtimeCDPNoTopLevelFrameMessage = "received DOM.documentUpdated when there's no top-level frame"

var ErrNoSupportedBrowser = errors.New("no supported browser detected")

var ErrExactExecutableUnavailable = errors.New("exact browser executable is unavailable")

var ErrManagedNetworkRouteRequired = errors.New("managed browser network route is required")

type launchNetworkBoundary uint8

const (
	launchNetworkBoundaryManaged launchNetworkBoundary = iota + 1
	launchNetworkBoundaryLoopbackOnly
)

func runtimeCDPErrorf(string, ...any) {}

func runtimeShouldSuppressCDPError(message string) bool {
	return strings.Contains(strings.TrimSpace(message), runtimeCDPNoTopLevelFrameMessage)
}

func ResolveStatus(preferred string, headless bool) Status {
	candidates := DetectCandidates()
	status := Status{
		Candidates:      candidates,
		SelectedBrowser: strings.TrimSpace(preferred),
		Headless:        headless,
	}
	candidate, ok := ChooseCandidate(candidates, preferred)
	if !ok {
		status.DetectError = ErrNoSupportedBrowser.Error()
		return status
	}
	status.ChosenBrowser = string(candidate.ID)
	status.DetectedExecutablePath = candidate.ExecPath
	status.Ready = candidate.Available
	if !candidate.Available {
		status.DetectError = candidate.Error
	}
	return status
}

func Start(ctx context.Context, options LaunchOptions) (*Runtime, error) {
	return startWithNetworkBoundary(ctx, options, launchNetworkBoundaryManaged)
}

// StartLoopbackOnly starts Chromium behind a guaranteed-dead proxy while
// allowing only literal loopback destinations to bypass it. It is the narrow
// entry point for local harnesses and browser tests which do not own a managed
// App gateway. Remote-capable production features must use Start and provide a
// valid NetworkRoute.
func StartLoopbackOnly(ctx context.Context, options LaunchOptions) (*Runtime, error) {
	return startWithNetworkBoundary(ctx, options, launchNetworkBoundaryLoopbackOnly)
}

func resolveLaunchCandidate(options LaunchOptions, candidates []Candidate) (Candidate, error) {
	if options.ExecutableIdentity != (ExecutableIdentity{}) {
		return candidateForExecutableIdentity(options.ExecutableIdentity, options.PreferredBrowser)
	}
	candidate, ok := ChooseCandidate(candidates, options.PreferredBrowser)
	if !ok {
		return Candidate{}, ErrNoSupportedBrowser
	}
	return candidate, nil
}

func startWithNetworkBoundary(ctx context.Context, options LaunchOptions, boundary launchNetworkBoundary) (*Runtime, error) {
	networkArgs, err := networkLaunchArgs(boundary, options.NetworkRoute, options.ExtraArgs)
	if err != nil {
		return nil, err
	}
	candidates := DetectCandidates()
	candidate, err := resolveLaunchCandidate(options, candidates)
	if err != nil {
		return nil, err
	}

	port := 0
	userDataDir := strings.TrimSpace(options.UserDataDir)
	if userDataDir == "" {
		userDataDir = filepath.Join(os.TempDir(), "xiadown", "browsercdp", string(candidate.ID))
	}
	if err := os.MkdirAll(userDataDir, 0o700); err != nil {
		return nil, err
	}
	_ = os.Chmod(userDataDir, 0o700)
	_ = os.Remove(devToolsActivePortPath(userDataDir))

	args := buildLaunchArgs(port, userDataDir, options)
	if options.NoSandbox {
		args = append([]string{"--no-sandbox"}, args...)
	}
	for _, extra := range options.ExtraArgs {
		if trimmed := strings.TrimSpace(extra); trimmed != "" {
			args = append(args, trimmed)
		}
	}
	args = append(args, networkArgs...)
	args = appendBrowserLaunchArgs(args, candidate.ID)

	cmd := exec.Command(candidate.ExecPath, args...)
	cmd.Stdout = io.Discard
	cmd.Stderr = io.Discard
	configureProcessGroup(cmd)
	if err := cmd.Start(); err != nil {
		zap.L().Warn(
			"browser runtime launch failed",
			zap.String("preferredBrowser", strings.TrimSpace(options.PreferredBrowser)),
			zap.String("chosenBrowser", string(candidate.ID)),
			zap.String("executableRef", browserLogReference(candidate.ExecPath)),
			browserErrorLogField(err),
		)
		return nil, err
	}
	rootPID, processGroupID := runtimeProcessIDs(cmd)
	registryID, registryErr := registerRuntimeProcess(runtimeProcessRecord{
		PID:            rootPID,
		ProcessGroupID: processGroupID,
		ExecutablePath: candidate.ExecPath,
		UserDataDir:    userDataDir,
		CDPPort:        port,
		CreatedAt:      time.Now(),
	})
	if registryErr != nil {
		zap.L().Warn(
			"browser runtime process registry failed",
			zap.String("preferredBrowser", strings.TrimSpace(options.PreferredBrowser)),
			zap.String("chosenBrowser", string(candidate.ID)),
			zap.String("executableRef", browserLogReference(candidate.ExecPath)),
			zap.Int("pid", rootPID),
			zap.Int("processGroupID", processGroupID),
			browserErrorLogField(registryErr),
		)
	}
	port, wsURL, err := waitForDevToolsActivePort(ctx, userDataDir, 10*time.Second)
	if err != nil {
		zap.L().Warn(
			"browser runtime devtools endpoint wait failed",
			zap.String("preferredBrowser", strings.TrimSpace(options.PreferredBrowser)),
			zap.String("chosenBrowser", string(candidate.ID)),
			zap.String("executableRef", browserLogReference(candidate.ExecPath)),
			browserErrorLogField(err),
		)
		stopStartedRuntimeCommand(cmd, 2*time.Second)
		unregisterRuntimeProcess(registryID)
		return nil, err
	}
	allocCtx, allocCancel := chromedp.NewRemoteAllocator(context.Background(), wsURL)
	browserCtx, browserCancel := chromedp.NewContext(allocCtx, chromedp.WithErrorf(runtimeCDPErrorf))
	if _, err := chromedp.Targets(browserCtx); err != nil {
		zap.L().Warn(
			"browser runtime chromedp attach failed",
			zap.String("preferredBrowser", strings.TrimSpace(options.PreferredBrowser)),
			zap.String("chosenBrowser", string(candidate.ID)),
			zap.String("executableRef", browserLogReference(candidate.ExecPath)),
			browserErrorLogField(err),
		)
		browserCancel()
		allocCancel()
		stopStartedRuntimeCommand(cmd, 2*time.Second)
		unregisterRuntimeProcess(registryID)
		return nil, err
	}
	if boundary == launchNetworkBoundaryManaged {
		if err := verifyManagedNetworkRoute(browserCtx, options.NetworkRoute, nil); err != nil {
			zap.L().Warn(
				"browser runtime network route attestation failed",
				zap.String("preferredBrowser", strings.TrimSpace(options.PreferredBrowser)),
				zap.String("chosenBrowser", string(candidate.ID)),
				zap.String("executableRef", browserLogReference(candidate.ExecPath)),
				browserErrorLogField(err),
			)
			browserCancel()
			allocCancel()
			stopStartedRuntimeCommand(cmd, 2*time.Second)
			unregisterRuntimeProcess(registryID)
			return nil, err
		}
	}

	if registryID != "" {
		previousRegistryID := registryID
		updatedRegistryID, err := registerRuntimeProcess(runtimeProcessRecord{
			PID:            rootPID,
			ProcessGroupID: processGroupID,
			ExecutablePath: candidate.ExecPath,
			UserDataDir:    userDataDir,
			CDPURL:         wsURL,
			CDPPort:        port,
			CreatedAt:      time.Now(),
		})
		if err == nil && updatedRegistryID != "" {
			registryID = updatedRegistryID
			if previousRegistryID != updatedRegistryID {
				unregisterRuntimeProcess(previousRegistryID)
			}
		}
	}
	if registryID == "" {
		registryID, registryErr = registerRuntimeProcess(runtimeProcessRecord{
			PID:            rootPID,
			ProcessGroupID: processGroupID,
			ExecutablePath: candidate.ExecPath,
			UserDataDir:    userDataDir,
			CDPURL:         wsURL,
			CDPPort:        port,
			CreatedAt:      time.Now(),
		})
		if registryErr != nil {
			zap.L().Warn(
				"browser runtime process registry failed",
				zap.String("preferredBrowser", strings.TrimSpace(options.PreferredBrowser)),
				zap.String("chosenBrowser", string(candidate.ID)),
				zap.String("executableRef", browserLogReference(candidate.ExecPath)),
				zap.Int("pid", rootPID),
				zap.Int("processGroupID", processGroupID),
				browserErrorLogField(registryErr),
			)
		}
	}
	runtime := &Runtime{
		ownership:      RuntimeOwnershipOwned,
		options:        options,
		candidate:      candidate,
		cmd:            cmd,
		userDataDir:    userDataDir,
		rootPID:        rootPID,
		processGroupID: processGroupID,
		registryID:     registryID,
		allocCtx:       allocCtx,
		allocCancel:    allocCancel,
		browserCtx:     browserCtx,
		browserCancel:  browserCancel,
		stopped:        make(chan struct{}),
		status: Status{
			Ready:                  true,
			Candidates:             candidates,
			SelectedBrowser:        strings.TrimSpace(options.PreferredBrowser),
			ChosenBrowser:          string(candidate.ID),
			DetectedExecutablePath: candidate.ExecPath,
			CDPURL:                 wsURL,
			CDPPort:                port,
			Headless:               options.Headless,
			Ownership:              string(RuntimeOwnershipOwned),
		},
	}
	if boundary == launchNetworkBoundaryManaged {
		monitorContext, monitorCancel := context.WithCancel(browserCtx)
		runtime.networkMonitorCancel = monitorCancel
		routeSnapshot := *options.NetworkRoute
		runtime.networkRoute = &routeSnapshot
		go runtime.monitorManagedNetworkRoute(monitorContext)
	}
	go func() {
		_ = cmd.Wait()
		unregisterRuntimeProcess(registryID)
		runtime.mu.Lock()
		runtime.status.Ready = false
		stopping := runtime.stopping
		if !stopping && runtime.status.DetectError == "" {
			runtime.status.DetectError = "browser process exited"
		}
		runtime.mu.Unlock()
		if !stopping {
			zap.L().Warn(
				"browser runtime exited unexpectedly",
				zap.String("preferredBrowser", strings.TrimSpace(options.PreferredBrowser)),
				zap.String("chosenBrowser", string(candidate.ID)),
				zap.String("executableRef", browserLogReference(candidate.ExecPath)),
			)
		}
		runtime.markRuntimeStopped()
	}()
	targetManager, err := startPageTargetManager(runtime)
	if err != nil {
		zap.L().Warn(
			"browser runtime target manager start failed",
			zap.String("preferredBrowser", strings.TrimSpace(options.PreferredBrowser)),
			zap.String("chosenBrowser", string(candidate.ID)),
			zap.String("executableRef", browserLogReference(candidate.ExecPath)),
			browserErrorLogField(err),
		)
		runtime.Stop()
		return nil, err
	}
	runtime.mu.Lock()
	runtime.targetManager = targetManager
	runtime.mu.Unlock()
	if !options.Headless {
		if _, err := createPageTarget(runtime, 10*time.Second, true); err != nil {
			zap.L().Warn(
				"browser runtime initial visible target creation failed",
				zap.String("preferredBrowser", strings.TrimSpace(options.PreferredBrowser)),
				zap.String("chosenBrowser", string(candidate.ID)),
				zap.String("executableRef", browserLogReference(candidate.ExecPath)),
				browserErrorLogField(err),
			)
			runtime.Stop()
			return nil, err
		}
	}

	return runtime, nil
}

func buildLaunchArgs(port int, userDataDir string, options LaunchOptions) []string {
	args := []string{
		fmt.Sprintf("--remote-debugging-port=%d", port),
		"--remote-debugging-address=127.0.0.1",
		fmt.Sprintf("--user-data-dir=%s", userDataDir),
		"--no-first-run",
		"--no-default-browser-check",
		"--disable-background-networking",
		"--disable-background-timer-throttling",
		"--disable-backgrounding-occluded-windows",
		"--disable-breakpad",
		"--disable-client-side-phishing-detection",
		"--disable-default-apps",
		"--disable-extensions",
		"--disable-component-extensions-with-background-pages",
		"--disable-features=Translate,OptimizationHints,MediaRouter,AutomationControlled",
		"--disable-hang-monitor",
		"--disable-popup-blocking",
		"--disable-prompt-on-repost",
		"--disable-sync",
		"--metrics-recording-only",
	}
	// The mock keychain is useful for disposable automation profiles, but it can
	// prevent encrypted cookies from surviving restarts in persistent profiles.
	if !options.PersistentProfile {
		args = append(args, "--password-store=basic", "--use-mock-keychain")
	}
	if options.Headless {
		return append([]string{"--headless=new", "--hide-scrollbars", "--mute-audio"}, args...)
	}
	// Do not let Chromium create or restore a visible page before the managed
	// gateway attestation. Start creates the first visible blank target through
	// CDP only after the proof succeeds.
	return append([]string{"--no-startup-window"}, args...)
}

func networkLaunchArgs(boundary launchNetworkBoundary, route *ManagedNetworkRoute, extras []string) ([]string, error) {
	switch boundary {
	case launchNetworkBoundaryManaged:
		return managedNetworkLaunchArgs(route, extras)
	case launchNetworkBoundaryLoopbackOnly:
		if route != nil {
			return nil, errors.New("loopback-only browser start cannot accept a managed network route")
		}
		return loopbackOnlyNetworkLaunchArgs(extras)
	default:
		return nil, errors.New("browser network boundary is invalid")
	}
}

func managedNetworkLaunchArgs(route *ManagedNetworkRoute, extras []string) ([]string, error) {
	if err := validateBrowserNetworkExtraArgs(extras); err != nil {
		return nil, err
	}
	if route == nil {
		return nil, ErrManagedNetworkRouteRequired
	}
	proxyURL, err := url.Parse(strings.TrimSpace(route.ProxyURL))
	if err != nil || proxyURL == nil || !strings.EqualFold(proxyURL.Scheme, "http") || proxyURL.User != nil ||
		proxyURL.Port() == "" || (proxyURL.Path != "" && proxyURL.Path != "/") || proxyURL.RawQuery != "" || proxyURL.Fragment != "" {
		return nil, errors.New("managed browser proxy URL is invalid")
	}
	proxyIP := net.ParseIP(strings.Trim(proxyURL.Hostname(), "[]"))
	if !strings.EqualFold(proxyURL.Hostname(), "localhost") && (proxyIP == nil || !proxyIP.IsLoopback()) {
		return nil, errors.New("managed browser proxy must be loopback")
	}
	if _, _, err := managedAttestationURL(strings.TrimSpace(route.AttestationURL)); err != nil || strings.TrimSpace(route.AttestationToken) == "" {
		return nil, errors.New("managed browser route attestation is invalid")
	}
	if strings.Contains(strings.TrimSpace(route.AttestationURL), strings.TrimSpace(route.AttestationToken)) {
		return nil, errors.New("managed browser route attestation secret must not appear in its URL")
	}
	return []string{
		"--proxy-server=" + strings.TrimSuffix(strings.TrimSpace(route.ProxyURL), "/"),
		"--proxy-bypass-list=<-loopback>",
		"--disable-quic",
		"--dns-prefetch-disable",
		"--force-webrtc-ip-handling-policy=disable_non_proxied_udp",
	}, nil
}

func loopbackOnlyNetworkLaunchArgs(extras []string) ([]string, error) {
	if err := validateBrowserNetworkExtraArgs(extras); err != nil {
		return nil, err
	}
	return []string{
		// Chromium evaluates bypass rules left-to-right. Preserve the explicit
		// literal loopback matches first, then subtract every remaining implicit
		// localhost/link-local bypass. Every other HTTP-family request is pinned
		// to a guaranteed-dead local proxy instead of escaping directly.
		"--proxy-server=http://127.0.0.1:1",
		"--proxy-bypass-list=127.0.0.0/8;[::1];<-loopback>",
		"--disable-quic",
		"--dns-prefetch-disable",
		"--force-webrtc-ip-handling-policy=disable_non_proxied_udp",
	}, nil
}

func validateBrowserNetworkExtraArgs(extras []string) error {
	for _, extra := range extras {
		trimmed := strings.TrimSpace(extra)
		if trimmed == "" {
			continue
		}
		if trimmed == "--" || !strings.HasPrefix(trimmed, "--") {
			return fmt.Errorf("browser startup URL or positional argument %q is forbidden; create a target after route verification", trimmed)
		}
		if browserArgCanCreateStartupTarget(trimmed) {
			return fmt.Errorf("browser startup target option %q is owned by the managed runtime", trimmed)
		}
		if browserArgCanOverrideNetworkRoute(trimmed) {
			return fmt.Errorf("managed browser network option %q must be supplied by the route provider", trimmed)
		}
	}
	return nil
}

func browserArgCanCreateStartupTarget(argument string) bool {
	name := strings.ToLower(strings.TrimSpace(argument))
	if before, _, ok := strings.Cut(name, "="); ok {
		name = before
	}
	switch name {
	case "--app", "--app-id", "--kiosk", "--new-window", "--homepage", "--restore-last-session", "--no-startup-window", "--headless":
		return true
	default:
		return false
	}
}

func browserArgCanOverrideNetworkRoute(argument string) bool {
	name := strings.ToLower(strings.TrimSpace(argument))
	if before, _, ok := strings.Cut(name, "="); ok {
		name = before
	}
	switch name {
	case "--proxy-server", "--proxy-bypass-list", "--proxy-pac-url", "--proxy-auto-detect", "--no-proxy-server",
		"--host-resolver-rules", "--host-rules", "--enable-quic", "--origin-to-force-quic-on",
		"--force-webrtc-ip-handling-policy", "--webrtc-ip-handling-policy",
		"--load-extension", "--disable-extensions-except", "--load-and-launch-app", "--enable-extensions":
		return true
	default:
		return false
	}
}

func verifyManagedNetworkRoute(browserContext context.Context, route *ManagedNetworkRoute, targetManager *PageTargetManager) error {
	if route == nil {
		return nil
	}
	chromeContext := chromedp.FromContext(browserContext)
	if chromeContext == nil || chromeContext.Browser == nil {
		return errors.New("managed browser executor is unavailable")
	}
	createContext, createCancel := context.WithTimeout(browserContext, 7*time.Second)
	defer createCancel()
	browserExecutor := cdp.WithExecutor(createContext, chromeContext.Browser)
	existingTargets, err := targetpkg.GetTargets().Do(browserExecutor)
	if err != nil {
		return fmt.Errorf("list managed network route probe targets: %w", err)
	}
	probeRequest := newManagedNetworkProbeTargetRequest(existingTargets)
	if !isManagedNetworkProbeURL(probeRequest.URL) {
		return errors.New("managed network route probe URL is unsafe")
	}
	var createdTarget struct {
		TargetID targetpkg.ID `json:"targetId"`
	}
	// Chromium cannot create a hidden target while --no-startup-window has left
	// the browser with no page target. Bootstrap that one case with an ordinary
	// background target whose URL is an App-owned data: document. Once a page
	// exists (including every repeated route verification), keep using a hidden
	// target so the probe never joins the managed browser's tab strip.
	err = cdp.Execute(browserExecutor, "Target.createTarget", probeRequest.protocolParams(), &createdTarget)
	if err != nil {
		return fmt.Errorf("create managed network route probe: %w", err)
	}
	targetID := createdTarget.TargetID
	if targetID == "" {
		return errors.New("create managed network route probe returned no target")
	}
	if targetManager != nil {
		targetManager.ExcludeTargetID(string(targetID))
	}
	defer func() {
		closeContext, closeCancel := context.WithTimeout(context.WithoutCancel(browserContext), 2*time.Second)
		defer closeCancel()
		if err := targetpkg.CloseTarget(targetID).Do(cdp.WithExecutor(closeContext, chromeContext.Browser)); err != nil {
			zap.L().Debug("managed browser network route probe cleanup failed", browserErrorLogField(err))
		}
	}()

	probeContext, probeCancel := chromedp.NewContext(browserContext, chromedp.WithTargetID(targetID))
	defer probeCancel()
	probeContext, timeoutCancel := context.WithTimeout(probeContext, 7*time.Second)
	defer timeoutCancel()

	if err := chromedp.Run(probeContext); err != nil {
		return fmt.Errorf("managed browser did not reach the App network gateway: %w", err)
	}
	var frameTree *page.FrameTree
	if err := chromedp.Run(probeContext, chromedp.ActionFunc(func(actionContext context.Context) error {
		var actionErr error
		frameTree, actionErr = page.GetFrameTree().Do(actionContext)
		return actionErr
	})); err != nil || frameTree == nil || frameTree.Frame == nil || frameTree.Frame.ID == "" {
		return fmt.Errorf("managed browser route probe has no frame: %w", err)
	}
	beginURL := strings.TrimSpace(route.AttestationURL)
	begin, err := loadManagedNetworkProbe(probeContext, frameTree.Frame.ID, beginURL)
	if err != nil {
		return fmt.Errorf("managed browser did not reach the App network gateway: %w", err)
	}
	challengeURL := managedNetworkHeader(begin.Headers, "X-XiaDown-Gateway-Connect-Challenge")
	if !begin.Success || int(begin.HTTPStatusCode) != http.StatusNoContent || challengeURL == "" {
		return fmt.Errorf(
			"managed browser HTTP gateway attestation mismatch (success=%t, status=%d, challengePresent=%t)",
			begin.Success,
			int(begin.HTTPStatusCode),
			challengeURL != "",
		)
	}
	proofID, err := managedConnectProofID(challengeURL, beginURL)
	if err != nil {
		return err
	}
	// The gateway deliberately rejects the challenge CONNECT after recording
	// it. Network.loadNetworkResource may therefore return either a failed
	// resource or a protocol-level network error; the one-shot completion call
	// below is the authoritative observation.
	_, _ = loadManagedNetworkProbe(probeContext, frameTree.Frame.ID, challengeURL)
	completionURL, err := url.Parse(beginURL)
	if err != nil {
		return errors.New("managed browser route attestation URL changed while probing")
	}
	query := completionURL.Query()
	query.Set("proof", proofID)
	completionURL.RawQuery = query.Encode()
	completion, err := loadManagedNetworkProbe(probeContext, frameTree.Frame.ID, completionURL.String())
	if err != nil {
		return fmt.Errorf("managed browser HTTPS CONNECT did not reach the App network gateway: %w", err)
	}
	attestation := managedNetworkHeader(completion.Headers, "X-XiaDown-Gateway-Attestation")
	if !completion.Success || int(completion.HTTPStatusCode) != http.StatusNoContent || attestation != strings.TrimSpace(route.AttestationToken) {
		return fmt.Errorf(
			"managed browser network gateway attestation mismatch (success=%t, status=%d, headerPresent=%t)",
			completion.Success,
			int(completion.HTTPStatusCode),
			attestation != "",
		)
	}
	return nil
}

func loadManagedNetworkProbe(ctx context.Context, frameID cdp.FrameID, rawURL string) (*network.LoadNetworkResourcePageResult, error) {
	var resource *network.LoadNetworkResourcePageResult
	err := chromedp.Run(ctx, chromedp.ActionFunc(func(actionContext context.Context) error {
		var actionErr error
		resource, actionErr = network.LoadNetworkResource(rawURL, &network.LoadNetworkResourceOptions{
			DisableCache:       true,
			IncludeCredentials: false,
		}).WithFrameID(frameID).Do(actionContext)
		return actionErr
	}))
	if err != nil {
		return nil, err
	}
	if resource == nil {
		return nil, errors.New("managed browser network probe returned no result")
	}
	return resource, nil
}

func managedNetworkHeader(headers network.Headers, name string) string {
	for headerName, rawValue := range headers {
		if !strings.EqualFold(strings.TrimSpace(headerName), name) {
			continue
		}
		switch value := rawValue.(type) {
		case string:
			return strings.TrimSpace(value)
		case []string:
			if len(value) > 0 {
				return strings.TrimSpace(value[0])
			}
		}
	}
	return ""
}

const (
	managedAttestationHostSuffix = ".attest.xiadown.invalid"
	managedAttestationPath       = "/.well-known/xiadown-network-route"
)

func managedAttestationURL(rawURL string) (*url.URL, string, error) {
	attestationURL, err := url.Parse(strings.TrimSpace(rawURL))
	if err != nil || attestationURL == nil || !strings.EqualFold(attestationURL.Scheme, "http") ||
		attestationURL.User != nil || attestationURL.Port() != "" || attestationURL.RawQuery != "" || attestationURL.ForceQuery ||
		attestationURL.Fragment != "" || attestationURL.RawPath != "" || attestationURL.Path != managedAttestationPath {
		return nil, "", errors.New("managed browser route attestation is invalid")
	}
	host := strings.ToLower(strings.TrimSpace(attestationURL.Hostname()))
	if !strings.HasSuffix(host, managedAttestationHostSuffix) {
		return nil, "", errors.New("managed browser route attestation has an invalid authority")
	}
	leadingLabel := strings.TrimSuffix(host, managedAttestationHostSuffix)
	if !isLowerHexString(leadingLabel, 32) {
		return nil, "", errors.New("managed browser route attestation has an invalid authority")
	}
	return attestationURL, host, nil
}

func managedConnectProofID(rawChallengeURL string, rawBeginURL string) (string, error) {
	_, beginHost, err := managedAttestationURL(rawBeginURL)
	if err != nil {
		return "", err
	}
	challenge, err := url.Parse(strings.TrimSpace(rawChallengeURL))
	if err != nil || challenge == nil || !strings.EqualFold(challenge.Scheme, "https") || challenge.User != nil ||
		challenge.Port() != "" || challenge.RawQuery != "" || challenge.ForceQuery || challenge.Fragment != "" ||
		challenge.RawPath != "" || challenge.Path != managedAttestationPath {
		return "", errors.New("managed browser CONNECT challenge is invalid")
	}
	host := strings.ToLower(strings.TrimSpace(challenge.Hostname()))
	suffix := ".connect." + beginHost
	if !strings.HasSuffix(host, suffix) {
		return "", errors.New("managed browser CONNECT challenge has an invalid authority")
	}
	proofID := strings.TrimSuffix(host, suffix)
	if !isLowerHexString(proofID, 56) {
		return "", errors.New("managed browser CONNECT challenge has an invalid proof")
	}
	return proofID, nil
}

func isLowerHexString(value string, length int) bool {
	if len(value) != length {
		return false
	}
	for _, character := range value {
		if (character < '0' || character > '9') && (character < 'a' || character > 'f') {
			return false
		}
	}
	return true
}

func (runtime *Runtime) monitorManagedNetworkRoute(ctx context.Context) {
	ticker := time.NewTicker(10 * time.Second)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			if err := runtime.VerifyNetworkRoute(ctx); err != nil {
				zap.L().Error("managed browser network route changed; terminating browser", browserErrorLogField(err))
				runtime.ForceTerminate(0)
				return
			}
		}
	}
}

// VerifyNetworkRoute repeats the launch-time gateway challenge. Managed
// features call it immediately before a public navigation, while the runtime
// monitor catches policy or extension changes during an existing session.
func (runtime *Runtime) VerifyNetworkRoute(ctx context.Context) error {
	if runtime == nil {
		return errors.New("browser runtime is unavailable")
	}
	// A borrowed current-browser session cannot inherit XiaDown's process-level
	// proxy flags. Its trust boundary is the user's existing Chrome network
	// configuration, so managed gateway attestation does not apply.
	if runtime.IsBorrowed() {
		return nil
	}
	runtime.networkRouteVerifyMu.Lock()
	defer runtime.networkRouteVerifyMu.Unlock()

	runtime.mu.Lock()
	browserContext := runtime.browserCtx
	targetManager := runtime.targetManager
	var route *ManagedNetworkRoute
	if runtime.networkRoute != nil {
		routeSnapshot := *runtime.networkRoute
		route = &routeSnapshot
	}
	stopping := runtime.stopping
	runtime.mu.Unlock()
	if route == nil {
		return errors.New("managed browser network route is unavailable")
	}
	if stopping || browserContext == nil {
		return errors.New("managed browser is stopping")
	}
	if ctx == nil {
		ctx = context.Background()
	}
	verifyContext, cancel := context.WithCancel(browserContext)
	stopCallerCancel := context.AfterFunc(ctx, cancel)
	defer func() {
		stopCallerCancel()
		cancel()
	}()
	return verifyManagedNetworkRoute(verifyContext, route, targetManager)
}

func appendBrowserLaunchArgs(args []string, id BrowserID) []string {
	switch id {
	case BrowserVivaldi:
		return append(args, "--disable-vivaldi")
	default:
		return args
	}
}

func (runtime *Runtime) BrowserContext() context.Context {
	runtime.mu.Lock()
	defer runtime.mu.Unlock()
	return runtime.browserCtx
}

func (runtime *Runtime) Ownership() RuntimeOwnership {
	if runtime == nil {
		return RuntimeOwnershipOwned
	}
	runtime.mu.Lock()
	defer runtime.mu.Unlock()
	if runtime.ownership == RuntimeOwnershipBorrowed {
		return RuntimeOwnershipBorrowed
	}
	return RuntimeOwnershipOwned
}

func (runtime *Runtime) IsBorrowed() bool {
	return runtime != nil && runtime.Ownership() == RuntimeOwnershipBorrowed
}

// BorrowedPageTargetInScope limits current-browser capture to page targets in
// the one non-empty Chrome browser context selected when the browser-only CDP
// connection starts. Chrome uses browser contexts to separate profile sessions
// exposed through one debugging endpoint.
func (runtime *Runtime) BorrowedPageTargetInScope(info *targetpkg.Info) bool {
	if runtime == nil || info == nil || info.Type != "page" {
		return false
	}
	runtime.mu.Lock()
	defer runtime.mu.Unlock()
	if runtime.ownership != RuntimeOwnershipBorrowed {
		return false
	}
	return runtime.borrowedContextSet &&
		strings.TrimSpace(string(runtime.borrowedContextID)) != "" &&
		info.BrowserContextID == runtime.borrowedContextID
}

func (runtime *Runtime) registerBorrowedTargetStop(cancel context.CancelFunc) (uint64, bool) {
	if runtime == nil || cancel == nil {
		return 0, false
	}
	runtime.mu.Lock()
	defer runtime.mu.Unlock()
	if runtime.ownership != RuntimeOwnershipBorrowed || runtime.stopping || !runtime.status.Ready {
		return 0, false
	}
	if runtime.borrowedTargetStops == nil {
		runtime.borrowedTargetStops = map[uint64]context.CancelFunc{}
	}
	runtime.borrowedTargetNextID++
	id := runtime.borrowedTargetNextID
	runtime.borrowedTargetStops[id] = cancel
	return id, true
}

func (runtime *Runtime) unregisterBorrowedTargetStop(id uint64) {
	if runtime == nil || id == 0 {
		return
	}
	runtime.mu.Lock()
	delete(runtime.borrowedTargetStops, id)
	runtime.mu.Unlock()
}

func (runtime *Runtime) UserDataDir() string {
	runtime.mu.Lock()
	defer runtime.mu.Unlock()
	return runtime.userDataDir
}

func (runtime *Runtime) Candidate() Candidate {
	runtime.mu.Lock()
	defer runtime.mu.Unlock()
	return runtime.candidate
}

func (runtime *Runtime) ProcessInfo() ProcessInfo {
	if runtime == nil {
		return ProcessInfo{}
	}
	runtime.mu.Lock()
	defer runtime.mu.Unlock()
	return ProcessInfo{
		PID:            runtime.rootPID,
		ProcessGroupID: runtime.processGroupID,
		ExecutablePath: runtime.candidate.ExecPath,
		UserDataDir:    runtime.userDataDir,
	}
}

func (runtime *Runtime) Status() Status {
	runtime.mu.Lock()
	defer runtime.mu.Unlock()
	return runtime.status
}

func (runtime *Runtime) TargetManager() *PageTargetManager {
	if runtime == nil {
		return nil
	}
	runtime.mu.Lock()
	defer runtime.mu.Unlock()
	return runtime.targetManager
}

func (runtime *Runtime) Stopped() bool {
	if runtime == nil {
		return true
	}
	runtime.mu.Lock()
	stopped := runtime.stopped
	runtime.mu.Unlock()
	if stopped == nil {
		return true
	}
	select {
	case <-stopped:
		return true
	default:
		return false
	}
}

func (runtime *Runtime) Stop() {
	if runtime == nil {
		return
	}
	if runtime.IsBorrowed() {
		runtime.stopBorrowed()
		return
	}
	runtime.mu.Lock()
	if runtime.stopping {
		stopped := runtime.stopped
		runtime.mu.Unlock()
		if !waitRuntimeStopped(stopped, 2*time.Second) {
			runtime.ForceTerminate(runtimeTerminateGraceTimeout(runtime.options))
			waitRuntimeStopped(stopped, 2*time.Second)
		}
		return
	}
	cmd := runtime.cmd
	browserCancel := runtime.browserCancel
	allocCancel := runtime.allocCancel
	networkMonitorCancel := runtime.networkMonitorCancel
	targetManager := runtime.targetManager
	stopped := runtime.stopped
	rootPID := runtime.rootPID
	processGroupID := runtime.processGroupID
	registryID := runtime.registryID
	runtime.stopping = true
	runtime.status.Ready = false
	runtime.networkMonitorCancel = nil
	runtime.targetManager = nil
	runtime.mu.Unlock()

	if networkMonitorCancel != nil {
		networkMonitorCancel()
	}
	if targetManager != nil {
		targetManager.Stop()
	}
	gracefulTimeout := runtimeGracefulCloseTimeout(runtime.options)
	runtime.closeBrowserGracefully(gracefulTimeout)
	gracefulStopped := waitRuntimeStopped(stopped, gracefulTimeout)
	if !gracefulStopped && cmd != nil && cmd.Process != nil {
		if rootPID <= 0 || processGroupID <= 0 {
			rootPID, processGroupID = runtimeProcessIDs(cmd)
		}
		killErr := terminateRuntimeProcessGroupWithGrace(rootPID, processGroupID, runtimeTerminateGraceTimeout(runtime.options))
		if killErr != nil {
			zap.L().Warn(
				"browser runtime process group terminate failed",
				zap.Int("pid", rootPID),
				zap.Int("processGroupID", processGroupID),
				zap.String("registryID", registryID),
				browserErrorLogField(killErr),
			)
		}
	}
	cancelRuntimeContextAsync("browser", browserCancel, rootPID, processGroupID, registryID)
	cancelRuntimeContextAsync("allocator", allocCancel, rootPID, processGroupID, registryID)
	stoppedFinally := waitRuntimeStopped(stopped, 2*time.Second)
	if stoppedFinally {
		unregisterRuntimeProcess(registryID)
	}
}

func (runtime *Runtime) RequestGracefulClose(timeout time.Duration) {
	if runtime == nil {
		return
	}
	if runtime.IsBorrowed() {
		go runtime.stopBorrowed()
		return
	}
	runtime.mu.Lock()
	if runtime.stopping {
		runtime.mu.Unlock()
		return
	}
	targetManager := runtime.targetManager
	networkMonitorCancel := runtime.networkMonitorCancel
	runtime.stopping = true
	runtime.status.Ready = false
	runtime.networkMonitorCancel = nil
	runtime.targetManager = nil
	runtime.mu.Unlock()

	if networkMonitorCancel != nil {
		networkMonitorCancel()
	}
	if targetManager != nil {
		targetManager.Stop()
	}
	go runtime.closeBrowserGracefully(timeout)
}

func (runtime *Runtime) WaitStopped(timeout time.Duration) bool {
	if runtime == nil {
		return true
	}
	runtime.mu.Lock()
	stopped := runtime.stopped
	runtime.mu.Unlock()
	return waitRuntimeStopped(stopped, timeout)
}

func (runtime *Runtime) ForceTerminate(killDelay time.Duration) {
	if runtime == nil {
		return
	}
	if runtime.IsBorrowed() {
		runtime.stopBorrowed()
		return
	}
	runtime.mu.Lock()
	cmd := runtime.cmd
	browserCancel := runtime.browserCancel
	allocCancel := runtime.allocCancel
	networkMonitorCancel := runtime.networkMonitorCancel
	targetManager := runtime.targetManager
	stopped := runtime.stopped
	rootPID := runtime.rootPID
	processGroupID := runtime.processGroupID
	registryID := runtime.registryID
	execPath := runtime.candidate.ExecPath
	runtime.stopping = true
	runtime.status.Ready = false
	runtime.networkMonitorCancel = nil
	runtime.targetManager = nil
	runtime.mu.Unlock()

	if networkMonitorCancel != nil {
		networkMonitorCancel()
	}
	if targetManager != nil {
		targetManager.Stop()
	}
	if waitRuntimeStopped(stopped, 1*time.Millisecond) {
		unregisterRuntimeProcess(registryID)
		return
	}
	if cmd != nil && cmd.Process != nil {
		if rootPID <= 0 || processGroupID <= 0 {
			rootPID, processGroupID = runtimeProcessIDs(cmd)
		}
		if err := terminateRuntimeProcessGroupWithGrace(rootPID, processGroupID, killDelay); err != nil {
			zap.L().Warn(
				"browser runtime force terminate failed",
				zap.Int("pid", rootPID),
				zap.Int("processGroupID", processGroupID),
				zap.String("executableRef", browserLogReference(execPath)),
				zap.String("registryID", registryID),
				browserErrorLogField(err),
			)
		}
	}
	cancelRuntimeContextAsync("browser", browserCancel, rootPID, processGroupID, registryID)
	cancelRuntimeContextAsync("allocator", allocCancel, rootPID, processGroupID, registryID)
	if waitRuntimeStopped(stopped, 2*time.Second) {
		unregisterRuntimeProcess(registryID)
	}
}

func runtimeGracefulCloseTimeout(options LaunchOptions) time.Duration {
	if options.PersistentProfile {
		return 3 * time.Second
	}
	return 1500 * time.Millisecond
}

func runtimeTerminateGraceTimeout(options LaunchOptions) time.Duration {
	if options.PersistentProfile {
		return 1 * time.Millisecond
	}
	return 300 * time.Millisecond
}

func cancelRuntimeContextAsync(name string, cancel context.CancelFunc, rootPID int, processGroupID int, registryID string) {
	if cancel == nil {
		return
	}
	go func() {
		defer func() {
			if recovered := recover(); recovered != nil {
				zap.L().Warn(
					"browser runtime context cancel panicked",
					zap.String("context", name),
					zap.Int("pid", rootPID),
					zap.Int("processGroupID", processGroupID),
					zap.String("registryID", registryID),
					browserRecoveredLogField(recovered),
				)
			}
		}()
		cancel()
	}()
}

func (runtime *Runtime) closeBrowserGracefully(timeout time.Duration) {
	if runtime == nil {
		return
	}
	runtime.mu.Lock()
	if runtime.ownership == RuntimeOwnershipBorrowed {
		runtime.mu.Unlock()
		return
	}
	browserCtx := runtime.browserCtx
	runtime.mu.Unlock()
	if browserCtx == nil {
		return
	}
	closeCtx, cancel := context.WithTimeout(browserCtx, timeout)
	defer cancel()
	if err := chromedp.Cancel(closeCtx); err != nil {
		return
	}
}

func (runtime *Runtime) stopBorrowed() {
	if runtime == nil {
		return
	}
	runtime.mu.Lock()
	if runtime.ownership != RuntimeOwnershipBorrowed {
		runtime.mu.Unlock()
		return
	}
	if runtime.stopping {
		stopped := runtime.stopped
		runtime.mu.Unlock()
		waitRuntimeStopped(stopped, 2*time.Second)
		return
	}
	browserCancel := runtime.browserCancel
	allocCancel := runtime.allocCancel
	targetManager := runtime.targetManager
	borrowedTargetStops := make([]context.CancelFunc, 0, len(runtime.borrowedTargetStops))
	for _, cancel := range runtime.borrowedTargetStops {
		borrowedTargetStops = append(borrowedTargetStops, cancel)
	}
	stopped := runtime.stopped
	runtime.stopping = true
	runtime.status.Ready = false
	runtime.browserCancel = nil
	runtime.allocCancel = nil
	runtime.targetManager = nil
	runtime.borrowedTargetStops = nil
	runtime.mu.Unlock()

	// Detach every user-owned page session while the browser websocket is still
	// alive. Each registered cancel is detach-only and cannot CloseTarget.
	for _, cancel := range borrowedTargetStops {
		if cancel != nil {
			cancel()
		}
	}
	if targetManager != nil {
		targetManager.Stop()
	}

	// browserCancel tears down XiaDown's browser-only CDP context. It has no page
	// target and therefore cannot close a user tab or issue Browser.close.
	if browserCancel != nil {
		browserCancel()
	}
	// Closing the remote allocator only closes XiaDown's websocket transport.
	// It never owns, registers, signals, or terminates the Chrome process.
	if allocCancel != nil {
		allocCancel()
	}
	runtime.markRuntimeStopped()
	waitRuntimeStopped(stopped, 2*time.Second)
}

func (runtime *Runtime) markRuntimeStopped() {
	if runtime == nil {
		return
	}
	runtime.mu.Lock()
	runtime.status.Ready = false
	stopped := runtime.stopped
	runtime.mu.Unlock()
	if stopped != nil {
		runtime.stoppedOnce.Do(func() { close(stopped) })
	}
}

func (runtime *Runtime) markBorrowedUnavailable(message string) {
	if runtime == nil {
		return
	}
	runtime.mu.Lock()
	if runtime.ownership == RuntimeOwnershipBorrowed {
		runtime.status.Ready = false
		if runtime.status.DetectError == "" {
			runtime.status.DetectError = strings.TrimSpace(message)
		}
	}
	runtime.mu.Unlock()
}

func (runtime *Runtime) monitorBorrowedConnection(lost <-chan struct{}) {
	if runtime == nil || lost == nil {
		return
	}
	select {
	case <-lost:
		runtime.markBorrowedUnavailable("Chrome closed the debugging connection")
		runtime.stopBorrowed()
	case <-runtime.stopped:
	}
}

func waitRuntimeStopped(stopped <-chan struct{}, timeout time.Duration) bool {
	if stopped == nil {
		return true
	}
	if timeout <= 0 {
		timeout = 2 * time.Second
	}
	select {
	case <-stopped:
		return true
	case <-time.After(timeout):
		return false
	}
}

func stopStartedRuntimeCommand(cmd *exec.Cmd, timeout time.Duration) {
	if cmd == nil || cmd.Process == nil {
		return
	}
	rootPID, processGroupID := runtimeProcessIDs(cmd)
	_ = terminateRuntimeProcessGroup(rootPID, processGroupID)
	done := make(chan struct{})
	go func() {
		_ = cmd.Wait()
		close(done)
	}()
	waitRuntimeStopped(done, timeout)
}

func waitForDevToolsActivePort(ctx context.Context, userDataDir string, timeout time.Duration) (int, string, error) {
	if ctx == nil {
		ctx = context.Background()
	}
	if timeout <= 0 {
		timeout = 10 * time.Second
	}
	deadline := time.Now().Add(timeout)
	path := devToolsActivePortPath(userDataDir)
	var lastErr error
	for {
		if err := ctx.Err(); err != nil {
			return 0, "", err
		}
		port, wsURL, err := readDevToolsActivePort(path)
		if err == nil {
			return port, wsURL, nil
		}
		lastErr = err
		if time.Now().After(deadline) {
			if lastErr == nil {
				lastErr = fmt.Errorf("devtools endpoint not available")
			}
			return 0, "", lastErr
		}
		select {
		case <-ctx.Done():
			return 0, "", ctx.Err()
		case <-time.After(50 * time.Millisecond):
		}
	}
}

func readDevToolsActivePort(path string) (int, string, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return 0, "", err
	}
	lines := strings.Split(strings.TrimSpace(string(data)), "\n")
	if len(lines) < 2 {
		return 0, "", fmt.Errorf("devtools endpoint file incomplete")
	}
	port, err := strconv.Atoi(strings.TrimSpace(lines[0]))
	if err != nil || port <= 0 {
		return 0, "", fmt.Errorf("devtools endpoint port invalid")
	}
	websocketEndpoint := strings.TrimSpace(lines[1])
	if websocketEndpoint == "" {
		return 0, "", fmt.Errorf("devtools websocket path missing")
	}
	if strings.Contains(websocketEndpoint, "://") {
		parsed, err := url.Parse(websocketEndpoint)
		if err != nil || !strings.EqualFold(parsed.Scheme, "ws") || parsed.User != nil || parsed.RawQuery != "" || parsed.Fragment != "" {
			return 0, "", fmt.Errorf("devtools websocket endpoint invalid")
		}
		endpointPort, err := strconv.Atoi(parsed.Port())
		endpointIP := net.ParseIP(strings.Trim(parsed.Hostname(), "[]"))
		if err != nil || endpointPort != port || endpointIP == nil || !endpointIP.IsLoopback() {
			return 0, "", fmt.Errorf("devtools websocket endpoint is not the expected loopback port")
		}
		websocketEndpoint = parsed.EscapedPath()
	}
	if !strings.HasPrefix(websocketEndpoint, "/") {
		websocketEndpoint = "/" + websocketEndpoint
	}
	if !strings.HasPrefix(websocketEndpoint, "/devtools/browser/") || len(strings.TrimPrefix(websocketEndpoint, "/devtools/browser/")) == 0 {
		return 0, "", fmt.Errorf("devtools websocket path invalid")
	}
	return port, fmt.Sprintf("ws://127.0.0.1:%d%s", port, websocketEndpoint), nil
}

func devToolsActivePortPath(userDataDir string) string {
	return filepath.Join(strings.TrimSpace(userDataDir), "DevToolsActivePort")
}
