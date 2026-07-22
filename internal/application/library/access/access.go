// Package access owns pairing credentials and authorization for the isolated
// Library public API. It deliberately does not know about the desktop HTTP/WS
// router, filesystem paths, or transport concerns.
package access

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"database/sql"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"

	"xiadown/internal/domain/library"
)

const (
	defaultPairingTTL  = 5 * time.Minute
	maxPairingFailures = 5
	tokenPrefix        = "xd1"
)

var (
	ErrUnauthorized   = errors.New("library access unauthorized")
	ErrPairingInvalid = errors.New("library pairing session is invalid or expired")
	ErrInvalidRequest = errors.New("library pairing request is invalid")
	ErrGrantNotFound  = errors.New("library device grant not found")
	ErrGrantRevoked   = errors.New("library device grant is revoked")
)

type Clock func() time.Time
type IDGenerator func() string

type Options struct {
	Clock       Clock
	Random      io.Reader
	IDGenerator IDGenerator
	PairingTTL  time.Duration
}

type PairingSession struct {
	Nonce     string    `json:"nonce"`
	Code      string    `json:"code"`
	ExpiresAt time.Time `json:"expiresAt"`
}

type PairRequest struct {
	Nonce         string
	Code          string
	DeviceID      string
	DeviceName    string
	PublicKeyHash string
}

type PairResult struct {
	GrantID string
	Token   string
	Scopes  []library.DeviceScope
}

type Principal struct {
	GrantID   string
	CatalogID string
	DeviceID  string
	Scopes    []library.DeviceScope
}

// DeviceGrantMetadata is the safe administration view. Credential and public
// key hashes are deliberately absent so no Wails caller can retrieve them.
type DeviceGrantMetadata struct {
	GrantID    string                    `json:"grantId"`
	DeviceID   string                    `json:"deviceId"`
	DeviceName string                    `json:"deviceName"`
	Scopes     []library.DeviceScope     `json:"scopes"`
	Status     library.DeviceGrantStatus `json:"status"`
	ExpiresAt  *time.Time                `json:"expiresAt,omitempty"`
	LastSeenAt *time.Time                `json:"lastSeenAt,omitempty"`
	RevokedAt  *time.Time                `json:"revokedAt,omitempty"`
	Revision   int64                     `json:"revision"`
	CreatedAt  time.Time                 `json:"createdAt"`
	UpdatedAt  time.Time                 `json:"updatedAt"`
}

type UpdateScopesRequest struct {
	GrantID          string
	ExpectedRevision int64
	Scopes           []string
	ActorID          string
}

type RevokeGrantRequest struct {
	GrantID          string
	ExpectedRevision int64
	ActorID          string
}

func (principal Principal) HasScope(scope library.DeviceScope) bool {
	for _, granted := range principal.Scopes {
		if granted == scope {
			return true
		}
	}
	return false
}

// Service keeps only short-lived pairing sessions in memory. Durable grants
// are delegated to DeviceGrantManagementRepository and contain SHA-256 hashes
// only.
type Service struct {
	repository library.DeviceGrantManagementRepository
	catalogID  string
	clock      Clock
	random     io.Reader
	newID      IDGenerator
	ttl        time.Duration

	mu       sync.Mutex
	sessions map[string]PairingSession
	failures map[string]int
}

func NewService(repository library.DeviceGrantManagementRepository, catalogID string, options Options) (*Service, error) {
	if repository == nil || strings.TrimSpace(catalogID) == "" {
		return nil, fmt.Errorf("library access requires a repository and catalog id")
	}
	clock := options.Clock
	if clock == nil {
		clock = func() time.Time { return time.Now().UTC() }
	}
	randomSource := options.Random
	if randomSource == nil {
		randomSource = rand.Reader
	}
	newID := options.IDGenerator
	if newID == nil {
		newID = uuid.NewString
	}
	ttl := options.PairingTTL
	if ttl == 0 {
		ttl = defaultPairingTTL
	}
	if ttl < 0 {
		return nil, fmt.Errorf("library access pairing ttl must be positive")
	}
	return &Service{
		repository: repository,
		catalogID:  strings.TrimSpace(catalogID),
		clock:      clock,
		random:     randomSource,
		newID:      newID,
		ttl:        ttl,
		sessions:   make(map[string]PairingSession),
		failures:   make(map[string]int),
	}, nil
}

