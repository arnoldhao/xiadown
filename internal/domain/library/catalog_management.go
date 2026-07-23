package library

import (
	"math"
	"strings"
	"time"
)

type StorageRootMode string

const (
	StorageRootModeManaged    StorageRootMode = "managed"
	StorageRootModeReferenced StorageRootMode = "referenced"
)

type StorageRootStatus string

const (
	StorageRootStatusOnline   StorageRootStatus = "online"
	StorageRootStatusOffline  StorageRootStatus = "offline"
	StorageRootStatusReadOnly StorageRootStatus = "read_only"
	StorageRootStatusError    StorageRootStatus = "error"
)

type StorageRoot struct {
	ID            string
	CatalogID     string
	Name          string
	Path          string
	VolumeID      string
	Mode          StorageRootMode
	Status        StorageRootStatus
	LastCheckedAt *time.Time
	LastError     string
	CreatedAt     time.Time
	UpdatedAt     time.Time
}

type StorageRootParams struct {
	ID            string
	CatalogID     string
	Name          string
	Path          string
	VolumeID      string
	Mode          string
	Status        string
	LastCheckedAt *time.Time
	LastError     string
	CreatedAt     *time.Time
	UpdatedAt     *time.Time
}

func NewStorageRoot(params StorageRootParams) (StorageRoot, error) {
	id, idOK := normalizeCatalogID(params.ID)
	catalogID, catalogIDOK := normalizeCatalogID(params.CatalogID)
	name, nameOK := normalizeCatalogName(params.Name)
	path, pathOK := normalizeCatalogOpaqueValue(params.Path)
	volumeID, volumeIDOK := normalizeCatalogOpaqueValue(params.VolumeID)
	lastError, errorOK := normalizeCatalogDescription(params.LastError)
	mode := StorageRootMode(normalizeCatalogEnum(params.Mode))
	status := StorageRootStatus(normalizeCatalogEnum(params.Status))
	if status == "" {
		status = StorageRootStatusOnline
	}
	createdAt, updatedAt, timesOK := normalizeCatalogTimes(params.CreatedAt, params.UpdatedAt)
	lastCheckedAt := normalizeOptionalCatalogTime(params.LastCheckedAt)
	if !idOK || !catalogIDOK || !nameOK || !pathOK || path == "" || !volumeIDOK || !errorOK || !timesOK {
		return StorageRoot{}, ErrInvalidStorageRoot
	}
	switch mode {
	case StorageRootModeManaged, StorageRootModeReferenced:
	default:
		return StorageRoot{}, ErrInvalidStorageRoot
	}
	switch status {
	case StorageRootStatusOnline, StorageRootStatusOffline, StorageRootStatusReadOnly:
		if lastError != "" {
			return StorageRoot{}, ErrInvalidStorageRoot
		}
	case StorageRootStatusError:
		if lastError == "" {
			return StorageRoot{}, ErrInvalidStorageRoot
		}
	default:
		return StorageRoot{}, ErrInvalidStorageRoot
	}
	if lastCheckedAt != nil && lastCheckedAt.After(updatedAt) {
		return StorageRoot{}, ErrInvalidStorageRoot
	}
	return StorageRoot{
		ID:            id,
		CatalogID:     catalogID,
		Name:          name,
		Path:          path,
		VolumeID:      volumeID,
		Mode:          mode,
		Status:        status,
		LastCheckedAt: lastCheckedAt,
		LastError:     lastError,
		CreatedAt:     createdAt,
		UpdatedAt:     updatedAt,
	}, nil
}

type CollectionKind string

const (
	CollectionKindManual   CollectionKind = "manual"
	CollectionKindSmart    CollectionKind = "smart"
	CollectionKindPlaylist CollectionKind = "playlist"
	CollectionKindAlbum    CollectionKind = "album"
	CollectionKindShelf    CollectionKind = "shelf"
	CollectionKindSeries   CollectionKind = "series"
)

type Collection struct {
	ID          string
	CatalogID   string
	Name        string
	Description string
	Kind        CollectionKind
	SmartQuery  string
	Revision    int64
	CreatedAt   time.Time
	UpdatedAt   time.Time
}

type CollectionParams struct {
	ID          string
	CatalogID   string
	Name        string
	Description string
	Kind        string
	SmartQuery  string
	Revision    int64
	CreatedAt   *time.Time
	UpdatedAt   *time.Time
}

func NewCollection(params CollectionParams) (Collection, error) {
	id, idOK := normalizeCatalogID(params.ID)
	catalogID, catalogIDOK := normalizeCatalogID(params.CatalogID)
	name, nameOK := normalizeCatalogName(params.Name)
	description, descriptionOK := normalizeCatalogDescription(params.Description)
	smartQuery, queryOK := normalizeCatalogOpaqueValue(params.SmartQuery)
	kind := CollectionKind(normalizeCatalogEnum(params.Kind))
	if kind == "" {
		kind = CollectionKindManual
	}
	revision, revisionOK := normalizeCatalogRevision(params.Revision)
	createdAt, updatedAt, timesOK := normalizeCatalogTimes(params.CreatedAt, params.UpdatedAt)
	if !idOK || !catalogIDOK || !nameOK || !descriptionOK || !queryOK || !revisionOK || !timesOK {
		return Collection{}, ErrInvalidCollection
	}
	switch kind {
	case CollectionKindSmart:
		if smartQuery == "" {
			return Collection{}, ErrInvalidCollection
		}
	case CollectionKindManual, CollectionKindPlaylist, CollectionKindAlbum, CollectionKindShelf, CollectionKindSeries:
		if smartQuery != "" {
			return Collection{}, ErrInvalidCollection
		}
	default:
		return Collection{}, ErrInvalidCollection
	}
	return Collection{
		ID:          id,
		CatalogID:   catalogID,
		Name:        name,
		Description: description,
		Kind:        kind,
		SmartQuery:  smartQuery,
		Revision:    revision,
		CreatedAt:   createdAt,
		UpdatedAt:   updatedAt,
	}, nil
}

