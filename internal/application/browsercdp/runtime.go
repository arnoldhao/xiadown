package browsercdp

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/chromedp/chromedp"
	"go.uber.org/zap"
)

type LaunchOptions struct {
	PreferredBrowser  string
	Headless          bool
	NoSandbox         bool
	ExtraArgs         []string
	UserDataDir       string
	PersistentProfile bool
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
}

type ProcessInfo struct {
	PID            int
	ProcessGroupID int
	ExecutablePath string
	UserDataDir    string
}

type Runtime struct {
	mu sync.Mutex

	options   LaunchOptions
	candidate Candidate
	status    Status

	cmd            *exec.Cmd
	userDataDir    string
	rootPID        int
	processGroupID int
	registryID     string
	allocCtx       context.Context
	allocCancel    context.CancelFunc
	browserCtx     context.Context
	browserCancel  context.CancelFunc
	targetManager  *PageTargetManager
	stopping       bool
	stopped        chan struct{}
}

const runtimeCDPNoTopLevelFrameMessage = "received DOM.documentUpdated when there's no top-level frame"

var ErrNoSupportedBrowser = errors.New("no supported browser detected")

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
	candidates := DetectCandidates()
	candidate, ok := ChooseCandidate(candidates, options.PreferredBrowser)
	if !ok {
		return nil, ErrNoSupportedBrowser
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
	args = appendBrowserLaunchArgs(args, candidate.ID)
	args = appendStartupPageArg(args, options)

	cmd := exec.Command(candidate.ExecPath, args...)
	cmd.Stdout = io.Discard
	cmd.Stderr = io.Discard
	configureProcessGroup(cmd)
	if err := cmd.Start(); err != nil {
		zap.L().Warn(
			"browser runtime launch failed",
			zap.String("preferredBrowser", strings.TrimSpace(options.PreferredBrowser)),
			zap.String("chosenBrowser", string(candidate.ID)),
			zap.String("execPath", candidate.ExecPath),
			zap.Error(err),
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
			zap.String("execPath", candidate.ExecPath),
			zap.Int("pid", rootPID),
			zap.Int("processGroupID", processGroupID),
			zap.Error(registryErr),
		)
	}
	port, wsURL, err := waitForDevToolsActivePort(ctx, userDataDir, 10*time.Second)
	if err != nil {
		zap.L().Warn(
			"browser runtime devtools endpoint wait failed",
			zap.String("preferredBrowser", strings.TrimSpace(options.PreferredBrowser)),
			zap.String("chosenBrowser", string(candidate.ID)),
			zap.String("execPath", candidate.ExecPath),
			zap.Error(err),
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
			zap.String("execPath", candidate.ExecPath),
			zap.Error(err),
		)
		browserCancel()
		allocCancel()
		stopStartedRuntimeCommand(cmd, 2*time.Second)
		unregisterRuntimeProcess(registryID)
		return nil, err
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
				zap.String("execPath", candidate.ExecPath),
				zap.Int("pid", rootPID),
				zap.Int("processGroupID", processGroupID),
				zap.Error(registryErr),
			)
		}
	}
	runtime := &Runtime{
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
		},
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
				zap.String("execPath", candidate.ExecPath),
			)
		}
		close(runtime.stopped)
	}()
	targetManager, err := startPageTargetManager(runtime)
	if err != nil {
		zap.L().Warn(
			"browser runtime target manager start failed",
			zap.String("preferredBrowser", strings.TrimSpace(options.PreferredBrowser)),
			zap.String("chosenBrowser", string(candidate.ID)),
			zap.String("execPath", candidate.ExecPath),
			zap.Error(err),
		)
		runtime.Stop()
		return nil, err
	}
	runtime.mu.Lock()
	runtime.targetManager = targetManager
	runtime.mu.Unlock()

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
	return args
}

func appendBrowserLaunchArgs(args []string, id BrowserID) []string {
	switch id {
	case BrowserVivaldi:
		return append(args, "--disable-vivaldi")
	default:
		return args
	}
}

func appendStartupPageArg(args []string, options LaunchOptions) []string {
	if options.Headless {
		return args
	}
	return append(args, "about:blank")
}

func (runtime *Runtime) BrowserContext() context.Context {
	runtime.mu.Lock()
	defer runtime.mu.Unlock()
	return runtime.browserCtx
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
	targetManager := runtime.targetManager
	stopped := runtime.stopped
	rootPID := runtime.rootPID
	processGroupID := runtime.processGroupID
	registryID := runtime.registryID
	runtime.stopping = true
	runtime.status.Ready = false
	runtime.targetManager = nil
	runtime.mu.Unlock()

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
				zap.Error(killErr),
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
	runtime.mu.Lock()
	if runtime.stopping {
		runtime.mu.Unlock()
		return
	}
	targetManager := runtime.targetManager
	runtime.stopping = true
	runtime.status.Ready = false
	runtime.targetManager = nil
	runtime.mu.Unlock()

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
	runtime.mu.Lock()
	cmd := runtime.cmd
	browserCancel := runtime.browserCancel
	allocCancel := runtime.allocCancel
	targetManager := runtime.targetManager
	stopped := runtime.stopped
	rootPID := runtime.rootPID
	processGroupID := runtime.processGroupID
	registryID := runtime.registryID
	execPath := runtime.candidate.ExecPath
	runtime.stopping = true
	runtime.status.Ready = false
	runtime.targetManager = nil
	runtime.mu.Unlock()

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
				zap.String("execPath", execPath),
				zap.String("registryID", registryID),
				zap.Error(err),
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
					zap.Any("panic", recovered),
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
	websocketPath := strings.TrimSpace(lines[1])
	if websocketPath == "" {
		return 0, "", fmt.Errorf("devtools websocket path missing")
	}
	if strings.HasPrefix(websocketPath, "ws://") || strings.HasPrefix(websocketPath, "wss://") {
		return port, websocketPath, nil
	}
	if !strings.HasPrefix(websocketPath, "/") {
		websocketPath = "/" + websocketPath
	}
	return port, fmt.Sprintf("ws://127.0.0.1:%d%s", port, websocketPath), nil
}

func devToolsActivePortPath(userDataDir string) string {
	return filepath.Join(strings.TrimSpace(userDataDir), "DevToolsActivePort")
}
