package main

import (
	"os"
	"regexp"
	"strings"
	"testing"
)

func TestWailsTaskfilesPreserveDevelopmentBuildContracts(t *testing.T) {
	t.Parallel()

	frontendTaskfile, err := os.ReadFile("build/Taskfile.yml")
	if err != nil {
		t.Fatal(err)
	}
	frontendSource := string(frontendTaskfile)
	for _, required := range []string{
		"node_modules/.wails-deps.stamp",
		"bun scripts/prepare-wails-deps.mjs",
		"bun scripts/wait-for-dev-server.mjs",
		"exclude: node_modules/**/*",
		"exclude: dist/**/*",
	} {
		if !strings.Contains(frontendSource, required) {
			t.Fatalf("build/Taskfile.yml is missing %q", required)
		}
	}

	devConfig, err := os.ReadFile("build/config.yml")
	if err != nil {
		t.Fatal(err)
	}
	devSource := string(devConfig)
	frontendIndex := strings.Index(devSource, "common:dev:frontend")
	readyIndex := strings.Index(devSource, "common:dev:wait-frontend")
	runIndex := strings.Index(devSource, "task run")
	if frontendIndex < 0 || readyIndex <= frontendIndex || runIndex <= readyIndex {
		t.Fatal("development app must start only after the Vite boot path is ready")
	}

	for _, path := range []string{
		"build/darwin/Taskfile.yml",
		"build/linux/Taskfile.yml",
		"build/windows/Taskfile.yml",
	} {
		contents, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		if !strings.Contains(string(contents), "EXTRA_TAGS") {
			t.Fatalf("%s does not forward Wails EXTRA_TAGS", path)
		}
	}
}

func TestWindowsBuildsUseTheGUISubsystemInDevelopmentAndProduction(t *testing.T) {
	t.Parallel()

	contents, err := os.ReadFile("build/windows/Taskfile.yml")
	if err != nil {
		t.Fatal(err)
	}
	if count := strings.Count(string(contents), "-H windowsgui"); count != 2 {
		t.Fatalf("Windows DEV and production builds must both select the GUI subsystem; found %d linker flags", count)
	}
}

func TestWindowsChildProcessesUseTheNoConsolePolicy(t *testing.T) {
	t.Parallel()

	policy, err := os.ReadFile("internal/infrastructure/processutil/cli_windows.go")
	if err != nil {
		t.Fatal(err)
	}
	for _, required := range []string{"HideWindow = true", "CreationFlags |= createNoWindow"} {
		if !strings.Contains(string(policy), required) {
			t.Fatalf("Windows CLI policy is missing %q", required)
		}
	}

	for _, path := range []string{
		"internal/application/browsercdp/process_group_windows.go",
		"internal/application/dependencies/service/command_windows.go",
		"internal/application/library/service/process_group_windows.go",
		"internal/application/ytdlp/process_group_windows.go",
		"internal/infrastructure/firewall/firewall.go",
		"internal/infrastructure/tailscale/manager.go",
		"internal/infrastructure/update/process_windows.go",
	} {
		contents, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		if !strings.Contains(string(contents), "processutil.ConfigureCLI") {
			t.Fatalf("%s does not apply the Windows no-console child-process policy", path)
		}
	}
}

func TestDarwinDevelopmentRunPrefersStableCodeSigning(t *testing.T) {
	t.Parallel()

	contents, err := os.ReadFile("build/darwin/Taskfile.yml")
	if err != nil {
		t.Fatal(err)
	}
	source := string(contents)
	for _, required := range []string{
		"task: codesign:dev",
		"dev_app_process.sh stop",
		"dev_app_process.sh run",
		"XIADOWN_DEV_CODESIGN_IDENTITY",
		"security find-identity -v -p codesigning",
		"Apple Development:",
		"Developer ID Application:",
		"--entitlements \"{{.ENTITLEMENTS}}\"",
	} {
		if !strings.Contains(source, required) {
			t.Fatalf("build/darwin/Taskfile.yml is missing stable dev signing contract %q", required)
		}
	}
	if strings.Contains(source, "codesign --force --deep --sign - \"{{.BIN_DIR}}/{{.APP_NAME}}.dev.app\"") {
		t.Fatal("darwin dev run still always replaces its TCC identity with an ad-hoc signature")
	}
	developerIDIndex := strings.Index(source, "'/Developer ID Application:/")
	appleDevelopmentIndex := strings.Index(source, "'/Apple Development:/")
	if developerIDIndex < 0 || appleDevelopmentIndex < 0 || developerIDIndex > appleDevelopmentIndex {
		t.Fatal("darwin dev signing must prefer Developer ID Application before Apple Development")
	}

	stopIndex := strings.Index(source, "dev_app_process.sh stop")
	copyAfterStop := -1
	if stopIndex >= 0 {
		copyAfterStop = strings.Index(source[stopIndex:], `cp "{{.BIN_DIR}}/{{.APP_NAME}}"`)
	}
	if stopIndex < 0 || copyAfterStop < 0 {
		t.Fatal("darwin dev run must stop the old Launch Services app before replacing its executable")
	}

	lifecycleScript, err := os.ReadFile("build/darwin/dev_app_process.sh")
	if err != nil {
		t.Fatal(err)
	}
	lifecycleSource := string(lifecycleScript)
	for _, required := range []string{
		`/usr/bin/open -n -W "$bundle_path" &`,
		"trap cleanup EXIT HUP INT TERM",
		"supervisor_pid=",
		`current_supervisor=$(/bin/ps -p "$runner_pid" -o ppid=`,
		`/bin/ps -axww -o pid=,command=`,
		`if ($0 == absolute_target || $0 == input_target)`,
		"kill -TERM $pids",
	} {
		if !strings.Contains(lifecycleSource, required) {
			t.Fatalf("darwin dev app lifecycle script is missing %q", required)
		}
	}
}

func TestDarwinBuildsRequireMacOS14OrLater(t *testing.T) {
	t.Parallel()

	darwinTaskfile, err := os.ReadFile("build/darwin/Taskfile.yml")
	if err != nil {
		t.Fatal(err)
	}
	darwinSource := string(darwinTaskfile)
	for _, required := range []string{
		`printf "14.0"`,
		`-mmacosx-version-min={{.DEPLOYMENT_TARGET}}`,
		`MACOSX_DEPLOYMENT_TARGET: "{{.DEPLOYMENT_TARGET}}"`,
	} {
		if !strings.Contains(darwinSource, required) {
			t.Fatalf("build/darwin/Taskfile.yml is missing macOS 14 deployment contract %q", required)
		}
	}

	commonTaskfile, err := os.ReadFile("build/Taskfile.yml")
	if err != nil {
		t.Fatal(err)
	}
	commonSource := string(commonTaskfile)
	for _, required := range []string{
		"render_metadata darwin-plist --input darwin/Info.plist --output darwin/Info.plist",
		"render_metadata darwin-plist --input darwin/Info.dev.plist --output darwin/Info.dev.plist",
	} {
		if !strings.Contains(commonSource, required) {
			t.Fatalf("build/Taskfile.yml must restore the macOS 14 plist contract after Wails asset refresh: missing %q", required)
		}
	}

	minimumVersion := regexp.MustCompile(`(?s)<key>LSMinimumSystemVersion</key>\s*<string>14\.0\.0</string>`)
	for _, path := range []string{"build/darwin/Info.plist", "build/darwin/Info.dev.plist"} {
		contents, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		if !minimumVersion.Match(contents) {
			t.Fatalf("%s must declare LSMinimumSystemVersion 14.0.0", path)
		}
	}
}
