package tailscale

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"net/url"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"time"
	"unicode"

	"xiadown/internal/domain/libraryaccess"
	"xiadown/internal/infrastructure/processutil"
)

type CommandRunner interface {
	Run(ctx context.Context, executable string, args ...string) ([]byte, error)
}

const (
	defaultCommandTimeout   = 10 * time.Second
	defaultCommandWaitDelay = time.Second
)

// ExecCommandRunner gives every Tailscale CLI invocation its own hard
// deadline. Callers usually provide a request or reconciler context, but some
// application lifecycle contexts intentionally live for the whole process and
// must not allow a stuck desktop CLI to block forever.
type ExecCommandRunner struct {
	commandTimeout time.Duration
	waitDelay      time.Duration
}

func (runner ExecCommandRunner) Run(ctx context.Context, executable string, args ...string) ([]byte, error) {
	if ctx == nil {
		ctx = context.Background()
	}
	if err := ctx.Err(); err != nil {
		return nil, fmt.Errorf("run %s: %w", executable, err)
	}
	resolved, err := resolveCommandExecutable(executable)
	if err != nil {
		return nil, err
	}
	commandTimeout := runner.commandTimeout
	if commandTimeout <= 0 {
		commandTimeout = defaultCommandTimeout
	}
	waitDelay := runner.waitDelay
	if waitDelay <= 0 {
		waitDelay = defaultCommandWaitDelay
	}
	runCtx, cancel := context.WithTimeout(ctx, commandTimeout)
	defer cancel()
	command := exec.CommandContext(runCtx, resolved, args...)
	command.WaitDelay = waitDelay
	processutil.ConfigureCLI(command)
	if executable == "tailscale" && runtime.GOOS == "darwin" {
		// The App Store and Standalone application binary otherwise decides from
		// terminal-related environment variables whether to open its GUI. The
		// official CLI documentation recommends this flag for scripted callers.
		command.Env = environmentWith(os.Environ(), "TAILSCALE_BE_CLI", "1")
	}
	output, commandErr := command.CombinedOutput()
	if contextErr := ctx.Err(); contextErr != nil {
		return output, fmt.Errorf("run %s: %w", filepath.Base(resolved), contextErr)
	}
	if errors.Is(runCtx.Err(), context.DeadlineExceeded) {
		return output, fmt.Errorf(
			"%s command timed out after %s: %w",
			filepath.Base(resolved), commandTimeout, context.DeadlineExceeded,
		)
	}
	if contextErr := runCtx.Err(); contextErr != nil {
		return output, fmt.Errorf("run %s: %w", filepath.Base(resolved), contextErr)
	}
	return output, commandErr
}

func resolveCommandExecutable(executable string) (string, error) {
	if executable != "tailscale" {
		return executable, nil
	}
	for _, candidate := range tailscaleExecutableCandidates(runtime.GOOS, os.Getenv) {
		if filepath.IsAbs(candidate) {
			info, err := os.Stat(candidate)
			if err == nil && info.Mode().IsRegular() {
				return candidate, nil
			}
			continue
		}
		if resolved, err := exec.LookPath(candidate); err == nil {
			return resolved, nil
		}
	}
	return "", exec.ErrNotFound
}

func tailscaleExecutableCandidates(goos string, getenv func(string) string) []string {
	switch goos {
	case "darwin":
		candidates := []string{
			"/Applications/Tailscale.app/Contents/MacOS/Tailscale",
			"/usr/local/bin/tailscale",
			"/opt/homebrew/bin/tailscale",
		}
		if home := strings.TrimSpace(getenv("HOME")); home != "" {
			candidates = append(candidates, filepath.Join(home, "Applications", "Tailscale.app", "Contents", "MacOS", "Tailscale"))
		}
		return append(candidates, "tailscale")
	case "windows":
		candidates := make([]string, 0, 4)
		for _, variable := range []string{"ProgramFiles", "ProgramFiles(x86)"} {
			if directory := strings.TrimSpace(getenv(variable)); directory != "" {
				candidates = append(candidates, filepath.Join(directory, "Tailscale", "tailscale.exe"))
			}
		}
		return append(candidates, "tailscale.exe", "tailscale")
	default:
		return []string{"tailscale"}
	}
}

func environmentWith(values []string, key string, value string) []string {
	prefix := key + "="
	result := make([]string, 0, len(values)+1)
	for _, item := range values {
		separator := strings.IndexByte(item, '=')
		if separator >= 0 && strings.EqualFold(item[:separator]+"=", prefix) {
			continue
		}
		result = append(result, item)
	}
	return append(result, prefix+value)
}

type Manager struct {
	runner CommandRunner
}

var _ libraryaccess.TailscaleManager = (*Manager)(nil)

func NewManager(runner CommandRunner) *Manager {
	if runner == nil {
		runner = ExecCommandRunner{}
	}
	return &Manager{runner: runner}
}

