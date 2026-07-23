package library

import "time"

type LegacyEntityType string

const (
	LegacyEntityLibrary      LegacyEntityType = "legacy_library"
	LegacyEntityFile         LegacyEntityType = "library_file"
	LegacyEntityTrack        LegacyEntityType = "listen_track"
	LegacyEntityPlaylist     LegacyEntityType = "listen_playlist"
	LegacyEntityPlaylistItem LegacyEntityType = "listen_playlist_item"
	LegacyEntityOperation    LegacyEntityType = "operation"
	LegacyEntityWorkspace    LegacyEntityType = "workspace_state"
)

type LegacyMapping struct {
	MigrationID       string
	CatalogID         string
	SourceType        LegacyEntityType
	SourceID          string
	TargetType        CatalogEntityType
	TargetID          string
	SourceFingerprint string
	MigratedAt        time.Time
}

type LegacyMappingParams struct {
	MigrationID       string
	CatalogID         string
	SourceType        string
	SourceID          string
	TargetType        string
	TargetID          string
	SourceFingerprint string
	MigratedAt        time.Time
}

// CatalogBackfillItem is the complete, deterministic projection of one
// logical catalog item. Assets and mappings are persisted together so a
// legacy file can never be marked migrated without its catalog association.
type CatalogBackfillItem struct {
	Item     Item
	Assets   []ItemAsset
	Mappings []LegacyMapping
}

// CatalogBackfillBundle is the atomic unit used while expanding legacy
// download/import bundles into the single user-facing Catalog. It contains no
// filesystem operation: LibraryFile IDs and paths remain the source of truth.
type CatalogBackfillBundle struct {
	LegacyLibraryID string
	BundleMapping   LegacyMapping
	Items           []CatalogBackfillItem
	Checkpoint      MigrationCheckpoint
}

func NewLegacyMapping(params LegacyMappingParams) (LegacyMapping, error) {
	migrationID, migrationIDOK := normalizeCatalogID(params.MigrationID)
	catalogID, catalogIDOK := normalizeCatalogID(params.CatalogID)
	sourceID, sourceIDOK := normalizeCatalogID(params.SourceID)
	targetID, targetIDOK := normalizeCatalogID(params.TargetID)
	fingerprint, fingerprintOK := normalizeCatalogOpaqueValue(params.SourceFingerprint)
	sourceType := LegacyEntityType(normalizeCatalogEnum(params.SourceType))
	targetType := CatalogEntityType(normalizeCatalogEnum(params.TargetType))
	if !migrationIDOK || !catalogIDOK || !sourceIDOK || !targetIDOK || !fingerprintOK ||
		!isLegacyEntityType(sourceType) || !isCatalogEntityType(targetType) {
		return LegacyMapping{}, ErrInvalidLegacyMapping
	}
	if params.MigratedAt.IsZero() {
		params.MigratedAt = time.Now().UTC()
	}
	return LegacyMapping{
		MigrationID: migrationID, CatalogID: catalogID, SourceType: sourceType,
		SourceID: sourceID, TargetType: targetType, TargetID: targetID,
		SourceFingerprint: fingerprint, MigratedAt: params.MigratedAt.UTC(),
	}, nil
}

func isLegacyEntityType(value LegacyEntityType) bool {
	switch value {
	case LegacyEntityLibrary, LegacyEntityFile, LegacyEntityTrack, LegacyEntityPlaylist,
		LegacyEntityPlaylistItem, LegacyEntityOperation, LegacyEntityWorkspace:
		return true
	default:
		return false
	}
}

type MigrationPhase string

const (
	MigrationPhasePreflight  MigrationPhase = "preflight"
	MigrationPhaseExpand     MigrationPhase = "expand"
	MigrationPhaseBackfill   MigrationPhase = "backfill"
	MigrationPhaseShadowRead MigrationPhase = "shadow_read"
	MigrationPhaseCutover    MigrationPhase = "cutover"
	MigrationPhaseStabilize  MigrationPhase = "stabilize"
)

type MigrationStatus string

