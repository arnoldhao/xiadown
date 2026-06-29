package service

import (
	"context"
	"encoding/json"
	"strings"
	"time"

	"xiadown/internal/application/appsessions/dto"
	appcookies "xiadown/internal/application/cookies"
	"xiadown/internal/application/sitepolicy"
	"xiadown/internal/domain/appsessions"
)

const appSessionAccountExpiresAtMetadataKey = "_expiresAt"

func (service *AppSessionsService) mapSessionDTO(ctx context.Context, session appsessions.Session) dto.AppSession {
	_ = ctx
	return service.mapSessionDTOFromCookies(session, nil, false)
}

func (service *AppSessionsService) mapSessionDTOWithCookies(session appsessions.Session, cookies []appcookies.Record) dto.AppSession {
	return service.mapSessionDTOFromCookies(session, cookies, true)
}

func (service *AppSessionsService) mapSessionDTOFromCookies(session appsessions.Session, cookies []appcookies.Record, cookiesLoaded bool) dto.AppSession {
	hasAuthCookies := appSessionHasAuthenticationCookies(session.SiteKey, cookies)
	status := session.Status
	if status == "" {
		status = appsessions.StatusDisconnected
	}
	if cookiesLoaded {
		if len(cookies) == 0 || !hasAuthCookies {
			status = appsessions.StatusDisconnected
		} else if status == appsessions.StatusDisconnected {
			status = appsessions.StatusConnected
		}
	}
	credentialHasAuthCookies := hasAuthCookies
	if !cookiesLoaded && status == appsessions.StatusConnected {
		credentialHasAuthCookies = true
	}
	policy, _ := sitepolicy.ForSiteKey(session.SiteKey)
	lastVerified := ""
	if session.LastVerifiedAt != nil {
		lastVerified = session.LastVerifiedAt.Format(time.RFC3339)
	}
	verificationStartedAt := ""
	if session.AccountVerificationStartedAt != nil {
		verificationStartedAt = session.AccountVerificationStartedAt.Format(time.RFC3339)
	}
	return dto.AppSession{
		ID:                           session.ID,
		SiteKey:                      session.SiteKey,
		Group:                        "video",
		Label:                        appSessionSiteLabel(session.SiteKey),
		Desc:                         appSessionSiteDesc(session.SiteKey),
		Status:                       string(status),
		CredentialState:              appSessionCredentialState(status, credentialHasAuthCookies),
		CookiesCount:                 len(cookies),
		Cookies:                      mapCookiesDTO(cookies),
		Domains:                      append([]string(nil), policy.Domains...),
		Account:                      accountDTO(session, cookies),
		PolicyKey:                    policy.Key,
		Capabilities:                 append([]string(nil), policy.Capabilities...),
		ProviderSupported:            service.provider != nil && service.provider.AppSessionsSupported(),
		AccountVerificationStatus:    string(appSessionAccountVerificationStatus(session, credentialHasAuthCookies)),
		AccountVerificationError:     session.AccountVerificationError,
		AccountVerificationStartedAt: verificationStartedAt,
		LastVerifiedAt:               lastVerified,
	}
}

func appSessionCredentialState(status appsessions.Status, hasAuthCookies bool) string {
	if status == appsessions.StatusConnected && hasAuthCookies {
		return "app_session"
	}
	return string(appsessions.StatusDisconnected)
}

func appSessionAccountVerificationStatus(session appsessions.Session, hasAuthCookies bool) appsessions.AccountVerificationStatus {
	if !hasAuthCookies {
		return appsessions.AccountVerificationUnverified
	}
	status := session.AccountVerificationStatus
	switch status {
	case appsessions.AccountVerificationVerifying,
		appsessions.AccountVerificationVerified,
		appsessions.AccountVerificationUnverified,
		appsessions.AccountVerificationUnsupported:
		return status
	}
	if !appSessionRequiresAccountVerification(session.SiteKey) {
		return appsessions.AccountVerificationUnsupported
	}
	if session.LastVerifiedAt != nil && !session.LastVerifiedAt.IsZero() {
		return appsessions.AccountVerificationVerified
	}
	return appsessions.AccountVerificationUnverified
}

