package access

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"sync"
	"testing"
	"time"

	"xiadown/internal/domain/library"
)

type memoryGrantRepository struct {
	mu      sync.Mutex
	items   map[string]library.DeviceGrant
	saveErr error
	changes []library.CatalogChange
}

func newMemoryGrantRepository() *memoryGrantRepository {
	return &memoryGrantRepository{items: make(map[string]library.DeviceGrant)}
}

func (repository *memoryGrantRepository) ListByCatalogID(_ context.Context, catalogID string) ([]library.DeviceGrant, error) {
	repository.mu.Lock()
	defer repository.mu.Unlock()
	result := make([]library.DeviceGrant, 0)
	for _, item := range repository.items {
		if item.CatalogID == catalogID {
			result = append(result, item)
		}
	}
	return result, nil
}

func (repository *memoryGrantRepository) Get(_ context.Context, id string) (library.DeviceGrant, error) {
	repository.mu.Lock()
	defer repository.mu.Unlock()
	item, ok := repository.items[id]
	if !ok {
		return library.DeviceGrant{}, errors.New("not found")
	}
	return item, nil
}

func (repository *memoryGrantRepository) Save(_ context.Context, item library.DeviceGrant) error {
	repository.mu.Lock()
	defer repository.mu.Unlock()
	if repository.saveErr != nil {
		return repository.saveErr
	}
	repository.items[item.ID] = item
	return nil
}

func (repository *memoryGrantRepository) Delete(_ context.Context, id string) error {
	repository.mu.Lock()
	defer repository.mu.Unlock()
	delete(repository.items, id)
	return nil
}

func (repository *memoryGrantRepository) SaveDeviceGrantMutation(
	_ context.Context,
	item library.DeviceGrant,
	expectedRevision int64,
	kind library.CatalogChangeKind,
	actorID string,
) error {
	repository.mu.Lock()
	defer repository.mu.Unlock()
	if repository.saveErr != nil {
		return repository.saveErr
	}
	current, exists := repository.items[item.ID]
	if expectedRevision == 0 {
		if exists || item.Revision != 1 {
			return library.ErrCatalogRevisionConflict
		}
	} else if !exists || current.Revision != expectedRevision || item.Revision != expectedRevision+1 {
		return library.ErrCatalogRevisionConflict
	}
	repository.items[item.ID] = item
	repository.changes = append(repository.changes, library.CatalogChange{
		Sequence: int64(len(repository.changes) + 1), CatalogID: item.CatalogID,
		EntityType: library.CatalogEntityDeviceGrant, EntityID: item.ID,
		Kind: kind, Revision: item.Revision, ActorID: actorID, OccurredAt: item.UpdatedAt,
	})
	return nil
}

func (repository *memoryGrantRepository) RecordDeviceGrantLastSeen(
	_ context.Context,
	catalogID string,
	id string,
	seenAt time.Time,
) error {
	repository.mu.Lock()
	defer repository.mu.Unlock()
	item, exists := repository.items[id]
	if !exists || item.CatalogID != catalogID || item.Status != library.DeviceGrantStatusActive {
		return errors.New("not found")
	}
	seenAt = seenAt.UTC()
	item.LastSeenAt = &seenAt
	repository.items[id] = item
	return nil
}

func TestPairStoresOnlyHashAndTokenAuthenticates(t *testing.T) {
	now := time.Date(2026, 7, 13, 12, 0, 0, 0, time.UTC)
	repository := newMemoryGrantRepository()
	service, err := NewService(repository, "catalog-1", Options{
		Clock:       func() time.Time { return now },
		IDGenerator: func() string { return "grant-1" },
	})
	if err != nil {
		t.Fatal(err)
	}
	session, err := service.StartPairing()
	if err != nil {
		t.Fatal(err)
	}
	result, err := service.Pair(context.Background(), PairRequest{
		Nonce: session.Nonce, Code: session.Code, DeviceID: "iphone-1", DeviceName: "Arnold's iPhone",
		PublicKeyHash: strings.Repeat("ab", 32),
	})
	if err != nil {
		t.Fatal(err)
	}
	if result.Token == "" || result.GrantID != "grant-1" {
		t.Fatalf("unexpected pairing result: %#v", result)
	}
	grant := repository.items["grant-1"]
	if grant.CredentialHash == result.Token || strings.Contains(grant.CredentialHash, result.Token) {
		t.Fatal("repository persisted the raw bearer token")
	}
	if len(grant.CredentialHash) != 64 || grant.CredentialHash != tokenSHA256(result.Token) {
		t.Fatalf("repository did not persist the SHA-256 token hash: %q", grant.CredentialHash)
	}
	if len(grant.Scopes) != 2 || !grant.HasScope(library.DeviceScopeLibraryRead) || !grant.HasScope(library.DeviceScopeTasksRead) {
		t.Fatalf("unexpected default scopes: %#v", grant.Scopes)
	}
	principal, err := service.Authenticate(context.Background(), result.Token)
	if err != nil {
		t.Fatal(err)
	}
	if principal.GrantID != grant.ID || !principal.HasScope(library.DeviceScopeLibraryRead) {
		t.Fatalf("unexpected principal: %#v", principal)
	}
}

