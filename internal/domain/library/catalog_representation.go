package library

import (
	"bytes"
	"encoding/hex"
	"encoding/json"
	"math"
	"strconv"
	"strings"
	"time"
	"unicode"
)

// RepresentationKind describes what a concrete asset represents. It is
// intentionally independent from ItemAssetRole: a file link is physical,
// while a representation is a revisioned, syncable technical description.
type RepresentationKind string

const (
	RepresentationKindOriginal   RepresentationKind = "original"
	RepresentationKindOptimized  RepresentationKind = "optimized"
	RepresentationKindThumbnail  RepresentationKind = "thumbnail"
	RepresentationKindTranscript RepresentationKind = "transcript"
	RepresentationKindSubtitle   RepresentationKind = "subtitle"
	RepresentationKindArtwork    RepresentationKind = "artwork"
	RepresentationKindPreview    RepresentationKind = "preview"
	RepresentationKindAttachment RepresentationKind = "attachment"
)

type RepresentationPurpose string

const (
	RepresentationPurposePrimary       RepresentationPurpose = "primary"
	RepresentationPurposePlayback      RepresentationPurpose = "playback"
	RepresentationPurposePreview       RepresentationPurpose = "preview"
	RepresentationPurposeAccessibility RepresentationPurpose = "accessibility"
	RepresentationPurposeArtwork       RepresentationPurpose = "artwork"
	RepresentationPurposeAttachment    RepresentationPurpose = "attachment"
	RepresentationPurposeIndexing      RepresentationPurpose = "indexing"
)

type RepresentationAvailability string

const (
	RepresentationAvailabilityAvailable  RepresentationAvailability = "available"
	RepresentationAvailabilityProcessing RepresentationAvailability = "processing"
	RepresentationAvailabilityOffline    RepresentationAvailability = "offline"
	RepresentationAvailabilityMissing    RepresentationAvailability = "missing"
	RepresentationAvailabilityCorrupt    RepresentationAvailability = "corrupt"
)

type RepresentationChecksumAlgorithm string

const (
	RepresentationChecksumSHA256 RepresentationChecksumAlgorithm = "sha256"
	RepresentationChecksumBLAKE3 RepresentationChecksumAlgorithm = "blake3"
)

type Representation struct {
	ID                string
	CatalogID         string
	ItemID            string
	AssetID           string
	Kind              RepresentationKind
	Purpose           RepresentationPurpose
	MediaType         string
	Container         string
	Codec             string
	Width             *int
	Height            *int
	DurationMs        *int64
	BitrateBps        *int64
	Language          string
	ChecksumAlgorithm RepresentationChecksumAlgorithm
	Checksum          string
	SizeBytes         *int64
	Availability      RepresentationAvailability
	Revision          int64
	CreatedAt         time.Time
	UpdatedAt         time.Time
}

type RepresentationParams struct {
	ID                string
	CatalogID         string
	ItemID            string
	AssetID           string
	Kind              string
	Purpose           string
	MediaType         string
	Container         string
	Codec             string
	Width             *int
	Height            *int
	DurationMs        *int64
	BitrateBps        *int64
	Language          string
	ChecksumAlgorithm string
	Checksum          string
	SizeBytes         *int64
	Availability      string
	Revision          int64
	CreatedAt         *time.Time
	UpdatedAt         *time.Time
}

