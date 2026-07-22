package tailscale

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"strconv"
	"strings"
	"testing"
	"time"

	"xiadown/internal/domain/libraryaccess"
)

const execCommandRunnerHelperMode = "XIADOWN_TEST_TAILSCALE_COMMAND_MODE"

func TestExecCommandRunnerHelperProcess(t *testing.T) {
	switch os.Getenv(execCommandRunnerHelperMode) {
	case "hang":
		_, _ = os.Stdout.WriteString("partial output before hang\n")
		time.Sleep(30 * time.Second)
	case "success":
		_, _ = os.Stdout.WriteString("command completed\n")
	}
}

func runExecCommandRunnerHelper(t *testing.T, runner ExecCommandRunner, ctx context.Context, mode string) ([]byte, error) {
	t.Helper()
	t.Setenv(execCommandRunnerHelperMode, mode)
	return runner.Run(ctx, os.Args[0], "-test.run=^TestExecCommandRunnerHelperProcess$")
}

func TestExecCommandRunnerEnforcesHardTimeoutAndPreservesCause(t *testing.T) {
	runner := ExecCommandRunner{commandTimeout: 40 * time.Millisecond, waitDelay: 40 * time.Millisecond}
	started := time.Now()
	output, err := runExecCommandRunnerHelper(t, runner, context.Background(), "hang")
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("timeout error = %v", err)
	}
	if elapsed := time.Since(started); elapsed > time.Second {
		t.Fatalf("hard timeout returned after %s", elapsed)
	}
	if !strings.Contains(string(output), "partial output before hang") {
		t.Fatalf("partial output = %q", output)
	}
	message := cleanCommandError(err, output)
	if !strings.Contains(message, "timed out") || !strings.Contains(message, "output: partial output") ||
		strings.Index(message, "timed out") > strings.Index(message, "partial output") {
		t.Fatalf("clean timeout error = %q", message)
	}
}

func TestExecCommandRunnerHonorsShorterCallerDeadline(t *testing.T) {
	runner := ExecCommandRunner{commandTimeout: time.Second, waitDelay: 40 * time.Millisecond}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Millisecond)
	defer cancel()
	started := time.Now()
	_, err := runExecCommandRunnerHelper(t, runner, ctx, "hang")
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("caller deadline error = %v", err)
	}
	if elapsed := time.Since(started); elapsed > time.Second {
		t.Fatalf("caller deadline returned after %s", elapsed)
	}
}

func TestExecCommandRunnerReturnsSuccessfulOutput(t *testing.T) {
	// Race-instrumented test binaries can take several seconds to initialize on
	// loaded CI workers; this case verifies success output, not the deadline.
	runner := ExecCommandRunner{commandTimeout: 10 * time.Second, waitDelay: 40 * time.Millisecond}
	output, err := runExecCommandRunnerHelper(t, runner, context.Background(), "success")
	if err != nil || !strings.Contains(string(output), "command completed\n") {
		t.Fatalf("successful command = %q, %v", output, err)
	}
}

func TestExecCommandRunnerRejectsAlreadyCancelledContext(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	started := time.Now()
	_, err := (ExecCommandRunner{}).Run(ctx, "command-that-must-not-be-resolved")
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("cancelled context error = %v", err)
	}
	if elapsed := time.Since(started); elapsed > 100*time.Millisecond {
		t.Fatalf("cancelled context returned after %s", elapsed)
	}
}

type runnerCall struct {
	executable string
	args       []string
}

type stubRunner struct {
	outputs [][]byte
	errors  []error
	calls   []runnerCall
}

func (runner *stubRunner) Run(_ context.Context, executable string, args ...string) ([]byte, error) {
	runner.calls = append(runner.calls, runnerCall{executable: executable, args: append([]string(nil), args...)})
	index := len(runner.calls) - 1
	var output []byte
	var err error
	if index < len(runner.outputs) {
		output = runner.outputs[index]
	}
	if index < len(runner.errors) {
		err = runner.errors[index]
	}
	return output, err
}