func TestPairingDuplicateDeviceAndKeyCannotReplaceExistingCredential(t *testing.T) {
	now := time.Date(2026, 7, 13, 12, 0, 0, 0, time.UTC)
	repository := newMemoryGrantRepository()
	service, err := NewService(repository, "catalog-1", Options{
		Clock: func() time.Time { return now }, IDGenerator: func() string { return "grant-1" },
	})
	if err != nil {
		t.Fatal(err)
	}
	firstSession, err := service.StartPairing()
	if err != nil {
		t.Fatal(err)
	}
	first, err := service.Pair(context.Background(), validPairRequest(firstSession))
	if err != nil {
		t.Fatal(err)
	}
	managed, err := service.UpdateDeviceGrantScopes(context.Background(), UpdateScopesRequest{
		GrantID: first.GrantID, ExpectedRevision: 1,
		Scopes: []string{"library.read", "rss.read", "rss.state"}, ActorID: "local:desktop",
	})
	if err != nil {
		t.Fatal(err)
	}
	now = now.Add(time.Second)
	secondSession, err := service.StartPairing()
	if err != nil {
		t.Fatal(err)
	}
	if _, err := service.Pair(context.Background(), validPairRequest(secondSession)); !errors.Is(err, ErrInvalidRequest) {
		t.Fatalf("duplicate device replaced the existing grant: %v", err)
	}
	principal, err := service.Authenticate(context.Background(), first.Token)
	if err != nil || !principal.HasScope(library.DeviceScopeRSSRead) || !principal.HasScope(library.DeviceScopeRSSState) {
		t.Fatalf("rejected duplicate changed the existing credential: principal=%#v err=%v", principal, err)
	}
	grant := repository.items[first.GrantID]
	if grant.Revision != managed.Revision || grant.CredentialHash == "" {
		t.Fatalf("rejected duplicate changed the stored grant: %#v", grant)
	}
}

func TestPairingCannotUseKnownDeviceIDAndNewKeyToReplaceCredential(t *testing.T) {
	now := time.Date(2026, 7, 13, 12, 0, 0, 0, time.UTC)
	repository := newMemoryGrantRepository()
	service, err := NewService(repository, "catalog-1", Options{
		Clock: func() time.Time { return now }, IDGenerator: func() string { return "grant-1" },
	})
	if err != nil {
		t.Fatal(err)
	}
	firstSession, err := service.StartPairing()
	if err != nil {
		t.Fatal(err)
	}
	first, err := service.Pair(context.Background(), validPairRequest(firstSession))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := service.UpdateDeviceGrantScopes(context.Background(), UpdateScopesRequest{
		GrantID: first.GrantID, ExpectedRevision: 1,
		Scopes: []string{"library.read", "tasks.read", "tasks.create", "tasks.control"}, ActorID: "local:desktop",
	}); err != nil {
		t.Fatal(err)
	}

	now = now.Add(time.Second)
	attackSession, err := service.StartPairing()
	if err != nil {
		t.Fatal(err)
	}
	attack := validPairRequest(attackSession)
	attack.PublicKeyHash = strings.Repeat("34", 32)
	if _, err := service.Pair(context.Background(), attack); !errors.Is(err, ErrInvalidRequest) {
		t.Fatalf("changed key replaced an active device grant: %v", err)
	}
	principal, err := service.Authenticate(context.Background(), first.Token)
	if err != nil || !principal.HasScope(library.DeviceScopeTasksCreate) ||
		!principal.HasScope(library.DeviceScopeTasksControl) {
		t.Fatalf("rejected collision invalidated or changed the existing credential: principal=%#v err=%v", principal, err)
	}
}

