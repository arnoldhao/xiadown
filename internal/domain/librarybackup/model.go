package librarybackup

import (
	"errors"
	"time"
)

const (
	ManifestFormatVersion     = 1
	LegacyMarkerFormatVersion = 1
	MarkerFormatVersion       = 2

	RestorePhasePlanned     = "planned"
	RestorePhaseInstalled   = "installed"
	RestorePhaseRollingBack = "rolling_back"

	PurposeUser            = "user"
	PurposeRestoreRollback = "pre_restore_rollback"
)

var (
	ErrInvalidConfiguration = errors.New("invalid library backup configuration")
	ErrBackupNotFound       = errors.New("library metadata backup not found")
	ErrInvalidBackup        = errors.New("invalid library metadata backup")
	ErrIncompatibleBackup   = errors.New("incompatible library metadata backup")
	ErrRestorePending       = errors.New("a library restore is already pending")
	ErrNoRestorePending     = errors.New("no library restore is pending")
)

// RetentionPolicy bounds automatic metadata snapshots. A zero field uses the
// service default; negative values are rejected by the infrastructure layer.
type RetentionPolicy struct {
	MaxBackups int           `json:"maxBackups"`
	MaxAge     time.Duration `json:"-"`
	MaxAgeDays int           `json:"maxAgeDays"`
}

type DatabaseIdentity struct {
	FileName      string `json:"fileName"`
	SHA256        string `json:"sha256"`
	SizeBytes     int64  `json:"sizeBytes"`
	ApplicationID int64  `json:"applicationId"`
	SchemaVersion int    `json:"schemaVersion"`
}

// FileInventory deliberately contains no absolute path, source URL,
// credential, device token, or credential hash. RelativePath is present only
// when the file can be proven to be beneath a declared storage root.
type FileInventory struct {
	CatalogID    string `json:"catalogId,omitempty"`
	ItemID       string `json:"itemId,omitempty"`
	AssetID      string `json:"assetId,omitempty"`
	FileID       string `json:"fileId"`
	Kind         string `json:"kind"`
	StorageMode  string `json:"storageMode"`
	StorageRoot  string `json:"storageRootId,omitempty"`
	RelativePath string `json:"relativePath,omitempty"`
	Role         string `json:"role,omitempty"`
	Position     int    `json:"position,omitempty"`
}

type StorageRootInventory struct {
	ID         string `json:"id"`
	Name       string `json:"name"`
	Mode       string `json:"mode"`
	IsDefault  bool   `json:"isDefault"`
	Status     string `json:"status"`
	AssetCount int    `json:"assetCount"`
}

type CatalogInventory struct {
	ID           string                 `json:"id"`
	IsDefault    bool                   `json:"isDefault"`
	ItemCount    int                    `json:"itemCount"`
	AssetCount   int                    `json:"assetCount"`
	StorageRoots []StorageRootInventory `json:"storageRoots"`
}

// Manifest describes a VACUUM INTO snapshot of XiaDown metadata. Referenced
// media is intentionally not copied by this backup format.
type Manifest struct {
	FormatVersion   int                `json:"formatVersion"`
	BackupID        string             `json:"backupId"`
	Purpose         string             `json:"purpose"`
	AppName         string             `json:"appName"`
	AppVersion      string             `json:"appVersion"`
	CreatedAt       time.Time          `json:"createdAt"`
	MetadataOnly    bool               `json:"metadataOnly"`
	ContentIncluded bool               `json:"contentIncluded"`
	Database        DatabaseIdentity   `json:"database"`
	Catalogs        []CatalogInventory `json:"catalogs"`
	Files           []FileInventory    `json:"files"`
}

type BackupSummary struct {
	BackupID        string    `json:"backupId"`
	Purpose         string    `json:"purpose"`
	AppVersion      string    `json:"appVersion"`
	SchemaVersion   int       `json:"schemaVersion"`
	CatalogIDs      []string  `json:"catalogIds"`
	CreatedAt       time.Time `json:"createdAt"`
	SizeBytes       int64     `json:"sizeBytes"`
	MetadataOnly    bool      `json:"metadataOnly"`
	ContentIncluded bool      `json:"contentIncluded"`
	State           string    `json:"state"`
	Error           string    `json:"error,omitempty"`
}

type VerificationResult struct {
	BackupID       string    `json:"backupId"`
	VerifiedAt     time.Time `json:"verifiedAt"`
	Valid          bool      `json:"valid"`
	ApplicationID  int64     `json:"applicationId"`
	SchemaVersion  int       `json:"schemaVersion"`
	DatabaseSHA256 string    `json:"databaseSha256"`
}

type RestoreMarker struct {
	FormatVersion        int       `json:"formatVersion"`
	Phase                string    `json:"phase,omitempty"`
	BackupID             string    `json:"backupId"`
	BackupManifestName   string    `json:"backupManifestName"`
	BackupDatabaseName   string    `json:"backupDatabaseName"`
	BackupDatabaseSHA256 string    `json:"backupDatabaseSha256"`
	RollbackBackupID     string    `json:"rollbackBackupId"`
	RequestedAt          time.Time `json:"requestedAt"`
	InstalledAt          time.Time `json:"installedAt,omitempty"`
	InstalledSHA256      string    `json:"installedSha256,omitempty"`
	PreviousWAL          bool      `json:"previousWal,omitempty"`
	PreviousSHM          bool      `json:"previousShm,omitempty"`
}

type RestorePlan struct {
	BackupID         string    `json:"backupId"`
	RollbackBackupID string    `json:"rollbackBackupId"`
	RequestedAt      time.Time `json:"requestedAt"`
	AppliesOnLaunch  bool      `json:"appliesOnLaunch"`
}

type RestoreApplyResult struct {
	Applied          bool   `json:"applied"`
	BackupID         string `json:"backupId,omitempty"`
	RollbackBackupID string `json:"rollbackBackupId,omitempty"`
}