func TestManagerInspectParsesStatusJSON(t *testing.T) {
	runner := &stubRunner{outputs: [][]byte{[]byte(`{
  "Version": "1.82.5-t123",
  "BackendState": "Running",
  "CurrentTailnet": {"Name": "Example Org", "MagicDNSSuffix": "example.ts.net"},
  "Self": {"DNSName": "studio.example.ts.net.", "HostName": "studio", "Online": true}
}`), []byte(`{
  "TCP": {"443": {"HTTPS": true}},
  "Web": {"studio.example.ts.net:443": {"Handlers": {
    "/xiadown": {"Proxy": "http://127.0.0.1:43123"}
  }}}
}`)}}
	info := NewManager(runner).Inspect(context.Background(), 443, "/xiadown")
	if !info.Installed || !info.Connected || info.Version != "1.82.5-t123" ||
		info.Tailnet != "Example Org" || info.Device != "studio" ||
		info.DNSName != "studio.example.ts.net" || info.ServeURL != "https://studio.example.ts.net/xiadown" ||
		!info.RouteChecked || !info.RouteExists || info.RouteTarget != "http://127.0.0.1:43123" ||
		info.RouteBackendPort != 43123 || info.LastError != "" {
		t.Fatalf("unexpected info: %+v", info)
	}
	want := []runnerCall{
		{executable: "tailscale", args: []string{"status", "--json"}},
		{executable: "tailscale", args: []string{"serve", "status", "--json"}},
	}
	if !reflect.DeepEqual(runner.calls, want) {
		t.Fatalf("calls = %#v, want %#v", runner.calls, want)
	}
}

func TestTailscaleExecutableCandidatesCoverOfficialDesktopInstalls(t *testing.T) {
	values := map[string]string{
		"HOME":              "/Users/tester",
		"ProgramFiles":      `C:\Program Files`,
		"ProgramFiles(x86)": `C:\Program Files (x86)`,
	}
	getenv := func(key string) string { return values[key] }
	mac := tailscaleExecutableCandidates("darwin", getenv)
	for _, expected := range []string{
		"/Applications/Tailscale.app/Contents/MacOS/Tailscale",
		"/usr/local/bin/tailscale",
		filepath.Join("/Users/tester", "Applications", "Tailscale.app", "Contents", "MacOS", "Tailscale"),
	} {
		if !containsString(mac, expected) {
			t.Fatalf("macOS candidates %q omit %q", mac, expected)
		}
	}
	windows := tailscaleExecutableCandidates("windows", getenv)
	if !containsString(windows, filepath.Join(`C:\Program Files`, "Tailscale", "tailscale.exe")) ||
		!containsString(windows, "tailscale.exe") {
		t.Fatalf("Windows candidates = %q", windows)
	}
}

func TestEnvironmentWithForcesSingleTailscaleCLIMode(t *testing.T) {
	result := environmentWith(
		[]string{"PATH=/usr/bin", "tailscale_be_cli=0", "HOME=/Users/tester"},
		"TAILSCALE_BE_CLI", "1",
	)
	count := 0
	for _, value := range result {
		if strings.EqualFold(strings.SplitN(value, "=", 2)[0], "TAILSCALE_BE_CLI") {
			count++
			if value != "TAILSCALE_BE_CLI=1" {
				t.Fatalf("CLI mode = %q", value)
			}
		}
	}
	if count != 1 {
		t.Fatalf("CLI mode entries = %d in %q", count, result)
	}
}

func containsString(values []string, expected string) bool {
	for _, value := range values {
		if value == expected {
			return true
		}
	}
	return false
}