func TestPairingReplacesRevokedDeviceWithRotatedLeastPrivilegeCredential(t *testing.T) {
	now := time.Date(2026, 7, 13, 12, 0, 0, 0, time.UTC)
	repository := newMemoryGrantRepository()
	service, err := NewService(repository, "catalog-1", Options{
		Clock: func() time.Time { return now }, IDGenerator: func() string { return "grant-1" },
	})
	if err != nil {
		t.Fatal(err)
	}
	firstSession, err := service.StartPairing()
	if err != nil {
		t.Fatal(err)
	}
	first, err := service.Pair(context.Background(), validPairRequest(firstSession))
	if err != nil {
		t.Fatal(err)
	}
	now = now.Add(time.Second)
	managed, err := service.UpdateDeviceGrantScopes(context.Background(), UpdateScopesRequest{
		GrantID: first.GrantID, ExpectedRevision: 1,
		Scopes: []string{
			"library.read", "rss.read", "rss.manage", "rss.fetch",
			"tasks.read", "tasks.create", "tasks.control",
		},
		ActorID: "local:desktop",
	})
	if err != nil || managed.Revision != 2 {
		t.Fatalf("elevate old grant: metadata=%#v err=%v", managed, err)
	}
	now = now.Add(time.Second)
	if _, err := service.RevokeDeviceGrant(context.Background(), RevokeGrantRequest{
		GrantID: first.GrantID, ExpectedRevision: 2, ActorID: "local:desktop",
	}); err != nil {
		t.Fatal(err)
	}
	revokedBefore := repository.items[first.GrantID]
	if _, err := service.Authenticate(context.Background(), first.Token); !errors.Is(err, ErrUnauthorized) {
		t.Fatalf("revoked token remained valid before replacement: %v", err)
	}

	now = now.Add(time.Second)
	repairSession, err := service.StartPairing()
	if err != nil {
		t.Fatal(err)
	}
	repair := validPairRequest(repairSession)
	repair.PublicKeyHash = strings.Repeat("56", 32)
	repair.DeviceName = "Replacement iPhone"
	replacement, err := service.Pair(context.Background(), repair)
	if err != nil {
		t.Fatalf("replace revoked device grant: %v", err)
	}
	if replacement.GrantID != first.GrantID || replacement.Token == "" || replacement.Token == first.Token {
		t.Fatalf("replacement identity/token = %#v", replacement)
	}
	if len(replacement.Scopes) != 2 || replacement.Scopes[0] != library.DeviceScopeLibraryRead ||
		replacement.Scopes[1] != library.DeviceScopeTasksRead {
		t.Fatalf("replacement inherited non-minimum scopes: %#v", replacement.Scopes)
	}
	if _, err := service.Authenticate(context.Background(), first.Token); !errors.Is(err, ErrUnauthorized) {
		t.Fatalf("old token became valid after replacement: %v", err)
	}
	principal, err := service.Authenticate(context.Background(), replacement.Token)
	if err != nil || principal.GrantID != first.GrantID ||
		!principal.HasScope(library.DeviceScopeLibraryRead) ||
		!principal.HasScope(library.DeviceScopeTasksRead) ||
		principal.HasScope(library.DeviceScopeRSSRead) ||
		principal.HasScope(library.DeviceScopeRSSManage) ||
		principal.HasScope(library.DeviceScopeRSSFetch) ||
		principal.HasScope(library.DeviceScopeTasksCreate) ||
		principal.HasScope(library.DeviceScopeTasksControl) {
		t.Fatalf("replacement principal inherited authority: principal=%#v err=%v", principal, err)
	}
	replaced := repository.items[first.GrantID]
	if replaced.Status != library.DeviceGrantStatusActive || replaced.RevokedAt != nil ||
		replaced.Revision != revokedBefore.Revision+1 || replaced.CreatedAt != revokedBefore.CreatedAt ||
		replaced.DeviceName != repair.DeviceName || replaced.PublicKeyHash != repair.PublicKeyHash ||
		replaced.CredentialHash == revokedBefore.CredentialHash ||
		replaced.CredentialHash != tokenSHA256(replacement.Token) {
		t.Fatalf("unsafe revoked-grant replacement: before=%#v after=%#v", revokedBefore, replaced)
	}
	if len(repository.changes) != 4 ||
		repository.changes[2].Kind != library.CatalogChangeDelete ||
		repository.changes[2].Revision != revokedBefore.Revision ||
		repository.changes[3].Kind != library.CatalogChangeUpsert ||
		repository.changes[3].Revision != replaced.Revision ||
		repository.changes[3].ActorID != "pairing" {
		t.Fatalf("replacement audit trail: %#v", repository.changes)
	}
}

