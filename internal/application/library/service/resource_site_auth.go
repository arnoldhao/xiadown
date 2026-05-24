package service

import (
	"context"
	"strings"
	"time"

	"github.com/chromedp/chromedp"

	appcookies "xiadown/internal/application/cookies"
)

const (
	resourceSniffAuthStatusLoggedIn  = "logged_in"
	resourceSniffAuthStatusLoggedOut = "logged_out"
	resourceSniffAuthStatusUnknown   = "unknown"
)

var resourceXiaohongshuLoginCookieNames = map[string]struct{}{
	"web_session":    {},
	"web_session_id": {},
}

type resourceSniffAuthInfo struct {
	Status string
	User   string
	Site   string
}

func resourceSniffAuthInfoForPage(ctx context.Context, pageURL string) resourceSniffAuthInfo {
	site := resourceSniffAuthSite(pageURL)
	if site == "" {
		return resourceSniffAuthInfo{}
	}
	info := resourceSniffAuthInfo{
		Status: resourceSniffAuthStatusUnknown,
		Site:   site,
	}
	records, err := resourceSniffReadBrowserCookies(ctx, pageURL)
	if err == nil {
		switch site {
		case "douyin":
			if resourceDouyinCookieRecordsLoggedIn(records, pageURL, time.Now()) {
				info.Status = resourceSniffAuthStatusLoggedIn
			} else {
				info.Status = resourceSniffAuthStatusLoggedOut
			}
		case "xiaohongshu":
			if resourceXiaohongshuCookieRecordsLoggedIn(records, pageURL, time.Now()) {
				info.Status = resourceSniffAuthStatusLoggedIn
			} else {
				info.Status = resourceSniffAuthStatusLoggedOut
			}
		}
	}
	if info.Status == resourceSniffAuthStatusLoggedOut {
		return info
	}
	if user := resourceSniffPageAuthUser(ctx, site); user != "" {
		info.User = user
		info.Status = resourceSniffAuthStatusLoggedIn
	}
	return info
}

func resourceSniffAuthSite(pageURL string) string {
	switch resourceExtractorForURL(pageURL).Name() {
	case (resourceDouyinSiteRules{}).Name():
		return "douyin"
	case (resourceXiaohongshuSiteRules{}).Name():
		return "xiaohongshu"
	default:
		return ""
	}
}

func resourceXiaohongshuCookieRecordsLoggedIn(records []appcookies.Record, pageURL string, now time.Time) bool {
	matched := appcookies.MatchURL(records, firstNonEmpty(strings.TrimSpace(pageURL), "https://www.xiaohongshu.com/"))
	if len(matched) == 0 {
		return false
	}
	for _, record := range matched {
		if resourceXiaohongshuLoginCookieValid(record, now) {
			return true
		}
	}
	return false
}

func resourceXiaohongshuLoginCookieValid(record appcookies.Record, now time.Time) bool {
	name := strings.ToLower(strings.TrimSpace(record.Name))
	if _, ok := resourceXiaohongshuLoginCookieNames[name]; !ok {
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
	return normalized != "0" && normalized != "false" && normalized != "null" && normalized != "undefined"
}

func resourceSniffPageAuthUser(ctx context.Context, site string) string {
	if ctx == nil {
		return ""
	}
	script := resourceSniffPageAuthScript(site)
	if script == "" {
		return ""
	}
	result := map[string]any{}
	err := chromedp.Run(ctx, chromedp.Evaluate(script, &result))
	if err != nil {
		return ""
	}
	return resourceCleanMetadataText(fmtString(result["user"]))
}

func resourceSniffPageAuthScript(site string) string {
	switch site {
	case "douyin":
		return resourceDouyinAuthUserScript()
	case "xiaohongshu":
		return resourceXiaohongshuAuthUserScript()
	default:
		return ""
	}
}

func resourceDouyinAuthUserScript() string {
	return resourceSiteAuthUserScript(`[
			window.__INITIAL_STATE__?.user,
			window.__INITIAL_STATE__?.userInfo,
			window.__INITIAL_STATE__?.loginUser,
			window.__UNIVERSAL_DATA_FOR_REHYDRATION__?.__DEFAULT_SCOPE__?.webapp?.user,
			window.__UNIVERSAL_DATA_FOR_REHYDRATION__?.__DEFAULT_SCOPE__?.user
		]`, []string{
		"LOGIN_USER",
		"USER_INFO",
		"CURRENT_USER",
		"ACCOUNT",
		"passport",
		"user",
	})
}

func resourceXiaohongshuAuthUserScript() string {
	return resourceSiteAuthUserScript(`[
			window.__INITIAL_STATE__?.user?.userInfo,
			window.__INITIAL_STATE__?.user?.loggedInUser,
			window.__INITIAL_STATE__?.user?.currentUser,
			window.__INITIAL_STATE__?.user,
			window.__INITIAL_SSR_STATE__?.user?.userInfo,
			window.__INITIAL_SSR_STATE__?.user
		]`, []string{
		"userInfo",
		"currentUser",
		"loggedInUser",
		"account",
		"profile",
	})
}

func resourceSiteAuthUserScript(rootExpression string, storageKeyHints []string) string {
	keyPattern := strings.Join(storageKeyHints, "|")
	return `(() => {
		const clean = (value) => String(value || "").replace(/\s+/g, " ").trim();
		const roots = ` + rootExpression + `;
		const keyPattern = /(?:` + keyPattern + `)/i;
		const readName = (object) => {
			if (!object || typeof object !== "object" || Array.isArray(object)) return "";
			for (const key of ["nickname", "nickName", "displayName", "display_name", "userName", "username", "name", "uniqueId", "unique_id", "redId", "red_id"]) {
				const value = clean(object[key]);
				if (value && value.length <= 80 && !/^(null|undefined|false|true)$/i.test(value)) return value;
			}
			return "";
		};
		const scan = (root) => {
			const queue = [root];
			let count = 0;
			while (queue.length && count < 400) {
				const value = queue.shift();
				count += 1;
				if (!value || typeof value !== "object") continue;
				const name = readName(value);
				if (name) return name;
				if (Array.isArray(value)) {
					for (const child of value.slice(0, 20)) queue.push(child);
					continue;
				}
				for (const [key, child] of Object.entries(value)) {
					if (!child || typeof child !== "object") continue;
					if (keyPattern.test(key) || keyPattern.test(JSON.stringify(Object.keys(child)).slice(0, 500))) {
						queue.push(child);
					}
				}
			}
			return "";
		};
		for (const root of roots) {
			const user = scan(root);
			if (user) return { user };
		}
		try {
			for (const store of [window.localStorage, window.sessionStorage]) {
				if (!store) continue;
				for (let index = 0; index < Math.min(store.length, 80); index += 1) {
					const key = String(store.key(index) || "");
					if (!keyPattern.test(key)) continue;
					const raw = String(store.getItem(key) || "");
					if (!raw || raw.length > 200000) continue;
					try {
						const user = scan(JSON.parse(raw));
						if (user) return { user };
					} catch (_) {}
				}
			}
		} catch (_) {}
		return { user: "" };
	})()`
}