func NewRepresentation(params RepresentationParams) (Representation, error) {
	id, idOK := normalizeCatalogID(params.ID)
	catalogID, catalogIDOK := normalizeCatalogID(params.CatalogID)
	itemID, itemIDOK := normalizeCatalogID(params.ItemID)
	assetID, assetIDOK := normalizeCatalogID(params.AssetID)
	kind := RepresentationKind(normalizeCatalogEnum(params.Kind))
	purpose := RepresentationPurpose(normalizeCatalogEnum(params.Purpose))
	if purpose == "" {
		purpose = defaultRepresentationPurpose(kind)
	}
	availability := RepresentationAvailability(normalizeCatalogEnum(params.Availability))
	if availability == "" {
		availability = RepresentationAvailabilityAvailable
	}
	mediaType, mediaTypeOK := normalizeRepresentationMediaType(params.MediaType)
	container, containerOK := normalizeCatalogOpaqueValue(params.Container)
	codec, codecOK := normalizeCatalogOpaqueValue(params.Codec)
	language, languageOK := normalizeRepresentationLanguage(params.Language)
	algorithm := RepresentationChecksumAlgorithm(normalizeCatalogEnum(params.ChecksumAlgorithm))
	checksum := strings.ToLower(strings.TrimSpace(params.Checksum))
	checksumOK := validRepresentationChecksum(algorithm, checksum)
	revision, revisionOK := normalizeCatalogRevision(params.Revision)
	createdAt, updatedAt, timesOK := normalizeCatalogTimes(params.CreatedAt, params.UpdatedAt)
	if !idOK || !catalogIDOK || !itemIDOK || !assetIDOK ||
		!validRepresentationKind(kind) || !validRepresentationPurpose(purpose) ||
		!validRepresentationAvailability(availability) || !mediaTypeOK || !containerOK ||
		!codecOK || !languageOK || !checksumOK || !revisionOK || !timesOK ||
		!validOptionalPositiveInt(params.Width) || !validOptionalPositiveInt(params.Height) ||
		!validOptionalNonNegativeInt64(params.DurationMs) || !validOptionalPositiveInt64(params.BitrateBps) ||
		!validOptionalNonNegativeInt64(params.SizeBytes) {
		return Representation{}, ErrInvalidRepresentation
	}
	return Representation{
		ID: id, CatalogID: catalogID, ItemID: itemID, AssetID: assetID,
		Kind: kind, Purpose: purpose, MediaType: mediaType, Container: container, Codec: codec,
		Width: copyInt(params.Width), Height: copyInt(params.Height),
		DurationMs: copyInt64(params.DurationMs), BitrateBps: copyInt64(params.BitrateBps),
		Language: language, ChecksumAlgorithm: algorithm, Checksum: checksum,
		SizeBytes: copyInt64(params.SizeBytes), Availability: availability, Revision: revision,
		CreatedAt: createdAt, UpdatedAt: updatedAt,
	}, nil
}

func validRepresentationKind(value RepresentationKind) bool {
	switch value {
	case RepresentationKindOriginal, RepresentationKindOptimized, RepresentationKindThumbnail,
		RepresentationKindTranscript, RepresentationKindSubtitle, RepresentationKindArtwork,
		RepresentationKindPreview, RepresentationKindAttachment:
		return true
	default:
		return false
	}
}

func defaultRepresentationPurpose(kind RepresentationKind) RepresentationPurpose {
	switch kind {
	case RepresentationKindOriginal:
		return RepresentationPurposePrimary
	case RepresentationKindOptimized:
		return RepresentationPurposePlayback
	case RepresentationKindThumbnail, RepresentationKindPreview:
		return RepresentationPurposePreview
	case RepresentationKindTranscript, RepresentationKindSubtitle:
		return RepresentationPurposeAccessibility
	case RepresentationKindArtwork:
		return RepresentationPurposeArtwork
	case RepresentationKindAttachment:
		return RepresentationPurposeAttachment
	default:
		return ""
	}
}

func validRepresentationPurpose(value RepresentationPurpose) bool {
	switch value {
	case RepresentationPurposePrimary, RepresentationPurposePlayback, RepresentationPurposePreview,
		RepresentationPurposeAccessibility, RepresentationPurposeArtwork,
		RepresentationPurposeAttachment, RepresentationPurposeIndexing:
		return true
	default:
		return false
	}
}

func validRepresentationAvailability(value RepresentationAvailability) bool {
	switch value {
	case RepresentationAvailabilityAvailable, RepresentationAvailabilityProcessing,
		RepresentationAvailabilityOffline, RepresentationAvailabilityMissing, RepresentationAvailabilityCorrupt:
		return true
	default:
		return false
	}
}

func normalizeRepresentationMediaType(value string) (string, bool) {
	value, ok := normalizeCatalogOpaqueValue(value)
	if !ok || value == "" {
		return value, ok
	}
	value = strings.ToLower(value)
	parts := strings.Split(value, "/")
	if len(parts) != 2 || parts[0] == "" || parts[1] == "" || strings.ContainsAny(value, " \t\r\n") {
		return "", false
	}
	return value, true
}

func normalizeRepresentationLanguage(value string) (string, bool) {
	value, ok := normalizeCatalogOpaqueValue(value)
	if !ok || value == "" {
		return value, ok
	}
	if len(value) > 63 || strings.HasPrefix(value, "-") || strings.HasSuffix(value, "-") {
		return "", false
	}
	for _, char := range value {
		if char != '-' && !unicode.IsLetter(char) && !unicode.IsDigit(char) {
			return "", false
		}
	}
	return value, true
}