func TestManagerInspectDoesNotPresentCandidateURLAsActiveServe(t *testing.T) {
	status := []byte(`{
  "Version":"1.98.8", "BackendState":"Running",
  "Self":{"DNSName":"studio.example.ts.net.","HostName":"studio","Online":true}
}`)
	for name, testCase := range map[string]struct {
		serveStatus      string
		routeExists      bool
		routeTarget      string
		routeBackendPort int
	}{
		"empty":        {serveStatus: `{}`},
		"wrong path":   {serveStatus: `{"TCP":{"443":{"HTTPS":true}},"Web":{"studio.example.ts.net:443":{"Handlers":{"/other":{"Proxy":"http://127.0.0.1:43123"}}}}}`},
		"wrong target": {serveStatus: `{"TCP":{"443":{"HTTPS":true}},"Web":{"studio.example.ts.net:443":{"Handlers":{"/xiadown":{"Proxy":"http://192.168.1.2:43123"}}}}}`, routeExists: true, routeTarget: "http://192.168.1.2:43123"},
		"not https": {
			serveStatus: `{"TCP":{"443":{"HTTP":true}},"Web":{"studio.example.ts.net:443":{"Handlers":{"/xiadown":{"Proxy":"http://127.0.0.1:43123"}}}}}`,
			routeExists: true, routeTarget: "http://127.0.0.1:43123", routeBackendPort: 43123,
		},
	} {
		t.Run(name, func(t *testing.T) {
			runner := &stubRunner{outputs: [][]byte{status, []byte(testCase.serveStatus)}}
			info := NewManager(runner).Inspect(context.Background(), 443, "/xiadown")
			if !info.Connected || !info.RouteChecked || info.RouteExists != testCase.routeExists ||
				info.RouteTarget != testCase.routeTarget || info.RouteBackendPort != testCase.routeBackendPort ||
				info.ServeURL != "" || info.LastError != "" {
				t.Fatalf("inactive route presented as active: %+v", info)
			}
		})
	}
}

func TestManagerInspectReportsServeStatusFailureWithoutFabricatingURL(t *testing.T) {
	runner := &stubRunner{
		outputs: [][]byte{[]byte(`{"BackendState":"Running","Self":{"DNSName":"studio.example.ts.net.","Online":true}}`), []byte("permission denied")},
		errors:  []error{nil, errors.New("exit status 1")},
	}
	info := NewManager(runner).Inspect(context.Background(), 443, "/xiadown")
	if info.ServeURL != "" || info.LastError != "permission denied" {
		t.Fatalf("serve inspection failure: %+v", info)
	}
}

func TestExactLoopbackBackendPortRejectsEquivalentButUnownedSpellings(t *testing.T) {
	for value, want := range map[string]int{
		"http://127.0.0.1:43123":   43123,
		"http://localhost:43123":   0,
		"http://127.0.0.1:43123/":  0,
		"http://127.0.0.1:043123":  0,
		"http://127.0.0.1:43123?x": 0,
		"HTTP://127.0.0.1:43123":   0,
	} {
		if got := exactLoopbackBackendPort(value); got != want {
			t.Fatalf("exactLoopbackBackendPort(%q) = %d, want %d", value, got, want)
		}
	}
}

func TestManagerInspectReportsMissingAndStoppedTailscale(t *testing.T) {
	missing := &stubRunner{errors: []error{exec.ErrNotFound}}
	info := NewManager(missing).Inspect(context.Background(), 443, "/xiadown")
	if info.Installed || info.LastError != "" {
		t.Fatalf("missing info: %+v", info)
	}

	stopped := &stubRunner{outputs: [][]byte{[]byte(`{"Version":"1.80","BackendState":"Stopped","Self":{"DNSName":"mac.tail.ts.net."}}`)}}
	info = NewManager(stopped).Inspect(context.Background(), 8443, "/xiadown")
	if !info.Installed || info.Connected || info.ServeURL != "" || info.Version != "1.80" {
		t.Fatalf("stopped info: %+v", info)
	}
}

func TestManagerEnableReportsMissingTailscaleWithoutExecutableLookupDetail(t *testing.T) {
	runner := &stubRunner{errors: []error{exec.ErrNotFound}}
	err := NewManager(runner).Enable(
		context.Background(),
		43123,
		443,
		"/xiadown",
		libraryaccess.TailscaleRouteOwnership{},
	)
	if err == nil || err.Error() != "Tailscale is not installed" {
		t.Fatalf("missing Tailscale enable error = %v", err)
	}
	if strings.Contains(strings.ToLower(err.Error()), "%path%") {
		t.Fatalf("raw executable lookup error leaked: %v", err)
	}
}

