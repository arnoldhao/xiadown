import { describe, expect, test } from "bun:test";

import type { ListenPlaylistItem } from "./types";
import {
  mergeListenLibraryPagePlaylists,
  shouldAutoLoadListenLibraryPage,
} from "./library-pagination";

function playlist(id: string, title = id): ListenPlaylistItem {
  return {
    id: `item-${id}`,
    playlistId: id,
    title,
    channel: "YouTube Music",
    description: "",
  };
}

describe("online library pagination", () => {
  test("appends and deduplicates playlist continuation pages", () => {
    const firstPage = [playlist("PL-one"), playlist("PL-shared", "First")];
    const continuation = [
      playlist("PL-shared", "Duplicate"),
      playlist("PL-two"),
    ];

    expect(
      mergeListenLibraryPagePlaylists(
        "playlists",
        firstPage,
        continuation,
      ).map((item) => [item.playlistId, item.title]),
    ).toEqual([
      ["PL-one", "PL-one"],
      ["PL-shared", "First"],
      ["PL-two", "PL-two"],
    ]);
  });

  test("does not change Home or other browse page playlist projections", () => {
    const current = [playlist("PL-home")];
    expect(
      mergeListenLibraryPagePlaylists("home", current, [playlist("PL-next")]),
    ).toBe(current);
  });

  test("enables automatic pagination while keeping search and liked routes excluded", () => {
    expect(
      shouldAutoLoadListenLibraryPage({
        normalizedQuery: "",
        continuation: "playlist-next",
        likedMusicWorkspaceRoute: false,
        workspaceSearchRoute: false,
      }),
    ).toBe(true);
    expect(
      shouldAutoLoadListenLibraryPage({
        normalizedQuery: "query",
        continuation: "playlist-next",
        likedMusicWorkspaceRoute: false,
        workspaceSearchRoute: false,
      }),
    ).toBe(false);
    expect(
      shouldAutoLoadListenLibraryPage({
        normalizedQuery: "",
        continuation: "playlist-next",
        likedMusicWorkspaceRoute: true,
        workspaceSearchRoute: false,
      }),
    ).toBe(false);
  });
});
