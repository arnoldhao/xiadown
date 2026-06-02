package sitepolicy

import (
	"net/url"
	"strings"
)

type Policy struct {
	Key                string
	SiteKey            string
	Domains            []string
	ProfileSites       []ProfileSite
	ReadySelectors     []string
	ExtractorSelectors []string
	RemoveSelectors    []string
	Capabilities       []string
}

type ProfileSite struct {
	Key   string
	Label string
	URL   string
}

var builtinPolicyOrder = []string{
	"youtube",
	"bilibili",
	"tiktok",
	"china_private",
	"instagram",
	"x",
	"facebook",
	"vimeo",
	"twitch",
	"niconico",
}

var builtinPolicies = map[string]Policy{
	"youtube": {
		Key:     "youtube",
		SiteKey: "youtube",
		Domains: []string{
			"youtube.com",
			"youtu.be",
			"youtube-nocookie.com",
		},
		ReadySelectors: []string{
			"ytd-watch-flexy",
			"#content",
			"main",
			"body",
		},
		ExtractorSelectors: []string{
			"#description",
			"#description-inline-expander",
			"ytd-watch-metadata",
			"main",
		},
		RemoveSelectors: []string{
			"#related",
			"ytd-comments",
			"ytd-merch-shelf-renderer",
			"ytd-rich-grid-renderer",
		},
		Capabilities: []string{"cookies", "web_fetch", "browser", "download"},
	},
	"bilibili": {
		Key:     "bilibili",
		SiteKey: "bilibili",
		Domains: []string{
			"bilibili.com",
			"b23.tv",
		},
		ReadySelectors: []string{
			"#app",
			"#arc_toolbar_report",
			"main",
			"body",
		},
		ExtractorSelectors: []string{
			"main",
			"article",
			"#app",
		},
		RemoveSelectors: []string{
			".video-toolbar-v1",
			".right-container",
			".comment-container",
		},
		Capabilities: []string{"cookies", "web_fetch", "browser", "download"},
	},
	"tiktok": {
		Key:     "tiktok",
		SiteKey: "tiktok",
		Domains: []string{
			"tiktok.com",
			"tiktokv.com",
			"vm.tiktok.com",
		},
		ReadySelectors: []string{
			"main",
			"#app",
			"body",
		},
		ExtractorSelectors: []string{
			"main",
			"article",
			"#app",
		},
		RemoveSelectors: []string{
			"[data-e2e=\"recommend-list-item-container\"]",
			"[data-e2e=\"comment-list\"]",
		},
		Capabilities: []string{"cookies", "web_fetch", "browser", "download"},
	},
	"china_private": {
		Key:     "china_private",
		SiteKey: "china_private",
		Domains: []string{
			"douyin.com",
			"iesdouyin.com",
			"xiaohongshu.com",
			"rednote.com",
			"xhs.cn",
			"xhslink.com",
			"xhslink.cn",
			"xhsurl.com",
			"rl.ink",
		},
		ProfileSites: []ProfileSite{
			{Key: "douyin", Label: "douyin.com", URL: "https://www.douyin.com/"},
			{Key: "xiaohongshu", Label: "xiaohongshu.com", URL: "https://www.xiaohongshu.com/explore"},
		},
		ReadySelectors: []string{
			"#douyin-web",
			"#app",
			"#root",
			"body",
		},
		ExtractorSelectors: []string{
			"main",
			"article",
			"#douyin-web",
			"#root",
		},
		RemoveSelectors: []string{
			".recommend",
			".comment",
		},
		Capabilities: []string{"profile", "browser", "download"},
	},
	"instagram": {
		Key:     "instagram",
		SiteKey: "instagram",
		Domains: []string{
			"instagram.com",
		},
		ReadySelectors: []string{
			"main",
			"article",
			"body",
		},
		ExtractorSelectors: []string{
			"main",
			"article",
		},
		RemoveSelectors: []string{
			"nav",
			"footer",
		},
		Capabilities: []string{"cookies", "web_fetch", "browser", "download"},
	},
	"x": {
		Key:     "x",
		SiteKey: "x",
		Domains: []string{
			"x.com",
			"twitter.com",
		},
		ReadySelectors: []string{
			"main",
			"[data-testid=\"primaryColumn\"]",
			"body",
		},
		ExtractorSelectors: []string{
			"main",
			"article",
			"[data-testid=\"tweet\"]",
		},
		RemoveSelectors: []string{
			"nav",
			"[data-testid=\"sidebarColumn\"]",
			"[aria-label=\"Timeline: Trending now\"]",
			"[aria-label=\"Who to follow\"]",
		},
		Capabilities: []string{"cookies", "web_fetch", "browser", "download"},
	},
	"facebook": {
		Key:     "facebook",
		SiteKey: "facebook",
		Domains: []string{
			"facebook.com",
			"fb.watch",
		},
		ReadySelectors: []string{
			"main",
			"[role=\"main\"]",
			"body",
		},
		ExtractorSelectors: []string{
			"main",
			"article",
			"[role=\"main\"]",
		},
		RemoveSelectors: []string{
			"nav",
			"[role=\"complementary\"]",
		},
		Capabilities: []string{"cookies", "web_fetch", "browser", "download"},
	},
	"vimeo": {
		Key:     "vimeo",
		SiteKey: "vimeo",
		Domains: []string{
			"vimeo.com",
			"player.vimeo.com",
		},
		ReadySelectors: []string{
			"main",
			".vp-video-wrapper",
			"body",
		},
		ExtractorSelectors: []string{
			"main",
			"article",
			".vp-video-wrapper",
		},
		RemoveSelectors: []string{
			"footer",
			".iris_nav",
		},
		Capabilities: []string{"cookies", "web_fetch", "browser", "download"},
	},
	"twitch": {
		Key:     "twitch",
		SiteKey: "twitch",
		Domains: []string{
			"twitch.tv",
			"clips.twitch.tv",
		},
		ReadySelectors: []string{
			"main",
			"[data-a-target=\"video-player\"]",
			"body",
		},
		ExtractorSelectors: []string{
			"main",
			"[data-a-target=\"video-player\"]",
		},
		RemoveSelectors: []string{
			"nav",
			"[data-a-target=\"right-column\"]",
		},
		Capabilities: []string{"cookies", "web_fetch", "browser", "download"},
	},
	"niconico": {
		Key:     "niconico",
		SiteKey: "niconico",
		Domains: []string{
			"nicovideo.jp",
			"nico.ms",
			"nicovideo.cdn.nimg.jp",
		},
		ReadySelectors: []string{
			"main",
			"#root",
			"body",
		},
		ExtractorSelectors: []string{
			"main",
			"article",
			"#root",
		},
		RemoveSelectors: []string{
			"aside",
			".CommentPanel",
		},
		Capabilities: []string{"cookies", "web_fetch", "browser", "download"},
	},
}

