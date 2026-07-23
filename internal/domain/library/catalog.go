package library

import "time"

type CatalogStatus string

const (
	CatalogStatusActive   CatalogStatus = "active"
	CatalogStatusArchived CatalogStatus = "archived"
)

// Catalog is a user-facing library. It intentionally sits above the legacy
// Library bundle so existing downloads can be projected without changing their
// identity or physical location.
type Catalog struct {
	ID          string
	Name        string
	Description string
	Status      CatalogStatus
	IsDefault   bool
	CreatedAt   time.Time
	UpdatedAt   time.Time
}

type CatalogParams struct {
	ID          string
	Name        string
	Description string
	Status      string
	IsDefault   bool
	CreatedAt   *time.Time
	UpdatedAt   *time.Time
}

func NewCatalog(params CatalogParams) (Catalog, error) {
	id, idOK := normalizeCatalogID(params.ID)
	name, nameOK := normalizeCatalogName(params.Name)
	description, descriptionOK := normalizeCatalogDescription(params.Description)
	status := CatalogStatus(normalizeCatalogEnum(params.Status))
	if status == "" {
		status = CatalogStatusActive
	}
	createdAt, updatedAt, timesOK := normalizeCatalogTimes(params.CreatedAt, params.UpdatedAt)
	if !idOK || !nameOK || !descriptionOK || !timesOK {
		return Catalog{}, ErrInvalidCatalog
	}
	switch status {
	case CatalogStatusActive, CatalogStatusArchived:
	default:
		return Catalog{}, ErrInvalidCatalog
	}
	return Catalog{
		ID:          id,
		Name:        name,
		Description: description,
		Status:      status,
		IsDefault:   params.IsDefault,
		CreatedAt:   createdAt,
		UpdatedAt:   updatedAt,
	}, nil
}
