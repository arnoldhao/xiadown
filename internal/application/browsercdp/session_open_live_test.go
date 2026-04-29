package browsercdp

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"

	targetpkg "github.com/chromedp/cdproto/target"
	"github.com/chromedp/chromedp"
	"go.uber.org/zap"
)

func TestSessionOpenLive(t *testing.T) {
	if os.Getenv("XIADOWN_BROWSER_OPEN_LIVE") != "1" {
		t.Skip("set XIADOWN_BROWSER_OPEN_LIVE=1 to run the live browser open probe")
	}

	logger, err := zap.NewDevelopment()
	if err != nil {
		t.Fatalf("create logger: %v", err)
	}
	defer func() {
		_ = logger.Sync()
	}()
	restore := zap.ReplaceGlobals(logger)
	defer restore()

	status := ResolveStatus("", true)
	if !status.Ready {
		t.Skipf("browser not available: %s", strings.TrimSpace(status.DetectError))
	}

	targetURL := strings.TrimSpace(os.Getenv("XIADOWN_BROWSER_OPEN_URL"))
	if targetURL == "" {
		targetURL = "https://example.com"
	}

	registry := NewSessionRegistry()
	session := registry.GetOrCreate("live-open", "xiadown", SessionOptions{
		SessionKey:       "live-open",
		ProfileName:      "xiadown",
		PreferredBrowser: strings.TrimSpace(status.ChosenBrowser),
		Headless:         true,
	})
	defer session.stop()

	ctx, cancel := context.WithTimeout(context.Background(), 45*time.Second)
	defer cancel()

	result, err := session.Open(ctx, targetURL, CommandOptions{
		Timeout: 30 * time.Second,
		Limit:   50,
	})
	if err != nil {
		t.Fatalf("session open failed: %v", err)
	}

	itemCount := len(result.Items)
	if result.State != nil && result.State.ItemCount > 0 {
		itemCount = result.State.ItemCount
	}
	t.Logf(
		"open result ok=%v targetId=%s url=%s hasTitle=%v stateAvailable=%v stateVersion=%d itemCount=%d stateError=%s",
		result.OK,
		result.TargetID,
		sanitizeLogURL(result.URL),
		strings.TrimSpace(result.Title) != "",
		result.State != nil || result.StateAvailable,
		result.StateVersion,
		itemCount,
		result.StateError,
	)

	tab, resolveErr := session.resolveTab(result.TargetID, true)
	if resolveErr != nil {
		t.Fatalf("resolve tab: %v", resolveErr)
	}
	probeTargetContext(t, "tab.ctx", tab.ctx)

	browserProbeCtx, browserProbeCancel := chromedp.NewContext(session.runtime.BrowserContext(), chromedp.WithTargetID(targetpkg.ID(result.TargetID)))
	defer browserProbeCancel()
	probeTargetContext(t, "fresh-browserCtx#1", browserProbeCtx)
	probeTargetContext(t, "fresh-browserCtx#2", browserProbeCtx)

	allocProbeCtx, allocProbeCancel := chromedp.NewContext(session.runtime.allocCtx, chromedp.WithTargetID(targetpkg.ID(result.TargetID)))
	defer allocProbeCancel()
	probeTargetContext(t, "fresh-allocCtx#1", allocProbeCtx)
	probeTargetContext(t, "fresh-allocCtx#2", allocProbeCtx)
}