func TestPairingExpiresAndCannotReplay(t *testing.T) {
	now := time.Date(2026, 7, 13, 12, 0, 0, 0, time.UTC)
	repository := newMemoryGrantRepository()
	service, err := NewService(repository, "catalog-1", Options{
		Clock: func() time.Time { return now }, PairingTTL: 5 * time.Minute,
		IDGenerator: func() string { return "grant-1" },
	})
	if err != nil {
		t.Fatal(err)
	}
	expired, err := service.StartPairing()
	if err != nil {
		t.Fatal(err)
	}
	now = now.Add(5 * time.Minute)
	if _, err := service.Pair(context.Background(), validPairRequest(expired)); !errors.Is(err, ErrPairingInvalid) {
		t.Fatalf("expected expired session rejection, got %v", err)
	}

	now = now.Add(time.Second)
	valid, err := service.StartPairing()
	if err != nil {
		t.Fatal(err)
	}
	if _, err := service.Pair(context.Background(), validPairRequest(valid)); err != nil {
		t.Fatal(err)
	}
	if _, err := service.Pair(context.Background(), validPairRequest(valid)); !errors.Is(err, ErrPairingInvalid) {
		t.Fatalf("expected replay rejection, got %v", err)
	}
}

func TestPairingLocksAfterRepeatedWrongCodes(t *testing.T) {
	repository := newMemoryGrantRepository()
	service, err := NewService(repository, "catalog-1", Options{IDGenerator: func() string { return "grant-1" }})
	if err != nil {
		t.Fatal(err)
	}
	session, err := service.StartPairing()
	if err != nil {
		t.Fatal(err)
	}
	wrong := validPairRequest(session)
	wrong.Code = "999999"
	if wrong.Code == session.Code {
		wrong.Code = "000000"
	}
	for attempt := 0; attempt < maxPairingFailures; attempt++ {
		if _, pairErr := service.Pair(context.Background(), wrong); !errors.Is(pairErr, ErrPairingInvalid) {
			t.Fatalf("attempt %d: expected invalid pairing, got %v", attempt+1, pairErr)
		}
	}
	if _, pairErr := service.Pair(context.Background(), validPairRequest(session)); !errors.Is(pairErr, ErrPairingInvalid) {
		t.Fatalf("locked pairing session accepted the correct code: %v", pairErr)
	}
}

func TestPairingAllowsCorrectCodeBeforeFailureLimit(t *testing.T) {
	repository := newMemoryGrantRepository()
	service, err := NewService(repository, "catalog-1", Options{IDGenerator: func() string { return "grant-1" }})
	if err != nil {
		t.Fatal(err)
	}
	session, err := service.StartPairing()
	if err != nil {
		t.Fatal(err)
	}
	wrong := validPairRequest(session)
	wrong.Code = "999999"
	if wrong.Code == session.Code {
		wrong.Code = "000000"
	}
	for attempt := 0; attempt < maxPairingFailures-1; attempt++ {
		_, _ = service.Pair(context.Background(), wrong)
	}
	if _, pairErr := service.Pair(context.Background(), validPairRequest(session)); pairErr != nil {
		t.Fatalf("correct code before lockout failed: %v", pairErr)
	}
}

func TestFailedSaveRestoresPairingSession(t *testing.T) {
	repository := newMemoryGrantRepository()
	repository.saveErr = errors.New("disk unavailable")
	service, err := NewService(repository, "catalog-1", Options{IDGenerator: func() string { return "grant-1" }})
	if err != nil {
		t.Fatal(err)
	}
	session, err := service.StartPairing()
	if err != nil {
		t.Fatal(err)
	}
	if _, err := service.Pair(context.Background(), validPairRequest(session)); err == nil {
		t.Fatal("expected repository failure")
	}
	repository.saveErr = nil
	if _, err := service.Pair(context.Background(), validPairRequest(session)); err != nil {
		t.Fatalf("pairing session was consumed before a durable save: %v", err)
	}
}

