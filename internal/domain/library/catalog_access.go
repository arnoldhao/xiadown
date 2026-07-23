package library

import (
	"sort"
	"time"
)

type DeviceScope string

const (
	DeviceScopeLibraryRead  DeviceScope = "library.read"
	DeviceScopeMusicRead    DeviceScope = "music.read"
	DeviceScopeMusicState   DeviceScope = "music.state"
	DeviceScopeMusicManage  DeviceScope = "music.manage"
	DeviceScopeRSSRead      DeviceScope = "rss.read"
	DeviceScopeRSSState     DeviceScope = "rss.state"
	DeviceScopeRSSManage    DeviceScope = "rss.manage"
	DeviceScopeRSSFetch     DeviceScope = "rss.fetch"
	DeviceScopeTasksRead    DeviceScope = "tasks.read"
	DeviceScopeTasksControl DeviceScope = "tasks.control"
	DeviceScopeTasksCreate  DeviceScope = "tasks.create"
)

type DeviceGrantStatus string

const (
	DeviceGrantStatusActive  DeviceGrantStatus = "active"
	DeviceGrantStatusRevoked DeviceGrantStatus = "revoked"
)

// DeviceGrant stores only a credential hash. Raw pairing credentials must
// never enter the catalog or its change feed.
type DeviceGrant struct {
	ID             string
	CatalogID      string
	DeviceID       string
	DeviceName     string
	CredentialHash string
	PublicKeyHash  string
	Scopes         []DeviceScope
	Status         DeviceGrantStatus
	ExpiresAt      *time.Time
	LastSeenAt     *time.Time
	RevokedAt      *time.Time
	Revision       int64
	CreatedAt      time.Time
	UpdatedAt      time.Time
}

type DeviceGrantParams struct {
	ID             string
	CatalogID      string
	DeviceID       string
	DeviceName     string
	CredentialHash string
	PublicKeyHash  string
	Scopes         []string
	Status         string
	ExpiresAt      *time.Time
	LastSeenAt     *time.Time
	RevokedAt      *time.Time
	Revision       int64
	CreatedAt      *time.Time
	UpdatedAt      *time.Time
}

func NewDeviceGrant(params DeviceGrantParams) (DeviceGrant, error) {
	id, idOK := normalizeCatalogID(params.ID)
	catalogID, catalogIDOK := normalizeCatalogID(params.CatalogID)
	deviceID, deviceIDOK := normalizeCatalogID(params.DeviceID)
	deviceName, deviceNameOK := normalizeCatalogName(params.DeviceName)
	credentialHash, credentialHashOK := normalizeCatalogID(params.CredentialHash)
	publicKeyHash, publicKeyHashOK := normalizeCatalogID(params.PublicKeyHash)
	status := DeviceGrantStatus(normalizeCatalogEnum(params.Status))
	if status == "" {
		status = DeviceGrantStatusActive
	}
	scopes, scopesOK := normalizeDeviceScopes(params.Scopes)
	createdAt, updatedAt, timesOK := normalizeCatalogTimes(params.CreatedAt, params.UpdatedAt)
	expiresAt := normalizeOptionalCatalogTime(params.ExpiresAt)
	lastSeenAt := normalizeOptionalCatalogTime(params.LastSeenAt)
	revokedAt := normalizeOptionalCatalogTime(params.RevokedAt)
	revision := params.Revision
	if revision == 0 {
		revision = 1
	}
	if !idOK || !catalogIDOK || !deviceIDOK || !deviceNameOK || !credentialHashOK ||
		!publicKeyHashOK || !scopesOK || !timesOK || revision < 1 {
		return DeviceGrant{}, ErrInvalidDeviceGrant
	}
	if expiresAt != nil && !expiresAt.After(createdAt) {
		return DeviceGrant{}, ErrInvalidDeviceGrant
	}
	switch status {
	case DeviceGrantStatusActive:
		if revokedAt != nil {
			return DeviceGrant{}, ErrInvalidDeviceGrant
		}
	case DeviceGrantStatusRevoked:
		if revokedAt == nil || revokedAt.Before(createdAt) || revokedAt.After(updatedAt) {
			return DeviceGrant{}, ErrInvalidDeviceGrant
		}
	default:
		return DeviceGrant{}, ErrInvalidDeviceGrant
	}
	return DeviceGrant{
		ID: id, CatalogID: catalogID, DeviceID: deviceID, DeviceName: deviceName,
		CredentialHash: credentialHash, PublicKeyHash: publicKeyHash, Scopes: scopes,
		Status: status, ExpiresAt: expiresAt, LastSeenAt: lastSeenAt, RevokedAt: revokedAt,
		Revision:  revision,
		CreatedAt: createdAt, UpdatedAt: updatedAt,
	}, nil
}

func (grant DeviceGrant) HasScope(scope DeviceScope) bool {
	index := sort.Search(len(grant.Scopes), func(index int) bool { return grant.Scopes[index] >= scope })
	return index < len(grant.Scopes) && grant.Scopes[index] == scope
}

func (grant DeviceGrant) IsEffective(at time.Time) bool {
	if grant.Status != DeviceGrantStatusActive {
		return false
	}
	if at.IsZero() {
		at = time.Now().UTC()
	}
	return grant.ExpiresAt == nil || at.UTC().Before(*grant.ExpiresAt)
}

func normalizeDeviceScopes(values []string) ([]DeviceScope, bool) {
	unique := make(map[DeviceScope]struct{}, len(values))
	for _, value := range values {
		scope := DeviceScope(normalizeCatalogEnum(value))
		switch scope {
		case DeviceScopeLibraryRead, DeviceScopeMusicRead, DeviceScopeMusicState, DeviceScopeMusicManage,
			DeviceScopeRSSRead, DeviceScopeRSSState, DeviceScopeRSSManage, DeviceScopeRSSFetch,
			DeviceScopeTasksRead,
			DeviceScopeTasksControl, DeviceScopeTasksCreate:
			unique[scope] = struct{}{}
		default:
			return nil, false
		}
	}
	if len(unique) == 0 {
		return nil, false
	}
	result := make([]DeviceScope, 0, len(unique))
	for scope := range unique {
		result = append(result, scope)
	}
	sort.Slice(result, func(left, right int) bool { return result[left] < result[right] })
	return result, true
}