type statusDocument struct {
	Version        string `json:"Version"`
	BackendState   string `json:"BackendState"`
	CurrentTailnet *struct {
		Name           string `json:"Name"`
		MagicDNSSuffix string `json:"MagicDNSSuffix"`
	} `json:"CurrentTailnet"`
	Self *struct {
		DNSName  string `json:"DNSName"`
		HostName string `json:"HostName"`
		Online   bool   `json:"Online"`
	} `json:"Self"`
}

type serveStatusDocument struct {
	TCP map[string]struct {
		HTTPS bool `json:"HTTPS"`
	} `json:"TCP"`
	Web map[string]struct {
		Handlers map[string]struct {
			Proxy string `json:"Proxy"`
		} `json:"Handlers"`
	} `json:"Web"`
}

// Inspect uses the stable machine-readable status command. A missing binary is
// reported as not installed; an installed but stopped or unhealthy daemon is
// reported with its diagnostic without losing parsed identity information.
func (manager *Manager) Inspect(ctx context.Context, httpsPort int, routePath string) libraryaccess.TailscaleInfo {
	if manager == nil || manager.runner == nil {
		return libraryaccess.TailscaleInfo{LastError: "tailscale command runner unavailable"}
	}
	output, commandErr := manager.runner.Run(ctx, "tailscale", "status", "--json")
	if errors.Is(commandErr, exec.ErrNotFound) {
		// The Library status path probes optional transports even when they are
		// disabled. A missing Tailscale CLI is therefore an availability state,
		// not a runtime failure; surfacing exec.ErrNotFound here makes an
		// otherwise healthy LAN-only Library look as though startup failed.
		return libraryaccess.TailscaleInfo{Installed: false}
	}

	info := libraryaccess.TailscaleInfo{Installed: true}
	var document statusDocument
	if err := json.Unmarshal(output, &document); err != nil {
		if commandErr != nil {
			info.LastError = cleanCommandError(commandErr, output)
		} else {
			info.LastError = fmt.Sprintf("decode tailscale status: %v", err)
		}
		return info
	}
	info.Version = strings.TrimSpace(document.Version)
	info.Connected = strings.EqualFold(strings.TrimSpace(document.BackendState), "Running")
	if document.CurrentTailnet != nil {
		info.Tailnet = firstNonEmpty(document.CurrentTailnet.Name, document.CurrentTailnet.MagicDNSSuffix)
	}
	if document.Self != nil {
		info.DNSName = strings.TrimSuffix(strings.TrimSpace(document.Self.DNSName), ".")
		info.Device = firstNonEmpty(document.Self.HostName, info.DNSName)
		info.Connected = info.Connected && document.Self.Online
	}
	if commandErr != nil {
		info.LastError = cleanCommandError(commandErr, output)
		return info
	}
	if !info.Connected || info.DNSName == "" || !validServeArguments(1, httpsPort, routePath) {
		return info
	}
	serveOutput, serveErr := manager.runner.Run(ctx, "tailscale", "serve", "status", "--json")
	if serveErr != nil {
		info.LastError = cleanCommandError(serveErr, serveOutput)
		return info
	}
	var serveDocument serveStatusDocument
	if err := json.Unmarshal(serveOutput, &serveDocument); err != nil {
		info.LastError = fmt.Sprintf("decode tailscale serve status: %v", err)
		return info
	}
	info.RouteChecked = true
	info.RouteExists, info.RouteTarget = serveStatusRoute(serveDocument, info.DNSName, httpsPort, routePath)
	info.RouteBackendPort = exactLoopbackBackendPort(info.RouteTarget)
	if info.RouteExists && info.RouteBackendPort > 0 && serveStatusHTTPS(serveDocument, httpsPort) {
		info.ServeURL = serveURL(info.DNSName, httpsPort, routePath)
	}
	return info
}

func serveStatusRoute(document serveStatusDocument, dnsName string, httpsPort int, routePath string) (bool, string) {
	port := strconv.Itoa(httpsPort)
	for hostPort, server := range document.Web {
		host, candidatePort, err := net.SplitHostPort(hostPort)
		if err != nil || candidatePort != port || !strings.EqualFold(strings.TrimSuffix(host, "."), strings.TrimSuffix(dnsName, ".")) {
			continue
		}
		handler, exists := server.Handlers[routePath]
		if !exists {
			continue
		}
		return true, strings.TrimSpace(handler.Proxy)
	}
	return false, ""
}

func serveStatusHTTPS(document serveStatusDocument, httpsPort int) bool {
	listener, exists := document.TCP[strconv.Itoa(httpsPort)]
	return exists && listener.HTTPS
}

func exactLoopbackBackendPort(value string) int {
	value = strings.TrimSpace(value)
	target, err := url.Parse(value)
	if err != nil || target.Port() == "" {
		return 0
	}
	port, err := strconv.Atoi(target.Port())
	if err != nil || port < 1 || port > 65535 || value != loopbackTarget(port) {
		return 0
	}
	return port
}