func TestAuthenticateRejectsWrongExpiredAndRevokedCredentials(t *testing.T) {
	now := time.Date(2026, 7, 13, 12, 0, 0, 0, time.UTC)
	repository := newMemoryGrantRepository()
	service, err := NewService(repository, "catalog-1", Options{
		Clock: func() time.Time { return now }, IDGenerator: func() string { return "grant-1" },
	})
	if err != nil {
		t.Fatal(err)
	}
	session, _ := service.StartPairing()
	result, err := service.Pair(context.Background(), validPairRequest(session))
	if err != nil {
		t.Fatal(err)
	}
	parts := strings.Split(result.Token, ".")
	wrong := strings.Join([]string{parts[0], parts[1], strings.Repeat("A", len(parts[2]))}, ".")
	if _, err := service.Authenticate(context.Background(), wrong); !errors.Is(err, ErrUnauthorized) {
		t.Fatalf("expected wrong token rejection, got %v", err)
	}

	grant := repository.items[result.GrantID]
	expiredAt := now.Add(time.Minute)
	grant.ExpiresAt = &expiredAt
	repository.items[result.GrantID] = grant
	now = expiredAt
	if _, err := service.Authenticate(context.Background(), result.Token); !errors.Is(err, ErrUnauthorized) {
		t.Fatalf("expected expiration rejection, got %v", err)
	}

	now = expiredAt.Add(-time.Second)
	revokedAt := now
	grant.Status = library.DeviceGrantStatusRevoked
	grant.RevokedAt = &revokedAt
	repository.items[result.GrantID] = grant
	if _, err := service.Authenticate(context.Background(), result.Token); !errors.Is(err, ErrUnauthorized) {
		t.Fatalf("expected revoked token rejection, got %v", err)
	}
}

func TestDeviceGrantManagementIsLeastPrivilegeAuditedAndRevocable(t *testing.T) {
	now := time.Date(2026, 7, 13, 15, 0, 0, 0, time.UTC)
	repository := newMemoryGrantRepository()
	service, err := NewService(repository, "catalog-1", Options{
		Clock: func() time.Time { return now }, IDGenerator: func() string { return "grant-1" },
	})
	if err != nil {
		t.Fatal(err)
	}
	session, err := service.StartPairing()
	if err != nil {
		t.Fatal(err)
	}
	paired, err := service.Pair(context.Background(), validPairRequest(session))
	if err != nil {
		t.Fatal(err)
	}
	if len(paired.Scopes) != 2 || paired.Scopes[0] != library.DeviceScopeLibraryRead || paired.Scopes[1] != library.DeviceScopeTasksRead {
		t.Fatalf("pairing did not use minimum read-only scopes: %#v", paired.Scopes)
	}
	if _, err := service.Authenticate(context.Background(), paired.Token); err != nil {
		t.Fatalf("authenticate: %v", err)
	}
	devices, err := service.ListDeviceGrants(context.Background())
	if err != nil || len(devices) != 1 || devices[0].LastSeenAt == nil || devices[0].Revision != 1 {
		t.Fatalf("safe device list: devices=%#v err=%v", devices, err)
	}
	encoded, err := json.Marshal(devices)
	if err != nil {
		t.Fatal(err)
	}
	lowerJSON := strings.ToLower(string(encoded))
	for _, secretName := range []string{"credential", "publickey", "token", "hash"} {
		if strings.Contains(lowerJSON, secretName) {
			t.Fatalf("device metadata exposed secret-bearing field %q: %s", secretName, encoded)
		}
	}

	now = now.Add(time.Minute)
	updated, err := service.UpdateDeviceGrantScopes(context.Background(), UpdateScopesRequest{
		GrantID: paired.GrantID, ExpectedRevision: 1,
		Scopes:  []string{"tasks.control", "tasks.create", "library.read", "tasks.create"},
		ActorID: "local:desktop",
	})
	if err != nil || updated.Revision != 2 || len(updated.Scopes) != 3 || updated.Status != library.DeviceGrantStatusActive {
		t.Fatalf("scope update: metadata=%#v err=%v", updated, err)
	}
	if _, err := service.UpdateDeviceGrantScopes(context.Background(), UpdateScopesRequest{
		GrantID: paired.GrantID, ExpectedRevision: 1,
		Scopes: []string{"library.read"}, ActorID: "local:desktop",
	}); !errors.Is(err, library.ErrCatalogRevisionConflict) {
		t.Fatalf("stale update must fail, got %v", err)
	}
	if _, err := service.UpdateDeviceGrantScopes(context.Background(), UpdateScopesRequest{
		GrantID: paired.GrantID, ExpectedRevision: 2,
		Scopes: []string{"library.manage"}, ActorID: "local:desktop",
	}); !errors.Is(err, ErrInvalidRequest) {
		t.Fatalf("unknown scope must fail closed, got %v", err)
	}

	now = now.Add(time.Minute)
	revoked, err := service.RevokeDeviceGrant(context.Background(), RevokeGrantRequest{
		GrantID: paired.GrantID, ExpectedRevision: 2, ActorID: "local:desktop",
	})
	if err != nil || revoked.Status != library.DeviceGrantStatusRevoked || revoked.RevokedAt == nil || revoked.Revision != 3 {
		t.Fatalf("revoke: metadata=%#v err=%v", revoked, err)
	}
	if _, err := service.Authenticate(context.Background(), paired.Token); !errors.Is(err, ErrUnauthorized) {
		t.Fatalf("revocation was not immediate: %v", err)
	}
	if len(repository.changes) != 3 ||
		repository.changes[0].Kind != library.CatalogChangeUpsert ||
		repository.changes[1].ActorID != "local:desktop" ||
		repository.changes[2].Kind != library.CatalogChangeDelete ||
		repository.changes[2].Revision != 3 {
		t.Fatalf("grant audit trail: %#v", repository.changes)
	}
}

