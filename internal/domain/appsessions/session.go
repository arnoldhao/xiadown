package appsessions

import (
	"context"
	"strings"
	"time"
)

type Status string
type AccountVerificationStatus string
type SourceType string

const (
	StatusDisconnected Status = "disconnected"
	StatusConnected    Status = "connected"
	StatusExpired      Status = "expired"
)

const (
	SourceTypeXiaDownProfile SourceType = "xiadown_profile"
	SourceTypeBrowserProfile SourceType = "browser_profile"
)

const (
	AccountVerificationUnverified  AccountVerificationStatus = "unverified"
	AccountVerificationVerifying   AccountVerificationStatus = "verifying"
	AccountVerificationVerified    AccountVerificationStatus = "verified"
	AccountVerificationUnsupported AccountVerificationStatus = "unsupported"
)

type Session struct {
	ID                           string
	SiteKey                      string
	Status                       Status
	AccountDisplayName           string
	AccountHandle                string
	AccountAvatarURL             string
	AccountTierKey               string
	AccountTierLabel             string
	AccountBadgesJSON            string
	AccountMetadataJSON          string
	AccountVerificationStatus    AccountVerificationStatus
	AccountVerificationError     string
	AccountVerificationStartedAt *time.Time
	LastVerifiedAt               *time.Time
	SourceType                   SourceType
	SourceBrowser                string
	SourceProfile                string
	LastSyncedAt                 *time.Time
	CreatedAt                    time.Time
	UpdatedAt                    time.Time
}

type SessionParams struct {
	ID                           string
	SiteKey                      string
	Status                       string
	AccountDisplayName           string
	AccountHandle                string
	AccountAvatarURL             string
	AccountTierKey               string
	AccountTierLabel             string
	AccountBadgesJSON            string
	AccountMetadataJSON          string
	AccountVerificationStatus    string
	AccountVerificationError     string
	AccountVerificationStartedAt *time.Time
	LastVerifiedAt               *time.Time
	SourceType                   string
	SourceBrowser                string
	SourceProfile                string
	LastSyncedAt                 *time.Time
	CreatedAt                    *time.Time
	UpdatedAt                    *time.Time
}

func NewSession(params SessionParams) (Session, error) {
	id := strings.TrimSpace(params.ID)
	if id == "" {
		return Session{}, ErrInvalidSession
	}
	siteKey := strings.TrimSpace(params.SiteKey)
	if siteKey == "" {
		return Session{}, ErrInvalidSession
	}
	status := Status(strings.TrimSpace(params.Status))
	if status == "" {
		status = StatusDisconnected
	}
	verificationStatus := normalizeAccountVerificationStatus(params.AccountVerificationStatus)
	if strings.TrimSpace(params.AccountVerificationStatus) == "" && params.LastVerifiedAt != nil && !params.LastVerifiedAt.IsZero() {
		verificationStatus = AccountVerificationVerified
	}

	createdAt := time.Now()
	updatedAt := createdAt
	if params.CreatedAt != nil {
		createdAt = *params.CreatedAt
	}
	if params.UpdatedAt != nil {
		updatedAt = *params.UpdatedAt
	}

	return Session{
		ID:                           id,
		SiteKey:                      siteKey,
		Status:                       status,
		AccountDisplayName:           strings.TrimSpace(params.AccountDisplayName),
		AccountHandle:                strings.TrimSpace(params.AccountHandle),
		AccountAvatarURL:             strings.TrimSpace(params.AccountAvatarURL),
		AccountTierKey:               strings.TrimSpace(params.AccountTierKey),
		AccountTierLabel:             strings.TrimSpace(params.AccountTierLabel),
		AccountBadgesJSON:            strings.TrimSpace(params.AccountBadgesJSON),
		AccountMetadataJSON:          strings.TrimSpace(params.AccountMetadataJSON),
		AccountVerificationStatus:    verificationStatus,
		AccountVerificationError:     strings.TrimSpace(params.AccountVerificationError),
		AccountVerificationStartedAt: params.AccountVerificationStartedAt,
		LastVerifiedAt:               params.LastVerifiedAt,
		SourceType:                   normalizeSourceType(params.SourceType),
		SourceBrowser:                strings.ToLower(strings.TrimSpace(params.SourceBrowser)),
		SourceProfile:                strings.TrimSpace(params.SourceProfile),
		LastSyncedAt:                 params.LastSyncedAt,
		CreatedAt:                    createdAt,
		UpdatedAt:                    updatedAt,
	}, nil
}

func normalizeSourceType(value string) SourceType {
	switch SourceType(strings.TrimSpace(value)) {
	case SourceTypeXiaDownProfile:
		return SourceTypeXiaDownProfile
	case SourceTypeBrowserProfile:
		return SourceTypeBrowserProfile
	default:
		return ""
	}
}

func normalizeAccountVerificationStatus(value string) AccountVerificationStatus {
	switch AccountVerificationStatus(strings.TrimSpace(value)) {
	case AccountVerificationVerifying:
		return AccountVerificationVerifying
	case AccountVerificationVerified:
		return AccountVerificationVerified
	case AccountVerificationUnsupported:
		return AccountVerificationUnsupported
	default:
		return AccountVerificationUnverified
	}
}

type Repository interface {
	List(ctx context.Context) ([]Session, error)
	Get(ctx context.Context, id string) (Session, error)
	GetBySiteKey(ctx context.Context, siteKey string) (Session, error)
	Save(ctx context.Context, session Session) error
	Delete(ctx context.Context, id string) error
}
