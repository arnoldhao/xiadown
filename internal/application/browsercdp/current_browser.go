package browsercdp

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"net/url"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"time"

	browserpkg "github.com/chromedp/cdproto/browser"
	"github.com/chromedp/cdproto/cdp"
	targetpkg "github.com/chromedp/cdproto/target"
	"github.com/chromedp/chromedp"
	"github.com/google/uuid"
)

const (
	CurrentBrowserStateReady                   = "ready"
	CurrentBrowserStateNotInstalled            = "not_installed"
	CurrentBrowserStateNotRunning              = "not_running"
	CurrentBrowserStateRemoteDebuggingDisabled = "remote_debugging_disabled"
	CurrentBrowserStatePermissionDenied        = "permission_denied"
	CurrentBrowserStateUnsupportedVersion      = "unsupported_version"
	CurrentBrowserStateEndpointUnavailable     = "endpoint_unavailable"
	CurrentBrowserStateUnsupportedBrowser      = "unsupported_browser"

	// Chrome 144 is the first supported consent-gated remote-debugging bridge.
	CurrentChromeMinimumMajorVersion  = 144
	currentBrowserEndpointReadLimit   = 64 * 1024
	currentBrowserLocalStateReadLimit = 4 * 1024 * 1024
)

// CurrentBrowserStatus is deliberately path-free. The UI may explain how to
// enable Chrome's consent-gated remote debugging bridge, but it must never be
// allowed to choose a user-data directory or CDP endpoint.
type CurrentBrowserStatus struct {
	BrowserID      string `json:"browserId"`
	State          string `json:"state"`
	Installed      bool   `json:"installed"`
	Running        bool   `json:"running"`
	Supported      bool   `json:"supported"`
	Ready          bool   `json:"ready"`
	Version        string `json:"version,omitempty"`
	MinimumVersion int    `json:"minimumVersion"`
	ProfileName    string `json:"profileName,omitempty"`
	Detail         string `json:"detail,omitempty"`
}

type CurrentBrowserError struct {
	State string
	Err   error
}

func (err *CurrentBrowserError) Error() string {
	if err == nil {
		return ""
	}
	if err.Err != nil {
		return err.Err.Error()
	}
	return strings.TrimSpace(err.State)
}

func (err *CurrentBrowserError) Unwrap() error {
	if err == nil {
		return nil
	}
	return err.Err
}

func CurrentBrowserErrorState(err error) string {
	var current *CurrentBrowserError
	if errors.As(err, &current) && current != nil {
		return strings.TrimSpace(current.State)
	}
	return ""
}

type currentBrowserInspection struct {
	status       CurrentBrowserStatus
	candidate    Candidate
	userDataDir  string
	websocketURL string
}

type currentChromeLocalState struct {
	Profile struct {
		LastUsed  string `json:"last_used"`
		InfoCache map[string]struct {
			Name string `json:"name"`
		} `json:"info_cache"`
	} `json:"profile"`
}

var (
	resolveCurrentChromeUserDataDir = defaultCurrentChromeUserDataDir
	resolveCurrentChromeCandidate   = defaultCurrentChromeCandidate
	detectCurrentChromeRunning      = currentChromeProcessRunning
)

func InspectCurrentBrowser(ctx context.Context, browserID string) CurrentBrowserStatus {
	inspection := inspectCurrentBrowser(ctx, browserID)
	return inspection.status
}