func accountDTO(session appsessions.Session, cookies []appcookies.Record) *dto.AppSessionAccount {
	displayName := strings.TrimSpace(session.AccountDisplayName)
	handle := strings.TrimSpace(session.AccountHandle)
	avatarURL := strings.TrimSpace(session.AccountAvatarURL)
	tierKey := strings.TrimSpace(session.AccountTierKey)
	tierLabel := strings.TrimSpace(session.AccountTierLabel)
	badges := decodeBadges(session.AccountBadgesJSON)
	metadata := decodeMetadata(session.AccountMetadataJSON)
	expiresAt := ""
	if appSessionHasAuthenticationCookies(session.SiteKey, cookies) {
		expiresAt = cookieExpiresAt(session.SiteKey, cookies, time.Now())
	}
	if expiresAt == "" {
		expiresAt = accountMetadataExpiresAt(metadata, time.Now())
	}
	metadata = accountPublicMetadata(metadata)
	if displayName == "" && handle == "" && avatarURL == "" && tierKey == "" && tierLabel == "" && len(badges) == 0 && len(metadata) == 0 && expiresAt == "" {
		return nil
	}
	return &dto.AppSessionAccount{
		DisplayName: displayName,
		Handle:      handle,
		AvatarURL:   avatarURL,
		TierKey:     tierKey,
		TierLabel:   tierLabel,
		Badges:      badges,
		Metadata:    metadata,
		ExpiresAt:   expiresAt,
	}
}

func decodeBadges(raw string) []dto.AppSessionBadge {
	if strings.TrimSpace(raw) == "" {
		return nil
	}
	var badges []dto.AppSessionBadge
	if err := json.Unmarshal([]byte(raw), &badges); err != nil {
		return nil
	}
	return badges
}

func decodeMetadata(raw string) map[string]any {
	if strings.TrimSpace(raw) == "" {
		return nil
	}
	var metadata map[string]any
	if err := json.Unmarshal([]byte(raw), &metadata); err != nil {
		return nil
	}
	return metadata
}

func accountMetadataExpiresAt(metadata map[string]any, now time.Time) string {
	value := metadataString(metadata, appSessionAccountExpiresAtMetadataKey)
	if value == "" {
		return ""
	}
	expiresAt, err := time.Parse(time.RFC3339, value)
	if err != nil || !expiresAt.After(now) {
		return ""
	}
	return expiresAt.UTC().Format(time.RFC3339)
}

func accountPublicMetadata(metadata map[string]any) map[string]any {
	if len(metadata) == 0 {
		return nil
	}
	if _, ok := metadata[appSessionAccountExpiresAtMetadataKey]; !ok {
		return metadata
	}
	result := make(map[string]any, len(metadata)-1)
	for key, value := range metadata {
		if key == appSessionAccountExpiresAtMetadataKey {
			continue
		}
		result[key] = value
	}
	if len(result) == 0 {
		return nil
	}
	return result
}

func metadataString(metadata map[string]any, key string) string {
	if len(metadata) == 0 {
		return ""
	}
	value, ok := metadata[key]
	if !ok {
		return ""
	}
	switch typed := value.(type) {
	case string:
		return strings.TrimSpace(typed)
	default:
		return ""
	}
}

func cookieExpiresAt(siteKey string, cookies []appcookies.Record, now time.Time) string {
	if names := authCookieExpiryNames(siteKey); len(names) > 0 {
		if expiresAt, ok := nearestCookieExpiresAt(cookies, now, names); ok {
			return expiresAt.Format(time.RFC3339)
		}
	}
	return ""
}

func appSessionHasAuthenticationCookies(siteKey string, cookies []appcookies.Record) bool {
	if len(cookies) == 0 {
		return false
	}
	names := cookieNameSet(cookies)
	has := func(name string) bool {
		_, ok := names[strings.ToLower(strings.TrimSpace(name))]
		return ok
	}
	switch strings.TrimSpace(siteKey) {
	case "youtube":
		for name := range authCookieExpiryNames("youtube") {
			if has(name) {
				return true
			}
		}
		return false
	case "bilibili":
		return has("SESSDATA")
	case "tiktok":
		return has("sessionid") || has("sessionid_ss") || has("sid_guard") || has("sid_tt")
	case "instagram":
		return has("sessionid")
	case "x":
		return has("auth_token") && has("ct0")
	case "facebook":
		return has("c_user") && has("xs")
	case "vimeo":
		return has("vimeo")
	case "twitch":
		return has("auth-token")
	case "niconico":
		return has("user_session")
	default:
		return false
	}
}

