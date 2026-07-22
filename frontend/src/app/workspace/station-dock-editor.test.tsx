import { describe, expect, test } from "bun:test";
import { renderToStaticMarkup } from "react-dom/server";

import {
  applyStationDockEditorValue,
  moveStationDockEditorItem,
  setStationDockEditorItemVisible,
  stationsToDockEditorValue,
  type StationDockEditorValue,
} from "./station-dock-editor";
import { StationDockEditorForm } from "./StationDockEditor";
import { resolveAppStationCatalog } from "./station-navigation";
import type { AppStation } from "./types";

const persistedStations: AppStation[] = [
  {
    id: "music",
    workspaceId: "music",
    label: "Changed music",
    iconKey: "changed-icon",
    order: 0,
    enabled: true,
    defaultRouteId: "changed-route",
  },
  {
    id: "sniff",
    workspaceId: "sniff",
    label: "Sniff",
    iconKey: "sniff",
    order: 1,
    enabled: true,
  },
];

const labels = {
  title: "dock.editor.title",
  description: "dock.editor.description",
  close: "dock.editor.close",
  visible: "dock.editor.visible",
  moveUp: "dock.editor.move.up",
  moveDown: "dock.editor.move.down",
  actions: {
    cancel: "action.cancel",
    save: "action.save",
  },
};

describe("station Dock catalog and editor", () => {
  test("delegates modal glass and layering to the shared Sheet", async () => {
    const [css, appearance] = await Promise.all([
      Bun.file(new URL("./station-dock-editor.css", import.meta.url)).text(),
      Bun.file(
        new URL("../../shared/styles/dream/workspace.css", import.meta.url),
      ).text(),
    ]);
    const source = await Bun.file(
      new URL("./StationDockEditor.tsx", import.meta.url),
    ).text();

    expect(source).toContain('from "@/shared/ui/sheet"');
    expect(source).toContain("<SheetContent");
    expect(source).toContain("centered");
    expect(source).toContain('size="md"');
    expect(css).not.toContain(".app-station-dock-editor__sheet");
    expect(css).not.toContain("backdrop-filter");
    expect(css).not.toContain("z-index");
    expect(css).not.toContain("--app-glass-");
    expect(appearance).toContain("border-radius: var(--app-radius-card)");
    expect(appearance).toContain(
      "border-radius: var(--app-radius-control-inner)",
    );
    expect(source).toContain("<DreamInlineSwitch");
    expect(source).toContain('size="compactIcon"');
    expect(source).toContain('variant="ghost"');
    expect(css).not.toContain("border-radius");
    expect(appearance).not.toMatch(/border-radius:\s*(?:\d|\.\d)/);
  });

  test("merges old persisted stations with fixed built-in definitions", () => {
    const catalog = resolveAppStationCatalog(persistedStations);

    expect(catalog.map((station) => station.id)).toEqual([
      "music",
      "sniff",
      "library",
      "rss",
      "youtube",
    ]);
    expect(catalog[0]).toMatchObject({
      label: "Music",
      iconKey: "music",
      defaultRouteId: "home",
      pinned: true,
    });
    expect(catalog[1]).toMatchObject({
      label: "Sniff",
      iconKey: "sniff",
      defaultRouteId: "resources",
      pinned: true,
    });
    expect(catalog[2]).toMatchObject({
      label: "Library",
      iconKey: "library",
      defaultRouteId: "all",
      pinned: true,
    });
    expect(catalog[3]).toMatchObject({
      label: "RSS",
      iconKey: "rss",
      defaultRouteId: "all",
      pinned: true,
    });
    expect(catalog[4]).toMatchObject({
      label: "YouTube",
      iconKey: "youtube",
      defaultRouteId: "home",
      pinned: false,
    });
  });

  test("keeps the built-in RSS label uppercase over stale persisted names", () => {
    const [rss] = resolveAppStationCatalog([{
      id: "rss",
      workspaceId: "rss",
      label: "rss",
      iconKey: "rss",
      order: 0,
      enabled: true,
    }]);
    expect(rss).toMatchObject({ id: "rss", label: "RSS" });
  });

  test("builds a visible-first draft and only applies visibility and order", () => {
    const draft = stationsToDockEditorValue(persistedStations);
    expect(draft).toEqual({
      items: [
        { stationId: "music", visible: true },
        { stationId: "sniff", visible: true },
        { stationId: "library", visible: true },
        { stationId: "rss", visible: true },
        { stationId: "youtube", visible: false },
      ],
    });

    const withYouTube = setStationDockEditorItemVisible(
      draft,
      "youtube",
      true,
    );
    const reordered = moveStationDockEditorItem(withYouTube, "youtube", -1);
    const saved = applyStationDockEditorValue(persistedStations, reordered);

    expect(saved.map((station) => [station.id, station.pinned])).toEqual([
      ["music", true],
      ["sniff", true],
      ["library", true],
      ["youtube", true],
      ["rss", true],
    ]);
    expect(saved.map((station) => station.order)).toEqual([0, 1, 2, 3, 4]);
    expect(saved[0]).toMatchObject({
      label: "Music",
      iconKey: "music",
      defaultRouteId: "home",
    });
  });

  test("enforces the five-station Dock limit in malformed drafts", () => {
    const extras = Array.from({ length: 4 }, (_, index): AppStation => ({
      id: `external-${index}`,
      workspaceId: `external-${index}`,
      label: `External ${index}`,
      iconKey: "external",
      order: index + 2,
      enabled: true,
      pinned: false,
    }));
    const catalog = [...persistedStations, ...extras];
    const draft: StationDockEditorValue = {
      items: resolveAppStationCatalog(catalog).map((station) => ({
        stationId: station.id,
        visible: true,
      })),
    };

    const saved = applyStationDockEditorValue(catalog, draft);
    expect(saved.filter((station) => station.pinned !== false)).toHaveLength(5);
  });

  test("renders fixed station rows without single-station metadata fields", () => {
    const catalog = resolveAppStationCatalog(persistedStations);
    const value = stationsToDockEditorValue(catalog);
    const markup = renderToStaticMarkup(
      <StationDockEditorForm
        catalog={catalog}
        labels={labels}
        onCancel={() => undefined}
        onChange={() => undefined}
        onSubmit={() => undefined}
        value={value}
      />,
    );

    expect(markup).toContain('aria-label="dock.editor.title"');
    expect(markup).toContain("Music");
    expect(markup).toContain("Sniff");
    expect(markup).toContain("YouTube");
    expect(markup).toContain('aria-label="dock.editor.visible: Music"');
    expect(markup).toContain('aria-label="dock.editor.move.down: Music"');
    expect(markup).not.toContain('type="text"');
    expect(markup).not.toContain('type="number"');
    expect(markup).not.toContain("defaultRoute");
  });
});
