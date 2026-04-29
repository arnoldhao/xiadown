package browsercdp

import (
	"context"
	"errors"
	"strings"
	"time"

	"github.com/chromedp/cdproto/cdp"
	targetpkg "github.com/chromedp/cdproto/target"
	"github.com/chromedp/chromedp"
)

// AttachOrCreatePageTarget returns a tab context bound to an existing reusable
// page target when possible. If none is available, it creates one explicitly.
func AttachOrCreatePageTarget(runtime *Runtime, waitTimeout time.Duration) (context.Context, context.CancelFunc, string, error) {
	if runtime == nil {
		return nil, nil, "", errors.New("browser runtime unavailable")
	}

	targetID, err := waitForReusablePageTarget(runtime, waitTimeout)
	if err != nil {
		return nil, nil, "", err
	}
	if strings.TrimSpace(targetID) == "" {
		targetID, err = createPageTarget(runtime, 10*time.Second, !runtime.Status().Headless)
		if err != nil {
			return nil, nil, "", err
		}
	}

	tabCtx, cancel := chromedp.NewContext(runtime.BrowserContext(), chromedp.WithTargetID(targetpkg.ID(targetID)))
	if err := chromedp.Run(tabCtx); err != nil {
		cancel()
		return nil, nil, "", wrapRuntimeHangError(err)
	}
	return tabCtx, cancel, targetID, nil
}

func waitForReusablePageTarget(runtime *Runtime, timeout time.Duration) (string, error) {
	if runtime == nil {
		return "", errors.New("browser runtime unavailable")
	}
	if timeout <= 0 {
		timeout = 5 * time.Second
	}

	deadline := time.Now().Add(timeout)
	for {
		targets, err := chromedp.Targets(runtime.BrowserContext())
		if err != nil {
			return "", err
		}
		if targetID := strings.TrimSpace(pickReusableTargetID(targets)); targetID != "" {
			return targetID, nil
		}
		if time.Now().After(deadline) {
			return "", nil
		}
		time.Sleep(100 * time.Millisecond)
	}
}

func createPageTarget(runtime *Runtime, timeout time.Duration, newWindow bool) (string, error) {
	runCtx, cancel, err := RuntimeBrowserExecutorContext(runtime, timeout)
	if err != nil {
		return "", err
	}
	defer cancel()

	createTarget := targetpkg.CreateTarget("about:blank")
	if newWindow {
		createTarget = createTarget.WithNewWindow(true)
	}
	createdTargetID, err := createTarget.Do(runCtx)
	if err != nil {
		return "", wrapRuntimeHangError(err)
	}

	targetID := strings.TrimSpace(string(createdTargetID))
	if targetID == "" {
		return "", errors.New("create target returned empty target id")
	}
	return targetID, nil
}

// RuntimeBrowserExecutorContext returns a CDP browser-level executor context.
// Use this for browser-scoped commands so chromedp does not create a page target.
func RuntimeBrowserExecutorContext(runtime *Runtime, timeout time.Duration) (context.Context, context.CancelFunc, error) {
	if runtime == nil {
		return nil, nil, errors.New("browser runtime unavailable")
	}
	baseCtx := runtime.BrowserContext()
	if baseCtx == nil {
		return nil, nil, errors.New("browser context unavailable")
	}
	chromeCtx := chromedp.FromContext(baseCtx)
	if chromeCtx == nil || chromeCtx.Browser == nil {
		return nil, nil, errors.New("browser executor unavailable")
	}
	runCtx, cancel := context.WithTimeout(baseCtx, timeout)
	return cdp.WithExecutor(runCtx, chromeCtx.Browser), cancel, nil
}
