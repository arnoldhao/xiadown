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

	attachTimeout := waitTimeout
	if attachTimeout <= 0 {
		attachTimeout = 5 * time.Second
	}
	return attachPageTarget(runtime, targetID, attachTimeout)
}

func attachPageTarget(runtime *Runtime, targetID string, attachTimeout time.Duration) (context.Context, context.CancelFunc, string, error) {
	if runtime == nil {
		return nil, nil, "", errors.New("browser runtime unavailable")
	}
	targetID = strings.TrimSpace(targetID)
	if targetID == "" {
		return nil, nil, "", errors.New("page target unavailable")
	}
	tabCtx, cancel := chromedp.NewContext(runtime.BrowserContext(), chromedp.WithTargetID(targetpkg.ID(targetID)))
	if err := runPageTargetAttach(tabCtx, cancel, attachTimeout); err != nil {
		cancel()
		return nil, nil, "", wrapRuntimeHangError(err)
	}
	return tabCtx, cancel, targetID, nil
}

func runPageTargetAttach(tabCtx context.Context, cancel context.CancelFunc, timeout time.Duration) error {
	if timeout <= 0 {
		timeout = 5 * time.Second
	}
	errCh := make(chan error, 1)
	go func() {
		errCh <- chromedp.Run(tabCtx)
	}()

	timer := time.NewTimer(timeout)
	defer timer.Stop()
	select {
	case err := <-errCh:
		return err
	case <-timer.C:
		cancel()
		return context.DeadlineExceeded
	case <-tabCtx.Done():
		return tabCtx.Err()
	}
}

func waitForReusablePageTarget(runtime *Runtime, timeout time.Duration) (string, error) {
	if runtime == nil {
		return "", errors.New("browser runtime unavailable")
	}
	if timeout <= 0 {
		timeout = 5 * time.Second
	}
	if manager := runtime.TargetManager(); manager != nil {
		infos := manager.ListPageTargets()
		if targetID := strings.TrimSpace(pickReusableTargetID(infos)); targetID != "" {
			return targetID, nil
		}
		if hasPageTargetInfo(infos) {
			return "", nil
		}
		waitCtx, cancel := context.WithTimeout(runtime.BrowserContext(), timeout)
		defer cancel()
		info, err := manager.WaitPageTarget(waitCtx, func(info *targetpkg.Info) bool {
			return isPageTargetInfo(info)
		})
		if err != nil {
			if errors.Is(err, context.DeadlineExceeded) || errors.Is(err, context.Canceled) {
				return "", nil
			}
			return "", err
		}
		if info == nil {
			return "", nil
		}
		if !isReusablePageTargetInfo(info) {
			return "", nil
		}
		return strings.TrimSpace(string(info.TargetID)), nil
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
		if hasPageTargetInfo(targets) {
			return "", nil
		}
		if time.Now().After(deadline) {
			return "", nil
		}
		time.Sleep(100 * time.Millisecond)
	}
}

func createPageTarget(runtime *Runtime, timeout time.Duration, newWindow bool) (string, error) {
	return createPageTargetForURL(runtime, "about:blank", timeout, newWindow)
}

func createPageTargetForURL(runtime *Runtime, rawURL string, timeout time.Duration, newWindow bool) (string, error) {
	runCtx, cancel, err := RuntimeBrowserExecutorContext(runtime, timeout)
	if err != nil {
		return "", err
	}
	defer cancel()

	rawURL = strings.TrimSpace(rawURL)
	if rawURL == "" {
		rawURL = "about:blank"
	}
	createTarget := targetpkg.CreateTarget(rawURL)
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
	if manager := runtime.TargetManager(); manager != nil {
		manager.RememberPageTargetID(targetID)
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
