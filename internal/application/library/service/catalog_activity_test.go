package service

import (
	"testing"
	"time"

	"xiadown/internal/domain/library"
)

func TestProjectCatalogItemActivityClassifiesRestoreAndReturnsNewestFirst(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, 7, 20, 16, 0, 0, 0, time.UTC)
	changes := []library.CatalogChange{
		{Kind: library.CatalogChangeUpsert, Revision: 2, ActorID: "desktop-user", OccurredAt: now},
		{Kind: library.CatalogChangeDelete, Revision: 3, ActorID: "desktop-user", OccurredAt: now.Add(time.Minute)},
		{Kind: library.CatalogChangeUpsert, Revision: 4, ActorID: "desktop-user", OccurredAt: now.Add(2 * time.Minute)},
		{Kind: library.CatalogChangeUpsert, Revision: 5, ActorID: "desktop-user", OccurredAt: now.Add(3 * time.Minute)},
	}

	result := projectCatalogItemActivity(changes, 3)
	if len(result) != 3 || result[0].Action != catalogItemActivityUpdated || result[0].Revision != 5 ||
		result[1].Action != catalogItemActivityRestored || result[1].Revision != 4 ||
		result[2].Action != catalogItemActivityTrashed || result[2].Revision != 3 {
		t.Fatalf("unexpected projected activity: %#v", result)
	}
	if result[0].OccurredAt != now.Add(3*time.Minute).Format(time.RFC3339) {
		t.Fatalf("unexpected activity timestamp: %#v", result[0])
	}
}

func TestCatalogActivityUserActorFiltersSynchronizationNoise(t *testing.T) {
	t.Parallel()

	for _, actor := range []string{"", " ", "migration", "migration:v2", "system", "system/projection", "backfill-worker"} {
		if catalogActivityUserActor(actor) {
			t.Fatalf("noise actor %q was treated as a user", actor)
		}
	}
	for _, actor := range []string{"desktop-user", "desktop-library", "local:desktop", "person-systematic"} {
		if !catalogActivityUserActor(actor) {
			t.Fatalf("user actor %q was filtered", actor)
		}
	}
}

func TestProjectCatalogItemActivityLetsHiddenRestoreCloseTrashState(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, 7, 20, 17, 0, 0, 0, time.UTC)
	changes := []library.CatalogChange{
		{Kind: library.CatalogChangeDelete, Revision: 2, ActorID: "desktop-library", OccurredAt: now},
		// Older maintenance builds omitted actorId when restoring an item.
		{Kind: library.CatalogChangeUpsert, Revision: 3, ActorID: "", OccurredAt: now.Add(time.Minute)},
		{Kind: library.CatalogChangeUpsert, Revision: 4, ActorID: "desktop-library", OccurredAt: now.Add(2 * time.Minute)},
	}

	result := projectCatalogItemActivity(changes, 20)
	if len(result) != 2 || result[0].Revision != 4 || result[0].Action != catalogItemActivityUpdated ||
		result[1].Revision != 2 || result[1].Action != catalogItemActivityTrashed {
		t.Fatalf("hidden restore leaked into the next user action: %#v", result)
	}
}

func TestCatalogItemActivityLimitIsBounded(t *testing.T) {
	t.Parallel()

	if normalizeCatalogItemActivityLimit(0) != defaultCatalogItemActivityLimit ||
		normalizeCatalogItemActivityLimit(-1) != defaultCatalogItemActivityLimit ||
		normalizeCatalogItemActivityLimit(101) != maximumCatalogItemActivityLimit ||
		normalizeCatalogItemActivityLimit(7) != 7 {
		t.Fatal("catalog activity limit normalization changed")
	}
}