func validRepresentationChecksum(algorithm RepresentationChecksumAlgorithm, checksum string) bool {
	if algorithm == "" || checksum == "" {
		return algorithm == "" && checksum == ""
	}
	if algorithm != RepresentationChecksumSHA256 && algorithm != RepresentationChecksumBLAKE3 || len(checksum) != 64 {
		return false
	}
	_, err := hex.DecodeString(checksum)
	return err == nil
}

func validOptionalPositiveInt(value *int) bool {
	return value == nil || *value > 0
}

func validOptionalNonNegativeInt64(value *int64) bool {
	return value == nil || *value >= 0
}

func validOptionalPositiveInt64(value *int64) bool {
	return value == nil || *value > 0
}

func copyInt(value *int) *int {
	if value == nil {
		return nil
	}
	copyValue := *value
	return &copyValue
}

func copyInt64(value *int64) *int64 {
	if value == nil {
		return nil
	}
	copyValue := *value
	return &copyValue
}

type MetadataValueType string

const (
	MetadataValueString     MetadataValueType = "string"
	MetadataValueInteger    MetadataValueType = "integer"
	MetadataValueNumber     MetadataValueType = "number"
	MetadataValueBoolean    MetadataValueType = "boolean"
	MetadataValueDate       MetadataValueType = "date"
	MetadataValueDateTime   MetadataValueType = "datetime"
	MetadataValueDurationMs MetadataValueType = "duration_ms"
	MetadataValueObject     MetadataValueType = "object"
	MetadataValueArray      MetadataValueType = "array"
	MetadataValueJSON       MetadataValueType = "json"
)

type MetadataSource string

const (
	MetadataSourceUser      MetadataSource = "user"
	MetadataSourceEmbedded  MetadataSource = "embedded"
	MetadataSourceSidecar   MetadataSource = "sidecar"
	MetadataSourceRemote    MetadataSource = "remote"
	MetadataSourceDerived   MetadataSource = "derived"
	MetadataSourceMigration MetadataSource = "migration"
	MetadataSourceSystem    MetadataSource = "system"
)

// MetadataEntry is one typed, repeatable metadata value. Provenance is
// mandatory, confidence is explicit when known, and Locked prevents automated
// scanners from silently replacing a curated value.
type MetadataEntry struct {
	ID               string
	CatalogID        string
	ItemID           string
	RepresentationID string
	Namespace        string
	Key              string
	ValueType        MetadataValueType
	Value            json.RawMessage
	Language         string
	Position         int
	Source           MetadataSource
	Provenance       string
	Confidence       *float64
	Locked           bool
	Revision         int64
	CreatedAt        time.Time
	UpdatedAt        time.Time
}

type MetadataEntryParams struct {
	ID               string
	CatalogID        string
	ItemID           string
	RepresentationID string
	Namespace        string
	Key              string
	ValueType        string
	ValueJSON        string
	Language         string
	Position         int
	Source           string
	Provenance       string
	Confidence       *float64
	Locked           bool
	Revision         int64
	CreatedAt        *time.Time
	UpdatedAt        *time.Time
}

func NewMetadataEntry(params MetadataEntryParams) (MetadataEntry, error) {
	id, idOK := normalizeCatalogID(params.ID)
	catalogID, catalogIDOK := normalizeCatalogID(params.CatalogID)
	itemID, itemIDOK := normalizeCatalogID(params.ItemID)
	representationID, representationIDOK := normalizeOptionalCatalogID(params.RepresentationID)
	namespace, namespaceOK := normalizeMetadataIdentifier(params.Namespace)
	key, keyOK := normalizeMetadataIdentifier(params.Key)
	valueType := MetadataValueType(normalizeCatalogEnum(params.ValueType))
	value, valueOK := normalizeMetadataValue(valueType, params.ValueJSON)
	language, languageOK := normalizeRepresentationLanguage(params.Language)
	source := MetadataSource(normalizeCatalogEnum(params.Source))
	provenance, provenanceOK := normalizeCatalogDescription(params.Provenance)
	provenanceOK = provenanceOK && provenance != ""
	revision, revisionOK := normalizeCatalogRevision(params.Revision)
	createdAt, updatedAt, timesOK := normalizeCatalogTimes(params.CreatedAt, params.UpdatedAt)
	confidence := copyFloat64(params.Confidence)
	confidenceOK := confidence == nil || !math.IsNaN(*confidence) && !math.IsInf(*confidence, 0) && *confidence >= 0 && *confidence <= 1
	if !idOK || !catalogIDOK || !itemIDOK || !representationIDOK || !namespaceOK || !keyOK ||
		!valueOK || !languageOK || params.Position < 0 || !validMetadataSource(source) ||
		!provenanceOK || !confidenceOK || !revisionOK || !timesOK {
		return MetadataEntry{}, ErrInvalidMetadataEntry
	}
	return MetadataEntry{
		ID: id, CatalogID: catalogID, ItemID: itemID, RepresentationID: representationID,
		Namespace: namespace, Key: key, ValueType: valueType, Value: value,
		Language: language, Position: params.Position, Source: source, Provenance: provenance,
		Confidence: confidence, Locked: params.Locked, Revision: revision,
		CreatedAt: createdAt, UpdatedAt: updatedAt,
	}, nil
}