func (service *Service) StartPairing() (PairingSession, error) {
	if service == nil {
		return PairingSession{}, ErrPairingInvalid
	}
	nonceBytes := make([]byte, 24)
	if _, err := io.ReadFull(service.random, nonceBytes); err != nil {
		return PairingSession{}, fmt.Errorf("generate pairing nonce: %w", err)
	}
	codeBytes := make([]byte, 4)
	if _, err := io.ReadFull(service.random, codeBytes); err != nil {
		return PairingSession{}, fmt.Errorf("generate pairing code: %w", err)
	}
	codeValue := uint32(codeBytes[0])<<24 | uint32(codeBytes[1])<<16 | uint32(codeBytes[2])<<8 | uint32(codeBytes[3])
	session := PairingSession{
		Nonce:     base64.RawURLEncoding.EncodeToString(nonceBytes),
		Code:      fmt.Sprintf("%06d", codeValue%1_000_000),
		ExpiresAt: service.now().Add(service.ttl),
	}
	service.mu.Lock()
	service.pruneExpiredLocked(service.now())
	service.sessions[session.Nonce] = session
	service.failures[session.Nonce] = 0
	service.mu.Unlock()
	return session, nil
}

func (service *Service) Pair(ctx context.Context, request PairRequest) (PairResult, error) {
	request.Nonce = strings.TrimSpace(request.Nonce)
	request.Code = strings.TrimSpace(request.Code)
	request.DeviceID = strings.TrimSpace(request.DeviceID)
	request.DeviceName = strings.TrimSpace(request.DeviceName)
	publicKeyHash, ok := normalizeSHA256(request.PublicKeyHash)
	if request.Nonce == "" || len(request.Code) != 6 || request.DeviceID == "" || request.DeviceName == "" || !ok {
		return PairResult{}, ErrInvalidRequest
	}

	session, ok := service.reserveSession(request.Nonce, request.Code)
	if !ok {
		return PairResult{}, ErrPairingInvalid
	}
	restore := true
	defer func() {
		if restore {
			service.restoreSession(session)
		}
	}()

	grantID := strings.TrimSpace(service.newID())
	createdAt := service.now()
	minimumUpdatedAt := createdAt
	expectedRevision := int64(0)
	revision := int64(1)
	scopes := []string{string(library.DeviceScopeLibraryRead), string(library.DeviceScopeTasksRead)}
	existing, err := service.repository.ListByCatalogID(ctx, service.catalogID)
	if err != nil {
		return PairResult{}, fmt.Errorf("list device grants: %w", err)
	}
	for _, grant := range existing {
		if grant.DeviceID == request.DeviceID {
			// DeviceID and PublicKeyHash are both client assertions. Protocol v1 has
			// no signature proving possession of the enrolled private key. An active
			// grant therefore remains collision-protected: a pairing code cannot
			// rotate its token or take over its managed scopes. A revoked grant is
			// different: the Desktop owner has already made its old credential
			// unusable, and the fresh short-lived pairing code explicitly authorizes
			// a replacement. Reuse only the durable identity/creation time required
			// by the schema and local namespace continuity; the new token, public key,
			// and least-privilege scopes below never inherit client assertions or the
			// revoked grant's former authority.
			if grant.Status != library.DeviceGrantStatusRevoked {
				return PairResult{}, ErrInvalidRequest
			}
			grantID = grant.ID
			createdAt = grant.CreatedAt
			minimumUpdatedAt = grant.UpdatedAt
			expectedRevision = grant.Revision
			revision = grant.Revision + 1
		}
	}
	if grantID == "" {
		return PairResult{}, fmt.Errorf("generate device grant id")
	}

	token, credentialHash, err := service.newToken(grantID)
	if err != nil {
		return PairResult{}, err
	}
	now := timestampAtLeast(service.now(), minimumUpdatedAt)
	grant, err := library.NewDeviceGrant(library.DeviceGrantParams{
		ID: grantID, CatalogID: service.catalogID, DeviceID: request.DeviceID, DeviceName: request.DeviceName,
		CredentialHash: credentialHash, PublicKeyHash: publicKeyHash, Scopes: scopes,
		Status: string(library.DeviceGrantStatusActive), Revision: revision,
		CreatedAt: &createdAt, UpdatedAt: &now,
	})
	if err != nil {
		return PairResult{}, fmt.Errorf("create device grant: %w", err)
	}
	if err := service.repository.SaveDeviceGrantMutation(
		ctx, grant, expectedRevision, library.CatalogChangeUpsert, "pairing",
	); err != nil {
		return PairResult{}, fmt.Errorf("save device grant: %w", err)
	}
	restore = false // A pairing session is consumed only after the grant is durable.
	return PairResult{GrantID: grant.ID, Token: token, Scopes: append([]library.DeviceScope(nil), grant.Scopes...)}, nil
}

