package service

import (
	"testing"
	"time"

	"xiadown/internal/domain/library"
)

func TestSortCatalogItemsSupportsOldestFirstPagination(t *testing.T) {
	older := time.Date(2025, time.January, 1, 0, 0, 0, 0, time.UTC)
	newer := older.Add(24 * time.Hour)
	items := []library.Item{
		catalogSortTestItem(t, "newer", newer),
		catalogSortTestItem(t, "older", older),
	}

	if err := sortCatalogItems(items, "created_asc"); err != nil {
		t.Fatalf("sort oldest first: %v", err)
	}
	if items[0].ID != "older" || items[1].ID != "newer" {
		t.Fatalf("unexpected order: %s, %s", items[0].ID, items[1].ID)
	}
}

func catalogSortTestItem(t *testing.T, id string, createdAt time.Time) library.Item {
	t.Helper()
	item, err := library.NewItem(library.ItemParams{
		ID: id, CatalogID: "catalog", Category: "video", Status: "active",
		Title: id, CreatedAt: &createdAt, UpdatedAt: &createdAt,
	})
	if err != nil {
		t.Fatalf("new item %s: %v", id, err)
	}
	return item
}