func TestManagerEnableAndDisableUseOnlyScopedServeRoute(t *testing.T) {
	runner := &stubRunner{outputs: [][]byte{
		runningStatusJSON(), []byte(`{}`), nil,
		runningStatusJSON(), exactServeStatusJSON(443, "/xiadown", 43123), nil,
	}}
	manager := NewManager(runner)
	if err := manager.Enable(context.Background(), 43123, 443, "/xiadown", libraryaccess.TailscaleRouteOwnership{}); err != nil {
		t.Fatalf("enable: %v", err)
	}
	if err := manager.Disable(context.Background(), 443, "/xiadown", libraryaccess.TailscaleRouteOwnership{BackendPort: 43123}); err != nil {
		t.Fatalf("disable: %v", err)
	}
	want := []runnerCall{
		{executable: "tailscale", args: []string{"status", "--json"}},
		{executable: "tailscale", args: []string{"serve", "status", "--json"}},
		{executable: "tailscale", args: []string{"serve", "--bg", "--https=443", "--set-path=/xiadown", "http://127.0.0.1:43123"}},
		{executable: "tailscale", args: []string{"status", "--json"}},
		{executable: "tailscale", args: []string{"serve", "status", "--json"}},
		{executable: "tailscale", args: []string{"serve", "--bg", "--https=443", "--set-path=/xiadown", "off"}},
	}
	if !reflect.DeepEqual(runner.calls, want) {
		t.Fatalf("calls = %#v, want %#v", runner.calls, want)
	}
	for _, call := range runner.calls {
		for _, arg := range call.args {
			if arg == "funnel" || arg == "reset" || arg == "serve reset" {
				t.Fatalf("unsafe global command in %#v", call)
			}
		}
	}
}

func TestManagerRejectsUnsafeRouteBeforeCommand(t *testing.T) {
	runner := new(stubRunner)
	err := NewManager(runner).Enable(context.Background(), 43123, 443, "/../admin", libraryaccess.TailscaleRouteOwnership{})
	if !errors.Is(err, libraryaccess.ErrInvalidConfig) {
		t.Fatalf("expected invalid config, got %v", err)
	}
	if len(runner.calls) != 0 {
		t.Fatalf("unexpected calls: %#v", runner.calls)
	}
}

func TestManagerSurfacesCommandOutput(t *testing.T) {
	runner := &stubRunner{
		outputs: [][]byte{runningStatusJSON(), exactServeStatusJSON(443, "/xiadown", 43123), []byte("permission denied\n")},
		errors:  []error{nil, nil, errors.New("exit status 1")},
	}
	err := NewManager(runner).Disable(context.Background(), 443, "/xiadown", libraryaccess.TailscaleRouteOwnership{BackendPort: 43123})
	if err == nil || err.Error() != "disable XiaDown Tailscale Serve route: permission denied" {
		t.Fatalf("error = %v", err)
	}
}

func TestManagerTimeoutStopsBeforeServeMutationAndLeadsPartialOutput(t *testing.T) {
	runner := &stubRunner{
		outputs: [][]byte{[]byte("partial status output")},
		errors: []error{fmt.Errorf(
			"Tailscale command timed out after 10s: %w", context.DeadlineExceeded,
		)},
	}
	err := NewManager(runner).Enable(
		context.Background(), 43123, 443, "/xiadown", libraryaccess.TailscaleRouteOwnership{},
	)
	if err == nil || !strings.Contains(err.Error(), "timed out") || !strings.Contains(err.Error(), "partial status output") ||
		strings.Index(err.Error(), "timed out") > strings.Index(err.Error(), "partial status output") {
		t.Fatalf("timeout error = %v", err)
	}
	if len(runner.calls) != 1 || !reflect.DeepEqual(runner.calls[0].args, []string{"status", "--json"}) {
		t.Fatalf("timeout continued into mutation: %#v", runner.calls)
	}
}