const (
	MigrationStatusPending   MigrationStatus = "pending"
	MigrationStatusRunning   MigrationStatus = "running"
	MigrationStatusCompleted MigrationStatus = "completed"
	MigrationStatusFailed    MigrationStatus = "failed"
)

type MigrationCheckpoint struct {
	MigrationID string
	CatalogID   string
	Phase       MigrationPhase
	Status      MigrationStatus
	Cursor      string
	Processed   int64
	Failed      int64
	LastError   string
	StartedAt   *time.Time
	FinishedAt  *time.Time
	CreatedAt   time.Time
	UpdatedAt   time.Time
}

type MigrationCheckpointParams struct {
	MigrationID string
	CatalogID   string
	Phase       string
	Status      string
	Cursor      string
	Processed   int64
	Failed      int64
	LastError   string
	StartedAt   *time.Time
	FinishedAt  *time.Time
	CreatedAt   *time.Time
	UpdatedAt   *time.Time
}

func NewMigrationCheckpoint(params MigrationCheckpointParams) (MigrationCheckpoint, error) {
	migrationID, migrationIDOK := normalizeCatalogID(params.MigrationID)
	catalogID, catalogIDOK := normalizeCatalogID(params.CatalogID)
	cursor, cursorOK := normalizeCatalogOpaqueValue(params.Cursor)
	lastError, errorOK := normalizeCatalogDescription(params.LastError)
	phase := MigrationPhase(normalizeCatalogEnum(params.Phase))
	status := MigrationStatus(normalizeCatalogEnum(params.Status))
	if status == "" {
		status = MigrationStatusPending
	}
	createdAt, updatedAt, timesOK := normalizeCatalogTimes(params.CreatedAt, params.UpdatedAt)
	startedAt := normalizeOptionalCatalogTime(params.StartedAt)
	finishedAt := normalizeOptionalCatalogTime(params.FinishedAt)
	if !migrationIDOK || !catalogIDOK || !cursorOK || !errorOK || !timesOK ||
		params.Processed < 0 || params.Failed < 0 || params.Failed > params.Processed ||
		!isMigrationPhase(phase) {
		return MigrationCheckpoint{}, ErrInvalidMigrationCheckpoint
	}
	switch status {
	case MigrationStatusPending:
		if startedAt != nil || finishedAt != nil || lastError != "" {
			return MigrationCheckpoint{}, ErrInvalidMigrationCheckpoint
		}
	case MigrationStatusRunning:
		if startedAt == nil || finishedAt != nil || lastError != "" {
			return MigrationCheckpoint{}, ErrInvalidMigrationCheckpoint
		}
	case MigrationStatusCompleted:
		if startedAt == nil || finishedAt == nil || lastError != "" {
			return MigrationCheckpoint{}, ErrInvalidMigrationCheckpoint
		}
	case MigrationStatusFailed:
		if startedAt == nil || finishedAt == nil || lastError == "" {
			return MigrationCheckpoint{}, ErrInvalidMigrationCheckpoint
		}
	default:
		return MigrationCheckpoint{}, ErrInvalidMigrationCheckpoint
	}
	if startedAt != nil && (startedAt.Before(createdAt) || startedAt.After(updatedAt)) ||
		finishedAt != nil && (startedAt == nil || finishedAt.Before(*startedAt) || finishedAt.After(updatedAt)) {
		return MigrationCheckpoint{}, ErrInvalidMigrationCheckpoint
	}
	return MigrationCheckpoint{
		MigrationID: migrationID, CatalogID: catalogID, Phase: phase, Status: status,
		Cursor: cursor, Processed: params.Processed, Failed: params.Failed, LastError: lastError,
		StartedAt: startedAt, FinishedAt: finishedAt, CreatedAt: createdAt, UpdatedAt: updatedAt,
	}, nil
}

func isMigrationPhase(value MigrationPhase) bool {
	switch value {
	case MigrationPhasePreflight, MigrationPhaseExpand, MigrationPhaseBackfill,
		MigrationPhaseShadowRead, MigrationPhaseCutover, MigrationPhaseStabilize:
		return true
	default:
		return false
	}
}
