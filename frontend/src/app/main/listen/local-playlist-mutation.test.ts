import { describe, expect, test } from "bun:test";

import {
  createListenLocalPlaylistMutationGuard,
  type ListenLocalPlaylistMutationContext,
} from "./local-playlist-mutation";

function deferred<T>() {
  let resolve!: (value: T) => void;
  const promise = new Promise<T>((resolvePromise) => {
    resolve = resolvePromise;
  });
  return { promise, resolve };
}

describe("local playlist mutation context", () => {
  test("ignores a pending playlist mutation after switching playlists", async () => {
    const guard = createListenLocalPlaylistMutationGuard();
    let context: ListenLocalPlaylistMutationContext = {
      routeId: "playlist:a",
      playlistId: "a",
    };
    const request = guard.begin(context);
    const pending = deferred<string>();
    let visiblePlaylist = "b:initial";
    const completion = pending.promise.then((value) => {
      if (guard.isCurrent(request, context)) {
        visiblePlaylist = value;
      }
    });

    context = { routeId: "playlist:b", playlistId: "b" };
    guard.invalidate();
    pending.resolve("a:stale-update");
    await completion;

    expect(visiblePlaylist).toBe("b:initial");
  });

  test("does not navigate when a pending delete finishes after leaving the route", async () => {
    const guard = createListenLocalPlaylistMutationGuard();
    let context: ListenLocalPlaylistMutationContext = {
      routeId: "playlist:a",
      playlistId: "a",
    };
    const request = guard.begin(context);
    const pending = deferred<void>();
    let navigatedTo = "";
    const completion = pending.promise.then(() => {
      if (guard.isCurrent(request, context)) {
        navigatedTo = "songs";
      }
    });

    context = { routeId: "albums", playlistId: "" };
    guard.invalidate();
    pending.resolve();
    await completion;

    expect(navigatedTo).toBe("");
  });

  test("lets only the latest mutation write within the same playlist", async () => {
    const guard = createListenLocalPlaylistMutationGuard();
    const context = { routeId: "playlist:a", playlistId: "a" };
    const olderRequest = guard.begin(context);
    const latestRequest = guard.begin(context);

    expect(guard.isCurrent(olderRequest, context)).toBe(false);
    expect(guard.isCurrent(latestRequest, context)).toBe(true);
  });
});