func (service *Service) Authenticate(ctx context.Context, token string) (Principal, error) {
	token = strings.TrimSpace(token)
	grantID, ok := parseToken(token)
	if !ok {
		return Principal{}, ErrUnauthorized
	}
	grant, err := service.repository.Get(ctx, grantID)
	if err != nil || !grant.IsEffective(service.now()) {
		return Principal{}, ErrUnauthorized
	}
	expectedHash, ok := normalizeSHA256(grant.CredentialHash)
	if !ok {
		return Principal{}, ErrUnauthorized
	}
	actualHash := tokenSHA256(token)
	expectedBytes, _ := hex.DecodeString(expectedHash)
	actualBytes, _ := hex.DecodeString(actualHash)
	if subtle.ConstantTimeCompare(expectedBytes, actualBytes) != 1 {
		return Principal{}, ErrUnauthorized
	}
	// Last-used activity is metadata, not an authorization decision. Recording
	// it is best-effort so a transient telemetry write cannot turn a valid
	// credential into a misleading 401 response.
	_ = service.repository.RecordDeviceGrantLastSeen(ctx, grant.CatalogID, grant.ID, service.now())
	return Principal{
		GrantID: grant.ID, CatalogID: grant.CatalogID, DeviceID: grant.DeviceID,
		Scopes: append([]library.DeviceScope(nil), grant.Scopes...),
	}, nil
}

func (service *Service) ListDeviceGrants(ctx context.Context) ([]DeviceGrantMetadata, error) {
	if service == nil || service.repository == nil {
		return nil, ErrGrantNotFound
	}
	grants, err := service.repository.ListByCatalogID(ctx, service.catalogID)
	if err != nil {
		return nil, fmt.Errorf("list library device grants: %w", err)
	}
	result := make([]DeviceGrantMetadata, 0, len(grants))
	for _, grant := range grants {
		result = append(result, deviceGrantMetadata(grant))
	}
	return result, nil
}

func (service *Service) UpdateDeviceGrantScopes(ctx context.Context, request UpdateScopesRequest) (DeviceGrantMetadata, error) {
	request.GrantID = strings.TrimSpace(request.GrantID)
	request.ActorID = strings.TrimSpace(request.ActorID)
	scopes, ok := normalizeManagedScopes(request.Scopes)
	if request.GrantID == "" || request.ExpectedRevision <= 0 || request.ActorID == "" || !ok {
		return DeviceGrantMetadata{}, ErrInvalidRequest
	}
	grant, err := service.loadManagedGrant(ctx, request.GrantID, request.ExpectedRevision)
	if err != nil {
		return DeviceGrantMetadata{}, err
	}
	if grant.Status != library.DeviceGrantStatusActive {
		return DeviceGrantMetadata{}, ErrGrantRevoked
	}
	grant.Scopes = scopes
	grant.Revision++
	grant.UpdatedAt = timestampAtLeast(service.now(), grant.UpdatedAt)
	if err := service.repository.SaveDeviceGrantMutation(
		ctx, grant, request.ExpectedRevision, library.CatalogChangeUpsert, request.ActorID,
	); err != nil {
		return DeviceGrantMetadata{}, fmt.Errorf("update library device grant scopes: %w", err)
	}
	return deviceGrantMetadata(grant), nil
}

