import { describe, expect, test } from "bun:test";

import {
  handleListenYouTubeAppSessionEvent,
  resolveListenYouTubeAppSessionTransition,
} from "./app-session-event";

describe("Listen YouTube app-session events", () => {
  test("refreshes the query when a disconnected account becomes connected", () => {
    expect(
      resolveListenYouTubeAppSessionTransition({
        data: { siteKey: "youtube", status: "connected" },
      }),
    ).toBe("connected");
  });

  test("treats another connected event as a refresh for account A to B", () => {
    expect(
      resolveListenYouTubeAppSessionTransition({
        siteKey: "YouTube",
        action: "finish",
        status: "connected",
      }),
    ).toBe("connected");
  });

  test("keeps clear events disconnected and ignores other sites", () => {
    expect(
      resolveListenYouTubeAppSessionTransition({
        siteKey: "youtube",
        action: "clear",
      }),
    ).toBe("disconnected");
    expect(
      resolveListenYouTubeAppSessionTransition({
        siteKey: "github",
        status: "connected",
      }),
    ).toBe("ignore");
  });

  test("refetches app sessions for every relevant event and never for ignored sites", () => {
    const calls: string[] = [];
    const handlers = {
      onConnected: () => calls.push("connected"),
      onDisconnected: () => calls.push("disconnected"),
      onReload: () => calls.push("reload"),
      onRefetch: () => calls.push("refetch"),
    };

    expect(
      handleListenYouTubeAppSessionEvent(
        { siteKey: "youtube", status: "connected" },
        handlers,
      ),
    ).toBe(true);
    expect(calls).toEqual(["connected", "reload", "refetch"]);

    calls.length = 0;
    expect(
      handleListenYouTubeAppSessionEvent(
        { siteKey: "github", status: "connected" },
        handlers,
      ),
    ).toBe(false);
    expect(calls).toEqual([]);
  });
});