func TestSessionWorkflowLive(t *testing.T) {
	if os.Getenv("XIADOWN_BROWSER_WORKFLOW_LIVE") != "1" {
		t.Skip("set XIADOWN_BROWSER_WORKFLOW_LIVE=1 to run the live browser workflow probe")
	}

	logger, err := zap.NewDevelopment()
	if err != nil {
		t.Fatalf("create logger: %v", err)
	}
	defer func() {
		_ = logger.Sync()
	}()
	restore := zap.ReplaceGlobals(logger)
	defer restore()

	status := ResolveStatus("", true)
	if !status.Ready {
		t.Skipf("browser not available: %s", strings.TrimSpace(status.DetectError))
	}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		switch r.URL.Path {
		case "/":
			_, _ = fmt.Fprint(w, `<!doctype html>
<html>
  <head><title>Workflow Home</title></head>
  <body>
    <form action="/search" method="get">
      <label for="q">Query</label>
      <input id="q" name="q" placeholder="Search weather" />
      <button type="submit">Search</button>
    </form>
    <a href="/next">Next Page</a>
  </body>
</html>`)
		case "/search":
			query := r.URL.Query().Get("q")
			_, _ = fmt.Fprintf(w, `<!doctype html>
<html>
  <head><title>Results %s</title></head>
  <body>
    <p id="result">Result for %s</p>
    <a href="/next">Next Page</a>
  </body>
</html>`, query, query)
		case "/next":
			_, _ = fmt.Fprint(w, `<!doctype html>
<html>
  <head><title>Next Page</title></head>
  <body>
    <button type="button">Done</button>
  </body>
</html>`)
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	registry := NewSessionRegistry()
	session := registry.GetOrCreate("live-workflow", "xiadown", SessionOptions{
		SessionKey:       "live-workflow",
		ProfileName:      "xiadown",
		PreferredBrowser: strings.TrimSpace(status.ChosenBrowser),
		Headless:         true,
		SSRFRules: SSRFPolicy{
			DangerouslyAllowPrivateNetwork: true,
		},
	})
	defer session.stop()

	ctx, cancel := context.WithTimeout(context.Background(), 45*time.Second)
	defer cancel()

	openResult, err := session.Open(ctx, server.URL, CommandOptions{
		Timeout: 20 * time.Second,
		Limit:   50,
	})
	if err != nil {
		t.Fatalf("open failed: %v", err)
	}
	stateResult, stateErr := session.State(openResult.TargetID, 50)
	if stateErr != nil {
		t.Logf("post-open state failed: %v", stateErr)
	} else {
		t.Logf("post-open state url=%s hasTitle=%v items=%d", sanitizeLogURL(stateResult.URL), strings.TrimSpace(stateResult.Title) != "", len(stateResult.Items))
	}
	tab, resolveErr := session.resolveTab(openResult.TargetID, true)
	if resolveErr != nil {
		t.Fatalf("resolve workflow tab: %v", resolveErr)
	}
	probeTargetContext(t, "workflow-tab.ctx", tab.ctx)
	browserProbeCtx, browserProbeCancel := chromedp.NewContext(session.runtime.BrowserContext(), chromedp.WithTargetID(targetpkg.ID(openResult.TargetID)))
	defer browserProbeCancel()
	probeTargetContext(t, "workflow-fresh-browserCtx", browserProbeCtx)

	currentItems := openResult.Items
	if stateErr == nil && len(stateResult.Items) > 0 {
		currentItems = stateResult.Items
	}
	inputRef := findSnapshotRef(currentItems, "textbox", "search weather")
	if inputRef == "" {
		t.Fatalf("textbox ref not found in latest snapshot: %+v", currentItems)
	}
	buttonRef := findSnapshotRef(currentItems, "button", "search")
	if buttonRef == "" {
		t.Fatalf("button ref not found in latest snapshot: %+v", currentItems)
	}

	typeResult, err := session.Act(ctx, ActRequest{
		Kind:     "type",
		TargetID: openResult.TargetID,
		Ref:      inputRef,
		Text:     "weather",
		Limit:    50,
		Timeout:  15 * time.Second,
	})
	if err != nil {
		t.Fatalf("type failed: %v", err)
	}
	nextItems := typeResult.Items
	if len(nextItems) == 0 {
		nextItems = currentItems
	}
	buttonRef = findSnapshotRef(nextItems, "button", "search")
	if buttonRef == "" {
		t.Fatalf("button ref not found after type: %+v", nextItems)
	}

	searchURL := server.URL + "/search?q=weather"
	clickResult, err := session.Act(ctx, ActRequest{
		Kind:     "click",
		TargetID: openResult.TargetID,
		Ref:      buttonRef,
		WaitFor: &WaitRequest{
			URL:     searchURL,
			Timeout: 8 * time.Second,
		},
		Limit:   50,
		Timeout: 20 * time.Second,
	})
	if err != nil {
		t.Fatalf("click failed: %v", err)
	}
	if !strings.Contains(clickResult.URL, "/search?q=weather") {
		t.Fatalf("unexpected search url: %s", sanitizeLogURL(clickResult.URL))
	}
	if !strings.Contains(clickResult.Title, "Results weather") {
		t.Fatalf("unexpected search title")
	}

	navigateResult, err := session.Navigate(ctx, clickResult.TargetID, server.URL+"/next", false, CommandOptions{
		Timeout: 20 * time.Second,
		Limit:   50,
	})
	if err != nil {
		t.Fatalf("navigate failed: %v", err)
	}
	if !strings.Contains(navigateResult.URL, "/next") {
		t.Fatalf("unexpected navigate url: %s", sanitizeLogURL(navigateResult.URL))
	}
	if strings.TrimSpace(navigateResult.Title) != "Next Page" {
		t.Fatalf("unexpected navigate title")
	}

	t.Logf("workflow open/type/click/navigate succeeded targetId=%s finalURL=%s hasTitle=%v", navigateResult.TargetID, sanitizeLogURL(navigateResult.URL), strings.TrimSpace(navigateResult.Title) != "")
}

func TestSessionBaiduSearchLive(t *testing.T) {
	if os.Getenv("XIADOWN_BROWSER_BAIDU_LIVE") != "1" {
		t.Skip("set XIADOWN_BROWSER_BAIDU_LIVE=1 to run the live Baidu browser probe")
	}

	logger, err := zap.NewDevelopment()
	if err != nil {
		t.Fatalf("create logger: %v", err)
	}
	defer func() {
		_ = logger.Sync()
	}()
	restore := zap.ReplaceGlobals(logger)
	defer restore()

	status := ResolveStatus("", false)
	if !status.Ready {
		t.Skipf("browser not available: %s", strings.TrimSpace(status.DetectError))
	}

	headless := strings.TrimSpace(os.Getenv("XIADOWN_BROWSER_BAIDU_HEADLESS")) == "1"
	query := strings.TrimSpace(os.Getenv("XIADOWN_BROWSER_BAIDU_QUERY"))
	if query == "" {
		query = "天气"
	}

	registry := NewSessionRegistry()
	session := registry.GetOrCreate("live-baidu", "xiadown", SessionOptions{
		SessionKey:       "live-baidu",
		ProfileName:      "xiadown",
		PreferredBrowser: strings.TrimSpace(status.ChosenBrowser),
		Headless:         headless,
	})
	defer session.stop()

	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	openResult, err := session.Open(ctx, "https://www.baidu.com", CommandOptions{
		Timeout: 30 * time.Second,
		Limit:   50,
	})
	if err != nil {
		t.Fatalf("open failed: %v", err)
	}

	inputRef := findSnapshotRef(openResult.Items, "textbox", "")
	if inputRef == "" {
		t.Fatalf("textbox ref not found in open items: %+v", openResult.Items)
	}

	typeResult, err := session.Act(ctx, ActRequest{
		Kind:     "type",
		TargetID: openResult.TargetID,
		Ref:      inputRef,
		Text:     query,
		Limit:    50,
		Timeout:  15 * time.Second,
	})
	if err != nil {
		t.Fatalf("type failed: %v", err)
	}

	buttonItems := typeResult.Items
	if len(buttonItems) == 0 {
		buttonItems = openResult.Items
	}
	buttonRef := findSnapshotRef(buttonItems, "button", "百度一下")
	if buttonRef == "" {
		buttonRef = findSnapshotRef(buttonItems, "button", "百度")
	}
	if buttonRef == "" {
		t.Fatalf("search button ref not found after type: %+v", buttonItems)
	}

	clickResult, err := session.Act(ctx, ActRequest{
		Kind:     "click",
		TargetID: openResult.TargetID,
		Ref:      buttonRef,
		Limit:    50,
		Timeout:  20 * time.Second,
	})
	if err != nil {
		t.Fatalf("click failed: %v", err)
	}

	t.Logf(
		"baidu click result targetId=%s url=%s hasTitle=%v stateAvailable=%v itemCount=%d stateError=%s",
		clickResult.TargetID,
		sanitizeLogURL(clickResult.URL),
		strings.TrimSpace(clickResult.Title) != "",
		clickResult.State != nil || clickResult.StateAvailable,
		len(clickResult.Items),
		clickResult.StateError,
	)
	if !strings.Contains(clickResult.URL, "baidu.com") {
		t.Fatalf("unexpected click url: %s", sanitizeLogURL(clickResult.URL))
	}
	if !clickResult.StateAvailable && clickResult.State == nil {
		t.Fatalf("click state unavailable: %s", clickResult.StateError)
	}
}

func probeTargetContext(t *testing.T, label string, ctx context.Context) {
	t.Helper()

	probeCtx, cancel := context.WithTimeout(ctx, 3*time.Second)
	defer cancel()

	var payload struct {
		URL        string `json:"url"`
		Title      string `json:"title"`
		ReadyState string `json:"readyState"`
	}
	err := chromedp.Run(probeCtx, chromedp.EvaluateAsDevTools(`({
		url: document.location.toString(),
		title: document.title,
		readyState: document.readyState,
	})`, &payload))
	if err != nil {
		t.Logf("%s probe failed: %v", label, err)
		return
	}
	t.Logf(
		"%s probe url=%s hasTitle=%v readyState=%s",
		label,
		sanitizeLogURL(payload.URL),
		strings.TrimSpace(payload.Title) != "",
		strings.TrimSpace(payload.ReadyState),
	)
}

func findSnapshotRef(items []SnapshotItem, role string, needle string) string {
	role = strings.TrimSpace(strings.ToLower(role))
	needle = strings.TrimSpace(strings.ToLower(needle))
	for _, item := range items {
		if role != "" && strings.TrimSpace(strings.ToLower(item.Role)) != role {
			continue
		}
		name := strings.ToLower(strings.TrimSpace(item.Name))
		text := strings.ToLower(strings.TrimSpace(item.Text))
		if needle == "" || strings.Contains(name, needle) || strings.Contains(text, needle) {
			return strings.TrimSpace(item.Ref)
		}
	}
	return ""
}