func inspectCurrentBrowser(ctx context.Context, browserID string) currentBrowserInspection {
	browserID = strings.ToLower(strings.TrimSpace(browserID))
	status := CurrentBrowserStatus{
		BrowserID:      browserID,
		State:          CurrentBrowserStateUnsupportedBrowser,
		MinimumVersion: CurrentChromeMinimumMajorVersion,
	}
	if browserID != string(BrowserChrome) {
		status.Detail = "Only the current stable Chrome browser is supported."
		return currentBrowserInspection{status: status}
	}

	candidate, err := resolveCurrentChromeCandidate()
	if err != nil || !candidate.Available {
		status.State = CurrentBrowserStateNotInstalled
		status.Detail = "Chrome Stable is not installed."
		return currentBrowserInspection{status: status}
	}
	status.Installed = true
	status.Supported = true

	userDataDir, err := resolveCurrentChromeUserDataDir()
	if err != nil || strings.TrimSpace(userDataDir) == "" {
		status.State = CurrentBrowserStateEndpointUnavailable
		status.Detail = "Chrome's default profile location is unavailable."
		return currentBrowserInspection{status: status, candidate: candidate}
	}
	inspection := inspectCurrentChromeAt(ctx, candidate, userDataDir, detectCurrentChromeRunning(userDataDir, candidate))
	inspection.status.BrowserID = browserID
	inspection.status.Installed = true
	inspection.status.MinimumVersion = CurrentChromeMinimumMajorVersion
	if inspection.status.State != CurrentBrowserStateUnsupportedVersion {
		inspection.status.Supported = true
	}
	return inspection
}

func inspectCurrentChromeAt(_ context.Context, candidate Candidate, userDataDir string, running bool) currentBrowserInspection {
	status := CurrentBrowserStatus{
		BrowserID:      string(BrowserChrome),
		State:          CurrentBrowserStateEndpointUnavailable,
		Installed:      candidate.Available,
		Running:        running,
		Supported:      true,
		MinimumVersion: CurrentChromeMinimumMajorVersion,
	}
	inspection := currentBrowserInspection{status: status, candidate: candidate, userDataDir: userDataDir}

	if err := validateTrustedCurrentBrowserDirectory(userDataDir); err != nil {
		if errors.Is(err, os.ErrNotExist) {
			inspection.status.State = CurrentBrowserStateNotRunning
			inspection.status.Detail = "Chrome is not running with its default profile."
			return inspection
		}
		if errors.Is(err, os.ErrPermission) {
			inspection.status.State = CurrentBrowserStatePermissionDenied
			inspection.status.Detail = "XiaDown cannot read Chrome's profile metadata."
			return inspection
		}
		inspection.status.Detail = "Chrome's default profile did not pass the local security check."
		return inspection
	}
	inspection.status.ProfileName = readCurrentChromeProfileName(userDataDir)
	if !running {
		inspection.status.State = CurrentBrowserStateNotRunning
		inspection.status.Detail = "Chrome is not running."
		return inspection
	}

	activePortPath := filepath.Join(userDataDir, "DevToolsActivePort")
	data, err := readTrustedCurrentBrowserFile(userDataDir, activePortPath, currentBrowserEndpointReadLimit)
	if err != nil {
		switch {
		case errors.Is(err, os.ErrNotExist):
			inspection.status.State = CurrentBrowserStateRemoteDebuggingDisabled
			inspection.status.Detail = "Enable Remote Debugging in chrome://inspect/#remote-debugging."
		case errors.Is(err, os.ErrPermission):
			inspection.status.State = CurrentBrowserStatePermissionDenied
			inspection.status.Detail = "XiaDown cannot read Chrome's remote debugging endpoint."
		default:
			inspection.status.Detail = "Chrome's remote debugging endpoint did not pass the local security check."
		}
		return inspection
	}
	_, websocketURL, err := parseTrustedDevToolsActivePort(data)
	if err != nil {
		inspection.status.Detail = "Chrome's remote debugging endpoint is invalid."
		return inspection
	}
	inspection.status.Running = true
	inspection.websocketURL = websocketURL
	inspection.status.State = CurrentBrowserStateReady
	inspection.status.Ready = true
	inspection.status.Detail = ""
	return inspection
}

func defaultCurrentChromeCandidate() (Candidate, error) {
	identity := DetectExecutableIdentity(string(BrowserChrome), "")
	return candidateForExecutableIdentity(identity, string(BrowserChrome))
}

