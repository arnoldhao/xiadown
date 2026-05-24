package service

import (
	"context"
	"strings"
	"time"

	"github.com/chromedp/chromedp"

	"xiadown/internal/application/browsercdp"
	appcookies "xiadown/internal/application/cookies"
)

const resourceDouyinLoginCookieReadTimeout = 2 * time.Second

var resourceDouyinLoginCookieNames = map[string]struct{}{
	"sessionid":               {},
	"sessionid_ss":            {},
	"sid_guard":               {},
	"sid_tt":                  {},
	"sid_ucp_v1":              {},
	"sid_ucp_sso_v1":          {},
	"ssid_ucp_v1":             {},
	"ssid_ucp_sso_v1":         {},
	"uid_tt":                  {},
	"uid_tt_ss":               {},
	"sso_uid_tt":              {},
	"sso_uid_tt_ss":           {},
	"toutiao_sso_user":        {},
	"toutiao_sso_user_ss":     {},
	"passport_auth_status":    {},
	"passport_auth_status_ss": {},
	"login_status":            {},
}

func resourceDouyinRecommendLoginFailure(ctx context.Context, pageURL string, pageMeta map[string]string) (resourceSniffFailure, bool) {
	pageURL = firstNonEmpty(strings.TrimSpace(pageMeta["location"]), strings.TrimSpace(pageURL))
	if !resourceDouyinIsRecommendPage(pageURL) {
		return resourceSniffFailure{}, false
	}
	if resourceDouyinBrowserSessionLoggedIn(ctx, pageURL) {
		return resourceSniffFailure{}, false
	}
	return resourceSniffDouyinRecommendLoginFailure(), true
}

func resourceDouyinBrowserSessionLoggedIn(ctx context.Context, pageURL string) bool {
	if ctx == nil {
		return false
	}
	readCtx, cancel := context.WithTimeout(ctx, resourceDouyinLoginCookieReadTimeout)
	defer cancel()
	records, err := resourceSniffReadBrowserCookies(readCtx, pageURL)
	if err != nil {
		return false
	}
	return resourceDouyinCookieRecordsLoggedIn(records, pageURL, time.Now())
}

func resourceSniffReadBrowserCookies(ctx context.Context, rawURL string) ([]appcookies.Record, error) {
	if ctx == nil {
		return nil, context.Canceled
	}
	cookieURL := firstNonEmpty(strings.TrimSpace(rawURL), "https://www.douyin.com/")
	var records []appcookies.Record
	err := chromedp.Run(ctx, chromedp.ActionFunc(func(actionCtx context.Context) error {
		items, err := browsercdp.GetCookiesForURLs(actionCtx, []string{cookieURL})
		if err == nil && len(items) > 0 {
			records = items
			return nil
		}
		storageItems, storageErr := browsercdp.GetStorageCookies(actionCtx)
		if storageErr != nil {
			if err != nil {
				return err
			}
			return storageErr
		}
		records = storageItems
		return nil
	}))
	return records, err
}

func resourceDouyinCookieRecordsLoggedIn(records []appcookies.Record, pageURL string, now time.Time) bool {
	matched := appcookies.MatchURL(records, firstNonEmpty(strings.TrimSpace(pageURL), "https://www.douyin.com/"))
	if len(matched) == 0 {
		return false
	}
	for _, record := range matched {
		if resourceDouyinLoginCookieValid(record, now) {
			return true
		}
	}
	return false
}

func resourceDouyinLoginCookieValid(record appcookies.Record, now time.Time) bool {
	name := strings.ToLower(strings.TrimSpace(record.Name))
	if !resourceDouyinLoginCookieNameLooksAuthenticated(name) {
		return false
	}
	if record.Expires > 0 && !now.IsZero() && record.Expires <= now.Unix() {
		return false
	}
	value := strings.TrimSpace(record.Value)
	if value == "" {
		return false
	}
	normalized := strings.ToLower(value)
	if normalized == "0" || normalized == "false" || normalized == "null" || normalized == "undefined" {
		return false
	}
	if name == "login_status" {
		return normalized == "1" || normalized == "true"
	}
	return true
}

func resourceDouyinLoginCookieNameLooksAuthenticated(name string) bool {
	name = strings.ToLower(strings.TrimSpace(name))
	if _, ok := resourceDouyinLoginCookieNames[name]; ok {
		return true
	}
	return strings.HasPrefix(name, "sessionid") ||
		strings.HasPrefix(name, "sid_") ||
		strings.HasPrefix(name, "ssid_") ||
		strings.HasPrefix(name, "uid_tt") ||
		strings.HasPrefix(name, "sso_uid_tt") ||
		strings.HasPrefix(name, "passport_auth_status") ||
		strings.HasPrefix(name, "toutiao_sso_user")
}