// Enable changes only the configured XiaDown path. It deliberately never
// invokes Funnel or a global `serve reset`, so unrelated Tailscale Serve routes
// remain untouched.
func (manager *Manager) Enable(
	ctx context.Context,
	localPort, httpsPort int,
	routePath string,
	ownership libraryaccess.TailscaleRouteOwnership,
) error {
	if manager == nil || manager.runner == nil {
		return errors.New("tailscale command runner unavailable")
	}
	if !validServeArguments(localPort, httpsPort, routePath) || !validOwnership(ownership) {
		return libraryaccess.ErrInvalidConfig
	}
	if _, err := manager.requireMutationOwnership(ctx, httpsPort, routePath, ownership); err != nil {
		return err
	}
	target := loopbackTarget(localPort)
	args := serveArgs(httpsPort, routePath, target)
	output, err := manager.runner.Run(ctx, "tailscale", args...)
	if err != nil {
		return fmt.Errorf("enable XiaDown Tailscale Serve route: %s", cleanCommandError(err, output))
	}
	return nil
}

// Disable uses exactly the same scoped flags as Enable and turns only that
// route off.
func (manager *Manager) Disable(
	ctx context.Context,
	httpsPort int,
	routePath string,
	ownership libraryaccess.TailscaleRouteOwnership,
) error {
	if manager == nil || manager.runner == nil {
		return errors.New("tailscale command runner unavailable")
	}
	if !validServeArguments(1, httpsPort, routePath) || !validOwnership(ownership) {
		return libraryaccess.ErrInvalidConfig
	}
	info, err := manager.requireMutationOwnership(ctx, httpsPort, routePath, ownership)
	if err != nil {
		return err
	}
	if !info.RouteExists {
		return nil
	}
	output, err := manager.runner.Run(ctx, "tailscale", serveArgs(httpsPort, routePath, "off")...)
	if err != nil {
		return fmt.Errorf("disable XiaDown Tailscale Serve route: %s", cleanCommandError(err, output))
	}
	return nil
}

func (manager *Manager) requireMutationOwnership(
	ctx context.Context,
	httpsPort int,
	routePath string,
	ownership libraryaccess.TailscaleRouteOwnership,
) (libraryaccess.TailscaleInfo, error) {
	info := manager.Inspect(ctx, httpsPort, routePath)
	switch {
	case !info.Installed:
		return info, errors.New("Tailscale is not installed")
	case strings.TrimSpace(info.LastError) != "":
		return info, fmt.Errorf("inspect exact Tailscale Serve route before mutation: %s", info.LastError)
	case !info.Connected:
		return info, errors.New("Tailscale is disconnected")
	case !info.RouteChecked:
		return info, errors.New("exact Tailscale Serve route could not be verified")
	case info.RouteExists && !ownership.AllowsBackendPort(info.RouteBackendPort):
		return info, fmt.Errorf(
			"%w: HTTPS port %d path %s is occupied by a handler XiaDown does not own",
			libraryaccess.ErrTailscaleRouteOwnershipConflict,
			httpsPort,
			routePath,
		)
	default:
		return info, nil
	}
}

func validOwnership(ownership libraryaccess.TailscaleRouteOwnership) bool {
	return ownership.BackendPort >= 0 && ownership.BackendPort <= 65535 &&
		ownership.PendingBackendPort >= 0 && ownership.PendingBackendPort <= 65535
}

func loopbackTarget(port int) string {
	return "http://127.0.0.1:" + strconv.Itoa(port)
}

func serveArgs(httpsPort int, routePath, target string) []string {
	return []string{
		"serve", "--bg", "--https=" + strconv.Itoa(httpsPort),
		"--set-path=" + routePath, target,
	}
}

func validServeArguments(localPort, httpsPort int, routePath string) bool {
	if localPort < 1 || localPort > 65535 || httpsPort < 1 || httpsPort > 65535 ||
		routePath == "" || routePath == "/" || !strings.HasPrefix(routePath, "/") ||
		path.Clean(routePath) != routePath || strings.ContainsAny(routePath, "?#\\") {
		return false
	}
	for _, character := range routePath {
		if unicode.IsLetter(character) || unicode.IsDigit(character) || strings.ContainsRune("/-._~", character) {
			continue
		}
		return false
	}
	return true
}

func serveURL(dnsName string, httpsPort int, routePath string) string {
	host := dnsName
	if httpsPort != libraryaccess.DefaultTailscaleHTTPSPort {
		host = net.JoinHostPort(dnsName, strconv.Itoa(httpsPort))
	}
	return (&url.URL{Scheme: "https", Host: host, Path: routePath}).String()
}

func cleanCommandError(err error, output []byte) string {
	if errors.Is(err, context.DeadlineExceeded) || errors.Is(err, context.Canceled) {
		message := strings.TrimSpace(string(output))
		if message != "" {
			// Lead with the lifecycle failure so partial CLI output cannot disguise
			// that the process was killed, while retaining actionable prompts such
			// as the tailnet Serve authorization URL.
			return fmt.Sprintf("%s; output: %s", err, message)
		}
		return err.Error()
	}
	message := strings.TrimSpace(string(output))
	if message != "" {
		return message
	}
	if err == nil {
		return "unknown tailscale command error"
	}
	return err.Error()
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if trimmed := strings.TrimSpace(value); trimmed != "" {
			return strings.TrimSuffix(trimmed, ".")
		}
	}
	return ""
}