func (service *Service) RevokeDeviceGrant(ctx context.Context, request RevokeGrantRequest) (DeviceGrantMetadata, error) {
	request.GrantID = strings.TrimSpace(request.GrantID)
	request.ActorID = strings.TrimSpace(request.ActorID)
	if request.GrantID == "" || request.ExpectedRevision <= 0 || request.ActorID == "" {
		return DeviceGrantMetadata{}, ErrInvalidRequest
	}
	grant, err := service.loadManagedGrant(ctx, request.GrantID, request.ExpectedRevision)
	if err != nil {
		return DeviceGrantMetadata{}, err
	}
	if grant.Status != library.DeviceGrantStatusActive {
		return DeviceGrantMetadata{}, ErrGrantRevoked
	}
	revokedAt := timestampAtLeast(service.now(), grant.UpdatedAt)
	grant.Status = library.DeviceGrantStatusRevoked
	grant.RevokedAt = &revokedAt
	grant.Revision++
	grant.UpdatedAt = revokedAt
	if err := service.repository.SaveDeviceGrantMutation(
		ctx, grant, request.ExpectedRevision, library.CatalogChangeDelete, request.ActorID,
	); err != nil {
		return DeviceGrantMetadata{}, fmt.Errorf("revoke library device grant: %w", err)
	}
	return deviceGrantMetadata(grant), nil
}

func (service *Service) loadManagedGrant(ctx context.Context, id string, expectedRevision int64) (library.DeviceGrant, error) {
	grant, err := service.repository.Get(ctx, id)
	if errors.Is(err, sql.ErrNoRows) || (err == nil && grant.CatalogID != service.catalogID) {
		return library.DeviceGrant{}, ErrGrantNotFound
	}
	if err != nil {
		return library.DeviceGrant{}, fmt.Errorf("load library device grant: %w", err)
	}
	if grant.Revision != expectedRevision {
		return library.DeviceGrant{}, library.ErrCatalogRevisionConflict
	}
	return grant, nil
}

func normalizeManagedScopes(values []string) ([]library.DeviceScope, bool) {
	unique := make(map[library.DeviceScope]struct{}, len(values))
	for _, value := range values {
		scope := library.DeviceScope(strings.ToLower(strings.TrimSpace(value)))
		switch scope {
		case library.DeviceScopeLibraryRead,
			library.DeviceScopeMusicRead, library.DeviceScopeMusicState, library.DeviceScopeMusicManage,
			library.DeviceScopeRSSRead, library.DeviceScopeRSSState,
			library.DeviceScopeRSSManage, library.DeviceScopeRSSFetch,
			library.DeviceScopeTasksRead,
			library.DeviceScopeTasksCreate, library.DeviceScopeTasksControl:
			unique[scope] = struct{}{}
		default:
			return nil, false
		}
	}
	if len(unique) == 0 {
		return nil, false
	}
	ordered := []library.DeviceScope{
		library.DeviceScopeLibraryRead,
		library.DeviceScopeMusicRead, library.DeviceScopeMusicState, library.DeviceScopeMusicManage,
		library.DeviceScopeRSSRead, library.DeviceScopeRSSState,
		library.DeviceScopeRSSManage, library.DeviceScopeRSSFetch,
		library.DeviceScopeTasksControl,
		library.DeviceScopeTasksCreate, library.DeviceScopeTasksRead,
	}
	result := make([]library.DeviceScope, 0, len(unique))
	for _, scope := range ordered {
		if _, exists := unique[scope]; exists {
			result = append(result, scope)
		}
	}
	return result, true
}