func defaultCurrentChromeUserDataDir() (string, error) {
	switch runtime.GOOS {
	case "darwin":
		home, err := os.UserHomeDir()
		if err != nil {
			return "", err
		}
		return filepath.Join(home, "Library", "Application Support", "Google", "Chrome"), nil
	case "windows":
		local := strings.TrimSpace(os.Getenv("LOCALAPPDATA"))
		if local == "" || !filepath.IsAbs(local) {
			return "", errors.New("LOCALAPPDATA is unavailable")
		}
		return filepath.Join(local, "Google", "Chrome", "User Data"), nil
	default:
		configDir, err := os.UserConfigDir()
		if err != nil {
			return "", err
		}
		return filepath.Join(configDir, "google-chrome"), nil
	}
}

func readCurrentChromeProfileName(userDataDir string) string {
	data, err := readTrustedCurrentBrowserFile(
		userDataDir,
		filepath.Join(userDataDir, "Local State"),
		currentBrowserLocalStateReadLimit,
	)
	if err != nil {
		return ""
	}
	var state currentChromeLocalState
	if err := json.Unmarshal(data, &state); err != nil {
		return ""
	}
	lastUsed := strings.TrimSpace(state.Profile.LastUsed)
	if lastUsed == "" || len(lastUsed) > 256 {
		return ""
	}
	entry, ok := state.Profile.InfoCache[lastUsed]
	if !ok {
		return ""
	}
	name := strings.TrimSpace(entry.Name)
	if len(name) > 256 {
		return ""
	}
	return name
}

func parseTrustedDevToolsActivePort(data []byte) (int, string, error) {
	lines := strings.Split(strings.TrimSpace(string(data)), "\n")
	if len(lines) != 2 {
		return 0, "", errors.New("devtools endpoint file must contain exactly two lines")
	}
	port, err := strconv.Atoi(strings.TrimSpace(lines[0]))
	if err != nil || port <= 0 || port > 65535 {
		return 0, "", errors.New("devtools endpoint port invalid")
	}
	websocketURL, err := normalizeCurrentBrowserWebSocketURL(strings.TrimSpace(lines[1]), port)
	if err != nil {
		return 0, "", err
	}
	return port, websocketURL, nil
}

func normalizeCurrentBrowserWebSocketURL(value string, port int) (string, error) {
	value = strings.TrimSpace(value)
	if value == "" || port <= 0 || port > 65535 {
		return "", errors.New("devtools websocket endpoint invalid")
	}
	if !strings.Contains(value, "://") {
		if !strings.HasPrefix(value, "/") {
			value = "/" + value
		}
		value = fmt.Sprintf("ws://127.0.0.1:%d%s", port, value)
	}
	parsed, err := url.Parse(value)
	if err != nil || !strings.EqualFold(parsed.Scheme, "ws") || parsed.User != nil || parsed.RawQuery != "" || parsed.Fragment != "" {
		return "", errors.New("devtools websocket endpoint invalid")
	}
	host := strings.TrimSpace(parsed.Hostname())
	ip := net.ParseIP(strings.Trim(host, "[]"))
	endpointPort, portErr := strconv.Atoi(parsed.Port())
	if portErr != nil || endpointPort != port || ip == nil || !ip.Equal(net.ParseIP("127.0.0.1")) {
		return "", errors.New("devtools websocket endpoint must use the expected loopback port")
	}
	path := parsed.Path
	identifier := strings.TrimPrefix(path, "/devtools/browser/")
	parsedIdentifier, identifierErr := uuid.Parse(identifier)
	if identifier == path || identifierErr != nil || parsedIdentifier.String() != strings.ToLower(identifier) {
		return "", errors.New("devtools websocket browser path invalid")
	}
	return fmt.Sprintf("ws://127.0.0.1:%d/devtools/browser/%s", port, parsedIdentifier.String()), nil
}

