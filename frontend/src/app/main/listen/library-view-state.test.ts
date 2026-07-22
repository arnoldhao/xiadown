import { describe, expect, test } from "bun:test";

import {
  isListenLibraryPageRequestCurrent,
  isListenLibraryRequestReady,
  isSameListenArtistBrowseIdentity,
  resolveListenLibraryPageCacheKey,
  resolveListenLibraryViewPhase,
} from "./library-view-state";

describe("Listen library view state", () => {
  test("keeps a newly selected workspace source in loading until state catches up", () => {
    expect(
      resolveListenLibraryViewPhase({
        workspaceLayout: true,
        workspaceRouteId: "charts",
        mode: "muse",
        onlineBrowseSource: "home",
        accountConnected: true,
        requestReady: true,
        settled: true,
        loading: false,
        error: false,
        hasVisibleContent: false,
      }),
    ).toBe("loading");
  });

  test("allows an empty prompt only after the selected source has settled", () => {
    expect(
      resolveListenLibraryViewPhase({
        workspaceLayout: true,
        workspaceRouteId: "charts",
        mode: "muse",
        onlineBrowseSource: "charts",
        accountConnected: true,
        requestReady: true,
        settled: true,
        loading: false,
        error: false,
        hasVisibleContent: false,
      }),
    ).toBe("empty");
  });

  test("keeps request failures out of the empty phase", () => {
    expect(
      resolveListenLibraryViewPhase({
        workspaceLayout: true,
        workspaceRouteId: "home",
        mode: "muse",
        onlineBrowseSource: "home",
        accountConnected: true,
        requestReady: true,
        settled: true,
        loading: false,
        error: true,
        hasVisibleContent: false,
      }),
    ).toBe("error");
  });

  test("does not expose an Online empty state while another station mode is taking over", () => {
    expect(
      resolveListenLibraryViewPhase({
        workspaceLayout: true,
        workspaceRouteId: "radio",
        mode: "muse",
        onlineBrowseSource: "home",
        accountConnected: true,
        requestReady: true,
        settled: true,
        loading: false,
        error: false,
        hasVisibleContent: false,
      }),
    ).toBe("loading");
  });

  test("keeps the first connected render loading until its library request settles", () => {
    expect(
      resolveListenLibraryViewPhase({
        workspaceLayout: true,
        workspaceRouteId: "home",
        mode: "muse",
        onlineBrowseSource: "home",
        accountConnected: true,
        requestReady: true,
        settled: false,
        loading: false,
        error: false,
        hasVisibleContent: false,
      }),
    ).toBe("loading");
  });

  test("keeps a disconnected account in the account-gate phase", () => {
    expect(
      resolveListenLibraryViewPhase({
        workspaceLayout: true,
        workspaceRouteId: "home",
        mode: "muse",
        onlineBrowseSource: "home",
        accountConnected: false,
        requestReady: false,
        settled: false,
        loading: false,
        error: false,
        hasVisibleContent: false,
      }),
    ).toBe("disconnected");
  });

  test("waits for the local HTTP base before the first connected request", () => {
    expect(
      isListenLibraryRequestReady({
        accountConnected: true,
        httpBaseURL: "",
      }),
    ).toBe(false);
    expect(
      resolveListenLibraryViewPhase({
        workspaceLayout: true,
        workspaceRouteId: "home",
        mode: "muse",
        onlineBrowseSource: "home",
        accountConnected: true,
        requestReady: false,
        settled: false,
        loading: false,
        error: false,
        hasVisibleContent: false,
      }),
    ).toBe("loading");
    expect(
      isListenLibraryRequestReady({
        accountConnected: true,
        httpBaseURL: "http://127.0.0.1:34115/_xiadown/token",
      }),
    ).toBe(true);
  });

  test("rejects a completed request after navigation changes the cache key", () => {
    expect(
      isListenLibraryPageRequestCurrent({
        activeCacheKey: "source:charts:locale:en",
        requestCacheKey: "source:home:locale:en",
        aborted: false,
      }),
    ).toBe(false);
    expect(
      isListenLibraryPageRequestCurrent({
        activeCacheKey: "source:charts:locale:en",
        requestCacheKey: "source:charts:locale:en",
        aborted: false,
      }),
    ).toBe(true);
  });

  test("builds stable cache keys for sources and browse details", () => {
    expect(resolveListenLibraryPageCacheKey("home", null, "")).toBe(
      "source:home:locale:en",
    );
    expect(
      resolveListenLibraryPageCacheKey(
        "playlists",
        {
          id: "albums",
          source: "playlists",
          browseId: "  albums ",
          params: " mine ",
          title: "Albums",
        },
        "zh-CN",
      ),
    ).toBe("detail:playlists:zh-CN:albums:mine");
  });

  test("compares artist browse identities after trimming API values", () => {
    expect(
      isSameListenArtistBrowseIdentity(
        { id: " UC123 ", name: " Artist " },
        { id: "UC123", name: "Artist" },
      ),
    ).toBe(true);
    expect(
      isSameListenArtistBrowseIdentity(
        { id: "UC123", name: "Artist" },
        { id: "UC456", name: "Artist" },
      ),
    ).toBe(false);
  });
});
