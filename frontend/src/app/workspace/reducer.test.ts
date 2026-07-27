import { describe, expect, test } from "bun:test";

import {
  appWorkspaceReducer,
  createInitialAppWorkspaceState,
  normalizeAppStations,
  normalizeAppWorkspaceState,
} from "./reducer";
import { APP_STATION_LIMIT, type AppStation } from "./types";

function station(index: number, overrides: Partial<AppStation> = {}): AppStation {
  return {
    id: `station-${index}`,
    workspaceId: `workspace-${index}`,
    label: `Station ${index}`,
    order: index,
    enabled: true,
    ...overrides,
  };
}

describe("app workspace reducer", () => {
  test("normalizes, sorts, and deduplicates without truncating the station catalog", () => {
    const stations = normalizeAppStations([
      station(6, { order: 6 }),
      station(2, { order: 2 }),
      station(1, { order: 1 }),
      station(1, { label: "Duplicate", order: 0 }),
      station(4, { order: 4 }),
      station(3, { order: 3 }),
      station(5, { order: 5 }),
      station(0, { label: "  " }),
    ]);

    expect(stations).toHaveLength(6);
    expect(stations.map((item) => item.id)).toEqual([
      "station-1",
      "station-2",
      "station-3",
      "station-4",
      "station-5",
      "station-6",
    ]);
    expect(stations.map((item) => item.order)).toEqual([0, 1, 2, 3, 4, 5]);
    expect(stations.filter((item) => item.pinned !== false)).toHaveLength(
      APP_STATION_LIMIT,
    );
    expect(stations.at(-1)?.pinned).toBe(false);
  });

  test("updates a normalized station id and retains an unpinned sixth station", () => {
    const initial = {
      ...createInitialAppWorkspaceState(),
      stations: Array.from({ length: APP_STATION_LIMIT }, (_, index) =>
        station(index),
      ),
    };
    const updated = appWorkspaceReducer(initial, {
      type: "upsert-station",
      station: station(1, { id: " station-1 ", label: "Updated" }),
    });
    const expanded = appWorkspaceReducer(updated, {
      type: "upsert-station",
      station: station(8),
    });

    expect(updated.stations).toHaveLength(APP_STATION_LIMIT);
    expect(updated.stations.find((item) => item.id === "station-1")?.label).toBe(
      "Updated",
    );
    expect(expanded.stations).toHaveLength(APP_STATION_LIMIT + 1);
    expect(expanded.stations.at(-1)).toMatchObject({
      id: "station-8",
      pinned: false,
    });
  });

  test("honors an edited station order before normalizing the dock", () => {
    const initial = {
      ...createInitialAppWorkspaceState(),
      stations: Array.from({ length: APP_STATION_LIMIT }, (_, index) =>
        station(index),
      ),
    };

    const updated = appWorkspaceReducer(initial, {
      type: "upsert-station",
      station: station(1, { order: 4 }),
    });

    expect(updated.stations.map((item) => item.id)).toEqual([
      "station-0",
      "station-2",
      "station-3",
      "station-1",
      "station-4",
    ]);
    expect(updated.stations.map((item) => item.order)).toEqual([0, 1, 2, 3, 4]);
  });

  test("preserves editable dock and default-route station settings", () => {
    const [normalized] = normalizeAppStations([
      station(1, {
        pinned: false,
        defaultRouteId: " radio ",
      }),
    ]);

    expect(normalized?.pinned).toBe(false);
    expect(normalized?.defaultRouteId).toBe("radio");
  });

  test("preserves a route location for every workspace", () => {
    let state = createInitialAppWorkspaceState();
    state = appWorkspaceReducer(state, {
      type: "navigate",
      location: { routeId: "downloads" },
    });
    state = appWorkspaceReducer(state, {
      type: "activate-workspace",
      workspaceId: "music",
      location: { routeId: "home", params: { source: "ytm" } },
    });
    state = appWorkspaceReducer(state, {
      type: "activate-workspace",
      workspaceId: "default",
    });

    expect(state.activeWorkspaceId).toBe("library");
    expect(state.locations.library?.routeId).toBe("all");
    expect(state.locations.music).toEqual({
      routeId: "home",
      params: { source: "ytm" },
    });
  });

  test("closes route-scoped companion content when its route is left", () => {
    let state = createInitialAppWorkspaceState("music");
    state = appWorkspaceReducer(state, {
      type: "navigate",
      location: { routeId: "home" },
    });
    state = appWorkspaceReducer(state, {
      type: "open-companion",
      destination: {
        id: "lyrics",
        scope: { kind: "route", workspaceId: "music", routeId: "home" },
      },
    });
    expect(state.companion.open).toBe(true);

    state = appWorkspaceReducer(state, {
      type: "navigate",
      location: { routeId: "search" },
    });
    expect(state.companion.open).toBe(false);
    expect(state.companion.destination?.id).toBe("lyrics");

    state = appWorkspaceReducer(state, {
      type: "toggle-companion",
    });
    expect(state.companion.open).toBe(false);

    state = appWorkspaceReducer(state, {
      type: "navigate",
      location: { routeId: "home" },
    });
    state = appWorkspaceReducer(state, { type: "toggle-companion" });
    expect(state.companion.open).toBe(true);
  });

  test("keeps global companion content open across workspace switches", () => {
    let state = createInitialAppWorkspaceState();
    state = appWorkspaceReducer(state, {
      type: "open-companion",
      destination: { id: "downloads", scope: { kind: "global" } },
    });
    state = appWorkspaceReducer(state, {
      type: "activate-workspace",
      workspaceId: "sniff",
    });

    expect(state.companion).toEqual({
      open: true,
      destination: { id: "downloads", scope: { kind: "global" } },
    });
  });

  test("closes workspace-only companion content when another station opens", () => {
    let state = createInitialAppWorkspaceState("music");
    state = appWorkspaceReducer(state, {
      type: "open-companion",
      destination: {
        id: "lyrics",
        scope: { kind: "workspace", workspaceId: "music" },
      },
    });
    state = appWorkspaceReducer(state, {
      type: "activate-workspace",
      workspaceId: "sniff",
    });

    expect(state.companion.open).toBe(false);
    expect(state.companion.destination?.id).toBe("lyrics");
  });

  test("restoring a partial snapshot preserves fallback slices", () => {
    const fallback = {
      ...createInitialAppWorkspaceState(),
      locations: { default: { routeId: "tasks" } },
      stations: [station(1)],
    };
    const restored = normalizeAppWorkspaceState(
      { activeWorkspaceId: "music" },
      fallback,
    );

    expect(restored.activeWorkspaceId).toBe("music");
    expect(restored.locations).toEqual({
      library: { routeId: "ended", params: undefined },
    });
    expect(restored.stations).toEqual([
      { ...fallback.stations[0], order: 0 },
    ]);
    expect(() =>
      normalizeAppWorkspaceState({ stations: {} as AppStation[] }),
    ).not.toThrow();
  });

  test("migrates the v1 default workspace, routes, and companion scope", () => {
    const restored = normalizeAppWorkspaceState({
      schemaVersion: 1 as never,
      activeWorkspaceId: "default",
      locations: {
        default: { routeId: "connections" },
      },
      companion: {
        open: true,
        destination: {
          id: "legacy-preview",
          scope: {
            kind: "route",
            workspaceId: "default",
            routeId: "petsGallery",
          },
        },
      },
    });

    expect(restored.schemaVersion).toBe(2);
    expect(restored.activeWorkspaceId).toBe("library");
    expect(restored.locations.library?.routeId).toBe("app-sessions");
    expect(restored.companion.destination?.scope).toEqual({
      kind: "route",
      workspaceId: "library",
      routeId: "pet-gallery",
    });
    expect(restored.companion.open).toBe(false);
  });

  test("migrates every legacy terminal-task route to Ended", () => {
    for (const routeId of ["pending", "tasks", "completed"]) {
      const restored = normalizeAppWorkspaceState({
        locations: {
          library: { routeId },
        },
      });
      expect(restored.locations.library?.routeId).toBe("ended");
    }
  });
});
