package sitepolicy

import (
	"net/url"
	"strings"
)

type Policy struct {
	Key                string
	ConnectorType      string
	Domains            []string
	ReadySelectors     []string
	ExtractorSelectors []string
	RemoveSelectors    []string
	Capabilities       []string
}

var builtinPolicyOrder = []string{
	"youtube",
	"bilibili",
	"tiktok",
	"douyin",
	"instagram",
	"x",
	"facebook",
	"vimeo",
	"twitch",
	"niconico",
}

var builtinPolicies = map[string]Policy{
	"youtube": {
		Key:           "youtube",
		ConnectorType: "youtube",
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
		Key:           "bilibili",
		ConnectorType: "bilibili",
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
		Key:           "tiktok",
		ConnectorType: "tiktok",
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
	"douyin": {
		Key:           "douyin",
		ConnectorType: "douyin",
		Domains: []string{
			"douyin.com",
			"iesdouyin.com",
		},
		ReadySelectors: []string{
			"#douyin-web",
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
		Capabilities: []string{"cookies", "web_fetch", "browser", "download"},
	},
	"instagram": {
		Key:           "instagram",
		ConnectorType: "instagram",
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
		Key:           "x",
		ConnectorType: "x",
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
		Key:           "facebook",
		ConnectorType: "facebook",
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
		Key:           "vimeo",
		ConnectorType: "vimeo",
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
		Key:           "twitch",
		ConnectorType: "twitch",
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
		Key:           "niconico",
		ConnectorType: "niconico",
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

func ForConnectorType(connectorType string) (Policy, bool) {
	policy, ok := builtinPolicies[strings.ToLower(strings.TrimSpace(connectorType))]
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

func DomainsForConnector(connectorType string) []string {
	policy, ok := ForConnectorType(connectorType)
	if !ok {
		return nil
	}
	return cloneStrings(policy.Domains)
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

func hostname(rawURL string) string {
	parsed, err := url.Parse(strings.TrimSpace(rawURL))
	if err != nil {
		return ""
	}
	return strings.ToLower(strings.TrimSpace(parsed.Hostname()))
}
