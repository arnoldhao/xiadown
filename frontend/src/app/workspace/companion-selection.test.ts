import { describe, expect, test } from "bun:test";

import {
  appWorkspaceReducer,
  createInitialAppWorkspaceState,
} from "./reducer";
import {
  createCompanionSelectionDestination,
  defineCompanionSelectionContract,
  resolveActiveCompanionSelection,
  resolveActiveCompanionSelectionFromMap,
  resolveActiveCompanionSelectionId,
} from "./companion-selection";
import type { CompanionState } from "./types";

const previewContract = defineCompanionSelectionContract({
  destinationId: "library-preview",
  contextKey: "itemId",
});

function openPreview(itemId = "item-1"): CompanionState {
  return {
    open: true,
    destination: createCompanionSelectionDestination(
      previewContract,
      { kind: "route", workspaceId: "library", routeId: "all" },
      itemId,
    ),
  };
}

describe("Primary and Companion selection contract", () => {
  test("uses the open destination context as the only active selection", () => {
    const item = { id: "item-1", title: "Selected item" };
    const companion = openPreview();

    expect(resolveActiveCompanionSelectionId(companion, previewContract)).toBe(
      "item-1",
    );
    expect(
      resolveActiveCompanionSelection(
        companion,
        previewContract,
        item,
        (candidate) => candidate.id,
      ),
    ).toBe(item);
    expect(
      resolveActiveCompanionSelection(
        companion,
        previewContract,
        { ...item, id: "item-2" },
        (candidate) => candidate.id,
      ),
    ).toBeNull();
  });

  test("releases selection for every closed or replaced Companion state", () => {
    const destination = openPreview().destination;
    const closedStates: CompanionState[] = [
      { open: false, destination },
      { open: false, destination: null },
      {
        open: true,
        destination: { id: "operations", scope: { kind: "global" } },
      },
      {
        open: true,
        destination: {
          id: "library-preview",
          scope: { kind: "global" },
          context: {},
        },
      },
    ];

    closedStates.forEach((companion) => {
      expect(
        resolveActiveCompanionSelectionId(companion, previewContract),
      ).toBeNull();
    });
  });

  test("route navigation closes a scoped Companion and releases Primary", () => {
    let state = createInitialAppWorkspaceState("library");
    state = appWorkspaceReducer(state, {
      type: "navigate",
      location: { routeId: "all" },
    });
    state = appWorkspaceReducer(state, {
      type: "open-companion",
      destination: createCompanionSelectionDestination(
        previewContract,
        { kind: "route", workspaceId: "library", routeId: "all" },
        "item-1",
      ),
    });
    expect(resolveActiveCompanionSelectionId(state.companion, previewContract)).toBe(
      "item-1",
    );

    state = appWorkspaceReducer(state, {
      type: "navigate",
      location: { routeId: "video" },
    });

    expect(state.companion.open).toBe(false);
    expect(resolveActiveCompanionSelectionId(state.companion, previewContract)).toBeNull();
  });

  test("opens the default Library route from a fresh workspace state", () => {
    let state = createInitialAppWorkspaceState("library");

    state = appWorkspaceReducer(state, {
      type: "open-companion",
      destination: createCompanionSelectionDestination(
        previewContract,
        { kind: "route", workspaceId: "library", routeId: "all" },
        "item-1",
      ),
    });

    expect(state.locations.library).toBeUndefined();
    expect(state.companion.open).toBe(true);
    expect(resolveActiveCompanionSelectionId(state.companion, previewContract)).toBe(
      "item-1",
    );
  });

  test("treats selection context as part of a toggled destination identity", () => {
    let state = createInitialAppWorkspaceState("library");
    const destination = (itemId: string) => createCompanionSelectionDestination(
      previewContract,
      { kind: "route", workspaceId: "library", routeId: "all" },
      itemId,
      { presentation: "preview" },
    );
    state = appWorkspaceReducer(state, {
      type: "open-companion",
      destination: destination("item-1"),
    });

    state = appWorkspaceReducer(state, {
      type: "toggle-companion",
      destination: destination("item-2"),
    });

    expect(state.companion.open).toBe(true);
    expect(resolveActiveCompanionSelectionId(state.companion, previewContract)).toBe(
      "item-2",
    );

    state = appWorkspaceReducer(state, {
      type: "toggle-companion",
      destination: destination("item-2"),
    });
    expect(state.companion.open).toBe(false);
  });

  test("normalizes identifiers when creating and resolving destinations", () => {
    const destination = createCompanionSelectionDestination(
      previewContract,
      { kind: "global" },
      "  item-2  ",
      { source: "catalog" },
    );

    expect(destination).toEqual({
      id: "library-preview",
      scope: { kind: "global" },
      context: { source: "catalog", itemId: "item-2" },
    });
    expect(
      resolveActiveCompanionSelectionId(
        { open: true, destination },
        previewContract,
      ),
    ).toBe("item-2");
  });

  test("reconciles refreshes and deletion against the canonical runtime map", () => {
    type PreviewItem = { id: string; progress: number; title: string };
    const clickedSnapshot: PreviewItem = {
      id: "item-1",
      progress: 10,
      title: "Initial title",
    };
    const refreshed: PreviewItem = {
      id: "item-1",
      progress: 72,
      title: "Refreshed title",
    };
    const companion = openPreview();

    const resolved = resolveActiveCompanionSelectionFromMap(
      companion,
      previewContract,
      new Map([[refreshed.id, refreshed]]),
      (item) => item.id,
      { loadingSnapshot: clickedSnapshot, authoritative: true },
    );
    expect(resolved).toEqual({
      id: "item-1",
      item: refreshed,
      status: "resolved",
    });
    expect(resolved.item?.progress).toBe(72);

    expect(
      resolveActiveCompanionSelectionFromMap(
        companion,
        previewContract,
        new Map(),
        (item: PreviewItem) => item.id,
        {
          loading: true,
          loadingSnapshot: clickedSnapshot,
          authoritative: false,
        },
      ),
    ).toEqual({
      id: "item-1",
      item: clickedSnapshot,
      status: "loading",
    });

    expect(
      resolveActiveCompanionSelectionFromMap(
        companion,
        previewContract,
        new Map(),
        (item: PreviewItem) => item.id,
        {
          loading: false,
          loadingSnapshot: clickedSnapshot,
          authoritative: true,
        },
      ),
    ).toEqual({ id: "item-1", item: null, status: "missing" });
  });

  test("wires MainApp to the reconciled item instead of the click snapshot", async () => {
    const source = await Bun.file(
      new URL("../main/MainApp.tsx", import.meta.url),
    ).text();

    expect(source).toContain("resolveActiveCompanionSelectionFromMap(");
    expect(source).toContain("const activeLibraryPreviewItem = libraryPreviewSelection.item");
    expect(source).toContain("selectedItemId={activeLibraryPreviewItem?.id}");
    expect(source).toContain("item={activeLibraryPreviewItem}");
    expect(source).toContain('libraryPreviewSelection.status !== "missing"');
    expect(source).toContain("const openLibraryPreview = React.useCallback(");
    expect(source).toContain("setLibraryPreviewTab(\"preview\")");
    expect(source).toContain("LIBRARY_PREVIEW_SELECTION_CONTRACT");
    expect(source).toContain("onItemClick={openLibraryPreview}");
    expect(source).toContain("adaptLegacyLibraryFiles(libraries, httpBaseURL)");
    expect(source).toContain("[...companionLegacyFileItems, ...candidates]");
    expect(source).toContain("const openLibraryPreviewById = React.useCallback(");
    expect(source).toContain("onOpenItem={openLibraryPreviewById}");
    expect(source).not.toContain("selectedItemId={libraryPreviewLoadingSnapshot?.id}");
  });
});
