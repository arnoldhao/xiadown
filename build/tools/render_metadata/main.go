package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

var versionPrefixPattern = regexp.MustCompile(`^(\d+)(?:\.(\d+))?(?:\.(\d+))?(?:\.(\d+))?`)

func main() {
	if len(os.Args) < 2 {
		fatal("usage: render_metadata <darwin-plist|windows-assets> [flags]")
	}

	switch os.Args[1] {
	case "darwin-plist":
		runDarwinPlist(os.Args[2:])
	case "windows-assets":
		runWindowsAssets(os.Args[2:])
	default:
		fatal("unknown command: %s", os.Args[1])
	}
}

func runDarwinPlist(args []string) {
	fs := flag.NewFlagSet("darwin-plist", flag.ExitOnError)
	inputPath := fs.String("input", "", "input plist path")
	outputPath := fs.String("output", "", "output plist path")
	version := fs.String("version", "", "display version")
	_ = fs.Parse(args)

	if strings.TrimSpace(*inputPath) == "" || strings.TrimSpace(*outputPath) == "" {
		fatal("darwin-plist requires --input and --output")
	}

	displayVersion := normalizeDisplayVersion(*version)
	_, _, numeric3 := normalizeNumericVersion(displayVersion)
	content := readText(*inputPath)
	content = replacePlistString(content, "CFBundleShortVersionString", displayVersion)
	content = replacePlistString(content, "CFBundleVersion", numeric3)
	writeText(*outputPath, content)
}

func runWindowsAssets(args []string) {
	fs := flag.NewFlagSet("windows-assets", flag.ExitOnError)
	root := fs.String("root", "build/windows", "windows build metadata root")
	version := fs.String("version", "", "display version")
	_ = fs.Parse(args)

	displayVersion := normalizeDisplayVersion(*version)
	numeric4, numeric4WithSuffix, numeric3 := normalizeNumericVersion(displayVersion)
	windowsRoot := filepath.Clean(*root)

	renderWindowsInfoJSON(filepath.Join(windowsRoot, "info.json"), displayVersion, numeric3)
	renderWindowsManifest(filepath.Join(windowsRoot, "wails.exe.manifest"), numeric3)
	renderWindowsNSH(filepath.Join(windowsRoot, "nsis", "wails_tools.nsh"), displayVersion, numeric4)
	renderWindowsMSIX(filepath.Join(windowsRoot, "msix", "app_manifest.xml"), numeric4WithSuffix)
	renderWindowsMSIX(filepath.Join(windowsRoot, "msix", "template.xml"), numeric4WithSuffix)
}

func normalizeDisplayVersion(version string) string {
	value := strings.TrimSpace(version)
	value = strings.TrimPrefix(strings.TrimPrefix(value, "v"), "V")
	if value == "" {
		return "dev"
	}
	return value
}

func normalizeNumericVersion(version string) (numeric4 string, numeric4WithSuffix string, numeric3 string) {
	matches := versionPrefixPattern.FindStringSubmatch(version)
	parts := []string{"0", "0", "0", "0"}
	if len(matches) > 0 {
		for index := 1; index < len(matches) && index <= 4; index++ {
			if trimmed := strings.TrimSpace(matches[index]); trimmed != "" {
				parts[index-1] = trimmed
			}
		}
	}
	numeric3 = strings.Join(parts[:3], ".")
	numeric4 = strings.Join(parts[:3], ".")
	numeric4WithSuffix = strings.Join(parts, ".")
	return numeric4, numeric4WithSuffix, numeric3
}

func replacePlistString(content string, key string, value string) string {
	pattern := regexp.MustCompile(fmt.Sprintf(`(<key>%s</key>\s*<string>)([^<]*)(</string>)`, regexp.QuoteMeta(key)))
	if !pattern.MatchString(content) {
		fatal("failed to update %s", key)
	}
	return pattern.ReplaceAllString(content, fmt.Sprintf("${1}%s${3}", value))
}

