package browsercdp

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
	"sync"
	"time"
)

type BrowserID string

const (
	BrowserChrome   BrowserID = "chrome"
	BrowserChromium BrowserID = "chromium"
	BrowserEdge     BrowserID = "edge"
	BrowserBrave    BrowserID = "brave"
	BrowserVivaldi  BrowserID = "vivaldi"
	BrowserOpera    BrowserID = "opera"
	BrowserOperaGX  BrowserID = "opera-gx"
	BrowserArc      BrowserID = "arc"
	BrowserYandex   BrowserID = "yandex"
	BrowserHelium   BrowserID = "helium"
)

type Candidate struct {
	ID        BrowserID `json:"id"`
	Label     string    `json:"label"`
	ExecPath  string    `json:"execPath,omitempty"`
	Available bool      `json:"available"`
	Error     string    `json:"error,omitempty"`
}

var (
	detectCandidatesCacheMu        sync.RWMutex
	detectCandidatesCache          []Candidate
	detectCandidatesCacheExpiresAt time.Time
	detectCandidatesCacheLoaded    bool
	detectCandidatesCacheTTL       = 5 * time.Second
	detectCandidatesNow            = time.Now
	detectCandidatesScan           = scanCandidates
)

func DetectCandidates() []Candidate {
	now := detectCandidatesNow()
	detectCandidatesCacheMu.RLock()
	if detectCandidatesCacheLoaded && now.Before(detectCandidatesCacheExpiresAt) {
		cached := cloneCandidates(detectCandidatesCache)
		detectCandidatesCacheMu.RUnlock()
		return cached
	}
	detectCandidatesCacheMu.RUnlock()

	detectCandidatesCacheMu.Lock()
	defer detectCandidatesCacheMu.Unlock()

	now = detectCandidatesNow()
	if detectCandidatesCacheLoaded && now.Before(detectCandidatesCacheExpiresAt) {
		return cloneCandidates(detectCandidatesCache)
	}

	return refreshCandidatesLocked(now)
}

func RefreshCandidates() []Candidate {
	detectCandidatesCacheMu.Lock()
	defer detectCandidatesCacheMu.Unlock()
	return refreshCandidatesLocked(detectCandidatesNow())
}

func ChooseCandidate(candidates []Candidate, preferred string) (Candidate, bool) {
	preferredID := BrowserID(strings.ToLower(strings.TrimSpace(preferred)))
	if preferredID != "" {
		for _, candidate := range candidates {
			if candidate.ID == preferredID && candidate.Available {
				return candidate, true
			}
		}
	}
	return ChooseDefaultCandidate(candidates)
}

func ChooseDefaultCandidate(candidates []Candidate) (Candidate, bool) {
	available := make([]Candidate, 0, len(candidates))
	for _, candidate := range candidates {
		if !candidate.Available {
			continue
		}
		if candidate.ID == BrowserChrome {
			return candidate, true
		}
		available = append(available, candidate)
	}
	if len(available) == 0 {
		return Candidate{}, false
	}
	sort.SliceStable(available, func(left, right int) bool {
		leftLabel := strings.ToLower(strings.TrimSpace(available[left].Label))
		rightLabel := strings.ToLower(strings.TrimSpace(available[right].Label))
		if leftLabel != rightLabel {
			return leftLabel < rightLabel
		}
		return available[left].ID < available[right].ID
	})
	return available[0], true
}

func CheckCDPReady(ctx context.Context, host string, port int) error {
	if port <= 0 {
		return fmt.Errorf("invalid cdp port")
	}
	if strings.TrimSpace(host) == "" {
		host = "127.0.0.1"
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, fmt.Sprintf("http://%s:%d/json/version", host, port), nil)
	if err != nil {
		return err
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("unexpected cdp status %d", resp.StatusCode)
	}
	return nil
}

func detectCandidate(id BrowserID) Candidate {
	candidate := Candidate{ID: id, Label: labelForID(id)}
	for _, path := range candidatesForID(id) {
		resolved := resolveExecutable(path)
		if strings.TrimSpace(resolved) == "" {
			continue
		}
		candidate.ExecPath = resolved
		candidate.Available = true
		candidate.Error = ""
		return candidate
	}
	candidate.Error = "browser executable not found"
	return candidate
}

func scanCandidates() []Candidate {
	order := []BrowserID{
		BrowserChrome,
		BrowserArc,
		BrowserBrave,
		BrowserChromium,
		BrowserEdge,
		BrowserOpera,
		BrowserOperaGX,
		BrowserVivaldi,
		BrowserYandex,
		BrowserHelium,
	}
	result := make([]Candidate, 0, len(order))
	for _, id := range order {
		result = append(result, detectCandidate(id))
	}
	return result
}

