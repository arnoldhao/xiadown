import { afterEach, describe, expect, test } from "bun:test";

import { APP_WORKSPACE_STORAGE_KEY, useAppWorkspaceStore } from "./store";
import { APP_STATION_LIMIT, type AppStation } from "./types";

afterEach(() => {
  useAppWorkspaceStore.getState().resetWorkspaceState();
});

describe("app workspace store", () => {
  test("exposes reducer actions without persisting functions", () => {
    const actions = useAppWorkspaceStore.getState();
    actions.activateWorkspace("music", { routeId: "home" });
    actions.openCompanion({ id: "now-playing", scope: { kind: "global" } });

    const current = useAppWorkspaceStore.getState();
    const partialize = useAppWorkspaceStore.persist.getOptions().partialize;
    const persisted = partialize ? partialize(current) : current;

    expect(current.activeWorkspaceId).toBe("music");
    expect(current.locations.music?.routeId).toBe("home");
    expect(JSON.parse(JSON.stringify(persisted))).toEqual({
      schemaVersion: 2,
      activeWorkspaceId: "music",
      locations: { music: { routeId: "home" } },
      stations: [
        {
          id: "library",
          workspaceId: "library",
          label: "Library",
          iconKey: "library",
          order: 0,
          enabled: true,
        },
        {
          id: "music",
          workspaceId: "music",
          label: "Music",
          iconKey: "music",
          order: 1,
          enabled: true,
          editable: true,
        },
        {
          id: "sniff",
          workspaceId: "sniff",
          label: "Sniff",
          iconKey: "sniff",
          order: 2,
          enabled: true,
          editable: true,
        },
        {
          id: "rss",
          workspaceId: "rss",
          label: "RSS",
          iconKey: "rss",
          order: 3,
          enabled: true,
          editable: true,
        },
      ],
      companion: {
        open: false,
        destination: { id: "now-playing", scope: { kind: "global" } },
      },
    });
    expect("dispatch" in persisted).toBe(false);
  });

  test("uses a versioned storage key and keeps actions after reset", () => {
    expect(useAppWorkspaceStore.persist.getOptions().name).toBe(
      APP_WORKSPACE_STORAGE_KEY,
    );
    useAppWorkspaceStore.getState().activateWorkspace("sniff");
    useAppWorkspaceStore.getState().resetWorkspaceState("default");

    expect(useAppWorkspaceStore.getState().activeWorkspaceId).toBe("library");
    expect(typeof useAppWorkspaceStore.getState().activateWorkspace).toBe(
      "function",
    );
  });

  test("round-trips more than five stations while limiting only pinned Dock items", () => {
    const stations = Array.from({ length: APP_STATION_LIMIT + 2 }, (_, index): AppStation => ({
      id: `station-${index}`,
      workspaceId: `workspace-${index}`,
      label: `Station ${index}`,
      order: index,
      enabled: true,
      pinned: true,
    }));
    useAppWorkspaceStore.getState().setStations(stations);

    const options = useAppWorkspaceStore.persist.getOptions();
    const current = useAppWorkspaceStore.getState();
    const persisted = options.partialize ? options.partialize(current) : current;
    const serialized = JSON.parse(JSON.stringify(persisted));

    useAppWorkspaceStore.getState().resetWorkspaceState();
    const rehydrated = options.merge?.(
      serialized,
      useAppWorkspaceStore.getState(),
    ) as ReturnType<typeof useAppWorkspaceStore.getState>;

    expect(rehydrated.stations).toHaveLength(APP_STATION_LIMIT + 2);
    expect(rehydrated.stations.map((station) => station.id)).toEqual(
      stations.map((station) => station.id),
    );
    expect(
      rehydrated.stations.filter((station) => station.pinned !== false),
    ).toHaveLength(APP_STATION_LIMIT);
    expect(rehydrated.stations.slice(APP_STATION_LIMIT)).toEqual(
      expect.arrayContaining([
        expect.objectContaining({ id: "station-5", pinned: false }),
        expect.objectContaining({ id: "station-6", pinned: false }),
      ]),
    );
  });
});
