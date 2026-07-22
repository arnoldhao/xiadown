package browsercdp

import (
	"context"
	"errors"
	"fmt"
	"net"
	"sort"
	"strings"
	"time"

	"github.com/chromedp/cdproto/cdp"
	"github.com/chromedp/cdproto/network"
	"github.com/chromedp/chromedp"

	appcookies "xiadown/internal/application/cookies"
)

var (
	ErrBorrowedCookieRuntimeUnavailable = errors.New("borrowed browser runtime unavailable")
	ErrBorrowedCookieTargetUnavailable  = errors.New("borrowed browser page target unavailable")
	ErrBorrowedCookieAllowlistInvalid   = errors.New("borrowed browser cookie domain allowlist is invalid")
)

const (
	borrowedCookieAttachTimeout = 5 * time.Second
	borrowedCookieReadTimeout   = 8 * time.Second
)

// SnapshotBorrowedCookiesForDomains reads only cookies applicable to the
// normalized domain allowlist from an existing page in the one browser context
// approved by StartBorrowedCurrentBrowser.
//
// It deliberately uses Network.getCookies with explicit HTTPS URLs instead of
// browser-level Storage.getCookies, which would disclose every cookie in the
// user's profile to this process. It never creates or navigates a target. The
// existing page attachment is always released through AttachBorrowedPageTarget's
// detach-only cancel function, so a user-owned tab cannot be closed.
func SnapshotBorrowedCookiesForDomains(
	ctx context.Context,
	runtimeBrowser *Runtime,
	domains []string,
) ([]appcookies.Record, error) {
	if runtimeBrowser == nil || !runtimeBrowser.IsBorrowed() {
		return nil, ErrBorrowedCookieRuntimeUnavailable
	}
	if ctx == nil {
		ctx = context.Background()
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	normalizedDomains, urls, err := normalizeBorrowedCookieAllowlist(domains)
	if err != nil {
		return nil, err
	}
	targetIDs := borrowedCookiePageTargetIDs(runtimeBrowser)
	if len(targetIDs) == 0 {
		return nil, ErrBorrowedCookieTargetUnavailable
	}

	var lastErr error
	for _, targetID := range targetIDs {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		attachTimeout := boundedBorrowedCookieTimeout(ctx, borrowedCookieAttachTimeout)
		if attachTimeout <= 0 {
			return nil, context.DeadlineExceeded
		}
		tabCtx, detach, _, err := AttachBorrowedPageTarget(runtimeBrowser, targetID, attachTimeout)
		if err != nil {
			lastErr = err
			continue
		}
		chromeContext := chromedp.FromContext(tabCtx)
		if chromeContext == nil || chromeContext.Target == nil {
			detach()
			lastErr = fmt.Errorf("%w: attached target executor unavailable", ErrBorrowedCookieTargetUnavailable)
			continue
		}
		records, readErr := readBorrowedCookiesForURLs(ctx, tabCtx, chromeContext.Target, urls, normalizedDomains)
		detach()
		if readErr == nil {
			return records, nil
		}
		lastErr = readErr
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if lastErr != nil {
		return nil, fmt.Errorf("%w: %v", ErrBorrowedCookieTargetUnavailable, lastErr)
	}
	return nil, ErrBorrowedCookieTargetUnavailable
}

func readBorrowedCookiesForURLs(
	callerCtx context.Context,
	tabCtx context.Context,
	executor cdp.Executor,
	urls []string,
	domains []string,
) ([]appcookies.Record, error) {
	if tabCtx == nil || executor == nil {
		return nil, ErrBorrowedCookieTargetUnavailable
	}
	if callerCtx == nil {
		callerCtx = context.Background()
	}
	readTimeout := boundedBorrowedCookieTimeout(callerCtx, borrowedCookieReadTimeout)
	if readTimeout <= 0 {
		return nil, context.DeadlineExceeded
	}
	readCtx, cancel := context.WithTimeout(tabCtx, readTimeout)
	stopCallerCancel := context.AfterFunc(callerCtx, cancel)
	defer func() {
		stopCallerCancel()
		cancel()
	}()
	// chromedp.Run normally injects a page's executor for protocol actions.
	// This helper issues the command directly, so bind the executor captured
	// from the already-attached, scope-checked user tab explicitly.
	readCtx = cdp.WithExecutor(readCtx, executor)

	items, err := network.GetCookies().WithURLs(urls).Do(readCtx)
	if err != nil {
		return nil, wrapRuntimeHangError(err)
	}
	// Network.getCookies is the first minimization boundary. Keep a second,
	// local boundary in case a browser backend returns an over-broad result.
	return appcookies.FilterByDomains(mapCDPCookies(items), domains), nil
}

func borrowedCookiePageTargetIDs(runtimeBrowser *Runtime) []string {
	if runtimeBrowser == nil || !runtimeBrowser.IsBorrowed() {
		return nil
	}
	manager := runtimeBrowser.TargetManager()
	if manager == nil {
		return nil
	}
	targetIDs := make([]string, 0)
	for _, info := range manager.ListPageTargets() {
		if !runtimeBrowser.BorrowedPageTargetInScope(info) {
			continue
		}
		if targetID := strings.TrimSpace(string(info.TargetID)); targetID != "" {
			targetIDs = append(targetIDs, targetID)
		}
	}
	sort.Strings(targetIDs)
	return targetIDs
}

func normalizeBorrowedCookieAllowlist(domains []string) ([]string, []string, error) {
	set := make(map[string]struct{}, len(domains))
	for _, rawDomain := range domains {
		domain := strings.Trim(strings.ToLower(strings.TrimSpace(rawDomain)), ".")
		if !validBorrowedCookieDomain(domain) {
			return nil, nil, ErrBorrowedCookieAllowlistInvalid
		}
		set[domain] = struct{}{}
	}
	if len(set) == 0 {
		return nil, nil, ErrBorrowedCookieAllowlistInvalid
	}
	normalized := make([]string, 0, len(set))
	for domain := range set {
		normalized = append(normalized, domain)
	}
	sort.Strings(normalized)

	urlSet := make(map[string]struct{}, len(normalized)*2)
	urls := make([]string, 0, len(normalized)*2)
	appendURL := func(rawURL string) {
		if _, exists := urlSet[rawURL]; exists {
			return
		}
		urlSet[rawURL] = struct{}{}
		urls = append(urls, rawURL)
	}
	for _, domain := range normalized {
		appendURL("https://" + domain + "/")
		if !strings.HasPrefix(domain, "www.") {
			appendURL("https://www." + domain + "/")
		}
	}
	return normalized, urls, nil
}

func validBorrowedCookieDomain(domain string) bool {
	if len(domain) == 0 || len(domain) > 253 || !strings.Contains(domain, ".") || net.ParseIP(domain) != nil {
		return false
	}
	for _, label := range strings.Split(domain, ".") {
		if len(label) == 0 || len(label) > 63 || label[0] == '-' || label[len(label)-1] == '-' {
			return false
		}
		for _, character := range label {
			if (character < 'a' || character > 'z') &&
				(character < '0' || character > '9') && character != '-' {
				return false
			}
		}
	}
	return true
}

func boundedBorrowedCookieTimeout(ctx context.Context, maximum time.Duration) time.Duration {
	if maximum <= 0 {
		return 0
	}
	if ctx == nil {
		return maximum
	}
	deadline, ok := ctx.Deadline()
	if !ok {
		return maximum
	}
	remaining := time.Until(deadline)
	if remaining <= 0 {
		return 0
	}
	if remaining < maximum {
		return remaining
	}
	return maximum
}