type CollectionItem struct {
	ID           string
	CollectionID string
	ItemID       string
	Position     int
	AddedAt      time.Time
}

func NewCollectionItem(id, collectionID, itemID string, position int, addedAt time.Time) (CollectionItem, error) {
	id, idOK := normalizeCatalogID(id)
	collectionID, collectionIDOK := normalizeCatalogID(collectionID)
	itemID, itemIDOK := normalizeCatalogID(itemID)
	if !idOK || !collectionIDOK || !itemIDOK || position < 0 {
		return CollectionItem{}, ErrInvalidCollectionItem
	}
	if addedAt.IsZero() {
		addedAt = time.Now().UTC()
	}
	return CollectionItem{ID: id, CollectionID: collectionID, ItemID: itemID, Position: position, AddedAt: addedAt.UTC()}, nil
}

type Tag struct {
	ID             string
	CatalogID      string
	Name           string
	NormalizedName string
	CreatedAt      time.Time
	UpdatedAt      time.Time
}

type TagParams struct {
	ID        string
	CatalogID string
	Name      string
	CreatedAt *time.Time
	UpdatedAt *time.Time
}

func NewTag(params TagParams) (Tag, error) {
	id, idOK := normalizeCatalogID(params.ID)
	catalogID, catalogIDOK := normalizeCatalogID(params.CatalogID)
	name, nameOK := normalizeCatalogName(strings.Join(strings.Fields(params.Name), " "))
	createdAt, updatedAt, timesOK := normalizeCatalogTimes(params.CreatedAt, params.UpdatedAt)
	if !idOK || !catalogIDOK || !nameOK || !timesOK {
		return Tag{}, ErrInvalidTag
	}
	return Tag{
		ID:             id,
		CatalogID:      catalogID,
		Name:           name,
		NormalizedName: strings.ToLower(name),
		CreatedAt:      createdAt,
		UpdatedAt:      updatedAt,
	}, nil
}

type ItemTag struct {
	ID      string
	ItemID  string
	TagID   string
	AddedBy string
	AddedAt time.Time
}

func NewItemTag(id, itemID, tagID, addedBy string, addedAt time.Time) (ItemTag, error) {
	id, idOK := normalizeCatalogID(id)
	itemID, itemIDOK := normalizeCatalogID(itemID)
	tagID, tagIDOK := normalizeCatalogID(tagID)
	addedBy = strings.TrimSpace(addedBy)
	if addedBy != "" {
		var addedByOK bool
		addedBy, addedByOK = normalizeCatalogID(addedBy)
		if !addedByOK {
			return ItemTag{}, ErrInvalidItemTag
		}
	}
	if !idOK || !itemIDOK || !tagIDOK {
		return ItemTag{}, ErrInvalidItemTag
	}
	if addedAt.IsZero() {
		addedAt = time.Now().UTC()
	}
	return ItemTag{ID: id, ItemID: itemID, TagID: tagID, AddedBy: addedBy, AddedAt: addedAt.UTC()}, nil
}

type UserState struct {
	ID           string
	CatalogID    string
	ItemID       string
	UserID       string
	Favorite     bool
	Rating       int
	Progress     float64
	PositionMs   int64
	Locator      string
	Completed    bool
	Revision     int64
	LastOpenedAt *time.Time
	CreatedAt    time.Time
	UpdatedAt    time.Time
}

type UserStateParams struct {
	ID           string
	CatalogID    string
	ItemID       string
	UserID       string
	Favorite     bool
	Rating       int
	Progress     float64
	PositionMs   int64
	Locator      string
	Completed    bool
	Revision     int64
	LastOpenedAt *time.Time
	CreatedAt    *time.Time
	UpdatedAt    *time.Time
}

func NewUserState(params UserStateParams) (UserState, error) {
	id, idOK := normalizeCatalogID(params.ID)
	catalogID, catalogIDOK := normalizeCatalogID(params.CatalogID)
	itemID, itemIDOK := normalizeCatalogID(params.ItemID)
	userID, userIDOK := normalizeCatalogID(params.UserID)
	locator, locatorOK := normalizeCatalogOpaqueValue(params.Locator)
	revision, revisionOK := normalizeCatalogRevision(params.Revision)
	createdAt, updatedAt, timesOK := normalizeCatalogTimes(params.CreatedAt, params.UpdatedAt)
	lastOpenedAt := normalizeOptionalCatalogTime(params.LastOpenedAt)
	if !idOK || !catalogIDOK || !itemIDOK || !userIDOK || !locatorOK || !revisionOK || !timesOK ||
		params.Rating < 0 || params.Rating > 5 || math.IsNaN(params.Progress) || math.IsInf(params.Progress, 0) ||
		params.Progress < 0 || params.Progress > 1 || params.PositionMs < 0 ||
		(params.Completed && params.Progress < 1) || (lastOpenedAt != nil && lastOpenedAt.After(updatedAt)) {
		return UserState{}, ErrInvalidUserState
	}
	return UserState{
		ID: id, CatalogID: catalogID, ItemID: itemID, UserID: userID,
		Favorite: params.Favorite, Rating: params.Rating, Progress: params.Progress,
		PositionMs: params.PositionMs, Locator: locator, Completed: params.Completed,
		Revision: revision, LastOpenedAt: lastOpenedAt, CreatedAt: createdAt, UpdatedAt: updatedAt,
	}, nil
}
