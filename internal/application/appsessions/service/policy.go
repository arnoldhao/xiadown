package service

import (
	"sort"
	"strings"

	"xiadown/internal/application/appsessions/dto"
	"xiadown/internal/application/sitepolicy"
)

func supportedSiteKeys() []string {
	result := make([]string, 0)
	for _, policy := range sitepolicy.List() {
		if !policyHasCapability(policy, "cookies") || !policyHasCapability(policy, "download") {
			continue
		}
		siteKey := strings.TrimSpace(policy.Key)
		if siteKey == "" {
			continue
		}
		result = append(result, siteKey)
	}
	return result
}

func supportedSiteKeySet() map[string]struct{} {
	result := make(map[string]struct{})
	for _, siteKey := range supportedSiteKeys() {
		result[siteKey] = struct{}{}
	}
	return result
}

func supportedSiteKeyOrder() map[string]int {
	result := make(map[string]int)
	for index, siteKey := range supportedSiteKeys() {
		result[siteKey] = index
	}
	return result
}

func isSupportedSiteKey(siteKey string) bool {
	_, ok := supportedSiteKeySet()[strings.TrimSpace(siteKey)]
	return ok
}

func policyHasCapability(policy sitepolicy.Policy, capability string) bool {
	for _, item := range policy.Capabilities {
		if strings.EqualFold(strings.TrimSpace(item), capability) {
			return true
		}
	}
	return false
}

func siteAppSessionID(siteKey string) string {
	return "site-app-session-" + sanitizeSiteKey(siteKey)
}

func sanitizeSiteKey(value string) string {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		return "default"
	}
	var builder strings.Builder
	builder.Grow(len(trimmed))
	for _, current := range trimmed {
		switch {
		case current >= 'a' && current <= 'z',
			current >= 'A' && current <= 'Z',
			current >= '0' && current <= '9',
			current == '-',
			current == '_',
			current == '.':
			builder.WriteRune(current)
		default:
			builder.WriteByte('_')
		}
	}
	result := strings.Trim(builder.String(), "._-")
	if result == "" {
		return "default"
	}
	return result
}

func appSessionHomeURL(siteKey string) string {
	switch strings.TrimSpace(siteKey) {
	case "youtube":
		return "https://www.youtube.com/"
	case "bilibili":
		return "https://www.bilibili.com/"
	case "tiktok":
		return "https://www.tiktok.com/"
	case "instagram":
		return "https://www.instagram.com/"
	case "x":
		return "https://x.com/"
	case "facebook":
		return "https://www.facebook.com/"
	case "vimeo":
		return "https://vimeo.com/"
	case "twitch":
		return "https://www.twitch.tv/"
	case "niconico":
		return "https://www.nicovideo.jp/"
	default:
		return ""
	}
}

func appSessionLoginURL(siteKey string, fallbackURL string) string {
	switch strings.TrimSpace(siteKey) {
	case "youtube":
		return "https://accounts.google.com/ServiceLogin?service=youtube&uilel=3&passive=true&continue=https%3A%2F%2Fwww.youtube.com%2Fsignin%3Faction_handle_signin%3Dtrue%26app%3Ddesktop%26hl%3Den%26next%3Dhttps%253A%252F%252Fmusic.youtube.com%252F"
	case "bilibili":
		return "https://passport.bilibili.com/login"
	case "tiktok":
		return "https://www.tiktok.com/login"
	case "instagram":
		return "https://www.instagram.com/accounts/login/"
	case "x":
		return "https://x.com/i/flow/login"
	case "facebook":
		return "https://www.facebook.com/login"
	case "vimeo":
		return "https://vimeo.com/log_in"
	case "twitch":
		return "https://www.twitch.tv/login"
	case "niconico":
		return "https://account.nicovideo.jp/login"
	default:
		return fallbackURL
	}
}

func appSessionCookieDomains(siteKey string) []string {
	policy, _ := sitepolicy.ForSiteKey(siteKey)
	domains := append([]string(nil), policy.Domains...)
	if strings.TrimSpace(siteKey) == "youtube" {
		domains = append(domains, "google.com", "googleusercontent.com", "gstatic.com", "ytimg.com")
	}
	return domains
}

func appSessionSiteLabel(siteKey string) string {
	switch strings.TrimSpace(siteKey) {
	case "youtube":
		return "YouTube"
	case "bilibili":
		return "Bilibili"
	case "tiktok":
		return "TikTok"
	case "instagram":
		return "Instagram"
	case "x":
		return "X"
	case "facebook":
		return "Facebook"
	case "vimeo":
		return "Vimeo"
	case "twitch":
		return "Twitch"
	case "niconico":
		return "Niconico"
	default:
		return strings.TrimSpace(siteKey)
	}
}

func appSessionSiteDesc(siteKey string) string {
	switch strings.TrimSpace(siteKey) {
	case "youtube":
		return "YouTube and YouTube Music authentication for downloads and playback."
	case "bilibili":
		return "Bilibili account session for member-only and authenticated downloads."
	case "tiktok":
		return "TikTok account session for authenticated creator and private-feed downloads."
	case "instagram":
		return "Instagram account session for authenticated media downloads."
	case "x":
		return "X account session for authenticated media downloads."
	case "facebook":
		return "Facebook account session for authenticated video downloads."
	case "vimeo":
		return "Vimeo account session for authenticated and private videos."
	case "twitch":
		return "Twitch account session for authenticated streams, VODs, and clips."
	case "niconico":
		return "Niconico account session for authenticated video downloads."
	default:
		return "External site authentication for downloads."
	}
}

func sortAppSessionDTOs(items []dto.AppSession, order map[string]int) {
	sort.SliceStable(items, func(i, j int) bool {
		left := strings.TrimSpace(items[i].SiteKey)
		right := strings.TrimSpace(items[j].SiteKey)
		leftIndex, leftKnown := order[left]
		rightIndex, rightKnown := order[right]
		if leftKnown != rightKnown {
			return leftKnown
		}
		if leftKnown && leftIndex != rightIndex {
			return leftIndex < rightIndex
		}
		return strings.ToLower(left) < strings.ToLower(right)
	})
}
