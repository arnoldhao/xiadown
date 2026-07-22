// Package catalogaudit defines the read-only integrity contract used to
// reconcile the legacy physical file registry with the Catalog projection.
package catalogaudit

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"strings"
	"time"
)

var ErrInvalidRequest = errors.New("invalid catalog audit request")

// Auditor is an application port. Implementations must not repair or mutate
// the data they inspect.
type Auditor interface {
	Audit(ctx context.Context, request Request) (Report, error)
}

type Request struct {
	CatalogID   string
	MigrationID string
}

type Counts struct {
	LegacyFiles                 int64
	LegacyMappings              int64
	AssetLinks                  int64
	Items                       int64
	ActiveItems                 int64
	MissingItems                int64
	TrashedItems                int64
	NeedsReviewItems            int64
	PreservedFileIDs            int64
	PreservedPhysicalReferences int64
	Representations             int64
	MetadataEntries             int64
}

type Findings struct {
	UnmappedLegacyFiles              int64
	DuplicateMappings                int64
	DanglingAssets                   int64
	MissingMappingSources            int64
	MissingMappingTargets            int64
	MappingAssetMismatches           int64
	ChangedPhysicalReferences        int64
	AssetsWithoutRepresentations     int64
	DanglingRepresentations          int64
	RepresentationMismatches         int64
	DanglingMetadataEntries          int64
	MetadataRepresentationMismatches int64
}

func (findings Findings) Total() int64 {
	return findings.UnmappedLegacyFiles +
		findings.DuplicateMappings +
		findings.DanglingAssets +
		findings.MissingMappingSources +
		findings.MissingMappingTargets +
		findings.MappingAssetMismatches +
		findings.ChangedPhysicalReferences +
		findings.AssetsWithoutRepresentations +
		findings.DanglingRepresentations +
		findings.RepresentationMismatches +
		findings.DanglingMetadataEntries +
		findings.MetadataRepresentationMismatches
}

type IssueKind string

const (
	IssueUnmappedLegacyFile             IssueKind = "unmapped_legacy_file"
	IssueDuplicateMapping               IssueKind = "duplicate_mapping"
	IssueDanglingAsset                  IssueKind = "dangling_asset"
	IssueMissingMappingSource           IssueKind = "missing_mapping_source"
	IssueMissingMappingTarget           IssueKind = "missing_mapping_target"
	IssueMappingAssetMismatch           IssueKind = "mapping_asset_mismatch"
	IssueChangedPhysicalReference       IssueKind = "changed_physical_reference"
	IssueAssetWithoutRepresentation     IssueKind = "asset_without_representation"
	IssueDanglingRepresentation         IssueKind = "dangling_representation"
	IssueRepresentationMismatch         IssueKind = "representation_mismatch"
	IssueDanglingMetadataEntry          IssueKind = "dangling_metadata_entry"
	IssueMetadataRepresentationMismatch IssueKind = "metadata_representation_mismatch"
)

type Issue struct {
	Kind             IssueKind
	SourceID         string
	TargetID         string
	AssetID          string
	RepresentationID string
	MetadataEntryID  string
	Description      string
}

type Report struct {
	CatalogID   string
	MigrationID string
	Counts      Counts
	Findings    Findings
	Issues      []Issue
	AuditedAt   time.Time
}

func (report Report) IsHealthy() bool {
	return report.Findings.Total() == 0 && len(report.Issues) == 0
}

// LegacyFileReference contains only the immutable identity and physical
// reference fields covered by the Catalog foundation migration fingerprint.
// LocalPath and DocumentID are both included because legacy assets may be
// file-backed or database-document-backed.
type LegacyFileReference struct {
	ID              string
	LibraryID       string
	Kind            string
	Name            string
	DisplayName     string
	StorageMode     string
	LocalPath       string
	DocumentID      string
	LineageRootID   string
	SourceUpdatedAt time.Time
}

// FingerprintLegacyFileReference must remain compatible with the Catalog
// foundation projector. A mismatch means the source reference is no
// longer byte-for-byte the reference that was audited during projection; the
// function never reads or hashes the file contents themselves.
func FingerprintLegacyFileReference(reference LegacyFileReference) string {
	digest := sha256.Sum256([]byte(strings.Join([]string{
		reference.ID,
		reference.LibraryID,
		reference.Kind,
		reference.Name,
		reference.DisplayName,
		reference.StorageMode,
		reference.LocalPath,
		reference.DocumentID,
		reference.LineageRootID,
		reference.SourceUpdatedAt.UTC().Format(time.RFC3339Nano),
	}, "\x00")))
	return "sha256:" + hex.EncodeToString(digest[:])
}
