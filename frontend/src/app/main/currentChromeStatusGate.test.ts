import { describe, expect, test } from "bun:test";

import type { CurrentResourceSniffBrowserStatus } from "@/shared/contracts/library";

import { canUseCurrentChrome } from "./currentChromeStatusGate";

const readyStatus: CurrentResourceSniffBrowserStatus = {
  browserId: "chrome",
  state: "ready",
  installed: true,
  running: true,
  supported: true,
  ready: true,
  minimumVersion: 144,
};

describe("current Chrome status gate", () => {
  test("keeps an old cached ready result disabled while this entry refreshes", () => {
    expect(
      canUseCurrentChrome({
        data: readyStatus,
        dataUpdatedAt: 100,
        entryDataUpdatedAt: 100,
        isFetching: true,
        isError: false,
        isRefetchError: false,
      }),
    ).toBeFalse();
  });

  test("fails closed after a refresh error even when ready data remains cached", () => {
    expect(
      canUseCurrentChrome({
        data: readyStatus,
        dataUpdatedAt: 200,
        entryDataUpdatedAt: 100,
        isFetching: false,
        isError: true,
        isRefetchError: true,
      }),
    ).toBeFalse();
  });

  test("enables only after this entry receives a newer successful ready result", () => {
    expect(
      canUseCurrentChrome({
        data: readyStatus,
        dataUpdatedAt: 201,
        entryDataUpdatedAt: 200,
        isFetching: false,
        isError: false,
        isRefetchError: false,
      }),
    ).toBeTrue();
  });
});
