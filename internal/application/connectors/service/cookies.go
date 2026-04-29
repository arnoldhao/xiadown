package service

import (
	"context"
	"time"

	"xiadown/internal/application/connectors/dto"
	appcookies "xiadown/internal/application/cookies"
	"xiadown/internal/application/sitepolicy"
	"xiadown/internal/domain/connectors"
)

func mapConnectorDTO(item connectors.Connector) dto.Connector {
	cookies := decodeCookies(item.CookiesJSON)
	status := item.Status
	if len(cookies) == 0 {
		status = connectors.StatusDisconnected
	} else if status == "" || status == connectors.StatusDisconnected {
		status = connectors.StatusConnected
	}
	lastVerified := ""
	if item.LastVerifiedAt != nil {
		lastVerified = item.LastVerifiedAt.Format(time.RFC3339)
	}
	policy, _ := sitepolicy.ForConnectorType(string(item.Type))
	return dto.Connector{
		ID:             item.ID,
		Type:           string(item.Type),
		Group:          connectorGroup(item.Type),
		Desc:           connectorDesc(item.Type),
		Status:         string(status),
		CookiesCount:   len(cookies),
		Cookies:        mapCookiesDTO(cookies),
		Domains:        append([]string(nil), policy.Domains...),
		PolicyKey:      policy.Key,
		Capabilities:   append([]string(nil), policy.Capabilities...),
		LastVerifiedAt: lastVerified,
	}
}

func (service *ConnectorsService) CookiesForConnectorType(ctx context.Context, connectorType connectors.ConnectorType) ([]appcookies.Record, error) {
	if service == nil {
		return nil, connectors.ErrConnectorNotFound
	}
	items, err := service.repo.List(ctx)
	if err != nil {
		return nil, err
	}
	for _, item := range items {
		if item.Type != connectorType {
			continue
		}
		records := decodeCookies(item.CookiesJSON)
		if len(records) == 0 {
			return nil, connectors.ErrNoCookies
		}
		return records, nil
	}
	return nil, connectors.ErrConnectorNotFound
}

func connectorGroup(connectorType connectors.ConnectorType) string {
	switch connectorType {
	case connectors.ConnectorYouTube,
		connectors.ConnectorBilibili,
		connectors.ConnectorTikTok,
		connectors.ConnectorDouyin,
		connectors.ConnectorInstagram,
		connectors.ConnectorX,
		connectors.ConnectorFacebook,
		connectors.ConnectorVimeo,
		connectors.ConnectorTwitch,
		connectors.ConnectorNiconico:
		return "video"
	default:
		return "other"
	}
}

func connectorDesc(connectorType connectors.ConnectorType) string {
	switch connectorType {
	case connectors.ConnectorYouTube:
		return "YouTube videos, playlists, live streams, age-restricted videos, and member-only content when cookies are available."
	case connectors.ConnectorBilibili:
		return "Bilibili videos, bangumi, playlists, higher-quality formats, and login-gated content when cookies are available."
	case connectors.ConnectorTikTok:
		return "TikTok videos and creator pages that often need browser cookies for region, login, or anti-bot checks."
	case connectors.ConnectorDouyin:
		return "Douyin videos and creator pages that often need browser cookies for login, region, or anti-bot checks."
	case connectors.ConnectorInstagram:
		return "Instagram reels, posts, stories, and private or login-gated media when cookies are available."
	case connectors.ConnectorX:
		return "X/Twitter videos, broadcasts, spaces, and posts that may need cookies for login-gated content."
	case connectors.ConnectorFacebook:
		return "Facebook videos, reels, and watch pages that commonly require a logged-in browser session."
	case connectors.ConnectorVimeo:
		return "Vimeo videos and private or account-restricted pages when cookies are available."
	case connectors.ConnectorTwitch:
		return "Twitch streams, VODs, clips, and subscriber or mature content when cookies are available."
	case connectors.ConnectorNiconico:
		return "Niconico videos and account-restricted Japanese media when cookies are available."
	default:
		return ""
	}
}

func mapCookiesDTO(records []appcookies.Record) []dto.ConnectorCookie {
	if len(records) == 0 {
		return nil
	}
	result := make([]dto.ConnectorCookie, 0, len(records))
	for _, record := range records {
		result = append(result, dto.ConnectorCookie{
			Name:     record.Name,
			Value:    record.Value,
			Domain:   record.Domain,
			Path:     record.Path,
			Expires:  record.Expires,
			HttpOnly: record.HttpOnly,
			Secure:   record.Secure,
			SameSite: record.SameSite,
		})
	}
	return result
}

func encodeCookies(records []appcookies.Record) (string, error) {
	return appcookies.EncodeJSON(records)
}

func decodeCookies(data string) []appcookies.Record {
	return appcookies.DecodeJSON(data)
}