func parseCurrentChromeVersion(product string) (int, string, error) {
	product = strings.TrimSpace(product)
	if !strings.HasPrefix(product, "Chrome/") {
		return 0, "", errors.New("debugging endpoint product is not Chrome")
	}
	version := strings.TrimSpace(strings.TrimPrefix(product, "Chrome/"))
	parts := strings.Split(version, ".")
	if len(parts) < 2 || len(parts) > 4 {
		return 0, "", errors.New("Chrome version invalid")
	}
	for _, part := range parts {
		if part == "" {
			return 0, "", errors.New("Chrome version invalid")
		}
		for _, character := range part {
			if character < '0' || character > '9' {
				return 0, "", errors.New("Chrome version invalid")
			}
		}
	}
	major, err := strconv.Atoi(parts[0])
	if err != nil || major <= 0 {
		return 0, "", errors.New("Chrome version invalid")
	}
	return major, version, nil
}

func validateCurrentChromeProduct(product string) (string, error) {
	major, version, err := parseCurrentChromeVersion(product)
	if err != nil {
		return "", &CurrentBrowserError{
			State: CurrentBrowserStateUnsupportedBrowser,
			Err:   errors.New("the approved debugging endpoint is not Chrome"),
		}
	}
	if major < CurrentChromeMinimumMajorVersion {
		return version, &CurrentBrowserError{
			State: CurrentBrowserStateUnsupportedVersion,
			Err:   fmt.Errorf("Chrome %d or newer is required", CurrentChromeMinimumMajorVersion),
		}
	}
	return version, nil
}

// StartBorrowedCurrentBrowser attaches to Chrome's consent-gated endpoint as a
// browser-level connection. It deliberately does not create or select a page:
// the caller may attach, detach and observe every page in the selected profile
// without XiaDown adding a blank tab to the user's browser.
func StartBorrowedCurrentBrowser(ctx context.Context, browserID string) (*Runtime, error) {
	inspection := inspectCurrentBrowser(ctx, browserID)
	if !inspection.status.Ready || strings.TrimSpace(inspection.websocketURL) == "" {
		return nil, &CurrentBrowserError{
			State: inspection.status.State,
			Err:   errors.New(firstNonEmptyCurrentBrowserDetail(inspection.status.Detail, "current browser is unavailable")),
		}
	}

	allocCtx, allocCancel := chromedp.NewRemoteAllocator(
		context.Background(),
		inspection.websocketURL,
		chromedp.NoModifyURL,
	)
	browserCtx, browserCancel := chromedp.NewContext(
		allocCtx,
		chromedp.WithErrorf(runtimeCDPErrorf),
		chromedp.WithBrowserOption(chromedp.WithDialTimeout(30*time.Second)),
	)
	// Targets initializes only the browser websocket; unlike chromedp.Run it
	// does not create or attach a page target. It must receive the persistent
	// browserCtx because RemoteAllocator binds the websocket to its first call.
	initialTargets, err := connectBorrowedCurrentBrowser(ctx, browserCtx, browserCancel, 30*time.Second)
	if err != nil {
		browserCancel()
		allocCancel()
		return nil, &CurrentBrowserError{
			State: CurrentBrowserStatePermissionDenied,
			Err:   fmt.Errorf("Chrome did not approve the debugging connection: %w", err),
		}
	}
	chromeContext := chromedp.FromContext(browserCtx)
	if chromeContext == nil || chromeContext.Browser == nil {
		browserCancel()
		allocCancel()
		return nil, &CurrentBrowserError{
			State: CurrentBrowserStateEndpointUnavailable,
			Err:   errors.New("Chrome did not establish the debugging connection"),
		}
	}
	profileContextID, ok := currentBrowserProfileContext(initialTargets)
	if !ok {
		browserCancel()
		allocCancel()
		return nil, &CurrentBrowserError{
			State: CurrentBrowserStateEndpointUnavailable,
			Err:   errors.New("Chrome did not expose a trusted profile context"),
		}
	}
	versionCtx, versionCancel := context.WithTimeout(browserCtx, 5*time.Second)
	_, product, _, _, _, err := browserpkg.GetVersion().Do(cdp.WithExecutor(versionCtx, chromeContext.Browser))
	versionCancel()
	if err != nil {
		browserCancel()
		allocCancel()
		return nil, &CurrentBrowserError{
			State: CurrentBrowserStateEndpointUnavailable,
			Err:   fmt.Errorf("verify the approved Chrome debugging endpoint: %w", err),
		}
	}
	if _, err := validateCurrentChromeProduct(product); err != nil {
		browserCancel()
		allocCancel()
		return nil, err
	}

	runtimeBrowser := &Runtime{
		ownership:          RuntimeOwnershipBorrowed,
		candidate:          inspection.candidate,
		userDataDir:        inspection.userDataDir,
		allocCtx:           allocCtx,
		allocCancel:        allocCancel,
		browserCtx:         browserCtx,
		browserCancel:      browserCancel,
		borrowedContextID:  profileContextID,
		borrowedContextSet: true,
		stopped:            make(chan struct{}),
		status: Status{
			Ready:                  true,
			SelectedBrowser:        string(BrowserChrome),
			ChosenBrowser:          string(BrowserChrome),
			DetectedExecutablePath: inspection.candidate.ExecPath,
			CDPURL:                 inspection.websocketURL,
			Headless:               false,
			Ownership:              string(RuntimeOwnershipBorrowed),
		},
	}
	targetManager, err := startPageTargetManager(runtimeBrowser)
	if err != nil {
		runtimeBrowser.Stop()
		return nil, &CurrentBrowserError{
			State: CurrentBrowserStateEndpointUnavailable,
			Err:   fmt.Errorf("start current Chrome target discovery: %w", err),
		}
	}
	runtimeBrowser.mu.Lock()
	runtimeBrowser.targetManager = targetManager
	runtimeBrowser.mu.Unlock()
	go runtimeBrowser.monitorBorrowedConnection(chromeContext.Browser.LostConnection)
	return runtimeBrowser, nil
}

