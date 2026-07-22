package library

import "time"

type ItemCategory string

const (
	ItemCategoryVideo ItemCategory = "video"
	ItemCategoryAudio ItemCategory = "audio"
	ItemCategoryBook  ItemCategory = "book"
	ItemCategoryImage ItemCategory = "image"
	ItemCategoryOther ItemCategory = "other"
)

type ItemStatus string

const (
	ItemStatusActive      ItemStatus = "active"
	ItemStatusNeedsReview ItemStatus = "needs_review"
	ItemStatusMissing     ItemStatus = "missing"
	ItemStatusTrashed     ItemStatus = "trashed"
)

type Item struct {
	ID          string
	CatalogID   string
	Category    ItemCategory
	Status      ItemStatus
	Title       string
	SortTitle   string
	Description string
	Revision    int64
	TrashedAt   *time.Time
	CreatedAt   time.Time
	UpdatedAt   time.Time
}

type ItemParams struct {
	ID          string
	CatalogID   string
	Category    string
	Status      string
	Title       string
	SortTitle   string
	Description string
	Revision    int64
	TrashedAt   *time.Time
	CreatedAt   *time.Time
	UpdatedAt   *time.Time
}

func NewItem(params ItemParams) (Item, error) {
	id, idOK := normalizeCatalogID(params.ID)
	catalogID, catalogIDOK := normalizeCatalogID(params.CatalogID)
	title, titleOK := normalizeCatalogName(params.Title)
	sortTitle, sortTitleOK := normalizeCatalogDescription(params.SortTitle)
	description, descriptionOK := normalizeCatalogDescription(params.Description)
	category := ItemCategory(normalizeCatalogEnum(params.Category))
	status := ItemStatus(normalizeCatalogEnum(params.Status))
	if status == "" {
		status = ItemStatusActive
	}
	revision, revisionOK := normalizeCatalogRevision(params.Revision)
	createdAt, updatedAt, timesOK := normalizeCatalogTimes(params.CreatedAt, params.UpdatedAt)
	trashedAt := normalizeOptionalCatalogTime(params.TrashedAt)
	if !idOK || !catalogIDOK || !titleOK || !sortTitleOK || !descriptionOK || !revisionOK || !timesOK {
		return Item{}, ErrInvalidCatalogItem
	}
	switch category {
	case ItemCategoryVideo, ItemCategoryAudio, ItemCategoryBook, ItemCategoryImage, ItemCategoryOther:
	default:
		return Item{}, ErrInvalidCatalogItem
	}
	switch status {
	case ItemStatusActive, ItemStatusNeedsReview, ItemStatusMissing:
		if trashedAt != nil {
			return Item{}, ErrInvalidCatalogItem
		}
	case ItemStatusTrashed:
		if trashedAt == nil || trashedAt.Before(createdAt) {
			return Item{}, ErrInvalidCatalogItem
		}
	default:
		return Item{}, ErrInvalidCatalogItem
	}
	if sortTitle == "" {
		sortTitle = title
	}
	return Item{
		ID:          id,
		CatalogID:   catalogID,
		Category:    category,
		Status:      status,
		Title:       title,
		SortTitle:   sortTitle,
		Description: description,
		Revision:    revision,
		TrashedAt:   trashedAt,
		CreatedAt:   createdAt,
		UpdatedAt:   updatedAt,
	}, nil
}

type ItemAssetRole string

const (
	ItemAssetRoleOriginal       ItemAssetRole = "original"
	ItemAssetRoleRepresentation ItemAssetRole = "representation"
	ItemAssetRoleAttachment     ItemAssetRole = "attachment"
	ItemAssetRoleArtwork        ItemAssetRole = "artwork"
)

// ItemAsset binds one logical Item to a stable legacy LibraryFile ID. Role is
// explicit so subtitles, covers, and transcodes do not pollute the main list.
type ItemAsset struct {
	ID        string
	ItemID    string
	FileID    string
	Role      ItemAssetRole
	Label     string
	Position  int
	CreatedAt time.Time
	UpdatedAt time.Time
}

type ItemAssetParams struct {
	ID        string
	ItemID    string
	FileID    string
	Role      string
	Label     string
	Position  int
	CreatedAt *time.Time
	UpdatedAt *time.Time
}

func NewItemAsset(params ItemAssetParams) (ItemAsset, error) {
	id, idOK := normalizeCatalogID(params.ID)
	itemID, itemIDOK := normalizeCatalogID(params.ItemID)
	fileID, fileIDOK := normalizeCatalogID(params.FileID)
	label, labelOK := normalizeCatalogNameOrEmpty(params.Label)
	role := ItemAssetRole(normalizeCatalogEnum(params.Role))
	createdAt, updatedAt, timesOK := normalizeCatalogTimes(params.CreatedAt, params.UpdatedAt)
	if !idOK || !itemIDOK || !fileIDOK || !labelOK || params.Position < 0 || !timesOK {
		return ItemAsset{}, ErrInvalidItemAsset
	}
	switch role {
	case ItemAssetRoleOriginal, ItemAssetRoleRepresentation, ItemAssetRoleAttachment, ItemAssetRoleArtwork:
	default:
		return ItemAsset{}, ErrInvalidItemAsset
	}
	return ItemAsset{
		ID:        id,
		ItemID:    itemID,
		FileID:    fileID,
		Role:      role,
		Label:     label,
		Position:  params.Position,
		CreatedAt: createdAt,
		UpdatedAt: updatedAt,
	}, nil
}

func normalizeCatalogNameOrEmpty(value string) (string, bool) {
	if normalizeCatalogEnum(value) == "" {
		return "", true
	}
	return normalizeCatalogName(value)
}
