import { describe, expect, test } from "bun:test";

import {
  abortStaleListenPaginationRequests,
  beginListenPaginationRequest,
  createListenPaginationContextKey,
  finishListenPaginationRequest,
  isListenPaginationContextCurrent,
  resolveListenNextContinuation,
} from "./pagination-race";

describe("listen pagination race guards", () => {
  test("synchronously rejects a duplicate context and continuation", () => {
    const requests = new Map();
    const context = createListenPaginationContextKey(["search", "aurora"]);
    const first = beginListenPaginationRequest(
      requests,
      "search",
      context,
      "next-page",
    );

    expect(first).not.toBeNull();
    expect(
      beginListenPaginationRequest(
        requests,
        "search",
        context,
        "next-page",
      ),
    ).toBeNull();

    finishListenPaginationRequest(requests, first!);
    expect(
      beginListenPaginationRequest(
        requests,
        "search",
        context,
        "next-page",
      ),
    ).not.toBeNull();
  });

  test("aborts an old response before a context changes away and back", () => {
    const requests = new Map();
    const first = beginListenPaginationRequest(
      requests,
      "playlist",
      "playlist:a",
      "next",
    );
    expect(first).not.toBeNull();

    abortStaleListenPaginationRequests(
      requests,
      "playlist",
      "playlist:b",
    );
    expect(first!.controller.signal.aborted).toBe(true);
    expect(requests.size).toBe(0);
    expect(
      beginListenPaginationRequest(
        requests,
        "playlist",
        "playlist:a",
        "next",
      ),
    ).not.toBeNull();
    expect(
      isListenPaginationContextCurrent("playlist:a", "playlist:b"),
    ).toBe(false);
  });

  test("stops when a server repeats the requested continuation", () => {
    expect(resolveListenNextContinuation("page-2", "page-2")).toBe("");
    expect(resolveListenNextContinuation(" page-2 ", "page-2 ")).toBe("");
    expect(resolveListenNextContinuation("page-2", "page-3")).toBe(
      "page-3",
    );
    expect(resolveListenNextContinuation("page-2", "")).toBe("");
  });

  test("allows the same continuation in different contexts", () => {
    const requests = new Map();

    expect(
      beginListenPaginationRequest(
        requests,
        "artist",
        "artist:a",
        "next",
      ),
    ).not.toBeNull();
    expect(
      beginListenPaginationRequest(
        requests,
        "artist",
        "artist:b",
        "next",
      ),
    ).not.toBeNull();
  });
});
