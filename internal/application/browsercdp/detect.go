package browsercdp

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
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

	result := detectCandidatesScan()
	detectCandidatesCache = cloneCandidates(result)
	detectCandidatesCacheExpiresAt = now.Add(detectCandidatesCacheTTL)
	detectCandidatesCacheLoaded = true
	return cloneCandidates(result)
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
	for _, candidate := range candidates {
		if candidate.Available {
			return candidate, true
		}
	}
	return Candidate{}, false
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
	order := []BrowserID{BrowserChrome, BrowserChromium, BrowserEdge, BrowserBrave}
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
			return []string{
				"/Applications/Google Chrome.app/Contents/MacOS/Google Chrome",
				"/Applications/Google Chrome Beta.app/Contents/MacOS/Google Chrome Beta",
				"/Applications/Google Chrome Dev.app/Contents/MacOS/Google Chrome Dev",
				"/Applications/Google Chrome Canary.app/Contents/MacOS/Google Chrome Canary",
			}
		case BrowserChromium:
			return []string{
				"/Applications/Chromium.app/Contents/MacOS/Chromium",
			}
		case BrowserEdge:
			return []string{
				"/Applications/Microsoft Edge.app/Contents/MacOS/Microsoft Edge",
				"/Applications/Microsoft Edge Beta.app/Contents/MacOS/Microsoft Edge Beta",
				"/Applications/Microsoft Edge Dev.app/Contents/MacOS/Microsoft Edge Dev",
			}
		case BrowserBrave:
			return []string{
				"/Applications/Brave Browser.app/Contents/MacOS/Brave Browser",
				"/Applications/Brave Browser Beta.app/Contents/MacOS/Brave Browser Beta",
			}
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
			})
		case BrowserBrave:
			return compact([]string{
				filepath.Join(programFiles, "BraveSoftware", "Brave-Browser", "Application", "brave.exe"),
				filepath.Join(programFilesX86, "BraveSoftware", "Brave-Browser", "Application", "brave.exe"),
				filepath.Join(localAppData, "BraveSoftware", "Brave-Browser", "Application", "brave.exe"),
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
			return []string{"brave-browser", "brave-browser-stable", "brave"}
		}
	}
	return nil
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
