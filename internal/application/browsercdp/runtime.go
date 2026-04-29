package browsercdp

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/chromedp/chromedp"
	"go.uber.org/zap"
)

type LaunchOptions struct {
	PreferredBrowser string
	Headless         bool
	NoSandbox        bool
	ExtraArgs        []string
	UserDataDir      string
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

type Runtime struct {
	mu sync.Mutex

	options   LaunchOptions
	candidate Candidate
	status    Status

	cmd           *exec.Cmd
	userDataDir   string
	allocCtx      context.Context
	allocCancel   context.CancelFunc
	browserCtx    context.Context
	browserCancel context.CancelFunc
	stopping      bool
	stopped       chan struct{}
}

type versionResponse struct {
	WebSocketDebuggerURL string `json:"webSocketDebuggerUrl"`
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
		status.DetectError = "no supported browser detected"
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
		return nil, fmt.Errorf("no supported browser detected")
	}

	port, err := reservePort()
	if err != nil {
		return nil, err
	}
	userDataDir := strings.TrimSpace(options.UserDataDir)
	if userDataDir == "" {
		userDataDir = filepath.Join(os.TempDir(), "xiadown", "browsercdp", string(candidate.ID))
	}
	if err := os.MkdirAll(userDataDir, 0o755); err != nil {
		return nil, err
	}

	args := buildLaunchArgs(port, userDataDir, options)
	if options.NoSandbox {
		args = append([]string{"--no-sandbox"}, args...)
	}
	for _, extra := range options.ExtraArgs {
		if trimmed := strings.TrimSpace(extra); trimmed != "" {
			args = append(args, trimmed)
		}
	}
	args = appendStartupPageArg(args, options)

	cmd := exec.Command(candidate.ExecPath, args...)
	cmd.Stdout = io.Discard
	cmd.Stderr = io.Discard
	cmd.SysProcAttr = &syscall.SysProcAttr{}
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

	if err := WaitForCDP(ctx, "127.0.0.1", port, 10*time.Second); err != nil {
		zap.L().Warn(
			"browser runtime cdp wait failed",
			zap.String("preferredBrowser", strings.TrimSpace(options.PreferredBrowser)),
			zap.String("chosenBrowser", string(candidate.ID)),
			zap.String("execPath", candidate.ExecPath),
			zap.Int("cdpPort", port),
			zap.Error(err),
		)
		_ = cmd.Process.Kill()
		return nil, err
	}
	wsURL, err := fetchWebSocketURL(ctx, port)
	if err != nil {
		zap.L().Warn(
			"browser runtime websocket resolve failed",
			zap.String("preferredBrowser", strings.TrimSpace(options.PreferredBrowser)),
			zap.String("chosenBrowser", string(candidate.ID)),
			zap.String("execPath", candidate.ExecPath),
			zap.Int("cdpPort", port),
			zap.Error(err),
		)
		_ = cmd.Process.Kill()
		return nil, err
	}
	allocCtx, allocCancel := chromedp.NewRemoteAllocator(context.Background(), wsURL)
	browserCtx, browserCancel := chromedp.NewContext(allocCtx)
	if _, err := chromedp.Targets(browserCtx); err != nil {
		zap.L().Warn(
			"browser runtime chromedp attach failed",
			zap.String("preferredBrowser", strings.TrimSpace(options.PreferredBrowser)),
			zap.String("chosenBrowser", string(candidate.ID)),
			zap.String("execPath", candidate.ExecPath),
			zap.String("cdpUrl", wsURL),
			zap.Error(err),
		)
		browserCancel()
		allocCancel()
		_ = cmd.Process.Kill()
		return nil, err
	}

	runtime := &Runtime{
		options:       options,
		candidate:     candidate,
		cmd:           cmd,
		userDataDir:   userDataDir,
		allocCtx:      allocCtx,
		allocCancel:   allocCancel,
		browserCtx:    browserCtx,
		browserCancel: browserCancel,
		stopped:       make(chan struct{}),
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
				zap.Int("cdpPort", port),
			)
		}
		close(runtime.stopped)
	}()

	return runtime, nil
}

func buildLaunchArgs(port int, userDataDir string, options LaunchOptions) []string {
	args := []string{
		fmt.Sprintf("--remote-debugging-port=%d", port),
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
		"--password-store=basic",
		"--use-mock-keychain",
	}
	if options.Headless {
		return append([]string{"--headless=new", "--hide-scrollbars", "--mute-audio"}, args...)
	}
	return args
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

func (runtime *Runtime) Status() Status {
	runtime.mu.Lock()
	defer runtime.mu.Unlock()
	return runtime.status
}

func (runtime *Runtime) Stop() {
	if runtime == nil {
		return
	}
	runtime.mu.Lock()
	cmd := runtime.cmd
	browserCancel := runtime.browserCancel
	allocCancel := runtime.allocCancel
	stopped := runtime.stopped
	runtime.stopping = true
	runtime.status.Ready = false
	runtime.mu.Unlock()

	if browserCancel != nil {
		browserCancel()
	}
	if allocCancel != nil {
		allocCancel()
	}
	if cmd != nil && cmd.Process != nil {
		_ = cmd.Process.Kill()
	}
	if stopped != nil {
		select {
		case <-stopped:
		case <-time.After(2 * time.Second):
		}
	}
}

func fetchWebSocketURL(ctx context.Context, port int) (string, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, fmt.Sprintf("http://127.0.0.1:%d/json/version", port), nil)
	if err != nil {
		return "", err
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("unexpected cdp status %d", resp.StatusCode)
	}
	var payload versionResponse
	if err := json.NewDecoder(resp.Body).Decode(&payload); err != nil {
		return "", err
	}
	if strings.TrimSpace(payload.WebSocketDebuggerURL) == "" {
		return "", fmt.Errorf("webSocketDebuggerUrl missing")
	}
	return strings.TrimSpace(payload.WebSocketDebuggerURL), nil
}

func reservePort() (int, error) {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return 0, err
	}
	defer listener.Close()
	addr, ok := listener.Addr().(*net.TCPAddr)
	if !ok {
		return 0, fmt.Errorf("failed to reserve tcp port")
	}
	return addr.Port, nil
}