func deviceGrantMetadata(grant library.DeviceGrant) DeviceGrantMetadata {
	return DeviceGrantMetadata{
		GrantID: grant.ID, DeviceID: grant.DeviceID, DeviceName: grant.DeviceName,
		Scopes: append([]library.DeviceScope(nil), grant.Scopes...), Status: grant.Status,
		ExpiresAt: grant.ExpiresAt, LastSeenAt: grant.LastSeenAt, RevokedAt: grant.RevokedAt,
		Revision: grant.Revision, CreatedAt: grant.CreatedAt, UpdatedAt: grant.UpdatedAt,
	}
}

func timestampAtLeast(candidate, minimum time.Time) time.Time {
	candidate = candidate.UTC()
	if candidate.Before(minimum) {
		return minimum.UTC()
	}
	return candidate
}

func (service *Service) reserveSession(nonce, code string) (PairingSession, bool) {
	now := service.now()
	service.mu.Lock()
	defer service.mu.Unlock()
	service.pruneExpiredLocked(now)
	session, ok := service.sessions[nonce]
	if !ok {
		return PairingSession{}, false
	}
	if subtle.ConstantTimeCompare([]byte(session.Code), []byte(code)) != 1 {
		service.failures[nonce]++
		if service.failures[nonce] >= maxPairingFailures {
			delete(service.sessions, nonce)
			delete(service.failures, nonce)
		}
		return PairingSession{}, false
	}
	delete(service.sessions, nonce)
	delete(service.failures, nonce)
	return session, true
}

func (service *Service) restoreSession(session PairingSession) {
	if !session.ExpiresAt.After(service.now()) {
		return
	}
	service.mu.Lock()
	if _, exists := service.sessions[session.Nonce]; !exists {
		service.sessions[session.Nonce] = session
		service.failures[session.Nonce] = 0
	}
	service.mu.Unlock()
}

func (service *Service) pruneExpiredLocked(now time.Time) {
	for nonce, session := range service.sessions {
		if !session.ExpiresAt.After(now) {
			delete(service.sessions, nonce)
			delete(service.failures, nonce)
		}
	}
}

func (service *Service) newToken(grantID string) (string, string, error) {
	secret := make([]byte, 32)
	if _, err := io.ReadFull(service.random, secret); err != nil {
		return "", "", fmt.Errorf("generate device credential: %w", err)
	}
	encodedID := base64.RawURLEncoding.EncodeToString([]byte(grantID))
	token := tokenPrefix + "." + encodedID + "." + base64.RawURLEncoding.EncodeToString(secret)
	return token, tokenSHA256(token), nil
}

func parseToken(token string) (string, bool) {
	parts := strings.Split(token, ".")
	if len(parts) != 3 || parts[0] != tokenPrefix || len(parts[2]) < 32 {
		return "", false
	}
	decodedID, err := base64.RawURLEncoding.DecodeString(parts[1])
	if err != nil || len(decodedID) == 0 || len(decodedID) > 255 {
		return "", false
	}
	secret, err := base64.RawURLEncoding.DecodeString(parts[2])
	if err != nil || len(secret) != 32 {
		return "", false
	}
	grantID := strings.TrimSpace(string(decodedID))
	return grantID, grantID != "" && !strings.ContainsAny(grantID, "\r\n\t ")
}

func tokenSHA256(value string) string {
	sum := sha256.Sum256([]byte(value))
	return hex.EncodeToString(sum[:])
}

func normalizeSHA256(value string) (string, bool) {
	value = strings.ToLower(strings.TrimSpace(value))
	value = strings.TrimPrefix(value, "sha256:")
	decoded, err := hex.DecodeString(value)
	return value, err == nil && len(decoded) == sha256.Size
}

func (service *Service) now() time.Time {
	return service.clock().UTC()
}