type borrowedCurrentBrowserConnectResult struct {
	targets []*targetpkg.Info
	err     error
}

func connectBorrowedCurrentBrowser(callerCtx context.Context, browserCtx context.Context, browserCancel context.CancelFunc, timeout time.Duration) ([]*targetpkg.Info, error) {
	if callerCtx == nil {
		callerCtx = context.Background()
	}
	if timeout <= 0 {
		timeout = 30 * time.Second
	}
	result := make(chan borrowedCurrentBrowserConnectResult, 1)
	go func() {
		targets, err := chromedp.Targets(browserCtx)
		result <- borrowedCurrentBrowserConnectResult{targets: targets, err: err}
	}()
	timer := time.NewTimer(timeout)
	defer timer.Stop()
	select {
	case connected := <-result:
		return connected.targets, connected.err
	case <-timer.C:
		if browserCancel != nil {
			browserCancel()
		}
		return nil, context.DeadlineExceeded
	case <-browserCtx.Done():
		return nil, browserCtx.Err()
	case <-callerCtx.Done():
		if browserCancel != nil {
			browserCancel()
		}
		return nil, callerCtx.Err()
	}
}

func currentBrowserProfileContext(targets []*targetpkg.Info) (cdp.BrowserContextID, bool) {
	var selected cdp.BrowserContextID
	for _, info := range targets {
		if !isPageTargetInfo(info) || strings.TrimSpace(string(info.BrowserContextID)) == "" {
			continue
		}
		if selected == "" {
			selected = info.BrowserContextID
			continue
		}
		if info.BrowserContextID != selected {
			// The consent endpoint exposed more than one profile context and CDP
			// does not provide a trustworthy profile-name mapping. Fail closed
			// instead of selecting an arbitrary profile and reading the wrong tabs.
			return "", false
		}
	}
	return selected, selected != ""
}

func firstNonEmptyCurrentBrowserDetail(values ...string) string {
	for _, value := range values {
		if value = strings.TrimSpace(value); value != "" {
			return value
		}
	}
	return ""
}