func cloneCandidates(source []Candidate) []Candidate {
	if len(source) == 0 {
		return []Candidate{}
	}
	result := make([]Candidate, len(source))
	copy(result, source)
	return result
}

func refreshCandidatesLocked(now time.Time) []Candidate {
	result := detectCandidatesScan()
	detectCandidatesCache = cloneCandidates(result)
	detectCandidatesCacheExpiresAt = now.Add(detectCandidatesCacheTTL)
	detectCandidatesCacheLoaded = true
	return cloneCandidates(result)
}

func resetDetectCandidatesCache() {
	detectCandidatesCacheMu.Lock()
	defer detectCandidatesCacheMu.Unlock()
	detectCandidatesCache = nil
	detectCandidatesCacheExpiresAt = time.Time{}
	detectCandidatesCacheLoaded = false
}

func labelForID(id BrowserID) string {
	switch id {
	case BrowserChrome:
		return "Chrome"
	case BrowserChromium:
		return "Chromium"
	case BrowserEdge:
		return "Edge"
	case BrowserBrave:
		return "Brave"
	case BrowserVivaldi:
		return "Vivaldi"
	case BrowserOpera:
		return "Opera"
	case BrowserOperaGX:
		return "Opera GX"
	case BrowserArc:
		return "Arc"
	case BrowserYandex:
		return "Yandex Browser"
	case BrowserHelium:
		return "Helium"
	default:
		return titleASCII(string(id))
	}
}

func titleASCII(value string) string {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		return ""
	}
	return strings.ToUpper(trimmed[:1]) + strings.ToLower(trimmed[1:])
}

