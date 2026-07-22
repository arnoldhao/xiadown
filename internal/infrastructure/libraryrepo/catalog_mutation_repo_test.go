package libraryrepo

import (
	"context"
	"database/sql"
	"errors"
	"path/filepath"
	"testing"
	"time"

	"xiadown/internal/domain/library"
	"xiadown/internal/infrastructure/persistence"
)

func TestSQLiteCatalogMutationRepositoryIsAtomicAndRevisionGuarded(t *testing.T) {
	ctx := context.Background()
	db, err := persistence.OpenSQLite(ctx, persistence.SQLiteConfig{Path: filepath.Join(t.TempDir(), "catalog-mutations.db")})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	defer db.Close()
	now := time.Date(2026, 7, 13, 14, 0, 0, 0, time.UTC)
	catalog, _ := library.NewCatalog(library.CatalogParams{
		ID: "catalog-1", Name: "Library", Status: "active", IsDefault: true,
		CreatedAt: &now, UpdatedAt: &now,
	})
	if err := NewSQLiteCatalogRepository(db.Bun).Save(ctx, catalog); err != nil {
		t.Fatalf("save catalog: %v", err)
	}
	item, _ := library.NewItem(library.ItemParams{
		ID: "item-1", CatalogID: catalog.ID, Category: "book", Status: "active", Title: "Book",
		Revision: 1, CreatedAt: &now, UpdatedAt: &now,
	})
	items := NewSQLiteCatalogItemRepository(db.Bun)
	if err := items.Save(ctx, item); err != nil {
		t.Fatalf("save item: %v", err)
	}
	writer := NewSQLiteCatalogMutationRepository(db.Bun)
	updatedAt := now.Add(time.Minute)
	updated, _ := library.NewItem(library.ItemParams{
		ID: item.ID, CatalogID: item.CatalogID, Category: "book", Status: "active", Title: "Edited Book",
		Revision: 2, CreatedAt: &now, UpdatedAt: &updatedAt,
	})

	if _, err := db.SQL.ExecContext(ctx, `
CREATE TRIGGER reject_catalog_change
BEFORE INSERT ON library_catalog_changes
WHEN NEW.entity_id = 'item-1'
BEGIN SELECT RAISE(ABORT, 'change feed unavailable'); END
`); err != nil {
		t.Fatalf("create rejecting trigger: %v", err)
	}
	if err := writer.SaveItemMutation(ctx, updated, 1, library.CatalogChangeUpsert, "actor-1", nil); err == nil {
		t.Fatal("expected rejected change feed write")
	}
	loaded, err := items.Get(ctx, item.ID)
	if err != nil || loaded.Revision != 1 || loaded.Title != "Book" {
		t.Fatalf("item escaped rolled back transaction: %#v, err=%v", loaded, err)
	}
	if _, err := db.SQL.ExecContext(ctx, "DROP TRIGGER reject_catalog_change"); err != nil {
		t.Fatalf("drop rejecting trigger: %v", err)
	}
	if err := writer.SaveItemMutation(ctx, updated, 1, library.CatalogChangeUpsert, "actor-1", nil); err != nil {
		t.Fatalf("save item mutation: %v", err)
	}
	if err := writer.SaveItemMutation(ctx, updated, 1, library.CatalogChangeUpsert, "actor-1", nil); !errors.Is(err, library.ErrCatalogRevisionConflict) {
		t.Fatalf("expected stale mutation rejection, got %v", err)
	}

	trashedAt := now.Add(2 * time.Minute)
	trashed, _ := library.NewItem(library.ItemParams{
		ID: item.ID, CatalogID: item.CatalogID, Category: "book", Status: "trashed", Title: "Edited Book",
		Revision: 3, TrashedAt: &trashedAt, CreatedAt: &now, UpdatedAt: &trashedAt,
	})
	expiresAt := trashedAt.Add(90 * 24 * time.Hour)
	if err := writer.SaveItemMutation(ctx, trashed, 2, library.CatalogChangeDelete, "actor-1", &expiresAt); err != nil {
		t.Fatalf("trash item: %v", err)
	}
	var tombstoneRevision int64
	if err := db.SQL.QueryRowContext(ctx, `
SELECT revision FROM library_catalog_tombstones
WHERE catalog_id = ? AND entity_type = 'item' AND entity_id = ?
`, catalog.ID, item.ID).Scan(&tombstoneRevision); err != nil || tombstoneRevision != 3 {
		t.Fatalf("unexpected tombstone revision=%d err=%v", tombstoneRevision, err)
	}

	state, _ := library.NewUserState(library.UserStateParams{
		ID: "state-1", CatalogID: catalog.ID, ItemID: item.ID, UserID: "user-1",
		Favorite: true, Revision: 1, CreatedAt: &now, UpdatedAt: &now,
	})
	if err := writer.SaveUserStateMutation(ctx, state, 0, "user-1"); err != nil {
		t.Fatalf("create user state mutation: %v", err)
	}
	if err := writer.SaveUserStateMutation(ctx, state, 0, "user-1"); !errors.Is(err, library.ErrCatalogRevisionConflict) {
		t.Fatalf("expected duplicate user state conflict, got %v", err)
	}
}

