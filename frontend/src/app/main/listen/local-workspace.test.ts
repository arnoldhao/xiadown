import { describe, expect, test } from "bun:test";

import {
  buildListenLocalAlbumGroups,
  buildListenLocalArtistGroups,
  buildListenLocalPlaybackQueueIds,
  filterListenLocalWorkspaceTracks,
  moveListenLocalPlaylistTrack,
  parseListenLocalWorkspaceRoute,
  sortListenLocalSongs,
  sortListenLocalTracksByRecent,
  resolveListenLocalPlaybackQueue,
} from "./local-workspace";
import type { ListenLocalItem } from "./types";

function track(
  id: string,
  overrides: Partial<ListenLocalItem> = {},
): ListenLocalItem {
  return {
    id,
    title: `Song ${id}`,
    author: "",
    album: "",
    albumArtist: "",
    genre: "",
    trackNumber: 0,
    discNumber: 0,
    year: 0,
    lyricsTitle: "",
    lyricsArtist: "",
    path: `/music/${id}.mp3`,
    previewURL: "",
    durationLabel: "3:00",
    durationSeconds: 180,
    coverURL: `${id}.jpg`,
    format: "mp3",
    audioCodec: "mp3",
    sizeBytes: 1024,
    metadataWritable: true,
    playbackSupported: true,
    playbackUnsupportedReason: "",
    probeError: "",
    modTimeUnix: 0,
    createdAtUnix: 0,
    ...overrides,
  };
}

describe("local workspace routes and collections", () => {
  test("parses supported library and playlist routes", () => {
    expect(parseListenLocalWorkspaceRoute("local-home")).toEqual({
      kind: "home",
    });
    expect(parseListenLocalWorkspaceRoute("local-search")).toEqual({
      kind: "search",
    });
    expect(parseListenLocalWorkspaceRoute("recently-added")).toEqual({
      kind: "recently-added",
    });
    expect(parseListenLocalWorkspaceRoute("playlist: abc ")).toEqual({
      kind: "playlist",
      playlistId: "abc",
    });
    expect(parseListenLocalWorkspaceRoute("playlist:")).toBeNull();
    expect(parseListenLocalWorkspaceRoute("home")).toBeNull();
  });

  test("sorts recently added by created time with modified fallback", () => {
    const items = [
      track("old", { createdAtUnix: 10, modTimeUnix: 100 }),
      track("fallback", { modTimeUnix: 30 }),
      track("new", { createdAtUnix: 40 }),
    ];

    expect(sortListenLocalTracksByRecent(items).map((item) => item.id)).toEqual([
      "new",
      "fallback",
      "old",
    ]);
    expect(items.map((item) => item.id)).toEqual(["old", "fallback", "new"]);
  });

  test("builds clickable artist and album groups with track ordering", () => {
    const items = [
      track("b", {
        title: "Second",
        author: "Artist",
        album: "Album",
        trackNumber: 2,
      }),
      track("a", {
        title: "First",
        author: "Artist",
        album: "Album",
        trackNumber: 1,
      }),
      track("unknown", { title: "Loose" }),
    ];
    const artists = buildListenLocalArtistGroups(items, "Unknown artist");
    const albums = buildListenLocalAlbumGroups(items, "Unknown album");

    expect(artists.map((group) => group.title)).toEqual([
      "Artist",
      "Unknown artist",
    ]);
    expect(artists[0]?.tracks.map((item) => item.id)).toEqual(["a", "b"]);
    expect(albums.find((group) => group.title === "Album")?.tracks.map(
      (item) => item.id,
    )).toEqual(["a", "b"]);
  });

  test("filters metadata, sorts songs, and reorders playlist ids", () => {
    const items = [
      track("z", { title: "Zulu", album: "Night" }),
      track("a", { title: "Alpha", genre: "Ambient" }),
    ];

    expect(
      filterListenLocalWorkspaceTracks(items, "ambient").map((item) => item.id),
    ).toEqual(["a"]);
    expect(sortListenLocalSongs(items).map((item) => item.id)).toEqual([
      "a",
      "z",
    ]);
    expect(moveListenLocalPlaylistTrack(["a", "b", "c"], "b", -1)).toEqual([
      "b",
      "a",
      "c",
    ]);
    expect(moveListenLocalPlaylistTrack(["a", "b", "c"], "c", 1)).toEqual([
      "a",
      "b",
      "c",
    ]);
  });

  test("keeps a collection playback queue ordered and removes missing tracks", () => {
    const items = [track("a"), track("b"), track("c")];
    const queueIds = buildListenLocalPlaybackQueueIds("b", [
      { id: "c" },
      { id: "c" },
      { id: "a" },
    ]);

    expect(queueIds).toEqual(["b", "c", "a"]);
    expect(
      resolveListenLocalPlaybackQueue(items, ["c", "missing", "a"]).map(
        (item) => item.id,
      ),
    ).toEqual(["c", "a"]);
    expect(
      resolveListenLocalPlaybackQueue(items, []).map((item) => item.id),
    ).toEqual([]);
    expect(
      resolveListenLocalPlaybackQueue(items, null).map((item) => item.id),
    ).toEqual(["a", "b", "c"]);
  });
});