func candidatesForID(id BrowserID) []string {
	switch runtime.GOOS {
	case "darwin":
		switch id {
		case BrowserChrome:
			return darwinAppCandidates(
				darwinApp{"Google Chrome", "Google Chrome"},
				darwinApp{"Google Chrome Beta", "Google Chrome Beta"},
				darwinApp{"Google Chrome Dev", "Google Chrome Dev"},
				darwinApp{"Google Chrome Canary", "Google Chrome Canary"},
			)
		case BrowserChromium:
			return darwinAppCandidates(darwinApp{"Chromium", "Chromium"})
		case BrowserEdge:
			return darwinAppCandidates(
				darwinApp{"Microsoft Edge", "Microsoft Edge"},
				darwinApp{"Microsoft Edge Beta", "Microsoft Edge Beta"},
				darwinApp{"Microsoft Edge Dev", "Microsoft Edge Dev"},
				darwinApp{"Microsoft Edge Canary", "Microsoft Edge Canary"},
			)
		case BrowserBrave:
			return darwinAppCandidates(
				darwinApp{"Brave Browser", "Brave Browser"},
				darwinApp{"Brave Browser Beta", "Brave Browser Beta"},
				darwinApp{"Brave Browser Nightly", "Brave Browser Nightly"},
			)
		case BrowserVivaldi:
			return darwinAppCandidates(
				darwinApp{"Vivaldi", "Vivaldi"},
				darwinApp{"Vivaldi Snapshot", "Vivaldi Snapshot"},
			)
		case BrowserOpera:
			return darwinAppCandidates(darwinApp{"Opera", "Opera"})
		case BrowserOperaGX:
			return darwinAppCandidates(
				darwinApp{"Opera GX", "Opera GX"},
				darwinApp{"Opera GX", "Opera"},
			)
		case BrowserArc:
			return darwinAppCandidates(darwinApp{"Arc", "Arc"})
		case BrowserYandex:
			return darwinAppCandidates(
				darwinApp{"Yandex", "Yandex"},
				darwinApp{"Yandex Browser", "Yandex Browser"},
				darwinApp{"Yandex Browser", "Yandex"},
			)
		case BrowserHelium:
			return darwinAppCandidates(darwinApp{"Helium", "Helium"})
		}
	case "windows":
		localAppData := strings.TrimSpace(os.Getenv("LOCALAPPDATA"))
		programFiles := strings.TrimSpace(os.Getenv("ProgramFiles"))
		programFilesX86 := strings.TrimSpace(os.Getenv("ProgramFiles(x86)"))
		switch id {
		case BrowserChrome:
			return compact([]string{
				filepath.Join(programFiles, "Google", "Chrome", "Application", "chrome.exe"),
				filepath.Join(programFilesX86, "Google", "Chrome", "Application", "chrome.exe"),
				filepath.Join(localAppData, "Google", "Chrome", "Application", "chrome.exe"),
				filepath.Join(programFiles, "Google", "Chrome Beta", "Application", "chrome.exe"),
				filepath.Join(programFilesX86, "Google", "Chrome Beta", "Application", "chrome.exe"),
				filepath.Join(localAppData, "Google", "Chrome Beta", "Application", "chrome.exe"),
				filepath.Join(programFiles, "Google", "Chrome Dev", "Application", "chrome.exe"),
				filepath.Join(programFilesX86, "Google", "Chrome Dev", "Application", "chrome.exe"),
				filepath.Join(localAppData, "Google", "Chrome Dev", "Application", "chrome.exe"),
				filepath.Join(localAppData, "Google", "Chrome SxS", "Application", "chrome.exe"),
			})
		case BrowserChromium:
			return compact([]string{
				filepath.Join(programFiles, "Chromium", "Application", "chrome.exe"),
				filepath.Join(programFilesX86, "Chromium", "Application", "chrome.exe"),
				filepath.Join(localAppData, "Chromium", "Application", "chrome.exe"),
			})
		case BrowserEdge:
			return compact([]string{
				filepath.Join(programFiles, "Microsoft", "Edge", "Application", "msedge.exe"),
				filepath.Join(programFilesX86, "Microsoft", "Edge", "Application", "msedge.exe"),
				filepath.Join(localAppData, "Microsoft", "Edge", "Application", "msedge.exe"),
				filepath.Join(programFiles, "Microsoft", "Edge Beta", "Application", "msedge.exe"),
				filepath.Join(programFilesX86, "Microsoft", "Edge Beta", "Application", "msedge.exe"),
				filepath.Join(localAppData, "Microsoft", "Edge Beta", "Application", "msedge.exe"),
				filepath.Join(programFiles, "Microsoft", "Edge Dev", "Application", "msedge.exe"),
				filepath.Join(programFilesX86, "Microsoft", "Edge Dev", "Application", "msedge.exe"),
				filepath.Join(localAppData, "Microsoft", "Edge Dev", "Application", "msedge.exe"),
				filepath.Join(localAppData, "Microsoft", "Edge SxS", "Application", "msedge.exe"),
			})
		case BrowserBrave:
			return compact([]string{
				filepath.Join(programFiles, "BraveSoftware", "Brave-Browser", "Application", "brave.exe"),
				filepath.Join(programFilesX86, "BraveSoftware", "Brave-Browser", "Application", "brave.exe"),
				filepath.Join(localAppData, "BraveSoftware", "Brave-Browser", "Application", "brave.exe"),
				filepath.Join(programFiles, "BraveSoftware", "Brave-Browser-Beta", "Application", "brave.exe"),
				filepath.Join(programFilesX86, "BraveSoftware", "Brave-Browser-Beta", "Application", "brave.exe"),
				filepath.Join(localAppData, "BraveSoftware", "Brave-Browser-Beta", "Application", "brave.exe"),
				filepath.Join(programFiles, "BraveSoftware", "Brave-Browser-Nightly", "Application", "brave.exe"),
				filepath.Join(programFilesX86, "BraveSoftware", "Brave-Browser-Nightly", "Application", "brave.exe"),
				filepath.Join(localAppData, "BraveSoftware", "Brave-Browser-Nightly", "Application", "brave.exe"),
			})
		case BrowserVivaldi:
			return compact([]string{
				filepath.Join(programFiles, "Vivaldi", "Application", "vivaldi.exe"),
				filepath.Join(programFilesX86, "Vivaldi", "Application", "vivaldi.exe"),
				filepath.Join(localAppData, "Vivaldi", "Application", "vivaldi.exe"),
				filepath.Join(programFiles, "Vivaldi Snapshot", "Application", "vivaldi.exe"),
				filepath.Join(programFilesX86, "Vivaldi Snapshot", "Application", "vivaldi.exe"),
				filepath.Join(localAppData, "Vivaldi Snapshot", "Application", "vivaldi.exe"),
			})
		case BrowserOpera:
			return compact([]string{
				filepath.Join(localAppData, "Programs", "Opera", "opera.exe"),
				filepath.Join(programFiles, "Opera", "opera.exe"),
				filepath.Join(programFilesX86, "Opera", "opera.exe"),
			})
		case BrowserOperaGX:
			return compact([]string{
				filepath.Join(localAppData, "Programs", "Opera GX", "opera.exe"),
				filepath.Join(programFiles, "Opera GX", "opera.exe"),
				filepath.Join(programFilesX86, "Opera GX", "opera.exe"),
			})
		case BrowserArc:
			return compact([]string{
				"Arc.exe",
				filepath.Join(localAppData, "Microsoft", "WindowsApps", "Arc.exe"),
				filepath.Join(localAppData, "Programs", "Arc", "Arc.exe"),
				filepath.Join(programFiles, "Arc", "Arc.exe"),
			})
		case BrowserYandex:
			return compact([]string{
				filepath.Join(localAppData, "Yandex", "YandexBrowser", "Application", "browser.exe"),
				filepath.Join(programFiles, "Yandex", "YandexBrowser", "Application", "browser.exe"),
				filepath.Join(programFilesX86, "Yandex", "YandexBrowser", "Application", "browser.exe"),
			})
		case BrowserHelium:
			return compact([]string{
				"helium.exe",
				"Helium.exe",
				filepath.Join(localAppData, "Helium", "Application", "helium.exe"),
				filepath.Join(localAppData, "Helium", "Application", "Helium.exe"),
				filepath.Join(localAppData, "Helium", "Application", "chrome.exe"),
				filepath.Join(localAppData, "Programs", "Helium", "helium.exe"),
				filepath.Join(localAppData, "Programs", "Helium", "Helium.exe"),
				filepath.Join(localAppData, "Programs", "Helium", "Application", "chrome.exe"),
				filepath.Join(programFiles, "Helium", "Application", "helium.exe"),
				filepath.Join(programFiles, "Helium", "Application", "Helium.exe"),
				filepath.Join(programFiles, "Helium", "Application", "chrome.exe"),
				filepath.Join(programFilesX86, "Helium", "Application", "helium.exe"),
				filepath.Join(programFilesX86, "Helium", "Application", "Helium.exe"),
				filepath.Join(programFilesX86, "Helium", "Application", "chrome.exe"),
			})
		}
	default:
		switch id {
		case BrowserChrome:
			return []string{"google-chrome", "google-chrome-stable"}
		case BrowserChromium:
			return []string{"chromium-browser", "chromium"}
		case BrowserEdge:
			return []string{"microsoft-edge", "microsoft-edge-stable", "msedge"}
		case BrowserBrave:
			return []string{"brave-browser", "brave-browser-stable", "brave-browser-beta", "brave-browser-nightly", "brave"}
		case BrowserVivaldi:
			return []string{"vivaldi", "vivaldi-stable", "vivaldi-snapshot"}
		case BrowserOpera:
			return []string{"opera", "opera-beta", "opera-developer"}
		case BrowserOperaGX:
			return []string{"opera-gx"}
		case BrowserYandex:
			return []string{"yandex-browser", "yandex-browser-stable", "yandex-browser-beta"}
		case BrowserHelium:
			return []string{"helium", "helium-browser"}
		}
	}
	return nil
}