func normalizeMetadataIdentifier(value string) (string, bool) {
	value = strings.ToLower(strings.TrimSpace(value))
	if value == "" || len(value) > 255 {
		return "", false
	}
	for _, char := range value {
		if char != '.' && char != '-' && char != '_' && !unicode.IsLetter(char) && !unicode.IsDigit(char) {
			return "", false
		}
	}
	return value, true
}

func normalizeMetadataValue(valueType MetadataValueType, raw string) (json.RawMessage, bool) {
	if !validMetadataValueType(valueType) || !json.Valid([]byte(raw)) {
		return nil, false
	}
	decoder := json.NewDecoder(strings.NewReader(raw))
	decoder.UseNumber()
	var decoded any
	if err := decoder.Decode(&decoded); err != nil {
		return nil, false
	}
	switch valueType {
	case MetadataValueString:
		_, ok := decoded.(string)
		if !ok {
			return nil, false
		}
	case MetadataValueInteger, MetadataValueDurationMs:
		number, ok := decoded.(json.Number)
		if !ok {
			return nil, false
		}
		integer, err := strconv.ParseInt(number.String(), 10, 64)
		if err != nil || valueType == MetadataValueDurationMs && integer < 0 {
			return nil, false
		}
	case MetadataValueNumber:
		number, ok := decoded.(json.Number)
		if !ok {
			return nil, false
		}
		parsed, err := strconv.ParseFloat(number.String(), 64)
		if err != nil || math.IsNaN(parsed) || math.IsInf(parsed, 0) {
			return nil, false
		}
	case MetadataValueBoolean:
		if _, ok := decoded.(bool); !ok {
			return nil, false
		}
	case MetadataValueDate, MetadataValueDateTime:
		text, ok := decoded.(string)
		if !ok {
			return nil, false
		}
		layout := "2006-01-02"
		if valueType == MetadataValueDateTime {
			layout = time.RFC3339Nano
		}
		if _, err := time.Parse(layout, text); err != nil {
			return nil, false
		}
	case MetadataValueObject:
		if _, ok := decoded.(map[string]any); !ok {
			return nil, false
		}
	case MetadataValueArray:
		if _, ok := decoded.([]any); !ok {
			return nil, false
		}
	case MetadataValueJSON:
		// Any valid JSON value is accepted for lossless migration/interchange.
	}
	var compact bytes.Buffer
	if err := json.Compact(&compact, []byte(raw)); err != nil {
		return nil, false
	}
	return json.RawMessage(append([]byte(nil), compact.Bytes()...)), true
}

func validMetadataValueType(value MetadataValueType) bool {
	switch value {
	case MetadataValueString, MetadataValueInteger, MetadataValueNumber, MetadataValueBoolean,
		MetadataValueDate, MetadataValueDateTime, MetadataValueDurationMs,
		MetadataValueObject, MetadataValueArray, MetadataValueJSON:
		return true
	default:
		return false
	}
}

func validMetadataSource(value MetadataSource) bool {
	switch value {
	case MetadataSourceUser, MetadataSourceEmbedded, MetadataSourceSidecar, MetadataSourceRemote,
		MetadataSourceDerived, MetadataSourceMigration, MetadataSourceSystem:
		return true
	default:
		return false
	}
}

func copyFloat64(value *float64) *float64 {
	if value == nil {
		return nil
	}
	copyValue := *value
	return &copyValue
}
