import { describe, expect, it, mock } from "bun:test";

mock.module("@wailsio/runtime", () => ({
  Call: { ByName: () => Promise.resolve(undefined) },
}));

const { StationTelemetryDispatcher, trackStationWhenMainWindowActive } =
  await import("./station-events");

const settle = async (predicate: () => boolean) => {
  for (let attempt = 0; attempt < 50; attempt += 1) {
    if (predicate()) {
      return;
    }
    await new Promise((resolve) => setTimeout(resolve, 0));
  }
  throw new Error("timed out waiting for station telemetry");
};

describe("station telemetry", () => {
  it("normalizes and records each coarse station once per app session", async () => {
    const stations: string[] = [];
    const dispatcher = new StationTelemetryDispatcher((station) => {
      stations.push(station);
      return Promise.resolve(true);
    });

    dispatcher.track(" MUSIC ");
    dispatcher.track("music");
    dispatcher.track("custom/private/station");
    dispatcher.track("another-custom-station");
    dispatcher.track("   ");

    await settle(() => stations.length === 2);
    expect(stations).toEqual(["music", "other"]);
    dispatcher.dispose();
  });

  it("retries transient bridge startup failures in order", async () => {
    const calls: string[] = [];
    const delays: number[] = [];
    let failures = 2;
    const dispatcher = new StationTelemetryDispatcher(
      (station) => {
        calls.push(station);
        if (failures > 0) {
          failures -= 1;
          return Promise.reject(new Error("bridge not ready"));
        }
        return Promise.resolve(true);
      },
      (callback, delay) => {
        delays.push(delay);
        setTimeout(callback, 0);
        return delays.length;
      },
    );

    dispatcher.track("library");
    await settle(() => calls.length === 3);

    expect(calls).toEqual(["library", "library", "library"]);
    expect(delays).toEqual([100, 250]);
    dispatcher.dispose();
  });

  it("does not mark a native-rejected hidden station as reported", async () => {
    const calls: string[] = [];
    let accepted = false;
    const dispatcher = new StationTelemetryDispatcher(
      (station) => {
        calls.push(station);
        return Promise.resolve(accepted);
      },
      () => 1,
    );

    dispatcher.track("library");
    await settle(() => calls.length === 1);
    await Promise.resolve();
    accepted = true;
    dispatcher.track("library");
    await settle(() => calls.length === 2);

    expect(calls).toEqual(["library", "library"]);
    dispatcher.dispose();
  });

  it("tracks only a focused, visible main-window station", async () => {
    const tracked: string[] = [];
    const track = (station: string) => tracked.push(station);
    const surface = {
      visibilityState: "hidden" as DocumentVisibilityState,
      hasFocus: () => false,
    };

    expect(
      trackStationWhenMainWindowActive("library", surface, track),
    ).toBe(false);
    surface.visibilityState = "visible";
    expect(
      trackStationWhenMainWindowActive("library", surface, track),
    ).toBe(false);
    surface.hasFocus = () => true;
    expect(
      trackStationWhenMainWindowActive("library", surface, track),
    ).toBe(true);
    expect(tracked).toEqual(["library"]);

    const mainAppSource = await Bun.file(
      new URL("../../app/main/MainApp.tsx", import.meta.url),
    ).text();

    expect(mainAppSource).toContain(
      'import { trackStationWhenMainWindowActive } from "@/shared/telemetry/station-events";',
    );
    expect(mainAppSource).toContain(
      'window.addEventListener("focus", trackFocusedStation)',
    );
    expect(mainAppSource).toContain(
      'document.addEventListener("visibilitychange", trackFocusedStation)',
    );
    expect(
      mainAppSource.match(
        /trackStationWhenMainWindowActive\(activeWorkspaceId\)/g,
      ),
    ).toHaveLength(1);
  });
});