func TestManagerRefusesInitiallyOccupiedRouteWithoutMutation(t *testing.T) {
	for name, serveStatus := range map[string][]byte{
		"https proxy": exactServeStatusJSON(443, "/xiadown", 49999),
		"http handler": []byte(`{
  "TCP":{"443":{"HTTP":true}},
  "Web":{"studio.example.ts.net:443":{"Handlers":{"/xiadown":{"Proxy":"http://127.0.0.1:49999"}}}}
}`),
		"non proxy handler": []byte(`{
  "TCP":{"443":{"HTTPS":true}},
  "Web":{"studio.example.ts.net:443":{"Handlers":{"/xiadown":{"Path":"/tmp/user-files"}}}}
}`),
	} {
		t.Run(name, func(t *testing.T) {
			runner := &stubRunner{outputs: [][]byte{runningStatusJSON(), serveStatus}}
			err := NewManager(runner).Enable(
				context.Background(), 43123, 443, "/xiadown", libraryaccess.TailscaleRouteOwnership{},
			)
			if !errors.Is(err, libraryaccess.ErrTailscaleRouteOwnershipConflict) {
				t.Fatalf("expected ownership conflict, got %v", err)
			}
			if len(runner.calls) != 2 {
				t.Fatalf("occupied route was mutated: %#v", runner.calls)
			}
		})
	}
}

func TestManagerReplacesOnlyExactPersistedOwnedTarget(t *testing.T) {
	for name, ownership := range map[string]libraryaccess.TailscaleRouteOwnership{
		"active backend":  {BackendPort: 43123},
		"pending backend": {BackendPort: 43000, PendingBackendPort: 43123},
	} {
		t.Run(name, func(t *testing.T) {
			runner := &stubRunner{outputs: [][]byte{
				runningStatusJSON(), exactServeStatusJSON(443, "/xiadown", 43123), nil,
			}}
			err := NewManager(runner).Enable(
				context.Background(), 43124, 443, "/xiadown", ownership,
			)
			if err != nil {
				t.Fatalf("replace owned route: %v", err)
			}
			if len(runner.calls) != 3 || !reflect.DeepEqual(runner.calls[2].args,
				[]string{"serve", "--bg", "--https=443", "--set-path=/xiadown", "http://127.0.0.1:43124"}) {
				t.Fatalf("calls = %#v", runner.calls)
			}
		})
	}
}

func TestManagerRefusesExternallyRewrittenTargetWithoutMutation(t *testing.T) {
	runner := &stubRunner{outputs: [][]byte{
		runningStatusJSON(), exactServeStatusJSON(443, "/xiadown", 49999),
	}}
	err := NewManager(runner).Disable(
		context.Background(), 443, "/xiadown",
		libraryaccess.TailscaleRouteOwnership{BackendPort: 43123},
	)
	if !errors.Is(err, libraryaccess.ErrTailscaleRouteOwnershipConflict) {
		t.Fatalf("expected ownership conflict, got %v", err)
	}
	if len(runner.calls) != 2 {
		t.Fatalf("rewritten route was mutated: %#v", runner.calls)
	}
}

func TestManagerDisableMissingOwnedRouteIsSafeAndScoped(t *testing.T) {
	runner := &stubRunner{outputs: [][]byte{runningStatusJSON(), []byte(`{}`)}}
	err := NewManager(runner).Disable(
		context.Background(), 443, "/xiadown",
		libraryaccess.TailscaleRouteOwnership{BackendPort: 43123},
	)
	if err != nil {
		t.Fatalf("disable missing route: %v", err)
	}
	if len(runner.calls) != 2 {
		t.Fatalf("missing route emitted mutation: %#v", runner.calls)
	}
}

func runningStatusJSON() []byte {
	return []byte(`{
  "Version":"1.98.8", "BackendState":"Running",
  "Self":{"DNSName":"studio.example.ts.net.","HostName":"studio","Online":true}
}`)
}

func exactServeStatusJSON(httpsPort int, routePath string, backendPort int) []byte {
	return []byte(`{
  "TCP":{"` + strconv.Itoa(httpsPort) + `":{"HTTPS":true}},
  "Web":{"studio.example.ts.net:` + strconv.Itoa(httpsPort) + `":{"Handlers":{"` + routePath + `":{"Proxy":"http://127.0.0.1:` + strconv.Itoa(backendPort) + `"}}}}
}`)
}