func TestSQLiteCatalogTaxonomyMutationsAdvanceCursorAndRollbackAtomically(t *testing.T) {
	ctx := context.Background()
	db, err := persistence.OpenSQLite(ctx, persistence.SQLiteConfig{Path: filepath.Join(t.TempDir(), "catalog-taxonomy-mutations.db")})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	defer db.Close()
	now := time.Date(2026, 7, 13, 15, 0, 0, 0, time.UTC)
	catalog, _ := library.NewCatalog(library.CatalogParams{
		ID: "catalog-taxonomy", Name: "Library", Status: "active", IsDefault: true,
		CreatedAt: &now, UpdatedAt: &now,
	})
	if err := NewSQLiteCatalogRepository(db.Bun).Save(ctx, catalog); err != nil {
		t.Fatalf("save catalog: %v", err)
	}
	item, _ := library.NewItem(library.ItemParams{
		ID: "item-taxonomy", CatalogID: catalog.ID, Category: "video", Status: "active", Title: "Film",
		Revision: 1, CreatedAt: &now, UpdatedAt: &now,
	})
	if err := NewSQLiteCatalogItemRepository(db.Bun).Save(ctx, item); err != nil {
		t.Fatalf("save item: %v", err)
	}

	writer := NewSQLiteCatalogMutationRepository(db.Bun)
	tags := NewSQLiteCatalogTagRepository(db.Bun)
	changes := NewSQLiteCatalogChangeRepository(db.Bun)
	cursor := taxonomyCursor(t, ctx, db.SQL, catalog.ID)
	tag, _ := library.NewTag(library.TagParams{
		ID: "tag-cinema", CatalogID: catalog.ID, Name: "Cinema", CreatedAt: &now, UpdatedAt: &now,
	})
	if err := writer.SaveTagMutation(ctx, tag, "desktop-user"); err != nil {
		t.Fatalf("create tag mutation: %v", err)
	}
	createdChanges, err := changes.ListAfter(ctx, catalog.ID, cursor, 10)
	if err != nil || len(createdChanges) != 1 {
		t.Fatalf("tag changes after cursor %d: %#v, err=%v", cursor, createdChanges, err)
	}
	if createdChanges[0].EntityType != library.CatalogEntityTag || createdChanges[0].EntityID != tag.ID ||
		createdChanges[0].Kind != library.CatalogChangeUpsert || createdChanges[0].Revision != 1 {
		t.Fatalf("unexpected created tag change: %#v", createdChanges[0])
	}
	cursor = createdChanges[0].Sequence

	updatedAt := now.Add(time.Minute)
	updatedTag, _ := library.NewTag(library.TagParams{
		ID: tag.ID, CatalogID: catalog.ID, Name: "Classic Cinema", CreatedAt: &now, UpdatedAt: &updatedAt,
	})
	if err := writer.SaveTagMutation(ctx, updatedTag, "desktop-user"); err != nil {
		t.Fatalf("update tag mutation: %v", err)
	}
	updatedChanges, err := changes.ListAfter(ctx, catalog.ID, cursor, 10)
	if err != nil || len(updatedChanges) != 1 || updatedChanges[0].EntityID != tag.ID || updatedChanges[0].Revision != 2 {
		t.Fatalf("updated tag change after cursor %d: %#v, err=%v", cursor, updatedChanges, err)
	}
	cursor = updatedChanges[0].Sequence

	binding, _ := library.NewItemTag("binding-cinema", item.ID, tag.ID, "desktop-user", now.Add(2*time.Minute))
	if err := writer.ReplaceItemTagsMutation(
		ctx, catalog.ID, item.ID, []library.ItemTag{binding}, "desktop-user", now.Add(2*time.Minute),
	); err != nil {
		t.Fatalf("replace item tags mutation: %v", err)
	}
	membershipChanges, err := changes.ListAfter(ctx, catalog.ID, cursor, 10)
	if err != nil || len(membershipChanges) != 2 {
		t.Fatalf("item-tag changes after cursor %d: %#v, err=%v", cursor, membershipChanges, err)
	}
	if membershipChanges[0].EntityType != library.CatalogEntityItemTag || membershipChanges[0].EntityID != item.ID ||
		membershipChanges[0].Kind != library.CatalogChangeUpsert || membershipChanges[0].Revision != 1 {
		t.Fatalf("unexpected item-tag aggregate change: %#v", membershipChanges[0])
	}
	if membershipChanges[1].EntityType != library.CatalogEntityItem || membershipChanges[1].EntityID != item.ID {
		t.Fatalf("missing owning item invalidation: %#v", membershipChanges)
	}
	storedBindings, err := tags.ListByItemID(ctx, item.ID)
	if err != nil || len(storedBindings) != 1 || storedBindings[0].ID != binding.ID {
		t.Fatalf("stored item-tag membership: %#v, err=%v", storedBindings, err)
	}
	cursor = membershipChanges[1].Sequence

	if err := writer.ReplaceItemTagsMutation(
		ctx, catalog.ID, item.ID, nil, "desktop-user", now.Add(3*time.Minute),
	); err != nil {
		t.Fatalf("clear item tags mutation: %v", err)
	}
	clearedChanges, err := changes.ListAfter(ctx, catalog.ID, cursor, 10)
	if err != nil || len(clearedChanges) != 2 || clearedChanges[0].EntityID != item.ID ||
		clearedChanges[0].Revision != 2 || clearedChanges[0].Kind != library.CatalogChangeUpsert {
		t.Fatalf("cleared item-tag aggregate change after cursor %d: %#v, err=%v", cursor, clearedChanges, err)
	}
	if clearedChanges[1].EntityType != library.CatalogEntityItem || clearedChanges[1].EntityID != item.ID {
		t.Fatalf("cleared membership missing owning item invalidation: %#v", clearedChanges)
	}
	storedBindings, err = tags.ListByItemID(ctx, item.ID)
	if err != nil || len(storedBindings) != 0 {
		t.Fatalf("cleared item-tag membership: %#v, err=%v", storedBindings, err)
	}

	// Restore one binding, then make all taxonomy change inserts fail. Both
	// the entity/member rows and the high-water cursor must stay unchanged.
	if err := writer.ReplaceItemTagsMutation(
		ctx, catalog.ID, item.ID, []library.ItemTag{binding}, "desktop-user", now.Add(4*time.Minute),
	); err != nil {
		t.Fatalf("restore item tags before rollback test: %v", err)
	}
	rollbackCursor := taxonomyCursor(t, ctx, db.SQL, catalog.ID)
	if _, err := db.SQL.ExecContext(ctx, `
CREATE TRIGGER reject_taxonomy_change
BEFORE INSERT ON library_catalog_changes
WHEN NEW.entity_type IN ('tag', 'item_tag')
BEGIN SELECT RAISE(ABORT, 'taxonomy change feed unavailable'); END
`); err != nil {
		t.Fatalf("create taxonomy change rejection trigger: %v", err)
	}
	failedAt := now.Add(5 * time.Minute)
	failedTag, _ := library.NewTag(library.TagParams{
		ID: tag.ID, CatalogID: catalog.ID, Name: "Should Roll Back", CreatedAt: &now, UpdatedAt: &failedAt,
	})
	if err := writer.SaveTagMutation(ctx, failedTag, "desktop-user"); err == nil {
		t.Fatal("tag mutation unexpectedly survived rejected change insert")
	}
	loadedTags, err := tags.ListByCatalogID(ctx, catalog.ID)
	if err != nil || len(loadedTags) != 1 || loadedTags[0].Name != updatedTag.Name {
		t.Fatalf("tag entity escaped rolled-back transaction: %#v, err=%v", loadedTags, err)
	}
	if err := writer.ReplaceItemTagsMutation(
		ctx, catalog.ID, item.ID, nil, "desktop-user", failedAt,
	); err == nil {
		t.Fatal("item-tag mutation unexpectedly survived rejected change insert")
	}
	storedBindings, err = tags.ListByItemID(ctx, item.ID)
	if err != nil || len(storedBindings) != 1 || storedBindings[0].ID != binding.ID {
		t.Fatalf("item-tag members escaped rolled-back transaction: %#v, err=%v", storedBindings, err)
	}
	if cursorAfterRollback := taxonomyCursor(t, ctx, db.SQL, catalog.ID); cursorAfterRollback != rollbackCursor {
		t.Fatalf("cursor advanced across rolled-back taxonomy mutations: %d -> %d", rollbackCursor, cursorAfterRollback)
	}
}

func taxonomyCursor(t *testing.T, ctx context.Context, db interface {
	QueryRowContext(context.Context, string, ...any) *sql.Row
}, catalogID string) int64 {
	t.Helper()
	var cursor int64
	if err := db.QueryRowContext(ctx, `
SELECT COALESCE(MAX(sequence), 0)
FROM library_catalog_changes
WHERE catalog_id = ?
`, catalogID).Scan(&cursor); err != nil {
		t.Fatalf("read taxonomy cursor: %v", err)
	}
	return cursor
}
