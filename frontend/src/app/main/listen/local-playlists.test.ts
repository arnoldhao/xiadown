import { describe, expect, test } from "bun:test";

import {
  createListenLocalPlaylistsClient,
  createListenLocalPlaylistsReloadCoordinator,
  isListenLocalPlaylistRevisionConflict,
  runListenLocalPlaylistMutation,
  type ListenLocalPlaylist,
} from "./local-playlists";

function playlist(id: string): ListenLocalPlaylist {
  return {
    id,
    name: id,
    revision: 1,
    itemCount: 0,
    createdAt: "",
    updatedAt: "",
  };
}

function deferred<T>() {
  let resolve!: (value: T) => void;
  let reject!: (reason?: unknown) => void;
  const promise = new Promise<T>((resolvePromise, rejectPromise) => {
    resolve = resolvePromise;
    reject = rejectPromise;
  });
  return { promise, resolve, reject };
}

describe("local playlists client", () => {
  test("maps CRUD and ordered track mutations to the playlist REST contract", async () => {
    const calls: Array<{ url: string; method: string; body: string }> = [];
    const fetcher = (async (input: RequestInfo | URL, init?: RequestInit) => {
      const url = String(input);
      const method = init?.method ?? "GET";
      calls.push({ url, method, body: String(init?.body ?? "") });
      if (method === "DELETE" && !url.includes("/items?")) {
        return new Response(null, { status: 204 });
      }
      if (url.endsWith("/playlists") && method === "GET") {
        return Response.json({
          items: [{ id: "p1", name: "Mix", revision: 1, itemCount: 0 }],
        });
      }
      if (url.includes("/items")) {
        return Response.json({
          playlist: { id: "p1", name: "Mix", itemCount: 0 },
          items: [],
        });
      }
      if (method === "GET") {
        return Response.json({
          playlist: { id: "p1", name: "Mix", itemCount: 2 },
          items: [
            {
              id: "item-b",
              position: 1,
              track: {
                id: "b",
                fileId: "b",
                title: "Second",
                localPath: "/b.mp3",
              },
            },
            {
              id: "item-a",
              position: 0,
              track: {
                id: "a",
                fileId: "a",
                title: "First",
                localPath: "/a.mp3",
              },
            },
          ],
        });
      }
      return Response.json({ id: "p1", name: "Mix", itemCount: 0 });
    }) as typeof fetch;
    const client = createListenLocalPlaylistsClient(
      "http://127.0.0.1:34115/",
      fetcher,
    );

    expect(await client.list()).toHaveLength(1);
    const detail = await client.get("p1");
    expect(detail.playlist.name).toBe("Mix");
    expect(detail.items.map((item) => item.track.id)).toEqual(["a", "b"]);
    expect(detail.items.map((item) => item.id)).toEqual(["item-a", "item-b"]);
    await client.create("Mix");
    await client.rename("p1", "Renamed", 7);
    await client.addTracks("p1", ["a", "b"], 7);
    await client.reorder("p1", ["item-b", "item-a"], 7);
    await client.removeTrack("p1", "item/a", 7);
    await client.remove("p1", 7);

    expect(calls.map((call) => [call.method, call.url])).toEqual([
      ["GET", "http://127.0.0.1:34115/api/listen/local/playlists"],
      ["GET", "http://127.0.0.1:34115/api/listen/local/playlists/p1"],
      ["POST", "http://127.0.0.1:34115/api/listen/local/playlists"],
      ["PATCH", "http://127.0.0.1:34115/api/listen/local/playlists/p1"],
      ["POST", "http://127.0.0.1:34115/api/listen/local/playlists/p1/items"],
      ["PUT", "http://127.0.0.1:34115/api/listen/local/playlists/p1/items"],
      [
        "DELETE",
        "http://127.0.0.1:34115/api/listen/local/playlists/p1/items?itemId=item%2Fa&expectedRevision=7",
      ],
      ["DELETE", "http://127.0.0.1:34115/api/listen/local/playlists/p1?expectedRevision=7"],
    ]);
    expect(JSON.parse(calls[3]?.body ?? "{}")).toEqual({
      name: "Renamed",
      expectedRevision: 7,
    });
    expect(JSON.parse(calls[4]?.body ?? "{}")).toEqual({
      fileIds: ["a", "b"],
      expectedRevision: 7,
    });
    expect(JSON.parse(calls[5]?.body ?? "{}")).toEqual({
      itemIds: ["item-b", "item-a"],
      expectedRevision: 7,
    });
  });

  test("preserves duplicate Tracks as independently addressable playlist Items", async () => {
    const client = createListenLocalPlaylistsClient(
      "http://localhost",
      (async () => Response.json({
        playlist: { id: "p1", name: "Duplicates", revision: 3, itemCount: 2 },
        items: [
          { id: "item-one", position: 0, track: { id: "track-a", fileId: "track-a", title: "A" } },
          { id: "item-two", position: 1, track: { id: "track-a", fileId: "track-a", title: "A" } },
        ],
      })) as typeof fetch,
    );

    const detail = await client.get("p1");
    expect(detail.items.map((item) => item.track.id)).toEqual(["track-a", "track-a"]);
    expect(detail.items.map((item) => item.id)).toEqual(["item-one", "item-two"]);
  });

  test("uses Item identity for playlist row keys, reorder, and remove actions", async () => {
    const source = await Bun.file(
      new URL("./LocalLibraryWorkspace.tsx", import.meta.url),
    ).text();

    expect(source).toContain("const currentItemIds = playlistDetail.items.map((item) => item.id)");
    expect(source).toContain("const expectedRevision = playlistDetail.playlist.revision");
    expect(source).toContain("localPlaylists.rename(playlistId, name, expectedRevision)");
    expect(source).toContain("localPlaylists.remove(playlistId, expectedRevision)");
    expect(source).toContain("[...selectedAddTrackIds],\n        expectedRevision,");
    expect(source).toContain("localPlaylists.reorder(playlistId, nextItemIds, expectedRevision)");
    expect(source).toContain("rowIds={visiblePlaylistItems.map((item) => item.id)}");
    expect(source).toContain("localPlaylists.removeTrack(playlistId, itemId, expectedRevision)");
    expect(source).toContain("filterListenLocalWorkspaceTracks(\n          props.tracks,");
    expect(source).not.toContain("existingPlaylistTrackIds");
    expect(source).not.toContain("const currentFileIds = playlistDetail.items");
  });

  test("surfaces the backend error message", async () => {
    const client = createListenLocalPlaylistsClient(
      "http://localhost",
      (async () =>
        Response.json(
          { error: { message: "playlist name is invalid" } },
          { status: 400 },
        )) as typeof fetch,
    );

    await expect(client.create("")).rejects.toThrow("playlist name is invalid");
  });

  test("preserves structured revision conflicts for refresh-and-retry handling", async () => {
    const client = createListenLocalPlaylistsClient(
      "http://localhost",
      (async () =>
        Response.json(
          {
            error: {
              code: "playlist_revision_conflict",
              message: "Listen Local Music revision conflict",
            },
          },
          { status: 409 },
        )) as typeof fetch,
    );

    let captured: unknown;
    try {
      await client.rename("p1", "Renamed", 2);
    } catch (error) {
      captured = error;
    }
    expect(isListenLocalPlaylistRevisionConflict(captured)).toBe(true);
    expect(captured).toBeInstanceOf(Error);
    expect((captured as Error).message).toBe("Listen Local Music revision conflict");
  });

  test("keeps a successful mutation resolved when its background list refresh fails", async () => {
    const methods: string[] = [];
    let finishRefresh!: () => void;
    const refreshFinished = new Promise<void>((resolve) => {
      finishRefresh = resolve;
    });
    const client = createListenLocalPlaylistsClient(
      "http://localhost",
      (async (_input, init) => {
        const method = init?.method ?? "GET";
        methods.push(method);
        if (method === "POST") {
          return Response.json({ id: "p1", name: "Mix", itemCount: 0 });
        }
        return Response.json(
          { error: { message: "error.refresh" } },
          { status: 500 },
        );
      }) as typeof fetch,
    );
    let notifications = 0;

    const created = await runListenLocalPlaylistMutation(
      () => client.create("Mix"),
      async () => {
        try {
          await client.list();
        } finally {
          finishRefresh();
        }
      },
      () => {
        notifications += 1;
      },
    );
    await refreshFinished;

    expect(created).toMatchObject({ id: "p1", name: "Mix" });
    expect(methods).toEqual(["POST", "GET"]);
    expect(notifications).toBe(1);
  });

  test("ignores an older reload response that settles after the latest response", async () => {
    const first = deferred<ListenLocalPlaylist[]>();
    const second = deferred<ListenLocalPlaylist[]>();
    let requestCount = 0;
    const coordinator = createListenLocalPlaylistsReloadCoordinator();
    let currentPlaylists: ListenLocalPlaylist[] = [];
    let loading = false;
    const target = {
      setPlaylists(next: ListenLocalPlaylist[]) {
        currentPlaylists = next;
      },
      setLoading(next: boolean) {
        loading = next;
      },
    };
    const client = {
      list: () => (requestCount++ === 0 ? first.promise : second.promise),
    };

    const olderReload = coordinator.reload(client, target);
    const latestReload = coordinator.reload(client, target);
    expect(loading).toBe(true);

    second.resolve([playlist("latest")]);
    await latestReload;
    expect(currentPlaylists.map((item) => item.id)).toEqual(["latest"]);
    expect(loading).toBe(false);

    first.resolve([playlist("older")]);
    await olderReload;
    expect(currentPlaylists.map((item) => item.id)).toEqual(["latest"]);
    expect(loading).toBe(false);
  });
});
