package connectors

import (
	"strings"
	"time"
)

type ConnectorType string

const (
	ConnectorYouTube   ConnectorType = "youtube"
	ConnectorBilibili  ConnectorType = "bilibili"
	ConnectorTikTok    ConnectorType = "tiktok"
	ConnectorDouyin    ConnectorType = "douyin"
	ConnectorInstagram ConnectorType = "instagram"
	ConnectorX         ConnectorType = "x"
	ConnectorFacebook  ConnectorType = "facebook"
	ConnectorVimeo     ConnectorType = "vimeo"
	ConnectorTwitch    ConnectorType = "twitch"
	ConnectorNiconico  ConnectorType = "niconico"
)

type ConnectorStatus string

const (
	StatusDisconnected ConnectorStatus = "disconnected"
	StatusConnected    ConnectorStatus = "connected"
	StatusExpired      ConnectorStatus = "expired"
)

type Connector struct {
	ID             string
	Type           ConnectorType
	Status         ConnectorStatus
	CookiesPath    string
	CookiesJSON    string
	LastVerifiedAt *time.Time
	CreatedAt      time.Time
	UpdatedAt      time.Time
}

type ConnectorParams struct {
	ID             string
	Type           string
	Status         string
	CookiesPath    string
	CookiesJSON    string
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
		CookiesPath:    strings.TrimSpace(params.CookiesPath),
		CookiesJSON:    strings.TrimSpace(params.CookiesJSON),
		LastVerifiedAt: params.LastVerifiedAt,
		CreatedAt:      createdAt,
		UpdatedAt:      updatedAt,
	}, nil
}