func renderWindowsInfoJSON(path string, displayVersion string, numericVersion string) {
	type fixedInfo struct {
		FileVersion string `json:"file_version"`
	}
	type stringInfo struct {
		ProductVersion  string `json:"ProductVersion"`
		CompanyName     string `json:"CompanyName"`
		FileDescription string `json:"FileDescription"`
		LegalCopyright  string `json:"LegalCopyright"`
		ProductName     string `json:"ProductName"`
		Comments        string `json:"Comments"`
	}
	type payload struct {
		Fixed fixedInfo             `json:"fixed"`
		Info  map[string]stringInfo `json:"info"`
	}

	var data payload
	raw := readBytes(path)
	if err := json.Unmarshal(raw, &data); err != nil {
		fatal("parse %s: %v", path, err)
	}
	data.Fixed.FileVersion = numericVersion
	if data.Info == nil {
		data.Info = map[string]stringInfo{}
	}
	info := data.Info["0000"]
	info.ProductVersion = displayVersion
	data.Info["0000"] = info

	encoded, err := json.MarshalIndent(data, "", "\t")
	if err != nil {
		fatal("encode %s: %v", path, err)
	}
	writeBytes(path, append(encoded, '\n'))
}

func renderWindowsManifest(path string, numericVersion string) {
	content := readText(path)
	pattern := regexp.MustCompile(`(<assemblyIdentity type="win32" name="[^"]+" version=")([^"]+)(" processorArchitecture="\*"/>)`)
	updated, changed := replaceFirstSubmatch(content, pattern, fmt.Sprintf("${1}%s${3}", numericVersion))
	if !changed {
		fatal("failed to update manifest version in %s", path)
	}
	writeText(path, updated)
}

func renderWindowsNSH(path string, displayVersion string, numericVersion string) {
	content := readText(path)
	content = replaceDefineBlock(content, "INFO_PRODUCTVERSION", displayVersion)
	if strings.Contains(content, "!ifndef INFO_PRODUCTVERSION_NUMERIC") {
		content = replaceDefineBlock(content, "INFO_PRODUCTVERSION_NUMERIC", numericVersion)
	} else {
		insertAfter := fmt.Sprintf("!define INFO_PRODUCTVERSION %q\n!endif\n", displayVersion)
		replacement := insertAfter +
			"!ifndef INFO_PRODUCTVERSION_NUMERIC\n" +
			fmt.Sprintf("    !define INFO_PRODUCTVERSION_NUMERIC %q\n", numericVersion) +
			"!endif\n"
		content = strings.Replace(content, insertAfter, replacement, 1)
		if !strings.Contains(content, "INFO_PRODUCTVERSION_NUMERIC") {
			fatal("failed to insert INFO_PRODUCTVERSION_NUMERIC into %s", path)
		}
	}
	writeText(path, content)
}

func replaceDefineBlock(content string, key string, value string) string {
	pattern := regexp.MustCompile(fmt.Sprintf(`(?ms)!ifndef %s\s*\n\s*!define %s "([^"]*)"\s*\n!endif`, regexp.QuoteMeta(key), regexp.QuoteMeta(key)))
	if !pattern.MatchString(content) {
		fatal("failed to update %s define", key)
	}
	replacement := fmt.Sprintf("!ifndef %s\n    !define %s %q\n!endif", key, key, value)
	return pattern.ReplaceAllString(content, replacement)
}

func renderWindowsMSIX(path string, numericVersion string) {
	content := readText(path)
	var pattern *regexp.Regexp
	if strings.Contains(content, "<Identity") {
		pattern = regexp.MustCompile(`(<Identity[\s\S]*?\bVersion=")([^"]+)(")`)
	} else {
		pattern = regexp.MustCompile(`(<PackageInformation[\s\S]*?\bVersion=")([^"]+)(")`)
	}
	updated, changed := replaceFirstSubmatch(content, pattern, fmt.Sprintf("${1}%s${3}", numericVersion))
	if !changed {
		fatal("failed to update Version in %s", path)
	}
	writeText(path, updated)
}

func replaceFirstSubmatch(content string, pattern *regexp.Regexp, replacement string) (string, bool) {
	indices := pattern.FindStringSubmatchIndex(content)
	if indices == nil {
		return content, false
	}
	matched := content[indices[0]:indices[1]]
	replaced := pattern.ReplaceAllString(matched, replacement)
	return content[:indices[0]] + replaced + content[indices[1]:], true
}

func readText(path string) string {
	return string(readBytes(path))
}

func readBytes(path string) []byte {
	data, err := os.ReadFile(path)
	if err != nil {
		fatal("read %s: %v", path, err)
	}
	return data
}

func writeText(path string, content string) {
	writeBytes(path, []byte(content))
}

func writeBytes(path string, content []byte) {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		fatal("mkdir %s: %v", filepath.Dir(path), err)
	}
	if err := os.WriteFile(path, content, 0o644); err != nil {
		fatal("write %s: %v", path, err)
	}
}

func fatal(format string, args ...any) {
	fmt.Fprintf(os.Stderr, format+"\n", args...)
	os.Exit(1)
}
