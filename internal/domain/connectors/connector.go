package connectors

import (
	"strings"
	"time"
)

type ConnectorType string

const (
	ConnectorYouTube      ConnectorType = "youtube"
	ConnectorBilibili     ConnectorType = "bilibili"
	ConnectorTikTok       ConnectorType = "tiktok"
	ConnectorChinaPrivate ConnectorType = "china_private"
	ConnectorInstagram    ConnectorType = "instagram"
	ConnectorX            ConnectorType = "x"
	ConnectorFacebook     ConnectorType = "facebook"
	ConnectorVimeo        ConnectorType = "vimeo"
	ConnectorTwitch       ConnectorType = "twitch"
	ConnectorNiconico     ConnectorType = "niconico"
)

type ConnectorStatus string

const (
	StatusDisconnected ConnectorStatus = "disconnected"
	StatusConnected    ConnectorStatus = "connected"
	StatusExpired      ConnectorStatus = "expired"
)

type CredentialMode string

const (
	CredentialModeCookies CredentialMode = "cookies"
	CredentialModeProfile CredentialMode = "profile"
)

type Connector struct {
	ID             string
	Type           ConnectorType
	Status         ConnectorStatus
	CredentialMode CredentialMode
	CookiesPath    string
	CookiesJSON    string
	ProfileKey     string
	ProfilePath    string
	ProfileBrowser string
	LastVerifiedAt *time.Time
	CreatedAt      time.Time
	UpdatedAt      time.Time
}

type ConnectorParams struct {
	ID             string
	Type           string
	Status         string
	CredentialMode string
	CookiesPath    string
	CookiesJSON    string
	ProfileKey     string
	ProfilePath    string
	ProfileBrowser string
	LastVerifiedAt *time.Time
	CreatedAt      *time.Time
	UpdatedAt      *time.Time
}

func NewConnector(params ConnectorParams) (Connector, error) {
	id := strings.TrimSpace(params.ID)
	if id == "" {
		return Connector{}, ErrInvalidConnector
	}
	connectorType := ConnectorType(strings.TrimSpace(params.Type))
	if connectorType == "" {
		return Connector{}, ErrInvalidConnector
	}
	status := ConnectorStatus(strings.TrimSpace(params.Status))
	if status == "" {
		status = StatusDisconnected
	}
	credentialMode := CredentialMode(strings.TrimSpace(params.CredentialMode))
	if credentialMode == "" {
		credentialMode = DefaultCredentialMode(connectorType)
	}
	if credentialMode != CredentialModeCookies && credentialMode != CredentialModeProfile {
		return Connector{}, ErrInvalidConnector
	}
	profileKey := strings.TrimSpace(params.ProfileKey)
	if credentialMode == CredentialModeProfile && profileKey == "" {
		profileKey = string(connectorType)
	}

	createdAt := time.Now()
	updatedAt := createdAt
	if params.CreatedAt != nil {
		createdAt = *params.CreatedAt
	}
	if params.UpdatedAt != nil {
		updatedAt = *params.UpdatedAt
	}

	return Connector{
		ID:             id,
		Type:           connectorType,
		Status:         status,
		CredentialMode: credentialMode,
		CookiesPath:    strings.TrimSpace(params.CookiesPath),
		CookiesJSON:    strings.TrimSpace(params.CookiesJSON),
		ProfileKey:     profileKey,
		ProfilePath:    strings.TrimSpace(params.ProfilePath),
		ProfileBrowser: strings.TrimSpace(params.ProfileBrowser),
		LastVerifiedAt: params.LastVerifiedAt,
		CreatedAt:      createdAt,
		UpdatedAt:      updatedAt,
	}, nil
}

func DefaultCredentialMode(connectorType ConnectorType) CredentialMode {
	switch connectorType {
	case ConnectorChinaPrivate:
		return CredentialModeProfile
	default:
		return CredentialModeCookies
	}
}
