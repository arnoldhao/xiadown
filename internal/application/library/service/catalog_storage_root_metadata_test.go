package service

import (
	"context"
	"testing"
	"time"

	"xiadown/internal/domain/library"
)

type catalogStorageRootMetadataCatalogRepository struct {
	items []library.Catalog
}

func (repo catalogStorageRootMetadataCatalogRepository) List(
	context.Context,
) ([]library.Catalog, error) {
	return append([]library.Catalog(nil), repo.items...), nil
}

func (repo catalogStorageRootMetadataCatalogRepository) Get(
	context.Context,
	string,
) (library.Catalog, error) {
	return repo.items[0], nil
}

func (catalogStorageRootMetadataCatalogRepository) Save(
	context.Context,
	library.Catalog,
) error {
	return nil
}

func (catalogStorageRootMetadataCatalogRepository) Delete(
	context.Context,
	string,
) error {
	return nil
}

type catalogStorageRootMetadataRepository struct {
	items []library.StorageRoot
}

func (repo catalogStorageRootMetadataRepository) ListByCatalogID(
	_ context.Context,
	catalogID string,
) ([]library.StorageRoot, error) {
	result := make([]library.StorageRoot, 0, len(repo.items))
	for _, item := range repo.items {
		if item.CatalogID == catalogID {
			result = append(result, item)
		}
	}
	return result, nil
}

func (repo catalogStorageRootMetadataRepository) Get(
	context.Context,
	string,
) (library.StorageRoot, error) {
	return repo.items[0], nil
}

func (catalogStorageRootMetadataRepository) Save(
	context.Context,
	library.StorageRoot,
) error {
	return nil
}

func (catalogStorageRootMetadataRepository) Delete(
	context.Context,
	string,
) error {
	return nil
}

func TestListCatalogStorageRootMetadataUsesOnlyBoundedRepositories(t *testing.T) {
	now := time.Date(2026, 7, 27, 10, 0, 0, 0, time.UTC)
	catalog, err := library.NewCatalog(library.CatalogParams{
		ID: "catalog-metadata", Name: "Library", Status: "active",
		IsDefault: true, CreatedAt: &now, UpdatedAt: &now,
	})
	if err != nil {
		t.Fatal(err)
	}
	root, err := library.NewStorageRoot(library.StorageRootParams{
		ID: "root-metadata", CatalogID: catalog.ID, Name: "Archive",
		Path: t.TempDir(), VolumeID: "volume-metadata", Mode: "referenced",
		Status: "read_only", CreatedAt: &now, UpdatedAt: &now,
	})
	if err != nil {
		t.Fatal(err)
	}
	service := &CatalogService{
		catalogs: catalogStorageRootMetadataCatalogRepository{
			items: []library.Catalog{catalog},
		},
		roots: catalogStorageRootMetadataRepository{
			items: []library.StorageRoot{root},
		},
	}

	items, err := service.ListCatalogStorageRootMetadata(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if len(items) != 1 ||
		items[0].ID != root.ID ||
		items[0].Name != root.Name ||
		items[0].Path != root.Path ||
		items[0].VolumeID != root.VolumeID ||
		items[0].Mode != root.Mode ||
		items[0].Status != root.Status {
		t.Fatalf("unexpected storage root metadata: %#v", items)
	}
}
