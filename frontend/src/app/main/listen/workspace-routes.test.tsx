import { describe, expect, test } from "bun:test";

import { LISTEN_LIKED_SONGS_SHELF_ID } from "./catalog";
import type { ListenLibraryShelf } from "./types";
import {
  isListenWorkspaceOnlinePlaylistsRoute,
  resolveListenWorkspaceViewMode,
  resolveMusicWorkspaceRoute,
  resolveMusicWorkspaceScopeRoute,
  resolveListenWorkspaceBrowseSource,
  selectListenWorkspaceHomeShelves,
  shouldLoadListenWorkspaceBrowse,
} from "./workspace-routes";

function shelf(id: string): ListenLibraryShelf {
  return {
    id,
    title: id,
    kind: "tracks",
    continuation: "",
    browseId: "",
    params: "",
    tracks: [],
    playlists: [],
    categories: [],
    artists: [],
  };
}

describe("music workspace routes", () => {
  test("resolves every visible route to an explicit scope and mode", () => {
    expect(resolveMusicWorkspaceRoute("search")).toMatchObject({
      scope: "online",
      mode: "muse",
      content: "search",
      browseSource: "home",
    });
    expect(resolveMusicWorkspaceRoute("radio")).toEqual({
      scope: "online",
      mode: "hush",
      content: "radio",
    });
    expect(resolveMusicWorkspaceRoute("podcasts")).toMatchObject({
      scope: "online",
      mode: "muse",
      browseSource: "podcasts",
    });
    expect(resolveMusicWorkspaceRoute("recent")).toMatchObject({
      scope: "online",
      mode: "muse",
      browseSource: "recent",
    });
    expect(resolveMusicWorkspaceRoute("local-search")).toEqual({
      scope: "local",
      mode: "linger",
      content: "local-search",
    });
    expect(resolveMusicWorkspaceRoute("local-home")).toEqual({
      scope: "local",
      mode: "linger",
      content: "local-home",
    });
    expect(resolveMusicWorkspaceRoute("artists")).toEqual({
      scope: "local",
      mode: "linger",
      content: "local-library",
    });
  });

  test("uses the route as the synchronous workspace view source", () => {
    expect(
      resolveListenWorkspaceViewMode({
        workspaceLayout: true,
        workspaceRouteId: "local-home",
        fallbackMode: "muse",
      }),
    ).toBe("linger");
    expect(
      resolveListenWorkspaceViewMode({
        workspaceLayout: false,
        workspaceRouteId: "local-home",
        fallbackMode: "muse",
      }),
    ).toBe("muse");
  });

  test("keeps a local primary view isolated from restored online playback and fetches", () => {
    const viewMode = resolveListenWorkspaceViewMode({
      workspaceLayout: true,
      workspaceRouteId: "local-home",
      fallbackMode: "muse",
    });
    const restoredPlaybackMode = "muse";
    let onlineBrowseFetchCount = 0;
    for (const fetchKind of ["library", "search", "artist", "playlist"]) {
      void fetchKind;
      if (
        shouldLoadListenWorkspaceBrowse({
          active: true,
          viewMode,
          targetMode: "muse",
        })
      ) {
        onlineBrowseFetchCount += 1;
      }
    }

    expect(viewMode).toBe("linger");
    expect(restoredPlaybackMode).toBe("muse");
    expect(onlineBrowseFetchCount).toBe(0);
    expect(
      shouldLoadListenWorkspaceBrowse({
        active: false,
        viewMode: "muse",
        targetMode: "muse",
      }),
    ).toBe(false);
  });

  test("never restores a remembered route from the opposite source scope", () => {
    expect(resolveMusicWorkspaceScopeRoute("local", "home")).toBe(
      "local-home",
    );
    expect(resolveMusicWorkspaceScopeRoute("online", "songs")).toBe("home");
    expect(resolveMusicWorkspaceScopeRoute("local", "albums")).toBe("albums");
    expect(resolveMusicWorkspaceScopeRoute("online", "history")).toBe(
      "history",
    );
  });

  test("wires the source switch and primary view to the route-derived helpers", async () => {
    const [listenSource, mainSource] = await Promise.all([
      Bun.file(new URL("../Listen.tsx", import.meta.url)).text(),
      Bun.file(new URL("../MainApp.tsx", import.meta.url)).text(),
    ]);

    expect(listenSource).toContain("resolveListenWorkspaceViewMode({");
    expect(listenSource).toContain("mode: activeViewMode");
    expect(listenSource).toContain("modeRef.current = activeViewMode");
    expect(listenSource).toContain("setLegacyBrowseMode");
    expect(listenSource).toContain("setMode: setLegacyBrowseMode");
    expect(
      listenSource.match(/props\.active \? "active" : "inactive"/g),
    ).toHaveLength(4);
    expect(listenSource.match(/!museBrowseActive/g)?.length ?? 0).toBeGreaterThanOrEqual(9);
    expect(listenSource.match(/!hushBrowseActive/g)?.length ?? 0).toBeGreaterThanOrEqual(3);
    expect(mainSource).toContain("resolveMusicWorkspaceScopeRoute(");
    expect(mainSource).toContain("const musicWorkspaceRouteId =");
    expect(mainSource).toContain("workspaceRouteId={musicWorkspaceRouteId}");
  });

  test("uses the cached Home response for liked music but keeps History separate", () => {
    expect(resolveListenWorkspaceBrowseSource("liked-music")).toBe("home");
    expect(resolveListenWorkspaceBrowseSource("history")).toBe("history");
  });

  test("treats the online playlist page as a dedicated workspace projection", () => {
    expect(resolveMusicWorkspaceRoute("online-playlists")).toMatchObject({
      scope: "online",
      mode: "muse",
      content: "playlists",
      browseSource: "playlists",
    });
    expect(resolveListenWorkspaceBrowseSource("online-playlists")).toBe(
      "playlists",
    );
    expect(isListenWorkspaceOnlinePlaylistsRoute(true, "online-playlists")).toBe(
      true,
    );
    expect(isListenWorkspaceOnlinePlaylistsRoute(false, "online-playlists")).toBe(
      false,
    );
  });

  test("shows only the liked songs shelf on the liked-music route", () => {
    const shelves = [
      shelf("quick-picks"),
      shelf(LISTEN_LIKED_SONGS_SHELF_ID),
      shelf("listen-again"),
    ];

    expect(
      selectListenWorkspaceHomeShelves(shelves, true, "liked-music").map(
        (item) => item.id,
      ),
    ).toEqual([LISTEN_LIKED_SONGS_SHELF_ID]);
    expect(selectListenWorkspaceHomeShelves(shelves, true, "home")).toBe(
      shelves,
    );
    expect(selectListenWorkspaceHomeShelves(shelves, false, "liked-music")).toBe(
      shelves,
    );
  });
});
