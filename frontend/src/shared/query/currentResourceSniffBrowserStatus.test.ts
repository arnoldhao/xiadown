import { describe, expect, test } from "bun:test";

import { normalizeCurrentResourceSniffBrowserStatus } from "./currentResourceSniffBrowserStatus";

describe("current resource Sniff browser status adapter", () => {
  test("preserves the stable ready contract and optional profile identity", () => {
    expect(
      normalizeCurrentResourceSniffBrowserStatus({
        browserId: "Chrome",
        state: "ready",
        installed: true,
        running: true,
        supported: true,
        ready: true,
        version: "145.0.0",
        minimumVersion: 144,
        profileName: "Personal",
        detail: "/private/backend/diagnostic",
      }),
    ).toEqual({
      browserId: "chrome",
      state: "ready",
      installed: true,
      running: true,
      supported: true,
      ready: true,
      version: "145.0.0",
      minimumVersion: 144,
      profileName: "Personal",
    });
  });

  test("fails closed for unknown states and never trusts a mismatched ready flag", () => {
    expect(
      normalizeCurrentResourceSniffBrowserStatus({
        state: "private_backend_state",
        ready: true,
      }),
    ).toMatchObject({
      browserId: "chrome",
      state: "endpoint_unavailable",
      ready: false,
    });
    expect(
      normalizeCurrentResourceSniffBrowserStatus({
        state: "remote_debugging_disabled",
        installed: true,
        running: true,
        supported: true,
        ready: true,
      }),
    ).toMatchObject({
      state: "remote_debugging_disabled",
      running: true,
      ready: false,
    });
  });
});
