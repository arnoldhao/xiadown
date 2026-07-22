import { describe, expect, test } from "bun:test";

describe("OS notification HMR lifecycle", () => {
  test("disposes the active-window tracker before the module is replaced", async () => {
    const source = await Bun.file(new URL("./os-notification.ts", import.meta.url)).text();

    expect(source).toContain("activeTrackerCleanup = cleanup");
    expect(source).toContain("activeTrackerCleanup?.()");
    expect(source).toContain("import.meta.hot.dispose(stopAppActiveTracker)");
  });
});