func List() []Policy {
	result := make([]Policy, 0, len(builtinPolicyOrder))
	for _, key := range builtinPolicyOrder {
		policy, ok := builtinPolicies[key]
		if !ok {
			continue
		}
		result = append(result, policy)
	}
	return result
}

func ForSiteKey(siteKey string) (Policy, bool) {
	policy, ok := builtinPolicies[strings.ToLower(strings.TrimSpace(siteKey))]
	return policy, ok
}

func ForURL(rawURL string) (Policy, bool) {
	host := hostname(rawURL)
	if host == "" {
		return Policy{}, false
	}
	for _, key := range builtinPolicyOrder {
		policy, ok := builtinPolicies[key]
		if !ok {
			continue
		}
		for _, domain := range policy.Domains {
			if HostMatchesDomain(host, domain) {
				return policy, true
			}
		}
	}
	return Policy{}, false
}

func DomainsForSiteKey(siteKey string) []string {
	policy, ok := ForSiteKey(siteKey)
	if !ok {
		return nil
	}
	return cloneStrings(policy.Domains)
}

func ProfileSitesForSiteKey(siteKey string) []ProfileSite {
	policy, ok := ForSiteKey(siteKey)
	if !ok {
		return nil
	}
	return cloneProfileSites(policy.ProfileSites)
}

func ReadySelectorForURL(rawURL string) string {
	policy, ok := ForURL(rawURL)
	if !ok {
		return ""
	}
	for _, selector := range policy.ReadySelectors {
		if strings.TrimSpace(selector) != "" {
			return strings.TrimSpace(selector)
		}
	}
	return ""
}

func HostMatchesDomain(host string, domain string) bool {
	normalizedHost := strings.TrimPrefix(strings.ToLower(strings.TrimSpace(host)), ".")
	normalizedDomain := strings.TrimPrefix(strings.ToLower(strings.TrimSpace(domain)), ".")
	if normalizedHost == "" || normalizedDomain == "" {
		return false
	}
	return normalizedHost == normalizedDomain || strings.HasSuffix(normalizedHost, "."+normalizedDomain)
}

func MatchDomains(rawURL string, domains []string) bool {
	host := hostname(rawURL)
	if host == "" {
		return false
	}
	for _, domain := range domains {
		if HostMatchesDomain(host, domain) {
			return true
		}
	}
	return false
}

func cloneStrings(values []string) []string {
	if len(values) == 0 {
		return nil
	}
	result := make([]string, 0, len(values))
	for _, value := range values {
		trimmed := strings.TrimSpace(value)
		if trimmed == "" {
			continue
		}
		result = append(result, trimmed)
	}
	return result
}

func cloneProfileSites(values []ProfileSite) []ProfileSite {
	if len(values) == 0 {
		return nil
	}
	result := make([]ProfileSite, 0, len(values))
	for _, value := range values {
		site := ProfileSite{
			Key:   strings.TrimSpace(value.Key),
			Label: strings.TrimSpace(value.Label),
			URL:   strings.TrimSpace(value.URL),
		}
		if site.URL == "" {
			continue
		}
		result = append(result, site)
	}
	return result
}

func hostname(rawURL string) string {
	parsed, err := url.Parse(strings.TrimSpace(rawURL))
	if err != nil {
		return ""
	}
	return strings.ToLower(strings.TrimSpace(parsed.Hostname()))
}