func cookieNameSet(cookies []appcookies.Record) map[string]struct{} {
	names := make(map[string]struct{}, len(cookies))
	for _, record := range cookies {
		name := strings.ToLower(strings.TrimSpace(record.Name))
		if name != "" {
			names[name] = struct{}{}
		}
	}
	return names
}

func authCookieExpiryNames(siteKey string) map[string]struct{} {
	switch strings.TrimSpace(siteKey) {
	case "youtube":
		return map[string]struct{}{
			"APISID":            {},
			"HSID":              {},
			"LOGIN_INFO":        {},
			"SAPISID":           {},
			"SID":               {},
			"SSID":              {},
			"__Secure-1PAPISID": {},
			"__Secure-1PSID":    {},
			"__Secure-1PSIDCC":  {},
			"__Secure-1PSIDTS":  {},
			"__Secure-3PAPISID": {},
			"__Secure-3PSID":    {},
			"__Secure-3PSIDCC":  {},
			"__Secure-3PSIDTS":  {},
		}
	case "bilibili":
		return map[string]struct{}{
			"DedeUserID": {},
			"SESSDATA":   {},
			"bili_jct":   {},
		}
	case "tiktok":
		return map[string]struct{}{
			"sessionid":    {},
			"sessionid_ss": {},
			"sid_guard":    {},
			"sid_tt":       {},
			"uid_tt":       {},
			"uid_tt_ss":    {},
		}
	case "instagram":
		return map[string]struct{}{
			"sessionid": {},
		}
	case "x":
		return map[string]struct{}{
			"auth_token": {},
			"ct0":        {},
		}
	case "facebook":
		return map[string]struct{}{
			"c_user": {},
			"xs":     {},
		}
	case "vimeo":
		return map[string]struct{}{
			"vimeo":        {},
			"is_logged_in": {},
		}
	case "twitch":
		return map[string]struct{}{
			"auth-token": {},
		}
	case "niconico":
		return map[string]struct{}{
			"user_session": {},
		}
	default:
		return nil
	}
}

func nearestCookieExpiresAt(cookies []appcookies.Record, now time.Time, names map[string]struct{}) (time.Time, bool) {
	var nearest time.Time
	var found bool
	for _, record := range cookies {
		if !cookieNameAllowed(record.Name, names) {
			continue
		}
		if record.Expires <= 0 {
			continue
		}
		candidate := time.Unix(record.Expires, 0).UTC()
		if !candidate.After(now) {
			continue
		}
		if !found || candidate.Before(nearest) {
			nearest = candidate
			found = true
		}
	}
	return nearest, found
}

func cookieNameAllowed(value string, names map[string]struct{}) bool {
	normalized := strings.ToLower(strings.TrimSpace(value))
	if normalized == "" {
		return false
	}
	for name := range names {
		if strings.ToLower(strings.TrimSpace(name)) == normalized {
			return true
		}
	}
	return false
}

func mapCookiesDTO(cookies []appcookies.Record) []dto.AppSessionCookie {
	if len(cookies) == 0 {
		return nil
	}
	result := make([]dto.AppSessionCookie, 0, len(cookies))
	for _, cookie := range cookies {
		result = append(result, dto.AppSessionCookie{
			Name:     cookie.Name,
			Value:    cookie.Value,
			Domain:   cookie.Domain,
			Path:     cookie.Path,
			Expires:  cookie.Expires,
			HttpOnly: cookie.HttpOnly,
			Secure:   cookie.Secure,
			SameSite: cookie.SameSite,
		})
	}
	return result
}

func cookieDomains(cookies []appcookies.Record) []string {
	seen := map[string]struct{}{}
	result := make([]string, 0)
	for _, cookie := range cookies {
		domain := strings.TrimSpace(cookie.Domain)
		if domain == "" {
			continue
		}
		key := strings.ToLower(domain)
		if _, ok := seen[key]; ok {
			continue
		}
		seen[key] = struct{}{}
		result = append(result, domain)
	}
	return result
}
