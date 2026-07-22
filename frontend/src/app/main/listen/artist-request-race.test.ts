import { describe, expect, test } from "bun:test";

import {
  beginListenArtistRequest,
  createListenArtistIdentity,
  createListenArtistRequestRegistry,
  finishListenArtistRequest,
  invalidateListenArtistRequests,
  isListenArtistRequestCurrent,
  isListenArtistShelfViewRequestCurrent,
  type ListenArtistShelfViewRequest,
} from "./artist-request-race";

function deferred<T>() {
  let resolve!: (value: T) => void;
  const promise = new Promise<T>((nextResolve) => {
    resolve = nextResolve;
  });
  return { promise, resolve };
}

describe("listen artist request race guards", () => {
  test("rejects a late shelf response when two artists share the same shelf id", async () => {
    const artistA = createListenArtistIdentity({ id: "artist-a", name: "A" });
    const artistB = createListenArtistIdentity({ id: "artist-b", name: "B" });
    const registry = createListenArtistRequestRegistry();
    const sharedShelfId = "top-songs";
    const responseA = deferred<string[]>();
    const responseB = deferred<string[]>();
    let activeArtistIdentity = artistA;
    let generation = 1;
    let view: ListenArtistShelfViewRequest & { tracks: string[] } = {
      artistIdentity: artistA,
      generation,
      shelfId: sharedShelfId,
      tracks: [],
    };
    const requestA: ListenArtistShelfViewRequest = { ...view };
    const networkRequestA = beginListenArtistRequest(
      registry,
      "shelf",
      artistA,
    );
    const commitA = responseA.promise.then((tracks) => {
      if (
        isListenArtistRequestCurrent(registry, networkRequestA) &&
        isListenArtistShelfViewRequestCurrent(
          view,
          requestA,
          activeArtistIdentity,
        )
      ) {
        view = { ...view, tracks };
      }
      finishListenArtistRequest(registry, networkRequestA);
    });

    activeArtistIdentity = artistB;
    invalidateListenArtistRequests(registry, artistB);
    generation += 1;
    view = {
      artistIdentity: artistB,
      generation,
      shelfId: sharedShelfId,
      tracks: [],
    };
    const requestB: ListenArtistShelfViewRequest = { ...view };
    const networkRequestB = beginListenArtistRequest(
      registry,
      "shelf",
      artistB,
    );
    const commitB = responseB.promise.then((tracks) => {
      if (
        isListenArtistRequestCurrent(registry, networkRequestB) &&
        isListenArtistShelfViewRequestCurrent(
          view,
          requestB,
          activeArtistIdentity,
        )
      ) {
        view = { ...view, tracks };
      }
      finishListenArtistRequest(registry, networkRequestB);
    });

    expect(networkRequestA.controller.signal.aborted).toBe(true);
    responseA.resolve(["track-a"]);
    await commitA;
    expect(view.tracks).toEqual([]);

    responseB.resolve(["track-b"]);
    await commitB;
    expect(view.tracks).toEqual(["track-b"]);
  });

  test("a late artist mix neither starts playback nor clears the next artist busy state", async () => {
    const registry = createListenArtistRequestRegistry();
    const artistA = createListenArtistIdentity({ id: "artist-a", name: "A" });
    const artistB = createListenArtistIdentity({ id: "artist-b", name: "B" });
    const responseA = deferred<string[]>();
    const responseB = deferred<string[]>();
    const played: string[] = [];
    let busyArtistIdentity = "";

    const requestA = beginListenArtistRequest(registry, "action", artistA);
    busyArtistIdentity = artistA;
    const settleA = responseA.promise
      .then((tracks) => {
        if (isListenArtistRequestCurrent(registry, requestA)) {
          played.push(...tracks);
        }
      })
      .finally(() => {
        if (isListenArtistRequestCurrent(registry, requestA)) {
          busyArtistIdentity = "";
        }
        finishListenArtistRequest(registry, requestA);
      });

    invalidateListenArtistRequests(registry, artistB);
    const requestB = beginListenArtistRequest(registry, "action", artistB);
    busyArtistIdentity = artistB;
    const settleB = responseB.promise
      .then((tracks) => {
        if (isListenArtistRequestCurrent(registry, requestB)) {
          played.push(...tracks);
        }
      })
      .finally(() => {
        if (isListenArtistRequestCurrent(registry, requestB)) {
          busyArtistIdentity = "";
        }
        finishListenArtistRequest(registry, requestB);
      });

    expect(requestA.controller.signal.aborted).toBe(true);
    responseA.resolve(["mix-a"]);
    await settleA;
    expect(played).toEqual([]);
    expect(busyArtistIdentity).toBe(artistB);

    responseB.resolve(["mix-b"]);
    await settleB;
    expect(played).toEqual(["mix-b"]);
    expect(busyArtistIdentity).toBe("");
  });
});