type darwinApp struct {
	appName  string
	execName string
}

func darwinAppCandidates(apps ...darwinApp) []string {
	roots := []string{"/Applications"}
	if home, err := os.UserHomeDir(); err == nil {
		home = strings.TrimSpace(home)
		if home != "" {
			roots = append(roots, filepath.Join(home, "Applications"))
		}
	}
	result := make([]string, 0, len(apps)*len(roots))
	for _, app := range apps {
		appName := strings.TrimSpace(app.appName)
		execName := strings.TrimSpace(app.execName)
		if appName == "" || execName == "" {
			continue
		}
		if !strings.HasSuffix(appName, ".app") {
			appName += ".app"
		}
		for _, root := range roots {
			result = append(result, filepath.Join(root, appName, "Contents", "MacOS", execName))
		}
	}
	return result
}

func resolveExecutable(candidate string) string {
	trimmed := strings.TrimSpace(candidate)
	if trimmed == "" {
		return ""
	}
	if filepath.IsAbs(trimmed) {
		if fileInfo, err := os.Stat(trimmed); err == nil && !fileInfo.IsDir() {
			return trimmed
		}
		return ""
	}
	resolved, err := exec.LookPath(trimmed)
	if err != nil {
		return ""
	}
	return resolved
}

func compact(values []string) []string {
	result := make([]string, 0, len(values))
	for _, value := range values {
		trimmed := strings.TrimSpace(value)
		if trimmed != "" {
			result = append(result, trimmed)
		}
	}
	return result
}

func WaitForCDP(ctx context.Context, host string, port int, timeout time.Duration) error {
	if ctx == nil {
		ctx = context.Background()
	}
	if timeout <= 0 {
		timeout = 8 * time.Second
	}
	deadline := time.Now().Add(timeout)
	for {
		if err := ctx.Err(); err != nil {
			return err
		}
		checkCtx, cancel := context.WithTimeout(ctx, 1200*time.Millisecond)
		err := CheckCDPReady(checkCtx, host, port)
		cancel()
		if err == nil {
			return nil
		}
		if time.Now().After(deadline) {
			if ctxErr := ctx.Err(); ctxErr != nil {
				return ctxErr
			}
			return err
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(250 * time.Millisecond):
		}
	}
}