func TestConcurrentScopeAdministratorsCannotLoseAnUpdate(t *testing.T) {
	repository := newMemoryGrantRepository()
	service, err := NewService(repository, "catalog-1", Options{IDGenerator: func() string { return "grant-1" }})
	if err != nil {
		t.Fatal(err)
	}
	session, err := service.StartPairing()
	if err != nil {
		t.Fatal(err)
	}
	paired, err := service.Pair(context.Background(), validPairRequest(session))
	if err != nil {
		t.Fatal(err)
	}

	start := make(chan struct{})
	results := make(chan error, 2)
	for _, scopes := range [][]string{
		{"library.read", "tasks.create"},
		{"library.read", "tasks.control"},
	} {
		scopes := scopes
		go func() {
			<-start
			_, err := service.UpdateDeviceGrantScopes(context.Background(), UpdateScopesRequest{
				GrantID: paired.GrantID, ExpectedRevision: 1, Scopes: scopes, ActorID: "local:desktop",
			})
			results <- err
		}()
	}
	close(start)
	successes, conflicts := 0, 0
	for range 2 {
		err := <-results
		switch {
		case err == nil:
			successes++
		case errors.Is(err, library.ErrCatalogRevisionConflict):
			conflicts++
		default:
			t.Fatalf("unexpected concurrent update error: %v", err)
		}
	}
	if successes != 1 || conflicts != 1 {
		t.Fatalf("concurrent results: success=%d conflict=%d", successes, conflicts)
	}
	devices, err := service.ListDeviceGrants(context.Background())
	if err != nil || len(devices) != 1 || devices[0].Revision != 2 {
		t.Fatalf("concurrent final state: devices=%#v err=%v", devices, err)
	}
}

func TestManagedScopesAcceptStationPermissionsWithoutChangingPairingDefaults(t *testing.T) {
	scopes, ok := normalizeManagedScopes([]string{
		"rss.fetch", "rss.state", "music.manage", "library.read", "rss.manage", "music.read",
		"rss.read", "music.state", "rss.state", "rss.fetch",
	})
	if !ok {
		t.Fatal("Station scopes were rejected")
	}
	want := []library.DeviceScope{
		library.DeviceScopeLibraryRead,
		library.DeviceScopeMusicRead,
		library.DeviceScopeMusicState,
		library.DeviceScopeMusicManage,
		library.DeviceScopeRSSRead,
		library.DeviceScopeRSSState,
		library.DeviceScopeRSSManage,
		library.DeviceScopeRSSFetch,
	}
	if len(scopes) != len(want) {
		t.Fatalf("managed scopes = %#v, want %#v", scopes, want)
	}
	for index := range want {
		if scopes[index] != want[index] {
			t.Fatalf("managed scopes = %#v, want %#v", scopes, want)
		}
	}
}

func validPairRequest(session PairingSession) PairRequest {
	return PairRequest{
		Nonce: session.Nonce, Code: session.Code, DeviceID: "device-1", DeviceName: "Test Device",
		PublicKeyHash: strings.Repeat("12", 32),
	}
}
