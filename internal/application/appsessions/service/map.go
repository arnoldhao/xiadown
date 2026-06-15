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

func (service *AppSessionsService) mapSessionDTO(ctx context.Context, session appsessions.Session) dto.AppSession {
	cookies := service.storedCookies(ctx, session.SiteKey)
	status := session.Status
	if status == "" {
		status = appsessions.StatusDisconnected
	}
	if len(cookies) == 0 {
		status = appsessions.StatusDisconnected
	} else if status == appsessions.StatusDisconnected {
		status = appsessions.StatusConnected
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
		CredentialState:              appSessionCredentialState(status, len(cookies)),
		CookiesCount:                 len(cookies),
		Cookies:                      mapCookiesDTO(cookies),
		Domains:                      append([]string(nil), policy.Domains...),
		Account:                      accountDTO(session, cookies),
		PolicyKey:                    policy.Key,
		Capabilities:                 append([]string(nil), policy.Capabilities...),
		ProviderSupported:            service.provider != nil && service.provider.AppSessionsSupported(),
		AccountVerificationStatus:    string(appSessionAccountVerificationStatus(session, len(cookies))),
		AccountVerificationError:     session.AccountVerificationError,
		AccountVerificationStartedAt: verificationStartedAt,
		LastVerifiedAt:               lastVerified,
	}
}

func (service *AppSessionsService) mapSessionDTOWithCookies(session appsessions.Session, cookies []appcookies.Record) dto.AppSession {
	status := session.Status
	if status == "" {
		status = appsessions.StatusDisconnected
	}
	if len(cookies) == 0 {
		status = appsessions.StatusDisconnected
	} else if status == appsessions.StatusDisconnected {
		status = appsessions.StatusConnected
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
		CredentialState:              appSessionCredentialState(status, len(cookies)),
		CookiesCount:                 len(cookies),
		Cookies:                      mapCookiesDTO(cookies),
		Domains:                      append([]string(nil), policy.Domains...),
		Account:                      accountDTO(session, cookies),
		PolicyKey:                    policy.Key,
		Capabilities:                 append([]string(nil), policy.Capabilities...),
		ProviderSupported:            service.provider != nil && service.provider.AppSessionsSupported(),
		AccountVerificationStatus:    string(appSessionAccountVerificationStatus(session, len(cookies))),
		AccountVerificationError:     session.AccountVerificationError,
		AccountVerificationStartedAt: verificationStartedAt,
		LastVerifiedAt:               lastVerified,
	}
}

func appSessionCredentialState(status appsessions.Status, cookiesCount int) string {
	if status == appsessions.StatusConnected && cookiesCount > 0 {
		return "app_session"
	}
	return string(appsessions.StatusDisconnected)
}

func appSessionAccountVerificationStatus(session appsessions.Session, cookiesCount int) appsessions.AccountVerificationStatus {
	if cookiesCount <= 0 {
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
	expiresAt := cookieExpiresAt(session.SiteKey, cookies, time.Now())
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

func cookieExpiresAt(siteKey string, cookies []appcookies.Record, now time.Time) string {
	if names := authCookieExpiryNames(siteKey); len(names) > 0 {
		if expiresAt, ok := nearestCookieExpiresAt(cookies, now, names); ok {
			return expiresAt.Format(time.RFC3339)
		}
	}
	if expiresAt, ok := latestCookieExpiresAt(cookies, now); ok {
		return expiresAt.Format(time.RFC3339)
	}
	return ""
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
	default:
		return nil
	}
}

func nearestCookieExpiresAt(cookies []appcookies.Record, now time.Time, names map[string]struct{}) (time.Time, bool) {
	var nearest time.Time
	var found bool
	for _, record := range cookies {
		if _, ok := names[record.Name]; !ok {
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

func latestCookieExpiresAt(cookies []appcookies.Record, now time.Time) (time.Time, bool) {
	var latest time.Time
	var found bool
	for _, record := range cookies {
		if record.Expires <= 0 {
			continue
		}
		candidate := time.Unix(record.Expires, 0).UTC()
		if !candidate.After(now) {
			continue
		}
		if !found || candidate.After(latest) {
			latest = candidate
			found = true
		}
	}
	return latest, found
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
